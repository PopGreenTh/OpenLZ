package store

import (
	"fmt"
	"os"
	"path/filepath"
)

// RawRecord is one raw cost row for Parquet I/O.
type RawRecord struct {
	LZ, AccountID, Month, Metric, AppTag, Currency string
	Amount                                         float64
	Estimated                                      bool
}

// AllocatedRecord is one allocated cost row for Parquet I/O.
type AllocatedRecord struct {
	LZ, AccountID, AccountName, OUPath, Project, App, Environment string
	BusinessUnit, CostCenter, ShareRule, AppRule, EnvironmentRule string
	AppTag, Month, Metric, Currency, TaxonomyHash                  string
	Share                                                          float64
	Amount                                                         float64
	Estimated                                                      bool
}

// WriteRawParquet writes raw records to Parquet via DuckDB.
func WriteRawParquet(path string, rows []RawRecord) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}
	db, err := Open("")
	if err != nil {
		return err
	}
	defer db.Close()
	if _, err := db.SQL.Exec(`CREATE TABLE raw (
		lz VARCHAR, account_id VARCHAR, month VARCHAR, metric VARCHAR,
		amount DOUBLE, app_tag VARCHAR, currency VARCHAR, estimated BOOLEAN
	)`); err != nil {
		return fmt.Errorf("create temp table: %w", err)
	}
	for _, r := range rows {
		_, err := db.SQL.Exec(`INSERT INTO raw VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			r.LZ, r.AccountID, r.Month, r.Metric, r.Amount, r.AppTag, r.Currency, r.Estimated)
		if err != nil {
			return fmt.Errorf("insert row: %w", err)
		}
	}
	escaped := filepath.ToSlash(path)
	_, err = db.SQL.Exec(fmt.Sprintf(`COPY raw TO '%s' (FORMAT PARQUET)`, escaped))
	return err
}

// ReadRawParquet reads raw records from Parquet.
func ReadRawParquet(path string) ([]RawRecord, error) {
	db, err := Open("")
	if err != nil {
		return nil, err
	}
	defer db.Close()
	escaped := filepath.ToSlash(path)
	q, err := db.SQL.Query(fmt.Sprintf(`SELECT lz, account_id, month, metric, amount, app_tag, currency, estimated FROM read_parquet('%s')`, escaped))
	if err != nil {
		return nil, err
	}
	defer q.Close()
	var out []RawRecord
	for q.Next() {
		var r RawRecord
		if err := q.Scan(&r.LZ, &r.AccountID, &r.Month, &r.Metric, &r.Amount, &r.AppTag, &r.Currency, &r.Estimated); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, q.Err()
}

// WriteAllocatedParquet writes allocated records to Parquet.
func WriteAllocatedParquet(path string, rows []AllocatedRecord) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	db, err := Open("")
	if err != nil {
		return err
	}
	defer db.Close()
	if _, err := db.SQL.Exec(`CREATE TABLE allocated (
		lz VARCHAR, account_id VARCHAR, account_name VARCHAR, ou_path VARCHAR,
		project VARCHAR, app VARCHAR, environment VARCHAR, business_unit VARCHAR, cost_center VARCHAR,
		share DOUBLE, share_rule VARCHAR, app_rule VARCHAR, environment_rule VARCHAR,
		app_tag VARCHAR, month VARCHAR, metric VARCHAR, amount DOUBLE, currency VARCHAR,
		estimated BOOLEAN, taxonomy_hash VARCHAR
	)`); err != nil {
		return err
	}
	for _, r := range rows {
		_, err := db.SQL.Exec(`INSERT INTO allocated VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			r.LZ, r.AccountID, r.AccountName, r.OUPath, r.Project, r.App, r.Environment, r.BusinessUnit, r.CostCenter,
			r.Share, r.ShareRule, r.AppRule, r.EnvironmentRule, r.AppTag, r.Month, r.Metric, r.Amount, r.Currency, r.Estimated, r.TaxonomyHash)
		if err != nil {
			return err
		}
	}
	escaped := filepath.ToSlash(path)
	_, err = db.SQL.Exec(fmt.Sprintf(`COPY allocated TO '%s' (FORMAT PARQUET)`, escaped))
	return err
}
