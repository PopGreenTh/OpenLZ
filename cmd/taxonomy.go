package cmd

import (
	"fmt"
	"os"

	"github.com/PopGreenTh/OpenLZ/internal/taxonomy"
	"github.com/spf13/cobra"
)

var taxFile string

var taxonomyCmd = &cobra.Command{
	Use:   "taxonomy",
	Short: "Validate and scaffold the Landing Zone cost hierarchy taxonomy",
	Long: `Inspect, validate and scaffold the cost taxonomy (docs/taxonomy-spec.md).

OpenLZ has one taxonomy: taxonomy/taxonomy.json. It mirrors the Directus collections
and holds the Biz, Project and Cloud hierarchies.`,
}

var taxonomyValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate a taxonomy snapshot against the hierarchy relations",
	RunE: func(cmd *cobra.Command, args []string) error {
		path := taxonomy.ResolvePath(taxFile)
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
			fmt.Printf("  %-20s %d\n", spec.Name, cat.Count(spec.Name))
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
	Short: "Write a starter taxonomy snapshot when none exists",
	RunE: func(cmd *cobra.Command, args []string) error {
		path := taxonomy.ResolvePath(taxFile)
		if fileExists(path) {
			return fmt.Errorf("taxonomy already exists: %s", path)
		}
		snap := taxonomy.NewSnapshot()
		snap.Add(taxonomy.BusinessUnits, taxonomy.Row{"code": "BU-MAIN", "name": "Main Division"})
		snap.Add(taxonomy.CostCenters, taxonomy.Row{"code": "CC-CORE", "name": "Core", "business_unit": "BU-MAIN"})
		snap.Add(taxonomy.OwnerTeams, taxonomy.Row{"code": "TEAM-CORE", "name": "Core Team", "cost_center": "CC-CORE"})
		snap.Add(taxonomy.ProductLines, taxonomy.Row{"code": "PL-CORE", "name": "Core Product", "owner_team": "TEAM-CORE"})
		snap.Add(taxonomy.Customers, taxonomy.Row{"code": "CUST-INTERNAL", "name": "Internal", "product_line": "PL-CORE"})
		snap.Add(taxonomy.Projects, taxonomy.Row{"code": "PRJ-MAIN", "name": "Main Project", "business_unit": "BU-MAIN"})
		snap.Add(taxonomy.Apps, taxonomy.Row{"code": "APP-CORE", "name": "Core App", "project": "PRJ-MAIN", "cost_center": "CC-CORE"})
		snap.Add(taxonomy.Environments, taxonomy.Row{"code": "ENV-PROD", "name": "Production", "app": "APP-CORE", "owner_team": "TEAM-CORE"})
		snap.Add(taxonomy.LandingZones, taxonomy.Row{"code": "LZ-MAIN", "name": "Main Landing Zone", "csp": "AWS", "currency": "USD"})
		snap.Add(taxonomy.Connections, taxonomy.Row{"code": "123456789012", "name": "Main payer account", "landing_zone": "LZ-MAIN"})
		snap.Add(taxonomy.OUs, taxonomy.Row{"code": "OU-WORKLOADS", "name": "Workloads", "landing_zone": "LZ-MAIN"})
		snap.Add(taxonomy.Accounts, taxonomy.Row{"code": "123456789012", "name": "lz-prod", "connection": "123456789012", "ou": "OU-WORKLOADS"})
		snap.Add(taxonomy.Regions, taxonomy.Row{"code": "ap-southeast-1", "name": "Asia Pacific (Singapore)", "csp": "AWS"})
		snap.Add(taxonomy.ServiceCats, taxonomy.Row{"code": "SC-COMPUTE", "name": "Compute"})
		snap.Add(taxonomy.Services, taxonomy.Row{"code": "AmazonEC2", "name": "Amazon Elastic Compute Cloud - Compute", "service_cat": "SC-COMPUTE"})
		snap.Add(taxonomy.ProjectLZs, taxonomy.Row{"project": "PRJ-MAIN", "landing_zone": "LZ-MAIN"})
		snap.Add(taxonomy.AppOUs, taxonomy.Row{"ou": "OU-WORKLOADS", "app": "APP-CORE"})

		if _, err := taxonomy.Build(snap); err != nil {
			return fmt.Errorf("starter taxonomy is invalid: %w", err)
		}
		if err := taxonomy.WriteSnapshot(path, snap); err != nil {
			return err
		}
		fmt.Printf("✅ Initialized starter taxonomy: %s\n", path)
		return nil
	},
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

func init() {
	taxonomyCmd.PersistentFlags().StringVar(&taxFile, "file", "", "Taxonomy snapshot JSON (default "+taxonomy.DefaultPath+")")

	taxonomyCmd.AddCommand(taxonomyValidateCmd)
	taxonomyCmd.AddCommand(taxonomyInitCmd)
}
