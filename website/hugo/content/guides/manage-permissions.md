---
title: "Permissions"
description: "Roles, scopes, and fine-grained access control"
weight: 100
type: docs
---

This guide explains the role hierarchy in Taskschmiede, what each role can do, and how to manage permissions and tier limits.

## Role Hierarchy

Taskschmiede uses a layered permission model. From highest to lowest privilege:

1. **Master Admin** -- full system control, bypasses all RBAC checks and tier limits
2. **Organization Owner** -- full control of their organizations, can archive and export
3. **Organization Admin** -- can manage members and update organization details
4. **Member** -- can read and write content within the organization or endeavour
5. **Guest / Viewer** -- read-only access

## Organization Roles

| Role | Read | Write | Manage Members | Archive | Export |
|------|------|-------|----------------|---------|--------|
| `owner` | Yes | Yes | Yes | Yes | Yes |
| `admin` | Yes | Yes | Yes | No | No |
| `member` | Yes | Yes | No | No | No |
| `guest` | Yes | No | No | No | No |

## Endeavour Roles

| Role | Read | Write (Tasks/Demands) | Cancel Tasks | Manage Members | Archive |
|------|------|----------------------|--------------|----------------|---------|
| `owner` | Yes | Yes | Yes | Yes | Yes |
| `admin` | Yes | Yes | Yes | Yes | No |
| `member` | Yes | Yes | Own only | No | No |
| `viewer` | Yes | No | No | No | No |

Members can cancel tasks they created or are assigned to. All other cancellations require `admin` or `owner` role.

## Changing Roles

Change a member's role within an organization:

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

Only users with `admin` or `owner` role can change other members' roles.

## Permission Resolution Order

When a user accesses an endeavour, permissions are resolved in this order:

1. **Master admin** -- bypasses all RBAC checks.
2. **Direct endeavour membership** -- user has a direct `member_of` relation to the endeavour with an explicit role.
3. **Organization membership** -- the user's organization role is mapped to an endeavour role (see table below).
4. **No access** -- 403 Forbidden.

### Organization-to-Endeavour Role Mapping

When an organization participates in an endeavour, its members inherit endeavour roles:

| Organization Role | Inherited Endeavour Role |
|-------------------|--------------------------|
| `owner` | `admin` |
| `admin` | `admin` |
| `member` | `member` |
| `guest` | `viewer` |

Direct endeavour membership always takes precedence over inherited organization membership.

## Entity Creation Permissions

| Entity | Required Permission |
|--------|---------------------|
| Organization | Any authenticated user |
| Endeavour | Any authenticated user |
| Task | `member` or higher in the endeavour |
| Demand | `member` or higher in the endeavour |
| Comment | `member` or higher in the endeavour |
| Artifact | `member` or higher in the endeavour |
| Ritual | `member` or higher in the endeavour |
| Approval | `member` or higher (write access to entity's endeavour) |
| Message | Any authenticated user (scope-aware delivery) |

## Tier Limits

Users are assigned a tier that controls quotas. Three tiers are available:

### Tier 1 -- Free

| Limit | Default |
|-------|---------|
| Max organizations | 1 |
| Max active endeavours | 1 |
| Max endeavours per org | 3 |
| Max agents per org | 5 |
| Max creations per hour | 60 |

### Tier 2 -- Professional

| Limit | Default |
|-------|---------|
| Max organizations | 3 |
| Max active endeavours | 30 |
| Max endeavours per org | Unlimited |
| Max agents per org | Unlimited |
| Max creations per hour | 300 |

### Tier 3 -- Enterprise

All limits are unlimited (set to -1).

### Changing a User's Tier

Via the REST API:

```bash
curl -X PATCH /api/v1/users/{id} \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"tier": 2}'
```

Via the Portal: navigate to `/admin/users/{id}` and update the tier field.

### Master Admin Bypass

The master admin bypasses all tier limits. This allows unrestricted operation for system administration without being subject to quota enforcement.

### Editing Quota Defaults

Tier 1 quota values can be modified through the admin API:

```bash
curl -X PATCH /api/v1/admin/quotas \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "tier.1.max_orgs": 2,
    "tier.1.max_endeavours_per_org": 5
  }'
```

Tier 2 and Tier 3 values exist in the policy table but are not in the admin-editable allowlist by default. To change them, modify the policy table directly or update the allowlist in the codebase.
