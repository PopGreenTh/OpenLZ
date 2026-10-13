package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/PopGreenTh/OpenLZ/internal/duckdb"
	"github.com/PopGreenTh/OpenLZ/internal/report"
	"github.com/PopGreenTh/OpenLZ/internal/rest"
	secopsstages "github.com/PopGreenTh/OpenLZ/internal/stages/secops"
	types "github.com/PopGreenTh/OpenLZ/internal/stages/types"
	"github.com/spf13/cobra"
)

var (
	secopsAccounts   string
	secopsService    string
	secopsRawPath    string
	secopsAnomPath   string
	secopsEnrichPath string
	secopsAuditPath  string
	secopsDryRun     bool
	secopsMock       bool
	secopsFormat     string
	secopsReportOut  string
	secopsAppendTS     bool
	secopsAppendOutput bool
	secopsUpsert       bool
	secopsTenant       string
)

var secopsCmd = &cobra.Command{
	Use:   "secops",
	Short: "SecOps domain operations (Public S3, Open SGs, Inactive IAM)",
	Long:  "Execute SecOps lifecycle stages: sense, analyze, enrich, enforce, and report.",
}

var secopsSenseCmd = &cobra.Command{
	Use:   "sense",
	Short: "Stage 1: Scan AWS security posture concurrently into DuckDB raw.parquet",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		tenant := secopsTenant
		if tenant == "" {
			tenant = rootTenant
		}
		rawPath := secopsRawPath
		if !cmd.Flags().Changed("output") {
			rawPath = types.AppendTenantToPath(rawPath, tenant)
		}

		var accounts []string
		if secopsAccounts != "" {
			accounts = strings.Split(secopsAccounts, ",")
		}
		res, err := secopsstages.ExecuteSensePosture(ctx, secopsstages.SensePostureInput{
			Accounts:        accounts,
			Service:         secopsService,
			OutputPath:      rawPath,
			AppendTimestamp: secopsAppendTS,
			AppendOutput:    secopsAppendOutput || secopsUpsert,
			Upsert:          secopsAppendOutput || secopsUpsert,
			Mock:            secopsMock,
			Tenant:          tenant,
		})
		if err != nil {
			return err
		}
		fmt.Printf("[SecOps:Sense] %s\n", res.Message)
		return nil
	},
}

var secopsAnalyzeCmd = &cobra.Command{
	Use:   "analyze",
	Short: "Stage 2: Run DuckDB OLAP queries to filter high-severity risks",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		tenant := secopsTenant
		if tenant == "" {
			tenant = rootTenant
		}
		rawPath := secopsRawPath
		if !cmd.Flags().Changed("input") {
			rawPath = types.AppendTenantToPath(rawPath, tenant)
		}
		anomPath := secopsAnomPath
		if !cmd.Flags().Changed("output") {
			anomPath = types.AppendTenantToPath(anomPath, tenant)
		}

		res, err := secopsstages.ExecuteAnalysisRiskPosture(ctx, secopsstages.AnalysisRiskPostureInput{
			InputPath:  rawPath,
			OutputPath: anomPath,
			Tenant:     tenant,
		})
		if err != nil {
			return err
		}
		fmt.Printf("[SecOps:Analyze] %s\n", res.Message)
		return nil
	},
}

var secopsEnrichCmd = &cobra.Command{
	Use:   "enrich",
	Short: "Stage 3: Annotate security findings with blast radius and environment",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		tenant := secopsTenant
		if tenant == "" {
			tenant = rootTenant
		}
		anomPath := secopsAnomPath
		if !cmd.Flags().Changed("input") {
			anomPath = types.AppendTenantToPath(anomPath, tenant)
		}
		enrichPath := secopsEnrichPath
		if !cmd.Flags().Changed("output") {
			enrichPath = types.AppendTenantToPath(enrichPath, tenant)
		}

		res, err := secopsstages.ExecuteEnrichMetadata(ctx, secopsstages.EnrichMetadataInput{
			InputPath:  anomPath,
			OutputPath: enrichPath,
			Tenant:     tenant,
		})
		if err != nil {
			return err
		}
		fmt.Printf("[SecOps:Enrich] %s\n", res.Message)
		return nil
	},
}

var secopsEnforceCmd = &cobra.Command{
	Use:   "enforce",
	Short: "Stage 4: Remediate public exposure and policy violations",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		tenant := secopsTenant
		if tenant == "" {
			tenant = rootTenant
		}
		enrichPath := secopsEnrichPath
		if !cmd.Flags().Changed("input") {
			enrichPath = types.AppendTenantToPath(enrichPath, tenant)
		}
		auditPath := secopsAuditPath
		if !cmd.Flags().Changed("output") {
			auditPath = types.AppendTenantToPath(auditPath, tenant)
		}

		res, err := secopsstages.ExecuteEnforceRemediation(ctx, secopsstages.EnforceRemediationInput{
			InputPath:  enrichPath,
			OutputPath: auditPath,
			DryRun:     secopsDryRun,
			Tenant:     tenant,
		})
		if err != nil {
			return err
		}
		fmt.Printf("[SecOps:Enforce] %s\n", res.Message)
		return nil
	},
}

var secopsReportCmd = &cobra.Command{
	Use:   "report",
	Short: "Stage 5: Generate SecOps reports (table, markdown, powerbi, excel, serve)",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		tenant := secopsTenant
		if tenant == "" {
			tenant = rootTenant
		}
		enrichPath := secopsEnrichPath
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

		switch strings.ToLower(secopsFormat) {
		case "markdown", "md":
			targetMD := secopsReportOut
			if targetMD == "" {
				targetMD = "secops_report.md"
			}
			if secopsAppendTS {
				targetMD = types.AppendTimestampToPath(targetMD)
			}
			md := report.RenderMarkdown(records, "SecOps")
			if secopsReportOut != "" || secopsAppendTS {
				_ = os.WriteFile(targetMD, []byte(md), 0644)
				fmt.Printf("[SecOps:Report] Markdown report written to %s\n", targetMD)
			} else {
				fmt.Println(md)
			}
		case "powerbi":
			pOut := "secops_report.parquet"
			if secopsReportOut != "" {
				pOut = secopsReportOut
			}
			if secopsAppendTS && !secopsUpsert {
				pOut = types.AppendTimestampToPath(pOut)
			}
			mOut := strings.TrimSuffix(pOut, ".parquet") + ".m"
			if secopsAppendTS && secopsUpsert {
				mOut = types.AppendTimestampToPath(mOut)
			}
			if secopsUpsert {
				if err := engine.UpsertEnrichedParquet(ctx, records, pOut); err != nil {
					return err
				}
			}
			return report.ExportPowerBI(ctx, records, pOut, mOut, engine)
		case "excel", "csv":
			cOut := "secops_report.csv"
			if secopsReportOut != "" {
				cOut = secopsReportOut
			}
			if secopsAppendTS {
				cOut = types.AppendTimestampToPath(cOut)
			}
			return report.ExportExcel(ctx, records, cOut, "", engine, report.DefaultSecOpsSheet)
		case "serve", "http", "rest":
			srv := rest.NewServer(8080, engine, "", secopsEnrichPath, "")
			return srv.Start(ctx)
		default:
			report.RenderConsoleTable(records, "SecOps")
		}
		return nil
	},
}

func init() {
	secopsCmd.PersistentFlags().StringVar(&secopsTenant, "tenant", "", "Tenant ID or name for multi-tenant isolation")

	// Flags for sense
	secopsSenseCmd.Flags().StringVar(&secopsAccounts, "accounts", "", "Comma-separated list of AWS Account IDs (auto-detected via STS if omitted)")
	secopsSenseCmd.Flags().StringVar(&secopsService, "service", "all", "Service filter (e.g. s3, iam, ec2, all)")
	secopsSenseCmd.Flags().StringVar(&secopsRawPath, "output", "data/secops_raw.parquet", "Output raw parquet file path")
	secopsSenseCmd.Flags().BoolVarP(&secopsAppendTS, "append-timestamp", "t", false, "Append formatted datetime (_YYYYMMDD_HHMMSS) to output file names")
	secopsSenseCmd.Flags().BoolVarP(&secopsAppendOutput, "append-output", "a", false, "Append/merge records into original un-timestamped output file (--output) in addition to timestamped file")
	secopsSenseCmd.Flags().BoolVarP(&secopsUpsert, "upsert", "u", false, "Alias for --append-output: upsert/merge records into original output file")
	secopsSenseCmd.Flags().BoolVar(&secopsMock, "mock", true, "Use mock/offline cloud data")

	// Flags for analyze
	secopsAnalyzeCmd.Flags().StringVar(&secopsRawPath, "input", "data/secops_raw.parquet", "Input raw parquet file path")
	secopsAnalyzeCmd.Flags().StringVar(&secopsAnomPath, "output", "data/secops_anomalies.parquet", "Output anomalies parquet file path")

	// Flags for enrich
	secopsEnrichCmd.Flags().StringVar(&secopsAnomPath, "input", "data/secops_anomalies.parquet", "Input anomalies parquet file path")
	secopsEnrichCmd.Flags().StringVar(&secopsEnrichPath, "output", "data/secops_enriched.parquet", "Output enriched parquet file path")

	// Flags for enforce
	secopsEnforceCmd.Flags().StringVar(&secopsEnrichPath, "input", "data/secops_enriched.parquet", "Input enriched parquet file path")
	secopsEnforceCmd.Flags().StringVar(&secopsAuditPath, "output", "data/secops_audit.parquet", "Output enforce audit parquet file path")
	secopsEnforceCmd.Flags().BoolVar(&secopsDryRun, "dry-run", true, "Perform dry-run simulation without altering cloud resources")

	// Flags for report
	secopsReportCmd.Flags().StringVar(&secopsEnrichPath, "input", "data/secops_enriched.parquet", "Input enriched parquet file path")
	secopsReportCmd.Flags().StringVar(&secopsFormat, "format", "table", "Report format (table, markdown, powerbi, excel, csv, serve)")
	secopsReportCmd.Flags().StringVar(&secopsReportOut, "output", "", "Output file path (for markdown, powerbi parquet, or excel csv)")
	secopsReportCmd.Flags().BoolVarP(&secopsAppendTS, "append-timestamp", "t", false, "Append formatted datetime (_YYYYMMDD_HHMMSS) to output file names")
	secopsReportCmd.Flags().BoolVarP(&secopsUpsert, "upsert", "u", false, "Upsert new records into existing Parquet file instead of overwriting")

	secopsCmd.AddCommand(secopsSenseCmd)
	secopsCmd.AddCommand(secopsAnalyzeCmd)
	secopsCmd.AddCommand(secopsEnrichCmd)
	secopsCmd.AddCommand(secopsEnforceCmd)
	secopsCmd.AddCommand(secopsReportCmd)
}
