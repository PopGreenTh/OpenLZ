# Decisions

Newest last. A decision is changed only by a later decision that says so.

## D-001

**OpenLZ is rebuilt from the ground up, standalone and as CloudValueDelta's proving ground**
(repo owner, 2026-10-25).

- OpenLZ is a standalone, single-tenant tool in its own right, and the place where taxonomy and
  mapping rules are proven before CloudValueDelta adopts them.
- The rebuild happens in this repository. The pre-rebuild code is tagged `v1-reference` and is
  read for intent only.
- The taxonomy target stays `docs/taxonomy-spec.md` (v3); each release builds a subset of it.

## D-002

**Four co-creators with fixed roles** (repo owner, 2026-10-25).

- Repo owner decides and approves; Claude Code plans and reviews, and never writes implementation
  code; Antigravity and Cursor implement.
- Planning lives in this repository under `planning/` (no separate planning repo, no ClickUp).
- One shared rule file, `AGENTS.md`; `CLAUDE.md` and `GEMINI.md` point to it.
- Dispatch by GitHub issue, claim by comment, PR opens with `Implementer:` and `Resolves #N`
  (the pattern proven in CloudValueDelta-Plan, D-054 and D-079 there).

## D-003

**Work is organized by release** (repo owner, 2026-10-25): Deliverable (one release) › Work Item ›
Work Order. Release 1 is `DD-01`.

## D-004

**The taxonomy is edited as YAML files in git, through PRs** (repo owner, 2026-10-25).

- The taxonomy lives in `taxonomy/*.yaml`; a change is a PR, so review and history come from git.
- Directus is not part of Release 1. A later editing surface must read and write the same files
  or a format converted losslessly from them.

## D-005

**An account maps to exactly one Environment, and may serve several Apps** (repo owner,
2026-10-25).

- An account has at most one Environment code (for example `ENV-PROD`). Environments stay under
  their App (spec v3 §5), so that code must be defined under **every** App the account serves.
- An account may serve one or more Apps, possibly of different Projects. Its resolved result is
  one allocation per App, all with the same Environment code.
- How a multi-App account's cost is shared between its Apps: see [Q-001](#q-001).
- This supersedes spec v3's "an Account can host several Apps" without an Environment rule, and
  matches CloudValueDelta D-081's one-Environment-per-account rule while relaxing its
  one-App-per-account rule.

## D-006

**Release 1 attributes cost per account; resource-level attribution comes later** (repo owner,
2026-10-25).

- Release 1 reads Cost Explorer spend grouped by linked account and allocates it through the
  account mapping. No tags, resources, regions, services or usage types are read.
- A later release adds resource- and tag-level attribution (spec v3 §6). Release 1's account
  mapping becomes the "account evidence" step of that attribution, so it must not be thrown away.

## D-007

**Release 1 adds the OU level and the LZ link from CloudValueDelta D-075/D-081** (planner,
2026-10-25, following the repo owner's Release 1 scope "LZ, OU, Account → Project, App,
Environment").

- The OU tree is discovered, nested to any depth, and never flattened.
- An OU may link to an App (only an OU that has child OUs) and may set an Environment code (any
  OU). Both are inherited down the tree; the deepest OU that says something wins.
- An OU is referenced by ID (`ou-…`) or by name path (`/Workloads/Checkout`); a reference that
  starts with `/` is a path. Path patterns are not in Release 1.
- Resolution order for each of App and Environment, separately: account entry › deepest OU ›
  LZ default › global default › system node.
- Release 1 has no `SHP`/`SHN` system Environments: a shared account is an account that serves
  several Apps (D-005).

## Q-001

**Open — blocks `WO-0104-A`.** How is a multi-App account's cost shared between its Apps?

Planner's proposal, used in the reference cases until answered:

- optional `weight` per App on the account entry; weights are all given or all omitted;
- with weights, share = weight ÷ sum of weights; without, equal shares;
- the allocation rule records which (`account-weighted` or `account-equal`);
- to keep totals exact, the App that sorts last by code takes the remainder after rounding.

Alternatives: keep the cost at `APP-UNALLOC` under the account's Environment until resource-level
attribution exists; or require weights for every multi-App account.
