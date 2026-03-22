---
title: "ts.dod.assign"
description: "Assign a DoD policy to an endeavour."
category: "dod"
requires_auth: true
since: "v0.3.0"
type: docs
---

Assign a DoD policy to an endeavour.

**Requires authentication**

## Description

Assigns a DoD policy to govern task completion within an endeavour.
Each endeavour can have at most one active DoD policy. Assigning a
new policy replaces the previous one.

After assignment, team members should endorse the policy via
ts.dod.endorse to signal agreement.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `endeavour_id` | string | Yes |  | Endeavour ID |
| `policy_id` | string | Yes |  | DoD policy ID |

## Response

Returns the assignment confirmation.

```json
{
  "assigned_at": "2026-02-12T10:00:00Z",
  "endeavour_id": "edv_a1b2c3d4e5f6",
  "policy_id": "dod_a1b2c3d4e5f6"
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |
| `invalid_input` | endeavour_id and policy_id are required |
| `not_found` | Endeavour or policy does not exist |

## Examples

### Assign DoD to endeavour

Set the DoD policy for an endeavour.

**Request:**

```json
{
  "endeavour_id": "edv_a1b2c3d4e5f6",
  "policy_id": "dod_a1b2c3d4e5f6"
}
```

**Response:**

```json
{
  "assigned_at": "2026-02-12T10:00:00Z",
  "endeavour_id": "edv_a1b2c3d4e5f6",
  "policy_id": "dod_a1b2c3d4e5f6"
}
```

## Related Tools

- [`ts.dod.unassign`](../ts.dod.unassign/)
- [`ts.dod.endorse`](../ts.dod.endorse/)
- [`ts.dod.status`](../ts.dod.status/)

