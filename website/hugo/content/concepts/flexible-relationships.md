---
title: "Flexible Relationships"
description: "Linking entities with typed, directional relationships"
weight: 60
type: docs
---

The Flexible Relationship Model (FRM) allows any entity in Taskschmiede to be related to any other entity. This provides a general-purpose mechanism for expressing dependencies, hierarchies, and associations beyond the built-in entity hierarchy.

## Overview

While the core hierarchy (Organization > Endeavour > Demand > Task) defines the structural containment of entities, relationships express semantic connections between them. Relationships are first-class entities -- they are created, queried, and deleted through the standard tool interface.

## Relation Types

Taskschmiede supports the following relation types:

| Relation | Meaning | Reverse |
|----------|---------|---------|
| `depends_on` | This entity requires another to proceed | `blocks` |
| `blocks` | This entity prevents another from proceeding | `depends_on` |
| `related_to` | A general association between entities | `related_to` |
| `parent_of` | This entity is a logical parent of another | `child_of` |
| `child_of` | This entity is a logical child of another | `parent_of` |

## Bidirectional Creation

When you create a relationship, the system automatically creates the reverse relationship. For example, creating a `depends_on` relation from Task A to Task B also creates a `blocks` relation from Task B to Task A.

This ensures consistency -- you never have a one-sided dependency.

## Creating Relationships

```json
{
  "tool": "ts.rel.create",
  "arguments": {
    "source_type": "task",
    "source_id": "tsk-uuid-a",
    "target_type": "task",
    "target_id": "tsk-uuid-b",
    "relation": "depends_on"
  }
}
```

This creates two records:
- Task A `depends_on` Task B
- Task B `blocks` Task A

## Listing Relationships

Query relationships for a specific entity:

```json
{
  "tool": "ts.rel.list",
  "arguments": {
    "entity_type": "task",
    "entity_id": "tsk-uuid-a"
  }
}
```

This returns all relationships where the specified entity is either the source or the target.

## Deleting Relationships

```json
{
  "tool": "ts.rel.delete",
  "arguments": {
    "id": "rel-uuid"
  }
}
```

Deleting a relationship also deletes its reverse counterpart.

## Cross-Entity Relationships

Relationships are not limited to entities of the same type. You can relate any entity to any other entity:

- **Task to Task** -- express dependencies between work items
- **Demand to Demand** -- link related requirements
- **Task to Demand** -- associate a task with a demand in a different endeavour
- **Endeavour to Endeavour** -- mark cross-project dependencies

### Example: Cross-Endeavour Dependency

A task in Project Alpha depends on a task in Project Beta:

```json
{
  "tool": "ts.rel.create",
  "arguments": {
    "source_type": "task",
    "source_id": "alpha-task-uuid",
    "target_type": "task",
    "target_id": "beta-task-uuid",
    "relation": "depends_on"
  }
}
```

This makes the dependency visible from both projects.

## Use Cases

### Task Dependencies

The most common use of relationships is expressing task dependencies. When Task B cannot start until Task A is complete, create a `depends_on` relationship:

```
Task B  --depends_on-->  Task A
Task A  --blocks-->      Task B  (auto-created)
```

### Demand-to-Task Linking

While tasks naturally belong to a demand through the hierarchy, additional relationships can link tasks to demands in other endeavours. This is useful when a single task addresses requirements from multiple sources.

### Related Items

Use `related_to` for loose associations where there is no dependency but the items are topically connected. This helps with discoverability -- when viewing one item, related items surface as context.

### Hierarchical Grouping

The `parent_of` and `child_of` relations allow you to create ad-hoc hierarchies within the same entity type. For example, you might group tasks into sub-tasks without creating separate demands for each group.

## Design Principles

The FRM follows several design principles:

1. **Any-to-any** -- no restrictions on which entity types can be related
2. **Bidirectional** -- every relationship has a reverse, maintained automatically
3. **Typed** -- relationships have explicit semantics (dependency, association, hierarchy)
4. **Queryable** -- relationships can be listed and filtered per entity
5. **Audited** -- relationship creation and deletion are recorded in the audit trail

## Next Steps

- [Core Concepts]({{< relref "core-concepts" >}}) -- the built-in entity hierarchy
- [Lifecycle and Workflows]({{< relref "lifecycle-and-workflows" >}}) -- how states and transitions work
- [MCP Integration]({{< relref "mcp-integration" >}}) -- using relationships through MCP tools
