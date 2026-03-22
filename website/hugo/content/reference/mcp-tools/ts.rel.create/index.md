---
title: "ts.rel.create"
description: "Create a relationship between two entities."
category: "relation"
requires_auth: true
since: "v0.2.0"
type: docs
---

Create a relationship between two entities.

**Requires authentication**

## Description

Creates a typed, directed relationship between any two entities in Taskschmiede.

This is the core of the Flexible Relationship Model (FRM). Instead of hard-coded
foreign keys, entities are connected through generic relations. Common relationship
types include:
- belongs_to: task belongs_to endeavour
- assigned_to: task assigned_to resource
- has_member: organization has_member resource
- governs: ritual governs endeavour
- uses: task uses artifact

Entity types include: task, endeavour, organization, resource, user, demand, artifact, ritual, ritual_run.

Relations can carry metadata (e.g., a role on a membership relation).

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `relationship_type` | string | Yes |  | Relationship type (e.g., belongs_to, assigned_to, has_member, governs, uses) |
| `source_entity_type` | string | Yes |  | Source entity type (e.g., task, organization, user, ritual) |
| `source_entity_id` | string | Yes |  | Source entity ID |
| `target_entity_type` | string | Yes |  | Target entity type (e.g., endeavour, resource, artifact) |
| `target_entity_id` | string | Yes |  | Target entity ID |
| `metadata` | object |  |  | Optional metadata on the relationship (e.g., role) |

## Response

Returns the created relation.

```json
{
  "created_at": "2026-02-09T10:00:00Z",
  "id": "rel_x1y2z3...",
  "metadata": {},
  "relationship_type": "governs",
  "source_entity_id": "rtl_a1b2c3d4e5f6...",
  "source_entity_type": "ritual",
  "target_entity_id": "edv_bd159eb7bb9a877a...",
  "target_entity_type": "endeavour"
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |
| `invalid_input` | All five required fields must be provided |

## Examples

### Link a ritual to an endeavour

Create a governs relation so the ritual applies to the endeavour.

**Request:**

```json
{
  "relationship_type": "governs",
  "source_entity_id": "rtl_a1b2c3d4e5f6...",
  "source_entity_type": "ritual",
  "target_entity_id": "edv_bd159eb7bb9a877a...",
  "target_entity_type": "endeavour"
}
```

**Response:**

```json
{
  "created_at": "2026-02-09T10:00:00Z",
  "id": "rel_x1y2z3...",
  "relationship_type": "governs",
  "source_entity_id": "rtl_a1b2c3d4e5f6...",
  "source_entity_type": "ritual",
  "target_entity_id": "edv_bd159eb7bb9a877a...",
  "target_entity_type": "endeavour"
}
```

### Link a task to an artifact

Record that a task uses an artifact.

**Request:**

```json
{
  "relationship_type": "uses",
  "source_entity_id": "tsk_68e9623ade9b1631...",
  "source_entity_type": "task",
  "target_entity_id": "art_d4e5f6a1b2c3...",
  "target_entity_type": "artifact"
}
```

**Response:**

```json
{
  "created_at": "2026-02-09T10:00:00Z",
  "id": "rel_a2b3c4...",
  "relationship_type": "uses",
  "source_entity_id": "tsk_68e9623ade9b1631...",
  "source_entity_type": "task",
  "target_entity_id": "art_d4e5f6a1b2c3...",
  "target_entity_type": "artifact"
}
```

## Related Tools

- [`ts.rel.list`](../ts.rel.list/)
- [`ts.rel.delete`](../ts.rel.delete/)

