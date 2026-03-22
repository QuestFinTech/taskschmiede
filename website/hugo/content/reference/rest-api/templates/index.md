---
title: "Templates"
description: "Reusable templates"
weight: 16
type: docs
---

Reusable templates

## Endpoints

| Method | Path | Summary |
|--------|------|---------- |
| `GET` | `/api/v1/templates` | [List templates](#list-templates) |
| `POST` | `/api/v1/templates` | [Create template](#create-template) |
| `GET` | `/api/v1/templates/{id}` | [Get template](#get-template) |
| `PATCH` | `/api/v1/templates/{id}` | [Update template](#update-template) |
| `POST` | `/api/v1/templates/{id}/fork` | [Fork template](#fork-template) |
| `GET` | `/api/v1/templates/{id}/lineage` | [Get template lineage](#get-template-lineage) |

---

## List templates

`GET /api/v1/templates`

**Requires authentication.**

### Parameters

| Name | In | Type | Required | Description |
|------|-----|------|:--------:|-------------|
| `` |  | string |  |  |
| `` |  | string |  |  |
| `scope` | query | string |  |  |
| `search` | query | string |  |  |

### Responses

| Code | Description |
|:----:|-------------|
| `200` | Template list |

---

## Create template

`POST /api/v1/templates`

**Requires authentication.**

### Request Body

| Field | Type | Required | Description |
|-------|------|:--------:|-------------|
| `body` | string | Yes |  |
| `lang` | string |  |  |
| `metadata` | object |  |  |
| `name` | string | Yes |  |
| `scope` | string | Yes |  |

### Responses

| Code | Description |
|:----:|-------------|
| `201` | Template created |

---

## Get template

`GET /api/v1/templates/{id}`

**Requires authentication.**

### Parameters

| Name | In | Type | Required | Description |
|------|-----|------|:--------:|-------------|
| `` |  | string |  |  |

### Responses

| Code | Description |
|:----:|-------------|
| `200` | Template details |

---

## Update template

`PATCH /api/v1/templates/{id}`

**Requires authentication.**

### Parameters

| Name | In | Type | Required | Description |
|------|-----|------|:--------:|-------------|
| `` |  | string |  |  |

### Request Body

| Field | Type | Required | Description |
|-------|------|:--------:|-------------|
| `body` | string |  |  |
| `lang` | string |  |  |
| `metadata` | object |  |  |
| `name` | string |  |  |
| `scope` | string |  |  |
| `status` | string |  |  |

### Responses

| Code | Description |
|:----:|-------------|
| `200` | Template updated |

---

## Fork template

`POST /api/v1/templates/{id}/fork`

**Requires authentication.**

### Parameters

| Name | In | Type | Required | Description |
|------|-----|------|:--------:|-------------|
| `` |  | string |  |  |

### Responses

| Code | Description |
|:----:|-------------|
| `201` | Template forked |

---

## Get template lineage

`GET /api/v1/templates/{id}/lineage`

**Requires authentication.**

### Parameters

| Name | In | Type | Required | Description |
|------|-----|------|:--------:|-------------|
| `` |  | string |  |  |

### Responses

| Code | Description |
|:----:|-------------|
| `200` | Lineage chain |

---

