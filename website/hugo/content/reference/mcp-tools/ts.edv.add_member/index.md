---
title: "ts.edv.add_member"
description: "Add a user to an endeavour."
category: "endeavour"
requires_auth: true
since: "v0.3.7"
type: docs
---

Add a user to an endeavour.

**Requires authentication**

## Description

Adds a user to an endeavour with a specified role. This controls
who can see and work on tasks within the endeavour.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `endeavour_id` | string | Yes |  | Endeavour ID |
| `user_id` | string | Yes |  | User ID to add |
| `role` | string |  | `member` | Role: owner, admin, member, viewer |

## Response

Returns the membership confirmation.

```json
{
  "endeavour_id": "edv_bd159eb7bb9a877a...",
  "joined_at": "2026-02-06T13:36:51Z",
  "role": "member",
  "user_id": "usr_476931df38eb2662..."
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |
| `not_found` | Endeavour or user not found |
| `already_member` | User is already a member of this endeavour |

## Related Tools

- [`ts.edv.remove_member`](../ts.edv.remove_member/)
- [`ts.edv.list_members`](../ts.edv.list_members/)
- [`ts.edv.get`](../ts.edv.get/)

