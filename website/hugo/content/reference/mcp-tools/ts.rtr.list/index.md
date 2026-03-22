---
title: "ts.rtr.list"
description: "Query ritual runs with filters."
category: "ritual_run"
requires_auth: true
since: "v0.2.0"
type: docs
---

Query ritual runs with filters.

**Requires authentication**

## Description

Lists ritual runs with optional filtering and pagination.

Filter by ritual to see all executions of a specific ritual, or by status
to find running or failed runs.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `ritual_id` | string |  |  | Filter by ritual |
| `status` | string |  |  | Filter by status: running, succeeded, failed, skipped |
| `limit` | integer |  | `50` | Maximum number of results to return |
| `offset` | integer |  | `0` | Number of results to skip (for pagination) |

## Response

Returns a paginated list of ritual runs.

```json
{
  "limit": 50,
  "offset": 0,
  "runs": [
    {
      "finished_at": "2026-02-09T10:05:00Z",
      "id": "rtr_x1y2z3a4b5c6...",
      "ritual_id": "rtl_a1b2c3d4e5f6...",
      "started_at": "2026-02-09T10:00:00Z",
      "status": "succeeded",
      "trigger": "manual"
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

### List runs for a ritual

See all executions of a specific ritual.

**Request:**

```json
{
  "ritual_id": "rtl_a1b2c3d4e5f6..."
}
```

**Response:**

```json
{
  "limit": 50,
  "offset": 0,
  "runs": [
    {
      "finished_at": "2026-02-09T10:05:00Z",
      "id": "rtr_x1y2z3a4b5c6...",
      "ritual_id": "rtl_a1b2c3d4e5f6...",
      "started_at": "2026-02-09T10:00:00Z",
      "status": "succeeded",
      "trigger": "manual"
    }
  ],
  "total": 1
}
```

### List failed runs

Find ritual runs that failed.

**Request:**

```json
{
  "status": "failed"
}
```

**Response:**

```json
{
  "limit": 50,
  "offset": 0,
  "runs": [],
  "total": 0
}
```

## Related Tools

- [`ts.rtr.create`](../ts.rtr.create/)
- [`ts.rtr.get`](../ts.rtr.get/)
- [`ts.rtr.update`](../ts.rtr.update/)

