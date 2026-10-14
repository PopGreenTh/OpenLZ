package finops

import (
	"context"
	"fmt"

	"github.com/PopGreenTh/OpenLZ/internal/duckdb"
	types "github.com/PopGreenTh/OpenLZ/internal/stages/types"
)

// AnalyzeVarianceImpactInput models inputs for finops-analyze-varianceimpact.
type AnalyzeVarianceImpactInput struct {
	InputPath  string  `json:"inputPath"`
	OutputPath string  `json:"outputPath"`
	Threshold  float64 `json:"threshold"`
}

// ExecuteAnalyzeVarianceImpact runs Stage 2: Cost Variance & Spikes Analysis.
func ExecuteAnalyzeVarianceImpact(ctx context.Context, in AnalyzeVarianceImpactInput) (*types.StageResult, error) {
	if in.InputPath == "" {
		in.InputPath = "data/finops_raw.parquet"
	}
	in.InputPath = types.ResolveAbsolutePath(in.InputPath)
	if in.OutputPath == "" {
		in.OutputPath = "data/finops_anomalies.parquet"
	}
	in.OutputPath = types.ResolveAbsolutePath(in.OutputPath)
	if in.Threshold <= 0 {
		in.Threshold = 1.3
	}

	engine, err := duckdb.NewEngine()
	if err != nil {
		return nil, fmt.Errorf("duckdb init failed: %w", err)
	}
	defer engine.Close()

	anomalies, err := engine.AnalyzeFinOps(ctx, in.InputPath, in.OutputPath, in.Threshold)
	if err != nil {
		return nil, err
	}

	return &types.StageResult{
		Stage:       types.StageFinOpsAnalyzeVarianceImpact,
		Success:     true,
		RecordCount: len(anomalies),
		OutputPath:  in.OutputPath,
		Message:     fmt.Sprintf("Detected %d cost variance anomalies into %s (threshold: %.2fx)", len(anomalies), in.OutputPath, in.Threshold),
	}, nil
}
