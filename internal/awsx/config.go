package awsx

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
)

const defaultOrganizationsRegion = "us-east-1"

// LoadConfig loads the default AWS config chain with optional profile and region.
// When region is empty, us-east-1 is used (Organizations API home region).
func LoadConfig(ctx context.Context, profile, region string) (aws.Config, error) {
	if region == "" {
		region = defaultOrganizationsRegion
	}
	opts := []func(*config.LoadOptions) error{
		config.WithRegion(region),
	}
	if profile != "" {
		opts = append(opts, config.WithSharedConfigProfile(profile))
	}
	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return aws.Config{}, fmt.Errorf("load AWS config: %w", err)
	}
	return cfg, nil
}
