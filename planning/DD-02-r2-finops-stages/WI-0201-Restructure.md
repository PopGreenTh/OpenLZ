# WI-0201 — Restructure to the stage layout

## Outcome

Today's code lives in the D-017 layout and builds as one binary, `openlz-finops`, whose
commands are named by stage. Behaviour, file formats and test results are unchanged: this Work
Item only moves, renames and merges, so later Work Items start from the new layout.

## Contracts

### Layout

```
finops/                 package main: the openlz-finops binary (main.go, version)
finops/sense/           Sense: Organization discovery (today's internal/inventory, minus the AWS client)
finops/enrich/          Enrich: account mapping (today's internal/mapping)
finops/taxonomy/        the taxonomy model, YAML loader and validation (today's internal/taxonomy)
finops/report/          Report: today's `inventory show` output
aws/                    AWS: config, retry wrapper (today's internal/awsx), the Organizations client
                        and the --mock demo organization
scripts/check/          unchanged in purpose
taxonomy/               the example taxonomy YAML, unchanged
```

- `finops/taxonomy/` is a package beside the stages, not a stage: Enrich, Analyze and Enforce
  all read the taxonomy.
- Each package keeps its own fixtures in its own `testdata/`. There is no top-level `testdata/`.
- A package has at most **5** non-test `.go` files. Files may be merged to get there; no
  behaviour may change.
- Each stage package exports `Command() *cobra.Command`; `finops/main.go` adds them to the root
  command. Command code lives in its stage's folder.

### Commands

| Today | After |
|---|---|
| `openlz inventory sense` | `openlz-finops sense accounts` |
| `openlz inventory show` | `openlz-finops report accounts` |
| `openlz taxonomy validate` | `openlz-finops enrich validate` |
| `openlz taxonomy list` | `openlz-finops enrich list` |
| `openlz map` | `openlz-finops enrich map` |
| `openlz version` | `openlz-finops version` |

Flags, defaults, outputs and exit codes stay as they are, except `--demo`'s default (below).
Build: `go build -tags=no_duckdb_arrow -o openlz-finops ./finops`.

### Reference cases

`planning/DD-01-r1-account-mapping/cases/mapping/*` moves to `finops/enrich/testdata/cases/mapping/`
and `planning/DD-01-r1-account-mapping/cases/cost/*` to `finops/enrich/testdata/cases/cost/`,
**byte for byte** (`git mv`; `git diff -M --stat` shows each as a 100% rename). This is the only
change to reference cases this Work Order may make (AGENTS.md §4).

## WO-0201-A — Move to `finops/` and `aws/`, binary `openlz-finops`

**Assignee:** open
**Depends on:** — (WO-0103-A merged)
**Packages:** the whole repository except `planning/`, `docs/`, `AGENTS.md`, `CLAUDE.md`,
`GEMINI.md`; within `planning/`, only the `git mv` of the reference cases above

**Goal.** Reach the layout and command names above with no change in behaviour.

**In scope.**
- Move `cmd/`, `internal/`, `main.go` and `testdata/` into the layout; delete the emptied folders.
- Rename the commands as in the table; the root command is `openlz-finops`.
- The `--mock` demo organization is embedded in the binary (`//go:embed` from `aws/testdata/`),
  so `--mock` works from any directory. `--demo PATH` still overrides it; its default becomes
  "the embedded demo".
- Move the reference cases as above and point the case test at the new folder.
- `scripts/check`, `.github/workflows/check.yml` and `.gitignore` follow the new paths and
  binary name.
- README: new layout, the five stages (D-014) with a one-line job each, the renamed commands,
  and no mention of CloudValueDelta (D-021).

**Out of scope.** JSON Lines, the cache and history (WI-0202). Any new command or flag. Any change
to resolution, validation or discovery behaviour. Any `azure/`, `gcp/` or `oci/` folder (Q-003).

**Done when.**
- `cmd/`, `internal/`, top-level `main.go` and top-level `testdata/` no longer exist.
- Every package is in the layout above and has at most 5 non-test `.go` files.
- The reference cases are 100% renames to `finops/enrich/testdata/cases/` and the case test
  reads them there.
- Every test that existed before still exists and passes; test changes are limited to package
  names, import paths and fixture paths.
- `openlz-finops sense accounts --mock` run from outside the repository writes the same
  inventory as `openlz inventory sense --mock` did (shown in the PR).
- Each command in the table runs under its new name with its old flags.
- README matches the commands, and the repository has no `CloudValueDelta` mention outside
  `planning/DECISIONS.md`.

**Verify.** `go run -tags=no_duckdb_arrow ./scripts/check` and
`go build -tags=no_duckdb_arrow -o /tmp/openlz-finops ./finops && cd /tmp && ./openlz-finops sense accounts --mock --out /tmp/inventory.json && ./openlz-finops report accounts --in /tmp/inventory.json`
