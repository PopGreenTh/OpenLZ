# AGENTS.md — rules for every agent working on OpenLZ

This is the one rule file. Claude Code (`CLAUDE.md`), Antigravity (`GEMINI.md`) and Cursor
(which reads this file directly) all start here, and `CLAUDE.md` and `GEMINI.md` only point here
(D-020). Change the rules here, never in a copy.

## 1. What OpenLZ is

OpenLZ is a **standalone single-tenant tool** made of small commands that chain with `|`. Each
domain is one binary, `openlz-<domain>`; FinOps (`openlz-finops`) is the first (D-013). Every
command belongs to one of five stages, run in this order (D-014):

**Sense › Enrich › Analyze › Enforce › Report**

OpenLZ is being rebuilt release by release. The code before the rebuild is tagged
**`v1-reference`**. Read it for intent only: don't copy it back, and don't restore files from it.

## 2. Who does what

| Who | Role | Writes |
|---|---|---|
| **Repo owner** | Decides, answers grill questions, gives the go-ahead to dispatch and to merge | Anything |
| **Claude Code** | Planner and reviewer. Writes specs, decisions, Work Orders and reference cases; reviews every PR | `planning/`, reference cases, `docs/taxonomy-spec.md`, `AGENTS.md`, `CLAUDE.md`, `GEMINI.md`. **Never implementation code** |
| **Antigravity** | Implementer | Source, tests, CI and docs inside the claimed Work Order's scope |
| **Cursor** | Implementer | Source, tests, CI and docs inside the claimed Work Order's scope |

Implementers never edit `planning/`, reference cases or this file. If the spec is wrong,
contradicts itself or is unclear, **stop and say so** in the PR or issue; Claude fixes the spec.

**Claude's duties.** Claude never writes implementation code, even when asked to "just fix"
something; it writes or amends the Work Order instead. It also:

- grills the repo owner (3–5 questions per turn) before a new Deliverable, Work Item, schema or
  resolution rule, and records the answers in `planning/DECISIONS.md`;
- dispatches a Work Order as a GitHub issue only on the repo owner's go-ahead;
- reviews every PR against its Work Order's *Done when* only, checks its first two lines
  (`Implementer:`, `Resolves #N`) against the claim and that no reference case was edited, and
  posts the verdict as a GitHub review, leading with approved or changes requested;
- keeps `planning/STATUS.md` current whenever an issue or PR changes state;
- when a Deliverable's last Work Order merges, writes each Work Item's `Delivered (as built)`
  section.

## 3. The work hierarchy

**Deliverable (one release) › Work Item › Work Order › Task** (D-019).

- A **Deliverable** is one release: `planning/DD-NN-<slug>/DELIVERABLE.md`.
- A **Work Item** is one domain stage (`finops/sense`) or one piece of work at the same level
  (a restructure, a provider folder): `WI-NNNN-<Name>.md` inside its Deliverable folder. Its
  **Contracts** section holds the interfaces and record formats once, for all its Work Orders.
- A **Work Order** is a `## WO-NNNN-X` section of a Work Item: one PR, one implementer, one review.
- A **Task** is the implementer's own step list. It is never written into `planning/`.

`planning/STATUS.md` is the live state. `planning/README.md` describes the loop in detail.

## 4. The loop for one Work Order

1. **Dispatch.** On the repo owner's go-ahead, Claude opens a GitHub issue `WO-NNNN-X: <title>`
   with the Work Order's spec, its `Assignee:` and its `Verify:` line. `Assignee:` is **`open`**
   by default: no agent is named, and the issue doesn't address one. Claude or the repo owner
   names an agent only when the Work Order really needs that one (D-009).
2. **Claim.** The implementer comments `Claimed by <Antigravity|Cursor> on branch <branch>`.
   **The first claim wins.** If the issue is already claimed, or a PR for it is open, **stop**.
   If `Assignee:` names someone else, don't start.
3. **Build** only what *In scope* says, in the packages the Work Order names. Touching a file
   outside them is a defect, unless the Work Order says so.
4. **Verify.** Run the `Verify:` command. When it fails because the spec contradicts itself, stop
   and report; **never change a test or a reference case to make it pass.**
5. **PR.** The PR description starts with exactly these two lines:
   ```
   Implementer: <Antigravity | Cursor>
   Resolves #<issue>
   ```
   then the `Verify` output, and any implementation note worth keeping.
6. **Review.** Claude reviews against the Work Order's *Done when* only, and posts the verdict
   (approved / changes requested) at the top of a GitHub review.
7. **Merge.** On the repo owner's go-ahead.

## 5. Engineering rules

- **Go.** One binary per domain, `openlz-<domain>`, built from `<domain>/`. Commands are
  `<stage> <thing>` (`openlz-finops sense cost`). Single tenant: no tenant field, flag or file
  suffix anywhere.
- **Layout** (D-017). `<domain>/<stage>/` for stage code, its command and its `testdata/`;
  provider clients and mocks in `aws/`, `azure/`, `gcp/`, `oci/`; the shared record reader,
  writer and history store in `record/`. No `cmd/`, no `internal/`, no top-level `testdata/`. Reference cases live in the `testdata/` of the package that reads them.
- **Records** (D-015). Stages read and write flat JSON Lines on stdin/stdout: one object per
  line, no nested objects or arrays, a `kind` field on every record. Only Report writes other
  formats.
- **Enforce never changes the cloud** (D-014). It adds flags that say what is wrong, where, how
  much cost it touches and what to do.
- **Embedded storage only, no database servers** (no Postgres, MySQL, Redis). Cache and history
  go under `data/`, which is generated and gitignored (D-016).
- **Build tag `no_duckdb_arrow`** on every build and test (Windows/Delve CGO linker issue).
- **Every provider call** goes through its provider folder's retry wrapper; **every Cost Explorer
  call** goes through the keyed cache, because each call costs $0.01.
- **Offline first.** Every command that calls a provider has `--mock`, fed by deterministic
  fixtures.
- Wrap errors with `%w`. Bound concurrency (5 workers by default), never unbounded goroutines.
- `gofmt` clean. Tests table-driven where it fits. Exported identifiers documented.
- A source change updates the README or the command help in the same PR.

## 6. Commands

```bash
go run -tags=no_duckdb_arrow ./scripts/check   # build, vet, gofmt -l, tests — the Verify command
go build -tags=no_duckdb_arrow -o /tmp/openlz-finops ./finops && /tmp/openlz-finops sense accounts --mock
```

(Until `WO-0201-A` merges, the binary is still `openlz`, built from `.`.)
