---
title: "DoD"
description: "Definition of Done policies and endorsements"
weight: 6
type: docs
---

Definition of Done policies and endorsements

## Endpoints

| Method | Path | Summary |
|--------|------|---------- |
| `GET` | `/api/v1/dod-endorsements` | [List endorsements](#list-endorsements) |
| `POST` | `/api/v1/dod-endorsements` | [Create endorsement](#create-endorsement) |
| `GET` | `/api/v1/dod-policies` | [List DoD policies](#list-dod-policies) |
| `POST` | `/api/v1/dod-policies` | [Create DoD policy](#create-dod-policy) |
| `GET` | `/api/v1/dod-policies/{id}` | [Get DoD policy](#get-dod-policy) |
| `PATCH` | `/api/v1/dod-policies/{id}` | [Update DoD policy](#update-dod-policy) |
| `GET` | `/api/v1/dod-policies/{id}/lineage` | [Get policy version lineage](#get-policy-version-lineage) |
| `POST` | `/api/v1/dod-policies/{id}/versions` | [Create new policy version](#create-new-policy-version) |
| `POST` | `/api/v1/endeavours/{id}/dod-policy` | [Assign DoD policy to endeavour](#assign-dod-policy-to-endeavour) |
| `DELETE` | `/api/v1/endeavours/{id}/dod-policy` | [Unassign DoD policy from endeavour](#unassign-dod-policy-from-endeavour) |
| `GET` | `/api/v1/endeavours/{id}/dod-status` | [Get endeavour DoD status](#get-endeavour-dod-status) |
| `POST` | `/api/v1/tasks/{id}/dod-check` | [Check task DoD compliance](#check-task-dod-compliance) |
| `POST` | `/api/v1/tasks/{id}/dod-override` | [Override DoD for task](#override-dod-for-task) |

---

## List endorsements

`GET /api/v1/dod-endorsements`

**Requires authentication.**

### Parameters

| Name | In | Type | Required | Description |
|------|-----|------|:--------:|-------------|
| `` |  | string |  |  |
| `` |  | string |  |  |
| `endeavour_id` | query | string |  |  |
| `policy_id` | query | string |  |  |

### Responses

| Code | Description |
|:----:|-------------|
| `200` | Endorsement list |

---

## Create endorsement

`POST /api/v1/dod-endorsements`

**Requires authentication.**

### Request Body

| Field | Type | Required | Description |
|-------|------|:--------:|-------------|
| `endeavour_id` | string | Yes |  |
| `policy_id` | string | Yes |  |

### Responses

| Code | Description |
|:----:|-------------|
| `201` | Endorsement created |

---

## List DoD policies

`GET /api/v1/dod-policies`

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
| `200` | Policy list |

---

## Create DoD policy

`POST /api/v1/dod-policies`

**Requires authentication.**

### Request Body

| Field | Type | Required | Description |
|-------|------|:--------:|-------------|
| `conditions` | array |  |  |
| `description` | string |  |  |
| `metadata` | object |  |  |
| `name` | string | Yes |  |
| `quorum` | integer |  |  |
| `scope` | string |  |  |
| `strictness` | string |  |  |

### Responses

| Code | Description |
|:----:|-------------|
| `201` | Policy created |

---

## Get DoD policy

`GET /api/v1/dod-policies/{id}`

**Requires authentication.**

### Parameters

| Name | In | Type | Required | Description |
|------|-----|------|:--------:|-------------|
| `` |  | string |  |  |

### Responses

| Code | Description |
|:----:|-------------|
| `200` | Policy details |

---

## Update DoD policy

`PATCH /api/v1/dod-policies/{id}`

**Requires authentication.**

### Parameters

| Name | In | Type | Required | Description |
|------|-----|------|:--------:|-------------|
| `` |  | string |  |  |

### Request Body

| Field | Type | Required | Description |
|-------|------|:--------:|-------------|
| `conditions` | array |  |  |
| `description` | string |  |  |
| `metadata` | object |  |  |
| `name` | string |  |  |
| `quorum` | integer |  |  |
| `status` | string |  |  |
| `strictness` | string |  |  |

### Responses

| Code | Description |
|:----:|-------------|
| `200` | Policy updated |

---

## Get policy version lineage

`GET /api/v1/dod-policies/{id}/lineage`

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

## Create new policy version

`POST /api/v1/dod-policies/{id}/versions`

**Requires authentication.**

### Parameters

| Name | In | Type | Required | Description |
|------|-----|------|:--------:|-------------|
| `` |  | string |  |  |

### Responses

| Code | Description |
|:----:|-------------|
| `201` | New version created |

---

## Assign DoD policy to endeavour

`POST /api/v1/endeavours/{id}/dod-policy`

**Requires authentication.**

### Parameters

| Name | In | Type | Required | Description |
|------|-----|------|:--------:|-------------|
| `` |  | string |  |  |

### Request Body

| Field | Type | Required | Description |
|-------|------|:--------:|-------------|
| `policy_id` | string | Yes |  |

### Responses

| Code | Description |
|:----:|-------------|
| `200` | Policy assigned |

---

## Unassign DoD policy from endeavour

`DELETE /api/v1/endeavours/{id}/dod-policy`

**Requires authentication.**

### Parameters

| Name | In | Type | Required | Description |
|------|-----|------|:--------:|-------------|
| `` |  | string |  |  |

### Responses

| Code | Description |
|:----:|-------------|
| `200` | Policy unassigned |

---

## Get endeavour DoD status

`GET /api/v1/endeavours/{id}/dod-status`

**Requires authentication.**

### Parameters

| Name | In | Type | Required | Description |
|------|-----|------|:--------:|-------------|
| `` |  | string |  |  |

### Responses

| Code | Description |
|:----:|-------------|
| `200` | DoD compliance status |

---

## Check task DoD compliance

`POST /api/v1/tasks/{id}/dod-check`

**Requires authentication.**

### Parameters

| Name | In | Type | Required | Description |
|------|-----|------|:--------:|-------------|
| `` |  | string |  |  |

### Responses

| Code | Description |
|:----:|-------------|
| `200` | DoD check result |

---

## Override DoD for task

`POST /api/v1/tasks/{id}/dod-override`

**Requires authentication.**

### Parameters

| Name | In | Type | Required | Description |
|------|-----|------|:--------:|-------------|
| `` |  | string |  |  |

### Request Body

| Field | Type | Required | Description |
|-------|------|:--------:|-------------|
| `reason` | string | Yes |  |

### Responses

| Code | Description |
|:----:|-------------|
| `200` | DoD overridden |

---

