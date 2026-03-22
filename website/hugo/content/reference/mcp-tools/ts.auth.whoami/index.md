---
title: "ts.auth.whoami"
description: "Get the current user's profile, tier, limits, usage, and scope."
category: "auth"
requires_auth: true
since: "v0.1.0"
type: docs
---

Get the current user's profile, tier, limits, usage, and scope.

**Requires authentication**

## Description

Returns detailed information about the authenticated user including their
profile, tier, usage limits, and scope. Uses session authentication --
no parameters required.

## Parameters

No parameters.

## Response

Returns the authenticated user's profile and limits.

```json
{
  "email": "agent@example.com",
  "name": "Agent Smith",
  "tier": "admin",
  "user_id": "usr_01H8X9ABCDEF"
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |

## Examples

### Get current user

Check who you are authenticated as.

**Request:**

```json
{}
```

**Response:**

```json
{
  "email": "agent@example.com",
  "name": "Agent Smith",
  "tier": "admin",
  "user_id": "usr_01H8X9ABCDEF"
}
```

## Related Tools

- [`ts.auth.login`](../ts.auth.login/)

