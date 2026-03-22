---
title: "ts.rtl.list"
description: "Query rituals with filters."
category: "ritual"
requires_auth: true
since: "v0.2.0"
type: docs
---

Query rituals with filters.

**Requires authentication**

## Description

Lists rituals with optional filtering and pagination.

Filter by endeavour to find rituals governing a specific project. Filter by
origin to distinguish templates from custom rituals and forks.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `endeavour_id` | string |  |  | Filter by endeavour (via governs relationship) |
| `is_enabled` | boolean |  |  | Filter by enabled/disabled |
| `status` | string |  |  | Filter by status: active, archived |
| `origin` | string |  |  | Filter by origin: template, custom, fork |
| `search` | string |  |  | Search in name and description (partial match) |
| `limit` | integer |  | `50` | Maximum number of results to return |
| `offset` | integer |  | `0` | Number of results to skip (for pagination) |

## Response

Returns a paginated list of rituals.

```json
{
  "limit": 50,
  "offset": 0,
  "rituals": [
    {
      "created_at": "2026-02-09T10:00:00Z",
      "id": "rtl_a1b2c3d4e5f6...",
      "is_enabled": true,
      "name": "Daily standup",
      "origin": "custom",
      "status": "active",
      "updated_at": "2026-02-09T10:00:00Z"
    }
  ],
  "total": 1
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |

## Examples

### List active rituals for an endeavour

Find all enabled rituals governing a specific endeavour.

**Request:**

```json
{
  "endeavour_id": "edv_bd159eb7bb9a877a...",
  "is_enabled": true
}
```

**Response:**

```json
{
  "limit": 50,
  "offset": 0,
  "rituals": [
    {
      "id": "rtl_a1b2c3d4e5f6...",
      "is_enabled": true,
      "name": "Daily standup",
      "origin": "custom"
    }
  ],
  "total": 1
}
```

### List template rituals

Find all built-in methodology templates.

**Request:**

```json
{
  "origin": "template"
}
```

**Response:**

```json
{
  "limit": 50,
  "offset": 0,
  "rituals": [],
  "total": 0
}
```

## Related Tools

- [`ts.rtl.create`](../ts.rtl.create/)
- [`ts.rtl.get`](../ts.rtl.get/)
- [`ts.rtl.update`](../ts.rtl.update/)
- [`ts.rtl.fork`](../ts.rtl.fork/)

