package mapping_test

import (
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/PopGreenTh/OpenLZ/internal/mapping"
	"github.com/PopGreenTh/OpenLZ/internal/taxonomy"
	"gopkg.in/yaml.v3"
)

type mappingCase struct {
	Name      string `yaml:"name"`
	Inventory struct {
		Organizations []mapping.Organization `yaml:"organizations"`
	} `yaml:"inventory"`
	Taxonomy taxonomy.Taxonomy `yaml:"taxonomy"`
	Expect   struct {
		Allocations []map[string]any `yaml:"allocations"`
		Findings    []struct {
			Code    string `yaml:"code"`
			Subject string `yaml:"subject"`
		} `yaml:"findings"`
	} `yaml:"expect"`
}

func TestReferenceMappingCases(t *testing.T) {
	dir := filepath.Join("..", "..", "planning", "DD-01-r1-account-mapping", "cases", "mapping")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read cases: %v", err)
	}
	for _, ent := range entries {
		if ent.IsDir() || filepath.Ext(ent.Name()) != ".yaml" {
			continue
		}
		name := ent.Name()
		t.Run(name, func(t *testing.T) {
			runMappingCase(t, filepath.Join(dir, name))
		})
	}
}

func runMappingCase(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var c mappingCase
	if err := yaml.Unmarshal(data, &c); err != nil {
		t.Fatalf("parse: %v", err)
	}
	inv := mapping.Inventory{Organizations: c.Inventory.Organizations}
	src := mapping.NewStaticSource(inv)
	result := mapping.Resolve(&c.Taxonomy, src)
	if result.Conflict != nil {
		t.Fatalf("unexpected ou-link-conflict: %+v", result.Conflict)
	}
	assertFindings(t, c.Expect.Findings, result.Findings)
	assertAllocations(t, c.Expect.Allocations, result.Allocations)
}

func assertFindings(t *testing.T, expect []struct {
	Code    string `yaml:"code"`
	Subject string `yaml:"subject"`
}, got []mapping.Finding) {
	t.Helper()
	expSet := make(map[[2]string]struct{}, len(expect))
	for _, f := range expect {
		expSet[[2]string{f.Code, f.Subject}] = struct{}{}
	}
	gotSet := make(map[[2]string]struct{}, len(got))
	for _, f := range got {
		gotSet[[2]string{f.Code, f.Subject}] = struct{}{}
	}
	if !reflect.DeepEqual(expSet, gotSet) {
		t.Fatalf("findings mismatch\nwant: %v\ngot:  %v", expSet, gotSet)
	}
}

func assertAllocations(t *testing.T, expect []map[string]any, got []mapping.Allocation) {
	t.Helper()
	for _, exp := range expect {
		matches := filterAllocations(got, exp)
		if len(matches) == 0 {
			t.Fatalf("no allocation matching keys %v in %+v", expKeys(exp), got)
		}
		if len(matches) > 1 {
			t.Fatalf("ambiguous allocation match for %v", exp)
		}
		compareAllocationFields(t, exp, matches[0])
	}
}

func expKeys(exp map[string]any) map[string]any {
	out := make(map[string]any)
	if v, ok := exp["account_id"]; ok {
		out["account_id"] = v
	}
	if v, ok := exp["app"]; ok {
		out["app"] = v
	}
	return out
}

func filterAllocations(allocs []mapping.Allocation, exp map[string]any) []mapping.Allocation {
	var out []mapping.Allocation
	for _, a := range allocs {
		if v, ok := exp["account_id"]; ok && a.AccountID != v {
			continue
		}
		if v, ok := exp["app"]; ok && a.App != v {
			continue
		}
		out = append(out, a)
	}
	return out
}

func compareAllocationFields(t *testing.T, exp map[string]any, got mapping.Allocation) {
	t.Helper()
	val := reflect.ValueOf(got)
	typ := val.Type()
	for k, want := range exp {
		f, ok := fieldByJSONTag(val, k)
		if !ok {
			t.Fatalf("unknown allocation field %q", k)
		}
		gotVal := f.Interface()
		switch want := want.(type) {
		case float64:
			if k == "share" {
				compareShare(t, k, gotVal.(*float64), want)
				continue
			}
		case int:
			if k == "share" {
				compareShare(t, k, gotVal.(*float64), float64(want))
				continue
			}
			if gotVal != want {
				t.Fatalf("field %s: got %v want %v", k, gotVal, want)
			}
			continue
		case string:
			if gotVal != want {
				t.Fatalf("field %s: got %q want %q", k, gotVal, want)
			}
			continue
		case nil:
			if k == "share" {
				if gotVal.(*float64) != nil {
					t.Fatalf("field share: got %v want null", gotVal)
				}
				continue
			}
		}
		if !reflect.DeepEqual(gotVal, want) {
			t.Fatalf("field %s on %s: got %#v want %#v", k, typ.Name(), gotVal, want)
		}
	}
}

func compareShare(t *testing.T, k string, got *float64, want float64) {
	t.Helper()
	if got == nil || math.Abs(*got-want) > 1e-9 {
		t.Fatalf("field %s: got %v want %v", k, got, want)
	}
}

func fieldByJSONTag(val reflect.Value, tag string) (reflect.Value, bool) {
	t := val.Type()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		jt := f.Tag.Get("json")
		if jt == "" {
			continue
		}
		name := jt
		if idx := indexComma(jt); idx >= 0 {
			name = jt[:idx]
		}
		if name == tag {
			return val.Field(i), true
		}
	}
	return reflect.Value{}, false
}

func indexComma(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			return i
		}
	}
	return -1
}
