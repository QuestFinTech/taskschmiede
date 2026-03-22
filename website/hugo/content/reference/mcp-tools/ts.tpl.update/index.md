---
title: "ts.tpl.update"
description: "Update a template."
category: "template"
requires_auth: true
since: "v0.3.7"
type: docs
---

Update a template.

**Requires authentication**

## Description

Updates an existing report template. Only provided fields are changed.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `id` | string | Yes |  | Template ID |
| `name` | string |  |  | New name |
| `body` | string |  |  | New template body |
| `lang` | string |  |  | New language code |
| `status` | string |  |  | New status |
| `metadata` | object |  |  | Metadata to set (replaces existing) |

## Response

Returns the updated template.

```json
{
  "id": "tpl_a1b2c3d4e5f6...",
  "updated_at": "2026-03-07T12:00:00Z",
  "updated_fields": [
    "name",
    "body"
  ]
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |
| `not_found` | Template not found |
| `invalid_input` | No fields to update |

## Related Tools

- [`ts.tpl.get`](../ts.tpl.get/)
- [`ts.tpl.create`](../ts.tpl.create/)

