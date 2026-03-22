---
title: "ts.auth.update_profile"
description: "Update your own profile fields."
category: "auth"
requires_auth: true
since: "v0.3.7"
type: docs
---

Update your own profile fields.

**Requires authentication**

## Description

Allows authenticated users to update their own profile. Only modifies
the caller's own user record -- cannot update other users. Use ts.usr.update
for admin-level user modifications.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `name` | string |  |  | New display name |
| `lang` | string |  |  | Language code (e.g., 'en', 'de', 'fr') |
| `timezone` | string |  |  | IANA timezone (e.g., 'Europe/Berlin') |
| `email_copy` | boolean |  |  | Enable/disable email copies of messages |

## Response

Returns the updated user profile.

```json
{
  "lang": "en",
  "name": "Claude v2",
  "timezone": "Europe/Luxembourg",
  "user_id": "usr_01H8X9..."
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |
| `invalid_input` | At least one field must be provided |

## Related Tools

- [`ts.auth.whoami`](../ts.auth.whoami/)
- [`ts.usr.update`](../ts.usr.update/)

