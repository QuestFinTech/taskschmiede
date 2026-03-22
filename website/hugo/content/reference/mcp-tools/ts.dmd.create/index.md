---
title: "ts.dmd.create"
description: "Create a new demand (what needs to be fulfilled)."
category: "demand"
requires_auth: true
since: "v0.2.0"
type: docs
---

Create a new demand (what needs to be fulfilled).

**Requires authentication**

## Description

Creates a new demand in Taskschmiede.

A demand represents what needs to be fulfilled -- a feature request, a bug report,
a goal, or any other need. Demands are distinct from tasks: a demand captures the
"what" while tasks capture the "how". A single demand may result in multiple tasks.

Demands start in "open" status and can optionally be linked to an endeavour.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `type` | string | Yes |  | Demand type (e.g., feature, bug, goal, meeting, epic) |
| `title` | string | Yes |  | Demand title |
| `description` | string |  |  | Detailed description |
| `priority` | string |  | `medium` | Priority: low, medium (default), high, urgent |
| `endeavour_id` | string |  |  | Endeavour this demand belongs to |
| `due_date` | string |  |  | Due date (ISO 8601) |
| `metadata` | object |  |  | Arbitrary key-value pairs |

## Response

Returns the created demand summary.

```json
{
  "created_at": "2026-02-09T10:00:00Z",
  "id": "dmd_a1b2c3d4e5f6...",
  "priority": "high",
  "status": "open",
  "title": "Add dark mode support",
  "type": "feature"
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |
| `invalid_input` | Type and title are required |

## Examples

### Create a feature demand

Record a new feature request linked to an endeavour.

**Request:**

```json
{
  "description": "Users need a dark theme option for reduced eye strain.",
  "endeavour_id": "edv_bd159eb7bb9a877a...",
  "priority": "high",
  "title": "Add dark mode support",
  "type": "feature"
}
```

**Response:**

```json
{
  "created_at": "2026-02-09T10:00:00Z",
  "id": "dmd_a1b2c3d4e5f6...",
  "priority": "high",
  "status": "open",
  "title": "Add dark mode support",
  "type": "feature"
}
```

### Create a bug demand

Report a bug with urgent priority.

**Request:**

```json
{
  "priority": "urgent",
  "title": "Login fails on mobile browsers",
  "type": "bug"
}
```

**Response:**

```json
{
  "created_at": "2026-02-09T10:00:00Z",
  "id": "dmd_f6e5d4c3b2a1...",
  "priority": "urgent",
  "status": "open",
  "title": "Login fails on mobile browsers",
  "type": "bug"
}
```

## Related Tools

- [`ts.dmd.get`](../ts.dmd.get/)
- [`ts.dmd.list`](../ts.dmd.list/)
- [`ts.dmd.update`](../ts.dmd.update/)
- [`ts.dmd.cancel`](../ts.dmd.cancel/)

