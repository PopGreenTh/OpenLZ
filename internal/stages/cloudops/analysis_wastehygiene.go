package cloudops

import (
	"context"
	"fmt"

	"github.com/PopGreenTh/OpenLZ/internal/duckdb"
	types "github.com/PopGreenTh/OpenLZ/internal/stages/types"
)

// AnalysisWasteHygieneInput models inputs for cloudops-analysis-wastehygiene.
type AnalysisWasteHygieneInput struct {
	InputPath  string `json:"inputPath"`
	OutputPath string `json:"outputPath"`
}

// ExecuteAnalysisWasteHygiene runs Stage 2: Waste & Idle Asset Analysis.
func ExecuteAnalysisWasteHygiene(ctx context.Context, in AnalysisWasteHygieneInput) (*types.StageResult, error) {
	if in.InputPath == "" {
		in.InputPath = "data/cloudops_raw.parquet"
	}
	in.InputPath = types.ResolveAbsolutePath(in.InputPath)
	if in.OutputPath == "" {
		in.OutputPath = "data/cloudops_anomalies.parquet"
	}
	in.OutputPath = types.ResolveAbsolutePath(in.OutputPath)

	engine, err := duckdb.NewEngine()
	if err != nil {
		return nil, fmt.Errorf("duckdb init failed: %w", err)
	}
	defer engine.Close()

	anomalies, err := engine.AnalyzeCloudOps(ctx, in.InputPath, in.OutputPath)
	if err != nil {
		return nil, err
	}

	return &types.StageResult{
		Stage:       types.StageCloudOpsAnalysisWasteHygiene,
		Success:     true,
		RecordCount: len(anomalies),
		OutputPath:  in.OutputPath,
		Message:     fmt.Sprintf("Detected %d orphaned & waste findings into %s", len(anomalies), in.OutputPath),
	}, nil
}
