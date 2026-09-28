package aws

import (
	"context"

	"github.com/PopGreenTh/OpenLZ/internal/cloud"
)

// FetchVulnerabilityFindings queries vulnerability findings or returns mock CVE records.
func (c *Client) FetchVulnerabilityFindings(ctx context.Context, accountID string) ([]cloud.SecurityRecord, error) {
	return GenerateMockVulnerabilityRecords(accountID), nil
}
