package finops_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PopGreenTh/OpenLZ/internal/stages/finops"
	_ "github.com/marcboeker/go-duckdb"
)

func TestSenseCostExplorer_WithTags(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	rawPath := filepath.Join(tempDir, "finops_raw.parquet")
	cachePath := filepath.Join(tempDir, "cache.duckdb")

	res, err := finops.ExecuteSenseCostExplorer(ctx, finops.SenseCostExplorerInput{
		Accounts:    []string{"111122223333"},
		StartDate:   "2026-09-01",
		EndDate:     "2026-09-07",
		GroupByTags: []string{"BusinessUnit", "Project", "Application", "Environment"},
		OutputPath:  rawPath,
		CacheDBPath: cachePath,
		Mock:        true,
	})
	if err != nil {
		t.Fatalf("ExecuteSenseCostExplorer failed: %v", err)
	}
	if res.RecordCount == 0 || !res.Success {
		t.Fatalf("expected positive records, got %d", res.RecordCount)
	}

	// Validate DuckDB parquet schema and tag contents
	db, err := sql.Open("duckdb", "")
	if err != nil {
		t.Fatalf("failed opening duckdb: %v", err)
	}
	defer db.Close()

	rows, err := db.QueryContext(ctx, "SELECT primary_tag, secondary_tag FROM read_parquet(?)", rawPath)
	if err != nil {
		t.Fatalf("failed querying parquet with duckdb: %v", err)
	}
	defer rows.Close()

	rowCount := 0
	for rows.Next() {
		var pTag, sTag string
		if err := rows.Scan(&pTag, &sTag); err != nil {
			t.Fatalf("scan failed: %v", err)
		}
		if pTag == "" {
			t.Errorf("expected non-empty primary_tag")
		}
		if sTag == "" {
			t.Errorf("expected non-empty secondary_tag")
		}
		rowCount++
	}

	if rowCount == 0 {
		t.Fatalf("expected rows read from parquet, got 0")
	}
}

func TestSenseCostExplorer_PayerAccount(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	rawPath := filepath.Join(tempDir, "finops_payer_raw.parquet")
	cachePath := filepath.Join(tempDir, "cache_payer.duckdb")

	res, err := finops.ExecuteSenseCostExplorer(ctx, finops.SenseCostExplorerInput{
		PayerAccount: "999988887777",
		Accounts:     []string{"111122223333", "444455556666"},
		StartDate:    "2026-09-01",
		EndDate:      "2026-09-07",
		GroupByTags:  []string{"BusinessUnit", "Project", "Application", "Environment"},
		OutputPath:   rawPath,
		CacheDBPath:  cachePath,
		Mock:         true,
	})
	if err != nil {
		t.Fatalf("ExecuteSenseCostExplorer with PayerAccount failed: %v", err)
	}
	if res.RecordCount == 0 || !res.Success {
		t.Fatalf("expected positive records, got %d", res.RecordCount)
	}
}

func TestSenseCostExplorer_MinCostThreshold(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	rawPath := filepath.Join(tempDir, "finops_thresh_raw.parquet")
	cachePath := filepath.Join(tempDir, "cache_thresh.duckdb")

	// High threshold to filter out low-cost services
	res, err := finops.ExecuteSenseCostExplorer(ctx, finops.SenseCostExplorerInput{
		Accounts:         []string{"111122223333"},
		StartDate:        "2026-09-01",
		EndDate:          "2026-09-07",
		MinCostThreshold: 50.0,
		OutputPath:       rawPath,
		CacheDBPath:      cachePath,
		Mock:             true,
	})
	if err != nil {
		t.Fatalf("ExecuteSenseCostExplorer failed: %v", err)
	}

	db, err := sql.Open("duckdb", "")
	if err != nil {
		t.Fatalf("failed opening duckdb: %v", err)
	}
	defer db.Close()

	var minFound float64
	err = db.QueryRowContext(ctx, "SELECT MIN(amount) FROM read_parquet(?)", rawPath).Scan(&minFound)
	if err != nil {
		t.Fatalf("failed querying min amount: %v", err)
	}
	if minFound < 50.0 {
		t.Errorf("expected min amount >= 50.0, found %f", minFound)
	}
	_ = res
}

func TestSenseCostExplorer_UsageQuantity(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	rawPath := filepath.Join(tempDir, "finops_usage_raw.parquet")
	cachePath := filepath.Join(tempDir, "cache_usage.duckdb")

	res, err := finops.ExecuteSenseCostExplorer(ctx, finops.SenseCostExplorerInput{
		Accounts:    []string{"111122223333"},
		StartDate:   "2026-09-01",
		EndDate:     "2026-09-07",
		Metric:      "UsageQuantity",
		Frequency:   "DAILY",
		OutputPath:  rawPath,
		CacheDBPath: cachePath,
		Mock:        true,
	})
	if err != nil {
		t.Fatalf("ExecuteSenseCostExplorer with UsageQuantity failed: %v", err)
	}
	if res.RecordCount == 0 || !res.Success {
		t.Fatalf("expected positive records, got %d", res.RecordCount)
	}

	db, err := sql.Open("duckdb", "")
	if err != nil {
		t.Fatalf("failed opening duckdb: %v", err)
	}
	defer db.Close()

	rows, err := db.QueryContext(ctx, "SELECT service, metric, frequency, unit, amount, daily_amount, days_in_period FROM read_parquet(?)", rawPath)
	if err != nil {
		t.Fatalf("failed querying parquet: %v", err)
	}
	defer rows.Close()

	seenUnits := make(map[string]bool)
	count := 0
	for rows.Next() {
		var svc, metric, freq, unit string
		var amount, dailyAmount, daysInPeriod float64
		if err := rows.Scan(&svc, &metric, &freq, &unit, &amount, &dailyAmount, &daysInPeriod); err != nil {
			t.Fatalf("failed scanning row: %v", err)
		}
		if metric != "UsageQuantity" {
			t.Errorf("expected metric UsageQuantity, got %s", metric)
		}
		if freq != "DAILY" {
			t.Errorf("expected frequency DAILY, got %s", freq)
		}
		if unit == "" || unit == "USD" {
			t.Errorf("expected usage unit (e.g. Hours, GB-Mo, Requests), got %s", unit)
		}
		if amount <= 0 || dailyAmount <= 0 {
			t.Errorf("expected positive usage amount, got amount=%f, dailyAmount=%f", amount, dailyAmount)
		}
		if daysInPeriod != 1.0 {
			t.Errorf("expected daysInPeriod 1.0 for DAILY, got %f", daysInPeriod)
		}
		seenUnits[unit] = true
		count++
	}

	if count == 0 {
		t.Fatalf("no rows scanned")
	}
	if len(seenUnits) < 2 {
		t.Errorf("expected diverse usage units, saw: %v", seenUnits)
	}
}

func TestSenseCostExplorer_FrequencyComparability(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	frequencies := []string{"HOURLY", "DAILY", "MONTHLY"}
	paths := make(map[string]string)

	for _, freq := range frequencies {
		p := filepath.Join(tempDir, "finops_"+freq+".parquet")
		paths[freq] = p
		res, err := finops.ExecuteSenseCostExplorer(ctx, finops.SenseCostExplorerInput{
			Accounts:    []string{"111122223333"},
			StartDate:   "2026-09-01",
			EndDate:     "2026-09-07",
			Frequency:   freq,
			OutputPath:  p,
			CacheDBPath: filepath.Join(tempDir, "cache_"+freq+".duckdb"),
			Mock:        true,
		})
		if err != nil {
			t.Fatalf("failed executing for frequency %s: %v", freq, err)
		}
		if res.RecordCount == 0 || !res.Success {
			t.Fatalf("expected positive records for %s, got %d", freq, res.RecordCount)
		}
	}

	db, err := sql.Open("duckdb", "")
	if err != nil {
		t.Fatalf("failed opening duckdb: %v", err)
	}
	defer db.Close()

	// Verify HOURLY records
	hourlyRows, err := db.QueryContext(ctx, "SELECT usage_period, amount, daily_amount, days_in_period, frequency FROM read_parquet(?)", paths["HOURLY"])
	if err != nil {
		t.Fatalf("failed querying HOURLY parquet: %v", err)
	}
	defer hourlyRows.Close()

	hourlyCount := 0
	for hourlyRows.Next() {
		var period, freq string
		var amount, dailyAmount, daysInPeriod float64
		if err := hourlyRows.Scan(&period, &amount, &dailyAmount, &daysInPeriod, &freq); err != nil {
			t.Fatalf("scan hourly failed: %v", err)
		}
		if freq != "HOURLY" {
			t.Errorf("expected HOURLY frequency, got %s", freq)
		}
		// In HOURLY: dailyAmount should be amount * 24
		diff := dailyAmount - (amount * 24.0)
		if diff < -0.001 || diff > 0.001 {
			t.Errorf("hourly daily_amount (%f) does not match amount*24 (%f)", dailyAmount, amount*24.0)
		}
		if daysInPeriod < 0.04 || daysInPeriod > 0.043 {
			t.Errorf("hourly days_in_period expected ~0.04167, got %f", daysInPeriod)
		}
		hourlyCount++
	}
	if hourlyCount == 0 {
		t.Fatalf("no hourly records found")
	}

	// Verify MONTHLY records
	monthlyRows, err := db.QueryContext(ctx, "SELECT usage_period, amount, daily_amount, days_in_period, frequency FROM read_parquet(?)", paths["MONTHLY"])
	if err != nil {
		t.Fatalf("failed querying MONTHLY parquet: %v", err)
	}
	defer monthlyRows.Close()

	monthlyCount := 0
	for monthlyRows.Next() {
		var period, freq string
		var amount, dailyAmount, daysInPeriod float64
		if err := monthlyRows.Scan(&period, &amount, &dailyAmount, &daysInPeriod, &freq); err != nil {
			t.Fatalf("scan monthly failed: %v", err)
		}
		if freq != "MONTHLY" {
			t.Errorf("expected MONTHLY frequency, got %s", freq)
		}
		// In MONTHLY: dailyAmount should be amount / daysInPeriod
		diff := dailyAmount - (amount / daysInPeriod)
		if diff < -0.001 || diff > 0.001 {
			t.Errorf("monthly daily_amount (%f) does not match amount/daysInPeriod (%f)", dailyAmount, amount/daysInPeriod)
		}
		if daysInPeriod != 30.0 {
			t.Errorf("monthly days_in_period expected 30.0, got %f", daysInPeriod)
		}
		monthlyCount++
	}
	if monthlyCount == 0 {
		t.Fatalf("no monthly records found")
	}
}

func TestSenseCostExplorer_DefaultMockAccounts(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	rawPath := filepath.Join(tempDir, "finops_default_raw.parquet")
	cachePath := filepath.Join(tempDir, "cache_default.duckdb")

	// Calling with empty Accounts in mock mode should auto-populate mock accounts without error
	res, err := finops.ExecuteSenseCostExplorer(ctx, finops.SenseCostExplorerInput{
		Accounts:    nil,
		OutputPath:  rawPath,
		CacheDBPath: cachePath,
		Mock:        true,
	})
	if err != nil {
		t.Fatalf("ExecuteSenseCostExplorer with empty accounts failed: %v", err)
	}
	if !res.Success || res.RecordCount == 0 {
		t.Fatalf("expected successful execution with positive record count, got %d", res.RecordCount)
	}
}

func TestSenseCostExplorer_MixedDimensions(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	rawPath := filepath.Join(tempDir, "finops_mixed_raw.parquet")
	cachePath := filepath.Join(tempDir, "cache_mixed.duckdb")

	res, err := finops.ExecuteSenseCostExplorer(ctx, finops.SenseCostExplorerInput{
		PayerAccount: "225989366329",
		GroupBy:      "LINKED_ACCOUNT_SERVICE",
		OutputPath:   rawPath,
		CacheDBPath:  cachePath,
		Mock:         true,
	})
	if err != nil {
		t.Fatalf("ExecuteSenseCostExplorer with LINKED_ACCOUNT_SERVICE failed: %v", err)
	}
	if !res.Success || res.RecordCount == 0 {
		t.Fatalf("expected successful execution with positive record count, got %d", res.RecordCount)
	}

	db, err := sql.Open("duckdb", "")
	if err != nil {
		t.Fatalf("failed opening duckdb: %v", err)
	}
	defer db.Close()

	rows, err := db.QueryContext(ctx, "SELECT account_id, service, primary_tag, secondary_tag FROM read_parquet(?)", rawPath)
	if err != nil {
		t.Fatalf("failed querying parquet: %v", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var accID, svc, pTag, sTag string
		if err := rows.Scan(&accID, &svc, &pTag, &sTag); err != nil {
			t.Fatalf("scan failed: %v", err)
		}
		if accID == "" {
			t.Errorf("expected non-empty account_id")
		}
		if svc == "" || svc == "Unknown" {
			t.Errorf("expected valid service name, got '%s'", svc)
		}
		count++
	}
	if count == 0 {
		t.Fatalf("expected records, got 0")
	}
}

func TestSenseCostExplorer_Hierarchical(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	rawPath := filepath.Join(tempDir, "finops_hierarchical_raw.parquet")
	cachePath := filepath.Join(tempDir, "cache_hierarchical.duckdb")

	res, err := finops.ExecuteSenseCostExplorer(ctx, finops.SenseCostExplorerInput{
		PayerAccount:     "225989366329",
		GroupBy:          "HIERARCHICAL",
		PrimaryTag:       "Project",
		SecondaryTag:     "Application",
		MinCostThreshold: 0.50,
		OutputPath:       rawPath,
		CacheDBPath:      cachePath,
		Mock:             true,
	})
	if err != nil {
		t.Fatalf("ExecuteSenseCostExplorer with HIERARCHICAL failed: %v", err)
	}
	if !res.Success || res.RecordCount == 0 {
		t.Fatalf("expected successful execution with positive record count, got %d", res.RecordCount)
	}

	db, err := sql.Open("duckdb", "")
	if err != nil {
		t.Fatalf("failed opening duckdb: %v", err)
	}
	defer db.Close()

	rows, err := db.QueryContext(ctx, "SELECT account_id, account_name, service, primary_tag, secondary_tag FROM read_parquet(?)", rawPath)
	if err != nil {
		t.Fatalf("failed querying parquet: %v", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var accID, accName, svc, pTag, sTag string
		if err := rows.Scan(&accID, &accName, &svc, &pTag, &sTag); err != nil {
			t.Fatalf("scan failed: %v", err)
		}
		if accID == "" {
			t.Errorf("expected non-empty account_id")
		}
		if accName == "" || strings.HasPrefix(accName, "Account-") {
			t.Errorf("expected proper account_name without placeholder prefix, got '%s'", accName)
		}
		if svc == "" || svc == "Unknown" {
			t.Errorf("expected valid service name, got '%s'", svc)
		}
		if pTag == "" || pTag == "Untagged" {
			t.Errorf("expected valid primary_tag, got '%s'", pTag)
		}
		if sTag == "" || sTag == "Untagged" {
			t.Errorf("expected valid secondary_tag, got '%s'", sTag)
		}
		if pTag == sTag {
			t.Errorf("primary_tag and secondary_tag should not be identical: got '%s'", pTag)
		}
		count++
	}
	if count == 0 {
		t.Fatalf("expected records, got 0")
	}

	// Verify zero duplicate records exist in the Parquet dataset
	dupRows, err := db.QueryContext(ctx, `
		SELECT account_id, service, primary_tag, secondary_tag, usage_date, count(*)
		FROM read_parquet(?)
		GROUP BY account_id, service, primary_tag, secondary_tag, usage_date
		HAVING count(*) > 1
	`, rawPath)
	if err != nil {
		t.Fatalf("failed checking for duplicate records: %v", err)
	}
	defer dupRows.Close()

	if dupRows.Next() {
		var acc, svc, p, s, dt string
		var cnt int
		_ = dupRows.Scan(&acc, &svc, &p, &s, &dt, &cnt)
		t.Fatalf("detected duplicate records in parquet: account=%s service=%s primary_tag=%s secondary_tag=%s date=%s count=%d", acc, svc, p, s, dt, cnt)
	}
}

func TestExecuteSenseCostExplorer_MultiFormatOutput(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "openlz_multiformat_*")
	if err != nil {
		t.Fatalf("failed creating temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	ctx := context.Background()
	outBase := filepath.Join(tempDir, "finops_test_raw.parquet")

	res, err := finops.ExecuteSenseCostExplorer(ctx, finops.SenseCostExplorerInput{
		Accounts:    []string{"111122223333"},
		Format:      "all",
		OutputPath:  outBase,
		CacheDBPath: filepath.Join(tempDir, "cache.duckdb"),
		Mock:        true,
	})
	if err != nil {
		t.Fatalf("ExecuteSenseCostExplorer with format=all failed: %v", err)
	}
	if !res.Success || res.RecordCount == 0 {
		t.Fatalf("expected positive record count, got %d", res.RecordCount)
	}

	parquetFile := filepath.Join(tempDir, "finops_test_raw.parquet")
	csvFile := filepath.Join(tempDir, "finops_test_raw.csv")
	excelMFile := filepath.Join(tempDir, "finops_test_raw_excel.m")
	powerbiMFile := filepath.Join(tempDir, "finops_test_raw_powerbi.m")

	if _, err := os.Stat(parquetFile); os.IsNotExist(err) {
		t.Errorf("expected parquet file %s to exist", parquetFile)
	}
	if _, err := os.Stat(csvFile); os.IsNotExist(err) {
		t.Errorf("expected csv file %s to exist", csvFile)
	}
	if _, err := os.Stat(excelMFile); os.IsNotExist(err) {
		t.Errorf("expected excel .m file %s to exist", excelMFile)
	}
	if _, err := os.Stat(powerbiMFile); os.IsNotExist(err) {
		t.Errorf("expected powerbi .m file %s to exist", powerbiMFile)
	}

	// Verify CSV contents have header
	csvBytes, err := os.ReadFile(csvFile)
	if err != nil {
		t.Fatalf("failed reading csv: %v", err)
	}
	csvContent := string(csvBytes)
	if !strings.Contains(csvContent, "account_id") || !strings.Contains(csvContent, "recorded_at") {
		t.Errorf("csv missing expected column headers: %s", csvContent[:min(100, len(csvContent))])
	}

	// Also test bare path without extension + "excel,par" aliases
	bareBase := filepath.Join(tempDir, "finops_bare")
	resBare, err := finops.ExecuteSenseCostExplorer(ctx, finops.SenseCostExplorerInput{
		Accounts:    []string{"111122223333"},
		Format:      "excel,par",
		OutputPath:  bareBase, // Notice: No extension specified!
		CacheDBPath: filepath.Join(tempDir, "cache2.duckdb"),
		Mock:        true,
	})
	if err != nil {
		t.Fatalf("ExecuteSenseCostExplorer with bare path failed: %v", err)
	}
	if !resBare.Success || resBare.RecordCount == 0 {
		t.Fatalf("expected positive record count for bare path, got %d", resBare.RecordCount)
	}

	// Verify all expected extensions are automatically appended to bare path
	if _, err := os.Stat(filepath.Join(tempDir, "finops_bare.parquet")); os.IsNotExist(err) {
		t.Errorf("expected finops_bare.parquet to be created automatically")
	}
	if _, err := os.Stat(filepath.Join(tempDir, "finops_bare.csv")); os.IsNotExist(err) {
		t.Errorf("expected finops_bare.csv to be created automatically")
	}
	if _, err := os.Stat(filepath.Join(tempDir, "finops_bare_excel.m")); os.IsNotExist(err) {
		t.Errorf("expected finops_bare_excel.m to be created automatically")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

