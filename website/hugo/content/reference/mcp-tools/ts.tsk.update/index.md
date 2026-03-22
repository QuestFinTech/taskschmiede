---
title: "ts.tsk.update"
description: "Update task attributes (partial update)."
category: "task"
requires_auth: true
since: "v0.2.0"
type: docs
---

Update task attributes (partial update).

**Requires authentication**

## Description

Updates one or more fields of an existing task. Only provided fields
are changed; omitted fields remain unchanged.

Status transitions are validated:
- planned -> active, canceled
- active -> done, canceled, planned
- done -> active (reopen)
- canceled -> planned (reopen)

Lifecycle timestamps are set automatically:
- started_at: set when status changes to active
- completed_at: set when status changes to done
- canceled_at: set when status changes to canceled

To unassign or unlink, pass an empty string for assignee_id or endeavour_id.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `id` | string | Yes |  | Task ID to update |
| `title` | string |  |  | New title |
| `description` | string |  |  | New description |
| `status` | string |  |  | New status: planned, active, done, canceled |
| `endeavour_id` | string |  |  | New endeavour (empty string to unlink) |
| `assignee_id` | string |  |  | New assignee resource (empty string to unassign) |
| `estimate` | number |  |  | Estimated hours |
| `actual` | number |  |  | Actual hours spent |
| `due_date` | string |  |  | Due date (ISO 8601, empty string to clear) |
| `canceled_reason` | string |  |  | Reason for cancellation (when setting status to canceled) |
| `metadata` | object |  |  | Metadata to set (replaces existing metadata) |

## Response

Returns the task ID and list of fields that were updated.

```json
{
  "id": "tsk_68e9623ade9b1631...",
  "updated_at": "2026-02-06T14:00:00Z",
  "updated_fields": [
    "status",
    "title"
  ]
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |
| `not_found` | Task with this ID does not exist |
| `invalid_transition` | Invalid status transition (e.g., planned -> done) |

## Examples

### Start working on a task

Transition a task from planned to active.

**Request:**

```json
{
  "id": "tsk_68e9623ade9b1631...",
  "status": "active"
}
```

**Response:**

```json
{
  "id": "tsk_68e9623ade9b1631...",
  "updated_at": "2026-02-06T14:00:00Z",
  "updated_fields": [
    "status"
  ]
}
```

### Complete a task

Mark an active task as done.

**Request:**

```json
{
  "actual": 3.5,
  "id": "tsk_68e9623ade9b1631...",
  "status": "done"
}
```

**Response:**

```json
{
  "id": "tsk_68e9623ade9b1631...",
  "updated_at": "2026-02-06T16:30:00Z",
  "updated_fields": [
    "status",
    "actual"
  ]
}
```

## Related Tools

- [`ts.tsk.create`](../ts.tsk.create/)
- [`ts.tsk.get`](../ts.tsk.get/)
- [`ts.tsk.list`](../ts.tsk.list/)
- [`ts.tsk.cancel`](../ts.tsk.cancel/)

