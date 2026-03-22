---
title: "ts.edv.list"
description: "Query endeavours with filters."
category: "endeavour"
requires_auth: true
since: "v0.2.0"
type: docs
---

Query endeavours with filters.

**Requires authentication**

## Description

Lists endeavours with optional filtering and pagination.

Supports filtering by organization (shows only endeavours linked to that org),
status, and text search.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `status` | string |  |  | Filter by status: pending, active, on_hold, completed, deleted |
| `organization_id` | string |  |  | Filter by organization (shows endeavours linked to this org) |
| `search` | string |  |  | Search by name or description (partial match) |
| `limit` | integer |  | `50` | Maximum number of results to return |
| `offset` | integer |  | `0` | Number of results to skip (for pagination) |

## Response

Returns a paginated list of endeavours.

```json
{
  "endeavours": [
    {
      "created_at": "2026-02-06T13:36:32Z",
      "description": "Develop the agent-first task management system",
      "id": "edv_bd159eb7bb9a877a...",
      "name": "Build Taskschmiede",
      "status": "active",
      "updated_at": "2026-02-06T13:36:32Z"
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

## Related Tools

- [`ts.edv.create`](../ts.edv.create/)
- [`ts.edv.get`](../ts.edv.get/)
- [`ts.edv.update`](../ts.edv.update/)

