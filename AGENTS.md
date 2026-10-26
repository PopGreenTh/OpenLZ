# AGENTS.md — rules for every agent working on OpenLZ

This is the one shared rule file. Claude Code (`CLAUDE.md`), Antigravity (`GEMINI.md`) and Cursor
(which reads this file directly) all start here. Change the rules here, never in a copy.

## 1. OpenLZ is being rebuilt

OpenLZ is being rebuilt from the ground up, in this repository, release by release. The code before
the rebuild is tagged **`v1-reference`**. Read it for intent only: don't copy it back, and don't
restore files from it. Release 1 deletes it from `main` (`WO-0101-A`).

OpenLZ is a **standalone single-tenant tool** and the **proving ground for CloudValueDelta**
([D-001](planning/DECISIONS.md#d-001)): a taxonomy rule proven here is written so that
CloudValueDelta can adopt it unchanged.

## 2. Who does what

| Who | Role | Writes |
|---|---|---|
| **Repo owner** | Decides, answers grill questions, gives the go-ahead to dispatch and to merge | Anything |
| **Claude Code** | Planner and reviewer. Writes specs, decisions, Work Orders and reference cases; reviews every PR | `planning/`, `docs/taxonomy-spec.md`, `AGENTS.md`, `CLAUDE.md`, `GEMINI.md`. **Never implementation code** |
| **Antigravity** | Implementer | Source, tests, CI and docs inside the claimed Work Order's scope |
| **Cursor** | Implementer | Source, tests, CI and docs inside the claimed Work Order's scope |

Implementers never edit `planning/` or this file. If the spec is wrong, contradicts itself or is
unclear, **stop and say so** in the PR or issue; Claude fixes the spec.

## 3. The work hierarchy

**Deliverable (one release) › Work Item › Work Order › Task.**

- A **Deliverable** is one release: `planning/DD-NN-<slug>/DELIVERABLE.md`. DD-01 is Release 1.
- A **Work Item** is one capability: `WI-NNNN-<Name>.md` inside its Deliverable folder. Its
  **Contracts** section holds the interfaces and file formats once, for all its Work Orders.
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
3. **Build** only what *In scope* says, in the packages the Work Item names. Touching a file
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

## 5. Engineering rules (carried from v1)

- **Go**, one binary `openlz`. Single tenant: no tenant field, flag or file suffix anywhere.
- **Embedded DuckDB, no database servers** (no Postgres, MySQL, Redis). Data goes to Parquet under
  `data/`, which is generated and gitignored.
- **Build tag `no_duckdb_arrow`** on every build and test (Windows/Delve CGO linker issue).
- **Every AWS call** goes through one retry wrapper; **every Cost Explorer call** goes through the
  local cache, because each call costs $0.01.
- **Offline first.** Every command that calls AWS has `--mock`, fed by deterministic fixtures.
- Wrap errors with `%w`. Bound concurrency (5 workers by default), never unbounded goroutines.
- `gofmt` clean. Tests table-driven where it fits. Exported identifiers documented.
- A source change updates the README or the command help in the same PR.

## 6. Commands

```bash
go run -tags=no_duckdb_arrow ./scripts/check   # build, vet, gofmt -l, tests — the Verify command
go build -tags=no_duckdb_arrow -o /tmp/openlz . && /tmp/openlz run --mock
```

(Until `WO-0101-A` merges, `scripts/check` doesn't exist and the tree is still v1.)
