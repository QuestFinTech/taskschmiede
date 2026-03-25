---
title: "Changelog"
linkTitle: "Changelog"
weight: 90
description: "Release history and what changed in each version."
---

## v0.5.3 (2026-03-24)

Portal improvements from dogfooding on the intranet Community Edition deployment.

### Features

- **Sortable table columns** across all major list views (demands, tasks, endeavours, organizations, messages). Click column headers to sort ascending/descending with visual indicators.
- **Demand type editing** on demand detail page with dropdown selector.
- **"Requirement" demand type** added to create and filter dropdowns.
- **Demand type i18n** across all 8 supported languages.
- **Create Demand form** expanded by default with collapse/expand toggle.
- **Consistent button sizing** on demand detail and report action buttons.

### Bug Fixes

- **Demand editing broken**: Updating title/description triggered "invalid status transition: open -> open" because unchanged fields were sent. Fixed to only send changed fields.
- **Missing Person record**: Self-registration with `require_kyc=false` created user and org but skipped the Person record, causing empty profile fields.
- **Orphan demands**: Demands created without an endeavour became inaccessible due to RBAC. Made `endeavour_id` mandatory at API and portal level.

### Infrastructure

- Added `upgrade-intranet-{app,portal,proxy}` Makefile targets.
- Added `lint-oss-drift` to detect divergence between community-edition files and OSS submodule.
- Fixed Glama.ai badge regression in OSS README.

---

## v0.5.2 (2026-03-24)

Patch release addressing post-launch feedback from four LLM reviewers.

### Features

- **"Why Taskschmiede?" section** added to README.
- **Security highlights** section in README (9 points from SECURITY.md).
- Enabled "View on GitHub" and "Edit this page" links in documentation (repo now public).
- Documented support (transactional) vs intercom (two-way bridge) email channels.

### Bug Fixes

- Fixed setup wizard: master admin setup now creates a personal organization (business accounts named after company, private accounts named after user).
- Fixed broken documentation links (`/reference/mcp/` and `/reference/rest/`).
- Fixed intranet email configuration.

### Content

- Tightened SaaS pricing language ("founding phase", 30-day notice commitment).
- Updated security reporting to use contact form.

---

## v0.5.1 (2026-03-24)

Security hardening, LLM assessment, and email UX improvements.

### Security

- Removed version string from `/mcp/health` endpoints to prevent information disclosure.
- Made `/api/v1/capacity` admin-only (was public).
- Per-environment `.env` files (`.env.ionos`, `.env.staging`, `.env.intranet`).

### Features

- **Ritual executor hardening**: System prompt treats context as untrusted data. Content sanitization redacts entities with harm_score >= 40 before passing to the ritual LLM.
- **Email HTML rendering**: Intercom emails containing Markdown now render as styled HTML using goldmark. Sent as multipart/alternative with both HTML and plain text.
- **Lightweight deploy targets**: `make upgrade-*` for single-binary deploys.

### Assessments

- Content Guard: 102/108 (Excellent) against GPT-OSS-120B.
- Ritual Execution: 313/390 (Good) against GPT-OSS-120B. One critical failure (adversarial injection) fixed via system prompt hardening + content sanitization.

---

## v0.5.0 (2026-03-22)

Initial public release. Taskschmiede goes live on [taskschmiede.dev](https://taskschmiede.dev) (Community Edition) and [taskschmiede.com](https://taskschmiede.com) (Cloud).

### Highlights

- **100+ MCP tools** across 19 categories covering task management, demand tracking, organizations, endeavours, messaging, approvals, definitions of done, rituals, reporting, and agent onboarding.
- **REST API** with bearer token authentication.
- **Web portal** with dashboard, audit trail, Ablecon/Harmcon behavioral indicators, and full entity management.
- **Demand-and-supply model**: all work originates as demands, fulfilled by tasks assigned to resources.
- **Agent onboarding** with structured 8-section interview, Content Guard integration, and email verification.
- **Ritual system** for automated governance reports with LLM-powered analysis.
- **Support agent** for customer inquiries via contact form (SaaS only).
- **Content Guard** for real-time content safety scoring.
- **Single-file storage**: SQLite backend, zero external dependencies.
- **8 languages**: English, German, French, Spanish, Italian, Russian, Chinese, Greek.

### Components

| Binary | Purpose | Default Port |
|--------|---------|:---:|
| `taskschmiede` | Core server (MCP + REST API) | 9000 |
| `taskschmiede-portal` | Web UI for users and administrators | 9090 |
| `taskschmiede-proxy` | MCP dev proxy (auto-reconnect, traffic logging) | 9001 |

### Documentation

- Product website: [taskschmiede.dev](https://taskschmiede.dev)
- API documentation: [docs.taskschmiede.dev](https://docs.taskschmiede.dev)
- GitHub: [QuestFinTech/taskschmiede](https://github.com/QuestFinTech/taskschmiede)
