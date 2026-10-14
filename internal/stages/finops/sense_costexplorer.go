package finops

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/PopGreenTh/OpenLZ/internal/aws"
	"github.com/PopGreenTh/OpenLZ/internal/cache"
	"github.com/PopGreenTh/OpenLZ/internal/cloud"
	"github.com/PopGreenTh/OpenLZ/internal/duckdb"
	"github.com/PopGreenTh/OpenLZ/internal/powerquery"
	"github.com/PopGreenTh/OpenLZ/internal/report"
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
	Format           string   `json:"format"` // "parquet", "csv", "excel", "powerbi", "all" (or comma-separated)
	OutputPath       string   `json:"outputPath"`
	AppendTimestamp  bool     `json:"appendTimestamp"`
	AppendOutput     bool     `json:"appendOutput"`
	Upsert           bool     `json:"upsert"`
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
	if in.PrimaryTag == "" {
		if envPrimary := os.Getenv("OPENLZ_PRIMARY_TAG"); envPrimary != "" {
			in.PrimaryTag = strings.TrimSpace(envPrimary)
		}
	}
	if in.SecondaryTag == "" {
		if envSecondary := os.Getenv("OPENLZ_SECONDARY_TAG"); envSecondary != "" {
			in.SecondaryTag = strings.TrimSpace(envSecondary)
		}
	}
	if len(in.GroupByTags) == 0 {
		if envTags := os.Getenv("OPENLZ_GROUP_BY_TAGS"); envTags != "" {
			for _, t := range strings.Split(envTags, ",") {
				if tr := strings.TrimSpace(t); tr != "" {
					in.GroupByTags = append(in.GroupByTags, tr)
				}
			}
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

	var generatedOutputs []string
	if in.OutputPath != "" {
		basePath := types.StripExtensions(in.OutputPath)

		fmtReq := strings.ToLower(strings.TrimSpace(in.Format))
		wantParquet := false
		wantCSV := false
		wantExcel := false
		wantPowerBI := false

		if fmtReq == "" {
			if strings.HasSuffix(strings.ToLower(in.OutputPath), ".csv") {
				wantCSV = true
			} else {
				wantParquet = true
			}
		} else {
			tokens := strings.FieldsFunc(fmtReq, func(r rune) bool {
				return r == ',' || r == ' ' || r == ';'
			})
			for _, t := range tokens {
				token := strings.TrimSpace(t)
				switch {
				case token == "all":
					wantParquet = true
					wantCSV = true
					wantExcel = true
					wantPowerBI = true
				case token == "parquet" || token == "par" || token == "pq":
					wantParquet = true
				case token == "csv":
					wantCSV = true
				case token == "excel" || token == "xls" || token == "xlsx":
					wantCSV = true
					wantExcel = true
				case token == "powerbi" || token == "pbi":
					wantParquet = true
					wantPowerBI = true
				}
			}
		}

		now := time.Now()
		tsSuffix := types.FormatTimestamp(now)

		// Each sense run must have a datetime suffix (_YYYYMMDD_HHMMSS)
		timestampedBase := fmt.Sprintf("%s_%s", basePath, tsSuffix)
		timestampedParquet := timestampedBase + ".parquet"
		originalParquet := basePath + ".parquet"

		slog.InfoContext(ctx, "Writing FinOps records to storage",
			"recordCount", len(records),
			"outputPath", in.OutputPath,
			"format", in.Format,
			"parquet", wantParquet,
			"csv", wantCSV,
			"excel", wantExcel,
			"powerbi", wantPowerBI,
			"timestampedParquet", timestampedParquet,
			"appendOutput", in.AppendOutput || in.Upsert,
		)

		// Always write the timestamped immutable parquet snapshot
		if err := engine.WriteFinOpsParquet(ctx, records, timestampedParquet); err != nil {
			return nil, fmt.Errorf("failed writing timestamped raw finops parquet: %w", err)
		}
		if wantParquet {
			generatedOutputs = append(generatedOutputs, timestampedParquet)
		}

		// Option to append/upsert to the original --output
		if in.AppendOutput || in.Upsert {
			if err := engine.UpsertFinOpsParquet(ctx, records, originalParquet); err != nil {
				return nil, fmt.Errorf("failed appending/upserting to original finops parquet: %w", err)
			}
			generatedOutputs = append(generatedOutputs, originalParquet)
		}

		if wantCSV || wantExcel {
			csvPath := timestampedBase + ".csv"
			if err := engine.ExportToCSV(ctx, timestampedParquet, csvPath); err != nil {
				return nil, fmt.Errorf("failed exporting to csv: %w", err)
			}
			generatedOutputs = append(generatedOutputs, csvPath)

			// Every CSV export must come with a Power Query .m file pointing to it
			excelMPath := timestampedBase + "_excel.m"
			mCode := powerquery.GenerateRawFinOpsCsvMScript(csvPath)
			if err := powerquery.WriteMScriptToFile(mCode, excelMPath); err != nil {
				return nil, fmt.Errorf("failed writing excel power query .m file: %w", err)
			}
			generatedOutputs = append(generatedOutputs, excelMPath)

			if wantExcel {
				xlsxPath := timestampedBase + ".xlsx"
				if err := report.ExportFinOpsToXLSX(records, xlsxPath); err != nil {
					return nil, fmt.Errorf("failed exporting to excel xlsx: %w", err)
				}
				generatedOutputs = append(generatedOutputs, xlsxPath)
			}
		}

		if wantPowerBI {
			powerbiMPath := timestampedBase + "_powerbi.m"
			mCode := powerquery.GenerateRawFinOpsParquetMScript(timestampedParquet)
			if err := powerquery.WriteMScriptToFile(mCode, powerbiMPath); err != nil {
				return nil, fmt.Errorf("failed writing powerbi power query .m file: %w", err)
			}
			generatedOutputs = append(generatedOutputs, powerbiMPath)
		}

		// If user only wanted CSV/Excel and not Parquet, clean up the temporary Parquet snapshot unless requested
		if !wantParquet && !wantPowerBI && !in.AppendOutput && !in.Upsert && (wantCSV || wantExcel) {
			_ = os.Remove(timestampedParquet)
		}
	}

	outSummary := in.OutputPath
	if len(generatedOutputs) > 0 {
		outSummary = strings.Join(generatedOutputs, ", ")
	}

	slog.InfoContext(ctx, "Stage 1a (FinOps Sense) completed successfully",
		"recordCount", len(records),
		"outputs", generatedOutputs,
	)

	return &types.StageResult{
		Stage:       types.StageFinOpsSenseCostExplorer,
		Success:     true,
		RecordCount: len(records),
		OutputPath:  outSummary,
		Message:     fmt.Sprintf("Sensed %d Cost Explorer records into %s", len(records), outSummary),
	}, nil
}
