---
title: "ts.apr.create"
description: "Record an approval on an entity."
category: "approval"
requires_auth: true
since: "v0.3.0"
type: docs
---

Record an approval on an entity.

**Requires authentication**

## Description

Records an approval decision on an entity (task, demand, endeavour,
artifact). Approvals are immutable -- once created, they cannot be
modified or deleted. This provides an auditable sign-off trail.

Three verdict types are supported:
- approved: work meets requirements
- rejected: work does not meet requirements
- needs_work: work requires changes before approval

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `entity_type` | string | Yes |  | Entity type: task, demand, endeavour, artifact |
| `entity_id` | string | Yes |  | ID of the entity being approved |
| `verdict` | string | Yes |  | Verdict: approved, rejected, needs_work |
| `role` | string |  |  | Role under which approval is given (e.g., reviewer, product_owner) |
| `comment` | string |  |  | Optional rationale or feedback |
| `metadata` | object |  |  | Arbitrary key-value pairs (e.g., checklist results, linked artifacts) |

## Response

Returns the created approval record.

```json
{
  "approver_id": "res_x1y2z3a4b5c6",
  "comment": "All acceptance criteria met. Good to ship.",
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
| `invalid_input` | entity_type, entity_id, or verdict missing |
| `invalid_input` | Invalid verdict value |
| `not_found` | Target entity does not exist |

## Examples

### Approve a task

Record an approval decision on a task.

**Request:**

```json
{
  "comment": "All acceptance criteria met. Good to ship.",
  "entity_id": "tsk_a1b2c3d4e5f6",
  "entity_type": "task",
  "role": "reviewer",
  "verdict": "approved"
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
  "verdict": "approved"
}
```

### Request changes

Record a needs_work verdict with feedback.

**Request:**

```json
{
  "comment": "Missing test coverage for edge cases.",
  "entity_id": "tsk_a1b2c3d4e5f6",
  "entity_type": "task",
  "verdict": "needs_work"
}
```

**Response:**

```json
{
  "approver_id": "res_x1y2z3a4b5c6",
  "created_at": "2026-02-12T10:05:00Z",
  "entity_id": "tsk_a1b2c3d4e5f6",
  "entity_type": "task",
  "id": "apr_f6e5d4c3b2a1",
  "verdict": "needs_work"
}
```

## Related Tools

- [`ts.apr.list`](../ts.apr.list/)
- [`ts.apr.get`](../ts.apr.get/)

