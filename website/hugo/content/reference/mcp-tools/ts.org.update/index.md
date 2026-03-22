---
title: "ts.org.update"
description: "Update organization attributes (partial update)."
category: "organization"
requires_auth: true
since: "v0.3.0"
type: docs
---

Update organization attributes (partial update).

**Requires authentication**

## Description

Updates an organization's name, description, status, or metadata.
Only provided fields are changed.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `id` | string | Yes |  | Organization ID |
| `name` | string |  |  | New name |
| `description` | string |  |  | New description |
| `status` | string |  |  | New status: active, inactive, archived |
| `metadata` | object |  |  | Metadata to set (replaces existing) |

## Response

Returns the updated organization.

```json
{
  "id": "org_1d9cb149497656c7",
  "name": "Updated Org Name",
  "status": "active",
  "updated_at": "2026-02-12T10:00:00Z"
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |
| `not_found` | Organization with this ID does not exist |
| `invalid_input` | No fields to update |

## Examples

### Update organization name

Change the name of an organization.

**Request:**

```json
{
  "id": "org_1d9cb149497656c7",
  "name": "Updated Org Name"
}
```

**Response:**

```json
{
  "id": "org_1d9cb149497656c7",
  "name": "Updated Org Name",
  "updated_at": "2026-02-12T10:00:00Z"
}
```

## Related Tools

- [`ts.org.get`](../ts.org.get/)
- [`ts.org.list`](../ts.org.list/)

