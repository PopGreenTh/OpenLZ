package taxonomy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func sampleCatalog(t *testing.T) *Catalog {
	t.Helper()
	cat, err := Load(filepath.Join("..", "..", "taxonomy", "taxonomy.json"))
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

func TestSampleTaxonomiesLoadWithoutWarnings(t *testing.T) {
	for _, name := range []string{"taxonomy.json", "taxonomy_tenant_a.json"} {
		cat, err := Load(filepath.Join("..", "..", "taxonomy", name))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(cat.Warnings()) > 0 {
			t.Fatalf("%s: unexpected warnings %v", name, cat.Warnings())
		}
	}
}

func TestResolveFullHierarchyFromResource(t *testing.T) {
	out := sampleCatalog(t).Resolve(Input{
		AccountID:  "111122223333",
		CSP:        "AWS",
		Region:     "ap-southeast-1",
		Service:    "Amazon Elastic Compute Cloud - Compute",
		UsageType:  "APS1-BoxUsage:t3.medium",
		ResourceID: "i-0111122223333a001",
		Project:    "Untagged",
		App:        "Untagged",
	})

	want := map[string]Slot{
		"Account":          {ID: "111122223333", Name: "lz-prod-payments"},
		"Environment":      {ID: "ENV-GW-PROD", Name: "Production"},
		"App":              {ID: "APP-GATEWAY", Name: "Payment Gateway"},
		"Project":          {ID: "PRJ-PAYMENTS", Name: "Payments Platform"},
		"BusinessUnit":     {ID: "BU-DIGITAL", Name: "Digital"},
		"CostCenter":       {ID: "CC-PAY", Name: "Payments"},
		"Owner":            {ID: "OWN-PAY", Name: "Payments Team"},
		"CustomerFunction": {ID: "CF-CHECKOUT", Name: "Checkout"},
		"CSP":              {ID: "AWS", Name: "Amazon Web Services"},
		"Region":           {ID: "ap-southeast-1", Name: "Asia Pacific (Singapore)"},
		"ServiceCategory":  {ID: "SC-COMPUTE", Name: "Compute"},
		"Service":          {ID: "AmazonEC2", Name: "Amazon Elastic Compute Cloud - Compute"},
		"ServiceUsage":     {ID: "SU-EC2-INSTANCE", Name: "EC2 Instance Hours"},
		"Usage":            {ID: "BoxUsage:t3.medium", Name: "t3.medium instance hours"},
		"ResourceGroup":    {ID: "RG-CHECKOUT", Name: "Checkout Compute"},
		"Resource":         {ID: "i-0111122223333a001", Name: "checkout-node-1"},
	}
	got := map[string]Slot{
		"Account": out.Account, "Environment": out.Environment, "App": out.App, "Project": out.Project,
		"BusinessUnit": out.BusinessUnit, "CostCenter": out.CostCenter, "Owner": out.Owner,
		"CustomerFunction": out.CustomerFunction, "CSP": out.CSP, "Region": out.Region,
		"ServiceCategory": out.ServiceCategory, "Service": out.Service, "ServiceUsage": out.ServiceUsage,
		"Usage": out.Usage, "ResourceGroup": out.ResourceGroup, "Resource": out.Resource,
	}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("%s = %+v, want %+v", k, got[k], w)
		}
	}
}

func TestResolveWithoutResourcePicksOnlyOwner(t *testing.T) {
	out := sampleCatalog(t).Resolve(Input{AccountID: "777788889999", Service: "AmazonS3"})
	if out.CostCenter.ID != "CC-DATA" || out.BusinessUnit.ID != "BU-DATA" {
		t.Fatalf("cost center/bu from app: %+v %+v", out.CostCenter, out.BusinessUnit)
	}
	// CC-DATA has a single owner with a single customer function.
	if out.Owner.ID != "OWN-DATA" || out.CustomerFunction.ID != "CF-LAKE" {
		t.Fatalf("owner/func should follow the only child: %+v %+v", out.Owner, out.CustomerFunction)
	}
	if out.ServiceCategory.ID != "SC-STORAGE" {
		t.Fatalf("service category: %+v", out.ServiceCategory)
	}
}

func TestResolveAmbiguousOwnerStaysEmpty(t *testing.T) {
	// CC-PAY has one owner, but OWN-PAY has two customer functions.
	out := sampleCatalog(t).Resolve(Input{AccountID: "111122223333"})
	if out.Owner.ID != "OWN-PAY" {
		t.Fatalf("owner: %+v", out.Owner)
	}
	if out.CustomerFunction != (Slot{}) {
		t.Fatalf("customer function must stay empty when ambiguous, got %+v", out.CustomerFunction)
	}
}

func TestUnmappedAccountDoesNotBorrowHierarchy(t *testing.T) {
	out := sampleCatalog(t).Resolve(Input{AccountID: "999900001111", Service: "AmazonEC2"})
	if out.ServiceCategory.ID != "SC-COMPUTE" {
		t.Fatalf("service should still map to its category, got %+v", out.ServiceCategory)
	}
	if out.Project != (Slot{}) || out.Environment != (Slot{}) || out.CostCenter != (Slot{}) || out.BusinessUnit != (Slot{}) {
		t.Fatalf("unmapped account must not inherit a hierarchy: %+v", out)
	}
	if out.Account.ID != "999900001111" {
		t.Fatalf("raw account id should pass through: %+v", out.Account)
	}
}

func TestResolveKeepsExplicitTags(t *testing.T) {
	out := sampleCatalog(t).Resolve(Input{AccountID: "111122223333", Project: "Kept Project", App: "Kept App"})
	if out.Project != (Slot{Name: "Kept Project"}) || out.App != (Slot{Name: "Kept App"}) {
		t.Fatalf("explicit tags were replaced: %+v %+v", out.Project, out.App)
	}
	if out.Environment.ID != "ENV-GW-PROD" {
		t.Fatalf("environment should still map from account, got %+v", out.Environment)
	}
}

func TestResolveTagByNameDrivesHierarchy(t *testing.T) {
	out := sampleCatalog(t).Resolve(Input{Project: "Lakehouse", App: "Data Lakehouse"})
	if out.Project.ID != "PRJ-LAKEHOUSE" || out.App.ID != "APP-LAKE" {
		t.Fatalf("tags by name: %+v %+v", out.Project, out.App)
	}
	if out.BusinessUnit.ID != "BU-DATA" || out.CostCenter.ID != "CC-DATA" {
		t.Fatalf("project must ref BU and app must ref cost center: %+v %+v", out.BusinessUnit, out.CostCenter)
	}
}

func TestRegionUniqueWithinCSP(t *testing.T) {
	cat, err := buildJSON(t, `{
	  "csps": [{"id": "AWS", "name": "AWS"}, {"id": "GCP", "name": "Google Cloud"}],
	  "regions": [
	    {"id": "asia-east", "name": "AWS Asia", "csp": "AWS"},
	    {"id": "asia-east", "name": "GCP Asia", "csp": "GCP"}
	  ]
	}`)
	if err != nil {
		t.Fatalf("same region id in different CSPs must be allowed: %v", err)
	}
	if out := cat.Resolve(Input{CSP: "GCP", Region: "asia-east"}); out.Region.Name != "GCP Asia" {
		t.Fatalf("region should resolve within its CSP: %+v", out.Region)
	}
	if out := cat.Resolve(Input{Region: "asia-east"}); out.Region.ID != "" {
		t.Fatalf("region without CSP is ambiguous, got %+v", out.Region)
	}

	_, err = buildJSON(t, `{
	  "csps": [{"id": "AWS", "name": "AWS"}],
	  "regions": [
	    {"id": "us-east-1", "name": "A", "csp": "AWS"},
	    {"id": "us-east-1", "name": "B", "csp": "AWS"}
	  ]
	}`)
	if err == nil || !strings.Contains(err.Error(), "duplicate id") {
		t.Fatalf("duplicate region within a CSP must fail, got %v", err)
	}
}

func TestValidationReportsAllErrors(t *testing.T) {
	_, err := buildJSON(t, `{
	  "business_units": [{"id": "BU-1", "name": "One"}],
	  "cost_centers": [{"id": "CC-1", "name": "CC"}],
	  "projects": [{"id": "P-1", "name": "P", "business_unit": "BU-404"}],
	  "apps": [{"id": "A-1", "name": "A", "project": "P-1"}],
	  "accounts": [{"id": "", "name": "no id"}]
	}`)
	if err == nil {
		t.Fatal("expected validation errors")
	}
	for _, want := range []string{
		`Cost Center "CC-1": business_unit is required`,
		`Project "P-1": business_unit "BU-404" does not match`,
		`App "A-1": cost_center is required`,
		`accounts[0]: id and name are required`,
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("missing error %q in:\n%v", want, err)
		}
	}
}

func TestRefsAcceptUniqueNames(t *testing.T) {
	cat, err := buildJSON(t, `{
	  "business_units": [{"id": "BU-1", "name": "One"}],
	  "cost_centers": [{"id": "CC-1", "name": "Ops", "business_unit": "One"}],
	  "projects": [{"id": "P-1", "name": "P", "business_unit": "one"}],
	  "apps": [{"id": "A-1", "name": "A", "project": "P", "cost_center": "Ops"}]
	}`)
	if err != nil {
		t.Fatalf("refs by name: %v", err)
	}
	if out := cat.Resolve(Input{App: "A-1"}); out.BusinessUnit.ID != "BU-1" {
		t.Fatalf("bu via app→project: %+v", out.BusinessUnit)
	}
}

func TestAppCostCenterInOtherBusinessUnitWarns(t *testing.T) {
	cat, err := buildJSON(t, `{
	  "business_units": [{"id": "BU-1", "name": "One"}, {"id": "BU-2", "name": "Two"}],
	  "cost_centers": [{"id": "CC-2", "name": "Two CC", "business_unit": "BU-2"}],
	  "projects": [{"id": "P-1", "name": "P", "business_unit": "BU-1"}],
	  "apps": [{"id": "A-1", "name": "A", "project": "P-1", "cost_center": "CC-2"}]
	}`)
	if err != nil {
		t.Fatalf("mismatch must only warn: %v", err)
	}
	if len(cat.Warnings()) != 1 || !strings.Contains(cat.Warnings()[0], `App "A-1"`) {
		t.Fatalf("expected one warning for A-1, got %v", cat.Warnings())
	}
	// Business unit follows the project; cost center follows the app.
	out := cat.Resolve(Input{App: "A-1"})
	if out.BusinessUnit.ID != "BU-1" || out.CostCenter.ID != "CC-2" {
		t.Fatalf("got bu=%+v cc=%+v", out.BusinessUnit, out.CostCenter)
	}
}

func TestDuplicateNamesAreAllowedButNotMatched(t *testing.T) {
	cat := sampleCatalog(t)
	// Two environments are named "Production".
	if out := cat.Resolve(Input{Environment: "Production"}); out.Environment != (Slot{Name: "Production"}) {
		t.Fatalf("ambiguous name must not resolve: %+v", out.Environment)
	}
}

func TestSnapshotRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out", "taxonomy_x.json")
	snap, err := ReadSnapshot(filepath.Join("..", "..", "taxonomy", "taxonomy.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteSnapshot(path, snap); err != nil {
		t.Fatal(err)
	}
	again, err := ReadSnapshot(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, spec := range Schema {
		if len(again.Rows(spec.Name)) != len(snap.Rows(spec.Name)) {
			t.Fatalf("%s: %d rows after round trip, want %d", spec.Name, len(again.Rows(spec.Name)), len(snap.Rows(spec.Name)))
		}
	}
	if again.Tenant != "default" {
		t.Fatalf("tenant: %q", again.Tenant)
	}
}

func TestNumericIDsAreKeptVerbatim(t *testing.T) {
	var snap Snapshot
	if err := json.Unmarshal([]byte(`{"accounts": [{"id": 100000000001, "name": "acct"}]}`), &snap); err != nil {
		t.Fatal(err)
	}
	if got := snap.Rows(Accounts)[0].ID(); got != "100000000001" {
		t.Fatalf("id = %q", got)
	}
}

func TestUnknownCollectionRejected(t *testing.T) {
	var snap Snapshot
	if err := json.Unmarshal([]byte(`{"owner": []}`), &snap); err == nil {
		t.Fatal("expected unknown collection error")
	}
}

func TestPathsAndDiscovery(t *testing.T) {
	if got := ResolvePath("", ""); got != filepath.Join("taxonomy", "taxonomy.json") {
		t.Fatalf("default path: %s", got)
	}
	if got := ResolvePath("", "acme"); got != filepath.Join("taxonomy", "taxonomy_acme.json") {
		t.Fatalf("tenant path: %s", got)
	}
	if got := ResolvePath("x.json", "acme"); got != "x.json" {
		t.Fatalf("explicit path: %s", got)
	}

	dir := t.TempDir()
	for _, f := range []string{"taxonomy.json", "taxonomy_b.json", "taxonomy_a.json", "notes.txt"} {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	tenants, err := DiscoverTenants(dir)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(tenants, ",") != "a,b" {
		t.Fatalf("tenants: %v", tenants)
	}
}

func TestLoadOptional(t *testing.T) {
	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if cat, err := LoadOptional("", ""); err != nil || cat != nil {
		t.Fatalf("missing default should skip enrichment: %v %v", cat, err)
	}
	if _, err := LoadOptional("missing.json", ""); err == nil {
		t.Fatal("missing explicit path must fail")
	}
}
