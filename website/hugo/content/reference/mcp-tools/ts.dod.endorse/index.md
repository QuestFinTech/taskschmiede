---
title: "ts.dod.endorse"
description: "Endorse the current DoD policy for an endeavour."
category: "dod"
requires_auth: true
since: "v0.3.0"
type: docs
---

Endorse the current DoD policy for an endeavour.

**Requires authentication**

## Description

Records the caller's endorsement of the DoD policy currently assigned
to an endeavour. Endorsement signals that the team member agrees with
the policy conditions and will follow them.

Endorsements are tracked per policy version. If the policy is
reassigned, previous endorsements are superseded.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `endeavour_id` | string | Yes |  | Endeavour ID |

## Response

Returns the endorsement record.

```json
{
  "endeavour_id": "edv_a1b2c3d4e5f6",
  "endorsed_at": "2026-02-12T10:00:00Z",
  "id": "end_a1b2c3d4e5f6",
  "policy_id": "dod_a1b2c3d4e5f6",
  "resource_id": "res_x1y2z3a4b5c6"
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |
| `invalid_input` | endeavour_id is required |
| `not_found` | Endeavour has no assigned DoD policy |
| `conflict` | Already endorsed the current policy version |

## Examples

### Endorse a DoD policy

Signal agreement with the assigned DoD policy.

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
  "endorsed_at": "2026-02-12T10:00:00Z",
  "id": "end_a1b2c3d4e5f6",
  "policy_id": "dod_a1b2c3d4e5f6",
  "resource_id": "res_x1y2z3a4b5c6"
}
```

## Related Tools

- [`ts.dod.assign`](../ts.dod.assign/)
- [`ts.dod.status`](../ts.dod.status/)

