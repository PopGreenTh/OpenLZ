package stages

import (
	"github.com/PopGreenTh/OpenLZ/internal/stages/types"
)

// Re-exported constants from types package
const (
	StageFinOpsSenseCostExplorer     = types.StageFinOpsSenseCostExplorer
	StageFinOpsSenseCostAnomaly      = types.StageFinOpsSenseCostAnomaly
	StageFinOpsAnalyzeVarianceImpact = types.StageFinOpsAnalyzeVarianceImpact
	StageFinOpsEnrichCostCenters     = types.StageFinOpsEnrichCostCenters
	StageFinOpsEnforceTagging        = types.StageFinOpsEnforceTagging
	StageFinOpsReportSummary         = types.StageFinOpsReportSummary

	StageSecOpsSensePosture          = types.StageSecOpsSensePosture
	StageSecOpsSenseVulner           = types.StageSecOpsSenseVulner
	StageSecOpsAnalysisRiskPosture   = types.StageSecOpsAnalysisRiskPosture
	StageSecOpsEnrichMetadata        = types.StageSecOpsEnrichMetadata
	StageSecOpsEnforceRemediation    = types.StageSecOpsEnforceRemediation
	StageSecOpsReportSummary         = types.StageSecOpsReportSummary

	StageCloudOpsSenseHygiene        = types.StageCloudOpsSenseHygiene
	StageCloudOpsAnalysisWasteHygiene = types.StageCloudOpsAnalysisWasteHygiene
	StageCloudOpsEnrichLifecycle     = types.StageCloudOpsEnrichLifecycle
	StageCloudOpsEnforceCleanup      = types.StageCloudOpsEnforceCleanup
	StageCloudOpsReportSummary       = types.StageCloudOpsReportSummary
)

type HandlerFunc = types.HandlerFunc
type StageEvent = types.StageEvent
type StageResult = types.StageResult
