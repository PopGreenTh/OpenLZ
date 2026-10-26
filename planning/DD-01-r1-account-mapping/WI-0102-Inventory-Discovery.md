# WI-0102 — Inventory discovery

## Outcome

`openlz inventory sense` reads an AWS Organization (its root, nested OU tree and accounts) and
writes one inventory file that the mapping step reads. `--mock` produces the demo landing zone's
inventory with no AWS access.

## Contracts

### Inventory file

Default path `data/inventory.json`. One document per run, every organization the run read:

```json
{
  "collected_at": "2026-10-25T08:00:00Z",
  "source": "aws",
  "organizations": [
    {
      "id": "o-a1b2c3d4e5",
      "management_account": "111111111111",
      "root_id": "r-ab12",
      "ous": [
        { "id": "ou-ab12-wkld0001", "name": "Workloads", "parent_id": "r-ab12" },
        { "id": "ou-ab12-chk00001", "name": "Checkout",  "parent_id": "ou-ab12-wkld0001" }
      ],
      "accounts": [
        { "id": "222222222222", "name": "chk-prod", "email": "chk-prod@example.com",
          "status": "ACTIVE", "parent_id": "ou-ab12-chk00001" }
      ]
    }
  ]
}
```

- `source` is `aws` or `mock`. Accounts include every status (`ACTIVE`, `SUSPENDED`, …); a
  suspended account can still have cost.
- `ous` and `accounts` are sorted by `id`, so two runs over the same organization produce the
  same file apart from `collected_at`.
- The management account appears in `accounts` with the root as its parent, as AWS reports it.

### OU paths

An OU's **path** is the names from the root down, joined with `/`, starting with `/`:
`/Workloads/Checkout`. The root itself is `/`. An account's path is its parent OU's path. Two
sibling OUs with the same name make the path ambiguous: the inventory package reports finding
`ou-path-ambiguous` (warning) for them, and a path reference to them resolves to nothing.

`internal/inventory` exposes the lookups the resolver needs: an account's chain of OUs from the
root down, an OU by ID, an OU by path, and whether an OU has child OUs.

### AWS access

Read-only, from the management account (or a delegated administrator):
`organizations:DescribeOrganization`, `ListRoots`, `ListOrganizationalUnitsForParent`,
`ListAccountsForParent`. Credentials come from the default AWS chain, or `--profile`. Every call
goes through the retry wrapper in `internal/awsx`, and paging is followed to the end.

## WO-0102-A — AWS Organization discovery and mock

**Assignee:** open
**Depends on:** WO-0101-B
**Packages:** `internal/inventory`, `internal/awsx`, `cmd/inventory*.go`, `testdata/inventory/`

**Goal.** Produce the inventory file from a live AWS Organization or from the mock.

**In scope.**
- The inventory model, JSON reader and writer, path building and the lookups above.
- `internal/awsx`: AWS config loading (`--profile`, `--region`, default `us-east-1` for
  Organizations), and `ExecuteWithRetry` with exponential backoff on throttling and 5xx errors.
- Discovery walking the tree breadth-first with at most 5 concurrent calls.
- `openlz inventory sense [--mock] [--profile P] [--out data/inventory.json]` and
  `openlz inventory show [--in …]`, which prints the tree with each account under its OU.
- The mock: the **demo landing zone** in `testdata/inventory/demo.json`, loaded by `--mock`. It
  has one organization with at least: a `Security` OU with two accounts; a
  `Workloads` OU containing two App OUs, each with `Prod` and `NonProd` child OUs and one account
  in each; a `Shared` OU with one account that serves two Apps; and one account directly under the
  root besides the management account. `WO-0104-B` writes the matching taxonomy.
- Unit tests for path building (including `ou-path-ambiguous`), for the lookups, and for discovery
  against a fake Organizations client (paging across at least two pages).

**Out of scope.** Mapping accounts to Apps (WO-0103-A). Cost (WO-0104-A). Tags on OUs or accounts
(a later release).

**Done when.**
- `openlz inventory sense --mock` writes a file matching the contract, and running it twice gives
  files that differ only in `collected_at`.
- The fake-client test covers paging and a throttling error that succeeds on retry.
- `openlz inventory show --in data/inventory.json` prints every OU path and account of the demo.
- The README lists the four IAM actions.
- `internal/inventory` imports nothing else from `internal/` except `internal/awsx`.

**Verify.** `go run -tags=no_duckdb_arrow ./scripts/check` and
`go run -tags=no_duckdb_arrow . inventory sense --mock && go run -tags=no_duckdb_arrow . inventory show`
