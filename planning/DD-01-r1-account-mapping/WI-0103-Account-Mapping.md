# WI-0103 — Account mapping

## Outcome

`openlz map` resolves every discovered account to one Environment and one or more
Project › App allocations (with BusinessUnit and CostCenter), records the rule that decided each
level, and reports every account it could not map and why. It is the account-evidence step that a
later release's resource-level attribution builds on (D-006).

## Contracts

### Resolution

Input: a valid taxonomy (WI-0101) and an inventory (WI-0102). For each account `a` in each
organization:

**0. LZ.** The organization's LZ is the taxonomy LZ whose `management_account` equals the
organization's. None → LZ `LZ-UNALLOC`, every level below `UNALLOC`, finding `lz-unknown`, and
steps 1–5 are skipped for the organization's accounts.

**1. OU links.** Each OU link's `ou` is resolved against the inventory: a value starting with `/`
by path, otherwise by OU ID. A path is matched in every organization, so `/Workloads` can reach
one OU per organization; an ID reaches at most one. A reference that matches no OU in any
organization is ignored (`ou-not-found`). Two links
that reach the same OU and set the same field are an error (`ou-link-conflict`) and `map` stops.
An `app` link on an OU with no child OUs (a leaf, D-007) is ignored (`app-on-leaf-ou`); an
`environment` on the same link still applies.

**2. Apps** — the first source that gives an answer wins:

| Order | Source | `app_rule` |
|---|---|---|
| 1 | The account entry's `apps` | `account` |
| 2 | The deepest OU on the account's chain with an `app` link | `ou:<OU path>` |
| 3 | The LZ's `default.app` | `lz-default` |
| 4 | The global `defaults.app` | `default` |
| 5 | Nothing: App `APP-UNALLOC`, Project `PRJ-UNALLOC`, finding `account-unmapped` | `unalloc` |

Shares: one App → `share` `1`, `share_rule` `single`. Several Apps (account entry only) → one
allocation per App with `share` `null` and `share_rule` `tag`: the cost step splits the account's
cost by the App tag (D-008, WI-0104).

**3. Environment** — resolved separately, same order:

| Order | Source | `environment_rule` |
|---|---|---|
| 1 | The account entry's `environment` | `account` |
| 2 | The deepest OU on the chain with an `environment` link | `ou:<OU path>` |
| 3 | The LZ's `default.environment` | `lz-default` |
| 4 | The global `defaults.environment` | `default` |
| 5 | Nothing: `ENV-UNSPEC` if the App resolved, `ENV-UNALLOC` if not; finding `environment-unresolved` when the App resolved | `unspec` / `unalloc` |

**4. Environment must be under the App.** For each allocation, a resolved Environment code (from
order 2–4) that is not defined under that App becomes `ENV-UNSPEC` with `environment_rule`
`unspec` and finding `environment-not-in-app`. (From order 1 this can't happen: validation
rejects it.) Under `APP-UNALLOC` any resolved Environment code is kept as is, so the report
still shows, say, unmapped production spend.

**5. Derived and cross-checks.** Project = the App's Project; BusinessUnit = the Project's;
CostCenter = the App's. Under `APP-UNALLOC` they are `PRJ-UNALLOC`, `BU-UNALLOC`, `CC-UNALLOC`.
If an allocation's Project lists `lzs` and the account's LZ is not among them, finding
`project-lz-unlinked`; the allocation stands.

**6. Inventory checks.** An account entry for an account not in the inventory → finding
`account-not-discovered`.

All App and Environment decisions are made on taxonomy nodes, never by joining bare codes.

### Findings

Every finding has `severity`, `code`, `subject` and `message`. Cases compare `(code, subject)`.

| Code | Severity | Subject |
|---|---|---|
| `lz-unknown` | warning | organization ID |
| `ou-not-found` | warning | the `ou` value as written |
| `ou-link-conflict` | error | the OU ID |
| `app-on-leaf-ou` | warning | the `ou` value as written |
| `account-unmapped` | warning | account ID |
| `environment-unresolved` | warning | account ID |
| `environment-not-in-app` | warning | `<account ID>/<App code>` |
| `project-lz-unlinked` | warning | `<account ID>/<Project code>` |
| `account-not-discovered` | warning | account ID |
| `ou-path-ambiguous` | warning | the OU path (raised by WI-0102, passed through) |

### Account map file

Default `data/account_map.json`: `{ "generated_at", "taxonomy_hash", "inventory_collected_at",
"allocations": [...], "findings": [...] }`. `taxonomy_hash` is the SHA-256 of the merged taxonomy
document in canonical JSON. Allocations are sorted by `account_id`, then `app`:

```json
{
  "lz": "LZ-MAIN",
  "account_id": "222222222222",
  "account_name": "chk-prod",
  "ou_path": "/Workloads/Checkout/Prod",
  "project": "PRJ-SHOP",
  "app": "APP-CHK",
  "environment": "ENV-PROD",
  "business_unit": "BU-RETAIL",
  "cost_center": "CC-ECOM",
  "share": 1,
  "share_rule": "single",
  "app_rule": "ou:/Workloads/Checkout",
  "environment_rule": "ou:/Workloads/Checkout/Prod"
}
```

For an account under the root, `ou_path` is `/` and an OU rule names `ou:/`.

### Reference cases

`cases/mapping/*.yaml`. Each file is one case:

```yaml
name: short-kebab-name
why: one or two sentences
inventory: { ... }        # the inventory document (WI-0102), as YAML
taxonomy: { ... }         # the merged taxonomy document (WI-0101), as YAML
expect:
  allocations:            # the full list; only the fields written here are compared
    - { account_id: "…", app: APP-…, environment: ENV-…, share: 1, app_rule: …, environment_rule: … }
  findings:               # the exact set of (code, subject)
    - { code: …, subject: "…" }
```

Shares are compared to within `1e-9`; `share: null` means the field is null. `findings` are the resolver's findings from the table above;
taxonomy validation warnings are not part of a mapping case. To keep cases small, a case's inventory may leave out the
management account; the resolver must not require it.

## WO-0103-A — Account resolver

**Assignee:** open
**Depends on:** WO-0101-B. Built in parallel with WO-0102-A, and **merges after it**.
**Packages:** `internal/mapping`, `cmd/map*.go`. Never `internal/inventory` (WO-0102-A owns it).

**Goal.** Implement *Resolution* and the account map file, proven by every reference case.

**In scope.**
- The resolver, the findings, and the account map writer and reader.
- The resolver reads the inventory through an **interface defined in `internal/mapping`** with only
  the lookups it needs (the organizations, each account's OU chain from the root, an OU by ID and by
  path, whether an OU has child OUs). The reference-case test feeds it from the case's `inventory:`
  block; `openlz map` feeds it from `internal/inventory` (WO-0102-A). If WO-0102-A has not merged
  when this PR is ready, it waits for that merge, then merges `main` in and wires `openlz map`.
- `openlz map [--taxonomy taxonomy/] [--inventory data/inventory.json] [--out data/account_map.json]`:
  validates the taxonomy first (stops on errors), resolves, writes the file, and prints one line per
  allocation (LZ, OU path, account, Project › App › Environment, share, rules) and the findings.
- A test that loads **every** file in `planning/DD-01-r1-account-mapping/cases/mapping/` and checks
  it against the resolver, one subtest per file.
- Deterministic output: the same inputs give a byte-identical file apart from `generated_at`.

**Out of scope.** Cost (WO-0104-A). Tags and resources (a later release). OU path patterns.

**Done when.**
- Every reference case passes, and no case file was changed by the PR.
- Every finding code in *Findings* (except `ou-path-ambiguous`) is produced by at least one case or
  unit test.
- `openlz map` stops with exit 1 and prints the findings when the taxonomy has an error or an
  `ou-link-conflict`.
- Running `openlz map` twice on the same inputs gives files that differ only in `generated_at`.

**Verify.** `go run -tags=no_duckdb_arrow ./scripts/check`
