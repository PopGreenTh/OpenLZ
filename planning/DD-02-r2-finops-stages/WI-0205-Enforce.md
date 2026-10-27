# WI-0205 — `finops/enforce`: one command per rule

## Outcome

`openlz-finops enforce` flags records that break a FinOps rule, with what is wrong, where, how
much cost it touches and what to do (D-014). Each rule is its own command, and one run can name
several (D-022). Enforce never calls the cloud.

## Contracts

`openlz-finops enforce RULE [RULE...] [--floor 30] [--by FIELDS] [--from-map[=ts]] [--from-rollup[=ts]]`

- Every input record passes through unchanged, in order; the `flag` records follow.
- One `flag` record per subject, rule and period:

| Field | Value |
|---|---|
| `kind` | `flag` |
| `rule` | the rule name |
| `level` | 1–4; `1` for rules without levels |
| `subject_type` / `subject` | what is flagged (`account` and its ID, `app`, …) |
| `period`, `granularity`, `metric` | the period the flag is about |
| `cost`, `currency` | the cost the flag touches |
| `what` | one sentence: what is wrong |
| `action` | one sentence: what to do, naming the taxonomy entry or tag to change |
| `owner` | the subject's `cost_center` when known, else `""` |

### Rules

| Rule | Reads | Flags |
|---|---|---|
| `unmapped` | allocated `cost` | cost at `APP-UNALLOC`, `ENV-UNALLOC` or `ENV-UNSPEC`, per account and period. Action: add the account entry or OU link |
| `untagged` | allocated `cost` | shared-account cost with `share_rule` `untagged-equal`, per account and period. Action: tag the resources with the App tag |
| `wrong-tag` | allocated `cost` | cost with `app_rule` `invalid`, per account, tag value and period. Action: fix the tag value or add the App to the account |
| `spike` | `cost` or `rollup` | D-024, D-025: grouped by `--by` (default `account_id`), baseline = average of the 3 periods before, factor = amount ÷ baseline; only when amount − baseline ≥ `--floor`; level 1 at factor ≥ 2, 2 at ≥ 4, 3 at ≥ 8, 4 at ≥ 16. `cost` is amount − baseline. Action: review the subject's cost for the period |

- An unknown rule name is an error that lists the rules. Rules run in the order named; a flag
  is never raised twice for the same rule, subject and period.
- Major parameters: the rules (sorted), `floor`, `by`. Its history is saved as `enforce/flags`
  (thing `flags`, WI-0202).

## WO-0205-A — The four rules

**Assignee:** open
**Depends on:** WO-0203-A (may run in parallel with WO-0204-A: different packages)
**Packages:** `finops/enforce/`, `README.md`

**Goal.** `enforce` with the four rules as specified.

**In scope.**
- The command, the four rules, `--save`, `--from-map` and `--from-rollup`.
- Table-driven tests per rule in `finops/enforce/testdata/`, including spike at each level
  boundary, below the floor, and with fewer than 3 earlier periods (no flag).

**Out of scope.** Any rule beyond the four. Any cloud call.

**Done when.**
- The tests above pass.
- `… | enrich map | openlz-finops enforce unmapped untagged wrong-tag spike` on the mock prints
  at least one flag for each of `unmapped`, `untagged` and `wrong-tag`, every input record
  unchanged, and the flags after them.
- README lists the rules with their `what` and `action`.

**Verify.** `go run -tags=no_duckdb_arrow ./scripts/check`
