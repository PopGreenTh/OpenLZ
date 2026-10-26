package inventory

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/PopGreenTh/OpenLZ/internal/awsx"
)

const discoveryWorkers = 5

// OrganizationsAPI is the subset of AWS Organizations used for discovery (mockable in tests).
type OrganizationsAPI interface {
	DescribeOrganization(ctx context.Context) (orgID, managementAccount string, err error)
	ListRoots(ctx context.Context) (rootID string, err error)
	ListOrganizationalUnitsForParent(ctx context.Context, parentID, nextToken string) (ous []OU, token string, err error)
	ListAccountsForParent(ctx context.Context, parentID, nextToken string) (accounts []Account, token string, err error)
}

// DiscoverOrganization walks the AWS Organization tree breadth-first with bounded concurrency.
func DiscoverOrganization(ctx context.Context, api OrganizationsAPI) (Organization, error) {
	var orgID, mgmt string
	err := awsx.ExecuteWithRetry(ctx, func(ctx context.Context) error {
		var e error
		orgID, mgmt, e = api.DescribeOrganization(ctx)
		return e
	})
	if err != nil {
		return Organization{}, fmt.Errorf("describe organization: %w", err)
	}

	var rootID string
	err = awsx.ExecuteWithRetry(ctx, func(ctx context.Context) error {
		var e error
		rootID, e = api.ListRoots(ctx)
		return e
	})
	if err != nil {
		return Organization{}, fmt.Errorf("list roots: %w", err)
	}

	org := Organization{
		ID:                orgID,
		ManagementAccount: mgmt,
		RootID:            rootID,
	}

	current := []string{rootID}
	seen := map[string]struct{}{rootID: {}}

	for len(current) > 0 {
		var mu sync.Mutex
		var next []string
		var firstErr error
		sem := make(chan struct{}, discoveryWorkers)
		var wg sync.WaitGroup

		for _, parentID := range current {
			parentID := parentID
			wg.Add(1)
			go func() {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				ous, accounts, err := listAllUnderParent(ctx, api, parentID)
				if err != nil {
					mu.Lock()
					if firstErr == nil {
						firstErr = err
					}
					mu.Unlock()
					return
				}
				mu.Lock()
				org.OUs = append(org.OUs, ous...)
				org.Accounts = append(org.Accounts, accounts...)
				for _, ou := range ous {
					if _, ok := seen[ou.ID]; ok {
						continue
					}
					seen[ou.ID] = struct{}{}
					next = append(next, ou.ID)
				}
				mu.Unlock()
			}()
		}
		wg.Wait()
		if firstErr != nil {
			return Organization{}, firstErr
		}
		current = next
	}

	normalized := NormalizeDocument(Document{Organizations: []Organization{org}})
	return normalized.Organizations[0], nil
}

func listAllUnderParent(ctx context.Context, api OrganizationsAPI, parentID string) ([]OU, []Account, error) {
	var allOUs []OU
	token := ""
	for {
		var ous []OU
		var next string
		err := awsx.ExecuteWithRetry(ctx, func(ctx context.Context) error {
			var e error
			ous, next, e = api.ListOrganizationalUnitsForParent(ctx, parentID, token)
			return e
		})
		if err != nil {
			return nil, nil, fmt.Errorf("list OUs for parent %s: %w", parentID, err)
		}
		allOUs = append(allOUs, ous...)
		if next == "" {
			break
		}
		token = next
	}

	var allAccounts []Account
	token = ""
	for {
		var accounts []Account
		var next string
		err := awsx.ExecuteWithRetry(ctx, func(ctx context.Context) error {
			var e error
			accounts, next, e = api.ListAccountsForParent(ctx, parentID, token)
			return e
		})
		if err != nil {
			return nil, nil, fmt.Errorf("list accounts for parent %s: %w", parentID, err)
		}
		allAccounts = append(allAccounts, accounts...)
		if next == "" {
			break
		}
		token = next
	}
	return allOUs, allAccounts, nil
}

// SenseAWS discovers the organization and wraps it in a Document.
func SenseAWS(ctx context.Context, api OrganizationsAPI) (*Document, error) {
	org, err := DiscoverOrganization(ctx, api)
	if err != nil {
		return nil, err
	}
	doc := &Document{
		CollectedAt:   time.Now().UTC().Truncate(time.Second),
		Source:        SourceAWS,
		Organizations: []Organization{org},
	}
	for i := range doc.Organizations {
		_, findings := BuildIndex(doc.Organizations[i])
		doc.PathFindings = append(doc.PathFindings, findings...)
	}
	normalized := NormalizeDocument(*doc)
	return &normalized, nil
}
