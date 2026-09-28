package stages_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/PopGreenTh/OpenLZ/internal/stages"
	cloudopsstages "github.com/PopGreenTh/OpenLZ/internal/stages/cloudops"
	finopsstages "github.com/PopGreenTh/OpenLZ/internal/stages/finops"
	secopsstages "github.com/PopGreenTh/OpenLZ/internal/stages/secops"
	"github.com/PopGreenTh/OpenLZ/internal/stages/types"
)

func TestDedicatedStages_FinOps(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	rawPath := filepath.Join(tempDir, "finops_raw.parquet")
	anomPath := filepath.Join(tempDir, "finops_anomalies.parquet")
	enrichPath := filepath.Join(tempDir, "finops_enriched.parquet")
	auditPath := filepath.Join(tempDir, "finops_audit.parquet")

	// 1a. finops-sense-costexplorer
	resCE, err := finopsstages.ExecuteSenseCostExplorer(ctx, finopsstages.SenseCostExplorerInput{
		Accounts:    []string{"111122223333"},
		OutputPath:  rawPath,
		CacheDBPath: filepath.Join(tempDir, "cache.duckdb"),
		Mock:        true,
	})
	if err != nil {
		t.Fatalf("ExecuteSenseCostExplorer failed: %v", err)
	}
	if resCE.RecordCount == 0 || !resCE.Success {
		t.Errorf("expected positive record count, got %d", resCE.RecordCount)
	}

	// 1b. finops-sense-costanomaly
	resAnom, err := finopsstages.ExecuteSenseCostAnomaly(ctx, finopsstages.SenseCostAnomalyInput{
		Accounts:   []string{"111122223333"},
		OutputPath: anomPath,
		Mock:       true,
	})
	if err != nil {
		t.Fatalf("ExecuteSenseCostAnomaly failed: %v", err)
	}
	if resAnom.RecordCount == 0 || !resAnom.Success {
		t.Errorf("expected positive anomaly count, got %d", resAnom.RecordCount)
	}

	// 2. finops-analyze-varianceimpact
	resAnalyze, err := finopsstages.ExecuteAnalyzeVarianceImpact(ctx, finopsstages.AnalyzeVarianceImpactInput{
		InputPath:  rawPath,
		OutputPath: anomPath,
		Threshold:  1.2,
	})
	if err != nil {
		t.Fatalf("ExecuteAnalyzeVarianceImpact failed: %v", err)
	}
	if !resAnalyze.Success {
		t.Errorf("expected analyze success, got false")
	}

	// 3. finops-enrich-costcenters
	resEnrich, err := finopsstages.ExecuteEnrichCostCenters(ctx, finopsstages.EnrichCostCentersInput{
		InputPath:  anomPath,
		OutputPath: enrichPath,
	})
	if err != nil {
		t.Fatalf("ExecuteEnrichCostCenters failed: %v", err)
	}
	if !resEnrich.Success {
		t.Errorf("expected enrich success, got false")
	}

	// 4. finops-enforce-tagging
	resEnforce, err := finopsstages.ExecuteEnforceTagging(ctx, finopsstages.EnforceTaggingInput{
		InputPath:  enrichPath,
		OutputPath: auditPath,
		DryRun:     true,
	})
	if err != nil {
		t.Fatalf("ExecuteEnforceTagging failed: %v", err)
	}
	if !resEnforce.Success {
		t.Errorf("expected enforce success, got false")
	}

	// 5. finops-report-summary
	resReport, err := finopsstages.ExecuteReportSummary(ctx, finopsstages.ReportSummaryInput{
		InputPath: enrichPath,
		Format:    "table",
	})
	if err != nil {
		t.Fatalf("ExecuteReportSummary failed: %v", err)
	}
	if !resReport.Success {
		t.Errorf("expected report success, got false")
	}
}

func TestDedicatedStages_SecOps(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	rawPath := filepath.Join(tempDir, "secops_raw.parquet")
	vulnPath := filepath.Join(tempDir, "secops_vulner.parquet")
	anomPath := filepath.Join(tempDir, "secops_anomalies.parquet")
	enrichPath := filepath.Join(tempDir, "secops_enriched.parquet")
	auditPath := filepath.Join(tempDir, "secops_audit.parquet")

	// 1a. secops-sense-posture
	resPosture, err := secopsstages.ExecuteSensePosture(ctx, secopsstages.SensePostureInput{
		Accounts:   []string{"111122223333"},
		OutputPath: rawPath,
		Mock:       true,
	})
	if err != nil {
		t.Fatalf("ExecuteSensePosture failed: %v", err)
	}
	if resPosture.RecordCount == 0 || !resPosture.Success {
		t.Errorf("expected positive posture findings count, got %d", resPosture.RecordCount)
	}

	// 1b. secops-sense-vulner
	resVuln, err := secopsstages.ExecuteSenseVulner(ctx, secopsstages.SenseVulnerInput{
		Accounts:   []string{"111122223333"},
		OutputPath: vulnPath,
		Mock:       true,
	})
	if err != nil {
		t.Fatalf("ExecuteSenseVulner failed: %v", err)
	}
	if resVuln.RecordCount == 0 || !resVuln.Success {
		t.Errorf("expected positive vulnerability count, got %d", resVuln.RecordCount)
	}

	// 2. secops-analysis-riskposture
	resAnalyze, err := secopsstages.ExecuteAnalysisRiskPosture(ctx, secopsstages.AnalysisRiskPostureInput{
		InputPath:  rawPath,
		OutputPath: anomPath,
	})
	if err != nil {
		t.Fatalf("ExecuteAnalysisRiskPosture failed: %v", err)
	}
	if !resAnalyze.Success {
		t.Errorf("expected analyze success, got false")
	}

	// 3. secops-enrich-metadata
	resEnrich, err := secopsstages.ExecuteEnrichMetadata(ctx, secopsstages.EnrichMetadataInput{
		InputPath:  anomPath,
		OutputPath: enrichPath,
	})
	if err != nil {
		t.Fatalf("ExecuteEnrichMetadata failed: %v", err)
	}
	if !resEnrich.Success {
		t.Errorf("expected enrich success, got false")
	}

	// 4. secops-enforce-remediation
	resEnforce, err := secopsstages.ExecuteEnforceRemediation(ctx, secopsstages.EnforceRemediationInput{
		InputPath:  enrichPath,
		OutputPath: auditPath,
		DryRun:     true,
	})
	if err != nil {
		t.Fatalf("ExecuteEnforceRemediation failed: %v", err)
	}
	if !resEnforce.Success {
		t.Errorf("expected enforce success, got false")
	}

	// 5. secops-report-summary
	resReport, err := secopsstages.ExecuteReportSummary(ctx, secopsstages.ReportSummaryInput{
		InputPath: enrichPath,
		Format:    "table",
	})
	if err != nil {
		t.Fatalf("ExecuteReportSummary failed: %v", err)
	}
	if !resReport.Success {
		t.Errorf("expected report success, got false")
	}
}

func TestDedicatedStages_CloudOps(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	rawPath := filepath.Join(tempDir, "cloudops_raw.parquet")
	anomPath := filepath.Join(tempDir, "cloudops_anomalies.parquet")
	enrichPath := filepath.Join(tempDir, "cloudops_enriched.parquet")
	auditPath := filepath.Join(tempDir, "cloudops_audit.parquet")

	// 1. cloudops-sense-hygiene
	resSense, err := cloudopsstages.ExecuteSenseHygiene(ctx, cloudopsstages.SenseHygieneInput{
		Accounts:   []string{"111122223333"},
		OutputPath: rawPath,
		Mock:       true,
	})
	if err != nil {
		t.Fatalf("ExecuteSenseHygiene failed: %v", err)
	}
	if resSense.RecordCount == 0 || !resSense.Success {
		t.Errorf("expected positive hygiene count, got %d", resSense.RecordCount)
	}

	// 2. cloudops-analysis-wastehygiene
	resAnalyze, err := cloudopsstages.ExecuteAnalysisWasteHygiene(ctx, cloudopsstages.AnalysisWasteHygieneInput{
		InputPath:  rawPath,
		OutputPath: anomPath,
	})
	if err != nil {
		t.Fatalf("ExecuteAnalysisWasteHygiene failed: %v", err)
	}
	if !resAnalyze.Success {
		t.Errorf("expected analyze success, got false")
	}

	// 3. cloudops-enrich-lifecycle
	resEnrich, err := cloudopsstages.ExecuteEnrichLifecycle(ctx, cloudopsstages.EnrichLifecycleInput{
		InputPath:  anomPath,
		OutputPath: enrichPath,
	})
	if err != nil {
		t.Fatalf("ExecuteEnrichLifecycle failed: %v", err)
	}
	if !resEnrich.Success {
		t.Errorf("expected enrich success, got false")
	}

	// 4. cloudops-enforce-cleanup
	resEnforce, err := cloudopsstages.ExecuteEnforceCleanup(ctx, cloudopsstages.EnforceCleanupInput{
		InputPath:  enrichPath,
		OutputPath: auditPath,
		DryRun:     true,
	})
	if err != nil {
		t.Fatalf("ExecuteEnforceCleanup failed: %v", err)
	}
	if !resEnforce.Success {
		t.Errorf("expected enforce success, got false")
	}

	// 5. cloudops-report-summary
	resReport, err := cloudopsstages.ExecuteReportSummary(ctx, cloudopsstages.ReportSummaryInput{
		InputPath: enrichPath,
		Format:    "table",
	})
	if err != nil {
		t.Fatalf("ExecuteReportSummary failed: %v", err)
	}
	if !resReport.Success {
		t.Errorf("expected report success, got false")
	}
}

func TestStageRegistryAndLambdaDispatcher(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	payload, err := json.Marshal(map[string]interface{}{
		"mock":       true,
		"outputPath": filepath.Join(tempDir, "lambda_sense_ce.parquet"),
	})
	if err != nil {
		t.Fatalf("failed marshaling payload: %v", err)
	}

	// Test ExecuteStage dispatch
	res, err := stages.ExecuteStage(ctx, types.StageFinOpsSenseCostExplorer, payload)
	if err != nil {
		t.Fatalf("ExecuteStage failed: %v", err)
	}
	if !res.Success || res.RecordCount == 0 {
		t.Errorf("expected successful execution with records, got %+v", res)
	}

	// Test LambdaHandler with embedded stage in JSON
	eventJSON := []byte(`{"stage": "finops-sense-costanomaly", "mock": true}`)
	resLambda, err := stages.LambdaHandler(ctx, eventJSON)
	if err != nil {
		t.Fatalf("LambdaHandler failed: %v", err)
	}
	if !resLambda.Success {
		t.Errorf("expected lambda handler success, got %+v", resLambda)
	}

	// Test LambdaHandler with environment variable OPENLZ_STAGE
	os.Setenv("OPENLZ_STAGE", types.StageSecOpsSenseVulner)
	defer os.Unsetenv("OPENLZ_STAGE")

	resEnv, err := stages.LambdaHandler(ctx, []byte(`{"mock": true}`))
	if err != nil {
		t.Fatalf("LambdaHandler with env failed: %v", err)
	}
	if !resEnv.Success {
		t.Errorf("expected lambda env success, got %+v", resEnv)
	}
}
