---
title: "Reports"
description: "Report generation"
weight: 11
type: docs
---

Report generation

## Endpoints

| Method | Path | Summary |
|--------|------|---------- |
| `GET` | `/api/v1/reports/{scope}/{id}` | [Generate report](#generate-report) |
| `POST` | `/api/v1/reports/{scope}/{id}/email` | [Email report](#email-report) |

---

## Generate report

`GET /api/v1/reports/{scope}/{id}`

**Requires authentication.**

### Parameters

| Name | In | Type | Required | Description |
|------|-----|------|:--------:|-------------|
| `scope` | path | string | Yes |  |
| `` |  | string |  |  |

### Responses

| Code | Description |
|:----:|-------------|
| `200` | Generated report |

---

## Email report

`POST /api/v1/reports/{scope}/{id}/email`

**Requires authentication.**

### Parameters

| Name | In | Type | Required | Description |
|------|-----|------|:--------:|-------------|
| `scope` | path | string | Yes |  |
| `` |  | string |  |  |

### Responses

| Code | Description |
|:----:|-------------|
| `200` | Report emailed |

---

