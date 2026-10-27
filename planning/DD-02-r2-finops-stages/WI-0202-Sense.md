# WI-0202 — `finops/sense`: accounts and cost

## Outcome

`openlz-finops sense accounts` and `openlz-finops sense cost` read the AWS Organization and Cost
Explorer (or the mock) and write flat JSON Lines records to stdout for the next stage. Every run
is saved under `data/history/`, and Cost Explorer is never asked twice for a closed period. This
Work Item also defines the record and history rules that every later stage follows.

## Contracts

### Records (all stages, D-015)

- UTF-8 JSON Lines: one JSON object per line, `\n` line ends, no blank lines.
- **Flat**: values are strings, numbers, booleans or `null`. No objects, no arrays. A list is
  written as one string with a stated separator.
- Every record has `kind`. A stage passes on every record whose `kind` it doesn't handle,
  unchanged and in order, so `finding` records reach Report from any stage.
- Field names are `snake_case`. Amounts are JSON numbers; dates are strings
  (`YYYY-MM`, `YYYY-MM-DD`, or RFC 3339 UTC for timestamps).
- A stage that reads stdin reads to the end before it fails, and reports a malformed line by its
  line number. An empty stdin is valid input.
- Package `record/` (top level, beside `aws/`, used by every domain) holds the reader, the
  writer, the flatness check and the history store below. It holds no stage logic.

### `finding` records

`{"kind":"finding","severity":"warning","code":"ou-path-ambiguous","subject":"/Workloads","message":"…","stage":"sense"}`

`severity` is `info`, `warning` or `error`. `stage` names the stage that raised it.

### `sense accounts`

`openlz-finops sense accounts [--mock] [--profile P]... [--save=true]`

For each organization read, one `ou` record per OU and one `account` record per account, OUs
first, each group sorted by `id`:

| `kind` | Fields |
|---|---|
| `ou` | `org_id`, `management_account`, `id`, `name`, `parent_id`, `path` (`/Workloads/Checkout`), `id_path` (OU IDs from the root down, joined by `/`), `has_children` |
| `account` | `org_id`, `management_account`, `id`, `name`, `email`, `status`, `parent_id`, `ou_path`, `ou_id_path` |

Paths, `ou-path-ambiguous` and AWS access are as in DD-01 WI-0102 (unchanged behaviour). The root
is written as an `ou` record with `path` `/` and `parent_id` `""`. Two runs over the same
organization produce the same output.

`--profile` may be repeated, one per organization; without it the default AWS chain is used.

### `sense cost`

`openlz-finops sense cost [--mock] [--profile P]... [--granularity monthly|daily] [--from] [--to] [--refresh] [--save=true]`

- Cost Explorer `GetCostAndUsage`, grouped by `LINKED_ACCOUNT` and by tag
  `cost_allocation.app_tag` (D-008), read through `aws/`'s retry wrapper with all pages.
- **Metric by grain** (D-022): `monthly` → `UnblendedCost`, `daily` → `AmortizedCost`.
- **Range.** `--from` and `--to` are inclusive, `YYYY-MM` for monthly and `YYYY-MM-DD` for daily.
  Defaults: monthly, the 24 complete months before the current one plus the current month to
  date (D-016); daily, the last 30 days. When Cost Explorer has less history than asked, the
  missing periods are skipped and one `finding` `cost-history-short` (subject: the first period
  returned) says how many months were available.
- The tag key is read from the taxonomy: `--taxonomy DIR`, default `taxonomy/`.

One `cost` record per management account, linked account, period, metric and App tag value:

| Field | Value |
|---|---|
| `kind` | `cost` |
| `management_account` | the account the call was made through |
| `account_id` | the linked account |
| `granularity` | `monthly` or `daily` |
| `period` | `YYYY-MM` or `YYYY-MM-DD` |
| `metric` | `UnblendedCost` or `AmortizedCost` |
| `amount` | number |
| `currency` | Cost Explorer's `Unit` |
| `app_tag` | the App tag value, trimmed; `""` when untagged (`<key>$`) |
| `estimated` | boolean |

Sorted by `account_id`, `period`, `app_tag`.

### History and cache store (D-015, D-016)

Everything lives under `data/history/` and is written once, never edited:

```
data/history/aws/accounts/<management_account>/<UTC timestamp>.jsonl
data/history/aws/cost/<management_account>/<granularity>-<metric>-<tag key>/<period>.jsonl
data/history/aws/cost/<management_account>/<granularity>-<metric>-<tag key>/<period>@<UTC timestamp>.jsonl
```

- Timestamps are `YYYYMMDDTHHMMSSZ`. The folder names are the run's **major parameters**; the
  cache key is that folder plus the period.
- **Closed period** (a month before the current one; a day before yesterday): stored once as
  `<period>.jsonl`. A later run reads it and makes no Cost Explorer call for it.
- **Open period** (the current month; today and yesterday): stored as `<period>@<timestamp>.jsonl`
  on every call; a run reuses the newest one if it is under 12 hours old.
- `--refresh` calls Cost Explorer for every period in range and stores the answer as a new
  `@<timestamp>` file; a closed `<period>.jsonl` is never overwritten. The newest file of a
  period wins when reading.
- Each Cost Explorer call fetches **one** period range of uncached periods; the run prints to
  stderr how many calls it made and their cost at 0.01 USD each.
- `--save=false` writes nothing (for tests and dry runs); the cache is then not read either.
- The `--mock` store is separate: `data/history/mock/…`, so mock runs never mix with real ones.

### Mock

The demo organization of WO-0201-A, embedded. Mock cost is deterministic: an amount per account
and period seeded from both, App tag values on the demo's shared account (tagged, untagged and
wrongly tagged rows), and one account that is not in the demo organization. Monthly data covers
26 months, so the 24-month default is fully served; daily covers the last 60 days.

## WO-0202-A — Records, `sense accounts` as JSON Lines, history store

**Assignee:** open
**Depends on:** WO-0201-A
**Packages:** `record/`, `finops/sense/`, `aws/` (only what `sense accounts` needs), `README.md`

**Goal.** `sense accounts` writes the `ou`, `account` and `finding` records to stdout and saves
its run under `data/history/`.

**In scope.**
- `record/`: reader, writer, flatness check, history store paths and the write-once rule.
- `sense accounts` as specified. Its old `--out` inventory file stays, unchanged, until WI-0203
  moves `enrich map` to JSON Lines; `--out` is then removed by that Work Order.
- Golden-file test: `sense accounts --mock --save=false` matches
  `finops/sense/testdata/accounts.jsonl`.
- Unit tests: a nested value is rejected; unknown `kind`s pass through unchanged and in order; a
  malformed line is reported by number; the store never overwrites a file.

**Out of scope.** `sense cost` (WO-0202-B). Reading these records in other stages (WI-0203+).

**Done when.**
- `openlz-finops sense accounts --mock | head` shows `ou` records then `account` records, each a
  flat JSON object with `kind`.
- Two `--mock` runs give identical stdout and two history files.
- The golden-file and unit tests above pass.
- README shows the command and the record fields.

**Verify.** `go run -tags=no_duckdb_arrow ./scripts/check` and
`go run -tags=no_duckdb_arrow ./finops sense accounts --mock --save=false`

## WO-0202-B — `sense cost`, keyed cache and mock cost

**Assignee:** open
**Depends on:** WO-0202-A; dispatched after Q-005 item 2 (whether `--metric` may override the
metric)
**Packages:** `finops/sense/`, `aws/` (Cost Explorer client and mock), `README.md`

**Goal.** `sense cost` writes `cost` records for up to 24 months, calling Cost Explorer only for
periods it has not stored.

**In scope.**
- The Cost Explorer client in `aws/`, through the retry wrapper, with paging.
- `sense cost` with the flags, metric rule, defaults, store and mock above.
- Tests with a counting fake client: a second run over closed months makes **zero** calls; an
  open period is reused under 12 hours and fetched again after; `--refresh` fetches and never
  overwrites a closed file; parsing `App$APP-CHK` and `App$`; `cost-history-short`.
- README: the command, the IAM action `ce:GetCostAndUsage`, activating the App tag as a cost
  allocation tag, and turning on multi-year data in AWS Billing for 24 months.

**Out of scope.** Splitting cost by App (WI-0203). Any metric choice beyond D-022 unless Q-005
item 2 adds it.

**Done when.**
- `openlz-finops sense cost --mock` prints 25 months of `cost` records for the demo accounts
  (24 complete months and the current one), sorted as specified.
- The counting-client tests above pass.
- A mock run with `--granularity daily` prints `AmortizedCost` records for the last 30 days.

**Verify.** `go run -tags=no_duckdb_arrow ./scripts/check` and
`go run -tags=no_duckdb_arrow ./finops sense cost --mock --save=false | head`
