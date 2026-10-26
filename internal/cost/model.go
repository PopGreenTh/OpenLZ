package cost

// RawRow is one row in cost_raw.parquet (WI-0104).
type RawRow struct {
	LZ        string  `json:"lz" yaml:"lz"`
	AccountID string  `json:"account_id" yaml:"account_id"`
	Month     string  `json:"month" yaml:"month"`
	Metric    string  `json:"metric" yaml:"metric"`
	Amount    float64 `json:"amount" yaml:"amount"`
	AppTag    string  `json:"app_tag" yaml:"app_tag"`
	Currency  string  `json:"currency" yaml:"currency"`
	Estimated bool    `json:"estimated" yaml:"estimated"`
}

// AllocatedRow is one row in cost_allocated.parquet.
type AllocatedRow struct {
	LZ              string  `json:"lz"`
	AccountID       string  `json:"account_id"`
	AccountName     string  `json:"account_name,omitempty"`
	OUPath          string  `json:"ou_path,omitempty"`
	Project         string  `json:"project"`
	App             string  `json:"app"`
	Environment     string  `json:"environment"`
	BusinessUnit    string  `json:"business_unit"`
	CostCenter      string  `json:"cost_center"`
	Share           float64 `json:"share"`
	ShareRule       string  `json:"share_rule"`
	AppRule         string  `json:"app_rule"`
	EnvironmentRule string  `json:"environment_rule"`
	AppTag          string  `json:"app_tag"`
	Month           string  `json:"month"`
	Metric          string  `json:"metric"`
	Amount          float64 `json:"amount"`
	Currency        string  `json:"currency"`
	Estimated       bool    `json:"estimated"`
	TaxonomyHash    string  `json:"taxonomy_hash"`
}

// Finding is a cost-step finding.
type Finding struct {
	Code    string `json:"code"`
	Subject string `json:"subject"`
	Message string `json:"message"`
}

// AllocateResult holds allocated rows and findings.
type AllocateResult struct {
	Rows     []AllocatedRow
	Findings []Finding
}
