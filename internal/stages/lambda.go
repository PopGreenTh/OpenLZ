package stages

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/lambda"
)

// IsLambdaEnvironment returns true if running inside the AWS Lambda execution environment.
func IsLambdaEnvironment() bool {
	return os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != "" || os.Getenv("_LAMBDA_SERVER_PORT") != ""
}

// ResolveStageFromEnv detects which stage to run based on environment variables.
func ResolveStageFromEnv() string {
	if stage := os.Getenv("OPENLZ_STAGE"); stage != "" {
		return strings.ToLower(strings.TrimSpace(stage))
	}
	// Check if Lambda function name contains or matches a stage
	fnName := strings.ToLower(os.Getenv("AWS_LAMBDA_FUNCTION_NAME"))
	for _, s := range ListStages() {
		if strings.Contains(fnName, s) {
			return s
		}
	}
	return ""
}

// LambdaHandler handles incoming AWS Lambda events.
func LambdaHandler(ctx context.Context, event json.RawMessage) (*StageResult, error) {
	stageName := ResolveStageFromEnv()

	// Check if the event JSON itself specifies a target stage: {"stage": "finops-sense-costexplorer", ...}
	var probe struct {
		Stage string `json:"stage"`
	}
	if len(event) > 0 {
		_ = json.Unmarshal(event, &probe)
		if probe.Stage != "" {
			stageName = strings.ToLower(strings.TrimSpace(probe.Stage))
		}
	}

	if stageName == "" {
		return nil, fmt.Errorf("could not determine stage from environment (OPENLZ_STAGE / AWS_LAMBDA_FUNCTION_NAME) or event JSON: %s", string(event))
	}

	return ExecuteStage(ctx, stageName, event)
}

// StartLambdaRuntime starts the AWS Lambda event listener.
func StartLambdaRuntime() {
	lambda.Start(LambdaHandler)
}
