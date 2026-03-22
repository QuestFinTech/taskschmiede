---
title: "Authentication"
description: "Configure login methods, sessions, and password policies"
weight: 60
type: docs
---

This guide covers setting up authentication in Taskschmiede, including the master admin account, email verification, sessions, API tokens, and agent invitations.

## Master Admin Setup

On first launch, Taskschmiede requires a master admin account. The setup flow runs through the Portal web UI.

1. Start the server:
   ```bash
   taskschmiede serve --config-file config.yaml
   ```

2. Open the Portal at `http://localhost:9090/setup`.

3. Enter your email address, display name, and password.

4. The system sends a verification email via the Support account. The code format is `xxx-xxx-xxx` (lowercase alphanumeric, hyphen-separated).

5. Enter the verification code on the `/verify` page, or click the link in the email.

6. On successful verification, the account is activated and you are redirected to `/login`.

After verification, the system sets the `setup.complete` policy to `"true"` and the setup endpoint becomes inactive. Subsequent visits to `/setup` are redirected.

Email configuration (SMTP via the Support account) must be in place before running the setup wizard. Without it, the verification email cannot be sent.

## Password Requirements

All passwords must meet these requirements:

- Minimum 12 characters
- At least one uppercase letter
- At least one lowercase letter
- At least one digit
- At least one special character (punctuation or symbol)

These requirements apply to the master admin setup, user registration, and password resets.

## Email Verification

Email verification is used during registration and password reset. Configure the verification timeout in `config.yaml`:

```yaml
email:
  verification-timeout: 15m
```

The default is 15 minutes. If the code expires, the user can request a new one from the verification page or via `POST /api/v1/auth/resend-verification`.

Verification codes are single-use. Expired codes are cleaned up automatically by the hourly session cleanup ticker.

## Session Duration

Web UI sessions are stored in the database with a configurable duration:

```yaml
server:
  session-timeout: 2h
```

The default session timeout is 2 hours. MCP sessions use a sliding window -- each tool call resets the timer.

Expired sessions are cleaned up automatically by the hourly ticker.

When a user changes their password (including via password reset), all existing sessions for that user are invalidated.

## API Token Creation

API tokens provide programmatic access to the REST API. Tokens are created via MCP or the REST API:

**MCP:**
```json
{
  "tool": "ts.tkn.create",
  "arguments": {}
}
```

**REST API:**
```bash
curl -X POST /api/v1/tokens \
  -H "Authorization: Bearer $SESSION_TOKEN" \
  -H "Content-Type: application/json"
```

The response includes the token value. Store it securely -- it cannot be retrieved again after creation.

Token lifetime is controlled by the `token.default_ttl` policy (default: 8 hours). To verify a token is still valid:

```json
{
  "tool": "ts.tkn.verify",
  "arguments": {
    "token": "the-token-value"
  }
}
```

## Agent Invitation Tokens

Agents register through an invitation-based flow. An admin or sponsor creates an invitation token, and the agent uses it to self-register.

**Create an invitation:**

```json
{
  "tool": "ts.inv.create",
  "arguments": {}
}
```

**List active invitations:**

```json
{
  "tool": "ts.inv.list",
  "arguments": {}
}
```

**Revoke an invitation:**

```json
{
  "tool": "ts.inv.revoke",
  "arguments": {
    "id": "inv_xxx"
  }
}
```

Invitation tokens have a maximum lifetime controlled by `server.agent-token-ttl` (default: 30 minutes). Tokens that are not used within this window expire automatically.

For the full agent onboarding flow, see [Onboard an AI Agent](/guides/onboard-agent/).

## Password Reset

Users who forget their password can reset it through the Portal:

1. Click "Forgot your password?" on `/login`.
2. Enter the registered email on `/forgot-password`.
3. The system sends a reset code (same `xxx-xxx-xxx` format) if the email exists. The response does not reveal whether the email is registered (prevents enumeration).
4. Enter the reset code and a new password on `/reset-password`.
5. On success, all existing sessions are invalidated and the user is redirected to `/login`.

The reset code uses the same timeout as verification codes (`email.verification-timeout`).
