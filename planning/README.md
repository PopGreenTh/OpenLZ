# Planning — how OpenLZ is planned, built and reviewed

Read [`AGENTS.md`](../AGENTS.md) first; this file adds the detail behind its §3 and §4.

## Files

| File | What it is | Who writes it |
|---|---|---|
| [`DECISIONS.md`](DECISIONS.md) | Numbered decisions (`D-NNN`) and open questions (`Q-NNN`) | Claude, on the repo owner's answer |
| [`STATUS.md`](STATUS.md) | One row per Work Order: status, assignee, issue, PR, branch | Claude |
| `DD-NN-<slug>/DELIVERABLE.md` | One release: goal, scope, Work Items, release criteria | Claude |
| `DD-NN-<slug>/WI-NNNN-<Name>.md` | One domain stage, or one piece of work at the same level: Outcome, Contracts, Work Orders | Claude |
| `<package>/testdata/cases/` | Reference cases: input plus expected output, read by tests unchanged (D-017) | Claude |
| [`../docs/taxonomy-spec.md`](../docs/taxonomy-spec.md) | The full target taxonomy (v3) that releases converge on | Claude |

## IDs

- `DD-NN` — Deliverable = one release (`DD-01` = Release 1, `DD-02` = Release 2).
- `WI-NNNN` — Work Item; the first two digits are its Deliverable (`WI-0103` is in DD-01).
- `WO-NNNN-X` — Work Order, a lettered section of its Work Item (`WO-0103-A`).
- IDs are never reused or renumbered.

## Work Order template

```markdown
## WO-NNNN-X — <title>

**Assignee:** open (default) | Antigravity | Cursor | @handle — name one only when it's really needed (D-009)
**Depends on:** WO-… (merged) | —
**Packages:** the only directories this Work Order may change

**Goal.** One sentence.

**In scope.** Bullets.

**Out of scope.** Bullets, each naming the Work Order that owns it.

**Done when.** Checkable bullets. Review checks these and only these.

**Verify.** `go run -tags=no_duckdb_arrow ./scripts/check` (plus any extra command)
```

## Sizing

A Work Order is one PR a reviewer can read in one sitting: roughly 300–1,200 changed lines,
one package area. Bigger → split it, except a pure move
(such as WO-0201-A), which changes no behaviour. Two Work Orders that touch the same package never run at the
same time.

## Status values

| Status | Meaning |
|---|---|
| `not started` | No dispatch issue yet |
| `to do` | Dispatch issue open, not claimed |
| `in progress` | Claimed, or draft PR |
| `in review` | PR ready for review |
| `changes` | Review asked for changes |
| `merged` | PR merged |
| `withdrawn` | Dropped by a decision before it was built |

## Grill before spec

Before Claude writes a new Deliverable or Work Item, or changes a schema or a resolution rule, it
asks the repo owner 3–5 pointed questions per turn and records the answers here as decisions.
Open questions that block a Work Order are listed as `Q-NNN`, and that Work Order is not
dispatched until they are answered.

## Reference cases

Reference cases are the shared contract between implementers. They live in the `testdata/cases/`
folder of the package that reads them (D-017); each case holds the inputs and the expected output. The implementing Work Order's test
reads every case file in place; it never copies or edits them. A case that looks wrong is
reported, and Claude fixes the case and records why.

## Release close-out

When a Deliverable's last Work Order merges, Claude adds a `## Delivered (as built)` section to
each of its Work Items (what was built, where, deviations, and each Acceptance criterion marked
met or not) and tags the release.
