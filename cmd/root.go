package cmd

import (
	"fmt"
	"os"

	"github.com/PopGreenTh/OpenLZ/internal/logger"
	"github.com/PopGreenTh/OpenLZ/internal/stages"
	"github.com/spf13/cobra"
)

var (
	cacheDBPath string
	verbose     bool
	logLevel    string
)

// RootCmd is the base command for OpenLZ.
var RootCmd = &cobra.Command{
	Use:   "openlz",
	Short: "OpenLZ (Open Landing Zone): Modular Zero-ETL FinOps, SecOps & CloudOps CLI",
	Long: `OpenLZ is an open-source (Apache 2.0) Zero-ETL Cloud Operations CLI tool.
It queries native cloud APIs concurrently, caches requests in embedded DuckDB to guard
against repeat API charges ($0.01/call guardrail), processes data locally using embedded DuckDB,
and orchestrates pipelines via go-task or standalone CLI commands across 5 modular stages:
Sense -> Analyze -> Enrich -> Enforce -> Report`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		logger.Init(logLevel, verbose, stages.IsLambdaEnvironment())
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	RootCmd.PersistentFlags().StringVar(&cacheDBPath, "cache-db", "openlz_cache.duckdb", "Path to persistent DuckDB cache file")
	RootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging")
	RootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "Log level (debug, info, warn, error)")

	RootCmd.AddCommand(finopsCmd)
	RootCmd.AddCommand(secopsCmd)
	RootCmd.AddCommand(cloudopsCmd)
	RootCmd.AddCommand(reportCmd)
	RootCmd.AddCommand(pipelineCmd)
}
