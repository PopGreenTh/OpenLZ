package finops

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/PopGreenTh/OpenLZ/internal/cloud"
	"github.com/PopGreenTh/OpenLZ/internal/duckdb"
)

func TestEnrichMapsEmptyHierarchyTags(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	rawPath := filepath.Join(dir, "finops_raw.parquet")
	anomPath := filepath.Join(dir, "finops_anomalies.parquet")
	enrichPath := filepath.Join(dir, "finops_enriched.parquet")

	engine, err := duckdb.NewEngine()
	if err != nil {
		t.Fatalf("duckdb: %v", err)
	}
	defer engine.Close()

	records := []cloud.CostRecord{
		{
			AccountID: "100000000003", AccountName: "lz-prod-data",
			Service: "Amazon Simple Storage Service", UsageDate: "2026-09-21",
			Amount: 40, Unit: "USD", Currency: "USD", Timestamp: time.Now(),
		},
		{
			AccountID: "100000000003", AccountName: "lz-prod-data",
			Service: "Amazon Simple Storage Service", UsageDate: "2026-09-22",
			Amount: 220, Unit: "USD", Currency: "USD", Timestamp: time.Now(),
		},
	}
	if err := engine.WriteFinOpsParquet(ctx, records, rawPath); err != nil {
		t.Fatalf("write raw: %v", err)
	}
	if _, err := engine.AnalyzeFinOps(ctx, rawPath, anomPath, 1.2); err != nil {
		t.Fatalf("analyze: %v", err)
	}

	root := filepath.Join("..", "..", "..", "taxonomy")
	res, err := ExecuteEnrichCostCenters(ctx, EnrichCostCentersInput{
		InputPath:            anomPath,
		OutputPath:           enrichPath,
		ProjectTaxonomyPath:  filepath.Join(root, "project_taxonomy.json"),
		BusinessTaxonomyPath: filepath.Join(root, "business_taxonomy.json"),
	})
	if err != nil {
		t.Fatalf("enrich: %v", err)
	}
	if res.RecordCount == 0 {
		t.Fatal("expected enriched rows")
	}

	got, err := engine.ReadEnrichedFromParquet(ctx, enrichPath)
	if err != nil {
		t.Fatalf("read enriched: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 enriched row, got %d", len(got))
	}
	row := got[0]
	if row.ProjectID != "BU-DIGITAL" || row.BusinessUnitID != "BU-DIGITAL" || row.Project != row.BusinessUnit {
		t.Fatalf("project must behave as business unit: %+v", row)
	}
	if row.ApplicationID != "OWN-DATA" || row.OwnerID != "OWN-DATA" || row.Application != "Data Platform" {
		t.Fatalf("app must behave as owner: app=%s owner=%s", row.Application, row.Owner)
	}
	if row.EnvironmentID != "CC-DATA" || row.CostCenterID != "CC-DATA" || row.Environment != "Data Production" {
		t.Fatalf("environment must behave as cost center: env=%s cc=%s", row.Environment, row.CostCenter)
	}
	if row.ServiceCategory != "Storage" || row.CustomerFunction != "Lakehouse" {
		t.Fatalf("service map: category=%s func=%s", row.ServiceCategory, row.CustomerFunction)
	}
}
