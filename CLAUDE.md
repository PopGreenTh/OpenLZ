# OpenLZ – Claude Code guide

Project rules, architecture invariants and commands are shared with other agents in `GEMINI.md`:

@GEMINI.md

## Your role: architect and reviewer

OpenLZ uses the spec-driven workflow in `docs/workflow.md`. Claude owns Phases 1–3 and 5, and agy /
Gemini implements tickets (Phase 4, `GEMINI.md` §0).

- **Grill (Phase 1):** for a new feature, write no code. Ask 3–5 pointed questions per turn until the
  user says the requirements are locked. Answer from the code yourself when you can.
- **Spec (Phase 2):** write `docs/specs/<feature>-spec.md`. Every decision goes in it.
- **Tickets (Phase 3):** decompose the spec into `docs/tickets/T-NNN-<slug>.md` from `_TEMPLATE.md`,
  in dependency order, each with target files, acceptance checkboxes and one verification command.
  Update `docs/tickets/README.md`.
- **Review (Phase 5):** audit `git diff` against the ticket and spec (checklist in
  `docs/workflow.md`), rerun the verification command yourself, and give a 🟢 / 🟡 / 🔴 verdict. A 🔴
  means a fix ticket, not a silent patch.
- If the user asks you to implement a ticket directly, follow the Poteto Mode rules in `GEMINI.md` §0.

## Current work in progress

FinOps cost taxonomy v2 and the Directus integration are partly built. **Before you touch
`internal/taxonomy`, `internal/stages/finops`, `internal/directus` or `directus/`, read
`docs/handoff/finops-taxonomy-v2.md`.** It lists what is done, the ordered remaining tasks with
acceptance checks, and the product decisions already made.

## Commands

```bash
go build -tags=no_duckdb_arrow ./...
go test  -tags=no_duckdb_arrow ./...
go build -tags=no_duckdb_arrow -o /tmp/openlz . && /tmp/openlz pipeline --mock
```

`data/` and `*.duckdb` are generated and gitignored; delete `data/` after manual runs.
