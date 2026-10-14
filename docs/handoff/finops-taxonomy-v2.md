# Handoff: FinOps Cost Taxonomy v2 + Directus

> **Audience:** the next coding agent (Claude Code / Sonnet, or Antigravity / Gemini Flash).
> Read this file top to bottom before you change any code. It is self-contained.
> Branch: `claude/vigilant-feynman-0xz7te`. Last updated: 2026-10-14.

---

## 0. Ground rules

1. **Build and test commands.** Always pass `-tags=no_duckdb_arrow`.
   ```bash
   go build -tags=no_duckdb_arrow ./...
   go test  -tags=no_duckdb_arrow ./...
   ```
   All tests pass at the start of this handoff. Keep them passing after every task.
2. **Don't hand-edit what generates itself.** Snapshot JSON files are written by `taxonomy.WriteSnapshot`. Directus collections are created by the bootstrap command (Task 1).
3. **`internal/taxonomy/schema.go` → `Schema` is the only definition of the taxonomy.** It lists every collection, every reference field, and which references are required. Directus bootstrap, pull, push and validation must all loop over `Schema`. Never hard-code collection or field names a second time.
4. **Scope.** Do the tasks in §4 in order. Don't refactor unrelated code. Several files are already not `gofmt`-clean (`engine_test.go`, `rest/server.go`, `sense_costanomaly.go`, `stages/types*.go`). Leave them alone unless a task touches them.
5. **Commits.** Make one commit per task, with a conventional message (`feat(directus): …`). Push to the branch above.
6. **Ask, don't guess.** The questions in §5 belong to the product owner. Ask them before coding anything that depends on the answer.

---

## 1. The business model (what the user specified)

```
Biz Cost Hierarchy                 Project Cost Hierarchy
+ BusinessUnit                     - Project
  + CostCenter                       - App
    + Owner                            - Environment / Account
      + CustomerFunc                     - CSP / Region
                                           - ServiceCat / Service
                                             - ServiceUsage / Usage
                                               - ResourceGroup / Resource
Relations
- Project       MUST ref  BusinessUnit
- App           MUST ref  CostCenter
- ResourceGroup CAN  ref  CustomerFunc
- Region   (id unique within its CSP) can be mapped to CSP
- Account   can be mapped to Environment
- Service   can be mapped to ServiceCat
- Resource  can be mapped to ResourceGroup
- Usage     can be mapped to ServiceUsage      (implied by the pair pattern)
```

The old model had "must behave" identity rules: Project = BU, App = Owner, Env = CostCenter. **Those rules are gone.** Don't bring them back. `GEMINI.md`, `ARCHITECTURE.md` §4D/§11 and `directus/README.md` still describe the old rules; Task 4 fixes them.

### Decisions already made with the user
| Topic | Decision |
|---|---|
| Source of truth | **Directus.** `openlz taxonomy pull` exports one JSON snapshot per tenant. Enrich reads only the JSON (offline, reproducible). `push` exists only to seed Directus from JSON. |
| Tenancy in Directus | **One Directus instance.** A `tenants` collection, and every taxonomy collection has a `tenant` M2O field. The tenant code `default` ↔ `taxonomy/taxonomy.json`; any other code ↔ `taxonomy/taxonomy_<code>.json`. |
| Depth | Model all 7 project levels **and** add resource-level sensing now (done, see §2). |
| App CostCenter in a different BU than App's Project | **Warning only.** `Catalog.Warnings()` and `openlz taxonomy validate` print it; loading still succeeds. |

---

## 2. What is DONE (commits `0fea324`, `472a208`, plus the Power Query fix)

| Area | File(s) | Notes |
|---|---|---|
| API cache restored | `internal/cache/cache.go` | The `.gitignore` rule `cache/` used to hide this package; it is now `/cache/`. |
| Taxonomy schema + snapshot JSON | `internal/taxonomy/schema.go` | `Schema`, `RefSpec{Field,Target,Kind}`, `Kind` = `parent` / `must` (required) or `can` (optional). `Snapshot` = `{Tenant, Collections map[string][]Row}`, `Row` = `map[string]string`. Regions use `ScopeRef: "csp"` (id is unique within its CSP). |
| Catalog build + Resolve | `internal/taxonomy/taxonomy.go` | `Build` collects **all** errors (`errors.Join`). A reference matches a target **id**, or a **unique** name. Duplicate names are allowed but never matched by name. `Resolve(Input) Output` fills 16 slots. |
| Paths | `internal/taxonomy/paths.go` | `PathForTenant`, `ResolvePath`, `LoadOptional(path, tenant)`, `DiscoverTenants`, `TenantCode`. |
| Sample data | `taxonomy/taxonomy.json`, `taxonomy/taxonomy_tenant_a.json` | Built around the mock account ids `111122223333`, `444455556666`, `777788889999`. |
| Raw FinOps schema | `internal/duckdb/engine.go` | 19 columns; adds `csp` and `resource_id` (after `currency` and `usage_type`). Upsert upgrades older 17-column files through `columnOr`. |
| Analyze | `engine.AnalyzeFinOps` | Groups by and carries `primary_tag, secondary_tag, csp, region, usage_type, resource_id`. `resource_or_key` = `resource_id` if set, else `service`. |
| Enriched schema | `engine.insertEnrichedRecords`, `enrichedColumnSelect`, `ReadEnrichedFromParquet` | Adds 13 columns: `csp_id, csp, region_id, region, service_id, service_usage_id, service_usage, usage_id, usage, resource_group_id, resource_group, resource_id, resource`. Columns missing from older files read back as `''`. |
| Enrich stage | `internal/stages/finops/enrich_costcenters.go` | `EnrichCostCentersInput.TaxonomyPath` (JSON `taxonomyPath`). With a taxonomy: `mapHierarchy`, which leaves unresolved levels **empty** on purpose so the gaps are visible. Without a taxonomy: the old SQL heuristic (`engine.EnrichRecords`). |
| Cost Explorer | `internal/aws/costexplorer.go` | New `--group-by` modes: `SERVICE_REGION`, `SERVICE_USAGE_TYPE`, `RESOURCE` (aliases `SERVICE_RESOURCE`, `RESOURCE_ID`). `RESOURCE` calls `GetCostAndUsageWithResources` through `getCostAndUsage(...)`. It clamps the start date to the last 14 days, rejects MONTHLY, and adds a required filter (LINKED_ACCOUNT, or RECORD_TYPE=Usage in payer mode). Every record gets `CSP: "AWS"`. |
| Mocks | `internal/aws/mocks.go` | Every row has a real usage type. With `GroupBy=RESOURCE` each service gets a resource id (e.g. `i-0<account>a001`). |
| CLI | `cmd/finops.go`, `cmd/stages.go`, `cmd/openlz-finops/main.go`, `cmd/taxonomy.go` | `--taxonomy <file>` replaces `--project-taxonomy` and `--business-taxonomy`. `openlz taxonomy list|validate|init` work on the new format. |
| Power Query | `internal/powerquery/script.go` | Raw FinOps M scripts now declare 19 columns. |

### How `Resolve` fills the hierarchy (read `taxonomy.go` for the details)
1. Explicit tag values are tried first (`Project`, `App`, `Environment`, …). A tag that doesn't match **is kept as-is** and blocks derivation for that slot (see Q1 in §5).
2. Account → Environment → App → Project.
3. CSP and Region. A region is looked up within the row's CSP; without a CSP it must be unique across CSPs.
4. Usage (exact match, then with the AWS region prefix stripped, e.g. `APS1-`) → ServiceUsage → Service (only if the Service is empty) → ServiceCat.
5. Resource → ResourceGroup → CustomerFunc → Owner.
6. CostCenter = App.cost_center, or else Owner.cost_center. BU = Project.business_unit, or else CostCenter.business_unit.
7. If the Owner is still empty: the CostCenter's only Owner. If the CustomerFunc is still empty: the Owner's only CustomerFunc.

### Snapshot file format (example)
```json
{
  "tenant": "default",
  "business_units": [{"id": "BU-DIGITAL", "name": "Digital"}],
  "cost_centers":   [{"id": "CC-PAY", "name": "Payments", "business_unit": "BU-DIGITAL"}],
  "apps":           [{"id": "APP-GATEWAY", "name": "Payment Gateway", "project": "PRJ-PAYMENTS", "cost_center": "CC-PAY"}],
  "regions":        [{"id": "ap-southeast-1", "name": "Asia Pacific (Singapore)", "csp": "AWS"}]
}
```
The top-level keys are exactly the `Schema` collection names. Reference values are target **ids**; a unique name is also accepted on read.

---

## 3. Verify the baseline before you start

```bash
go build -tags=no_duckdb_arrow -o /tmp/openlz .
rm -rf data
/tmp/openlz finops sense --mock --accounts 111122223333,777788889999 --group-by RESOURCE -a
/tmp/openlz finops analyze
/tmp/openlz finops enrich          # expect: "Enriched 2 records with cost taxonomy"
/tmp/openlz taxonomy validate      # expect: PASSED (0 warnings)
/tmp/openlz taxonomy validate --tenant tenant_a
rm -rf data                        # data/ is gitignored, but keep the tree clean
```

---

## 4. Remaining tasks (do them in order)

### Task 1: Directus integration (`internal/directus` + CLI)  ★ main task

**Goal:** `openlz taxonomy directus bootstrap`, `openlz taxonomy pull --tenant X` and `openlz taxonomy push --tenant X`.

#### 1a. Package layout
```
internal/directus/
  client.go      // HTTP client: base URL, auth, JSON helpers, error mapping
  schema.go      // Bootstrap(ctx): create collections/fields/relations from taxonomy.Schema
  sync.go        // Pull(ctx, tenant) (*taxonomy.Snapshot, error); Push(ctx, snap) (PushReport, error)
  client_test.go // httptest.Server fake Directus; no Docker needed
```
Use only the standard library (`net/http`, `encoding/json`). Add no new modules.

#### 1b. Configuration (flags override environment variables)
| Flag | Env | Default |
|---|---|---|
| `--directus-url` | `OPENLZ_DIRECTUS_URL` | `http://localhost:8055` |
| `--directus-token` | `OPENLZ_DIRECTUS_TOKEN` | — (static admin token, preferred) |
| `--directus-email` / `--directus-password` | `OPENLZ_DIRECTUS_EMAIL` / `OPENLZ_DIRECTUS_PASSWORD` | — (calls `POST /auth/login`, reads `data.access_token`) |

Send `Authorization: Bearer <token>` on every request. Never log the token or the password.

#### 1c. Directus data model (built from `taxonomy.Schema`)
- Collection **`tenants`**: PK `code` (string, entered by hand), plus `name` (string).
- Every taxonomy collection `C` in `Schema`:
  - `uid`: integer, PK, auto-increment, hidden, read-only. **The business id is not the PK**, because the same id (e.g. `SC-COMPUTE`) can exist in several tenants.
  - `tenant`: string, M2O → `tenants.code`, required.
  - `code`: string, required. This is the JSON `id`.
  - `name`: string, required.
  - for each `RefSpec r`: field `r.Field`, type integer, M2O → `r.Target.uid`. Set `meta.required = r.Required()`. Display template `{{code}} - {{name}}`.
  - `meta.note`: the collection's `Label`, plus the relation text from the `Schema` doc comment.
- Uniqueness of `(tenant, code)` (regions: `(tenant, csp, code)`) is **not** enforced by Directus. `taxonomy.Build` enforces it after pull. Don't try to add composite unique constraints through the API.

REST calls (Directus 10/11):
```
GET  /collections                 -> list existing (skip if present: idempotent)
POST /collections                 {"collection":C,"meta":{"icon":"account_tree","note":...},
                                   "schema":{},
                                   "fields":[{"field":"uid","type":"integer",
                                              "meta":{"hidden":true,"readonly":true,"interface":"input"},
                                              "schema":{"is_primary_key":true,"has_auto_increment":true}}]}
GET  /fields/{C}                  -> list existing fields (skip if present)
POST /fields/{C}                  {"field":"code","type":"string","meta":{"interface":"input","required":true},"schema":{"is_nullable":false}}
POST /fields/{C}                  {"field":"business_unit","type":"integer",
                                   "meta":{"interface":"select-dropdown-m2o","special":["m2o"],
                                           "options":{"template":"{{code}} - {{name}}"},"required":true},
                                   "schema":{}}
POST /relations                   {"collection":C,"field":"business_unit","related_collection":"business_units",
                                   "schema":{"on_delete":"RESTRICT"}}
```
Create collections in `Schema` order. Targets always come earlier in the list, so relations can be created right after each collection's fields. Bootstrap must be **idempotent**: running it twice is a no-op with exit code 0.

#### 1d. Pull (Directus → JSON)
1. For each collection: `GET /items/{C}?filter[tenant][_eq]=<code>&limit=-1&fields=uid,code,name,<ref fields>`.
2. Build a `uid → code` map per collection. Turn each item into a `taxonomy.Row` with `id=code`, `name`, and each ref = the target code. A ref uid that isn't in the same tenant is an error that names the row.
3. Run `taxonomy.Build(snap)`. If it fails, print the joined errors and **do not write the file** unless `--force` is given. Print the warnings.
4. Write the snapshot with `taxonomy.WriteSnapshot(taxonomy.PathForTenant(dir, tenant), snap)`.
5. Pulling the same data twice must produce a byte-identical file. Sort rows by `id` inside each collection before writing.

#### 1e. Push (JSON → Directus, seed / upsert)
1. Read the snapshot and run `taxonomy.Build` (refuse invalid input).
2. If the tenant doesn't exist, create it (`POST /items/tenants {"code":..,"name":..}`).
3. In `Schema` order, load the existing items for the tenant and index them by `code` (regions by `csp|code`). For each row: translate ref codes → uids using the maps filled earlier, then `PATCH /items/{C}/{uid}` if the row exists, or `POST /items/{C}` if not. Read the new uid from `data.uid`.
4. **Never delete** rows in Directus. Report rows that exist in Directus but not in the JSON as "extra" in the summary.
5. Support `--dry-run` (print the plan, send no writes).

#### 1f. CLI wiring (`cmd/taxonomy.go`)
```
openlz taxonomy directus bootstrap
openlz taxonomy pull  [--tenant X] [--dir taxonomy] [--force]
openlz taxonomy push  [--tenant X] [--dir taxonomy] [--dry-run]
```
Add the same commands to the standalone `cmd/openlz-finops/main.go` only if that binary already has a `taxonomy` command (it doesn't today, so skip it).

Taskfile (`Taskfile.yml`), next to `directus:*`:
```yaml
directus:bootstrap: { cmds: ["./{{.BINARY}} taxonomy directus bootstrap"] }
taxonomy:pull:      { cmds: ["./{{.BINARY}} taxonomy pull {{.CLI_ARGS}}"] }
taxonomy:push:      { cmds: ["./{{.BINARY}} taxonomy push {{.CLI_ARGS}}"] }
```
`directus/docker-compose.yml`:
- Remove the `../taxonomy:/directus/taxonomy` mount. Directus never reads it.
- Pin the image (e.g. `directus/directus:11.5.1`) instead of `latest`.
- Add `ADMIN_TOKEN: "openlz-dev-admin-token"` so the CLI can use a static token locally.

#### 1g. Tests and acceptance
- Unit tests with `httptest.Server` that records requests:
  - bootstrap creates N collections, the right fields and the right relations;
  - a second bootstrap sends no POSTs;
  - pull maps uids → codes and filters by tenant;
  - push creates the tenant, inserts in dependency order and PATCHes existing rows;
  - a push → pull round trip returns the same snapshot.
- Manual end-to-end check (Docker is available in the Claude cloud container):
  ```bash
  docker compose -f directus/docker-compose.yml up -d    # wait for http://localhost:8055/server/health
  export OPENLZ_DIRECTUS_TOKEN=openlz-dev-admin-token
  /tmp/openlz taxonomy directus bootstrap && /tmp/openlz taxonomy directus bootstrap   # 2nd = no-op
  /tmp/openlz taxonomy push --tenant default
  /tmp/openlz taxonomy push --tenant tenant_a
  cp taxonomy/taxonomy.json /tmp/before.json
  /tmp/openlz taxonomy pull --tenant default
  diff <(jq -S . /tmp/before.json) <(jq -S . taxonomy/taxonomy.json)   # only row order may differ
  ```

### Task 2: Pipeline flags
`cmd/pipeline.go` hard-codes `SenseCostExplorerInput` without `GroupBy` and `EnrichCostCentersInput` without `TaxonomyPath`. Add `--group-by` (passed to FinOps sense) and `--taxonomy` (passed to FinOps enrich). The defaults must stay the same as today.

### Task 3: Reports show the hierarchy
Today the enriched Excel sheet (`internal/report/excel.go` `AddEnrichedSheet`, 16 columns A–P), the enriched M scripts (`internal/powerquery/script.go`: `GenerateParquetMScript` and `GenerateEnrichedCsvMScript`, `Columns=16`) and `ExportFinOpsToXLSX` (raw) show none of the hierarchy ids.
- Raw: add `CSP` and `Resource ID` to the raw Excel export.
- Enriched: append these columns after `Action Recommended`: Project, App, Cost Center, Owner, Customer Function, CSP, Region, Service Category, Service Usage, Usage, Resource Group, Resource. Update the `AutoFilter` range, the column widths and the M-script column list/count to match `enrichedColumnSelect` order. **The CSV that DuckDB exports has every enriched column (41)**, so the CSV M script must say `Columns=41` and list all of them in that order.
- Update `excel_test.go` and `script_test.go`.

### Task 4: Docs
- `ARCHITECTURE.md`: rewrite §4D (the hierarchy and relations in §1 of this file, plus the Resolve order) and §5 (raw schema 19 columns, enriched additions). Rewrite §11.2–11.4 for one snapshot per tenant, the Directus data model and the pull/push commands. Add the new `--group-by` modes to §4A / §8.
- `directus/README.md`: replace the "Must Behave" table with the relations, and add bootstrap → push → edit → pull.
- `README.md`: one short "Cost taxonomy" section that links to ARCHITECTURE.
- `GEMINI.md` and `CLAUDE.md` already point here. After Task 1, change the "in progress" note to "done".

### Task 5 (only after the product owner answers §5): resolve-rule follow-ups.

---

## 5. Open questions for the product owner (ask before coding them)
- **Q1 – unmatched tags.** Mock rows carry `Project=CloudMigration` and `Application=PaymentGateway`, and neither exists in the taxonomy. Today an unmatched tag is *kept* and **blocks** derivation from the account. For example, account `777788889999` maps to `ENV-LAKE-PROD → APP-LAKE`, yet its rows come out with an empty project/app/cost center. The options are:
  - (a) keep it as is (tags win);
  - (b) the account mapping wins, and the unmatched tag is kept in a separate `*_tag` column for review;
  - (c) tags win only when they match the taxonomy.
- **Q2 – owner vs cost center conflict.** When a resource's CustomerFunc → Owner belongs to a different CostCenter than the App's, the code keeps both, so Owner doesn't roll up to the row's CostCenter. Should this be flagged?
- **Q3 – Environment names.** Environments are per App, so names like "Production" repeat and can never be matched by name from a tag. Is matching by id only acceptable?

## 6. Known backlog (out of scope unless asked; listed so you don't "fix" them by accident)
- Analyze compares raw `amount` (not `daily_amount`) and mixes metrics and frequencies in the master parquet. The default 1-day window makes max = avg.
- The cache stores today's partial-day costs under a stable key.
- `finops sense` without `-a` doesn't update `data/finops_raw.parquet`, which `analyze` reads by default.
- `finops-sense-costanomaly` and `analyze` both write `data/finops_anomalies.parquet`, in different schemas.
- Enforce never calls AWS, even with `--dry-run=false`.
- Live, non-region group-by modes still stamp `region` with the client's region (`c.cfg.Region`).
