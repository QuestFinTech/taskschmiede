---
title: "ts.rtl.create"
description: "Create a new ritual (stored methodology prompt)."
category: "ritual"
requires_auth: true
since: "v0.2.0"
type: docs
---

Create a new ritual (stored methodology prompt).

**Requires authentication**

## Description

Creates a new ritual in Taskschmiede.

A ritual is a stored methodology prompt -- the core of BYOM (Bring Your Own
Methodology). Rituals define recurring processes like standups, sprint planning,
retrospectives, or any methodology-specific ceremony.

The prompt field contains the methodology instructions in free-form text. This is
what gets executed during a ritual run. Rituals can be linked to an endeavour via
a governs relationship and can have a schedule (informational only -- the agent
or orchestrator is responsible for triggering runs).

Rituals have an origin field: "custom" for user-created, "template" for built-in
templates, "fork" for rituals derived from another.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `name` | string | Yes |  | Ritual name (e.g., 'Weekly planning (Shape Up)') |
| `prompt` | string | Yes |  | The methodology prompt (free-form text, BYOM core) |
| `description` | string |  |  | Longer explanation of the ritual |
| `endeavour_id` | string |  |  | Endeavour this ritual governs (creates a governs relation) |
| `schedule` | object |  |  | Schedule metadata: {"type":"cron\|interval\|manual", ...} (informational only) |
| `metadata` | object |  |  | Arbitrary key-value pairs |

## Response

Returns the created ritual summary.

```json
{
  "created_at": "2026-02-09T10:00:00Z",
  "id": "rtl_a1b2c3d4e5f6...",
  "is_enabled": true,
  "name": "Daily standup",
  "origin": "custom",
  "status": "active"
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |
| `invalid_input` | Name and prompt are required |

## Examples

### Create a daily standup ritual

Create a ritual for daily standups linked to an endeavour.

**Request:**

```json
{
  "description": "A brief daily check-in to align the team on progress and blockers.",
  "endeavour_id": "edv_bd159eb7bb9a877a...",
  "name": "Daily standup",
  "prompt": "Review yesterday's progress, identify blockers, plan today's work.",
  "schedule": {
    "expression": "0 9 * * 1-5",
    "type": "cron"
  }
}
```

**Response:**

```json
{
  "created_at": "2026-02-09T10:00:00Z",
  "id": "rtl_a1b2c3d4e5f6...",
  "is_enabled": true,
  "name": "Daily standup",
  "origin": "custom",
  "status": "active"
}
```

## Related Tools

- [`ts.rtl.get`](../ts.rtl.get/)
- [`ts.rtl.list`](../ts.rtl.list/)
- [`ts.rtl.update`](../ts.rtl.update/)
- [`ts.rtl.fork`](../ts.rtl.fork/)
- [`ts.rtr.create`](../ts.rtr.create/)

