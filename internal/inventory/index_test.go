package inventory

import "testing"

func TestBuildIndexPathsAndLookups(t *testing.T) {
	org := Organization{
		ID:     "o-test",
		RootID: "r-test",
		OUs: []OU{
			{ID: "ou-wkld", Name: "Workloads", ParentID: "r-test"},
			{ID: "ou-chk", Name: "Checkout", ParentID: "ou-wkld"},
			{ID: "ou-prod", Name: "Prod", ParentID: "ou-chk"},
		},
		Accounts: []Account{
			{ID: "111111111111", Name: "acct", Status: "ACTIVE", ParentID: "ou-prod"},
		},
	}
	idx, findings := BuildIndex(org)
	if len(findings) != 0 {
		t.Fatalf("unexpected findings: %+v", findings)
	}
	ou, ok := idx.OUByPath("/Workloads/Checkout/Prod")
	if !ok || ou.ID != "ou-prod" {
		t.Fatalf("OUByPath: got %+v ok=%v", ou, ok)
	}
	if !idx.HasChildOUs("ou-wkld") {
		t.Fatal("expected workloads to have child OUs")
	}
	if idx.HasChildOUs("ou-prod") {
		t.Fatal("prod OU should have no child OUs")
	}
	chain, ok := idx.AccountOUChain("111111111111")
	if !ok || len(chain) != 3 {
		t.Fatalf("AccountOUChain: %+v ok=%v", chain, ok)
	}
	if got := idx.AccountPath("111111111111"); got != "/Workloads/Checkout/Prod" {
		t.Fatalf("AccountPath = %q", got)
	}
}

func TestBuildIndexAmbiguousSiblingNames(t *testing.T) {
	org := Organization{
		ID:     "o-test",
		RootID: "r-test",
		OUs: []OU{
			{ID: "ou-a", Name: "Shared", ParentID: "r-test"},
			{ID: "ou-b", Name: "Shared", ParentID: "r-test"},
		},
	}
	idx, findings := BuildIndex(org)
	if len(findings) != 1 || findings[0].Code != findingOUPathAmbiguous {
		t.Fatalf("findings = %+v", findings)
	}
	if _, ok := idx.OUByPath("/Shared"); ok {
		t.Fatal("ambiguous path should not resolve")
	}
}
