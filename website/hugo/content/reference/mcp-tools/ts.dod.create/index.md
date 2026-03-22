---
title: "ts.dod.create"
description: "Create a new Definition of Done policy."
category: "dod"
requires_auth: true
since: "v0.3.0"
type: docs
---

Create a new Definition of Done policy.

**Requires authentication**

## Description

Creates a DoD policy with a set of conditions that must be met before
a task can be considered done. Conditions are evaluated automatically
by ts.dod.check.

Each condition has a type (e.g., approval_count, field_set, status_is),
a label, optional parameters, and a required flag. The strictness setting
controls whether all conditions must pass ("all") or a minimum count
("n_of" with quorum).

Four built-in templates are seeded on first run.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `name` | string | Yes |  | Policy name |
| `description` | string |  |  | Policy description |
| `origin` | string |  | `custom` | Origin: custom (default), derived |
| `conditions` | array | Yes |  | Array of condition objects with id, type, label, params, required |
| `strictness` | string |  | `all` | Strictness: all (default), n_of |
| `quorum` | integer |  |  | Required count when strictness is n_of |
| `metadata` | object |  |  | Arbitrary key-value pairs |

## Response

Returns the created DoD policy.

```json
{
  "created_at": "2026-02-12T10:00:00Z",
  "id": "dod_a1b2c3d4e5f6",
  "name": "Standard Task Completion",
  "origin": "custom",
  "status": "active",
  "strictness": "all"
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |
| `invalid_input` | Name or conditions missing |

## Examples

### Create a DoD policy

Define a policy requiring approval and hours logged.

**Request:**

```json
{
  "conditions": [
    {
      "id": "c1",
      "label": "At least one approval",
      "params": {
        "min": 1
      },
      "required": true,
      "type": "approval_count"
    },
    {
      "id": "c2",
      "label": "Actual hours logged",
      "params": {
        "field": "actual"
      },
      "required": true,
      "type": "field_set"
    }
  ],
  "description": "Requires approval and actual hours logged.",
  "name": "Standard Task Completion"
}
```

**Response:**

```json
{
  "created_at": "2026-02-12T10:00:00Z",
  "id": "dod_a1b2c3d4e5f6",
  "name": "Standard Task Completion",
  "status": "active"
}
```

## Related Tools

- [`ts.dod.get`](../ts.dod.get/)
- [`ts.dod.list`](../ts.dod.list/)
- [`ts.dod.assign`](../ts.dod.assign/)
- [`ts.dod.check`](../ts.dod.check/)

