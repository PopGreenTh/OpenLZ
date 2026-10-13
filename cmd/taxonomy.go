package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/PopGreenTh/OpenLZ/internal/taxonomy"
	"github.com/spf13/cobra"
)

var (
	taxTenantDir string
	taxTenant    string
)

var taxonomyCmd = &cobra.Command{
	Use:   "taxonomy",
	Short: "Manage and validate Landing Zone cost hierarchy taxonomy JSON files",
	Long: `Inspect, list, validate, and scaffold multi-tenant project and business taxonomy JSON files.
Supports standalone files (taxonomy/project_taxonomy.json) and multi-tenant files (e.g. taxonomy/project_taxonomy_<tenant>.json).`,
}

var taxonomyListCmd = &cobra.Command{
	Use:   "list",
	Short: "List discovered multi-tenant taxonomies and default taxonomy files",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := taxTenantDir
		if dir == "" {
			dir = "taxonomy"
		}

		tenants, err := taxonomy.DiscoverTenants(dir)
		if err != nil {
			return err
		}

		fmt.Println("==================================================")
		fmt.Println(" OpenLZ Cost Hierarchy Taxonomies")
		fmt.Println("==================================================")

		// Check default taxonomy
		defProj := filepath.Join(dir, "project_taxonomy.json")
		defBiz := filepath.Join(dir, "business_taxonomy.json")
		hasDefProj := fileExists(defProj)
		hasDefBiz := fileExists(defBiz)

		fmt.Printf("Default Taxonomy:\n")
		fmt.Printf("  Project:  %s (exists: %t)\n", defProj, hasDefProj)
		fmt.Printf("  Business: %s (exists: %t)\n\n", defBiz, hasDefBiz)

		if len(tenants) == 0 {
			fmt.Println("Discovered Tenants: (none detected in " + dir + ")")
		} else {
			fmt.Printf("Discovered Tenants (%d):\n", len(tenants))
			for _, t := range tenants {
				projP := taxonomy.ResolveProjectTaxonomyPath("", t)
				bizP := taxonomy.ResolveBusinessTaxonomyPath("", t)
				fmt.Printf("  • Tenant: %-15s -> Project: %-40s Business: %s\n", t, projP, bizP)
			}
		}
		fmt.Println("==================================================")
		return nil
	},
}

var taxonomyValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate taxonomy files against hierarchy rules and shared identity invariants",
	RunE: func(cmd *cobra.Command, args []string) error {
		tenant := taxTenant
		if tenant == "" {
			tenant = rootTenant
		}

		projP := taxonomy.ResolveProjectTaxonomyPath("", tenant)
		bizP := taxonomy.ResolveBusinessTaxonomyPath("", tenant)

		targetDesc := "default taxonomy"
		if tenant != "" {
			targetDesc = fmt.Sprintf("tenant %q", tenant)
		}

		fmt.Printf("Validating %s...\n", targetDesc)
		fmt.Printf("  Project file:  %s\n", projP)
		fmt.Printf("  Business file: %s\n", bizP)

		if !fileExists(projP) {
			return fmt.Errorf("project taxonomy file not found: %s", projP)
		}
		if !fileExists(bizP) {
			return fmt.Errorf("business taxonomy file not found: %s", bizP)
		}

		cat, err := taxonomy.Load(projP, bizP)
		if err != nil {
			fmt.Printf("❌ Validation FAILED: %v\n", err)
			return err
		}

		fmt.Println("✅ Validation PASSED: Taxonomy schema and shared identity invariants are valid.")
		_ = cat
		return nil
	},
}

var taxonomyInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize starter project and business taxonomy JSON files for a tenant",
	RunE: func(cmd *cobra.Command, args []string) error {
		tenant := taxTenant
		if tenant == "" {
			tenant = rootTenant
		}
		if strings.TrimSpace(tenant) == "" {
			return fmt.Errorf("--tenant is required to initialize tenant taxonomy files")
		}

		dir := taxTenantDir
		if dir == "" {
			dir = "taxonomy"
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}

		projPath := filepath.Join(dir, fmt.Sprintf("project_taxonomy_%s.json", tenant))
		bizPath := filepath.Join(dir, fmt.Sprintf("business_taxonomy_%s.json", tenant))

		if fileExists(projPath) || fileExists(bizPath) {
			return fmt.Errorf("taxonomy files already exist for tenant %q (%s or %s)", tenant, projPath, bizPath)
		}

		upperTenant := strings.ToUpper(tenant)
		starterBiz := map[string]interface{}{
			"businessUnits": []map[string]interface{}{
				{
					"id":   fmt.Sprintf("BU-%s", upperTenant),
					"name": fmt.Sprintf("%s Division", strings.Title(tenant)),
					"owners": []map[string]interface{}{
						{
							"id":   fmt.Sprintf("OWN-%s-CORE", upperTenant),
							"name": fmt.Sprintf("%s Core Team", strings.Title(tenant)),
							"costCenters": []map[string]interface{}{
								{
									"id":   fmt.Sprintf("CC-%s-PROD", upperTenant),
									"name": fmt.Sprintf("%s Production", strings.Title(tenant)),
									"customerFuncs": []map[string]interface{}{
										{
											"id":   fmt.Sprintf("CF-%s-PRIMARY", upperTenant),
											"name": "Primary Service",
										},
									},
								},
							},
						},
					},
				},
			},
		}

		starterProj := map[string]interface{}{
			"projects": []map[string]interface{}{
				{
					"id":             fmt.Sprintf("BU-%s", upperTenant),
					"name":           fmt.Sprintf("%s Division", strings.Title(tenant)),
					"businessUnitId": fmt.Sprintf("BU-%s", upperTenant),
					"apps": []map[string]interface{}{
						{
							"id":        fmt.Sprintf("OWN-%s-CORE", upperTenant),
							"name":      fmt.Sprintf("%s Core Team", strings.Title(tenant)),
							"ownerId":   fmt.Sprintf("OWN-%s-CORE", upperTenant),
							"projectId": fmt.Sprintf("BU-%s", upperTenant),
							"environments": []map[string]interface{}{
								{
									"id":           fmt.Sprintf("CC-%s-PROD", upperTenant),
									"name":         fmt.Sprintf("%s Production", strings.Title(tenant)),
									"costCenterId": fmt.Sprintf("CC-%s-PROD", upperTenant),
									"accounts": []map[string]interface{}{
										{
											"id":            "111122223333",
											"name":          fmt.Sprintf("lz-%s-prod", strings.ToLower(tenant)),
											"environmentId": fmt.Sprintf("CC-%s-PROD", upperTenant),
										},
									},
									"serviceCategories": []map[string]interface{}{
										{
											"id":             "SC-COMPUTE",
											"name":           "Compute",
											"customerFuncId": fmt.Sprintf("CF-%s-PRIMARY", upperTenant),
											"services": []map[string]interface{}{
												{
													"id":           "AmazonEC2",
													"name":         "Amazon Elastic Compute Cloud - Compute",
													"serviceCatId": "SC-COMPUTE",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}

		bizData, _ := json.MarshalIndent(starterBiz, "", "  ")
		projData, _ := json.MarshalIndent(starterProj, "", "  ")

		if err := os.WriteFile(bizPath, bizData, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", bizPath, err)
		}
		if err := os.WriteFile(projPath, projData, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", projPath, err)
		}

		fmt.Printf("✅ Initialized starter taxonomy for tenant %q:\n", tenant)
		fmt.Printf("  • %s\n", bizPath)
		fmt.Printf("  • %s\n", projPath)
		return nil
	},
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

func init() {
	taxonomyCmd.PersistentFlags().StringVar(&taxTenantDir, "dir", "taxonomy", "Directory containing taxonomy JSON files")
	taxonomyCmd.PersistentFlags().StringVar(&taxTenant, "tenant", "", "Tenant identifier for multi-tenant taxonomy")

	taxonomyCmd.AddCommand(taxonomyListCmd)
	taxonomyCmd.AddCommand(taxonomyValidateCmd)
	taxonomyCmd.AddCommand(taxonomyInitCmd)
}
