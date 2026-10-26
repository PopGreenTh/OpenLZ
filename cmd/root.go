package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "openlz",
	Short: "OpenLZ — Landing zone governance and FinOps toolkit",
	Long:  "OpenLZ is a standalone single-tenant landing zone governance and FinOps toolkit.",
}

// Execute runs the root command and exits on error.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
