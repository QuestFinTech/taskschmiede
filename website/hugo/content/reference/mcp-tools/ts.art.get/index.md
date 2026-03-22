---
title: "ts.art.get"
description: "Retrieve an artifact by ID."
category: "artifact"
requires_auth: true
since: "v0.2.0"
type: docs
---

Retrieve an artifact by ID.

**Requires authentication**

## Description

Retrieves detailed information about a specific artifact, including
its kind, URL, tags, and any linked endeavour or task.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `id` | string | Yes |  | Artifact ID |

## Response

Returns the full artifact object.

```json
{
  "created_at": "2026-02-09T10:00:00Z",
  "endeavour_id": "edv_bd159eb7bb9a877a...",
  "id": "art_d4e5f6a1b2c3...",
  "kind": "doc",
  "metadata": {},
  "status": "active",
  "summary": "Documents the decision to migrate from hard-coded foreign keys to FRM.",
  "tags": [
    "architecture",
    "decision-record"
  ],
  "title": "Architecture Decision Record: FRM Migration",
  "updated_at": "2026-02-09T10:00:00Z",
  "url": "https://docs.example.com/adr-001"
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |
| `not_found` | Artifact with this ID does not exist |

## Examples

### Get artifact by ID

**Request:**

```json
{
  "id": "art_d4e5f6a1b2c3..."
}
```

**Response:**

```json
{
  "created_at": "2026-02-09T10:00:00Z",
  "endeavour_id": "edv_bd159eb7bb9a877a...",
  "id": "art_d4e5f6a1b2c3...",
  "kind": "doc",
  "status": "active",
  "title": "Architecture Decision Record: FRM Migration",
  "updated_at": "2026-02-09T10:00:00Z"
}
```

## Related Tools

- [`ts.art.create`](../ts.art.create/)
- [`ts.art.list`](../ts.art.list/)
- [`ts.art.update`](../ts.art.update/)

