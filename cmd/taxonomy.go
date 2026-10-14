package cmd

import (
	"fmt"
	"os"
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
	Short: "Manage and validate Landing Zone cost hierarchy taxonomy snapshots",
	Long: `Inspect, list, validate, and scaffold multi-tenant cost taxonomy snapshots.

One JSON snapshot per tenant: taxonomy/taxonomy.json (default tenant) and
taxonomy/taxonomy_<tenant>.json. Snapshots mirror the Directus collections.`,
}

func taxonomyTenant() string {
	if taxTenant != "" {
		return taxTenant
	}
	return rootTenant
}

var taxonomyListCmd = &cobra.Command{
	Use:   "list",
	Short: "List the default and per-tenant taxonomy snapshots",
	RunE: func(cmd *cobra.Command, args []string) error {
		tenants, err := taxonomy.DiscoverTenants(taxTenantDir)
		if err != nil {
			return err
		}
		fmt.Println("==================================================")
		fmt.Println(" OpenLZ Cost Hierarchy Taxonomies")
		fmt.Println("==================================================")
		def := taxonomy.PathForTenant(taxTenantDir, "")
		fmt.Printf("Default: %s (exists: %t)\n\n", def, fileExists(def))
		if len(tenants) == 0 {
			fmt.Println("Tenants: (none detected in " + taxTenantDir + ")")
		} else {
			fmt.Printf("Tenants (%d):\n", len(tenants))
			for _, t := range tenants {
				fmt.Printf("  • %-15s -> %s\n", t, taxonomy.PathForTenant(taxTenantDir, t))
			}
		}
		fmt.Println("==================================================")
		return nil
	},
}

var taxonomyValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate a taxonomy snapshot against the hierarchy relations",
	RunE: func(cmd *cobra.Command, args []string) error {
		path := taxonomy.PathForTenant(taxTenantDir, taxonomyTenant())
		fmt.Printf("Validating %s...\n", path)
		if !fileExists(path) {
			return fmt.Errorf("taxonomy snapshot not found: %s", path)
		}
		cat, err := taxonomy.Load(path)
		if err != nil {
			fmt.Printf("❌ Validation FAILED:\n%v\n", err)
			return fmt.Errorf("taxonomy validation failed")
		}
		for _, spec := range taxonomy.Schema {
			fmt.Printf("  %-16s %d\n", spec.Name, cat.Count(spec.Name))
		}
		for _, w := range cat.Warnings() {
			fmt.Printf("⚠️  %s\n", w)
		}
		fmt.Printf("✅ Validation PASSED (%d warnings).\n", len(cat.Warnings()))
		return nil
	},
}

var taxonomyInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a starter taxonomy snapshot for a tenant",
	RunE: func(cmd *cobra.Command, args []string) error {
		tenant := strings.TrimSpace(taxonomyTenant())
		if tenant == "" {
			return fmt.Errorf("--tenant is required to initialize a tenant taxonomy")
		}
		path := taxonomy.PathForTenant(taxTenantDir, tenant)
		if fileExists(path) {
			return fmt.Errorf("taxonomy already exists for tenant %q: %s", tenant, path)
		}

		u := strings.ToUpper(tenant)
		snap := taxonomy.NewSnapshot(tenant)
		snap.Add(taxonomy.BusinessUnits, taxonomy.Row{"id": "BU-" + u, "name": tenant + " Division"})
		snap.Add(taxonomy.CostCenters, taxonomy.Row{"id": "CC-" + u + "-CORE", "name": tenant + " Core", "business_unit": "BU-" + u})
		snap.Add(taxonomy.Owners, taxonomy.Row{"id": "OWN-" + u + "-CORE", "name": tenant + " Core Team", "cost_center": "CC-" + u + "-CORE"})
		snap.Add(taxonomy.CustomerFuncs, taxonomy.Row{"id": "CF-" + u + "-PRIMARY", "name": "Primary Service", "owner": "OWN-" + u + "-CORE"})
		snap.Add(taxonomy.Projects, taxonomy.Row{"id": "PRJ-" + u, "name": tenant + " Project", "business_unit": "BU-" + u})
		snap.Add(taxonomy.Apps, taxonomy.Row{"id": "APP-" + u + "-CORE", "name": tenant + " Core App", "project": "PRJ-" + u, "cost_center": "CC-" + u + "-CORE"})
		snap.Add(taxonomy.Environments, taxonomy.Row{"id": "ENV-" + u + "-PROD", "name": "Production", "app": "APP-" + u + "-CORE"})
		snap.Add(taxonomy.Accounts, taxonomy.Row{"id": "123456789012", "name": "lz-" + tenant + "-prod", "environment": "ENV-" + u + "-PROD"})
		snap.Add(taxonomy.CSPs, taxonomy.Row{"id": "AWS", "name": "Amazon Web Services"})
		snap.Add(taxonomy.Regions, taxonomy.Row{"id": "ap-southeast-1", "name": "Asia Pacific (Singapore)", "csp": "AWS"})
		snap.Add(taxonomy.ServiceCats, taxonomy.Row{"id": "SC-COMPUTE", "name": "Compute"})
		snap.Add(taxonomy.Services, taxonomy.Row{"id": "AmazonEC2", "name": "Amazon Elastic Compute Cloud - Compute", "service_cat": "SC-COMPUTE"})

		if _, err := taxonomy.Build(snap); err != nil {
			return fmt.Errorf("starter taxonomy is invalid: %w", err)
		}
		if err := taxonomy.WriteSnapshot(path, snap); err != nil {
			return err
		}
		fmt.Printf("✅ Initialized starter taxonomy for tenant %q: %s\n", tenant, path)
		return nil
	},
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

func init() {
	taxonomyCmd.PersistentFlags().StringVar(&taxTenantDir, "dir", taxonomy.DefaultDir, "Directory containing taxonomy JSON snapshots")
	taxonomyCmd.PersistentFlags().StringVar(&taxTenant, "tenant", "", "Tenant identifier for multi-tenant taxonomy")

	taxonomyCmd.AddCommand(taxonomyListCmd)
	taxonomyCmd.AddCommand(taxonomyValidateCmd)
	taxonomyCmd.AddCommand(taxonomyInitCmd)
}
