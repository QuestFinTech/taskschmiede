---
title: "ts.msg.inbox"
description: "List unread/recent messages for current resource."
category: "message"
requires_auth: true
since: "v0.3.0"
type: docs
---

List unread/recent messages for current resource.

**Requires authentication**

## Description

Retrieves the inbox for the authenticated user's resource. Returns
messages delivered to the user, newest first.

Supports filtering by delivery status, message intent, entity
context, and unread flag.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `status` | string |  |  | Filter by status: pending, delivered, read |
| `intent` | string |  |  | Filter by intent: info, question, action, alert |
| `unread` | boolean |  |  | Show only unread messages (status != read) |
| `entity_type` | string |  |  | Filter by context entity type |
| `entity_id` | string |  |  | Filter by context entity ID |
| `limit` | integer |  | `50` | Max results (default: 50) |
| `offset` | integer |  | `0` | Pagination offset |

## Response

Returns a paginated list of inbox messages.

```json
{
  "limit": 50,
  "messages": [
    {
      "id": "msg_a1b2c3d4e5f6",
      "intent": "info",
      "status": "pending",
      "subject": "Update"
    }
  ],
  "offset": 0,
  "total": 1
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |

## Examples

### Check inbox

Get unread messages.

**Request:**

```json
{
  "unread": true
}
```

**Response:**

```json
{
  "limit": 50,
  "messages": [
    {
      "id": "msg_a1b2c3d4e5f6",
      "intent": "action",
      "status": "pending"
    }
  ],
  "offset": 0,
  "total": 1
}
```

## Related Tools

- [`ts.msg.read`](../ts.msg.read/)
- [`ts.msg.send`](../ts.msg.send/)

