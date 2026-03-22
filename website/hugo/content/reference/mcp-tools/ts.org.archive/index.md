---
title: "ts.org.archive"
description: "Archive an organization with cascade."
category: "organization"
requires_auth: true
since: "v0.3.7"
type: docs
---

Archive an organization with cascade.

**Requires authentication**

## Description

Archives an organization and cascades to all associated endeavours and tasks.

Use confirm=false (default) for a dry-run that shows what would be archived.
Use confirm=true to execute the archive operation.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `id` | string | Yes |  | Organization ID |
| `reason` | string | Yes |  | Reason for archiving |
| `confirm` | boolean |  | `false` | Execute the archive (false = dry-run) |

## Response

Returns the archive result or dry-run summary.

```json
{
  "archived": true,
  "id": "org_1d9cb149497656c7...",
  "reason": "Project completed"
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |
| `not_found` | Organization not found |
| `forbidden` | Admin privileges required |

## Related Tools

- [`ts.org.get`](../ts.org.get/)
- [`ts.org.update`](../ts.org.update/)

