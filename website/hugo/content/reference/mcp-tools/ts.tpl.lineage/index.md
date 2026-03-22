---
title: "ts.tpl.lineage"
description: "Walk the version chain for a template."
category: "template"
requires_auth: true
since: "v0.3.7"
type: docs
---

Walk the version chain for a template.

**Requires authentication**

## Description

Returns the version lineage of a template, showing the chain of
forks from the original to the current version.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `id` | string | Yes |  | Template ID |

## Response

Returns the version chain.

```json
{
  "lineage": [
    {
      "id": "tpl_original...",
      "name": "Sprint Summary",
      "version": 1
    },
    {
      "id": "tpl_a1b2c3d4e5f6...",
      "name": "Sprint Summary v2",
      "version": 2
    }
  ]
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |
| `not_found` | Template not found |

## Related Tools

- [`ts.tpl.get`](../ts.tpl.get/)
- [`ts.tpl.fork`](../ts.tpl.fork/)

