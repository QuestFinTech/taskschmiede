---
title: "ts.rtl.lineage"
description: "Walk the version chain for a ritual (oldest to newest)."
category: "ritual"
requires_auth: true
since: "v0.2.0"
type: docs
---

Walk the version chain for a ritual (oldest to newest).

**Requires authentication**

## Description

Traces the lineage of a ritual by walking the predecessor chain.

Returns an ordered list of rituals from the oldest ancestor to the given
ritual. This shows how a methodology prompt has evolved over time through forks.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `id` | string | Yes |  | Ritual ID to trace lineage from |

## Response

Returns the lineage chain as an ordered list (oldest first).

```json
{
  "lineage": [
    {
      "created_at": "2026-02-09T10:00:00Z",
      "id": "rtl_a1b2c3d4e5f6...",
      "name": "Daily standup",
      "origin": "custom"
    },
    {
      "created_at": "2026-02-09T12:00:00Z",
      "id": "rtl_b2c3d4e5f6a1...",
      "name": "Daily standup v2",
      "origin": "fork"
    }
  ]
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |
| `not_found` | Ritual with this ID does not exist |

## Examples

### Trace ritual lineage

See the full version history of a forked ritual.

**Request:**

```json
{
  "id": "rtl_b2c3d4e5f6a1..."
}
```

**Response:**

```json
{
  "lineage": [
    {
      "id": "rtl_a1b2c3d4e5f6...",
      "name": "Daily standup",
      "origin": "custom"
    },
    {
      "id": "rtl_b2c3d4e5f6a1...",
      "name": "Daily standup v2",
      "origin": "fork"
    }
  ]
}
```

## Related Tools

- [`ts.rtl.get`](../ts.rtl.get/)
- [`ts.rtl.fork`](../ts.rtl.fork/)

