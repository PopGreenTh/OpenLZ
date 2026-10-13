package finops

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/PopGreenTh/OpenLZ/internal/duckdb"
	"github.com/PopGreenTh/OpenLZ/internal/report"
	types "github.com/PopGreenTh/OpenLZ/internal/stages/types"
)

// ReportSummaryInput models inputs for finops-report-summary.
type ReportSummaryInput struct {
	InputPath       string `json:"inputPath"`
	Format          string `json:"format"`
	OutputPath      string `json:"outputPath"`
	AppendTimestamp bool   `json:"appendTimestamp"`
	Upsert          bool   `json:"upsert"`
	Tenant          string `json:"tenant"`
}

// ExecuteReportSummary runs Stage 5: FinOps Reporting & Visualization.
func ExecuteReportSummary(ctx context.Context, in ReportSummaryInput) (*types.StageResult, error) {
	in.Tenant = strings.TrimSpace(in.Tenant)
	if in.InputPath == "" {
		in.InputPath = types.AppendTenantToPath("data/finops_enriched.parquet", in.Tenant)
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

	outPath := in.OutputPath

	switch strings.ToLower(in.Format) {
	case "markdown", "md":
		targetMD := in.OutputPath
		if targetMD == "" {
			targetMD = "finops_report.md"
		}
		if in.AppendTimestamp {
			targetMD = types.AppendTimestampToPath(targetMD)
		}
		md := report.RenderMarkdown(records, "FinOps")
		if in.OutputPath != "" || in.AppendTimestamp {
			_ = os.WriteFile(targetMD, []byte(md), 0644)
			outPath = targetMD
		} else {
			fmt.Println(md)
		}
	case "powerbi":
		pOut := "finops_report.parquet"
		if in.OutputPath != "" {
			pOut = in.OutputPath
		}
		if in.AppendTimestamp && !in.Upsert {
			pOut = types.AppendTimestampToPath(pOut)
		}
		mOut := strings.TrimSuffix(pOut, ".parquet") + ".m"
		if in.AppendTimestamp && in.Upsert {
			mOut = types.AppendTimestampToPath(mOut)
		}
		if in.Upsert {
			if err := engine.UpsertEnrichedParquet(ctx, records, pOut); err != nil {
				return nil, err
			}
		}
		if err := report.ExportPowerBI(ctx, records, pOut, mOut, engine); err != nil {
			return nil, err
		}
		outPath = fmt.Sprintf("%s, %s", pOut, mOut)
	case "excel", "csv":
		cOut := "finops_report.csv"
		if in.OutputPath != "" {
			cOut = in.OutputPath
		}
		if in.AppendTimestamp {
			cOut = types.AppendTimestampToPath(cOut)
		}
		if err := report.ExportExcel(ctx, records, cOut, "", engine); err != nil {
			return nil, err
		}
		baseOut := types.StripExtensions(cOut)
		outPath = fmt.Sprintf("%s.xlsx, %s.csv, %s_excel.m", baseOut, baseOut, baseOut)
	default:
		report.RenderConsoleTable(records, "FinOps")
	}

	return &types.StageResult{
		Stage:       types.StageFinOpsReportSummary,
		Success:     true,
		RecordCount: len(records),
		OutputPath:  outPath,
		Message:     fmt.Sprintf("Generated FinOps report for %d records (format: %s)", len(records), in.Format),
	}, nil
}
