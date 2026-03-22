---
title: "ts.apr.get"
description: "Retrieve an approval by ID."
category: "approval"
requires_auth: true
since: "v0.3.0"
type: docs
---

Retrieve an approval by ID.

**Requires authentication**

## Description

Fetches a single approval record by ID. Returns the full approval
including verdict, role, comment, and metadata.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `id` | string | Yes |  | Approval ID |

## Response

Returns the approval record.

```json
{
  "approver_id": "res_x1y2z3a4b5c6",
  "comment": "All acceptance criteria met.",
  "created_at": "2026-02-12T10:00:00Z",
  "entity_id": "tsk_a1b2c3d4e5f6",
  "entity_type": "task",
  "id": "apr_a1b2c3d4e5f6",
  "role": "reviewer",
  "verdict": "approved"
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |
| `invalid_input` | Approval ID is required |
| `not_found` | Approval with this ID does not exist |

## Examples

### Get an approval

**Request:**

```json
{
  "id": "apr_a1b2c3d4e5f6"
}
```

**Response:**

```json
{
  "approver_id": "res_x1y2z3a4b5c6",
  "created_at": "2026-02-12T10:00:00Z",
  "entity_id": "tsk_a1b2c3d4e5f6",
  "entity_type": "task",
  "id": "apr_a1b2c3d4e5f6",
  "role": "reviewer",
  "verdict": "approved"
}
```

## Related Tools

- [`ts.apr.create`](../ts.apr.create/)
- [`ts.apr.list`](../ts.apr.list/)

