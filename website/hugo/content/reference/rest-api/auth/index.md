---
title: "Auth"
description: "Authentication and user profile"
weight: 3
type: docs
---

Authentication and user profile

## Endpoints

| Method | Path | Summary |
|--------|------|---------- |
| `POST` | `/api/v1/auth/change-password` | [Change password](#change-password) |
| `POST` | `/api/v1/auth/forgot-password` | [Request password reset](#request-password-reset) |
| `POST` | `/api/v1/auth/login` | [Login](#login) |
| `POST` | `/api/v1/auth/logout` | [Logout (invalidate token)](#logout-invalidate-token) |
| `PATCH` | `/api/v1/auth/profile` | [Update profile](#update-profile) |
| `POST` | `/api/v1/auth/register` | [Register new user](#register-new-user) |
| `POST` | `/api/v1/auth/resend-verification` | [Resend verification code](#resend-verification-code) |
| `POST` | `/api/v1/auth/reset-password` | [Reset password with code](#reset-password-with-code) |
| `POST` | `/api/v1/auth/verify` | [Verify email with code](#verify-email-with-code) |
| `GET` | `/api/v1/auth/whoami` | [Get current user profile with tier info](#get-current-user-profile-with-tier-info) |

---

## Change password

`POST /api/v1/auth/change-password`

**Requires authentication.**

### Request Body

| Field | Type | Required | Description |
|-------|------|:--------:|-------------|
| `current_password` | string | Yes |  |
| `new_password` | string | Yes |  |

### Responses

| Code | Description |
|:----:|-------------|
| `200` | Password changed |
| `400` |  |

---

## Request password reset

`POST /api/v1/auth/forgot-password`

**Public** -- no authentication required.

### Request Body

| Field | Type | Required | Description |
|-------|------|:--------:|-------------|
| `email` | string <email> | Yes |  |

### Responses

| Code | Description |
|:----:|-------------|
| `200` | Reset code sent (same response whether email exists or not) |

---

## Login

`POST /api/v1/auth/login`

**Public** -- no authentication required.

### Request Body

| Field | Type | Required | Description |
|-------|------|:--------:|-------------|
| `email` | string <email> | Yes |  |
| `password` | string | Yes |  |

### Responses

| Code | Description |
|:----:|-------------|
| `200` | Login successful |
| `401` |  |

---

## Logout (invalidate token)

`POST /api/v1/auth/logout`

**Requires authentication.**

### Responses

| Code | Description |
|:----:|-------------|
| `200` | Logged out |

---

## Update profile

`PATCH /api/v1/auth/profile`

**Requires authentication.**

### Request Body

| Field | Type | Required | Description |
|-------|------|:--------:|-------------|
| `email_copy` | boolean |  |  |
| `lang` | string |  |  |
| `name` | string |  |  |
| `timezone` | string |  |  |

### Responses

| Code | Description |
|:----:|-------------|
| `200` | Profile updated |
| `401` |  |

---

## Register new user

`POST /api/v1/auth/register`

**Public** -- no authentication required.

### Request Body

| Field | Type | Required | Description |
|-------|------|:--------:|-------------|
| `email` | string <email> | Yes |  |
| `invitation_token` | string |  |  |
| `name` | string | Yes |  |
| `password` | string | Yes |  |

### Responses

| Code | Description |
|:----:|-------------|
| `201` | Registration initiated (verification email sent) |
| `400` |  |
| `409` | Email already registered |

---

## Resend verification code

`POST /api/v1/auth/resend-verification`

**Public** -- no authentication required.

### Request Body

| Field | Type | Required | Description |
|-------|------|:--------:|-------------|
| `email` | string <email> | Yes |  |

### Responses

| Code | Description |
|:----:|-------------|
| `200` | Verification code resent |

---

## Reset password with code

`POST /api/v1/auth/reset-password`

**Public** -- no authentication required.

### Request Body

| Field | Type | Required | Description |
|-------|------|:--------:|-------------|
| `code` | string | Yes |  |
| `email` | string <email> | Yes |  |
| `new_password` | string | Yes |  |

### Responses

| Code | Description |
|:----:|-------------|
| `200` | Password reset successful |
| `400` |  |

---

## Verify email with code

`POST /api/v1/auth/verify`

**Public** -- no authentication required.

### Request Body

| Field | Type | Required | Description |
|-------|------|:--------:|-------------|
| `code` | string | Yes |  |
| `email` | string <email> | Yes |  |

### Responses

| Code | Description |
|:----:|-------------|
| `200` | Email verified, account activated |
| `400` |  |

---

## Get current user profile with tier info

`GET /api/v1/auth/whoami`

**Requires authentication.**

### Responses

| Code | Description |
|:----:|-------------|
| `200` | User profile |
| `401` |  |

---

