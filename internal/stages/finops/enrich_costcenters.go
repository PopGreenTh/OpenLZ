package finops

import (
	"context"
	"fmt"

	"github.com/PopGreenTh/OpenLZ/internal/duckdb"
	types "github.com/PopGreenTh/OpenLZ/internal/stages/types"
)

// EnrichCostCentersInput models inputs for finops-enrich-costcenters.
type EnrichCostCentersInput struct {
	InputPath  string `json:"inputPath"`
	OutputPath string `json:"outputPath"`
}

// ExecuteEnrichCostCenters runs Stage 3: Annotate with Landing Zone Cost Centers & Metadata.
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
