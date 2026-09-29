package report

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/PopGreenTh/OpenLZ/internal/cloud"
	"github.com/PopGreenTh/OpenLZ/internal/duckdb"
)

func TestExportExcel_AlwaysCreatesMScriptPointingToCSV(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	outPath := filepath.Join(tempDir, "finops_enriched_report.xlsx")

	engine, err := duckdb.NewEngine()
	if err != nil {
		t.Fatalf("failed initializing duckdb: %v", err)
	}
	defer engine.Close()

	records := []cloud.EnrichedRecord{
		{
			OpsDomain:         "finops",
			AccountID:         "111122223333",
			AccountName:       "LZ-Prod",
			Environment:       "Production",
			BusinessUnit:      "Digital Products",
			Owner:             "finops@company.com",
			CostCenter:        "CC-100",
			Service:           "AmazonEC2",
			ResourceOrKey:     "AmazonEC2",
			Metric:            "CostSpike",
			ActualValue:       350.0,
			ExpectedValue:     120.0,
			PotentialSavings:  230.0,
			Severity:          "HIGH",
			ActionRecommended: "Rightsize instances",
			Timestamp:         time.Now(),
		},
	}

	if err := ExportExcel(ctx, records, outPath, "", engine, "FinOps_Findings"); err != nil {
		t.Fatalf("ExportExcel failed: %v", err)
	}

	base := filepath.Join(tempDir, "finops_enriched_report")
	expectedXLSX := base + ".xlsx"
	expectedCSV := base + ".csv"
	expectedM := base + "_excel.m"

	// 1. Verify XLSX exists
	if _, err := os.Stat(expectedXLSX); os.IsNotExist(err) {
		t.Errorf("expected xlsx file %s to exist", expectedXLSX)
	}

	// 2. Verify CSV exists
	if _, err := os.Stat(expectedCSV); os.IsNotExist(err) {
		t.Errorf("expected csv file %s to exist", expectedCSV)
	}

	// 3. Verify .m file exists
	if _, err := os.Stat(expectedM); os.IsNotExist(err) {
		t.Errorf("expected .m file %s to exist", expectedM)
	}

	// 4. Verify .m file contents point directly to CSV with absolute path and Csv.Document
	mBytes, err := os.ReadFile(expectedM)
	if err != nil {
		t.Fatalf("failed reading .m file: %v", err)
	}
	mContent := string(mBytes)

	if !strings.Contains(mContent, "Csv.Document(File.Contents(") {
		t.Errorf("expected Csv.Document in .m file, got:\n%s", mContent)
	}

	absCSV, err := filepath.Abs(expectedCSV)
	if err != nil {
		t.Fatalf("failed resolving abs csv path: %v", err)
	}
	escapedAbsCSV := strings.ReplaceAll(filepath.Clean(absCSV), `\`, `\\`)

	if !strings.Contains(mContent, escapedAbsCSV) {
		t.Errorf("expected .m file to contain absolute escaped CSV path %q, got:\n%s", escapedAbsCSV, mContent)
	}
}
