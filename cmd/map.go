package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/PopGreenTh/OpenLZ/internal/inventory"
	"github.com/PopGreenTh/OpenLZ/internal/mapping"
	"github.com/PopGreenTh/OpenLZ/internal/taxonomy"
	"github.com/spf13/cobra"
)

var (
	mapTaxonomyDir string
	mapInventoryIn string
	mapOut         string
)

var mapCmd = &cobra.Command{
	Use:   "map",
	Short: "Resolve accounts to Project > App > Environment",
	RunE: func(cmd *cobra.Command, args []string) error {
		tax, loadFindings, err := taxonomy.LoadDir(mapTaxonomyDir)
		if err != nil {
			return fmt.Errorf("load taxonomy: %w", err)
		}
		var validationFindings []taxonomy.Finding
		if tax != nil {
			validationFindings = taxonomy.Validate(tax)
		}
		allFindings := append(loadFindings, validationFindings...)
		if taxonomyHasErrors(allFindings) {
			printTaxonomyFindings(cmd, allFindings)
			os.Exit(1)
		}

		invDoc, err := inventory.ReadFile(mapInventoryIn)
		if err != nil {
			return err
		}
		inv := mapping.Inventory{
			CollectedAt:   invDoc.CollectedAt,
			Organizations: copyOrganizations(invDoc.Organizations),
		}
		for _, pf := range invDoc.PathFindings {
			inv.PathFindings = append(inv.PathFindings, mapping.Finding{
				Severity: mapping.SeverityWarning,
				Code:     pf.Code,
				Subject:  pf.Subject,
				Message:  pf.Message,
			})
		}
		src := mapping.NewStaticSource(inv)
		result := mapping.Resolve(tax, src)
		if result.Conflict != nil {
			printMappingFindings(cmd, []mapping.Finding{*result.Conflict})
			os.Exit(1)
		}

		now := time.Now().UTC().Truncate(time.Second)
		doc, err := mapping.BuildAccountMapDocument(tax, inv, result, now)
		if err != nil {
			return err
		}
		if err := mapping.WriteAccountMap(mapOut, doc); err != nil {
			return err
		}

		out := cmd.OutOrStdout()
		for _, a := range doc.Allocations {
			share := "null"
			if a.Share != nil {
				share = fmt.Sprintf("%g", *a.Share)
			}
			fmt.Fprintf(out, "%s\t%s\t%s\t%s\t%s\t%s\tshare=%s\tapp_rule=%s\tenv_rule=%s\n",
				a.LZ, a.OUPath, a.AccountID, a.Project, a.App, a.Environment, share, a.AppRule, a.EnvironmentRule)
		}
		if len(doc.Findings) > 0 {
			printMappingFindings(cmd, doc.Findings)
		}
		fmt.Fprintf(out, "Wrote account map to %s\n", mapOut)
		return nil
	},
}

func init() {
	mapCmd.Flags().StringVar(&mapTaxonomyDir, "taxonomy", "taxonomy", "directory containing taxonomy YAML files")
	mapCmd.Flags().StringVar(&mapInventoryIn, "inventory", "data/inventory.json", "input inventory JSON path")
	mapCmd.Flags().StringVar(&mapOut, "out", "data/account_map.json", "output account map JSON path")
	rootCmd.AddCommand(mapCmd)
}

func taxonomyHasErrors(findings []taxonomy.Finding) bool {
	for _, f := range findings {
		if f.Severity == taxonomy.SeverityError {
			return true
		}
	}
	return false
}

func printTaxonomyFindings(cmd *cobra.Command, findings []taxonomy.Finding) {
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "SEVERITY\tCODE\tFILE\tPATH\tMESSAGE")
	for _, f := range findings {
		file, path := f.File, f.Path
		if file == "" {
			file = "-"
		}
		if path == "" {
			path = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", f.Severity, f.Code, file, path, f.Message)
	}
	w.Flush()
}

func printMappingFindings(cmd *cobra.Command, findings []mapping.Finding) {
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "SEVERITY\tCODE\tSUBJECT\tMESSAGE")
	for _, f := range findings {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", f.Severity, f.Code, f.Subject, f.Message)
	}
	w.Flush()
}

func copyOrganizations(in []inventory.Organization) []mapping.Organization {
	out := make([]mapping.Organization, len(in))
	for i, org := range in {
		out[i] = mapping.Organization{
			ID:                org.ID,
			ManagementAccount: org.ManagementAccount,
			RootID:            org.RootID,
		}
		for _, ou := range org.OUs {
			out[i].OUs = append(out[i].OUs, mapping.OU{ID: ou.ID, Name: ou.Name, ParentID: ou.ParentID})
		}
		for _, acct := range org.Accounts {
			out[i].Accounts = append(out[i].Accounts, mapping.Account{
				ID: acct.ID, Name: acct.Name, Email: acct.Email, Status: acct.Status, ParentID: acct.ParentID,
			})
		}
	}
	return out
}
