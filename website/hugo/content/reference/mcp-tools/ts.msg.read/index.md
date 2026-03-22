---
title: "ts.msg.read"
description: "Get a message and mark as read."
category: "message"
requires_auth: true
since: "v0.3.0"
type: docs
---

Get a message and mark as read.

**Requires authentication**

## Description

Retrieves a message by ID and marks the delivery as read for the
authenticated user. The read_at timestamp is set automatically.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `id` | string | Yes |  | Message ID |

## Response

Returns the full message content.

```json
{
  "content": "The deployment is complete. All services are green.",
  "id": "msg_a1b2c3d4e5f6",
  "intent": "info",
  "read_at": "2026-02-12T10:05:00Z",
  "sender_id": "res_x1y2z3a4b5c6",
  "subject": "Deployment Status Update"
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |
| `invalid_input` | Message ID is required |
| `not_found` | Message not found or not delivered to this user |

## Examples

### Read a message

**Request:**

```json
{
  "id": "msg_a1b2c3d4e5f6"
}
```

**Response:**

```json
{
  "content": "The deployment is complete.",
  "id": "msg_a1b2c3d4e5f6",
  "intent": "info",
  "read_at": "2026-02-12T10:05:00Z",
  "sender_id": "res_x1y2z3a4b5c6"
}
```

## Related Tools

- [`ts.msg.inbox`](../ts.msg.inbox/)
- [`ts.msg.reply`](../ts.msg.reply/)

