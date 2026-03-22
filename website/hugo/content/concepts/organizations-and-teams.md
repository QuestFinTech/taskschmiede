---
title: "Organizations"
description: "How teams, roles, and membership work"
weight: 40
type: docs
---

Organizations are the foundational unit of structure in Taskschmiede. They group users, projects, and resources under a shared context with role-based access control.

## Organizations

An organization is the top-level container for all work in Taskschmiede. Every endeavour, demand, task, and resource belongs to exactly one organization.

### Creating an Organization

Organizations are created via the `ts.org.create` tool or the web portal:

```json
{
  "tool": "ts.org.create",
  "arguments": {
    "name": "Engineering Team",
    "description": "Core product engineering organization"
  }
}
```

The user who creates the organization is automatically assigned the **owner** role.

### Organization Structure

Within an organization, work is organized as:

```
Organization
  ├── Members (users with roles)
  ├── Resources (membership records)
  ├── Endeavours (projects)
  │     ├── Demands (requirements)
  │     │     └── Tasks (work items)
  │     └── ...
  └── ...
```

## Members and Resources

When a user joins an organization, a **resource** record is created. The resource represents the user's membership and tracks their role, permissions, and participation within that organization.

### Adding Members

Organization owners and admins can add members:

```json
{
  "tool": "ts.org.add_member",
  "arguments": {
    "org_id": "org-uuid",
    "user_id": "user-uuid",
    "role": "member"
  }
}
```

### Listing Members

```json
{
  "tool": "ts.org.list_members",
  "arguments": {
    "org_id": "org-uuid"
  }
}
```

### Removing Members

```json
{
  "tool": "ts.org.remove_member",
  "arguments": {
    "org_id": "org-uuid",
    "user_id": "user-uuid"
  }
}
```

Owners cannot be removed. Ownership must be transferred first.

## Roles

Each member has a role within the organization that determines what they can do.

### Owner

- Full control over the organization
- Can manage all settings, members, and entities
- Can delete or archive the organization
- Can transfer ownership to another member
- Every organization must have exactly one owner

### Admin

- Can add and remove members (except the owner)
- Can change member roles (up to admin level)
- Can manage all endeavours, demands, and tasks
- Cannot delete the organization or change the owner

### Member

- Can create and manage endeavours, demands, and tasks
- Can view all organization data
- Cannot manage other members or organization settings
- This is the standard working role for most users and agents

### Guest

- Read-only access to the organization
- Can view endeavours, demands, tasks, and other entities
- Cannot create, modify, or delete anything
- Useful for stakeholders or observers who need visibility without write access

### Changing Roles

Owners and admins can change member roles:

```json
{
  "tool": "ts.org.set_member_role",
  "arguments": {
    "org_id": "org-uuid",
    "user_id": "user-uuid",
    "role": "admin"
  }
}
```

## Teams

Teams in Taskschmiede are modeled as resources with type "team" within an organization. A team groups related members for assignment and coordination purposes.

Resources can represent individual users or teams, and both can be assigned to tasks, demands, or endeavours. This unified model keeps the assignment system simple while supporting both individual and team-based workflows.

## Invitations

Users can be invited to join an organization via invitation tokens:

```json
{
  "tool": "ts.inv.create",
  "arguments": {
    "org_id": "org-uuid",
    "role": "member"
  }
}
```

The invitation generates a token that the invitee uses during registration or onboarding. Invitations can be revoked before they are used:

```json
{
  "tool": "ts.inv.revoke",
  "arguments": {
    "id": "inv-uuid"
  }
}
```

## Multi-Organization Membership

A single user can be a member of multiple organizations simultaneously. Each membership is independent -- a user might be an owner in one organization, a member in another, and a guest in a third.

When using MCP tools, the organization context is specified per request (via `org_id` parameters), so there is no ambiguity about which organization a given action applies to.

## Next Steps

- [Core Concepts]({{< relref "core-concepts" >}}) -- the entity hierarchy and lifecycle overview
- [Security Model]({{< relref "security-model" >}}) -- how roles and authorization are enforced
- [Lifecycle and Workflows]({{< relref "lifecycle-and-workflows" >}}) -- how work moves through states
