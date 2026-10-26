package cmd

import (
	"fmt"

	"github.com/PopGreenTh/OpenLZ/internal/inventory"
	"github.com/spf13/cobra"
)

var inventoryIn string

var inventoryShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Print the inventory OU tree and accounts",
	RunE: func(cmd *cobra.Command, args []string) error {
		doc, err := inventory.ReadFile(inventoryIn)
		if err != nil {
			return err
		}
		out := cmd.OutOrStdout()
		for i, org := range doc.Organizations {
			if i > 0 {
				fmt.Fprintln(out)
			}
			fmt.Fprintf(out, "Organization %s (management account %s)\n", org.ID, org.ManagementAccount)
			fmt.Fprintln(out, inventory.FormatShowTree(org))
		}
		return nil
	},
}

func init() {
	inventoryShowCmd.Flags().StringVar(&inventoryIn, "in", "data/inventory.json", "input inventory JSON path")
}
