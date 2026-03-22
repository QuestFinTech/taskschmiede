---
title: "ts.tsk.get"
description: "Retrieve a task by ID."
category: "task"
requires_auth: true
since: "v0.2.0"
type: docs
---

Retrieve a task by ID.

**Requires authentication**

## Description

Retrieves detailed information about a specific task.

The response includes the assignee's display name (if assigned) and
lifecycle timestamps (started_at, completed_at, canceled_at) when applicable.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `id` | string | Yes |  | Task ID |

## Response

Returns the full task object.

```json
{
  "assignee_id": "res_claude",
  "assignee_name": "Claude",
  "created_at": "2026-02-06T13:37:12Z",
  "description": "Implement CRUD tools for the demand entity",
  "endeavour_id": "edv_bd159eb7bb9a877a...",
  "estimate": 4,
  "id": "tsk_68e9623ade9b1631...",
  "metadata": {
    "type": "feature"
  },
  "started_at": "2026-02-06T14:00:00Z",
  "status": "active",
  "title": "Implement demand MCP tools",
  "updated_at": "2026-02-06T14:00:00Z"
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |
| `not_found` | Task with this ID does not exist |

## Related Tools

- [`ts.tsk.create`](../ts.tsk.create/)
- [`ts.tsk.list`](../ts.tsk.list/)
- [`ts.tsk.update`](../ts.tsk.update/)

