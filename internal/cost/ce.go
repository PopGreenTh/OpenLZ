package cost

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

func storeHash(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

func expiryForRange(toMonth string, now time.Time) time.Time {
	cur := now.UTC().Format("2006-01")
	if toMonth >= cur {
		return now.UTC().Add(12 * time.Hour)
	}
	return time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC)
}

// CostExplorerAPI is the subset of Cost Explorer used for sense (mockable in tests).
type CostExplorerAPI interface {
	GetCostAndUsagePages(ctx context.Context, req Request) ([]RawRow, error)
}

// Request describes one Cost Explorer pull.
type Request struct {
	From, To  string
	Metric    string
	AppTagKey string
	LZCode    string
}

// FetchRawCost calls CE with retry and paging.
func FetchRawCost(ctx context.Context, api CostExplorerAPI, req Request) ([]RawRow, error) {
	return api.GetCostAndUsagePages(ctx, req)
}

// FetchRawCostCached loads raw rows through the DuckDB cache when db is non-nil.
func FetchRawCostCached(ctx context.Context, api CostExplorerAPI, db interface {
	Get(string, time.Time) (string, bool, error)
	Put(string, string, time.Time) error
}, req Request, now time.Time, refresh bool) ([]RawRow, int, error) {
	keyBytes, _ := json.Marshal(req)
	hash := storeHash(string(keyBytes))
	if db != nil && !refresh {
		if body, ok, err := db.Get(hash, now); err == nil && ok {
			var rows []RawRow
			if err := json.Unmarshal([]byte(body), &rows); err == nil {
				return rows, 0, nil
			}
		}
	}
	rows, err := FetchRawCost(ctx, api, req)
	if err != nil {
		return nil, 0, err
	}
	calls := 1
	if db != nil {
		body, _ := json.Marshal(rows)
		exp := expiryForRange(req.To, now)
		_ = db.Put(hash, string(body), exp)
	}
	return rows, calls, nil
}

// ParseGroupKeys splits Cost Explorer group keys into account and tag value.
func ParseGroupKeys(keys []string, appTagKey string) (accountID, tagValue string, err error) {
	for _, k := range keys {
		if strings.HasPrefix(k, "LINKED_ACCOUNT") {
			parts := strings.SplitN(k, "$", 2)
			if len(parts) == 2 {
				accountID = parts[1]
			}
		}
		if strings.HasPrefix(k, appTagKey+"$") || strings.Contains(k, appTagKey+"$") {
			tagValue = ParseTagGroupKey(k)
		}
	}
	if accountID == "" {
		return "", "", fmt.Errorf("missing linked account in group keys")
	}
	return accountID, tagValue, nil
}

// ParseAmount reads the metric amount from a CE metric value map.
func ParseAmount(values map[string]string, metric string) (float64, error) {
	v, ok := values[metric]
	if !ok {
		return 0, nil
	}
	return strconv.ParseFloat(v, 64)
}

// ThrottleOnce wraps a call to simulate throttling once (tests).
func ThrottleOnce(fn func(context.Context) error) func(context.Context) error {
	var n int
	return func(ctx context.Context) error {
		n++
		if n == 1 {
			return throttleErr{}
		}
		return fn(ctx)
	}
}

type throttleErr struct{}

func (throttleErr) Error() string { return "ThrottlingException" }
