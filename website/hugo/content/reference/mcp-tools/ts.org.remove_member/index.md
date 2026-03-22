---
title: "ts.org.remove_member"
description: "Remove a user from an organization."
category: "organization"
requires_auth: true
since: "v0.3.7"
type: docs
---

Remove a user from an organization.

**Requires authentication**

## Description

Removes a user's membership from an organization.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `org_id` | string | Yes |  | Organization ID |
| `user_id` | string | Yes |  | User ID to remove |

## Response

Returns confirmation of removal.

```json
{
  "org_id": "org_1d9cb149497656c7...",
  "removed": true,
  "user_id": "usr_476931df38eb2662..."
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |
| `not_found` | Organization or membership not found |
| `forbidden` | Cannot remove the last owner |

## Related Tools

- [`ts.org.add_member`](../ts.org.add_member/)
- [`ts.org.list_members`](../ts.org.list_members/)

