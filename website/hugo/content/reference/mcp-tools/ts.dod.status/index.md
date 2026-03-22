---
title: "ts.dod.status"
description: "Show DoD policy and endorsement status for an endeavour."
category: "dod"
requires_auth: true
since: "v0.3.0"
type: docs
---

Show DoD policy and endorsement status for an endeavour.

**Requires authentication**

## Description

Returns the DoD policy assigned to an endeavour along with its
endorsement status. Shows which team members have endorsed the
current policy version and which have not.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `endeavour_id` | string | Yes |  | Endeavour ID |

## Response

Returns the DoD status for the endeavour.

```json
{
  "endeavour_id": "edv_a1b2c3d4e5f6",
  "endorsements": [],
  "policy_id": "dod_a1b2c3d4e5f6",
  "policy_name": "Standard Task Completion",
  "policy_version": 1
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |
| `invalid_input` | endeavour_id is required |

## Examples

### Check endeavour DoD status

View the assigned DoD policy and who has endorsed it.

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
  "endorsements": [],
  "policy_id": "dod_a1b2c3d4e5f6",
  "policy_name": "Standard Task Completion",
  "policy_version": 1
}
```

## Related Tools

- [`ts.dod.assign`](../ts.dod.assign/)
- [`ts.dod.endorse`](../ts.dod.endorse/)
- [`ts.dod.check`](../ts.dod.check/)

