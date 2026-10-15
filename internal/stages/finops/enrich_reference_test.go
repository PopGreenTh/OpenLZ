package finops

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/PopGreenTh/OpenLZ/internal/cloud"
	"github.com/PopGreenTh/OpenLZ/internal/duckdb"
	"github.com/PopGreenTh/OpenLZ/internal/taxonomy"
)

// referenceTaxonomy mirrors internal/attribution/testdata/fixtures.sql as a real taxonomy.
// Biz TEAM-SRE sits under CC-101 in the fixture and under CC-202 in the expected hashes of the
// Loyalty cases; a taxonomy cannot have one team under two cost centers, so the Loyalty
// environment uses TEAM-SRE-M and the test maps it back to TEAM-SRE.
func referenceTaxonomy(t *testing.T, path string) {
	t.Helper()
	snap := taxonomy.NewSnapshot()
	add := func(coll string, kv ...string) {
		row := taxonomy.Row{}
		for i := 0; i < len(kv); i += 2 {
			row[kv[i]] = kv[i+1]
		}
		snap.Add(coll, row)
	}
	add(taxonomy.BusinessUnits, "code", "BU-RETAIL", "name", "Retail")
	add(taxonomy.BusinessUnits, "code", "BU-MKTG", "name", "Marketing")
	add(taxonomy.CostCenters, "code", "CC-101", "name", "CC 101", "business_unit", "BU-RETAIL")
	add(taxonomy.CostCenters, "code", "CC-202", "name", "CC 202", "business_unit", "BU-MKTG")
	add(taxonomy.OwnerTeams, "code", "TEAM-SRE", "name", "SRE Retail", "cost_center", "CC-101")
	add(taxonomy.OwnerTeams, "code", "TEAM-SRE-M", "name", "SRE Marketing", "cost_center", "CC-202")
	add(taxonomy.ProductLines, "code", "PL-WALLET", "name", "Wallet", "owner_team", "TEAM-SRE")
	add(taxonomy.ProductLines, "code", "PL-REWARDS", "name", "Rewards", "owner_team", "TEAM-SRE-M")
	add(taxonomy.Customers, "code", "CUST-B2C", "name", "B2C", "product_line", "PL-WALLET")
	add(taxonomy.Customers, "code", "CUST-VIP", "name", "VIP", "product_line", "PL-REWARDS")
	add(taxonomy.Projects, "code", "PRJ-ECOM", "name", "E-Commerce", "business_unit", "BU-RETAIL")
	add(taxonomy.Projects, "code", "PRJ-CRM", "name", "CRM", "business_unit", "BU-MKTG")
	add(taxonomy.Apps, "code", "APP-CHK", "name", "Checkout", "project", "PRJ-ECOM", "cost_center", "CC-101")
	add(taxonomy.Apps, "code", "APP-LOY", "name", "Loyalty", "project", "PRJ-CRM", "cost_center", "CC-202")
	add(taxonomy.Environments, "code", "ENV-PROD", "name", "Production", "app", "APP-CHK", "owner_team", "TEAM-SRE")
	add(taxonomy.Environments, "code", "ENV-PROD", "name", "Production", "app", "APP-LOY", "owner_team", "TEAM-SRE-M")
	add(taxonomy.Features, "code", "FEAT-PAY", "name", "Pay", "environment", "|PRJ-ECOM|APP-CHK|ENV-PROD|", "product_line", "PL-WALLET")
	add(taxonomy.Features, "code", "FEAT-PTS", "name", "Points", "environment", "|PRJ-CRM|APP-LOY|ENV-PROD|", "product_line", "PL-REWARDS")
	add(taxonomy.Functions, "code", "FUNC-QR", "name", "QR", "feature", "|PRJ-ECOM|APP-CHK|ENV-PROD|FEAT-PAY|", "customer", "CUST-B2C")
	add(taxonomy.Functions, "code", "FUNC-REDEEM", "name", "Redeem", "feature", "|PRJ-CRM|APP-LOY|ENV-PROD|FEAT-PTS|", "customer", "CUST-VIP")
	add(taxonomy.LandingZones, "code", "LZ-AWS-01", "name", "AWS LZ", "csp", "AWS", "currency", "USD")
	add(taxonomy.Connections, "code", "CONN-1", "name", "Connection", "landing_zone", "LZ-AWS-01")
	add(taxonomy.OUs, "code", "OU-DEDICATED", "name", "Dedicated", "landing_zone", "LZ-AWS-01")
	add(taxonomy.OUs, "code", "OU-SHARED", "name", "Shared", "landing_zone", "LZ-AWS-01")
	add(taxonomy.Accounts, "code", "ACC-111", "name", "Dedicated", "connection", "CONN-1", "ou", "OU-DEDICATED")
	add(taxonomy.Accounts, "code", "ACC-SHARED-EKS", "name", "Shared EKS", "connection", "CONN-1", "ou", "OU-SHARED")
	add(taxonomy.Regions, "code", "ap-southeast-1", "name", "Singapore", "csp", "AWS")
	add(taxonomy.ResourceGroups, "code", "RG-QR-WORKER", "name", "QR worker", "project_path", "|PRJ-ECOM|APP-CHK|ENV-PROD|FEAT-PAY|FUNC-QR|")
	add(taxonomy.Resources, "code", "arn:aws:rds:ap-southeast-1:111122223333:db:shared-pg", "name", "shared-pg",
		"project_path", "|PRJ-CRM|APP-LOY|ENV-PROD|FEAT-PTS|FUNC-REDEEM|")
	add(taxonomy.ProjectLZs, "project", "PRJ-ECOM", "landing_zone", "LZ-AWS-01")
	add(taxonomy.AppOUs, "ou", "OU-DEDICATED", "app", "APP-CHK")
	add(taxonomy.AppOUs, "ou", "OU-SHARED", "app", "APP-CHK")
	add(taxonomy.AppOUs, "ou", "OU-SHARED", "app", "APP-LOY")
	add(taxonomy.EnvironmentRegions, "environment", "|PRJ-ECOM|APP-CHK|ENV-PROD|", "region", "ap-southeast-1")
	add(taxonomy.EnvironmentRegions, "environment", "|PRJ-CRM|APP-LOY|ENV-PROD|", "region", "ap-southeast-1")
	if _, err := taxonomy.Build(snap); err != nil {
		t.Fatalf("reference taxonomy: %v", err)
	}
	if err := taxonomy.WriteSnapshot(path, snap); err != nil {
		t.Fatal(err)
	}
}

// Enrich sees two sensed tags (Project, App); the reference cases that need Environment,
// Feature, Function or Biz tags cannot reach it and are covered by internal/attribution.
func TestReferenceCasesThroughEnrich(t *testing.T) {
	type refCase struct {
		ID      int      `json:"id"`
		Title   string   `json:"title"`
		Project string   `json:"project"`
		Biz     string   `json:"biz"`
		Flags   []string `json:"flags"`
	}
	raw, err := os.ReadFile(filepath.Join("..", "..", "attribution", "testdata", "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []refCase
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	want := map[int]refCase{}
	for _, c := range cases {
		want[c.ID] = c
	}

	// id → (account, region, resource, project tag, app tag)
	type line struct{ account, region, resource, project, app string }
	lines := map[int]line{
		7:  {"ACC-SHARED-EKS", "ap-southeast-1", "", "", "APP-LOY"},
		8:  {"ACC-111", "ap-southeast-1", "", "", ""},
		10: {"ACC-111", "ap-southeast-1", "arn:aws:rds:ap-southeast-1:111122223333:db:shared-pg", "PRJ-ECOM", "APP-CHK"},
		12: {"ACC-SHARED-EKS", "ap-southeast-1", "", "", ""},
		13: {"ACC-111", "us-east-1", "", "", ""},
	}
	var records []cloud.CostRecord
	for id, l := range lines {
		rec := cloud.CostRecord{
			AccountID: l.account, AccountName: l.account, Service: fmt.Sprintf("case-%d", id),
			UsageDate: "2026-09-22", Amount: 100, Unit: "USD", Currency: "USD", CSP: "AWS",
			Region: l.region, UsageType: "Usage", ResourceID: l.resource, Timestamp: time.Now(),
			PrimaryTag: l.project, SecondaryTag: l.app,
		}
		records = append(records, rec)
	}

	ctx := context.Background()
	dir := t.TempDir()
	taxPath := filepath.Join(dir, "taxonomy.json")
	referenceTaxonomy(t, taxPath)
	rawPath, anomPath, enrichPath := filepath.Join(dir, "raw.parquet"), filepath.Join(dir, "anom.parquet"), filepath.Join(dir, "enr.parquet")

	engine, err := duckdb.NewEngine()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	if err := engine.WriteFinOpsParquet(ctx, records, rawPath); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.AnalyzeFinOps(ctx, rawPath, anomPath, 1.0); err != nil {
		t.Fatal(err)
	}
	if _, err := ExecuteEnrichCostCenters(ctx, EnrichCostCentersInput{InputPath: anomPath, OutputPath: enrichPath, TaxonomyPath: taxPath}); err != nil {
		t.Fatal(err)
	}
	got, err := engine.ReadEnrichedFromParquet(ctx, enrichPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(lines) {
		t.Fatalf("got %d rows for %d cases", len(got), len(lines))
	}
	for _, r := range got {
		var id int
		fmt.Sscanf(r.Service, "case-%d", &id)
		c := want[id]
		t.Run(c.Title, func(t *testing.T) {
			project := "|" + strings.Join([]string{r.ProjectID, r.ApplicationID, r.EnvironmentID, r.FeatureID, r.FunctionID}, "|") + "|"
			biz := "|" + strings.Join([]string{r.BusinessUnitID, r.CostCenterID, r.OwnerTeamID, r.ProductLineID, r.CustomerID}, "|") + "|"
			biz = strings.Replace(biz, "TEAM-SRE-M", "TEAM-SRE", 1)
			if project != c.Project {
				t.Errorf("project\n got  %s\n want %s (trail %q region %q)", project, c.Project, r.AttributionTrail, r.RegionID)
			}
			if biz != c.Biz {
				t.Errorf("biz\n got  %s\n want %s", biz, c.Biz)
			}
			for _, f := range c.Flags {
				if !strings.Contains(r.AttributionTrail, f) {
					t.Errorf("trail %q is missing %q", r.AttributionTrail, f)
				}
			}
		})
	}
}
