---
title: "MCP Integration"
description: "How Taskschmiede implements the Model Context Protocol"
weight: 70
type: docs
---

Taskschmiede uses the Model Context Protocol (MCP) as its primary interface for AI agents. This page explains what MCP is, how Taskschmiede implements it, and how agents interact with the system.

## What is MCP?

The Model Context Protocol is an open standard for AI agent tool use. It defines how language models discover, invoke, and receive results from external tools. MCP provides a structured, type-safe interface that replaces ad-hoc function calling with a well-defined protocol.

Key characteristics of MCP:

- **Tool discovery** -- clients can list available tools and their schemas
- **Typed parameters** -- each tool defines its input and output schemas
- **Session management** -- persistent connections with state
- **Transport agnostic** -- works over multiple transport mechanisms

## How Taskschmiede Implements MCP

Taskschmiede implements an MCP server using JSON-RPC over Streamable HTTP transport. The server listens on port 9000 at the `/mcp` path.

### Transport

Taskschmiede uses **Streamable HTTP** as the MCP transport. Clients send JSON-RPC requests over HTTP POST and receive responses. For long-running operations, the server can stream incremental results using Server-Sent Events (SSE).

### Endpoint

```
POST http://localhost:9000/mcp
```

All MCP communication goes through this single endpoint. The JSON-RPC payload determines which tool is being called and with what arguments.

### Health Check

```
GET http://localhost:9000/mcp/health
```

Returns `{"status":"ok"}` when the MCP server is operational.

## Tool Naming Convention

Taskschmiede tools follow a consistent naming pattern:

```
ts.<domain>.<action>
```

Where:

- `ts` -- the Taskschmiede namespace prefix
- `<domain>` -- a short abbreviation for the entity or feature area
- `<action>` -- the operation being performed

### Domain Abbreviations

| Abbreviation | Domain |
|-------------|--------|
| `auth` | Authentication |
| `org` | Organizations |
| `edv` | Endeavours |
| `dmd` | Demands |
| `tsk` | Tasks |
| `usr` | Users |
| `res` | Resources |
| `rel` | Relationships |
| `cmt` | Comments |
| `dod` | Definition of Done |
| `apr` | Approvals |
| `inv` | Invitations |
| `tkn` | Tokens |
| `msg` | Messages |
| `rtl` | Rituals |
| `rtr` | Ritual triggers |
| `tpl` | Templates |
| `art` | Artifacts |
| `doc` | Documents |
| `audit` | Audit trail |
| `onboard` | Onboarding |
| `reg` | Registration |
| `rpt` | Reports |

### Common Actions

| Action | Description |
|--------|-------------|
| `create` | Create a new entity |
| `get` | Retrieve a single entity by ID |
| `list` | List entities with optional filters |
| `update` | Modify an existing entity |
| `delete` | Remove an entity |

### Examples

- `ts.tsk.create` -- create a task
- `ts.org.list` -- list organizations
- `ts.dmd.update` -- update a demand
- `ts.auth.login` -- authenticate
- `ts.audit.entity_changes` -- view audit history for an entity

## Session-Based Authentication

MCP connections use session-based authentication. After connecting, the client calls `ts.auth.login` to establish a session. All subsequent tool calls in the same connection are authenticated against this session. Sessions expire after 24 hours.

For practical setup steps, see [Connecting]({{< relref "/guides/connecting" >}}).

## Tool Discovery

MCP clients can discover available tools by sending a `tools/list` request. The server responds with the complete list of tools, including their names, descriptions, and parameter schemas. This allows language models to understand what operations are available without prior knowledge.

## Development Proxy

Taskschmiede provides a development proxy that sits between MCP clients and the server. The proxy maintains client connections when the upstream server restarts, logs traffic for debugging, and provides a stable endpoint so clients never need to be reconfigured during development.

For proxy setup and configuration, see [Running (CLI)]({{< relref "/guides/cli" >}}).

## Error Handling

MCP tool calls return structured error responses when something goes wrong. Errors include:

- **Authentication errors** -- the session is not established or has expired
- **Authorization errors** -- the user does not have permission for the requested operation
- **Validation errors** -- the input parameters are invalid or incomplete
- **Not found errors** -- the referenced entity does not exist
- **Rate limit errors** -- too many requests in a short period

Error responses include a human-readable message that describes the problem and, where applicable, guidance on how to resolve it.

## Next Steps

- [Connecting]({{< relref "/guides/connecting" >}}) -- configure your MCP client or use the REST API
- [Architecture]({{< relref "architecture" >}}) -- how MCP fits into the system
- [Security Model]({{< relref "security-model" >}}) -- authentication and authorization details
