---
title: "ts.cmt.list"
description: "List comments on an entity."
category: "comment"
requires_auth: true
since: "v0.3.0"
type: docs
---

List comments on an entity.

**Requires authentication**

## Description

Lists comments on a specific entity in chronological order (oldest first).
Supports filtering by author and pagination.

Soft-deleted comments appear as placeholders with content replaced by
"[deleted]" to preserve thread structure.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `entity_type` | string | Yes |  | Entity type: task, demand, endeavour, artifact, ritual, organization |
| `entity_id` | string | Yes |  | ID of the entity |
| `author_id` | string |  |  | Filter by author resource ID |
| `limit` | integer |  | `50` | Max results (default: 50) |
| `offset` | integer |  | `0` | Pagination offset |

## Response

Returns a paginated list of comments.

```json
{
  "comments": [
    {
      "author_id": "res_x1y2z3a4b5c6",
      "content": "First comment",
      "id": "cmt_a1b2c3d4e5f6"
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

### List task comments

Get all comments on a task.

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
  "comments": [
    {
      "content": "Looks good!",
      "id": "cmt_a1b2c3d4e5f6"
    }
  ],
  "limit": 50,
  "offset": 0,
  "total": 1
}
```

## Related Tools

- [`ts.cmt.create`](../ts.cmt.create/)
- [`ts.cmt.get`](../ts.cmt.get/)

