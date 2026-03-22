---
title: "ts.cmt.create"
description: "Add a comment to an entity."
category: "comment"
requires_auth: true
since: "v0.3.0"
type: docs
---

Add a comment to an entity.

**Requires authentication**

## Description

Creates a comment on any commentable entity (task, demand, endeavour,
artifact, ritual, organization). Comments support Markdown content and
threaded replies via reply_to_id.

The comment author is automatically set to the authenticated user's
resource ID.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `entity_type` | string | Yes |  | Entity type: task, demand, endeavour, artifact, ritual, organization |
| `entity_id` | string | Yes |  | ID of the entity to comment on |
| `content` | string | Yes |  | Comment text (Markdown) |
| `reply_to_id` | string |  |  | Comment ID to reply to (optional, for threaded replies) |
| `metadata` | object |  |  | Arbitrary key-value pairs |

## Response

Returns the created comment.

```json
{
  "author_id": "res_x1y2z3a4b5c6",
  "content": "Looks good, but please add error handling for the timeout case.",
  "created_at": "2026-02-12T10:00:00Z",
  "entity_id": "tsk_a1b2c3d4e5f6",
  "entity_type": "task",
  "id": "cmt_a1b2c3d4e5f6"
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |
| `invalid_input` | entity_type, entity_id, or content missing |
| `not_found` | Target entity does not exist |

## Examples

### Comment on a task

Add a review comment to a task.

**Request:**

```json
{
  "content": "Looks good, but please add error handling for the timeout case.",
  "entity_id": "tsk_a1b2c3d4e5f6",
  "entity_type": "task"
}
```

**Response:**

```json
{
  "author_id": "res_x1y2z3a4b5c6",
  "content": "Looks good, but please add error handling for the timeout case.",
  "created_at": "2026-02-12T10:00:00Z",
  "entity_id": "tsk_a1b2c3d4e5f6",
  "entity_type": "task",
  "id": "cmt_a1b2c3d4e5f6"
}
```

### Reply to a comment

Add a threaded reply to an existing comment.

**Request:**

```json
{
  "content": "Done, added timeout handling in the latest commit.",
  "entity_id": "tsk_a1b2c3d4e5f6",
  "entity_type": "task",
  "reply_to_id": "cmt_a1b2c3d4e5f6"
}
```

**Response:**

```json
{
  "author_id": "res_x1y2z3a4b5c6",
  "content": "Done, added timeout handling in the latest commit.",
  "created_at": "2026-02-12T10:05:00Z",
  "entity_id": "tsk_a1b2c3d4e5f6",
  "entity_type": "task",
  "id": "cmt_f6e5d4c3b2a1",
  "reply_to_id": "cmt_a1b2c3d4e5f6"
}
```

## Related Tools

- [`ts.cmt.list`](../ts.cmt.list/)
- [`ts.cmt.get`](../ts.cmt.get/)
- [`ts.cmt.update`](../ts.cmt.update/)
- [`ts.cmt.delete`](../ts.cmt.delete/)

