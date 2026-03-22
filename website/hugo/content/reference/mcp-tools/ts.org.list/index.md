---
title: "ts.org.list"
description: "Query organizations with filters."
category: "organization"
requires_auth: true
since: "v0.2.0"
type: docs
---

Query organizations with filters.

**Requires authentication**

## Description

Lists organizations with optional filtering and pagination.

Results include basic organization info. Use ts.org.get for full details
including member and endeavour counts.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `status` | string |  |  | Filter by status: active, inactive, archived |
| `search` | string |  |  | Search by name (partial match) |
| `limit` | integer |  | `50` | Maximum number of results to return |
| `offset` | integer |  | `0` | Number of results to skip (for pagination) |

## Response

Returns a paginated list of organizations.

```json
{
  "limit": 50,
  "offset": 0,
  "organizations": [
    {
      "created_at": "2026-02-06T13:36:24Z",
      "description": "Software and consulting for financial technology",
      "id": "org_1d9cb149497656c7...",
      "name": "Quest Financial Technologies",
      "status": "active",
      "updated_at": "2026-02-06T13:36:24Z"
    }
  ],
  "total": 1
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |

## Related Tools

- [`ts.org.create`](../ts.org.create/)
- [`ts.org.get`](../ts.org.get/)

