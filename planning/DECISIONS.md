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
- A tag value that equals the code of one of the account's Apps goes to that App
  (`share_rule` `tag`).
- Weights on account entries are withdrawn; the Q-001 proposal is not used.
- The key must be activated as a **cost allocation tag** in AWS Billing. Cost Explorer shows the
  tag only from activation onward, never for earlier spend.
- Defaults for the rest are in [Q-002](#q-002) until the repo owner confirms them.

## Q-002

**Open, not blocking: defaults applied until answered.** Edge cases of the App-tag split (D-008):

1. **Untagged cost** in a shared account. *Default:* `APP-UNALLOC` under the account's
   Environment, finding `shared-untagged`. *Alternative:* split equally between the account's Apps.
2. **A tag naming a real App that is not on the account's list.** *Default:* treated as invalid,
   `APP-UNALLOC`, finding `shared-tag-invalid`. *Alternative:* accept it when the App has the
   account's Environment, since a shared account decides nothing (spec v3 §6 "the tag wins").
3. **Matching.** *Default:* exact App code after trimming spaces; `app-chk` doesn't match
   `APP-CHK`. *Alternative:* case-insensitive, or also match the App name.
