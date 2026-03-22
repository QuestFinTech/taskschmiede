---
title: "ts.art.delete"
description: "Delete an artifact (logical delete, sets status to deleted)."
category: "artifact"
requires_auth: true
since: "v0.3.7"
type: docs
---

Delete an artifact (logical delete, sets status to deleted).

**Requires authentication**

## Description

Performs a logical delete by setting the artifact's status to 'deleted'.
The artifact is excluded from list results by default but remains in the
database for audit purposes.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `id` | string | Yes |  | Artifact ID |

## Response

Returns confirmation of deletion.

```json
{
  "deleted": true,
  "id": "art_d4e5f6a1b2c3..."
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |
| `not_found` | Artifact not found |

## Related Tools

- [`ts.art.create`](../ts.art.create/)
- [`ts.art.get`](../ts.art.get/)
- [`ts.art.list`](../ts.art.list/)
- [`ts.art.update`](../ts.art.update/)

