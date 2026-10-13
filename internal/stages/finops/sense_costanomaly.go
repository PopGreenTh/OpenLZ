package finops

import (
	"context"
	"fmt"
	"strings"

	"github.com/PopGreenTh/OpenLZ/internal/aws"
	"github.com/PopGreenTh/OpenLZ/internal/cloud"
	"github.com/PopGreenTh/OpenLZ/internal/duckdb"
	types "github.com/PopGreenTh/OpenLZ/internal/stages/types"
)

// SenseCostAnomalyInput models inputs for finops-sense-costanomaly.
type SenseCostAnomalyInput struct {
	Accounts        []string `json:"accounts"`
	StartDate       string   `json:"startDate"`
	EndDate         string   `json:"endDate"`
	OutputPath      string   `json:"outputPath"`
	AppendTimestamp bool     `json:"appendTimestamp"`
	AppendOutput    bool     `json:"appendOutput"`
	Upsert          bool     `json:"upsert"`
	Mock            bool     `json:"mock"`
	Tenant          string   `json:"tenant"`
}

// ExecuteSenseCostAnomaly runs Stage 1b: AWS Cost Anomaly Detection Sense.
func ExecuteSenseCostAnomaly(ctx context.Context, in SenseCostAnomalyInput) (*types.StageResult, error) {
	in.Tenant = strings.TrimSpace(in.Tenant)
	if in.OutputPath == "" {
		in.OutputPath = types.AppendTenantToPath("data/finops_anomalies.parquet", in.Tenant)
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

	targetPath := types.AppendTimestampToPath(in.OutputPath)

	if in.OutputPath != "" {
		// Always write timestamped immutable parquet snapshot
		if err := engine.WriteAnomaliesParquet(ctx, allAnomalies, targetPath); err != nil {
			return nil, fmt.Errorf("failed writing cost anomalies parquet: %w", err)
		}

		// Option to append/upsert to the original --output
		if in.AppendOutput || in.Upsert {
			if err := engine.UpsertAnomaliesParquet(ctx, allAnomalies, in.OutputPath); err != nil {
				return nil, fmt.Errorf("failed appending/upserting to original anomalies parquet: %w", err)
			}
		}
	}

	msg := fmt.Sprintf("Sensed %d Cost Anomaly records into %s", len(allAnomalies), targetPath)
	if in.AppendOutput || in.Upsert {
		msg = fmt.Sprintf("Sensed %d Cost Anomaly records into %s (and appended to %s)", len(allAnomalies), targetPath, in.OutputPath)
	}

	return &types.StageResult{
		Stage:       types.StageFinOpsSenseCostAnomaly,
		Success:     true,
		RecordCount: len(allAnomalies),
		OutputPath:  targetPath,
		Message:     msg,
	}, nil
}

