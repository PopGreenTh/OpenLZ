package cloudops

import (
	"context"
	"fmt"
	"strings"

	"github.com/PopGreenTh/OpenLZ/internal/duckdb"
	types "github.com/PopGreenTh/OpenLZ/internal/stages/types"
)

// EnrichLifecycleInput models inputs for cloudops-enrich-lifecycle.
type EnrichLifecycleInput struct {
	InputPath  string `json:"inputPath"`
	OutputPath string `json:"outputPath"`
	Tenant     string `json:"tenant"`
}

// ExecuteEnrichLifecycle runs Stage 3: Annotate Assets with Lifecycle and Ownership.
func ExecuteEnrichLifecycle(ctx context.Context, in EnrichLifecycleInput) (*types.StageResult, error) {
	in.Tenant = strings.TrimSpace(in.Tenant)
	if in.InputPath == "" {
		in.InputPath = types.AppendTenantToPath("data/cloudops_anomalies.parquet", in.Tenant)
	}
	in.InputPath = types.ResolveAbsolutePath(in.InputPath)
	if in.OutputPath == "" {
		in.OutputPath = types.AppendTenantToPath("data/cloudops_enriched.parquet", in.Tenant)
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
