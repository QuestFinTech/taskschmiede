---
title: "Rituals"
description: "Recurring process templates"
weight: 14
type: docs
---

Recurring process templates

## Endpoints

| Method | Path | Summary |
|--------|------|---------- |
| `GET` | `/api/v1/rituals` | [List rituals](#list-rituals) |
| `POST` | `/api/v1/rituals` | [Create ritual](#create-ritual) |
| `GET` | `/api/v1/rituals/{id}` | [Get ritual](#get-ritual) |
| `PATCH` | `/api/v1/rituals/{id}` | [Update ritual](#update-ritual) |
| `POST` | `/api/v1/rituals/{id}/fork` | [Fork ritual (create new version)](#fork-ritual-create-new-version) |
| `GET` | `/api/v1/rituals/{id}/lineage` | [Get ritual version lineage](#get-ritual-version-lineage) |

---

## List rituals

`GET /api/v1/rituals`

**Requires authentication.**

### Parameters

| Name | In | Type | Required | Description |
|------|-----|------|:--------:|-------------|
| `` |  | string |  |  |
| `` |  | string |  |  |
| `endeavour_id` | query | string |  |  |
| `search` | query | string |  |  |

### Responses

| Code | Description |
|:----:|-------------|
| `200` | Ritual list |

---

## Create ritual

`POST /api/v1/rituals`

**Requires authentication.**

### Request Body

| Field | Type | Required | Description |
|-------|------|:--------:|-------------|
| `description` | string |  |  |
| `endeavour_id` | string |  |  |
| `is_enabled` | boolean |  |  |
| `lang` | string |  |  |
| `metadata` | object |  |  |
| `name` | string | Yes |  |
| `prompt` | string |  |  |
| `schedule` | object |  |  |

### Responses

| Code | Description |
|:----:|-------------|
| `201` | Ritual created |

---

## Get ritual

`GET /api/v1/rituals/{id}`

**Requires authentication.**

### Parameters

| Name | In | Type | Required | Description |
|------|-----|------|:--------:|-------------|
| `` |  | string |  |  |

### Responses

| Code | Description |
|:----:|-------------|
| `200` | Ritual details |

---

## Update ritual

`PATCH /api/v1/rituals/{id}`

**Requires authentication.**

### Parameters

| Name | In | Type | Required | Description |
|------|-----|------|:--------:|-------------|
| `` |  | string |  |  |

### Request Body

| Field | Type | Required | Description |
|-------|------|:--------:|-------------|
| `description` | string |  |  |
| `is_enabled` | boolean |  |  |
| `lang` | string |  |  |
| `metadata` | object |  |  |
| `name` | string |  |  |
| `prompt` | string |  |  |
| `schedule` | object |  |  |
| `status` | string |  |  |

### Responses

| Code | Description |
|:----:|-------------|
| `200` | Ritual updated |

---

## Fork ritual (create new version)

`POST /api/v1/rituals/{id}/fork`

**Requires authentication.**

### Parameters

| Name | In | Type | Required | Description |
|------|-----|------|:--------:|-------------|
| `` |  | string |  |  |

### Responses

| Code | Description |
|:----:|-------------|
| `201` | Ritual forked |

---

## Get ritual version lineage

`GET /api/v1/rituals/{id}/lineage`

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

