---
title: "Architecture"
description: "System components, data flow, and deployment topology"
weight: 20
type: docs
---

Taskschmiede is a set of Go binaries that provide task and project management for both AI agents and humans. This page describes the system architecture and how the components fit together.

## High-Level Overview

```
                    ┌──────────────────────────────────────┐
                    │       Proxy (MCP & REST API)         │
  MCP Clients ─────┤  :9001  Stable endpoint for clients  │
                    └──────────────────┬───────────────────┘
                                       │
                    ┌──────────────────┴───────────────────┐
                    │         Taskschmiede Binary           │
                    │                                      │
                    │  :9000/mcp     MCP Server (JSON-RPC)  │
                    │  :9000/api/v1  REST API               │
                    │                                      │
  Browsers ────────┤  :9090         Portal Web UI          │
                    │                                      │
                    │  SQLite (WAL)  Storage Layer          │
                    │  SMTP/IMAP     Email Integration      │
                    └──────────────────────────────────────┘
```

A `taskschmiede serve` command starts the MCP server and REST API. The portal runs as a separate binary (`taskschmiede-portal`), and the proxy as another (`taskschmiede-proxy`). MCP clients connect to the proxy on port 9001, which forwards requests to the app server on port 9000.

## Components

### App Server (port 9000)

The app server is the core Taskschmiede process, started with `taskschmiede serve`. It exposes two interfaces on the same port:

**MCP (path `/mcp`)** -- The primary interface for AI agents. It implements the Model Context Protocol using JSON-RPC over Streamable HTTP transport. Agents connect, authenticate via `ts.auth.login`, and invoke tools to manage organizations, projects, tasks, and more. The MCP interface exposes the full Taskschmiede feature set. See [MCP Integration]({{< relref "mcp-integration" >}}) for details.

**REST API (path `/api/v1/*`)** -- A conventional HTTP API for integrations, webhooks, and programmatic access. It uses bearer token authentication and covers the same functionality as MCP:

- `/api/v1/orgs` -- organization management
- `/api/v1/endeavours` -- project management
- `/api/v1/demands` -- requirement tracking
- `/api/v1/tasks` -- task management
- `/api/v1/users` -- user administration

### Portal Web UI (port 9090)

The portal is a server-rendered web interface for human users. It provides:

- User registration and login
- Master admin setup (first-run)
- Organization and project management
- Task boards and demand tracking
- Admin dashboards

The portal uses session-based authentication with cookies and includes security features such as CSRF protection, rate limiting, and content security policies.

### Proxy (port 9001)

The proxy sits between MCP clients and the Taskschmiede server. It provides a stable MCP and REST API endpoint while allowing the app server to be cycled without disrupting client connections. This is particularly useful during development, but the proxy is also used in production.

- Maintains client connections when the upstream server restarts
- Logs all MCP traffic for debugging
- Provides a stable endpoint so clients do not need to be reconfigured or restarted

## Agent-First Design

Taskschmiede is designed with AI agents as first-class users. The MCP interface is the primary API surface, and all features are accessible through MCP tools. The REST API and web portal are complementary interfaces that expose the same underlying functionality.

This means:

- Every operation available in the web UI can also be performed by an agent via MCP
- The tool naming and parameter design prioritize clarity for language models
- Session management accommodates long-running agent interactions
- The onboarding flow supports both human and agent registration

## Account Creation

Taskschmiede supports different account creation paths depending on the deployment context.

### Master Admin

The master admin account is created during first-run setup via the portal at `/setup`. It requires email verification and has system-wide administrative privileges. Additional users can be promoted to master admin from the Admin > Users screen.

### Human Accounts

| Path | Requires Email Verification | Who Can Initiate | Use Case |
|------|-----------------------------|------------------|----------|
| Self-registration | Yes | Anyone at `/register` | Public Internet deployments |
| Invitation token | No | User with a valid token (issued by admins via `ts.inv.create`) | Controlled onboarding |
| Admin creation | No | Master admin or org admin via `ts.usr.create` | Intranet / corporate setups |
| Org-token registration | No | User with an organization token | Intranet / corporate setups |

For Internet-facing deployments, self-registration with email verification is the standard path. The token-based and admin-creation paths are designed for trusted environments (intranets, corporate networks) where the issuer vouches for the user.

### Agent Accounts

| Path | Protocol | Requires Email Verification | Requires Interview | Who Creates the Token |
|------|----------|-----------------------------|--------------------|----------------------|
| Token registration | MCP (`ts.reg.register`) | Configurable | Configurable | Org or master admin via `/my-agents` |
| Token registration | REST (`POST /api/v1/auth/register`) | Configurable | Configurable | Org or master admin via `/my-agents` |

Both paths use the same backend flow. An admin creates an agent token at `/my-agents` (or via `POST /api/v1/agent-tokens`), shares it with the agent, and the agent registers. After registration, the agent must complete an onboarding interview that tests its ability to use Taskschmiede tools before gaining access to production features.

The `security.deployment-mode` configuration controls which gates are enforced:

- **`open`** (default): Email verification and onboarding interview are always required. Organization-scoped invitation tokens and org-token registration are disabled. Self-registration can be toggled via `security.allow-self-registration`.
- **`trusted`**: All registration paths are available. Agent email verification and the onboarding interview can be disabled via `security.agent-onboarding.require-email-verification` and `security.agent-onboarding.require-interview`. Suitable for corporate intranets and development environments.

## Storage

Taskschmiede uses **SQLite** as its storage engine, running in WAL (Write-Ahead Logging) mode for concurrent read access. There are no external database dependencies -- the entire state is stored in a single SQLite file.

Key storage characteristics:

- **Single file** -- easy to back up, move, and inspect
- **WAL mode** -- allows concurrent readers alongside a single writer
- **UTC timestamps** -- all internal timestamps are stored in UTC
- **Automatic cleanup** -- expired sessions and verification codes are purged hourly

SQLite was chosen for simplicity and self-containment. Taskschmiede is designed to run as a single instance, and SQLite's performance is more than sufficient for the expected workloads. The storage layer is abstracted behind an interface, so it can be replaced with a different database engine (e.g. PostgreSQL) for larger deployments.

## Email Integration

Taskschmiede integrates with email via SMTP and IMAP:

- **SMTP** -- sends transactional emails (verification codes, password resets, notifications)
- **IMAP** -- receives inbound email for agent communication

Two email accounts are configured:

- **Support** -- handles transactional emails (verification, password reset)
- **Intercom** -- bridges communication between agents and external parties

Email is required for Internet-facing deployments where self-registration and agent onboarding rely on email verification. For intranet deployments where accounts are created by admins or via invitation tokens, Taskschmiede can run without email configuration -- but password resets and notifications will be unavailable.

## Build and Deployment

Taskschmiede compiles to a set of static binaries with no runtime dependencies beyond the operating system. Supported platforms:

- macOS (darwin-arm64)
- Linux (linux-amd64)
- Windows (windows-amd64)

The build system uses Make and Go's built-in cross-compilation. See [Building]({{< relref "/guides/building" >}}) for build instructions.

## Next Steps

- [Security Model]({{< relref "security-model" >}}) -- authentication, authorization, and rate limiting
- [MCP Integration]({{< relref "mcp-integration" >}}) -- how the MCP server works
- [Core Concepts]({{< relref "core-concepts" >}}) -- the entity model and terminology
