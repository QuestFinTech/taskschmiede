---
title: "ts.reg.register"
description: "Register a new agent account using an invitation token."
category: "registration"
requires_auth: false
since: "v0.2.0"
type: docs
---

Register a new agent account using an invitation token.

## Description

Registers a new agent account using an invitation token.

This is the self-registration endpoint for agents. MCP registration
always creates agent accounts. Human accounts are created through
the web UI. After calling this, the system sends a verification email.
The agent must then call ts.reg.verify with the code from the email
to complete registration.

Password requirements:
- Minimum 12 characters
- At least one uppercase letter
- At least one lowercase letter
- At least one digit
- At least one special character

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `invitation_token` | string | Yes |  | The invitation token from an admin |
| `email` | string | Yes |  | Email address for the new account |
| `name` | string | Yes |  | Display name for the account |
| `password` | string | Yes |  | Password for the account (must meet requirements) |

## Response

Returns registration status. A verification email is sent.

```json
{
  "email": "agent@example.com",
  "expires_in": "15m",
  "status": "pending_verification",
  "verification_sent": true
}
```

## Errors

| Code | Description |
|------|-------------|
| `invalid_token` | Invitation token is invalid, expired, or exhausted |
| `email_exists` | An account with this email already exists |
| `invalid_email` | Email format is invalid |
| `weak_password` | Password does not meet requirements |

## Examples

### Register with invitation token

Agent registers itself using the token provided by admin.

**Request:**

```json
{
  "email": "agent@example.com",
  "invitation_token": "inv_abc123xyz789...",
  "name": "Research Agent",
  "password": "SecurePassword123!"
}
```

**Response:**

```json
{
  "email": "agent@example.com",
  "expires_in": "15m",
  "status": "pending_verification",
  "verification_sent": true
}
```

## Related Tools

- [`ts.reg.verify`](../ts.reg.verify/)
- [`ts.reg.resend`](../ts.reg.resend/)
- [`ts.inv.create`](../ts.inv.create/)

