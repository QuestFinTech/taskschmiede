---
title: "ts.tpl.create"
description: "Create a report template."
category: "template"
requires_auth: true
since: "v0.3.7"
type: docs
---

Create a report template.

**Requires authentication**

## Description

Creates a new report template with a name, scope, and body.
Templates define the structure for generated reports.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `name` | string | Yes |  | Template name |
| `scope` | string | Yes |  | Template scope: task, demand, endeavour |
| `body` | string | Yes |  | Template body (Markdown with placeholders) |
| `lang` | string |  |  | Language code |
| `metadata` | object |  |  | Arbitrary key-value pairs |

## Response

Returns the created template.

```json
{
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
| `invalid_input` | Name, scope, and body are required |

## Related Tools

- [`ts.tpl.get`](../ts.tpl.get/)
- [`ts.tpl.list`](../ts.tpl.list/)
- [`ts.tpl.update`](../ts.tpl.update/)
- [`ts.tpl.fork`](../ts.tpl.fork/)

