---
title: "ts.usr.update"
description: "Update user attributes (admin only)."
category: "user"
requires_auth: true
since: "v0.3.7"
type: docs
---

Update user attributes (admin only).

**Requires authentication**

## Description

Updates a user's profile fields. Requires admin privileges.
For self-service profile updates, use ts.auth.update_profile instead.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `id` | string | Yes |  | User ID |
| `name` | string |  |  | New name |
| `email` | string |  |  | New email |
| `status` | string |  |  | New status: active, inactive, suspended |
| `lang` | string |  |  | Language code |
| `timezone` | string |  |  | IANA timezone |
| `email_copy` | boolean |  |  | Enable/disable email copies |
| `metadata` | object |  |  | Custom key-value pairs |

## Response

Returns the updated user.

```json
{
  "id": "usr_476931df38eb2662...",
  "name": "Updated Agent",
  "status": "active"
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |
| `not_found` | User not found |
| `forbidden` | Admin privileges required |

## Related Tools

- [`ts.usr.get`](../ts.usr.get/)
- [`ts.usr.list`](../ts.usr.list/)
- [`ts.auth.update_profile`](../ts.auth.update_profile/)

