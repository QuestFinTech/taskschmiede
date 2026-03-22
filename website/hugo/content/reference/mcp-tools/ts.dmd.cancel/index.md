---
title: "ts.dmd.cancel"
description: "Cancel a demand with a reason."
category: "demand"
requires_auth: true
since: "v0.8.0"
type: docs
---

Cancel a demand with a reason.

**Requires authentication**

## Description

Convenience tool to cancel a demand. Equivalent to calling ts.dmd.update
with status "canceled" and a canceled_reason, but with a simpler interface.

Both id and reason are required.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `id` | string | Yes |  | Demand ID to cancel |
| `reason` | string | Yes |  | Reason for cancellation |

## Response

Returns the demand ID and list of fields that were updated.

```json
{
  "id": "dmd_a1b2c3d4e5f6...",
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
| `not_found` | Demand with this ID does not exist |
| `invalid_input` | Demand ID or reason is missing |
| `invalid_transition` | Demand cannot be canceled from its current status |

## Examples

### Cancel a demand

Cancel a demand that is no longer relevant.

**Request:**

```json
{
  "id": "dmd_a1b2c3d4e5f6...",
  "reason": "No longer needed after scope change"
}
```

**Response:**

```json
{
  "id": "dmd_a1b2c3d4e5f6...",
  "updated_at": "2026-02-16T10:00:00Z",
  "updated_fields": [
    "status",
    "canceled_reason"
  ]
}
```

## Related Tools

- [`ts.dmd.update`](../ts.dmd.update/)
- [`ts.dmd.get`](../ts.dmd.get/)
- [`ts.dmd.list`](../ts.dmd.list/)

