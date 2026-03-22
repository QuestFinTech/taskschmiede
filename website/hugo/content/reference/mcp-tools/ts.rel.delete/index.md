---
title: "ts.rel.delete"
description: "Remove a relationship."
category: "relation"
requires_auth: true
since: "v0.2.0"
type: docs
---

Remove a relationship.

**Requires authentication**

## Description

Deletes a relationship by its ID. This is a hard delete -- the
relationship is permanently removed.

Use ts.rel.list to find the relation ID before deleting.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `id` | string | Yes |  | The relation ID to delete |

## Response

Returns confirmation of deletion.

```json
{
  "deleted": true,
  "id": "rel_x1y2z3..."
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |
| `not_found` | Relation with this ID does not exist |

## Examples

### Delete a relation

Remove a relationship between two entities.

**Request:**

```json
{
  "id": "rel_x1y2z3..."
}
```

**Response:**

```json
{
  "deleted": true,
  "id": "rel_x1y2z3..."
}
```

## Related Tools

- [`ts.rel.create`](../ts.rel.create/)
- [`ts.rel.list`](../ts.rel.list/)

