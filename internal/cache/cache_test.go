package cache

import (
	"context"
	"path/filepath"
	"testing"
)

func TestDuckDBCacheRoundTrip(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "sub", "cache.duckdb")

	c, err := NewDuckDBCache(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, found, err := c.Get(ctx, "ns", "k", "2026-09-01"); err != nil || found {
		t.Fatalf("expected miss, found=%v err=%v", found, err)
	}
	if err := c.Set(ctx, "ns", "k", "2026-09-01", []byte("one")); err != nil {
		t.Fatalf("set: %v", err)
	}
	if err := c.Set(ctx, "ns", "k", "2026-09-01", []byte("two")); err != nil {
		t.Fatalf("replace: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	c, err = NewDuckDBCache(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer c.Close()
	got, found, err := c.Get(ctx, "ns", "k", "2026-09-01")
	if err != nil || !found || string(got) != "two" {
		t.Fatalf("expected persisted payload two, got %q found=%v err=%v", got, found, err)
	}
	if _, found, _ := c.Get(ctx, "ns", "k", "2026-09-02"); found {
		t.Fatal("different query date must miss")
	}
}
