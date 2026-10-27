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

## D-012

**Antigravity's models** (repo owner, 2026-10-26).

- Antigravity implements Work Orders with **Gemini 3.8 Flash**; its planning runs on **Gemini 4
  Argon**.
- Both work as Antigravity, so a PR still opens with `Implementer: Antigravity`, and nothing in
  the loop (AGENTS.md §4) changes.

## D-013

**OpenLZ becomes small, pipeable stage commands, one binary per domain; FinOps first** (repo
owner, 2026-10-27).

- Each domain is one binary, `openlz-<domain>`, whose commands are `<stage> <thing>`:
  `openlz-finops sense cost`, `openlz-finops enrich map`. Commands read records on stdin and
  write records on stdout, so they chain with `|`.
- FinOps is the first and only domain built until a later decision adds another (SecOps and
  CloudOps, as in `v1-reference`).
- Supersedes AGENTS.md's "one binary `openlz`". The single-tenant rule (no tenant anywhere)
  holds.

## D-014

**The five stages of `v1-reference`, in the order Sense › Enrich › Analyze › Enforce › Report**
(repo owner, 2026-10-27).

| Stage | Job |
|---|---|
| **Sense** | Read a provider API (or `--mock`) and emit raw records |
| **Enrich** | Add taxonomy context: LZ, Project, App, Environment, BusinessUnit, CostCenter |
| **Analyze** | Group, compare and trend records, including over history (D-016) |
| **Enforce** | Flag records that break a rule; it never changes the cloud |
| **Report** | Turn records into something people read: table, CSV, Markdown |

- Enrich runs before Analyze, unlike `v1-reference`, so analysis can group by App or Project.
- An Enforce flag carries the actionable information needed to fix it: what is wrong, on which
  subject (account, App, …), how much cost it touches, and the action to take. Enforce passes
  every record through and adds its flags; it makes no cloud call that changes anything.

## D-015

**Records between stages are flat JSON Lines; results are cached by their key** (repo owner,
2026-10-27).

- One JSON object per line, flat: no nested objects or arrays. Each record has a `kind` field.
- A stage that calls a provider writes **cache records**, keyed either by the record's unique
  key or by a hash of the call's major parameters (for Cost Explorer: account scope, metric,
  granularity, group-by and month). A later call whose key matches is served from the cache, so
  it costs nothing.
- Report is the only stage that writes anything other than JSON Lines.

## D-016

**History: both restated and as-reported, at least 24 months of cost, saved locally by major
parameters, on a schedule and by hand** (repo owner, 2026-10-27).

- **Restating.** By default, analysis applies the current taxonomy to every period. With
  `--as-reported`, each period uses the snapshot that was current then (taxonomy spec §8.2).
- **Depth.** Cost history goes back at least 24 months. Cost Explorer returns about 14 months
  unless multi-year data is turned on in AWS Billing (up to 38 months), so the repo owner turns
  it on; until then a run reports how many months were actually available.
- **Saving.** Every Sense run saves its output under `data/history/`, in a path named by the run's
  major parameters (the same ones as the D-015 cache key) and dated. Saved files are append-only
  and never rewritten. *Planner's reading:* the D-015 cache and this history are one store.
- **Where.** Local, under `data/` (gitignored). No git, S3 or database server.
- **When.** By hand, and on a schedule with cron. OpenLZ ships an example crontab line; it does
  not install a scheduler.
- Supersedes DD-01's "re-attribution history out of scope".

## D-017

**The repo is laid out as `<domain>/<stage>/` plus one folder per cloud provider, each with its
own `testdata/`** (repo owner, 2026-10-27).

- `finops/sense/`, `finops/enrich/`, `finops/analyze/`, `finops/enforce/`, `finops/report/`.
  `finops/` itself holds the `openlz-finops` binary.
- Provider folders at the top: `aws/`, `azure/`, `gcp/`, `oci/`. They hold the provider
  clients, the retry wrapper and the mocks. *Planner's reading:* a provider folder is created by
  the first Work Order that needs it; DD-02 builds `aws/` only (Q-003).
- No `cmd/` or `internal/`. Every package keeps its fixtures in its own `testdata/`.
- Reference cases live in the `testdata/` of the package that reads them, no longer under
  `planning/`. They stay Claude's: implementers never edit them (AGENTS.md §4). No CODEOWNERS
  rule.

## D-018

**DD-01 stops; the pivot is DD-02, starting with one restructure PR** (repo owner, 2026-10-27).

- DD-01 closes with WO-0101-A, WO-0101-B, WO-0102-A and WO-0103-A merged. WO-0104-A and
  WO-0104-B are withdrawn; their scope moves to DD-02.
- DD-02's first Work Order moves all of today's code into the D-017 layout in one PR, before
  any new stage is built.
- D-005, D-007, D-008, D-010 and D-011 (mapping and App-tag rules) still hold.

## D-019

**Planning keeps its hierarchy and a separate decision log** (repo owner, 2026-10-27).

- Deliverable = one release. A Work Item is one domain stage (`finops/sense`) or one piece of
  work at the same level (the restructure, a provider folder). Work Orders stay as they are.
- `DECISIONS.md` stays its own file.

## D-020

**`CLAUDE.md` and `GEMINI.md` only point to `AGENTS.md`** (repo owner, 2026-10-27). Every rule,
including each agent's duties, lives in `AGENTS.md`.

## D-021

**OpenLZ goes its own way: it is no longer CloudValueDelta's proving ground** (repo owner,
2026-10-27).

- Supersedes the proving-ground part of D-001; the rebuild and the `v1-reference` rule stay.
- From now on nothing in OpenLZ refers to CloudValueDelta, and decisions are not passed on to
  CloudValueDelta-Plan. Earlier decisions keep their wording as history.

## Q-003

**Closed** by [D-022](#d-022). What DD-02's stages need before their Work Items are specified.

1. Provider folders: create only `aws/` now, or empty `azure/`, `gcp/` and `oci/` too?
2. Sense cost grain: monthly only, as in DD-01, or daily too?
3. Analyze in DD-02: which of rollup, trend (month over month), diff (two snapshots) and
   movement (cost that moved because the taxonomy changed)?
4. Enforce in DD-02: which rules? For example unmapped account, untagged shared-account cost,
   wrong App tag, cost spike.
5. Report formats in DD-02: table, CSV and Markdown; is Excel needed now?

## D-022

**DD-02's stages: AWS only, monthly unblended and daily amortized cost, two Analyze commands, every
Enforce rule its own command, Report to table, CSV and Excel** (repo owner, 2026-10-27; answers
Q-003).

- **Providers.** DD-02 builds `aws/` only. `azure/`, `gcp/` and `oci/` are created by the first
  Work Order that needs them.
- **Cost grain and metric.** Monthly cost uses `UnblendedCost`; daily cost uses `AmortizedCost`.
  AWS keeps daily data for about 14 months only, so the 24-month depth of D-016 is monthly.
- **Analyze** has two separate commands: `analyze changes` (what changed between two snapshots
  and the cost that moved with it) and `analyze trend` (cost over time and the impact of each
  change on it). *Planner's reading* of "changes and trend impact as separate things".
- **Enforce.** Every rule is its own command: unmapped account, untagged shared-account cost,
  wrong App tag, cost spike. One run can name several rules.
- **Report** writes a table, CSV or Excel. Markdown is dropped. A report written to a file that
  already exists appends to it instead of replacing it. *Planner's reading* of "table as append";
  Q-004 item 1 confirms it.

## Q-004

**Partly closed** by [D-023](#d-023) (items 1, 2, 5); items 3 and 4 move to [Q-005](#q-005).
Details for Enrich, Analyze, Enforce and Report (planner, 2026-10-27).

1. Report append: does each run add its rows to the end of the same CSV file and the same Excel
   sheet, building one growing table? Or should Excel get a new sheet per run?
2. Rollup (cost summed by Project, App and Environment for one period): part of `analyze trend`,
   or its own command `analyze rollup`?
3. Cost spike: what counts as a spike? Proposal: a month more than 30% and more than 100 USD
   above the average of the three months before it.
4. Metric: always the D-022 metric for the grain, or may `--metric` override it?
5. As-reported history needs the taxonomy as it was. Should Enrich save each taxonomy it uses
   under `data/history/` (recommended), or should OpenLZ read old versions from git?

## D-023

**Report appends to one table; rollup is its own Analyze command; Enrich saves each taxonomy it
uses; a spike has four impact levels against a baseline** (repo owner, 2026-10-27; answers Q-004
items 1, 2, 5 and part of 3).

- **Append.** Each Report run adds its rows to the end of the same CSV file, or the same Excel
  sheet, building one growing table. A header is written only when the file or sheet is new.
- **Rollup** is `analyze rollup`, beside `analyze changes` and `analyze trend`. Commands of the
  same stage pipe into each other like any others (`analyze rollup | analyze trend`).
- **Taxonomy history.** Enrich saves a copy of each taxonomy it uses under `data/history/`,
  named by its hash, and stamps every record it writes with `taxonomy_hash`. As-reported
  analysis (D-016) reads those copies; it never reads git.
- **Cost spike.** A spike is measured against a baseline and graded in **four impact levels** by
  a power factor. The baseline, the factor and the level bounds: Q-005.

## Q-005

**Closed** by [D-024](#d-024). Spike levels, cost metric, and reading earlier runs.

1. Spike: baseline = the average of the 3 months before; factor = amount ÷ baseline; levels by
   powers of 2: level 1 ≥ 1.25×, level 2 ≥ 1.5×, level 3 ≥ 2×, level 4 ≥ 4×; a month less
   than 100 USD above its baseline is never a spike. Right?
2. Metric: fixed by grain (D-022), or `--metric` allowed to override it?
3. Reading earlier runs: may every stage save its output (not only Sense), and may every stage
   read a saved run instead of stdin with `--from-history`?

## D-024

**Spike levels double from the floor; the metric has a default users can change; every stage
saves its output and can read a saved run** (repo owner, 2026-10-27; answers Q-005).

- **Spike.** Baseline = the average of the 3 periods before; factor = amount ÷ baseline. A
  period is a spike only when it is at least the **floor** (default 100 USD) above its baseline.
  From there, four levels by powers of 2: level 1 at a factor ≥ 2, level 2 ≥ 4, level 3 ≥ 8,
  level 4 ≥ 16. *Planner's reading* of "power of 2, 4 from floor"; the floor is a flag.
- **Metric.** The D-022 metric per grain is the default; `--metric` changes it to any Cost
  Explorer cost metric. The metric is part of the cache key.
- **History for every stage.** Every stage command saves what it writes under
  `data/history/finops/<stage>/<thing>/<params>/`, and every command that reads stdin can read
  a saved run instead with `--from-history <stage>/<thing>[@<timestamp>]`.
- Supersedes D-016's planner's reading that cache and history are one store: the provider cache
  is `data/cache/`, stage history is `data/history/` (WI-0202).

## D-025

**The spike floor defaults to 30; `--from-history` can be repeated** (repo owner, 2026-10-27).

- The spike floor (D-024) defaults to **30** in the record's currency, instead of 100.
- A stage may need several kinds of record from different runs; Enrich needs accounts to enrich
  cost. So `--from-history` may be given more than once, each adding one saved run to the
  input, and it may be combined with stdin. `enrich map`'s separate `--accounts` flag is
  dropped (WI-0202, WI-0203).
