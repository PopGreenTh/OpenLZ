package secops

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/PopGreenTh/OpenLZ/internal/duckdb"
	"github.com/PopGreenTh/OpenLZ/internal/report"
	types "github.com/PopGreenTh/OpenLZ/internal/stages/types"
)

// ReportSummaryInput models inputs for secops-report-summary.
type ReportSummaryInput struct {
	InputPath  string `json:"inputPath"`
	Format     string `json:"format"`
	OutputPath string `json:"outputPath"`
}

// ExecuteReportSummary runs Stage 5: SecOps Posture Reporting.
func ExecuteReportSummary(ctx context.Context, in ReportSummaryInput) (*types.StageResult, error) {
	if in.InputPath == "" {
		in.InputPath = "data/secops_enriched.parquet"
	}
	in.InputPath = types.ResolveAbsolutePath(in.InputPath)
	if in.OutputPath != "" {
		in.OutputPath = types.ResolveAbsolutePath(in.OutputPath)
	}
	if in.Format == "" {
		in.Format = "table"
	}

	engine, err := duckdb.NewEngine()
	if err != nil {
		return nil, fmt.Errorf("duckdb init failed: %w", err)
	}
	defer engine.Close()

	records, err := engine.ReadEnrichedFromParquet(ctx, in.InputPath)
	if err != nil {
		return nil, fmt.Errorf("could not read enriched dataset from %s: %w", in.InputPath, err)
	}

	switch strings.ToLower(in.Format) {
	case "markdown", "md":
		md := report.RenderMarkdown(records, "SecOps")
		if in.OutputPath != "" {
			_ = os.WriteFile(in.OutputPath, []byte(md), 0644)
		} else {
			fmt.Println(md)
		}
	case "powerbi":
		mOut := "secops_report.m"
		pOut := "secops_report.parquet"
		if in.OutputPath != "" {
			pOut = in.OutputPath
			mOut = in.OutputPath + ".m"
		}
		if err := report.ExportPowerBI(ctx, records, pOut, mOut, engine); err != nil {
			return nil, err
		}
	case "excel", "csv":
		cOut := "secops_report.csv"
		if in.OutputPath != "" {
			cOut = in.OutputPath
		}
		if err := report.ExportExcel(ctx, records, cOut, "", engine, report.DefaultSecOpsSheet); err != nil {
			return nil, err
		}
	default:
		report.RenderConsoleTable(records, "SecOps")
	}

	return &types.StageResult{
		Stage:       types.StageSecOpsReportSummary,
		Success:     true,
		RecordCount: len(records),
		OutputPath:  in.OutputPath,
		Message:     fmt.Sprintf("Generated SecOps report for %d records (format: %s)", len(records), in.Format),
	}, nil
}
