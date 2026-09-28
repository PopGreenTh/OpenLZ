# OpenLZ (Open Landing Zone)

[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)](go.mod)
[![Data Engine](https://img.shields.io/badge/DuckDB-Embedded_OLAP-FFF000?logo=duckdb)](https://duckdb.org)

**OpenLZ** is a modular, high-performance Zero-ETL Cloud Operations CLI tool for AWS Landing Zones, covering **FinOps**, **SecOps**, and **CloudOps**.

It queries native cloud APIs concurrently, caches queries in an embedded DuckDB persistent database to guard against repeat API charges ($0.01/call guardrail), processes and transforms data locally using embedded DuckDB into Parquet, and provides first-class connectivity to **Power BI** and **Microsoft Excel (Power Query)**.

---

## Key Features

1. **Pure DuckDB Zero-ETL Architecture**:
   - Single embedded database engine for both transactional API guardrail caching (`openlz_cache.duckdb`) and in-memory OLAP Parquet transformations.
   - Zero external database services or heavyweight ETL clusters required.

2. **5 Standalone, Skippable Stages**:
   $$\textbf{Sense} \longrightarrow \textbf{Analyze} \longrightarrow \textbf{Enrich} \longrightarrow \textbf{Enforce} \longrightarrow \textbf{Report}$$
   - **Sense**: Concurrently queries cloud APIs using `errgroup` and `retry-go`, enforcing the DuckDB $0.01 API fee cache.
   - **Analyze**: Runs statistical anomaly queries directly over Parquet in DuckDB.
   - **Enrich**: Joins findings with Landing Zone organizational context (Account, Environment, Business Unit, Cost Center, Owner).
   - **Enforce**: Executes guardrail policies (dry-run simulation by default, or active cloud remediation).
   - **Report**: Outputs summaries across terminal, GitHub Markdown, Power BI, Excel, and live HTTP REST feeds.

3. **Separation by Ops Domain**:
   - **FinOps**: Cost Explorer spend anomalies, compute rightsizing, S3 tiering.
   - **SecOps**: Public S3 buckets, open security groups (0.0.0.0/0), stale IAM credentials.
   - **CloudOps**: Orphaned EBS volumes, stale snapshots, unattached Elastic IPs.

4. **Universal 4-Way Reporting & Power BI / Excel Integration**:
   - **Text / Markdown**: Formatted console tables and GitHub-flavored markdown.
   - **Power BI**: Native Parquet dataset (`report.parquet`) and auto-generated `.m` Power Query formula ready to paste into Power BI Desktop.
   - **Excel**: Excel-optimized CSV dataset (`report.csv`) and Power Query M formula.
   - **Live HTTP REST Feed (`openlz report serve`)**: Micro-server streaming `/api/v1/{ops}` in JSON and CSV for automated, scheduled "Get Data $\rightarrow$ From Web" refreshes in Power BI and Excel Online.

---

## Quick Start

### 1. Build
```bash
go build -o openlz main.go
```
Or on Windows:
```pwsh
go build -o openlz.exe main.go
```

### 2. Run the Full Pipeline
Run all 5 stages across FinOps, SecOps, and CloudOps:
```bash
./openlz pipeline --mock
```

To skip specific stages or domains:
```bash
./openlz pipeline --skip-enforce --skip-cloudops
```

---

## CLI Usage Guide

### FinOps Commands
```bash
# Stage 1: Sense AWS Cost Explorer data to raw.parquet
./openlz finops sense --accounts 111122223333,444455556666 --output data/finops_raw.parquet

# Stage 2: Analyze cost spikes and anomalies
./openlz finops analyze --input data/finops_raw.parquet --output data/finops_anomalies.parquet --threshold 1.3

# Stage 3: Enrich anomalies with Landing Zone metadata
./openlz finops enrich --input data/finops_anomalies.parquet --output data/finops_enriched.parquet

# Stage 4: Enforce FinOps tagging guardrails (dry-run)
./openlz finops enforce --input data/finops_enriched.parquet --output data/finops_audit.parquet --dry-run

# Stage 5: Generate FinOps reports
./openlz finops report --input data/finops_enriched.parquet --format table
./openlz finops report --input data/finops_enriched.parquet --format markdown
./openlz finops report --input data/finops_enriched.parquet --format powerbi
./openlz finops report --input data/finops_enriched.parquet --format excel
```

### SecOps Commands
```bash
./openlz secops sense --output data/secops_raw.parquet
./openlz secops analyze --input data/secops_raw.parquet --output data/secops_anomalies.parquet
./openlz secops enrich --input data/secops_anomalies.parquet --output data/secops_enriched.parquet
./openlz secops enforce --input data/secops_enriched.parquet --output data/secops_audit.parquet --dry-run
./openlz secops report --input data/secops_enriched.parquet --format table
```

### CloudOps Commands
```bash
./openlz cloudops sense --output data/cloudops_raw.parquet
./openlz cloudops analyze --input data/cloudops_raw.parquet --output data/cloudops_anomalies.parquet
./openlz cloudops enrich --input data/cloudops_anomalies.parquet --output data/cloudops_enriched.parquet
./openlz cloudops enforce --input data/cloudops_enriched.parquet --output data/cloudops_audit.parquet --dry-run
./openlz cloudops report --input data/cloudops_enriched.parquet --format table
```

---

## Power BI & Excel (Power Query) Setup

### Option A: Direct Parquet / CSV Import
1. Run `openlz report powerbi` (or `openlz report excel`).
2. Open **Power BI Desktop** or **Microsoft Excel**.
3. Go to **Data $\rightarrow$ Get Data $\rightarrow$ Blank Query**.
4. Open the **Advanced Editor**.
5. Paste the contents of the generated `.m` file (e.g. `finops_report.m` or `powerbi_dataset.m`) and click **Done**.

### Option B: Live Web Refresh via HTTP REST
1. Start the embedded live feed:
   ```bash
   ./openlz report serve --port 8080
   ```
2. In Power BI or Excel:
   - Click **Get Data $\rightarrow$ From Web**.
   - URL: `http://localhost:8080/api/v1/finops` (or `http://localhost:8080/api/v1/secops`)
   - For CSV format: `http://localhost:8080/api/v1/finops?format=csv`
3. Click **Transform Data** or load directly. Scheduled or manual refresh will instantly fetch the latest cloud scan!

---

## Pipeline Orchestration with Taskfile

OpenLZ provides a complete `Taskfile.yml` for orchestration with [go-task](https://taskfile.dev):

```bash
# Build
task build

# Run dedicated stage tasks
task sense:finops
task analyze:finops
task enrich:finops
task enforce:finops
task report:finops

# Run SecOps or CloudOps
task sense:secops
task sense:cloudops

# Generate Power BI and Excel exports
task report:powerbi
task report:excel
task report:executive

# Start live HTTP feed
task report:serve

# Clean build artifacts and DuckDB cache
task clean
```

---

## License

Licensed under the [Apache License, Version 2.0](LICENSE).
