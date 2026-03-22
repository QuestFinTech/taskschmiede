---
title: "ts.rtl.get"
description: "Retrieve a ritual by ID."
category: "ritual"
requires_auth: true
since: "v0.2.0"
type: docs
---

Retrieve a ritual by ID.

**Requires authentication**

## Description

Retrieves detailed information about a specific ritual, including
its prompt, schedule, origin, lineage (predecessor_id), and enabled state.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `id` | string | Yes |  | Ritual ID |

## Response

Returns the full ritual object.

```json
{
  "created_at": "2026-02-09T10:00:00Z",
  "description": "A brief daily check-in to align the team.",
  "id": "rtl_a1b2c3d4e5f6...",
  "is_enabled": true,
  "metadata": {},
  "name": "Daily standup",
  "origin": "custom",
  "prompt": "Review yesterday's progress, identify blockers, plan today's work.",
  "schedule": {
    "expression": "0 9 * * 1-5",
    "type": "cron"
  },
  "status": "active",
  "updated_at": "2026-02-09T10:00:00Z"
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |
| `not_found` | Ritual with this ID does not exist |

## Examples

### Get ritual by ID

**Request:**

```json
{
  "id": "rtl_a1b2c3d4e5f6..."
}
```

**Response:**

```json
{
  "created_at": "2026-02-09T10:00:00Z",
  "id": "rtl_a1b2c3d4e5f6...",
  "is_enabled": true,
  "name": "Daily standup",
  "origin": "custom",
  "prompt": "Review yesterday's progress, identify blockers, plan today's work.",
  "status": "active"
}
```

## Related Tools

- [`ts.rtl.create`](../ts.rtl.create/)
- [`ts.rtl.list`](../ts.rtl.list/)
- [`ts.rtl.update`](../ts.rtl.update/)
- [`ts.rtl.fork`](../ts.rtl.fork/)
- [`ts.rtl.lineage`](../ts.rtl.lineage/)

