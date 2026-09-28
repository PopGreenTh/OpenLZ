package cloudops

import (
	"context"
	"fmt"

	"github.com/PopGreenTh/OpenLZ/internal/duckdb"
	types "github.com/PopGreenTh/OpenLZ/internal/stages/types"
)

// EnrichLifecycleInput models inputs for cloudops-enrich-lifecycle.
type EnrichLifecycleInput struct {
	InputPath  string `json:"inputPath"`
	OutputPath string `json:"outputPath"`
}

// ExecuteEnrichLifecycle runs Stage 3: Annotate Assets with Lifecycle and Ownership.
func ExecuteEnrichLifecycle(ctx context.Context, in EnrichLifecycleInput) (*types.StageResult, error) {
	if in.InputPath == "" {
		in.InputPath = "data/cloudops_anomalies.parquet"
	}
	in.InputPath = types.ResolveAbsolutePath(in.InputPath)
	if in.OutputPath == "" {
		in.OutputPath = "data/cloudops_enriched.parquet"
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
		Stage:       types.StageCloudOpsEnrichLifecycle,
		Success:     true,
		RecordCount: len(enriched),
		OutputPath:  in.OutputPath,
		Message:     fmt.Sprintf("Enriched %d hygiene records with metadata into %s", len(enriched), in.OutputPath),
	}, nil
}
