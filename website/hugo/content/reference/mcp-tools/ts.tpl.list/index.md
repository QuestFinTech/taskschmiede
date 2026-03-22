---
title: "ts.tpl.list"
description: "Query templates with filters."
category: "template"
requires_auth: true
since: "v0.3.7"
type: docs
---

Query templates with filters.

**Requires authentication**

## Description

Lists report templates with optional filtering and pagination.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `scope` | string |  |  | Filter by scope: task, demand, endeavour |
| `lang` | string |  |  | Filter by language code |
| `status` | string |  |  | Filter by status |
| `search` | string |  |  | Search by name (partial match) |
| `limit` | integer |  | `50` | Maximum number of results to return |
| `offset` | integer |  | `0` | Number of results to skip (for pagination) |

## Response

Returns a paginated list of templates.

```json
{
  "limit": 50,
  "offset": 0,
  "templates": [
    {
      "id": "tpl_a1b2c3d4e5f6...",
      "name": "Sprint Summary",
      "scope": "endeavour"
    }
  ],
  "total": 1
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |

## Related Tools

- [`ts.tpl.create`](../ts.tpl.create/)
- [`ts.tpl.get`](../ts.tpl.get/)

