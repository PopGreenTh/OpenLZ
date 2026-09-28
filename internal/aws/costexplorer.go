package aws

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/PopGreenTh/OpenLZ/internal/cloud"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer/types"
	"golang.org/x/sync/errgroup"
)

// FetchCostOptions configures Cost Explorer retrieval parameters.
type FetchCostOptions struct {
	AccountID         string
	PayerAccountID    string
	IncludeAccounts   []string
	ExcludeAccounts   []string
	StartDate         string
	EndDate           string
	Service           string
	Metric            string   // e.g. "UnblendedCost", "UsageQuantity", "AmortizedCost"
	Frequency         string   // "HOURLY", "DAILY", "MONTHLY"
	GroupBy           string   // e.g. "LINKED_ACCOUNT_SERVICE", "SERVICE", "LINKED_ACCOUNT", "SERVICE_TAG", "LINKED_ACCOUNT_TAG", "TAG", "HIERARCHICAL"
	GroupByTagKey     string   // e.g. "Project", "BusinessUnit" (used when GroupBy includes TAG)
	PrimaryTag        string   // e.g. "Project" for hierarchical mode (Round 1 tag)
	SecondaryTag      string   // e.g. "Application" for hierarchical mode (Round 2 tag)
	GroupByDimensions []string // e.g. ["SERVICE"], ["SERVICE", "LINKED_ACCOUNT"]
	GroupByTags       []string // e.g. ["BusinessUnit", "Project", "Application", "Environment"]
	ExcludeDiscounts  bool
	ExcludeCredits    bool
	MinCostThreshold  float64
}

func assignTagToRecord(r *cloud.CostRecord, tagKey, tagVal string) {
	if r.Tags == nil {
		r.Tags = make(map[string]string)
	}
	r.Tags[tagKey] = tagVal

	switch strings.ToLower(strings.ReplaceAll(tagKey, "_", "")) {
	case "bu", "businessunit":
		r.BusinessUnit = tagVal
	case "project", "proj":
		r.Project = tagVal
	case "application", "app":
		r.Application = tagVal
	case "environment", "env":
		r.Environment = tagVal
	case "owner":
		r.Owner = tagVal
	case "costcenter", "costctr", "cc":
		r.CostCenter = tagVal
	}
}

// daysInMonth returns the calendar days in the month of the provided date.
func daysInMonth(t time.Time) int {
	return time.Date(t.Year(), t.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// computeDailyMetrics calculates DailyAmount and DaysInPeriod so HOURLY, DAILY, and MONTHLY figures are directly comparable.
func computeDailyMetrics(frequency string, amount float64, periodStart string, rangeEnd string) (dailyAmount float64, daysInPeriod float64, usageDate string, usagePeriod string) {
	freq := strings.ToUpper(strings.TrimSpace(frequency))
	if freq == "" {
		freq = "DAILY"
	}

	switch freq {
	case "HOURLY":
		daysInPeriod = 1.0 / 24.0
		dailyAmount = amount * 24.0 // 24-hr daily equivalent run-rate
		usagePeriod = periodStart
		if len(periodStart) >= 10 {
			usageDate = periodStart[:10]
		} else {
			usageDate = periodStart
		}

	case "MONTHLY":
		usagePeriod = periodStart
		if len(periodStart) >= 7 {
			usagePeriod = periodStart[:7]
		}
		usageDate = periodStart
		if len(periodStart) >= 10 {
			usageDate = periodStart[:10]
		}

		t, err := time.Parse("2006-01-02", usageDate)
		if err != nil {
			t, _ = time.Parse(time.RFC3339, periodStart)
		}
		if t.IsZero() {
			t = time.Now()
		}

		totalDays := float64(daysInMonth(t))
		daysInPeriod = totalDays

		// Handle partial month if rangeEnd falls within the same month
		if rangeEnd != "" {
			endT, endErr := time.Parse("2006-01-02", rangeEnd)
			if endErr == nil && endT.Year() == t.Year() && endT.Month() == t.Month() {
				elapsed := float64(endT.Day())
				if elapsed > 0 && elapsed < totalDays {
					daysInPeriod = elapsed
				}
			}
		}

		if daysInPeriod > 0 {
			dailyAmount = amount / daysInPeriod
		} else {
			dailyAmount = amount
		}

	default: // "DAILY"
		daysInPeriod = 1.0
		dailyAmount = amount
		usagePeriod = periodStart
		if len(periodStart) >= 10 {
			usageDate = periodStart[:10]
		} else {
			usageDate = periodStart
		}
	}

	return dailyAmount, daysInPeriod, usageDate, usagePeriod
}

// getLinkedAccountNames queries Cost Explorer GetDimensionValues to retrieve actual AWS account names.
func (c *Client) getLinkedAccountNames(ctx context.Context, startDate, endDate string) map[string]string {
	names := make(map[string]string)
	if c.isMock || c.costExplorer == nil {
		return names
	}

	start := startDate
	end := endDate
	if start == "" {
		start = time.Now().AddDate(0, 0, -7).Format("2006-01-02")
	}
	if end == "" {
		end = time.Now().Format("2006-01-02")
	}

	var nextToken *string
	for {
		input := &costexplorer.GetDimensionValuesInput{
			Dimension: types.DimensionLinkedAccount,
			TimePeriod: &types.DateInterval{
				Start: aws.String(start),
				End:   aws.String(end),
			},
			NextPageToken: nextToken,
		}

		resp, err := c.costExplorer.GetDimensionValues(ctx, input)
		if err != nil {
			slog.DebugContext(ctx, "GetDimensionValues for LINKED_ACCOUNT returned error", "error", err)
			break
		}

		for _, dv := range resp.DimensionValues {
			accID := aws.ToString(dv.Value)
			if desc, ok := dv.Attributes["description"]; ok && desc != "" {
				names[accID] = desc
			}
		}

		if resp.NextPageToken == nil || *resp.NextPageToken == "" {
			break
		}
		nextToken = resp.NextPageToken
	}
	return names
}

// FetchCostsWithOptions queries AWS Cost Explorer with multi-dimensional groupings, tags, and filters.
func (c *Client) FetchCostsWithOptions(ctx context.Context, opts FetchCostOptions) ([]cloud.CostRecord, error) {
	if c.isMock || c.costExplorer == nil {
		slog.DebugContext(ctx, "Generating mock Cost Explorer records", "account", opts.AccountID, "metric", opts.Metric)
		return GenerateMockCostRecordsWithOptions(opts), nil
	}

	// Resolve real account names from Cost Explorer dimension attributes
	accountNames := c.getLinkedAccountNames(ctx, opts.StartDate, opts.EndDate)

	metric := "UnblendedCost"
	if opts.Metric != "" {
		metric = opts.Metric
	}

	freq := strings.ToUpper(strings.TrimSpace(opts.Frequency))
	if freq == "" {
		freq = "DAILY"
	}

	var granularity types.Granularity
	switch freq {
	case "HOURLY":
		granularity = types.GranularityHourly
	case "MONTHLY":
		granularity = types.GranularityMonthly
	default:
		granularity = types.GranularityDaily
	}

	startDate := opts.StartDate
	if startDate == "" {
		startDate = time.Now().AddDate(0, 0, -7).Format("2006-01-02")
	}
	endDate := opts.EndDate
	if endDate == "" {
		endDate = time.Now().Format("2006-01-02")
	}

	slog.InfoContext(ctx, "Querying AWS Cost Explorer",
		"account", opts.AccountID,
		"payer", opts.PayerAccountID,
		"metric", metric,
		"frequency", freq,
		"startDate", startDate,
		"endDate", endDate,
	)

	// Align monthly intervals to first-of-month per AWS Cost Explorer requirements
	if freq == "MONTHLY" {
		if t, err := time.Parse("2006-01-02", startDate); err == nil {
			startDate = time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
		}
		if t, err := time.Parse("2006-01-02", endDate); err == nil {
			if t.Day() != 1 {
				endDate = time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
			}
		}
	}

	accountID := opts.AccountID
	if opts.PayerAccountID != "" {
		accountID = opts.PayerAccountID
	}

	// Determine GroupBy mode (aligned with gather_cost_explorer.py lines 442-474)
	groupBy := strings.ToUpper(strings.TrimSpace(opts.GroupBy))
	if groupBy == "" {
		if opts.PayerAccountID != "" {
			groupBy = "LINKED_ACCOUNT_SERVICE"
		} else {
			groupBy = "SERVICE"
		}
	}

	// Build filter expression (constructed once)
	var andExprs []types.Expression
	var excludeRecordTypes []string
	if opts.ExcludeDiscounts {
		excludeRecordTypes = append(excludeRecordTypes,
			"Enterprise Discount Program Discount",
			"Solution Provider Program Discount",
		)
	}
	if opts.ExcludeCredits {
		excludeRecordTypes = append(excludeRecordTypes, "Credit")
	}
	if len(excludeRecordTypes) > 0 {
		andExprs = append(andExprs, types.Expression{
			Not: &types.Expression{
				Dimensions: &types.DimensionValues{
					Key:    types.DimensionRecordType,
					Values: excludeRecordTypes,
				},
			},
		})
	}

	if len(opts.IncludeAccounts) > 0 {
		andExprs = append(andExprs, types.Expression{
			Dimensions: &types.DimensionValues{
				Key:    types.DimensionLinkedAccount,
				Values: opts.IncludeAccounts,
			},
		})
	}

	if opts.Service != "" && opts.Service != "all" {
		andExprs = append(andExprs, types.Expression{
			Dimensions: &types.DimensionValues{
				Key:          types.DimensionService,
				Values:       []string{opts.Service},
				MatchOptions: []types.MatchOption{types.MatchOptionContains},
			},
		})
	}

	var ceFilter *types.Expression
	if len(andExprs) == 1 {
		ceFilter = &andExprs[0]
	} else if len(andExprs) > 1 {
		ceFilter = &types.Expression{And: andExprs}
	}

	// Check for Hierarchical Two-Round Sense mode
	if groupBy == "HIERARCHICAL" || groupBy == "HIERARCHICAL_PROJECT_APP" || groupBy == "TWO_ROUND" || groupBy == "TWO_LEVEL" {
		return c.fetchHierarchicalCosts(ctx, opts, startDate, endDate, granularity, freq, metric, accountNames, excludeRecordTypes)
	}

	// Build grouping passes
	type groupPass struct {
		groupDefs []types.GroupDefinition
		tagKey    string
	}
	var passes []groupPass

	switch groupBy {
	case "LINKED_ACCOUNT_SERVICE", "ACCOUNT_SERVICE", "BOTH":
		// gather_cost_explorer.py:469-474: Mix both SERVICE and LINKED_ACCOUNT dimensions
		passes = append(passes, groupPass{
			groupDefs: []types.GroupDefinition{
				{Type: types.GroupDefinitionTypeDimension, Key: aws.String("SERVICE")},
				{Type: types.GroupDefinitionTypeDimension, Key: aws.String("LINKED_ACCOUNT")},
			},
		})
	case "SERVICE":
		passes = append(passes, groupPass{
			groupDefs: []types.GroupDefinition{
				{Type: types.GroupDefinitionTypeDimension, Key: aws.String("SERVICE")},
			},
		})
	case "LINKED_ACCOUNT":
		passes = append(passes, groupPass{
			groupDefs: []types.GroupDefinition{
				{Type: types.GroupDefinitionTypeDimension, Key: aws.String("LINKED_ACCOUNT")},
			},
		})
	case "SERVICE_TAG":
		tagKeys := opts.GroupByTags
		if opts.GroupByTagKey != "" {
			tagKeys = []string{opts.GroupByTagKey}
		} else if len(tagKeys) == 0 {
			tagKeys = []string{"Project"}
		}
		for _, tk := range tagKeys {
			passes = append(passes, groupPass{
				groupDefs: []types.GroupDefinition{
					{Type: types.GroupDefinitionTypeDimension, Key: aws.String("SERVICE")},
					{Type: types.GroupDefinitionTypeTag, Key: aws.String(tk)},
				},
				tagKey: tk,
			})
		}
	case "LINKED_ACCOUNT_TAG", "ACCOUNT_TAG":
		tagKeys := opts.GroupByTags
		if opts.GroupByTagKey != "" {
			tagKeys = []string{opts.GroupByTagKey}
		} else if len(tagKeys) == 0 {
			tagKeys = []string{"Project"}
		}
		for _, tk := range tagKeys {
			passes = append(passes, groupPass{
				groupDefs: []types.GroupDefinition{
					{Type: types.GroupDefinitionTypeDimension, Key: aws.String("LINKED_ACCOUNT")},
					{Type: types.GroupDefinitionTypeTag, Key: aws.String(tk)},
				},
				tagKey: tk,
			})
		}
	case "TAG":
		tagKeys := opts.GroupByTags
		if opts.GroupByTagKey != "" {
			tagKeys = []string{opts.GroupByTagKey}
		} else if len(tagKeys) == 0 {
			tagKeys = []string{"Project"}
		}
		for _, tk := range tagKeys {
			passes = append(passes, groupPass{
				groupDefs: []types.GroupDefinition{
					{Type: types.GroupDefinitionTypeTag, Key: aws.String(tk)},
				},
				tagKey: tk,
			})
		}
	default:
		if opts.PayerAccountID != "" {
			passes = append(passes, groupPass{
				groupDefs: []types.GroupDefinition{
					{Type: types.GroupDefinitionTypeDimension, Key: aws.String("SERVICE")},
					{Type: types.GroupDefinitionTypeDimension, Key: aws.String("LINKED_ACCOUNT")},
				},
			})
		} else {
			passes = append(passes, groupPass{
				groupDefs: []types.GroupDefinition{
					{Type: types.GroupDefinitionTypeDimension, Key: aws.String("SERVICE")},
				},
			})
		}
	}

	recordMap := make(map[string]*cloud.CostRecord)
	var orderedKeys []string

	for passIdx, pass := range passes {
		groupDefs := pass.groupDefs
		tagKey := pass.tagKey

		err := ExecuteWithRetry(ctx, func() error {
			var nextToken *string
			for {
				input := &costexplorer.GetCostAndUsageInput{
					TimePeriod: &types.DateInterval{
						Start: aws.String(startDate),
						End:   aws.String(endDate),
					},
					Granularity:   granularity,
					Metrics:       []string{metric},
					GroupBy:       groupDefs,
					Filter:        ceFilter,
					NextPageToken: nextToken,
				}

				slog.DebugContext(ctx, "Calling Cost Explorer GetCostAndUsage",
					"groupBy", groupBy,
					"tagKey", tagKey,
					"hasNextToken", nextToken != nil,
				)
				resp, callErr := c.costExplorer.GetCostAndUsage(ctx, input)
				if callErr != nil {
					slog.ErrorContext(ctx, "Cost Explorer GetCostAndUsage call failed", "error", callErr)
					return callErr
				}
				slog.DebugContext(ctx, "Cost Explorer response page received",
					"periodsCount", len(resp.ResultsByTime),
					"hasNextPage", resp.NextPageToken != nil && *resp.NextPageToken != "",
				)

				for _, resultByTime := range resp.ResultsByTime {
					rawStart := aws.ToString(resultByTime.TimePeriod.Start)
					for _, group := range resultByTime.Groups {
						svcName := "Unknown"
						rowAccountID := accountID
						var tagVal string

						for idx, key := range group.Keys {
							if idx < len(groupDefs) {
								def := groupDefs[idx]
								if def.Type == types.GroupDefinitionTypeDimension {
									if *def.Key == "SERVICE" {
										svcName = key
									} else if *def.Key == "LINKED_ACCOUNT" {
										rowAccountID = key
									}
								} else if def.Type == types.GroupDefinitionTypeTag {
									tKey := *def.Key
									tagVal = strings.TrimPrefix(key, tKey+"$")
									if tagVal == "" || tagVal == key {
										tagVal = "Untagged"
									}
								}
							}
						}

						amount := 0.0
						unit := "USD"
						if metric == "UsageQuantity" {
							unit = "Quantity"
						}
						if m, ok := group.Metrics[metric]; ok {
							if m.Amount != nil {
								fmt.Sscanf(*m.Amount, "%f", &amount)
							}
							if m.Unit != nil && *m.Unit != "" {
								unit = *m.Unit
							}
						}

						if opts.MinCostThreshold > 0 && amount < opts.MinCostThreshold {
							continue
						}

						dailyAmount, daysInPeriod, usageDate, usagePeriod := computeDailyMetrics(freq, amount, rawStart, endDate)
						mergeKey := fmt.Sprintf("%s|%s|%s|%s|%s", rowAccountID, svcName, usageDate, usagePeriod, metric)

						if existing, ok := recordMap[mergeKey]; ok {
							// If record exists from prior tag pass, enrich with this tag without duplicating amount
							if tagKey != "" && tagVal != "" {
								assignTagToRecord(existing, tagKey, tagVal)
								if passIdx == 0 {
									existing.PrimaryTag = tagVal
								} else if passIdx == 1 {
									existing.SecondaryTag = tagVal
								}
							}
						} else {
							accName := rowAccountID
							if realName, ok := accountNames[rowAccountID]; ok && realName != "" {
								accName = realName
							}
							primaryVal := "Untagged"
							secondaryVal := "Untagged"
							if passIdx == 0 && tagVal != "" {
								primaryVal = tagVal
							} else if passIdx == 1 && tagVal != "" {
								secondaryVal = tagVal
							}
							rec := cloud.CostRecord{
								AccountID:    rowAccountID,
								AccountName:  accName,
								Service:      svcName,
								UsageDate:    usageDate,
								UsagePeriod:  usagePeriod,
								Amount:       amount,
								DailyAmount:  dailyAmount,
								DaysInPeriod: daysInPeriod,
								Metric:       metric,
								Frequency:    freq,
								Unit:         unit,
								Currency:     "USD",
								Region:       c.cfg.Region,
								UsageType:    "Usage",
								PrimaryTag:   primaryVal,
								SecondaryTag: secondaryVal,
								BusinessUnit: "Untagged",
								Project:      "Untagged",
								Application:  "Untagged",
								Environment:  "Untagged",
								Owner:        "Untagged",
								CostCenter:   "Untagged",
								Tags:         make(map[string]string),
								Timestamp:    time.Now(),
							}
							if tagKey != "" && tagVal != "" {
								assignTagToRecord(&rec, tagKey, tagVal)
							}
							recordMap[mergeKey] = &rec
							orderedKeys = append(orderedKeys, mergeKey)
						}
					}
				}

				pageCount := 0
				for _, rbt := range resp.ResultsByTime {
					pageCount += len(rbt.Groups)
				}
				slog.DebugContext(ctx, "Cost Explorer page processed",
					"pass", fmt.Sprintf("%d/%d", passIdx+1, len(passes)),
					"groupsInPage", pageCount,
					"totalUniqueRecordsSoFar", len(recordMap),
					"hasNextPage", resp.NextPageToken != nil && *resp.NextPageToken != "",
				)

				if resp.NextPageToken == nil || *resp.NextPageToken == "" {
					break
				}
				nextToken = resp.NextPageToken
			}
			return nil
		})

		if err != nil {
			if c.isMock {
				return GenerateMockCostRecordsWithOptions(opts), nil
			}
			return nil, fmt.Errorf("AWS Cost Explorer GetCostAndUsage failed (pass %d/%d): %w", passIdx+1, len(passes), err)
		}

		slog.InfoContext(ctx, "Cost Explorer pass completed",
			"pass", fmt.Sprintf("%d/%d", passIdx+1, len(passes)),
			"tagKey", tagKey,
			"totalRecordsSoFar", len(recordMap),
		)
	}

	var allRecords []cloud.CostRecord
	for _, k := range orderedKeys {
		if rec, ok := recordMap[k]; ok {
			allRecords = append(allRecords, *rec)
		}
	}

	slog.InfoContext(ctx, "Completed Cost Explorer query",
		"totalRecords", len(allRecords),
		"groupBy", groupBy,
		"metric", metric,
		"frequency", freq,
	)
	return allRecords, nil
}

// fetchHierarchicalCosts executes Hierarchical Two-Round Sense (Round 1: Account+PrimaryTag -> Round 2: Service+SecondaryTag).
func (c *Client) fetchHierarchicalCosts(
	ctx context.Context,
	opts FetchCostOptions,
	startDate, endDate string,
	granularity types.Granularity,
	freq, metric string,
	accountNames map[string]string,
	excludeRecordTypes []string,
) ([]cloud.CostRecord, error) {
	primaryTag := opts.PrimaryTag
	if primaryTag == "" {
		if opts.GroupByTagKey != "" {
			primaryTag = opts.GroupByTagKey
		} else if len(opts.GroupByTags) > 0 {
			primaryTag = opts.GroupByTags[0]
		} else {
			primaryTag = "Project"
		}
	}

	secondaryTag := opts.SecondaryTag
	if secondaryTag == "" {
		if len(opts.GroupByTags) > 1 {
			secondaryTag = opts.GroupByTags[1]
		} else {
			secondaryTag = "Application"
		}
	}

	slog.InfoContext(ctx, "Starting Hierarchical Two-Round Sense",
		"primaryTag", primaryTag,
		"secondaryTag", secondaryTag,
		"minCostThreshold", opts.MinCostThreshold,
	)

	// --- Round 1: Discovery (LINKED_ACCOUNT + primaryTag) ---
	var r1AndExprs []types.Expression
	if len(excludeRecordTypes) > 0 {
		r1AndExprs = append(r1AndExprs, types.Expression{
			Not: &types.Expression{
				Dimensions: &types.DimensionValues{
					Key:    types.DimensionRecordType,
					Values: excludeRecordTypes,
				},
			},
		})
	}
	if len(opts.IncludeAccounts) > 0 {
		r1AndExprs = append(r1AndExprs, types.Expression{
			Dimensions: &types.DimensionValues{
				Key:    types.DimensionLinkedAccount,
				Values: opts.IncludeAccounts,
			},
		})
	}
	var r1Filter *types.Expression
	if len(r1AndExprs) == 1 {
		r1Filter = &r1AndExprs[0]
	} else if len(r1AndExprs) > 1 {
		r1Filter = &types.Expression{And: r1AndExprs}
	}

	r1GroupDefs := []types.GroupDefinition{
		{Type: types.GroupDefinitionTypeDimension, Key: aws.String("LINKED_ACCOUNT")},
		{Type: types.GroupDefinitionTypeTag, Key: aws.String(primaryTag)},
	}

	type activePair struct {
		accountID  string
		projectTag string
		amount     float64
	}
	var activePairs []activePair
	pairMap := make(map[string]*activePair)

	err := ExecuteWithRetry(ctx, func() error {
		var nextToken *string
		pageIdx := 0
		for {
			pageIdx++
			input := &costexplorer.GetCostAndUsageInput{
				TimePeriod: &types.DateInterval{
					Start: aws.String(startDate),
					End:   aws.String(endDate),
				},
				Granularity:   granularity,
				Metrics:       []string{metric},
				GroupBy:       r1GroupDefs,
				Filter:        r1Filter,
				NextPageToken: nextToken,
			}
			slog.DebugContext(ctx, "Calling Cost Explorer GetCostAndUsage for Round 1 (Discovery)",
				"primaryTag", primaryTag,
				"page", pageIdx,
				"hasNextToken", nextToken != nil,
			)
			resp, callErr := c.costExplorer.GetCostAndUsage(ctx, input)
			if callErr != nil {
				slog.ErrorContext(ctx, "Hierarchical Sense Round 1 GetCostAndUsage failed", "page", pageIdx, "error", callErr)
				return callErr
			}
			pagePairs := 0
			for _, resultByTime := range resp.ResultsByTime {
				for _, group := range resultByTime.Groups {
					rowAccountID := opts.AccountID
					if opts.PayerAccountID != "" {
						rowAccountID = opts.PayerAccountID
					}
					var pTag string
					for idx, key := range group.Keys {
						if idx < len(r1GroupDefs) {
							def := r1GroupDefs[idx]
							if def.Type == types.GroupDefinitionTypeDimension && *def.Key == "LINKED_ACCOUNT" {
								rowAccountID = key
							} else if def.Type == types.GroupDefinitionTypeTag {
								val := strings.TrimPrefix(key, primaryTag+"$")
								if val == "" || val == key {
									val = "Untagged"
								}
								pTag = val
							}
						}
					}
					amount := 0.0
					if m, ok := group.Metrics[metric]; ok && m.Amount != nil {
						fmt.Sscanf(*m.Amount, "%f", &amount)
					}
					pairKey := fmt.Sprintf("%s|%s", rowAccountID, pTag)
					if existing, exists := pairMap[pairKey]; exists {
						existing.amount += amount
					} else {
						pairMap[pairKey] = &activePair{
							accountID:  rowAccountID,
							projectTag: pTag,
							amount:     amount,
						}
					}
					pagePairs++
				}
			}
			slog.DebugContext(ctx, "Hierarchical Sense Round 1 page processed",
				"page", pageIdx,
				"groupsInPage", pagePairs,
				"uniquePairsSoFar", len(pairMap),
				"hasNextPage", resp.NextPageToken != nil && *resp.NextPageToken != "",
			)
			if resp.NextPageToken == nil || *resp.NextPageToken == "" {
				break
			}
			nextToken = resp.NextPageToken
		}
		return nil
	})
	if err != nil {
		if c.isMock {
			return GenerateMockCostRecordsWithOptions(opts), nil
		}
		return nil, fmt.Errorf("hierarchical sense Round 1 failed: %w", err)
	}

	for _, p := range pairMap {
		if opts.MinCostThreshold > 0 && p.amount < opts.MinCostThreshold {
			continue
		}
		activePairs = append(activePairs, *p)
	}
	sort.Slice(activePairs, func(i, j int) bool {
		if activePairs[i].accountID != activePairs[j].accountID {
			return activePairs[i].accountID < activePairs[j].accountID
		}
		return activePairs[i].projectTag < activePairs[j].projectTag
	})

	slog.InfoContext(ctx, "Hierarchical Sense Round 1 completed",
		"activePairsCount", len(activePairs),
		"primaryTag", primaryTag,
	)

	if len(activePairs) == 0 {
		return nil, nil
	}

	// --- Round 2: Two-Phase Drill-down ---
	// Phase 2a: Query tagged projects with specific Tag filter
	// Phase 2b: Query untagged accounts with net deduction of tagged spend to prevent duplicate records and 2x costs
	var taggedPairs []activePair
	var untaggedPairs []activePair
	for _, p := range activePairs {
		if p.projectTag == "" || p.projectTag == "Untagged" {
			if p.amount > 0.0001 {
				untaggedPairs = append(untaggedPairs, p)
			}
		} else {
			taggedPairs = append(taggedPairs, p)
		}
	}

	totalPairs := len(taggedPairs) + len(untaggedPairs)
	slog.InfoContext(ctx, "Starting Hierarchical Sense Round 2 drill-down",
		"taggedPairsCount", len(taggedPairs),
		"untaggedPairsCount", len(untaggedPairs),
		"primaryTag", primaryTag,
		"secondaryTag", secondaryTag,
		"concurrency", 5,
	)

	var mu sync.Mutex
	var hierRecords []cloud.CostRecord
	taggedAmounts := make(map[string]float64) // key -> sum(amount) already accounted for by tagged projects
	var completedPairs atomic.Int32
	var totalRecordsCount atomic.Int64

	// Phase 2a: Tagged projects drill-down
	g1, g1Ctx := errgroup.WithContext(ctx)
	g1.SetLimit(5)

	for _, pair := range taggedPairs {
		p := pair
		g1.Go(func() error {
			var r2AndExprs []types.Expression
			if len(excludeRecordTypes) > 0 {
				r2AndExprs = append(r2AndExprs, types.Expression{
					Not: &types.Expression{
						Dimensions: &types.DimensionValues{
							Key:    types.DimensionRecordType,
							Values: excludeRecordTypes,
						},
					},
				})
			}
			r2AndExprs = append(r2AndExprs, types.Expression{
				Dimensions: &types.DimensionValues{
					Key:    types.DimensionLinkedAccount,
					Values: []string{p.accountID},
				},
			})
			r2AndExprs = append(r2AndExprs, types.Expression{
				Tags: &types.TagValues{
					Key:    aws.String(primaryTag),
					Values: []string{p.projectTag},
				},
			})
			if opts.Service != "" && opts.Service != "all" {
				r2AndExprs = append(r2AndExprs, types.Expression{
					Dimensions: &types.DimensionValues{
						Key:          types.DimensionService,
						Values:       []string{opts.Service},
						MatchOptions: []types.MatchOption{types.MatchOptionContains},
					},
				})
			}

			var r2Filter *types.Expression
			if len(r2AndExprs) == 1 {
				r2Filter = &r2AndExprs[0]
			} else if len(r2AndExprs) > 1 {
				r2Filter = &types.Expression{And: r2AndExprs}
			}

			r2GroupDefs := []types.GroupDefinition{
				{Type: types.GroupDefinitionTypeDimension, Key: aws.String("SERVICE")},
				{Type: types.GroupDefinitionTypeTag, Key: aws.String(secondaryTag)},
			}

			return ExecuteWithRetry(g1Ctx, func() error {
				var nextToken *string
				var pairRecords []cloud.CostRecord
				pageIdx := 0
				for {
					pageIdx++
					input := &costexplorer.GetCostAndUsageInput{
						TimePeriod: &types.DateInterval{
							Start: aws.String(startDate),
							End:   aws.String(endDate),
						},
						Granularity:   granularity,
						Metrics:       []string{metric},
						GroupBy:       r2GroupDefs,
						Filter:        r2Filter,
						NextPageToken: nextToken,
					}

					resp, callErr := c.costExplorer.GetCostAndUsage(g1Ctx, input)
					if callErr != nil {
						slog.ErrorContext(g1Ctx, "Round 2 Cost Explorer GetCostAndUsage failed",
							"accountID", p.accountID,
							"primaryTag", p.projectTag,
							"page", pageIdx,
							"error", callErr,
						)
						return callErr
					}

					var pageRecords []cloud.CostRecord
					for _, resultByTime := range resp.ResultsByTime {
						rawStart := aws.ToString(resultByTime.TimePeriod.Start)
						for _, group := range resultByTime.Groups {
							svcName := "Unknown"
							secTagVal := "Untagged"

							for idx, key := range group.Keys {
								if idx < len(r2GroupDefs) {
									def := r2GroupDefs[idx]
									if def.Type == types.GroupDefinitionTypeDimension && *def.Key == "SERVICE" {
										svcName = key
									} else if def.Type == types.GroupDefinitionTypeTag {
										val := strings.TrimPrefix(key, secondaryTag+"$")
										if val != "" && val != key {
											secTagVal = val
										}
									}
								}
							}

							amount := 0.0
							unit := "USD"
							if metric == "UsageQuantity" {
								unit = "Quantity"
							}
							if m, ok := group.Metrics[metric]; ok {
								if m.Amount != nil {
									fmt.Sscanf(*m.Amount, "%f", &amount)
								}
								if m.Unit != nil && *m.Unit != "" {
									unit = *m.Unit
								}
							}

							if opts.MinCostThreshold > 0 && amount < opts.MinCostThreshold {
								continue
							}

							dailyAmount, daysInPeriod, usageDate, usagePeriod := computeDailyMetrics(freq, amount, rawStart, endDate)
							accName := p.accountID
							if realName, ok := accountNames[p.accountID]; ok && realName != "" {
								accName = realName
							}

							rec := cloud.CostRecord{
								AccountID:    p.accountID,
								AccountName:  accName,
								Service:      svcName,
								UsageDate:    usageDate,
								UsagePeriod:  usagePeriod,
								Amount:       amount,
								DailyAmount:  dailyAmount,
								DaysInPeriod: daysInPeriod,
								Metric:       metric,
								Frequency:    freq,
								Unit:         unit,
								Currency:     "USD",
								Region:       c.cfg.Region,
								UsageType:    "Usage",
								PrimaryTag:   p.projectTag,
								SecondaryTag: secTagVal,
								BusinessUnit: "Untagged",
								Project:      "Untagged",
								Application:  "Untagged",
								Environment:  "Untagged",
								Owner:        "Untagged",
								CostCenter:   "Untagged",
								Tags:         make(map[string]string),
								Timestamp:    time.Now(),
							}
							assignTagToRecord(&rec, primaryTag, p.projectTag)
							assignTagToRecord(&rec, secondaryTag, secTagVal)

							pageRecords = append(pageRecords, rec)
						}
					}

					pairRecords = append(pairRecords, pageRecords...)

					if resp.NextPageToken == nil || *resp.NextPageToken == "" {
						break
					}
					nextToken = resp.NextPageToken
				}

				if len(pairRecords) > 0 {
					mu.Lock()
					for _, rec := range pairRecords {
						key := fmt.Sprintf("%s|%s|%s|%s|%s", rec.AccountID, rec.Service, rec.SecondaryTag, rec.UsageDate, rec.UsagePeriod)
						taggedAmounts[key] += rec.Amount
						hierRecords = append(hierRecords, rec)
					}
					mu.Unlock()
				}
				currentTotal := totalRecordsCount.Add(int64(len(pairRecords)))
				done := completedPairs.Add(1)

				slog.InfoContext(g1Ctx, "Round 2 tagged pair completed",
					"progress", fmt.Sprintf("%d/%d", done, totalPairs),
					"accountID", p.accountID,
					"primaryTag", p.projectTag,
					"pairRecords", len(pairRecords),
					"totalRecordsSoFar", currentTotal,
				)

				return nil
			})
		})
	}

	if err := g1.Wait(); err != nil {
		if c.isMock {
			return GenerateMockCostRecordsWithOptions(opts), nil
		}
		return nil, fmt.Errorf("hierarchical sense Round 2 tagged query failed: %w", err)
	}

	// Phase 2b: Untagged accounts query with net deduction of tagged spend
	if len(untaggedPairs) > 0 {
		g2, g2Ctx := errgroup.WithContext(ctx)
		g2.SetLimit(5)

		for _, pair := range untaggedPairs {
			p := pair
			g2.Go(func() error {
				var r2AndExprs []types.Expression
				if len(excludeRecordTypes) > 0 {
					r2AndExprs = append(r2AndExprs, types.Expression{
						Not: &types.Expression{
							Dimensions: &types.DimensionValues{
								Key:    types.DimensionRecordType,
								Values: excludeRecordTypes,
							},
						},
					})
				}
				r2AndExprs = append(r2AndExprs, types.Expression{
					Dimensions: &types.DimensionValues{
						Key:    types.DimensionLinkedAccount,
						Values: []string{p.accountID},
					},
				})
				if opts.Service != "" && opts.Service != "all" {
					r2AndExprs = append(r2AndExprs, types.Expression{
						Dimensions: &types.DimensionValues{
							Key:          types.DimensionService,
							Values:       []string{opts.Service},
							MatchOptions: []types.MatchOption{types.MatchOptionContains},
						},
					})
				}

				var r2Filter *types.Expression
				if len(r2AndExprs) == 1 {
					r2Filter = &r2AndExprs[0]
				} else if len(r2AndExprs) > 1 {
					r2Filter = &types.Expression{And: r2AndExprs}
				}

				r2GroupDefs := []types.GroupDefinition{
					{Type: types.GroupDefinitionTypeDimension, Key: aws.String("SERVICE")},
					{Type: types.GroupDefinitionTypeTag, Key: aws.String(secondaryTag)},
				}

				return ExecuteWithRetry(g2Ctx, func() error {
					var nextToken *string
					var pairRecords []cloud.CostRecord
					pageIdx := 0
					for {
						pageIdx++
						input := &costexplorer.GetCostAndUsageInput{
							TimePeriod: &types.DateInterval{
								Start: aws.String(startDate),
								End:   aws.String(endDate),
							},
							Granularity:   granularity,
							Metrics:       []string{metric},
							GroupBy:       r2GroupDefs,
							Filter:        r2Filter,
							NextPageToken: nextToken,
						}

						resp, callErr := c.costExplorer.GetCostAndUsage(g2Ctx, input)
						if callErr != nil {
							slog.ErrorContext(g2Ctx, "Round 2 Cost Explorer Untagged GetCostAndUsage failed",
								"accountID", p.accountID,
								"page", pageIdx,
								"error", callErr,
							)
							return callErr
						}

						var pageRecords []cloud.CostRecord
						for _, resultByTime := range resp.ResultsByTime {
							rawStart := aws.ToString(resultByTime.TimePeriod.Start)
							for _, group := range resultByTime.Groups {
								svcName := "Unknown"
								secTagVal := "Untagged"

								for idx, key := range group.Keys {
									if idx < len(r2GroupDefs) {
										def := r2GroupDefs[idx]
										if def.Type == types.GroupDefinitionTypeDimension && *def.Key == "SERVICE" {
											svcName = key
										} else if def.Type == types.GroupDefinitionTypeTag {
											val := strings.TrimPrefix(key, secondaryTag+"$")
											if val != "" && val != key {
												secTagVal = val
											}
										}
									}
								}

								amount := 0.0
								unit := "USD"
								if metric == "UsageQuantity" {
									unit = "Quantity"
								}
								if m, ok := group.Metrics[metric]; ok {
									if m.Amount != nil {
										fmt.Sscanf(*m.Amount, "%f", &amount)
									}
									if m.Unit != nil && *m.Unit != "" {
										unit = *m.Unit
									}
								}

								_, _, usageDate, usagePeriod := computeDailyMetrics(freq, amount, rawStart, endDate)
								key := fmt.Sprintf("%s|%s|%s|%s|%s", p.accountID, svcName, secTagVal, usageDate, usagePeriod)

								mu.Lock()
								taggedAmt := taggedAmounts[key]
								mu.Unlock()

								untaggedAmt := amount - taggedAmt
								if untaggedAmt <= 0.0001 {
									// Fully accounted for by tagged project records; skip to prevent duplicate records and double cost
									continue
								}

								if opts.MinCostThreshold > 0 && untaggedAmt < opts.MinCostThreshold {
									continue
								}

								dailyAmount, daysInPeriod, _, _ := computeDailyMetrics(freq, untaggedAmt, rawStart, endDate)
								accName := p.accountID
								if realName, ok := accountNames[p.accountID]; ok && realName != "" {
									accName = realName
								}

								rec := cloud.CostRecord{
									AccountID:    p.accountID,
									AccountName:  accName,
									Service:      svcName,
									UsageDate:    usageDate,
									UsagePeriod:  usagePeriod,
									Amount:       untaggedAmt,
									DailyAmount:  dailyAmount,
									DaysInPeriod: daysInPeriod,
									Metric:       metric,
									Frequency:    freq,
									Unit:         unit,
									Currency:     "USD",
									Region:       c.cfg.Region,
									UsageType:    "Usage",
									PrimaryTag:   "Untagged",
									SecondaryTag: secTagVal,
									BusinessUnit: "Untagged",
									Project:      "Untagged",
									Application:  secTagVal,
									Environment:  "Untagged",
									Owner:        "Untagged",
									CostCenter:   "Untagged",
									Tags:         make(map[string]string),
									Timestamp:    time.Now(),
								}
								assignTagToRecord(&rec, primaryTag, "Untagged")
								assignTagToRecord(&rec, secondaryTag, secTagVal)

								pageRecords = append(pageRecords, rec)
							}
						}

						pairRecords = append(pairRecords, pageRecords...)

						if resp.NextPageToken == nil || *resp.NextPageToken == "" {
							break
						}
						nextToken = resp.NextPageToken
					}

					if len(pairRecords) > 0 {
						mu.Lock()
						hierRecords = append(hierRecords, pairRecords...)
						mu.Unlock()
					}
					currentTotal := totalRecordsCount.Add(int64(len(pairRecords)))
					done := completedPairs.Add(1)

					slog.InfoContext(g2Ctx, "Round 2 untagged deduction completed",
						"progress", fmt.Sprintf("%d/%d", done, totalPairs),
						"accountID", p.accountID,
						"pairRecords", len(pairRecords),
						"totalRecordsSoFar", currentTotal,
					)

					return nil
				})
			})
		}

		if err := g2.Wait(); err != nil {
			if c.isMock {
				return GenerateMockCostRecordsWithOptions(opts), nil
			}
			return nil, fmt.Errorf("hierarchical sense Round 2 untagged query failed: %w", err)
		}
	}

	slog.InfoContext(ctx, "Completed Hierarchical Cost Explorer query",
		"activePairsCount", len(activePairs),
		"totalRecords", len(hierRecords),
		"primaryTag", primaryTag,
		"secondaryTag", secondaryTag,
	)
	return hierRecords, nil
}

// FetchAccountCosts queries AWS Cost Explorer or generates mock records if in mock mode.
func (c *Client) FetchAccountCosts(ctx context.Context, accountID, startDate, endDate string) ([]cloud.CostRecord, error) {
	return c.FetchCostsWithOptions(ctx, FetchCostOptions{
		AccountID: accountID,
		StartDate: startDate,
		EndDate:   endDate,
	})
}

// FetchCostAnomalies queries AWS Cost Explorer Anomaly Detection API or returns mock anomalies.
func (c *Client) FetchCostAnomalies(ctx context.Context, accountID, startDate, endDate string) ([]cloud.AnomalyRecord, error) {
	if c.isMock || c.costExplorer == nil {
		slog.DebugContext(ctx, "Generating mock Cost Anomaly records", "account", accountID)
		return GenerateMockCostAnomalies(accountID), nil
	}

	if startDate == "" {
		startDate = time.Now().AddDate(0, 0, -30).Format("2006-01-02")
	}
	if endDate == "" {
		endDate = time.Now().Format("2006-01-02")
	}

	slog.InfoContext(ctx, "Querying AWS Cost Anomalies", "account", accountID, "startDate", startDate, "endDate", endDate)

	var records []cloud.AnomalyRecord
	err := ExecuteWithRetry(ctx, func() error {
		input := &costexplorer.GetAnomaliesInput{
			DateInterval: &types.AnomalyDateInterval{
				StartDate: aws.String(startDate),
				EndDate:   aws.String(endDate),
			},
		}
		resp, callErr := c.costExplorer.GetAnomalies(ctx, input)
		if callErr != nil {
			return callErr
		}
		now := time.Now()
		for _, a := range resp.Anomalies {
			score := 0.0
			if a.AnomalyScore != nil {
				score = a.AnomalyScore.CurrentScore
			}
			actualVal := 0.0
			expectedVal := 0.0
			if a.Impact != nil {
				actualVal = a.Impact.TotalImpact
				if a.Impact.TotalActualSpend != nil {
					actualVal = *a.Impact.TotalActualSpend
				}
				if a.Impact.TotalExpectedSpend != nil {
					expectedVal = *a.Impact.TotalExpectedSpend
				}
			}
			svc := "AWS Cost Anomaly"
			if a.DimensionValue != nil {
				svc = *a.DimensionValue
			}
			anomalyID := ""
			if a.AnomalyId != nil {
				anomalyID = *a.AnomalyId
			}
			sev := "MEDIUM"
			if score >= 80 || actualVal >= 500 {
				sev = "CRITICAL"
			} else if score >= 50 || actualVal >= 100 {
				sev = "HIGH"
			}
			records = append(records, cloud.AnomalyRecord{
				OpsDomain:     "finops",
				AccountID:     accountID,
				AccountName:   fmt.Sprintf("LandingZone-%s", accountID),
				Service:       svc,
				ResourceOrKey: anomalyID,
				Metric:        "CostAnomaly",
				ActualValue:   actualVal,
				ExpectedValue: expectedVal,
				Deviation:     score / 100.0,
				Severity:      sev,
				Status:        "OPEN",
				Timestamp:     now,
			})
		}
		return nil
	})

	if err != nil {
		if c.isMock {
			return GenerateMockCostAnomalies(accountID), nil
		}
		return nil, fmt.Errorf("AWS Cost Anomaly GetAnomalies failed: %w", err)
	}
	slog.InfoContext(ctx, "Completed Cost Anomalies query", "anomalyCount", len(records))
	return records, nil
}
