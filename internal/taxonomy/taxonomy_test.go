package taxonomy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSampleTaxonomy(t *testing.T) {
	root := filepath.Join("..", "..", "taxonomy")
	cat, err := Load(filepath.Join(root, "project_taxonomy.json"), filepath.Join(root, "business_taxonomy.json"))
	if err != nil {
		t.Fatalf("Load sample taxonomy: %v", err)
	}

	out := cat.Resolve(Input{
		AccountID: "100000000001",
		Service:   "Amazon Elastic Compute Cloud - Compute",
		Project:   "Untagged",
		App:       "Untagged",
	})
	if out.Project != (Slot{ID: "BU-DIGITAL", Name: "Digital"}) {
		t.Fatalf("project: %+v", out.Project)
	}
	if out.BusinessUnit != out.Project {
		t.Fatalf("project must behave as business unit, got %+v vs %+v", out.Project, out.BusinessUnit)
	}
	if out.App != (Slot{ID: "OWN-PAY", Name: "Payments"}) || out.Owner != out.App {
		t.Fatalf("app/owner: %+v %+v", out.App, out.Owner)
	}
	if out.Environment != (Slot{ID: "CC-PROD", Name: "Production"}) || out.CostCenter != out.Environment {
		t.Fatalf("environment/cost center: %+v %+v", out.Environment, out.CostCenter)
	}
	if out.ServiceCategory != (Slot{ID: "SC-COMPUTE", Name: "Compute"}) {
		t.Fatalf("service category: %+v", out.ServiceCategory)
	}
	if out.CustomerFunction != (Slot{ID: "CF-CHECKOUT", Name: "Checkout"}) {
		t.Fatalf("customer function: %+v", out.CustomerFunction)
	}
}

func TestServiceDoesNotStealUnmappedAccount(t *testing.T) {
	cat := loadFixture(t)
	out := cat.Resolve(Input{
		AccountID: "999900001111",
		Service:   "Amazon Elastic Compute Cloud - Compute",
	})
	if out.ServiceCategory.ID != "SC-COMPUTE" {
		t.Fatalf("service should still map to its category, got %+v", out.ServiceCategory)
	}
	if out.Project.ID != "" || out.CustomerFunction.ID != "" || out.Environment.ID != "" {
		t.Fatalf("unmapped account must not inherit another environment: %+v", out)
	}
}

func TestResolveKeepsExplicitTags(t *testing.T) {
	cat := loadFixture(t)
	out := cat.Resolve(Input{
		AccountID: "100000000001",
		Project:   "Kept Project",
		App:       "Kept App",
	})
	if out.Project.Name != "Kept Project" || out.App.Name != "Kept App" {
		t.Fatalf("explicit tags were replaced: %+v %+v", out.Project, out.App)
	}
	if out.BusinessUnit.Name != "Kept Project" || out.Owner.Name != "Kept App" {
		t.Fatalf("explicit tags must still behave across hierarchies: %+v %+v", out.BusinessUnit, out.Owner)
	}
	if out.Environment.ID != "CC-PROD" {
		t.Fatalf("empty environment should still map from account, got %+v", out.Environment)
	}
}

func TestAppMapsToProject(t *testing.T) {
	dir := t.TempDir()
	business := `{
	  "businessUnits": [{
	    "id": "BU-A", "name": "Alpha",
	    "owners": [
	      {"id": "OWN-1", "name": "One", "costCenters": [{"id": "CC-1", "name": "Prod", "customerFuncs": [{"id": "CF-1", "name": "Func"}]}]},
	      {"id": "OWN-2", "name": "Two", "costCenters": [{"id": "CC-2", "name": "Dev", "customerFuncs": []}]}
	    ]
	  }]
	}`
	project := `{
	  "projects": [
	    {"id": "BU-A", "name": "Alpha", "businessUnitId": "BU-A", "apps": [
	      {"id": "OWN-2", "name": "Two", "ownerId": "OWN-2", "environments": [
	        {"id": "CC-2", "name": "Dev", "costCenterId": "CC-2", "accounts": [], "serviceCategories": []}
	      ]},
	      {"id": "OWN-1", "name": "One", "ownerId": "OWN-1", "projectId": "BU-A", "environments": [
	        {"id": "CC-1", "name": "Prod", "costCenterId": "CC-1", "accounts": [
	          {"id": "acct-1", "name": "prod-acct", "environmentId": "CC-2"}
	        ], "serviceCategories": [
	          {"id": "SC-1", "name": "Compute", "customerFuncId": "CF-1", "services": [
	            {"id": "svc-1", "name": "Lambda", "serviceCatId": "SC-1"}
	          ]}
	        ]}
	      ]}
	    ]}
	  ]
	}`
	cat := writeAndLoad(t, dir, project, business)
	out := cat.Resolve(Input{App: "OWN-1"})
	if out.Project.ID != "BU-A" || out.Owner.ID != "OWN-1" {
		t.Fatalf("app should map to its project and behave as owner: %+v", out)
	}
	mapped := cat.Resolve(Input{AccountID: "acct-1"})
	if mapped.Environment.ID != "CC-2" {
		t.Fatalf("account should map to environmentId CC-2, got %+v", mapped.Environment)
	}
	svc := cat.Resolve(Input{Service: "Lambda"})
	if svc.ServiceCategory.ID != "SC-1" || svc.CustomerFunction.ID != "CF-1" {
		t.Fatalf("service should map to category and customer function: %+v", svc)
	}
}

func TestMustBehaveRejectsDifferentIDs(t *testing.T) {
	dir := t.TempDir()
	business := `{"businessUnits":[{"id":"BU-A","name":"Alpha","owners":[{"id":"OWN-1","name":"One","costCenters":[{"id":"CC-1","name":"Prod","customerFuncs":[]}]}]}]}`
	project := `{"projects":[{"id":"PRJ-OTHER","name":"Alpha","businessUnitId":"BU-A","apps":[]}]}`
	_, err := Load(writePair(t, dir, project, business))
	if err == nil {
		t.Fatal("expected project id to be rejected when it does not behave as the business unit id")
	}
}

func TestHierarchyTaxonomyRelations(t *testing.T) {
	dir := t.TempDir()
	business := `{
	  "businessUnits": [
	    {
	      "id": "BU-CORP",
	      "name": "Corporate",
	      "owners": [
	        {
	          "id": "OWN-FIN",
	          "name": "Finance",
	          "costCenters": [
	            {
	              "id": "CC-FIN-PROD",
	              "name": "Finance Production",
	              "customerFuncs": [
	                { "id": "CF-GL", "name": "General Ledger" }
	              ]
	            }
	          ]
	        }
	      ]
	    }
	  ]
	}`

	// Test mapping by name as well as ID:
	// - ProjectID/Name behaves as BusinessUnitID/Name
	// - AppID/Name behaves as OwnerID/Name
	// - EnvironmentID/Name behaves as CostCenterID/Name
	// - App mapped to Project by Name ("Corporate")
	// - Account mapped to Environment by Name ("Finance Production")
	// - Service mapped to ServiceCategory by Name ("Ledger Compute")
	project := `{
	  "projects": [
	    {
	      "id": "BU-CORP",
	      "name": "Corporate",
	      "businessUnitId": "BU-CORP",
	      "apps": [
	        {
	          "id": "OWN-FIN",
	          "name": "Finance",
	          "ownerId": "OWN-FIN",
	          "projectId": "BU-CORP",
	          "environments": [
	            {
	              "id": "CC-FIN-PROD",
	              "name": "Finance Production",
	              "costCenterName": "Finance Production",
	              "accounts": [
	                {
	                  "id": "999888777666",
	                  "name": "lz-corp-fin-prod",
	                  "environmentName": "Finance Production"
	                }
	              ],
	              "serviceCategories": [
	                {
	                  "id": "SC-LEDGER",
	                  "name": "Ledger Compute",
	                  "customerFuncName": "General Ledger",
	                  "services": [
	                    {
	                      "id": "AWSLambda",
	                      "name": "AWS Lambda Serverless",
	                      "serviceCatName": "Ledger Compute"
	                    }
	                  ]
	                }
	              ]
	            }
	          ]
	        }
	      ]
	    }
	  ]
	}`

	cat := writeAndLoad(t, dir, project, business)

	// 1. Account -> Environment mapping + CostCenter behave + App/Owner + Project/BusinessUnit
	res := cat.Resolve(Input{
		AccountID: "999888777666",
		Service:   "AWSLambda",
	})

	// Rule 1: ProjectID/Name must behave BusinessUnitID/Name
	if res.Project.ID != "BU-CORP" || res.Project.Name != "Corporate" {
		t.Errorf("Project mismatch: %+v", res.Project)
	}
	if res.BusinessUnit.ID != res.Project.ID || res.BusinessUnit.Name != res.Project.Name {
		t.Errorf("Project must behave as BusinessUnit: Project=%+v BU=%+v", res.Project, res.BusinessUnit)
	}

	// Rule 2: AppID/Name must behave OwnerID/Name
	if res.App.ID != "OWN-FIN" || res.App.Name != "Finance" {
		t.Errorf("App mismatch: %+v", res.App)
	}
	if res.Owner.ID != res.App.ID || res.Owner.Name != res.App.Name {
		t.Errorf("App must behave as Owner: App=%+v Owner=%+v", res.App, res.Owner)
	}

	// Rule 3: EnvironmentID/Name must behave CostCenterID/Name
	if res.Environment.ID != "CC-FIN-PROD" || res.Environment.Name != "Finance Production" {
		t.Errorf("Environment mismatch: %+v", res.Environment)
	}
	if res.CostCenter.ID != res.Environment.ID || res.CostCenter.Name != res.Environment.Name {
		t.Errorf("Environment must behave as CostCenter: Env=%+v CC=%+v", res.Environment, res.CostCenter)
	}

	// Rule 4: AppID/Name can be mapped to ProjectID/Name
	appRes := cat.Resolve(Input{App: "Finance"})
	if appRes.Project.ID != "BU-CORP" || appRes.Project.Name != "Corporate" {
		t.Errorf("App mapped to Project failed: %+v", appRes.Project)
	}

	// Rule 5: AccountID/Name can be mapped to EnvironmentID/Name
	acctRes := cat.Resolve(Input{AccountName: "lz-corp-fin-prod"})
	if acctRes.Environment.ID != "CC-FIN-PROD" || acctRes.Environment.Name != "Finance Production" {
		t.Errorf("Account mapped to Environment failed: %+v", acctRes.Environment)
	}

	// Rule 6: ServiceID/Name can be mapped to ServiceCatID/Name
	if res.ServiceCategory.ID != "SC-LEDGER" || res.ServiceCategory.Name != "Ledger Compute" {
		t.Errorf("Service mapped to ServiceCategory failed: %+v", res.ServiceCategory)
	}
	if res.CustomerFunction.ID != "CF-GL" || res.CustomerFunction.Name != "General Ledger" {
		t.Errorf("CustomerFunction linked failed: %+v", res.CustomerFunction)
	}
}

func TestLoadOptionalMissingDefaults(t *testing.T) {
	t.Chdir(t.TempDir())
	cat, err := LoadOptional("", "")
	if err != nil {
		t.Fatalf("missing defaults should skip: %v", err)
	}
	if cat != nil {
		t.Fatal("expected nil catalog when taxonomy files are absent")
	}
}

func loadFixture(t *testing.T) *Catalog {
	t.Helper()
	root := filepath.Join("..", "..", "taxonomy")
	cat, err := Load(filepath.Join(root, "project_taxonomy.json"), filepath.Join(root, "business_taxonomy.json"))
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}
	return cat
}

func writeAndLoad(t *testing.T, dir, project, business string) *Catalog {
	t.Helper()
	p, b := writePair(t, dir, project, business)
	cat, err := Load(p, b)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return cat
}

func writePair(t *testing.T, dir, project, business string) (string, string) {
	t.Helper()
	p := filepath.Join(dir, "project_taxonomy.json")
	b := filepath.Join(dir, "business_taxonomy.json")
	if err := os.WriteFile(p, []byte(project), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte(business), 0o644); err != nil {
		t.Fatal(err)
	}
	return p, b
}

func TestTenantPathResolution(t *testing.T) {
	if got := DefaultProjectPathForTenant("tenant_a"); got != "taxonomy/project_taxonomy_tenant_a.json" {
		t.Errorf("expected taxonomy/project_taxonomy_tenant_a.json, got %s", got)
	}
	if got := DefaultBusinessPathForTenant("tenant_a"); got != "taxonomy/business_taxonomy_tenant_a.json" {
		t.Errorf("expected taxonomy/business_taxonomy_tenant_a.json, got %s", got)
	}
	if got := DefaultProjectPathForTenant(""); got != DefaultProjectPath {
		t.Errorf("expected %s, got %s", DefaultProjectPath, got)
	}
	if got := DefaultBusinessPathForTenant(""); got != DefaultBusinessPath {
		t.Errorf("expected %s, got %s", DefaultBusinessPath, got)
	}

	// Explicit override takes precedence
	if got := ResolveProjectTaxonomyPath("custom/path.json", "tenant_a"); got != "custom/path.json" {
		t.Errorf("expected custom/path.json, got %s", got)
	}
	if got := ResolveBusinessTaxonomyPath("custom/biz.json", "tenant_a"); got != "custom/biz.json" {
		t.Errorf("expected custom/biz.json, got %s", got)
	}
}

func TestDiscoverTenants(t *testing.T) {
	tempDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tempDir, "project_taxonomy_tenant_x.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "business_taxonomy_tenant_x.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "tenant_y_project_taxonomy.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "project_taxonomy.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	tenants, err := DiscoverTenants(tempDir)
	if err != nil {
		t.Fatalf("DiscoverTenants error: %v", err)
	}
	if len(tenants) != 2 || tenants[0] != "tenant_x" || tenants[1] != "tenant_y" {
		t.Errorf("expected [tenant_x, tenant_y], got %v", tenants)
	}
}

func TestLoadTenantATaxonomy(t *testing.T) {
	root := filepath.Join("..", "..", "taxonomy")
	cat, err := LoadOptionalForTenant(
		filepath.Join(root, "project_taxonomy_tenant_a.json"),
		filepath.Join(root, "business_taxonomy_tenant_a.json"),
		"tenant_a",
	)
	if err != nil {
		t.Fatalf("Load tenant_a taxonomy: %v", err)
	}
	if cat == nil {
		t.Fatal("expected non-nil catalog for tenant_a")
	}

	out := cat.Resolve(Input{
		AccountID: "111122223333",
		Service:   "Amazon Elastic Compute Cloud - Compute",
	})
	if out.Project.ID != "BU-RETAIL" || out.Project.Name != "Retail Platform" {
		t.Errorf("expected BU-RETAIL project, got %+v", out.Project)
	}
	if out.App.ID != "OWN-COMMERCE" || out.App.Name != "E-Commerce" {
		t.Errorf("expected OWN-COMMERCE app, got %+v", out.App)
	}
	if out.Environment.ID != "CC-RETAIL-PROD" || out.Environment.Name != "Retail Production" {
		t.Errorf("expected CC-RETAIL-PROD environment, got %+v", out.Environment)
	}
	if out.CustomerFunction.ID != "CF-STOREFRONT" {
		t.Errorf("expected CF-STOREFRONT customer function, got %+v", out.CustomerFunction)
	}
}


