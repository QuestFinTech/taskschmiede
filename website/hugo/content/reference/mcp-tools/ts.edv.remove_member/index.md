---
title: "ts.edv.remove_member"
description: "Remove a user from an endeavour."
category: "endeavour"
requires_auth: true
since: "v0.3.7"
type: docs
---

Remove a user from an endeavour.

**Requires authentication**

## Description

Removes a user's membership from an endeavour.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `endeavour_id` | string | Yes |  | Endeavour ID |
| `user_id` | string | Yes |  | User ID to remove |

## Response

Returns confirmation of removal.

```json
{
  "endeavour_id": "edv_bd159eb7bb9a877a...",
  "removed": true,
  "user_id": "usr_476931df38eb2662..."
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |
| `not_found` | Endeavour or membership not found |

## Related Tools

- [`ts.edv.add_member`](../ts.edv.add_member/)
- [`ts.edv.list_members`](../ts.edv.list_members/)

