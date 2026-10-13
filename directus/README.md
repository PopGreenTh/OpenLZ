# Directus Headless CMS for OpenLZ Multi-Tenant Taxonomy

This Directus project allows cloud architects and FinOps administrators to visually edit and manage multi-tenant cost hierarchy taxonomies (`business_taxonomy_<tenant>.json` and `project_taxonomy_<tenant>.json`).

---

## 1. Quick Start

Run the Directus service using Docker Compose:

```powershell
# From project root
task directus:up

# Or directly with Docker Compose
docker compose -f directus/docker-compose.yml up -d
```

### Accessing Directus
- **URL**: [http://localhost:8055](http://localhost:8055)
- **Admin Email**: `admin@openlz.io`
- **Admin Password**: `admin`

---

## 2. Shared Identity Rules ("Must Behave")

When configuring and editing taxonomies, the following levels share identity:

| Project Level | Business Level | Rule |
| :--- | :--- | :--- |
| `ProjectID` / `ProjectName` | `BusinessUnitID` / `BusinessUnitName` | **Must match exactly** |
| `AppID` / `AppName` | `OwnerID` / `OwnerName` | **Must match exactly** |
| `EnvironmentID` / `EnvironmentName` | `CostCenterID` / `CostCenterName` | **Must match exactly** |

The following relationships can be mapped to different targets:
- `App` → `Project` (optional cross-project mapping)
- `Account` → `Environment` (maps AWS account ID to landing zone environment)
- `Service` → `ServiceCategory` (maps AWS service name to functional category)

---

## 3. Multi-Tenant File Separation

Taxonomies are separated by filename suffix:
- Default: `taxonomy/project_taxonomy.json`, `taxonomy/business_taxonomy.json`
- Tenant A: `taxonomy/project_taxonomy_tenant_a.json`, `taxonomy/business_taxonomy_tenant_a.json`
- Tenant B: `taxonomy/project_taxonomy_tenant_b.json`, `taxonomy/business_taxonomy_tenant_b.json`

Validate any tenant's taxonomy files via OpenLZ CLI:
```powershell
openlz taxonomy validate --tenant tenant_a
```

---

## 4. Stopping Directus

```powershell
task directus:down
```
