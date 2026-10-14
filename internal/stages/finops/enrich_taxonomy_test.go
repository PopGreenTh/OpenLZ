package finops

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/PopGreenTh/OpenLZ/internal/cloud"
	"github.com/PopGreenTh/OpenLZ/internal/duckdb"
)

func TestEnrichMapsCostRowsOntoTaxonomy(t *testing.T) {
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

	row := func(day string, amount float64, svc, usage, resource string) cloud.CostRecord {
		return cloud.CostRecord{
			AccountID: "111122223333", AccountName: "lz-prod-payments",
			Service: svc, UsageDate: day, Amount: amount, Unit: "USD", Currency: "USD",
			CSP: "AWS", Region: "ap-southeast-1", UsageType: usage, ResourceID: resource,
			Timestamp: time.Now(),
		}
	}
	records := []cloud.CostRecord{
		row("2026-09-21", 40, "Amazon Elastic Compute Cloud - Compute", "APS1-BoxUsage:t3.medium", "i-0111122223333a001"),
		row("2026-09-22", 220, "Amazon Elastic Compute Cloud - Compute", "APS1-BoxUsage:t3.medium", "i-0111122223333a001"),
		row("2026-09-21", 10, "Amazon Simple Storage Service", "TimedStorage-ByteHrs", ""),
		row("2026-09-22", 150, "Amazon Simple Storage Service", "TimedStorage-ByteHrs", ""),
	}
	if err := engine.WriteFinOpsParquet(ctx, records, rawPath); err != nil {
		t.Fatalf("write raw: %v", err)
	}
	if _, err := engine.AnalyzeFinOps(ctx, rawPath, anomPath, 1.2); err != nil {
		t.Fatalf("analyze: %v", err)
	}

	res, err := ExecuteEnrichCostCenters(ctx, EnrichCostCentersInput{
		InputPath:    anomPath,
		OutputPath:   enrichPath,
		TaxonomyPath: filepath.Join("..", "..", "..", "taxonomy", "taxonomy.json"),
	})
	if err != nil {
		t.Fatalf("enrich: %v", err)
	}
	if res.RecordCount != 2 {
		t.Fatalf("expected 2 enriched rows, got %d", res.RecordCount)
	}

	got, err := engine.ReadEnrichedFromParquet(ctx, enrichPath)
	if err != nil {
		t.Fatalf("read enriched: %v", err)
	}
	byService := map[string]cloud.EnrichedRecord{}
	for _, r := range got {
		byService[r.Service] = r
	}

	ec2 := byService["Amazon Elastic Compute Cloud - Compute"]
	checks := map[string][2]string{
		"project":           {ec2.ProjectID, "PRJ-PAYMENTS"},
		"business_unit":     {ec2.BusinessUnitID, "BU-DIGITAL"},
		"application":       {ec2.ApplicationID, ""}, // account 111122223333 serves two apps
		"cost_center":       {ec2.CostCenterID, "CC-PAY"},
		"environment":       {ec2.EnvironmentID, ""}, // the legacy resolver ignores project_path
		"owner":             {ec2.OwnerID, "TEAM-PAY"},
		"owner_team":        {ec2.OwnerTeamID, "TEAM-PAY"},
		"customer":          {ec2.CustomerID, ""},
		"lz":                {ec2.LZID, "LZ-MAIN"},
		"billing_currency":  {ec2.BillingCurrency, "USD"},
		"csp":               {ec2.CSPID, "AWS"},
		"region":            {ec2.RegionID, "ap-southeast-1"},
		"service_category":  {ec2.ServiceCategoryID, "SC-COMPUTE"},
		"service":           {ec2.ServiceID, "AmazonEC2"},
		"service_usage":     {ec2.ServiceUsageID, "SU-EC2-INSTANCE"},
		"usage":             {ec2.UsageID, "BoxUsage:t3.medium"},
		"resource_group":    {ec2.ResourceGroupID, "RG-CHECKOUT"},
		"resource":          {ec2.ResourceID, "i-0111122223333a001"},
		"resource_or_key":   {ec2.ResourceOrKey, "i-0111122223333a001"},
	}
	for name, c := range checks {
		if c[0] != c[1] {
			t.Errorf("ec2 %s = %q, want %q", name, c[0], c[1])
		}
	}

	s3 := byService["Amazon Simple Storage Service"]
	if s3.ResourceGroupID != "" || s3.ResourceID != "" {
		t.Errorf("service-level row must not get a resource: %+v", s3)
	}
	// TEAM-PAY has two product lines, so the product line and customer stay empty.
	if s3.OwnerTeamID != "TEAM-PAY" || s3.ProductLineID != "" || s3.CustomerID != "" {
		t.Errorf("s3 owner team/product line/customer: %q %q %q", s3.OwnerTeamID, s3.ProductLineID, s3.CustomerID)
	}
	if s3.ServiceUsageID != "SU-S3-STORAGE" || s3.ResourceOrKey != "Amazon Simple Storage Service" {
		t.Errorf("s3 usage/resource key: %q %q", s3.ServiceUsageID, s3.ResourceOrKey)
	}
}

func TestZoneAndConnectionTravelFromRawToEnriched(t *testing.T) {
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

	rec := cloud.CostRecord{
		AccountID: "777788889999", AccountName: "lz-prod-data", ConnectionID: "100000000001",
		Service: "Amazon Simple Storage Service", UsageDate: "2026-09-22", Amount: 150, Unit: "USD", Currency: "USD",
		CSP: "AWS", Region: "ap-southeast-1", Zone: "ap-southeast-1b", UsageType: "TimedStorage-ByteHrs",
		Timestamp: time.Now(),
	}
	if err := engine.WriteFinOpsParquet(ctx, []cloud.CostRecord{rec}, rawPath); err != nil {
		t.Fatalf("write raw: %v", err)
	}
	if _, err := engine.AnalyzeFinOps(ctx, rawPath, anomPath, 1.2); err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if _, err := ExecuteEnrichCostCenters(ctx, EnrichCostCentersInput{
		InputPath: anomPath, OutputPath: enrichPath,
		TaxonomyPath: filepath.Join("..", "..", "..", "taxonomy", "taxonomy.json"),
	}); err != nil {
		t.Fatalf("enrich: %v", err)
	}
	got, err := engine.ReadEnrichedFromParquet(ctx, enrichPath)
	if err != nil || len(got) != 1 {
		t.Fatalf("read enriched: %v (%d rows)", err, len(got))
	}
	r := got[0]
	if r.ZoneID != "ap-southeast-1b" || r.Zone != "Singapore AZ b" || r.RegionID != "ap-southeast-1" {
		t.Errorf("zone/region: %+v", r)
	}
	if r.ConnectionID != "100000000001" || r.LZID != "LZ-MAIN" || r.BillingCurrency != "USD" {
		t.Errorf("connection/lz/currency: %q %q %q", r.ConnectionID, r.LZID, r.BillingCurrency)
	}
	if r.ApplicationID != "APP-LAKE" {
		t.Errorf("app: %q", r.ApplicationID)
	}
}
