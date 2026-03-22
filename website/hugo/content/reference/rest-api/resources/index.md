---
title: "Resources"
description: "People, teams, and other capacity units"
weight: 12
type: docs
---

People, teams, and other capacity units

## Endpoints

| Method | Path | Summary |
|--------|------|---------- |
| `GET` | `/api/v1/resources` | [List resources](#list-resources) |
| `POST` | `/api/v1/resources` | [Create resource](#create-resource) |
| `GET` | `/api/v1/resources/{id}` | [Get resource](#get-resource) |
| `PATCH` | `/api/v1/resources/{id}` | [Update resource](#update-resource) |
| `DELETE` | `/api/v1/resources/{id}` | [Delete resource](#delete-resource) |

---

## List resources

`GET /api/v1/resources`

**Requires authentication.**

### Parameters

| Name | In | Type | Required | Description |
|------|-----|------|:--------:|-------------|
| `` |  | string |  |  |
| `` |  | string |  |  |
| `type` | query | string |  |  |
| `endeavour_id` | query | string |  |  |
| `organization_id` | query | string |  |  |
| `search` | query | string |  |  |

### Responses

| Code | Description |
|:----:|-------------|
| `200` | Resource list |

---

## Create resource

`POST /api/v1/resources`

**Requires authentication.**

### Request Body

| Field | Type | Required | Description |
|-------|------|:--------:|-------------|
| `capacity_model` | string |  |  |
| `capacity_value` | number |  |  |
| `metadata` | object |  |  |
| `name` | string | Yes |  |
| `skills` | array |  |  |
| `type` | string | Yes |  |

### Responses

| Code | Description |
|:----:|-------------|
| `201` | Resource created |

---

## Get resource

`GET /api/v1/resources/{id}`

**Requires authentication.**

### Parameters

| Name | In | Type | Required | Description |
|------|-----|------|:--------:|-------------|
| `` |  | string |  |  |

### Responses

| Code | Description |
|:----:|-------------|
| `200` | Resource details |

---

## Update resource

`PATCH /api/v1/resources/{id}`

**Requires authentication.**

### Parameters

| Name | In | Type | Required | Description |
|------|-----|------|:--------:|-------------|
| `` |  | string |  |  |

### Request Body

| Field | Type | Required | Description |
|-------|------|:--------:|-------------|
| `capacity_model` | string |  |  |
| `capacity_value` | number |  |  |
| `metadata` | object |  |  |
| `name` | string |  |  |
| `skills` | array |  |  |
| `status` | string |  |  |
| `type` | string |  |  |

### Responses

| Code | Description |
|:----:|-------------|
| `200` | Resource updated |

---

## Delete resource

`DELETE /api/v1/resources/{id}`

**Requires authentication.**

### Parameters

| Name | In | Type | Required | Description |
|------|-----|------|:--------:|-------------|
| `` |  | string |  |  |

### Responses

| Code | Description |
|:----:|-------------|
| `200` | Resource deleted |

---

