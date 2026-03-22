---
title: "ts.org.create"
description: "Create a new organization."
category: "organization"
requires_auth: true
since: "v0.2.0"
type: docs
---

Create a new organization.

**Requires authentication**

## Description

Creates a new organization in Taskschmiede.

Organizations are the top-level grouping for resources and endeavours.
They represent teams, companies, or any group that works together.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `name` | string | Yes |  | Organization name |
| `description` | string |  |  | Organization description |
| `metadata` | object |  |  | Arbitrary key-value pairs |

## Response

Returns the created organization summary.

```json
{
  "created_at": "2026-02-06T13:36:24Z",
  "id": "org_1d9cb149497656c7...",
  "name": "Quest Financial Technologies",
  "status": "active"
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |
| `invalid_input` | Name is required |

## Examples

### Create an organization

Create a new organization for a team.

**Request:**

```json
{
  "description": "Software and consulting for financial technology",
  "name": "Quest Financial Technologies"
}
```

**Response:**

```json
{
  "created_at": "2026-02-06T13:36:24Z",
  "id": "org_1d9cb149497656c7...",
  "name": "Quest Financial Technologies",
  "status": "active"
}
```

## Related Tools

- [`ts.org.get`](../ts.org.get/)
- [`ts.org.list`](../ts.org.list/)
- [`ts.org.add_resource`](../ts.org.add_resource/)
- [`ts.org.add_endeavour`](../ts.org.add_endeavour/)

