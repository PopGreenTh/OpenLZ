package report

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/PopGreenTh/OpenLZ/internal/cloud"
	"github.com/xuri/excelize/v2"
)

// Standard default sheet names for OpenLZ operational domains and pipeline stages.
const (
	DefaultFinOpsSheet           = "FinOps_Raw"
	DefaultSecOpsSheet           = "SecOps_Findings"
	DefaultCloudOpsSheet         = "CloudOps_Hygiene"
	DefaultEnrichedSheet         = "Enriched_Findings"
	DefaultAnomaliesSheet        = "Anomalies_Analysis"
	DefaultEnforceAuditSheet     = "Enforce_Audit"
	DefaultExecutiveSummarySheet = "Executive_Summary"
)

// EnsureXLSXExtension ensures the provided path has a valid .xlsx file extension.
func EnsureXLSXExtension(path string) string {
	lower := strings.ToLower(path)
	if strings.HasSuffix(lower, ".xlsx") {
		return path
	}
	if strings.HasSuffix(lower, ".csv") {
		return path[:len(path)-4] + ".xlsx"
	}
	if strings.HasSuffix(lower, ".parquet") {
		return path[:len(path)-8] + ".xlsx"
	}
	return path + ".xlsx"
}

// sanitizeSheetName ensures the sheet name conforms to Excel workbook rules (<= 31 chars, no forbidden chars).
func sanitizeSheetName(name string) string {
	clean := strings.TrimSpace(name)
	if clean == "" {
		return "Sheet"
	}
	replacer := strings.NewReplacer(
		"\\", "_",
		"/", "_",
		"?", "_",
		"*", "_",
		":", "_",
		"[", "_",
		"]", "_",
	)
	clean = replacer.Replace(clean)
	if len(clean) > 31 {
		clean = clean[:31]
	}
	return clean
}

// OpenOrCreateWorkbook loads an existing Excel workbook from outputPath or initializes a fresh one.
func OpenOrCreateWorkbook(outputPath string) (*excelize.File, error) {
	target := EnsureXLSXExtension(outputPath)
	if _, err := os.Stat(target); err == nil {
		f, err := excelize.OpenFile(target)
		if err == nil {
			return f, nil
		}
	}
	return excelize.NewFile(), nil
}

// EnsureSheet ensures the named sheet exists in the workbook and is clean of stale data.
// - If the workbook has only the initial untouched "Sheet1", it renames it.
// - If the sheet already exists, its contents are recreated so new data cleanly replaces old rows.
// - If the sheet does not exist, a new sheet is added.
func EnsureSheet(f *excelize.File, sheetName string) error {
	sheetName = sanitizeSheetName(sheetName)
	sheets := f.GetSheetList()

	// 1. If fresh workbook with only default "Sheet1", rename it to target sheet.
	if len(sheets) == 1 && sheets[0] == "Sheet1" {
		return f.SetSheetName("Sheet1", sheetName)
	}

	// 2. Check if the sheet already exists.
	idx, err := f.GetSheetIndex(sheetName)
	if err == nil && idx >= 0 {
		// If it's the only sheet, excelize prevents deleting the last sheet.
		// Workaround: create a temporary placeholder, delete old sheet, create new sheet, delete placeholder.
		if len(sheets) == 1 {
			const tempPlaceholder = "__openlz_tmp__"
			if _, err := f.NewSheet(tempPlaceholder); err != nil {
				return err
			}
			_ = f.DeleteSheet(sheetName)
			if _, err := f.NewSheet(sheetName); err != nil {
				return err
			}
			_ = f.DeleteSheet(tempPlaceholder)
			return nil
		}
		// If multiple sheets exist, safely delete the existing sheet to clear its rows.
		_ = f.DeleteSheet(sheetName)
	}

	// 3. Create fresh new sheet
	_, err = f.NewSheet(sheetName)
	return err
}

// EnsureCleanSheet is an alias for EnsureSheet, ensuring clean sheet creation.
func EnsureCleanSheet(f *excelize.File, sheetName string) error {
	return EnsureSheet(f, sheetName)
}

// SaveWorkbook cleans default unused sheets, sets the active sheet, and saves the workbook.
func SaveWorkbook(f *excelize.File, outputPath string) error {
	target := EnsureXLSXExtension(outputPath)
	sheets := f.GetSheetList()
	if len(sheets) > 1 {
		for _, s := range sheets {
			if s == "Sheet1" {
				_ = f.DeleteSheet("Sheet1")
				break
			}
		}
	}
	if len(f.GetSheetList()) > 0 {
		f.SetActiveSheet(0)
	}
	dir := filepath.Dir(target)
	if dir != "" && dir != "." {
		_ = os.MkdirAll(dir, 0755)
	}
	return f.SaveAs(target)
}

// GetWorkbookSheets returns the list of all sheet names currently in an Excel file.
func GetWorkbookSheets(outputPath string) ([]string, error) {
	target := EnsureXLSXExtension(outputPath)
	f, err := excelize.OpenFile(target)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return f.GetSheetList(), nil
}

// MergeSheetToWorkbook opens an existing workbook (or creates a new one), executes the builder to
// populate or update a specific sheet, and saves the merged workbook back to disk.
func MergeSheetToWorkbook(outputPath, sheetName string, builder func(f *excelize.File, sheet string) error) error {
	target := EnsureXLSXExtension(outputPath)
	f, err := OpenOrCreateWorkbook(target)
	if err != nil {
		return err
	}
	defer f.Close()

	if sheetName == "" {
		sheetName = "Sheet1"
	}
	sheetName = sanitizeSheetName(sheetName)

	if err := builder(f, sheetName); err != nil {
		return err
	}
	return SaveWorkbook(f, target)
}

// ============================================================================
// Modular Sheet Builders (Separated by Sheet Name for Clean Merging)
// ============================================================================

// AddExecutiveSummarySheet appends or updates an Executive Dashboard KPI Summary sheet.
func AddExecutiveSummarySheet(
	f *excelize.File,
	sheetName string,
	finops []cloud.CostRecord,
	secops []cloud.SecurityRecord,
	cloudops []cloud.CloudOpsRecord,
	enriched []cloud.EnrichedRecord,
) error {
	if sheetName == "" {
		sheetName = DefaultExecutiveSummarySheet
	}
	sheetName = sanitizeSheetName(sheetName)
	if err := EnsureSheet(f, sheetName); err != nil {
		return fmt.Errorf("failed creating sheet %s: %w", sheetName, err)
	}

	titleStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 16, Color: "#1F4E79"},
	})
	kpiHeaderStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "#FFFFFF", Size: 11},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#1F4E79"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})
	currencyStyle, _ := f.NewStyle(&excelize.Style{
		CustomNumFmt: &[]string{"$#,##0.00"}[0],
		Font:         &excelize.Font{Bold: true, Size: 14},
	})
	countStyle, _ := f.NewStyle(&excelize.Style{
		CustomNumFmt: &[]string{"#,##0"}[0],
		Font:         &excelize.Font{Bold: true, Size: 14},
	})

	_ = f.SetCellValue(sheetName, "B2", "OpenLZ Landing Zone Operations — Executive Summary")
	_ = f.SetCellStyle(sheetName, "B2", "B2", titleStyle)

	_ = f.SetCellValue(sheetName, "B4", "Operational KPI / Metric")
	_ = f.SetCellValue(sheetName, "C4", "Value")
	_ = f.SetCellStyle(sheetName, "B4", "C4", kpiHeaderStyle)

	totalFinOpsSpend := 0.0
	for _, r := range finops {
		totalFinOpsSpend += r.Amount
	}
	totalPotentialSavings := 0.0
	for _, r := range enriched {
		totalPotentialSavings += r.PotentialSavings
	}
	criticalSecOpsCount := 0
	for _, r := range secops {
		if r.Severity == "CRITICAL" || r.Severity == "HIGH" {
			criticalSecOpsCount++
		}
	}
	totalCloudOpsWaste := 0.0
	for _, r := range cloudops {
		totalCloudOpsWaste += r.EstimatedWaste
	}

	_ = f.SetCellValue(sheetName, "B5", "Total Tracked Cloud Spend")
	_ = f.SetCellValue(sheetName, "C5", totalFinOpsSpend)
	_ = f.SetCellStyle(sheetName, "C5", "C5", currencyStyle)

	_ = f.SetCellValue(sheetName, "B6", "Identified Monthly Cost Savings")
	_ = f.SetCellValue(sheetName, "C6", totalPotentialSavings)
	_ = f.SetCellStyle(sheetName, "C6", "C6", currencyStyle)

	_ = f.SetCellValue(sheetName, "B7", "Critical / High Security Findings")
	_ = f.SetCellValue(sheetName, "C7", criticalSecOpsCount)
	_ = f.SetCellStyle(sheetName, "C7", "C7", countStyle)

	_ = f.SetCellValue(sheetName, "B8", "Estimated Orphaned Resource Waste")
	_ = f.SetCellValue(sheetName, "C8", totalCloudOpsWaste)
	_ = f.SetCellStyle(sheetName, "C8", "C8", currencyStyle)

	_ = f.SetColWidth(sheetName, "B", "B", 35)
	_ = f.SetColWidth(sheetName, "C", "C", 20)

	return nil
}

// AddFinOpsSheet appends or updates a FinOps cost/usage dataset on the specified sheet in an Excel workbook.
func AddFinOpsSheet(f *excelize.File, sheetName string, records []cloud.CostRecord) error {
	if sheetName == "" {
		sheetName = DefaultFinOpsSheet
	}
	sheetName = sanitizeSheetName(sheetName)
	if err := EnsureSheet(f, sheetName); err != nil {
		return fmt.Errorf("failed creating sheet %s: %w", sheetName, err)
	}

	headers := []string{
		"Account ID", "Account Name", "Service", "Usage Date", "Usage Period",
		"Amount", "Daily Amount", "Days In Period", "Metric", "Frequency",
		"Unit", "Currency", "Region", "Usage Type", "Primary Tag",
		"Secondary Tag", "Recorded At",
	}

	headerStyle, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "#FFFFFF", Size: 11},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#1F4E79"}, Pattern: 1}, // Deep Navy
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	if err == nil {
		_ = f.SetRowStyle(sheetName, 1, 1, headerStyle)
	}

	for colIdx, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(colIdx+1, 1)
		_ = f.SetCellValue(sheetName, cell, h)
	}

	numStyle, _ := f.NewStyle(&excelize.Style{
		CustomNumFmt: &[]string{"#,##0.0000"}[0],
	})

	now := time.Now()
	for rowIdx, r := range records {
		row := rowIdx + 2

		dailyAmount := r.DailyAmount
		if dailyAmount == 0 && r.Amount != 0 {
			dailyAmount = r.Amount
		}
		daysInPeriod := r.DaysInPeriod
		if daysInPeriod == 0 {
			daysInPeriod = 1.0
		}
		ts := r.Timestamp
		if ts.IsZero() {
			ts = now
		}
		pTag := r.PrimaryTag
		if pTag == "" {
			pTag = "Untagged"
		}
		sTag := r.SecondaryTag
		if sTag == "" {
			sTag = "Untagged"
		}

		_ = f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), r.AccountID)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("B%d", row), r.AccountName)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("C%d", row), r.Service)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("D%d", row), r.UsageDate)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("E%d", row), r.UsagePeriod)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("F%d", row), r.Amount)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("G%d", row), dailyAmount)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("H%d", row), daysInPeriod)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("I%d", row), r.Metric)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("J%d", row), r.Frequency)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("K%d", row), r.Unit)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("L%d", row), r.Currency)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("M%d", row), r.Region)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("N%d", row), r.UsageType)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("O%d", row), pTag)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("P%d", row), sTag)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("Q%d", row), ts.Format("2006-01-02 15:04:05"))

		_ = f.SetCellStyle(sheetName, fmt.Sprintf("F%d", row), fmt.Sprintf("H%d", row), numStyle)
	}

	if len(records) > 0 {
		_ = f.AutoFilter(sheetName, fmt.Sprintf("A1:Q%d", len(records)+1), nil)
	}

	_ = f.SetColWidth(sheetName, "A", "A", 16)
	_ = f.SetColWidth(sheetName, "B", "B", 24)
	_ = f.SetColWidth(sheetName, "C", "C", 32)
	_ = f.SetColWidth(sheetName, "D", "E", 14)
	_ = f.SetColWidth(sheetName, "F", "H", 16)
	_ = f.SetColWidth(sheetName, "I", "N", 16)
	_ = f.SetColWidth(sheetName, "O", "P", 20)
	_ = f.SetColWidth(sheetName, "Q", "Q", 20)

	return nil
}

// AddSecOpsSheet appends or updates a SecOps security posture dataset on the specified sheet in an Excel workbook.
func AddSecOpsSheet(f *excelize.File, sheetName string, records []cloud.SecurityRecord) error {
	if sheetName == "" {
		sheetName = DefaultSecOpsSheet
	}
	sheetName = sanitizeSheetName(sheetName)
	if err := EnsureSheet(f, sheetName); err != nil {
		return fmt.Errorf("failed creating sheet %s: %w", sheetName, err)
	}

	headers := []string{
		"Account ID", "Account Name", "Service", "Resource ID",
		"Finding Type", "Severity", "Description", "Remediation", "Recorded At",
	}

	headerStyle, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "#FFFFFF", Size: 11},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#C00000"}, Pattern: 1}, // Security Crimson Red
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	if err == nil {
		_ = f.SetRowStyle(sheetName, 1, 1, headerStyle)
	}

	for colIdx, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(colIdx+1, 1)
		_ = f.SetCellValue(sheetName, cell, h)
	}

	now := time.Now()
	for rowIdx, r := range records {
		row := rowIdx + 2

		ts := r.Timestamp
		if ts.IsZero() {
			ts = now
		}

		_ = f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), r.AccountID)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("B%d", row), r.AccountName)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("C%d", row), r.Service)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("D%d", row), r.ResourceID)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("E%d", row), r.FindingType)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("F%d", row), r.Severity)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("G%d", row), r.Description)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("H%d", row), r.Remediation)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("I%d", row), ts.Format("2006-01-02 15:04:05"))
	}

	if len(records) > 0 {
		_ = f.AutoFilter(sheetName, fmt.Sprintf("A1:I%d", len(records)+1), nil)
	}

	_ = f.SetColWidth(sheetName, "A", "B", 18)
	_ = f.SetColWidth(sheetName, "C", "D", 26)
	_ = f.SetColWidth(sheetName, "E", "F", 16)
	_ = f.SetColWidth(sheetName, "G", "H", 45)
	_ = f.SetColWidth(sheetName, "I", "I", 20)

	return nil
}

// AddCloudOpsSheet appends or updates a CloudOps hygiene dataset on the specified sheet in an Excel workbook.
func AddCloudOpsSheet(f *excelize.File, sheetName string, records []cloud.CloudOpsRecord) error {
	if sheetName == "" {
		sheetName = DefaultCloudOpsSheet
	}
	sheetName = sanitizeSheetName(sheetName)
	if err := EnsureSheet(f, sheetName); err != nil {
		return fmt.Errorf("failed creating sheet %s: %w", sheetName, err)
	}

	headers := []string{
		"Account ID", "Account Name", "Service", "Resource ID",
		"Issue Type", "Resource Age (Days)", "Estimated Waste ($)", "Recommendation", "Recorded At",
	}

	headerStyle, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "#FFFFFF", Size: 11},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#375623"}, Pattern: 1}, // Forest Green
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	if err == nil {
		_ = f.SetRowStyle(sheetName, 1, 1, headerStyle)
	}

	for colIdx, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(colIdx+1, 1)
		_ = f.SetCellValue(sheetName, cell, h)
	}

	currencyStyle, _ := f.NewStyle(&excelize.Style{
		CustomNumFmt: &[]string{"$#,##0.00"}[0],
	})

	now := time.Now()
	for rowIdx, r := range records {
		row := rowIdx + 2

		ts := r.Timestamp
		if ts.IsZero() {
			ts = now
		}

		_ = f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), r.AccountID)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("B%d", row), r.AccountName)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("C%d", row), r.Service)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("D%d", row), r.ResourceID)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("E%d", row), r.IssueType)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("F%d", row), r.ResourceAge)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("G%d", row), r.EstimatedWaste)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("H%d", row), r.Recommendation)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("I%d", row), ts.Format("2006-01-02 15:04:05"))

		_ = f.SetCellStyle(sheetName, fmt.Sprintf("G%d", row), fmt.Sprintf("G%d", row), currencyStyle)
	}

	if len(records) > 0 {
		_ = f.AutoFilter(sheetName, fmt.Sprintf("A1:I%d", len(records)+1), nil)
	}

	_ = f.SetColWidth(sheetName, "A", "B", 18)
	_ = f.SetColWidth(sheetName, "C", "D", 26)
	_ = f.SetColWidth(sheetName, "E", "F", 18)
	_ = f.SetColWidth(sheetName, "G", "G", 18)
	_ = f.SetColWidth(sheetName, "H", "H", 45)
	_ = f.SetColWidth(sheetName, "I", "I", 20)

	return nil
}

// AddEnrichedSheet appends or updates an enriched anomaly/findings dataset on the specified sheet in an Excel workbook.
func AddEnrichedSheet(f *excelize.File, sheetName string, records []cloud.EnrichedRecord) error {
	if sheetName == "" {
		sheetName = DefaultEnrichedSheet
	}
	sheetName = sanitizeSheetName(sheetName)
	if err := EnsureSheet(f, sheetName); err != nil {
		return fmt.Errorf("failed creating sheet %s: %w", sheetName, err)
	}

	headers := []string{
		"Ops Domain", "Account ID", "Account Name", "Environment", "Business Unit",
		"Owner", "Cost Center", "Service", "Resource / Key", "Metric",
		"Actual Value", "Expected Value", "Potential Savings ($)", "Severity",
		"Action Recommended", "Recorded At",
	}

	headerStyle, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "#FFFFFF", Size: 11},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#2F5597"}, Pattern: 1}, // Slate Blue
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	if err == nil {
		_ = f.SetRowStyle(sheetName, 1, 1, headerStyle)
	}

	for colIdx, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(colIdx+1, 1)
		_ = f.SetCellValue(sheetName, cell, h)
	}

	currencyStyle, _ := f.NewStyle(&excelize.Style{
		CustomNumFmt: &[]string{"$#,##0.00"}[0],
	})
	numStyle, _ := f.NewStyle(&excelize.Style{
		CustomNumFmt: &[]string{"#,##0.00"}[0],
	})

	now := time.Now()
	for rowIdx, r := range records {
		row := rowIdx + 2

		ts := r.Timestamp
		if ts.IsZero() {
			ts = now
		}

		_ = f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), r.OpsDomain)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("B%d", row), r.AccountID)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("C%d", row), r.AccountName)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("D%d", row), r.Environment)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("E%d", row), r.BusinessUnit)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("F%d", row), r.Owner)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("G%d", row), r.CostCenter)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("H%d", row), r.Service)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("I%d", row), r.ResourceOrKey)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("J%d", row), r.Metric)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("K%d", row), r.ActualValue)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("L%d", row), r.ExpectedValue)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("M%d", row), r.PotentialSavings)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("N%d", row), r.Severity)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("O%d", row), r.ActionRecommended)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("P%d", row), ts.Format("2006-01-02 15:04:05"))

		_ = f.SetCellStyle(sheetName, fmt.Sprintf("K%d", row), fmt.Sprintf("L%d", row), numStyle)
		_ = f.SetCellStyle(sheetName, fmt.Sprintf("M%d", row), fmt.Sprintf("M%d", row), currencyStyle)
	}

	if len(records) > 0 {
		_ = f.AutoFilter(sheetName, fmt.Sprintf("A1:P%d", len(records)+1), nil)
	}

	_ = f.SetColWidth(sheetName, "A", "A", 14)
	_ = f.SetColWidth(sheetName, "B", "C", 18)
	_ = f.SetColWidth(sheetName, "D", "G", 18)
	_ = f.SetColWidth(sheetName, "H", "I", 26)
	_ = f.SetColWidth(sheetName, "J", "M", 18)
	_ = f.SetColWidth(sheetName, "N", "N", 12)
	_ = f.SetColWidth(sheetName, "O", "O", 45)
	_ = f.SetColWidth(sheetName, "P", "P", 20)

	return nil
}

// AddAnomalySheet appends or updates detected deviations and anomalies on the specified sheet in an Excel workbook.
func AddAnomalySheet(f *excelize.File, sheetName string, records []cloud.AnomalyRecord) error {
	if sheetName == "" {
		sheetName = DefaultAnomaliesSheet
	}
	sheetName = sanitizeSheetName(sheetName)
	if err := EnsureSheet(f, sheetName); err != nil {
		return fmt.Errorf("failed creating sheet %s: %w", sheetName, err)
	}

	headers := []string{
		"Ops Domain", "Account ID", "Account Name", "Service",
		"Resource / Key", "Metric", "Actual Value", "Expected Value",
		"Deviation Ratio", "Severity", "Status", "Recorded At",
	}

	headerStyle, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "#FFFFFF", Size: 11},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#C65911"}, Pattern: 1}, // Amber / Investigation Orange
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	if err == nil {
		_ = f.SetRowStyle(sheetName, 1, 1, headerStyle)
	}

	for colIdx, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(colIdx+1, 1)
		_ = f.SetCellValue(sheetName, cell, h)
	}

	numStyle, _ := f.NewStyle(&excelize.Style{
		CustomNumFmt: &[]string{"#,##0.00"}[0],
	})

	now := time.Now()
	for rowIdx, r := range records {
		row := rowIdx + 2

		ts := r.Timestamp
		if ts.IsZero() {
			ts = now
		}

		_ = f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), r.OpsDomain)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("B%d", row), r.AccountID)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("C%d", row), r.AccountName)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("D%d", row), r.Service)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("E%d", row), r.ResourceOrKey)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("F%d", row), r.Metric)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("G%d", row), r.ActualValue)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("H%d", row), r.ExpectedValue)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("I%d", row), r.Deviation)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("J%d", row), r.Severity)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("K%d", row), r.Status)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("L%d", row), ts.Format("2006-01-02 15:04:05"))

		_ = f.SetCellStyle(sheetName, fmt.Sprintf("G%d", row), fmt.Sprintf("I%d", row), numStyle)
	}

	if len(records) > 0 {
		_ = f.AutoFilter(sheetName, fmt.Sprintf("A1:L%d", len(records)+1), nil)
	}

	_ = f.SetColWidth(sheetName, "A", "A", 14)
	_ = f.SetColWidth(sheetName, "B", "C", 18)
	_ = f.SetColWidth(sheetName, "D", "E", 24)
	_ = f.SetColWidth(sheetName, "F", "I", 18)
	_ = f.SetColWidth(sheetName, "J", "K", 14)
	_ = f.SetColWidth(sheetName, "L", "L", 20)

	return nil
}

// AddEnforceAuditSheet appends or updates remediation/policy audit findings on the specified sheet in an Excel workbook.
func AddEnforceAuditSheet(f *excelize.File, sheetName string, records []cloud.EnforceAuditRecord) error {
	if sheetName == "" {
		sheetName = DefaultEnforceAuditSheet
	}
	sheetName = sanitizeSheetName(sheetName)
	if err := EnsureSheet(f, sheetName); err != nil {
		return fmt.Errorf("failed creating sheet %s: %w", sheetName, err)
	}

	headers := []string{
		"Recorded At", "Ops Domain", "Account ID", "Service",
		"Resource ID", "Action Taken", "Dry Run", "Success", "Details",
	}

	headerStyle, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "#FFFFFF", Size: 11},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#1E4D54"}, Pattern: 1}, // Deep Teal
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	if err == nil {
		_ = f.SetRowStyle(sheetName, 1, 1, headerStyle)
	}

	for colIdx, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(colIdx+1, 1)
		_ = f.SetCellValue(sheetName, cell, h)
	}

	now := time.Now()
	for rowIdx, r := range records {
		row := rowIdx + 2

		ts := r.Timestamp
		if ts.IsZero() {
			ts = now
		}

		_ = f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), ts.Format("2006-01-02 15:04:05"))
		_ = f.SetCellValue(sheetName, fmt.Sprintf("B%d", row), r.OpsDomain)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("C%d", row), r.AccountID)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("D%d", row), r.Service)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("E%d", row), r.ResourceID)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("F%d", row), r.ActionTaken)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("G%d", row), r.DryRun)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("H%d", row), r.Success)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("I%d", row), r.Details)
	}

	if len(records) > 0 {
		_ = f.AutoFilter(sheetName, fmt.Sprintf("A1:I%d", len(records)+1), nil)
	}

	_ = f.SetColWidth(sheetName, "A", "A", 20)
	_ = f.SetColWidth(sheetName, "B", "C", 16)
	_ = f.SetColWidth(sheetName, "D", "E", 22)
	_ = f.SetColWidth(sheetName, "F", "F", 28)
	_ = f.SetColWidth(sheetName, "G", "H", 12)
	_ = f.SetColWidth(sheetName, "I", "I", 45)

	return nil
}

// ============================================================================
// Standalone & Multi-Domain File Exporters (Supports Merging into Existing Files)
// ============================================================================

// ExportFinOpsToXLSX writes raw FinOps records to an Excel workbook on the specified sheet (default: "FinOps_Raw").
// If the destination file already exists, it updates or adds the sheet without overwriting other sheets.
func ExportFinOpsToXLSX(records []cloud.CostRecord, outputPath string, sheetName ...string) error {
	sheet := DefaultFinOpsSheet
	if len(sheetName) > 0 && sheetName[0] != "" {
		sheet = sheetName[0]
	}
	return MergeSheetToWorkbook(outputPath, sheet, func(f *excelize.File, s string) error {
		return AddFinOpsSheet(f, s, records)
	})
}

// ExportSecOpsToXLSX writes SecOps posture findings to an Excel workbook on the specified sheet (default: "SecOps_Findings").
// If the destination file already exists, it updates or adds the sheet without overwriting other sheets.
func ExportSecOpsToXLSX(records []cloud.SecurityRecord, outputPath string, sheetName ...string) error {
	sheet := DefaultSecOpsSheet
	if len(sheetName) > 0 && sheetName[0] != "" {
		sheet = sheetName[0]
	}
	return MergeSheetToWorkbook(outputPath, sheet, func(f *excelize.File, s string) error {
		return AddSecOpsSheet(f, s, records)
	})
}

// ExportCloudOpsToXLSX writes CloudOps hygiene findings to an Excel workbook on the specified sheet (default: "CloudOps_Hygiene").
// If the destination file already exists, it updates or adds the sheet without overwriting other sheets.
func ExportCloudOpsToXLSX(records []cloud.CloudOpsRecord, outputPath string, sheetName ...string) error {
	sheet := DefaultCloudOpsSheet
	if len(sheetName) > 0 && sheetName[0] != "" {
		sheet = sheetName[0]
	}
	return MergeSheetToWorkbook(outputPath, sheet, func(f *excelize.File, s string) error {
		return AddCloudOpsSheet(f, s, records)
	})
}

// ExportEnrichedToXLSX writes enriched findings to an Excel workbook on the specified sheet (default: "Enriched_Findings").
// If the destination file already exists, it updates or adds the sheet without overwriting other sheets.
func ExportEnrichedToXLSX(records []cloud.EnrichedRecord, outputPath string, sheetName ...string) error {
	sheet := DefaultEnrichedSheet
	if len(sheetName) > 0 && sheetName[0] != "" {
		sheet = sheetName[0]
	}
	return MergeSheetToWorkbook(outputPath, sheet, func(f *excelize.File, s string) error {
		return AddEnrichedSheet(f, s, records)
	})
}

// ExportAnomaliesToXLSX writes anomaly detection records to an Excel workbook on the specified sheet (default: "Anomalies_Analysis").
// If the destination file already exists, it updates or adds the sheet without overwriting other sheets.
func ExportAnomaliesToXLSX(records []cloud.AnomalyRecord, outputPath string, sheetName ...string) error {
	sheet := DefaultAnomaliesSheet
	if len(sheetName) > 0 && sheetName[0] != "" {
		sheet = sheetName[0]
	}
	return MergeSheetToWorkbook(outputPath, sheet, func(f *excelize.File, s string) error {
		return AddAnomalySheet(f, s, records)
	})
}

// ExportEnforceAuditToXLSX writes enforcement audit records to an Excel workbook on the specified sheet (default: "Enforce_Audit").
// If the destination file already exists, it updates or adds the sheet without overwriting other sheets.
func ExportEnforceAuditToXLSX(records []cloud.EnforceAuditRecord, outputPath string, sheetName ...string) error {
	sheet := DefaultEnforceAuditSheet
	if len(sheetName) > 0 && sheetName[0] != "" {
		sheet = sheetName[0]
	}
	return MergeSheetToWorkbook(outputPath, sheet, func(f *excelize.File, s string) error {
		return AddEnforceAuditSheet(f, s, records)
	})
}

// ExportExecutiveSummaryToXLSX writes an executive summary KPI dashboard to an Excel workbook on the specified sheet (default: "Executive_Summary").
// If the destination file already exists, it updates or adds the sheet without overwriting other sheets.
func ExportExecutiveSummaryToXLSX(
	finops []cloud.CostRecord,
	secops []cloud.SecurityRecord,
	cloudops []cloud.CloudOpsRecord,
	enriched []cloud.EnrichedRecord,
	outputPath string,
	sheetName ...string,
) error {
	sheet := DefaultExecutiveSummarySheet
	if len(sheetName) > 0 && sheetName[0] != "" {
		sheet = sheetName[0]
	}
	return MergeSheetToWorkbook(outputPath, sheet, func(f *excelize.File, s string) error {
		return AddExecutiveSummarySheet(f, s, finops, secops, cloudops, enriched)
	})
}

// ExportMultiDomainWorkbook creates or merges a unified multi-sheet Excel workbook containing all operational domains.
func ExportMultiDomainWorkbook(
	finops []cloud.CostRecord,
	secops []cloud.SecurityRecord,
	cloudops []cloud.CloudOpsRecord,
	enriched []cloud.EnrichedRecord,
	outputPath string,
) error {
	target := EnsureXLSXExtension(outputPath)
	f, err := OpenOrCreateWorkbook(target)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := AddExecutiveSummarySheet(f, DefaultExecutiveSummarySheet, finops, secops, cloudops, enriched); err != nil {
		return err
	}
	if len(finops) > 0 {
		if err := AddFinOpsSheet(f, DefaultFinOpsSheet, finops); err != nil {
			return err
		}
	}
	if len(secops) > 0 {
		if err := AddSecOpsSheet(f, DefaultSecOpsSheet, secops); err != nil {
			return err
		}
	}
	if len(cloudops) > 0 {
		if err := AddCloudOpsSheet(f, DefaultCloudOpsSheet, cloudops); err != nil {
			return err
		}
	}
	if len(enriched) > 0 {
		if err := AddEnrichedSheet(f, DefaultEnrichedSheet, enriched); err != nil {
			return err
		}
	}

	return SaveWorkbook(f, target)
}
