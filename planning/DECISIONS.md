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
- How a multi-App account's cost is shared between its Apps: [D-008](#d-008).
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

## D-008

**A shared account's cost is split by an App tag** (repo owner, 2026-10-26; resolves Q-001).

- The taxonomy names the tag key once: `cost_allocation.app_tag`, default **`App`**. Its value
  is an App code (for example `App=APP-CHK`).
- The tag is read only for accounts that serve two or more Apps. A single-App account is decided
  by its account mapping, whatever its resources are tagged.
- Release 1 stays at account grain: Cost Explorer is grouped by linked account **and** by the App
  tag, in the same call. No resource IDs are read (D-006 holds).
- A tag value that equals the code of one of the account's Apps (ignoring case, D-011) goes to that App
  (`share_rule` `tag`).
- Weights on account entries are withdrawn; the Q-001 proposal is not used.
- The key must be activated as a **cost allocation tag** in AWS Billing. Cost Explorer shows the
  tag only from activation onward, never for earlier spend.
- Edge cases: [D-010](#d-010) (wrong App) and [D-011](#d-011) (untagged, matching).

## D-009

**Work Orders are `open` by default** (repo owner, 2026-10-26).

- A Work Order and its dispatch issue carry `Assignee: open` unless Claude or the repo owner
  really needs a specific implementer for it; then the reason is written next to the name.
- An `open` issue names no agent and addresses none; the first implementer to claim it takes it.
- Supersedes the "proposed assignees" in the DD-01 plan; `WO-0101-A` (#5) was re-dispatched open.

## D-010

**A wrong App tag on a shared account goes to `APP-INVALID`** (repo owner, 2026-10-26; answers
Q-002 item 2).

- `APP-INVALID` is a new system node, beside `APP-UNALLOC`. Untagged cost stays at
  `APP-UNALLOC` (Q-002 item 1); `APP-INVALID` holds cost that **was** tagged, but wrongly.
- A non-empty App tag value that names none of the shared account's Apps goes to `APP-INVALID`
  under the account's Environment, with Project `PRJ-UNALLOC`, BusinessUnit `BU-UNALLOC` and
  CostCenter `CC-UNALLOC`, and `app_rule` `invalid`.
- Two findings tell the cases apart: `shared-tag-wrong-app` when the value is a real App that the
  account doesn't serve, `shared-tag-unknown` when it is no App at all. *Planner's reading:* the
  repo owner named the first case; the second is treated the same way because it is equally a
  wrong tag.
- The suffix `-INVALID` is reserved like `-UNALLOC` and `-UNSPEC`.

## D-011

**Untagged shared-account cost is split equally; App tags match ignoring case** (repo owner,
2026-10-26; answers Q-002 items 1 and 3, closing Q-002).

- An untagged row of a shared account is split equally between the account's Apps, instead of
  going to `APP-UNALLOC`. The finding `shared-untagged` is still raised, so the missing tags are
  visible. The App sorting last by code takes the remainder, so the split adds back up exactly.
- An App tag value matches an App when it equals the App's code ignoring case and surrounding
  spaces (`app-chk` = `APP-CHK`). App names are not matched. This applies to both the D-008
  split and the D-010 wrong-App check.

## Q-002

**Closed** by [D-010](#d-010) (item 2) and [D-011](#d-011) (items 1 and 3). Edge cases of the
App-tag split (D-008): untagged cost, a tag naming an App the account doesn't serve, and matching.
