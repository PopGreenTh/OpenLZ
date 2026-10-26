package cost_test

import (
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/PopGreenTh/OpenLZ/internal/cost"
	"github.com/PopGreenTh/OpenLZ/internal/mapping"
	"github.com/PopGreenTh/OpenLZ/internal/taxonomy"
	"gopkg.in/yaml.v3"
)

type costCase struct {
	Inventory struct {
		Organizations []mapping.Organization `yaml:"organizations"`
	} `yaml:"inventory"`
	Taxonomy taxonomy.Taxonomy `yaml:"taxonomy"`
	Cost     []cost.RawRow     `yaml:"cost"`
	Expect   struct {
		Allocated []map[string]any `yaml:"allocated"`
		Findings  []struct {
			Code    string `yaml:"code"`
			Subject string `yaml:"subject"`
		} `yaml:"findings"`
	} `yaml:"expect"`
}

func TestReferenceCostCases(t *testing.T) {
	dir := filepath.Join("..", "..", "planning", "DD-01-r1-account-mapping", "cases", "cost")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read cases: %v", err)
	}
	for _, ent := range entries {
		if ent.IsDir() || filepath.Ext(ent.Name()) != ".yaml" {
			continue
		}
		t.Run(ent.Name(), func(t *testing.T) {
			runCostCase(t, filepath.Join(dir, ent.Name()))
		})
	}
}

func runCostCase(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var c costCase
	if err := yaml.Unmarshal(data, &c); err != nil {
		t.Fatalf("parse: %v", err)
	}
	src := mapping.NewStaticSource(mapping.Inventory{Organizations: c.Inventory.Organizations})
	mapResult := mapping.Resolve(&c.Taxonomy, src)
	if mapResult.Conflict != nil {
		t.Fatalf("mapping conflict: %+v", mapResult.Conflict)
	}
	hash, err := mapping.TaxonomyHash(&c.Taxonomy)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	acctMap := &mapping.AccountMapDocument{
		TaxonomyHash: hash,
		Allocations:  mapResult.Allocations,
		Findings:     mapResult.Findings,
	}
	result, err := cost.Allocate(&c.Taxonomy, acctMap, c.Cost)
	if err != nil {
		t.Fatalf("allocate: %v", err)
	}
	assertCostFindings(t, c.Expect.Findings, result.Findings)
	for _, exp := range c.Expect.Allocated {
		matchAllocated(t, exp, result.Rows)
	}
}

func assertCostFindings(t *testing.T, expect []struct {
	Code    string `yaml:"code"`
	Subject string `yaml:"subject"`
}, got []cost.Finding) {
	t.Helper()
	expSet := map[[2]string]struct{}{}
	for _, f := range expect {
		expSet[[2]string{f.Code, f.Subject}] = struct{}{}
	}
	gotSet := map[[2]string]struct{}{}
	for _, f := range got {
		gotSet[[2]string{f.Code, f.Subject}] = struct{}{}
	}
	if !reflect.DeepEqual(expSet, gotSet) {
		t.Fatalf("findings mismatch\nwant: %v\ngot:  %v", expSet, gotSet)
	}
}

func matchAllocated(t *testing.T, exp map[string]any, rows []cost.AllocatedRow) {
	t.Helper()
	var matches []cost.AllocatedRow
	for _, r := range rows {
		if v, ok := exp["account_id"]; ok && r.AccountID != v {
			continue
		}
		if v, ok := exp["app"]; ok && r.App != v {
			continue
		}
		if v, ok := exp["app_tag"]; ok && r.AppTag != v {
			continue
		}
		if v, ok := exp["month"]; ok && r.Month != v {
			continue
		}
		matches = append(matches, r)
	}
	if len(matches) != 1 {
		t.Fatalf("expected one allocated row for %v, got %d", exp, len(matches))
	}
	row := matches[0]
	for k, want := range exp {
		switch k {
		case "account_id", "app", "app_tag", "month":
			continue
		case "amount", "share":
			gotF := rowFieldFloat(row, k)
			wantF, ok := toFloat(want)
			if !ok {
				t.Fatalf("field %s: bad want %v", k, want)
			}
			if math.Abs(gotF-wantF) > 1e-9 {
				t.Fatalf("field %s: got %g want %g", k, gotF, wantF)
			}
		case "estimated":
			if row.Estimated != want.(bool) {
				t.Fatalf("estimated: got %v want %v", row.Estimated, want)
			}
		default:
			gotS := rowFieldString(row, k)
			if gotS != want.(string) {
				t.Fatalf("field %s: got %q want %q", k, gotS, want)
			}
		}
	}
}

func rowFieldString(row cost.AllocatedRow, jsonKey string) string {
	switch jsonKey {
	case "project":
		return row.Project
	case "app":
		return row.App
	case "environment":
		return row.Environment
	case "business_unit":
		return row.BusinessUnit
	case "cost_center":
		return row.CostCenter
	case "share_rule":
		return row.ShareRule
	case "app_rule":
		return row.AppRule
	case "environment_rule":
		return row.EnvironmentRule
	case "currency":
		return row.Currency
	default:
		return ""
	}
}

func rowFieldFloat(row cost.AllocatedRow, jsonKey string) float64 {
	switch jsonKey {
	case "amount":
		return row.Amount
	case "share":
		return row.Share
	default:
		return 0
	}
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	default:
		return 0, false
	}
}
