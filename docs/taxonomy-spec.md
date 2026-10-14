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
```
+ Project
  + App
    + CSP
      + LZ / Connection          (one CSP and one billing currency per LZ)
        + Environment / Account
          + Region / Zone
            + ServiceCat / Service
              + ServiceUsage / Usage
                + ResourceGroup / Resource
```

- **CSP** is the cloud service provider (AWS, Azure, GCP). Each LZ belongs to exactly one CSP.
- **LZ** is a Landing Zone: a cloud organization the company runs (e.g. an AWS Organization). Each LZ
  has exactly one CSP and one **billing currency**.
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
| LZ | is under | CSP | One CSP per LZ |
| Connection | is under | LZ | A billing source belongs to one LZ |
| Environment | is under | LZ | |
| App | ↔ many-to-many | Environment | An App can run in several Environments; an Environment can host several Apps |
| Account | can be mapped | Connection | The billing source the Account is billed through; for AWS, the payer account in Cost Explorer |
| Account | can be mapped | Environment | |
| Region | can be mapped | CSP | Region Code is unique only within its CSP |
| Zone | can be mapped | Region | Zone Code is unique only within its Region |
| Service | can be mapped | ServiceCat | |
| ServiceUsage | can be mapped | Service | |
| Usage | can be mapped | ServiceUsage | |
| Resource | can be mapped | ResourceGroup | |
| Resource | can be mapped | Environment | Overrides the Account's Environment |

An App's CSPs and LZs are not stored. They come from its Environments (App ↔ Environment → LZ → CSP).

### 3.3 Consistency checks (warnings, never errors)
- An App's CostCenter should be in the same BusinessUnit as the App's Project.
- A ProductLine's App should charge the same CostCenter that the ProductLine's OwnerTeam sits under.
- An Environment should be linked to at least one App.
- An Account's Connection should be in the same LZ as the Account's Environment.
- A cost row's Region should belong to the CSP of its LZ.
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

Names may repeat. A repeated name only matches when another known level narrows it down (§6, rule 5).

## 6. Matching / Mapping

Each cost row is matched in both directions, repeated until nothing changes.

1. **Evidence:** tags that match the taxonomy (by Code, or by a Name that is unique), plus the
   row's own Connection (payer), Account, CSP, Region, Zone, Service, Usage type and Resource ID.
2. **Bottom-up**, most specific first:
   - Resource → ResourceGroup → CustomerFunc → ProductLine → OwnerTeam → CostCenter → BusinessUnit
   - Resource → ResourceGroup → CustomerFunc → ProductLine → App → Project → BusinessUnit
   - Resource → Environment → App → Project → BusinessUnit
   - Account → Environment → App → Project → BusinessUnit
   - Account → Connection → LZ → CSP;  Environment → LZ → CSP
   - App → CostCenter → BusinessUnit
   - Usage → ServiceUsage → Service → ServiceCat;  Zone → Region → CSP
3. **Top-down:**
   - BusinessUnit → Project → App → Environment → Account
   - BusinessUnit → CostCenter → OwnerTeam → ProductLine → CustomerFunc
   - App → ProductLine
   - CSP → LZ → Connection;  LZ → Environment

   A level is filled only when exactly one candidate remains after intersecting what every known
   level allows. Example: a shared Environment's Apps ∩ the Project's Apps.
4. **Ambiguity:** if several candidates remain but all share the same parent, the parent is
   filled and the level itself stays empty. Example: a shared Environment whose Apps are all in
   one Project still yields the Project and BusinessUnit.
5. **Repeated names** (e.g. "Production") are matched within an already-known parent. Codes always
   match directly.
6. **Never inferred top-down:** Region/Zone, ServiceCat/Service, ServiceUsage/Usage and
   ResourceGroup/Resource. These come only from the cost row itself. A value the row already
   carries (e.g. an unknown Account or Connection) is never replaced.
7. **Priority:** a tag that matches the taxonomy beats a derived value. A tag that matches
   nothing does not block derivation; it is kept only if nothing else fills that level.
8. **Conflicts are not flagged** during matching (e.g. an OwnerTeam under a different CostCenter
   than the App's). The checks in §3.3 report taxonomy-level problems at validation time.
