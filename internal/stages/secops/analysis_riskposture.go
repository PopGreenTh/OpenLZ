package secops

import (
	"context"
	"fmt"
	"strings"

	"github.com/PopGreenTh/OpenLZ/internal/duckdb"
	types "github.com/PopGreenTh/OpenLZ/internal/stages/types"
)

// AnalysisRiskPostureInput models inputs for secops-analysis-riskposture.
type AnalysisRiskPostureInput struct {
	InputPath  string `json:"inputPath"`
	OutputPath string `json:"outputPath"`
	Tenant     string `json:"tenant"`
}

// ExecuteAnalysisRiskPosture runs Stage 2: Security Risk Analysis.
func ExecuteAnalysisRiskPosture(ctx context.Context, in AnalysisRiskPostureInput) (*types.StageResult, error) {
	in.Tenant = strings.TrimSpace(in.Tenant)
	if in.InputPath == "" {
		in.InputPath = types.AppendTenantToPath("data/secops_raw.parquet", in.Tenant)
	}
	in.InputPath = types.ResolveAbsolutePath(in.InputPath)
	if in.OutputPath == "" {
		in.OutputPath = types.AppendTenantToPath("data/secops_anomalies.parquet", in.Tenant)
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
