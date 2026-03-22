---
title: "ts.tpl.get"
description: "Retrieve a template by ID."
category: "template"
requires_auth: true
since: "v0.3.7"
type: docs
---

Retrieve a template by ID.

**Requires authentication**

## Description

Retrieves detailed information about a specific report template.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `id` | string | Yes |  | Template ID |

## Response

Returns the full template object.

```json
{
  "body": "# {{.Name}} Report\n\n{{.Summary}}",
  "created_at": "2026-03-07T10:00:00Z",
  "id": "tpl_a1b2c3d4e5f6...",
  "name": "Sprint Summary",
  "scope": "endeavour",
  "status": "active"
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |
| `not_found` | Template not found |

## Related Tools

- [`ts.tpl.list`](../ts.tpl.list/)
- [`ts.tpl.update`](../ts.tpl.update/)
- [`ts.tpl.fork`](../ts.tpl.fork/)

