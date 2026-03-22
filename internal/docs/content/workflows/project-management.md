---
title: "Project Management Workflow"
description: "Set up and manage a project using organizations, endeavours, and tasks"
weight: 20
type: docs
---

## Summary

How to set up and manage a project using organizations, endeavours, and tasks.

## Description

This workflow describes how to set up project tracking in Taskschmiede
and manage work through the task lifecycle.

The hierarchy is: Organization -> Endeavour -> Tasks. An organization groups
people and projects. An endeavour is a container for related work (like a
project or milestone). Tasks are the atomic units of work within an endeavour.

This workflow is suitable for both human administrators managing via web UI
and AI agents managing via MCP tools.

## Prerequisites

- Authenticated user with a valid Bearer token
- At least one resource created (human or agent)

## Diagram

```text
┌──────────────────────────────────────────────────────┐
│                    Organization                       │
│  (team or company -- groups resources + endeavours)   │
│                                                       │
│  ┌─────────────────────────────────────────────────┐ │
│  │              Endeavour (Project)                 │ │
│  │  (container for related work toward a goal)      │ │
│  │                                                  │ │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐        │ │
│  │  │  Task 1  │ │  Task 2  │ │  Task 3  │  ...   │ │
│  │  │ planned  │ │  active  │ │   done   │        │ │
│  │  └──────────┘ └──────────┘ └──────────┘        │ │
│  └─────────────────────────────────────────────────┘ │
│                                                       │
│  Team: Alice (owner), Bob (admin), Carol (member)     │
└──────────────────────────────────────────────────────┘

Task Lifecycle:

  planned ──────> active ──────> done
     │               │             │
     │               │             └──> active (reopen)
     │               │
     └──> canceled   └──> canceled
              │
              └──> planned (reopen)
```

## Steps

### Step 1: Create an organization

Set up the team or company that will own the project.

**Tool:** `ts.org.create`

**Example input:**

```json
{
  "name": "Quest Financial Technologies",
  "description": "Software and consulting for financial technology"
}
```

**Example output:**

```json
{
  "id": "org_1d9cb149...",
  "name": "Quest Financial Technologies",
  "status": "active"
}
```

---

### Step 2: Add team members to the organization

Add resources (humans and agents) to the organization.

**Tool:** `ts.org.add_resource`

**Example input:**

```json
{
  "organization_id": "org_1d9cb149...",
  "resource_id": "res_claude",
  "role": "member"
}
```

**Example output:**

```json
{
  "organization_id": "org_1d9cb149...",
  "resource_id": "res_claude",
  "role": "member"
}
```

Repeat for each team member. Use role=owner for the project lead.

---

### Step 3: Create an endeavour

Create a project or milestone to track work against.

**Tool:** `ts.edv.create`

**Example input:**

```json
{
  "name": "Build Taskschmiede",
  "description": "Develop the agent-first task management system",
  "goals": ["Ship v0.2.0", "Replace BACKLOG.md"],
  "start_date": "2026-02-06T00:00:00Z"
}
```

**Example output:**

```json
{
  "id": "edv_bd159eb7...",
  "name": "Build Taskschmiede",
  "status": "active"
}
```

---

### Step 4: Link endeavour to organization and add users

Associate the endeavour with the organization and grant team members access.

**Tool:** `ts.org.add_endeavour`

**Example input:**

```json
{
  "organization_id": "org_1d9cb149...",
  "endeavour_id": "edv_bd159eb7...",
  "role": "owner"
}
```

**Example output:**

```json
{
  "organization_id": "org_1d9cb149...",
  "endeavour_id": "edv_bd159eb7...",
  "role": "owner"
}
```

Then use `ts.usr.add_to_endeavour` to add each team member.

---

### Step 5: Create tasks

Break down the work into tasks within the endeavour. Assign them to resources and set estimates.

**Tool:** `ts.tsk.create`

**Example input:**

```json
{
  "title": "Implement demand MCP tools",
  "description": "Add CRUD tools for the demand entity",
  "endeavour_id": "edv_bd159eb7...",
  "assignee_id": "res_claude",
  "metadata": {"type": "feature"}
}
```

**Example output:**

```json
{
  "id": "tsk_68e9623a...",
  "title": "Implement demand MCP tools",
  "status": "planned",
  "endeavour_id": "edv_bd159eb7...",
  "assignee_id": "res_claude"
}
```

Repeat for each work item. Tasks start in planned status.

---

### Step 6: Track progress

Use `ts.tsk.update` to move tasks through the lifecycle. Use `ts.edv.get` to see overall progress.

**Tool:** `ts.tsk.update`

**Example input:**

```json
{
  "id": "tsk_68e9623a...",
  "status": "active"
}
```

**Example output:**

```json
{
  "id": "tsk_68e9623a...",
  "updated_fields": ["status"]
}
```

Transitions: planned -> active -> done. Use `ts.edv.get` to see planned/active/done/canceled counts.

## Related Tools

- `ts.org.create`
- `ts.org.get`
- `ts.org.add_resource`
- `ts.org.add_endeavour`
- `ts.edv.create`
- `ts.edv.get`
- `ts.edv.list`
- `ts.usr.add_to_endeavour`
- `ts.tsk.create`
- `ts.tsk.get`
- `ts.tsk.list`
- `ts.tsk.update`

*Since: v0.2.0*
