---
title: "ts.rtr.update"
description: "Update a ritual run (status, results, effects, error)."
category: "ritual_run"
requires_auth: true
since: "v0.2.0"
type: docs
---

Update a ritual run (status, results, effects, error).

**Requires authentication**

## Description

Updates a ritual run to record its outcome. This is how you complete a run.

Status transitions:
- running -> succeeded (work completed successfully)
- running -> failed (work encountered an error)
- running -> skipped (run was skipped, e.g., nothing to do)

When transitioning to a terminal status, finished_at is set automatically.

The effects field records what the run produced (tasks created, updated, etc.).
The error field captures failure details when status=failed.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `id` | string | Yes |  | Ritual run ID |
| `status` | string |  |  | New status: succeeded, failed, skipped |
| `result_summary` | string |  |  | Free-form summary of what happened |
| `effects` | object |  |  | Effects of the run: {"tasks_created":[], "tasks_updated":[], ...} |
| `error` | object |  |  | Error details if failed: {"code":"...", "message":"..."} |
| `metadata` | object |  |  | Metadata to set (replaces existing) |

## Response

Returns the run ID and list of fields that were updated.

```json
{
  "id": "rtr_x1y2z3a4b5c6...",
  "updated_at": "2026-02-09T10:05:00Z",
  "updated_fields": [
    "status",
    "result_summary",
    "effects"
  ]
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |
| `not_found` | Ritual run with this ID does not exist |
| `invalid_input` | No fields to update or invalid status transition |

## Examples

### Complete a successful run

Mark a ritual run as succeeded with results.

**Request:**

```json
{
  "effects": {
    "tasks_created": [
      "tsk_new1...",
      "tsk_new2..."
    ],
    "tasks_updated": [
      "tsk_existing..."
    ]
  },
  "id": "rtr_x1y2z3a4b5c6...",
  "result_summary": "Created 2 tasks, updated 1 task status.",
  "status": "succeeded"
}
```

**Response:**

```json
{
  "id": "rtr_x1y2z3a4b5c6...",
  "updated_at": "2026-02-09T10:05:00Z",
  "updated_fields": [
    "status",
    "result_summary",
    "effects"
  ]
}
```

### Record a failed run

Mark a ritual run as failed with error details.

**Request:**

```json
{
  "error": {
    "code": "timeout",
    "message": "Ritual execution timed out after 5 minutes."
  },
  "id": "rtr_x1y2z3a4b5c6...",
  "status": "failed"
}
```

**Response:**

```json
{
  "id": "rtr_x1y2z3a4b5c6...",
  "updated_at": "2026-02-09T10:05:00Z",
  "updated_fields": [
    "status",
    "error"
  ]
}
```

## Related Tools

- [`ts.rtr.create`](../ts.rtr.create/)
- [`ts.rtr.get`](../ts.rtr.get/)
- [`ts.rtr.list`](../ts.rtr.list/)

