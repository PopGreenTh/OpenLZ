package aws

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

const DEFAULT_REGION = "ap-southeast-1"

// Client wraps AWS SDK v2 service clients with resilient retries.
type Client struct {
	cfg          aws.Config
	costExplorer *costexplorer.Client
	ec2Client    *ec2.Client
	s3Client     *s3.Client
	iamClient    *iam.Client
	stsClient    *sts.Client
	isMock       bool
}

// NewClient initializes the AWS SDK v2 client configuration.
func NewClient(ctx context.Context, region string, mock bool) (*Client, error) {
	if mock {
		slog.DebugContext(ctx, "Initializing AWS client in MOCK mode")
		return &Client{isMock: true}, nil
	}

	if region == "" || region == "us-east-1" {
		if envRegion := os.Getenv("AWS_REGION"); envRegion != "" {
			region = envRegion
		} else if envRegion := os.Getenv("AWS_DEFAULT_REGION"); envRegion != "" {
			region = envRegion
		} else {
			region = DEFAULT_REGION
		}
	}

	slog.InfoContext(ctx, "Initializing live AWS SDK client", "region", region)
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		slog.ErrorContext(ctx, "Failed loading AWS SDK config", "error", err, "region", region)
		return nil, fmt.Errorf("failed to load AWS SDK config: %w", err)
	}
	slog.DebugContext(ctx, "AWS SDK config loaded successfully", "region", cfg.Region)

	return &Client{
		cfg:          cfg,
		costExplorer: costexplorer.NewFromConfig(cfg),
		ec2Client:    ec2.NewFromConfig(cfg),
		s3Client:     s3.NewFromConfig(cfg),
		iamClient:    iam.NewFromConfig(cfg),
		stsClient:    sts.NewFromConfig(cfg),
		isMock:       false,
	}, nil
}

// GetCallerAccountID retrieves the AWS Account ID associated with the current credentials via STS.
func (c *Client) GetCallerAccountID(ctx context.Context) (string, error) {
	if c.isMock {
		return "111122223333", nil
	}
	if c.stsClient == nil {
		return "", fmt.Errorf("STS client is not initialized")
	}

	var accountID string
	err := ExecuteWithRetry(ctx, func() error {
		out, err := c.stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
		if err != nil {
			return err
		}
		if out.Account != nil {
			accountID = *out.Account
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("failed to get caller identity via STS: %w", err)
	}
	if accountID == "" {
		return "", fmt.Errorf("caller identity returned empty account ID")
	}
	return accountID, nil
}

// ExecuteWithRetry wraps any AWS SDK call with exponential backoff retries.
func ExecuteWithRetry(ctx context.Context, fn func() error) error {
	return retry.Do(
		fn,
		retry.Context(ctx),
		retry.Attempts(3),
		retry.Delay(500*time.Millisecond),
		retry.DelayType(retry.BackOffDelay),
	)
}
