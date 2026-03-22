---
title: "Tasks"
description: "Atomic units of work"
weight: 15
type: docs
---

Atomic units of work

## Endpoints

| Method | Path | Summary |
|--------|------|---------- |
| `GET` | `/api/v1/tasks` | [List tasks](#list-tasks) |
| `POST` | `/api/v1/tasks` | [Create task](#create-task) |
| `GET` | `/api/v1/tasks/{id}` | [Get task](#get-task) |
| `PATCH` | `/api/v1/tasks/{id}` | [Update task](#update-task) |

---

## List tasks

`GET /api/v1/tasks`

**Requires authentication.**

### Parameters

| Name | In | Type | Required | Description |
|------|-----|------|:--------:|-------------|
| `` |  | string |  |  |
| `` |  | string |  |  |
| `status` | query | string |  |  |
| `endeavour_id` | query | string |  |  |
| `assignee_id` | query | string |  | Use 'me' to filter by current user |
| `demand_id` | query | string |  |  |
| `search` | query | string |  |  |
| `unassigned` | query | string |  |  |
| `summary` | query | string |  | Return status counts instead of list |

### Responses

| Code | Description |
|:----:|-------------|
| `200` | Task list |

---

## Create task

`POST /api/v1/tasks`

**Requires authentication.**

### Request Body

| Field | Type | Required | Description |
|-------|------|:--------:|-------------|
| `assignee_id` | string |  |  |
| `demand_id` | string |  |  |
| `description` | string |  |  |
| `due_date` | string <date-time> |  |  |
| `endeavour_id` | string | Yes |  |
| `estimate` | number |  |  |
| `metadata` | object |  |  |
| `title` | string | Yes |  |

### Responses

| Code | Description |
|:----:|-------------|
| `201` | Task created |
| `400` |  |

---

## Get task

`GET /api/v1/tasks/{id}`

**Requires authentication.**

### Parameters

| Name | In | Type | Required | Description |
|------|-----|------|:--------:|-------------|
| `` |  | string |  |  |

### Responses

| Code | Description |
|:----:|-------------|
| `200` | Task details |
| `404` |  |

---

## Update task

`PATCH /api/v1/tasks/{id}`

**Requires authentication.**

### Parameters

| Name | In | Type | Required | Description |
|------|-----|------|:--------:|-------------|
| `` |  | string |  |  |

### Request Body

| Field | Type | Required | Description |
|-------|------|:--------:|-------------|
| `actual` | number |  |  |
| `assignee_id` | string |  |  |
| `canceled_reason` | string |  |  |
| `demand_id` | string |  |  |
| `description` | string |  |  |
| `due_date` | string <date-time> |  |  |
| `endeavour_id` | string |  |  |
| `estimate` | number |  |  |
| `metadata` | object |  |  |
| `owner_id` | string |  |  |
| `status` | string |  |  |
| `title` | string |  |  |

### Responses

| Code | Description |
|:----:|-------------|
| `200` | Task updated |
| `400` |  |
| `404` |  |

---

