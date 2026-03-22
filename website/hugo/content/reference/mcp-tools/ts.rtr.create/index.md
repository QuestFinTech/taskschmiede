---
title: "ts.rtr.create"
description: "Create a ritual run (marks execution start, status=running)."
category: "ritual_run"
requires_auth: true
since: "v0.2.0"
type: docs
---

Create a ritual run (marks execution start, status=running).

**Requires authentication**

## Description

Creates a new ritual run to track execution of a ritual.

A ritual run records when a ritual was executed, by what trigger, and what the
outcome was. New runs start in "running" status with started_at set automatically.

Triggers:
- manual: Agent or human explicitly started the run
- schedule: Triggered by a scheduled event
- api: Triggered by an external API call

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `ritual_id` | string | Yes |  | Ritual ID to execute |
| `trigger` | string |  | `manual` | What triggered the run: schedule, manual (default), api |
| `metadata` | object |  |  | Arbitrary key-value pairs |

## Response

Returns the created ritual run.

```json
{
  "created_at": "2026-02-09T10:00:00Z",
  "id": "rtr_x1y2z3a4b5c6...",
  "ritual_id": "rtl_a1b2c3d4e5f6...",
  "started_at": "2026-02-09T10:00:00Z",
  "status": "running",
  "trigger": "manual"
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |
| `invalid_input` | ritual_id is required |
| `not_found` | Ritual with this ID does not exist |

## Examples

### Start a manual ritual run

Begin executing a ritual triggered by an agent.

**Request:**

```json
{
  "ritual_id": "rtl_a1b2c3d4e5f6...",
  "trigger": "manual"
}
```

**Response:**

```json
{
  "id": "rtr_x1y2z3a4b5c6...",
  "ritual_id": "rtl_a1b2c3d4e5f6...",
  "started_at": "2026-02-09T10:00:00Z",
  "status": "running",
  "trigger": "manual"
}
```

## Related Tools

- [`ts.rtr.get`](../ts.rtr.get/)
- [`ts.rtr.list`](../ts.rtr.list/)
- [`ts.rtr.update`](../ts.rtr.update/)
- [`ts.rtl.get`](../ts.rtl.get/)

