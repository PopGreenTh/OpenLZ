package stages

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	cloudopsstages "github.com/PopGreenTh/OpenLZ/internal/stages/cloudops"
	finopsstages "github.com/PopGreenTh/OpenLZ/internal/stages/finops"
	secopsstages "github.com/PopGreenTh/OpenLZ/internal/stages/secops"
)

var registry = map[string]HandlerFunc{
	// FinOps
	StageFinOpsSenseCostExplorer: func(ctx context.Context, payload []byte) (interface{}, error) {
		var in finopsstages.SenseCostExplorerInput
		if len(payload) > 0 {
			if err := json.Unmarshal(payload, &in); err != nil {
				return nil, fmt.Errorf("invalid json payload for %s: %w", StageFinOpsSenseCostExplorer, err)
			}
		}
		return finopsstages.ExecuteSenseCostExplorer(ctx, in)
	},
	StageFinOpsSenseCostAnomaly: func(ctx context.Context, payload []byte) (interface{}, error) {
		var in finopsstages.SenseCostAnomalyInput
		if len(payload) > 0 {
			if err := json.Unmarshal(payload, &in); err != nil {
				return nil, fmt.Errorf("invalid json payload for %s: %w", StageFinOpsSenseCostAnomaly, err)
			}
		}
		return finopsstages.ExecuteSenseCostAnomaly(ctx, in)
	},
	StageFinOpsAnalyzeVarianceImpact: func(ctx context.Context, payload []byte) (interface{}, error) {
		var in finopsstages.AnalyzeVarianceImpactInput
		if len(payload) > 0 {
			if err := json.Unmarshal(payload, &in); err != nil {
				return nil, fmt.Errorf("invalid json payload for %s: %w", StageFinOpsAnalyzeVarianceImpact, err)
			}
		}
		return finopsstages.ExecuteAnalyzeVarianceImpact(ctx, in)
	},
	StageFinOpsEnrichCostCenters: func(ctx context.Context, payload []byte) (interface{}, error) {
		var in finopsstages.EnrichCostCentersInput
		if len(payload) > 0 {
			if err := json.Unmarshal(payload, &in); err != nil {
				return nil, fmt.Errorf("invalid json payload for %s: %w", StageFinOpsEnrichCostCenters, err)
			}
		}
		return finopsstages.ExecuteEnrichCostCenters(ctx, in)
	},
	StageFinOpsEnforceTagging: func(ctx context.Context, payload []byte) (interface{}, error) {
		var in finopsstages.EnforceTaggingInput
		if len(payload) > 0 {
			if err := json.Unmarshal(payload, &in); err != nil {
				return nil, fmt.Errorf("invalid json payload for %s: %w", StageFinOpsEnforceTagging, err)
			}
		}
		return finopsstages.ExecuteEnforceTagging(ctx, in)
	},
	StageFinOpsReportSummary: func(ctx context.Context, payload []byte) (interface{}, error) {
		var in finopsstages.ReportSummaryInput
		if len(payload) > 0 {
			if err := json.Unmarshal(payload, &in); err != nil {
				return nil, fmt.Errorf("invalid json payload for %s: %w", StageFinOpsReportSummary, err)
			}
		}
		return finopsstages.ExecuteReportSummary(ctx, in)
	},

	// SecOps
	StageSecOpsSensePosture: func(ctx context.Context, payload []byte) (interface{}, error) {
		var in secopsstages.SensePostureInput
		if len(payload) > 0 {
			if err := json.Unmarshal(payload, &in); err != nil {
				return nil, fmt.Errorf("invalid json payload for %s: %w", StageSecOpsSensePosture, err)
			}
		}
		return secopsstages.ExecuteSensePosture(ctx, in)
	},
	StageSecOpsSenseVulner: func(ctx context.Context, payload []byte) (interface{}, error) {
		var in secopsstages.SenseVulnerInput
		if len(payload) > 0 {
			if err := json.Unmarshal(payload, &in); err != nil {
				return nil, fmt.Errorf("invalid json payload for %s: %w", StageSecOpsSenseVulner, err)
			}
		}
		return secopsstages.ExecuteSenseVulner(ctx, in)
	},
	StageSecOpsAnalysisRiskPosture: func(ctx context.Context, payload []byte) (interface{}, error) {
		var in secopsstages.AnalysisRiskPostureInput
		if len(payload) > 0 {
			if err := json.Unmarshal(payload, &in); err != nil {
				return nil, fmt.Errorf("invalid json payload for %s: %w", StageSecOpsAnalysisRiskPosture, err)
			}
		}
		return secopsstages.ExecuteAnalysisRiskPosture(ctx, in)
	},
	StageSecOpsEnrichMetadata: func(ctx context.Context, payload []byte) (interface{}, error) {
		var in secopsstages.EnrichMetadataInput
		if len(payload) > 0 {
			if err := json.Unmarshal(payload, &in); err != nil {
				return nil, fmt.Errorf("invalid json payload for %s: %w", StageSecOpsEnrichMetadata, err)
			}
		}
		return secopsstages.ExecuteEnrichMetadata(ctx, in)
	},
	StageSecOpsEnforceRemediation: func(ctx context.Context, payload []byte) (interface{}, error) {
		var in secopsstages.EnforceRemediationInput
		if len(payload) > 0 {
			if err := json.Unmarshal(payload, &in); err != nil {
				return nil, fmt.Errorf("invalid json payload for %s: %w", StageSecOpsEnforceRemediation, err)
			}
		}
		return secopsstages.ExecuteEnforceRemediation(ctx, in)
	},
	StageSecOpsReportSummary: func(ctx context.Context, payload []byte) (interface{}, error) {
		var in secopsstages.ReportSummaryInput
		if len(payload) > 0 {
			if err := json.Unmarshal(payload, &in); err != nil {
				return nil, fmt.Errorf("invalid json payload for %s: %w", StageSecOpsReportSummary, err)
			}
		}
		return secopsstages.ExecuteReportSummary(ctx, in)
	},

	// CloudOps
	StageCloudOpsSenseHygiene: func(ctx context.Context, payload []byte) (interface{}, error) {
		var in cloudopsstages.SenseHygieneInput
		if len(payload) > 0 {
			if err := json.Unmarshal(payload, &in); err != nil {
				return nil, fmt.Errorf("invalid json payload for %s: %w", StageCloudOpsSenseHygiene, err)
			}
		}
		return cloudopsstages.ExecuteSenseHygiene(ctx, in)
	},
	StageCloudOpsAnalysisWasteHygiene: func(ctx context.Context, payload []byte) (interface{}, error) {
		var in cloudopsstages.AnalysisWasteHygieneInput
		if len(payload) > 0 {
			if err := json.Unmarshal(payload, &in); err != nil {
				return nil, fmt.Errorf("invalid json payload for %s: %w", StageCloudOpsAnalysisWasteHygiene, err)
			}
		}
		return cloudopsstages.ExecuteAnalysisWasteHygiene(ctx, in)
	},
	StageCloudOpsEnrichLifecycle: func(ctx context.Context, payload []byte) (interface{}, error) {
		var in cloudopsstages.EnrichLifecycleInput
		if len(payload) > 0 {
			if err := json.Unmarshal(payload, &in); err != nil {
				return nil, fmt.Errorf("invalid json payload for %s: %w", StageCloudOpsEnrichLifecycle, err)
			}
		}
		return cloudopsstages.ExecuteEnrichLifecycle(ctx, in)
	},
	StageCloudOpsEnforceCleanup: func(ctx context.Context, payload []byte) (interface{}, error) {
		var in cloudopsstages.EnforceCleanupInput
		if len(payload) > 0 {
			if err := json.Unmarshal(payload, &in); err != nil {
				return nil, fmt.Errorf("invalid json payload for %s: %w", StageCloudOpsEnforceCleanup, err)
			}
		}
		return cloudopsstages.ExecuteEnforceCleanup(ctx, in)
	},
	StageCloudOpsReportSummary: func(ctx context.Context, payload []byte) (interface{}, error) {
		var in cloudopsstages.ReportSummaryInput
		if len(payload) > 0 {
			if err := json.Unmarshal(payload, &in); err != nil {
				return nil, fmt.Errorf("invalid json payload for %s: %w", StageCloudOpsReportSummary, err)
			}
		}
		return cloudopsstages.ExecuteReportSummary(ctx, in)
	},
}

// GetHandler returns the handler for a given stage name, matching full or short names.
func GetHandler(stageName string) (HandlerFunc, bool) {
	norm := strings.ToLower(strings.TrimSpace(stageName))
	if h, ok := registry[norm]; ok {
		return h, true
	}
	for _, prefix := range []string{"finops-", "secops-", "cloudops-"} {
		if h, ok := registry[prefix+norm]; ok {
			return h, true
		}
	}
	return nil, false
}

// ListStages returns a slice of all registered stage names.
func ListStages() []string {
	var names []string
	for k := range registry {
		names = append(names, k)
	}
	return names
}

// ExecuteStage dispatches and executes the requested stage with payload.
func ExecuteStage(ctx context.Context, stageName string, payload []byte) (*StageResult, error) {
	handler, ok := GetHandler(stageName)
	if !ok {
		return nil, fmt.Errorf("unknown stage '%s'; available stages: %s", stageName, strings.Join(ListStages(), ", "))
	}
	res, err := handler(ctx, payload)
	if err != nil {
		return nil, err
	}
	if stageRes, ok := res.(*StageResult); ok {
		return stageRes, nil
	}
	return &StageResult{
		Stage:   stageName,
		Success: true,
		Data:    res,
	}, nil
}
