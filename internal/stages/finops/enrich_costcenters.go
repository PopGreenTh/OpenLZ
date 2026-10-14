package finops

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/PopGreenTh/OpenLZ/internal/cloud"
	"github.com/PopGreenTh/OpenLZ/internal/duckdb"
	types "github.com/PopGreenTh/OpenLZ/internal/stages/types"
	"github.com/PopGreenTh/OpenLZ/internal/taxonomy"
)

// EnrichCostCentersInput models inputs for finops-enrich-costcenters.
type EnrichCostCentersInput struct {
	InputPath    string `json:"inputPath"`
	OutputPath   string `json:"outputPath"`
	TaxonomyPath string `json:"taxonomyPath"` // Defaults to taxonomy/taxonomy.json when present
}

// ExecuteEnrichCostCenters runs Stage 3: Annotate with Landing Zone Cost Centers & Metadata.
// When a taxonomy snapshot is present, every cost row is mapped onto the business and
// project cost hierarchies; otherwise the built-in heuristic enrichment is used.
func ExecuteEnrichCostCenters(ctx context.Context, in EnrichCostCentersInput) (*types.StageResult, error) {
	if in.InputPath == "" {
		in.InputPath = "data/finops_anomalies.parquet"
	}
	in.InputPath = types.ResolveAbsolutePath(in.InputPath)
	if in.OutputPath == "" {
		in.OutputPath = "data/finops_enriched.parquet"
	}
	in.OutputPath = types.ResolveAbsolutePath(in.OutputPath)

	engine, err := duckdb.NewEngine()
	if err != nil {
		return nil, fmt.Errorf("duckdb init failed: %w", err)
	}
	defer engine.Close()

	catalog, err := taxonomy.LoadOptional(in.TaxonomyPath)
	if err != nil {
		return nil, err
	}
	if catalog == nil {
		enriched, err := engine.EnrichRecords(ctx, in.InputPath, in.OutputPath)
		if err != nil {
			return nil, err
		}
		return &types.StageResult{
			Stage:       types.StageFinOpsEnrichCostCenters,
			Success:     true,
			RecordCount: len(enriched),
			OutputPath:  in.OutputPath,
			Message:     fmt.Sprintf("Enriched %d records with cost centers into %s", len(enriched), in.OutputPath),
		}, nil
	}
	for _, w := range catalog.Warnings() {
		slog.WarnContext(ctx, "Cost taxonomy warning", "warning", w)
	}

	rows, err := engine.ReadAnalyzedCostRows(ctx, in.InputPath)
	if err != nil {
		return nil, err
	}
	enriched := make([]cloud.EnrichedRecord, 0, len(rows))
	var regionCSP, zoneRegion int
	for _, row := range rows {
		rec, res := mapHierarchy(row, catalog)
		if res.RegionCSPMismatch {
			regionCSP++
		}
		if res.ZoneRegionMismatch {
			zoneRegion++
		}
		enriched = append(enriched, rec)
	}
	if regionCSP > 0 {
		slog.WarnContext(ctx, "Cost rows whose Region CSP differs from their LZ CSP", "rows", regionCSP)
	}
	if zoneRegion > 0 {
		slog.WarnContext(ctx, "Cost rows whose Zone belongs to another Region", "rows", zoneRegion)
	}
	if err := engine.WriteEnrichedParquet(ctx, enriched, in.OutputPath); err != nil {
		return nil, err
	}

	return &types.StageResult{
		Stage:       types.StageFinOpsEnrichCostCenters,
		Success:     true,
		RecordCount: len(enriched),
		OutputPath:  in.OutputPath,
		Message:     fmt.Sprintf("Enriched %d records with cost taxonomy into %s", len(enriched), in.OutputPath),
	}, nil
}

// mapHierarchy resolves one analyzed cost row against the taxonomy. Levels the taxonomy
// cannot resolve stay empty so the gaps are visible and can be fixed in Directus.
func mapHierarchy(row duckdb.AnalyzedCostRow, catalog *taxonomy.Catalog) (cloud.EnrichedRecord, taxonomy.Output) {
	res := catalog.Resolve(taxonomy.Input{
		AccountID:    row.AccountID,
		AccountName:  row.AccountName,
		ConnectionID: row.ConnectionID,
		CSP:          row.CSP,
		Region:       row.Region,
		Zone:         row.Zone,
		Service:      row.Service,
		UsageType:    row.UsageType,
		ResourceID:   row.ResourceID,
		Project:      row.PrimaryTag,
		App:          row.SecondaryTag,
	})

	rec := cloud.EnrichedRecord{
		OpsDomain:      row.OpsDomain,
		AccountID:      row.AccountID,
		AccountName:    row.AccountName,
		BusinessUnitID: res.BusinessUnit.ID,
		BusinessUnit:   res.BusinessUnit.Name,
		CostCenterID:   res.CostCenter.ID,
		CostCenter:     res.CostCenter.Name,
		// owner/owner_id keep their meaning for reports and Power Query: they hold the OwnerTeam.
		OwnerID:       res.OwnerTeam.ID,
		Owner:         res.OwnerTeam.Name,
		OwnerTeamID:   res.OwnerTeam.ID,
		OwnerTeam:     res.OwnerTeam.Name,
		ProductLineID: res.ProductLine.ID,
		ProductLine:   res.ProductLine.Name,
		CustomerID:    res.Customer.ID,
		Customer:      res.Customer.Name,
		// customer_function* are the pre-v3 names of Customer.
		CustomerFunctionID: res.Customer.ID,
		CustomerFunction:   res.Customer.Name,
		FeatureID:          res.Feature.ID,
		Feature:            res.Feature.Name,
		FunctionID:         res.Function.ID,
		Function:           res.Function.Name,
		LZID:               res.LZ.ID,
		LZ:                 res.LZ.Name,
		ConnectionID:       res.Connection.ID,
		Connection:         res.Connection.Name,
		BillingCurrency:    res.BillingCurrency,
		ZoneID:             res.Zone.ID,
		Zone:               res.Zone.Name,
		ProjectID:          res.Project.ID,
		Project:            res.Project.Name,
		ApplicationID:      res.App.ID,
		Application:        res.App.Name,
		EnvironmentID:      res.Environment.ID,
		Environment:        res.Environment.Name,
		CSPID:              res.CSP.ID,
		CSP:                res.CSP.Name,
		RegionID:           res.Region.ID,
		Region:             res.Region.Name,
		ServiceCategoryID:  res.ServiceCategory.ID,
		ServiceCategory:    res.ServiceCategory.Name,
		ServiceID:          res.Service.ID,
		Service:            row.Service,
		ServiceUsageID:     res.ServiceUsage.ID,
		ServiceUsage:       res.ServiceUsage.Name,
		UsageID:            res.Usage.ID,
		Usage:              res.Usage.Name,
		ResourceGroupID:    res.ResourceGroup.ID,
		ResourceGroup:      res.ResourceGroup.Name,
		ResourceID:         res.Resource.ID,
		Resource:           res.Resource.Name,
		ResourceOrKey:      row.ResourceOrKey,
		Metric:             row.Metric,
		ActualValue:        row.ActualValue,
		ExpectedValue:      row.ExpectedValue,
		Severity:           row.Severity,
		Timestamp:          row.Timestamp,
	}
	// Unmatched raw values pass through as names; keep the sensed code in the id column.
	if rec.ZoneID == "" {
		rec.ZoneID = row.Zone
		if rec.Zone == row.Zone {
			rec.Zone = ""
		}
	}
	if rec.ConnectionID == "" {
		rec.ConnectionID = row.ConnectionID
		if rec.Connection == row.ConnectionID {
			rec.Connection = ""
		}
	}
	if rec.CSPID == "" && rec.CSP == "" {
		rec.CSPID = row.CSP
	}
	if rec.RegionID == "" {
		rec.RegionID = row.Region
		if rec.Region == row.Region {
			rec.Region = ""
		}
	}
	switch row.OpsDomain {
	case "finops":
		rec.PotentialSavings = row.ActualValue - row.ExpectedValue
		rec.ActionRecommended = "Rightsize underutilized compute or commit to Savings Plans"
	case "cloudops":
		rec.PotentialSavings = row.ActualValue
		rec.ActionRecommended = "Snapshot and terminate orphaned resource"
	case "secops":
		rec.ActionRecommended = "Apply restrictive bucket policy or rotate stale credentials immediately"
	}
	return rec, res
}
