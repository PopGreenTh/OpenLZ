package finops

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/PopGreenTh/OpenLZ/internal/cloud"
	"github.com/PopGreenTh/OpenLZ/internal/duckdb"
)

func TestEnrichLegacyResolverMapsCostRowsOntoTaxonomy(t *testing.T) {
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
		InputPath:      anomPath,
		OutputPath:     enrichPath,
		TaxonomyPath:   filepath.Join("..", "..", "..", "taxonomy", "taxonomy.json"),
		LegacyResolver: true,
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
		"project":          {ec2.ProjectID, "PRJ-PAYMENTS"},
		"business_unit":    {ec2.BusinessUnitID, "BU-DIGITAL"},
		"application":      {ec2.ApplicationID, ""}, // account 111122223333 serves two apps
		"cost_center":      {ec2.CostCenterID, "CC-PAY"},
		"environment":      {ec2.EnvironmentID, ""}, // the legacy resolver ignores project_path
		"owner":            {ec2.OwnerID, "TEAM-PAY"},
		"owner_team":       {ec2.OwnerTeamID, "TEAM-PAY"},
		"customer":         {ec2.CustomerID, ""},
		"lz":               {ec2.LZID, "LZ-MAIN"},
		"billing_currency": {ec2.BillingCurrency, "USD"},
		"csp":              {ec2.CSPID, "AWS"},
		"region":           {ec2.RegionID, "ap-southeast-1"},
		"service_category": {ec2.ServiceCategoryID, "SC-COMPUTE"},
		"service":          {ec2.ServiceID, "AmazonEC2"},
		"service_usage":    {ec2.ServiceUsageID, "SU-EC2-INSTANCE"},
		"usage":            {ec2.UsageID, "BoxUsage:t3.medium"},
		"resource_group":   {ec2.ResourceGroupID, "RG-CHECKOUT"},
		"resource":         {ec2.ResourceID, "i-0111122223333a001"},
		"resource_or_key":  {ec2.ResourceOrKey, "i-0111122223333a001"},
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

// enrichSample senses the given rows, analyzes them and enriches them against the sample taxonomy.
func enrichSample(t *testing.T, records []cloud.CostRecord) map[string]cloud.EnrichedRecord {
	t.Helper()
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
	if err := engine.WriteFinOpsParquet(ctx, records, rawPath); err != nil {
		t.Fatalf("write raw: %v", err)
	}
	if _, err := engine.AnalyzeFinOps(ctx, rawPath, anomPath, 1.0); err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if _, err := ExecuteEnrichCostCenters(ctx, EnrichCostCentersInput{
		InputPath: anomPath, OutputPath: enrichPath,
		TaxonomyPath: filepath.Join("..", "..", "..", "taxonomy", "taxonomy.json"),
	}); err != nil {
		t.Fatalf("enrich: %v", err)
	}
	got, err := engine.ReadEnrichedFromParquet(ctx, enrichPath)
	if err != nil {
		t.Fatalf("read enriched: %v", err)
	}
	out := map[string]cloud.EnrichedRecord{}
	for _, r := range got {
		out[r.AccountID+"|"+r.ResourceOrKey] = r
	}
	return out
}

func TestEnrichAttributesByTagsEvidenceAndPadding(t *testing.T) {
	base := func(acc, name, svc, resource, project, app string) cloud.CostRecord {
		return cloud.CostRecord{
			AccountID: acc, AccountName: name, Service: svc, UsageDate: "2026-09-22", Amount: 150,
			Unit: "USD", Currency: "USD", CSP: "AWS", Region: "ap-southeast-1", UsageType: "BoxUsage:t3.medium",
			ResourceID: resource, PrimaryTag: project, SecondaryTag: app, Timestamp: time.Now(),
		}
	}
	got := enrichSample(t, []cloud.CostRecord{
		// Resource group RG-CHECKOUT maps to a Feature path.
		base("111122223333", "prod", "ec2", "i-0111122223333a001", "Untagged", "Untagged"),
		// Shared account, untagged: both apps share the Project, so only the Project survives.
		base("111122223333", "prod", "s3", "", "Untagged", "Untagged"),
		// Unknown account, untagged, no evidence at all: nothing resolves.
		withRegion(base("999999999999", "mystery", "sqs", "", "Untagged", "Untagged"), "eu-west-1"),
		// Shared account, one valid App tag: the tag wins and the other app is dropped.
		base("111122223333", "prod", "lambda", "", "Untagged", "APP-ORDERS"),
		// Dedicated account: App from the account, Environment from the Region.
		base("777788889999", "data", "glue", "", "Untagged", "Untagged"),
		// Dedicated staging account, no tags, a region the taxonomy does not know: the account
		// lives in one environment.
		withRegion(base("444455556666", "stg", "cloudwatch", "", "Untagged", "Untagged"), "eu-west-1"),
		// Resource override to ENV-STG of the gateway, whatever the tags say.
		base("111122223333", "prod", "rds", "arn:aws:rds:ap-southeast-1:111122223333:db:inventory", "PRJ-LAKEHOUSE", "APP-LAKE"),
	})
	check := func(key, project, trailPart string) {
		t.Helper()
		r, ok := got[key]
		if !ok {
			t.Fatalf("missing row %q in %v", key, keys(got))
		}
		hash := "|" + r.ProjectID + "|" + r.ApplicationID + "|" + r.EnvironmentID + "|" + r.FeatureID + "|" + r.FunctionID + "|"
		if hash != project {
			t.Errorf("%s: project path %s, want %s", key, hash, project)
		}
		if trailPart != "" && !contains(r.AttributionTrail, trailPart) {
			t.Errorf("%s: trail %q is missing %q", key, r.AttributionTrail, trailPart)
		}
	}
	check("111122223333|i-0111122223333a001", "|PRJ-PAYMENTS|APP-GATEWAY|ENV-PROD|FEAT-CARD|FUNC-UNSPEC|", "rg")
	check("111122223333|s3", "|PRJ-PAYMENTS|APP-UNSPEC|ENV-UNSPEC|FEAT-UNSPEC|FUNC-UNSPEC|", "account")
	check("999999999999|sqs", "|PRJ-UNALLOC|APP-UNALLOC|ENV-UNALLOC|FEAT-UNALLOC|FUNC-UNALLOC|", "")
	check("111122223333|lambda", "|PRJ-PAYMENTS|APP-ORDERS|ENV-PROD|FEAT-UNSPEC|FUNC-UNSPEC|", "tags")
	check("777788889999|glue", "|PRJ-LAKEHOUSE|APP-LAKE|ENV-PROD|FEAT-UNSPEC|FUNC-UNSPEC|", "acctenv")
	check("444455556666|cloudwatch", "|PRJ-PAYMENTS|APP-GATEWAY|ENV-STG|FEAT-UNSPEC|FUNC-UNSPEC|", "acctenv")
	check("111122223333|arn:aws:rds:ap-southeast-1:111122223333:db:inventory", "|PRJ-PAYMENTS|APP-GATEWAY|ENV-STG|FEAT-UNSPEC|FUNC-UNSPEC|", "override")

	// Biz path derives from the Project path.
	if r := got["111122223333|lambda"]; r.BusinessUnitID != "BU-DIGITAL" || r.CostCenterID != "CC-PAY" || r.OwnerTeamID != "TEAM-PAY" ||
		r.ProductLineID != "PL-UNSPEC" || r.CustomerID != "CUST-UNSPEC" || r.OwnerID != "TEAM-PAY" {
		t.Errorf("biz path of lambda: %+v", r)
	}
	if r := got["999999999999|sqs"]; r.BusinessUnitID != "BU-UNALLOC" || r.ProjectID != "PRJ-UNALLOC" || r.Project != "Unallocated" {
		t.Errorf("unallocated row: %+v", r)
	}
	if r := got["111122223333|i-0111122223333a001"]; r.Feature != "Card Payments" || r.ProductLineID != "PL-CHECKOUT" {
		t.Errorf("feature name / product line: %q %q", r.Feature, r.ProductLineID)
	}
	// Cloud side stays resolved.
	if r := got["777788889999|glue"]; r.LZID != "LZ-MAIN" || r.CSPID != "AWS" || r.AccountID != "777788889999" {
		t.Errorf("cloud side: %+v", r)
	}
}

func withRegion(r cloud.CostRecord, region string) cloud.CostRecord {
	r.Region = region
	return r
}

func keys(m map[string]cloud.EnrichedRecord) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}

func contains(s, sub string) bool { return strings.Contains(s, sub) }

func TestAttributeLinesStageIsIdempotent(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	rawPath := filepath.Join(dir, "finops_raw.parquet")
	engine, err := duckdb.NewEngine()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	rec := cloud.CostRecord{
		AccountID: "777788889999", AccountName: "lz-prod-data", Service: "Amazon Simple Storage Service",
		UsageDate: "2026-09-22", Amount: 150, Unit: "USD", Currency: "USD", CSP: "AWS",
		Region: "ap-southeast-1", UsageType: "TimedStorage-ByteHrs", Timestamp: time.Now(),
	}
	if err := engine.WriteFinOpsParquet(ctx, []cloud.CostRecord{rec}, rawPath); err != nil {
		t.Fatal(err)
	}
	in := AttributeLinesInput{
		InputPath:    rawPath,
		TaxonomyPath: filepath.Join("..", "..", "..", "taxonomy", "taxonomy.json"),
		StorePath:    filepath.Join(dir, "sub", "attribution.duckdb"),
	}
	first, err := ExecuteAttributeLines(ctx, in)
	if err != nil || first.RecordCount != 1 {
		t.Fatalf("first run: %v %+v", err, first)
	}
	second, err := ExecuteAttributeLines(ctx, in)
	if err != nil || second.RecordCount != 0 || !strings.Contains(second.Message, "0 rows written") {
		t.Fatalf("second run must write nothing: %v %+v", err, second)
	}
}
