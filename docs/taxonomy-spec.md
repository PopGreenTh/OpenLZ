# Cost Hierarchy Taxonomy — Specification

> Status: **agreed spec** (2026-10-14). Implementation status and remaining work:
> `docs/handoff/finops-taxonomy-v2.md`. Shareable page with diagram:
> [Cost Hierarchy Taxonomy](https://claude.ai/artifact/DPKHjS5xmot6cWeiQzgFyu).

## 1. Notation

- **Code / Name.** Every item has a **Code** (stable business key, managed by people) and a **Name**
  (display text). Codes are unique as described in §5. Names may repeat.
- **Resource ID.** Resources use the cloud provider's own identifier (instance id, ARN) instead of
  a managed Code.
- **Cost-data codes.** For levels that also appear on cost data, the Code must equal the value
  the cloud reports: Account Code = AWS account id, Region Code = `ap-southeast-1`,
  Service Code = `AmazonEC2`, Usage Code = usage type `BoxUsage:t3.medium`.
- **`Group/Member`** is one level with two parts. The *member* is what appears on cost data
  (Account, Region, Service, Usage, Resource). The *group* is the business label it maps to
  (Environment, CSP, ServiceCat, ServiceUsage, ResourceGroup).

## 2. Hierarchies

### Biz Cost Hierarchy (per tenant)
```
+ Tenant
  + BusinessUnit
    + CostCenter
      + OwnerTeam
        + ProductLine
          + CustomerFunc
```

### Project Cost Hierarchy
```
+ Tenant
  + Project
    + App
      + Environment / Account
        + CSP / Region                  ┐ shared catalog (§4)
          + ServiceCat / Service        ┘
            + ServiceUsage / Usage          (per tenant)
              + ResourceGroup / Resource    (per tenant)
```

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
| From (member) | Relation | To (group) | Note |
|---|---|---|---|
| App | ↔ many-to-many | Environment | An App can run in several Environments; an Environment can host several Apps |
| Account | can be mapped | Environment | |
| Region | can be mapped | CSP | Region Code is unique only within its CSP |
| Service | can be mapped | ServiceCat | |
| ServiceUsage | can be mapped | Service | |
| Usage | can be mapped | ServiceUsage | |
| Resource | can be mapped | ResourceGroup | |
| Resource | can be mapped | Environment | Overrides the Account's Environment |

### 3.3 Consistency checks (warnings, never errors)
- An App's CostCenter should be in the same BusinessUnit as the App's Project.
- A ProductLine's App should charge the same CostCenter that the ProductLine's OwnerTeam sits under.
- An Environment should be linked to at least one App.

## 4. Tenancy and shared catalogs

| Scope | Levels |
|---|---|
| **Per tenant** | BusinessUnit, CostCenter, OwnerTeam, ProductLine, CustomerFunc, Project, App, Environment, Account, ServiceUsage, Usage, ResourceGroup, Resource |
| **Shared by all tenants** | CSP, Region, ServiceCat, Service |

- Shared catalogs describe the cloud provider, not an organization. They are maintained once and
  every tenant uses them.
- ServiceUsage/Usage are per tenant: each tenant groups usage types its own way, and a tenant's
  ServiceUsage may map to a shared Service.
- Matching never crosses tenants. Only shared catalogs are visible to every tenant.
- A tenant-owned item may reference a shared catalog item; a shared item never references a
  tenant-owned item.

## 5. Uniqueness

| Level | Unique within |
|---|---|
| Tenant-owned levels | tenant + level |
| Region | CSP |
| Other shared catalog levels | level (global) |
| Resource ID | tenant |
| App ↔ Environment link | tenant (the pair appears once) |

Names may repeat. A repeated name only matches when another known level narrows it down (§6.5).

## 6. Matching / Mapping

Each cost row is matched in both directions, repeated until nothing changes.

1. **Evidence:** tags that match the taxonomy (by Code, or by a Name that is unique), plus the
   row's own Account, CSP, Region, Service, Usage type and Resource ID.
2. **Bottom-up**, most specific first:
   - Resource → ResourceGroup → CustomerFunc → ProductLine → OwnerTeam → CostCenter → BusinessUnit
   - Resource → ResourceGroup → CustomerFunc → ProductLine → App → Project → BusinessUnit
   - Resource → Environment → App → Project → BusinessUnit
   - Account → Environment → App → Project → BusinessUnit
   - App → CostCenter → BusinessUnit
   - Usage → ServiceUsage → Service → ServiceCat;  Region → CSP
3. **Top-down:**
   - BusinessUnit → Project → App → Environment → Account
   - BusinessUnit → CostCenter → OwnerTeam → ProductLine → CustomerFunc
   - App → ProductLine

   A level is filled only when exactly one candidate remains after intersecting what every known
   level allows. Example: a shared Environment's Apps ∩ the Project's Apps.
4. **Ambiguity:** if several candidates remain but all share the same parent, the parent is
   filled and the level itself stays empty. Example: a shared Environment whose Apps are all in
   one Project still yields the Project and BusinessUnit.
5. **Repeated names** (e.g. "Production") are matched within an already-known parent. Codes always
   match directly.
6. **Never inferred top-down:** CSP/Region, ServiceCat/Service, ServiceUsage/Usage and
   ResourceGroup/Resource. These come only from the cost row itself. A value the row already
   carries (e.g. an unknown Account) is never replaced.
7. **Priority:** a tag that matches the taxonomy beats a derived value. A tag that matches
   nothing does not block derivation; it is kept only if nothing else fills that level.
8. **Conflicts are not flagged** during matching (e.g. an OwnerTeam under a different CostCenter
   than the App's). The checks in §3.3 report taxonomy-level problems at validation time.
