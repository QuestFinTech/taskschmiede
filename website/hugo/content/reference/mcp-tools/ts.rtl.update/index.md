---
title: "ts.rtl.update"
description: "Update ritual attributes (cannot change prompt -- fork instead)."
category: "ritual"
requires_auth: true
since: "v0.2.0"
type: docs
---

Update ritual attributes (cannot change prompt -- fork instead).

**Requires authentication**

## Description

Updates one or more fields of an existing ritual. Only provided fields
are changed; omitted fields remain unchanged.

The prompt field cannot be updated directly. To change a ritual's prompt,
use ts.rtl.fork to create a new version with the modified prompt. This
preserves the lineage and audit trail of methodology changes.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `id` | string | Yes |  | Ritual ID |
| `name` | string |  |  | New name |
| `description` | string |  |  | New description |
| `schedule` | object |  |  | New schedule metadata |
| `is_enabled` | boolean |  |  | Enable or disable the ritual |
| `status` | string |  |  | New status: active, archived |
| `endeavour_id` | string |  |  | New endeavour (empty string to unlink) |
| `metadata` | object |  |  | Metadata to set (replaces existing) |

## Response

Returns the ritual ID and list of fields that were updated.

```json
{
  "id": "rtl_a1b2c3d4e5f6...",
  "updated_at": "2026-02-09T12:00:00Z",
  "updated_fields": [
    "is_enabled"
  ]
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |
| `not_found` | Ritual with this ID does not exist |
| `invalid_input` | No fields to update, or attempted to change prompt |

## Examples

### Disable a ritual

Temporarily disable a ritual without deleting it.

**Request:**

```json
{
  "id": "rtl_a1b2c3d4e5f6...",
  "is_enabled": false
}
```

**Response:**

```json
{
  "id": "rtl_a1b2c3d4e5f6...",
  "updated_at": "2026-02-09T12:00:00Z",
  "updated_fields": [
    "is_enabled"
  ]
}
```

### Update schedule

Change the schedule for a ritual.

**Request:**

```json
{
  "id": "rtl_a1b2c3d4e5f6...",
  "schedule": {
    "expression": "0 10 * * 1",
    "type": "cron"
  }
}
```

**Response:**

```json
{
  "id": "rtl_a1b2c3d4e5f6...",
  "updated_at": "2026-02-09T12:00:00Z",
  "updated_fields": [
    "schedule"
  ]
}
```

## Related Tools

- [`ts.rtl.create`](../ts.rtl.create/)
- [`ts.rtl.get`](../ts.rtl.get/)
- [`ts.rtl.fork`](../ts.rtl.fork/)

