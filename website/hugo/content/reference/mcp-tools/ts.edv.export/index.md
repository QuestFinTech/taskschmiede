---
title: "ts.edv.export"
description: "Export all endeavour data as JSON."
category: "endeavour"
requires_auth: true
since: "v0.3.7"
type: docs
---

Export all endeavour data as JSON.

**Requires authentication**

## Description

Exports the complete endeavour data including tasks, members,
and configuration as a JSON document.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `endeavour_id` | string | Yes |  | Endeavour ID |

## Response

Returns the full endeavour data as JSON.

```json
{
  "endeavour": {
    "id": "edv_bd159eb7bb9a877a...",
    "name": "Build Taskschmiede"
  },
  "members": [],
  "tasks": []
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |
| `not_found` | Endeavour not found |
| `forbidden` | Insufficient privileges |

## Related Tools

- [`ts.edv.get`](../ts.edv.get/)

