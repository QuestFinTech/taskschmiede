---
title: "Messages"
description: "Internal messaging"
weight: 8
type: docs
---

Internal messaging

## Endpoints

| Method | Path | Summary |
|--------|------|---------- |
| `GET` | `/api/v1/messages` | [List inbox](#list-inbox) |
| `POST` | `/api/v1/messages` | [Send message](#send-message) |
| `GET` | `/api/v1/messages/{id}` | [Get message](#get-message) |
| `PATCH` | `/api/v1/messages/{id}` | [Mark message as read](#mark-message-as-read) |
| `POST` | `/api/v1/messages/{id}/reply` | [Reply to message](#reply-to-message) |
| `GET` | `/api/v1/messages/{id}/thread` | [Get message thread](#get-message-thread) |

---

## List inbox

`GET /api/v1/messages`

**Requires authentication.**

### Parameters

| Name | In | Type | Required | Description |
|------|-----|------|:--------:|-------------|
| `` |  | string |  |  |
| `` |  | string |  |  |
| `unread` | query | string |  |  |

### Responses

| Code | Description |
|:----:|-------------|
| `200` | Inbox messages |

---

## Send message

`POST /api/v1/messages`

**Requires authentication.**

### Request Body

| Field | Type | Required | Description |
|-------|------|:--------:|-------------|
| `content` | string | Yes |  |
| `entity_id` | string |  |  |
| `entity_type` | string |  |  |
| `intent` | string |  |  |
| `metadata` | object |  |  |
| `recipient_id` | string |  |  |
| `scope_id` | string |  |  |
| `scope_type` | string |  |  |
| `subject` | string | Yes |  |

### Responses

| Code | Description |
|:----:|-------------|
| `201` | Message sent |

---

## Get message

`GET /api/v1/messages/{id}`

**Requires authentication.**

### Parameters

| Name | In | Type | Required | Description |
|------|-----|------|:--------:|-------------|
| `` |  | string |  |  |

### Responses

| Code | Description |
|:----:|-------------|
| `200` | Message details |

---

## Mark message as read

`PATCH /api/v1/messages/{id}`

**Requires authentication.**

### Parameters

| Name | In | Type | Required | Description |
|------|-----|------|:--------:|-------------|
| `` |  | string |  |  |

### Responses

| Code | Description |
|:----:|-------------|
| `200` | Message marked read |

---

## Reply to message

`POST /api/v1/messages/{id}/reply`

**Requires authentication.**

### Parameters

| Name | In | Type | Required | Description |
|------|-----|------|:--------:|-------------|
| `` |  | string |  |  |

### Request Body

| Field | Type | Required | Description |
|-------|------|:--------:|-------------|
| `content` | string | Yes |  |
| `metadata` | object |  |  |

### Responses

| Code | Description |
|:----:|-------------|
| `201` | Reply sent |

---

## Get message thread

`GET /api/v1/messages/{id}/thread`

**Requires authentication.**

### Parameters

| Name | In | Type | Required | Description |
|------|-----|------|:--------:|-------------|
| `` |  | string |  |  |

### Responses

| Code | Description |
|:----:|-------------|
| `200` | Thread messages |

---

