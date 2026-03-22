---
title: "ts.apr.list"
description: "List approvals for an entity."
category: "approval"
requires_auth: true
since: "v0.3.0"
type: docs
---

List approvals for an entity.

**Requires authentication**

## Description

Lists approval records for a specific entity, newest first.
Supports filtering by approver, verdict, and role.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `entity_type` | string | Yes |  | Entity type: task, demand, endeavour, artifact |
| `entity_id` | string | Yes |  | ID of the entity |
| `approver_id` | string |  |  | Filter by approver resource ID |
| `verdict` | string |  |  | Filter by verdict: approved, rejected, needs_work |
| `role` | string |  |  | Filter by role |
| `limit` | integer |  | `50` | Max results (default: 50) |
| `offset` | integer |  | `0` | Pagination offset |

## Response

Returns a paginated list of approvals.

```json
{
  "approvals": [
    {
      "approver_id": "res_x1y2z3a4b5c6",
      "id": "apr_a1b2c3d4e5f6",
      "verdict": "approved"
    }
  ],
  "limit": 50,
  "offset": 0,
  "total": 1
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |
| `invalid_input` | entity_type or entity_id missing |

## Examples

### List task approvals

Get all approvals for a task.

**Request:**

```json
{
  "entity_id": "tsk_a1b2c3d4e5f6",
  "entity_type": "task"
}
```

**Response:**

```json
{
  "approvals": [
    {
      "id": "apr_a1b2c3d4e5f6",
      "verdict": "approved"
    }
  ],
  "limit": 50,
  "offset": 0,
  "total": 1
}
```

## Related Tools

- [`ts.apr.create`](../ts.apr.create/)
- [`ts.apr.get`](../ts.apr.get/)

