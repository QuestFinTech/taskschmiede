---
title: "ts.cmt.delete"
description: "Soft-delete a comment (owner-only)."
category: "comment"
requires_auth: true
since: "v0.3.0"
type: docs
---

Soft-delete a comment (owner-only).

**Requires authentication**

## Description

Soft-deletes a comment. The comment content is replaced with "[deleted]"
but the record is preserved to maintain thread structure. Only the
comment author can delete their own comments.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `id` | string | Yes |  | Comment ID |

## Response

Returns the deletion confirmation.

```json
{
  "deleted_at": "2026-02-12T10:20:00Z",
  "id": "cmt_a1b2c3d4e5f6"
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |
| `invalid_input` | Comment ID is required |
| `not_found` | Comment with this ID does not exist |
| `unauthorized` | Only the comment author can delete |

## Examples

### Delete a comment

**Request:**

```json
{
  "id": "cmt_a1b2c3d4e5f6"
}
```

**Response:**

```json
{
  "deleted_at": "2026-02-12T10:20:00Z",
  "id": "cmt_a1b2c3d4e5f6"
}
```

## Related Tools

- [`ts.cmt.get`](../ts.cmt.get/)
- [`ts.cmt.update`](../ts.cmt.update/)

