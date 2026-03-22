---
title: "ts.msg.send"
description: "Send a message to one or more recipients."
category: "message"
requires_auth: true
since: "v0.3.0"
type: docs
---

Send a message to one or more recipients.

**Requires authentication**

## Description

Sends an internal message with support for direct delivery,
endeavour-scoped broadcast, or organization-scoped broadcast.

Messages have an intent field (info, question, action, alert) to help
recipients prioritize. Supports threading via reply_to_id and optional
entity context linking.

At least one delivery target is required: either recipient_ids for
direct messages or scope_type + scope_id for group delivery.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `content` | string | Yes |  | Message body (Markdown) |
| `subject` | string |  |  | Message subject (optional) |
| `intent` | string |  | `info` | Message intent: info, question, action, alert (default: info) |
| `recipient_ids` | array |  |  | Resource IDs of direct recipients |
| `scope_type` | string |  |  | Scope for group delivery: endeavour, organization |
| `scope_id` | string |  |  | ID of the endeavour or organization (required when scope_type is set) |
| `reply_to_id` | string |  |  | Message ID to reply to (creates a thread) |
| `entity_type` | string |  |  | Optional context: entity type (task, endeavour, ...) |
| `entity_id` | string |  |  | Optional context: entity ID |
| `metadata` | object |  |  | Arbitrary key-value pairs |

## Response

Returns the sent message with delivery information.

```json
{
  "content": "The deployment is complete.",
  "created_at": "2026-02-12T10:00:00Z",
  "deliveries": 2,
  "id": "msg_a1b2c3d4e5f6",
  "intent": "info",
  "sender_id": "res_x1y2z3a4b5c6"
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |
| `invalid_input` | content is required |
| `invalid_input` | No delivery target (recipient_ids or scope_type+scope_id) |

## Examples

### Send a direct message

Send a message to specific recipients.

**Request:**

```json
{
  "content": "The deployment is complete. All services are green.",
  "intent": "info",
  "recipient_ids": [
    "res_x1y2z3a4b5c6"
  ],
  "subject": "Deployment Status Update"
}
```

**Response:**

```json
{
  "created_at": "2026-02-12T10:00:00Z",
  "deliveries": 1,
  "id": "msg_a1b2c3d4e5f6",
  "sender_id": "res_x1y2z3a4b5c6"
}
```

### Broadcast to an endeavour

Send a message to all members of an endeavour.

**Request:**

```json
{
  "content": "Sprint planning tomorrow at 10:00 UTC.",
  "intent": "action",
  "scope_id": "edv_a1b2c3d4e5f6",
  "scope_type": "endeavour"
}
```

**Response:**

```json
{
  "created_at": "2026-02-12T10:00:00Z",
  "deliveries": 5,
  "id": "msg_f6e5d4c3b2a1",
  "sender_id": "res_x1y2z3a4b5c6"
}
```

## Related Tools

- [`ts.msg.inbox`](../ts.msg.inbox/)
- [`ts.msg.reply`](../ts.msg.reply/)
- [`ts.msg.thread`](../ts.msg.thread/)

