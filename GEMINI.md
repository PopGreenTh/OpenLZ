# GEMINI.md - OpenLZ Project Guidelines & Context

This document provides context, architectural rules, and engineering standards for AI assistants (Antigravity / Gemini) and contributors working on the **OpenLZ** codebase.

---

## 1. Project Overview

**OpenLZ (Open Landing Zone)** is a high-performance, Zero-ETL Cloud Operations platform for enterprise AWS Landing Zones. It covers three operational domains:
- **FinOps**: Cost Explorer spend ingestion, cost anomaly detection, usage quantity metrics, tagging enforcement, and comparable daily rate calculation.
- **SecOps**: Security posture audits, public S3 buckets, open security groups, stale IAM keys, and CVE vulnerability ingestion.
- **CloudOps**: Operational hygiene, orphaned EBS volumes, unattached Elastic IPs, and stale snapshots.

---

## 2. Core Architectural Invariants

### 1. Zero-ETL with Embedded DuckDB
- **Engine**: All data transformations, OLAP aggregation, and parquet creation are powered by embedded [DuckDB](https://duckdb.org) (`github.com/marcboeker/go-duckdb`).
- **Do not introduce external database servers** (e.g. Postgres, MySQL, Redis).
- **API Guardrail Cache**: Transactional queries must use the local DuckDB cache (`openlz_cache.duckdb`) to avoid repeated AWS API query fees ($0.01 per Cost Explorer call).

### 2. Strict AWS Client Layering (`internal/aws/`)
To prevent the "God Client" anti-pattern:
- **`client.go`**: Contains **only** core AWS SDK configuration, client instantiation, credentials, region defaults, and retry wrappers (`ExecuteWithRetry`).
- **`costexplorer.go`**: Contains FinOps queries (`FetchCostsWithOptions`, `FetchCostAnomalies`), tag assignment, and daily metric calculations.
- **`secops.go`**: Contains SecOps findings retrieval.
- **`mocks.go`**: Contains **all** offline mock data generation functions.
- **RULE**: **Never** add stage-specific business logic, domain transformations, or mock data arrays directly into `client.go`.

### 3. Pipeline Lifecycle (The 5 Stages)
Every domain follows the 5-stage sequential lifecycle:
$$\textbf{Sense} \longrightarrow \textbf{Analyze} \longrightarrow \textbf{Enrich} \longrightarrow \textbf{Enforce} \longrightarrow \textbf{Report}$$
- **Sense**: Concurrently queries cloud APIs (or mock generators), writing raw records to `data/*_raw.parquet`.
- **Analyze**: Runs DuckDB OLAP SQL queries over parquet to identify anomalies and spikes.
- **Enrich**: Joins anomalies with organizational Landing Zone metadata (Account, BusinessUnit, Project, Environment).
- **Enforce**: Assesses compliance guardrails (default `--dry-run`).
- **Report**: Emits formatted outputs (Console tables, Markdown, Power BI/Excel Power Query `.m`, or HTTP REST streaming).

### 4. Normalized Daily Metrics (`DailyAmount`)
When dealing with Cost Explorer data across different frequencies (`HOURLY`, `DAILY`, `MONTHLY`):
- `HOURLY`: `DailyAmount = Amount * 24.0` (24-hour run-rate baseline, `DaysInPeriod = 1.0 / 24.0`).
- `DAILY`: `DailyAmount = Amount` (`DaysInPeriod = 1.0`).
- `MONTHLY`: `DailyAmount = Amount / DaysInPeriod` (`DaysInPeriod` is the number of days in the month, or elapsed days for partial active months).
- Both raw `Amount` and comparable `DailyAmount` must be persisted in DuckDB Parquet.

### 5. Cost Taxonomy & Multi-Tenancy
> **Authoritative spec: `docs/taxonomy-spec.md`.** The summary below describes the current code; the
> spec adds OwnerTeam, ProductLine, LZ/Connection and Code naming, and **OpenLZ is now single-tenant**:
> the Tenant level and all `--tenant` support are being removed (handoff Task 0). Don't add new tenant code.

- **Hierarchies** (single source of truth: `internal/taxonomy/schema.go` → `Schema`):
  - Biz: `BusinessUnit → CostCenter → Owner → CustomerFunc`
  - Project: `Project → App → Environment/Account → CSP/Region → ServiceCat/Service → ServiceUsage/Usage → ResourceGroup/Resource`
- **Relations**: Project *must ref* BusinessUnit; App *must ref* CostCenter; ResourceGroup *can ref* CustomerFunc; Region (unique within its CSP), Account, Service, Usage and Resource *can be mapped* to their group level; Resource *can be mapped* to Environment.
- **App ↔ Environment is many-to-many** (junction collection `app_environments`): an App can run in several Environments and an Environment can host several Apps.
- **Matching is two-way** (`Catalog.Resolve`): bottom-up from the row's resource/account, and top-down plus candidate intersection from known levels. See the handoff doc §2 for the exact rules.
- The old "must behave" identity rules (Project = BU, App = Owner, Env = CostCenter) are **removed**. Do not reintroduce them.
- One flat JSON snapshot per tenant: `taxonomy/taxonomy.json` (tenant `default`) and `taxonomy/taxonomy_<tenant>.json`. Data files keep the suffix pattern `data/finops_raw_<tenant>.parquet`.
- **Directus is the editing source of truth** (one instance, a `tenant` field on every row); `openlz taxonomy pull` writes the JSON that Enrich reads offline.

> **Work in progress:** see `docs/handoff/finops-taxonomy-v2.md` for what is done, the remaining tasks (Directus bootstrap/pull/push, reports, docs) and the product decisions. Read it before you change taxonomy, enrich or Directus code.

---

## 3. Directory Layout & Key Packages

```
OpenLZ/
├── cmd/
│   ├── root.go             # Root Cobra CLI command
│   ├── pipeline.go         # Full 5-stage pipeline runner (openlz pipeline)
│   ├── finops.go           # openlz finops subcommands
│   ├── secops.go           # openlz secops subcommands
│   ├── cloudops.go         # openlz cloudops subcommands
│   ├── stages.go           # Granular stage-level commands (finops-sense-costexplorer, etc.)
│   ├── report.go           # Multi-format report generators & HTTP REST server
│   ├── taxonomy.go         # openlz taxonomy subcommands (list, validate, init)
│   ├── openlz-finops/      # Standalone FinOps binary entrypoint (main.go)
│   ├── openlz-secops/      # Standalone SecOps binary entrypoint (main.go)
│   └── openlz-cloudops/    # Standalone CloudOps binary entrypoint (main.go)
├── directus/               # Directus Headless CMS (Docker Compose, PostgreSQL backend)
│   ├── docker-compose.yml  # Directus container specification
│   └── README.md           # Visual taxonomy editing instructions
├── taxonomy/               # Cost taxonomy snapshots (pulled from Directus)
│   ├── taxonomy.json       # default tenant
│   └── taxonomy_<tenant>.json
├── internal/
│   ├── aws/
│   │   ├── client.go       # Lean AWS SDK client & retries
│   │   ├── costexplorer.go # Cost Explorer & Anomaly queries, tag parsing, daily metrics
│   │   ├── secops.go       # SecOps queries
│   │   └── mocks.go        # Offline mock data generators
│   ├── cache/              # Persistent DuckDB API fee cache ($0.01 guardrail)
│   ├── cloud/              # Core domain models (CostRecord, SecurityRecord, CloudOpsRecord)
│   ├── duckdb/             # In-memory OLAP engine & Parquet persistence
│   ├── powerquery/         # Power BI & Excel Power Query (.m) formula generators
│   ├── report/             # Report generators (Markdown, console table, CSV)
│   ├── rest/               # Embedded HTTP REST server for Power BI / Excel web feeds
│   ├── stages/             # Stage execution handlers
│   │   ├── finops/         # sense_costexplorer.go, sense_costanomaly.go, analyze, etc.
│   │   ├── secops/         # sense_vulner.go, sense_posture.go, etc.
│   │   └── cloudops/       # sense_hygiene.go, etc.
│   ├── taxonomy/           # Taxonomy Schema, snapshot JSON, validation, Resolve, tenant paths
│   └── workflow/           # Multi-account concurrent scanner (errgroup)
├── ARCHITECTURE.md         # Full technical architecture specification
├── Taskfile.yml            # Task runner workflow definitions
├── go.mod                  # Go 1.21+ module dependencies
└── main.go                 # Main openlz unified CLI entrypoint
```

---

## 4. Go & DuckDB Engineering Practices

### 1. Parquet Schema Standards
- `raw_finops` (`data/finops_raw.parquet`) uses 17 normalized columns with `primary_tag` and `secondary_tag` as dedicated VARCHAR columns.
- Tag discovery maps primary (Level 1) and secondary (Level 2) tags directly to `primary_tag` and `secondary_tag`. Dynamic extra tag columns (`project`, `application`) and `tags JSON` are excluded from the raw parquet schema for high performance and clean downstream ingestion.

### 2. Error Handling & Retries
- Wrap all AWS SDK calls with `aws.ExecuteWithRetry(ctx, func() error { ... })`.
- Always wrap errors with `%w` for traceability: `fmt.Errorf("failed scanning account %s: %w", acc, err)`.

### 3. Concurrency
- Use `golang.org/x/sync/errgroup` with a bounded concurrency pool (typically 5 workers) when scanning multiple AWS accounts. Never spawn unbounded goroutines.

### 4. Backward Compatibility
- When adding flags or inputs to stages, ensure defaults preserve existing behavior (e.g. `Frequency` defaults to `"DAILY"`, `Metric` defaults to `"UnblendedCost"`).

### 5. Delve Debugging & Windows CGO Linker Note
- `github.com/marcboeker/go-duckdb` includes an optional Apache Arrow C Data Interface bridge (`arrow.go`) that defines `inline` functions in `helpers.h`.
- On Windows MinGW GCC (especially under unoptimized debug builds `-N -l` used by Delve `dlv`), GCC treats these as external references, triggering `undefined reference to ArrowSchemaMarkReleased`.
- **Solution**: Always compile debug sessions and builds with `-tags=no_duckdb_arrow`. This excludes the unneeded Arrow C bridge while keeping DuckDB in-memory OLAP, SQL engine, and Parquet read/write fully functional.
- This is pre-configured in `.vscode/launch.json`, `.vscode/settings.json`, and `Taskfile.yml`.

---

## 6. Development & Verification Commands

```powershell
# Run all tests (always use the no_duckdb_arrow tag)
go test -tags=no_duckdb_arrow ./...

# Run finops stage unit tests with verbose logging
go test -v ./internal/stages/finops/...

# Test standalone CLI in mock mode
go run ./cmd/openlz-finops sense-costexplorer --mock --metric UsageQuantity --frequency monthly

# Test unified CLI in mock mode
go run . finops-sense-costexplorer --mock --metric UnblendedCost --frequency daily

# Build all 4 binaries
task build
```
