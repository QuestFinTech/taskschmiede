---
title: "Organizations"
description: "Organizational units"
weight: 9
type: docs
---

Organizational units

## Endpoints

| Method | Path | Summary |
|--------|------|---------- |
| `GET` | `/api/v1/organizations` | [List organizations](#list-organizations) |
| `POST` | `/api/v1/organizations` | [Create organization](#create-organization) |
| `GET` | `/api/v1/organizations/{id}` | [Get organization](#get-organization) |
| `PATCH` | `/api/v1/organizations/{id}` | [Update organization](#update-organization) |
| `GET` | `/api/v1/organizations/{id}/alert-terms` | [List alert terms](#list-alert-terms) |
| `PUT` | `/api/v1/organizations/{id}/alert-terms` | [Update alert terms](#update-alert-terms) |
| `GET` | `/api/v1/organizations/{id}/archive` | [Preview archive impact](#preview-archive-impact) |
| `POST` | `/api/v1/organizations/{id}/archive` | [Archive organization](#archive-organization) |
| `POST` | `/api/v1/organizations/{id}/endeavours` | [Add endeavour to organization](#add-endeavour-to-organization) |
| `GET` | `/api/v1/organizations/{id}/export` | [Export organization data](#export-organization-data) |
| `POST` | `/api/v1/organizations/{id}/resources` | [Add resource to organization](#add-resource-to-organization) |

---

## List organizations

`GET /api/v1/organizations`

**Requires authentication.**

### Parameters

| Name | In | Type | Required | Description |
|------|-----|------|:--------:|-------------|
| `` |  | string |  |  |
| `` |  | string |  |  |
| `search` | query | string |  |  |

### Responses

| Code | Description |
|:----:|-------------|
| `200` | Organization list |

---

## Create organization

`POST /api/v1/organizations`

**Requires authentication.**

### Request Body

| Field | Type | Required | Description |
|-------|------|:--------:|-------------|
| `description` | string |  |  |
| `metadata` | object |  |  |
| `name` | string | Yes |  |

### Responses

| Code | Description |
|:----:|-------------|
| `201` | Organization created |

---

## Get organization

`GET /api/v1/organizations/{id}`

**Requires authentication.**

### Parameters

| Name | In | Type | Required | Description |
|------|-----|------|:--------:|-------------|
| `` |  | string |  |  |

### Responses

| Code | Description |
|:----:|-------------|
| `200` | Organization details |

---

## Update organization

`PATCH /api/v1/organizations/{id}`

**Requires authentication.**

### Parameters

| Name | In | Type | Required | Description |
|------|-----|------|:--------:|-------------|
| `` |  | string |  |  |

### Request Body

| Field | Type | Required | Description |
|-------|------|:--------:|-------------|
| `description` | string |  |  |
| `metadata` | object |  |  |
| `name` | string |  |  |

### Responses

| Code | Description |
|:----:|-------------|
| `200` | Organization updated |

---

## List alert terms

`GET /api/v1/organizations/{id}/alert-terms`

**Requires authentication.**

### Parameters

| Name | In | Type | Required | Description |
|------|-----|------|:--------:|-------------|
| `` |  | string |  |  |

### Responses

| Code | Description |
|:----:|-------------|
| `200` | Alert terms list |

---

## Update alert terms

`PUT /api/v1/organizations/{id}/alert-terms`

**Requires authentication.**

### Parameters

| Name | In | Type | Required | Description |
|------|-----|------|:--------:|-------------|
| `` |  | string |  |  |

### Request Body

| Field | Type | Required | Description |
|-------|------|:--------:|-------------|
| `terms` | array |  |  |

### Responses

| Code | Description |
|:----:|-------------|
| `200` | Alert terms updated |

---

## Preview archive impact

`GET /api/v1/organizations/{id}/archive`

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

## Archive organization

`POST /api/v1/organizations/{id}/archive`

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
| `200` | Organization archived |

---

## Add endeavour to organization

`POST /api/v1/organizations/{id}/endeavours`

**Requires authentication.**

### Parameters

| Name | In | Type | Required | Description |
|------|-----|------|:--------:|-------------|
| `` |  | string |  |  |

### Request Body

| Field | Type | Required | Description |
|-------|------|:--------:|-------------|
| `endeavour_id` | string | Yes |  |

### Responses

| Code | Description |
|:----:|-------------|
| `200` | Endeavour added |

---

## Export organization data

`GET /api/v1/organizations/{id}/export`

**Requires authentication.**

### Parameters

| Name | In | Type | Required | Description |
|------|-----|------|:--------:|-------------|
| `` |  | string |  |  |

### Responses

| Code | Description |
|:----:|-------------|
| `200` | Full organization export (JSON) |

---

## Add resource to organization

`POST /api/v1/organizations/{id}/resources`

**Requires authentication.**

### Parameters

| Name | In | Type | Required | Description |
|------|-----|------|:--------:|-------------|
| `` |  | string |  |  |

### Request Body

| Field | Type | Required | Description |
|-------|------|:--------:|-------------|
| `resource_id` | string | Yes |  |
| `role` | string |  |  |

### Responses

| Code | Description |
|:----:|-------------|
| `200` | Resource added |

---

