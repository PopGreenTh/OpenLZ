# Status

Written by Claude only. Status values: see [README.md](README.md#status-values).

## Before dispatch

| Step | Who | State |
|---|---|---|
| Review and merge the planning set, [PR #4](https://github.com/PopGreenTh/OpenLZ/pull/4) | Repo owner | done (D-009–D-011 follow in a second PR) |
| Answer [Q-001](DECISIONS.md#d-008) (multi-App cost split): by App tag, D-008 | Repo owner | done |
| Answer [Q-002](DECISIONS.md#q-002) (App-tag edge cases): D-010, D-011 | Repo owner | done |
| Push tag `v1-reference` on `e41e24d` | Repo owner | done |

## DD-01 — Release 1: account mapping

### WI-0101 — Foundations and taxonomy

| Work Order | Assignee | Depends on | Status | Issue | PR | Branch | Revisions |
|---|---|---|---|---|---|---|---|
| [WO-0101-A](DD-01-r1-account-mapping/WI-0101-Foundations-And-Taxonomy.md#wo-0101-a--skeleton-check-script-and-ci) | open | — (met) | to do | [#5](https://github.com/PopGreenTh/OpenLZ/issues/5) | — | — | 0 |
| [WO-0101-B](DD-01-r1-account-mapping/WI-0101-Foundations-And-Taxonomy.md#wo-0101-b--taxonomy-model-yaml-loader-and-validation) | open | WO-0101-A | not started | — | — | — | 0 |

### WI-0102 — Inventory discovery

| Work Order | Assignee | Depends on | Status | Issue | PR | Branch | Revisions |
|---|---|---|---|---|---|---|---|
| [WO-0102-A](DD-01-r1-account-mapping/WI-0102-Inventory-Discovery.md#wo-0102-a--aws-organization-discovery-and-mock) | open | WO-0101-B | not started | — | — | — | 0 |

### WI-0103 — Account mapping

| Work Order | Assignee | Depends on | Status | Issue | PR | Branch | Revisions |
|---|---|---|---|---|---|---|---|
| [WO-0103-A](DD-01-r1-account-mapping/WI-0103-Account-Mapping.md#wo-0103-a--account-resolver) | open | WO-0101-B | not started | — | — | — | 0 |

### WI-0104 — Cost allocation and reports

| Work Order | Assignee | Depends on | Status | Issue | PR | Branch | Revisions |
|---|---|---|---|---|---|---|---|
| [WO-0104-A](DD-01-r1-account-mapping/WI-0104-Cost-And-Reports.md#wo-0104-a--cost-explorer-cache-and-allocation) | open | WO-0102-A, WO-0103-A | not started | — | — | — | 0 |
| [WO-0104-B](DD-01-r1-account-mapping/WI-0104-Cost-And-Reports.md#wo-0104-b--reports-openlz-run-and-the-demo) | open | WO-0104-A | not started | — | — | — | 0 |
