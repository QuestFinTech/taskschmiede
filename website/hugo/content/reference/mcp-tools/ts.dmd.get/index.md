---
title: "ts.dmd.get"
description: "Retrieve a demand by ID."
category: "demand"
requires_auth: true
since: "v0.2.0"
type: docs
---

Retrieve a demand by ID.

**Requires authentication**

## Description

Retrieves detailed information about a specific demand, including
its type, priority, status, and any linked endeavour.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `id` | string | Yes |  | Demand ID |

## Response

Returns the full demand object.

```json
{
  "created_at": "2026-02-09T10:00:00Z",
  "description": "Users need a dark theme option for reduced eye strain.",
  "id": "dmd_a1b2c3d4e5f6...",
  "metadata": {},
  "priority": "high",
  "status": "open",
  "title": "Add dark mode support",
  "type": "feature",
  "updated_at": "2026-02-09T10:00:00Z"
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |
| `not_found` | Demand with this ID does not exist |

## Examples

### Get demand by ID

**Request:**

```json
{
  "id": "dmd_a1b2c3d4e5f6..."
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
  "type": "feature",
  "updated_at": "2026-02-09T10:00:00Z"
}
```

## Related Tools

- [`ts.dmd.create`](../ts.dmd.create/)
- [`ts.dmd.list`](../ts.dmd.list/)
- [`ts.dmd.update`](../ts.dmd.update/)

