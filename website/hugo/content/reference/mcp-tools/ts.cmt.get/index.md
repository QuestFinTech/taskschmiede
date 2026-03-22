---
title: "ts.cmt.get"
description: "Retrieve a comment by ID, including its direct replies."
category: "comment"
requires_auth: true
since: "v0.3.0"
type: docs
---

Retrieve a comment by ID, including its direct replies.

**Requires authentication**

## Description

Fetches a single comment by ID. The response includes the comment's
direct replies (one level deep) for convenient thread viewing.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `id` | string | Yes |  | Comment ID |

## Response

Returns the comment with its direct replies.

```json
{
  "author_id": "res_x1y2z3a4b5c6",
  "content": "Review comment",
  "created_at": "2026-02-12T10:00:00Z",
  "id": "cmt_a1b2c3d4e5f6",
  "replies": []
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |
| `invalid_input` | Comment ID is required |
| `not_found` | Comment with this ID does not exist |

## Examples

### Get a comment

**Request:**

```json
{
  "id": "cmt_a1b2c3d4e5f6"
}
```

**Response:**

```json
{
  "author_id": "res_x1y2z3a4b5c6",
  "content": "Review comment",
  "created_at": "2026-02-12T10:00:00Z",
  "id": "cmt_a1b2c3d4e5f6",
  "replies": []
}
```

## Related Tools

- [`ts.cmt.list`](../ts.cmt.list/)
- [`ts.cmt.update`](../ts.cmt.update/)
- [`ts.cmt.delete`](../ts.cmt.delete/)

