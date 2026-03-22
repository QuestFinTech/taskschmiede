---
title: "ts.res.list"
description: "Query resources with filters."
category: "resource"
requires_auth: true
since: "v0.2.2"
type: docs
---

Query resources with filters.

**Requires authentication**

## Description

Lists resources with optional filtering by type, status, organization
membership, and text search. Supports pagination.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `type` | string |  |  | Filter by type: human, agent, service, budget |
| `status` | string |  |  | Filter by status: active, inactive |
| `organization_id` | string |  |  | Filter by organization membership |
| `search` | string |  |  | Search by name (partial match) |
| `limit` | integer |  | `50` | Maximum number of results to return |
| `offset` | integer |  | `0` | Number of results to skip (for pagination) |

## Response

Returns a paginated list of resources.

```json
{
  "limit": 50,
  "offset": 0,
  "resources": [
    {
      "created_at": "2026-02-07T18:00:00Z",
      "id": "res_a1b2c3d4e5f6...",
      "name": "Claude",
      "status": "active",
      "type": "agent",
      "updated_at": "2026-02-07T18:00:00Z"
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

- [`ts.res.create`](../ts.res.create/)
- [`ts.res.get`](../ts.res.get/)
- [`ts.res.update`](../ts.res.update/)
- [`ts.org.add_resource`](../ts.org.add_resource/)

