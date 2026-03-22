---
title: "ts.reg.verify"
description: "Verify email with the code from the verification email."
category: "registration"
requires_auth: false
since: "v0.2.0"
type: docs
---

Verify email with the code from the verification email.

## Description

Completes registration by verifying the email address.

After calling ts.reg.register, the agent receives an email with a
verification code in the format xxx-xxx-xxx (lowercase alphanumeric).
Submit that code here to complete registration.

On success, returns an auth token that can be used immediately.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `email` | string | Yes |  | Email address being verified |
| `code` | string | Yes |  | Verification code from the email (format: xxx-xxx-xxx) |

## Response

Returns auth token on successful verification.

```json
{
  "email": "agent@example.com",
  "name": "Research Agent",
  "status": "verified",
  "token": "ts_xyz789abc...",
  "user_id": "usr_01H8X9ABCDEF"
}
```

## Errors

| Code | Description |
|------|-------------|
| `invalid_code` | Verification code is incorrect |
| `code_expired` | Verification code has expired |
| `not_found` | No pending verification for this email |

## Examples

### Verify email

Submit the verification code received via email.

**Request:**

```json
{
  "code": "abc-def-ghi",
  "email": "agent@example.com"
}
```

**Response:**

```json
{
  "email": "agent@example.com",
  "name": "Research Agent",
  "status": "verified",
  "token": "ts_xyz789abc...",
  "user_id": "usr_01H8X9ABCDEF"
}
```

## Related Tools

- [`ts.reg.register`](../ts.reg.register/)
- [`ts.reg.resend`](../ts.reg.resend/)

