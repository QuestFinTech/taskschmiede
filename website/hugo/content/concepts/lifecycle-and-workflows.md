---
title: "Lifecycles"
description: "State machines for demands, tasks, and approvals"
weight: 50
type: docs
---

Every major entity in Taskschmiede has a defined lifecycle with clear states and transitions. This page documents the state machines, workflow patterns, and supporting features like Definition of Done policies and approvals.

## Demand-to-Task Workflow

The primary workflow in Taskschmiede flows from demands down to tasks:

1. A **demand** is drafted to capture a requirement or user story
2. The demand is opened when it is ready for work
3. One or more **tasks** are created to fulfill the demand
4. Tasks are worked on, reviewed, and completed
5. When all tasks are done, the demand is marked as fulfilled

This pattern ensures traceability from high-level requirements down to individual work items.

## Task States and Transitions

```
open --> in_progress --> review --> completed
  |         |             |
  +---------+-------------+--> cancelled
```

| State | Description |
|-------|-------------|
| `open` | Created and ready to be picked up. No work has started. |
| `in_progress` | Actively being worked on by an assignee. |
| `review` | Work is complete and awaiting review or approval. |
| `completed` | Reviewed and accepted. The task is done. |
| `cancelled` | Abandoned. The task will not be completed. |

### Valid Transitions

| From | To |
|------|----|
| `open` | `in_progress`, `cancelled` |
| `in_progress` | `review`, `open`, `cancelled` |
| `review` | `completed`, `in_progress`, `cancelled` |

Tasks cannot skip states (for example, moving directly from `open` to `completed`). Moving a task back from `review` to `in_progress` is allowed when changes are requested during review.

## Demand States and Transitions

```
draft --> open --> in_progress --> fulfilled
  |        |          |
  +--------+----------+--> cancelled
```

| State | Description |
|-------|-------------|
| `draft` | Being written. Not yet ready for work. |
| `open` | Defined and ready for tasks to be created. |
| `in_progress` | Tasks are actively being worked on. |
| `fulfilled` | All associated tasks are complete. The requirement is satisfied. |
| `cancelled` | Dropped. The demand will not be fulfilled. |

## Endeavour States and Transitions

```
planning --> active --> completed --> archived
```

| State | Description |
|-------|-------------|
| `planning` | The project is being scoped and defined. |
| `active` | Work is underway. Demands and tasks are being created and completed. |
| `completed` | All demands have been fulfilled. The project objectives are met. |
| `archived` | Closed and retained for historical reference. |

## Definition of Done (DoD)

Definition of Done policies define the criteria that must be met before an entity can transition to a completed state. DoD policies can be assigned to endeavours, demands, or tasks.

### Creating a DoD

```json
{
  "tool": "ts.dod.create",
  "arguments": {
    "title": "Code Review Required",
    "description": "All code changes must be reviewed by at least one other team member",
    "check_type": "manual"
  }
}
```

### Assigning a DoD

```json
{
  "tool": "ts.dod.assign",
  "arguments": {
    "dod_id": "dod-uuid",
    "entity_type": "task",
    "entity_id": "tsk-uuid"
  }
}
```

### Checking DoD Status

Before completing a task or demand, check whether all DoD criteria are met:

```json
{
  "tool": "ts.dod.check",
  "arguments": {
    "entity_type": "task",
    "entity_id": "tsk-uuid"
  }
}
```

### DoD Versioning

DoD policies support versioning. When a policy needs to be updated, a new version can be created while preserving the original. Entities that were evaluated against an older version retain that reference for auditability.

```json
{
  "tool": "ts.dod.new_version",
  "arguments": {
    "id": "dod-uuid",
    "description": "Updated to require two reviewers instead of one"
  }
}
```

### DoD Endorsement and Override

- **Endorsement** -- a reviewer confirms that a DoD criterion has been met
- **Override** -- an admin bypasses a DoD criterion with a documented reason

Both actions are recorded in the audit trail.

## Approvals

Approvals provide a formal review mechanism for entities. An approval request specifies a quorum -- the number of endorsements required before the entity can proceed.

```json
{
  "tool": "ts.apr.create",
  "arguments": {
    "entity_type": "demand",
    "entity_id": "dmd-uuid",
    "quorum": 2
  }
}
```

When the required number of approvers endorse the entity, the approval is satisfied and the entity can transition to the next state.

## Rituals

Rituals are recurring processes driven by templates. A ritual template defines a repeatable workflow that can be instantiated on a schedule or on demand. Examples include:

- Weekly planning meetings
- Sprint retrospectives
- Release checklists
- Onboarding procedures

### Ritual Templates

A ritual template defines the structure of a recurring process:

```json
{
  "tool": "ts.rtl.create",
  "arguments": {
    "name": "Weekly Planning",
    "description": "Weekly team planning ritual",
    "template_id": "tpl-uuid"
  }
}
```

Templates can be forked to create variations, and their lineage is tracked for auditability.

### Templates

Templates define reusable structures that can be instantiated as rituals, tasks, or other entities:

```json
{
  "tool": "ts.tpl.create",
  "arguments": {
    "name": "Sprint Retrospective Template",
    "description": "Standard retrospective with what went well, what to improve, and action items"
  }
}
```

Templates support versioning and forking, allowing teams to evolve their processes while maintaining a history of changes.

## Audit Trail

All state transitions, approvals, endorsements, and overrides are recorded in the audit trail. This provides a complete history of how entities moved through their lifecycle:

```json
{
  "tool": "ts.audit.entity_changes",
  "arguments": {
    "entity_type": "task",
    "entity_id": "tsk-uuid"
  }
}
```

The audit trail is immutable and cannot be modified or deleted.

## Next Steps

- [Core Concepts]({{< relref "core-concepts" >}}) -- entity hierarchy and terminology
- [Flexible Relationship Model]({{< relref "flexible-relationships" >}}) -- linking entities across the hierarchy
- [Organizations and Teams]({{< relref "organizations-and-teams" >}}) -- member roles and permissions
