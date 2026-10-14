package attribution

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"strings"
	"testing"

	_ "github.com/marcboeker/go-duckdb"
)

type wantCase struct {
	ID      int      `json:"id"`
	Title   string   `json:"title"`
	Project string   `json:"project"`
	Biz     string   `json:"biz"`
	Flags   []string `json:"flags"`
}

// loadFixtures creates the node/evidence/lines tables for the 13 reference cases in testdata.
func loadFixtures(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("duckdb", "")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1) // in-memory DuckDB: every connection would otherwise get its own database
	t.Cleanup(func() { db.Close() })
	raw, err := os.ReadFile("testdata/fixtures.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range strings.Split(string(raw), ";") {
		if strings.TrimSpace(stmt) == "" {
			continue
		}
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("fixture %q: %v", strings.TrimSpace(stmt)[:40], err)
		}
	}
	return db
}

func TestReferenceCases(t *testing.T) {
	db := loadFixtures(t)
	raw, err := os.ReadFile("testdata/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []wantCase
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	got, err := Resolve(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[int]Result{}
	for _, r := range got {
		byID[r.LineID] = r
	}
	if len(byID) != len(cases) {
		t.Fatalf("got %d results for %d cases", len(byID), len(cases))
	}
	for _, c := range cases {
		r := byID[c.ID]
		t.Run(c.Title, func(t *testing.T) {
			if r.ProjectHash != c.Project {
				t.Errorf("project hash\n got  %s\n want %s", r.ProjectHash, c.Project)
			}
			if r.BizHash != c.Biz {
				t.Errorf("biz hash\n got  %s\n want %s", r.BizHash, c.Biz)
			}
			for _, f := range c.Flags {
				if !strings.Contains(r.Trail, f) {
					t.Errorf("trail %q is missing %q", r.Trail, f)
				}
			}
		})
	}
}

// Cases 11-13 are the ones the earlier ClickHouse draft got wrong; keep them as explicit
// regression guards so the reason they exist is visible.
func TestRegressionGuards(t *testing.T) {
	db := loadFixtures(t)
	got, err := Resolve(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	by := map[int]Result{}
	for _, r := range got {
		by[r.LineID] = r
	}
	// 11: valid Loyalty tags must not be overwritten by a resource group of another app.
	if !strings.HasPrefix(by[11].ProjectHash, "|PRJ-CRM|APP-LOY|") || !strings.Contains(by[11].Trail, "rg!") {
		t.Errorf("case 11: %+v", by[11])
	}
	// 12: a shared account must not be resolved by the LZ default.
	if !strings.HasPrefix(by[12].ProjectHash, "|PRJ-UNALLOC|") {
		t.Errorf("case 12: %+v", by[12])
	}
	// 13: the result must be a real tree path padded with UNSPEC, never ENV-UNALLOC under an app.
	if strings.Contains(by[13].ProjectHash, "UNALLOC") {
		t.Errorf("case 13: %+v", by[13])
	}
}

func TestSQLIsPortable(t *testing.T) {
	q := SQL()
	for _, banned := range []string{"dictGet", "arrayJoin", "QUALIFY", "ILIKE", "::"} {
		if strings.Contains(q, banned) {
			t.Errorf("query uses engine-specific syntax %q", banned)
		}
	}
}

// An alias repairs an invalid tag value before scoring; a repaired tag is flagged "alias",
// never "tag-corrected", and a line without a matching alias is unchanged.
func TestAliasRepairsTags(t *testing.T) {
	db := loadFixtures(t)
	got, err := Resolve(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	by := map[int]Result{}
	for _, r := range got {
		by[r.LineID] = r
	}
	r := by[14]
	if r.ProjectHash != "|PRJ-ECOM|APP-CHK|ENV-PROD|FEAT-UNSPEC|FUNC-UNSPEC|" {
		t.Errorf("project hash %s", r.ProjectHash)
	}
	if !strings.Contains(r.Trail, "alias") || strings.Contains(r.Trail, "tag-corrected") {
		t.Errorf("trail %q", r.Trail)
	}
	if strings.Contains(by[1].Trail, "alias") {
		t.Errorf("line 1 has no alias, trail %q", by[1].Trail)
	}
	// Without the alias row the same line cannot be repaired by tags alone.
	if _, err := db.Exec("DELETE FROM alias"); err != nil {
		t.Fatal(err)
	}
	got, _ = Resolve(context.Background(), db)
	for _, x := range got {
		if x.LineID == 14 && strings.Contains(x.ProjectHash, "APP-CHK") && strings.Contains(x.Trail, "alias") {
			t.Errorf("alias still applied after deleting the alias table: %+v", x)
		}
	}
}
