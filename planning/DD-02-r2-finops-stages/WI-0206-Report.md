# WI-0206 — `finops/report`: table, CSV, Excel, appending

## Outcome

`openlz-finops report` turns any records into a table on screen, CSV files or an Excel workbook.
Writing to a file that already exists appends, so repeated runs build one growing table
(D-022, D-023). The whole pipeline runs on the mock from a clean clone.

## Contracts

`openlz-finops report table|csv|excel [--out PATH] [--kind KINDS]`

- **One table per `kind`.** Records are grouped by `kind`, in first-seen order; `--kind`
  limits which kinds are written. Columns are the fields in first-seen order, `kind` left out.
- **`table`** prints aligned columns to stdout, one titled block per kind; amounts with two
  decimals and their currency. With `--out FILE`, the same text is appended to the file.
- **`csv`**: `--out DIR` (required) writes `DIR/<kind>.csv`.
- **`excel`**: `--out FILE.xlsx` (required) writes one sheet per kind, named by the kind.
- **Appending.** A new file or sheet gets a header row, then the rows. An existing one gets
  only the rows, at the end. If the records have a field the existing header lacks, the report
  stops with an error naming the fields and writes nothing; a header field the records lack is
  written empty.
- Report writes nothing to stdout except in `table` mode, and is the end of a pipeline.
- Excel is written by a pure-Go library (no CGO beyond DuckDB's).

### `report accounts`

Prints the OU tree with each account under its OU, as today, but from `ou` and `account`
records from the pipe or `--from-accounts`. `sense accounts --out` and the inventory file are removed.

### Demo and schedule

README: a quickstart whose whole pipeline runs on the mock with no AWS credentials, the full
list of read-only IAM actions, and an example crontab line that runs the monthly pipeline with
`--save` (D-016).

## WO-0206-A — Report formats, appending, quickstart

**Assignee:** open
**Depends on:** WO-0204-A, WO-0205-A
**Packages:** `finops/report/`, `finops/sense/` (removing `--out` only), `README.md`, `go.mod`,
`go.sum`

**Goal.** The three formats with appending, `report accounts` on records, and the quickstart.

**In scope.**
- `report table|csv|excel|accounts` as specified. `table`, `csv` and `excel` have every
  `--from-<thing>` flag of WI-0202, and may combine several; `accounts` has `--from-accounts`.
- Golden-file tests on a fixed record file for each format; appending twice gives the header
  once and the rows twice; a new field stops the append; one sheet or file per kind.

**Out of scope.** Markdown, Power BI, REST. Any summing or grouping (that is Analyze).

**Done when.**
- The tests above pass.
- On a clean clone with no AWS credentials, the README quickstart pipeline (Sense › Enrich ›
  Analyze › Enforce › Report on `--mock`) prints a report and exits 0 (shown in the PR).
- Running it twice with `report excel --out x.xlsx` leaves one workbook whose sheets hold both
  runs' rows under one header.
- `sense accounts --out` no longer exists, and `report accounts` prints the demo OU tree.

**Verify.** `go run -tags=no_duckdb_arrow ./scripts/check` and the README quickstart pipeline
