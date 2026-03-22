---
title: "ts.org.add_endeavour"
description: "Associate an endeavour with an organization."
category: "organization"
requires_auth: true
since: "v0.2.0"
type: docs
---

Associate an endeavour with an organization.

**Requires authentication**

## Description

Links an endeavour to an organization. This creates a many-to-many
relationship: an endeavour can belong to multiple organizations, and
an organization can have multiple endeavours.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `organization_id` | string | Yes |  | Organization ID |
| `endeavour_id` | string | Yes |  | Endeavour ID to associate |
| `role` | string |  | `participant` | Role: owner, participant |

## Response

Returns the association confirmation.

```json
{
  "created_at": "2026-02-06T13:36:38Z",
  "endeavour_id": "edv_bd159eb7bb9a877a...",
  "organization_id": "org_1d9cb149497656c7...",
  "role": "owner"
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |
| `not_found` | Organization not found |
| `invalid_input` | organization_id and endeavour_id are required |

## Related Tools

- [`ts.org.create`](../ts.org.create/)
- [`ts.edv.create`](../ts.edv.create/)
- [`ts.edv.list`](../ts.edv.list/)

