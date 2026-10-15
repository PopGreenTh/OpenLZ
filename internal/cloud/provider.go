// Package cloud defines core interfaces and standardized data structures
// across FinOps, SecOps, and CloudOps domains in OpenLZ.
package cloud

import (
	"context"
	"time"
)

// CostRecord represents normalized cloud billing and usage data.
type CostRecord struct {
	AccountID    string            `json:"account_id"`
	AccountName  string            `json:"account_name"`
	Service      string            `json:"service"`
	UsageDate    string            `json:"usage_date"`              // YYYY-MM-DD
	UsagePeriod  string            `json:"usage_period,omitempty"`  // Specific period identifier (e.g. hourly ISO8601 or month YYYY-MM)
	Amount       float64           `json:"amount"`                  // Raw amount for the specific period
	DailyAmount  float64           `json:"daily_amount"`            // Normalized daily cost or usage for comparability across frequencies
	DaysInPeriod float64           `json:"days_in_period"`          // Divisor: 1/24 (HOURLY), 1.0 (DAILY), 28..31 (MONTHLY)
	Metric       string            `json:"metric"`                  // e.g. "UnblendedCost", "UsageQuantity", "AmortizedCost"
	Frequency    string            `json:"frequency"`               // "HOURLY", "DAILY", "MONTHLY"
	Unit         string            `json:"unit"`                    // e.g. USD, Hours, GB-Mo, Requests
	Currency     string            `json:"currency"`                // USD
	CSP          string            `json:"csp,omitempty"`           // Cloud service provider, e.g. AWS
	ConnectionID string            `json:"connection_id,omitempty"` // Billing connection: the payer account id in payer mode
	Region       string            `json:"region"`
	Zone         string            `json:"zone,omitempty"` // Availability zone, set with the SERVICE_AZ group-by
	UsageType    string            `json:"usage_type"`
	ResourceID   string            `json:"resource_id,omitempty"` // Set when sensed with resource-level granularity
	PrimaryTag   string            `json:"primary_tag,omitempty"`
	SecondaryTag string            `json:"secondary_tag,omitempty"`
	BusinessUnit string            `json:"business_unit,omitempty"`
	Project      string            `json:"project,omitempty"`
	Application  string            `json:"application,omitempty"`
	Environment  string            `json:"environment,omitempty"`
	Owner        string            `json:"owner,omitempty"`
	CostCenter   string            `json:"cost_center,omitempty"`
	Tags         map[string]string `json:"tags,omitempty"`
	Timestamp    time.Time         `json:"timestamp"`
}

// SecurityRecord represents security posture findings and compliance violations.
type SecurityRecord struct {
	AccountID   string    `json:"account_id"`
	AccountName string    `json:"account_name"`
	Service     string    `json:"service"`
	ResourceID  string    `json:"resource_id"`
	FindingType string    `json:"finding_type"` // e.g. PublicBucket, OpenSecurityGroup, InactiveCredentials
	Severity    string    `json:"severity"`     // CRITICAL, HIGH, MEDIUM, LOW
	Description string    `json:"description"`
	Remediation string    `json:"remediation"`
	Timestamp   time.Time `json:"timestamp"`
}

// CloudOpsRecord represents operational hygiene, waste, and reliability findings.
type CloudOpsRecord struct {
	AccountID      string    `json:"account_id"`
	AccountName    string    `json:"account_name"`
	Service        string    `json:"service"`
	ResourceID     string    `json:"resource_id"`
	IssueType      string    `json:"issue_type"` // e.g. OrphanedVolume, StaleSnapshot, UnattachedEIP
	ResourceAge    int       `json:"resource_age_days"`
	EstimatedWaste float64   `json:"estimated_waste_usd"`
	Recommendation string    `json:"recommendation"`
	Timestamp      time.Time `json:"timestamp"`
}

// AnomalyRecord represents detected deviations, anomalies, or high-risk findings.
type AnomalyRecord struct {
	OpsDomain     string    `json:"ops_domain"` // finops, secops, cloudops
	AccountID     string    `json:"account_id"`
	AccountName   string    `json:"account_name"`
	Service       string    `json:"service"`
	ResourceOrKey string    `json:"resource_or_key"`
	Metric        string    `json:"metric"`
	ActualValue   float64   `json:"actual_value"`
	ExpectedValue float64   `json:"expected_value"`
	Deviation     float64   `json:"deviation_ratio"`
	Severity      string    `json:"severity"`
	Status        string    `json:"status"` // OPEN, RESOLVED, IGNORED
	Timestamp     time.Time `json:"timestamp"`
}

// EnrichedRecord represents an anomaly or finding joined with organizational metadata.
type EnrichedRecord struct {
	OpsDomain          string    `json:"ops_domain"`
	AccountID          string    `json:"account_id"`
	AccountName        string    `json:"account_name"`
	Environment        string    `json:"environment"`   // Production, Staging, Development, Sandbox
	BusinessUnit       string    `json:"business_unit"` // E-Commerce, Core Platform, Data & AI, Security
	Owner              string    `json:"owner"`         // Team or individual email
	CostCenter         string    `json:"cost_center"`
	Service            string    `json:"service"`
	ResourceOrKey      string    `json:"resource_or_key"`
	Metric             string    `json:"metric"`
	ActualValue        float64   `json:"actual_value"`
	ExpectedValue      float64   `json:"expected_value"`
	PotentialSavings   float64   `json:"potential_savings"`
	Severity           string    `json:"severity"`
	ActionRecommended  string    `json:"action_recommended"`
	ProjectID          string    `json:"project_id,omitempty"`
	Project            string    `json:"project,omitempty"`
	ApplicationID      string    `json:"application_id,omitempty"`
	Application        string    `json:"application,omitempty"`
	EnvironmentID      string    `json:"environment_id,omitempty"`
	ServiceCategoryID  string    `json:"service_category_id,omitempty"`
	ServiceCategory    string    `json:"service_category,omitempty"`
	BusinessUnitID     string    `json:"business_unit_id,omitempty"`
	OwnerID            string    `json:"owner_id,omitempty"`
	CostCenterID       string    `json:"cost_center_id,omitempty"`
	CustomerFunctionID string    `json:"customer_function_id,omitempty"`
	CustomerFunction   string    `json:"customer_function,omitempty"`
	CSPID              string    `json:"csp_id,omitempty"`
	CSP                string    `json:"csp,omitempty"`
	RegionID           string    `json:"region_id,omitempty"`
	Region             string    `json:"region,omitempty"`
	ServiceID          string    `json:"service_id,omitempty"`
	ServiceUsageID     string    `json:"service_usage_id,omitempty"`
	ServiceUsage       string    `json:"service_usage,omitempty"`
	UsageID            string    `json:"usage_id,omitempty"`
	Usage              string    `json:"usage,omitempty"`
	ResourceGroupID    string    `json:"resource_group_id,omitempty"`
	ResourceGroup      string    `json:"resource_group,omitempty"`
	ResourceID         string    `json:"resource_id,omitempty"`
	Resource           string    `json:"resource,omitempty"`
	FeatureID          string    `json:"feature_id,omitempty"`
	Feature            string    `json:"feature,omitempty"`
	FunctionID         string    `json:"function_id,omitempty"`
	Function           string    `json:"function,omitempty"`
	OwnerTeamID        string    `json:"owner_team_id,omitempty"`
	OwnerTeam          string    `json:"owner_team,omitempty"`
	ProductLineID      string    `json:"product_line_id,omitempty"`
	ProductLine        string    `json:"product_line,omitempty"`
	CustomerID         string    `json:"customer_id,omitempty"`
	Customer           string    `json:"customer,omitempty"`
	LZID               string    `json:"lz_id,omitempty"`
	LZ                 string    `json:"lz,omitempty"`
	ConnectionID       string    `json:"connection_id,omitempty"`
	Connection         string    `json:"connection,omitempty"`
	BillingCurrency    string    `json:"billing_currency,omitempty"`
	ZoneID             string    `json:"zone_id,omitempty"`
	Zone               string    `json:"zone,omitempty"`
	AttributionTrail   string    `json:"attribution_trail,omitempty"`
	OUID               string    `json:"ou_id,omitempty"`
	OU                 string    `json:"ou,omitempty"`
	Timestamp          time.Time `json:"timestamp"`
}

// EnforceAuditRecord captures the result of policy enforcement or remediation.
type EnforceAuditRecord struct {
	Timestamp   time.Time `json:"timestamp"`
	OpsDomain   string    `json:"ops_domain"`
	AccountID   string    `json:"account_id"`
	Service     string    `json:"service"`
	ResourceID  string    `json:"resource_id"`
	ActionTaken string    `json:"action_taken"`
	DryRun      bool      `json:"dry_run"`
	Success     bool      `json:"success"`
	Details     string    `json:"details"`
}

// CloudProvider is the top-level interface unifying multi-cloud implementations.
type CloudProvider interface {
	Name() string
	FetchCosts(ctx context.Context, accountID, startDate, endDate string) ([]CostRecord, error)
	ScanSecurity(ctx context.Context, accountID string) ([]SecurityRecord, error)
	ScanOperations(ctx context.Context, accountID string) ([]CloudOpsRecord, error)
	Remediate(ctx context.Context, audit EnforceAuditRecord, dryRun bool) (EnforceAuditRecord, error)
}
