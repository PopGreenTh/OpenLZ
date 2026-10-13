package finops

import (
	"context"
	"fmt"
	"strings"

	"github.com/PopGreenTh/OpenLZ/internal/cloud"
	"github.com/PopGreenTh/OpenLZ/internal/duckdb"
	types "github.com/PopGreenTh/OpenLZ/internal/stages/types"
	"github.com/PopGreenTh/OpenLZ/internal/taxonomy"
)

// EnrichCostCentersInput models inputs for finops-enrich-costcenters.
type EnrichCostCentersInput struct {
	InputPath            string `json:"inputPath"`
	OutputPath           string `json:"outputPath"`
	ProjectTaxonomyPath  string `json:"projectTaxonomyPath"`
	BusinessTaxonomyPath string `json:"businessTaxonomyPath"`
	Tenant               string `json:"tenant"`
}

// ExecuteEnrichCostCenters runs Stage 3: Annotate with Landing Zone Cost Centers & Metadata.
// When project_taxonomy.json and business_taxonomy.json are present, empty hierarchy tags
// sensed from Cost Explorer are filled from those trees.
func ExecuteEnrichCostCenters(ctx context.Context, in EnrichCostCentersInput) (*types.StageResult, error) {
	in.Tenant = strings.TrimSpace(in.Tenant)
	if in.InputPath == "" {
		in.InputPath = types.AppendTenantToPath("data/finops_anomalies.parquet", in.Tenant)
	}
	in.InputPath = types.ResolveAbsolutePath(in.InputPath)
	if in.OutputPath == "" {
		in.OutputPath = types.AppendTenantToPath("data/finops_enriched.parquet", in.Tenant)
	}
	in.OutputPath = types.ResolveAbsolutePath(in.OutputPath)

	if in.Tenant != "" {
		if in.ProjectTaxonomyPath == "" {
			in.ProjectTaxonomyPath = taxonomy.ResolveProjectTaxonomyPath("", in.Tenant)
		}
		if in.BusinessTaxonomyPath == "" {
			in.BusinessTaxonomyPath = taxonomy.ResolveBusinessTaxonomyPath("", in.Tenant)
		}
	}
	if in.ProjectTaxonomyPath != "" {
		in.ProjectTaxonomyPath = types.ResolveAbsolutePath(in.ProjectTaxonomyPath)
	}
	if in.BusinessTaxonomyPath != "" {
		in.BusinessTaxonomyPath = types.ResolveAbsolutePath(in.BusinessTaxonomyPath)
	}

	engine, err := duckdb.NewEngine()
	if err != nil {
		return nil, fmt.Errorf("duckdb init failed: %w", err)
	}
	defer engine.Close()

	catalog, err := taxonomy.LoadOptional(in.ProjectTaxonomyPath, in.BusinessTaxonomyPath)
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

	rows, err := engine.ReadAnalyzedCostRows(ctx, in.InputPath)
	if err != nil {
		return nil, err
	}
	enriched := make([]cloud.EnrichedRecord, 0, len(rows))
	for _, row := range rows {
		enriched = append(enriched, mapEmptyTags(row, catalog))
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

func mapEmptyTags(row duckdb.AnalyzedCostRow, catalog *taxonomy.Catalog) cloud.EnrichedRecord {
	resolved := catalog.Resolve(taxonomy.Input{
		AccountID:   row.AccountID,
		AccountName: row.AccountName,
		Service:     row.Service,
		Project:     row.PrimaryTag,
		App:         row.SecondaryTag,
	})

	rec := cloud.EnrichedRecord{
		OpsDomain:          row.OpsDomain,
		AccountID:          row.AccountID,
		AccountName:        row.AccountName,
		Environment:        resolved.Environment.Name,
		EnvironmentID:      resolved.Environment.ID,
		BusinessUnit:       resolved.BusinessUnit.Name,
		BusinessUnitID:     resolved.BusinessUnit.ID,
		Owner:              resolved.Owner.Name,
		OwnerID:            resolved.Owner.ID,
		CostCenter:         resolved.CostCenter.Name,
		CostCenterID:       resolved.CostCenter.ID,
		Project:            resolved.Project.Name,
		ProjectID:          resolved.Project.ID,
		Application:        resolved.App.Name,
		ApplicationID:      resolved.App.ID,
		ServiceCategory:    resolved.ServiceCategory.Name,
		ServiceCategoryID:  resolved.ServiceCategory.ID,
		CustomerFunction:   resolved.CustomerFunction.Name,
		CustomerFunctionID: resolved.CustomerFunction.ID,
		Service:            row.Service,
		ResourceOrKey:      row.ResourceOrKey,
		Metric:             row.Metric,
		ActualValue:        row.ActualValue,
		ExpectedValue:      row.ExpectedValue,
		Severity:           row.Severity,
		Timestamp:          row.Timestamp,
	}
	if row.OpsDomain == "finops" {
		rec.PotentialSavings = row.ActualValue - row.ExpectedValue
	} else if row.OpsDomain == "cloudops" {
		rec.PotentialSavings = row.ActualValue
	}
	applyHeuristic(&rec)
	return rec
}

func applyHeuristic(r *cloud.EnrichedRecord) {
	accountName := strings.ToLower(r.AccountName)
	if r.Environment == "" {
		switch {
		case strings.HasSuffix(r.AccountID, "1") || strings.Contains(accountName, "prod"):
			r.Environment = "Production"
		case strings.HasSuffix(r.AccountID, "2") || strings.Contains(accountName, "stg"):
			r.Environment = "Staging"
		default:
			r.Environment = "Development"
		}
	}
	if r.BusinessUnit == "" {
		switch r.Service {
		case "AmazonEC2", "Compute":
			r.BusinessUnit = "Core Platform"
		case "AmazonS3", "Storage":
			r.BusinessUnit = "Data Engineering"
		case "AWSIdentityAccessManagement", "Security":
			r.BusinessUnit = "Security Operations"
		default:
			r.BusinessUnit = "Digital Products"
		}
	}
	if r.Owner == "" {
		switch r.OpsDomain {
		case "secops":
			r.Owner = "secops-lead@company.com"
		case "finops":
			r.Owner = "finops-team@company.com"
		default:
			r.Owner = "devops-infra@company.com"
		}
	}
	if r.CostCenter == "" {
		prefix := r.AccountID
		if len(prefix) > 4 {
			prefix = prefix[:4]
		}
		r.CostCenter = "CC-" + prefix
	}
	if r.ActionRecommended == "" {
		switch r.OpsDomain {
		case "finops":
			r.ActionRecommended = "Rightsize underutilized compute or commit to Savings Plans"
		case "secops":
			r.ActionRecommended = "Apply restrictive bucket policy or rotate stale credentials immediately"
		default:
			r.ActionRecommended = "Snapshot and terminate orphaned resource"
		}
	}

	// Enforce "must behave" identity relations:
	// 1. ProjectID/Name must behave BusinessUnitID/Name
	if r.Project == "" && r.BusinessUnit != "" {
		r.Project = r.BusinessUnit
	}
	if r.BusinessUnit == "" && r.Project != "" {
		r.BusinessUnit = r.Project
	}
	if r.ProjectID == "" && r.BusinessUnitID != "" {
		r.ProjectID = r.BusinessUnitID
	}
	if r.BusinessUnitID == "" && r.ProjectID != "" {
		r.BusinessUnitID = r.ProjectID
	}

	// 2. AppID/Name must behave OwnerID/Name
	if r.Application == "" && r.Owner != "" && !strings.Contains(r.Owner, "@") {
		r.Application = r.Owner
	}
	if r.Owner == "" && r.Application != "" {
		r.Owner = r.Application
	}
	if r.ApplicationID == "" && r.OwnerID != "" {
		r.ApplicationID = r.OwnerID
	}
	if r.OwnerID == "" && r.ApplicationID != "" {
		r.OwnerID = r.ApplicationID
	}

	// 3. EnvironmentID/Name must behave CostCenterID/Name
	if r.Environment == "" && r.CostCenter != "" {
		r.Environment = r.CostCenter
	}
	if r.CostCenter == "" && r.Environment != "" {
		r.CostCenter = r.Environment
	}
	if r.EnvironmentID == "" && r.CostCenterID != "" {
		r.EnvironmentID = r.CostCenterID
	}
	if r.CostCenterID == "" && r.EnvironmentID != "" {
		r.CostCenterID = r.EnvironmentID
	}
}
