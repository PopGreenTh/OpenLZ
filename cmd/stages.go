package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/PopGreenTh/OpenLZ/internal/stages"
	cloudopsstages "github.com/PopGreenTh/OpenLZ/internal/stages/cloudops"
	finopsstages "github.com/PopGreenTh/OpenLZ/internal/stages/finops"
	secopsstages "github.com/PopGreenTh/OpenLZ/internal/stages/secops"
	"github.com/spf13/cobra"
)

// Lambda runner command for CLI-based local Lambda simulation
var lambdaCmd = &cobra.Command{
	Use:   "lambda <stage-name> [json-payload]",
	Short: "Execute any dedicated stage using Lambda JSON event calling convention",
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

// Register dedicated commands
func init() {
	RootCmd.AddCommand(lambdaCmd)

	// ==========================================
	// FinOps Dedicated Stages
	// ==========================================
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
		fSenseMock          bool
		fSenseNoCache       bool
		fSenseCacheDB       string
		fAnomAccs           string
		fAnomStart          string
		fAnomEnd            string
		fAnomOut            string
		fAnomMock           bool
		fAnaIn              string
		fAnaOut             string
		fAnaThresh          float64
		fEnrIn              string
		fEnrOut             string
		fEnfIn              string
		fEnfOut             string
		fEnfDry             bool
		fRepIn              string
		fRepFmt             string
		fRepOut             string
	)

	// 1a. finops-sense-costexplorer
	cmdFinOpsSenseCE := &cobra.Command{
		Use:   stages.StageFinOpsSenseCostExplorer,
		Short: "Dedicated Stage 1a: Concurrently query AWS Cost Explorer into DuckDB raw parquet",
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
				CacheDBPath:      fSenseCacheDB,
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
	cmdFinOpsSenseCE.Flags().StringVar(&fSenseAccs, "accounts", "", "Comma-separated list of AWS Account IDs (auto-detected via STS if omitted)")
	cmdFinOpsSenseCE.Flags().StringVar(&fSensePayer, "payer-account", "", "AWS Payer/Management Account ID for consolidated billing")
	cmdFinOpsSenseCE.Flags().StringVar(&fSenseGroupBy, "group-by", "", "Cost Explorer GroupBy mode (LINKED_ACCOUNT_SERVICE, SERVICE, LINKED_ACCOUNT, SERVICE_TAG, LINKED_ACCOUNT_TAG, TAG, HIERARCHICAL)")
	cmdFinOpsSenseCE.Flags().StringVar(&fSenseGroupByTagKey, "group-by-tag-key", "", "Tag key used when --group-by includes TAG (e.g. Project, BusinessUnit)")
	cmdFinOpsSenseCE.Flags().StringVar(&fSensePrimaryTag, "primary-tag", "Project", "Primary tag for hierarchical sense (Round 1 tag, e.g. Project)")
	cmdFinOpsSenseCE.Flags().StringVar(&fSenseSecondaryTag, "secondary-tag", "Application", "Secondary tag for hierarchical sense (Round 2 tag, e.g. Application)")
	cmdFinOpsSenseCE.Flags().StringVar(&fSenseTags, "group-by-tags", "BusinessUnit,Project,Application,Environment", "Comma-separated tag keys to group costs by")
	cmdFinOpsSenseCE.Flags().StringVar(&fSenseMetric, "metric", "UnblendedCost", "Cost Explorer metric (UnblendedCost, UsageQuantity, AmortizedCost, NetAmortizedCost)")
	cmdFinOpsSenseCE.Flags().StringVar(&fSenseFreq, "frequency", "daily", "Cost Explorer frequency/granularity (hourly, daily, monthly)")
	cmdFinOpsSenseCE.Flags().BoolVar(&fSenseExclDisc, "exclude-discounts", false, "Exclude discount record types (EDP, SPP)")
	cmdFinOpsSenseCE.Flags().BoolVar(&fSenseExclCred, "exclude-credits", false, "Exclude credit record types")
	cmdFinOpsSenseCE.Flags().Float64Var(&fSenseMinCost, "min-cost", 0.0, "Minimum dollar cost threshold to retain record")
	cmdFinOpsSenseCE.Flags().StringVar(&fSenseFormat, "format", "", "Output format(s): parquet, csv, excel, powerbi, all (comma-separated, e.g. parquet,csv)")
	cmdFinOpsSenseCE.Flags().StringVar(&fSenseStart, "start-date", "", "Start date (YYYY-MM-DD)")
	cmdFinOpsSenseCE.Flags().StringVar(&fSenseEnd, "end-date", "", "End date (YYYY-MM-DD)")
	cmdFinOpsSenseCE.Flags().StringVar(&fSenseSvc, "service", "all", "Service filter (e.g. ec2, s3, all)")
	cmdFinOpsSenseCE.Flags().StringVar(&fSenseOut, "output", "data/finops_raw.parquet", "Output raw file path")
	cmdFinOpsSenseCE.Flags().StringVar(&fSenseCacheDB, "cache-db", "openlz_cache.duckdb", "DuckDB cache file path")
	cmdFinOpsSenseCE.Flags().BoolVar(&fSenseNoCache, "no-cache", false, "Bypass DuckDB API cache and query live cloud APIs directly")
	cmdFinOpsSenseCE.Flags().BoolVar(&fSenseMock, "mock", true, "Use mock/offline cloud data")
	RootCmd.AddCommand(cmdFinOpsSenseCE)

	// 1b. finops-sense-costanomaly
	cmdFinOpsSenseAnom := &cobra.Command{
		Use:   stages.StageFinOpsSenseCostAnomaly,
		Short: "Dedicated Stage 1b: Query AWS Cost Anomaly Detection into parquet",
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
	cmdFinOpsSenseAnom.Flags().StringVar(&fAnomAccs, "accounts", "", "Comma-separated list of AWS Account IDs (auto-detected via STS if omitted)")
	cmdFinOpsSenseAnom.Flags().StringVar(&fAnomStart, "start-date", "", "Start date (YYYY-MM-DD)")
	cmdFinOpsSenseAnom.Flags().StringVar(&fAnomEnd, "end-date", "", "End date (YYYY-MM-DD)")
	cmdFinOpsSenseAnom.Flags().StringVar(&fAnomOut, "output", "data/finops_anomalies.parquet", "Output anomalies parquet path")
	cmdFinOpsSenseAnom.Flags().BoolVar(&fAnomMock, "mock", true, "Use mock/offline cloud data")
	RootCmd.AddCommand(cmdFinOpsSenseAnom)

	// 2. finops-analyze-varianceimpact
	cmdFinOpsAna := &cobra.Command{
		Use:   stages.StageFinOpsAnalyzeVarianceImpact,
		Short: "Dedicated Stage 2: DuckDB OLAP cost variance & spike detection",
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
	cmdFinOpsAna.Flags().StringVar(&fAnaIn, "input", "data/finops_raw.parquet", "Input raw parquet path")
	cmdFinOpsAna.Flags().StringVar(&fAnaOut, "output", "data/finops_anomalies.parquet", "Output anomalies parquet path")
	cmdFinOpsAna.Flags().Float64Var(&fAnaThresh, "threshold", 1.3, "Cost spike threshold multiplier (e.g. 1.3)")
	RootCmd.AddCommand(cmdFinOpsAna)

	// 3. finops-enrich-costcenters
	cmdFinOpsEnr := &cobra.Command{
		Use:   stages.StageFinOpsEnrichCostCenters,
		Short: "Dedicated Stage 3: Annotate cost anomalies with Landing Zone cost centers & owners",
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
	cmdFinOpsEnr.Flags().StringVar(&fEnrIn, "input", "data/finops_anomalies.parquet", "Input anomalies parquet path")
	cmdFinOpsEnr.Flags().StringVar(&fEnrOut, "output", "data/finops_enriched.parquet", "Output enriched parquet path")
	RootCmd.AddCommand(cmdFinOpsEnr)

	// 4. finops-enforce-tagging
	cmdFinOpsEnf := &cobra.Command{
		Use:   stages.StageFinOpsEnforceTagging,
		Short: "Dedicated Stage 4: Apply CostCenter tagging & guardrail directives",
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
	cmdFinOpsEnf.Flags().StringVar(&fEnfIn, "input", "data/finops_enriched.parquet", "Input enriched parquet path")
	cmdFinOpsEnf.Flags().StringVar(&fEnfOut, "output", "data/finops_audit.parquet", "Output audit parquet path")
	cmdFinOpsEnf.Flags().BoolVar(&fEnfDry, "dry-run", true, "Perform dry-run simulation without modifying cloud resources")
	RootCmd.AddCommand(cmdFinOpsEnf)

	// 5. finops-report-summary
	cmdFinOpsRep := &cobra.Command{
		Use:   stages.StageFinOpsReportSummary,
		Short: "Dedicated Stage 5: Generate FinOps reports (table, markdown, powerbi, excel)",
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
	cmdFinOpsRep.Flags().StringVar(&fRepIn, "input", "data/finops_enriched.parquet", "Input enriched parquet path")
	cmdFinOpsRep.Flags().StringVar(&fRepFmt, "format", "table", "Format (table, markdown, powerbi, excel)")
	cmdFinOpsRep.Flags().StringVar(&fRepOut, "output", "", "Output report path")
	RootCmd.AddCommand(cmdFinOpsRep)

	// ==========================================
	// SecOps Dedicated Stages
	// ==========================================
	var (
		sSenseAccs string
		sSenseSvc  string
		sSenseOut  string
		sSenseMock bool
		sVulnAccs  string
		sVulnOut   string
		sVulnMock  bool
		sAnaIn     string
		sAnaOut    string
		sEnrIn     string
		sEnrOut    string
		sEnfIn     string
		sEnfOut    string
		sEnfDry    bool
		sRepIn     string
		sRepFmt    string
		sRepOut    string
	)

	// 1a. secops-sense-posture
	cmdSecOpsSense := &cobra.Command{
		Use:   stages.StageSecOpsSensePosture,
		Short: "Dedicated Stage 1a: Scan S3, IAM, and Security Groups posture",
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
	cmdSecOpsSense.Flags().StringVar(&sSenseAccs, "accounts", "", "Comma-separated list of AWS Account IDs (auto-detected via STS if omitted)")
	cmdSecOpsSense.Flags().StringVar(&sSenseSvc, "service", "all", "Service filter (e.g. s3, iam, ec2, all)")
	cmdSecOpsSense.Flags().StringVar(&sSenseOut, "output", "data/secops_raw.parquet", "Output raw parquet path")
	cmdSecOpsSense.Flags().BoolVar(&sSenseMock, "mock", true, "Use mock/offline cloud data")
	RootCmd.AddCommand(cmdSecOpsSense)

	// 1b. secops-sense-vulner
	cmdSecOpsVuln := &cobra.Command{
		Use:   stages.StageSecOpsSenseVulner,
		Short: "Dedicated Stage 1b: Scan vulnerabilities, CVEs, and container packages",
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
	cmdSecOpsVuln.Flags().StringVar(&sVulnAccs, "accounts", "", "Comma-separated list of AWS Account IDs (auto-detected via STS if omitted)")
	cmdSecOpsVuln.Flags().StringVar(&sVulnOut, "output", "data/secops_vulner_raw.parquet", "Output vulnerabilities parquet path")
	cmdSecOpsVuln.Flags().BoolVar(&sVulnMock, "mock", true, "Use mock/offline cloud data")
	RootCmd.AddCommand(cmdSecOpsVuln)

	// 2. secops-analysis-riskposture
	cmdSecOpsAna := &cobra.Command{
		Use:   stages.StageSecOpsAnalysisRiskPosture,
		Short: "Dedicated Stage 2: Evaluate security risks and extract critical findings",
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
	cmdSecOpsAna.Flags().StringVar(&sAnaIn, "input", "data/secops_raw.parquet", "Input raw parquet path")
	cmdSecOpsAna.Flags().StringVar(&sAnaOut, "output", "data/secops_anomalies.parquet", "Output anomalies parquet path")
	RootCmd.AddCommand(cmdSecOpsAna)

	// 3. secops-enrich-metadata
	cmdSecOpsEnr := &cobra.Command{
		Use:   stages.StageSecOpsEnrichMetadata,
		Short: "Dedicated Stage 3: Annotate security findings with blast radius & ownership",
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
	cmdSecOpsEnr.Flags().StringVar(&sEnrIn, "input", "data/secops_anomalies.parquet", "Input anomalies parquet path")
	cmdSecOpsEnr.Flags().StringVar(&sEnrOut, "output", "data/secops_enriched.parquet", "Output enriched parquet path")
	RootCmd.AddCommand(cmdSecOpsEnr)

	// 4. secops-enforce-remediation
	cmdSecOpsEnf := &cobra.Command{
		Use:   stages.StageSecOpsEnforceRemediation,
		Short: "Dedicated Stage 4: Remediate open access and enforce security guardrails",
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
	cmdSecOpsEnf.Flags().StringVar(&sEnfIn, "input", "data/secops_enriched.parquet", "Input enriched parquet path")
	cmdSecOpsEnf.Flags().StringVar(&sEnfOut, "output", "data/secops_audit.parquet", "Output audit parquet path")
	cmdSecOpsEnf.Flags().BoolVar(&sEnfDry, "dry-run", true, "Perform dry-run simulation without altering cloud resources")
	RootCmd.AddCommand(cmdSecOpsEnf)

	// 5. secops-report-summary
	cmdSecOpsRep := &cobra.Command{
		Use:   stages.StageSecOpsReportSummary,
		Short: "Dedicated Stage 5: Generate SecOps summary report",
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
	cmdSecOpsRep.Flags().StringVar(&sRepIn, "input", "data/secops_enriched.parquet", "Input enriched parquet path")
	cmdSecOpsRep.Flags().StringVar(&sRepFmt, "format", "table", "Format (table, markdown)")
	cmdSecOpsRep.Flags().StringVar(&sRepOut, "output", "", "Output report path")
	RootCmd.AddCommand(cmdSecOpsRep)

	// ==========================================
	// CloudOps Dedicated Stages
	// ==========================================
	var (
		cSenseAccs string
		cSenseSvc  string
		cSenseOut  string
		cSenseMock bool
		cAnaIn     string
		cAnaOut    string
		cEnrIn     string
		cEnrOut    string
		cEnfIn     string
		cEnfOut    string
		cEnfDry    bool
		cRepIn     string
		cRepFmt    string
		cRepOut    string
	)

	// 1. cloudops-sense-hygiene
	cmdCloudOpsSense := &cobra.Command{
		Use:   stages.StageCloudOpsSenseHygiene,
		Short: "Dedicated Stage 1: Scan operational hygiene and orphaned cloud resources",
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
	cmdCloudOpsSense.Flags().StringVar(&cSenseAccs, "accounts", "", "Comma-separated list of AWS Account IDs (auto-detected via STS if omitted)")
	cmdCloudOpsSense.Flags().StringVar(&cSenseSvc, "service", "all", "Service filter (e.g. ec2, ebs, all)")
	cmdCloudOpsSense.Flags().StringVar(&cSenseOut, "output", "data/cloudops_raw.parquet", "Output raw parquet path")
	cmdCloudOpsSense.Flags().BoolVar(&cSenseMock, "mock", true, "Use mock/offline cloud data")
	RootCmd.AddCommand(cmdCloudOpsSense)

	// 2. cloudops-analysis-wastehygiene
	cmdCloudOpsAna := &cobra.Command{
		Use:   stages.StageCloudOpsAnalysisWasteHygiene,
		Short: "Dedicated Stage 2: Detect orphaned volumes, stale snapshots, and waste",
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
	cmdCloudOpsAna.Flags().StringVar(&cAnaIn, "input", "data/cloudops_raw.parquet", "Input raw parquet path")
	cmdCloudOpsAna.Flags().StringVar(&cAnaOut, "output", "data/cloudops_anomalies.parquet", "Output anomalies parquet path")
	RootCmd.AddCommand(cmdCloudOpsAna)

	// 3. cloudops-enrich-lifecycle
	cmdCloudOpsEnr := &cobra.Command{
		Use:   stages.StageCloudOpsEnrichLifecycle,
		Short: "Dedicated Stage 3: Annotate orphaned assets with lifecycle & owner metadata",
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
	cmdCloudOpsEnr.Flags().StringVar(&cEnrIn, "input", "data/cloudops_anomalies.parquet", "Input anomalies parquet path")
	cmdCloudOpsEnr.Flags().StringVar(&cEnrOut, "output", "data/cloudops_enriched.parquet", "Output enriched parquet path")
	RootCmd.AddCommand(cmdCloudOpsEnr)

	// 4. cloudops-enforce-cleanup
	cmdCloudOpsEnf := &cobra.Command{
		Use:   stages.StageCloudOpsEnforceCleanup,
		Short: "Dedicated Stage 4: Clean up orphaned resources and log savings",
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
	cmdCloudOpsEnf.Flags().StringVar(&cEnfIn, "input", "data/cloudops_enriched.parquet", "Input enriched parquet path")
	cmdCloudOpsEnf.Flags().StringVar(&cEnfOut, "output", "data/cloudops_audit.parquet", "Output audit parquet path")
	cmdCloudOpsEnf.Flags().BoolVar(&cEnfDry, "dry-run", true, "Perform dry-run simulation without deleting assets")
	RootCmd.AddCommand(cmdCloudOpsEnf)

	// 5. cloudops-report-summary
	cmdCloudOpsRep := &cobra.Command{
		Use:   stages.StageCloudOpsReportSummary,
		Short: "Dedicated Stage 5: Generate CloudOps hygiene report",
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
	cmdCloudOpsRep.Flags().StringVar(&cRepIn, "input", "data/cloudops_enriched.parquet", "Input enriched parquet path")
	cmdCloudOpsRep.Flags().StringVar(&cRepFmt, "format", "table", "Format (table, markdown)")
	cmdCloudOpsRep.Flags().StringVar(&cRepOut, "output", "", "Output report path")
	RootCmd.AddCommand(cmdCloudOpsRep)
}
