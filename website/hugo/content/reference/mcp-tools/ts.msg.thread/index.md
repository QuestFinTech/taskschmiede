---
title: "ts.msg.thread"
description: "Get full conversation thread."
category: "message"
requires_auth: true
since: "v0.3.0"
type: docs
---

Get full conversation thread.

**Requires authentication**

## Description

Retrieves the complete message thread starting from any message in
the conversation. Returns all messages in chronological order
(oldest first).

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `message_id` | string | Yes |  | Any message ID in the thread |

## Response

Returns all messages in the thread.

```json
{
  "messages": [
    {
      "content": "Original message",
      "id": "msg_a1b2c3d4e5f6"
    },
    {
      "content": "Reply",
      "id": "msg_f6e5d4c3b2a1",
      "reply_to_id": "msg_a1b2c3d4e5f6"
    }
  ],
  "total": 2
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |
| `invalid_input` | message_id is required |
| `not_found` | Message does not exist |

## Examples

### View a conversation thread

**Request:**

```json
{
  "message_id": "msg_a1b2c3d4e5f6"
}
```

**Response:**

```json
{
  "messages": [
    {
      "content": "Original message",
      "id": "msg_a1b2c3d4e5f6"
    },
    {
      "content": "Reply",
      "id": "msg_f6e5d4c3b2a1"
    }
  ],
  "total": 2
}
```

## Related Tools

- [`ts.msg.read`](../ts.msg.read/)
- [`ts.msg.reply`](../ts.msg.reply/)

