package report

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/PopGreenTh/OpenLZ/internal/cloud"
	"github.com/PopGreenTh/OpenLZ/internal/duckdb"
	"github.com/PopGreenTh/OpenLZ/internal/powerquery"
)

// RenderMarkdown formats enriched records into an executive/engineering GitHub Markdown report.
func RenderMarkdown(records []cloud.EnrichedRecord, title string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# OpenLZ %s Report\n\n", title))

	totalSavings := 0.0
	criticalCount := 0
	highCount := 0

	for _, r := range records {
		totalSavings += r.PotentialSavings
		if r.Severity == "CRITICAL" {
			criticalCount++
		} else if r.Severity == "HIGH" {
			highCount++
		}
	}

	sb.WriteString("### Executive Summary\n\n")
	sb.WriteString(fmt.Sprintf("- **Total Findings**: %d\n", len(records)))
	sb.WriteString(fmt.Sprintf("- **Potential Monthly Savings**: **$%.2f**\n", totalSavings))
	sb.WriteString(fmt.Sprintf("- **Critical Risks / Spikes**: %d\n", criticalCount))
	sb.WriteString(fmt.Sprintf("- **High Priority Items**: %d\n\n", highCount))

	sb.WriteString("| Account ID | Env | Business Unit | Service | Metric | Actual | Expected | Savings | Severity | Action Recommended |\n")
	sb.WriteString("| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |\n")

	for _, r := range records {
		sb.WriteString(fmt.Sprintf("| `%s` | %s | %s | %s | %s | %.2f | %.2f | $%.2f | **%s** | %s |\n",
			r.AccountID, r.Environment, r.BusinessUnit, r.Service, r.Metric,
			r.ActualValue, r.ExpectedValue, r.PotentialSavings, r.Severity, r.ActionRecommended))
	}

	return sb.String()
}

// RenderConsoleTable prints a formatted ASCII table directly to stdout.
func RenderConsoleTable(records []cloud.EnrichedRecord, title string) {
	fmt.Println("\n" + strings.Repeat("=", 105))
	fmt.Printf(" OPENLZ %s SUMMARY\n", strings.ToUpper(title))
	fmt.Println(strings.Repeat("=", 105))
	fmt.Printf("%-14s %-12s %-16s %-18s %-10s %-10s %-10s %-20s\n",
		"ACCOUNT", "ENV", "BusinessUnit", "SERVICE", "ACTUAL", "EXPECTED", "SEVERITY", "ACTION")
	fmt.Println(strings.Repeat("-", 105))

	for _, r := range records {
		fmt.Printf("%-14s %-12s %-16s %-18s %-10.2f %-10.2f %-10s %-20s\n",
			r.AccountID, r.Environment, r.BusinessUnit, truncate(r.Service, 17),
			r.ActualValue, r.ExpectedValue, r.Severity, truncate(r.ActionRecommended, 20))
	}
	fmt.Println(strings.Repeat("=", 105))
}

func truncate(s string, max int) string {
	if len(s) > max {
		return s[:max-3] + "..."
	}
	return s
}

// ExportPowerBI exports Parquet dataset and generates the Power Query M script.
func ExportPowerBI(ctx context.Context, records []cloud.EnrichedRecord, parquetPath, mScriptPath string, engine *duckdb.Engine) error {
	if parquetPath == "" {
		parquetPath = "report_powerbi.parquet"
	}
	if mScriptPath == "" {
		mScriptPath = "report_powerbi.m"
	}

	if err := engine.WriteEnrichedParquet(ctx, records, parquetPath); err != nil {
		return fmt.Errorf("failed to write powerbi parquet: %w", err)
	}

	mCode := powerquery.GenerateParquetMScript(parquetPath)
	if err := powerquery.WriteMScriptToFile(mCode, mScriptPath); err != nil {
		return fmt.Errorf("failed to write power query .m file: %w", err)
	}

	fmt.Printf("[Power BI Export Ready]\n")
	fmt.Printf("  -> Parquet Dataset: %s\n", parquetPath)
	fmt.Printf("  -> Power Query .m Formula: %s\n", mScriptPath)
	return nil
}

// ExportExcel exports CSV dataset and writes an Excel Power Query snippet.
func ExportExcel(ctx context.Context, records []cloud.EnrichedRecord, csvPath, mScriptPath string, engine *duckdb.Engine) error {
	if csvPath == "" {
		csvPath = "report_excel.csv"
	}
	if mScriptPath == "" {
		mScriptPath = "report_excel.m"
	}

	tempParquet := csvPath + ".tmp.parquet"
	defer os.Remove(tempParquet)

	if err := engine.WriteEnrichedParquet(ctx, records, tempParquet); err != nil {
		return fmt.Errorf("failed writing intermediate parquet for excel: %w", err)
	}

	if err := engine.ExportToCSV(ctx, tempParquet, csvPath); err != nil {
		return fmt.Errorf("failed to export excel csv: %w", err)
	}

	mCode := powerquery.GenerateParquetMScript(csvPath)
	_ = powerquery.WriteMScriptToFile(mCode, mScriptPath)

	fmt.Printf("[Excel Export Ready]\n")
	fmt.Printf("  -> CSV Dataset: %s\n", csvPath)
	fmt.Printf("  -> Power Query .m Formula: %s\n", mScriptPath)
	return nil
}
