package cost

import (
	"context"
	"testing"
	"time"
)

type fakeCE struct {
	pages [][]RawRow
	calls int
}

func (f *fakeCE) GetCostAndUsagePages(ctx context.Context, req Request) ([]RawRow, error) {
	f.calls++
	if len(f.pages) == 0 {
		return nil, nil
	}
	out := f.pages[0]
	f.pages = f.pages[1:]
	return out, nil
}

func TestCEPagingAndCacheHit(t *testing.T) {
	fake := &fakeCE{pages: [][]RawRow{
		{{LZ: "LZ-MAIN", AccountID: "1", Month: "2026-08", Metric: "UnblendedCost", Amount: 1, Currency: "USD"}},
		{{LZ: "LZ-MAIN", AccountID: "2", Month: "2026-08", Metric: "UnblendedCost", Amount: 2, Currency: "USD"}},
	}}
	ctx := context.Background()
	req := Request{From: "2026-08", To: "2026-08", Metric: "UnblendedCost", AppTagKey: "Application", LZCode: "LZ-MAIN"}
	page1, err := FetchRawCost(ctx, fake, req)
	if err != nil || len(page1) != 1 {
		t.Fatalf("page1: %+v err=%v", page1, err)
	}
	page2, err := FetchRawCost(ctx, fake, req)
	if err != nil || len(page2) != 1 || fake.calls != 2 {
		t.Fatalf("page2: %+v calls=%d err=%v", page2, fake.calls, err)
	}

	mem := newMemCache()
	now := mustTime("2026-09-01")
	fake2 := &fakeCE{pages: [][]RawRow{{{LZ: "LZ-MAIN", AccountID: "9", Month: "2026-08", Metric: "UnblendedCost", Amount: 9, Currency: "USD"}}}}
	rows1, calls1, err := FetchRawCostCached(ctx, fake2, mem, req, now, false)
	if err != nil || calls1 != 1 || len(rows1) != 1 {
		t.Fatalf("first cached fetch: rows=%d calls=%d err=%v", len(rows1), calls1, err)
	}
	rows2, calls2, err := FetchRawCostCached(ctx, fake2, mem, req, now, false)
	if err != nil || calls2 != 0 || len(rows2) != 1 {
		t.Fatalf("cache hit: rows=%d calls=%d err=%v", len(rows2), calls2, err)
	}
}

type memCache struct {
	data map[string]struct {
		body    string
		expires time.Time
	}
}

func newMemCache() *memCache {
	return &memCache{data: make(map[string]struct {
		body    string
		expires time.Time
	})}
}

func (m *memCache) Get(hash string, now time.Time) (string, bool, error) {
	v, ok := m.data[hash]
	if !ok || now.After(v.expires) {
		return "", false, nil
	}
	return v.body, true, nil
}

func (m *memCache) Put(hash, body string, expires time.Time) error {
	m.data[hash] = struct {
		body    string
		expires time.Time
	}{body, expires}
	return nil
}

func mustTime(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}
