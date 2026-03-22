---
title: "Rotate API Keys"
description: "Regenerate tokens and API keys without downtime"
weight: 130
type: docs
---

This guide covers the process of rotating API tokens used for REST API access in Taskschmiede.

## Overview

API tokens authenticate programmatic access to the Taskschmiede REST API. Tokens have a configurable lifetime and should be rotated regularly as a security practice. The rotation process creates a new token before retiring the old one, ensuring continuous access.

## Step 1: Create a New Token

Generate a new API token:

```json
{
  "tool": "ts.tkn.create",
  "arguments": {}
}
```

Response:

```json
{
  "token": "ts_new_token_value...",
  "expires_at": "2026-03-09T02:00:00Z"
}
```

Store the token value securely. It cannot be retrieved again after creation.

Token lifetime is controlled by the `token.default_ttl` policy key (default: 8 hours).

## Step 2: Verify the Old Token

Confirm the existing token is still valid while transitioning:

```json
{
  "tool": "ts.tkn.verify",
  "arguments": {
    "token": "ts_old_token_value..."
  }
}
```

This returns the token's status and expiration time. If the old token is still active, both tokens work simultaneously during the transition window.

## Step 3: Update Clients

Update all clients and automation scripts to use the new token. Common locations to update:

- MCP client configurations
- CI/CD pipeline environment variables
- Monitoring and alerting integrations
- Scheduled scripts or cron jobs

Use the new token in the `Authorization` header:

```bash
curl -H "Authorization: Bearer ts_new_token_value..." \
  https://api.example.com/api/v1/auth/whoami
```

## Step 4: Old Token Expiration

Old tokens expire based on the configured `token.default_ttl` duration. There is no need to explicitly revoke them unless you want to force immediate invalidation.

To revoke sessions immediately (for example, after a security incident), the master admin can invalidate all sessions for a user:

```bash
curl -X PATCH /api/v1/users/{id} \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d '{"status": "active"}'
```

A password change also invalidates all existing sessions and tokens for that user.

## Best Practices

- Rotate tokens on a regular schedule, not just when they expire.
- Never commit tokens to version control. Use environment variables or secrets management.
- Monitor the audit log for `token_created` and `token_revoked` events to track token lifecycle.
- Use separate tokens for different clients or environments so they can be rotated independently.
