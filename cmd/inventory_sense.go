package cmd

import (
	"context"
	"fmt"

	"github.com/PopGreenTh/OpenLZ/internal/awsx"
	"github.com/PopGreenTh/OpenLZ/internal/inventory"
	"github.com/aws/aws-sdk-go-v2/service/organizations"
	"github.com/spf13/cobra"
)

var (
	inventoryMock    bool
	inventoryProfile string
	inventoryRegion  string
	inventoryOut     string
	inventoryDemo    string
)

var inventorySenseCmd = &cobra.Command{
	Use:   "sense",
	Short: "Collect AWS Organization inventory to a JSON file",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}

		var doc *inventory.Document
		var err error
		if inventoryMock {
			doc, err = inventory.MockDocument(inventoryDemo)
			if err != nil {
				return fmt.Errorf("mock inventory: %w", err)
			}
		} else {
			cfg, err := awsx.LoadConfig(ctx, inventoryProfile, inventoryRegion)
			if err != nil {
				return err
			}
			client := organizations.NewFromConfig(cfg)
			doc, err = inventory.SenseAWS(ctx, &inventory.AWSOrganizations{Client: client})
			if err != nil {
				return fmt.Errorf("discover inventory: %w", err)
			}
		}

		if err := inventory.WriteFile(inventoryOut, doc); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Wrote inventory to %s (%s, %d organization(s))\n",
			inventoryOut, doc.Source, len(doc.Organizations))
		return nil
	},
}

func init() {
	inventorySenseCmd.Flags().BoolVar(&inventoryMock, "mock", false, "use the demo landing zone fixture instead of AWS")
	inventorySenseCmd.Flags().StringVar(&inventoryProfile, "profile", "", "AWS shared config profile")
	inventorySenseCmd.Flags().StringVar(&inventoryRegion, "region", "", "AWS region for Organizations API (default us-east-1)")
	inventorySenseCmd.Flags().StringVar(&inventoryOut, "out", "data/inventory.json", "output inventory JSON path")
	inventorySenseCmd.Flags().StringVar(&inventoryDemo, "demo", "testdata/inventory/demo.json", "demo fixture path when using --mock")
}
