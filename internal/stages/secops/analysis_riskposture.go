package secops

import (
	"context"
	"fmt"

	"github.com/PopGreenTh/OpenLZ/internal/duckdb"
	types "github.com/PopGreenTh/OpenLZ/internal/stages/types"
)

// AnalysisRiskPostureInput models inputs for secops-analysis-riskposture.
type AnalysisRiskPostureInput struct {
	InputPath  string `json:"inputPath"`
	OutputPath string `json:"outputPath"`
}

// ExecuteAnalysisRiskPosture runs Stage 2: Security Risk Analysis.
func ExecuteAnalysisRiskPosture(ctx context.Context, in AnalysisRiskPostureInput) (*types.StageResult, error) {
	if in.InputPath == "" {
		in.InputPath = "data/secops_raw.parquet"
	}
	in.InputPath = types.ResolveAbsolutePath(in.InputPath)
	if in.OutputPath == "" {
		in.OutputPath = "data/secops_anomalies.parquet"
	}
	in.OutputPath = types.ResolveAbsolutePath(in.OutputPath)

	engine, err := duckdb.NewEngine()
	if err != nil {
		return nil, fmt.Errorf("duckdb init failed: %w", err)
	}
	defer engine.Close()

	risks, err := engine.AnalyzeSecOps(ctx, in.InputPath, in.OutputPath)
	if err != nil {
		return nil, err
	}

	return &types.StageResult{
		Stage:       types.StageSecOpsAnalysisRiskPosture,
		Success:     true,
		RecordCount: len(risks),
		OutputPath:  in.OutputPath,
		Message:     fmt.Sprintf("Extracted %d security risks into %s", len(risks), in.OutputPath),
	}, nil
}
