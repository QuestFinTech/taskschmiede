---
title: "Definition of Done"
description: "Set completion criteria for tasks and demands"
weight: 110
type: docs
---

Definition of Done (DoD) policies define completion criteria for tasks within an endeavour. This guide covers creating, assigning, endorsing, and checking DoD policies.

## Create a DoD Policy

```json
{
  "tool": "ts.dod.create",
  "arguments": {
    "name": "Standard Completion Criteria",
    "description": "Minimum requirements for task completion",
    "conditions": [
      {"type": "field_set", "field": "actual", "label": "Actual hours recorded"},
      {"type": "field_set", "field": "description", "label": "Description provided"},
      {"type": "approval_received", "params": {"verdict": "approved", "role": "reviewer"}, "label": "Peer review approved"}
    ],
    "strictness": "all"
  }
}
```

**Strictness modes:**

- `all` -- every condition must pass for the task to be considered done.
- `n_of` -- at least `quorum` conditions must pass.

For quorum-based policies:

```json
{
  "tool": "ts.dod.create",
  "arguments": {
    "name": "Flexible Criteria",
    "conditions": [
      {"type": "field_set", "field": "actual", "label": "Hours recorded"},
      {"type": "field_set", "field": "description", "label": "Description"},
      {"type": "approval_received", "params": {"verdict": "approved"}, "label": "Approved"}
    ],
    "strictness": "n_of",
    "quorum": 2
  }
}
```

### Built-In Template Policies

Taskschmiede includes four template policies:

- `dod_tmpl_minimal` -- minimal requirements
- `dod_tmpl_peer_reviewed` -- requires peer review
- `dod_tmpl_full_governance` -- full governance compliance
- `dod_tmpl_agent_autonomous` -- agent self-governance

Template policies cannot be updated directly. Fork them to create a derived version.

## Assign to an Endeavour

Assign a DoD policy to an endeavour so it governs all tasks within that endeavour:

```json
{
  "tool": "ts.dod.assign",
  "arguments": {
    "endeavour_id": "edv_xyz789",
    "policy_id": "dod_abc123"
  }
}
```

To remove a DoD assignment:

```json
{
  "tool": "ts.dod.unassign",
  "arguments": {
    "endeavour_id": "edv_xyz789"
  }
}
```

## Endorse a Policy

Team members endorse a DoD policy to acknowledge they accept its conditions for a given endeavour:

```json
{
  "tool": "ts.dod.endorse",
  "arguments": {
    "policy_id": "dod_abc123",
    "endeavour_id": "edv_xyz789"
  }
}
```

Only one active endorsement exists per (policy, resource, endeavour) combination. Endorsement statuses:

- `active` -- current endorsement
- `superseded` -- replaced by a newer policy version
- `withdrawn` -- withdrawn by the user

## Check Task Compliance

Check whether a task meets the DoD conditions (dry run, no side effects):

```json
{
  "tool": "ts.dod.check",
  "arguments": {
    "task_id": "tsk_task1"
  }
}
```

Returns pass/fail for each condition in the policy. This does not change the task's status -- it only reports compliance.

## Override When Needed

When a task must be completed despite failing some DoD conditions, use an override:

```json
{
  "tool": "ts.dod.override",
  "arguments": {
    "task_id": "tsk_task1",
    "reason": "Client deadline requires immediate release; documentation will follow."
  }
}
```

Overrides require `admin` or `owner` role. Every override is recorded in the audit log for traceability.

## Check DoD Status for an Endeavour

View the overall DoD status for an endeavour, including which policy is assigned and the endorsement state:

```json
{
  "tool": "ts.dod.status",
  "arguments": {
    "endeavour_id": "edv_xyz789"
  }
}
```

## DoD Versioning

When a policy needs to be updated, create a new version rather than editing in place:

```json
{
  "tool": "ts.dod.new_version",
  "arguments": {
    "policy_id": "dod_abc123",
    "conditions": [
      {"type": "field_set", "field": "actual", "label": "Actual hours recorded"},
      {"type": "field_set", "field": "description", "label": "Description provided"},
      {"type": "approval_received", "params": {"verdict": "approved", "role": "reviewer"}, "label": "Peer review approved"},
      {"type": "field_set", "field": "metadata.test_results", "label": "Test results attached"}
    ],
    "strictness": "all"
  }
}
```

Creating a new version:

- Archives the old version
- Marks existing endorsements as `superseded`
- Creates a new policy linked to the old one via `predecessor_id`
- The new version inherits the endeavour assignment

Team members must re-endorse the new version.

## View Policy Lineage

To see the full version chain of a DoD policy:

```json
{
  "tool": "ts.dod.lineage",
  "arguments": {
    "policy_id": "dod_abc123"
  }
}
```

Returns all versions in the predecessor chain, sorted by version ascending.
