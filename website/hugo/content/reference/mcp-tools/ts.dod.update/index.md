---
title: "ts.dod.update"
description: "Update DoD policy attributes."
category: "dod"
requires_auth: true
since: "v0.3.0"
type: docs
---

Update DoD policy attributes.

**Requires authentication**

## Description

Updates a DoD policy's name, description, status, or metadata.
Conditions cannot be changed via update -- use ts.dod.new_version
to create a new version with updated conditions.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `id` | string | Yes |  | DoD policy ID |
| `name` | string |  |  | New name |
| `description` | string |  |  | New description |
| `status` | string |  |  | New status: active, archived |
| `metadata` | object |  |  | Metadata to set (replaces existing) |

## Response

Returns the updated DoD policy.

```json
{
  "id": "dod_a1b2c3d4e5f6",
  "name": "Updated Policy Name",
  "status": "active",
  "updated_at": "2026-02-12T10:15:00Z"
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |
| `invalid_input` | DoD policy ID is required |
| `not_found` | DoD policy with this ID does not exist |

## Examples

### Archive a policy

Set a DoD policy to archived status.

**Request:**

```json
{
  "id": "dod_a1b2c3d4e5f6",
  "status": "archived"
}
```

**Response:**

```json
{
  "id": "dod_a1b2c3d4e5f6",
  "status": "archived",
  "updated_at": "2026-02-12T10:15:00Z"
}
```

## Related Tools

- [`ts.dod.get`](../ts.dod.get/)
- [`ts.dod.create`](../ts.dod.create/)
- [`ts.dod.new_version`](../ts.dod.new_version/)

