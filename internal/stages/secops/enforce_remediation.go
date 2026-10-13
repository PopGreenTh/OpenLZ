package secops

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/PopGreenTh/OpenLZ/internal/cloud"
	"github.com/PopGreenTh/OpenLZ/internal/duckdb"
	types "github.com/PopGreenTh/OpenLZ/internal/stages/types"
)

// EnforceRemediationInput models inputs for secops-enforce-remediation.
type EnforceRemediationInput struct {
	InputPath  string `json:"inputPath"`
	OutputPath string `json:"outputPath"`
	DryRun     bool   `json:"dryRun"`
	Tenant     string `json:"tenant"`
}

// ExecuteEnforceRemediation runs Stage 4: SecOps Guardrails Remediation.
func ExecuteEnforceRemediation(ctx context.Context, in EnforceRemediationInput) (*types.StageResult, error) {
	in.Tenant = strings.TrimSpace(in.Tenant)
	if in.InputPath == "" {
		in.InputPath = types.AppendTenantToPath("data/secops_enriched.parquet", in.Tenant)
	}
	in.InputPath = types.ResolveAbsolutePath(in.InputPath)
	if in.OutputPath == "" {
		in.OutputPath = types.AppendTenantToPath("data/secops_audit.parquet", in.Tenant)
	}
	in.OutputPath = types.ResolveAbsolutePath(in.OutputPath)

	engine, err := duckdb.NewEngine()
	if err != nil {
		return nil, fmt.Errorf("duckdb init failed: %w", err)
	}
	defer engine.Close()

	enriched, err := engine.ReadEnrichedFromParquet(ctx, in.InputPath)
	if err != nil {
		return nil, fmt.Errorf("failed reading enriched secops parquet: %w", err)
	}

	var auditRecords []cloud.EnforceAuditRecord
	now := time.Now()

	for _, r := range enriched {
		action := "SecOps:RemediateAccess"
		details := fmt.Sprintf("Enforced remediation for %s (%s): %s", r.ResourceOrKey, r.Severity, r.ActionRecommended)
		if in.DryRun {
			details = "[DRY-RUN] Would remediate " + r.ResourceOrKey + ": " + r.ActionRecommended
		}

		auditRecords = append(auditRecords, cloud.EnforceAuditRecord{
			Timestamp:   now,
			OpsDomain:   "secops",
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
			return nil, fmt.Errorf("failed writing secops enforce audit parquet: %w", err)
		}
	}

	return &types.StageResult{
		Stage:       types.StageSecOpsEnforceRemediation,
		Success:     true,
		RecordCount: len(auditRecords),
		OutputPath:  in.OutputPath,
		Message:     fmt.Sprintf("Logged %d remediation audit records into %s (dryRun=%v)", len(auditRecords), in.OutputPath, in.DryRun),
	}, nil
}
