---
title: "Your First Workflow"
description: "Create an organization, project, and tasks end-to-end"
weight: 80
type: docs
---

This tutorial walks you through a complete workflow in Taskschmiede: creating an organization, setting up a project, defining requirements, and completing tasks. Examples use MCP tool calls. The same operations are available through the [REST API]({{< relref "connecting" >}}) and the Portal web UI.

## Prerequisites

- Taskschmiede is running (see [Installation]({{< relref "installation" >}}))
- An MCP client is connected and authenticated (see [Connecting]({{< relref "connecting" >}}))

## The Entity Hierarchy

Taskschmiede organizes work in a clear hierarchy:

```
Organization
  └── Endeavour (project)
        └── Demand (requirement / user story)
              └── Task (work item)
```

- **Organizations** are the top-level container. They group people, projects, and resources.
- **Endeavours** are projects or initiatives within an organization.
- **Demands** are requirements, user stories, or feature requests within an endeavour.
- **Tasks** are concrete work items that fulfill a demand.

## Step 1: Create an Organization

Every workflow starts with an organization:

```json
{
  "tool": "ts.org.create",
  "arguments": {
    "name": "Acme Corp",
    "description": "Our main organization for product development"
  }
}
```

The response includes the organization ID. You will need this for subsequent steps.

## Step 2: Create an Endeavour

An endeavour represents a project within the organization:

```json
{
  "tool": "ts.edv.create",
  "arguments": {
    "org_id": "org-uuid-from-step-1",
    "name": "Website Redesign",
    "description": "Redesign the company website with modern standards",
    "status": "planning"
  }
}
```

Endeavours start in `planning` status and move through `active`, `completed`, and `archived` as the project progresses.

## Step 3: Create a Demand

A demand captures what needs to be done -- a requirement or user story:

```json
{
  "tool": "ts.dmd.create",
  "arguments": {
    "endeavour_id": "edv-uuid-from-step-2",
    "title": "Implement responsive navigation",
    "description": "The navigation bar should work on mobile, tablet, and desktop screens. It should collapse into a hamburger menu on screens narrower than 768px.",
    "priority": "high"
  }
}
```

Demands start in `draft` status. Move them to `open` when they are ready for work.

## Step 4: Update the Demand Status

Move the demand from `draft` to `open` to signal it is ready:

```json
{
  "tool": "ts.dmd.update",
  "arguments": {
    "id": "dmd-uuid-from-step-3",
    "status": "open"
  }
}
```

## Step 5: Create a Task

Tasks are the actionable work items that fulfill a demand:

```json
{
  "tool": "ts.tsk.create",
  "arguments": {
    "demand_id": "dmd-uuid-from-step-3",
    "title": "Build responsive navbar component",
    "description": "Create a React component for the navigation bar that collapses to a hamburger menu below 768px breakpoint.",
    "priority": "high"
  }
}
```

Tasks start in `open` status.

## Step 6: Work on the Task

When you start working on a task, update its status:

```json
{
  "tool": "ts.tsk.update",
  "arguments": {
    "id": "tsk-uuid-from-step-5",
    "status": "in_progress"
  }
}
```

## Step 7: Submit for Review

When the work is done, move the task to review:

```json
{
  "tool": "ts.tsk.update",
  "arguments": {
    "id": "tsk-uuid-from-step-5",
    "status": "review"
  }
}
```

## Step 8: Complete the Task

After review, mark the task as completed:

```json
{
  "tool": "ts.tsk.update",
  "arguments": {
    "id": "tsk-uuid-from-step-5",
    "status": "completed"
  }
}
```

## Step 9: Fulfill the Demand

With all tasks completed, update the demand:

```json
{
  "tool": "ts.dmd.update",
  "arguments": {
    "id": "dmd-uuid-from-step-3",
    "status": "fulfilled"
  }
}
```

## Checking Progress

At any point, you can list and inspect entities:

```json
{
  "tool": "ts.tsk.list",
  "arguments": {
    "demand_id": "dmd-uuid"
  }
}
```

```json
{
  "tool": "ts.dmd.list",
  "arguments": {
    "endeavour_id": "edv-uuid"
  }
}
```

```json
{
  "tool": "ts.edv.get",
  "arguments": {
    "id": "edv-uuid"
  }
}
```

## Summary

The complete workflow follows this path:

1. **Organization** -- create the container for your team
2. **Endeavour** -- define a project (planning -> active)
3. **Demand** -- capture requirements (draft -> open -> in_progress -> fulfilled)
4. **Task** -- do the work (open -> in_progress -> review -> completed)

Each level feeds into the next. Demands break down into tasks, tasks fulfill demands, and demands drive endeavours toward completion.

## Next Steps

- [Core Concepts]({{< relref "/concepts/core-concepts" >}}) -- deeper look at the entity model
- [Lifecycle and Workflows]({{< relref "/concepts/lifecycle-and-workflows" >}}) -- detailed state machines and transitions
- [Organizations and Teams]({{< relref "/concepts/organizations-and-teams" >}}) -- managing members and roles
