---
title: "ts.dod.override"
description: "Override DoD for a specific task (requires reason)."
category: "dod"
requires_auth: true
since: "v0.3.0"
type: docs
---

Override DoD for a specific task (requires reason).

**Requires authentication**

## Description

Allows a task to be marked as done even if it does not pass all DoD
conditions. A reason is required and recorded for audit purposes.

Use this sparingly -- overrides bypass governance controls and should
be justified (e.g., hotfix, external dependency, scope change).

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `task_id` | string | Yes |  | Task ID |
| `reason` | string | Yes |  | Reason for override (required) |

## Response

Returns the override confirmation.

```json
{
  "overridden": true,
  "override_by": "res_x1y2z3a4b5c6",
  "reason": "Hotfix: customer-blocking issue, approval deferred to post-deploy review.",
  "task_id": "tsk_a1b2c3d4e5f6"
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |
| `invalid_input` | task_id and reason are required |
| `not_found` | Task does not exist |

## Examples

### Override DoD for a hotfix

Allow a task to complete despite failing DoD checks.

**Request:**

```json
{
  "reason": "Hotfix: customer-blocking issue, approval deferred to post-deploy review.",
  "task_id": "tsk_a1b2c3d4e5f6"
}
```

**Response:**

```json
{
  "overridden": true,
  "override_by": "res_x1y2z3a4b5c6",
  "task_id": "tsk_a1b2c3d4e5f6"
}
```

## Related Tools

- [`ts.dod.check`](../ts.dod.check/)
- [`ts.tsk.update`](../ts.tsk.update/)

