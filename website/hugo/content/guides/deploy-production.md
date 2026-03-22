---
title: "Deployment"
description: "Run Taskschmiede on a server with TLS, backups, and monitoring"
weight: 40
type: docs
---

This guide covers building Taskschmiede from source, configuring a production instance, and setting up the infrastructure around it.

## Build from Source

Taskschmiede ships as statically linked Go binaries. Build for all supported platforms:

```bash
make build-all
```

This produces binaries for `darwin-arm64`, `linux-amd64`, and `windows-amd64` under `build/`. Individual binaries can be built separately:

```bash
make build          # Current platform only
make build-portal   # Portal binary only
make build-proxy    # Proxy binary only
make build-notify   # Notification service binary only
```

For a deployable package targeting Linux:

```bash
make package
```

This creates a tar.gz archive in `build/release/` containing all binaries needed for a production deployment.

For automated deployment to staging or production servers:

```bash
make deploy-staging       # Package and deploy to staging server
make deploy-production    # Package and deploy to production server
```

## Configuration

Taskschmiede uses two configuration files in the working directory.

### Environment Variables (.env)

Store secrets and environment-specific values in `.env`. This file is loaded by systemd's `EnvironmentFile` directive and must not be committed to version control.

```bash
# Mail server
OUTGOING_MAIL_SERVER=mail.example.com
INCOMING_MAIL_SERVER=mail.example.com

# Support account (transactional: verification, password reset)
EMAIL_SUPPORT_NAME=Taskschmiede
EMAIL_SUPPORT_ADDRESS=support@example.com
EMAIL_SUPPORT_USER=support@example.com
EMAIL_SUPPORT_PASSWORD=secret-support-password

# Intercom account (email bridge for user messaging)
EMAIL_INTERCOM_NAME=Taskschmiede Intercom
EMAIL_INTERCOM_ADDRESS=intercom@example.com
EMAIL_INTERCOM_USER=intercom@example.com
EMAIL_INTERCOM_PASSWORD=secret-intercom-password

# Notification service shared secret
NOTIFY_AUTH_TOKEN=your-notify-token
```

### Primary Configuration (config.yaml)

The main configuration file references `.env` values using `${VAR}` syntax. Environment variables are expanded at startup.

```yaml
database:
  path: /var/lib/taskschmiede/taskschmiede.db

server:
  mcp-port: 9000
  session-timeout: 2h
  agent-token-ttl: 30m

log:
  file: /var/log/taskschmiede/taskschmiede.log
  level: INFO

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
  intercom:
    name: ${EMAIL_INTERCOM_NAME}
    address: ${EMAIL_INTERCOM_ADDRESS}
    username: ${EMAIL_INTERCOM_USER}
    password: ${EMAIL_INTERCOM_PASSWORD}
  verification-timeout: 15m
  portal-url: https://portal.example.com

security:
  rate-limit:
    global-per-ip:
      requests: 120
      window: 1m
      enabled: true
    per-session:
      requests: 60
      window: 1m
      enabled: true
    auth-endpoint:
      requests: 5
      window: 1m
      enabled: true
    cleanup-interval: 5m
  conn-limit:
    max-global: 1000
    max-per-ip: 50
  headers:
    hsts-enabled: true
    hsts-max-age: 31536000
    csp-policy: "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'"
    frame-options: DENY
    referrer-policy: strict-origin-when-cross-origin
  body-limit:
    max-body-size: 1048576
  audit:
    buffer-size: 1024
```

See the [Configuration]({{< relref "configuration" >}}) guide for all available keys and defaults.

## Reverse Proxy (NGINX)

Place NGINX in front of Taskschmiede to handle TLS termination, static assets, and request routing.

### Example NGINX Configuration

```nginx
upstream taskschmiede_api {
    server 127.0.0.1:9000;
}

upstream taskschmiede_portal {
    server 127.0.0.1:9090;
}

server {
    listen 443 ssl http2;
    server_name api.example.com;

    ssl_certificate     /etc/ssl/certs/example.com.pem;
    ssl_certificate_key /etc/ssl/private/example.com.key;

    # MCP endpoint (Server-Sent Events -- requires extended timeouts)
    location /mcp {
        proxy_pass http://taskschmiede_api;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_read_timeout 300s;
        proxy_buffering off;
    }

    # REST API
    location /api/ {
        proxy_pass http://taskschmiede_api;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # Health check
    location /mcp/health {
        proxy_pass http://taskschmiede_api;
    }
}

server {
    listen 443 ssl http2;
    server_name portal.example.com;

    ssl_certificate     /etc/ssl/certs/example.com.pem;
    ssl_certificate_key /etc/ssl/private/example.com.key;

    location / {
        proxy_pass http://taskschmiede_portal;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

Key points:

- Disable proxy buffering for the `/mcp` endpoint. MCP uses Server-Sent Events, which requires unbuffered responses.
- Set an extended read timeout (300s or more) for `/mcp` to keep SSE connections alive.
- Forward `X-Real-IP` and `X-Forwarded-For` headers so Taskschmiede can apply per-IP rate limiting correctly.

## Systemd Service

Create a systemd unit file at `/etc/systemd/system/taskschmiede.service`:

```ini
[Unit]
Description=Taskschmiede MCP + REST API Server
After=network.target

[Service]
Type=simple
User=taskschmiede
WorkingDirectory=/opt/taskschmiede
EnvironmentFile=/opt/taskschmiede/.env
ExecStart=/opt/taskschmiede/taskschmiede serve --config-file /opt/taskschmiede/config.yaml
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
```

The `EnvironmentFile` directive loads `.env` so that `${VAR}` references in `config.yaml` resolve correctly.

Create a separate unit for the Portal:

```ini
[Unit]
Description=Taskschmiede Portal
After=network.target taskschmiede.service

[Service]
Type=simple
User=taskschmiede
WorkingDirectory=/opt/taskschmiede
EnvironmentFile=/opt/taskschmiede/.env
ExecStart=/opt/taskschmiede/taskschmiede-portal --config-file /opt/taskschmiede/config.yaml
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
```

Enable and start both services:

```bash
sudo systemctl daemon-reload
sudo systemctl enable taskschmiede taskschmiede-portal
sudo systemctl start taskschmiede taskschmiede-portal
```

## Database Backups

Taskschmiede runs an automatic backup handler via the internal ticker. The backup runs every 24 hours and creates full copies of both databases using SQLite's `VACUUM INTO` command.

**Backup location:** `<db-dir>/db-backups/`

**Backup format:** `YYYYMMDD-HHMMSS_taskschmiede.db` and `YYYYMMDD-HHMMSS_taskschmiede_messages.db`

**Retention:** The 7 most recent backups per database are kept. Older backups are deleted automatically.

`VACUUM INTO` creates a consistent, defragmented copy of the database. It is safe to use with WAL mode and does not interfere with concurrent reads or writes.

To verify a backup:

```bash
sqlite3 db-backups/20260301-000000_taskschmiede.db "PRAGMA integrity_check;"
```

For additional safety, consider copying the `db-backups/` directory to off-server storage on a regular schedule.

## Data Purge

The ticker also runs a daily purge handler that removes old records from two tables:

| Table | Policy Key | Default Retention |
|-------|-----------|-------------------|
| `audit_log` | `purge.audit_log_days` | 90 days |
| `entity_change` | `purge.entity_change_days` | 180 days |

Retention values are read from the policy table on each run. Changes take effect without restarting the server. Update them via the admin API:

```bash
curl -X PATCH /api/v1/admin/quotas \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"purge.audit_log_days": 180, "purge.entity_change_days": 365}'
```

## Maintenance Mode

For planned downtime, enable maintenance mode in `config.yaml`:

```yaml
maintenance:
  enabled: true
  management-listen: "127.0.0.1:9999"
```

When enabled, the server returns maintenance responses to all client requests while remaining manageable via the management port.

## Rollback

To rollback a deployment:

1. Stop the running server.
2. Replace binaries with the previous version (from `build/release/` or a GitHub release).
3. Restart the server.

Database migrations run automatically on startup and are forward-only. Restoring an older database backup is safe -- migrations will re-apply.
