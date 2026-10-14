// Package cache persists cloud API responses in a local DuckDB file so repeated
// queries do not pay the Cost Explorer per-request fee ($0.01 per call) again.
package cache

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	_ "github.com/marcboeker/go-duckdb"
)

// DuckDBCache stores raw API payloads keyed by namespace, cache key, and query date.
type DuckDBCache struct {
	db *sql.DB
	mu sync.Mutex
}

// NewDuckDBCache opens (or creates) the persistent cache database at path.
func NewDuckDBCache(path string) (*DuckDBCache, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create cache dir: %w", err)
		}
	}
	db, err := sql.Open("duckdb", path)
	if err != nil {
		return nil, fmt.Errorf("open duckdb cache %s: %w", path, err)
	}
	if _, err := db.Exec(`
	CREATE TABLE IF NOT EXISTS api_cache (
		namespace  VARCHAR NOT NULL,
		cache_key  VARCHAR NOT NULL,
		query_date VARCHAR NOT NULL,
		payload    BLOB,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (namespace, cache_key, query_date)
	);`); err != nil {
		db.Close()
		return nil, fmt.Errorf("create api_cache table: %w", err)
	}
	return &DuckDBCache{db: db}, nil
}

// Get returns the cached payload and whether it was found.
func (c *DuckDBCache) Get(ctx context.Context, namespace, key, queryDate string) ([]byte, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	var payload []byte
	err := c.db.QueryRowContext(ctx,
		`SELECT payload FROM api_cache WHERE namespace = ? AND cache_key = ? AND query_date = ?`,
		namespace, key, queryDate,
	).Scan(&payload)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("read api cache: %w", err)
	}
	return payload, true, nil
}

// Set stores or replaces the payload for the given key.
func (c *DuckDBCache) Set(ctx context.Context, namespace, key, queryDate string, payload []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, err := c.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO api_cache (namespace, cache_key, query_date, payload, created_at)
		 VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)`,
		namespace, key, queryDate, payload,
	)
	if err != nil {
		return fmt.Errorf("write api cache: %w", err)
	}
	return nil
}

// Close releases the cache database.
func (c *DuckDBCache) Close() error {
	if c == nil || c.db == nil {
		return nil
	}
	return c.db.Close()
}
