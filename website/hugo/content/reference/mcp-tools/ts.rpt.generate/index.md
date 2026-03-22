---
title: "ts.rpt.generate"
description: "Generate a Markdown report."
category: "report"
requires_auth: true
since: "v0.3.7"
type: docs
---

Generate a Markdown report.

**Requires authentication**

## Description

Generates a Markdown report for a given entity using its associated
report template. The scope determines the entity type.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `scope` | string | Yes |  | Report scope: task, demand, endeavour, project |
| `entity_id` | string | Yes |  | Entity ID to generate report for |

## Response

Returns the generated Markdown report.

```json
{
  "entity_id": "edv_bd159eb7bb9a877a...",
  "markdown": "# Build Taskschmiede Report\n\n...",
  "scope": "endeavour"
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |
| `not_found` | Entity not found |
| `invalid_input` | Scope and entity_id are required |

## Related Tools

- [`ts.tpl.get`](../ts.tpl.get/)
- [`ts.tpl.list`](../ts.tpl.list/)

