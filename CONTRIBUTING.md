# Contributing to Taskschmiede

Thank you for your interest in contributing to Taskschmiede. This document explains
how to contribute and what to expect during the process.

## Code of Conduct

Be respectful, constructive, and professional. We are building tools for humans and
AI agents to collaborate -- set a good example of collaboration.

## How to Contribute

### Reporting Issues

- Use [GitHub Issues](https://github.com/QuestFinTech/taskschmiede/issues) to report bugs or request features.
- Search existing issues before creating a new one.
- For bugs, include: what happened, what you expected, steps to reproduce, and your environment (OS, Go version, Taskschmiede version).

### Submitting Changes

1. Fork the repository.
2. Create a feature branch from `main` (`git checkout -b feature/your-feature`).
3. Make your changes, following the code style guidelines below.
4. Run `make lint` and `make test` to verify your changes.
5. Commit with a clear message and a DCO sign-off (see below).
6. Open a pull request against `main`.

### What We Accept

- Bug fixes with tests or clear reproduction steps.
- Documentation improvements.
- New MCP tools that follow existing patterns.
- Ritual templates.
- Translations and locale improvements.
- Performance improvements with benchmarks.

### What Needs Discussion First

Open an issue before starting work on:

- New features or significant changes to existing behavior.
- Architectural changes.
- New dependencies.
- Changes to the database schema.

## Developer Certificate of Origin (DCO)

By contributing to this project, you certify that your contribution is in accordance
with the [Developer Certificate of Origin](https://developercertificate.org/):

```
Developer Certificate of Origin
Version 1.1

Copyright (C) 2004, 2006 The Linux Foundation and its contributors.

Everyone is permitted to copy and distribute verbatim copies of this
license document, but changing it is not allowed.

Developer's Certificate of Origin 1.1

By making a contribution to this project, I certify that:

(a) The contribution was created in whole or in part by me and I
    have the right to submit it under the open source license
    indicated in the file; or

(b) The contribution is based upon previous work that, to the best
    of my knowledge, is covered under an appropriate open source
    license and I have the right under that license to submit that
    work with modifications, whether created in whole or in part
    by me, under the same open source license (unless I am
    permitted to submit under a different license), as indicated
    in the file; or

(c) The contribution was provided directly to me by some other
    person who certified (a), (b) or (c) and I have not modified
    it.

(d) I understand and agree that this project and the contribution
    are public and that a record of the contribution (including all
    personal information I submit with it, including my sign-off) is
    maintained indefinitely and may be redistributed consistent with
    this project or the open source license(s) involved.
```

**All commits must include a sign-off line:**

```
Signed-off-by: Your Name <your.email@example.com>
```

Use `git commit -s` to add this automatically. Unsigned commits will not be accepted.

## Code Style

- Follow standard Go conventions (`go fmt`, `go vet`).
- Run `make lint` before submitting -- it enforces project-specific rules.
- Keep functions focused and small.
- Prefer clarity over cleverness.
- No emojis in code, comments, documentation, or commit messages.
- All timestamps must use UTC. Use `time.Now().UTC()` -- never bare `time.Now()`.

## Building and Testing

```bash
make build      # Build for current platform
make test       # Run tests
make lint       # Run linter (required before submitting)
```

## License

By contributing, you agree that your contributions will be licensed under the
Apache License 2.0, the same license that covers the project. See the [LICENSE](LICENSE)
file for details.
