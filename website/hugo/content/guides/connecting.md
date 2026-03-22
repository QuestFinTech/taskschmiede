---
title: "Connecting"
description: "Connect an MCP client or use the REST API"
weight: 50
type: docs
---

Taskschmiede exposes its full functionality through two interfaces: MCP (Model Context Protocol) for AI agents and a REST API for scripts and integrations.

## MCP Clients

### Endpoint

Taskschmiede serves MCP over Streamable HTTP at:

```
http://localhost:9000/mcp
```

If you are using the development proxy (see [Running (CLI)]({{< relref "cli" >}})), connect to the proxy instead:

```
http://localhost:9001/mcp
```

### Claude Code

Add Taskschmiede to `.mcp.json` in your project or `~/.claude.json` globally:

```json
{
  "mcpServers": {
    "taskschmiede": {
      "type": "url",
      "url": "http://localhost:9001/mcp"
    }
  }
}
```

### Codex (OpenAI)

Add to `~/.codex/config.toml` globally or `.codex/config.toml` per project:

```toml
[mcp_servers.taskschmiede]
url = "http://localhost:9001/mcp"
```

### Gemini CLI

Add to `~/.gemini/settings.json` globally or `.gemini/settings.json` per project:

```json
{
  "mcpServers": {
    "taskschmiede": {
      "httpUrl": "http://localhost:9001/mcp"
    }
  }
}
```

### Opencode

Add to your Opencode project configuration:

```json
{
  "mcp": {
    "taskschmiede": {
      "type": "url",
      "url": "http://localhost:9001/mcp"
    }
  }
}
```

### Other MCP Clients

Any client that supports MCP over Streamable HTTP can connect:

| Parameter | Value |
|-----------|-------|
| Transport | Streamable HTTP |
| URL | `http://localhost:9000/mcp` (direct) or `http://localhost:9001/mcp` (proxy) |
| Authentication | Session-based (via `ts.auth.login` tool call after connecting) |

### Authenticate (MCP)

MCP connections use session-based authentication. After connecting, call `ts.auth.login`:

```json
{
  "tool": "ts.auth.login",
  "arguments": {
    "email": "admin@example.com",
    "password": "your-password"
  }
}
```

The session is established server-side. All subsequent tool calls in the same connection are authenticated. Sessions last 24 hours.

Verify your identity:

```json
{
  "tool": "ts.auth.whoami",
  "arguments": {}
}
```

### Troubleshooting

**Client cannot connect:**
- Verify Taskschmiede is running: `curl http://localhost:9000/mcp/health`
- If using the proxy, verify it is running: `curl http://localhost:9001/proxy/health`
- Check that the URL includes the `/mcp` path

**Authentication fails:**
- Confirm your user account exists and is verified
- Check the email and password are correct

**Tools not appearing:**
- Some MCP clients require a restart after adding a new server configuration

---

## REST API

### Base URL

The REST API is served on the same port as the MCP server:

```
http://localhost:9000/api/v1/
```

All endpoints are prefixed with `/api/v1/`. The API accepts and returns JSON.

### Authenticate (REST)

```bash
TOKEN=$(curl -s http://localhost:9000/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email": "admin@example.com", "password": "your-password"}' \
  | jq -r '.token')
```

Pass the token in the `Authorization` header:

```bash
curl -s http://localhost:9000/api/v1/auth/whoami \
  -H "Authorization: Bearer $TOKEN"
```

### Examples

**List organizations:**

```bash
curl -s http://localhost:9000/api/v1/organizations \
  -H "Authorization: Bearer $TOKEN"
```

**Create an organization:**

```bash
curl -s http://localhost:9000/api/v1/organizations \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "Acme Corp", "description": "Our main organization"}'
```

### HTTP Methods

| Method | Purpose | Example |
|--------|---------|---------|
| `GET` | Read or list | `GET /api/v1/tasks` |
| `POST` | Create | `POST /api/v1/tasks` |
| `PATCH` | Update (partial) | `PATCH /api/v1/tasks/{id}` |
| `DELETE` | Delete | `DELETE /api/v1/resources/{id}` |

### Error Responses

```json
{
  "error": {
    "code": "not_found",
    "message": "Task not found"
  },
  "status": 404
}
```

| Code | Meaning |
|------|---------|
| 200 | Success |
| 201 | Created |
| 400 | Bad request (invalid input) |
| 401 | Unauthorized (missing or invalid token) |
| 403 | Forbidden (insufficient permissions) |
| 404 | Not found |
| 429 | Rate limited |

### API Reference

For the complete list of endpoints, request parameters, and response schemas, see the [REST API Reference]({{< relref "/reference/rest-api" >}}).

## Portal Web UI

The portal at `http://localhost:9090` provides a browser-based interface. No additional setup is required -- just log in with your account.

## Next Steps

- [Your First Workflow]({{< relref "first-workflow" >}}) -- create an organization, project, and tasks end-to-end
- [MCP Tools Reference]({{< relref "/reference/mcp-tools" >}}) -- full MCP tool documentation
- [Core Concepts]({{< relref "/concepts/core-concepts" >}}) -- understand the entity model
