package secops

import (
	"context"
	"fmt"
	"strings"

	"github.com/PopGreenTh/OpenLZ/internal/duckdb"
	types "github.com/PopGreenTh/OpenLZ/internal/stages/types"
)

// EnrichMetadataInput models inputs for secops-enrich-metadata.
type EnrichMetadataInput struct {
	InputPath  string `json:"inputPath"`
	OutputPath string `json:"outputPath"`
	Tenant     string `json:"tenant"`
}

// ExecuteEnrichMetadata runs Stage 3: Annotate Security Findings with Blast Radius.
func ExecuteEnrichMetadata(ctx context.Context, in EnrichMetadataInput) (*types.StageResult, error) {
	in.Tenant = strings.TrimSpace(in.Tenant)
	if in.InputPath == "" {
		in.InputPath = types.AppendTenantToPath("data/secops_anomalies.parquet", in.Tenant)
	}
	in.InputPath = types.ResolveAbsolutePath(in.InputPath)
	if in.OutputPath == "" {
		in.OutputPath = types.AppendTenantToPath("data/secops_enriched.parquet", in.Tenant)
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
		Stage:       types.StageSecOpsEnrichMetadata,
		Success:     true,
		RecordCount: len(enriched),
		OutputPath:  in.OutputPath,
		Message:     fmt.Sprintf("Enriched %d security findings with ownership metadata into %s", len(enriched), in.OutputPath),
	}, nil
}
