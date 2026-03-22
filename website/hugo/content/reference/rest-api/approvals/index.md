---
title: "Approvals"
description: "Approval decisions"
weight: 1
type: docs
---

Approval decisions

## Endpoints

| Method | Path | Summary |
|--------|------|---------- |
| `GET` | `/api/v1/approvals` | [List approvals](#list-approvals) |
| `POST` | `/api/v1/approvals` | [Create approval](#create-approval) |
| `GET` | `/api/v1/approvals/{id}` | [Get approval](#get-approval) |

---

## List approvals

`GET /api/v1/approvals`

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
| `200` | Approval list |

---

## Create approval

`POST /api/v1/approvals`

**Requires authentication.**

### Request Body

| Field | Type | Required | Description |
|-------|------|:--------:|-------------|
| `comment` | string |  |  |
| `entity_id` | string | Yes |  |
| `entity_type` | string | Yes |  |
| `metadata` | object |  |  |
| `role` | string |  |  |
| `verdict` | string | Yes |  |

### Responses

| Code | Description |
|:----:|-------------|
| `201` | Approval created |

---

## Get approval

`GET /api/v1/approvals/{id}`

**Requires authentication.**

### Parameters

| Name | In | Type | Required | Description |
|------|-----|------|:--------:|-------------|
| `` |  | string |  |  |

### Responses

| Code | Description |
|:----:|-------------|
| `200` | Approval details |

---

