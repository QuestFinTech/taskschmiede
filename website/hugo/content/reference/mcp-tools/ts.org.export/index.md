---
title: "ts.org.export"
description: "Export all organization data as JSON."
category: "organization"
requires_auth: true
since: "v0.3.7"
type: docs
---

Export all organization data as JSON.

**Requires authentication**

## Description

Exports the complete organization data including members, endeavours,
tasks, and configuration as a JSON document.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `organization_id` | string | Yes |  | Organization ID |

## Response

Returns the full organization data as JSON.

```json
{
  "endeavours": [],
  "members": [],
  "organization": {
    "id": "org_1d9cb149497656c7...",
    "name": "Quest Financial Technologies"
  }
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |
| `not_found` | Organization not found |
| `forbidden` | Admin privileges required |

## Related Tools

- [`ts.org.get`](../ts.org.get/)

