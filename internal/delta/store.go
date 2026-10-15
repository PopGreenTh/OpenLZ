// Package delta keeps attributions current (spec §8, CloudValueDelta). It stores the result of
// the last attribution of every cost line, classifies what changed since the last run (data
// delta: new lines, restated amounts, changed keys, removed lines; mapping delta: a Directus
// edit between two taxonomy snapshots), re-attributes only the lines that need it, keeps every
// earlier result in an append-only history and reports the cost that moved.
//
// Properties: a run with no data and no mapping change writes nothing (idempotent); the same
// inputs and taxonomy version always give the same result (deterministic); the total cost per
// currency is identical before and after a re-attribution (no silent loss).
package delta

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/PopGreenTh/OpenLZ/internal/attribution"
	"github.com/PopGreenTh/OpenLZ/internal/duckdb"
	_ "github.com/marcboeker/go-duckdb"
)

// Line is one cost line: its identity, its amount and the inputs attribution reads.
type Line struct {
	Key       string // Cost Explorer grouping keys plus the Project/App tag pair
	UsageDate string // YYYY-MM-DD
	Metric    string
	Currency  string
	Amount    float64
	Row       attribution.Row
}

// KeyOf is the identity of a raw FinOps line: the Cost Explorer grouping keys and the
// Project/App tag pair. A two-level (tag) run returns one row per tag pair for the same
// account, service and day, each with its own cost, so the tag pair is part of the grain.
func KeyOf(l duckdb.RawCostLine) string {
	return strings.Join([]string{l.AccountID, l.Service, l.UsageDate, l.UsagePeriod, l.Metric, l.Frequency,
		l.Region, l.Zone, l.UsageType, l.ResourceID, l.PrimaryTag, l.SecondaryTag}, "|")
}

// FromRaw converts a raw FinOps line.
func FromRaw(l duckdb.RawCostLine) Line {
	return Line{
		Key: KeyOf(l), UsageDate: l.UsageDate, Metric: l.Metric, Currency: l.Currency, Amount: l.Amount,
		Row: attribution.Row{
			AccountID: l.AccountID, AccountName: l.AccountName, ConnectionID: l.ConnectionID,
			CSP: l.CSP, Region: l.Region, Zone: l.Zone,
			Service: l.Service, UsageType: l.UsageType, ResourceID: l.ResourceID,
			PrimaryTag: l.PrimaryTag, SecondaryTag: l.SecondaryTag,
		},
	}
}

// InputHash fingerprints everything attribution reads from the line itself: the Project tags,
// connection, account, region, zone, service, usage type, resource id and currency (spec
// §8.2). The amount is deliberately excluded: an amount change needs re-aggregation, not
// re-attribution. The resource group is derived from the taxonomy, so it belongs to the
// mapping delta and is not part of the hash.
func InputHash(l Line) string {
	r := l.Row
	h := sha256.New()
	for _, f := range []string{r.PrimaryTag, r.SecondaryTag, r.ConnectionID, r.AccountID, r.AccountName, r.CSP,
		r.Region, r.Zone, r.Service, r.UsageType, r.ResourceID, l.Currency} {
		h.Write([]byte(f))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// StoredLine is a line with its current and previous attribution.
type StoredLine struct {
	Line
	InputHash       string
	TaxonomyVersion string
	ProjectHash     string
	BizHash         string
	Trail           string
	PrevProjectHash string
	PrevBizHash     string
	ResolvedAt      time.Time
	ChangedAt       time.Time
	Active          bool // false once the line was REMOVED from the data
}

// HistoryRow is one result a line has had, valid from the time it was computed.
type HistoryRow struct {
	LineKey         string
	TaxonomyVersion string
	ProjectHash     string
	BizHash         string
	ValidFrom       time.Time
}

// Store is the DuckDB database holding the attribution line table and its history.
type Store struct {
	db *sql.DB
}

// Open opens (creating it when needed) the store at path; "" gives an in-memory store.
func Open(ctx context.Context, path string) (*Store, error) {
	db, err := sql.Open("duckdb", path)
	if err != nil {
		return nil, fmt.Errorf("open attribution store: %w", err)
	}
	db.SetMaxOpenConns(1) // the attribution query uses scratch tables on the same database
	s := &Store{db: db}
	for _, ddl := range []string{
		`CREATE TABLE IF NOT EXISTS attribution_line (
			line_key TEXT, usage_date TEXT, metric TEXT, currency TEXT, amount DOUBLE,
			account_id TEXT, account_name TEXT, connection_id TEXT, csp TEXT, region TEXT, zone TEXT,
			service TEXT, usage_type TEXT, resource_id TEXT, primary_tag TEXT, secondary_tag TEXT,
			input_hash TEXT, taxonomy_version TEXT, project_hash TEXT, biz_hash TEXT, trail TEXT,
			prev_project_hash TEXT, prev_biz_hash TEXT, resolved_at TIMESTAMP, changed_at TIMESTAMP, active BOOLEAN)`,
		`CREATE TABLE IF NOT EXISTS attribution_history (
			line_key TEXT, taxonomy_version TEXT, project_hash TEXT, biz_hash TEXT, valid_from TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS taxonomy_state (id INTEGER, version TEXT, snapshot TEXT)`,
	} {
		if _, err := db.ExecContext(ctx, ddl); err != nil {
			db.Close()
			return nil, fmt.Errorf("create attribution store: %w", err)
		}
	}
	return s, nil
}

// Close closes the store.
func (s *Store) Close() error { return s.db.Close() }

const lineColumns = `line_key, usage_date, metric, currency, amount,
	account_id, account_name, connection_id, csp, region, zone,
	service, usage_type, resource_id, primary_tag, secondary_tag,
	input_hash, taxonomy_version, project_hash, biz_hash, trail,
	prev_project_hash, prev_biz_hash, resolved_at, changed_at, active`

// Lines returns every stored line ordered by key.
func (s *Store) Lines(ctx context.Context) ([]StoredLine, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT "+lineColumns+" FROM attribution_line ORDER BY line_key")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []StoredLine
	for rows.Next() {
		var l StoredLine
		var resolved, changed sql.NullTime
		var prevP, prevB, trail, version, project, biz sql.NullString
		if err := rows.Scan(&l.Key, &l.UsageDate, &l.Metric, &l.Currency, &l.Amount,
			&l.Row.AccountID, &l.Row.AccountName, &l.Row.ConnectionID, &l.Row.CSP, &l.Row.Region, &l.Row.Zone,
			&l.Row.Service, &l.Row.UsageType, &l.Row.ResourceID, &l.Row.PrimaryTag, &l.Row.SecondaryTag,
			&l.InputHash, &version, &project, &biz, &trail,
			&prevP, &prevB, &resolved, &changed, &l.Active); err != nil {
			return nil, err
		}
		l.TaxonomyVersion, l.ProjectHash, l.BizHash, l.Trail = version.String, project.String, biz.String, trail.String
		l.PrevProjectHash, l.PrevBizHash = prevP.String, prevB.String
		l.ResolvedAt, l.ChangedAt = resolved.Time, changed.Time
		out = append(out, l)
	}
	return out, rows.Err()
}

// History returns every result ever computed, ordered by line and time.
func (s *Store) History(ctx context.Context) ([]HistoryRow, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT line_key, taxonomy_version, project_hash, biz_hash, valid_from
		FROM attribution_history ORDER BY line_key, valid_from, taxonomy_version`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []HistoryRow
	for rows.Next() {
		var h HistoryRow
		if err := rows.Scan(&h.LineKey, &h.TaxonomyVersion, &h.ProjectHash, &h.BizHash, &h.ValidFrom); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// state returns the taxonomy version and snapshot JSON of the last run ("" when none).
func (s *Store) state(ctx context.Context) (version, snapshot string, err error) {
	err = s.db.QueryRowContext(ctx, "SELECT version, snapshot FROM taxonomy_state WHERE id = 1").Scan(&version, &snapshot)
	if err == sql.ErrNoRows {
		return "", "", nil
	}
	return version, snapshot, err
}

// runner runs the attribution query on the store's own database.
func (s *Store) runner(ctx context.Context, nodes []attribution.Node, ev []attribution.Evidence, al []attribution.Alias, lines []attribution.Line) ([]attribution.Result, error) {
	if err := attribution.Load(ctx, s.db, nodes, ev, al, lines); err != nil {
		return nil, err
	}
	return attribution.Resolve(ctx, s.db)
}
