---
title: "ts.org.add_resource"
description: "Add a resource to an organization."
category: "organization"
requires_auth: true
since: "v0.2.0"
type: docs
---

Add a resource to an organization.

**Requires authentication**

## Description

Adds a non-user resource (budget, service, equipment) to an organization.

For adding users (humans or agents) as members, use ts.org.add_member instead.
This tool is for non-user capacity entities that need to be associated with an
organization -- e.g., shared infrastructure, budget allocations, or external services.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `organization_id` | string | Yes |  | Organization ID |
| `resource_id` | string | Yes |  | Resource ID to add |
| `role` | string |  | `member` | Role: owner, admin, member, guest |

## Response

Returns the membership confirmation.

```json
{
  "joined_at": "2026-02-06T13:36:43Z",
  "organization_id": "org_1d9cb149497656c7...",
  "resource_id": "res_claude",
  "role": "member"
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |
| `not_found` | Organization not found |
| `invalid_input` | organization_id and resource_id are required |

## Related Tools

- [`ts.org.create`](../ts.org.create/)
- [`ts.org.get`](../ts.org.get/)

