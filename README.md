# OpenLZ (Open Landing Zone)

OpenLZ is a standalone, single-tenant landing zone governance and FinOps toolkit.

It serves as the proving ground for **CloudValueDelta** ([D-001](planning/DECISIONS.md#d-001)): taxonomy and account-mapping rules proven here are written so that CloudValueDelta can adopt them unchanged. OpenLZ is being rebuilt from the ground up release by release; the code before the rebuild is preserved in git tag `v1-reference` and read for intent only.

## Release 1 Status

Work is currently underway on **DD-01: Release 1 (Account Mapping)**:
- Maps AWS Organization landing zones, nested OU hierarchies, and accounts to **Project › App › Environment** (and to **BusinessUnit › CostCenter**).
- Validates taxonomy YAML schemas offline.
- Allocates monthly AWS Cost Explorer spend across mapped accounts, splitting shared-account spend by App tag ([D-008](planning/DECISIONS.md#d-008)).
- Produces rollup reports, unmapped spend coverage reports, and actionable findings.

Current state: foundations, repo skeleton, and CI check script (WI-0101).

## Building and Running

### Prerequisites

- Go 1.27+ (or Go toolchain managing the version in `go.mod`)

### Build

```bash
go build -tags=no_duckdb_arrow -o openlz .
```

### Version

```bash
./openlz version
```

### Verification and Checks

All builds, vetting, code formatting, and tests are verified via the check script:

```bash
go run -tags=no_duckdb_arrow ./scripts/check
```

## Contributing and Agent Rules

OpenLZ follows a spec-driven dual-agent workflow with strict roles and boundaries. See [AGENTS.md](AGENTS.md) for shared rules, work hierarchy, and development loop instructions.
