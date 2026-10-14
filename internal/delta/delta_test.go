package delta

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/PopGreenTh/OpenLZ/internal/attribution"
	"github.com/PopGreenTh/OpenLZ/internal/taxonomy"
)

var t0 = time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)

func sampleSnap(t *testing.T) *taxonomy.Snapshot {
	t.Helper()
	snap, err := taxonomy.ReadSnapshot("../../taxonomy/taxonomy.json")
	if err != nil {
		t.Fatal(err)
	}
	return snap
}

func openStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

var days = []string{"2026-09-20", "2026-09-21", "2026-09-22"}

type spec struct {
	name, account, region, resource, project, app string
	amount                                        float64
}

// mk builds one line per day for every spec.
func mk(specs ...spec) []Line {
	var out []Line
	for _, sp := range specs {
		for i, d := range days {
			out = append(out, Line{
				Key: sp.name + "|" + d, UsageDate: d, Metric: "UnblendedCost", Currency: "USD",
				Amount: sp.amount + float64(i),
				Row: attribution.Row{
					AccountID: sp.account, AccountName: sp.account, CSP: "AWS", Region: sp.region,
					Service: sp.name, UsageType: "Usage", ResourceID: sp.resource,
					PrimaryTag: sp.project, SecondaryTag: sp.app,
				},
			})
		}
	}
	return out
}

func baseLines() []Line {
	return mk(
		spec{"ec2", "111122223333", "ap-southeast-1", "i-0111122223333a001", "Untagged", "Untagged", 100}, // RG-CHECKOUT → FEAT-CARD
		spec{"orders", "111122223333", "ap-southeast-1", "", "PRJ-PAYMENTS", "APP-ORDERS", 50},            // fully tagged
		spec{"shared", "111122223333", "ap-southeast-1", "", "Untagged", "Untagged", 20},                  // shared account
		spec{"lake", "777788889999", "ap-southeast-1", "", "Untagged", "Untagged", 70},                    // dedicated account
		spec{"mystery", "999999999999", "eu-west-1", "", "Untagged", "Untagged", 10},                      // nothing resolves
	)
}

func byKey(t *testing.T, s *Store) map[string]StoredLine {
	t.Helper()
	ls, err := s.Lines(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	m := map[string]StoredLine{}
	for _, l := range ls {
		m[l.Key] = l
	}
	return m
}

func sync(t *testing.T, s *Store, snap *taxonomy.Snapshot, lines []Line, now time.Time) *Report {
	t.Helper()
	rep, err := s.Sync(context.Background(), snap, lines, Options{Now: now})
	if err != nil {
		t.Fatal(err)
	}
	return rep
}

func TestFirstRunAttributesEveryLine(t *testing.T) {
	s := openStore(t)
	rep := sync(t, s, sampleSnap(t), baseLines(), t0)
	if rep.New != 15 || rep.Reattributed != 15 || rep.PrevTaxonomyVersion != "" {
		t.Fatalf("report: %+v", rep)
	}
	got := byKey(t, s)
	want := map[string]string{
		"ec2":     "|PRJ-PAYMENTS|APP-GATEWAY|ENV-PROD|FEAT-CARD|FUNC-UNSPEC|",
		"orders":  "|PRJ-PAYMENTS|APP-ORDERS|ENV-PROD|FEAT-UNSPEC|FUNC-UNSPEC|",
		"shared":  "|PRJ-PAYMENTS|APP-UNSPEC|ENV-UNSPEC|FEAT-UNSPEC|FUNC-UNSPEC|",
		"lake":    "|PRJ-LAKEHOUSE|APP-LAKE|ENV-PROD|FEAT-UNSPEC|FUNC-UNSPEC|",
		"mystery": "|PRJ-UNALLOC|APP-UNALLOC|ENV-UNALLOC|FEAT-UNALLOC|FUNC-UNALLOC|",
	}
	for k, l := range got {
		name := strings.SplitN(k, "|", 2)[0]
		if l.ProjectHash != want[name] || l.PrevProjectHash != "" || !l.Active || l.TaxonomyVersion != rep.TaxonomyVersion {
			t.Errorf("%s: %+v", k, l)
		}
	}
	hist, _ := s.History(context.Background())
	if len(hist) != 15 {
		t.Fatalf("history rows: %d", len(hist))
	}
}

func TestSecondRunWithNothingNewWritesNothing(t *testing.T) {
	ctx := context.Background()
	s := openStore(t)
	snap := sampleSnap(t)
	sync(t, s, snap, baseLines(), t0)
	before, _ := s.Lines(ctx)
	histBefore, _ := s.History(ctx)

	rep := sync(t, s, snap, baseLines(), t0.Add(time.Hour))
	if rep.RowsWritten != 0 || rep.Selected != 0 || rep.Reattributed != 0 || rep.New+rep.Amount+rep.Keys+rep.Removed != 0 {
		t.Fatalf("idempotent run changed something: %+v", rep)
	}
	after, _ := s.Lines(ctx)
	histAfter, _ := s.History(ctx)
	if !reflect.DeepEqual(before, after) || !reflect.DeepEqual(histBefore, histAfter) {
		t.Fatal("store changed")
	}
}

func TestResourceOverrideChangeReattributesExactlyThatResource(t *testing.T) {
	ctx := context.Background()
	s := openStore(t)
	snap := sampleSnap(t)
	first := sync(t, s, snap, baseLines(), t0)
	before := byKey(t, s)

	// ec2's resource now maps to the lake environment, whatever its group says.
	const newPath = "|PRJ-LAKEHOUSE|APP-LAKE|ENV-PROD|"
	changed := false
	for _, r := range snap.Collections[taxonomy.Resources] {
		if r.Code() == "i-0111122223333a001" {
			r["project_path"] = newPath
			changed = true
		}
	}
	if !changed {
		t.Fatal("resource not found")
	}
	rep := sync(t, s, snap, baseLines(), t0.Add(time.Hour))

	// Adding evidence also re-resolves the lines that are unresolved (they may improve), but
	// only the 3 periods of the resource change.
	if rep.Selected != 9 || rep.Reattributed != 3 || rep.New != 0 {
		t.Fatalf("exactly the 3 periods of the resource change: %+v", rep)
	}
	after := byKey(t, s)
	for k, l := range after {
		if strings.HasPrefix(k, "ec2|") {
			if l.ProjectHash != "|PRJ-LAKEHOUSE|APP-LAKE|ENV-PROD|FEAT-UNSPEC|FUNC-UNSPEC|" ||
				l.PrevProjectHash != before[k].ProjectHash || l.PrevProjectHash == "" || !strings.Contains(l.Trail, "override") {
				t.Errorf("%s: %+v", k, l)
			}
		} else if !reflect.DeepEqual(l, before[k]) {
			t.Errorf("%s must be untouched: %+v vs %+v", k, l, before[k])
		}
	}
	if len(rep.Moved) != 1 || rep.Moved[0].Lines != 3 || rep.Moved[0].Currency != "USD" || rep.Moved[0].Amount != 100+101+102 {
		t.Fatalf("movement: %+v", rep.Moved)
	}
	if rep.TaxonomyVersion == first.TaxonomyVersion || rep.PrevTaxonomyVersion != first.TaxonomyVersion {
		t.Fatalf("versions: %+v", rep)
	}
	hist, _ := s.History(ctx)
	if len(hist) != 15+3 {
		t.Fatalf("history keeps the earlier results: %d rows", len(hist))
	}
}

func TestNewEnvironmentImprovesOnlyUnresolvedLines(t *testing.T) {
	s := openStore(t)
	snap := sampleSnap(t)
	sync(t, s, snap, baseLines(), t0)
	before := byKey(t, s)

	// A DR environment of the lake app lives in eu-west-1: the unallocated lines can improve.
	snap.Add(taxonomy.Environments, taxonomy.Row{"code": "ENV-DR", "name": "Disaster Recovery", "app": "APP-LAKE", "owner_team": "TEAM-DATA"})
	snap.Add(taxonomy.Regions, taxonomy.Row{"code": "eu-west-1", "name": "Ireland", "csp": "AWS"})
	snap.Add(taxonomy.EnvironmentRegions, taxonomy.Row{"environment": "|PRJ-LAKEHOUSE|APP-LAKE|ENV-DR|", "region": "eu-west-1"})
	rep := sync(t, s, snap, baseLines(), t0.Add(time.Hour))

	after := byKey(t, s)
	moved := 0
	for k, l := range after {
		name := strings.SplitN(k, "|", 2)[0]
		if name == "mystery" {
			moved++
			if l.ProjectHash != "|PRJ-LAKEHOUSE|APP-LAKE|ENV-DR|FEAT-UNSPEC|FUNC-UNSPEC|" || l.PrevProjectHash != before[k].ProjectHash {
				t.Errorf("%s: %+v", k, l)
			}
			continue
		}
		// Fully tagged and already resolved lines are re-resolved (the node set changed) but
		// their result is identical, so the stored row is untouched.
		if !reflect.DeepEqual(l, before[k]) {
			t.Errorf("%s must be untouched: %+v vs %+v", k, l, before[k])
		}
	}
	if moved != 3 || rep.Reattributed != 3 {
		t.Fatalf("moved=%d report=%+v", moved, rep)
	}
	if len(rep.Unresolved) != 3 || !strings.Contains(rep.Unresolved[0].From, "UNALLOC") {
		t.Fatalf("audit list: %+v", rep.Unresolved)
	}
	// 3 lines + 3 history rows + the taxonomy state.
	if rep.RowsWritten != 7 {
		t.Fatalf("rows written: %d", rep.RowsWritten)
	}
}

func TestTotalCostPerCurrencyIsPreserved(t *testing.T) {
	s := openStore(t)
	snap := sampleSnap(t)
	first := sync(t, s, snap, baseLines(), t0)
	for _, r := range snap.Collections[taxonomy.Resources] {
		if r.Code() == "i-0111122223333a001" {
			r["project_path"] = "|PRJ-LAKEHOUSE|APP-LAKE|ENV-PROD|"
		}
	}
	snap.Add(taxonomy.Regions, taxonomy.Row{"code": "eu-west-1", "name": "Ireland", "csp": "AWS"})
	snap.Add(taxonomy.EnvironmentRegions, taxonomy.Row{"environment": "|PRJ-LAKEHOUSE|APP-LAKE|ENV-PROD|", "region": "eu-west-1"})
	second := sync(t, s, snap, baseLines(), t0.Add(time.Hour))
	if !reflect.DeepEqual(first.Totals, second.Totals) || first.Totals["USD/UnblendedCost"] == 0 {
		t.Fatalf("totals %v vs %v", first.Totals, second.Totals)
	}
}

func TestSameInputsGiveIdenticalStores(t *testing.T) {
	ctx := context.Background()
	run := func() ([]StoredLine, []HistoryRow) {
		s := openStore(t)
		snap := sampleSnap(t)
		sync(t, s, snap, baseLines(), t0)
		snap.Add(taxonomy.Environments, taxonomy.Row{"code": "ENV-DR", "name": "DR", "app": "APP-LAKE"})
		sync(t, s, snap, baseLines(), t0.Add(time.Hour))
		l, _ := s.Lines(ctx)
		h, _ := s.History(ctx)
		return l, h
	}
	l1, h1 := run()
	l2, h2 := run()
	if !reflect.DeepEqual(l1, l2) || !reflect.DeepEqual(h1, h2) {
		t.Fatal("two runs over the same inputs differ")
	}
}

func TestDataDeltaKinds(t *testing.T) {
	s := openStore(t)
	snap := sampleSnap(t)
	sync(t, s, snap, baseLines(), t0)
	before := byKey(t, s)

	lines := baseLines()                      // 3 days each: ec2 0-2, orders 3-5, shared 6-8, lake 9-11, mystery 12-14
	lines[0].Amount += 5                      // ec2 day 1: restated amount → AMOUNT
	lines[3].Row.SecondaryTag = "APP-GATEWAY" // orders day 1: tag changed → KEYS
	lines = append(lines[:6], lines[7:]...)   // shared day 1 is gone → REMOVED
	extra := mk(spec{"newsvc", "777788889999", "ap-southeast-1", "", "Untagged", "Untagged", 5})
	lines = append(lines, extra[0]) // NEW
	rep := sync(t, s, snap, lines, t0.Add(time.Hour))

	if rep.Amount != 1 || rep.New != 1 || rep.Removed != 1 || rep.Keys != 1 {
		t.Fatalf("data delta: %+v", rep)
	}
	after := byKey(t, s)
	if l := after["ec2|2026-09-20"]; l.Amount != before["ec2|2026-09-20"].Amount+5 || !reflect.DeepEqual(l.ProjectHash, before["ec2|2026-09-20"].ProjectHash) || l.ResolvedAt != before["ec2|2026-09-20"].ResolvedAt {
		t.Errorf("an amount change must not re-attribute: %+v", l)
	}
	removed := 0
	for _, l := range after {
		if !l.Active {
			removed++
		}
	}
	if removed != 1 {
		t.Fatalf("removed lines: %d", removed)
	}
	if l := after["orders|2026-09-20"]; !strings.HasPrefix(l.ProjectHash, "|PRJ-PAYMENTS|APP-GATEWAY|") ||
		l.PrevProjectHash != before["orders|2026-09-20"].ProjectHash {
		t.Errorf("a changed tag must re-attribute: %+v", l)
	}
	if after["newsvc|2026-09-20"].ProjectHash == "" {
		t.Error("new line must be attributed")
	}
}

func TestAliasEditReattributesMatchingLines(t *testing.T) {
	s := openStore(t)
	snap := sampleSnap(t)
	lines := mk(spec{"typo", "111122223333", "eu-west-1", "", "PRJ-PAYMENTS", "APP-ORDRS", 30})
	sync(t, s, snap, lines, t0)
	before := byKey(t, s)
	if strings.Contains(before["typo|2026-09-20"].ProjectHash, "APP-ORDERS") {
		t.Fatalf("typo should not resolve the app: %+v", before["typo|2026-09-20"])
	}
	snap.Add(taxonomy.Aliases, taxonomy.Row{"code": "APP-ORDRS", "name": "typo", "level": "app", "target": "|PRJ-PAYMENTS|APP-ORDERS|"})
	rep := sync(t, s, snap, lines, t0.Add(time.Hour))
	if rep.Reattributed != 3 {
		t.Fatalf("alias edit: %+v", rep)
	}
	l := byKey(t, s)["typo|2026-09-20"]
	if !strings.HasPrefix(l.ProjectHash, "|PRJ-PAYMENTS|APP-ORDERS|") || !strings.Contains(l.Trail, "alias") {
		t.Fatalf("after alias: %+v", l)
	}
}

func TestReportPrints(t *testing.T) {
	s := openStore(t)
	snap := sampleSnap(t)
	sync(t, s, snap, baseLines(), t0)
	for _, r := range snap.Collections[taxonomy.Resources] {
		if r.Code() == "i-0111122223333a001" {
			r["project_path"] = "|PRJ-LAKEHOUSE|APP-LAKE|ENV-PROD|"
		}
	}
	rep := sync(t, s, snap, baseLines(), t0.Add(time.Hour))
	var b strings.Builder
	rep.Print(&b)
	for _, want := range []string{"NEW 0", "cost moved between projects", "|PRJ-LAKEHOUSE|APP-LAKE|ENV-PROD|FEAT-UNSPEC|FUNC-UNSPEC|", "303.00 USD"} {
		if !strings.Contains(b.String(), want) {
			t.Errorf("report is missing %q:\n%s", want, b.String())
		}
	}
}
