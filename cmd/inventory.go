package cmd

import (
	"github.com/spf13/cobra"
)

var inventoryCmd = &cobra.Command{
	Use:   "inventory",
	Short: "Discover and inspect AWS Organization inventory",
}

func init() {
	inventoryCmd.AddCommand(inventorySenseCmd)
	inventoryCmd.AddCommand(inventoryShowCmd)
	rootCmd.AddCommand(inventoryCmd)
}
