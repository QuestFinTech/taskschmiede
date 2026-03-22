---
title: "ts.dmd.update"
description: "Update demand attributes (partial update)."
category: "demand"
requires_auth: true
since: "v0.2.0"
type: docs
---

Update demand attributes (partial update).

**Requires authentication**

## Description

Updates one or more fields of an existing demand. Only provided fields
are changed; omitted fields remain unchanged.

Demand status transitions follow a defined lifecycle:
- open -> in_progress, fulfilled, canceled
- in_progress -> fulfilled, canceled
- fulfilled (terminal state)
- canceled (terminal state)

When setting status to canceled, provide a canceled_reason.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `id` | string | Yes |  | Demand ID |
| `title` | string |  |  | New title |
| `description` | string |  |  | New description |
| `type` | string |  |  | New demand type |
| `status` | string |  |  | New status: open, in_progress, fulfilled, canceled |
| `priority` | string |  |  | New priority: low, medium, high, urgent |
| `endeavour_id` | string |  |  | New endeavour (empty string to unlink) |
| `due_date` | string |  |  | Due date (ISO 8601, empty string to clear) |
| `canceled_reason` | string |  |  | Reason for cancellation (when status=canceled) |
| `metadata` | object |  |  | Metadata to set (replaces existing) |

## Response

Returns the demand ID and list of fields that were updated.

```json
{
  "id": "dmd_a1b2c3d4e5f6...",
  "updated_at": "2026-02-09T11:00:00Z",
  "updated_fields": [
    "status",
    "priority"
  ]
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |
| `not_found` | Demand with this ID does not exist |
| `invalid_input` | No fields to update or invalid priority value |
| `invalid_transition` | Invalid status transition |

## Examples

### Start working on a demand

Move a demand from open to in_progress.

**Request:**

```json
{
  "id": "dmd_a1b2c3d4e5f6...",
  "status": "in_progress"
}
```

**Response:**

```json
{
  "id": "dmd_a1b2c3d4e5f6...",
  "updated_at": "2026-02-09T11:00:00Z",
  "updated_fields": [
    "status"
  ]
}
```

### Fulfill a demand

Mark a demand as fulfilled.

**Request:**

```json
{
  "id": "dmd_a1b2c3d4e5f6...",
  "status": "fulfilled"
}
```

**Response:**

```json
{
  "id": "dmd_a1b2c3d4e5f6...",
  "updated_at": "2026-02-09T12:00:00Z",
  "updated_fields": [
    "status"
  ]
}
```

## Related Tools

- [`ts.dmd.create`](../ts.dmd.create/)
- [`ts.dmd.get`](../ts.dmd.get/)
- [`ts.dmd.list`](../ts.dmd.list/)
- [`ts.dmd.cancel`](../ts.dmd.cancel/)

