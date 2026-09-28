package aws

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/PopGreenTh/OpenLZ/internal/cloud"
)

// GenerateMockCostRecords generates representative Landing Zone FinOps records.
func GenerateMockCostRecords(accountID, startDate, endDate string) []cloud.CostRecord {
	return GenerateMockCostRecordsWithOptions(FetchCostOptions{
		AccountID: accountID,
		StartDate: startDate,
		EndDate:   endDate,
	})
}

// GenerateMockCostRecordsWithOptions generates realistic Landing Zone FinOps records with rich tags, metrics, and frequencies.
func GenerateMockCostRecordsWithOptions(opts FetchCostOptions) []cloud.CostRecord {
	type mockSvc struct {
		name      string
		costBase  float64
		usageBase float64
		usageUnit string
		usageType string
		bu        string
		proj      string
		app       string
		env       string
	}
	services := []mockSvc{
		{"Amazon Elastic Compute Cloud - Compute", 120.50, 24.0, "Hours", "BoxUsage:t3.medium", "Core Platform", "CloudMigration", "PaymentGateway", "Production"},
		{"Amazon Simple Storage Service", 45.20, 150.0, "GB-Mo", "TimedStorage-ByteHrs", "Data Engineering", "ProjectOmega", "DataLakehouse", "Production"},
		{"Amazon Relational Database Service", 88.75, 24.0, "Hours", "InstanceUsage:db.r5.large", "Digital Products", "ProjectAlpha", "InventoryService", "Staging"},
		{"Amazon DynamoDB", 22.10, 1500000.0, "Requests", "ReadRequestUnits", "Digital Products", "ProjectBeta", "UserAuth", "Development"},
		{"AWS Lambda", 12.40, 2500000.0, "Requests", "Invocations", "Core Platform", "ServerlessAPI", "OrderProcessor", "Production"},
		{"Amazon CloudWatch", 34.00, 500.0, "Metrics", "MetricMonitorUsage", "Security Operations", "Observability", "TelemetryAgent", "Production"},
	}

	startDate := opts.StartDate
	if startDate == "" {
		startDate = time.Now().AddDate(0, 0, -7).Format("2006-01-02")
	}

	metric := opts.Metric
	if metric == "" {
		metric = "UnblendedCost"
	}

	freq := strings.ToUpper(strings.TrimSpace(opts.Frequency))
	if freq == "" {
		freq = "DAILY"
	}

	accounts := []string{opts.AccountID}
	if opts.PayerAccountID != "" {
		if len(opts.IncludeAccounts) > 0 {
			accounts = opts.IncludeAccounts
		} else {
			accounts = []string{"111122223333", "444455556666"}
		}
	}
	if len(accounts) == 0 || accounts[0] == "" {
		accounts = []string{"111122223333"}
	}

	var records []cloud.CostRecord
	now := time.Now()

	for _, acc := range accounts {
		for i, s := range services {
			isUsage := metric == "UsageQuantity"
			var base float64
			var unit string
			var usageType string

			if isUsage {
				base = s.usageBase
				unit = s.usageUnit
				usageType = s.usageType
			} else {
				base = s.costBase
				unit = "USD"
				usageType = "DailyCost"
			}

			jitterRatio := ((rand.Float64() * 0.2) - 0.1) // +/- 10%
			if i == 0 {
				jitterRatio += 1.5 // Introduce an intentional spike in the first service
			}
			dailyBase := base * (1.0 + jitterRatio)
			if dailyBase < 0 {
				dailyBase = base * 0.1
			}

			if freq == "HOURLY" {
				hourlyAmount := dailyBase / 24.0
				if opts.MinCostThreshold > 0 && dailyBase < opts.MinCostThreshold {
					continue
				}
				// Generate 24 hours
				for h := 0; h < 24; h++ {
					rec := cloud.CostRecord{
						AccountID:    acc,
						AccountName:  fmt.Sprintf("LandingZone-%s", acc),
						Service:      s.name,
						UsageDate:    startDate,
						UsagePeriod:  fmt.Sprintf("%sT%02d:00:00Z", startDate, h),
						Amount:       hourlyAmount,
						DailyAmount:  hourlyAmount * 24.0, // exactly equals dailyBase
						DaysInPeriod: 1.0 / 24.0,
						Metric:       metric,
						Frequency:    freq,
						Unit:         unit,
						Currency:     "USD",
						Region:       DEFAULT_REGION,
						UsageType:    usageType,
						PrimaryTag:   s.proj,
						SecondaryTag: s.app,
						BusinessUnit: s.bu,
						Project:      s.proj,
						Application:  s.app,
						Environment:  s.env,
						Owner:        "CloudPlatform",
						CostCenter:   "FinOps-101",
						Tags: map[string]string{
							"BusinessUnit": s.bu,
							"Project":      s.proj,
							"Application":  s.app,
							"Environment":  s.env,
							"Owner":        "CloudPlatform",
							"CostCenter":   "FinOps-101",
						},
						Timestamp: now,
					}
					if opts.PrimaryTag != "" {
						rec.Tags[opts.PrimaryTag] = s.proj
					}
					if opts.SecondaryTag != "" {
						rec.Tags[opts.SecondaryTag] = s.app
					}
					records = append(records, rec)
				}
			} else if freq == "MONTHLY" {
				days := 30.0
				monthlyAmount := dailyBase * days
				if opts.MinCostThreshold > 0 && dailyBase < opts.MinCostThreshold {
					continue
				}
				rec := cloud.CostRecord{
					AccountID:    acc,
					AccountName:  fmt.Sprintf("LandingZone-%s", acc),
					Service:      s.name,
					UsageDate:    startDate,
					UsagePeriod:  startDate[:len(startDate)-3], // e.g. "2026-09"
					Amount:       monthlyAmount,
					DailyAmount:  monthlyAmount / days, // exactly equals dailyBase
					DaysInPeriod: days,
					Metric:       metric,
					Frequency:    freq,
					Unit:         unit,
					Currency:     "USD",
					Region:       DEFAULT_REGION,
					UsageType:    usageType,
					PrimaryTag:   s.proj,
					SecondaryTag: s.app,
					BusinessUnit: s.bu,
					Project:      s.proj,
					Application:  s.app,
					Environment:  s.env,
					Owner:        "CloudPlatform",
					CostCenter:   "FinOps-101",
					Tags: map[string]string{
						"BusinessUnit": s.bu,
						"Project":      s.proj,
						"Application":  s.app,
						"Environment":  s.env,
						"Owner":        "CloudPlatform",
						"CostCenter":   "FinOps-101",
					},
					Timestamp: now,
				}
				if opts.PrimaryTag != "" {
					rec.Tags[opts.PrimaryTag] = s.proj
				}
				if opts.SecondaryTag != "" {
					rec.Tags[opts.SecondaryTag] = s.app
				}
				records = append(records, rec)
			} else { // "DAILY"
				if opts.MinCostThreshold > 0 && dailyBase < opts.MinCostThreshold {
					continue
				}
				rec := cloud.CostRecord{
					AccountID:    acc,
					AccountName:  fmt.Sprintf("LandingZone-%s", acc),
					Service:      s.name,
					UsageDate:    startDate,
					UsagePeriod:  startDate,
					Amount:       dailyBase,
					DailyAmount:  dailyBase,
					DaysInPeriod: 1.0,
					Metric:       metric,
					Frequency:    freq,
					Unit:         unit,
					Currency:     "USD",
					Region:       DEFAULT_REGION,
					UsageType:    usageType,
					PrimaryTag:   s.proj,
					SecondaryTag: s.app,
					BusinessUnit: s.bu,
					Project:      s.proj,
					Application:  s.app,
					Environment:  s.env,
					Owner:        "CloudPlatform",
					CostCenter:   "FinOps-101",
					Tags: map[string]string{
						"BusinessUnit": s.bu,
						"Project":      s.proj,
						"Application":  s.app,
						"Environment":  s.env,
						"Owner":        "CloudPlatform",
						"CostCenter":   "FinOps-101",
					},
					Timestamp: now,
				}
				if opts.PrimaryTag != "" {
					rec.Tags[opts.PrimaryTag] = s.proj
				}
				if opts.SecondaryTag != "" {
					rec.Tags[opts.SecondaryTag] = s.app
				}
				records = append(records, rec)
			}
		}
	}
	return records
}

// GenerateMockCostAnomalies generates realistic Landing Zone Cost Anomaly records.
func GenerateMockCostAnomalies(accountID string) []cloud.AnomalyRecord {
	now := time.Now()
	return []cloud.AnomalyRecord{
		{
			OpsDomain:     "finops",
			AccountID:     accountID,
			AccountName:   fmt.Sprintf("LandingZone-%s", accountID),
			Service:       "Amazon Elastic Compute Cloud - Compute",
			ResourceOrKey: "anom-ec2-unplanned-autoscaling",
			Metric:        "CostSpike",
			ActualValue:   480.25,
			ExpectedValue: 120.00,
			Deviation:     4.0,
			Severity:      "CRITICAL",
			Status:        "OPEN",
			Timestamp:     now,
		},
		{
			OpsDomain:     "finops",
			AccountID:     accountID,
			AccountName:   fmt.Sprintf("LandingZone-%s", accountID),
			Service:       "Amazon Relational Database Service",
			ResourceOrKey: "anom-rds-provisioned-iops",
			Metric:        "CostSpike",
			ActualValue:   210.80,
			ExpectedValue: 95.00,
			Deviation:     2.22,
			Severity:      "HIGH",
			Status:        "OPEN",
			Timestamp:     now,
		},
		{
			OpsDomain:     "finops",
			AccountID:     accountID,
			AccountName:   fmt.Sprintf("LandingZone-%s", accountID),
			Service:       "AWS Lambda",
			ResourceOrKey: "anom-lambda-recursive-loop",
			Metric:        "InvocationSpike",
			ActualValue:   75.50,
			ExpectedValue: 12.00,
			Deviation:     6.29,
			Severity:      "HIGH",
			Status:        "OPEN",
			Timestamp:     now,
		},
	}
}

// GenerateMockSecurityRecords generates representative Landing Zone SecOps records.
func GenerateMockSecurityRecords(accountID string) []cloud.SecurityRecord {
	now := time.Now()
	return []cloud.SecurityRecord{
		{
			AccountID:   accountID,
			AccountName: fmt.Sprintf("LandingZone-%s", accountID),
			Service:     "AmazonS3",
			ResourceID:  fmt.Sprintf("lz-data-lake-%s-public", accountID),
			FindingType: "PublicBucketAccess",
			Severity:    "CRITICAL",
			Description: "S3 Bucket allows unrestricted public read/write access via policy",
			Remediation: "Enable S3 Block Public Access at the bucket and account level",
			Timestamp:   now,
		},
		{
			AccountID:   accountID,
			AccountName: fmt.Sprintf("LandingZone-%s", accountID),
			Service:     "AmazonEC2",
			ResourceID:  "sg-0941829abc123",
			FindingType: "OpenSecurityGroup",
			Severity:    "HIGH",
			Description: "Security Group allows inbound traffic from 0.0.0.0/0 on port 22 (SSH)",
			Remediation: "Restrict ingress to internal VPC CIDR or AWS Systems Manager Session Manager",
			Timestamp:   now,
		},
		{
			AccountID:   accountID,
			AccountName: fmt.Sprintf("LandingZone-%s", accountID),
			Service:     "AWSIdentityAccessManagement",
			ResourceID:  "admin-svc-deployer",
			FindingType: "StaleAccessKey",
			Severity:    "MEDIUM",
			Description: "IAM access key has not been rotated in > 90 days",
			Remediation: "Rotate credentials and migrate to IAM Roles Anywhere or OIDC",
			Timestamp:   now,
		},
	}
}

// GenerateMockCloudOpsRecords generates representative Landing Zone CloudOps records.
func GenerateMockCloudOpsRecords(accountID string) []cloud.CloudOpsRecord {
	now := time.Now()
	return []cloud.CloudOpsRecord{
		{
			AccountID:      accountID,
			AccountName:    fmt.Sprintf("LandingZone-%s", accountID),
			Service:        "AmazonEC2",
			ResourceID:     "vol-0872161feda8921",
			IssueType:      "OrphanedEBSVolume",
			ResourceAge:    45,
			EstimatedWaste: 38.50,
			Recommendation: "Create snapshot and delete unattached gp3 EBS volume",
			Timestamp:      now,
		},
		{
			AccountID:      accountID,
			AccountName:    fmt.Sprintf("LandingZone-%s", accountID),
			Service:        "AmazonEC2",
			ResourceID:     "eipalloc-01928374",
			IssueType:      "UnassociatedElasticIP",
			ResourceAge:    28,
			EstimatedWaste: 7.20,
			Recommendation: "Release unassociated Elastic IP address to avoid hourly idle charges",
			Timestamp:      now,
		},
		{
			AccountID:      accountID,
			AccountName:    fmt.Sprintf("LandingZone-%s", accountID),
			Service:        "AmazonEC2",
			ResourceID:     "snap-019281726a88b",
			IssueType:      "StaleSnapshot",
			ResourceAge:    180,
			EstimatedWaste: 24.00,
			Recommendation: "Archive snapshot to cold storage tier or delete after lifecycle review",
			Timestamp:      now,
		},
	}
}

// GenerateMockVulnerabilityRecords generates realistic CVE vulnerability findings for SecOps.
func GenerateMockVulnerabilityRecords(accountID string) []cloud.SecurityRecord {
	now := time.Now()
	return []cloud.SecurityRecord{
		{
			AccountID:   accountID,
			AccountName: fmt.Sprintf("LandingZone-%s", accountID),
			Service:     "AmazonEC2",
			ResourceID:  fmt.Sprintf("i-%s-webserver01", accountID[len(accountID)-4:]),
			FindingType: "Vulnerability:CVE-2024-3094",
			Severity:    "CRITICAL",
			Description: "Malicious backdoor in upstream xz-utils (liblzma) leading to remote code execution",
			Remediation: "Downgrade or patch xz-utils to version 5.6.2 or later across base AMI",
			Timestamp:   now,
		},
		{
			AccountID:   accountID,
			AccountName: fmt.Sprintf("LandingZone-%s", accountID),
			Service:     "AmazonECR",
			ResourceID:  fmt.Sprintf("app-backend:%s-latest", accountID[len(accountID)-4:]),
			FindingType: "Vulnerability:CVE-2023-48795",
			Severity:    "HIGH",
			Description: "Terrapin SSH vulnerability in OpenSSH allowing sequence number manipulation",
			Remediation: "Upgrade OpenSSH client/server to version >= 9.6p1 in Dockerfile",
			Timestamp:   now,
		},
		{
			AccountID:   accountID,
			AccountName: fmt.Sprintf("LandingZone-%s", accountID),
			Service:     "AmazonEKS",
			ResourceID:  "node-worker-k8s-pod",
			FindingType: "Vulnerability:CVE-2023-38545",
			Severity:    "HIGH",
			Description: "Heap-based buffer overflow in libcurl during SOCKS5 proxy handshake",
			Remediation: "Update curl package to version 8.4.0 or rebuild container image",
			Timestamp:   now,
		},
	}
}
