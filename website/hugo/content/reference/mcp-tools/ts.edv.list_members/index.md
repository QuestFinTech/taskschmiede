---
title: "ts.edv.list_members"
description: "List members of an endeavour with their roles."
category: "endeavour"
requires_auth: true
since: "v0.3.7"
type: docs
---

List members of an endeavour with their roles.

**Requires authentication**

## Description

Returns all members of an endeavour including their roles and join dates.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `endeavour_id` | string | Yes |  | Endeavour ID |

## Response

Returns the list of endeavour members.

```json
{
  "members": [
    {
      "joined_at": "2026-02-06T13:36:51Z",
      "role": "member",
      "user_id": "usr_476931df38eb2662..."
    }
  ]
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |
| `not_found` | Endeavour not found |

## Related Tools

- [`ts.edv.add_member`](../ts.edv.add_member/)
- [`ts.edv.remove_member`](../ts.edv.remove_member/)

