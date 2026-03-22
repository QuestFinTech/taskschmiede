---
title: "Users"
description: "User management"
weight: 17
type: docs
---

User management

## Endpoints

| Method | Path | Summary |
|--------|------|---------- |
| `GET` | `/api/v1/users` | [List users (admin)](#list-users-admin) |
| `POST` | `/api/v1/users` | [Create user (admin)](#create-user-admin) |
| `GET` | `/api/v1/users/{id}` | [Get user](#get-user) |
| `PATCH` | `/api/v1/users/{id}` | [Update user](#update-user) |
| `POST` | `/api/v1/users/{id}/endeavours` | [Add user to endeavour](#add-user-to-endeavour) |

---

## List users (admin)

`GET /api/v1/users`

**Requires authentication.**

### Parameters

| Name | In | Type | Required | Description |
|------|-----|------|:--------:|-------------|
| `` |  | string |  |  |
| `` |  | string |  |  |
| `status` | query | string |  |  |
| `user_type` | query | string |  |  |
| `search` | query | string |  |  |

### Responses

| Code | Description |
|:----:|-------------|
| `200` | User list |

---

## Create user (admin)

`POST /api/v1/users`

**Requires authentication.**

### Request Body

| Field | Type | Required | Description |
|-------|------|:--------:|-------------|
| `email` | string <email> | Yes |  |
| `lang` | string |  |  |
| `metadata` | object |  |  |
| `name` | string | Yes |  |
| `password` | string | Yes | Min 12 chars, uppercase, lowercase, digit, special char |
| `timezone` | string |  |  |
| `user_type` | string |  |  |

### Responses

| Code | Description |
|:----:|-------------|
| `201` | User created |

---

## Get user

`GET /api/v1/users/{id}`

**Requires authentication.**

### Parameters

| Name | In | Type | Required | Description |
|------|-----|------|:--------:|-------------|
| `` |  | string |  |  |

### Responses

| Code | Description |
|:----:|-------------|
| `200` | User details |

---

## Update user

`PATCH /api/v1/users/{id}`

**Requires authentication.**

### Parameters

| Name | In | Type | Required | Description |
|------|-----|------|:--------:|-------------|
| `` |  | string |  |  |

### Request Body

| Field | Type | Required | Description |
|-------|------|:--------:|-------------|
| `email_copy` | boolean |  |  |
| `lang` | string |  |  |
| `metadata` | object |  |  |
| `name` | string |  |  |
| `status` | string |  |  |
| `timezone` | string |  |  |

### Responses

| Code | Description |
|:----:|-------------|
| `200` | User updated |

---

## Add user to endeavour

`POST /api/v1/users/{id}/endeavours`

**Requires authentication.**

### Parameters

| Name | In | Type | Required | Description |
|------|-----|------|:--------:|-------------|
| `` |  | string |  |  |

### Request Body

| Field | Type | Required | Description |
|-------|------|:--------:|-------------|
| `endeavour_id` | string | Yes |  |
| `role` | string | Yes |  |

### Responses

| Code | Description |
|:----:|-------------|
| `200` | User added to endeavour |

---

