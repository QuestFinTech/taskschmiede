---
title: "ts.art.list"
description: "Query artifacts with filters."
category: "artifact"
requires_auth: true
since: "v0.2.0"
type: docs
---

Query artifacts with filters.

**Requires authentication**

## Description

Lists artifacts with optional filtering and pagination.

Filters can be combined: for example, list all active doc artifacts in an
endeavour with a specific tag. Text search matches against title and summary.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `endeavour_id` | string |  |  | Filter by endeavour |
| `task_id` | string |  |  | Filter by task |
| `kind` | string |  |  | Filter by kind: link, doc, repo, file, dataset, dashboard, runbook, other |
| `status` | string |  |  | Filter by status: active, archived |
| `tags` | string |  |  | Filter by tag. Single string value; matches artifacts whose tags array contains this substring. |
| `search` | string |  |  | Search in title and summary (partial match) |
| `limit` | integer |  | `50` | Maximum number of results to return |
| `offset` | integer |  | `0` | Number of results to skip (for pagination) |

## Response

Returns a paginated list of artifacts.

```json
{
  "artifacts": [
    {
      "created_at": "2026-02-09T10:00:00Z",
      "id": "art_d4e5f6a1b2c3...",
      "kind": "doc",
      "status": "active",
      "tags": [
        "architecture",
        "decision-record"
      ],
      "title": "Architecture Decision Record: FRM Migration",
      "updated_at": "2026-02-09T10:00:00Z"
    }
  ],
  "limit": 50,
  "offset": 0,
  "total": 1
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |

## Examples

### List docs in an endeavour

Find all doc artifacts linked to an endeavour.

**Request:**

```json
{
  "endeavour_id": "edv_bd159eb7bb9a877a...",
  "kind": "doc"
}
```

**Response:**

```json
{
  "artifacts": [
    {
      "id": "art_d4e5f6a1b2c3...",
      "kind": "doc",
      "tags": [
        "architecture"
      ],
      "title": "Architecture Decision Record: FRM Migration"
    }
  ],
  "limit": 50,
  "offset": 0,
  "total": 1
}
```

### Search artifacts by tag

Find artifacts with a specific tag.

**Request:**

```json
{
  "tags": "architecture"
}
```

**Response:**

```json
{
  "artifacts": [
    {
      "id": "art_d4e5f6a1b2c3...",
      "kind": "doc",
      "tags": [
        "architecture",
        "decision-record"
      ],
      "title": "Architecture Decision Record: FRM Migration"
    }
  ],
  "limit": 50,
  "offset": 0,
  "total": 1
}
```

## Related Tools

- [`ts.art.create`](../ts.art.create/)
- [`ts.art.get`](../ts.art.get/)
- [`ts.art.update`](../ts.art.update/)

