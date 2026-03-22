---
title: "ts.tsk.list"
description: "Query tasks with filters."
category: "task"
requires_auth: true
since: "v0.2.0"
type: docs
---

Query tasks with filters.

**Requires authentication**

## Description

Lists tasks with optional filtering and pagination.

Filters can be combined: for example, list all active tasks in an endeavour
assigned to a specific resource. Text search matches against title and description.

Use summary mode (summary: true) to get task counts grouped by status instead of
individual tasks. This is useful for a quick backlog overview.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `status` | string |  |  | Filter by status: planned, active, done, canceled |
| `endeavour_id` | string |  |  | Filter by endeavour |
| `assignee_id` | string |  |  | Filter by assignee resource |
| `search` | string |  |  | Search in title and description (partial match) |
| `limit` | integer |  | `50` | Maximum number of results to return |
| `offset` | integer |  | `0` | Number of results to skip (for pagination) |
| `summary` | boolean |  | `false` | If true, return status counts instead of individual tasks |

## Response

Returns a paginated list of tasks, or status counts when summary is true.

```json
{
  "limit": 50,
  "offset": 0,
  "tasks": [
    {
      "assignee_id": "res_claude",
      "assignee_name": "Claude",
      "created_at": "2026-02-06T13:37:12Z",
      "endeavour_id": "edv_bd159eb7bb9a877a...",
      "id": "tsk_68e9623ade9b1631...",
      "status": "planned",
      "title": "Implement demand MCP tools",
      "updated_at": "2026-02-06T13:37:12Z"
    }
  ],
  "total": 1
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session. Call ts.auth.login first. |

## Examples

### Summary by status

Get a quick overview of task counts grouped by status.

**Request:**

```json
{
  "endeavour_id": "edv_bd159eb7bb9a877a...",
  "summary": true
}
```

**Response:**

```json
{
  "summary": {
    "active": 1,
    "canceled": 0,
    "done": 10,
    "planned": 5
  },
  "total": 16
}
```

### List tasks by endeavour

Get all tasks belonging to an endeavour.

**Request:**

```json
{
  "endeavour_id": "edv_bd159eb7bb9a877a..."
}
```

**Response:**

```json
{
  "limit": 50,
  "offset": 0,
  "tasks": [
    {
      "id": "tsk_68e9623ade9b1631...",
      "status": "planned",
      "title": "Implement demand MCP tools"
    }
  ],
  "total": 1
}
```

### List active tasks assigned to a resource

Find tasks currently being worked on by a specific agent.

**Request:**

```json
{
  "assignee_id": "res_claude",
  "status": "active"
}
```

**Response:**

```json
{
  "limit": 50,
  "offset": 0,
  "tasks": [
    {
      "assignee_id": "res_claude",
      "assignee_name": "Claude",
      "id": "tsk_55c1a70b3e18cdde...",
      "status": "active",
      "title": "Vertical slice implementation"
    }
  ],
  "total": 1
}
```

## Related Tools

- [`ts.tsk.create`](../ts.tsk.create/)
- [`ts.tsk.get`](../ts.tsk.get/)
- [`ts.tsk.update`](../ts.tsk.update/)

