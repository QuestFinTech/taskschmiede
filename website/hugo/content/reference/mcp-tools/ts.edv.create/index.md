---
title: "ts.edv.create"
description: "Create a new endeavour (container for related work toward a goal)."
category: "endeavour"
requires_auth: true
since: "v0.2.0"
type: docs
---

Create a new endeavour (container for related work toward a goal).

**Requires authentication**

## Description

Creates a new endeavour in Taskschmiede.

An endeavour is a container for related work toward a goal -- similar to a
project, sprint, or epic. Endeavours have optional goals, start/end dates,
and aggregate task progress.

After creation, associate the endeavour with an organization using
ts.org.add_endeavour and add team members with ts.edv.add_member.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `name` | string | Yes |  | Endeavour name |
| `description` | string |  |  | Detailed description |
| `goals` | array |  |  | Success criteria or goals (array of strings) |
| `start_date` | string |  |  | Start date (ISO 8601) |
| `end_date` | string |  |  | End date (ISO 8601) |
| `metadata` | object |  |  | Arbitrary key-value pairs |

## Response

Returns the created endeavour summary.

```json
{
  "created_at": "2026-02-06T13:36:32Z",
  "id": "edv_bd159eb7bb9a877a...",
  "name": "Build Taskschmiede",
  "status": "active"
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |
| `invalid_input` | Name is required |

## Examples

### Create an endeavour with goals

Create an endeavour with goals and a start date.

**Request:**

```json
{
  "description": "Develop the agent-first task management system",
  "goals": [
    "Ship v0.2.0",
    "Replace BACKLOG.md"
  ],
  "name": "Build Taskschmiede",
  "start_date": "2026-02-06T00:00:00Z"
}
```

**Response:**

```json
{
  "created_at": "2026-02-06T13:36:32Z",
  "id": "edv_bd159eb7bb9a877a...",
  "name": "Build Taskschmiede",
  "status": "active"
}
```

## Related Tools

- [`ts.edv.get`](../ts.edv.get/)
- [`ts.edv.list`](../ts.edv.list/)
- [`ts.edv.update`](../ts.edv.update/)
- [`ts.org.add_endeavour`](../ts.org.add_endeavour/)
- [`ts.edv.add_member`](../ts.edv.add_member/)

