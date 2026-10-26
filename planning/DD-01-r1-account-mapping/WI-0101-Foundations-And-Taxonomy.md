# WI-0101 — Foundations and taxonomy

## Outcome

A fresh Go module with one `openlz` binary, a check script that CI and every implementer run, and
a taxonomy package that loads `taxonomy/*.yaml` into a typed model and reports every problem in it.

## Contracts

### Package areas (Release 1)

The names are indicative; the boundaries are not. One Work Order owns each area at a time.

| Area | Owner Work Order | Holds |
|---|---|---|
| `cmd/` | each Work Order adds its own subcommand file | Cobra commands only, no logic |
| `internal/taxonomy` | WO-0101-B | Model, YAML loader, validation |
| `internal/inventory` | WO-0102-A | Inventory model, AWS discovery, mock |
| `internal/mapping` | WO-0103-A | Account resolution |
| `internal/cost` | WO-0104-A | Cost Explorer, cache, allocation |
| `internal/report` | WO-0104-B | Rollup, coverage, output writers |
| `internal/awsx` | WO-0102-A | AWS config and the one retry wrapper |
| `internal/store` | WO-0104-A | DuckDB and Parquet helpers |

`internal/taxonomy` imports nothing else from `internal/`; `internal/inventory` imports only
`internal/awsx`.

### Codes

- A Code matches `^[A-Z][A-Z0-9-]{1,31}$`. Names are free text and may repeat.
- Codes ending in `-UNALLOC` or `-UNSPEC` are reserved for system nodes and rejected in YAML.
- Unique within their level: BusinessUnit, CostCenter, Project, App, LZ. Environment codes are
  unique within their App only.
- System nodes (generated, never in YAML): `LZ-UNALLOC`, `PRJ-UNALLOC`, `APP-UNALLOC`,
  `ENV-UNALLOC`, `ENV-UNSPEC`, `BU-UNALLOC`, `CC-UNALLOC`.

### Taxonomy files

Every `*.yaml` file in the taxonomy directory (default `taxonomy/`) is read, and their top-level
keys are merged into one document. A key that appears in two files is an error. Unknown keys are
errors. The merged document:

```yaml
business_units:
  - code: BU-RETAIL
    name: Retail

cost_centers:
  - code: CC-ECOM
    name: E-commerce
    business_unit: BU-RETAIL          # must ref

projects:
  - code: PRJ-SHOP
    name: Online shop
    business_unit: BU-RETAIL          # must ref
    lzs: [LZ-MAIN]                    # optional; Project ↔ LZ, many-to-many

apps:
  - code: APP-CHK
    name: Checkout
    project: PRJ-SHOP                 # is under
    cost_center: CC-ECOM              # must ref
    environments:                     # Environment is under App
      - code: ENV-PROD
        name: Production
      - code: ENV-DEV
        name: Development

lzs:
  - code: LZ-MAIN
    name: Main AWS organization
    csp: AWS                          # Release 1: AWS only
    currency: USD                     # ISO 4217
    management_account: "111111111111"
    default:                          # optional, either or both
      app: APP-PLATFORM
      environment: ENV-SHARED

defaults:                             # optional, global; either or both
  app: APP-PLATFORM
  environment: ENV-SHARED

ou_links:
  - ou: /Workloads/Checkout           # starts with "/" → name path; else an OU ID
    app: APP-CHK                      # optional
  - ou: ou-ab12-prod0001
    environment: ENV-PROD             # optional; at least one of app/environment

accounts:
  - account: "222222222222"           # 12 digits, quoted
    environment: ENV-PROD             # optional
    apps:                             # optional; at least one of environment/apps
      - app: APP-CHK                  # two or more Apps: cost split by the App tag (D-008)
      - app: APP-PAY

cost_allocation:                      # optional
  app_tag: App                        # tag key whose value is an App code; default "App"
```

### Validation

`taxonomy.Validate` returns a list of findings `{severity, code, path, message}`, where `path`
points into the merged document (for example `apps[APP-CHK].cost_center`). It never stops at the
first one. Severity `error` makes `openlz taxonomy validate` exit 1; `warning` does not.

Errors:

| Finding code | When |
|---|---|
| `yaml-invalid` | A file doesn't parse, a key is unknown, or a key appears in two files |
| `code-invalid` | A Code fails the pattern or uses a reserved suffix |
| `code-duplicate` | A Code repeats within its uniqueness scope |
| `ref-missing` | A must-ref is empty |
| `ref-unknown` | Any reference names a Code that doesn't exist |
| `lz-invalid` | `csp` isn't `AWS`, `currency` isn't three capital letters, or `management_account` isn't 12 digits |
| `ou-link-empty` | An OU link sets neither `app` nor `environment` |
| `ou-link-duplicate` | Two links on the same OU reference set the same field |
| `account-invalid` | Not 12 digits, listed twice, or sets neither `environment` nor `apps` |
| `account-app-duplicate` | The same App listed twice on one account |
| `account-env-not-in-app` | The account's Environment is not defined under one of its Apps (D-005) |
| `tag-key-invalid` | `cost_allocation.app_tag` is empty, longer than 128 characters, or starts with `aws:` |
| `default-env-not-in-app` | A default (LZ or global) sets both App and Environment and the Environment isn't under that App |

Warnings:

| Finding code | When |
|---|---|
| `biz-path-mismatch` | An App's CostCenter is not under its Project's BusinessUnit (spec v3 §3.5) |
| `unused-node` | A BusinessUnit, CostCenter or Environment that nothing references |

Checks that need the discovered inventory (OU exists, App on a leaf OU, account exists) belong to
WI-0103.

## WO-0101-A — Skeleton, check script and CI

**Assignee:** open
**Depends on:** the repo owner has pushed tag `v1-reference` on `main`
**Packages:** repo root, `cmd/`, `scripts/`, `.github/workflows/`, `.gitignore`, `README.md`

**Goal.** Replace the v1 tree with an empty, green skeleton that every later Work Order builds on.

**In scope.**
- Delete the v1 code and its leftovers: `cmd/`, `internal/`, `main.go`, `directus/`,
  `Taskfile.yml`, `taxonomy/*.json`, `docs/handoff/`, `ARCHITECTURE.md`, `.vscode/`. Keep
  `docs/taxonomy-spec.md`, `planning/`, `AGENTS.md`, `CLAUDE.md`, `GEMINI.md`, `LICENSE`.
- New `go.mod` (`github.com/PopGreenTh/OpenLZ`, keep the current Go version) and a fresh `go.sum`.
- `main.go` and a Cobra root command `openlz` with `openlz version` (version from `-ldflags`,
  default `dev`).
- `scripts/check` (a Go program): `go build`, `go vet`, `gofmt -l` (fails if it lists anything) and
  `go test`, all with `-tags=no_duckdb_arrow`; prints one PASS/FAIL line per stage and exits non-zero
  on any failure.
- `.github/workflows/check.yml`: runs `scripts/check` on every pull request and on push to `main`.
- `.gitignore` covering `data/`, `*.duckdb`, `/openlz`.
- `README.md` rewritten for the rebuild: what OpenLZ is (D-001), Release 1 status, how to build,
  and a link to `AGENTS.md`.

**Out of scope.** Any taxonomy, inventory or cost code (WO-0101-B onward). Adding DuckDB as a
dependency (WO-0104-A).

**Done when.**
- `git ls-files` shows none of the deleted paths, and all of the kept ones.
- `go run -tags=no_duckdb_arrow ./scripts/check` passes locally and in CI on the PR head.
- A deliberately unformatted file makes `scripts/check` fail at the `gofmt` stage (shown in the PR
  description, not committed).
- `openlz version` prints `dev` without ldflags.

**Verify.** `go run -tags=no_duckdb_arrow ./scripts/check`

## WO-0101-B — Taxonomy model, YAML loader and validation

**Assignee:** open
**Depends on:** WO-0101-A
**Packages:** `internal/taxonomy`, `cmd/taxonomy*.go`, `taxonomy/`

**Goal.** Load the taxonomy files into a typed model and report every problem in them.

**In scope.**
- Types for every entity in *Taxonomy files*, the merged-directory loader, and lookups the
  resolver needs (App by code, Environment by App and code, LZ by management account).
- `Validate` with every finding in *Validation* (errors and warnings).
- `openlz taxonomy validate [--dir taxonomy/]`: prints findings as a table, exit 1 on any error.
- `openlz taxonomy list [--dir taxonomy/]`: prints the Project › App › Environment tree with each
  Project's BusinessUnit and each App's CostCenter.
- `taxonomy/` starts with a small valid example (one BU, one CostCenter, one Project, one App with
  two Environments, one LZ), so `validate` passes on `main`.
- Unit tests: one test case per finding code, plus a valid document with zero findings.

**Out of scope.** Inventory-dependent checks (WO-0103-A). The demo landing zone's taxonomy
(WO-0104-B). Writing YAML (Release 1 only reads it).

**Done when.**
- Every finding code in both tables has a test that produces it, and only it.
- `validate` reports all findings in one run, each with the file name and the document path.
- `validate` on the shipped `taxonomy/` exits 0 with no findings.
- `internal/taxonomy` imports nothing else from `internal/`.

**Verify.** `go run -tags=no_duckdb_arrow ./scripts/check` and
`go run -tags=no_duckdb_arrow . taxonomy validate`
