---
title: "ts.org.get"
description: "Retrieve an organization by ID."
category: "organization"
requires_auth: true
since: "v0.2.0"
type: docs
---

Retrieve an organization by ID.

**Requires authentication**

## Description

Retrieves detailed information about a specific organization,
including member count and endeavour count.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `id` | string | Yes |  | Organization ID |

## Response

Returns the organization with member and endeavour counts.

```json
{
  "created_at": "2026-02-06T13:36:24Z",
  "description": "Software and consulting for financial technology",
  "endeavour_count": 1,
  "id": "org_1d9cb149497656c7...",
  "member_count": 3,
  "metadata": {},
  "name": "Quest Financial Technologies",
  "status": "active",
  "updated_at": "2026-02-06T13:36:24Z"
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |
| `not_found` | Organization with this ID does not exist |

## Examples

### Get organization by ID

**Request:**

```json
{
  "id": "org_1d9cb149497656c7..."
}
```

**Response:**

```json
{
  "created_at": "2026-02-06T13:36:24Z",
  "endeavour_count": 1,
  "id": "org_1d9cb149497656c7...",
  "member_count": 3,
  "name": "Quest Financial Technologies",
  "status": "active",
  "updated_at": "2026-02-06T13:36:24Z"
}
```

## Related Tools

- [`ts.org.create`](../ts.org.create/)
- [`ts.org.list`](../ts.org.list/)

