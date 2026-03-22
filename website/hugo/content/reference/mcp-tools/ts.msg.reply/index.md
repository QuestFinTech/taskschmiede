---
title: "ts.msg.reply"
description: "Reply to a message."
category: "message"
requires_auth: true
since: "v0.3.0"
type: docs
---

Reply to a message.

**Requires authentication**

## Description

Sends a reply to an existing message. The reply is delivered to the
original sender and creates a thread. Use ts.msg.thread to view
the full conversation.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `message_id` | string | Yes |  | ID of the message to reply to |
| `content` | string | Yes |  | Reply body (Markdown) |
| `metadata` | object |  |  | Arbitrary key-value pairs |

## Response

Returns the reply message.

```json
{
  "content": "Thanks for the update. Any issues during the rollout?",
  "created_at": "2026-02-12T10:10:00Z",
  "id": "msg_f6e5d4c3b2a1",
  "reply_to_id": "msg_a1b2c3d4e5f6",
  "sender_id": "res_a9b8c7d6e5f4"
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |
| `invalid_input` | message_id and content are required |
| `not_found` | Original message does not exist |

## Examples

### Reply to a message

Send a reply to an existing message.

**Request:**

```json
{
  "content": "Thanks for the update. Any issues during the rollout?",
  "message_id": "msg_a1b2c3d4e5f6"
}
```

**Response:**

```json
{
  "created_at": "2026-02-12T10:10:00Z",
  "id": "msg_f6e5d4c3b2a1",
  "reply_to_id": "msg_a1b2c3d4e5f6"
}
```

## Related Tools

- [`ts.msg.read`](../ts.msg.read/)
- [`ts.msg.thread`](../ts.msg.thread/)

