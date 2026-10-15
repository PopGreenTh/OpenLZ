package delta

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/PopGreenTh/OpenLZ/internal/attribution"
	"github.com/PopGreenTh/OpenLZ/internal/taxonomy"
)

// Options tune a Sync run.
type Options struct {
	// Now stamps resolved_at, changed_at and the history. Zero means the current time.
	Now time.Time
	// Runner runs the attribution query; nil uses the store's own database.
	Runner attribution.Runner
}

// Sync brings the store up to date with the current cost lines and taxonomy snapshot:
//
//  1. data delta: classify every incoming line as NEW, AMOUNT (restated amount), KEYS (its
//     attribution inputs changed) or REMOVED (stored, inside the incoming date window, but
//     gone);
//  2. mapping delta: diff the previous and the new taxonomy snapshot;
//  3. select the lines to re-attribute (spec §8.3 step 3);
//  4. re-attribute them, moving the old result to prev_* and to the history;
//  5. report the movement.
//
// A mapping change is applied to all periods: the stored lines of every date are candidates.
// A run with no data and no mapping change writes nothing.
func (s *Store) Sync(ctx context.Context, snap *taxonomy.Snapshot, incoming []Line, opt Options) (*Report, error) {
	now := opt.Now
	if now.IsZero() {
		now = time.Now()
	}
	now = now.UTC().Truncate(time.Microsecond)
	run := opt.Runner
	if run == nil {
		run = s.runner
	}

	cat, err := taxonomy.Build(snap)
	if err != nil {
		return nil, fmt.Errorf("taxonomy: %w", err)
	}
	version := taxonomy.Version(snap)
	prevVersion, prevJSON, err := s.state(ctx)
	if err != nil {
		return nil, err
	}
	storedList, err := s.Lines(ctx)
	if err != nil {
		return nil, err
	}
	byKey := make(map[string]*StoredLine, len(storedList))
	for i := range storedList {
		byKey[storedList[i].Key] = &storedList[i]
	}
	rep := &Report{RunAt: now, TaxonomyVersion: version, PrevTaxonomyVersion: prevVersion}

	// Mapping delta.
	var cs taxonomy.ChangeSet
	fullResolve := false
	switch {
	case prevVersion == version:
	case prevVersion == "":
		fullResolve = len(storedList) > 0 // lines without a recorded taxonomy: nothing to diff against
	default:
		var prev taxonomy.Snapshot
		if err := json.Unmarshal([]byte(prevJSON), &prev); err != nil {
			fullResolve = true
		} else if cs, err = taxonomy.Diff(&prev, snap); err != nil {
			fullResolve = true
		}
	}

	// Data delta.
	dirty := map[string]*StoredLine{}
	selected := map[string]bool{}
	sort.Slice(incoming, func(i, j int) bool { return incoming[i].Key < incoming[j].Key })
	seen := make(map[string]bool, len(incoming))
	minDate, maxDate := "", ""
	for _, l := range incoming {
		seen[l.Key] = true
		if minDate == "" || l.UsageDate < minDate {
			minDate = l.UsageDate
		}
		if l.UsageDate > maxDate {
			maxDate = l.UsageDate
		}
		h := InputHash(l)
		st := byKey[l.Key]
		switch {
		case st == nil:
			st = &StoredLine{Line: l, InputHash: h, Active: true}
			byKey[l.Key] = st
			dirty[l.Key], selected[l.Key] = st, true
			rep.New++
		case !st.Active:
			st.Line, st.InputHash, st.Active = l, h, true
			dirty[l.Key], selected[l.Key] = st, true
			rep.New++
		case st.InputHash != h:
			st.Line, st.InputHash = l, h
			dirty[l.Key], selected[l.Key] = st, true
			rep.Keys++
		case st.Amount != l.Amount:
			st.Amount = l.Amount
			dirty[l.Key] = st
			rep.Amount++
		}
	}
	if len(incoming) > 0 {
		for _, st := range storedList2(byKey) {
			if st.Active && !seen[st.Key] && st.UsageDate >= minDate && st.UsageDate <= maxDate {
				st.Active = false
				dirty[st.Key] = st
				rep.Removed++
			}
		}
	}
	totalsBefore := activeTotals(byKey)

	// Select the lines to re-attribute.
	active := activeKeys(byKey)
	switch {
	case fullResolve || cs.NodeSetChanged():
		// A changed node set can change any line; resolution runs on distinct input
		// combinations, so re-resolving everything is cheap. Only changed results are written.
		for _, k := range active {
			selected[k] = true
		}
	case len(cs.Keys) > 0 || cs.AddedAnything():
		for _, k := range active {
			if selected[k] {
				continue
			}
			st := byKey[k]
			if touchesChange(st, cat, cs) || (cs.AddedAnything() && isUnresolved(st.ProjectHash)) {
				selected[k] = true
			}
		}
	}
	var sel []string
	for _, k := range active {
		if selected[k] {
			sel = append(sel, k)
		}
	}
	rep.Selected = len(sel)

	// Re-attribute.
	var history []HistoryRow
	if len(sel) > 0 {
		rows := make([]attribution.Row, len(sel))
		for i, k := range sel {
			rows[i] = byKey[k].Row
		}
		res, err := attribution.ResolveRows(ctx, run, cat, rows)
		if err != nil {
			return nil, err
		}
		moves := map[moveKey]*Movement{}
		for i, k := range sel {
			st, r := byKey[k], res[i].Result
			isNew := st.ProjectHash == ""
			if isNew || st.ProjectHash != r.ProjectHash || st.BizHash != r.BizHash {
				from := st.ProjectHash
				if !isNew {
					st.PrevProjectHash, st.PrevBizHash = st.ProjectHash, st.BizHash
					mk := moveKey{from, r.ProjectHash, st.Currency, st.Metric}
					if moves[mk] == nil {
						moves[mk] = &Movement{From: from, To: r.ProjectHash, Currency: st.Currency, Metric: st.Metric}
					}
					moves[mk].Lines++
					moves[mk].Amount += st.Amount
					if isUnresolved(from) || isUnresolved(r.ProjectHash) {
						rep.Unresolved = append(rep.Unresolved, LineMove{LineKey: k, From: from, To: r.ProjectHash,
							Currency: st.Currency, Amount: st.Amount})
					}
				}
				st.ProjectHash, st.BizHash, st.Trail = r.ProjectHash, r.BizHash, r.Trail
				st.TaxonomyVersion, st.ResolvedAt, st.ChangedAt = version, now, now
				dirty[k] = st
				history = append(history, HistoryRow{LineKey: k, TaxonomyVersion: version,
					ProjectHash: r.ProjectHash, BizHash: r.BizHash, ValidFrom: now})
				rep.Reattributed++
			} else if st.Trail != r.Trail {
				st.Trail, st.ResolvedAt = r.Trail, now
				dirty[k] = st
			}
		}
		for _, m := range moves {
			rep.Moved = append(rep.Moved, *m)
		}
	}
	rep.sort()

	// Write.
	stateChanged := prevVersion != version
	if len(dirty) > 0 || len(history) > 0 || stateChanged {
		snapJSON, err := snap.MarshalJSON()
		if err != nil {
			return nil, err
		}
		if err := s.write(ctx, dirty, history, stateChanged, version, string(snapJSON)); err != nil {
			return nil, err
		}
		rep.RowsWritten = len(dirty) + len(history)
		if stateChanged {
			rep.RowsWritten++
		}
	}

	// No silent loss: re-attribution moves cost, it never changes the total.
	totalsAfter, err := s.totals(ctx)
	if err != nil {
		return nil, err
	}
	for k, v := range totalsBefore {
		if math.Abs(totalsAfter[k]-v) > 1e-6*math.Max(1, math.Abs(v)) {
			return nil, fmt.Errorf("cost total for %s changed from %.6f to %.6f during attribution", k, v, totalsAfter[k])
		}
	}
	for k, v := range totalsAfter {
		if _, ok := totalsBefore[k]; !ok && math.Abs(v) > 1e-9 {
			return nil, fmt.Errorf("cost total for %s appeared during attribution (%.6f)", k, v)
		}
	}
	rep.Totals = totalsAfter
	return rep, nil
}

// storedList2 returns the stored lines in key order.
func storedList2(byKey map[string]*StoredLine) []*StoredLine {
	out := make([]*StoredLine, 0, len(byKey))
	for _, st := range byKey {
		out = append(out, st)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

func activeKeys(byKey map[string]*StoredLine) []string {
	var keys []string
	for k, st := range byKey {
		if st.Active {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	return keys
}

// activeTotals sums the amount of every active line per "currency/metric".
func activeTotals(byKey map[string]*StoredLine) map[string]float64 {
	out := map[string]float64{}
	for _, st := range storedList2(byKey) {
		if st.Active {
			out[st.Currency+"/"+st.Metric] += st.Amount
		}
	}
	return out
}

// isUnresolved reports whether the evidence did not reach the Project, App or Environment of
// a path (spec §4). Feature and Function have no Cloud counterpart, so FEAT-UNSPEC and
// FUNC-UNSPEC are the normal case and do not make a line unresolved.
func isUnresolved(hash string) bool {
	parts := strings.Split(strings.Trim(hash, "|"), "|")
	for i, p := range parts {
		if i < 3 && (strings.HasSuffix(p, "-UNSPEC") || strings.HasSuffix(p, "-UNALLOC")) {
			return true
		}
	}
	return false
}

// touchesChange reports whether a line's inputs match a key changed by the mapping delta, or
// whether its current path uses a node the delta removed or modified.
func touchesChange(st *StoredLine, cat *taxonomy.Catalog, cs taxonomy.ChangeSet) bool {
	r := st.Row
	co := cat.ResolveCloud(r.CloudInput())
	cand := []string{
		"resource:" + r.ResourceID,
		"account:" + r.AccountID, "account:" + co.Account.ID,
		"acctenv:" + r.AccountID, "acctenv:" + co.Account.ID,
		"connection:" + r.ConnectionID,
		"region:" + r.Region, "region:" + co.Region.ID,
		"rg:" + co.ResourceGroup.ID,
		"lz:" + co.LZ.ID,
		taxonomy.AliasKey("project", r.PrimaryTag), taxonomy.AliasKey("project", cat.CodeOf(taxonomy.Projects, r.PrimaryTag)),
		taxonomy.AliasKey("app", r.SecondaryTag), taxonomy.AliasKey("app", cat.CodeOf(taxonomy.Apps, r.SecondaryTag)),
	}
	for _, k := range cand {
		if _, ok := cs.Keys[k]; ok {
			return true
		}
	}
	for _, p := range cs.ChangedPaths() {
		if strings.HasPrefix(st.ProjectHash, p) {
			return true
		}
	}
	return false
}

func (s *Store) write(ctx context.Context, dirty map[string]*StoredLine, history []HistoryRow, writeState bool, version, snapshot string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	keys := make([]string, 0, len(dirty))
	for k := range dirty {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	del, err := tx.PrepareContext(ctx, "DELETE FROM attribution_line WHERE line_key = ?")
	if err != nil {
		return err
	}
	defer del.Close()
	ins, err := tx.PrepareContext(ctx, "INSERT INTO attribution_line ("+lineColumns+") VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)")
	if err != nil {
		return err
	}
	defer ins.Close()
	for _, k := range keys {
		l := dirty[k]
		if _, err := del.ExecContext(ctx, k); err != nil {
			return err
		}
		if _, err := ins.ExecContext(ctx, l.Key, l.UsageDate, l.Metric, l.Currency, l.Amount,
			l.Row.AccountID, l.Row.AccountName, l.Row.ConnectionID, l.Row.CSP, l.Row.Region, l.Row.Zone,
			l.Row.Service, l.Row.UsageType, l.Row.ResourceID, l.Row.PrimaryTag, l.Row.SecondaryTag,
			l.InputHash, l.TaxonomyVersion, l.ProjectHash, l.BizHash, l.Trail,
			l.PrevProjectHash, l.PrevBizHash, nullTime(l.ResolvedAt), nullTime(l.ChangedAt), l.Active); err != nil {
			return fmt.Errorf("write line %s: %w", k, err)
		}
	}
	for _, h := range history {
		if _, err := tx.ExecContext(ctx, "INSERT INTO attribution_history VALUES (?,?,?,?,?)",
			h.LineKey, h.TaxonomyVersion, h.ProjectHash, h.BizHash, h.ValidFrom); err != nil {
			return err
		}
	}
	if writeState {
		if _, err := tx.ExecContext(ctx, "DELETE FROM taxonomy_state"); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO taxonomy_state VALUES (1, ?, ?)", version, snapshot); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func nullTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}

// totals sums the stored amount of every active line per "currency/metric".
func (s *Store) totals(ctx context.Context) (map[string]float64, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT currency, metric, SUM(amount) FROM attribution_line WHERE active GROUP BY currency, metric")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]float64{}
	for rows.Next() {
		var cur, metric string
		var sum float64
		if err := rows.Scan(&cur, &metric, &sum); err != nil {
			return nil, err
		}
		out[cur+"/"+metric] = sum
	}
	return out, rows.Err()
}
