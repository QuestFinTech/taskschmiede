---
title: "ts.res.create"
description: "Create a new resource (human, agent, service, or budget)."
category: "resource"
requires_auth: true
since: "v0.2.2"
type: docs
---

Create a new resource (human, agent, service, or budget).

**Requires authentication**

## Description

Creates a new resource in Taskschmiede.

Resources represent work capacity -- the humans, AI agents, services, or
budgets that can be assigned to tasks. Each resource has a type, optional
capacity model, skills, and metadata.

Resource types:
- human: a person
- agent: an AI agent or automated system
- service: an external service or API
- budget: a financial allocation

After creation, add the resource to an organization with ts.org.add_resource.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `type` | string | Yes |  | Resource type: human, agent, service, budget |
| `name` | string | Yes |  | Resource name |
| `capacity_model` | string |  |  | Capacity model: hours_per_week, tokens_per_day, always_on, budget |
| `capacity_value` | number |  |  | Amount of capacity (interpretation depends on capacity_model) |
| `skills` | array |  |  | List of skills or capabilities (array of strings) |
| `metadata` | object |  |  | Arbitrary key-value pairs (e.g., email, timezone, model_id) |

## Response

Returns the created resource summary.

```json
{
  "created_at": "2026-02-07T18:00:00Z",
  "id": "res_a1b2c3d4e5f6...",
  "name": "Claude",
  "status": "active",
  "type": "agent"
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |
| `invalid_input` | Type and name are required; type must be human, agent, service, or budget |

## Examples

### Create an AI agent resource

Register an AI agent with skills.

**Request:**

```json
{
  "capacity_model": "always_on",
  "metadata": {
    "model_id": "claude-opus-4-6"
  },
  "name": "Claude",
  "skills": [
    "code_review",
    "testing",
    "documentation"
  ],
  "type": "agent"
}
```

**Response:**

```json
{
  "created_at": "2026-02-07T18:00:00Z",
  "id": "res_a1b2c3d4e5f6...",
  "name": "Claude",
  "status": "active",
  "type": "agent"
}
```

## Related Tools

- [`ts.res.get`](../ts.res.get/)
- [`ts.res.list`](../ts.res.list/)
- [`ts.res.update`](../ts.res.update/)
- [`ts.org.add_resource`](../ts.org.add_resource/)

