package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/PopGreenTh/OpenLZ/internal/duckdb"
	"github.com/PopGreenTh/OpenLZ/internal/report"
	"github.com/PopGreenTh/OpenLZ/internal/rest"
	finopsstages "github.com/PopGreenTh/OpenLZ/internal/stages/finops"
	types "github.com/PopGreenTh/OpenLZ/internal/stages/types"
	"github.com/spf13/cobra"
)

var (
	finopsAccounts         string
	finopsPayerAccount     string
	finopsGroupBy          string
	finopsGroupByTagKey    string
	finopsPrimaryTag       string
	finopsSecondaryTag     string
	finopsGroupByTags      string
	finopsMetric           string
	finopsFrequency        string
	finopsExcludeDiscounts bool
	finopsExcludeCredits   bool
	finopsMinCost          float64
	finopsStartDate        string
	finopsEndDate          string
	finopsService          string
	finopsRawPath          string
	finopsAnomPath         string
	finopsEnrichPath       string
	finopsProjectTaxonomy  string
	finopsBusinessTaxonomy string
	finopsAuditPath        string
	finopsThreshold        float64
	finopsDryRun           bool
	finopsMock             bool
	finopsFormat           string
	finopsReportOut        string
	finopsAppendTS         bool
	finopsAppendOutput     bool
	finopsUpsert           bool
	finopsTenant           string
)

var finopsCmd = &cobra.Command{
	Use:   "finops",
	Short: "FinOps domain operations (Spend anomalies, Rightsizing, Tiering)",
	Long:  "Execute FinOps lifecycle stages: sense, analyze, enrich, enforce, and report.",
}

var finopsSenseCmd = &cobra.Command{
	Use:   "sense",
	Short: "Stage 1: Scan AWS Cost Explorer concurrently into DuckDB raw.parquet",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		var accounts []string
		if finopsAccounts != "" {
			accounts = strings.Split(finopsAccounts, ",")
		}
		if !cmd.Flags().Changed("group-by-tags") {
			if env := os.Getenv("OPENLZ_GROUP_BY_TAGS"); env != "" {
				finopsGroupByTags = env
			}
		}
		if !cmd.Flags().Changed("primary-tag") {
			if env := os.Getenv("OPENLZ_PRIMARY_TAG"); env != "" {
				finopsPrimaryTag = env
			}
		}
		if !cmd.Flags().Changed("secondary-tag") {
			if env := os.Getenv("OPENLZ_SECONDARY_TAG"); env != "" {
				finopsSecondaryTag = env
			}
		}
		var tags []string
		if finopsGroupByTags != "" {
			for _, t := range strings.Split(finopsGroupByTags, ",") {
				if tr := strings.TrimSpace(t); tr != "" {
					tags = append(tags, tr)
				}
			}
		}
		tenant := finopsTenant
		if tenant == "" {
			tenant = rootTenant
		}
		rawPath := finopsRawPath
		if !cmd.Flags().Changed("output") {
			rawPath = types.AppendTenantToPath(rawPath, tenant)
		}

		res, err := finopsstages.ExecuteSenseCostExplorer(ctx, finopsstages.SenseCostExplorerInput{
			Accounts:         accounts,
			PayerAccount:     finopsPayerAccount,
			StartDate:        finopsStartDate,
			EndDate:          finopsEndDate,
			Service:          finopsService,
			Metric:           finopsMetric,
			Frequency:        finopsFrequency,
			GroupBy:          finopsGroupBy,
			GroupByTagKey:    finopsGroupByTagKey,
			PrimaryTag:       finopsPrimaryTag,
			SecondaryTag:     finopsSecondaryTag,
			GroupByTags:      tags,
			ExcludeDiscounts: finopsExcludeDiscounts,
			ExcludeCredits:   finopsExcludeCredits,
			MinCostThreshold: finopsMinCost,
			OutputPath:       rawPath,
			AppendTimestamp:  finopsAppendTS,
			AppendOutput:     finopsAppendOutput || finopsUpsert,
			Upsert:           finopsAppendOutput || finopsUpsert,
			CacheDBPath:      cacheDBPath,
			Mock:             finopsMock,
			Tenant:           tenant,
		})
		if err != nil {
			return err
		}
		fmt.Printf("[FinOps:Sense] %s\n", res.Message)
		return nil
	},
}

var finopsAnalyzeCmd = &cobra.Command{
	Use:   "analyze",
	Short: "Stage 2: Run DuckDB OLAP queries to detect cost spikes & anomalies",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		tenant := finopsTenant
		if tenant == "" {
			tenant = rootTenant
		}
		rawPath := finopsRawPath
		if !cmd.Flags().Changed("input") {
			rawPath = types.AppendTenantToPath(rawPath, tenant)
		}
		anomPath := finopsAnomPath
		if !cmd.Flags().Changed("output") {
			anomPath = types.AppendTenantToPath(anomPath, tenant)
		}

		res, err := finopsstages.ExecuteAnalyzeVarianceImpact(ctx, finopsstages.AnalyzeVarianceImpactInput{
			InputPath:  rawPath,
			OutputPath: anomPath,
			Threshold:  finopsThreshold,
			Tenant:     tenant,
		})
		if err != nil {
			return err
		}
		fmt.Printf("[FinOps:Analyze] %s\n", res.Message)
		return nil
	},
}

var finopsEnrichCmd = &cobra.Command{
	Use:   "enrich",
	Short: "Stage 3: Annotate anomalies with Landing Zone ownership & cost centers",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		tenant := finopsTenant
		if tenant == "" {
			tenant = rootTenant
		}
		anomPath := finopsAnomPath
		if !cmd.Flags().Changed("input") {
			anomPath = types.AppendTenantToPath(anomPath, tenant)
		}
		enrichPath := finopsEnrichPath
		if !cmd.Flags().Changed("output") {
			enrichPath = types.AppendTenantToPath(enrichPath, tenant)
		}

		res, err := finopsstages.ExecuteEnrichCostCenters(ctx, finopsstages.EnrichCostCentersInput{
			InputPath:            anomPath,
			OutputPath:           enrichPath,
			ProjectTaxonomyPath:  finopsProjectTaxonomy,
			BusinessTaxonomyPath: finopsBusinessTaxonomy,
			Tenant:               tenant,
		})
		if err != nil {
			return err
		}
		fmt.Printf("[FinOps:Enrich] %s\n", res.Message)
		return nil
	},
}

var finopsEnforceCmd = &cobra.Command{
	Use:   "enforce",
	Short: "Stage 4: Execute FinOps policies and guardrail tagging",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		tenant := finopsTenant
		if tenant == "" {
			tenant = rootTenant
		}
		enrichPath := finopsEnrichPath
		if !cmd.Flags().Changed("input") {
			enrichPath = types.AppendTenantToPath(enrichPath, tenant)
		}
		auditPath := finopsAuditPath
		if !cmd.Flags().Changed("output") {
			auditPath = types.AppendTenantToPath(auditPath, tenant)
		}

		res, err := finopsstages.ExecuteEnforceTagging(ctx, finopsstages.EnforceTaggingInput{
			InputPath:  enrichPath,
			OutputPath: auditPath,
			DryRun:     finopsDryRun,
			Tenant:     tenant,
		})
		if err != nil {
			return err
		}
		fmt.Printf("[FinOps:Enforce] %s\n", res.Message)
		return nil
	},
}

var finopsReportCmd = &cobra.Command{
	Use:   "report",
	Short: "Stage 5: Generate FinOps reports (table, markdown, powerbi, excel, serve)",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		tenant := finopsTenant
		if tenant == "" {
			tenant = rootTenant
		}
		enrichPath := finopsEnrichPath
		if !cmd.Flags().Changed("input") {
			enrichPath = types.AppendTenantToPath(enrichPath, tenant)
		}

		engine, err := duckdb.NewEngine()
		if err != nil {
			return err
		}
		defer engine.Close()

		records, err := engine.ReadEnrichedFromParquet(ctx, enrichPath)
		if err != nil {
			return fmt.Errorf("could not read enriched dataset from %s: %w", enrichPath, err)
		}

		switch strings.ToLower(finopsFormat) {
		case "markdown", "md":
			targetMD := finopsReportOut
			if targetMD == "" {
				targetMD = "finops_report.md"
			}
			if finopsAppendTS {
				targetMD = types.AppendTimestampToPath(targetMD)
			}
			md := report.RenderMarkdown(records, "FinOps")
			if finopsReportOut != "" || finopsAppendTS {
				_ = os.WriteFile(targetMD, []byte(md), 0644)
				fmt.Printf("[FinOps:Report] Markdown report written to %s\n", targetMD)
			} else {
				fmt.Println(md)
			}
		case "powerbi":
			pOut := "finops_report.parquet"
			if finopsReportOut != "" {
				pOut = finopsReportOut
			}
			if finopsAppendTS && !finopsUpsert {
				pOut = types.AppendTimestampToPath(pOut)
			}
			mOut := strings.TrimSuffix(pOut, ".parquet") + ".m"
			if finopsAppendTS && finopsUpsert {
				mOut = types.AppendTimestampToPath(mOut)
			}
			if finopsUpsert {
				if err := engine.UpsertEnrichedParquet(ctx, records, pOut); err != nil {
					return err
				}
			}
			return report.ExportPowerBI(ctx, records, pOut, mOut, engine)
		case "excel", "csv":
			cOut := "finops_report.csv"
			if finopsReportOut != "" {
				cOut = finopsReportOut
			}
			if finopsAppendTS {
				cOut = types.AppendTimestampToPath(cOut)
			}
			return report.ExportExcel(ctx, records, cOut, "", engine)
		case "serve", "http", "rest":
			srv := rest.NewServer(8080, engine, finopsEnrichPath, "", "")
			return srv.Start(ctx)
		default:
			report.RenderConsoleTable(records, "FinOps")
		}
		return nil
	},
}

func init() {
	finopsCmd.PersistentFlags().StringVar(&finopsTenant, "tenant", "", "Tenant ID or name for multi-tenant isolation")

	// Flags for sense
	finopsSenseCmd.Flags().StringVar(&finopsAccounts, "accounts", "", "Comma-separated list of AWS Account IDs (auto-detected via STS if omitted)")
	finopsSenseCmd.Flags().StringVar(&finopsPayerAccount, "payer-account", "", "AWS Payer/Management Account ID for consolidated billing")
	finopsSenseCmd.Flags().StringVar(&finopsGroupBy, "group-by", "", "Cost Explorer GroupBy mode (LINKED_ACCOUNT_SERVICE, SERVICE, LINKED_ACCOUNT, SERVICE_TAG, LINKED_ACCOUNT_TAG, TAG, HIERARCHICAL)")
	finopsSenseCmd.Flags().StringVar(&finopsGroupByTagKey, "group-by-tag-key", "", "Tag key used when --group-by includes TAG (e.g. Project, BusinessUnit)")
	finopsSenseCmd.Flags().StringVar(&finopsPrimaryTag, "primary-tag", "Project", "Primary tag for hierarchical sense (Round 1 tag, e.g. Project)")
	finopsSenseCmd.Flags().StringVar(&finopsSecondaryTag, "secondary-tag", "Application", "Secondary tag for hierarchical sense (Round 2 tag, e.g. Application)")
	finopsSenseCmd.Flags().StringVar(&finopsGroupByTags, "group-by-tags", "BusinessUnit,Project,Application,Environment", "Comma-separated tag keys to group costs by")
	finopsSenseCmd.Flags().StringVar(&finopsMetric, "metric", "UnblendedCost", "Cost Explorer metric (UnblendedCost, UsageQuantity, AmortizedCost, NetAmortizedCost)")
	finopsSenseCmd.Flags().StringVar(&finopsFrequency, "frequency", "daily", "Cost Explorer frequency/granularity (hourly, daily, monthly)")
	finopsSenseCmd.Flags().BoolVar(&finopsExcludeDiscounts, "exclude-discounts", false, "Exclude discount record types (EDP, SPP)")
	finopsSenseCmd.Flags().BoolVar(&finopsExcludeCredits, "exclude-credits", false, "Exclude credit record types")
	finopsSenseCmd.Flags().Float64Var(&finopsMinCost, "min-cost", 0.0, "Minimum dollar cost threshold to retain record")
	finopsSenseCmd.Flags().StringVar(&finopsStartDate, "start-date", "", "Start date (YYYY-MM-DD)")
	finopsSenseCmd.Flags().StringVar(&finopsEndDate, "end-date", "", "End date (YYYY-MM-DD)")
	finopsSenseCmd.Flags().StringVar(&finopsService, "service", "all", "Service filter (e.g. ec2, s3, all)")
	finopsSenseCmd.Flags().StringVar(&finopsRawPath, "output", "data/finops_raw.parquet", "Output raw parquet file path")
	finopsSenseCmd.Flags().BoolVarP(&finopsAppendTS, "append-timestamp", "t", false, "Append formatted datetime (_YYYYMMDD_HHMMSS) to output file names")
	finopsSenseCmd.Flags().BoolVarP(&finopsAppendOutput, "append-output", "a", false, "Append/merge records into original un-timestamped output file (--output) in addition to timestamped file")
	finopsSenseCmd.Flags().BoolVarP(&finopsUpsert, "upsert", "u", false, "Alias for --append-output: upsert/merge records into original output file")
	finopsSenseCmd.Flags().BoolVar(&finopsMock, "mock", true, "Use mock/offline cloud data")

	// Flags for analyze
	finopsAnalyzeCmd.Flags().StringVar(&finopsRawPath, "input", "data/finops_raw.parquet", "Input raw parquet file path")
	finopsAnalyzeCmd.Flags().StringVar(&finopsAnomPath, "output", "data/finops_anomalies.parquet", "Output anomalies parquet file path")
	finopsAnalyzeCmd.Flags().Float64Var(&finopsThreshold, "threshold", 1.3, "Cost spike threshold multiplier (e.g. 1.3 for 30% increase)")

	// Flags for enrich
	finopsEnrichCmd.Flags().StringVar(&finopsAnomPath, "input", "data/finops_anomalies.parquet", "Input anomalies parquet file path")
	finopsEnrichCmd.Flags().StringVar(&finopsEnrichPath, "output", "data/finops_enriched.parquet", "Output enriched parquet file path")
	finopsEnrichCmd.Flags().StringVar(&finopsProjectTaxonomy, "project-taxonomy", "", "Project cost hierarchy JSON (default taxonomy/project_taxonomy.json when present)")
	finopsEnrichCmd.Flags().StringVar(&finopsBusinessTaxonomy, "business-taxonomy", "", "Business cost hierarchy JSON (default taxonomy/business_taxonomy.json when present)")

	// Flags for enforce
	finopsEnforceCmd.Flags().StringVar(&finopsEnrichPath, "input", "data/finops_enriched.parquet", "Input enriched parquet file path")
	finopsEnforceCmd.Flags().StringVar(&finopsAuditPath, "output", "data/finops_audit.parquet", "Output enforce audit parquet file path")
	finopsEnforceCmd.Flags().BoolVar(&finopsDryRun, "dry-run", true, "Perform dry-run simulation without altering cloud resources")

	// Flags for report
	finopsReportCmd.Flags().StringVar(&finopsEnrichPath, "input", "data/finops_enriched.parquet", "Input enriched parquet file path")
	finopsReportCmd.Flags().StringVar(&finopsFormat, "format", "table", "Report format (table, markdown, powerbi, excel, serve)")
	finopsReportCmd.Flags().StringVar(&finopsReportOut, "output", "", "Output file path (for markdown, powerbi parquet, or excel csv)")
	finopsReportCmd.Flags().BoolVarP(&finopsAppendTS, "append-timestamp", "t", false, "Append formatted datetime (_YYYYMMDD_HHMMSS) to output file names")
	finopsReportCmd.Flags().BoolVarP(&finopsUpsert, "upsert", "u", false, "Upsert new records into existing Parquet file instead of overwriting")

	finopsCmd.AddCommand(finopsSenseCmd)
	finopsCmd.AddCommand(finopsAnalyzeCmd)
	finopsCmd.AddCommand(finopsEnrichCmd)
	finopsCmd.AddCommand(finopsEnforceCmd)
	finopsCmd.AddCommand(finopsReportCmd)
}
