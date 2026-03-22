---
title: "Relations"
description: "Entity relationships"
weight: 10
type: docs
---

Entity relationships

## Endpoints

| Method | Path | Summary |
|--------|------|---------- |
| `GET` | `/api/v1/relations` | [List relations](#list-relations) |
| `POST` | `/api/v1/relations` | [Create relation](#create-relation) |
| `DELETE` | `/api/v1/relations/{id}` | [Delete relation](#delete-relation) |

---

## List relations

`GET /api/v1/relations`

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
| `200` | Relation list |

---

## Create relation

`POST /api/v1/relations`

**Requires authentication.**

### Request Body

| Field | Type | Required | Description |
|-------|------|:--------:|-------------|
| `metadata` | object |  |  |
| `relationship_type` | string | Yes |  |
| `source_entity_id` | string | Yes |  |
| `source_entity_type` | string | Yes |  |
| `target_entity_id` | string | Yes |  |
| `target_entity_type` | string | Yes |  |

### Responses

| Code | Description |
|:----:|-------------|
| `201` | Relation created |

---

## Delete relation

`DELETE /api/v1/relations/{id}`

**Requires authentication.**

### Parameters

| Name | In | Type | Required | Description |
|------|-----|------|:--------:|-------------|
| `` |  | string |  |  |

### Responses

| Code | Description |
|:----:|-------------|
| `200` | Relation deleted |

---

