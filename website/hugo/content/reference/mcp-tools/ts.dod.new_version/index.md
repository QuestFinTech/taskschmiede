---
title: "ts.dod.new_version"
description: "Create a new version of a DoD policy with updated conditions."
category: "dod"
requires_auth: true
since: "v0.3.0"
type: docs
---

Create a new version of a DoD policy with updated conditions.

**Requires authentication**

## Description

Creates a new version of an existing DoD policy. The old version is
archived and a predecessor link is established. Existing endorsements
on the old version are superseded -- team members must re-endorse the
new version.

Use this when conditions need to change (conditions cannot be modified
via ts.dod.update). Templates cannot be versioned directly -- fork them
first with ts.dod.create using origin "derived".

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `id` | string | Yes |  | ID of the policy to create a new version of |
| `name` | string |  |  | Name for the new version (defaults to source name) |
| `description` | string |  |  | Description (defaults to source description) |
| `conditions` | array |  |  | Array of condition objects with id, type, label, params, required |
| `strictness` | string |  |  | Strictness: all (default), n_of |
| `quorum` | integer |  |  | Required count when strictness is n_of |
| `metadata` | object |  |  | Arbitrary key-value pairs |

## Response

Returns the new policy version.

```json
{
  "conditions": [
    "..."
  ],
  "id": "dod_new123abc",
  "name": "Review Policy v2",
  "predecessor_id": "dod_a1b2c3d4e5f6",
  "status": "active",
  "version": 2
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |
| `invalid_input` | Policy ID is required |
| `not_found` | Source policy not found |
| `invalid_input` | Cannot version a template policy |

## Examples

### Update conditions on a policy

Create v2 of a policy with an additional peer review condition.

**Request:**

```json
{
  "conditions": [
    {
      "id": "cond_01",
      "label": "Comment needed",
      "params": {
        "min_comments": 1
      },
      "required": true,
      "type": "comment_required"
    },
    {
      "id": "cond_02",
      "label": "Peer review",
      "params": {
        "min_reviewers": 1
      },
      "required": true,
      "type": "peer_review"
    }
  ],
  "id": "dod_a1b2c3d4e5f6",
  "name": "Review Policy v2"
}
```

**Response:**

```json
{
  "id": "dod_new123abc",
  "name": "Review Policy v2",
  "predecessor_id": "dod_a1b2c3d4e5f6",
  "status": "active",
  "version": 2
}
```

## Related Tools

- [`ts.dod.update`](../ts.dod.update/)
- [`ts.dod.endorse`](../ts.dod.endorse/)
- [`ts.dod.get`](../ts.dod.get/)

