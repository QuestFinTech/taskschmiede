---
title: "Artifacts"
description: "Documents, links, and deliverables"
weight: 2
type: docs
---

Documents, links, and deliverables

## Endpoints

| Method | Path | Summary |
|--------|------|---------- |
| `GET` | `/api/v1/artifacts` | [List artifacts](#list-artifacts) |
| `POST` | `/api/v1/artifacts` | [Create artifact](#create-artifact) |
| `GET` | `/api/v1/artifacts/{id}` | [Get artifact](#get-artifact) |
| `PATCH` | `/api/v1/artifacts/{id}` | [Update artifact](#update-artifact) |

---

## List artifacts

`GET /api/v1/artifacts`

**Requires authentication.**

### Parameters

| Name | In | Type | Required | Description |
|------|-----|------|:--------:|-------------|
| `` |  | string |  |  |
| `` |  | string |  |  |
| `endeavour_id` | query | string |  |  |
| `task_id` | query | string |  |  |
| `kind` | query | string |  |  |
| `search` | query | string |  |  |

### Responses

| Code | Description |
|:----:|-------------|
| `200` | Artifact list |

---

## Create artifact

`POST /api/v1/artifacts`

**Requires authentication.**

### Request Body

| Field | Type | Required | Description |
|-------|------|:--------:|-------------|
| `endeavour_id` | string |  |  |
| `kind` | string | Yes |  |
| `metadata` | object |  |  |
| `summary` | string |  |  |
| `tags` | array |  |  |
| `task_id` | string |  |  |
| `title` | string | Yes |  |
| `url` | string |  |  |

### Responses

| Code | Description |
|:----:|-------------|
| `201` | Artifact created |

---

## Get artifact

`GET /api/v1/artifacts/{id}`

**Requires authentication.**

### Parameters

| Name | In | Type | Required | Description |
|------|-----|------|:--------:|-------------|
| `` |  | string |  |  |

### Responses

| Code | Description |
|:----:|-------------|
| `200` | Artifact details |

---

## Update artifact

`PATCH /api/v1/artifacts/{id}`

**Requires authentication.**

### Parameters

| Name | In | Type | Required | Description |
|------|-----|------|:--------:|-------------|
| `` |  | string |  |  |

### Request Body

| Field | Type | Required | Description |
|-------|------|:--------:|-------------|
| `kind` | string |  |  |
| `metadata` | object |  |  |
| `status` | string |  |  |
| `summary` | string |  |  |
| `tags` | array |  |  |
| `title` | string |  |  |
| `url` | string |  |  |

### Responses

| Code | Description |
|:----:|-------------|
| `200` | Artifact updated |

---

