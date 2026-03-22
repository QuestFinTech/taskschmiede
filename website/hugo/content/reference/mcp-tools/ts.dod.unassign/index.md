---
title: "ts.dod.unassign"
description: "Remove DoD policy from an endeavour."
category: "dod"
requires_auth: true
since: "v0.3.0"
type: docs
---

Remove DoD policy from an endeavour.

**Requires authentication**

## Description

Removes the currently assigned DoD policy from an endeavour.
Tasks in the endeavour will no longer be subject to DoD checks.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `endeavour_id` | string | Yes |  | Endeavour ID |

## Response

Returns the unassignment confirmation.

```json
{
  "endeavour_id": "edv_a1b2c3d4e5f6",
  "unassigned": true
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |
| `invalid_input` | endeavour_id is required |
| `not_found` | Endeavour does not exist or has no assigned policy |

## Examples

### Remove DoD from endeavour

**Request:**

```json
{
  "endeavour_id": "edv_a1b2c3d4e5f6"
}
```

**Response:**

```json
{
  "endeavour_id": "edv_a1b2c3d4e5f6",
  "unassigned": true
}
```

## Related Tools

- [`ts.dod.assign`](../ts.dod.assign/)
- [`ts.dod.status`](../ts.dod.status/)

