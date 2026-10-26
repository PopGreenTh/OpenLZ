package taxonomy

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func validBaseTaxonomy() *Taxonomy {
	return &Taxonomy{
		BusinessUnits: []*BusinessUnit{
			{Code: "BU-RETAIL", Name: "Retail", File: "taxonomy/bunits.yaml"},
		},
		CostCenters: []*CostCenter{
			{Code: "CC-ECOM", Name: "E-commerce", BusinessUnit: "BU-RETAIL", File: "taxonomy/ccs.yaml"},
		},
		Projects: []*Project{
			{Code: "PRJ-SHOP", Name: "Online shop", BusinessUnit: "BU-RETAIL", LZs: []string{"LZ-MAIN"}, File: "taxonomy/projects.yaml"},
		},
		Apps: []*App{
			{
				Code:       "APP-CHK",
				Name:       "Checkout",
				Project:    "PRJ-SHOP",
				CostCenter: "CC-ECOM",
				Environments: []*Environment{
					{Code: "ENV-PROD", Name: "Production"},
					{Code: "ENV-DEV", Name: "Development"},
				},
				File: "taxonomy/apps.yaml",
			},
		},
		LZs: []*LZ{
			{
				Code:              "LZ-MAIN",
				Name:              "Main",
				CSP:               "AWS",
				Currency:          "USD",
				ManagementAccount: "111111111111",
				Default: &LZDefault{
					App:         "APP-CHK",
					Environment: "ENV-PROD",
				},
				File: "taxonomy/lzs.yaml",
			},
		},
		Defaults: &Defaults{
			App:         "APP-CHK",
			Environment: "ENV-DEV",
			File:        "taxonomy/defaults.yaml",
		},
		OULinks: []*OULink{
			{OU: "/Workloads/Checkout", App: "APP-CHK", File: "taxonomy/ou_links.yaml"},
		},
		Accounts: []*AccountEntry{
			{
				Account:     "222222222222",
				Environment: "ENV-PROD",
				Apps: []*AccountApp{
					{App: "APP-CHK"},
				},
				File: "taxonomy/accounts.yaml",
			},
		},
		CostAllocation: &CostAllocation{
			AppTag: "App",
			File:   "taxonomy/cost_allocation.yaml",
		},
	}
}

func assertOnlyFinding(t *testing.T, findings []Finding, expectedCode string) {
	t.Helper()
	if len(findings) != 1 {
		t.Fatalf("expected exactly 1 finding with code %q, got %d findings: %+v", expectedCode, len(findings), findings)
	}
	if findings[0].Code != expectedCode {
		t.Fatalf("expected finding code %q, got %q (path: %s, message: %s)", expectedCode, findings[0].Code, findings[0].Path, findings[0].Message)
	}
	if findings[0].Path == "" && expectedCode != "yaml-invalid" {
		t.Fatalf("finding path must not be empty for %q", expectedCode)
	}
}

func TestValidate_ValidDocument(t *testing.T) {
	tax := validBaseTaxonomy()
	findings := Validate(tax)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings for valid base taxonomy, got: %+v", findings)
	}
}

func TestValidate_CodeInvalid(t *testing.T) {
	tax := validBaseTaxonomy()
	// Change APP-CHK to use reserved suffix -UNALLOC
	tax.Apps[0].Code = "APP-UNALLOC"
	tax.LZs[0].Default.App = "APP-UNALLOC"
	tax.Defaults.App = "APP-UNALLOC"
	tax.OULinks[0].App = "APP-UNALLOC"
	tax.Accounts[0].Apps[0].App = "APP-UNALLOC"
	findings := Validate(tax)
	assertOnlyFinding(t, findings, "code-invalid")
}

func TestValidate_CodeDuplicate(t *testing.T) {
	tax := validBaseTaxonomy()
	tax.Projects = append(tax.Projects, &Project{
		Code:         "PRJ-SHOP",
		Name:         "Duplicate Project",
		BusinessUnit: "BU-RETAIL",
		File:         "taxonomy/projects.yaml",
	})
	findings := Validate(tax)
	assertOnlyFinding(t, findings, "code-duplicate")
}

func TestValidate_RefMissing(t *testing.T) {
	tax := validBaseTaxonomy()
	tax.CostCenters[0].BusinessUnit = ""
	findings := Validate(tax)
	assertOnlyFinding(t, findings, "ref-missing")
}

func TestValidate_RefUnknown(t *testing.T) {
	t.Run("project_lz_unknown", func(t *testing.T) {
		tax := validBaseTaxonomy()
		tax.Projects[0].LZs = append(tax.Projects[0].LZs, "LZ-UNKNOWN")
		findings := Validate(tax)
		assertOnlyFinding(t, findings, "ref-unknown")
	})

	t.Run("ou_link_env_without_app_unknown", func(t *testing.T) {
		tax := validBaseTaxonomy()
		tax.OULinks = append(tax.OULinks, &OULink{
			OU:          "/Workloads/Unknown",
			Environment: "ENV-NONEXISTENT",
			File:        "taxonomy/ou_links.yaml",
		})
		findings := Validate(tax)
		assertOnlyFinding(t, findings, "ref-unknown")
	})

	t.Run("account_env_without_apps_unknown", func(t *testing.T) {
		tax := validBaseTaxonomy()
		tax.Accounts = append(tax.Accounts, &AccountEntry{
			Account:     "999999999999",
			Environment: "ENV-NONEXISTENT",
			File:        "taxonomy/accounts.yaml",
		})
		findings := Validate(tax)
		assertOnlyFinding(t, findings, "ref-unknown")
	})

	t.Run("lz_default_env_without_app_unknown", func(t *testing.T) {
		tax := validBaseTaxonomy()
		tax.LZs[0].Default = &LZDefault{
			Environment: "ENV-NONEXISTENT",
		}
		findings := Validate(tax)
		assertOnlyFinding(t, findings, "ref-unknown")
	})

	t.Run("global_default_env_without_app_unknown", func(t *testing.T) {
		tax := validBaseTaxonomy()
		// Keep ENV-DEV referenced via an OU link so it's not reported as unused-node
		tax.OULinks = append(tax.OULinks, &OULink{
			OU:          "/Workloads/Dev",
			Environment: "ENV-DEV",
			File:        "taxonomy/ou_links.yaml",
		})
		tax.Defaults = &Defaults{
			Environment: "ENV-NONEXISTENT",
			File:        "taxonomy/defaults.yaml",
		}
		findings := Validate(tax)
		assertOnlyFinding(t, findings, "ref-unknown")
	})
}

func TestValidate_LZInvalid(t *testing.T) {
	tax := validBaseTaxonomy()
	tax.LZs[0].CSP = "GCP"
	findings := Validate(tax)
	assertOnlyFinding(t, findings, "lz-invalid")
}

func TestValidate_OULinkEmpty(t *testing.T) {
	tax := validBaseTaxonomy()
	tax.OULinks = append(tax.OULinks, &OULink{
		OU:   "/Empty/OU",
		File: "taxonomy/ou_links.yaml",
	})
	findings := Validate(tax)
	assertOnlyFinding(t, findings, "ou-link-empty")
}

func TestValidate_OULinkDuplicate(t *testing.T) {
	tax := validBaseTaxonomy()
	tax.OULinks = append(tax.OULinks, &OULink{
		OU:   "/Workloads/Checkout",
		App:  "APP-CHK",
		File: "taxonomy/ou_links.yaml",
	})
	findings := Validate(tax)
	assertOnlyFinding(t, findings, "ou-link-duplicate")
}

func TestValidate_AccountInvalid(t *testing.T) {
	tax := validBaseTaxonomy()
	tax.Accounts[0].Account = "12345" // not 12 digits
	findings := Validate(tax)
	assertOnlyFinding(t, findings, "account-invalid")
}

func TestValidate_AccountAppDuplicate(t *testing.T) {
	tax := validBaseTaxonomy()
	tax.Accounts[0].Apps = []*AccountApp{
		{App: "APP-CHK"},
		{App: "APP-CHK"},
	}
	findings := Validate(tax)
	assertOnlyFinding(t, findings, "account-app-duplicate")
}

func TestValidate_AccountEnvNotInApp(t *testing.T) {
	tax := validBaseTaxonomy()
	tax.Apps = append(tax.Apps, &App{
		Code:       "APP-PAY",
		Name:       "Payment",
		Project:    "PRJ-SHOP",
		CostCenter: "CC-ECOM",
		Environments: []*Environment{
			{Code: "ENV-DEV", Name: "Development"},
		},
		File: "taxonomy/apps.yaml",
	})
	// Reference ENV-DEV under APP-PAY so it's not reported as unused-node
	tax.OULinks = append(tax.OULinks, &OULink{
		OU:          "/Workloads/Pay",
		App:         "APP-PAY",
		Environment: "ENV-DEV",
		File:        "taxonomy/ou_links.yaml",
	})
	// Account has ENV-PROD, but APP-PAY only has ENV-DEV
	tax.Accounts[0].Apps = append(tax.Accounts[0].Apps, &AccountApp{App: "APP-PAY"})
	findings := Validate(tax)
	assertOnlyFinding(t, findings, "account-env-not-in-app")
}

func TestValidate_TagKeyInvalid(t *testing.T) {
	tax := validBaseTaxonomy()
	tax.CostAllocation.AppTag = "aws:Application"
	findings := Validate(tax)
	assertOnlyFinding(t, findings, "tag-key-invalid")
}

func TestValidate_DefaultEnvNotInApp(t *testing.T) {
	tax := validBaseTaxonomy()
	tax.Apps = append(tax.Apps, &App{
		Code:       "APP-PAY",
		Name:       "Payment",
		Project:    "PRJ-SHOP",
		CostCenter: "CC-ECOM",
		Environments: []*Environment{
			{Code: "ENV-DEV", Name: "Development"},
		},
		File: "taxonomy/apps.yaml",
	})
	// LZ-MAIN default sets APP-PAY and ENV-PROD, but APP-PAY only defines ENV-DEV
	tax.LZs[0].Default = &LZDefault{
		App:         "APP-PAY",
		Environment: "ENV-PROD",
	}
	// Add an account referencing APP-PAY so it's not unused
	tax.Accounts = append(tax.Accounts, &AccountEntry{
		Account:     "333333333333",
		Environment: "ENV-DEV",
		Apps:        []*AccountApp{{App: "APP-PAY"}},
	})
	findings := Validate(tax)
	assertOnlyFinding(t, findings, "default-env-not-in-app")
}

func TestValidate_BizPathMismatch(t *testing.T) {
	tax := validBaseTaxonomy()
	tax.BusinessUnits = append(tax.BusinessUnits, &BusinessUnit{
		Code: "BU-CORP",
		Name: "Corporate",
		File: "taxonomy/bunits.yaml",
	})
	// Change CC-ECOM to BU-CORP, while PRJ-SHOP is under BU-RETAIL
	tax.CostCenters[0].BusinessUnit = "BU-CORP"
	findings := Validate(tax)
	assertOnlyFinding(t, findings, "biz-path-mismatch")
}

func TestValidate_UnusedNode(t *testing.T) {
	tax := validBaseTaxonomy()
	tax.BusinessUnits = append(tax.BusinessUnits, &BusinessUnit{
		Code: "BU-UNUSED",
		Name: "Unused BU",
		File: "taxonomy/bunits.yaml",
	})
	findings := Validate(tax)
	assertOnlyFinding(t, findings, "unused-node")
}

func TestLoader_YamlInvalid(t *testing.T) {
	t.Run("syntax_error", func(t *testing.T) {
		tmpDir := t.TempDir()
		badFile := filepath.Join(tmpDir, "bad.yaml")
		if err := os.WriteFile(badFile, []byte(": : invalid yaml syntax"), 0644); err != nil {
			t.Fatal(err)
		}

		_, findings, err := LoadDir(tmpDir)
		if err != nil {
			t.Fatalf("unexpected load error: %v", err)
		}
		assertOnlyFinding(t, findings, "yaml-invalid")
	})

	t.Run("unknown_top_level_key", func(t *testing.T) {
		tmpDir := t.TempDir()
		file := filepath.Join(tmpDir, "extra.yaml")
		if err := os.WriteFile(file, []byte("unknown_key:\n  - foo: bar\n"), 0644); err != nil {
			t.Fatal(err)
		}

		_, findings, err := LoadDir(tmpDir)
		if err != nil {
			t.Fatalf("unexpected load error: %v", err)
		}
		assertOnlyFinding(t, findings, "yaml-invalid")
	})

	t.Run("nested_unknown_key", func(t *testing.T) {
		tmpDir := t.TempDir()
		file := filepath.Join(tmpDir, "accounts.yaml")
		content := `accounts:
  - account: "222222222222"
    environment: ENV-PROD
    apps:
      - app: APP-CHK
        weight: 70
`
		if err := os.WriteFile(file, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		_, findings, err := LoadDir(tmpDir)
		if err != nil {
			t.Fatalf("unexpected load error: %v", err)
		}
		assertOnlyFinding(t, findings, "yaml-invalid")
	})

	t.Run("duplicate_top_level_key_in_one_file", func(t *testing.T) {
		tmpDir := t.TempDir()
		file := filepath.Join(tmpDir, "dup.yaml")
		content := `business_units:
  - code: BU-A
    name: A
business_units:
  - code: BU-B
    name: B
`
		if err := os.WriteFile(file, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		_, findings, err := LoadDir(tmpDir)
		if err != nil {
			t.Fatalf("unexpected load error: %v", err)
		}
		assertOnlyFinding(t, findings, "yaml-invalid")
		if !strings.Contains(findings[0].Message, "appears twice in") {
			t.Fatalf("expected message to mention 'appears twice in', got: %s", findings[0].Message)
		}
	})

	t.Run("duplicate_top_level_key_across_two_files", func(t *testing.T) {
		tmpDir := t.TempDir()
		f1 := filepath.Join(tmpDir, "f1.yaml")
		f2 := filepath.Join(tmpDir, "f2.yaml")
		if err := os.WriteFile(f1, []byte("business_units:\n  - code: BU-A\n    name: A\n"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f2, []byte("business_units:\n  - code: BU-B\n    name: B\n"), 0644); err != nil {
			t.Fatal(err)
		}

		_, findings, err := LoadDir(tmpDir)
		if err != nil {
			t.Fatalf("unexpected load error: %v", err)
		}
		assertOnlyFinding(t, findings, "yaml-invalid")
		if !strings.Contains(findings[0].Message, "appears in two files") {
			t.Fatalf("expected message to mention 'appears in two files', got: %s", findings[0].Message)
		}
	})
}

func TestLoader_MultipleFindingsInOneRun(t *testing.T) {
	tax := validBaseTaxonomy()
	tax.LZs[0].CSP = "INVALID_CSP"
	tax.CostCenters[0].BusinessUnit = "" // ref-missing
	findings := Validate(tax)
	if len(findings) < 2 {
		t.Fatalf("expected at least 2 findings, got %d", len(findings))
	}
}

func TestLookups(t *testing.T) {
	tax := validBaseTaxonomy()

	app, ok := tax.AppByCode("APP-CHK")
	if !ok || app == nil || app.Name != "Checkout" {
		t.Fatalf("AppByCode failed")
	}

	appTag, ok := tax.AppByTag("  app-chk  ")
	if !ok || appTag == nil || appTag.Code != "APP-CHK" {
		t.Fatalf("AppByTag case-insensitive lookup failed")
	}

	env, ok := tax.EnvironmentByAppAndCode("APP-CHK", "ENV-PROD")
	if !ok || env == nil || env.Name != "Production" {
		t.Fatalf("EnvironmentByAppAndCode failed")
	}

	lz, ok := tax.LZByManagementAccount("111111111111")
	if !ok || lz == nil || lz.Code != "LZ-MAIN" {
		t.Fatalf("LZByManagementAccount failed")
	}
}

func TestShippedTaxonomyValid(t *testing.T) {
	tax, findings, err := LoadDir("../../taxonomy")
	if err != nil {
		t.Fatalf("failed to load shipped taxonomy: %v", err)
	}
	if len(findings) > 0 {
		t.Fatalf("expected zero findings from loader on shipped taxonomy, got: %+v", findings)
	}
	validationFindings := Validate(tax)
	if len(validationFindings) > 0 {
		t.Fatalf("expected zero validation findings on shipped taxonomy, got: %+v", validationFindings)
	}
}

func TestReferenceCasesValidate(t *testing.T) {
	pattern := filepath.Join("..", "..", "planning", "DD-01-r1-account-mapping", "cases", "*", "*.yaml")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) == 0 {
		t.Fatalf("no case files found matching %s", pattern)
	}

	for _, file := range matches {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		var doc struct {
			Taxonomy *Taxonomy `yaml:"taxonomy"`
		}
		dec := yaml.NewDecoder(bytes.NewReader(data))
		if err := dec.Decode(&doc); err != nil {
			t.Fatalf("failed to decode %s: %v", file, err)
		}
		if doc.Taxonomy != nil {
			findings := Validate(doc.Taxonomy)
			for _, f := range findings {
				if f.Severity == SeverityError {
					t.Errorf("%s: unexpected error finding: %+v", file, f)
				}
			}
		}
	}
}
