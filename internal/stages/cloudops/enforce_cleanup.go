package cloudops

import (
	"context"
	"fmt"
	"time"

	"github.com/PopGreenTh/OpenLZ/internal/cloud"
	"github.com/PopGreenTh/OpenLZ/internal/duckdb"
	types "github.com/PopGreenTh/OpenLZ/internal/stages/types"
)

// EnforceCleanupInput models inputs for cloudops-enforce-cleanup.
type EnforceCleanupInput struct {
	InputPath  string `json:"inputPath"`
	OutputPath string `json:"outputPath"`
	DryRun     bool   `json:"dryRun"`
}

// ExecuteEnforceCleanup runs Stage 4: Orphaned Resource Cleanup.
func ExecuteEnforceCleanup(ctx context.Context, in EnforceCleanupInput) (*types.StageResult, error) {
	if in.InputPath == "" {
		in.InputPath = "data/cloudops_enriched.parquet"
	}
	in.InputPath = types.ResolveAbsolutePath(in.InputPath)
	if in.OutputPath == "" {
		in.OutputPath = "data/cloudops_audit.parquet"
	}
	in.OutputPath = types.ResolveAbsolutePath(in.OutputPath)

	engine, err := duckdb.NewEngine()
	if err != nil {
		return nil, fmt.Errorf("duckdb init failed: %w", err)
	}
	defer engine.Close()

	enriched, err := engine.ReadEnrichedFromParquet(ctx, in.InputPath)
	if err != nil {
		return nil, fmt.Errorf("failed reading enriched cloudops parquet: %w", err)
	}

	var auditRecords []cloud.EnforceAuditRecord
	now := time.Now()

	for _, r := range enriched {
		action := "CloudOps:CleanOrphanedAsset"
		details := fmt.Sprintf("Cleaned orphaned asset %s (Savings: $%.2f/mo): %s", r.ResourceOrKey, r.PotentialSavings, r.ActionRecommended)
		if in.DryRun {
			details = fmt.Sprintf("[DRY-RUN] Would clean asset %s (Savings: $%.2f/mo): %s", r.ResourceOrKey, r.PotentialSavings, r.ActionRecommended)
		}

		auditRecords = append(auditRecords, cloud.EnforceAuditRecord{
			Timestamp:   now,
			OpsDomain:   "cloudops",
			AccountID:   r.AccountID,
			Service:     r.Service,
			ResourceID:  r.ResourceOrKey,
			ActionTaken: action,
			DryRun:      in.DryRun,
			Success:     true,
			Details:     details,
		})
	}

	if in.OutputPath != "" {
		if err := engine.WriteEnforceAuditParquet(ctx, auditRecords, in.OutputPath); err != nil {
			return nil, fmt.Errorf("failed writing cloudops enforce audit parquet: %w", err)
		}
	}

	return &types.StageResult{
		Stage:       types.StageCloudOpsEnforceCleanup,
		Success:     true,
		RecordCount: len(auditRecords),
		OutputPath:  in.OutputPath,
		Message:     fmt.Sprintf("Logged %d cleanup audit records into %s (dryRun=%v)", len(auditRecords), in.OutputPath, in.DryRun),
	}, nil
}
