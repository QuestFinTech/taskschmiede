---
title: "ts.rtr.get"
description: "Retrieve a ritual run by ID."
category: "ritual_run"
requires_auth: true
since: "v0.2.0"
type: docs
---

Retrieve a ritual run by ID.

**Requires authentication**

## Description

Retrieves detailed information about a specific ritual run, including
its status, trigger, result summary, effects, error details, and timing.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `id` | string | Yes |  | Ritual run ID |

## Response

Returns the full ritual run object.

```json
{
  "created_at": "2026-02-09T10:00:00Z",
  "effects": {
    "tasks_created": [
      "tsk_new1...",
      "tsk_new2..."
    ],
    "tasks_updated": [
      "tsk_existing..."
    ]
  },
  "finished_at": "2026-02-09T10:05:00Z",
  "id": "rtr_x1y2z3a4b5c6...",
  "metadata": {},
  "result_summary": "Created 2 tasks, updated 1 task status.",
  "ritual_id": "rtl_a1b2c3d4e5f6...",
  "started_at": "2026-02-09T10:00:00Z",
  "status": "succeeded",
  "trigger": "manual",
  "updated_at": "2026-02-09T10:05:00Z"
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |
| `not_found` | Ritual run with this ID does not exist |

## Examples

### Get ritual run by ID

**Request:**

```json
{
  "id": "rtr_x1y2z3a4b5c6..."
}
```

**Response:**

```json
{
  "finished_at": "2026-02-09T10:05:00Z",
  "id": "rtr_x1y2z3a4b5c6...",
  "result_summary": "Created 2 tasks, updated 1 task status.",
  "ritual_id": "rtl_a1b2c3d4e5f6...",
  "started_at": "2026-02-09T10:00:00Z",
  "status": "succeeded",
  "trigger": "manual"
}
```

## Related Tools

- [`ts.rtr.create`](../ts.rtr.create/)
- [`ts.rtr.list`](../ts.rtr.list/)
- [`ts.rtr.update`](../ts.rtr.update/)

