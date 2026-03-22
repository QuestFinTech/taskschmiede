---
title: "ts.usr.get"
description: "Retrieve a user by ID."
category: "user"
requires_auth: true
since: "v0.1.0"
type: docs
---

Retrieve a user by ID.

**Requires authentication**

## Description

Retrieves detailed information about a specific user.

Requires authentication. Users can always retrieve their own information.
Admins can retrieve any user's information.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `id` | string | Yes |  | User ID to retrieve |

## Response

Returns the user object if found.

```json
{
  "created_at": "2026-02-04T15:30:00Z",
  "email": "smith@example.com",
  "id": "usr_01H8X9ABCDEF",
  "metadata": {
    "department": "engineering"
  },
  "name": "Agent Smith",
  "status": "active",
  "updated_at": "2026-02-04T15:30:00Z"
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |
| `not_found` | User with this ID does not exist |
| `unauthorized` | Not allowed to view this user |

## Examples

### Get user by ID

**Request:**

```json
{
  "id": "usr_01H8X9ABCDEF"
}
```

**Response:**

```json
{
  "created_at": "2026-02-04T15:30:00Z",
  "email": "smith@example.com",
  "id": "usr_01H8X9ABCDEF",
  "name": "Agent Smith",
  "status": "active",
  "updated_at": "2026-02-04T15:30:00Z"
}
```

## Related Tools

- [`ts.usr.create`](../ts.usr.create/)
- [`ts.usr.list`](../ts.usr.list/)

