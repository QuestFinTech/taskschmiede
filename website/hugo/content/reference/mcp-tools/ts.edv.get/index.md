---
title: "ts.edv.get"
description: "Retrieve an endeavour by ID with progress summary."
category: "endeavour"
requires_auth: true
since: "v0.2.0"
type: docs
---

Retrieve an endeavour by ID with progress summary.

**Requires authentication**

## Description

Retrieves detailed information about an endeavour, including a
task progress breakdown showing how many tasks are in each status
(planned, active, done, canceled).

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `id` | string | Yes |  | Endeavour ID |

## Response

Returns the endeavour with task progress counts.

```json
{
  "created_at": "2026-02-06T13:36:32Z",
  "description": "Develop the agent-first task management system",
  "goals": [
    "Ship v0.2.0",
    "Replace BACKLOG.md"
  ],
  "id": "edv_bd159eb7bb9a877a...",
  "name": "Build Taskschmiede",
  "progress": {
    "tasks": {
      "active": 2,
      "canceled": 0,
      "done": 7,
      "planned": 5
    }
  },
  "status": "active",
  "updated_at": "2026-02-06T13:36:32Z"
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |
| `not_found` | Endeavour with this ID does not exist |

## Examples

### Get endeavour with progress

Retrieve an endeavour to see task breakdown.

**Request:**

```json
{
  "id": "edv_bd159eb7bb9a877a..."
}
```

**Response:**

```json
{
  "created_at": "2026-02-06T13:36:32Z",
  "id": "edv_bd159eb7bb9a877a...",
  "name": "Build Taskschmiede",
  "progress": {
    "tasks": {
      "active": 2,
      "canceled": 0,
      "done": 7,
      "planned": 5
    }
  },
  "status": "active",
  "updated_at": "2026-02-06T13:36:32Z"
}
```

## Related Tools

- [`ts.edv.create`](../ts.edv.create/)
- [`ts.edv.list`](../ts.edv.list/)
- [`ts.edv.update`](../ts.edv.update/)
- [`ts.tsk.list`](../ts.tsk.list/)

