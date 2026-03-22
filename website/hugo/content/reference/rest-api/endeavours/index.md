---
title: "Endeavours"
description: "Goal-oriented work containers"
weight: 7
type: docs
---

Goal-oriented work containers

## Endpoints

| Method | Path | Summary |
|--------|------|---------- |
| `GET` | `/api/v1/endeavours` | [List endeavours](#list-endeavours) |
| `POST` | `/api/v1/endeavours` | [Create endeavour](#create-endeavour) |
| `GET` | `/api/v1/endeavours/{id}` | [Get endeavour](#get-endeavour) |
| `PATCH` | `/api/v1/endeavours/{id}` | [Update endeavour](#update-endeavour) |
| `GET` | `/api/v1/endeavours/{id}/archive` | [Preview archive impact](#preview-archive-impact) |
| `POST` | `/api/v1/endeavours/{id}/archive` | [Archive endeavour](#archive-endeavour) |
| `GET` | `/api/v1/endeavours/{id}/export` | [Export endeavour data](#export-endeavour-data) |

---

## List endeavours

`GET /api/v1/endeavours`

**Requires authentication.**

### Parameters

| Name | In | Type | Required | Description |
|------|-----|------|:--------:|-------------|
| `` |  | string |  |  |
| `` |  | string |  |  |
| `status` | query | string |  |  |
| `organization_id` | query | string |  |  |
| `search` | query | string |  |  |

### Responses

| Code | Description |
|:----:|-------------|
| `200` | Endeavour list |

---

## Create endeavour

`POST /api/v1/endeavours`

**Requires authentication.**

### Request Body

| Field | Type | Required | Description |
|-------|------|:--------:|-------------|
| `description` | string |  |  |
| `end_date` | string <date-time> |  |  |
| `goals` | array |  |  |
| `lang` | string |  |  |
| `metadata` | object |  |  |
| `name` | string | Yes |  |
| `start_date` | string <date-time> |  |  |
| `timezone` | string |  |  |

### Responses

| Code | Description |
|:----:|-------------|
| `201` | Endeavour created |

---

## Get endeavour

`GET /api/v1/endeavours/{id}`

**Requires authentication.**

### Parameters

| Name | In | Type | Required | Description |
|------|-----|------|:--------:|-------------|
| `` |  | string |  |  |

### Responses

| Code | Description |
|:----:|-------------|
| `200` | Endeavour details |
| `404` |  |

---

## Update endeavour

`PATCH /api/v1/endeavours/{id}`

**Requires authentication.**

### Parameters

| Name | In | Type | Required | Description |
|------|-----|------|:--------:|-------------|
| `` |  | string |  |  |

### Request Body

| Field | Type | Required | Description |
|-------|------|:--------:|-------------|
| `description` | string |  |  |
| `end_date` | string <date-time> |  |  |
| `goals` | array |  |  |
| `lang` | string |  |  |
| `metadata` | object |  |  |
| `name` | string |  |  |
| `start_date` | string <date-time> |  |  |
| `status` | string |  |  |
| `timezone` | string |  |  |

### Responses

| Code | Description |
|:----:|-------------|
| `200` | Endeavour updated |

---

## Preview archive impact

`GET /api/v1/endeavours/{id}/archive`

**Requires authentication.**

### Parameters

| Name | In | Type | Required | Description |
|------|-----|------|:--------:|-------------|
| `` |  | string |  |  |

### Responses

| Code | Description |
|:----:|-------------|
| `200` | Archive impact summary |

---

## Archive endeavour

`POST /api/v1/endeavours/{id}/archive`

**Requires authentication.**

### Parameters

| Name | In | Type | Required | Description |
|------|-----|------|:--------:|-------------|
| `` |  | string |  |  |

### Request Body

| Field | Type | Required | Description |
|-------|------|:--------:|-------------|
| `reason` | string |  |  |

### Responses

| Code | Description |
|:----:|-------------|
| `200` | Endeavour archived |

---

## Export endeavour data

`GET /api/v1/endeavours/{id}/export`

**Requires authentication.**

### Parameters

| Name | In | Type | Required | Description |
|------|-----|------|:--------:|-------------|
| `` |  | string |  |  |

### Responses

| Code | Description |
|:----:|-------------|
| `200` | Full endeavour export (JSON) |

---

