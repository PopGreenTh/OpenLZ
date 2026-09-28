package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/PopGreenTh/OpenLZ/internal/aws"
	"github.com/PopGreenTh/OpenLZ/internal/cache"
	"github.com/PopGreenTh/OpenLZ/internal/cloud"
	"golang.org/x/sync/errgroup"
)

// Scanner coordinates concurrent account scans across Landing Zone accounts using errgroup.
type Scanner struct {
	client      *aws.Client
	duckdbCache *cache.DuckDBCache
	concurrency int
}

// NewScanner creates a new multi-account scanner.
func NewScanner(client *aws.Client, duckdbCache *cache.DuckDBCache, concurrency int) *Scanner {
	if concurrency <= 0 {
		concurrency = 5
	}
	return &Scanner{
		client:      client,
		duckdbCache: duckdbCache,
		concurrency: concurrency,
	}
}

// ScanCostOptions models options for scanning Cost Explorer across landing zone accounts.
type ScanCostOptions struct {
	AccountIDs        []string
	PayerAccountID    string
	IncludeAccounts   []string
	ExcludeAccounts   []string
	StartDate         string
	EndDate           string
	Service           string
	Metric            string
	Frequency         string
	GroupBy           string
	GroupByTagKey     string
	PrimaryTag        string
	SecondaryTag      string
	GroupByDimensions []string
	GroupByTags       []string
	ExcludeDiscounts  bool
	ExcludeCredits    bool
	MinCostThreshold  float64
}

// ScanFinOpsAccountsWithOptions scans accounts with consolidated billing or concurrent account scans and tag support.
func (s *Scanner) ScanFinOpsAccountsWithOptions(ctx context.Context, opts ScanCostOptions) ([]cloud.CostRecord, error) {
	if opts.StartDate == "" {
		opts.StartDate = time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	}
	if opts.EndDate == "" {
		opts.EndDate = time.Now().Format("2006-01-02")
	}

	// 1. Consolidated Payer Account Mode
	if opts.PayerAccountID != "" {
		queryDate := opts.StartDate
		includeSuffix := ""
		if len(opts.IncludeAccounts) > 0 {
			includeSuffix = "_" + strings.Join(opts.IncludeAccounts, "-")
		}
		groupBy := opts.GroupBy
		if groupBy == "" {
			groupBy = "LINKED_ACCOUNT_SERVICE"
		}
		tagSuffix := ""
		if strings.HasPrefix(strings.ToUpper(groupBy), "HIERARCHICAL") {
			tagSuffix = fmt.Sprintf("_%s_%s", opts.PrimaryTag, opts.SecondaryTag)
		}
		dateSuffix := fmt.Sprintf("_%s_to_%s", opts.StartDate, opts.EndDate)
		cacheKey := fmt.Sprintf("payer_%s_%s_%s_%s%s%s%s", opts.PayerAccountID, opts.Frequency, opts.Metric, strings.ToLower(groupBy), dateSuffix, tagSuffix, includeSuffix)

		if s.duckdbCache != nil {
			if cached, found, err := s.duckdbCache.Get(ctx, "aws_costexplorer", cacheKey, queryDate); err == nil && found {
				var cachedRecords []cloud.CostRecord
				if jsonErr := json.Unmarshal(cached, &cachedRecords); jsonErr == nil && len(cachedRecords) > 0 {
					slog.InfoContext(ctx, "Loaded FinOps cost records from DuckDB cache", "cacheKey", cacheKey, "records", len(cachedRecords))
					return cachedRecords, nil
				}
			}
			slog.DebugContext(ctx, "DuckDB cache miss, querying AWS Cost Explorer", "cacheKey", cacheKey)
		}

		records, err := s.client.FetchCostsWithOptions(ctx, aws.FetchCostOptions{
			PayerAccountID:    opts.PayerAccountID,
			IncludeAccounts:   opts.IncludeAccounts,
			ExcludeAccounts:   opts.ExcludeAccounts,
			StartDate:         opts.StartDate,
			EndDate:           opts.EndDate,
			Service:           opts.Service,
			Metric:            opts.Metric,
			Frequency:         opts.Frequency,
			GroupBy:           opts.GroupBy,
			GroupByTagKey:     opts.GroupByTagKey,
			PrimaryTag:        opts.PrimaryTag,
			SecondaryTag:      opts.SecondaryTag,
			GroupByDimensions: opts.GroupByDimensions,
			GroupByTags:       opts.GroupByTags,
			ExcludeDiscounts:  opts.ExcludeDiscounts,
			ExcludeCredits:    opts.ExcludeCredits,
			MinCostThreshold:  opts.MinCostThreshold,
		})
		if err != nil {
			return nil, fmt.Errorf("payer account %s consolidated scan failed: %w", opts.PayerAccountID, err)
		}

		if s.duckdbCache != nil && len(records) > 0 {
			slog.DebugContext(ctx, "Saving scan results to DuckDB cache", "records", len(records), "cacheKey", cacheKey)
			if payload, jsonErr := json.Marshal(records); jsonErr == nil {
				_ = s.duckdbCache.Set(ctx, "aws_costexplorer", cacheKey, queryDate, payload)
			}
			slog.DebugContext(ctx, "Scan results cached successfully", "cacheKey", cacheKey)
		}
		return records, nil
	}

	// 2. Multi-Account Concurrent Scan Mode
	accountIDs := opts.AccountIDs
	if len(accountIDs) == 0 {
		accountIDs = []string{"111122223333", "444455556666", "777788889999"}
	}

	var mu sync.Mutex
	var allRecords []cloud.CostRecord

	g, gCtx := errgroup.WithContext(ctx)
	g.SetLimit(s.concurrency)

	for _, id := range accountIDs {
		accountID := id
		g.Go(func() error {
			queryDate := opts.StartDate
			groupBy := opts.GroupBy
			if groupBy == "" {
				groupBy = "SERVICE"
			}
			tagSuffix := ""
			if strings.HasPrefix(strings.ToUpper(groupBy), "HIERARCHICAL") {
				tagSuffix = fmt.Sprintf("_%s_%s", opts.PrimaryTag, opts.SecondaryTag)
			}
			dateSuffix := fmt.Sprintf("_%s_to_%s", opts.StartDate, opts.EndDate)
			cacheKey := fmt.Sprintf("%s_%s_%s_%s%s%s", accountID, opts.Frequency, opts.Metric, strings.ToLower(groupBy), dateSuffix, tagSuffix)

			// DuckDB Cache Check: Guardrail against duplicate $0.01 API charges
			if s.duckdbCache != nil {
				if cached, found, err := s.duckdbCache.Get(gCtx, "aws_costexplorer", cacheKey, queryDate); err == nil && found {
					var cachedRecords []cloud.CostRecord
					if jsonErr := json.Unmarshal(cached, &cachedRecords); jsonErr == nil && len(cachedRecords) > 0 {
						slog.InfoContext(gCtx, "Loaded FinOps cost records from DuckDB cache", "accountID", accountID, "cacheKey", cacheKey, "records", len(cachedRecords))
						mu.Lock()
						allRecords = append(allRecords, cachedRecords...)
						mu.Unlock()
						return nil
					}
				}
				slog.DebugContext(gCtx, "DuckDB cache miss, querying AWS Cost Explorer", "accountID", accountID, "cacheKey", cacheKey)
			}

			// Query AWS API
			records, err := s.client.FetchCostsWithOptions(gCtx, aws.FetchCostOptions{
				AccountID:         accountID,
				StartDate:         opts.StartDate,
				EndDate:           opts.EndDate,
				Service:           opts.Service,
				Metric:            opts.Metric,
				Frequency:         opts.Frequency,
				GroupBy:           opts.GroupBy,
				GroupByTagKey:     opts.GroupByTagKey,
				PrimaryTag:        opts.PrimaryTag,
				SecondaryTag:      opts.SecondaryTag,
				GroupByDimensions: opts.GroupByDimensions,
				GroupByTags:       opts.GroupByTags,
				ExcludeDiscounts:  opts.ExcludeDiscounts,
				ExcludeCredits:    opts.ExcludeCredits,
				MinCostThreshold:  opts.MinCostThreshold,
			})
			if err != nil {
				return fmt.Errorf("account %s scan failed: %w", accountID, err)
			}

			// Save to DuckDB Cache
			if s.duckdbCache != nil && len(records) > 0 {
				slog.DebugContext(gCtx, "Saving scan results to DuckDB cache", "accountID", accountID, "records", len(records), "cacheKey", cacheKey)
				if payload, jsonErr := json.Marshal(records); jsonErr == nil {
					_ = s.duckdbCache.Set(gCtx, "aws_costexplorer", cacheKey, queryDate, payload)
				}
				slog.DebugContext(gCtx, "Scan results cached successfully", "accountID", accountID, "cacheKey", cacheKey)
			}

			mu.Lock()
			allRecords = append(allRecords, records...)
			mu.Unlock()
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}
	return allRecords, nil
}

// ScanFinOpsAccounts scans multiple accounts for cost data concurrently, enforcing the DuckDB $0.01 API guardrail.
func (s *Scanner) ScanFinOpsAccounts(ctx context.Context, accountIDs []string, startDate, endDate string) ([]cloud.CostRecord, error) {
	return s.ScanFinOpsAccountsWithOptions(ctx, ScanCostOptions{
		AccountIDs: accountIDs,
		StartDate:  startDate,
		EndDate:    endDate,
	})
}

// ScanSecOpsAccounts scans multiple accounts for security posture findings concurrently.
func (s *Scanner) ScanSecOpsAccounts(ctx context.Context, accountIDs []string) ([]cloud.SecurityRecord, error) {
	if len(accountIDs) == 0 {
		accountIDs = []string{"111122223333", "444455556666", "777788889999"}
	}

	var mu sync.Mutex
	var allRecords []cloud.SecurityRecord

	g, _ := errgroup.WithContext(ctx)
	g.SetLimit(s.concurrency)

	for _, id := range accountIDs {
		accountID := id
		g.Go(func() error {
			records := aws.GenerateMockSecurityRecords(accountID)
			mu.Lock()
			allRecords = append(allRecords, records...)
			mu.Unlock()
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}
	return allRecords, nil
}

// ScanCloudOpsAccounts scans multiple accounts for operational waste & hygiene concurrently.
func (s *Scanner) ScanCloudOpsAccounts(ctx context.Context, accountIDs []string) ([]cloud.CloudOpsRecord, error) {
	if len(accountIDs) == 0 {
		accountIDs = []string{"111122223333", "444455556666", "777788889999"}
	}

	var mu sync.Mutex
	var allRecords []cloud.CloudOpsRecord

	g, _ := errgroup.WithContext(ctx)
	g.SetLimit(s.concurrency)

	for _, id := range accountIDs {
		accountID := id
		g.Go(func() error {
			records := aws.GenerateMockCloudOpsRecords(accountID)
			mu.Lock()
			allRecords = append(allRecords, records...)
			mu.Unlock()
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}
	return allRecords, nil
}
