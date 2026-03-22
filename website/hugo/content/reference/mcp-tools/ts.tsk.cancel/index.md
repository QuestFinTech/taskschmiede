---
title: "ts.tsk.cancel"
description: "Cancel a task with a reason."
category: "task"
requires_auth: true
since: "v0.8.0"
type: docs
---

Cancel a task with a reason.

**Requires authentication**

## Description

Convenience tool to cancel a task. Equivalent to calling ts.tsk.update
with status "canceled" and a canceled_reason, but with a simpler interface.

Both id and reason are required.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `id` | string | Yes |  | Task ID to cancel |
| `reason` | string | Yes |  | Reason for cancellation |

## Response

Returns the task ID and list of fields that were updated.

```json
{
  "id": "tsk_68e9623ade9b1631...",
  "updated_at": "2026-02-16T10:00:00Z",
  "updated_fields": [
    "status",
    "canceled_reason"
  ]
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |
| `not_found` | Task with this ID does not exist |
| `invalid_input` | Task ID or reason is missing |
| `invalid_transition` | Task cannot be canceled from its current status |

## Examples

### Cancel a task

Cancel a planned task that is no longer needed.

**Request:**

```json
{
  "id": "tsk_68e9623ade9b1631...",
  "reason": "Superseded by new approach"
}
```

**Response:**

```json
{
  "id": "tsk_68e9623ade9b1631...",
  "updated_at": "2026-02-16T10:00:00Z",
  "updated_fields": [
    "status",
    "canceled_reason"
  ]
}
```

## Related Tools

- [`ts.tsk.update`](../ts.tsk.update/)
- [`ts.tsk.get`](../ts.tsk.get/)
- [`ts.tsk.list`](../ts.tsk.list/)

