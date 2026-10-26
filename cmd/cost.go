package cmd

import "github.com/spf13/cobra"

var costCmd = &cobra.Command{
	Use:   "cost",
	Short: "Read AWS cost and allocate through the account map",
}

func init() {
	costCmd.AddCommand(costSenseCmd)
	costCmd.AddCommand(costAllocateCmd)
	rootCmd.AddCommand(costCmd)
}
