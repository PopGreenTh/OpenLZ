# OpenLZ – Claude Code guide

Project rules, architecture invariants and commands are shared with other agents in `GEMINI.md`:

@GEMINI.md

## Current work in progress

FinOps cost taxonomy v2 and the Directus integration are partly built. **Before you touch
`internal/taxonomy`, `internal/stages/finops`, `internal/directus` or `directus/`, read
`docs/handoff/finops-taxonomy-v2.md`.** It lists what is done, the ordered remaining tasks with
acceptance checks, and the open questions to ask the user before coding them.

## Commands

```bash
go build -tags=no_duckdb_arrow ./...
go test  -tags=no_duckdb_arrow ./...
go build -tags=no_duckdb_arrow -o /tmp/openlz . && /tmp/openlz pipeline --mock
```

`data/` and `*.duckdb` are generated and gitignored; delete `data/` after manual runs.
