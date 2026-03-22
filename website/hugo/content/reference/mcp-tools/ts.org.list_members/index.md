---
title: "ts.org.list_members"
description: "List members of an organization with their roles."
category: "organization"
requires_auth: true
since: "v0.3.7"
type: docs
---

List members of an organization with their roles.

**Requires authentication**

## Description

Returns all members of an organization including their roles and join dates.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `org_id` | string | Yes |  | Organization ID |

## Response

Returns the list of organization members.

```json
{
  "members": [
    {
      "joined_at": "2026-02-06T13:36:43Z",
      "role": "owner",
      "user_id": "usr_476931df38eb2662..."
    }
  ]
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |
| `not_found` | Organization not found |

## Related Tools

- [`ts.org.add_member`](../ts.org.add_member/)
- [`ts.org.remove_member`](../ts.org.remove_member/)
- [`ts.org.set_member_role`](../ts.org.set_member_role/)

