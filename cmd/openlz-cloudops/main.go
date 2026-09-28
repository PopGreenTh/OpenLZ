package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/PopGreenTh/OpenLZ/internal/logger"
	"github.com/PopGreenTh/OpenLZ/internal/stages"
	cloudopsstages "github.com/PopGreenTh/OpenLZ/internal/stages/cloudops"
	"github.com/PopGreenTh/OpenLZ/internal/stages/types"
	"github.com/spf13/cobra"
)

var (
	verbose  bool
	logLevel string
)

var rootCmd = &cobra.Command{
	Use:   "openlz-cloudops",
	Short: "OpenLZ CloudOps: Dedicated CLI & Lambda runner for CloudOps hygiene & waste cleanup stages",
	Long: `OpenLZ CloudOps operates exclusively on cloud hygiene and waste optimization:
Sense (Operational Hygiene) -> Analyze (Waste & Stale Assets) -> Enrich (Lifecycle & Ownership) -> Enforce (Cleanup) -> Report`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		logger.Init(logLevel, verbose, stages.IsLambdaEnvironment())
	},
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "Log level (debug, info, warn, error)")

	// 1. sense-hygiene
	var (
		cSenseAccs string
		cSenseSvc  string
		cSenseOut  string
		cSenseMock bool
	)
	cmdSenseHygiene := &cobra.Command{
		Use:     "sense-hygiene",
		Aliases: []string{"cloudops-sense-hygiene", "sense"},
		Short:   "Stage 1: Scan operational hygiene and orphaned cloud resources",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			var accs []string
			if cSenseAccs != "" {
				accs = strings.Split(cSenseAccs, ",")
			}
			res, err := cloudopsstages.ExecuteSenseHygiene(ctx, cloudopsstages.SenseHygieneInput{
				Accounts:   accs,
				Service:    cSenseSvc,
				OutputPath: cSenseOut,
				Mock:       cSenseMock,
			})
			if err != nil {
				return err
			}
			fmt.Printf("[%s] %s\n", res.Stage, res.Message)
			return nil
		},
	}
	cmdSenseHygiene.Flags().StringVar(&cSenseAccs, "accounts", "", "Comma-separated AWS Account IDs (auto-detected via STS if omitted)")
	cmdSenseHygiene.Flags().StringVar(&cSenseSvc, "service", "all", "Service filter (e.g. ec2, ebs, all)")
	cmdSenseHygiene.Flags().StringVar(&cSenseOut, "output", "data/cloudops_raw.parquet", "Output raw parquet path")
	cmdSenseHygiene.Flags().BoolVar(&cSenseMock, "mock", true, "Use mock/offline cloud data")
	rootCmd.AddCommand(cmdSenseHygiene)

	// 2. analyze-wastehygiene
	var (
		cAnaIn  string
		cAnaOut string
	)
	cmdAnalyze := &cobra.Command{
		Use:     "analyze-wastehygiene",
		Aliases: []string{"cloudops-analysis-wastehygiene", "analyze"},
		Short:   "Stage 2: Detect orphaned volumes, stale snapshots, and waste",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			res, err := cloudopsstages.ExecuteAnalysisWasteHygiene(ctx, cloudopsstages.AnalysisWasteHygieneInput{
				InputPath:  cAnaIn,
				OutputPath: cAnaOut,
			})
			if err != nil {
				return err
			}
			fmt.Printf("[%s] %s\n", res.Stage, res.Message)
			return nil
		},
	}
	cmdAnalyze.Flags().StringVar(&cAnaIn, "input", "data/cloudops_raw.parquet", "Input raw parquet path")
	cmdAnalyze.Flags().StringVar(&cAnaOut, "output", "data/cloudops_anomalies.parquet", "Output anomalies parquet path")
	rootCmd.AddCommand(cmdAnalyze)

	// 3. enrich-lifecycle
	var (
		cEnrIn  string
		cEnrOut string
	)
	cmdEnrich := &cobra.Command{
		Use:     "enrich-lifecycle",
		Aliases: []string{"cloudops-enrich-lifecycle", "enrich"},
		Short:   "Stage 3: Annotate orphaned assets with lifecycle & owner metadata",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			res, err := cloudopsstages.ExecuteEnrichLifecycle(ctx, cloudopsstages.EnrichLifecycleInput{
				InputPath:  cEnrIn,
				OutputPath: cEnrOut,
			})
			if err != nil {
				return err
			}
			fmt.Printf("[%s] %s\n", res.Stage, res.Message)
			return nil
		},
	}
	cmdEnrich.Flags().StringVar(&cEnrIn, "input", "data/cloudops_anomalies.parquet", "Input anomalies parquet path")
	cmdEnrich.Flags().StringVar(&cEnrOut, "output", "data/cloudops_enriched.parquet", "Output enriched parquet path")
	rootCmd.AddCommand(cmdEnrich)

	// 4. enforce-cleanup
	var (
		cEnfIn  string
		cEnfOut string
		cEnfDry bool
	)
	cmdEnforce := &cobra.Command{
		Use:     "enforce-cleanup",
		Aliases: []string{"cloudops-enforce-cleanup", "enforce"},
		Short:   "Stage 4: Clean up orphaned resources and log savings",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			res, err := cloudopsstages.ExecuteEnforceCleanup(ctx, cloudopsstages.EnforceCleanupInput{
				InputPath:  cEnfIn,
				OutputPath: cEnfOut,
				DryRun:     cEnfDry,
			})
			if err != nil {
				return err
			}
			fmt.Printf("[%s] %s\n", res.Stage, res.Message)
			return nil
		},
	}
	cmdEnforce.Flags().StringVar(&cEnfIn, "input", "data/cloudops_enriched.parquet", "Input enriched parquet path")
	cmdEnforce.Flags().StringVar(&cEnfOut, "output", "data/cloudops_audit.parquet", "Output audit parquet path")
	cmdEnforce.Flags().BoolVar(&cEnfDry, "dry-run", true, "Dry-run simulation mode")
	rootCmd.AddCommand(cmdEnforce)

	// 5. report-summary
	var (
		cRepIn  string
		cRepFmt string
		cRepOut string
	)
	cmdReport := &cobra.Command{
		Use:     "report-summary",
		Aliases: []string{"cloudops-report-summary", "report"},
		Short:   "Stage 5: Generate CloudOps hygiene report",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			res, err := cloudopsstages.ExecuteReportSummary(ctx, cloudopsstages.ReportSummaryInput{
				InputPath:  cRepIn,
				Format:     cRepFmt,
				OutputPath: cRepOut,
			})
			if err != nil {
				return err
			}
			fmt.Printf("[%s] %s\n", res.Stage, res.Message)
			return nil
		},
	}
	cmdReport.Flags().StringVar(&cRepIn, "input", "data/cloudops_enriched.parquet", "Input enriched parquet path")
	cmdReport.Flags().StringVar(&cRepFmt, "format", "table", "Format (table, markdown)")
	cmdReport.Flags().StringVar(&cRepOut, "output", "", "Output report path")
	rootCmd.AddCommand(cmdReport)

	// Lambda simulation command
	cmdLambda := &cobra.Command{
		Use:   "lambda <stage-name> [json-payload]",
		Short: "Invoke a CloudOps stage using Lambda JSON event calling convention",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			stageName := args[0]
			var payload []byte
			if len(args) > 1 {
				payload = []byte(args[1])
			}
			res, err := stages.ExecuteStage(ctx, stageName, payload)
			if err != nil {
				return err
			}
			formatted, _ := json.MarshalIndent(res, "", "  ")
			fmt.Println(string(formatted))
			return nil
		},
	}
	rootCmd.AddCommand(cmdLambda)
}

func main() {
	if stages.IsLambdaEnvironment() {
		if os.Getenv("OPENLZ_STAGE") == "" {
			os.Setenv("OPENLZ_STAGE", types.StageCloudOpsSenseHygiene)
		}
		stages.StartLambdaRuntime()
		return
	}
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
