package cmd

import (
	"fmt"

	"github.com/PopGreenTh/OpenLZ/internal/cost"
	"github.com/PopGreenTh/OpenLZ/internal/mapping"
	"github.com/PopGreenTh/OpenLZ/internal/taxonomy"
	"github.com/spf13/cobra"
)

var (
	costAllocateMap string
	costAllocateRaw string
	costAllocateOut string
	costAllocateTax string
)

var costAllocateCmd = &cobra.Command{
	Use:   "allocate",
	Short: "Allocate raw cost through the account map",
	RunE: func(cmd *cobra.Command, args []string) error {
		tax, loadFindings, err := taxonomy.LoadDir(costAllocateTax)
		if err != nil {
			return fmt.Errorf("load taxonomy: %w", err)
		}
		if tax != nil {
			loadFindings = append(loadFindings, taxonomy.Validate(tax)...)
		}
		if taxonomyHasErrors(loadFindings) {
			printTaxonomyFindings(cmd, loadFindings)
			return fmt.Errorf("taxonomy has errors")
		}

		acctMap, err := mapping.ReadAccountMap(costAllocateMap)
		if err != nil {
			return err
		}
		raw, err := readRawParquet(costAllocateRaw)
		if err != nil {
			return err
		}
		result, err := cost.Allocate(tax, acctMap, raw)
		if err != nil {
			return err
		}
		if err := writeAllocatedParquet(costAllocateOut, result.Rows); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Wrote %d allocated rows to %s (%d findings)\n",
			len(result.Rows), costAllocateOut, len(result.Findings))
		return nil
	},
}

func init() {
	costAllocateCmd.Flags().StringVar(&costAllocateMap, "map", "data/account_map.json", "account map JSON path")
	costAllocateCmd.Flags().StringVar(&costAllocateRaw, "raw", "data/cost_raw.parquet", "raw cost Parquet path")
	costAllocateCmd.Flags().StringVar(&costAllocateOut, "out", "data/cost_allocated.parquet", "allocated cost Parquet path")
	costAllocateCmd.Flags().StringVar(&costAllocateTax, "taxonomy", "taxonomy", "taxonomy directory")
}
