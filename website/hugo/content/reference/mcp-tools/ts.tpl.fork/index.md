---
title: "ts.tpl.fork"
description: "Fork a template to create a derived version."
category: "template"
requires_auth: true
since: "v0.3.7"
type: docs
---

Fork a template to create a derived version.

**Requires authentication**

## Description

Creates a new template derived from an existing one. The new template
inherits the source's body and metadata unless overridden.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `source_id` | string | Yes |  | Source template ID to fork from |
| `name` | string |  |  | Name for the forked template |
| `body` | string |  |  | Override body |
| `lang` | string |  |  | Override language code |
| `metadata` | object |  |  | Override metadata |

## Response

Returns the newly created forked template.

```json
{
  "id": "tpl_f1g2h3i4j5k6...",
  "name": "Sprint Summary (forked)",
  "source_id": "tpl_a1b2c3d4e5f6...",
  "status": "active"
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |
| `not_found` | Source template not found |

## Related Tools

- [`ts.tpl.get`](../ts.tpl.get/)
- [`ts.tpl.create`](../ts.tpl.create/)
- [`ts.tpl.lineage`](../ts.tpl.lineage/)

