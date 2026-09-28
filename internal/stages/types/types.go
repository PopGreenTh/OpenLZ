package types

import (
	"context"
	"path/filepath"
)

// ResolveAbsolutePath converts any relative path to a clean absolute path.
func ResolveAbsolutePath(p string) string {
	if p == "" {
		return ""
	}
	if abs, err := filepath.Abs(p); err == nil {
		return filepath.Clean(abs)
	}
	return filepath.Clean(p)
}

// Standardized Dedicated Stage Names
const (
	// FinOps Dedicated Stages
	StageFinOpsSenseCostExplorer     = "finops-sense-costexplorer"
	StageFinOpsSenseCostAnomaly      = "finops-sense-costanomaly"
	StageFinOpsAnalyzeVarianceImpact = "finops-analyze-varianceimpact"
	StageFinOpsEnrichCostCenters     = "finops-enrich-costcenters"
	StageFinOpsEnforceTagging        = "finops-enforce-tagging"
	StageFinOpsReportSummary         = "finops-report-summary"

	// SecOps Dedicated Stages
	StageSecOpsSensePosture          = "secops-sense-posture"
	StageSecOpsSenseVulner           = "secops-sense-vulner"
	StageSecOpsAnalysisRiskPosture   = "secops-analysis-riskposture"
	StageSecOpsEnrichMetadata        = "secops-enrich-metadata"
	StageSecOpsEnforceRemediation    = "secops-enforce-remediation"
	StageSecOpsReportSummary         = "secops-report-summary"

	// CloudOps Dedicated Stages
	StageCloudOpsSenseHygiene        = "cloudops-sense-hygiene"
	StageCloudOpsAnalysisWasteHygiene = "cloudops-analysis-wastehygiene"
	StageCloudOpsEnrichLifecycle     = "cloudops-enrich-lifecycle"
	StageCloudOpsEnforceCleanup      = "cloudops-enforce-cleanup"
	StageCloudOpsReportSummary       = "cloudops-report-summary"
)

// HandlerFunc is the uniform signature for invoking a stage with JSON-encoded input.
type HandlerFunc func(ctx context.Context, payload []byte) (interface{}, error)

// StageEvent models an incoming event payload for AWS Lambda or generic invocation.
type StageEvent struct {
	Stage   string                 `json:"stage,omitempty"`
	Payload map[string]interface{} `json:"payload,omitempty"`
}

// StageResult provides a standardized response format for Lambda and CLI execution.
type StageResult struct {
	Stage       string      `json:"stage"`
	Success     bool        `json:"success"`
	RecordCount int         `json:"recordCount,omitempty"`
	OutputPath  string      `json:"outputPath,omitempty"`
	Message     string      `json:"message,omitempty"`
	Data        interface{} `json:"data,omitempty"`
}
