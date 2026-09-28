package finops

import (
	"context"
	"fmt"

	"github.com/PopGreenTh/OpenLZ/internal/aws"
	"github.com/PopGreenTh/OpenLZ/internal/cloud"
	"github.com/PopGreenTh/OpenLZ/internal/duckdb"
	types "github.com/PopGreenTh/OpenLZ/internal/stages/types"
)

// SenseCostAnomalyInput models inputs for finops-sense-costanomaly.
type SenseCostAnomalyInput struct {
	Accounts   []string `json:"accounts"`
	StartDate  string   `json:"startDate"`
	EndDate    string   `json:"endDate"`
	OutputPath string   `json:"outputPath"`
	Mock       bool     `json:"mock"`
}

// ExecuteSenseCostAnomaly runs Stage 1b: AWS Cost Anomaly Detection Sense.
func ExecuteSenseCostAnomaly(ctx context.Context, in SenseCostAnomalyInput) (*types.StageResult, error) {
	if in.OutputPath == "" {
		in.OutputPath = "data/finops_anomalies.parquet"
	}
	in.OutputPath = types.ResolveAbsolutePath(in.OutputPath)
	if in.Mock && len(in.Accounts) == 0 {
		in.Accounts = []string{"111122223333", "444455556666"}
	}

	client, err := aws.NewClient(ctx, "ap-southeast-1", in.Mock)
	if err != nil {
		return nil, fmt.Errorf("aws client init failed: %w", err)
	}

	if !in.Mock && len(in.Accounts) == 0 {
		callerAcc, err := client.GetCallerAccountID(ctx)
		if err != nil {
			return nil, fmt.Errorf("no accounts specified and failed to auto-detect caller AWS account ID: %w", err)
		}
		in.Accounts = []string{callerAcc}
	}

	engine, err := duckdb.NewEngine()
	if err != nil {
		return nil, fmt.Errorf("duckdb init failed: %w", err)
	}
	defer engine.Close()

	var allAnomalies []cloud.AnomalyRecord
	for _, acc := range in.Accounts {
		anoms, err := client.FetchCostAnomalies(ctx, acc, in.StartDate, in.EndDate)
		if err != nil {
			return nil, fmt.Errorf("failed fetching cost anomalies for account %s: %w", acc, err)
		}
		allAnomalies = append(allAnomalies, anoms...)
	}

	if err := engine.WriteAnomaliesParquet(ctx, allAnomalies, in.OutputPath); err != nil {
		return nil, fmt.Errorf("failed writing cost anomalies parquet: %w", err)
	}

	return &types.StageResult{
		Stage:       types.StageFinOpsSenseCostAnomaly,
		Success:     true,
		RecordCount: len(allAnomalies),
		OutputPath:  in.OutputPath,
		Message:     fmt.Sprintf("Sensed %d Cost Anomaly records into %s", len(allAnomalies), in.OutputPath),
	}, nil
}
