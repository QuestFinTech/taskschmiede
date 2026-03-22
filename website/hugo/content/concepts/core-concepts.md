---
title: "Core Concepts"
description: "Organizations, endeavours, demands, tasks, and how they fit together"
weight: 10
type: docs
---

This page introduces the fundamental concepts and terminology used throughout Taskschmiede.

## Entity Hierarchy

Taskschmiede organizes work in a four-level hierarchy:

```
Organization
  └── Endeavour
        └── Demand
              └── Task
```

### Organizations

An organization is the top-level container. It groups users, resources, endeavours, and all associated data. Every entity in Taskschmiede belongs to an organization.

Organizations are created by the master admin or by users with the appropriate permissions. Each organization has its own membership, roles, and settings.

### Endeavours

An endeavour is a project or initiative within an organization. It defines a bounded scope of work with a clear objective. Endeavours progress through a lifecycle: `planning`, `active`, `completed`, and `archived`.

Examples of endeavours: a product launch, a migration project, a quarterly sprint.

### Demands

A demand is a requirement, user story, or feature request within an endeavour. Demands capture *what* needs to be done without prescribing *how*. They follow a lifecycle: `draft`, `open`, `in_progress`, `fulfilled`, and `cancelled`.

Demands can be broken down into one or more tasks.

### Tasks

A task is a concrete, actionable work item. Tasks are assigned to users or agents and tracked through their lifecycle: `open`, `in_progress`, `review`, `completed`, and `cancelled`.

Tasks are the leaf nodes of the hierarchy -- they represent the actual work being performed.

## Users and Resources

Taskschmiede distinguishes between **users** and **resources**:

- **Users** are people or AI agents who have accounts in the system. A user has credentials, can authenticate, and can interact with Taskschmiede via MCP or the web portal.
- **Resources** represent a user's membership within a specific organization. When a user joins an organization, a resource record is created to track their role, permissions, and participation in that organization.

A single user can be a resource in multiple organizations, potentially with different roles in each.

## Roles

Taskschmiede uses role-based access control (RBAC) scoped to organizations. The available roles are:

| Role | Scope | Description |
|------|-------|-------------|
| **Master Admin** | System-wide | Full control over the Taskschmiede instance. Created during initial setup. |
| **Owner** | Organization | Full control over the organization. Can manage members, settings, and all entities. |
| **Admin** | Organization | Can manage members and most organization settings. Cannot delete the organization or transfer ownership. |
| **Member** | Organization | Can create and manage endeavours, demands, and tasks. Standard working role. |
| **Guest** | Organization | Read-only access to the organization. Can view entities but not create or modify them. |

Roles are assigned per organization. A user who is an admin in one organization may be a guest in another.

## Authentication and Tokens

Taskschmiede supports two authentication mechanisms:

### Session Tokens (MCP)

MCP clients authenticate by calling `ts.auth.login` with email and password. This establishes a session that persists for the duration of the MCP connection (up to 24 hours). The session is tied to the transport connection.

### Bearer Tokens (REST API)

The REST API uses bearer tokens for authentication. Tokens are created via `ts.tkn.create` and included in the `Authorization` header:

```
Authorization: Bearer <token>
```

Bearer tokens can be scoped and have configurable expiration.

## Task Lifecycle

Tasks move through the following states:

```
open --> in_progress --> review --> completed
  |         |             |
  +---------+-------------+--> cancelled
```

- **open** -- the task has been created and is ready to be worked on
- **in_progress** -- someone is actively working on the task
- **review** -- the work is done and awaiting review
- **completed** -- the task has been reviewed and accepted
- **cancelled** -- the task has been abandoned

## Endeavour Lifecycle

```
planning --> active --> completed --> archived
```

- **planning** -- the endeavour is being defined and scoped
- **active** -- work is underway
- **completed** -- all demands have been fulfilled
- **archived** -- the endeavour is closed and retained for reference

## Demand Lifecycle

```
draft --> open --> in_progress --> fulfilled
  |        |          |
  +--------+----------+--> cancelled
```

- **draft** -- the demand is being written and is not yet ready for work
- **open** -- the demand is defined and ready for tasks to be created
- **in_progress** -- tasks are being worked on to fulfill this demand
- **fulfilled** -- all tasks are complete and the demand is satisfied
- **cancelled** -- the demand has been dropped

## Next Steps

- [Architecture]({{< relref "architecture" >}}) -- how the system is built
- [Organizations and Teams]({{< relref "organizations-and-teams" >}}) -- managing membership and roles
- [Lifecycle and Workflows]({{< relref "lifecycle-and-workflows" >}}) -- detailed state transitions and policies
