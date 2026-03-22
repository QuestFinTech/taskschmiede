---
title: "Running (CLI)"
description: "Start services, create the master admin, and manage Taskschmiede from the command line"
weight: 30
type: docs
---

Taskschmiede consists of several binaries, each with a specific role:

| Binary | Purpose | Default Port |
|--------|---------|-------------|
| `taskschmiede` | MCP server + REST API | 9000 |
| `taskschmiede-portal` | Web UI for users and admins | 9090 |
| `taskschmiede-proxy` | MCP development proxy | 9001 |

## Starting the Services

Open two terminal windows:

```bash
# Terminal 1: Start the MCP server and REST API
taskschmiede serve

# Terminal 2: Start the portal web UI
taskschmiede-portal
```

You now have:

- **MCP server + REST API** on port 9000
- **Portal web UI** on port 9090

### Development Proxy (Optional)

For development, start the MCP proxy in a third terminal:

```bash
taskschmiede-proxy --upstream http://localhost:9000
```

The proxy (port 9001) keeps MCP client connections alive when you restart the app server. Connect your MCP clients to port 9001 instead of 9000 during development.

### Development Script

For local development, use `scripts/taskschmiede.sh` to manage all services:

```bash
scripts/taskschmiede.sh start all      # Start all services
scripts/taskschmiede.sh stop all       # Stop all services
scripts/taskschmiede.sh restart app    # Restart MCP + REST server only
scripts/taskschmiede.sh status         # Show status of all services
```

**Available targets:** `app` (MCP+REST, :9000), `portal` (:9090), `proxy` (:9001), `testmail` (:9002), `playwright` (:9003), `all`.

The script copies `config.yaml` and `.env` from the project root to `run/` on every start. Always edit the project root copies.

**Health endpoints:**
- App: `http://localhost:9000/mcp/health`
- Proxy: `http://localhost:9001/proxy/health`

**Logs:** `run/server.out`, `run/portal.out`, `run/proxy.out`. MCP traffic: `run/taskschmiede-mcp-traffic.log`.

### Verify

```bash
curl http://localhost:9000/mcp/health
# {"service":"taskschmiede-mcp","status":"healthy","version":"vX.Y.Z"}
```

## Create the Master Admin

On first run, Taskschmiede has no users. Create the master admin through the portal:

1. Open `http://localhost:9090/setup` in your browser.
2. Fill in the form:
   - **Email** -- a valid email address (or any address if running without email -- check the server log for the verification code)
   - **Name** -- your display name
   - **Password** -- minimum 12 characters, with at least one uppercase letter, one lowercase letter, one digit, and one special character
3. Submit. The system sends a verification code in the format `xxx-xxx-xxx`.
4. Enter the code on the verification page.
5. Once verified, log in at `http://localhost:9090/login`.

The master admin has full control over the instance: creating organizations, managing users, and configuring system settings.

---

## Command Reference

### taskschmiede serve

Start the MCP and REST API server.

```bash
taskschmiede serve [flags]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--port` | `9000` | Listen port for MCP and REST API |
| `--config-file` | `config.yaml` | Path to configuration file |
| `--log-level` | `INFO` | Log level: `DEBUG`, `INFO`, `WARN`, `ERROR` |

The server exposes:

- `/mcp` -- MCP endpoint (Streamable HTTP)
- `/mcp/health` -- Health check endpoint
- `/api/v1/*` -- REST API endpoints

### taskschmiede docs

Documentation management commands.

#### taskschmiede docs export

Export documentation in a structured format (JSON, OpenAPI).

```bash
taskschmiede docs export [--format json|openapi]
```

#### taskschmiede docs hugo

Generate Hugo-compatible documentation content (MCP tool pages, REST API pages).

```bash
taskschmiede docs hugo [--output <dir>]
```

### taskschmiede version

Print the current version and build information.

```bash
taskschmiede version
```

Output includes version tag, git commit hash, build timestamp, Go version, and target platform.

---

### taskschmiede-portal

```bash
taskschmiede-portal [flags]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--config-file` | `config.yaml` | Path to configuration file |
| `--log-level` | `INFO` | Log level: `DEBUG`, `INFO`, `WARN`, `ERROR` |

The portal listens on port 9090 by default and provides the browser-based interface for account management, organization setup, and administration.

### taskschmiede-proxy

```bash
taskschmiede-proxy [flags]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--listen` | `:9001` | Proxy listen address |
| `--upstream` | `http://localhost:9000` | Upstream MCP server URL |
| `--log-traffic` | `true` | Enable detailed MCP traffic logging |
| `--log-level` | `DEBUG` | Log level |
| `--config-file` | `config.yaml` | Path to configuration file |

The proxy exposes:

- `/proxy/health` -- Proxy status (clients connected, upstream status)
- `/mcp/health` -- Proxied upstream health check
- `/mcp` -- Proxied MCP endpoint

## Next Steps

- [Connecting]({{< relref "connecting" >}}) -- connect an MCP client or use the REST API
- [Your First Workflow]({{< relref "first-workflow" >}}) -- create an organization, project, and tasks
