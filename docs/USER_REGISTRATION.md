# User Registration

Status: Implemented

## Overview

Registration uses a two-phase flow separated by email verification. The first phase collects account essentials and validates email uniqueness immediately. The second phase (optional, controlled by `require-kyc`) collects address and consent data.

This design ensures users discover problems like "email already taken" before investing in a long form, and that only verified users with completed profiles consume capacity slots.

## Flow

### SaaS Mode (require-kyc: true)

```
/register            /verify              /complete-profile     /dashboard
+---------------+    +---------------+    +------------------+  +----------+
| Type + Name   | -> | Enter code    | -> | Address (KYC)    |->| Ready    |
| Email + Pwd   |    | (verify email)|    | Consent + Legal  |  |          |
| [Company]     |    |               |    | [DPA if business]|  |          |
+---------------+    +---------------+    +------------------+  +----------+
                      Creates user         Records identity
                      Auto-login           Creates auto-org
                                           Capacity counted
```

### Community / Intranet Mode (require-kyc: false)

```
/register            /verify              /dashboard
+---------------+    +---------------+    +----------+
| Type + Name   | -> | Enter code    | -> | Ready    |
| Email + Pwd   |    | (verify email)|    |          |
| [Company]     |    |               |    |          |
+---------------+    +---------------+    +----------+
                      Creates user
                      Creates auto-org
                      Auto-login
```

No address collection, no consent gate, no complete-profile step. Users go straight to the dashboard after email verification.

## Configuration

### config.yaml

```yaml
registration:
  # require-kyc controls whether users must provide their address and accept
  # legal terms after email verification ("Complete Your Profile" step).
  # When true (default, SaaS): users complete a KYC address form + consent
  # before accessing the dashboard. Their auto-org is created at that point.
  # When false (Community/Intranet): users go straight to the dashboard after
  # email verification. No address collection, no consent gate. The auto-org
  # is created immediately at verification time.
  require-kyc: true
```

The setting is stored in the policy table (`registration.require_kyc`) at startup and read by the API at runtime. No service restart needed to check the current value -- but changing the config requires a restart to update the policy.

## Phase 1: /register

Single-page form collecting:
- Account type (private/business)
- First name, last name
- Email, password, confirm password
- Company name (if business)
- Language

Required fields are marked with red `*` indicators. Password mismatch shows an inline error alert.

On POST: the API validates email uniqueness, creates a `pending_user` record, and sends the verification email. The portal redirects to `/verify?email=...`.

Address, consent checkboxes, company registration, VAT number, and the KYC notice are NOT collected here.

## Verification: /verify

User enters the verification code (format: `xxx-xxx-xxx`, 15-minute expiry). On success:
- The API creates the active user (without identity/address data)
- Account type and company name are stored in user metadata
- The portal auto-logs the user in (sets session cookie)
- When `require-kyc: true`: redirects to `/complete-profile`
- When `require-kyc: false`: auto-org is created immediately, redirects to `/dashboard`

## Phase 2: /complete-profile (KYC mode only)

Authenticated page collecting:
- Street, street 2, postal code, city, state, country (KYC address)
- Company registration, VAT number (business accounts)
- Consent checkboxes: terms, privacy, age declaration, DPA (business only)
- KYC transparency notice

On POST: the API records the registration identity (person + address + consent records), creates the auto-organization, and the user is redirected to the dashboard.

### Profile Completion Gate

The `requireAuth` middleware checks `profile_complete` from the Whoami response:
- `true` for master admins, when `require-kyc` is false, or when a person record exists
- `false` for users who verified but haven't completed the profile
- Redirects to `/complete-profile` (excluded from the redirect, along with `/accept-terms` and `/logout`)

### Consent Gate

After profile completion, the existing consent version gate may redirect to `/accept-terms` if legal document versions have been updated since the user last accepted them. When `require-kyc` is false, the consent gate is also skipped (no consent records are created).

## DPA Handling

The Data Processing Agreement (DPA) applies only to business accounts. The `GetPendingConsents` function skips the DPA check for users who have never accepted any DPA version, preventing private account users from being asked to accept the DPA.

## Key Files

| File | Role |
|------|------|
| `internal/portal/server.go` | handleRegister, handleVerify (auto-login), handleCompleteProfile, requireAuth gate |
| `internal/portal/templates/register.html` | Single-page registration form |
| `internal/portal/templates/complete_profile.html` | Address + consent form (KYC step) |
| `internal/portal/restclient.go` | CompleteProfile REST client method |
| `internal/api/auth.go` | handleCompleteProfile endpoint, Whoami (profile_complete, account_type), consent gate skip |
| `internal/api/router.go` | POST /api/v1/auth/complete-profile route |
| `internal/storage/invitation.go` | VerifyAndCreateUser: defers identity to complete-profile, auto-org when KYC disabled |
| `internal/storage/consent.go` | GetPendingConsents: DPA skip for private accounts |
| `cmd/taskschmiede/main.go` | RegistrationConfig, require-kyc policy storage |
| `config.yaml.example` | registration.require-kyc documentation |

## Backward Compatibility

- Token-based registration (MCP agents, invitation links) is unchanged -- those paths collect all data at once and skip email verification.
- Existing active users with completed profiles are unaffected.
- Users who verified but haven't completed their profile are redirected to `/complete-profile` on next login (KYC mode only).
- Missing `registration` config section defaults to `require-kyc: true` (SaaS behavior).
