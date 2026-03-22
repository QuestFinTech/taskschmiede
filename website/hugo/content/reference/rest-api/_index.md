---
title: REST API
description: Complete reference for the Taskschmiede REST API
weight: 10
type: docs
no_list: true
---

The Taskschmiede REST API provides HTTP endpoints for all operations.
All endpoints return JSON. Timestamps are UTC in RFC 3339 format.

[Download OpenAPI Specification (YAML)](/openapi.yaml)

## Authentication

Most endpoints require a bearer token in the `Authorization` header:

```
Authorization: Bearer <token>
```

Obtain a token by calling [POST /api/v1/auth/login](/reference/rest-api/auth/#login).

## Endpoint Groups

| Group | Description | Endpoints |
|-------|-------------|:---------:|
| [Approvals](approvals/) | Approval decisions | 3 |
| [Artifacts](artifacts/) | Documents, links, and deliverables | 4 |
| [Auth](auth/) | Authentication and user profile | 10 |
| [Comments](comments/) | Discussion on any entity | 5 |
| [Demands](demands/) | Work requests (stories, bugs, spikes, etc.) | 4 |
| [DoD](dod/) | Definition of Done policies and endorsements | 13 |
| [Endeavours](endeavours/) | Goal-oriented work containers | 7 |
| [Messages](messages/) | Internal messaging | 6 |
| [Organizations](organizations/) | Organizational units | 11 |
| [Relations](relations/) | Entity relationships | 3 |
| [Reports](reports/) | Report generation | 2 |
| [Resources](resources/) | People, teams, and other capacity units | 5 |
| [RitualRuns](ritualruns/) | Ritual execution records | 4 |
| [Rituals](rituals/) | Recurring process templates | 6 |
| [Tasks](tasks/) | Atomic units of work | 4 |
| [Templates](templates/) | Reusable templates | 6 |
| [Users](users/) | User management | 5 |
