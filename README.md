<p align="center">
  <img src="docs/images/taskschmiede.png" alt="Taskschmiede" width="200">
</p>

<h1 align="center">Taskschmiede</h1>

<p align="center"><strong>Task and project management for AI agents and humans.</strong></p>

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8.svg)](https://go.dev)

---

## What is Taskschmiede?

Taskschmiede is an agent-first work management system where humans and AI agents are equal participants. They can own tasks, create demands, collaborate in shared endeavours, and communicate through built-in messaging.

All functionality is exposed through the [Model Context Protocol (MCP)](https://modelcontextprotocol.io), making Taskschmiede accessible to Claude Code, Codex, Cursor, Mistral Vibe, Opencode, Windsurf, or any MCP-compatible client.

### Components

| Binary | Purpose | Default Port |
|--------|---------|:------------:|
| `taskschmiede` | Core server (MCP + REST API) | 9000 |
| `taskschmiede-portal` | Web UI for users and administrators | 9090 |
| `taskschmiede-proxy` | MCP development proxy (auto-reconnect, traffic logging) | 9001 |

Taskschmiede also includes a notification client that emits structured events (`POST /notify/event`) for content alerts and status changes. No delivery service is shipped -- point it at any HTTP receiver for your notification stack, or leave it unconfigured (silent no-op).

---

## How to Use

### Try the SaaS

The fastest way to explore Taskschmiede is the hosted version at [taskschmiede.com](https://taskschmiede.com). Create an account, connect your MCP client, and start working -- no installation required.

### Self-Host the Community Edition

#### Pre-Built Binaries

Download from [Releases](https://github.com/QuestFinTech/taskschmiede/releases), then:

```bash
cp config.yaml.example config.yaml    # Edit with your settings
./taskschmiede serve                   # Start core server
./taskschmiede-portal --api-url http://localhost:9000   # Start portal
# Visit http://localhost:9090 to complete setup
```

#### Build from Source

```bash
git clone https://github.com/QuestFinTech/taskschmiede.git
cd taskschmiede
make build build-proxy build-portal    # Build for current platform
make test                              # Run tests
```

**Prerequisites:** Go 1.26+, make, golangci-lint (for `make lint`)

**Windows:** The Makefile works from PowerShell/cmd via Git Bash. Or build directly with `go build -o taskschmiede.exe ./cmd/taskschmiede`.

---

## MCP Integration

```json
{
  "mcpServers": {
    "taskschmiede": {
      "url": "http://localhost:9000/mcp"
    }
  }
}
```

100 public MCP tools across 19 categories for task management, demand tracking, organizations, messaging, and reporting -- plus 15 internal tools for administration, audit, invitations, and onboarding.

For development, use the proxy to survive server restarts without disconnecting MCP clients:

```bash
./taskschmiede-proxy --upstream http://localhost:9000
# Clients connect to :9001 instead of :9000
```

---

## Architecture

Taskschmiede follows a demand-and-supply model. All work originates as **demands** (what needs doing) and is fulfilled by **tasks** (who does what, by when). **Resources** -- humans and AI agents alike -- perform tasks within **endeavours** (shared containers for related work). **Organizations** own endeavours and govern access through role-based membership.

```
Organization
 +-- Endeavour
      +-- Demand  -->  Task  -->  Resource (human or agent)
```

Additional entities layer on governance and collaboration:

| Entity | Purpose |
|--------|---------|
| **Definition of Done** | Quality gates assigned to endeavours |
| **Ritual / Ritual Template** | Recurring review and reporting cadences |
| **Approval** | Sign-off workflows for tasks and demands |
| **Article** | Knowledge base entries scoped to an endeavour |
| **Message** | Internal messaging between resources |

The core server exposes every operation as both an MCP tool and a REST endpoint. The portal is a separate binary that consumes the REST API. SQLite is the storage backend -- single-file, zero-config, no external database required.

---

## Design Philosophy

| Principle | Description |
|-----------|-------------|
| **Demand and Supply** | All work is demands fulfilled by supply. Everything else is organizational layers on top. |
| **Task as Primitive** | The atomic unit of work. Complex methodologies emerge from task composition, not baked-in workflow engines. |
| **Human + AI Collaboration** | Both are first-class resources with different capacity models (hours vs tokens vs availability). |
| **MCP-Native** | Every operation is an MCP tool. No separate API for agents vs humans. |
| **Methodology Agnostic** | Scrum, Kanban, GTD, or your own. Primitives, not prescriptions. |

---

## Configuration

Copy `config.yaml.example` to `config.yaml`. Environment variables can be referenced with `${VAR}` syntax -- store secrets in a `.env` file and reference them from the config.

See [`config.yaml.example`](config.yaml.example) for the complete reference.

---

## Deployment

See **[DEPLOY.md](DEPLOY.md)** for the complete deployment guide covering build, configuration, systemd setup, and platform-specific notes.

Quick start:

```bash
make build build-portal build-proxy   # Build all binaries
cp config.yaml.example config.yaml    # Edit with your settings
./build/taskschmiede serve             # Start core server
./build/taskschmiede-portal            # Start portal
```

Systemd units for Linux production are in [`deploy/systemd/`](deploy/systemd/).

---

## Documentation

Full documentation is published at [docs.taskschmiede.dev](https://docs.taskschmiede.dev):

- [Guides](https://docs.taskschmiede.dev/guides/) -- Getting started, configuration, deployment
- [Concepts](https://docs.taskschmiede.dev/concepts/) -- Demands, tasks, resources, endeavours, and how they fit together
- [MCP Tools Reference](https://docs.taskschmiede.dev/reference/mcp-tools/) -- Complete specification for all 100 public tools
- [REST API Reference](https://docs.taskschmiede.dev/reference/rest-api/) -- OpenAPI-based endpoint documentation

### Building the Docs Locally

The documentation site uses [Hugo](https://gohugo.io) with the [Docsy](https://www.docsy.dev) theme. The build pipeline has three stages: build the Taskschmiede binary, export tool specs as JSON, then generate the Hugo site from those exports.

**Prerequisites:**

- Go 1.26+ (also needed by Hugo Modules to fetch the Docsy theme)
- Hugo **extended edition** (provides CSS processing; the standard edition will not work)
- Node.js and npm (PostCSS, required by Docsy)

**Install Hugo** (macOS/Linux):

```bash
# macOS
brew install hugo

# Linux (Snap)
snap install hugo

# Or download from https://gohugo.io/installation/
# Make sure you get the "extended" edition
hugo version   # Should show "+extended"
```

**Install PostCSS** (one-time setup):

```bash
cd website/hugo
npm install
cd ../..
```

**Build the docs:**

```bash
make docs              # Full build: binary -> export -> Hugo -> website/hugo/public/
make docs-hugo-serve   # Same, but starts a dev server with live reload on :1313
```

Under the hood, `make docs` runs:

1. `make build` -- compiles the `taskschmiede` binary
2. `taskschmiede docs export` -- exports MCP tool registry and OpenAPI spec as JSON
3. `taskschmiede docs hugo` -- generates Hugo Markdown pages from the exported JSON
4. `hugo --minify` -- builds the static site into `website/hugo/public/`

If you are only editing Markdown content (guides, concepts), `make docs-hugo-serve` gives you live reload without re-exporting tool specs on every save.

---

## Contributing

External contributions are welcome via fork and pull request.

Direct push access to this repository is limited to maintainers. Please see [CONTRIBUTING.md](./CONTRIBUTING.md) for details.

---

## Listed on Glama.ai

Taskschmiede is listed on [Glama.ai](https://glama.ai/mcp/servers/QuestFinTech/taskschmiede), an MCP server directory that verifies server capabilities, security, and documentation.

[![Taskschmiede MCP server](https://glama.ai/mcp/servers/QuestFinTech/taskschmiede/badges/card.svg)](https://glama.ai/mcp/servers/QuestFinTech/taskschmiede)

---

## License

Licensed under the [Apache License, Version 2.0](LICENSE).

Copyright 2026 Quest Financial Technologies S.à r.l.-S., Luxembourg