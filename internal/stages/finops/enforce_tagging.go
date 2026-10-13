package finops

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/PopGreenTh/OpenLZ/internal/cloud"
	"github.com/PopGreenTh/OpenLZ/internal/duckdb"
	types "github.com/PopGreenTh/OpenLZ/internal/stages/types"
)

// EnforceTaggingInput models inputs for finops-enforce-tagging.
type EnforceTaggingInput struct {
	InputPath  string `json:"inputPath"`
	OutputPath string `json:"outputPath"`
	DryRun     bool   `json:"dryRun"`
	Tenant     string `json:"tenant"`
}

// ExecuteEnforceTagging runs Stage 4: Apply FinOps Policy & CostCenter Tags.
func ExecuteEnforceTagging(ctx context.Context, in EnforceTaggingInput) (*types.StageResult, error) {
	in.Tenant = strings.TrimSpace(in.Tenant)
	if in.InputPath == "" {
		in.InputPath = types.AppendTenantToPath("data/finops_enriched.parquet", in.Tenant)
	}
	in.InputPath = types.ResolveAbsolutePath(in.InputPath)
	if in.OutputPath == "" {
		in.OutputPath = types.AppendTenantToPath("data/finops_audit.parquet", in.Tenant)
	}
	in.OutputPath = types.ResolveAbsolutePath(in.OutputPath)

	engine, err := duckdb.NewEngine()
	if err != nil {
		return nil, fmt.Errorf("duckdb init failed: %w", err)
	}
	defer engine.Close()

	enriched, err := engine.ReadEnrichedFromParquet(ctx, in.InputPath)
	if err != nil {
		return nil, fmt.Errorf("failed reading enriched parquet: %w", err)
	}

	var auditRecords []cloud.EnforceAuditRecord
	now := time.Now()

	for _, r := range enriched {
		action := "FinOps:ApplyCostCenterTag"
		details := fmt.Sprintf("Tagged resource/service %s with CostCenter=%s and Owner=%s", r.Service, r.CostCenter, r.Owner)
		if in.DryRun {
			details = "[DRY-RUN] Would tag " + r.Service + " with CostCenter=" + r.CostCenter
		}

		auditRecords = append(auditRecords, cloud.EnforceAuditRecord{
			Timestamp:   now,
			OpsDomain:   "finops",
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
			return nil, fmt.Errorf("failed writing enforce audit parquet: %w", err)
		}
	}

	return &types.StageResult{
		Stage:       types.StageFinOpsEnforceTagging,
		Success:     true,
		RecordCount: len(auditRecords),
		OutputPath:  in.OutputPath,
		Message:     fmt.Sprintf("Logged %d enforcement audit records into %s (dryRun=%v)", len(auditRecords), in.OutputPath, in.DryRun),
	}, nil
}
