# DD-01 — Release 1: LZ, OU and Account mapped to Project, App and Environment

## Goal

A FinOps analyst describes their Projects, Apps and Environments in YAML, links them to the
landing zone's OUs and accounts, and gets every account's monthly AWS spend allocated to
Project › App › Environment (and to BusinessUnit › CostCenter through the Project and App). Every
account is accounted for: anything that cannot be mapped is shown explicitly as `UNALLOC` or
`UNSPEC`, with a finding that says why.

## Scope

**In:**
- Discover an AWS Organization: the LZ, its nested OU tree and its accounts (live or `--mock`).
- Taxonomy in YAML (D-004): BusinessUnit, CostCenter, Project, App, Environment, LZ, OU links,
  account entries, defaults.
- Validation of the taxonomy on its own (no AWS needed), usable in CI on a taxonomy PR.
- Account mapping: each account to one Environment and one or more Apps (D-005, D-007), with the
  rule that decided each level.
- Monthly Cost Explorer spend per linked account, allocated through the mapping and rolled up;
  a shared account's spend is split by the App tag (D-008).
- Reports: rollup, coverage (what is unmapped and how much it costs), findings. Console, CSV and
  Parquet.
- `openlz run --mock`: the whole flow offline, on a demo landing zone.

**Out (later releases):** tags other than the App tag on shared accounts; resources and resource groups; Region/Zone; ServiceCat/Service and
ServiceUsage/UsageType; Feature and Function; OwnerTeam, ProductLine and Customer; alias table;
OU path patterns; budgets; re-attribution history (spec v3 §8); Directus or any UI; clouds other
than AWS; currency conversion.

## Work Items

| Work Item | Capability | Work Orders |
|---|---|---|
| [WI-0101](WI-0101-Foundations-And-Taxonomy.md) | Repo skeleton, check script, CI; taxonomy model, YAML loader, validation | A, B |
| [WI-0102](WI-0102-Inventory-Discovery.md) | AWS Organization discovery and mock inventory | A |
| [WI-0103](WI-0103-Account-Mapping.md) | Resolve every account to Project › App › Environment | A |
| [WI-0104](WI-0104-Cost-And-Reports.md) | Per-account cost, allocation, rollup, reports, `openlz run` | A, B |

## Order

```
WO-0101-A ─► WO-0101-B ─┬─► WO-0102-A ─┐
                        └─► WO-0103-A ─┴─► WO-0104-A ─► WO-0104-B
```

`WO-0102-A` and `WO-0103-A` run in parallel; they meet only through the inventory contract in
WI-0102, and `WO-0103-A`'s reference cases carry their own inventory.

## Release criteria

1. `openlz run --mock` on a clean clone prints the rollup, coverage and findings for the demo
   landing zone, with no AWS credentials.
2. Every reference case in `cases/` passes.
3. For every month, metric and currency, allocated cost equals Cost Explorer cost (to within
   `1e-6`).
4. A taxonomy PR with an error fails `openlz taxonomy validate` in CI.
5. Against a real AWS Organization (repo owner's run), `openlz run` completes with read-only
   permissions listed in the README.

## Assignees

Every Work Order is `open`: the first implementer to claim it takes it (D-009). Only
`WO-0102-A` and `WO-0103-A` run at the same time, and they touch different packages.
