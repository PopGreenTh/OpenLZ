# WI-0104 — Cost allocation and reports

## Outcome

Monthly AWS spend per linked account is read from Cost Explorer (or the mock), allocated through
the account map to Project › App › Environment, and reported as a rollup, a coverage view and a
findings list. No cost is lost: per month, metric and currency, allocated cost equals Cost
Explorer cost.

## Contracts

### Reading cost

- `GetCostAndUsage`, granularity `MONTHLY`, grouped by dimension `LINKED_ACCOUNT` and by tag
  `<cost_allocation.app_tag>` (D-008), one metric per run: `--metric` one of `UnblendedCost` (default), `AmortizedCost`, `NetUnblendedCost`,
  `NetAmortizedCost`, `BlendedCost`.
- Range `--from YYYY-MM --to YYYY-MM`, inclusive. Default: the last three complete months and the
  current month to date.
- Credentials per LZ: `--profile <LZ code>=<profile>`, repeatable. Without it, the default chain is
  used, which works for one LZ. The command checks which LZ it reached with
  `organizations:DescribeOrganization` and stops if the management account matches no LZ in the
  taxonomy.
- IAM: `ce:GetCostAndUsage`, `organizations:DescribeOrganization`. The App tag key must be
  activated as a cost allocation tag in AWS Billing (README states it).

### Cache ($0.01 per call)

Every Cost Explorer call goes through a local DuckDB cache, `openlz_cache.duckdb`, keyed by a
hash of the full request. A response that covers only closed months is reused forever; one that
covers the current month is reused for 12 hours. `--refresh` skips the cache. Each run prints how
many calls it made and their cost at $0.01 each.

### Raw cost file

`data/cost_raw.parquet`, one row per LZ, account, App tag value, month and metric:

| Column | Type |
|---|---|
| `lz` | VARCHAR |
| `account_id` | VARCHAR |
| `month` | VARCHAR, `YYYY-MM` |
| `metric` | VARCHAR |
| `amount` | DOUBLE |
| `app_tag` | VARCHAR: the App tag's value, trimmed; empty when untagged (Cost Explorer's `<key>$`) |
| `currency` | VARCHAR (Cost Explorer's `Unit`) |
| `estimated` | BOOLEAN |

### Allocation

For each raw row, the account's allocations are taken from the account map (WI-0103):

- An account with one allocation (`share_rule` `single`, or any system node) takes the row
  whole, whatever its `app_tag`.
- An account whose allocations have `share_rule` `tag` (a shared account, D-008): a row whose
  `app_tag` equals the code of one of those Apps goes whole to that App's allocation. Otherwise
  it goes to `APP-UNALLOC` (Project, BusinessUnit and CostCenter `UNALLOC`) under the account's
  Environment, with `app_rule` `unalloc`, and finding `shared-untagged` (subject
  `<account ID>/<month>`) when `app_tag` is empty, or `shared-tag-invalid` (subject
  `<account ID>/<tag value>`) when it isn't. These two are the Q-002 defaults.
- No row is ever divided, so every row lands whole in exactly one place.
- An account in the cost data but not in the account map (for example a closed account) is
  allocated whole to `PRJ-UNALLOC › APP-UNALLOC › ENV-UNALLOC` under its row's LZ, with finding
  `account-not-in-inventory` (subject: account ID).
- A row whose currency differs from its LZ's currency keeps its own currency and gets finding
  `currency-mismatch` (subject `<LZ code>/<month>`). Amounts are never converted (spec v3 §7).
- If the run has shared accounts and no raw row at all carries an App tag value, finding
  `app-tag-not-seen` (subject: the tag key): the tag is probably not activated for cost
  allocation.
- **Conservation check:** after allocation, for every month, metric and currency, the allocated
  sum equals the raw sum to within `1e-6`. If not, the command fails; that is a bug, not a data
  problem.

`data/cost_allocated.parquet` has the account-map columns (`lz`, `account_id`, `account_name`,
`ou_path`, `project`, `app`, `environment`, `business_unit`, `cost_center`, `share`,
`share_rule`, `app_rule`, `environment_rule`) plus `app_tag`, `month`, `metric`, `amount`, `currency`,
`estimated` and `taxonomy_hash`.

### Reports

All read `data/cost_allocated.parquet` through DuckDB SQL. Amounts are never summed across
currencies; every report groups by currency.

| Report | Shows |
|---|---|
| `rollup` | Month × BusinessUnit › Project › App › Environment, with subtotals per Project and BusinessUnit |
| `coverage` | Per LZ and month: total cost; cost and share at `APP-UNALLOC`; cost and share at `ENV-UNSPEC` or `ENV-UNALLOC`; accounts mapped vs not; the unmapped accounts by cost, largest first |
| `findings` | Findings from the account map and the cost step, grouped by code, with subjects |

### Reference cases

`cases/cost/*.yaml`: like the mapping cases, plus a `cost` list of raw rows and
`expect.allocated` (only the fields written are compared; amounts to within `1e-9`).

## WO-0104-A — Cost Explorer, cache and allocation

**Assignee:** open
**Depends on:** WO-0102-A, WO-0103-A
**Packages:** `internal/cost`, `internal/store`, `cmd/cost*.go`, `testdata/cost/`

**Goal.** Produce `cost_raw.parquet` and `cost_allocated.parquet` as specified.

**In scope.**
- `internal/store`: open DuckDB (embedded, `-tags=no_duckdb_arrow`), write and read Parquet.
- The cache, the Cost Explorer reader (through `internal/awsx`'s retry wrapper, all pages read),
  and the mock: deterministic monthly amounts per account of the demo inventory (seeded by
  account ID and month), App tag values for the demo's shared account, plus one account that is
  not in the demo inventory.
- `openlz cost sense [--mock] [--from] [--to] [--metric] [--profile LZ=P] [--refresh]` and
  `openlz cost allocate [--map data/account_map.json]`.
- A test that loads every file in `planning/DD-01-r1-account-mapping/cases/cost/`.
- Unit tests: cache hit, miss and expiry; paging; parsing the tag group key (`App$APP-CHK`,
  `App$`); `app-tag-not-seen`; the conservation check
  failing on a hand-broken allocation.

**Out of scope.** Report formatting (WO-0104-B). Daily or hourly granularity, tags, services and
regions (later releases). Currency conversion.

**Done when.**
- Every cost reference case passes, unchanged.
- `cost sense --mock` twice gives identical `cost_raw.parquet` contents.
- A second `cost sense` over closed months makes zero Cost Explorer calls (shown by a test with a
  counting fake client).
- `cost allocate` fails when the conservation check fails, and the test proves it.

**Verify.** `go run -tags=no_duckdb_arrow ./scripts/check`

## WO-0104-B — Reports, `openlz run` and the demo

**Assignee:** open
**Depends on:** WO-0104-A
**Packages:** `internal/report`, `cmd/report*.go`, `cmd/run*.go`, `examples/demo/`, `README.md`

**Goal.** Turn the allocated cost into the three reports, and run the whole flow in one command.

**In scope.**
- `openlz report rollup|coverage|findings [--format table|csv|md] [--month YYYY-MM] [--out DIR]`.
  CSV goes to `data/reports/<report>.csv` by default.
- `openlz run [--mock] [--taxonomy DIR]`: inventory sense › taxonomy validate › map › cost sense ›
  cost allocate › all three reports to the console; stops at the first failing step and names it.
- `examples/demo/taxonomy/*.yaml`: the taxonomy for the demo landing zone (WO-0102-A's mock), used
  by `run --mock` when `--taxonomy` is not given. It has two BusinessUnits, at least two Projects,
  the shared account split by App tag between two Apps (the mock cost includes tagged, untagged
  and wrongly tagged rows for it), and one account left unmapped so the
  coverage report has something to show. It passes `taxonomy validate` with no errors.
- README: a five-minute quickstart with `openlz run --mock`, the full IAM action list for a live
  run, and how to edit the taxonomy through a PR.

**Out of scope.** Power BI, Excel and REST outputs (a later release). Any UI.

**Done when.**
- `openlz run --mock` on a clean clone, with no AWS credentials, prints all three reports and
  exits 0.
- The coverage report for the demo shows the unmapped account, its cost and the mapped share.
- Golden-file tests for each report in `table` and `csv` format on a fixed allocated file.
- No report sums amounts across currencies (a test with two currencies proves it).

**Verify.** `go run -tags=no_duckdb_arrow ./scripts/check` and
`go run -tags=no_duckdb_arrow . run --mock`
