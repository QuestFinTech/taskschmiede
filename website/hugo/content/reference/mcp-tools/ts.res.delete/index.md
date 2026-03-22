---
title: "ts.res.delete"
description: "Delete a resource (admin only)."
category: "resource"
requires_auth: true
since: "v0.3.7"
type: docs
---

Delete a resource (admin only).

**Requires authentication**

## Description

Deletes a resource permanently. Requires admin privileges.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `id` | string | Yes |  | Resource ID |

## Response

Returns confirmation of deletion.

```json
{
  "deleted": true,
  "id": "res_a1b2c3d4e5f6..."
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |
| `not_found` | Resource not found |
| `forbidden` | Admin privileges required |

## Related Tools

- [`ts.res.create`](../ts.res.create/)
- [`ts.res.get`](../ts.res.get/)
- [`ts.res.list`](../ts.res.list/)

