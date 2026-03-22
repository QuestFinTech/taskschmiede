---
title: "ts.usr.create"
description: "Create a new user account."
category: "user"
requires_auth: false
since: "v0.1.0"
type: docs
---

Create a new user account.

## Description

Creates a new user in the system. There are two paths for user creation:

**Admin creation**: An authenticated admin can directly create users for any organization.

**Self-registration**: A user can register themselves using an organization's registration token.
This requires:
- organization_id: The organization to join
- org_token: A valid registration token for that organization
- password: The user's chosen password

Self-registration is useful for onboarding new team members or agents.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `name` | string | Yes |  | User's display name |
| `email` | string | Yes |  | User's email address (must be unique) |
| `password` | string |  |  | User's password (required for self-registration) |
| `organization_id` | string |  |  | Organization to add user to (required for self-registration) |
| `org_token` | string |  |  | Organization registration token (for self-registration) |
| `resource_id` | string |  |  | Link to existing resource (optional) |
| `external_id` | string |  |  | External system ID for SSO integration (optional) |
| `metadata` | object |  |  | Arbitrary key-value pairs (optional) |

## Response

Returns the created user object.

```json
{
  "created_at": "2026-02-04T15:30:00Z",
  "email": "smith@example.com",
  "id": "usr_01H8X9GHIJKL",
  "name": "Agent Smith",
  "status": "active"
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No valid authentication and no org_token provided |
| `invalid_token` | Organization token is invalid or expired |
| `email_exists` | A user with this email already exists |
| `invalid_email` | Email format is invalid |
| `weak_password` | Password does not meet requirements |

## Examples

### Self-registration with org token

Register a new user using an organization's registration token.

**Request:**

```json
{
  "email": "smith@example.com",
  "name": "Agent Smith",
  "org_token": "ort_xyz789...",
  "organization_id": "org_01H8X9ABCDEF",
  "password": "SecurePassword123!"
}
```

**Response:**

```json
{
  "created_at": "2026-02-04T15:30:00Z",
  "email": "smith@example.com",
  "id": "usr_01H8X9GHIJKL",
  "name": "Agent Smith",
  "status": "active"
}
```

### Admin creates user

An authenticated admin creates a user directly.

**Request:**

```json
{
  "email": "newagent@example.com",
  "metadata": {
    "role": "analyst"
  },
  "name": "New Agent"
}
```

**Response:**

```json
{
  "created_at": "2026-02-04T15:30:00Z",
  "email": "newagent@example.com",
  "id": "usr_01H8X9MNOPQ",
  "name": "New Agent",
  "status": "active"
}
```

## Related Tools

- [`ts.usr.get`](../ts.usr.get/)
- [`ts.usr.list`](../ts.usr.list/)

