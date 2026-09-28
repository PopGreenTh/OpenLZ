package cloudops

import (
	"context"
	"fmt"
	"strings"

	"github.com/PopGreenTh/OpenLZ/internal/aws"
	"github.com/PopGreenTh/OpenLZ/internal/cloud"
	"github.com/PopGreenTh/OpenLZ/internal/duckdb"
	types "github.com/PopGreenTh/OpenLZ/internal/stages/types"
	"github.com/PopGreenTh/OpenLZ/internal/workflow"
)

// SenseHygieneInput models inputs for cloudops-sense-hygiene.
type SenseHygieneInput struct {
	Accounts   []string `json:"accounts"`
	Service    string   `json:"service"`
	OutputPath string   `json:"outputPath"`
	Mock       bool     `json:"mock"`
}

// ExecuteSenseHygiene runs Stage 1: CloudOps Operational Hygiene Sense.
func ExecuteSenseHygiene(ctx context.Context, in SenseHygieneInput) (*types.StageResult, error) {
	if in.OutputPath == "" {
		in.OutputPath = "data/cloudops_raw.parquet"
	}
	in.OutputPath = types.ResolveAbsolutePath(in.OutputPath)
	if in.Service == "" {
		in.Service = "all"
	}
	if in.Mock && len(in.Accounts) == 0 {
		in.Accounts = []string{"111122223333", "444455556666"}
	}

	engine, err := duckdb.NewEngine()
	if err != nil {
		return nil, fmt.Errorf("duckdb init failed: %w", err)
	}
	defer engine.Close()

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

	scanner := workflow.NewScanner(client, nil, 5)
	records, err := scanner.ScanCloudOpsAccounts(ctx, in.Accounts)
	if err != nil {
		return nil, fmt.Errorf("cloudops sense failed: %w", err)
	}

	if in.Service != "" && in.Service != "all" {
		var filtered []cloud.CloudOpsRecord
		for _, r := range records {
			if strings.Contains(strings.ToLower(r.Service), strings.ToLower(in.Service)) {
				filtered = append(filtered, r)
			}
		}
		records = filtered
	}

	if in.OutputPath != "" {
		if err := engine.WriteCloudOpsParquet(ctx, records, in.OutputPath); err != nil {
			return nil, fmt.Errorf("failed writing raw cloudops parquet: %w", err)
		}
	}

	return &types.StageResult{
		Stage:       types.StageCloudOpsSenseHygiene,
		Success:     true,
		RecordCount: len(records),
		OutputPath:  in.OutputPath,
		Message:     fmt.Sprintf("Sensed %d operational hygiene records into %s", len(records), in.OutputPath),
	}, nil
}
