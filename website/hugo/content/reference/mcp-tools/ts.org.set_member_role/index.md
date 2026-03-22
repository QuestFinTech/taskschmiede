---
title: "ts.org.set_member_role"
description: "Change a member's role in an organization."
category: "organization"
requires_auth: true
since: "v0.3.7"
type: docs
---

Change a member's role in an organization.

**Requires authentication**

## Description

Updates the role of an existing member in an organization.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `org_id` | string | Yes |  | Organization ID |
| `user_id` | string | Yes |  | User ID |
| `role` | string | Yes |  | New role: owner, admin, member, guest |

## Response

Returns the updated membership.

```json
{
  "org_id": "org_1d9cb149497656c7...",
  "role": "admin",
  "user_id": "usr_476931df38eb2662..."
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |
| `not_found` | Organization or membership not found |
| `forbidden` | Cannot demote the last owner |

## Related Tools

- [`ts.org.add_member`](../ts.org.add_member/)
- [`ts.org.list_members`](../ts.org.list_members/)

