package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/PopGreenTh/OpenLZ/internal/duckdb"
	"github.com/PopGreenTh/OpenLZ/internal/report"
	"github.com/PopGreenTh/OpenLZ/internal/rest"
	cloudopsstages "github.com/PopGreenTh/OpenLZ/internal/stages/cloudops"
	types "github.com/PopGreenTh/OpenLZ/internal/stages/types"
	"github.com/spf13/cobra"
)

var (
	cloudopsAccounts   string
	cloudopsService    string
	cloudopsRawPath    string
	cloudopsAnomPath   string
	cloudopsEnrichPath string
	cloudopsAuditPath  string
	cloudopsDryRun     bool
	cloudopsMock       bool
	cloudopsFormat     string
	cloudopsReportOut  string
	cloudopsAppendTS     bool
	cloudopsAppendOutput bool
	cloudopsUpsert       bool
	cloudopsTenant       string
)

var cloudopsCmd = &cobra.Command{
	Use:   "cloudops",
	Short: "CloudOps domain operations (Orphaned disks, stale snapshots, idle IPs)",
	Long:  "Execute CloudOps lifecycle stages: sense, analyze, enrich, enforce, and report.",
}

var cloudopsSenseCmd = &cobra.Command{
	Use:   "sense",
	Short: "Stage 1: Scan AWS operational hygiene concurrently into DuckDB raw.parquet",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		tenant := cloudopsTenant
		if tenant == "" {
			tenant = rootTenant
		}
		rawPath := cloudopsRawPath
		if !cmd.Flags().Changed("output") {
			rawPath = types.AppendTenantToPath(rawPath, tenant)
		}

		var accounts []string
		if cloudopsAccounts != "" {
			accounts = strings.Split(cloudopsAccounts, ",")
		}
		res, err := cloudopsstages.ExecuteSenseHygiene(ctx, cloudopsstages.SenseHygieneInput{
			Accounts:        accounts,
			Service:         cloudopsService,
			OutputPath:      rawPath,
			AppendTimestamp: cloudopsAppendTS,
			AppendOutput:    cloudopsAppendOutput || cloudopsUpsert,
			Upsert:          cloudopsAppendOutput || cloudopsUpsert,
			Mock:            cloudopsMock,
			Tenant:          tenant,
		})
		if err != nil {
			return err
		}
		fmt.Printf("[CloudOps:Sense] %s\n", res.Message)
		return nil
	},
}

var cloudopsAnalyzeCmd = &cobra.Command{
	Use:   "analyze",
	Short: "Stage 2: Run DuckDB OLAP queries to calculate operational waste and stale assets",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		tenant := cloudopsTenant
		if tenant == "" {
			tenant = rootTenant
		}
		rawPath := cloudopsRawPath
		if !cmd.Flags().Changed("input") {
			rawPath = types.AppendTenantToPath(rawPath, tenant)
		}
		anomPath := cloudopsAnomPath
		if !cmd.Flags().Changed("output") {
			anomPath = types.AppendTenantToPath(anomPath, tenant)
		}

		res, err := cloudopsstages.ExecuteAnalysisWasteHygiene(ctx, cloudopsstages.AnalysisWasteHygieneInput{
			InputPath:  rawPath,
			OutputPath: anomPath,
			Tenant:     tenant,
		})
		if err != nil {
			return err
		}
		fmt.Printf("[CloudOps:Analyze] %s\n", res.Message)
		return nil
	},
}

var cloudopsEnrichCmd = &cobra.Command{
	Use:   "enrich",
	Short: "Stage 3: Annotate waste findings with environment and lifecycle metadata",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		tenant := cloudopsTenant
		if tenant == "" {
			tenant = rootTenant
		}
		anomPath := cloudopsAnomPath
		if !cmd.Flags().Changed("input") {
			anomPath = types.AppendTenantToPath(anomPath, tenant)
		}
		enrichPath := cloudopsEnrichPath
		if !cmd.Flags().Changed("output") {
			enrichPath = types.AppendTenantToPath(enrichPath, tenant)
		}

		res, err := cloudopsstages.ExecuteEnrichLifecycle(ctx, cloudopsstages.EnrichLifecycleInput{
			InputPath:  anomPath,
			OutputPath: enrichPath,
			Tenant:     tenant,
		})
		if err != nil {
			return err
		}
		fmt.Printf("[CloudOps:Enrich] %s\n", res.Message)
		return nil
	},
}

var cloudopsEnforceCmd = &cobra.Command{
	Use:   "enforce",
	Short: "Stage 4: Execute cleanup and asset remediation policies",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		tenant := cloudopsTenant
		if tenant == "" {
			tenant = rootTenant
		}
		enrichPath := cloudopsEnrichPath
		if !cmd.Flags().Changed("input") {
			enrichPath = types.AppendTenantToPath(enrichPath, tenant)
		}
		auditPath := cloudopsAuditPath
		if !cmd.Flags().Changed("output") {
			auditPath = types.AppendTenantToPath(auditPath, tenant)
		}

		res, err := cloudopsstages.ExecuteEnforceCleanup(ctx, cloudopsstages.EnforceCleanupInput{
			InputPath:  enrichPath,
			OutputPath: auditPath,
			DryRun:     cloudopsDryRun,
			Tenant:     tenant,
		})
		if err != nil {
			return err
		}
		fmt.Printf("[CloudOps:Enforce] %s\n", res.Message)
		return nil
	},
}

var cloudopsReportCmd = &cobra.Command{
	Use:   "report",
	Short: "Stage 5: Generate CloudOps reports (table, markdown, powerbi, excel, serve)",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		tenant := cloudopsTenant
		if tenant == "" {
			tenant = rootTenant
		}
		enrichPath := cloudopsEnrichPath
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

		switch strings.ToLower(cloudopsFormat) {
		case "markdown", "md":
			targetMD := cloudopsReportOut
			if targetMD == "" {
				targetMD = "cloudops_report.md"
			}
			if cloudopsAppendTS {
				targetMD = types.AppendTimestampToPath(targetMD)
			}
			md := report.RenderMarkdown(records, "CloudOps")
			if cloudopsReportOut != "" || cloudopsAppendTS {
				_ = os.WriteFile(targetMD, []byte(md), 0644)
				fmt.Printf("[CloudOps:Report] Markdown report written to %s\n", targetMD)
			} else {
				fmt.Println(md)
			}
		case "powerbi":
			pOut := "cloudops_report.parquet"
			if cloudopsReportOut != "" {
				pOut = cloudopsReportOut
			}
			if cloudopsAppendTS && !cloudopsUpsert {
				pOut = types.AppendTimestampToPath(pOut)
			}
			mOut := strings.TrimSuffix(pOut, ".parquet") + ".m"
			if cloudopsAppendTS && cloudopsUpsert {
				mOut = types.AppendTimestampToPath(mOut)
			}
			if cloudopsUpsert {
				if err := engine.UpsertEnrichedParquet(ctx, records, pOut); err != nil {
					return err
				}
			}
			return report.ExportPowerBI(ctx, records, pOut, mOut, engine)
		case "excel", "csv":
			cOut := "cloudops_report.csv"
			if cloudopsReportOut != "" {
				cOut = cloudopsReportOut
			}
			if cloudopsAppendTS {
				cOut = types.AppendTimestampToPath(cOut)
			}
			return report.ExportExcel(ctx, records, cOut, "", engine, report.DefaultCloudOpsSheet)
		case "serve", "http", "rest":
			srv := rest.NewServer(8080, engine, "", "", cloudopsEnrichPath)
			return srv.Start(ctx)
		default:
			report.RenderConsoleTable(records, "CloudOps")
		}
		return nil
	},
}

func init() {
	cloudopsCmd.PersistentFlags().StringVar(&cloudopsTenant, "tenant", "", "Tenant ID or name for multi-tenant isolation")

	// Flags for sense
	cloudopsSenseCmd.Flags().StringVar(&cloudopsAccounts, "accounts", "", "Comma-separated list of AWS Account IDs (auto-detected via STS if omitted)")
	cloudopsSenseCmd.Flags().StringVar(&cloudopsService, "service", "all", "Service filter (e.g. ec2, eip, snapshot, all)")
	cloudopsSenseCmd.Flags().StringVar(&cloudopsRawPath, "output", "data/cloudops_raw.parquet", "Output raw parquet file path")
	cloudopsSenseCmd.Flags().BoolVarP(&cloudopsAppendTS, "append-timestamp", "t", false, "Append formatted datetime (_YYYYMMDD_HHMMSS) to output file names")
	cloudopsSenseCmd.Flags().BoolVarP(&cloudopsAppendOutput, "append-output", "a", false, "Append/merge records into original un-timestamped output file (--output) in addition to timestamped file")
	cloudopsSenseCmd.Flags().BoolVarP(&cloudopsUpsert, "upsert", "u", false, "Alias for --append-output: upsert/merge records into original output file")
	cloudopsSenseCmd.Flags().BoolVar(&cloudopsMock, "mock", true, "Use mock/offline cloud data")

	// Flags for analyze
	cloudopsAnalyzeCmd.Flags().StringVar(&cloudopsRawPath, "input", "data/cloudops_raw.parquet", "Input raw parquet file path")
	cloudopsAnalyzeCmd.Flags().StringVar(&cloudopsAnomPath, "output", "data/cloudops_anomalies.parquet", "Output anomalies parquet file path")

	// Flags for enrich
	cloudopsEnrichCmd.Flags().StringVar(&cloudopsAnomPath, "input", "data/cloudops_anomalies.parquet", "Input anomalies parquet file path")
	cloudopsEnrichCmd.Flags().StringVar(&cloudopsEnrichPath, "output", "data/cloudops_enriched.parquet", "Output enriched parquet file path")

	// Flags for enforce
	cloudopsEnforceCmd.Flags().StringVar(&cloudopsEnrichPath, "input", "data/cloudops_enriched.parquet", "Input enriched parquet file path")
	cloudopsEnforceCmd.Flags().StringVar(&cloudopsAuditPath, "output", "data/cloudops_audit.parquet", "Output enforce audit parquet file path")
	cloudopsEnforceCmd.Flags().BoolVar(&cloudopsDryRun, "dry-run", true, "Perform dry-run simulation without altering cloud resources")

	// Flags for report
	cloudopsReportCmd.Flags().StringVar(&cloudopsEnrichPath, "input", "data/cloudops_enriched.parquet", "Input enriched parquet file path")
	cloudopsReportCmd.Flags().StringVar(&cloudopsFormat, "format", "table", "Report format (table, markdown, powerbi, excel, csv, serve)")
	cloudopsReportCmd.Flags().StringVar(&cloudopsReportOut, "output", "", "Output file path (for markdown, powerbi parquet, or excel csv)")
	cloudopsReportCmd.Flags().BoolVarP(&cloudopsAppendTS, "append-timestamp", "t", false, "Append formatted datetime (_YYYYMMDD_HHMMSS) to output file names")
	cloudopsReportCmd.Flags().BoolVarP(&cloudopsUpsert, "upsert", "u", false, "Upsert new records into existing Parquet file instead of overwriting")

	cloudopsCmd.AddCommand(cloudopsSenseCmd)
	cloudopsCmd.AddCommand(cloudopsAnalyzeCmd)
	cloudopsCmd.AddCommand(cloudopsEnrichCmd)
	cloudopsCmd.AddCommand(cloudopsEnforceCmd)
	cloudopsCmd.AddCommand(cloudopsReportCmd)
}
