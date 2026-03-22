---
title: "Installation"
description: "Install Taskschmiede from pre-built binaries or build from source"
weight: 10
type: docs
---

## From Pre-built Binaries

Go to the [Releases page on GitHub](https://github.com/QuestFinTech/taskschmiede/releases) and download the archive for your platform:

| Platform | File |
|----------|------|
| macOS (Apple Silicon) | `taskschmiede-vX.Y.Z-darwin-arm64.zip` |
| Linux (x86-64) | `taskschmiede-vX.Y.Z-linux-amd64.zip` |
| Windows (x86-64) | `taskschmiede-vX.Y.Z-windows-amd64.zip` |

Or use the GitHub CLI:

```bash
gh release download --repo QuestFinTech/taskschmiede --pattern '*darwin-arm64*'
```

Extract the archive:

```bash
unzip taskschmiede-vX.Y.Z-darwin-arm64.zip
```

Verify the installation:

```bash
taskschmiede version
```

## From Source

### Prerequisites

- **Go 1.26+** ([download](https://go.dev/dl/))
- **Git**
- **Make** (included on macOS and most Linux distributions)

### Clone and Build

```bash
git clone https://github.com/QuestFinTech/taskschmiede.git
cd taskschmiede
make build
```

To build for all supported platforms (darwin-arm64, linux-amd64, windows-amd64):

```bash
make build-all
```

To build and copy binaries to the `run/` folder for local development:

```bash
make deploy-development
```

## Binaries

The build produces three binaries:

| Binary | Purpose |
|--------|---------|
| `taskschmiede` | MCP server and REST API (the core) |
| `taskschmiede-portal` | Web UI for humans |
| `taskschmiede-proxy` | Development proxy (optional) |

You need `taskschmiede` and `taskschmiede-portal` to get started. The proxy is optional.

## Next Steps

- [Configuration]({{< relref "configuration" >}}) -- set up `config.yaml` and `.env`
- [Running (CLI)]({{< relref "cli" >}}) -- start the services and create the master admin
