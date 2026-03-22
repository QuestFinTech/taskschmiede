---
title: "ts.auth.reset_password"
description: "Complete a password reset with the emailed code."
category: "auth"
requires_auth: false
since: "v0.3.0"
type: docs
---

Complete a password reset with the emailed code.

## Description

Validates the reset code and sets a new password. On success, all
existing sessions and tokens for the user are invalidated.

The new password must meet the same requirements as registration:
12+ characters, at least one uppercase, lowercase, digit, and special character.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `email` | string | Yes |  | Email address of the account |
| `code` | string | Yes |  | Reset code from the email (format: xxx-xxx-xxx) |
| `new_password` | string | Yes |  | New password (12+ chars, mixed case, digit, special) |

## Response

Returns confirmation that the password was reset.

```json
{
  "note": "Password has been changed. All existing sessions have been invalidated.",
  "status": "password_reset"
}
```

## Errors

| Code | Description |
|------|-------------|
| `invalid_input` | Email, code, and new_password are required |
| `invalid_input` | Password does not meet requirements |
| `reset_failed` | Invalid or expired reset code |

## Examples

### Complete password reset

Use the emailed code to set a new password.

**Request:**

```json
{
  "code": "abc-def-ghi",
  "email": "agent@example.com",
  "new_password": "NewSecurePassword123!"
}
```

**Response:**

```json
{
  "note": "Password has been changed. All existing sessions have been invalidated.",
  "status": "password_reset"
}
```

## Related Tools

- [`ts.auth.forgot_password`](../ts.auth.forgot_password/)
- [`ts.auth.login`](../ts.auth.login/)

