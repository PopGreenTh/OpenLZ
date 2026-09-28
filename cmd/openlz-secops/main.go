package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/PopGreenTh/OpenLZ/internal/logger"
	"github.com/PopGreenTh/OpenLZ/internal/stages"
	secopsstages "github.com/PopGreenTh/OpenLZ/internal/stages/secops"
	"github.com/PopGreenTh/OpenLZ/internal/stages/types"
	"github.com/spf13/cobra"
)

var (
	verbose  bool
	logLevel string
)

var rootCmd = &cobra.Command{
	Use:   "openlz-secops",
	Short: "OpenLZ SecOps: Dedicated CLI & Lambda runner for SecOps posture & vulnerability stages",
	Long: `OpenLZ SecOps operates exclusively on cloud security posture and vulnerability management:
Sense (Posture & Vulnerabilities) -> Analyze (Risk Posture) -> Enrich (Blast Radius & Ownership) -> Enforce (Remediation) -> Report`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		logger.Init(logLevel, verbose, stages.IsLambdaEnvironment())
	},
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "Log level (debug, info, warn, error)")

	// 1a. sense-posture
	var (
		sSenseAccs string
		sSenseSvc  string
		sSenseOut  string
		sSenseMock bool
	)
	cmdSensePosture := &cobra.Command{
		Use:     "sense-posture",
		Aliases: []string{"secops-sense-posture", "sense"},
		Short:   "Stage 1a: Scan S3, IAM, and Security Groups posture findings",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			var accs []string
			if sSenseAccs != "" {
				accs = strings.Split(sSenseAccs, ",")
			}
			res, err := secopsstages.ExecuteSensePosture(ctx, secopsstages.SensePostureInput{
				Accounts:   accs,
				Service:    sSenseSvc,
				OutputPath: sSenseOut,
				Mock:       sSenseMock,
			})
			if err != nil {
				return err
			}
			fmt.Printf("[%s] %s\n", res.Stage, res.Message)
			return nil
		},
	}
	cmdSensePosture.Flags().StringVar(&sSenseAccs, "accounts", "", "Comma-separated AWS Account IDs (auto-detected via STS if omitted)")
	cmdSensePosture.Flags().StringVar(&sSenseSvc, "service", "all", "Service filter (e.g. s3, iam, ec2, all)")
	cmdSensePosture.Flags().StringVar(&sSenseOut, "output", "data/secops_raw.parquet", "Output raw parquet path")
	cmdSensePosture.Flags().BoolVar(&sSenseMock, "mock", true, "Use mock/offline cloud data")
	rootCmd.AddCommand(cmdSensePosture)

	// 1b. sense-vulner
	var (
		sVulnAccs string
		sVulnOut  string
		sVulnMock bool
	)
	cmdSenseVuln := &cobra.Command{
		Use:     "sense-vulner",
		Aliases: []string{"secops-sense-vulner"},
		Short:   "Stage 1b: Scan vulnerabilities, CVEs, and container packages",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			var accs []string
			if sVulnAccs != "" {
				accs = strings.Split(sVulnAccs, ",")
			}
			res, err := secopsstages.ExecuteSenseVulner(ctx, secopsstages.SenseVulnerInput{
				Accounts:   accs,
				OutputPath: sVulnOut,
				Mock:       sVulnMock,
			})
			if err != nil {
				return err
			}
			fmt.Printf("[%s] %s\n", res.Stage, res.Message)
			return nil
		},
	}
	cmdSenseVuln.Flags().StringVar(&sVulnAccs, "accounts", "", "Comma-separated AWS Account IDs (auto-detected via STS if omitted)")
	cmdSenseVuln.Flags().StringVar(&sVulnOut, "output", "data/secops_vulner_raw.parquet", "Output vulnerabilities parquet path")
	cmdSenseVuln.Flags().BoolVar(&sVulnMock, "mock", true, "Use mock/offline cloud data")
	rootCmd.AddCommand(cmdSenseVuln)

	// 2. analyze-riskposture
	var (
		sAnaIn  string
		sAnaOut string
	)
	cmdAnalyze := &cobra.Command{
		Use:     "analyze-riskposture",
		Aliases: []string{"secops-analysis-riskposture", "analyze"},
		Short:   "Stage 2: Evaluate security risks and extract critical findings",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			res, err := secopsstages.ExecuteAnalysisRiskPosture(ctx, secopsstages.AnalysisRiskPostureInput{
				InputPath:  sAnaIn,
				OutputPath: sAnaOut,
			})
			if err != nil {
				return err
			}
			fmt.Printf("[%s] %s\n", res.Stage, res.Message)
			return nil
		},
	}
	cmdAnalyze.Flags().StringVar(&sAnaIn, "input", "data/secops_raw.parquet", "Input raw parquet path")
	cmdAnalyze.Flags().StringVar(&sAnaOut, "output", "data/secops_anomalies.parquet", "Output anomalies parquet path")
	rootCmd.AddCommand(cmdAnalyze)

	// 3. enrich-metadata
	var (
		sEnrIn  string
		sEnrOut string
	)
	cmdEnrich := &cobra.Command{
		Use:     "enrich-metadata",
		Aliases: []string{"secops-enrich-metadata", "enrich"},
		Short:   "Stage 3: Annotate security findings with blast radius & ownership",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			res, err := secopsstages.ExecuteEnrichMetadata(ctx, secopsstages.EnrichMetadataInput{
				InputPath:  sEnrIn,
				OutputPath: sEnrOut,
			})
			if err != nil {
				return err
			}
			fmt.Printf("[%s] %s\n", res.Stage, res.Message)
			return nil
		},
	}
	cmdEnrich.Flags().StringVar(&sEnrIn, "input", "data/secops_anomalies.parquet", "Input anomalies parquet path")
	cmdEnrich.Flags().StringVar(&sEnrOut, "output", "data/secops_enriched.parquet", "Output enriched parquet path")
	rootCmd.AddCommand(cmdEnrich)

	// 4. enforce-remediation
	var (
		sEnfIn  string
		sEnfOut string
		sEnfDry bool
	)
	cmdEnforce := &cobra.Command{
		Use:     "enforce-remediation",
		Aliases: []string{"secops-enforce-remediation", "enforce"},
		Short:   "Stage 4: Remediate open access and enforce security guardrails",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			res, err := secopsstages.ExecuteEnforceRemediation(ctx, secopsstages.EnforceRemediationInput{
				InputPath:  sEnfIn,
				OutputPath: sEnfOut,
				DryRun:     sEnfDry,
			})
			if err != nil {
				return err
			}
			fmt.Printf("[%s] %s\n", res.Stage, res.Message)
			return nil
		},
	}
	cmdEnforce.Flags().StringVar(&sEnfIn, "input", "data/secops_enriched.parquet", "Input enriched parquet path")
	cmdEnforce.Flags().StringVar(&sEnfOut, "output", "data/secops_audit.parquet", "Output audit parquet path")
	cmdEnforce.Flags().BoolVar(&sEnfDry, "dry-run", true, "Dry-run simulation mode")
	rootCmd.AddCommand(cmdEnforce)

	// 5. report-summary
	var (
		sRepIn  string
		sRepFmt string
		sRepOut string
	)
	cmdReport := &cobra.Command{
		Use:     "report-summary",
		Aliases: []string{"secops-report-summary", "report"},
		Short:   "Stage 5: Generate SecOps summary report",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			res, err := secopsstages.ExecuteReportSummary(ctx, secopsstages.ReportSummaryInput{
				InputPath:  sRepIn,
				Format:     sRepFmt,
				OutputPath: sRepOut,
			})
			if err != nil {
				return err
			}
			fmt.Printf("[%s] %s\n", res.Stage, res.Message)
			return nil
		},
	}
	cmdReport.Flags().StringVar(&sRepIn, "input", "data/secops_enriched.parquet", "Input enriched parquet path")
	cmdReport.Flags().StringVar(&sRepFmt, "format", "table", "Format (table, markdown)")
	cmdReport.Flags().StringVar(&sRepOut, "output", "", "Output report path")
	rootCmd.AddCommand(cmdReport)

	// Lambda simulation command
	cmdLambda := &cobra.Command{
		Use:   "lambda <stage-name> [json-payload]",
		Short: "Invoke a SecOps stage using Lambda JSON event calling convention",
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
			os.Setenv("OPENLZ_STAGE", types.StageSecOpsSensePosture)
		}
		stages.StartLambdaRuntime()
		return
	}
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
