---
title: "Demands"
description: "Work requests (stories, bugs, spikes, etc.)"
weight: 5
type: docs
---

Work requests (stories, bugs, spikes, etc.)

## Endpoints

| Method | Path | Summary |
|--------|------|---------- |
| `GET` | `/api/v1/demands` | [List demands](#list-demands) |
| `POST` | `/api/v1/demands` | [Create demand](#create-demand) |
| `GET` | `/api/v1/demands/{id}` | [Get demand](#get-demand) |
| `PATCH` | `/api/v1/demands/{id}` | [Update demand](#update-demand) |

---

## List demands

`GET /api/v1/demands`

**Requires authentication.**

### Parameters

| Name | In | Type | Required | Description |
|------|-----|------|:--------:|-------------|
| `` |  | string |  |  |
| `` |  | string |  |  |
| `status` | query | string |  |  |
| `endeavour_id` | query | string |  |  |
| `type` | query | string |  |  |
| `priority` | query | string |  |  |
| `search` | query | string |  |  |

### Responses

| Code | Description |
|:----:|-------------|
| `200` | Demand list |

---

## Create demand

`POST /api/v1/demands`

**Requires authentication.**

### Request Body

| Field | Type | Required | Description |
|-------|------|:--------:|-------------|
| `description` | string |  |  |
| `due_date` | string <date-time> |  |  |
| `endeavour_id` | string | Yes |  |
| `metadata` | object |  |  |
| `priority` | string |  |  |
| `title` | string | Yes |  |
| `type` | string | Yes |  |

### Responses

| Code | Description |
|:----:|-------------|
| `201` | Demand created |

---

## Get demand

`GET /api/v1/demands/{id}`

**Requires authentication.**

### Parameters

| Name | In | Type | Required | Description |
|------|-----|------|:--------:|-------------|
| `` |  | string |  |  |

### Responses

| Code | Description |
|:----:|-------------|
| `200` | Demand details |

---

## Update demand

`PATCH /api/v1/demands/{id}`

**Requires authentication.**

### Parameters

| Name | In | Type | Required | Description |
|------|-----|------|:--------:|-------------|
| `` |  | string |  |  |

### Request Body

| Field | Type | Required | Description |
|-------|------|:--------:|-------------|
| `canceled_reason` | string |  |  |
| `description` | string |  |  |
| `due_date` | string <date-time> |  |  |
| `metadata` | object |  |  |
| `owner_id` | string |  |  |
| `priority` | string |  |  |
| `status` | string |  |  |
| `title` | string |  |  |

### Responses

| Code | Description |
|:----:|-------------|
| `200` | Demand updated |

---

