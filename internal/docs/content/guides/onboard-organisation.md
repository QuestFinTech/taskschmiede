---
title: "Organisations"
description: "Create organizations, invite members, and assign roles"
weight: 90
type: docs
---

This guide covers creating an organization in Taskschmiede, adding members, configuring roles, and setting up endeavours.

## Create an Organization

Any authenticated user can create an organization. The creator is automatically linked as `owner`.

```json
{
  "tool": "ts.org.create",
  "arguments": {
    "name": "Acme Engineering",
    "description": "Product development team"
  }
}
```

Response:

```json
{
  "id": "org_abc123",
  "name": "Acme Engineering",
  "description": "Product development team",
  "status": "active",
  "created_at": "2026-03-08T10:00:00Z"
}
```

Required fields: `name`. Optional: `description`, `metadata`.

## Add Members

Add users to the organization by their resource ID. Requires `admin` or `owner` role.

```json
{
  "tool": "ts.org.add_member",
  "arguments": {
    "organization_id": "org_abc123",
    "user_id": "usr_member1",
    "role": "member"
  }
}
```

Available roles: `owner`, `admin`, `member`, `guest`. If no role is specified, the default is `member`.

To add multiple members, call `ts.org.add_member` once per user.

## Set Member Roles

Change a member's role within the organization:

```json
{
  "tool": "ts.org.set_member_role",
  "arguments": {
    "organization_id": "org_abc123",
    "user_id": "usr_member1",
    "role": "admin"
  }
}
```

Only `owner` and `admin` roles can change other members' roles.

### Role Capabilities

| Role | Read | Write | Manage Members | Archive |
|------|------|-------|----------------|---------|
| `owner` | Yes | Yes | Yes | Yes |
| `admin` | Yes | Yes | Yes | No |
| `member` | Yes | Yes | No | No |
| `guest` | Yes | No | No | No |

## Create Endeavours

Endeavours are containers for related work toward a goal. Create one within the context of the organization:

```json
{
  "tool": "ts.edv.create",
  "arguments": {
    "name": "Q2 Product Launch",
    "description": "All tasks related to the Q2 product launch",
    "goals": [
      {"title": "Feature complete", "status": "open"},
      {"title": "Documentation shipped", "status": "open"}
    ]
  }
}
```

Then link the endeavour to the organization:

```json
{
  "tool": "ts.org.add_endeavour",
  "arguments": {
    "organization_id": "org_abc123",
    "endeavour_id": "edv_xyz789",
    "role": "owner"
  }
}
```

The `role` field indicates the organization's relationship to the endeavour. Use `owner` when the organization owns the endeavour, or `participant` for secondary involvement.

## Add Endeavour Members

Add individual users directly to an endeavour:

```json
{
  "tool": "ts.edv.add_member",
  "arguments": {
    "endeavour_id": "edv_xyz789",
    "user_id": "usr_dev1",
    "role": "member"
  }
}
```

Organization members inherit endeavour roles automatically when the organization is linked:

| Organization Role | Inherited Endeavour Role |
|-------------------|--------------------------|
| `owner` | `admin` |
| `admin` | `admin` |
| `member` | `member` |
| `guest` | `viewer` |

Direct endeavour membership takes precedence over inherited organization membership.

## Create Team Resources

Teams are modeled as resources. Create a team resource to represent a working group:

```json
{
  "tool": "ts.res.create",
  "arguments": {
    "name": "Backend Team",
    "type": "human",
    "description": "Server-side development team"
  }
}
```

Resource types: `human`, `agent`, `service`, `budget`.

Link the resource to the organization:

```json
{
  "tool": "ts.org.add_resource",
  "arguments": {
    "organization_id": "org_abc123",
    "resource_id": "res_team1",
    "role": "member"
  }
}
```

## List Organization Members

View all members and their roles:

```json
{
  "tool": "ts.org.list_members",
  "arguments": {
    "organization_id": "org_abc123"
  }
}
```

## Remove Members

Remove a user from the organization:

```json
{
  "tool": "ts.org.remove_member",
  "arguments": {
    "organization_id": "org_abc123",
    "user_id": "usr_member1"
  }
}
```

Requires `admin` or `owner` role. Owners cannot be removed (transfer ownership first).
