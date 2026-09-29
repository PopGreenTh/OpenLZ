package report

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/PopGreenTh/OpenLZ/internal/cloud"
	"github.com/xuri/excelize/v2"
)

// ExportFinOpsToXLSX writes raw FinOps records to a native Microsoft Excel (.xlsx) workbook.
func ExportFinOpsToXLSX(records []cloud.CostRecord, outputPath string) error {
	f := excelize.NewFile()
	defer f.Close()

	sheet := "FinOps_Raw"
	_ = f.SetSheetName("Sheet1", sheet)

	headers := []string{
		"Account ID", "Account Name", "Service", "Usage Date", "Usage Period",
		"Amount", "Daily Amount", "Days In Period", "Metric", "Frequency",
		"Unit", "Currency", "Region", "Usage Type", "Primary Tag",
		"Secondary Tag", "Recorded At",
	}

	headerStyle, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "#FFFFFF", Size: 11},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#1F4E79"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	if err == nil {
		_ = f.SetRowStyle(sheet, 1, 1, headerStyle)
	}

	for colIdx, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(colIdx+1, 1)
		_ = f.SetCellValue(sheet, cell, h)
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

		_ = f.SetCellValue(sheet, fmt.Sprintf("A%d", row), r.AccountID)
		_ = f.SetCellValue(sheet, fmt.Sprintf("B%d", row), r.AccountName)
		_ = f.SetCellValue(sheet, fmt.Sprintf("C%d", row), r.Service)
		_ = f.SetCellValue(sheet, fmt.Sprintf("D%d", row), r.UsageDate)
		_ = f.SetCellValue(sheet, fmt.Sprintf("E%d", row), r.UsagePeriod)
		_ = f.SetCellValue(sheet, fmt.Sprintf("F%d", row), r.Amount)
		_ = f.SetCellValue(sheet, fmt.Sprintf("G%d", row), dailyAmount)
		_ = f.SetCellValue(sheet, fmt.Sprintf("H%d", row), daysInPeriod)
		_ = f.SetCellValue(sheet, fmt.Sprintf("I%d", row), r.Metric)
		_ = f.SetCellValue(sheet, fmt.Sprintf("J%d", row), r.Frequency)
		_ = f.SetCellValue(sheet, fmt.Sprintf("K%d", row), r.Unit)
		_ = f.SetCellValue(sheet, fmt.Sprintf("L%d", row), r.Currency)
		_ = f.SetCellValue(sheet, fmt.Sprintf("M%d", row), r.Region)
		_ = f.SetCellValue(sheet, fmt.Sprintf("N%d", row), r.UsageType)
		_ = f.SetCellValue(sheet, fmt.Sprintf("O%d", row), pTag)
		_ = f.SetCellValue(sheet, fmt.Sprintf("P%d", row), sTag)
		_ = f.SetCellValue(sheet, fmt.Sprintf("Q%d", row), ts.Format("2006-01-02 15:04:05"))

		_ = f.SetCellStyle(sheet, fmt.Sprintf("F%d", row), fmt.Sprintf("H%d", row), numStyle)
	}

	if len(records) > 0 {
		_ = f.AutoFilter(sheet, fmt.Sprintf("A1:Q%d", len(records)+1), nil)
	}

	_ = f.SetColWidth(sheet, "A", "A", 16)
	_ = f.SetColWidth(sheet, "B", "B", 24)
	_ = f.SetColWidth(sheet, "C", "C", 32)
	_ = f.SetColWidth(sheet, "D", "E", 14)
	_ = f.SetColWidth(sheet, "F", "H", 16)
	_ = f.SetColWidth(sheet, "I", "N", 16)
	_ = f.SetColWidth(sheet, "O", "P", 20)
	_ = f.SetColWidth(sheet, "Q", "Q", 20)

	dir := filepath.Dir(outputPath)
	if dir != "" && dir != "." {
		_ = os.MkdirAll(dir, 0755)
	}

	return f.SaveAs(outputPath)
}

// ExportEnrichedToXLSX writes cross-domain enriched findings and anomaly reports to a native Excel workbook.
func ExportEnrichedToXLSX(records []cloud.EnrichedRecord, outputPath string) error {
	f := excelize.NewFile()
	defer f.Close()

	sheet := "Enriched_Findings"
	_ = f.SetSheetName("Sheet1", sheet)

	headers := []string{
		"Ops Domain", "Account ID", "Account Name", "Environment", "Business Unit",
		"Owner", "Cost Center", "Service", "Resource / Key", "Metric",
		"Actual Value", "Expected Value", "Potential Savings ($)", "Severity",
		"Action Recommended", "Timestamp",
	}

	headerStyle, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "#FFFFFF", Size: 11},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#2F5597"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	if err == nil {
		_ = f.SetRowStyle(sheet, 1, 1, headerStyle)
	}

	for colIdx, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(colIdx+1, 1)
		_ = f.SetCellValue(sheet, cell, h)
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

		_ = f.SetCellValue(sheet, fmt.Sprintf("A%d", row), r.OpsDomain)
		_ = f.SetCellValue(sheet, fmt.Sprintf("B%d", row), r.AccountID)
		_ = f.SetCellValue(sheet, fmt.Sprintf("C%d", row), r.AccountName)
		_ = f.SetCellValue(sheet, fmt.Sprintf("D%d", row), r.Environment)
		_ = f.SetCellValue(sheet, fmt.Sprintf("E%d", row), r.BusinessUnit)
		_ = f.SetCellValue(sheet, fmt.Sprintf("F%d", row), r.Owner)
		_ = f.SetCellValue(sheet, fmt.Sprintf("G%d", row), r.CostCenter)
		_ = f.SetCellValue(sheet, fmt.Sprintf("H%d", row), r.Service)
		_ = f.SetCellValue(sheet, fmt.Sprintf("I%d", row), r.ResourceOrKey)
		_ = f.SetCellValue(sheet, fmt.Sprintf("J%d", row), r.Metric)
		_ = f.SetCellValue(sheet, fmt.Sprintf("K%d", row), r.ActualValue)
		_ = f.SetCellValue(sheet, fmt.Sprintf("L%d", row), r.ExpectedValue)
		_ = f.SetCellValue(sheet, fmt.Sprintf("M%d", row), r.PotentialSavings)
		_ = f.SetCellValue(sheet, fmt.Sprintf("N%d", row), r.Severity)
		_ = f.SetCellValue(sheet, fmt.Sprintf("O%d", row), r.ActionRecommended)
		_ = f.SetCellValue(sheet, fmt.Sprintf("P%d", row), ts.Format("2006-01-02 15:04:05"))

		_ = f.SetCellStyle(sheet, fmt.Sprintf("K%d", row), fmt.Sprintf("L%d", row), numStyle)
		_ = f.SetCellStyle(sheet, fmt.Sprintf("M%d", row), fmt.Sprintf("M%d", row), currencyStyle)
	}

	if len(records) > 0 {
		_ = f.AutoFilter(sheet, fmt.Sprintf("A1:P%d", len(records)+1), nil)
	}

	_ = f.SetColWidth(sheet, "A", "A", 14)
	_ = f.SetColWidth(sheet, "B", "C", 18)
	_ = f.SetColWidth(sheet, "D", "G", 18)
	_ = f.SetColWidth(sheet, "H", "I", 26)
	_ = f.SetColWidth(sheet, "J", "M", 18)
	_ = f.SetColWidth(sheet, "N", "N", 12)
	_ = f.SetColWidth(sheet, "O", "O", 45)
	_ = f.SetColWidth(sheet, "P", "P", 20)

	dir := filepath.Dir(outputPath)
	if dir != "" && dir != "." {
		_ = os.MkdirAll(dir, 0755)
	}

	return f.SaveAs(outputPath)
}

// ExportSecOpsToXLSX writes security posture findings to a native Excel workbook.
func ExportSecOpsToXLSX(records []cloud.SecurityRecord, outputPath string) error {
	f := excelize.NewFile()
	defer f.Close()

	sheet := "SecOps_Findings"
	_ = f.SetSheetName("Sheet1", sheet)

	headers := []string{
		"Account ID", "Account Name", "Service", "Resource ID",
		"Finding Type", "Severity", "Description", "Remediation", "Recorded At",
	}

	headerStyle, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "#FFFFFF", Size: 11},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#C00000"}, Pattern: 1}, // Red security theme
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	if err == nil {
		_ = f.SetRowStyle(sheet, 1, 1, headerStyle)
	}

	for colIdx, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(colIdx+1, 1)
		_ = f.SetCellValue(sheet, cell, h)
	}

	now := time.Now()
	for rowIdx, r := range records {
		row := rowIdx + 2

		ts := r.Timestamp
		if ts.IsZero() {
			ts = now
		}

		_ = f.SetCellValue(sheet, fmt.Sprintf("A%d", row), r.AccountID)
		_ = f.SetCellValue(sheet, fmt.Sprintf("B%d", row), r.AccountName)
		_ = f.SetCellValue(sheet, fmt.Sprintf("C%d", row), r.Service)
		_ = f.SetCellValue(sheet, fmt.Sprintf("D%d", row), r.ResourceID)
		_ = f.SetCellValue(sheet, fmt.Sprintf("E%d", row), r.FindingType)
		_ = f.SetCellValue(sheet, fmt.Sprintf("F%d", row), r.Severity)
		_ = f.SetCellValue(sheet, fmt.Sprintf("G%d", row), r.Description)
		_ = f.SetCellValue(sheet, fmt.Sprintf("H%d", row), r.Remediation)
		_ = f.SetCellValue(sheet, fmt.Sprintf("I%d", row), ts.Format("2006-01-02 15:04:05"))
	}

	if len(records) > 0 {
		_ = f.AutoFilter(sheet, fmt.Sprintf("A1:I%d", len(records)+1), nil)
	}

	_ = f.SetColWidth(sheet, "A", "B", 18)
	_ = f.SetColWidth(sheet, "C", "D", 26)
	_ = f.SetColWidth(sheet, "E", "F", 16)
	_ = f.SetColWidth(sheet, "G", "H", 45)
	_ = f.SetColWidth(sheet, "I", "I", 20)

	dir := filepath.Dir(outputPath)
	if dir != "" && dir != "." {
		_ = os.MkdirAll(dir, 0755)
	}

	return f.SaveAs(outputPath)
}

// ExportCloudOpsToXLSX writes operational waste and hygiene findings to a native Excel workbook.
func ExportCloudOpsToXLSX(records []cloud.CloudOpsRecord, outputPath string) error {
	f := excelize.NewFile()
	defer f.Close()

	sheet := "CloudOps_Hygiene"
	_ = f.SetSheetName("Sheet1", sheet)

	headers := []string{
		"Account ID", "Account Name", "Service", "Resource ID",
		"Issue Type", "Resource Age (Days)", "Estimated Waste ($)", "Recommendation", "Recorded At",
	}

	headerStyle, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "#FFFFFF", Size: 11},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#375623"}, Pattern: 1}, // Forest green theme
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	if err == nil {
		_ = f.SetRowStyle(sheet, 1, 1, headerStyle)
	}

	for colIdx, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(colIdx+1, 1)
		_ = f.SetCellValue(sheet, cell, h)
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

		_ = f.SetCellValue(sheet, fmt.Sprintf("A%d", row), r.AccountID)
		_ = f.SetCellValue(sheet, fmt.Sprintf("B%d", row), r.AccountName)
		_ = f.SetCellValue(sheet, fmt.Sprintf("C%d", row), r.Service)
		_ = f.SetCellValue(sheet, fmt.Sprintf("D%d", row), r.ResourceID)
		_ = f.SetCellValue(sheet, fmt.Sprintf("E%d", row), r.IssueType)
		_ = f.SetCellValue(sheet, fmt.Sprintf("F%d", row), r.ResourceAge)
		_ = f.SetCellValue(sheet, fmt.Sprintf("G%d", row), r.EstimatedWaste)
		_ = f.SetCellValue(sheet, fmt.Sprintf("H%d", row), r.Recommendation)
		_ = f.SetCellValue(sheet, fmt.Sprintf("I%d", row), ts.Format("2006-01-02 15:04:05"))

		_ = f.SetCellStyle(sheet, fmt.Sprintf("G%d", row), fmt.Sprintf("G%d", row), currencyStyle)
	}

	if len(records) > 0 {
		_ = f.AutoFilter(sheet, fmt.Sprintf("A1:I%d", len(records)+1), nil)
	}

	_ = f.SetColWidth(sheet, "A", "B", 18)
	_ = f.SetColWidth(sheet, "C", "D", 26)
	_ = f.SetColWidth(sheet, "E", "F", 18)
	_ = f.SetColWidth(sheet, "G", "G", 18)
	_ = f.SetColWidth(sheet, "H", "H", 45)
	_ = f.SetColWidth(sheet, "I", "I", 20)

	dir := filepath.Dir(outputPath)
	if dir != "" && dir != "." {
		_ = os.MkdirAll(dir, 0755)
	}

	return f.SaveAs(outputPath)
}

// ExportMultiDomainWorkbook creates a unified, multi-sheet Executive Excel workbook covering all operational domains.
func ExportMultiDomainWorkbook(
	finops []cloud.CostRecord,
	secops []cloud.SecurityRecord,
	cloudops []cloud.CloudOpsRecord,
	enriched []cloud.EnrichedRecord,
	outputPath string,
) error {
	f := excelize.NewFile()
	defer f.Close()

	// 1. Executive Summary Sheet
	summarySheet := "Executive_Summary"
	_ = f.SetSheetName("Sheet1", summarySheet)

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

	_ = f.SetCellValue(summarySheet, "B2", "OpenLZ Landing Zone Operations — Executive Summary")
	_ = f.SetCellStyle(summarySheet, "B2", "B2", titleStyle)

	_ = f.SetCellValue(summarySheet, "B4", "Metric / KPI")
	_ = f.SetCellValue(summarySheet, "C4", "Value")
	_ = f.SetCellStyle(summarySheet, "B4", "C4", kpiHeaderStyle)

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

	_ = f.SetCellValue(summarySheet, "B5", "Total Tracked Cloud Spend")
	_ = f.SetCellValue(summarySheet, "C5", totalFinOpsSpend)
	_ = f.SetCellStyle(summarySheet, "C5", "C5", currencyStyle)

	_ = f.SetCellValue(summarySheet, "B6", "Identified Monthly Cost Savings")
	_ = f.SetCellValue(summarySheet, "C6", totalPotentialSavings)
	_ = f.SetCellStyle(summarySheet, "C6", "C6", currencyStyle)

	_ = f.SetCellValue(summarySheet, "B7", "Critical / High Security Findings")
	_ = f.SetCellValue(summarySheet, "C7", criticalSecOpsCount)
	_ = f.SetCellStyle(summarySheet, "C7", "C7", countStyle)

	_ = f.SetCellValue(summarySheet, "B8", "Estimated Orphaned Resource Waste")
	_ = f.SetCellValue(summarySheet, "C8", totalCloudOpsWaste)
	_ = f.SetCellStyle(summarySheet, "C8", "C8", currencyStyle)

	_ = f.SetColWidth(summarySheet, "B", "B", 35)
	_ = f.SetColWidth(summarySheet, "C", "C", 20)

	dir := filepath.Dir(outputPath)
	if dir != "" && dir != "." {
		_ = os.MkdirAll(dir, 0755)
	}

	return f.SaveAs(outputPath)
}
