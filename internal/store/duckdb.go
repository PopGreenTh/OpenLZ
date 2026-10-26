package store

import (
	"database/sql"
	"fmt"

	_ "github.com/marcboeker/go-duckdb"
)

// DB wraps an embedded DuckDB connection.
type DB struct {
	SQL *sql.DB
}

// Open opens or creates a DuckDB database file.
func Open(path string) (*DB, error) {
	conn, err := sql.Open("duckdb", path)
	if err != nil {
		return nil, fmt.Errorf("open duckdb: %w", err)
	}
	return &DB{SQL: conn}, nil
}

// Close closes the database.
func (db *DB) Close() error {
	if db == nil || db.SQL == nil {
		return nil
	}
	return db.SQL.Close()
}
