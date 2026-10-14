package taxonomy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sampleFile = "../../taxonomy/taxonomy.json"

func sampleCatalog(t *testing.T) *Catalog {
	t.Helper()
	cat, err := Load(sampleFile)
	if err != nil {
		t.Fatalf("load sample taxonomy: %v", err)
	}
	return cat
}

func buildJSON(t *testing.T, doc string) (*Catalog, error) {
	t.Helper()
	var snap Snapshot
	if err := json.Unmarshal([]byte(doc), &snap); err != nil {
		t.Fatalf("parse: %v", err)
	}
	return Build(&snap)
}

// minimal is a valid Biz + Project tree: two apps in one project, one shared account.
const minimal = `
  "business_units": [{"code": "BU", "name": "BU"}, {"code": "BU2", "name": "BU Two"}],
  "cost_centers": [{"code": "CC", "name": "CC", "business_unit": "BU"}, {"code": "CC2", "name": "CC Two", "business_unit": "BU2"}],
  "owner_teams": [{"code": "T1", "name": "Team One", "cost_center": "CC"}],
  "product_lines": [{"code": "PL1", "name": "Line One", "owner_team": "T1"}],
  "customers": [{"code": "C1", "name": "Customer One", "product_line": "PL1"}],
  "projects": [{"code": "P", "name": "Project", "business_unit": "BU"}],
  "apps": [
    {"code": "A1", "name": "App One", "project": "P", "cost_center": "CC"},
    {"code": "A2", "name": "App Two", "project": "P", "cost_center": "CC"}
  ],
  "environments": [
    {"code": "PROD", "name": "Production", "app": "A1", "owner_team": "T1"},
    {"code": "PROD", "name": "Production", "app": "A2"}
  ]`

func TestSampleLoadsWithoutWarnings(t *testing.T) {
	cat := sampleCatalog(t)
	if len(cat.Warnings()) > 0 {
		t.Fatalf("unexpected warnings %v", cat.Warnings())
	}
	for _, spec := range Schema {
		if cat.Count(spec.Name) == 0 {
			t.Errorf("sample has no rows for %s", spec.Name)
		}
	}
}

func TestSchemaIsInDependencyOrder(t *testing.T) {
	seen := map[string]bool{}
	for _, spec := range Schema {
		for _, ref := range spec.Refs {
			if !seen[ref.Target] {
				t.Errorf("%s references %s before it is defined", spec.Name, ref.Target)
			}
		}
		seen[spec.Name] = true
	}
	if _, ok := Spec("csps"); ok {
		t.Error("CSP must be an attribute, not a collection")
	}
}

func TestResolveFullHierarchyFromResource(t *testing.T) {
	out := sampleCatalog(t).Resolve(Input{
		AccountID:  "444455556666",
		CSP:        "AWS",
		Region:     "ap-southeast-1",
		Zone:       "ap-southeast-1a",
		Service:    "Amazon Elastic Compute Cloud - Compute",
		UsageType:  "APS1-BoxUsage:t3.medium",
		ResourceID: "i-0444455556666a001",
		Project:    "Untagged",
		App:        "Untagged",
	})
	want := map[string]Slot{
		"Account":         {ID: "444455556666", Name: "lz-stg-payments"},
		"Connection":      {ID: "100000000001", Name: "Main payer account"},
		"LZ":              {ID: "LZ-MAIN", Name: "Main AWS Organization"},
		"App":             {ID: "APP-GATEWAY", Name: "Payment Gateway"},
		"Project":         {ID: "PRJ-PAYMENTS", Name: "Payments Platform"},
		"BusinessUnit":    {ID: "BU-DIGITAL", Name: "Digital"},
		"CostCenter":      {ID: "CC-PAY", Name: "Payments"},
		"CSP":             {ID: "AWS", Name: "Amazon Web Services"},
		"Region":          {ID: "ap-southeast-1", Name: "Asia Pacific (Singapore)"},
		"Zone":            {ID: "ap-southeast-1a", Name: "Singapore AZ a"},
		"ServiceCategory": {ID: "SC-COMPUTE", Name: "Compute"},
		"Service":         {ID: "AmazonEC2", Name: "Amazon Elastic Compute Cloud - Compute"},
		"ServiceUsage":    {ID: "SU-EC2-INSTANCE", Name: "EC2 Instance Hours"},
		"Usage":           {ID: "BoxUsage:t3.medium", Name: "t3.medium instance hours"},
		"ResourceGroup":   {ID: "RG-CHECKOUT", Name: "Checkout Compute"},
		"Resource":        {ID: "i-0444455556666a001", Name: "checkout-staging-node-1"},
	}
	got := map[string]Slot{
		"Account": out.Account, "Connection": out.Connection, "LZ": out.LZ, "App": out.App,
		"Project": out.Project, "BusinessUnit": out.BusinessUnit, "CostCenter": out.CostCenter,
		"CSP": out.CSP, "Region": out.Region, "Zone": out.Zone, "ServiceCategory": out.ServiceCategory,
		"Service": out.Service, "ServiceUsage": out.ServiceUsage, "Usage": out.Usage,
		"ResourceGroup": out.ResourceGroup, "Resource": out.Resource,
	}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("%s = %+v, want %+v", k, got[k], w)
		}
	}
	if out.BillingCurrency != "USD" || out.RegionCSPMismatch || out.ZoneRegionMismatch {
		t.Errorf("currency/mismatch flags: %+v", out)
	}
}

func TestDedicatedAccountResolvesApp(t *testing.T) {
	out := sampleCatalog(t).Resolve(Input{AccountID: "777788889999", Service: "AmazonS3"})
	if out.App.ID != "APP-LAKE" || out.Project.ID != "PRJ-LAKEHOUSE" ||
		out.CostCenter.ID != "CC-DATA" || out.BusinessUnit.ID != "BU-DATA" {
		t.Fatalf("app/project/biz from the only app of the account: %+v", out)
	}
	if out.ServiceCategory.ID != "SC-STORAGE" {
		t.Fatalf("service category: %+v", out.ServiceCategory)
	}
}

func TestSharedAccountResolvesCommonParentsOnly(t *testing.T) {
	// 111122223333 serves APP-GATEWAY and APP-ORDERS: the App is ambiguous, but both are in
	// PRJ-PAYMENTS and CC-PAY, so bottom-up continues through the ambiguous level.
	out := sampleCatalog(t).Resolve(Input{AccountID: "111122223333"})
	if out.App != (Slot{}) {
		t.Fatalf("app must stay empty for a shared account: %+v", out.App)
	}
	if out.Project.ID != "PRJ-PAYMENTS" || out.CostCenter.ID != "CC-PAY" || out.BusinessUnit.ID != "BU-DIGITAL" {
		t.Fatalf("common parents: project=%+v cc=%+v bu=%+v", out.Project, out.CostCenter, out.BusinessUnit)
	}
	if out.Feature != (Slot{}) || out.Function != (Slot{}) {
		t.Fatalf("feature and function are never guessed: %+v %+v", out.Feature, out.Function)
	}
}

func TestAppTagNarrowsSharedAccount(t *testing.T) {
	out := sampleCatalog(t).Resolve(Input{AccountID: "111122223333", App: "APP-ORDERS"})
	if out.App.ID != "APP-ORDERS" || out.Project.ID != "PRJ-PAYMENTS" {
		t.Fatalf("app tag wins: %+v", out)
	}
}

func TestEnvironmentCodeIsUniqueWithinApp(t *testing.T) {
	cat, err := buildJSON(t, `{`+minimal+`}`)
	if err != nil {
		t.Fatalf("PROD under two apps must be valid: %v", err)
	}
	// A bare ENV code is ambiguous; the App scopes it.
	if out := cat.Resolve(Input{App: "A2", Environment: "PROD"}); out.Environment.Name != "Production" || out.Environment.ID != "PROD" {
		t.Fatalf("environment within app: %+v", out.Environment)
	}
	_, err = buildJSON(t, `{`+minimal+`,
	  "environments": [
	    {"code": "PROD", "name": "Production", "app": "A1"},
	    {"code": "PROD", "name": "Production again", "app": "A1"}
	  ]}`)
	if err == nil || !strings.Contains(err.Error(), "duplicate code within its app") {
		t.Fatalf("duplicate environment under one app: %v", err)
	}
}

func TestRepeatedCodeNeedsPathHash(t *testing.T) {
	_, err := buildJSON(t, `{`+minimal+`,
	  "features": [{"code": "F1", "name": "Feature", "environment": "PROD"}]}`)
	if err == nil || !strings.Contains(err.Error(), "use the path hash") {
		t.Fatalf("ambiguous ref must ask for a path hash: %v", err)
	}
	cat, err := buildJSON(t, `{`+minimal+`,
	  "features": [{"code": "F1", "name": "Feature", "environment": "|P|A2|PROD|"}]}`)
	if err != nil {
		t.Fatal(err)
	}
	if n := cat.ProjectNodes(); len(n) == 0 {
		t.Fatal("no project nodes")
	}
}

func TestBizRefsMustFormValidBizPath(t *testing.T) {
	cat, err := buildJSON(t, `{`+minimal+`,
	  "features": [{"code": "F1", "name": "Feature", "environment": "|P|A1|PROD|", "product_line": "PL1"}]}`)
	if err != nil || len(cat.Warnings()) != 0 {
		t.Fatalf("consistent biz path: %v %v", err, cat.Warnings())
	}
	// App refs CC2, which is under BU2, but its Project refs BU.
	cat, err = buildJSON(t, `{`+strings.Replace(minimal,
		`{"code": "A1", "name": "App One", "project": "P", "cost_center": "CC"}`,
		`{"code": "A1", "name": "App One", "project": "P", "cost_center": "CC2"}`, 1)+`}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(cat.Warnings()) == 0 || !strings.Contains(cat.Warnings()[0], "is not under") {
		t.Fatalf("expected a warning, got %v", cat.Warnings())
	}
}

func TestLZAttributesAreValidated(t *testing.T) {
	_, err := buildJSON(t, `{"landing_zones": [
	  {"code": "LZ1", "name": "ok", "csp": "AWS", "currency": "THB"},
	  {"code": "LZ2", "name": "bad csp", "csp": "IBM", "currency": "USD"},
	  {"code": "LZ3", "name": "bad currency", "csp": "GCP", "currency": "baht"},
	  {"code": "LZ4", "name": "missing", "currency": "USD"}
	]}`)
	if err == nil {
		t.Fatal("expected attribute errors")
	}
	for _, want := range []string{`csp "IBM" is not one of AWS, Azure, GCP, OCI`, `currency "baht"`, "csp is required"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("missing %q in:\n%v", want, err)
		}
	}
}

func TestRegionUniqueWithinCSPAndZoneWithinRegion(t *testing.T) {
	cat, err := buildJSON(t, `{
	  "landing_zones": [{"code": "LZ-A", "name": "A", "csp": "AWS", "currency": "USD"}, {"code": "LZ-Z", "name": "Z", "csp": "Azure", "currency": "USD"}],
	  "connections": [{"code": "CA", "name": "CA", "landing_zone": "LZ-A"}, {"code": "CZ", "name": "CZ", "landing_zone": "LZ-Z"}],
	  "accounts": [{"code": "1", "name": "aws", "connection": "CA"}, {"code": "2", "name": "azure", "connection": "CZ"}],
	  "regions": [
	    {"code": "east", "name": "AWS East", "csp": "AWS"},
	    {"code": "east", "name": "Azure East", "csp": "Azure"}
	  ],
	  "zones": [
	    {"code": "1", "name": "AWS east 1", "region": "AWS East"},
	    {"code": "1", "name": "Azure east 1", "region": "Azure East"}
	  ]}`)
	if err != nil {
		t.Fatalf("same region code in two CSPs, same zone code in two regions must be valid: %v", err)
	}
	// The LZ gives the CSP, so the region code resolves within it.
	out := cat.Resolve(Input{AccountID: "2", Region: "east", Zone: "1"})
	if out.Region.Name != "Azure East" || out.Zone.Name != "Azure east 1" || out.CSP.ID != "Azure" {
		t.Fatalf("azure: %+v %+v %+v", out.Region, out.Zone, out.CSP)
	}
	out = cat.Resolve(Input{AccountID: "1", Region: "east", Zone: "1"})
	if out.Region.Name != "AWS East" || out.Zone.Name != "AWS east 1" {
		t.Fatalf("aws: %+v %+v", out.Region, out.Zone)
	}
	// A region code that exists in both CSPs cannot be resolved without a CSP.
	if out := cat.Resolve(Input{Region: "east"}); out.Region.Name != "east" || out.Region.ID != "" {
		t.Fatalf("ambiguous region must stay raw: %+v", out.Region)
	}
	// Mismatch flags.
	if out := cat.Resolve(Input{AccountID: "1", Region: "east", CSP: "Azure"}); out.Region.Name != "Azure East" {
		t.Fatalf("row csp wins: %+v", out.Region)
	}
	_, err = buildJSON(t, `{"regions": [{"code": "r", "name": "a", "csp": "AWS"}, {"code": "r", "name": "b", "csp": "AWS"}]}`)
	if err == nil || !strings.Contains(err.Error(), "duplicate code within its csp") {
		t.Fatalf("duplicate region in one CSP: %v", err)
	}
}

func TestZoneRegionMismatchIsFlagged(t *testing.T) {
	out := sampleCatalog(t).Resolve(Input{Region: "us-east-1", Zone: "ap-southeast-1a", CSP: "AWS"})
	if !out.ZoneRegionMismatch || out.Zone.ID != "ap-southeast-1a" || out.Region.ID != "us-east-1" {
		t.Fatalf("zone of another region must be flagged and kept: %+v", out)
	}
	// A region alone is weak evidence: it must not pick an environment or an app.
	if out := sampleCatalog(t).Resolve(Input{Region: "us-east-1"}); out.Environment != (Slot{}) || out.App != (Slot{}) {
		t.Fatalf("region must not resolve the project: %+v", out)
	}
}

func TestRegionCSPMismatchIsFlagged(t *testing.T) {
	cat, err := buildJSON(t, `{
	  "landing_zones": [{"code": "LZ", "name": "azure lz", "csp": "Azure", "currency": "THB"}],
	  "connections": [{"code": "C", "name": "C", "landing_zone": "LZ"}],
	  "accounts": [{"code": "1", "name": "a", "connection": "C"}],
	  "regions": [{"code": "east", "name": "East", "csp": "AWS"}]}`)
	if err != nil {
		t.Fatal(err)
	}
	out := cat.Resolve(Input{AccountID: "1", Region: "east", CSP: "AWS"})
	if !out.RegionCSPMismatch || out.BillingCurrency != "THB" {
		t.Fatalf("region csp vs lz csp: %+v", out)
	}
}

func TestZoneBelongsToRegionBottomUp(t *testing.T) {
	out := sampleCatalog(t).Resolve(Input{Zone: "ap-southeast-1b"})
	if out.Region.ID != "ap-southeast-1" || out.CSP.ID != "AWS" {
		t.Fatalf("zone → region → csp: %+v %+v", out.Region, out.CSP)
	}
}

func TestResourceProjectPathMapping(t *testing.T) {
	cat := sampleCatalog(t)
	var mapped, group int
	for _, n := range cat.ordered[Resources] {
		if n.mapped != nil {
			mapped++
		}
	}
	for _, n := range cat.ordered[ResourceGroups] {
		if n.mapped != nil {
			group++
		}
	}
	if mapped != 2 || group != 3 {
		t.Fatalf("mapped resources=%d groups=%d", mapped, group)
	}
	_, err := buildJSON(t, `{`+minimal+`,
	  "resource_groups": [{"code": "RG", "name": "RG", "project_path": "|P|NOPE|"}]}`)
	if err == nil || !strings.Contains(err.Error(), "not a valid Project path") {
		t.Fatalf("invalid project path: %v", err)
	}
}

func TestAccountOutsideItsProjectsLZWarns(t *testing.T) {
	cat, err := buildJSON(t, `{`+minimal+`,
	  "landing_zones": [
	    {"code": "LZ1", "name": "one", "csp": "AWS", "currency": "USD"},
	    {"code": "LZ2", "name": "two", "csp": "AWS", "currency": "USD"}],
	  "connections": [{"code": "C2", "name": "C2", "landing_zone": "LZ2"}],
	  "accounts": [{"code": "9", "name": "nine", "connection": "C2"}],
	  "project_lzs": [{"project": "P", "landing_zone": "LZ1"}],
	  "app_accounts": [{"account": "9", "app": "A1"}]}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(cat.Warnings()) != 1 || !strings.Contains(cat.Warnings()[0], "hosts none of the Projects") {
		t.Fatalf("warnings: %v", cat.Warnings())
	}
}

func TestProjectNodesIncludeSystemUnspecNodes(t *testing.T) {
	cat, err := buildJSON(t, `{`+minimal+`,
	  "features": [{"code": "F1", "name": "Feature", "environment": "|P|A1|PROD|", "product_line": "PL1"}],
	  "functions": [{"code": "FN1", "name": "Function", "feature": "F1", "customer": "C1"}]}`)
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]ProjectNode{}
	for _, n := range cat.ProjectNodes() {
		by[n.Path] = n
	}
	fn := by["|P|A1|PROD|F1|FN1|"]
	if fn.Level != 5 || fn.Biz != [5]string{"BU", "CC", "T1", "PL1", "C1"} {
		t.Fatalf("function node: %+v", fn)
	}
	// The App node carries its cost center and, through it, the business unit.
	app := by["|P|A1|"]
	if app.Biz != [5]string{"BU", "CC", "", "", ""} {
		t.Fatalf("app node: %+v", app)
	}
	// PROD of A2 has no owner team: the level stays empty (UNSPEC at attribution time).
	if env := by["|P|A2|PROD|"]; env.Biz != [5]string{"BU", "CC", "", "", ""} {
		t.Fatalf("env without owner team: %+v", env)
	}
	unspecFeat := by["|P|A1|PROD|FEAT-UNSPEC|"]
	unspecFn := by["|P|A1|PROD|FEAT-UNSPEC|FUNC-UNSPEC|"]
	if !unspecFeat.System || unspecFeat.Biz[3] != "PL-UNSPEC" || !unspecFn.System || unspecFn.Biz[4] != "CUST-UNSPEC" {
		t.Fatalf("unspec nodes: %+v %+v", unspecFeat, unspecFn)
	}
	seen := map[int]bool{}
	for _, n := range cat.ProjectNodes() {
		if seen[n.ID] {
			t.Fatalf("duplicate node id %d", n.ID)
		}
		seen[n.ID] = true
	}
}

func TestUnmatchedTagsDoNotBlockBottomUp(t *testing.T) {
	out := sampleCatalog(t).Resolve(Input{AccountID: "777788889999", Project: "ProjectOmega", App: "DataLakehouse"})
	if out.Project.ID != "PRJ-LAKEHOUSE" || out.App.ID != "APP-LAKE" {
		t.Fatalf("derived from account despite bad tags: %+v %+v", out.Project, out.App)
	}
	out = sampleCatalog(t).Resolve(Input{Project: "ProjectOmega"})
	if out.Project != (Slot{Name: "ProjectOmega"}) {
		t.Fatalf("unmatched tag kept when nothing derives: %+v", out.Project)
	}
}

func TestUnmappedAccountDoesNotBorrowHierarchy(t *testing.T) {
	out := sampleCatalog(t).Resolve(Input{AccountID: "999999999999", AccountName: "mystery"})
	if out.Project != (Slot{}) || out.Account.ID != "999999999999" {
		t.Fatalf("unmapped account: %+v", out)
	}
}

func TestValidationReportsAllErrors(t *testing.T) {
	_, err := buildJSON(t, `{
	  "business_units": [{"code": "BU", "name": "BU"}, {"code": "BU", "name": "again"}, {"code": "", "name": "no code"}],
	  "cost_centers": [{"code": "CC", "name": "CC"}],
	  "projects": [{"code": "P", "name": "P", "business_unit": "NOPE"}]
	}`)
	if err == nil {
		t.Fatal("expected errors")
	}
	for _, want := range []string{"duplicate code", "code and name are required", "business_unit is required", `business_unit "NOPE"`} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("missing %q in:\n%v", want, err)
		}
	}
}

func TestRefsAcceptUniqueNames(t *testing.T) {
	if _, err := buildJSON(t, `{
	  "business_units": [{"code": "BU", "name": "Digital"}],
	  "cost_centers": [{"code": "CC", "name": "CC", "business_unit": "Digital"}]}`); err != nil {
		t.Fatalf("a unique name must resolve: %v", err)
	}
	_, err := buildJSON(t, `{
	  "business_units": [{"code": "A", "name": "Same"}, {"code": "B", "name": "Same"}],
	  "cost_centers": [{"code": "CC", "name": "CC", "business_unit": "Same"}]}`)
	if err == nil || !strings.Contains(err.Error(), "matches several") {
		t.Fatalf("duplicate names are allowed but never matched by name: %v", err)
	}
}

func TestSnapshotRoundTripAndLegacyKeys(t *testing.T) {
	snap, err := ReadSnapshot(sampleFile)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "sub", "taxonomy.json")
	if err := WriteSnapshot(path, snap); err != nil {
		t.Fatal(err)
	}
	again, err := ReadSnapshot(path)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := snap.MarshalJSON()
	b, _ := again.MarshalJSON()
	if string(a) != string(b) {
		t.Fatal("round trip changed the snapshot")
	}
	if strings.Contains(string(a), `"id"`) || strings.Contains(string(a), "tenant") {
		t.Fatal("written snapshots use code and have no tenant")
	}

	// "id" is read as "code" and a legacy "tenant" key is ignored.
	legacy := `{"tenant": "default", "business_units": [{"id": "BU", "name": "BU"}]}`
	var old Snapshot
	if err := json.Unmarshal([]byte(legacy), &old); err != nil {
		t.Fatal(err)
	}
	if old.Rows(BusinessUnits)[0].Code() != "BU" {
		t.Fatalf("id alias: %+v", old.Rows(BusinessUnits))
	}
}

func TestNumericCodesAreKeptVerbatim(t *testing.T) {
	var snap Snapshot
	if err := json.Unmarshal([]byte(`{"accounts": [{"code": 111122223333, "name": "n"}]}`), &snap); err != nil {
		t.Fatal(err)
	}
	if snap.Rows(Accounts)[0].Code() != "111122223333" {
		t.Fatalf("code: %q", snap.Rows(Accounts)[0].Code())
	}
}

func TestUnknownCollectionRejected(t *testing.T) {
	var snap Snapshot
	if err := json.Unmarshal([]byte(`{"app_environments": []}`), &snap); err == nil {
		t.Fatal("collections that are not in Schema must be rejected")
	}
}

func TestLoadOptional(t *testing.T) {
	dir := t.TempDir()
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	_ = os.Chdir(dir)
	if cat, err := LoadOptional(""); cat != nil || err != nil {
		t.Fatalf("missing default snapshot means no taxonomy: %v %v", cat, err)
	}
	if _, err := LoadOptional("missing.json"); err == nil {
		t.Fatal("an explicit missing path must fail")
	}
}
