package cmd

import (
	"fmt"

	"github.com/PopGreenTh/OpenLZ/internal/cost"
	"github.com/spf13/cobra"
)

var (
	costSenseMock   bool
	costSenseOut    string
	costSenseDemo   string
	costSenseMetric string
)

var costSenseCmd = &cobra.Command{
	Use:   "sense",
	Short: "Collect Cost Explorer data to cost_raw.parquet",
	RunE: func(cmd *cobra.Command, args []string) error {
		var rows []cost.RawRow
		var err error
		var ceCalls int
		if costSenseMock {
			rows, err = cost.LoadMockRaw(costSenseDemo)
			if err != nil {
				return err
			}
		} else {
			return fmt.Errorf("live Cost Explorer sense is not wired in this build; use --mock")
		}
		if err := writeRawParquet(costSenseOut, rows); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Wrote %d raw cost rows to %s (%d Cost Explorer calls, $%.2f)\n",
			len(rows), costSenseOut, ceCalls, float64(ceCalls)*0.01)
		return nil
	},
}

func init() {
	costSenseCmd.Flags().BoolVar(&costSenseMock, "mock", false, "use demo cost fixture")
	costSenseCmd.Flags().StringVar(&costSenseOut, "out", "data/cost_raw.parquet", "output Parquet path")
	costSenseCmd.Flags().StringVar(&costSenseDemo, "demo", "testdata/cost/demo.json", "demo fixture when using --mock")
	costSenseCmd.Flags().StringVar(&costSenseMetric, "metric", "UnblendedCost", "Cost Explorer metric")
}
