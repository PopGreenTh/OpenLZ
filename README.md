# OpenLZ (Open Landing Zone)

OpenLZ is a standalone, single-tenant landing zone governance and FinOps toolkit.

It serves as the proving ground for **CloudValueDelta** ([D-001](planning/DECISIONS.md#d-001)): taxonomy and account-mapping rules proven here are written so that CloudValueDelta can adopt them unchanged. OpenLZ is being rebuilt from the ground up release by release; the code before the rebuild is preserved in git tag `v1-reference` and read for intent only.

## Release 1 Status

Work is currently underway on **DD-01: Release 1 (Account Mapping)**:
- Maps AWS Organization landing zones, nested OU hierarchies, and accounts to **Project › App › Environment** (and to **BusinessUnit › CostCenter**).
- Validates taxonomy YAML schemas offline.
- Allocates monthly AWS Cost Explorer spend across mapped accounts, splitting shared-account spend by App tag ([D-008](planning/DECISIONS.md#d-008)).
- Produces rollup reports, unmapped spend coverage reports, and actionable findings.

Current state: taxonomy validation (WI-0101-B) and AWS Organization inventory discovery with mock demo (WI-0102-A in progress).

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

### Inventory Commands

Collect organization inventory from the demo fixture (offline):
```bash
./openlz inventory sense --mock [--out data/inventory.json]
```

Collect from a live AWS Organization (read-only; requires credentials):
```bash
./openlz inventory sense [--profile PROFILE] [--region us-east-1] [--out data/inventory.json]
```

Print the OU tree and accounts:
```bash
./openlz inventory show [--in data/inventory.json]
```

Live discovery uses these IAM actions on the management account (or a delegated administrator):

- `organizations:DescribeOrganization`
- `organizations:ListRoots`
- `organizations:ListOrganizationalUnitsForParent`
- `organizations:ListAccountsForParent`

### Taxonomy Commands

Validate taxonomy YAML files (offline, exits 1 on any error):
```bash
./openlz taxonomy validate [--dir taxonomy/]
```

Inspect the Project › App › Environment hierarchy tree:
```bash
./openlz taxonomy list [--dir taxonomy/]
```

### Verification and Checks

All builds, vetting, code formatting, and tests are verified via the check script:

```bash
go run -tags=no_duckdb_arrow ./scripts/check
```

## Contributing and Agent Rules

OpenLZ follows a spec-driven dual-agent workflow with strict roles and boundaries. See [AGENTS.md](AGENTS.md) for shared rules, work hierarchy, and development loop instructions.
