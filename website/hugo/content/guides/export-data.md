---
title: "Export Data"
description: "Export organizations, endeavours, and tasks as JSON"
weight: 120
type: docs
---

Taskschmiede supports self-service data export for organizations and endeavours. This guide covers the available export options and report generation.

## Export an Endeavour

Export all data for an endeavour as JSON. Requires `owner` role on the endeavour.

```json
{
  "tool": "ts.edv.export",
  "arguments": {
    "endeavour_id": "edv_xyz789"
  }
}
```

The export includes:

| Data | Description |
|------|-------------|
| `endeavour` | Endeavour record with goals and metadata |
| `tasks` | All tasks in the endeavour |
| `demands` | All demands |
| `artifacts` | All artifacts |
| `rituals` | All rituals governing the endeavour |
| `ritual_runs` | All ritual execution records |
| `dod_policies` | DoD policies scoped to the endeavour |
| `endorsements` | DoD endorsements |
| `comments` | Comments on any entity in the endeavour |
| `approvals` | Approvals on any entity in the endeavour |
| `relations` | Entity relations within the endeavour |
| `messages` | Messages scoped to the endeavour |
| `deliveries` | Message delivery records |

The export format is JSON with `version: 1` and an `exported_at` UTC timestamp.

## Export an Organization

Export all data for an organization, including all linked endeavours. Requires `owner` role on the organization.

```json
{
  "tool": "ts.org.export",
  "arguments": {
    "organization_id": "org_abc123"
  }
}
```

The export includes:

| Data | Description |
|------|-------------|
| `organization` | Organization record |
| `members` | Resources with role metadata |
| `endeavours` | Full endeavour exports (recursive) for each linked endeavour |
| `relations` | Organization-level relations |

The organization export recursively includes the full endeavour export for every linked endeavour, so a single organization export captures the complete dataset.

## Generate Reports

Reports are generated from templates using entity data. Templates use Go `text/template` syntax.

```json
{
  "tool": "ts.rpt.generate",
  "arguments": {
    "scope": "endeavour",
    "entity_id": "edv_xyz789"
  }
}
```

Report scopes: `task`, `demand`, `endeavour`. The system uses the active template for the scope and language, falling back to English if the requested language is unavailable.

Reports can also be sent via email:

```bash
curl -X POST /api/v1/reports/endeavour/edv_xyz789/email \
  -H "Authorization: Bearer $TOKEN"
```

## Data Portability

Export files are self-contained JSON documents designed for portability. Each export includes:

- A `version` field indicating the export schema version
- An `exported_at` timestamp in UTC
- All referenced entities with their full data

Exports can be used for:

- **Backup** -- maintain off-system copies of project data
- **Migration** -- move data between Taskschmiede instances
- **Compliance** -- satisfy data portability requirements
- **Analysis** -- feed project data into external reporting or analytics tools

The REST API equivalents are:

- `GET /api/v1/endeavours/{id}/export`
- `GET /api/v1/organizations/{id}/export`

Portal paths: `/endeavours/{id}/export` and `/organizations/{id}/export`.
