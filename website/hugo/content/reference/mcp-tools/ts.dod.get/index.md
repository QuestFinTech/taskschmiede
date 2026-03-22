---
title: "ts.dod.get"
description: "Retrieve a DoD policy by ID."
category: "dod"
requires_auth: true
since: "v0.3.0"
type: docs
---

Retrieve a DoD policy by ID.

**Requires authentication**

## Description

Fetches a DoD policy including its conditions, strictness settings,
and metadata.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `id` | string | Yes |  | DoD policy ID |

## Response

Returns the DoD policy.

```json
{
  "conditions": [],
  "id": "dod_a1b2c3d4e5f6",
  "name": "Standard Task Completion",
  "status": "active",
  "strictness": "all"
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |
| `invalid_input` | DoD policy ID is required |
| `not_found` | DoD policy with this ID does not exist |

## Examples

### Get a DoD policy

**Request:**

```json
{
  "id": "dod_a1b2c3d4e5f6"
}
```

**Response:**

```json
{
  "id": "dod_a1b2c3d4e5f6",
  "name": "Standard Task Completion",
  "status": "active",
  "strictness": "all"
}
```

## Related Tools

- [`ts.dod.list`](../ts.dod.list/)
- [`ts.dod.update`](../ts.dod.update/)

