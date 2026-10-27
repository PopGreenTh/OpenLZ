# WI-0203 — `finops/enrich`: map accounts and cost to the taxonomy

## Outcome

`openlz-finops enrich map` reads `ou`, `account` and `cost` records and adds the taxonomy to
them: each account gets its LZ, Project, App and Environment, and each cost record is allocated
to them, a shared account's cost split by the App tag. With `--as-reported`, each period is
mapped with the taxonomy and the organization as they were then.

## Contracts

### Rules (unchanged from DD-01)

Account resolution is DD-01 [WI-0103](../DD-01-r1-account-mapping/WI-0103-Account-Mapping.md)
(steps 0–6, findings). Cost allocation is the *Allocation* section of DD-01
[WI-0104](../DD-01-r1-account-mapping/WI-0104-Cost-And-Reports.md) (App-tag split D-008,
`APP-INVALID` D-010, equal split of untagged cost D-011, closed accounts, currency mismatch,
`app-tag-not-seen`, the conservation check). Only the input and output change, to records.

### `enrich map`

`openlz-finops enrich map [--taxonomy DIR] [--accounts <stage>/<thing>[@ts]] [--as-reported] [--from-history …] [--save=true]`

- **Organization input.** The `ou` and `account` records on stdin. When stdin has none, the
  saved run named by `--accounts` (default `sense/accounts`, newest) is read; the file read is
  printed to stderr. Neither → error.
- **Output**, in this order:
  1. every `ou` and `account` record, unchanged;
  2. one `allocation` record per account and App: `account_id`, `account_name`, `lz`,
     `ou_path`, `project`, `app`, `environment`, `business_unit`, `cost_center`, `share`
     (number, or `null` for a tag split), `share_rule`, `app_rule`, `environment_rule`,
     `taxonomy_hash`;
  3. each `cost` record replaced by one allocated `cost` record per share it lands in: all its
     Sense fields, `amount` set to the allocated amount, plus `source_amount` (the amount
     before the split), the `allocation` fields above (`share` and `share_rule` describe this
     row's split, as in WI-0104) and `taxonomy_hash`;
  4. `finding` records, the input's first and then this stage's, with `stage` `enrich`.
- Records of any other `kind` pass through unchanged, after the `cost` records.
- **Conservation.** For every period, metric and currency, the allocated `amount` sum equals the
  input `amount` sum to within `1e-6`, or the command fails.
- **Taxonomy history (D-023).** On every saved run the taxonomy files used are copied once to
  `data/history/finops/taxonomy/<taxonomy_hash>/`, unchanged, and never overwritten.
- **`--as-reported` (D-016).** Each cost record's period is mapped with the newest saved
  taxonomy copy and the newest saved `sense/accounts` run whose timestamps are on or before the
  period's last day. When none is that old, the oldest saved one is used and finding
  `as-reported-fallback` (subject: the period) says so. Without `--as-reported`, every period
  uses the current taxonomy and organization (restated).
- Major parameters: `taxonomy` (its hash), `as-reported`.

### `enrich validate` and `enrich list`

As built in WO-0201-A; they also accept `--from-history` like any command, and are otherwise
unchanged.

### Reference cases

`finops/enrich/testdata/cases/mapping/*.yaml` and `…/cases/cost/*.yaml` (moved unchanged in
WO-0201-A). The test turns each case's inventory and cost rows into input records (`month`
becomes `period` with `granularity` `monthly`) and compares the output records to the case's
`expect`. The case files stay as they are.

### Demo taxonomy

`taxonomy/` becomes the taxonomy for the mock organization, so the mock pipeline maps
end to end: at least two BusinessUnits and two Projects, the demo's shared account served by two
Apps with `cost_allocation.app_tag`, and one account left unmapped. It passes
`enrich validate` with no errors.

## WO-0203-A — `enrich map` on records, `--as-reported`, demo taxonomy

**Assignee:** open
**Depends on:** WO-0202-B
**Packages:** `finops/enrich/`, `finops/taxonomy/`, `taxonomy/`, `README.md`

**Goal.** `sense accounts`, `sense cost` and `enrich map` chain to allocated cost.

**In scope.**
- `enrich map` as specified, with `--save` and `--from-history`.
- The old `--inventory` input and `account_map.json` output of `map` are removed.
- The demo taxonomy above.
- Tests: every reference case passes unchanged; organization from stdin and from history;
  conservation failure on a hand-broken allocation; the taxonomy copy is written once;
  `--as-reported` picks the right snapshots and raises `as-reported-fallback`.

**Out of scope.** Analyze (WI-0204). `sense accounts --out` (removed in WO-0206-A).

**Done when.**
- Every mapping and cost reference case passes, unchanged.
- `(openlz-finops sense accounts --mock; openlz-finops sense cost --mock) | openlz-finops enrich map`
  prints `allocation` and allocated `cost` records, and the shared account's cost is split by
  the App tag.
- The `--as-reported` and conservation tests pass.
- README shows the command, the record fields and the two ways to give it the organization.

**Verify.** `go run -tags=no_duckdb_arrow ./scripts/check` and
`go run -tags=no_duckdb_arrow ./finops sense cost --mock --save=false | go run -tags=no_duckdb_arrow ./finops enrich map --save=false | head`
