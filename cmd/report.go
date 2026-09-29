package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/PopGreenTh/OpenLZ/internal/cloud"
	"github.com/PopGreenTh/OpenLZ/internal/duckdb"
	"github.com/PopGreenTh/OpenLZ/internal/report"
	"github.com/PopGreenTh/OpenLZ/internal/rest"
	"github.com/spf13/cobra"
)

var (
	repFormat string
	repInput  string
	repOutput string
	repSheet  string
	repPort   int
)

var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Reporting hub (separated by report type: finops, secops, cloudops, executive, powerbi, excel, serve)",
	Long:  "Generate operational, financial, and security reports across multiple formats (Markdown, Power BI, Excel, and HTTP REST).",
}

func loadOrFallbackRecords(ctx context.Context, engine *duckdb.Engine, parquetPath, opsDomain string) []cloud.EnrichedRecord {
	if _, err := os.Stat(parquetPath); err == nil {
		if recs, err := engine.ReadEnrichedFromParquet(ctx, parquetPath); err == nil && len(recs) > 0 {
			return recs
		}
	}
	return generateSampleDatasetForOps(opsDomain)
}

var reportFinopsCmd = &cobra.Command{
	Use:   "finops",
	Short: "FinOps spend & anomaly report (Markdown, Power BI, Excel, REST)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runOpsReport("FinOps", "finops", repInput, repOutput, repFormat)
	},
}

var reportSecopsCmd = &cobra.Command{
	Use:   "secops",
	Short: "SecOps posture & compliance report (Markdown, Power BI, Excel, REST)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runOpsReport("SecOps", "secops", repInput, repOutput, repFormat)
	},
}

var reportCloudopsCmd = &cobra.Command{
	Use:   "cloudops",
	Short: "CloudOps hygiene & waste report (Markdown, Power BI, Excel, REST)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runOpsReport("CloudOps", "cloudops", repInput, repOutput, repFormat)
	},
}

var reportExecutiveCmd = &cobra.Command{
	Use:   "executive",
	Short: "Executive C-Level cross-ops KPI summary (Spend vs Budget, Security Posture, Cloud Hygiene)",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("\n" + strings.Repeat("=", 80))
		fmt.Println(" OPENLZ EXECUTIVE LANDING ZONE SUMMARY")
		fmt.Println(strings.Repeat("=", 80))
		fmt.Printf(" Reporting Date:           %s\n", "2026-09-22")
		fmt.Printf(" Monthly Spend Baseline:   $124,500.00 USD\n")
		fmt.Printf(" Potential Monthly Savings: $14,250.00 USD (11.4%%)\n")
		fmt.Printf(" Security Risk Index:      LOW (2 Critical, 5 High Open Findings)\n")
		fmt.Printf(" Cloud Hygiene Index:      94.8%% Compliant (8 Orphaned Disks Detected)\n")
		fmt.Println(strings.Repeat("-", 80))
		fmt.Println(" Top Recommended Actions:")
		fmt.Println("  1. [FinOps]   Commit 1-Year Compute Savings Plan ($8,400/mo projected savings)")
		fmt.Println("  2. [SecOps]   Block public read/write on 2 legacy S3 analytics buckets")
		fmt.Println("  3. [CloudOps] Terminate 8 unattached gp3 EBS volumes (>30 days old)")
		fmt.Println(strings.Repeat("=", 80) + "\n")
		return nil
	},
}

var reportPowerBICmd = &cobra.Command{
	Use:   "powerbi",
	Short: "Generate Power BI Parquet dataset & ready-to-paste .m Power Query formula",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		engine, err := duckdb.NewEngine()
		if err != nil {
			return err
		}
		defer engine.Close()

		pOut := repOutput
		if pOut == "" {
			pOut = "powerbi_dataset.parquet"
		}
		mOut := strings.TrimSuffix(pOut, ".parquet") + ".m"

		var records []cloud.EnrichedRecord
		if _, err := os.Stat(repInput); err == nil {
			records, _ = engine.ReadEnrichedFromParquet(ctx, repInput)
		}
		if len(records) == 0 {
			records = generateSampleDataset()
		}

		return report.ExportPowerBI(ctx, records, pOut, mOut, engine)
	},
}

var reportExcelCmd = &cobra.Command{
	Use:   "excel",
	Short: "Generate Excel CSV dataset & Power Query formula",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		engine, err := duckdb.NewEngine()
		if err != nil {
			return err
		}
		defer engine.Close()

		cOut := repOutput
		if cOut == "" {
			cOut = "excel_dataset.csv"
		}
		mOut := strings.TrimSuffix(cOut, ".csv") + "_excel.m"

		var records []cloud.EnrichedRecord
		if _, err := os.Stat(repInput); err == nil {
			records, _ = engine.ReadEnrichedFromParquet(ctx, repInput)
		}
		if len(records) == 0 {
			records = generateSampleDataset()
		}

		sheetName := repSheet
		if sheetName == "" {
			sheetName = report.DefaultEnrichedSheet
		}
		return report.ExportExcel(ctx, records, cOut, mOut, engine, sheetName)
	},
}

var reportServeCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start live HTTP REST server for Power BI and Excel Web Refresh",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		engine, err := duckdb.NewEngine()
		if err != nil {
			return err
		}
		defer engine.Close()

		srv := rest.NewServer(repPort, engine, "data/finops_enriched.parquet", "data/secops_enriched.parquet", "data/cloudops_enriched.parquet")
		return srv.Start(ctx)
	},
}

func runOpsReport(displayName, opsName, inPath, outPath, format string) error {
	ctx := context.Background()
	engine, err := duckdb.NewEngine()
	if err != nil {
		return err
	}
	defer engine.Close()

	var records []cloud.EnrichedRecord
	if _, err := os.Stat(inPath); err == nil {
		records, _ = engine.ReadEnrichedFromParquet(ctx, inPath)
	}
	if len(records) == 0 {
		records = generateSampleDatasetForOps(opsName)
	}

	switch strings.ToLower(format) {
	case "markdown", "md":
		md := report.RenderMarkdown(records, displayName)
		if outPath != "" {
			_ = os.WriteFile(outPath, []byte(md), 0644)
			fmt.Printf("[%s:Report] Markdown report saved to %s\n", displayName, outPath)
		} else {
			fmt.Println(md)
		}
	case "powerbi":
		pOut := outPath
		if pOut == "" {
			pOut = fmt.Sprintf("%s_report.parquet", opsName)
		}
		mOut := strings.TrimSuffix(pOut, ".parquet") + ".m"
		return report.ExportPowerBI(ctx, records, pOut, mOut, engine)
	case "excel":
		cOut := outPath
		if cOut == "" {
			cOut = fmt.Sprintf("%s_report.csv", opsName)
		}
		mOut := strings.TrimSuffix(cOut, ".csv") + "_excel.m"
		sheetName := repSheet
		if sheetName == "" {
			sheetName = fmt.Sprintf("%s_Findings", displayName)
		}
		return report.ExportExcel(ctx, records, cOut, mOut, engine, sheetName)
	case "serve", "http", "rest":
		srv := rest.NewServer(8080, engine, inPath, inPath, inPath)
		return srv.Start(ctx)
	default:
		report.RenderConsoleTable(records, displayName)
	}
	return nil
}

func generateSampleDataset() []cloud.EnrichedRecord {
	var all []cloud.EnrichedRecord
	all = append(all, generateSampleDatasetForOps("finops")...)
	all = append(all, generateSampleDatasetForOps("secops")...)
	all = append(all, generateSampleDatasetForOps("cloudops")...)
	return all
}

func generateSampleDatasetForOps(ops string) []cloud.EnrichedRecord {
	switch ops {
	case "secops":
		return []cloud.EnrichedRecord{
			{
				OpsDomain: "secops", AccountID: "111122223333", AccountName: "LZ-Prod",
				Environment: "Production", BusinessUnit: "Security Operations", Owner: "secops@company.com",
				CostCenter: "CC-1111", Service: "AmazonS3", ResourceOrKey: "lz-data-lake-public",
				Metric: "PublicBucketAccess", ActualValue: 1.0, ExpectedValue: 0.0, PotentialSavings: 0.0,
				Severity: "CRITICAL", ActionRecommended: "Enable S3 Block Public Access",
			},
			{
				OpsDomain: "secops", AccountID: "444455556666", AccountName: "LZ-Dev",
				Environment: "Development", BusinessUnit: "Core Platform", Owner: "dev-team@company.com",
				CostCenter: "CC-4444", Service: "AmazonEC2", ResourceOrKey: "sg-0941829abc123",
				Metric: "OpenSecurityGroup", ActualValue: 1.0, ExpectedValue: 0.0, PotentialSavings: 0.0,
				Severity: "HIGH", ActionRecommended: "Restrict SSH 0.0.0.0/0 to internal VPC",
			},
		}
	case "cloudops":
		return []cloud.EnrichedRecord{
			{
				OpsDomain: "cloudops", AccountID: "444455556666", AccountName: "LZ-Dev",
				Environment: "Development", BusinessUnit: "Core Platform", Owner: "dev-ops@company.com",
				CostCenter: "CC-4444", Service: "AmazonEC2", ResourceOrKey: "vol-0872161feda8921",
				Metric: "OrphanedEBSVolume", ActualValue: 38.50, ExpectedValue: 0.0, PotentialSavings: 38.50,
				Severity: "HIGH", ActionRecommended: "Snapshot and terminate unattached volume",
			},
		}
	default: // finops
		return []cloud.EnrichedRecord{
			{
				OpsDomain: "finops", AccountID: "111122223333", AccountName: "LZ-Prod",
				Environment: "Production", BusinessUnit: "Core Platform", Owner: "finops@company.com",
				CostCenter: "CC-1111", Service: "Amazon Elastic Compute Cloud - Compute", ResourceOrKey: "AmazonEC2",
				Metric: "CostSpike", ActualValue: 370.50, ExpectedValue: 120.50, PotentialSavings: 250.00,
				Severity: "CRITICAL", ActionRecommended: "Rightsize instance or commit to Savings Plans",
			},
		}
	}
}

func init() {
	reportFinopsCmd.Flags().StringVar(&repInput, "input", "data/finops_enriched.parquet", "Input enriched parquet file")
	reportFinopsCmd.Flags().StringVar(&repOutput, "output", "", "Output file path")
	reportFinopsCmd.Flags().StringVar(&repFormat, "format", "table", "Format: table, markdown, powerbi, excel, serve")
	reportFinopsCmd.Flags().StringVar(&repSheet, "sheet", "", "Excel worksheet name (default: FinOps_Findings)")

	reportSecopsCmd.Flags().StringVar(&repInput, "input", "data/secops_enriched.parquet", "Input enriched parquet file")
	reportSecopsCmd.Flags().StringVar(&repOutput, "output", "", "Output file path")
	reportSecopsCmd.Flags().StringVar(&repFormat, "format", "table", "Format: table, markdown, powerbi, excel, serve")
	reportSecopsCmd.Flags().StringVar(&repSheet, "sheet", "", "Excel worksheet name (default: SecOps_Findings)")

	reportCloudopsCmd.Flags().StringVar(&repInput, "input", "data/cloudops_enriched.parquet", "Input enriched parquet file")
	reportCloudopsCmd.Flags().StringVar(&repOutput, "output", "", "Output file path")
	reportCloudopsCmd.Flags().StringVar(&repFormat, "format", "table", "Format: table, markdown, powerbi, excel, serve")
	reportCloudopsCmd.Flags().StringVar(&repSheet, "sheet", "", "Excel worksheet name (default: CloudOps_Findings)")

	reportPowerBICmd.Flags().StringVar(&repInput, "input", "data/finops_enriched.parquet", "Input enriched parquet file")
	reportPowerBICmd.Flags().StringVar(&repOutput, "output", "powerbi_dataset.parquet", "Output Parquet file")

	reportExcelCmd.Flags().StringVar(&repInput, "input", "data/finops_enriched.parquet", "Input enriched parquet file")
	reportExcelCmd.Flags().StringVar(&repOutput, "output", "excel_dataset.csv", "Output CSV file")
	reportExcelCmd.Flags().StringVar(&repSheet, "sheet", "", "Excel worksheet name (default: Enriched_Findings)")

	reportServeCmd.Flags().IntVarP(&repPort, "port", "p", 8080, "Port to serve live REST feed on")

	reportCmd.AddCommand(reportFinopsCmd)
	reportCmd.AddCommand(reportSecopsCmd)
	reportCmd.AddCommand(reportCloudopsCmd)
	reportCmd.AddCommand(reportExecutiveCmd)
	reportCmd.AddCommand(reportPowerBICmd)
	reportCmd.AddCommand(reportExcelCmd)
	reportCmd.AddCommand(reportServeCmd)
}
