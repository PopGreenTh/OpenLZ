package inventory

import "time"

// SourceAWS marks inventory collected from a live AWS Organization.
const SourceAWS = "aws"

// SourceMock marks inventory produced from the demo fixture.
const SourceMock = "mock"

// Document is the on-disk inventory file (WI-0102).
type Document struct {
	CollectedAt   time.Time      `json:"collected_at"`
	Source        string         `json:"source"`
	Organizations []Organization `json:"organizations"`
	PathFindings  []PathFinding  `json:"-"`
}

// Organization is one AWS Organization snapshot.
type Organization struct {
	ID                string    `json:"id"`
	ManagementAccount string    `json:"management_account"`
	RootID            string    `json:"root_id"`
	OUs               []OU      `json:"ous"`
	Accounts          []Account `json:"accounts"`
}

// OU is an organizational unit in the tree.
type OU struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	ParentID string `json:"parent_id"`
}

// Account is a member account in the organization.
type Account struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Email    string `json:"email,omitempty"`
	Status   string `json:"status"`
	ParentID string `json:"parent_id"`
}

// PathFinding records OU path issues discovered while indexing.
type PathFinding struct {
	Code    string `json:"code"`
	Subject string `json:"subject"`
	Message string `json:"message"`
}
