package finops

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/PopGreenTh/OpenLZ/internal/aws"
	"github.com/PopGreenTh/OpenLZ/internal/cache"
	"github.com/PopGreenTh/OpenLZ/internal/cloud"
	"github.com/PopGreenTh/OpenLZ/internal/duckdb"
	types "github.com/PopGreenTh/OpenLZ/internal/stages/types"
	"github.com/PopGreenTh/OpenLZ/internal/workflow"
)

// SenseCostExplorerInput models inputs for finops-sense-costexplorer.
type SenseCostExplorerInput struct {
	Accounts         []string `json:"accounts"`
	PayerAccount     string   `json:"payerAccount"`
	StartDate        string   `json:"startDate"`
	EndDate          string   `json:"endDate"`
	Service          string   `json:"service"`
	Metric           string   `json:"metric"`
	Frequency        string   `json:"frequency"` // HOURLY, DAILY (default), MONTHLY
	Region           string   `json:"region"`    // Defaults to ap-southeast-1
	GroupBy          string   `json:"groupBy"`   // LINKED_ACCOUNT_SERVICE (default for payer/multi), SERVICE, LINKED_ACCOUNT, HIERARCHICAL, etc.
	GroupByTagKey    string   `json:"groupByTagKey"`
	PrimaryTag       string   `json:"primaryTag"`
	SecondaryTag     string   `json:"secondaryTag"`
	GroupByTags      []string `json:"groupByTags"`
	ExcludeDiscounts bool     `json:"excludeDiscounts"`
	ExcludeCredits   bool     `json:"excludeCredits"`
	MinCostThreshold float64  `json:"minCostThreshold"`
	OutputPath       string   `json:"outputPath"`
	CacheDBPath      string   `json:"cacheDBPath"`
	NoCache          bool     `json:"noCache"`
	Mock             bool     `json:"mock"`
}

// ExecuteSenseCostExplorer runs Stage 1a: AWS Cost Explorer Sense.
func ExecuteSenseCostExplorer(ctx context.Context, in SenseCostExplorerInput) (*types.StageResult, error) {
	if in.OutputPath == "" {
		in.OutputPath = "data/finops_raw.parquet"
	}
	in.OutputPath = types.ResolveAbsolutePath(in.OutputPath)
	if in.CacheDBPath == "" {
		in.CacheDBPath = "openlz_cache.duckdb"
	}
	in.CacheDBPath = types.ResolveAbsolutePath(in.CacheDBPath)
	if in.Service == "" {
		in.Service = "all"
	}
	if in.Metric == "" {
		in.Metric = "UnblendedCost"
	}
	if in.Frequency == "" {
		in.Frequency = "DAILY"
	}
	if in.GroupBy == "" {
		if in.PayerAccount != "" || len(in.Accounts) > 1 {
			in.GroupBy = "LINKED_ACCOUNT_SERVICE"
		} else {
			in.GroupBy = "SERVICE"
		}
	}
	if len(in.GroupByTags) == 0 {
		in.GroupByTags = []string{"BusinessUnit", "Project", "Application", "Environment"}
	}

	// In mock mode without specific accounts or payer, default to standard mock accounts
	if in.Mock && len(in.Accounts) == 0 && in.PayerAccount == "" {
		in.Accounts = []string{"111122223333", "444455556666"}
	}

	reg := in.Region
	if reg == "" {
		reg = "ap-southeast-1"
	}
	client, err := aws.NewClient(ctx, reg, in.Mock)
	if err != nil {
		return nil, fmt.Errorf("aws client init failed: %w", err)
	}

	// In live mode, auto-detect active caller account via STS if no account or payer account was specified
	if !in.Mock && len(in.Accounts) == 0 && in.PayerAccount == "" {
		callerAcc, err := client.GetCallerAccountID(ctx)
		if err != nil {
			return nil, fmt.Errorf("no accounts or payer account specified and failed to auto-detect caller AWS account ID: %w", err)
		}
		slog.InfoContext(ctx, "Auto-detected caller AWS account ID via STS", "accountID", callerAcc)
		in.Accounts = []string{callerAcc}
	}

	slog.InfoContext(ctx, "Executing Stage 1a: FinOps Cost Explorer Sense",
		"accounts", in.Accounts,
		"payerAccount", in.PayerAccount,
		"metric", in.Metric,
		"frequency", in.Frequency,
		"mock", in.Mock,
		"outputPath", in.OutputPath,
	)

	engine, err := duckdb.NewEngine()
	if err != nil {
		return nil, fmt.Errorf("duckdb init failed: %w", err)
	}
	defer engine.Close()

	var c *cache.DuckDBCache
	if !in.NoCache {
		var cacheErr error
		c, cacheErr = cache.NewDuckDBCache(in.CacheDBPath)
		if cacheErr != nil {
			return nil, fmt.Errorf("cache init failed: %w", cacheErr)
		}
		defer c.Close()
	} else {
		slog.InfoContext(ctx, "Cache bypassed via --no-cache; querying live cloud APIs directly")
	}

	var includeAccounts []string
	if in.PayerAccount != "" && len(in.Accounts) > 0 {
		includeAccounts = in.Accounts
	}

	scanner := workflow.NewScanner(client, c, 5)
	records, err := scanner.ScanFinOpsAccountsWithOptions(ctx, workflow.ScanCostOptions{
		AccountIDs:       in.Accounts,
		PayerAccountID:   in.PayerAccount,
		IncludeAccounts:  includeAccounts,
		StartDate:        in.StartDate,
		EndDate:          in.EndDate,
		Service:          in.Service,
		Metric:           in.Metric,
		Frequency:        in.Frequency,
		GroupBy:          in.GroupBy,
		GroupByTagKey:    in.GroupByTagKey,
		PrimaryTag:       in.PrimaryTag,
		SecondaryTag:     in.SecondaryTag,
		GroupByTags:      in.GroupByTags,
		ExcludeDiscounts: in.ExcludeDiscounts,
		ExcludeCredits:   in.ExcludeCredits,
		MinCostThreshold: in.MinCostThreshold,
	})
	if err != nil {
		return nil, fmt.Errorf("finops sense failed: %w", err)
	}

	// Filter by service if specified
	if in.Service != "" && in.Service != "all" {
		var filtered []cloud.CostRecord
		for _, r := range records {
			if strings.Contains(strings.ToLower(r.Service), strings.ToLower(in.Service)) {
				filtered = append(filtered, r)
			}
		}
		records = filtered
	}

	if in.OutputPath != "" {
		slog.InfoContext(ctx, "Writing FinOps records to storage",
			"recordCount", len(records),
			"outputPath", in.OutputPath,
		)
		if strings.HasSuffix(strings.ToLower(in.OutputPath), ".csv") {
			tmpParquet := in.OutputPath + ".tmp.parquet"
			if err := engine.WriteFinOpsParquet(ctx, records, tmpParquet); err != nil {
				return nil, fmt.Errorf("failed writing raw finops parquet: %w", err)
			}
			if err := engine.ExportToCSV(ctx, tmpParquet, in.OutputPath); err != nil {
				_ = os.Remove(tmpParquet)
				return nil, fmt.Errorf("failed exporting to csv: %w", err)
			}
			_ = os.Remove(tmpParquet)
		} else {
			if err := engine.WriteFinOpsParquet(ctx, records, in.OutputPath); err != nil {
				return nil, fmt.Errorf("failed writing raw finops parquet: %w", err)
			}
		}
	}

	slog.InfoContext(ctx, "Stage 1a (FinOps Sense) completed successfully",
		"recordCount", len(records),
		"outputPath", in.OutputPath,
	)

	return &types.StageResult{
		Stage:       types.StageFinOpsSenseCostExplorer,
		Success:     true,
		RecordCount: len(records),
		OutputPath:  in.OutputPath,
		Message:     fmt.Sprintf("Sensed %d Cost Explorer records into %s", len(records), in.OutputPath),
	}, nil
}
