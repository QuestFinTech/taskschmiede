---
title: "ts.usr.list"
description: "Query users with filters."
category: "user"
requires_auth: true
since: "v0.1.0"
type: docs
---

Query users with filters.

**Requires authentication**

## Description

Lists users with optional filtering and pagination.

Requires authentication. Results are filtered based on the caller's permissions:
- Regular users see users in their organizations
- Admins can see all users

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `status` | string |  |  | Filter by status: active, inactive, suspended |
| `organization_id` | string |  |  | Filter by organization membership |
| `search` | string |  |  | Search by name or email (partial match) |
| `limit` | integer |  | `50` | Maximum number of results to return |
| `offset` | integer |  | `0` | Number of results to skip (for pagination) |

## Response

Returns a list of users matching the filters.

```json
{
  "limit": 50,
  "offset": 0,
  "total": 42,
  "users": [
    {
      "email": "smith@example.com",
      "id": "usr_01H8X9ABCDEF",
      "name": "Agent Smith",
      "status": "active"
    },
    {
      "email": "jones@example.com",
      "id": "usr_01H8X9GHIJKL",
      "name": "Agent Jones",
      "status": "active"
    }
  ]
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |

## Examples

### List active users

Get all active users.

**Request:**

```json
{
  "status": "active"
}
```

**Response:**

```json
{
  "limit": 50,
  "offset": 0,
  "total": 1,
  "users": [
    {
      "email": "smith@example.com",
      "id": "usr_01H8X9ABCDEF",
      "name": "Agent Smith",
      "status": "active"
    }
  ]
}
```

### Search users

Search for users by name or email.

**Request:**

```json
{
  "limit": 10,
  "search": "smith"
}
```

**Response:**

```json
{
  "limit": 10,
  "offset": 0,
  "total": 1,
  "users": [
    {
      "email": "smith@example.com",
      "id": "usr_01H8X9ABCDEF",
      "name": "Agent Smith",
      "status": "active"
    }
  ]
}
```

## Related Tools

- [`ts.usr.create`](../ts.usr.create/)
- [`ts.usr.get`](../ts.usr.get/)

