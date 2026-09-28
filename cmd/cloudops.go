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
		var accounts []string
		if cloudopsAccounts != "" {
			accounts = strings.Split(cloudopsAccounts, ",")
		}
		res, err := cloudopsstages.ExecuteSenseHygiene(ctx, cloudopsstages.SenseHygieneInput{
			Accounts:   accounts,
			Service:    cloudopsService,
			OutputPath: cloudopsRawPath,
			Mock:       cloudopsMock,
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
		res, err := cloudopsstages.ExecuteAnalysisWasteHygiene(ctx, cloudopsstages.AnalysisWasteHygieneInput{
			InputPath:  cloudopsRawPath,
			OutputPath: cloudopsAnomPath,
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
		res, err := cloudopsstages.ExecuteEnrichLifecycle(ctx, cloudopsstages.EnrichLifecycleInput{
			InputPath:  cloudopsAnomPath,
			OutputPath: cloudopsEnrichPath,
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
		res, err := cloudopsstages.ExecuteEnforceCleanup(ctx, cloudopsstages.EnforceCleanupInput{
			InputPath:  cloudopsEnrichPath,
			OutputPath: cloudopsAuditPath,
			DryRun:     cloudopsDryRun,
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
		engine, err := duckdb.NewEngine()
		if err != nil {
			return err
		}
		defer engine.Close()

		records, err := engine.ReadEnrichedFromParquet(ctx, cloudopsEnrichPath)
		if err != nil {
			return fmt.Errorf("could not read enriched dataset from %s: %w", cloudopsEnrichPath, err)
		}

		switch strings.ToLower(cloudopsFormat) {
		case "markdown", "md":
			md := report.RenderMarkdown(records, "CloudOps")
			if cloudopsReportOut != "" {
				_ = os.WriteFile(cloudopsReportOut, []byte(md), 0644)
				fmt.Printf("[CloudOps:Report] Markdown report written to %s\n", cloudopsReportOut)
			} else {
				fmt.Println(md)
			}
		case "powerbi":
			mOut := "cloudops_report.m"
			pOut := "cloudops_report.parquet"
			if cloudopsReportOut != "" {
				pOut = cloudopsReportOut
				mOut = cloudopsReportOut + ".m"
			}
			return report.ExportPowerBI(ctx, records, pOut, mOut, engine)
		case "excel":
			cOut := "cloudops_report.csv"
			mOut := "cloudops_report_excel.m"
			if cloudopsReportOut != "" {
				cOut = cloudopsReportOut
				mOut = cloudopsReportOut + ".m"
			}
			return report.ExportExcel(ctx, records, cOut, mOut, engine)
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
	// Flags for sense
	cloudopsSenseCmd.Flags().StringVar(&cloudopsAccounts, "accounts", "", "Comma-separated list of AWS Account IDs (auto-detected via STS if omitted)")
	cloudopsSenseCmd.Flags().StringVar(&cloudopsService, "service", "all", "Service filter (e.g. ec2, eip, snapshot, all)")
	cloudopsSenseCmd.Flags().StringVar(&cloudopsRawPath, "output", "data/cloudops_raw.parquet", "Output raw parquet file path")
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
	cloudopsReportCmd.Flags().StringVar(&cloudopsFormat, "format", "table", "Report format (table, markdown, powerbi, excel, serve)")
	cloudopsReportCmd.Flags().StringVar(&cloudopsReportOut, "output", "", "Output file path (for markdown, powerbi parquet, or excel csv)")

	cloudopsCmd.AddCommand(cloudopsSenseCmd)
	cloudopsCmd.AddCommand(cloudopsAnalyzeCmd)
	cloudopsCmd.AddCommand(cloudopsEnrichCmd)
	cloudopsCmd.AddCommand(cloudopsEnforceCmd)
	cloudopsCmd.AddCommand(cloudopsReportCmd)
}
