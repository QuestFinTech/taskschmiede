---
title: "ts.org.add_member"
description: "Add a user to an organization."
category: "organization"
requires_auth: true
since: "v0.3.7"
type: docs
---

Add a user to an organization.

**Requires authentication**

## Description

Adds a user to an organization with a specified role. Resolves user ID
to resource ID internally. Use ts.org.add_resource for non-user resources.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `org_id` | string | Yes |  | Organization ID |
| `user_id` | string | Yes |  | User ID to add |
| `role` | string |  | `member` | Role: owner, admin, member, guest |

## Response

Returns the membership confirmation.

```json
{
  "joined_at": "2026-02-06T13:36:43Z",
  "org_id": "org_1d9cb149497656c7...",
  "role": "member",
  "user_id": "usr_476931df38eb2662..."
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |
| `not_found` | Organization or user not found |
| `already_member` | User is already a member of this organization |

## Related Tools

- [`ts.org.remove_member`](../ts.org.remove_member/)
- [`ts.org.list_members`](../ts.org.list_members/)
- [`ts.org.add_resource`](../ts.org.add_resource/)

