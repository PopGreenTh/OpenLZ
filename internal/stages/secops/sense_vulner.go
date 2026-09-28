package secops

import (
	"context"
	"fmt"

	"github.com/PopGreenTh/OpenLZ/internal/aws"
	"github.com/PopGreenTh/OpenLZ/internal/cloud"
	"github.com/PopGreenTh/OpenLZ/internal/duckdb"
	types "github.com/PopGreenTh/OpenLZ/internal/stages/types"
)

// SenseVulnerInput models inputs for secops-sense-vulner.
type SenseVulnerInput struct {
	Accounts   []string `json:"accounts"`
	OutputPath string   `json:"outputPath"`
	Mock       bool     `json:"mock"`
}

// ExecuteSenseVulner runs Stage 1b: SecOps Vulnerabilities & CVEs Sense.
func ExecuteSenseVulner(ctx context.Context, in SenseVulnerInput) (*types.StageResult, error) {
	if in.OutputPath == "" {
		in.OutputPath = "data/secops_vulner_raw.parquet"
	}
	in.OutputPath = types.ResolveAbsolutePath(in.OutputPath)
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

	var allFindings []cloud.SecurityRecord
	for _, acc := range in.Accounts {
		findings, err := client.FetchVulnerabilityFindings(ctx, acc)
		if err != nil {
			return nil, fmt.Errorf("failed scanning vulnerabilities for account %s: %w", acc, err)
		}
		allFindings = append(allFindings, findings...)
	}

	if err := engine.WriteSecOpsParquet(ctx, allFindings, in.OutputPath); err != nil {
		return nil, fmt.Errorf("failed writing vulnerability parquet: %w", err)
	}

	return &types.StageResult{
		Stage:       types.StageSecOpsSenseVulner,
		Success:     true,
		RecordCount: len(allFindings),
		OutputPath:  in.OutputPath,
		Message:     fmt.Sprintf("Sensed %d vulnerability findings into %s", len(allFindings), in.OutputPath),
	}, nil
}
