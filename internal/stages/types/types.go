package types

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"
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

// FormatTimestamp returns a standardized cross-platform timestamp: YYYYMMDD_HHMMSS
func FormatTimestamp(t ...time.Time) string {
	target := time.Now()
	if len(t) > 0 && !t[0].IsZero() {
		target = t[0]
	}
	return target.Format("20060102_150405")
}

// AppendTimestampToPath appends a formatted datetime stamp (_YYYYMMDD_HHMMSS) to a file path before its extension.
// Example: "data/finops_raw.parquet" -> "data/finops_raw_20260929_115205.parquet"
func AppendTimestampToPath(filePath string, t ...time.Time) string {
	if filePath == "" {
		return ""
	}
	ts := FormatTimestamp(t...)
	ext := filepath.Ext(filePath)
	base := strings.TrimSuffix(filePath, ext)
	return fmt.Sprintf("%s_%s%s", base, ts, ext)
}

// AppendTenantToPath inserts a tenant name into a file path before its extension.
// Example: "data/finops_raw.parquet" with tenant "acme" -> "data/finops_raw_acme.parquet"
// If tenant is empty or filePath is empty, filePath is returned unmodified.
func AppendTenantToPath(filePath string, tenant string) string {
	tenant = strings.TrimSpace(tenant)
	if filePath == "" || tenant == "" {
		return filePath
	}
	ext := filepath.Ext(filePath)
	base := strings.TrimSuffix(filePath, ext)
	if strings.HasSuffix(base, "_"+tenant) {
		return filePath
	}
	return fmt.Sprintf("%s_%s%s", base, tenant, ext)
}

// StripExtensions removes export extensions (.parquet, .csv, .xlsx, .m) from a path.
func StripExtensions(p string) string {
	lower := strings.ToLower(p)
	for _, ext := range []string{".parquet", ".csv", ".xlsx", ".m"} {
		if strings.HasSuffix(lower, ext) {
			return p[:len(p)-len(ext)]
		}
	}
	return p
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
