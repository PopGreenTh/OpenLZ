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
keyed cache), D-016 (history), D-017 (layout), D-018 (DD-01 stops), D-019 (planning), D-022
(providers, cost grain, Analyze, Enforce, Report), D-023 (append, rollup, taxonomy history),
D-024 (spike levels, metric, history for every stage). The
mapping and App-tag rules of DD-01 still hold (D-005, D-007, D-008, D-010, D-011).

## Scope

**In:**
- The D-017 layout, reached in one restructure PR (WI-0201).
- FinOps, AWS only. One Work Item per stage:
  - Sense: accounts (AWS Organizations) and cost (Cost Explorer: monthly unblended for at least
    24 months, daily amortized), with the keyed cache.
  - Enrich: map accounts and cost to the taxonomy.
  - Analyze: `rollup`, `changes` and `trend`, over saved history, restated or as reported.
  - Enforce: one command per rule (unmapped account, untagged shared cost, wrong App tag, cost
    spike), flags with actionable information.
  - Report: table, CSV and Excel, appending to an existing file.
- Provider cache under `data/cache/`, every stage's output saved under `data/history/` by
  major parameters, `--from-history` on every stage that reads stdin; an example crontab line.

**Out (later releases):** SecOps and CloudOps; Azure, GCP and OCI; anything that changes the
cloud; Markdown, Power BI and REST; any UI; currency conversion; resource-level attribution
(D-006).

## Work Items

| Work Item | Capability | Work Orders | State |
|---|---|---|---|
| [WI-0201](WI-0201-Restructure.md) | Move today's code to the D-017 layout; `openlz-finops` | A | specified |
| [WI-0202](WI-0202-Sense.md) | `finops/sense`: accounts and cost; the shared record rules and history store | A, B | specified |
| [WI-0203](WI-0203-Enrich.md) | `finops/enrich`: map records to the taxonomy, App-tag split | A | specified |
| [WI-0204](WI-0204-Analyze.md) | `finops/analyze`: `rollup`, `changes` and `trend` | A | specified |
| [WI-0205](WI-0205-Enforce.md) | `finops/enforce`: one command per rule | A | specified |
| [WI-0206](WI-0206-Report.md) | `finops/report`: table, CSV, Excel, append | A | specified |

WI-0202 defines the record rules and the two stores (D-015, D-016, D-024) that every later
stage follows.

## Order

```
WO-0201-A ─► WO-0202-A ─► WO-0202-B ─► WO-0203-A ─┬─► WO-0204-A ─┬─► WO-0206-A
                                                  └─► WO-0205-A ─┘
```

WO-0204-A and WO-0205-A run in parallel; they touch different packages.

## Release criteria

1. The repo matches D-017: no `cmd/`, no `internal/`, no reference case under `planning/`.
2. On a clean clone with no AWS credentials, the pipeline above with `--mock` prints a report and
   exits 0.
3. Every reference case passes, unchanged.
4. A second run over closed months makes zero Cost Explorer calls.
5. Over two saved organization and taxonomy snapshots, `enrich map` gives restated results and
   `enrich map --as-reported` gives as-reported ones, and `analyze trend` runs on both.
6. Against a real AWS Organization (repo owner's run), the pipeline completes with read-only
   permissions listed in the README.
