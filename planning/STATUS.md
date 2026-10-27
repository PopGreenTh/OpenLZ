# Status

Written by Claude only. Status values: see [README.md](README.md#status-values).

## Open questions

| Question | Blocks | State |
|---|---|---|
| [Q-003](DECISIONS.md#q-003): provider folders, cost grain, Analyze, Enforce and Report scope | — | closed by D-022 |
| [Q-004](DECISIONS.md#q-004): report append, rollup, spike rule, metric override, taxonomy history | — | closed by D-023 and Q-005 |
| [Q-005](DECISIONS.md#q-005): spike levels, metric override, reading earlier runs | — | closed by D-024 |

## DD-02 — Release 2: FinOps as small pipeable stages

### WI-0201 — Restructure to the stage layout

| Work Order | Assignee | Depends on | Status | Issue | PR | Branch | Revisions |
|---|---|---|---|---|---|---|---|
| [WO-0201-A](DD-02-r2-finops-stages/WI-0201-Restructure.md#wo-0201-a--move-to-finops-and-aws-binary-openlz-finops) | open | — (met) | not started (awaiting go-ahead) | — | — | — | 0 |

### WI-0202 — `finops/sense`

| Work Order | Assignee | Depends on | Status | Issue | PR | Branch | Revisions |
|---|---|---|---|---|---|---|---|
| [WO-0202-A](DD-02-r2-finops-stages/WI-0202-Sense.md#wo-0202-a--records-sense-accounts-as-json-lines-history-store) | open | WO-0201-A | not started | — | — | — | 0 |
| [WO-0202-B](DD-02-r2-finops-stages/WI-0202-Sense.md#wo-0202-b--sense-cost-keyed-cache-and-mock-cost) | open | WO-0202-A | not started | — | — | — | 0 |

### WI-0203 — `finops/enrich`

| Work Order | Assignee | Depends on | Status | Issue | PR | Branch | Revisions |
|---|---|---|---|---|---|---|---|
| [WO-0203-A](DD-02-r2-finops-stages/WI-0203-Enrich.md#wo-0203-a--enrich-map-on-records---as-reported-demo-taxonomy) | open | WO-0202-B | not started | — | — | — | 0 |

### WI-0204 — `finops/analyze`

| Work Order | Assignee | Depends on | Status | Issue | PR | Branch | Revisions |
|---|---|---|---|---|---|---|---|
| [WO-0204-A](DD-02-r2-finops-stages/WI-0204-Analyze.md#wo-0204-a--analyze-rollup-trend-and-changes) | open | WO-0203-A | not started | — | — | — | 0 |

### WI-0205 — `finops/enforce`

| Work Order | Assignee | Depends on | Status | Issue | PR | Branch | Revisions |
|---|---|---|---|---|---|---|---|
| [WO-0205-A](DD-02-r2-finops-stages/WI-0205-Enforce.md#wo-0205-a--the-four-rules) | open | WO-0203-A | not started | — | — | — | 0 |

### WI-0206 — `finops/report`

| Work Order | Assignee | Depends on | Status | Issue | PR | Branch | Revisions |
|---|---|---|---|---|---|---|---|
| [WO-0206-A](DD-02-r2-finops-stages/WI-0206-Report.md#wo-0206-a--report-formats-appending-quickstart) | open | WO-0204-A, WO-0205-A | not started | — | — | — | 0 |


## DD-01 — Release 1: account mapping (closed early, D-018)

| Work Order | Assignee | Status | Issue | PR | Branch | Revisions |
|---|---|---|---|---|---|---|
| [WO-0101-A](DD-01-r1-account-mapping/WI-0101-Foundations-And-Taxonomy.md#wo-0101-a--skeleton-check-script-and-ci) | Antigravity (claimed) | merged | [#5](https://github.com/PopGreenTh/OpenLZ/issues/5) | [#7](https://github.com/PopGreenTh/OpenLZ/pull/7) | `antigravity/wo-0101-a-skeleton` | 0 |
| [WO-0101-B](DD-01-r1-account-mapping/WI-0101-Foundations-And-Taxonomy.md#wo-0101-b--taxonomy-model-yaml-loader-and-validation) | Antigravity (claimed) | merged | [#8](https://github.com/PopGreenTh/OpenLZ/issues/8) | [#9](https://github.com/PopGreenTh/OpenLZ/pull/9) | `antigravity/wo-0101-b-taxonomy` | 1 |
| [WO-0102-A](DD-01-r1-account-mapping/WI-0102-Inventory-Discovery.md#wo-0102-a--aws-organization-discovery-and-mock) | open | merged | [#12](https://github.com/PopGreenTh/OpenLZ/issues/12) | [#15](https://github.com/PopGreenTh/OpenLZ/pull/15) | `cursor/wo-0102-a-inventory` | — |
| [WO-0103-A](DD-01-r1-account-mapping/WI-0103-Account-Mapping.md#wo-0103-a--account-resolver) | open | merged | [#13](https://github.com/PopGreenTh/OpenLZ/issues/13) | [#19](https://github.com/PopGreenTh/OpenLZ/pull/19) | `cursor/wo-0103-a-mapping` | — |
| [WO-0104-A](DD-01-r1-account-mapping/WI-0104-Cost-And-Reports.md) | — | withdrawn (D-018) | — | — | — | — |
| [WO-0104-B](DD-01-r1-account-mapping/WI-0104-Cost-And-Reports.md) | — | withdrawn (D-018) | — | — | — | — |
