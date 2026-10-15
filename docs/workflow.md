# Spec-driven dual-agent workflow

OpenLZ work goes through five phases. **Claude** is the architect and the quality gate.
**agy / Gemini** runs the tickets in **Poteto Mode**.

```
Grill ──► Spec ──► Tickets ──► Poteto Mode (one ticket at a time) ──► Review ──► Merge
  ▲                  ▲              │ 3 failed verifications                │ 🔴 blocker
  └── ambiguity      └──────────────┴── escalate (spec/ticket flaw)         └──► fix ticket
```

| Phase | Agent | Input | Output |
|---|---|---|---|
| 1. Grill | Claude | Idea, notes, existing docs, `CLAUDE.md` | Q&A thread; the user signs off |
| 2. Spec | Claude | Resolved Q&A | `docs/specs/<feature>-spec.md` |
| 3. Tickets | Claude | Approved spec | `docs/tickets/T-NNN-<slug>.md` |
| 4. Implement | agy (Poteto) | `GEMINI.md` + **one** ticket | Code and tests, ticket boxes `[x]`, 3-line handoff |
| 5. Review | Claude | `git diff` + spec + tickets | Verdict 🟢 / 🟡 / 🔴 |

## Phase 1: Grill (no code)
- Claude asks 3–5 questions per turn about data schemas, state changes, edge cases, failure modes
  and security boundaries. It reads the code and does not ask about anything the code already shows.
- Decisions are recorded as they are made. Where a feature already has a handoff doc
  (`docs/handoff/`), the decisions go in its "Product decisions" section.
- **Exit:** the user says the requirements are locked.

## Phase 2: Spec
- One file per feature: `docs/specs/<feature>-spec.md`. It is the single source of truth. It covers
  schemas, payloads, state changes, error handling, and the properties the feature must keep.
- The spec opens with a status line (`draft` / `agreed <date>`) and ends with **Open points**.
- Existing spec: [`docs/specs/cost-taxonomy-spec.md`](specs/cost-taxonomy-spec.md) (agreed 2026-10-14).

## Phase 3: Tickets
- Copy [`docs/tickets/_TEMPLATE.md`](tickets/_TEMPLATE.md). Number tickets in dependency order.
- A ticket is **atomic**: one reviewable change, normally under ~400 changed lines, that leaves
  `go build` and `go test` green on its own.
- Every ticket names its **target files**, cites the spec sections it implements, and gives
  acceptance criteria as checkboxes plus one **verification command** that proves them.
- A ticket that would need a judgment call the spec doesn't make is not ready. Go back to Phase 1.
- [`docs/tickets/README.md`](tickets/README.md) is the index: id, title, depends on, status.

## Phase 4: Poteto Mode (agy / Gemini)
Rules are in `GEMINI.md` §0. In short: one ticket, target files only, run the verification command
until it passes, and stop after 3 failed attempts on the same problem.

## Phase 5: Review (Claude)
Claude reads the ticket, the spec sections it cites, and `git diff <base>...HEAD`, then checks:
1. **Spec drift**: the code does what the spec says, no more and no less.
2. **Scope**: every changed file is a target file, or the handoff explains why it isn't.
3. **Invariants**: the rules in `GEMINI.md` §2 (DuckDB only, AWS client layering, the 5 stages,
   `DailyAmount`, `Schema` as the only taxonomy definition).
4. **Correctness**: edge cases, error wrapping (`%w`), bounded concurrency, no secrets in logs.
5. **Tests**: the acceptance criteria are covered by tests, and the verification command passes when
   Claude runs it again.

Verdict:
- 🟢 **Approved**: ready to merge.
- 🟡 **Approved with nits**: merge; the nits go into the next ticket that touches those files.
- 🔴 **Blocker**: Claude writes `T-NNN-fix-<slug>.md` (or reopens the ticket with a `## Review`
  section) and the work goes back to Poteto Mode.

## Directory layout
```
docs/
  workflow.md                 # this file
  specs/<feature>-spec.md     # Phase 2
  tickets/README.md           # ticket index and status
  tickets/_TEMPLATE.md
  tickets/T-NNN-<slug>.md     # Phase 3
  handoff/                    # longer context for a feature in progress
```
