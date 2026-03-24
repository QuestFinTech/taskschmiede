# Deploying Taskschmiede

This guide covers building, configuring, and running Taskschmiede Community Edition on a server.

---

## Prerequisites

- **Go 1.26+** -- [go.dev/dl](https://go.dev/dl/)
- **make** -- included on Linux/macOS; on Windows use Git Bash or `go build` directly
- **SMTP account** -- for email verification and password reset (any provider)
- **Linux recommended** for production (systemd units included). macOS and Windows work for development.

---

## 1. Build

```bash
git clone https://github.com/QuestFinTech/taskschmiede.git
cd taskschmiede
make build build-portal build-proxy
```

This produces three binaries in `build/`:

| Binary | Purpose | Default Port |
|--------|---------|:------------:|
| `taskschmiede` | Core server (MCP + REST API) | 9000 |
| `taskschmiede-portal` | Web UI for users and admins | 9090 |
| `taskschmiede-proxy` | MCP proxy (auto-reconnect, traffic logging) | 9001 |

To build a specific binary: `make build`, `make build-portal`, or `make build-proxy`.

---

## 2. Configure

### Directory layout

Create the deployment directory and copy binaries and config:

```bash
sudo mkdir -p /opt/taskschmiede/{bin,config,data}
sudo cp build/taskschmiede build/taskschmiede-portal build/taskschmiede-proxy /opt/taskschmiede/bin/
sudo cp config.yaml.example /opt/taskschmiede/config/config.yaml
```

### Environment file

Create `/opt/taskschmiede/config/.env` with your mail server credentials:

```bash
# Mail server
OUTGOING_MAIL_SERVER=mail.example.com
OUTGOING_MAIL_PORT=465
OUTGOING_MAIL_USE_SSL=true
OUTGOING_MAIL_USE_TLS=false
INCOMING_MAIL_SERVER=mail.example.com
INCOMING_MAIL_PORT=993
INCOMING_MAIL_USE_SSL=true
INCOMING_MAIL_USE_TLS=false

# Support account (transactional: verification codes, password reset)
EMAIL_SUPPORT_NAME=Taskschmiede
EMAIL_SUPPORT_ADDRESS=support@example.com
EMAIL_SUPPORT_USER=support@example.com
EMAIL_SUPPORT_PASSWORD=your-password

# Portal URL (used in email links)
PORTAL_URL=http://localhost:9090
```

Set ownership and permissions:

```bash
sudo chown taskschmiede:taskschmiede /opt/taskschmiede/config/.env
sudo chmod 600 /opt/taskschmiede/config/.env
```

**Important:** The `.env` file must be owned by the `taskschmiede` user, not root. Services run as `taskschmiede` and need to read this file.

### Configuration file

Edit `/opt/taskschmiede/config/config.yaml`. The key sections for a basic deployment:

```yaml
database:
  path: /opt/taskschmiede/data/taskschmiede.db

server:
  mcp-port: 9000

portal:
  listen: ":9090"
  api-url: "http://localhost:9000"

proxy:
  listen: ":9001"
  upstream: "http://localhost:9000"

log:
  file: /var/log/taskschmiede/taskschmiede.log
  level: INFO

email:
  smtp-host: ${OUTGOING_MAIL_SERVER}
  smtp-port: ${OUTGOING_MAIL_PORT}
  smtp-use-ssl: ${OUTGOING_MAIL_USE_SSL}
  smtp-use-tls: ${OUTGOING_MAIL_USE_TLS}
  imap-host: ${INCOMING_MAIL_SERVER}
  imap-port: ${INCOMING_MAIL_PORT}
  imap-use-ssl: ${INCOMING_MAIL_USE_SSL}
  imap-use-tls: ${INCOMING_MAIL_USE_TLS}
  support:
    name: ${EMAIL_SUPPORT_NAME}
    address: ${EMAIL_SUPPORT_ADDRESS}
    username: ${EMAIL_SUPPORT_USER}
    password: ${EMAIL_SUPPORT_PASSWORD}
  verification-timeout: 15m
  portal-url: "${PORTAL_URL}"

# Community Edition: single unlimited tier, no KYC requirement
tiers:
  default-tier: 1
  definitions:
    - id: 1
      name: community
      max-users: -1
      max-orgs: -1
      max-agents-per-org: -1
      max-endeavours-per-org: -1
      max-active-endeavours: -1
      max-teams-per-org: -1
      max-creations-per-hour: -1

registration:
  require-kyc: false

security:
  deployment-mode: trusted
  allow-self-registration: true
```

See `config.yaml.example` for the complete reference with all available options.

---

## 3. Run

### Quick start (foreground)

```bash
cd /opt/taskschmiede
bin/taskschmiede serve --config-file config/config.yaml &
bin/taskschmiede-portal --config-file config/config.yaml &
bin/taskschmiede-proxy --config-file config/config.yaml &
```

### Firewall

If your server runs a firewall (e.g., `ufw`, `firewalld`), open the service ports:

```bash
# ufw example
sudo ufw allow 9000/tcp comment 'taskschmiede mcp+api'
sudo ufw allow 9001/tcp comment 'taskschmiede proxy'
sudo ufw allow 9090/tcp comment 'taskschmiede portal'
sudo ufw reload
```

If you use a reverse proxy (NGINX, Caddy), only the proxy port (typically 80/443) needs to be open; the Taskschmiede ports can remain localhost-only.

### First-run setup

1. Open the portal at `http://<server>:9090/setup`
2. Create the master admin account (email, name, password)
3. Check your email for the verification code
4. Enter the code to activate the account
5. Log in at `http://<server>:9090/login`

### systemd (Linux production)

Create a service user and set ownership:

```bash
sudo useradd --system --shell /usr/sbin/nologin --home-dir /opt/taskschmiede taskschmiede
sudo chown -R taskschmiede:taskschmiede /opt/taskschmiede
sudo mkdir -p /var/log/taskschmiede
sudo chown taskschmiede:taskschmiede /var/log/taskschmiede
```

**All files under `/opt/taskschmiede/` must be owned by the `taskschmiede` user.** The systemd units run services as this user with `ProtectSystem=strict`, which makes the filesystem read-only except for paths listed in `ReadWritePaths`.

Copy systemd units:

```bash
sudo cp deploy/systemd/taskschmiede.service /etc/systemd/system/
sudo cp deploy/systemd/taskschmiede-portal.service /etc/systemd/system/
sudo cp deploy/systemd/taskschmiede-proxy.service /etc/systemd/system/
sudo systemctl daemon-reload
```

Start and enable:

```bash
sudo systemctl enable --now taskschmiede
sudo systemctl enable --now taskschmiede-portal
sudo systemctl enable --now taskschmiede-proxy
```

Check status:

```bash
sudo systemctl status taskschmiede
sudo journalctl -u taskschmiede -f
```

---

## 4. Connect MCP clients

Add to your MCP client configuration (Claude Code, Opencode, Cursor, etc.):

```json
{
  "mcpServers": {
    "taskschmiede": {
      "url": "http://<server>:9001/mcp"
    }
  }
}
```

Port 9001 is the proxy, which survives server restarts without disconnecting clients. For direct connections, use port 9000.

---

## 5. Verify

```bash
# Health check
curl http://localhost:9000/mcp/health

# Portal
curl http://localhost:9090/health

# MCP proxy
curl http://localhost:9001/mcp/health
```

All should return JSON with `"status":"healthy"`.

---

## 6. Optional Features

### Intercom (Email Bridge)

The intercom bridges internal Taskschmiede messages to external email. When enabled, messages sent within Taskschmiede are copied to the recipient's email, and email replies are ingested back as message replies.

This requires a **dedicated email account** (separate from the support account used for verification). The intercom account needs both SMTP (outgoing) and IMAP (incoming) access.

Add these to `.env`:

```bash
EMAIL_INTERCOM_NAME=Taskschmiede Intercom
EMAIL_INTERCOM_ADDRESS=intercom@example.com
EMAIL_INTERCOM_USER=intercom@example.com
EMAIL_INTERCOM_PASSWORD=your-intercom-password
```

Add the intercom section to `config.yaml` under `email:`:

```yaml
email:
  # ... existing support config ...
  intercom:
    name: ${EMAIL_INTERCOM_NAME}
    address: ${EMAIL_INTERCOM_ADDRESS}
    username: ${EMAIL_INTERCOM_USER}
    password: ${EMAIL_INTERCOM_PASSWORD}

messaging:
  intercom:
    enabled: true
    reply-ttl: 720h            # 30 days -- how long inbound replies are accepted
    sweep-interval: 1m         # IMAP inbox check frequency
    send-interval: 30s         # Outbound email send frequency
    max-retries: 3
    max-inbound-per-hour: 20   # Anti-flooding limit per sender
    dedup-window: 1h
```

Restart the core server after configuration changes.

### Taskschmied (LLM Intelligence)

Taskschmied is the optional governance agent that provides two capabilities:

- **Content Guard**: Reviews entity content (tasks, demands, comments) for prompt injection and adversarial patterns using an LLM. Heuristic scoring always runs; LLM scoring is opt-in.
- **Ritual Executor**: Periodically evaluates scheduled rituals, gathers endeavour context, and generates structured reports via an LLM. Reports are delivered as internal messages to endeavour members.

Both features require access to an OpenAI-compatible LLM endpoint. They share the same configuration pattern: a primary LLM and an optional local fallback, wrapped in a circuit breaker for automatic failover.

Add the LLM credentials to `.env`:

```bash
# Primary LLM (remote, high quality)
TSKSCHMD_PROVIDER=openai          # "openai" (OpenAI-compatible) or "anthropic"
TSKSCHMD_MODEL=gpt-4o             # Model ID as reported by the server
TSKSCHMD_API_KEY=sk-...           # API key (leave empty for keyless endpoints)
TSKSCHMD_HOST=api.openai.com      # Hostname (without protocol)

# Fallback LLM (optional, local CPU model for degraded operation)
TSKSCHMD_FALLBACK_PROVIDER=openai
TSKSCHMD_FALLBACK_MODEL=phi-4-mini
TSKSCHMD_FALLBACK_HOST=localhost:8080
TSKSCHMD_FALLBACK_API_KEY=
```

Add to `config.yaml`:

```yaml
content-guard:
  enabled: true
  provider: ${TSKSCHMD_PROVIDER}
  model: ${TSKSCHMD_MODEL}
  api-key: ${TSKSCHMD_API_KEY}
  api-url: http://${TSKSCHMD_HOST}
  temperature: 0
  reasoning-effort: medium
  reasoning-tokens: 8192
  timeout: 30s
  ticker-interval: 15s
  max-retries: 3
  score-threshold: 0
  # Fallback (optional)
  fallback-provider: ${TSKSCHMD_FALLBACK_PROVIDER}
  fallback-model: ${TSKSCHMD_FALLBACK_MODEL}
  fallback-api-url: http://${TSKSCHMD_FALLBACK_HOST}
  fallback-api-key: ${TSKSCHMD_FALLBACK_API_KEY}
  fallback-timeout: 30s

ritual-executor:
  enabled: true
  provider: ${TSKSCHMD_PROVIDER}
  model: ${TSKSCHMD_MODEL}
  api-key: ${TSKSCHMD_API_KEY}
  api-url: http://${TSKSCHMD_HOST}
  temperature: 0
  reasoning-effort: low
  reasoning-tokens: 0
  timeout: 120s
  ticker-interval: 30s
  max-tokens: 2048
  # Fallback (optional)
  fallback-provider: ${TSKSCHMD_FALLBACK_PROVIDER}
  fallback-model: ${TSKSCHMD_FALLBACK_MODEL}
  fallback-api-url: http://${TSKSCHMD_FALLBACK_HOST}
  fallback-api-key: ${TSKSCHMD_FALLBACK_API_KEY}
  fallback-timeout: 30s
```

Taskschmied is opt-in per endeavour. After enabling it in config, the master admin can toggle it for individual endeavours via the portal (Admin > Endeavours) or the API.

Restart the core server after configuration changes.

---

## Troubleshooting

### Service fails to start (exit code 1)

Check the journal for the error message:

```bash
sudo journalctl -u taskschmiede-proxy --no-pager -n 20
```

If the journal only shows the banner and "FAILURE" with no details, check the log file:

```bash
sudo cat /var/log/taskschmiede/proxy.log
```

### "address already in use"

**Common cause:** A stale process from manual testing (e.g., running the binary directly to debug) is still holding the port. Always stop manual processes before restarting via systemd.

```bash
# Check what's using the port
ss -tlnp | grep 9001

# Kill the stale process
sudo fuser -k 9001/tcp

# Restart the service
sudo systemctl restart taskschmiede-proxy
```

### Email verification fails

Check SMTP connectivity and credentials:

```bash
# Check the log for email errors
sudo grep -i "email\|smtp\|auth" /var/log/taskschmiede/taskschmiede.log | tail -10

# Test SMTP connection directly
echo | openssl s_client -connect your-smtp-server:465 -brief
```

If email is not configured, the setup wizard displays the verification code directly on-screen.

---

## Updating

```bash
cd /path/to/taskschmiede
git pull
make build build-portal build-proxy
sudo systemctl stop taskschmiede taskschmiede-portal taskschmiede-proxy
sudo cp build/taskschmiede build/taskschmiede-portal build/taskschmiede-proxy /opt/taskschmiede/bin/
sudo systemctl start taskschmiede taskschmiede-portal taskschmiede-proxy
```

The database is automatically migrated on startup when needed.

---

## Platform Notes

### macOS

No systemd. Run binaries directly or use `launchd`. The build and configuration steps are the same.

### Windows

Build with `go build -o taskschmiede.exe ./cmd/taskschmiede` (repeat for portal and proxy). Run as a Windows Service using [NSSM](https://nssm.cc/) or similar, or run directly from PowerShell.
