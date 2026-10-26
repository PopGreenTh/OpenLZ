package inventory

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/organizations"
)

// AWSOrganizations adapts the AWS SDK Organizations client to OrganizationsAPI.
type AWSOrganizations struct {
	Client *organizations.Client
}

// DescribeOrganization returns organization ID and management account ID.
func (a *AWSOrganizations) DescribeOrganization(ctx context.Context) (string, string, error) {
	out, err := a.Client.DescribeOrganization(ctx, &organizations.DescribeOrganizationInput{})
	if err != nil {
		return "", "", err
	}
	if out.Organization == nil {
		return "", "", fmt.Errorf("describe organization: empty response")
	}
	return stringValue(out.Organization.Id), stringValue(out.Organization.MasterAccountId), nil
}

// ListRoots returns the organization root ID.
func (a *AWSOrganizations) ListRoots(ctx context.Context) (string, error) {
	out, err := a.Client.ListRoots(ctx, &organizations.ListRootsInput{})
	if err != nil {
		return "", err
	}
	if len(out.Roots) == 0 {
		return "", fmt.Errorf("list roots: no roots returned")
	}
	return stringValue(out.Roots[0].Id), nil
}

// ListOrganizationalUnitsForParent lists one page of OUs for a parent.
func (a *AWSOrganizations) ListOrganizationalUnitsForParent(ctx context.Context, parentID, nextToken string) ([]OU, string, error) {
	in := &organizations.ListOrganizationalUnitsForParentInput{
		ParentId: &parentID,
	}
	if nextToken != "" {
		in.NextToken = &nextToken
	}
	out, err := a.Client.ListOrganizationalUnitsForParent(ctx, in)
	if err != nil {
		return nil, "", err
	}
	ous := make([]OU, 0, len(out.OrganizationalUnits))
	for _, u := range out.OrganizationalUnits {
		ous = append(ous, OU{
			ID:       stringValue(u.Id),
			Name:     stringValue(u.Name),
			ParentID: parentID,
		})
	}
	return ous, stringValue(out.NextToken), nil
}

// ListAccountsForParent lists one page of accounts for a parent.
func (a *AWSOrganizations) ListAccountsForParent(ctx context.Context, parentID, nextToken string) ([]Account, string, error) {
	in := &organizations.ListAccountsForParentInput{
		ParentId: &parentID,
	}
	if nextToken != "" {
		in.NextToken = &nextToken
	}
	out, err := a.Client.ListAccountsForParent(ctx, in)
	if err != nil {
		return nil, "", err
	}
	accounts := make([]Account, 0, len(out.Accounts))
	for _, acct := range out.Accounts {
		accounts = append(accounts, Account{
			ID:       stringValue(acct.Id),
			Name:     stringValue(acct.Name),
			Email:    stringValue(acct.Email),
			Status:   string(acct.Status),
			ParentID: parentID,
		})
	}
	return accounts, stringValue(out.NextToken), nil
}

func stringValue(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// Ensure AWSOrganizations implements OrganizationsAPI.
var _ OrganizationsAPI = (*AWSOrganizations)(nil)
