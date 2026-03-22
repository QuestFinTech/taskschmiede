---
title: "ts.art.update"
description: "Update artifact attributes (partial update)."
category: "artifact"
requires_auth: true
since: "v0.2.0"
type: docs
---

Update artifact attributes (partial update).

**Requires authentication**

## Description

Updates one or more fields of an existing artifact. Only provided fields
are changed; omitted fields remain unchanged.

To archive an artifact, set status to "archived". To unlink from an endeavour
or task, pass an empty string.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `id` | string | Yes |  | Artifact ID |
| `title` | string |  |  | New title |
| `kind` | string |  |  | New kind |
| `url` | string |  |  | New URL |
| `summary` | string |  |  | New summary |
| `tags` | array |  |  | New tags (replaces existing) |
| `status` | string |  |  | New status: active, archived |
| `endeavour_id` | string |  |  | New endeavour (empty string to unlink) |
| `task_id` | string |  |  | New task (empty string to unlink) |
| `metadata` | object |  |  | Metadata to set (replaces existing) |

## Response

Returns the artifact ID and list of fields that were updated.

```json
{
  "id": "art_d4e5f6a1b2c3...",
  "updated_at": "2026-02-09T12:00:00Z",
  "updated_fields": [
    "status"
  ]
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |
| `not_found` | Artifact with this ID does not exist |
| `invalid_input` | No fields to update |

## Examples

### Archive an artifact

Mark an artifact as archived.

**Request:**

```json
{
  "id": "art_d4e5f6a1b2c3...",
  "status": "archived"
}
```

**Response:**

```json
{
  "id": "art_d4e5f6a1b2c3...",
  "updated_at": "2026-02-09T12:00:00Z",
  "updated_fields": [
    "status"
  ]
}
```

### Update tags

Replace the tag set on an artifact.

**Request:**

```json
{
  "id": "art_d4e5f6a1b2c3...",
  "tags": [
    "architecture",
    "decision-record",
    "approved"
  ]
}
```

**Response:**

```json
{
  "id": "art_d4e5f6a1b2c3...",
  "updated_at": "2026-02-09T12:00:00Z",
  "updated_fields": [
    "tags"
  ]
}
```

## Related Tools

- [`ts.art.create`](../ts.art.create/)
- [`ts.art.get`](../ts.art.get/)
- [`ts.art.list`](../ts.art.list/)

