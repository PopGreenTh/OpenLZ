package store

import (
	"testing"
	"time"
)

func TestCacheHitMissAndExpiry(t *testing.T) {
	db, err := Open("")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	if err := EnsureCacheSchema(db.SQL); err != nil {
		t.Fatalf("schema: %v", err)
	}
	hash := HashRequest("req-1")
	now := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
	expires := now.Add(time.Hour)
	if err := CachePut(db.SQL, hash, "body", expires); err != nil {
		t.Fatalf("put: %v", err)
	}
	body, ok, err := CacheGet(db.SQL, hash, now)
	if err != nil || !ok || body != "body" {
		t.Fatalf("hit: ok=%v body=%q err=%v", ok, body, err)
	}
	_, ok, err = CacheGet(db.SQL, hash, now.Add(2*time.Hour))
	if err != nil || ok {
		t.Fatalf("expected miss after expiry, ok=%v err=%v", ok, err)
	}
}
