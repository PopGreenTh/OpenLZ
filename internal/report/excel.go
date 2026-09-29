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

	// Header style: Bold, navy fill, white text
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

	// Number formatting style for currency/amounts
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

	// Auto-filter across all columns
	if len(records) > 0 {
		_ = f.AutoFilter(sheet, fmt.Sprintf("A1:Q%d", len(records)+1), nil)
	}

	// Set clean readable column widths
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
