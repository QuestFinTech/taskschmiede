---
title: "Building"
description: "Build targets, cross-compilation, and development commands"
weight: 20
type: docs
---

This guide covers the Makefile targets for building, testing, and packaging Taskschmiede. For initial setup, see [Installation]({{< relref "installation" >}}).

## Build Targets

```bash
make build          # Build all binaries for the current platform
make build-all      # Cross-compile for darwin-arm64, linux-amd64, windows-amd64
```

Individual binaries can be built separately:

```bash
make build-proxy    # Build the MCP proxy only
make build-portal   # Build the portal only
make build-notify   # Build the notification service only
```

## Development Workflow

```bash
make deploy-development   # Build and copy binaries to run/
make test                 # Run all tests with race detector
make lint                 # Run linter (includes UTC policy check)
make check                # Run both lint and test
make fmt                  # Format Go source files
make tidy                 # Run go mod tidy
```

## Packaging and Release

```bash
make package        # Create deployment archive (tar.gz)
make version        # Show current version info
make bump           # Increment patch version and create tag
make bump-minor     # Increment minor version
make bump-major     # Increment major version
make release        # Build, package, and create GitHub release
```

Version format:

- Clean tag: `v0.3.7`
- Commits after tag: `v0.3.7-3-gd619fe6` (3 commits after tag, at commit d619fe6)
- Uncommitted changes: `v0.3.7-dirty`

## Documentation

```bash
make docs               # Full Hugo documentation build
make docs-hugo-serve    # Start Hugo dev server with live reload
make deploy-docs        # Build and deploy docs to docs.taskschmiede.dev
```

## Next Steps

- [Configuration]({{< relref "configuration" >}}) -- set up `config.yaml` and `.env`
- [Deployment]({{< relref "deploy-production" >}}) -- deploy to a server
