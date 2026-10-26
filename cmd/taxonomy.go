package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/PopGreenTh/OpenLZ/internal/taxonomy"
	"github.com/spf13/cobra"
)

var taxonomyDir string

var taxonomyCmd = &cobra.Command{
	Use:   "taxonomy",
	Short: "Manage and validate taxonomy configurations",
}

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate taxonomy YAML files",
	RunE: func(cmd *cobra.Command, args []string) error {
		tax, findings, err := taxonomy.LoadDir(taxonomyDir)
		if err != nil {
			return fmt.Errorf("load taxonomy: %w", err)
		}

		if tax != nil {
			validationFindings := taxonomy.Validate(tax)
			findings = append(findings, validationFindings...)
		}

		if len(findings) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "Taxonomy is valid (0 findings).")
			return nil
		}

		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "SEVERITY\tCODE\tFILE\tPATH\tMESSAGE")
		hasErrors := false
		for _, f := range findings {
			if f.Severity == taxonomy.SeverityError {
				hasErrors = true
			}
			file := f.File
			if file == "" {
				file = "-"
			}
			path := f.Path
			if path == "" {
				path = "-"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", f.Severity, f.Code, file, path, f.Message)
		}
		w.Flush()

		if hasErrors {
			os.Exit(1)
		}
		return nil
	},
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List Project > App > Environment hierarchy",
	RunE: func(cmd *cobra.Command, args []string) error {
		tax, _, err := taxonomy.LoadDir(taxonomyDir)
		if err != nil {
			return fmt.Errorf("load taxonomy: %w", err)
		}

		appsByProject := make(map[string][]*taxonomy.App)
		var standaloneApps []*taxonomy.App
		for _, app := range tax.Apps {
			if app.Project != "" {
				appsByProject[app.Project] = append(appsByProject[app.Project], app)
			} else {
				standaloneApps = append(standaloneApps, app)
			}
		}

		out := cmd.OutOrStdout()
		for _, prj := range tax.Projects {
			fmt.Fprintf(out, "Project: %s (%s) [BusinessUnit: %s]\n", prj.Code, prj.Name, prj.BusinessUnit)
			apps := appsByProject[prj.Code]
			for _, app := range apps {
				fmt.Fprintf(out, "  App: %s (%s) [CostCenter: %s]\n", app.Code, app.Name, app.CostCenter)
				for _, env := range app.Environments {
					fmt.Fprintf(out, "    Environment: %s (%s)\n", env.Code, env.Name)
				}
			}
		}

		if len(standaloneApps) > 0 {
			fmt.Fprintln(out, "Unassigned Apps:")
			for _, app := range standaloneApps {
				fmt.Fprintf(out, "  App: %s (%s) [CostCenter: %s]\n", app.Code, app.Name, app.CostCenter)
				for _, env := range app.Environments {
					fmt.Fprintf(out, "    Environment: %s (%s)\n", env.Code, env.Name)
				}
			}
		}

		return nil
	},
}

func init() {
	taxonomyCmd.PersistentFlags().StringVar(&taxonomyDir, "dir", "taxonomy", "directory containing taxonomy YAML files")
	taxonomyCmd.AddCommand(validateCmd)
	taxonomyCmd.AddCommand(listCmd)
	rootCmd.AddCommand(taxonomyCmd)
}
