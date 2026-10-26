package mapping

import "time"

// Organization is one AWS Organization snapshot for mapping (WI-0102 shape).
type Organization struct {
	ID                string    `json:"id" yaml:"id"`
	ManagementAccount string    `json:"management_account" yaml:"management_account"`
	RootID            string    `json:"root_id" yaml:"root_id"`
	OUs               []OU      `json:"ous" yaml:"ous"`
	Accounts          []Account `json:"accounts" yaml:"accounts"`
}

// OU is an organizational unit.
type OU struct {
	ID       string `json:"id" yaml:"id"`
	Name     string `json:"name" yaml:"name"`
	ParentID string `json:"parent_id" yaml:"parent_id"`
}

// Account is a member account.
type Account struct {
	ID       string `json:"id" yaml:"id"`
	Name     string `json:"name" yaml:"name"`
	Email    string `json:"email,omitempty" yaml:"email,omitempty"`
	Status   string `json:"status" yaml:"status"`
	ParentID string `json:"parent_id" yaml:"parent_id"`
}

// Inventory holds organizations and optional path findings from discovery.
type Inventory struct {
	CollectedAt   time.Time
	Organizations []Organization
	PathFindings  []Finding
}

// Severity of a mapping finding.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)

// Finding is a resolver or inventory finding.
type Finding struct {
	Severity Severity `json:"severity"`
	Code     string   `json:"code"`
	Subject  string   `json:"subject"`
	Message  string   `json:"message"`
}

// Allocation is one account-to-taxonomy mapping row.
type Allocation struct {
	LZ              string   `json:"lz"`
	AccountID       string   `json:"account_id"`
	AccountName     string   `json:"account_name,omitempty"`
	OUPath          string   `json:"ou_path,omitempty"`
	Project         string   `json:"project"`
	App             string   `json:"app"`
	Environment     string   `json:"environment"`
	BusinessUnit    string   `json:"business_unit"`
	CostCenter      string   `json:"cost_center"`
	Share           *float64 `json:"share"`
	ShareRule       string   `json:"share_rule,omitempty"`
	AppRule         string   `json:"app_rule"`
	EnvironmentRule string   `json:"environment_rule"`
}

// AccountMapDocument is the on-disk account map file.
type AccountMapDocument struct {
	GeneratedAt          time.Time    `json:"generated_at"`
	TaxonomyHash         string       `json:"taxonomy_hash"`
	InventoryCollectedAt time.Time    `json:"inventory_collected_at"`
	Allocations          []Allocation `json:"allocations"`
	Findings             []Finding    `json:"findings"`
}

// ResolveResult holds resolver output.
type ResolveResult struct {
	Allocations []Allocation
	Findings    []Finding
	Conflict    *Finding
}
