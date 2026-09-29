package report

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/PopGreenTh/OpenLZ/internal/cloud"
	"github.com/xuri/excelize/v2"
)

func TestExportFinOpsToXLSX_SingleSheet(t *testing.T) {
	tempDir := t.TempDir()
	outPath := filepath.Join(tempDir, "finops_test.xlsx")

	records := []cloud.CostRecord{
		{
			AccountID:    "111122223333",
			AccountName:  "LZ-Prod",
			Service:      "AmazonEC2",
			UsageDate:    "2026-09-01",
			UsagePeriod:  "2026-09",
			Amount:       150.25,
			DailyAmount:  150.25,
			DaysInPeriod: 1.0,
			Metric:       "UnblendedCost",
			Frequency:    "DAILY",
			Unit:         "USD",
			Currency:     "USD",
			Region:       "us-east-1",
			UsageType:    "BoxUsage:t3.medium",
			PrimaryTag:   "CorePlatform",
			SecondaryTag: "ApiGateway",
			Timestamp:    time.Now(),
		},
	}

	if err := ExportFinOpsToXLSX(records, outPath); err != nil {
		t.Fatalf("ExportFinOpsToXLSX failed: %v", err)
	}

	f, err := excelize.OpenFile(outPath)
	if err != nil {
		t.Fatalf("failed opening generated excel file: %v", err)
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) != 1 || sheets[0] != DefaultFinOpsSheet {
		t.Errorf("expected 1 sheet named %q, got %v", DefaultFinOpsSheet, sheets)
	}

	val, err := f.GetCellValue(DefaultFinOpsSheet, "A1")
	if err != nil || val != "Account ID" {
		t.Errorf("expected A1 to be 'Account ID', got %q (err: %v)", val, err)
	}

	acc, _ := f.GetCellValue(DefaultFinOpsSheet, "A2")
	if acc != "111122223333" {
		t.Errorf("expected A2 to be '111122223333', got %q", acc)
	}
}

func TestMergeMultipleSheets(t *testing.T) {
	tempDir := t.TempDir()
	outPath := filepath.Join(tempDir, "master_operations.xlsx")

	finopsRecs := []cloud.CostRecord{
		{AccountID: "111122223333", AccountName: "LZ-Prod", Service: "AmazonEC2", Amount: 200.0, Timestamp: time.Now()},
	}
	secopsRecs := []cloud.SecurityRecord{
		{AccountID: "111122223333", AccountName: "LZ-Prod", FindingType: "PublicBucket", Severity: "CRITICAL", Timestamp: time.Now()},
	}
	cloudopsRecs := []cloud.CloudOpsRecord{
		{AccountID: "444455556666", AccountName: "LZ-Dev", IssueType: "OrphanedVolume", EstimatedWaste: 45.0, Timestamp: time.Now()},
	}
	enrichedRecs := []cloud.EnrichedRecord{
		{OpsDomain: "finops", AccountID: "111122223333", Severity: "HIGH", PotentialSavings: 50.0, Timestamp: time.Now()},
	}
	anomRecs := []cloud.AnomalyRecord{
		{OpsDomain: "finops", AccountID: "111122223333", Metric: "CostSpike", ActualValue: 120.0, ExpectedValue: 50.0, Timestamp: time.Now()},
	}
	auditRecs := []cloud.EnforceAuditRecord{
		{OpsDomain: "secops", AccountID: "111122223333", ActionTaken: "BlockPublicS3", DryRun: true, Success: true, Timestamp: time.Now()},
	}

	// 1. Export FinOps (creates the file)
	if err := ExportFinOpsToXLSX(finopsRecs, outPath); err != nil {
		t.Fatalf("ExportFinOpsToXLSX failed: %v", err)
	}

	// 2. Merge SecOps into the SAME file
	if err := ExportSecOpsToXLSX(secopsRecs, outPath); err != nil {
		t.Fatalf("ExportSecOpsToXLSX failed: %v", err)
	}

	// 3. Merge CloudOps into the SAME file
	if err := ExportCloudOpsToXLSX(cloudopsRecs, outPath); err != nil {
		t.Fatalf("ExportCloudOpsToXLSX failed: %v", err)
	}

	// 4. Merge Enriched into the SAME file
	if err := ExportEnrichedToXLSX(enrichedRecs, outPath); err != nil {
		t.Fatalf("ExportEnrichedToXLSX failed: %v", err)
	}

	// 5. Merge Anomalies into the SAME file
	if err := ExportAnomaliesToXLSX(anomRecs, outPath); err != nil {
		t.Fatalf("ExportAnomaliesToXLSX failed: %v", err)
	}

	// 6. Merge Enforce Audit into the SAME file
	if err := ExportEnforceAuditToXLSX(auditRecs, outPath); err != nil {
		t.Fatalf("ExportEnforceAuditToXLSX failed: %v", err)
	}

	// Verify all 6 sheets exist
	f, err := excelize.OpenFile(outPath)
	if err != nil {
		t.Fatalf("failed opening merged workbook: %v", err)
	}
	defer f.Close()

	sheets := f.GetSheetList()
	expectedSheets := []string{
		DefaultFinOpsSheet,
		DefaultSecOpsSheet,
		DefaultCloudOpsSheet,
		DefaultEnrichedSheet,
		DefaultAnomaliesSheet,
		DefaultEnforceAuditSheet,
	}

	sheetMap := make(map[string]bool)
	for _, s := range sheets {
		sheetMap[s] = true
		if s == "Sheet1" {
			t.Errorf("found unwanted default 'Sheet1' in merged workbook")
		}
	}

	for _, expected := range expectedSheets {
		if !sheetMap[expected] {
			t.Errorf("missing expected sheet %q in workbook sheets: %v", expected, sheets)
		}
	}
}

func TestSheetDataReplacementOnReExport(t *testing.T) {
	tempDir := t.TempDir()
	outPath := filepath.Join(tempDir, "replace_test.xlsx")

	// Initial batch: 5 records
	batch1 := make([]cloud.CostRecord, 5)
	for i := 0; i < 5; i++ {
		batch1[i] = cloud.CostRecord{AccountID: "OLD_ACC", Service: "EC2", Amount: 10.0}
	}
	if err := ExportFinOpsToXLSX(batch1, outPath); err != nil {
		t.Fatalf("batch1 export failed: %v", err)
	}

	// Second batch: only 2 records
	batch2 := []cloud.CostRecord{
		{AccountID: "NEW_ACC_1", Service: "S3", Amount: 50.0},
		{AccountID: "NEW_ACC_2", Service: "Lambda", Amount: 30.0},
	}
	if err := ExportFinOpsToXLSX(batch2, outPath); err != nil {
		t.Fatalf("batch2 export failed: %v", err)
	}

	f, err := excelize.OpenFile(outPath)
	if err != nil {
		t.Fatalf("failed opening workbook: %v", err)
	}
	defer f.Close()

	// Row 2 and 3 should have new data
	acc2, _ := f.GetCellValue(DefaultFinOpsSheet, "A2")
	acc3, _ := f.GetCellValue(DefaultFinOpsSheet, "A3")
	if acc2 != "NEW_ACC_1" || acc3 != "NEW_ACC_2" {
		t.Errorf("expected A2='NEW_ACC_1' and A3='NEW_ACC_2', got A2=%q, A3=%q", acc2, acc3)
	}

	// Row 4 and above should be empty (old rows cleanly wiped)
	acc4, _ := f.GetCellValue(DefaultFinOpsSheet, "A4")
	if acc4 != "" {
		t.Errorf("expected A4 to be empty after replacing sheet, but found stale value %q", acc4)
	}
}

func TestCustomSheetName(t *testing.T) {
	tempDir := t.TempDir()
	outPath := filepath.Join(tempDir, "custom_sheets.xlsx")

	recordsProd := []cloud.CostRecord{{AccountID: "1111", Service: "EC2", Amount: 100.0}}
	recordsDev := []cloud.CostRecord{{AccountID: "2222", Service: "S3", Amount: 25.0}}

	if err := ExportFinOpsToXLSX(recordsProd, outPath, "FinOps_Production"); err != nil {
		t.Fatalf("export prod failed: %v", err)
	}
	if err := ExportFinOpsToXLSX(recordsDev, outPath, "FinOps_Development"); err != nil {
		t.Fatalf("export dev failed: %v", err)
	}

	sheets, err := GetWorkbookSheets(outPath)
	if err != nil {
		t.Fatalf("GetWorkbookSheets failed: %v", err)
	}

	if len(sheets) != 2 {
		t.Fatalf("expected 2 sheets, got %d: %v", len(sheets), sheets)
	}
	if sheets[0] != "FinOps_Production" || sheets[1] != "FinOps_Development" {
		t.Errorf("unexpected sheet names: %v", sheets)
	}
}

func TestExportMultiDomainWorkbook(t *testing.T) {
	tempDir := t.TempDir()
	outPath := filepath.Join(tempDir, "multi_domain_executive.xlsx")

	finops := []cloud.CostRecord{{AccountID: "111", Amount: 1500.0}}
	secops := []cloud.SecurityRecord{{AccountID: "111", Severity: "CRITICAL"}}
	cloudops := []cloud.CloudOpsRecord{{AccountID: "111", EstimatedWaste: 200.0}}
	enriched := []cloud.EnrichedRecord{{AccountID: "111", PotentialSavings: 400.0}}

	if err := ExportMultiDomainWorkbook(finops, secops, cloudops, enriched, outPath); err != nil {
		t.Fatalf("ExportMultiDomainWorkbook failed: %v", err)
	}

	if _, err := os.Stat(outPath); err != nil {
		t.Fatalf("expected output file to exist: %v", err)
	}

	f, err := excelize.OpenFile(outPath)
	if err != nil {
		t.Fatalf("failed opening multi-domain workbook: %v", err)
	}
	defer f.Close()

	sheets := f.GetSheetList()
	expected := []string{
		DefaultExecutiveSummarySheet,
		DefaultFinOpsSheet,
		DefaultSecOpsSheet,
		DefaultCloudOpsSheet,
		DefaultEnrichedSheet,
	}

	for _, exp := range expected {
		idx, err := f.GetSheetIndex(exp)
		if err != nil || idx < 0 {
			t.Errorf("missing sheet %q in %v", exp, sheets)
		}
	}
}
