package inventory

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/aws/smithy-go"
)

type throttlingError struct{}

func (throttlingError) Error() string                 { return "ThrottlingException: rate exceeded" }
func (throttlingError) ErrorCode() string             { return "ThrottlingException" }
func (throttlingError) ErrorMessage() string          { return "rate exceeded" }
func (throttlingError) ErrorFault() smithy.ErrorFault { return smithy.FaultClient }

type fakeOrganizations struct {
	ouPages      map[string][][]OU
	accountPages map[string][][]Account
	throttleOU   int32
}

func (f *fakeOrganizations) DescribeOrganization(ctx context.Context) (string, string, error) {
	return "o-fake", "111111111111", nil
}

func (f *fakeOrganizations) ListRoots(ctx context.Context) (string, error) {
	return "r-fake", nil
}

func (f *fakeOrganizations) ListOrganizationalUnitsForParent(ctx context.Context, parentID, nextToken string) ([]OU, string, error) {
	if parentID == "r-fake" && atomic.AddInt32(&f.throttleOU, 1) == 1 {
		return nil, "", throttlingError{}
	}
	pages := f.ouPages[parentID]
	if len(pages) == 0 {
		return nil, "", nil
	}
	page := 0
	if nextToken != "" {
		fmt.Sscanf(nextToken, "page-%d", &page)
	}
	if page >= len(pages) {
		return nil, "", nil
	}
	next := ""
	if page+1 < len(pages) {
		next = fmt.Sprintf("page-%d", page+1)
	}
	return pages[page], next, nil
}

func (f *fakeOrganizations) ListAccountsForParent(ctx context.Context, parentID, nextToken string) ([]Account, string, error) {
	pages := f.accountPages[parentID]
	if len(pages) == 0 {
		return nil, "", nil
	}
	page := 0
	if nextToken != "" {
		fmt.Sscanf(nextToken, "page-%d", &page)
	}
	if page >= len(pages) {
		return nil, "", nil
	}
	next := ""
	if page+1 < len(pages) {
		next = fmt.Sprintf("page-%d", page+1)
	}
	return pages[page], next, nil
}

func TestDiscoverOrganizationPagingAndRetry(t *testing.T) {
	fake := &fakeOrganizations{
		ouPages: map[string][][]OU{
			"r-fake": {
				{{ID: "ou-wkld", Name: "Workloads", ParentID: "r-fake"}},
				{{ID: "ou-sec", Name: "Security", ParentID: "r-fake"}},
			},
			"ou-wkld": {
				{{ID: "ou-app", Name: "App", ParentID: "ou-wkld"}},
			},
		},
		accountPages: map[string][][]Account{
			"r-fake": {
				{{ID: "111111111111", Name: "mgmt", Status: "ACTIVE", ParentID: "r-fake"}},
			},
			"ou-app": {
				{{ID: "222222222222", Name: "workload", Status: "ACTIVE", ParentID: "ou-app"}},
			},
		},
	}

	org, err := DiscoverOrganization(context.Background(), fake)
	if err != nil {
		t.Fatalf("DiscoverOrganization: %v", err)
	}
	if len(org.OUs) != 3 {
		t.Fatalf("OUs = %d, want 3", len(org.OUs))
	}
	if len(org.Accounts) != 2 {
		t.Fatalf("accounts = %d, want 2", len(org.Accounts))
	}
	if fake.throttleOU < 2 {
		t.Fatalf("expected throttled OU list to retry, calls=%d", fake.throttleOU)
	}
}
