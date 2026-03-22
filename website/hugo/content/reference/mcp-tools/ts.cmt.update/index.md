---
title: "ts.cmt.update"
description: "Edit a comment (owner-only)."
category: "comment"
requires_auth: true
since: "v0.3.0"
type: docs
---

Edit a comment (owner-only).

**Requires authentication**

## Description

Updates the content or metadata of a comment. Only the comment author
can edit their own comments. Editing sets the edited_at timestamp
to track that the comment was modified.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `id` | string | Yes |  | Comment ID |
| `content` | string |  |  | New comment text |
| `metadata` | object |  |  | New metadata (replaces existing) |

## Response

Returns the updated comment.

```json
{
  "content": "Updated review comment with more detail.",
  "edited_at": "2026-02-12T10:15:00Z",
  "id": "cmt_a1b2c3d4e5f6"
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |
| `invalid_input` | Comment ID is required |
| `not_found` | Comment with this ID does not exist |
| `unauthorized` | Only the comment author can edit |

## Examples

### Edit a comment

Update the content of a comment you authored.

**Request:**

```json
{
  "content": "Updated review comment with more detail.",
  "id": "cmt_a1b2c3d4e5f6"
}
```

**Response:**

```json
{
  "content": "Updated review comment with more detail.",
  "edited_at": "2026-02-12T10:15:00Z",
  "id": "cmt_a1b2c3d4e5f6"
}
```

## Related Tools

- [`ts.cmt.get`](../ts.cmt.get/)
- [`ts.cmt.delete`](../ts.cmt.delete/)

