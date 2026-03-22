---
title: "ts.rtl.fork"
description: "Fork a ritual (create a new ritual derived from an existing one)."
category: "ritual"
requires_auth: true
since: "v0.2.0"
type: docs
---

Fork a ritual (create a new ritual derived from an existing one).

**Requires authentication**

## Description

Creates a new ritual derived from an existing one. This is how you
evolve methodology prompts while preserving history.

The forked ritual:
- Gets origin="fork" and predecessor_id pointing to the source
- Inherits name, prompt, description, and schedule from the source (unless overridden)
- Is a fully independent ritual that can be modified separately
- Can optionally be linked to a different endeavour

Use ts.rtl.lineage to trace the full version chain.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `source_id` | string | Yes |  | The ritual to fork from |
| `name` | string |  |  | Name for the fork (defaults to source name) |
| `prompt` | string |  |  | Modified prompt (defaults to source prompt) |
| `description` | string |  |  | Description (defaults to source description) |
| `endeavour_id` | string |  |  | Endeavour the forked ritual governs |
| `schedule` | object |  |  | Schedule metadata (defaults to source schedule) |
| `metadata` | object |  |  | Arbitrary key-value pairs |

## Response

Returns the forked ritual summary.

```json
{
  "created_at": "2026-02-09T12:00:00Z",
  "id": "rtl_b2c3d4e5f6a1...",
  "is_enabled": true,
  "name": "Daily standup v2",
  "origin": "fork",
  "predecessor_id": "rtl_a1b2c3d4e5f6...",
  "status": "active"
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |
| `not_found` | Source ritual does not exist |

## Examples

### Fork with modified prompt

Create a new version of a ritual with an updated prompt.

**Request:**

```json
{
  "name": "Daily standup v2",
  "prompt": "Review progress, blockers, and today's plan. Include metrics from the dashboard.",
  "source_id": "rtl_a1b2c3d4e5f6..."
}
```

**Response:**

```json
{
  "created_at": "2026-02-09T12:00:00Z",
  "id": "rtl_b2c3d4e5f6a1...",
  "is_enabled": true,
  "name": "Daily standup v2",
  "origin": "fork",
  "predecessor_id": "rtl_a1b2c3d4e5f6...",
  "status": "active"
}
```

### Fork for a different endeavour

Fork a template ritual for a specific project.

**Request:**

```json
{
  "endeavour_id": "edv_newproject...",
  "source_id": "rtl_template_standup..."
}
```

**Response:**

```json
{
  "created_at": "2026-02-09T12:00:00Z",
  "id": "rtl_c3d4e5f6a1b2...",
  "is_enabled": true,
  "name": "Daily standup",
  "origin": "fork",
  "predecessor_id": "rtl_template_standup...",
  "status": "active"
}
```

## Related Tools

- [`ts.rtl.create`](../ts.rtl.create/)
- [`ts.rtl.get`](../ts.rtl.get/)
- [`ts.rtl.lineage`](../ts.rtl.lineage/)

