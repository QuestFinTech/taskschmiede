---
title: "RitualRuns"
description: "Ritual execution records"
weight: 13
type: docs
---

Ritual execution records

## Endpoints

| Method | Path | Summary |
|--------|------|---------- |
| `GET` | `/api/v1/ritual-runs` | [List ritual runs](#list-ritual-runs) |
| `POST` | `/api/v1/ritual-runs` | [Create ritual run](#create-ritual-run) |
| `GET` | `/api/v1/ritual-runs/{id}` | [Get ritual run](#get-ritual-run) |
| `PATCH` | `/api/v1/ritual-runs/{id}` | [Update ritual run](#update-ritual-run) |

---

## List ritual runs

`GET /api/v1/ritual-runs`

**Requires authentication.**

### Parameters

| Name | In | Type | Required | Description |
|------|-----|------|:--------:|-------------|
| `` |  | string |  |  |
| `` |  | string |  |  |
| `ritual_id` | query | string |  |  |
| `status` | query | string |  |  |

### Responses

| Code | Description |
|:----:|-------------|
| `200` | Ritual run list |

---

## Create ritual run

`POST /api/v1/ritual-runs`

**Requires authentication.**

### Request Body

| Field | Type | Required | Description |
|-------|------|:--------:|-------------|
| `metadata` | object |  |  |
| `ritual_id` | string | Yes |  |
| `trigger` | string |  |  |

### Responses

| Code | Description |
|:----:|-------------|
| `201` | Ritual run created |

---

## Get ritual run

`GET /api/v1/ritual-runs/{id}`

**Requires authentication.**

### Parameters

| Name | In | Type | Required | Description |
|------|-----|------|:--------:|-------------|
| `` |  | string |  |  |

### Responses

| Code | Description |
|:----:|-------------|
| `200` | Ritual run details |

---

## Update ritual run

`PATCH /api/v1/ritual-runs/{id}`

**Requires authentication.**

### Parameters

| Name | In | Type | Required | Description |
|------|-----|------|:--------:|-------------|
| `` |  | string |  |  |

### Request Body

| Field | Type | Required | Description |
|-------|------|:--------:|-------------|
| `effects` | object |  |  |
| `error` | object |  |  |
| `metadata` | object |  |  |
| `result_summary` | string |  |  |
| `status` | string |  |  |

### Responses

| Code | Description |
|:----:|-------------|
| `200` | Ritual run updated |

---

