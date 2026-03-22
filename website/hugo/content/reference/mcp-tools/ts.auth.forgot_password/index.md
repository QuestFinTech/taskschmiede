---
title: "ts.auth.forgot_password"
description: "Request a password reset code via email."
category: "auth"
requires_auth: false
since: "v0.3.0"
type: docs
---

Request a password reset code via email.

## Description

Initiates the password reset flow. If the email address is associated with
an account, a reset code is sent via email. The response is identical
regardless of whether the account exists (prevents email enumeration).

When email is not configured, the reset code is returned directly in
the response for development convenience.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `email` | string | Yes |  | Email address of the account to reset |

## Response

Returns a confirmation that the reset was requested.

```json
{
  "expires_in": "15m0s",
  "note": "If an account exists with that email, a reset code has been sent.",
  "status": "reset_requested"
}
```

## Errors

| Code | Description |
|------|-------------|
| `invalid_input` | Email is required |
| `rate_limited` | Too many requests for this email |

## Examples

### Request password reset

Initiate a password reset for an account.

**Request:**

```json
{
  "email": "agent@example.com"
}
```

**Response:**

```json
{
  "expires_in": "15m0s",
  "note": "If an account exists with that email, a reset code has been sent.",
  "status": "reset_requested"
}
```

## Related Tools

- [`ts.auth.reset_password`](../ts.auth.reset_password/)
- [`ts.auth.login`](../ts.auth.login/)

