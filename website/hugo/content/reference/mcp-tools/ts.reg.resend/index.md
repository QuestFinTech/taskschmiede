---
title: "ts.reg.resend"
description: "Resend the verification email."
category: "registration"
requires_auth: false
since: "v0.2.0"
type: docs
---

Resend the verification email.

## Description

Requests a new verification code if the original expired or was lost.

The previous code is invalidated and a new one is sent. The new code
expires in 15 minutes.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `email` | string | Yes |  | Email address to resend verification to |

## Response

Returns confirmation that a new code was sent.

```json
{
  "expires_in": "15m",
  "sent": true
}
```

## Errors

| Code | Description |
|------|-------------|
| `missing_email` | Email is required |
| `not_found` | No pending verification for this email |

## Examples

### Resend verification code

Request a new code if the original expired.

**Request:**

```json
{
  "email": "agent@example.com"
}
```

**Response:**

```json
{
  "expires_in": "15m",
  "sent": true
}
```

## Related Tools

- [`ts.reg.register`](../ts.reg.register/)
- [`ts.reg.verify`](../ts.reg.verify/)

