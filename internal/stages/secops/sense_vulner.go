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
	Accounts        []string `json:"accounts"`
	Format          string   `json:"format"`
	OutputPath      string   `json:"outputPath"`
	AppendTimestamp bool     `json:"appendTimestamp"`
	AppendOutput    bool     `json:"appendOutput"`
	Upsert          bool     `json:"upsert"`
	Mock            bool     `json:"mock"`
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

	targetPath := types.AppendTimestampToPath(in.OutputPath)

	if in.OutputPath != "" {
		// Always write timestamped immutable parquet snapshot
		if err := engine.WriteSecOpsParquet(ctx, allFindings, targetPath); err != nil {
			return nil, fmt.Errorf("failed writing vulnerability parquet: %w", err)
		}

		// Option to append/upsert to the original --output
		if in.AppendOutput || in.Upsert {
			if err := engine.UpsertSecOpsParquet(ctx, allFindings, in.OutputPath); err != nil {
				return nil, fmt.Errorf("failed appending/upserting vulnerability parquet: %w", err)
			}
		}
	}

	msg := fmt.Sprintf("Sensed %d vulnerability findings into %s", len(allFindings), targetPath)
	if in.AppendOutput || in.Upsert {
		msg = fmt.Sprintf("Sensed %d vulnerability findings into %s (and appended to %s)", len(allFindings), targetPath, in.OutputPath)
	}

	return &types.StageResult{
		Stage:       types.StageSecOpsSenseVulner,
		Success:     true,
		RecordCount: len(allFindings),
		OutputPath:  targetPath,
		Message:     msg,
	}, nil
}
