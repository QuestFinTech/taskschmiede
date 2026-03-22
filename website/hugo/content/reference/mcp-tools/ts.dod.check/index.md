---
title: "ts.dod.check"
description: "Evaluate DoD conditions for a task (dry run)."
category: "dod"
requires_auth: true
since: "v0.3.0"
type: docs
---

Evaluate DoD conditions for a task (dry run).

**Requires authentication**

## Description

Checks whether a task meets the DoD conditions defined by the policy
assigned to the task's endeavour. This is a dry run -- it does not
modify the task or block status transitions.

Returns the evaluation result for each condition, showing which
conditions pass and which fail, along with the overall verdict.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `task_id` | string | Yes |  | Task ID to check |

## Response

Returns the DoD check results.

```json
{
  "pass": false,
  "policy_id": "dod_a1b2c3d4e5f6",
  "results": [
    {
      "id": "c1",
      "label": "At least one approval",
      "pass": true
    },
    {
      "id": "c2",
      "label": "Actual hours logged",
      "pass": false,
      "reason": "field 'actual' is not set"
    }
  ],
  "task_id": "tsk_a1b2c3d4e5f6"
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |
| `invalid_input` | task_id is required |
| `not_found` | Task does not exist or has no DoD policy |

## Examples

### Check task DoD

Evaluate whether a task meets its DoD conditions.

**Request:**

```json
{
  "task_id": "tsk_a1b2c3d4e5f6"
}
```

**Response:**

```json
{
  "pass": false,
  "policy_id": "dod_a1b2c3d4e5f6",
  "results": [
    {
      "id": "c1",
      "label": "At least one approval",
      "pass": true
    },
    {
      "id": "c2",
      "label": "Actual hours logged",
      "pass": false
    }
  ],
  "task_id": "tsk_a1b2c3d4e5f6"
}
```

## Related Tools

- [`ts.dod.override`](../ts.dod.override/)
- [`ts.dod.status`](../ts.dod.status/)
- [`ts.tsk.update`](../ts.tsk.update/)

