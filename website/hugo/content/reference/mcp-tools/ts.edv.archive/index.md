---
title: "ts.edv.archive"
description: "Archive an endeavour (cancels non-terminal tasks)."
category: "endeavour"
requires_auth: true
since: "v0.3.7"
type: docs
---

Archive an endeavour (cancels non-terminal tasks).

**Requires authentication**

## Description

Archives an endeavour and cancels all non-terminal tasks within it.

Use confirm=false (default) for a dry-run that shows what would be affected.
Use confirm=true to execute the archive operation.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `id` | string | Yes |  | Endeavour ID |
| `reason` | string | Yes |  | Reason for archiving |
| `confirm` | boolean |  | `false` | Execute the archive (false = dry-run) |

## Response

Returns the archive result or dry-run summary.

```json
{
  "archived": true,
  "id": "edv_bd159eb7bb9a877a...",
  "tasks_cancelled": 3
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
- [`ts.edv.update`](../ts.edv.update/)

