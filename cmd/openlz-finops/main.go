package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/PopGreenTh/OpenLZ/internal/logger"
	"github.com/PopGreenTh/OpenLZ/internal/stages"
	finopsstages "github.com/PopGreenTh/OpenLZ/internal/stages/finops"
	"github.com/PopGreenTh/OpenLZ/internal/stages/types"
	"github.com/spf13/cobra"
)

var (
	cacheDBPath string
	verbose     bool
	logLevel    string
)

var rootCmd = &cobra.Command{
	Use:   "openlz-finops",
	Short: "OpenLZ FinOps: Dedicated CLI & Lambda runner for FinOps lifecycle stages",
	Long: `OpenLZ FinOps operates exclusively on cloud financial management:
Sense (Cost Explorer & Cost Anomaly) -> Analyze (Variance & Spikes) -> Enrich (Cost Centers) -> Enforce (Tagging) -> Report`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		logger.Init(logLevel, verbose, stages.IsLambdaEnvironment())
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cacheDBPath, "cache-db", "openlz_cache.duckdb", "Path to persistent DuckDB cache file")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "Log level (debug, info, warn, error)")

	// 1a. sense-costexplorer
	var (
		fSenseAccs          string
		fSensePayer         string
		fSenseGroupBy       string
		fSenseGroupByTagKey string
		fSensePrimaryTag    string
		fSenseSecondaryTag  string
		fSenseTags          string
		fSenseMetric        string
		fSenseFreq          string
		fSenseExclDisc      bool
		fSenseExclCred      bool
		fSenseMinCost       float64
		fSenseFormat        string
		fSenseStart         string
		fSenseEnd           string
		fSenseSvc           string
		fSenseOut           string
		fSenseNoCache       bool
		fSenseMock          bool
	)
	cmdSenseCE := &cobra.Command{
		Use:     "sense-costexplorer",
		Aliases: []string{"finops-sense-costexplorer", "sense"},
		Short:   "Stage 1a: Concurrently query AWS Cost Explorer into DuckDB raw parquet",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			var accs []string
			if fSenseAccs != "" {
				accs = strings.Split(fSenseAccs, ",")
			}
			var tags []string
			if fSenseTags != "" {
				for _, t := range strings.Split(fSenseTags, ",") {
					if tr := strings.TrimSpace(t); tr != "" {
						tags = append(tags, tr)
					}
				}
			}
			res, err := finopsstages.ExecuteSenseCostExplorer(ctx, finopsstages.SenseCostExplorerInput{
				Accounts:         accs,
				PayerAccount:     fSensePayer,
				StartDate:        fSenseStart,
				EndDate:          fSenseEnd,
				Service:          fSenseSvc,
				Metric:           fSenseMetric,
				Frequency:        fSenseFreq,
				GroupBy:          fSenseGroupBy,
				GroupByTagKey:    fSenseGroupByTagKey,
				PrimaryTag:       fSensePrimaryTag,
				SecondaryTag:     fSenseSecondaryTag,
				GroupByTags:      tags,
				ExcludeDiscounts: fSenseExclDisc,
				ExcludeCredits:   fSenseExclCred,
				MinCostThreshold: fSenseMinCost,
				Format:           fSenseFormat,
				OutputPath:       fSenseOut,
				CacheDBPath:      cacheDBPath,
				NoCache:          fSenseNoCache,
				Mock:             fSenseMock,
			})
			if err != nil {
				return err
			}
			fmt.Printf("[%s] %s\n", res.Stage, res.Message)
			return nil
		},
	}
	cmdSenseCE.Flags().StringVar(&fSenseAccs, "accounts", "", "Comma-separated AWS Account IDs (auto-detected via STS if omitted)")
	cmdSenseCE.Flags().StringVar(&fSensePayer, "payer-account", "", "AWS Payer/Management Account ID for consolidated billing")
	cmdSenseCE.Flags().StringVar(&fSenseGroupBy, "group-by", "", "Cost Explorer GroupBy mode (LINKED_ACCOUNT_SERVICE, SERVICE, LINKED_ACCOUNT, SERVICE_TAG, LINKED_ACCOUNT_TAG, TAG, HIERARCHICAL)")
	cmdSenseCE.Flags().StringVar(&fSenseGroupByTagKey, "group-by-tag-key", "", "Tag key used when --group-by includes TAG (e.g. Project, BusinessUnit)")
	cmdSenseCE.Flags().StringVar(&fSensePrimaryTag, "primary-tag", "Project", "Primary tag for hierarchical sense (Round 1 tag, e.g. Project)")
	cmdSenseCE.Flags().StringVar(&fSenseSecondaryTag, "secondary-tag", "Application", "Secondary tag for hierarchical sense (Round 2 tag, e.g. Application)")
	cmdSenseCE.Flags().StringVar(&fSenseTags, "group-by-tags", "BusinessUnit,Project,Application,Environment", "Comma-separated tag keys to group costs by")
	cmdSenseCE.Flags().StringVar(&fSenseMetric, "metric", "UnblendedCost", "Cost Explorer metric (UnblendedCost, AmortizedCost, NetAmortizedCost, UsageQuantity)")
	cmdSenseCE.Flags().StringVar(&fSenseFreq, "frequency", "daily", "Cost Explorer frequency/granularity (hourly, daily, monthly)")
	cmdSenseCE.Flags().BoolVar(&fSenseExclDisc, "exclude-discounts", false, "Exclude discount record types (EDP, SPP)")
	cmdSenseCE.Flags().BoolVar(&fSenseExclCred, "exclude-credits", false, "Exclude credit record types")
	cmdSenseCE.Flags().Float64Var(&fSenseMinCost, "min-cost", 0.0, "Minimum dollar cost threshold to retain record")
	cmdSenseCE.Flags().StringVar(&fSenseFormat, "format", "", "Output format(s): parquet, csv, excel, powerbi, all (comma-separated, e.g. parquet,csv)")
	cmdSenseCE.Flags().StringVar(&fSenseStart, "start-date", "", "Start date (YYYY-MM-DD)")
	cmdSenseCE.Flags().StringVar(&fSenseEnd, "end-date", "", "End date (YYYY-MM-DD)")
	cmdSenseCE.Flags().StringVar(&fSenseSvc, "service", "all", "Service filter (e.g. ec2, s3, all)")
	cmdSenseCE.Flags().StringVar(&fSenseOut, "output", "data/finops_raw.parquet", "Output raw file path")
	cmdSenseCE.Flags().BoolVar(&fSenseNoCache, "no-cache", false, "Bypass DuckDB API cache and query live cloud APIs directly")
	cmdSenseCE.Flags().BoolVar(&fSenseMock, "mock", true, "Use mock/offline cloud data")
	rootCmd.AddCommand(cmdSenseCE)

	// 1b. sense-costanomaly
	var (
		fAnomAccs  string
		fAnomStart string
		fAnomEnd   string
		fAnomOut   string
		fAnomMock  bool
	)
	cmdSenseAnom := &cobra.Command{
		Use:     "sense-costanomaly",
		Aliases: []string{"finops-sense-costanomaly"},
		Short:   "Stage 1b: Query AWS Cost Anomaly Detection into anomalies parquet",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			var accs []string
			if fAnomAccs != "" {
				accs = strings.Split(fAnomAccs, ",")
			}
			res, err := finopsstages.ExecuteSenseCostAnomaly(ctx, finopsstages.SenseCostAnomalyInput{
				Accounts:   accs,
				StartDate:  fAnomStart,
				EndDate:    fAnomEnd,
				OutputPath: fAnomOut,
				Mock:       fAnomMock,
			})
			if err != nil {
				return err
			}
			fmt.Printf("[%s] %s\n", res.Stage, res.Message)
			return nil
		},
	}
	cmdSenseAnom.Flags().StringVar(&fAnomAccs, "accounts", "", "Comma-separated AWS Account IDs (auto-detected via STS if omitted)")
	cmdSenseAnom.Flags().StringVar(&fAnomStart, "start-date", "", "Start date (YYYY-MM-DD)")
	cmdSenseAnom.Flags().StringVar(&fAnomEnd, "end-date", "", "End date (YYYY-MM-DD)")
	cmdSenseAnom.Flags().StringVar(&fAnomOut, "output", "data/finops_anomalies.parquet", "Output anomalies parquet path")
	cmdSenseAnom.Flags().BoolVar(&fAnomMock, "mock", true, "Use mock/offline cloud data")
	rootCmd.AddCommand(cmdSenseAnom)

	// 2. analyze-varianceimpact
	var (
		fAnaIn     string
		fAnaOut    string
		fAnaThresh float64
	)
	cmdAnalyze := &cobra.Command{
		Use:     "analyze-varianceimpact",
		Aliases: []string{"finops-analyze-varianceimpact", "analyze"},
		Short:   "Stage 2: DuckDB OLAP cost variance & spike detection",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			res, err := finopsstages.ExecuteAnalyzeVarianceImpact(ctx, finopsstages.AnalyzeVarianceImpactInput{
				InputPath:  fAnaIn,
				OutputPath: fAnaOut,
				Threshold:  fAnaThresh,
			})
			if err != nil {
				return err
			}
			fmt.Printf("[%s] %s\n", res.Stage, res.Message)
			return nil
		},
	}
	cmdAnalyze.Flags().StringVar(&fAnaIn, "input", "data/finops_raw.parquet", "Input raw parquet path")
	cmdAnalyze.Flags().StringVar(&fAnaOut, "output", "data/finops_anomalies.parquet", "Output anomalies parquet path")
	cmdAnalyze.Flags().Float64Var(&fAnaThresh, "threshold", 1.3, "Cost spike threshold multiplier (e.g. 1.3)")
	rootCmd.AddCommand(cmdAnalyze)

	// 3. enrich-costcenters
	var (
		fEnrIn  string
		fEnrOut string
	)
	cmdEnrich := &cobra.Command{
		Use:     "enrich-costcenters",
		Aliases: []string{"finops-enrich-costcenters", "enrich"},
		Short:   "Stage 3: Annotate anomalies with Landing Zone cost centers & ownership",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			res, err := finopsstages.ExecuteEnrichCostCenters(ctx, finopsstages.EnrichCostCentersInput{
				InputPath:  fEnrIn,
				OutputPath: fEnrOut,
			})
			if err != nil {
				return err
			}
			fmt.Printf("[%s] %s\n", res.Stage, res.Message)
			return nil
		},
	}
	cmdEnrich.Flags().StringVar(&fEnrIn, "input", "data/finops_anomalies.parquet", "Input anomalies parquet path")
	cmdEnrich.Flags().StringVar(&fEnrOut, "output", "data/finops_enriched.parquet", "Output enriched parquet path")
	rootCmd.AddCommand(cmdEnrich)

	// 4. enforce-tagging
	var (
		fEnfIn  string
		fEnfOut string
		fEnfDry bool
	)
	cmdEnforce := &cobra.Command{
		Use:     "enforce-tagging",
		Aliases: []string{"finops-enforce-tagging", "enforce"},
		Short:   "Stage 4: Apply CostCenter tagging & guardrail directives",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			res, err := finopsstages.ExecuteEnforceTagging(ctx, finopsstages.EnforceTaggingInput{
				InputPath:  fEnfIn,
				OutputPath: fEnfOut,
				DryRun:     fEnfDry,
			})
			if err != nil {
				return err
			}
			fmt.Printf("[%s] %s\n", res.Stage, res.Message)
			return nil
		},
	}
	cmdEnforce.Flags().StringVar(&fEnfIn, "input", "data/finops_enriched.parquet", "Input enriched parquet path")
	cmdEnforce.Flags().StringVar(&fEnfOut, "output", "data/finops_audit.parquet", "Output audit parquet path")
	cmdEnforce.Flags().BoolVar(&fEnfDry, "dry-run", true, "Dry-run simulation mode")
	rootCmd.AddCommand(cmdEnforce)

	// 5. report-summary
	var (
		fRepIn  string
		fRepFmt string
		fRepOut string
	)
	cmdReport := &cobra.Command{
		Use:     "report-summary",
		Aliases: []string{"finops-report-summary", "report"},
		Short:   "Stage 5: Generate FinOps reports (table, markdown, powerbi, excel)",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			res, err := finopsstages.ExecuteReportSummary(ctx, finopsstages.ReportSummaryInput{
				InputPath:  fRepIn,
				Format:     fRepFmt,
				OutputPath: fRepOut,
			})
			if err != nil {
				return err
			}
			fmt.Printf("[%s] %s\n", res.Stage, res.Message)
			return nil
		},
	}
	cmdReport.Flags().StringVar(&fRepIn, "input", "data/finops_enriched.parquet", "Input enriched parquet path")
	cmdReport.Flags().StringVar(&fRepFmt, "format", "table", "Format (table, markdown, powerbi, excel)")
	cmdReport.Flags().StringVar(&fRepOut, "output", "", "Output report path")
	rootCmd.AddCommand(cmdReport)

	// Lambda simulation command
	cmdLambda := &cobra.Command{
		Use:   "lambda <stage-name> [json-payload]",
		Short: "Invoke a FinOps stage using Lambda JSON event calling convention",
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
		logger.Init("", false, true)
		// Default to FinOps sense if not specified
		if os.Getenv("OPENLZ_STAGE") == "" {
			os.Setenv("OPENLZ_STAGE", types.StageFinOpsSenseCostExplorer)
		}
		stages.StartLambdaRuntime()
		return
	}
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
