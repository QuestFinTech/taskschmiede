---
title: "ts.dmd.list"
description: "Query demands with filters."
category: "demand"
requires_auth: true
since: "v0.2.0"
type: docs
---

Query demands with filters.

**Requires authentication**

## Description

Lists demands with optional filtering and pagination.

Filters can be combined: for example, list all high-priority open demands
in an endeavour. Text search matches against title and description.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `status` | string |  |  | Filter by status: open, in_progress, fulfilled, canceled |
| `type` | string |  |  | Filter by demand type (e.g., feature, bug, goal) |
| `priority` | string |  |  | Filter by priority: low, medium, high, urgent |
| `endeavour_id` | string |  |  | Filter by endeavour |
| `search` | string |  |  | Search in title and description (partial match) |
| `limit` | integer |  | `50` | Maximum number of results to return |
| `offset` | integer |  | `0` | Number of results to skip (for pagination) |

## Response

Returns a paginated list of demands.

```json
{
  "demands": [
    {
      "created_at": "2026-02-09T10:00:00Z",
      "id": "dmd_a1b2c3d4e5f6...",
      "priority": "high",
      "status": "open",
      "title": "Add dark mode support",
      "type": "feature",
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

### List open demands

Get all open demands.

**Request:**

```json
{
  "status": "open"
}
```

**Response:**

```json
{
  "demands": [
    {
      "id": "dmd_a1b2c3d4e5f6...",
      "priority": "high",
      "status": "open",
      "title": "Add dark mode support",
      "type": "feature"
    }
  ],
  "limit": 50,
  "offset": 0,
  "total": 1
}
```

### Filter by priority and endeavour

Find urgent demands in a specific endeavour.

**Request:**

```json
{
  "endeavour_id": "edv_bd159eb7bb9a877a...",
  "priority": "urgent"
}
```

**Response:**

```json
{
  "demands": [
    {
      "id": "dmd_f6e5d4c3b2a1...",
      "priority": "urgent",
      "status": "open",
      "title": "Login fails on mobile browsers",
      "type": "bug"
    }
  ],
  "limit": 50,
  "offset": 0,
  "total": 1
}
```

## Related Tools

- [`ts.dmd.create`](../ts.dmd.create/)
- [`ts.dmd.get`](../ts.dmd.get/)
- [`ts.dmd.update`](../ts.dmd.update/)

