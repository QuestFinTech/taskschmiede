---
title: "Comments"
description: "Discussion on any entity"
weight: 4
type: docs
---

Discussion on any entity

## Endpoints

| Method | Path | Summary |
|--------|------|---------- |
| `GET` | `/api/v1/comments` | [List comments](#list-comments) |
| `POST` | `/api/v1/comments` | [Create comment](#create-comment) |
| `GET` | `/api/v1/comments/{id}` | [Get comment](#get-comment) |
| `PATCH` | `/api/v1/comments/{id}` | [Update comment](#update-comment) |
| `DELETE` | `/api/v1/comments/{id}` | [Delete comment (soft delete)](#delete-comment-soft-delete) |

---

## List comments

`GET /api/v1/comments`

**Requires authentication.**

### Parameters

| Name | In | Type | Required | Description |
|------|-----|------|:--------:|-------------|
| `` |  | string |  |  |
| `` |  | string |  |  |
| `entity_type` | query | string |  |  |
| `entity_id` | query | string |  |  |

### Responses

| Code | Description |
|:----:|-------------|
| `200` | Comment list |

---

## Create comment

`POST /api/v1/comments`

**Requires authentication.**

### Request Body

| Field | Type | Required | Description |
|-------|------|:--------:|-------------|
| `content` | string | Yes |  |
| `entity_id` | string | Yes |  |
| `entity_type` | string | Yes |  |
| `metadata` | object |  |  |
| `reply_to_id` | string |  |  |

### Responses

| Code | Description |
|:----:|-------------|
| `201` | Comment created |

---

## Get comment

`GET /api/v1/comments/{id}`

**Requires authentication.**

### Parameters

| Name | In | Type | Required | Description |
|------|-----|------|:--------:|-------------|
| `` |  | string |  |  |

### Responses

| Code | Description |
|:----:|-------------|
| `200` | Comment details |

---

## Update comment

`PATCH /api/v1/comments/{id}`

**Requires authentication.**

### Parameters

| Name | In | Type | Required | Description |
|------|-----|------|:--------:|-------------|
| `` |  | string |  |  |

### Request Body

| Field | Type | Required | Description |
|-------|------|:--------:|-------------|
| `content` | string |  |  |
| `metadata` | object |  |  |

### Responses

| Code | Description |
|:----:|-------------|
| `200` | Comment updated |

---

## Delete comment (soft delete)

`DELETE /api/v1/comments/{id}`

**Requires authentication.**

### Parameters

| Name | In | Type | Required | Description |
|------|-----|------|:--------:|-------------|
| `` |  | string |  |  |

### Responses

| Code | Description |
|:----:|-------------|
| `200` | Comment deleted |

---

