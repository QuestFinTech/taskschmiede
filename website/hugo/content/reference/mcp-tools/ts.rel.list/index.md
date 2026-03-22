---
title: "ts.rel.list"
description: "Query relationships (by source, target, or type)."
category: "relation"
requires_auth: true
since: "v0.2.0"
type: docs
---

Query relationships (by source, target, or type).

**Requires authentication**

## Description

Lists relationships with optional filtering. You can filter by
source entity, target entity, relationship type, or any combination.

This is how you discover what is connected to what. For example:
- Find all tasks assigned to a resource
- Find all rituals governing an endeavour
- Find all artifacts used by a task

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `source_entity_type` | string |  |  | Filter by source entity type |
| `source_entity_id` | string |  |  | Filter by source entity ID |
| `target_entity_type` | string |  |  | Filter by target entity type |
| `target_entity_id` | string |  |  | Filter by target entity ID |
| `relationship_type` | string |  |  | Filter by relationship type (e.g., governs, uses, assigned_to) |
| `limit` | integer |  | `50` | Maximum number of results to return |
| `offset` | integer |  | `0` | Number of results to skip (for pagination) |

## Response

Returns a paginated list of relations.

```json
{
  "limit": 50,
  "offset": 0,
  "relations": [
    {
      "created_at": "2026-02-09T10:00:00Z",
      "id": "rel_x1y2z3...",
      "relationship_type": "governs",
      "source_entity_id": "rtl_a1b2c3d4e5f6...",
      "source_entity_type": "ritual",
      "target_entity_id": "edv_bd159eb7bb9a877a...",
      "target_entity_type": "endeavour"
    }
  ],
  "total": 1
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |

## Examples

### Find rituals governing an endeavour

List all governs relations targeting a specific endeavour.

**Request:**

```json
{
  "relationship_type": "governs",
  "target_entity_id": "edv_bd159eb7bb9a877a...",
  "target_entity_type": "endeavour"
}
```

**Response:**

```json
{
  "limit": 50,
  "offset": 0,
  "relations": [
    {
      "id": "rel_x1y2z3...",
      "relationship_type": "governs",
      "source_entity_id": "rtl_a1b2c3d4e5f6...",
      "source_entity_type": "ritual",
      "target_entity_id": "edv_bd159eb7bb9a877a...",
      "target_entity_type": "endeavour"
    }
  ],
  "total": 1
}
```

## Related Tools

- [`ts.rel.create`](../ts.rel.create/)
- [`ts.rel.delete`](../ts.rel.delete/)

