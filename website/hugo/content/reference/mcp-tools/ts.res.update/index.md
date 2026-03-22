---
title: "ts.res.update"
description: "Update resource attributes (partial update)."
category: "resource"
requires_auth: true
since: "v0.2.4"
type: docs
---

Update resource attributes (partial update).

**Requires authentication**

## Description

Updates one or more fields on an existing resource. Only the provided
fields are changed; omitted fields retain their current values.

The resource type cannot be changed after creation.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `id` | string | Yes |  | Resource ID |
| `name` | string |  |  | New name |
| `capacity_model` | string |  |  | Capacity model: hours_per_week, tokens_per_day, always_on, budget |
| `capacity_value` | number |  |  | Amount of capacity |
| `skills` | array |  |  | List of skills or capabilities (replaces existing) |
| `metadata` | object |  |  | Metadata to set (replaces existing) |
| `status` | string |  |  | New status: active, inactive |

## Response

Returns the updated resource.

```json
{
  "capacity_model": "always_on",
  "created_at": "2026-02-07T10:00:00Z",
  "id": "res_abc123",
  "metadata": {
    "timezone": "UTC"
  },
  "name": "Senior Build Agent",
  "skills": [
    "go",
    "testing",
    "deployment",
    "monitoring"
  ],
  "status": "active",
  "type": "agent",
  "updated_at": "2026-02-10T14:30:00Z"
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |
| `not_found` | Resource with this ID does not exist |
| `invalid_input` | No fields to update, or invalid status value |

## Examples

### Update resource name and skills

Change a resource's name and add new skills.

**Request:**

```json
{
  "id": "res_abc123",
  "name": "Senior Build Agent",
  "skills": [
    "go",
    "testing",
    "deployment",
    "monitoring"
  ]
}
```

**Response:**

```json
null
```

### Deactivate a resource

Set a resource to inactive status.

**Request:**

```json
{
  "id": "res_abc123",
  "status": "inactive"
}
```

**Response:**

```json
null
```

## Related Tools

- [`ts.res.create`](../ts.res.create/)
- [`ts.res.get`](../ts.res.get/)
- [`ts.res.list`](../ts.res.list/)

