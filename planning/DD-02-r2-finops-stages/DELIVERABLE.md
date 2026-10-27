# DD-02 — Release 2: FinOps as small pipeable stages

## Goal

A FinOps analyst chains small commands of one binary, `openlz-finops`, with `|`, from the AWS
Organization and Cost Explorer to a readable report, and can look back over at least 24 months:
restated with today's taxonomy, or as it was reported then.

```bash
openlz-finops sense accounts --mock | openlz-finops enrich map \
  | openlz-finops analyze rollup --by app | openlz-finops report table
```

## Decisions

D-013 (pivot, binary per domain), D-014 (five stages and their order), D-015 (flat JSON Lines,
keyed cache), D-016 (history), D-017 (layout), D-018 (DD-01 stops), D-019 (planning). The
mapping and App-tag rules of DD-01 still hold (D-005, D-007, D-008, D-010, D-011).

## Scope

**In:**
- The D-017 layout, reached in one restructure PR (WI-0201).
- FinOps, AWS only. One Work Item per stage:
  - Sense: accounts (AWS Organizations) and cost (Cost Explorer, at least 24 months), with the
    keyed cache.
  - Enrich: map accounts and cost to the taxonomy.
  - Analyze: over the current run and over saved history, restated or as reported.
  - Enforce: flags with actionable information.
  - Report: table, CSV and Markdown.
- History saved under `data/history/` by major parameters; an example crontab line.

**Out (later releases):** SecOps and CloudOps; Azure, GCP and OCI; anything that changes the
cloud; Excel, Power BI and REST; any UI; currency conversion; resource-level attribution
(D-006).

## Work Items

| Work Item | Capability | Work Orders | State |
|---|---|---|---|
| [WI-0201](WI-0201-Restructure.md) | Move today's code to the D-017 layout; `openlz-finops` | A | specified |
| WI-0202 | `finops/sense`: accounts and cost, keyed cache, history saving | — | blocked by Q-003 |
| WI-0203 | `finops/enrich`: map records to the taxonomy, App-tag split | — | blocked by Q-003 |
| WI-0204 | `finops/analyze`: rollup and history analysis | — | blocked by Q-003 |
| WI-0205 | `finops/enforce`: flags | — | blocked by Q-003 |
| WI-0206 | `finops/report`: table, CSV, Markdown | — | blocked by Q-003 |

WI-0202 to WI-0206 are written once Q-003 is answered. WI-0202 also writes the shared record and
cache contract (D-015, D-016) that the other stages read.

## Order

```
WO-0201-A ─► WI-0202 ─► WI-0203 ─┬─► WI-0204 ─┐
                                 └─► WI-0205 ─┴─► WI-0206
```

## Release criteria

1. The repo matches D-017: no `cmd/`, no `internal/`, no reference case under `planning/`.
2. On a clean clone with no AWS credentials, the pipeline above with `--mock` prints a report and
   exits 0.
3. Every reference case passes, unchanged.
4. A second run over closed months makes zero Cost Explorer calls.
5. `analyze` gives both restated and as-reported results over at least two saved snapshots.
6. Against a real AWS Organization (repo owner's run), the pipeline completes with read-only
   permissions listed in the README.
