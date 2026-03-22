---
title: "ts.auth.login"
description: "Authenticate with email and password to get an access token."
category: "auth"
requires_auth: false
since: "v0.1.0"
type: docs
---

Authenticate with email and password to get an access token.

## Description

Authenticates a user with their email and password credentials.
On success, returns an access token that can be used for subsequent API calls.

The token should be included in the Authorization header as a Bearer token:
Authorization: Bearer <token>

Tokens are single-use session tokens by default. For long-lived API tokens,
use ts.tkn.create after authentication.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `email` | string | Yes |  | User's email address |
| `password` | string | Yes |  | User's password |

## Response

Returns a session token and user information on success.

```json
{
  "expires_at": "2026-02-05T10:30:00Z",
  "token": "ts_abc123def456...",
  "user_id": "usr_01H8X9..."
}
```

## Errors

| Code | Description |
|------|-------------|
| `invalid_input` | Email and password are required |
| `rate_limited` | Too many login attempts |
| `invalid_credentials` | Email or password is incorrect |

## Examples

### Basic login

Authenticate to get a session token.

**Request:**

```json
{
  "email": "agent@example.com",
  "password": "SecurePassword123!"
}
```

**Response:**

```json
{
  "expires_at": "2026-02-05T10:30:00Z",
  "token": "ts_abc123def456...",
  "user_id": "usr_01H8X9ABCDEF"
}
```

## Related Tools

- [`ts.tkn.verify`](../ts.tkn.verify/)
- [`ts.tkn.create`](../ts.tkn.create/)

