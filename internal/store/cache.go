package store

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"time"
)

const cacheTTLCurrentMonth = 12 * time.Hour

// EnsureCacheSchema creates the Cost Explorer cache table.
func EnsureCacheSchema(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS ce_cache (
		request_hash VARCHAR PRIMARY KEY,
		response_body VARCHAR NOT NULL,
		expires_at TIMESTAMP NOT NULL
	)`)
	return err
}

// CacheGet returns a cached response body when still valid.
func CacheGet(db *sql.DB, requestHash string, now time.Time) (string, bool, error) {
	var body string
	var expires time.Time
	err := db.QueryRow(`SELECT response_body, expires_at FROM ce_cache WHERE request_hash = ?`, requestHash).Scan(&body, &expires)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	if now.After(expires) {
		return "", false, nil
	}
	return body, true, nil
}

// CachePut stores a response with expiry.
func CachePut(db *sql.DB, requestHash, body string, expires time.Time) error {
	_, err := db.Exec(`INSERT OR REPLACE INTO ce_cache (request_hash, response_body, expires_at) VALUES (?, ?, ?)`,
		requestHash, body, expires.UTC())
	return err
}

// HashRequest returns SHA-256 hex of the request key.
func HashRequest(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

// ExpiryForRange picks cache TTL: closed months forever, ranges touching current month 12h.
func ExpiryForRange(toMonth string, now time.Time) time.Time {
	cur := now.UTC().Format("2006-01")
	if toMonth >= cur {
		return now.UTC().Add(cacheTTLCurrentMonth)
	}
	return time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC)
}
