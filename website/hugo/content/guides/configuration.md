---
title: "Configuration"
description: "All configuration keys, defaults, and environment variables"
weight: 15
type: docs
---

Taskschmiede is configured via a YAML file (`config.yaml`) with environment variable expansion. Secrets are stored in a `.env` file and referenced using `${VAR}` syntax.

## Quick Start

For a minimal local setup without email:

```yaml
server:
  mcp-port: 9000

database:
  path: ./taskschmiede.db
```

Email verification codes will be printed to the server log instead of sent via email.

## Full Configuration (with Email)

Create both files:

**config.yaml:**

```yaml
server:
  mcp-port: 9000

database:
  path: ./taskschmiede.db

email:
  smtp-host: ${OUTGOING_MAIL_SERVER}
  smtp-port: 465
  smtp-use-ssl: true
  imap-host: ${INCOMING_MAIL_SERVER}
  imap-port: 993
  imap-use-ssl: true
  support:
    name: ${EMAIL_SUPPORT_NAME}
    address: ${EMAIL_SUPPORT_ADDRESS}
    username: ${EMAIL_SUPPORT_USER}
    password: ${EMAIL_SUPPORT_PASSWORD}
  verification-timeout: 15m
```

**.env:**

```bash
EMAIL_SUPPORT_NAME=Taskschmiede
EMAIL_SUPPORT_ADDRESS=support@example.com
EMAIL_SUPPORT_USER=support@example.com
EMAIL_SUPPORT_PASSWORD=your-password
OUTGOING_MAIL_SERVER=mail.example.com
INCOMING_MAIL_SERVER=mail.example.com
```

The `.env` file should be excluded from version control (it is gitignored by default). See `config.yaml.example` for a complete template.

## Environment Variable Expansion

Any value in `config.yaml` can reference environment variables using `${VAR}` syntax. Variables are expanded at startup from the process environment, which typically comes from a `.env` file loaded via systemd's `EnvironmentFile` directive.

## Configuration Sections

### database

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `path` | String | `./taskschmiede.db` | Path to the SQLite database file |

### server

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `mcp-port` | Integer | `9000` | Listen port for MCP and REST API (`/mcp` and `/api/v1/*`) |
| `session-timeout` | Duration | `2h` | MCP session inactivity timeout (sliding window; each tool call resets the timer) |
| `agent-token-ttl` | Duration | `30m` | Maximum lifetime for agent invitation tokens |

The Portal binary (`taskschmiede-portal`) listens on port `9090` by default.

### proxy

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `listen` | String | `:9001` | Proxy listen address |
| `upstream` | String | `http://localhost:9000` | Upstream MCP server URL |
| `log-traffic` | Boolean | `true` | Enable MCP traffic logging |
| `traffic-log-file` | String | `./taskschmiede-mcp-traffic.log` | Path for the MCP traffic log (JSON lines) |

### maintenance

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `enabled` | Boolean | `false` | Enable maintenance mode |
| `management-listen` | String | `127.0.0.1:9010` | Management API listen address (localhost only) |
| `management-api-key` | String | | API key for management endpoints |
| `auto-detect` | Boolean | `true` | Auto-detect upstream failures |
| `auto-detect-grace` | Duration | `10s` | Grace period before entering error state |
| `health-check-interval` | Duration | `5s` | Upstream health poll frequency |
| `upstream-timeout` | Duration | `30s` | Timeout for non-SSE upstream requests |
| `upstream-timeout-sse` | Duration | `300s` | Timeout for SSE streaming requests |

#### maintenance.notifications

State change notifications sent when the proxy enters or leaves maintenance mode.

##### maintenance.notifications.webhook

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `url` | String | | Webhook URL (e.g., Slack incoming webhook) |
| `headers` | Map | | Optional custom headers (e.g., `Authorization: "Bearer xxx"`) |

##### maintenance.notifications.smtp

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `host` | String | | SMTP server hostname |
| `port` | Integer | | SMTP server port |
| `use-ssl` | Boolean | | Use implicit TLS |
| `username` | String | | SMTP authentication username |
| `password` | String | | SMTP authentication password |
| `from` | String | | Sender email address |
| `from-name` | String | | Sender display name |
| `to` | List | | Recipient email addresses |

### mcp-security

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `enabled` | Boolean | `false` | Enable MCP-level security |
| `validation` | Boolean | `true` | Validate JSON-RPC structure and methods |
| `tool-rate-limits` | Map | | Per-tool rate limits (tool name or glob pattern) |
| `api-versions.current` | String | `v1` | Current API version |
| `api-versions.supported` | List | `["v1"]` | Supported API versions |
| `api-versions.deprecated` | List | `[]` | Deprecated API versions |

### log

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `file` | String | `./taskschmiede.log` | Log file path (`-` or empty for stdout) |
| `level` | String | `INFO` | Log level: `DEBUG`, `INFO`, `WARN`, `ERROR` |

### ticker

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `interval` | Duration | `1s` | How often to check if handlers are due |
| `kpi.enabled` | Boolean | `true` | Enable KPI snapshot collection |
| `kpi.interval` | Duration | `1m` | Snapshot collection frequency |
| `kpi.output-dir` | String | `<db-dir>/kpi/` | Directory for KPI JSON output |

Always-on ticker handlers (no config toggle):

- **db-backup** -- daily `VACUUM INTO` for main and message databases. Backups stored in `<db-dir>/db-backups/`, keeps 7 per database.
- **data-purge** -- daily deletion of old `audit_log` and `entity_change` records. Retention configured via policy table keys `purge.audit_log_days` (default: 90) and `purge.entity_change_days` (default: 180).

### email

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `smtp-host` | String | | SMTP server hostname |
| `smtp-port` | Integer | | SMTP server port |
| `smtp-use-tls` | Boolean | | Use STARTTLS |
| `smtp-use-ssl` | Boolean | | Use implicit TLS (SSL) |
| `imap-host` | String | | IMAP server hostname |
| `imap-port` | Integer | | IMAP server port |
| `imap-use-tls` | Boolean | | Use STARTTLS for IMAP |
| `imap-use-ssl` | Boolean | | Use implicit TLS for IMAP |
| `verification-timeout` | Duration | `15m` | Verification and reset code timeout |
| `portal-url` | String | | Portal URL for links in emails |

#### email.support

Transactional email account (verification codes, password resets, waitlist notifications, inactivity warnings). Requires SMTP only.

| Key | Type | Description |
|-----|------|-------------|
| `name` | String | Sender display name |
| `address` | String | Sender email address |
| `username` | String | SMTP authentication username |
| `password` | String | SMTP authentication password |

#### email.intercom

Email bridge account for user messaging. Requires both SMTP and IMAP.

| Key | Type | Description |
|-----|------|-------------|
| `name` | String | Sender display name |
| `address` | String | Sender email address |
| `username` | String | Authentication username |
| `password` | String | Authentication password |

### messaging

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `database-path` | String | `<name>_messages.db` | Path for the message database |
| `intercom.enabled` | Boolean | `false` | Enable email bridge |
| `intercom.reply-ttl` | Duration | `720h` | Max reply window (30 days) |
| `intercom.sweep-interval` | Duration | `1m` | IMAP inbox check frequency |
| `intercom.send-interval` | Duration | `30s` | Outbound email send frequency |
| `intercom.max-retries` | Integer | `3` | Email delivery retries |
| `intercom.max-inbound-per-hour` | Integer | `20` | Anti-bombing limit per sender |
| `intercom.dedup-window` | Duration | `1h` | Duplicate rejection window |

### injection-review

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `enabled` | Boolean | `false` | Enable post-hoc injection detection |
| `provider` | String | `openai` | LLM provider: `anthropic` or `openai` |
| `model` | String | | Model ID |
| `api-key` | String | | API key for the LLM provider |
| `api-url` | String | | Custom API base URL (for local models) |
| `max-retries` | Integer | `3` | Max retry attempts on failure |
| `ticker-interval` | Duration | `2m` | Check frequency for pending reviews |
| `timeout` | Duration | `60s` | HTTP timeout for LLM calls |

### content-guard

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `enabled` | Boolean | `false` | Enable LLM-assisted content scoring |
| `provider` | String | `openai` | LLM provider |
| `model` | String | | Model ID |
| `api-url` | String | | API endpoint URL |
| `api-key` | String | | API key |
| `ticker-interval` | Duration | `1m` | Check frequency for pending items |
| `timeout` | Duration | `30s` | HTTP timeout |
| `max-retries` | Integer | `3` | Max retry attempts |
| `score-threshold` | Integer | `20` | Minimum heuristic score to trigger LLM review |

### instance

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `max-active-users` | Integer | `200` | Max concurrent active users (triggers waitlist) |

### security

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `deployment-mode` | String | `open` | Deployment mode: `open` or `trusted`. See [Open vs Trusted]({{< relref "deployment-modes" >}}). |
| `allow-self-registration` | Boolean | `true` | Whether self-registration at `/register` is available. When `false`, all accounts must be created by admins or via invitation tokens. |

#### security.agent-onboarding

Controls verification gates for agent registration. In `open` mode, both gates are always enforced regardless of these settings. In `trusted` mode, they are configurable.

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `require-email-verification` | Boolean | `true` | Require agents to verify their email address during registration |
| `require-interview` | Boolean | `true` | Require agents to pass the onboarding interview before activation |

#### security.rate-limit

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `global-per-ip.requests` | Integer | `120` | Requests per IP per window |
| `global-per-ip.window` | Duration | `1m` | Window duration |
| `global-per-ip.enabled` | Boolean | `true` | Enable global rate limit |
| `per-session.requests` | Integer | `60` | Requests per session per window |
| `per-session.window` | Duration | `1m` | Window duration |
| `per-session.enabled` | Boolean | `true` | Enable per-session rate limit |
| `auth-endpoint.requests` | Integer | `5` | Auth requests per window |
| `auth-endpoint.window` | Duration | `1m` | Window duration |
| `auth-endpoint.enabled` | Boolean | `true` | Enable auth rate limit |
| `cleanup-interval` | Duration | `5m` | Expired entry cleanup interval |

#### security.conn-limit

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `max-global` | Integer | `1000` | Max concurrent connections (0 = unlimited) |
| `max-per-ip` | Integer | `50` | Max concurrent connections per IP (0 = unlimited) |

#### security.headers

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `hsts-enabled` | Boolean | `false` | Enable HSTS header |
| `hsts-max-age` | Integer | `31536000` | HSTS max-age in seconds |
| `csp-policy` | String | `default-src 'self'; ...` | Content Security Policy |
| `frame-options` | String | `DENY` | X-Frame-Options value |
| `referrer-policy` | String | `strict-origin-when-cross-origin` | Referrer-Policy value |

#### security.body-limit

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `max-body-size` | Integer | `1048576` | Maximum request body size in bytes (1 MB) |

#### security.audit

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `buffer-size` | Integer | `1024` | Audit log async buffer size |

## Port Summary

| Service | Default Port | Description |
|---------|-------------|-------------|
| MCP + REST API | 9000 | Main server (`/mcp`, `/api/v1/*`) |
| Portal | 9090 | Web UI for users and admins |
| Proxy | 9001 | MCP development proxy |
| Notification Service | 9004 | Standalone notification delivery |
