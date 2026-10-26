package mapping_test

import (
	"testing"

	"github.com/PopGreenTh/OpenLZ/internal/mapping"
	"github.com/PopGreenTh/OpenLZ/internal/taxonomy"
)

func TestOULinkConflictStopsResolve(t *testing.T) {
	org := mapping.Organization{
		ID:                "o-test",
		ManagementAccount: "111111111111",
		RootID:            "r-test",
		OUs: []mapping.OU{
			{ID: "ou-wkld", Name: "Workloads", ParentID: "r-test"},
			{ID: "ou-child", Name: "Child", ParentID: "ou-wkld"},
		},
	}
	tax := &taxonomy.Taxonomy{
		LZs: []*taxonomy.LZ{{
			Code:              "LZ-MAIN",
			Name:              "Main",
			CSP:               "AWS",
			Currency:          "USD",
			ManagementAccount: "111111111111",
		}},
		Apps: []*taxonomy.App{{
			Code:       "APP-A",
			Name:       "A",
			Project:    "PRJ",
			CostCenter: "CC",
		}, {
			Code:       "APP-B",
			Name:       "B",
			Project:    "PRJ",
			CostCenter: "CC",
		}},
		Projects:      []*taxonomy.Project{{Code: "PRJ", Name: "P", BusinessUnit: "BU"}},
		BusinessUnits: []*taxonomy.BusinessUnit{{Code: "BU", Name: "BU"}},
		CostCenters:   []*taxonomy.CostCenter{{Code: "CC", Name: "C", BusinessUnit: "BU"}},
		OULinks: []*taxonomy.OULink{
			{OU: "/Workloads", App: "APP-A"},
			{OU: "/Workloads", App: "APP-B"},
		},
	}
	src := mapping.NewStaticSource(mapping.Inventory{Organizations: []mapping.Organization{org}})
	result := mapping.Resolve(tax, src)
	if result.Conflict == nil || result.Conflict.Code != "ou-link-conflict" {
		t.Fatalf("expected ou-link-conflict, got %+v", result.Conflict)
	}
}
