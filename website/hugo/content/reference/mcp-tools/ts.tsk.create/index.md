---
title: "ts.tsk.create"
description: "Create a new task (atomic unit of work)."
category: "task"
requires_auth: true
since: "v0.2.0"
type: docs
---

Create a new task (atomic unit of work).

**Requires authentication**

## Description

Creates a new task in Taskschmiede.

Tasks are the atomic units of work. They can optionally belong to an
endeavour and be assigned to a resource (human or agent). New tasks
start in "planned" status.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `title` | string | Yes |  | Task title |
| `description` | string |  |  | Detailed description of the work |
| `endeavour_id` | string |  |  | Endeavour this task belongs to |
| `assignee_id` | string |  |  | Resource ID to assign the task to |
| `estimate` | number |  |  | Estimated hours of work |
| `due_date` | string |  |  | Due date (ISO 8601) |
| `metadata` | object |  |  | Arbitrary key-value pairs (e.g., type, priority, tags) |

## Response

Returns the created task summary.

```json
{
  "assignee_id": "res_claude",
  "created_at": "2026-02-06T13:37:12Z",
  "endeavour_id": "edv_bd159eb7bb9a877a...",
  "id": "tsk_68e9623ade9b1631...",
  "status": "planned",
  "title": "Implement demand MCP tools"
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |
| `invalid_input` | Title is required |

## Examples

### Create a task in an endeavour

Create a task assigned to an agent within an endeavour.

**Request:**

```json
{
  "assignee_id": "res_claude",
  "description": "Implement CRUD tools for the demand entity",
  "endeavour_id": "edv_bd159eb7bb9a877a...",
  "metadata": {
    "type": "feature"
  },
  "title": "Implement demand MCP tools"
}
```

**Response:**

```json
{
  "assignee_id": "res_claude",
  "created_at": "2026-02-06T13:37:12Z",
  "endeavour_id": "edv_bd159eb7bb9a877a...",
  "id": "tsk_68e9623ade9b1631...",
  "status": "planned",
  "title": "Implement demand MCP tools"
}
```

## Related Tools

- [`ts.tsk.get`](../ts.tsk.get/)
- [`ts.tsk.list`](../ts.tsk.list/)
- [`ts.tsk.update`](../ts.tsk.update/)
- [`ts.tsk.cancel`](../ts.tsk.cancel/)

