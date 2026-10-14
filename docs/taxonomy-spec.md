# Cost Hierarchy Taxonomy — Specification

> Status: **agreed spec** (2026-10-14). OpenLZ supports a **single tenant**: there is no Tenant
> level and one taxonomy holds every level. Implementation status and remaining work:
> `docs/handoff/finops-taxonomy-v2.md`. Shareable page with diagram:
> [Cost Hierarchy Taxonomy](https://claude.ai/artifact/DPKHjS5xmot6cWeiQzgFyu).

## 1. Notation

- **Code / Name.** Every item has a **Code** (stable business key, managed by people) and a **Name**
  (display text). Codes are unique as described in §5. Names may repeat.
- **Resource ID.** Resources use the cloud provider's own identifier (instance id, ARN) instead of
  a managed Code.
- **CSP** (cloud service provider) is an attribute of LZ and Region, not a level (§3.3).
- **Cost-data codes.** For levels that also appear on cost data, the Code must equal the value
  the cloud reports: Account Code = AWS account id, Connection Code = billing source id (e.g. the
  AWS payer account id), Region Code = `ap-southeast-1`, Zone Code = availability zone
  `ap-southeast-1a`, Service Code = `AmazonEC2`,
  Usage Code = usage type `BoxUsage:t3.medium`.
- **`Group/Member`** is one level with two parts. The *member* is what appears on cost data
  (Connection, Account, Zone, Service, Usage, Resource). The *group* is the label it maps to
  (LZ, Environment, Region, ServiceCat, ServiceUsage, ResourceGroup).

## 2. Hierarchies

### Biz Cost Hierarchy
```
+ BusinessUnit
  + CostCenter
    + OwnerTeam
      + ProductLine
        + CustomerFunc
```

### Project Cost Hierarchy
Environment/Account is where two parents meet: the **App side** and the **LZ side**.

```
+ Project                     + LZ / Connection      (LZ attributes: CSP, billing currency)
  + App ─────────┐                 │
    many-to-many │                 │ is under (required)
                 + Environment / Account ─────────────┘
                   + Region / Zone                    (Region attribute: CSP)
                     + ServiceCat / Service
                       + ServiceUsage / Usage
                         + ResourceGroup / Resource
```

An LZ is **not** under an App. An LZ hosts many Projects and Apps, and an App can run in several
LZs. Both work through Environments: an Environment belongs to exactly one LZ and can host several
Apps, and an App can have Environments in several LZs. See §3.2 and the example in §6.

- **LZ** is a Landing Zone: a cloud organization the company runs (e.g. an AWS Organization). Each LZ
  has exactly one **CSP** and one **billing currency**, both stored as attributes of the LZ (§3.3).
  CSP is not a level of its own.
- **Connection** is one billing source inside an LZ: the account or enrollment that receives the
  bill (e.g. an AWS payer/management account). It bills in its LZ's currency.
- **Region / Zone** stays below Account because one Account runs in many Regions. A Zone is an
  availability zone inside a Region; both are taken from the cost row.

## 3. Relations

Relation types:
- **is under**: hierarchy parent, required.
- **must ref**: required cross-hierarchy link.
- **can ref / can be mapped**: optional link.

### 3.1 Cross-hierarchy (Project ↔ Biz)
| From | Relation | To |
|---|---|---|
| Project | must ref (is under) | BusinessUnit |
| App | must ref | CostCenter |
| ProductLine | can ref | App |
| ResourceGroup | can ref | CustomerFunc |

### 3.2 Within the Project hierarchy
| From | Relation | To | Note |
|---|---|---|---|
| Connection | is under | LZ | A billing source belongs to one LZ |
| Environment | is under | LZ | |
| App | ↔ many-to-many | Environment | An App can run in several Environments; an Environment can host several Apps |
| Account | can be mapped | Connection | The billing source the Account is billed through; for AWS, the payer account in Cost Explorer |
| Account | can be mapped | Environment | |
| Zone | can be mapped | Region | Zone Code is unique only within its Region |
| Service | can be mapped | ServiceCat | |
| ServiceUsage | can be mapped | Service | |
| Usage | can be mapped | ServiceUsage | |
| Resource | can be mapped | ResourceGroup | |
| Resource | can be mapped | Environment | Overrides the Account's Environment |

There is no direct App ↔ LZ or Project ↔ LZ link. An App's LZs (and so its CSPs) come from its
Environments (App ↔ Environment → LZ), and a Project's LZs come from its Apps. An App that runs in
an LZ therefore needs at least one Environment in that LZ.

### 3.3 Attributes
Attributes are plain values on an item, not links to another level.

| Level | Attribute | Required | Values |
|---|---|---|---|
| LZ | `csp` | yes | One of a fixed list: `AWS`, `Azure`, `GCP`, `OCI` (extend the list in code) |
| LZ | `currency` | yes | ISO 4217 code, e.g. `USD`, `THB` |
| Region | `csp` | no | Same list as LZ. A Region Code is unique only within its CSP, e.g. the same code under two providers |

### 3.4 Consistency checks (warnings, never errors)
- An App's CostCenter should be in the same BusinessUnit as the App's Project.
- A ProductLine's App should charge the same CostCenter that the ProductLine's OwnerTeam sits under.
- An Environment should be linked to at least one App.
- An Account's Connection should be in the same LZ as the Account's Environment.
- A cost row's Region should have the same CSP as its LZ.
- A cost row's Zone should belong to its Region.

## 4. Currency

- Every LZ has one **billing currency** (ISO 4217, e.g. `USD`, `THB`). Every Connection in the LZ
  bills in that currency.
- A cost row whose reported currency differs from its LZ's currency is a data error and is reported.
- Amounts in different currencies are never summed as-is. A roll-up across LZs with different
  currencies converts to one reporting currency first, or shows each currency separately.

## 5. Uniqueness

| Level | Unique within |
|---|---|
| Every level except Region and Zone | its level (one Code per item) |
| Region | its CSP (the same Region Code may exist under two CSPs) |
| Zone | its Region (e.g. Azure zones `1`, `2`, `3` repeat in every Region) |
| Resource ID | all Resources |
| App ↔ Environment link | each pair appears once |

Names may repeat. A repeated name only matches when another known level narrows it down (§6, rule 6).

## 6. Matching / Mapping

Each cost row is matched in both directions, repeated until nothing changes.

1. **Evidence:** tags that match the taxonomy (by Code, or by a Name that is unique), plus the
   row's own Connection (payer), Account, CSP, Region, Zone, Service, Usage type and Resource ID.
2. **Bottom-up**, most specific first:
   - Resource → ResourceGroup → CustomerFunc → ProductLine → OwnerTeam → CostCenter → BusinessUnit
   - Resource → ResourceGroup → CustomerFunc → ProductLine → App → Project → BusinessUnit
   - Resource → Environment → App → Project → BusinessUnit
   - Account → Environment → App → Project → BusinessUnit
   - Account → Connection → LZ;  Environment → LZ   (the row's CSP and currency are read from the LZ)
   - App → CostCenter → BusinessUnit
   - Usage → ServiceUsage → Service → ServiceCat;  Zone → Region
3. **Top-down:**
   - BusinessUnit → Project → App → Environment → Account
   - BusinessUnit → CostCenter → OwnerTeam → ProductLine → CustomerFunc
   - App → ProductLine
   - LZ → Connection;  LZ → Environment

   A level is filled only when exactly one candidate remains after intersecting what every known
   level allows. Example: a shared Environment's Apps ∩ the Project's Apps.
4. **LZ and App together:** a Connection or Account identifies the LZ, and a tag identifies the
   App. The row's Environment is then the Environments of that App ∩ the Environments of that LZ.
   Example: App `APP-GATEWAY` has `ENV-PAY-PROD` in `LZ-MAIN` and `ENV-PAY-DR` in `LZ-DR`; a row billed
   through a `LZ-DR` Connection and tagged `APP-GATEWAY` resolves to `ENV-PAY-DR`.
5. **Ambiguity:** if several candidates remain but all share the same parent, the parent is
   filled and the level itself stays empty. Example: a shared Environment whose Apps are all in
   one Project still yields the Project and BusinessUnit.
6. **Repeated names** (e.g. "Production") are matched within an already-known parent. Codes always
   match directly.
7. **Never inferred top-down:** Region/Zone, ServiceCat/Service, ServiceUsage/Usage and
   ResourceGroup/Resource. These come only from the cost row itself. A value the row already
   carries (e.g. an unknown Account or Connection) is never replaced.
8. **Priority:** a tag that matches the taxonomy beats a derived value. A tag that matches
   nothing does not block derivation; it is kept only if nothing else fills that level.
9. **Conflicts are not flagged** during matching (e.g. an OwnerTeam under a different CostCenter
   than the App's). The checks in §3.4 report taxonomy-level problems at validation time.
