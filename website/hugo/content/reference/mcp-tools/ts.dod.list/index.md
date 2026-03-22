---
title: "ts.dod.list"
description: "Query DoD policies with filters."
category: "dod"
requires_auth: true
since: "v0.3.0"
type: docs
---

Query DoD policies with filters.

**Requires authentication**

## Description

Lists DoD policies with optional filtering by status, origin, scope,
and search text.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `status` | string |  |  | Filter by status: active, archived |
| `origin` | string |  |  | Filter by origin: template, custom, derived |
| `scope` | string |  |  | Filter by scope: task |
| `search` | string |  |  | Search in name and description |
| `limit` | integer |  | `50` | Max results (default: 50) |
| `offset` | integer |  | `0` | Pagination offset |

## Response

Returns a paginated list of DoD policies.

```json
{
  "limit": 50,
  "offset": 0,
  "policies": [
    {
      "id": "dod_a1b2c3d4e5f6",
      "name": "Standard Task Completion",
      "status": "active"
    }
  ],
  "total": 1
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |

## Examples

### List active policies

Get all active DoD policies.

**Request:**

```json
{
  "status": "active"
}
```

**Response:**

```json
{
  "limit": 50,
  "offset": 0,
  "policies": [
    {
      "id": "dod_a1b2c3d4e5f6",
      "name": "Standard Task Completion"
    }
  ],
  "total": 1
}
```

## Related Tools

- [`ts.dod.get`](../ts.dod.get/)
- [`ts.dod.create`](../ts.dod.create/)

