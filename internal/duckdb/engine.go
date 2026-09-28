package duckdb

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/PopGreenTh/OpenLZ/internal/cloud"
	duckdbDriver "github.com/marcboeker/go-duckdb"
)

// Engine handles embedded DuckDB OLAP transformations, statistical analysis, and Parquet I/O.
type Engine struct {
	db *sql.DB
}

// NewEngine initializes an in-memory DuckDB analytical engine.
func NewEngine() (*Engine, error) {
	db, err := sql.Open("duckdb", "")
	if err != nil {
		return nil, fmt.Errorf("failed to initialize duckdb engine: %w", err)
	}
	return &Engine{db: db}, nil
}

// Close closes the DuckDB connection.
func (e *Engine) Close() error {
	if e.db != nil {
		return e.db.Close()
	}
	return nil
}

func (e *Engine) newAppender(ctx context.Context, schema, table string) (*duckdbDriver.Appender, *sql.Conn, error) {
	conn, err := e.db.Conn(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get connection for appender: %w", err)
	}
	var appender *duckdbDriver.Appender
	err = conn.Raw(func(driverConn any) error {
		dConn, ok := driverConn.(driver.Conn)
		if !ok {
			return fmt.Errorf("underlying connection is not driver.Conn: %T", driverConn)
		}
		var aErr error
		appender, aErr = duckdbDriver.NewAppenderFromConn(dConn, schema, table)
		return aErr
	})
	if err != nil {
		conn.Close()
		return nil, nil, fmt.Errorf("failed to create duckdb appender: %w", err)
	}
	return appender, conn, nil
}

func ensureDir(path string) error {
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		return os.MkdirAll(dir, 0755)
	}
	return nil
}

func cleanPath(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	return filepath.ToSlash(filepath.Clean(path))
}


// WriteFinOpsParquet ingests CostRecord slice and writes directly to Parquet with primary_tag and secondary_tag columns.
func (e *Engine) WriteFinOpsParquet(ctx context.Context, records []cloud.CostRecord, outputPath string) error {
	if err := ensureDir(outputPath); err != nil {
		return err
	}

	// 1. Build CREATE TABLE DDL with exactly 17 normalized FinOps columns
	colDefs := []string{
		"account_id VARCHAR",
		"account_name VARCHAR",
		"service VARCHAR",
		"usage_date VARCHAR",
		"usage_period VARCHAR",
		"amount DOUBLE",
		"daily_amount DOUBLE",
		"days_in_period DOUBLE",
		"metric VARCHAR",
		"frequency VARCHAR",
		"unit VARCHAR",
		"currency VARCHAR",
		"region VARCHAR",
		"usage_type VARCHAR",
		"primary_tag VARCHAR",
		"secondary_tag VARCHAR",
		"recorded_at DATETIME",
	}

	_, _ = e.db.ExecContext(ctx, `DROP TABLE IF EXISTS raw_finops;`)
	createTable := fmt.Sprintf("CREATE TABLE raw_finops (\n\t%s\n);", strings.Join(colDefs, ",\n\t"))
	if _, err := e.db.ExecContext(ctx, createTable); err != nil {
		return fmt.Errorf("failed to create finops table: %w", err)
	}

	if len(records) > 0 {
		slog.DebugContext(ctx, "Bulk appending FinOps records via DuckDB Appender", "recordCount", len(records))
		appender, conn, err := e.newAppender(ctx, "", "raw_finops")
		if err != nil {
			return err
		}
		defer conn.Close()
		defer appender.Close()

		now := time.Now()
		for idx, r := range records {
			ts := r.Timestamp
			if ts.IsZero() {
				ts = now
			}
			dailyAmount := r.DailyAmount
			if dailyAmount == 0 && r.Amount != 0 {
				dailyAmount = r.Amount
			}
			daysInPeriod := r.DaysInPeriod
			if daysInPeriod == 0 {
				daysInPeriod = 1.0
			}
			metric := r.Metric
			if metric == "" {
				metric = "UnblendedCost"
			}
			frequency := r.Frequency
			if frequency == "" {
				frequency = "DAILY"
			}
			usagePeriod := r.UsagePeriod
			if usagePeriod == "" {
				usagePeriod = r.UsageDate
			}

			// Resolve primary_tag and secondary_tag
			primaryTag := r.PrimaryTag
			if primaryTag == "" {
				if r.Project != "" {
					primaryTag = r.Project
				} else if r.BusinessUnit != "" {
					primaryTag = r.BusinessUnit
				} else {
					primaryTag = "Untagged"
				}
			}

			secondaryTag := r.SecondaryTag
			if secondaryTag == "" {
				if r.Application != "" {
					secondaryTag = r.Application
				} else if r.Environment != "" {
					secondaryTag = r.Environment
				} else {
					secondaryTag = "Untagged"
				}
			}

			rowVals := []driver.Value{
				r.AccountID,
				r.AccountName,
				r.Service,
				r.UsageDate,
				usagePeriod,
				r.Amount,
				dailyAmount,
				daysInPeriod,
				metric,
				frequency,
				r.Unit,
				r.Currency,
				r.Region,
				r.UsageType,
				primaryTag,
				secondaryTag,
				ts,
			}

			if err := appender.AppendRow(rowVals...); err != nil {
				return fmt.Errorf("failed appending finops row: %w", err)
			}

			if (idx+1)%50000 == 0 {
				slog.InfoContext(ctx, "DuckDB bulk append progress", "appended", idx+1, "total", len(records))
			}
		}

		if err := appender.Flush(); err != nil {
			return fmt.Errorf("failed flushing finops appender: %w", err)
		}
		slog.DebugContext(ctx, "DuckDB appender flushed successfully", "recordCount", len(records))
	}

	copySQL := fmt.Sprintf("COPY raw_finops TO '%s' (FORMAT PARQUET);", cleanPath(outputPath))
	slog.DebugContext(ctx, "Exporting DuckDB raw_finops table to Parquet", "outputPath", cleanPath(outputPath))
	if _, err := e.db.ExecContext(ctx, copySQL); err != nil {
		return fmt.Errorf("failed to export finops parquet: %w", err)
	}
	slog.InfoContext(ctx, "Parquet export completed successfully", "outputPath", cleanPath(outputPath), "recordCount", len(records))
	return nil
}

// WriteSecOpsParquet ingests SecurityRecord slice and writes directly to Parquet.
func (e *Engine) WriteSecOpsParquet(ctx context.Context, records []cloud.SecurityRecord, outputPath string) error {
	if err := ensureDir(outputPath); err != nil {
		return err
	}

	_, _ = e.db.ExecContext(ctx, `DROP TABLE IF EXISTS raw_secops;`)
	createTable := `
	CREATE TABLE raw_secops (
		account_id VARCHAR,
		account_name VARCHAR,
		service VARCHAR,
		resource_id VARCHAR,
		finding_type VARCHAR,
		severity VARCHAR,
		description VARCHAR,
		remediation VARCHAR,
		recorded_at DATETIME
	);
	`
	if _, err := e.db.ExecContext(ctx, createTable); err != nil {
		return fmt.Errorf("failed to create secops table: %w", err)
	}

	if len(records) > 0 {
		appender, conn, err := e.newAppender(ctx, "", "raw_secops")
		if err != nil {
			return err
		}
		defer conn.Close()
		defer appender.Close()

		now := time.Now()
		for _, r := range records {
			ts := r.Timestamp
			if ts.IsZero() {
				ts = now
			}
			if err := appender.AppendRow(r.AccountID, r.AccountName, r.Service, r.ResourceID, r.FindingType, r.Severity, r.Description, r.Remediation, ts); err != nil {
				return fmt.Errorf("failed appending secops row: %w", err)
			}
		}
		if err := appender.Flush(); err != nil {
			return fmt.Errorf("failed flushing secops appender: %w", err)
		}
	}

	copySQL := fmt.Sprintf("COPY raw_secops TO '%s' (FORMAT PARQUET);", cleanPath(outputPath))
	if _, err := e.db.ExecContext(ctx, copySQL); err != nil {
		return fmt.Errorf("failed to export secops parquet: %w", err)
	}
	return nil
}

// WriteCloudOpsParquet ingests CloudOpsRecord slice and writes directly to Parquet.
func (e *Engine) WriteCloudOpsParquet(ctx context.Context, records []cloud.CloudOpsRecord, outputPath string) error {
	if err := ensureDir(outputPath); err != nil {
		return err
	}

	_, _ = e.db.ExecContext(ctx, `DROP TABLE IF EXISTS raw_cloudops;`)
	createTable := `
	CREATE TABLE raw_cloudops (
		account_id VARCHAR,
		account_name VARCHAR,
		service VARCHAR,
		resource_id VARCHAR,
		issue_type VARCHAR,
		resource_age_days INTEGER,
		estimated_waste_usd DOUBLE,
		recommendation VARCHAR,
		recorded_at DATETIME
	);
	`
	if _, err := e.db.ExecContext(ctx, createTable); err != nil {
		return fmt.Errorf("failed to create cloudops table: %w", err)
	}

	if len(records) > 0 {
		appender, conn, err := e.newAppender(ctx, "", "raw_cloudops")
		if err != nil {
			return err
		}
		defer conn.Close()
		defer appender.Close()

		now := time.Now()
		for _, r := range records {
			ts := r.Timestamp
			if ts.IsZero() {
				ts = now
			}
			if err := appender.AppendRow(r.AccountID, r.AccountName, r.Service, r.ResourceID, r.IssueType, r.ResourceAge, r.EstimatedWaste, r.Recommendation, ts); err != nil {
				return fmt.Errorf("failed appending cloudops row: %w", err)
			}
		}
		if err := appender.Flush(); err != nil {
			return fmt.Errorf("failed flushing cloudops appender: %w", err)
		}
	}

	copySQL := fmt.Sprintf("COPY raw_cloudops TO '%s' (FORMAT PARQUET);", cleanPath(outputPath))
	if _, err := e.db.ExecContext(ctx, copySQL); err != nil {
		return fmt.Errorf("failed to export cloudops parquet: %w", err)
	}
	return nil
}

// WriteAnomaliesParquet writes AnomalyRecord slice directly to Parquet.
func (e *Engine) WriteAnomaliesParquet(ctx context.Context, records []cloud.AnomalyRecord, outputPath string) error {
	if err := ensureDir(outputPath); err != nil {
		return err
	}

	_, _ = e.db.ExecContext(ctx, `DROP TABLE IF EXISTS raw_anomalies;`)
	createTable := `
	CREATE TABLE raw_anomalies (
		ops_domain VARCHAR,
		account_id VARCHAR,
		account_name VARCHAR,
		service VARCHAR,
		resource_or_key VARCHAR,
		metric VARCHAR,
		actual_value DOUBLE,
		expected_value DOUBLE,
		deviation_ratio DOUBLE,
		severity VARCHAR,
		status VARCHAR,
		timestamp TIMESTAMP
	);
	`
	if _, err := e.db.ExecContext(ctx, createTable); err != nil {
		return fmt.Errorf("failed to create anomalies table: %w", err)
	}

	if len(records) > 0 {
		appender, conn, err := e.newAppender(ctx, "", "raw_anomalies")
		if err != nil {
			return err
		}
		defer conn.Close()
		defer appender.Close()

		now := time.Now()
		for _, r := range records {
			ts := r.Timestamp
			if ts.IsZero() {
				ts = now
			}
			if err := appender.AppendRow(r.OpsDomain, r.AccountID, r.AccountName, r.Service, r.ResourceOrKey, r.Metric, r.ActualValue, r.ExpectedValue, r.Deviation, r.Severity, r.Status, ts); err != nil {
				return fmt.Errorf("failed appending anomalies row: %w", err)
			}
		}
		if err := appender.Flush(); err != nil {
			return fmt.Errorf("failed flushing anomalies appender: %w", err)
		}
	}

	copySQL := fmt.Sprintf("COPY raw_anomalies TO '%s' (FORMAT PARQUET);", cleanPath(outputPath))
	if _, err := e.db.ExecContext(ctx, copySQL); err != nil {
		return fmt.Errorf("failed to export anomalies parquet: %w", err)
	}
	return nil
}

// AnalyzeFinOps runs statistical anomaly queries over raw FinOps parquet and saves anomalies to parquet.
func (e *Engine) AnalyzeFinOps(ctx context.Context, inputParquet, outputParquet string, thresholdMultiplier float64) ([]cloud.AnomalyRecord, error) {
	if err := ensureDir(outputParquet); err != nil {
		return nil, err
	}

	if thresholdMultiplier <= 0 {
		thresholdMultiplier = 1.3 // 30% increase threshold
	}

	query := fmt.Sprintf(`
	WITH service_stats AS (
		SELECT 
			account_id,
			account_name,
			service,
			AVG(amount) AS avg_cost,
			MAX(amount) AS max_cost
		FROM read_parquet('%s')
		GROUP BY account_id, account_name, service
	)
	SELECT 
		'finops' AS ops_domain,
		account_id,
		account_name,
		service,
		service AS resource_or_key,
		'CostSpike' AS metric,
		max_cost AS actual_value,
		avg_cost AS expected_value,
		CASE WHEN avg_cost > 0 THEN (max_cost / avg_cost) ELSE 1.0 END AS deviation_ratio,
		CASE 
			WHEN max_cost > 500 AND (max_cost / NULLIF(avg_cost, 0)) >= 2.0 THEN 'CRITICAL'
			WHEN (max_cost / NULLIF(avg_cost, 0)) >= 1.5 THEN 'HIGH'
			ELSE 'MEDIUM'
		END AS severity,
		'OPEN' AS status,
		CURRENT_TIMESTAMP AS timestamp
	FROM service_stats
	WHERE max_cost >= (avg_cost * %f) OR max_cost > 100.0
	`, cleanPath(inputParquet), thresholdMultiplier)

	exportSQL := fmt.Sprintf("COPY (%s) TO '%s' (FORMAT PARQUET);", query, cleanPath(outputParquet))
	if _, err := e.db.ExecContext(ctx, exportSQL); err != nil {
		return nil, fmt.Errorf("failed to execute finops anomaly analysis: %w", err)
	}

	return e.readAnomaliesFromParquet(ctx, outputParquet)
}

// AnalyzeSecOps runs security risk analysis over raw SecOps parquet.
func (e *Engine) AnalyzeSecOps(ctx context.Context, inputParquet, outputParquet string) ([]cloud.AnomalyRecord, error) {
	if err := ensureDir(outputParquet); err != nil {
		return nil, err
	}

	query := fmt.Sprintf(`
	SELECT 
		'secops' AS ops_domain,
		account_id,
		account_name,
		service,
		resource_id AS resource_or_key,
		finding_type AS metric,
		CAST(1.0 AS DOUBLE) AS actual_value,
		CAST(0.0 AS DOUBLE) AS expected_value,
		CAST(1.0 AS DOUBLE) AS deviation_ratio,
		severity,
		'OPEN' AS status,
		recorded_at AS timestamp
	FROM read_parquet('%s')
	WHERE severity IN ('CRITICAL', 'HIGH')
	`, cleanPath(inputParquet))

	exportSQL := fmt.Sprintf("COPY (%s) TO '%s' (FORMAT PARQUET);", query, cleanPath(outputParquet))
	if _, err := e.db.ExecContext(ctx, exportSQL); err != nil {
		return nil, fmt.Errorf("failed to execute secops analysis: %w", err)
	}

	return e.readAnomaliesFromParquet(ctx, outputParquet)
}

// AnalyzeCloudOps runs operational waste analysis over raw CloudOps parquet.
func (e *Engine) AnalyzeCloudOps(ctx context.Context, inputParquet, outputParquet string) ([]cloud.AnomalyRecord, error) {
	if err := ensureDir(outputParquet); err != nil {
		return nil, err
	}

	query := fmt.Sprintf(`
	SELECT 
		'cloudops' AS ops_domain,
		account_id,
		account_name,
		service,
		resource_id AS resource_or_key,
		issue_type AS metric,
		CAST(estimated_waste_usd AS DOUBLE) AS actual_value,
		CAST(0.0 AS DOUBLE) AS expected_value,
		CAST(resource_age_days AS DOUBLE) AS deviation_ratio,
		CASE 
			WHEN estimated_waste_usd > 100 OR resource_age_days > 60 THEN 'HIGH'
			WHEN estimated_waste_usd > 30 OR resource_age_days > 30 THEN 'MEDIUM'
			ELSE 'LOW'
		END AS severity,
		'OPEN' AS status,
		recorded_at AS timestamp
	FROM read_parquet('%s')
	WHERE estimated_waste_usd > 5.0 OR resource_age_days > 14
	`, cleanPath(inputParquet))

	exportSQL := fmt.Sprintf("COPY (%s) TO '%s' (FORMAT PARQUET);", query, cleanPath(outputParquet))
	if _, err := e.db.ExecContext(ctx, exportSQL); err != nil {
		return nil, fmt.Errorf("failed to execute cloudops analysis: %w", err)
	}

	return e.readAnomaliesFromParquet(ctx, outputParquet)
}

func (e *Engine) readAnomaliesFromParquet(ctx context.Context, parquetPath string) ([]cloud.AnomalyRecord, error) {
	readSQL := fmt.Sprintf("SELECT ops_domain, account_id, account_name, service, resource_or_key, metric, CAST(actual_value AS DOUBLE), CAST(expected_value AS DOUBLE), CAST(deviation_ratio AS DOUBLE), severity, status, timestamp FROM read_parquet('%s');", cleanPath(parquetPath))
	rows, err := e.db.QueryContext(ctx, readSQL)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []cloud.AnomalyRecord
	for rows.Next() {
		var a cloud.AnomalyRecord
		if err := rows.Scan(&a.OpsDomain, &a.AccountID, &a.AccountName, &a.Service, &a.ResourceOrKey, &a.Metric, &a.ActualValue, &a.ExpectedValue, &a.Deviation, &a.Severity, &a.Status, &a.Timestamp); err != nil {
			return nil, err
		}
		records = append(records, a)
	}
	return records, nil
}

// EnrichRecords enriches anomaly records with Landing Zone organizational context and writes enriched.parquet.
func (e *Engine) EnrichRecords(ctx context.Context, inputParquet, outputParquet string) ([]cloud.EnrichedRecord, error) {
	if err := ensureDir(outputParquet) ; err != nil {
		return nil, err
	}

	query := fmt.Sprintf(`
	SELECT 
		ops_domain,
		account_id,
		account_name,
		CASE 
			WHEN account_id LIKE '%%1' OR account_name LIKE '%%prod%%' THEN 'Production'
			WHEN account_id LIKE '%%2' OR account_name LIKE '%%stg%%' THEN 'Staging'
			ELSE 'Development'
		END AS environment,
		CASE 
			WHEN service IN ('AmazonEC2', 'Compute') THEN 'Core Platform'
			WHEN service IN ('AmazonS3', 'Storage') THEN 'Data Engineering'
			WHEN service IN ('AWSIdentityAccessManagement', 'Security') THEN 'Security Operations'
			ELSE 'Digital Products'
		END AS business_unit,
		CASE 
			WHEN ops_domain = 'secops' THEN 'secops-lead@company.com'
			WHEN ops_domain = 'finops' THEN 'finops-team@company.com'
			ELSE 'devops-infra@company.com'
		END AS owner,
		'CC-' || SUBSTR(account_id, 1, 4) AS cost_center,
		service,
		resource_or_key,
		metric,
		CAST(actual_value AS DOUBLE) AS actual_value,
		CAST(expected_value AS DOUBLE) AS expected_value,
		CASE 
			WHEN ops_domain = 'finops' THEN CAST((actual_value - expected_value) AS DOUBLE)
			WHEN ops_domain = 'cloudops' THEN CAST(actual_value AS DOUBLE)
			ELSE CAST(0.0 AS DOUBLE)
		END AS potential_savings,
		severity,
		CASE 
			WHEN ops_domain = 'finops' THEN 'Rightsize underutilized compute or commit to Savings Plans'
			WHEN ops_domain = 'secops' THEN 'Apply restrictive bucket policy or rotate stale credentials immediately'
			ELSE 'Snapshot and terminate orphaned resource'
		END AS action_recommended,
		timestamp
	FROM read_parquet('%s')
	`, cleanPath(inputParquet))

	exportSQL := fmt.Sprintf("COPY (%s) TO '%s' (FORMAT PARQUET);", query, cleanPath(outputParquet))
	if _, err := e.db.ExecContext(ctx, exportSQL); err != nil {
		return nil, fmt.Errorf("failed to enrich parquet: %w", err)
	}

	return e.ReadEnrichedFromParquet(ctx, outputParquet)
}

// WriteEnrichedParquet ingests EnrichedRecord slice and writes directly to Parquet.
func (e *Engine) WriteEnrichedParquet(ctx context.Context, records []cloud.EnrichedRecord, outputPath string) error {
	if err := ensureDir(outputPath); err != nil {
		return err
	}

	_, _ = e.db.ExecContext(ctx, `DROP TABLE IF EXISTS raw_enriched;`)
	createTable := `
	CREATE TABLE raw_enriched (
		ops_domain VARCHAR,
		account_id VARCHAR,
		account_name VARCHAR,
		environment VARCHAR,
		business_unit VARCHAR,
		owner VARCHAR,
		cost_center VARCHAR,
		service VARCHAR,
		resource_or_key VARCHAR,
		metric VARCHAR,
		actual_value DOUBLE,
		expected_value DOUBLE,
		potential_savings DOUBLE,
		severity VARCHAR,
		action_recommended VARCHAR,
		timestamp TIMESTAMP
	);
	`
	if _, err := e.db.ExecContext(ctx, createTable); err != nil {
		return fmt.Errorf("failed to create enriched table: %w", err)
	}

	tx, err := e.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO raw_enriched VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
	`)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()

	now := time.Now()
	for _, r := range records {
		ts := r.Timestamp
		if ts.IsZero() {
			ts = now
		}
		if _, err := stmt.ExecContext(ctx,
			r.OpsDomain, r.AccountID, r.AccountName, r.Environment, r.BusinessUnit,
			r.Owner, r.CostCenter, r.Service, r.ResourceOrKey, r.Metric,
			r.ActualValue, r.ExpectedValue, r.PotentialSavings, r.Severity,
			r.ActionRecommended, ts,
		); err != nil {
			tx.Rollback()
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}

	copySQL := fmt.Sprintf("COPY raw_enriched TO '%s' (FORMAT PARQUET);", cleanPath(outputPath))
	if _, err := e.db.ExecContext(ctx, copySQL); err != nil {
		return fmt.Errorf("failed to export enriched parquet: %w", err)
	}
	return nil
}

// ReadEnrichedFromParquet reads enriched records from Parquet.
func (e *Engine) ReadEnrichedFromParquet(ctx context.Context, parquetPath string) ([]cloud.EnrichedRecord, error) {
	readSQL := fmt.Sprintf(`
	SELECT 
		ops_domain, account_id, account_name, environment, business_unit, owner, 
		cost_center, service, resource_or_key, metric, 
		CAST(actual_value AS DOUBLE), CAST(expected_value AS DOUBLE), 
		CAST(potential_savings AS DOUBLE), severity, action_recommended, timestamp 
	FROM read_parquet('%s');`, cleanPath(parquetPath))

	rows, err := e.db.QueryContext(ctx, readSQL)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []cloud.EnrichedRecord
	for rows.Next() {
		var r cloud.EnrichedRecord
		if err := rows.Scan(
			&r.OpsDomain, &r.AccountID, &r.AccountName, &r.Environment, &r.BusinessUnit, &r.Owner,
			&r.CostCenter, &r.Service, &r.ResourceOrKey, &r.Metric, &r.ActualValue, &r.ExpectedValue,
			&r.PotentialSavings, &r.Severity, &r.ActionRecommended, &r.Timestamp,
		); err != nil {
			return nil, err
		}
		records = append(records, r)
	}
	return records, nil
}

// WriteEnforceAuditParquet writes enforcement audit logs to Parquet.
func (e *Engine) WriteEnforceAuditParquet(ctx context.Context, records []cloud.EnforceAuditRecord, outputPath string) error {
	if err := ensureDir(outputPath); err != nil {
		return err
	}

	_, _ = e.db.ExecContext(ctx, `DROP TABLE IF EXISTS raw_audit;`)
	createTable := `
	CREATE TABLE raw_audit (
		timestamp TIMESTAMP,
		ops_domain VARCHAR,
		account_id VARCHAR,
		service VARCHAR,
		resource_id VARCHAR,
		action_taken VARCHAR,
		dry_run BOOLEAN,
		success BOOLEAN,
		details VARCHAR
	);
	`
	if _, err := e.db.ExecContext(ctx, createTable); err != nil {
		return err
	}

	tx, err := e.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO raw_audit VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);`)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()

	for _, r := range records {
		if _, err := stmt.ExecContext(ctx, r.Timestamp, r.OpsDomain, r.AccountID, r.Service, r.ResourceID, r.ActionTaken, r.DryRun, r.Success, r.Details); err != nil {
			tx.Rollback()
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}

	copySQL := fmt.Sprintf("COPY raw_audit TO '%s' (FORMAT PARQUET);", cleanPath(outputPath))
	if _, err := e.db.ExecContext(ctx, copySQL); err != nil {
		return fmt.Errorf("failed to export audit parquet: %w", err)
	}
	return nil
}

// ExportToCSV converts any Parquet file to standard CSV.
func (e *Engine) ExportToCSV(ctx context.Context, parquetPath, csvPath string) error {
	if err := ensureDir(csvPath); err != nil {
		return err
	}
	copySQL := fmt.Sprintf("COPY (SELECT * FROM read_parquet('%s')) TO '%s' (HEADER, DELIMITER ',');", cleanPath(parquetPath), cleanPath(csvPath))
	_, err := e.db.ExecContext(ctx, copySQL)
	return err
}

// ExportToJSON exports enriched records to JSON bytes.
func (e *Engine) ExportToJSON(ctx context.Context, parquetPath string) ([]byte, error) {
	records, err := e.ReadEnrichedFromParquet(ctx, parquetPath)
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(records, "", "  ")
}
