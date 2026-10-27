# WI-0204 — `finops/analyze`: rollup, trend, changes

## Outcome

Three Analyze commands that pipe into each other and into Enforce and Report (D-022, D-023):
`analyze rollup` sums cost by any taxonomy level, `analyze trend` shows cost over time and how
much each period moved, and `analyze changes` shows what changed between two runs and the cost
that moved with it.

## Contracts

All three: amounts are never summed across currencies or metrics; groups always include
`metric` and `currency`. Records of kinds a command doesn't read pass through unchanged, after
its own output. `--save` and the `--from-<thing>` flags as in WI-0202; each command's flags
are listed with it.

### `analyze rollup`

`openlz-finops analyze rollup [--by FIELDS] [--from-cost[=ts] | --from-map[=ts]]`

- Reads `cost` records; `--by` is a comma list of record fields, default
  `business_unit,project,app,environment`.
- Emits one `rollup` record per `--by` values, `granularity`, `period`, `metric` and `currency`:
  those fields, plus `amount` (sum), `rows` (count) and `by` (the `--by` list as written). Sorted
  by `period`, then the `--by` fields. The `cost` records it read are not passed on.
- A field in `--by` missing from a record counts as `""`, with one `finding`
  `rollup-field-missing` (subject: the field).
- Major parameters: `by`.

### `analyze trend`

`openlz-finops analyze trend [--by FIELDS] [--from-map[=ts] | --from-rollup[=ts]]`

- Reads `cost` or `rollup` records (rollup records keep their own `by` when `--by` is not
  given). Groups by `--by`, `granularity`, `metric` and `currency`, ordered by `period`.
- Emits one `trend` record per group and period: the group fields, `period`, `amount`,
  `prev_amount` (`null` for the first period), `change` (amount − prev_amount), `change_pct`
  (`null` when prev_amount is 0 or null), `baseline` (average of the 3 periods before; `null`
  with fewer than 3), `factor` (amount ÷ baseline; `null` when baseline is null or 0), and
  `impact_share` (this group's `change` ÷ the sum of all groups' `change` in that period and
  currency; `null` when that sum is 0). A missing period inside a group's range counts as 0.
- Major parameters: `by`.

### `analyze changes`

`openlz-finops analyze changes [--from-map[=ts]] [--against=ts]`

- Reads the current `account` and `allocation` records (and `cost`, if present) from the pipe
  or `--from-map`, and the same kinds from an earlier saved `enrich map` run: `--against=<ts>`,
  default the newest saved `enrich map` run older than the current one.
- Emits one `change` record per difference: `subject_type` (`account` or `allocation`),
  `subject` (account ID, or `<account ID>/<app>`), `change` (`added`, `removed`, `moved`,
  `remapped`), `field` (for `moved`: `ou_path`; for `remapped`: `app`, `environment`,
  `project`, …), `old`, `new`, `against` (the run compared with), and, when `cost` records
  are present, `cost_moved` and `currency`: the cost on that subject over the periods in stdin.
- Major parameters: `against`.

## WO-0204-A — `analyze rollup`, `trend` and `changes`

**Assignee:** open
**Depends on:** WO-0203-A
**Packages:** `finops/analyze/`, `README.md`

**Goal.** The three commands as specified.

**In scope.**
- The three commands with `--save` and their `--from-<thing>` flags.
- Table-driven tests with fixed record files in `finops/analyze/testdata/`: rollup by one and by
  several fields; two currencies never summed; trend with a gap, with fewer than 3 periods, and
  with a zero previous period; changes with each `change` kind and with and without cost.

**Out of scope.** Flags (WI-0205). Formatting (WI-0206).

**Done when.**
- The tests above pass.
- `… | enrich map | analyze rollup --by app | analyze trend` on the mock prints `trend` records for
  25 months per App.
- Two saved `enrich map` runs with one account moved between OUs give one `moved` change for it.
- README shows the three commands and their record fields.

**Verify.** `go run -tags=no_duckdb_arrow ./scripts/check`
