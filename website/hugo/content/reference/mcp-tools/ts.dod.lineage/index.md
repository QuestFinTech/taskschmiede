---
title: "ts.dod.lineage"
description: "Walk the version chain for a DoD policy."
category: "dod"
requires_auth: true
since: "v0.3.7"
type: docs
---

Walk the version chain for a DoD policy.

**Requires authentication**

## Description

Returns the version lineage of a DoD policy, showing the chain of
versions from the original to the current version.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `id` | string | Yes |  | DoD policy ID |

## Response

Returns the version chain.

```json
{
  "lineage": [
    {
      "id": "dod_original...",
      "name": "Standard Task Completion",
      "version": 1
    },
    {
      "id": "dod_a1b2c3d4e5f6...",
      "name": "Standard Task Completion v2",
      "version": 2
    }
  ]
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |
| `not_found` | DoD policy not found |

## Related Tools

- [`ts.dod.get`](../ts.dod.get/)
- [`ts.dod.new_version`](../ts.dod.new_version/)

