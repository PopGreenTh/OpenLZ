# T-NNN: <short imperative title>

- **Status:** todo | in-progress | done | blocked
- **Spec:** `docs/specs/<feature>-spec.md` §x.y
- **Depends on:** T-NNN, … (or none)

## Goal
One or two sentences: what changes and why.

## Target files
Only these files may change. Anything else is out of scope: stop and escalate.
- `path/to/file.go`
- `path/to/file_test.go`

## Steps
1. …

## Acceptance criteria
- [ ] …
- [ ] `go build -tags=no_duckdb_arrow ./...` and `go test -tags=no_duckdb_arrow ./...` pass

## Verification command
```bash
go test -tags=no_duckdb_arrow ./internal/<pkg>/... -run '<TestName>' -v
```

## Out of scope
- Things a reasonable implementer might be tempted to do here, and must not.

## Handoff
<!-- Filled in by the implementer when done: 3 lines. -->
<!-- 1. What changed. 2. How it was verified. 3. Anything the reviewer should look at. -->
