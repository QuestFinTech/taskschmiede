---
title: "ts.edv.update"
description: "Update endeavour attributes (partial update)."
category: "endeavour"
requires_auth: true
since: "v0.2.2"
type: docs
---

Update endeavour attributes (partial update).

**Requires authentication**

## Description

Updates an existing endeavour. Only provided fields are changed;
omitted fields remain unchanged.

Endeavour status transitions follow a defined lifecycle:
- pending -> active, deleted
- active -> on_hold, completed, deleted
- on_hold -> active, deleted
- completed -> archived (completed is terminal; archive to reclaim tier slot)

The deleted status is a soft delete. Deleted endeavours are excluded from
list results by default but can be retrieved by filtering with status=deleted.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `id` | string | Yes |  | Endeavour ID |
| `name` | string |  |  | New name |
| `description` | string |  |  | New description |
| `status` | string |  |  | New status: pending, active, on_hold, completed, deleted |
| `goals` | array |  |  | Success criteria / goals (replaces existing array) |
| `start_date` | string |  |  | Start date (ISO 8601, empty string to clear) |
| `end_date` | string |  |  | End date (ISO 8601, empty string to clear) |
| `metadata` | object |  |  | Metadata to set (replaces existing) |

## Response

Returns the list of updated field names.

```json
{
  "id": "edv_bd159eb7bb9a877a...",
  "updated_at": "2026-02-07T18:00:00Z",
  "updated_fields": [
    "status"
  ]
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |
| `not_found` | Endeavour with this ID does not exist |
| `invalid_input` | No fields to update |
| `invalid_transition` | Invalid status transition |

## Examples

### Complete an endeavour

Mark an active endeavour as completed.

**Request:**

```json
{
  "id": "edv_bd159eb7bb9a877a...",
  "status": "completed"
}
```

**Response:**

```json
{
  "id": "edv_bd159eb7bb9a877a...",
  "updated_at": "2026-02-07T18:00:00Z",
  "updated_fields": [
    "status"
  ]
}
```

### Update goals and end date

Modify goals and set an end date.

**Request:**

```json
{
  "end_date": "2026-04-30T00:00:00Z",
  "goals": [
    "Ship v0.3.0",
    "Agent marketplace prototype"
  ],
  "id": "edv_bd159eb7bb9a877a..."
}
```

**Response:**

```json
{
  "id": "edv_bd159eb7bb9a877a...",
  "updated_at": "2026-02-07T18:00:00Z",
  "updated_fields": [
    "goals",
    "end_date"
  ]
}
```

## Related Tools

- [`ts.edv.create`](../ts.edv.create/)
- [`ts.edv.get`](../ts.edv.get/)
- [`ts.edv.list`](../ts.edv.list/)

