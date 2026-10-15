# Cost Hierarchy Taxonomy — Specification v3 (three hierarchies)

> Status: **agreed design** (2026-10-14). It replaces the single Project hierarchy of v2 with three
> hierarchies — **Biz**, **Project**, **Cloud** — and adds the mechanism that keeps attributions
> current (CloudValueDelta, §8). OpenLZ supports a **single tenant**. Implementation status and
> remaining work: `docs/handoff/finops-taxonomy-v2.md`. Attribution engine: `internal/attribution`.

## 1. Notation

- **Code / Name.** Every item has a **Code** (stable business key, managed by people) and a **Name**
  (display text). Codes are unique as described in §5. Names may repeat.
- **`Group/Member`** is one level with two parts. The *member* is what appears on cost data
  (CSPAccount, Zone, Service, UsageType, Resource); the *group* is the business label it maps to
  (Account, Region, ServiceCat, ServiceUsage, ResourceGroup).
- **Cost-data codes.** A level that appears on cost data uses the cloud's own value as its Code:
  CSP account id `111122223333`, Zone `ap-southeast-1a`, Service `AmazonEC2`, UsageType
  `BoxUsage:t3.medium`, Resource id (instance id or ARN).
- **Valid hash.** Every valid path is written `|Level1|Level2|…|`, for example
  `|PRJ-PAY|APP-GATEWAY|ENV-PROD|FEAT-CARD|FUNC-AUTH|`. Every prefix of a valid path is valid too.

## 2. The three hierarchies

| Hierarchy | Question it answers | Levels |
|---|---|---|
| **Biz** | Who pays? | BusinessUnit → CostCenter → OwnerTeam → ProductLine → Customer |
| **Project** | What do we build? | Project → App → Environment → Feature → Function |
| **Cloud** | Where does it run and how is it billed? | LZ/Connection → Account/CSPAccount → Region/Zone → ServiceCat/Service → ServiceUsage/UsageType → ResourceGroup/Resource |

- **LZ** is a Landing Zone, a cloud organization the company runs (e.g. an AWS Organization).
  **Connection** is one billing source inside it (e.g. an AWS payer account). An LZ has exactly one
  **CSP** and one **billing currency**, both stored as attributes of the LZ (§3.4). CSP is not a level.
- **Region / Zone** sits below Account because one Account runs in many Regions. A Zone is an
  availability zone inside a Region. Both come from the cost row.
- **Environment** belongs to one App, so an Environment Code only has to be unique within its App.

```
Biz                    Project                  Cloud
BusinessUnit      ◄──  Project          ◄·····► LZ / Connection   (CSP, currency)
CostCenter        ◄──  App              ◄·····► Account / CSPAccount
OwnerTeam         ◄··  Environment      ◄·····► Region / Zone
ProductLine       ◄··  Feature                  ServiceCat / Service
Customer          ◄··  Function                ServiceUsage / UsageType
                                                ResourceGroup / Resource ····► Project path
──►  must ref     ··►  can ref      ◄·····►  many-to-many
```

## 3. Relations

Relation types: **is under** (hierarchy parent, required), **must ref** (required link),
**can ref / can be mapped** (optional link).

### 3.1 Project → Biz, level by level
| Project level | Relation | Biz level |
|---|---|---|
| Project | must ref | BusinessUnit |
| App | must ref | CostCenter |
| Environment | can ref | OwnerTeam |
| Feature | can ref | ProductLine |
| Function | can ref | Customer |

The references along any Project path must form a valid Biz path (§5 check). This is why a Biz path
can always be derived from a Project path.

### 3.2 Cloud ↔ Project, many-to-many at aligned levels
| Project level | Cloud level | Meaning |
|---|---|---|
| Project | LZ / Connection | A Project can run in several LZs; an LZ hosts many Projects |
| App | Account | An App can use several Accounts; an Account can host several Apps (shared account) |
| Environment | Region | Narrows an App to an Environment (weak evidence on its own) |
| Feature, Function | — | No Cloud counterpart: reached only through ResourceGroup/Resource mappings |
| (any level) | ResourceGroup / Resource | A Resource or ResourceGroup can be mapped to a Project path at any depth |

**Every Resource must carry Project tags or be mapped to a Project path.** When neither is possible
the cost goes to the explicit `UNALLOC` bucket (§4), never silently dropped.

### 3.3 Cloud value mappings
| From (member) | Can be mapped to (group) |
|---|---|
| CSPAccount id/name | Account code/name |
| Zone id/name | Region code/name |
| Service id/name | ServiceCat code/name |
| UsageType | ServiceUsage |
| Resource id/name | ResourceGroup code/name |

An **alias table** maps invalid tag values to the correct code (e.g. a mistyped App code), per level.
Each alias row has the invalid value as its Code, a `level` (`project`, `app`, `environment`, `feature`,
`function`, `business_unit`, `cost_center`, `owner_team`, `product_line`, `customer`) and a `target`: a Project
path hash of that depth for Project levels, a Biz code for Biz levels. An alias value is unique within its level.

### 3.4 Attributes
| Level | Attribute | Required | Values |
|---|---|---|---|
| LZ | `csp` | yes | Fixed list: `AWS`, `Azure`, `GCP`, `OCI` |
| LZ | `currency` | yes | ISO 4217, e.g. `USD`, `THB` |
| Region | `csp` | no | Same list; a Region Code is unique only within its CSP |

### 3.5 Consistency checks (warnings, never errors)
- The Biz refs of a Project path must form a valid Biz path (an App's CostCenter must be under the
  Project's BusinessUnit, and so on down the levels).
- A cost row's Connection currency must equal the row's currency.
- A cost row's Region should have the same CSP as its LZ.
- A cost row's Zone should belong to its Region.
- An Account's Connection should be consistent with the LZs of the Projects it serves.

## 4. System nodes: UNSPEC and UNALLOC

- **`<LEVEL>-UNSPEC`** pads levels the evidence could not reach below a resolved path, e.g.
  `|PRJ-PAY|APP-GATEWAY|ENV-PROD|FEAT-UNSPEC|FUNC-UNSPEC|`. The Biz refs of UNSPEC nodes are
  `PL-UNSPEC` and `CUST-UNSPEC`, so BU, CostCenter and OwnerTeam are still attributed and the
  gap is visible to the product owner.
- **`<LEVEL>-UNALLOC`** is used when nothing resolves at all: `|PRJ-UNALLOC|APP-UNALLOC|…|`.
- Both are generated by the system for every valid path; people never maintain them.

## 5. Uniqueness

| Level | Unique within |
|---|---|
| BusinessUnit, CostCenter, OwnerTeam, ProductLine, Customer, Project, App | its level |
| Environment, Feature, Function | its parent (Environment per App, and so on) |
| Region | its CSP |
| Zone | its Region |
| Resource id | all Resources |
| Every other Cloud level | its level |

Names may repeat. Because codes below App repeat, lookups use **nodes of the tree** (a full path),
never a bare code.

## 6. Attribution

Each cost line is attributed to one Project path, and its Biz path is derived from the Project path.

1. **Alias.** Invalid tag values are mapped through the alias table.
2. **Override.** An explicit Resource → Project path mapping always wins.
3. **Tag anchor.** Every valid Project path is scored against the line's tags; deeper levels weigh
   more (Project 1, App 2, Environment 4, Feature 8, Function 16), so **the deepest valid tag wins**.
   A wrong upper tag is corrected and the line is flagged `tag-corrected`.
4. **Evidence refines the anchor**, most specific first: ResourceGroup, Biz tag, Account, Region.
   Evidence that contradicts what is already known is **ignored and flagged**, never applied.
5. **LZ** is used as a last resort, only when nothing more specific is known (an LZ hosts many
   Projects, so it must not settle an ambiguous shared Account).
6. **Common ancestor.** A level survives only if every remaining candidate agrees on it.
7. **Padding.** Missing levels become `UNSPEC`, or `UNALLOC` when nothing resolved.

The result is always built from real nodes of the tree, never by joining independently resolved
codes, so it cannot be a path that does not exist.

**Examples (reference data in `internal/attribution/testdata`)**
| Line | Result |
|---|---|
| All tags valid | Kept as tagged |
| Only the Function tagged | Parents walked up from the Function |
| Typo in a middle tag (`APP-CHKKK`) | Repaired from the valid Function |
| Valid Project tag, wrong App from another branch | Deepest tag wins, flagged `tag-corrected` |
| Shared account, one valid App tag | The App tag wins; the account's other App is dropped |
| Shared account, untagged | Unallocated (LZ does not decide) |
| Valid tags but ResourceGroup maps to another App | Tags kept; ResourceGroup ignored and flagged |
| Dedicated account, no tags | App from the Account, Environment from the Region, rest `UNSPEC` |
| Resource override | The override path, whatever the tags say |

## 7. Currency

- Every LZ has one billing currency; every Connection in it bills in that currency.
- A cost row whose currency differs from its LZ's currency is a data error and is reported.
- Amounts in different currencies are never summed as-is. A roll-up across LZs converts to one
  reporting currency first, or shows each currency separately.

## 8. Keeping attributions current: CloudValueDelta

**CloudValueDelta** is the set of Cloud-side changes since the last run. It decides what is
re-attributed, so history is never recomputed blindly and nothing is lost when mappings change.

### 8.1 Two kinds of change
| Kind | Examples | Detected by |
|---|---|---|
| **Data delta** | New cost lines; AWS restating recent days; a line's tags or cloud keys changed; a cost amount changed | Fingerprints of each line |
| **Mapping delta** | A Directus edit: new resource override, Account ↔ App link, new Environment, moved or removed node | Comparing the previous and new taxonomy snapshot |

### 8.2 Stored per line
- `input_hash`: hash of everything attribution reads (Project and Biz tags, Connection, Account,
  Region, Zone, Service, UsageType, Resource id, ResourceGroup, currency).
- `amount`: kept separately, because an amount change needs re-aggregation but not re-attribution.
- `taxonomy_version`: hash of the snapshot the result was computed with.
- Current result: `project_hash`, `biz_hash`, `trail`; previous result: `prev_project_hash`,
  `prev_biz_hash`; `resolved_at`, `changed_at`.

An append-only **attribution history** (line key, taxonomy version, hashes, valid from) keeps every
earlier result so a report can show "as reported then" next to "as restated now".

### 8.3 Update steps
1. **Sense** merges new data by business key (latest record wins) and records, per line, one delta
   row: `NEW`, `AMOUNT` (old and new amount), `KEYS` (input hash changed) or `REMOVED`.
2. **Pull** from Directus computes the new `taxonomy_version` and diffs the snapshot: changed
   nodes, evidence and alias keys form the **changed key set**.
3. **Select** the lines to re-attribute:
   - every line with a `NEW` or `KEYS` delta;
   - every line whose inputs match a changed key;
   - every line whose current path uses a node that was removed, renamed or moved;
   - every line currently `UNSPEC` or `UNALLOC` when nodes or evidence were added (they may improve).
   If the node set itself changed, re-resolve all distinct input combinations; this is cheap
   because resolution runs on distinct combinations and is joined back.
4. **Re-attribute** the selected lines with `internal/attribution` and write the new result; the
   old one moves to `prev_*` and to the history.
5. **Restate:** a mapping change is applied to **all periods**, not only from the change onward.
   Reports use the latest result; history keeps what was reported before.
6. **Report the movement:** per run, counts by delta kind and the cost that moved between Projects
   (lines whose `project_hash` changed, summed by amount and currency). Lines that moved to or from
   `UNSPEC`/`UNALLOC` feed the audit view, where an analyst turns them into override rules.

### 8.4 Properties to keep
- **Idempotent:** a run with no data and no mapping change changes nothing.
- **Deterministic:** the same inputs and taxonomy version always give the same result.
- **No silent loss:** total cost per currency before and after a re-attribution is identical; only
  its allocation moves.

## 9. Open points
- **Tenancy:** the latest draft again mentioned tenants and "shared between tenants". This spec
  keeps the earlier decision of a single tenant, so the shared catalogs are simply one list.
- **Environment under App** replaces the earlier many-to-many App ↔ Environment link; an
  Environment used by several Apps is now one Environment per App.
- **Alias table** is implemented as the `aliases` collection and the `alias` input table of `internal/attribution`
  (flag `alias` in the trail). A line repaired by an alias is not flagged `tag-corrected`.
- **Unresolved** in the movement report means the evidence did not reach the Project, App or Environment of the
  path; `FEAT-UNSPEC` and `FUNC-UNSPEC` are the normal case (no Cloud counterpart) and are not reported.
- **Line identity** for the delta is the Cost Explorer grouping keys (account, service, period, metric, frequency,
  region, zone, usage type, resource id) **plus the Project/App tag pair**: a two-level tag run returns one row per
  tag pair for the same account, service and day, each with its own cost. A restated tag is therefore a `NEW` line
  plus a `REMOVED` line; `KEYS` covers the other inputs (connection, CSP, currency, account name).
  `input_hash` leaves out the resource group (it comes from the taxonomy, so a change to it is a mapping delta).
- **REMOVED** applies only inside the date window of the incoming data: a stored line with a date between the
  earliest and latest incoming date that is no longer present is marked inactive (kept for history).
