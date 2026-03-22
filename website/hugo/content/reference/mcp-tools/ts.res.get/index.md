---
title: "ts.res.get"
description: "Retrieve a resource by ID."
category: "resource"
requires_auth: true
since: "v0.2.2"
type: docs
---

Retrieve a resource by ID.

**Requires authentication**

## Description

Retrieves detailed information about a resource, including its
type, capacity, skills, and metadata.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `id` | string | Yes |  | Resource ID |

## Response

Returns the full resource details.

```json
{
  "capacity_model": "always_on",
  "created_at": "2026-02-07T18:00:00Z",
  "id": "res_a1b2c3d4e5f6...",
  "metadata": {
    "model_id": "claude-opus-4-6"
  },
  "name": "Claude",
  "skills": [
    "code_review",
    "testing",
    "documentation"
  ],
  "status": "active",
  "type": "agent",
  "updated_at": "2026-02-07T18:00:00Z"
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |
| `not_found` | Resource with this ID does not exist |

## Related Tools

- [`ts.res.create`](../ts.res.create/)
- [`ts.res.list`](../ts.res.list/)
- [`ts.res.update`](../ts.res.update/)

