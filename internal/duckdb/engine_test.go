package duckdb

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/PopGreenTh/OpenLZ/internal/cloud"
)

func TestDuckDBEngineFinOps(t *testing.T) {
	tempDir := t.TempDir()
	rawParquet := filepath.Join(tempDir, "finops_raw.parquet")
	anomParquet := filepath.Join(tempDir, "finops_anomalies.parquet")
	enrichParquet := filepath.Join(tempDir, "finops_enriched.parquet")
	csvOut := filepath.Join(tempDir, "finops.csv")

	engine, err := NewEngine()
	if err != nil {
		t.Fatalf("Failed to initialize duckdb engine: %v", err)
	}
	defer engine.Close()

	ctx := context.Background()

	// 1. Ingest FinOps records
	records := []cloud.CostRecord{
		{AccountID: "111122223333", AccountName: "Prod", Service: "AmazonEC2", UsageDate: "2026-09-21", Amount: 100.0, Unit: "USD", Currency: "USD", Timestamp: time.Now()},
		{AccountID: "111122223333", AccountName: "Prod", Service: "AmazonEC2", UsageDate: "2026-09-22", Amount: 350.0, Unit: "USD", Currency: "USD", Timestamp: time.Now()},
	}

	if err := engine.WriteFinOpsParquet(ctx, records, rawParquet); err != nil {
		t.Fatalf("WriteFinOpsParquet failed: %v", err)
	}

	if _, err := os.Stat(rawParquet); os.IsNotExist(err) {
		t.Fatalf("raw parquet file not found")
	}

	// 2. Anomaly Analysis
	anomalies, err := engine.AnalyzeFinOps(ctx, rawParquet, anomParquet, 1.3)
	if err != nil {
		t.Fatalf("AnalyzeFinOps failed: %v", err)
	}
	if len(anomalies) == 0 {
		t.Fatalf("Expected at least 1 anomaly, found 0")
	}

	// 3. Enrichment
	enriched, err := engine.EnrichRecords(ctx, anomParquet, enrichParquet)
	if err != nil {
		t.Fatalf("EnrichRecords failed: %v", err)
	}
	if len(enriched) == 0 {
		t.Fatalf("Expected enriched records, found 0")
	}

	// 4. Export to CSV
	if err := engine.ExportToCSV(ctx, enrichParquet, csvOut); err != nil {
		t.Fatalf("ExportToCSV failed: %v", err)
	}
	if _, err := os.Stat(csvOut); os.IsNotExist(err) {
		t.Fatalf("CSV file not created")
	}
}

func TestDuckDBEngine_TagColumns(t *testing.T) {
	tempDir := t.TempDir()
	parquetPath := filepath.Join(tempDir, "finops_tags.parquet")

	engine, err := NewEngine()
	if err != nil {
		t.Fatalf("Failed to initialize duckdb engine: %v", err)
	}
	defer engine.Close()

	ctx := context.Background()

	records := []cloud.CostRecord{
		{
			AccountID:    "111122223333",
			AccountName:  "Production",
			Service:      "AmazonEC2",
			UsageDate:    "2026-09-23",
			Amount:       50.0,
			PrimaryTag:   "Alpha",
			SecondaryTag: "Storefront",
			Tags: map[string]string{
				"Department": "Engineering",
				"ManagedBy":  "Terraform",
			},
			Timestamp: time.Now(),
		},
	}

	if err := engine.WriteFinOpsParquet(ctx, records, parquetPath); err != nil {
		t.Fatalf("WriteFinOpsParquet failed: %v", err)
	}

	// 1. Query primary_tag and secondary_tag directly from Parquet
	row := engine.db.QueryRowContext(ctx, `
		SELECT primary_tag, secondary_tag 
		FROM read_parquet(?)
	`, parquetPath)

	var pTag, sTag string
	if err := row.Scan(&pTag, &sTag); err != nil {
		t.Fatalf("failed scanning tag columns from parquet: %v", err)
	}

	if pTag != "Alpha" || sTag != "Storefront" {
		t.Errorf("unexpected tags: primary_tag=%s, secondary_tag=%s", pTag, sTag)
	}

	// 2. Verify that application, project, and tags columns do NOT exist in the schema
	rows, err := engine.db.QueryContext(ctx, fmt.Sprintf("DESCRIBE SELECT * FROM read_parquet('%s');", cleanPath(parquetPath)))
	if err != nil {
		t.Fatalf("DESCRIBE failed: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var colName, colType, null, key, dflt, extra sql.NullString
		_ = rows.Scan(&colName, &colType, &null, &key, &dflt, &extra)
		name := strings.ToLower(colName.String)
		if name == "application" || name == "project" || name == "tags" {
			t.Errorf("column %s should be removed from parquet schema", name)
		}
	}
}

func TestInspectCurrentParquet(t *testing.T) {
	parquetPath := filepath.Join("..", "..", "data", "finops_raw.parquet")
	if _, err := os.Stat(parquetPath); os.IsNotExist(err) {
		t.Skip("data/finops_raw.parquet does not exist")
	}

	engine, err := NewEngine()
	if err != nil {
		t.Fatalf("Failed to initialize duckdb engine: %v", err)
	}
	defer engine.Close()

	ctx := context.Background()

	// 1. Describe table
	rows, err := engine.db.QueryContext(ctx, fmt.Sprintf("DESCRIBE SELECT * FROM read_parquet('%s');", cleanPath(parquetPath)))
	if err != nil {
		t.Fatalf("DESCRIBE failed: %v", err)
	}
	defer rows.Close()

	t.Log("=== COLUMNS IN data/finops_raw.parquet ===")
	for rows.Next() {
		var colName, colType, null, key, dflt, extra sql.NullString
		_ = rows.Scan(&colName, &colType, &null, &key, &dflt, &extra)
		t.Logf("Column: %-25s Type: %s", colName.String, colType.String)
	}

	// 2. Distinct primary_tag, secondary_tag
	rows2, err := engine.db.QueryContext(ctx, fmt.Sprintf("SELECT primary_tag, secondary_tag, count(*) FROM read_parquet('%s') GROUP BY primary_tag, secondary_tag ORDER BY count(*) DESC LIMIT 20;", cleanPath(parquetPath)))
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	defer rows2.Close()

	t.Log("\n=== SAMPLE DISTINCT (primary_tag, secondary_tag, count) ===")
	for rows2.Next() {
		var pTag, sTag string
		var cnt int
		_ = rows2.Scan(&pTag, &sTag, &cnt)
		t.Logf("primary_tag=%-25s secondary_tag=%-25s count=%d", pTag, sTag, cnt)
	}
}

