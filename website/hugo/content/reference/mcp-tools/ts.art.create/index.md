---
title: "ts.art.create"
description: "Create a new artifact (reference to external doc, repo, dashboard, etc.)."
category: "artifact"
requires_auth: true
since: "v0.2.0"
type: docs
---

Create a new artifact (reference to external doc, repo, dashboard, etc.).

**Requires authentication**

## Description

Creates a new artifact in Taskschmiede.

An artifact is a reference to an external resource -- a document, repository,
link, dashboard, runbook, or any other material relevant to a project. Artifacts
can be linked to an endeavour and/or a specific task.

Artifact kinds: link, doc, repo, file, dataset, dashboard, runbook, other.
Artifacts start in "active" status.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `kind` | string | Yes |  | Artifact kind: link, doc, repo, file, dataset, dashboard, runbook, other |
| `title` | string | Yes |  | Artifact title |
| `url` | string |  |  | External URL |
| `summary` | string |  |  | 1-3 line description |
| `tags` | array |  |  | Free-form string tags |
| `endeavour_id` | string |  |  | Endeavour this artifact belongs to |
| `task_id` | string |  |  | Task this artifact belongs to |
| `metadata` | object |  |  | Arbitrary key-value pairs |

## Response

Returns the created artifact summary.

```json
{
  "created_at": "2026-02-09T10:00:00Z",
  "id": "art_d4e5f6a1b2c3...",
  "kind": "doc",
  "status": "active",
  "title": "Architecture Decision Record: FRM Migration"
}
```

## Errors

| Code | Description |
|------|-------------|
| `not_authenticated` | No active login for this session |
| `invalid_input` | Kind and title are required |

## Examples

### Create a doc artifact

Register an architecture decision record linked to an endeavour.

**Request:**

```json
{
  "endeavour_id": "edv_bd159eb7bb9a877a...",
  "kind": "doc",
  "summary": "Documents the decision to migrate from hard-coded foreign keys to FRM.",
  "tags": [
    "architecture",
    "decision-record"
  ],
  "title": "Architecture Decision Record: FRM Migration",
  "url": "https://docs.example.com/adr-001"
}
```

**Response:**

```json
{
  "created_at": "2026-02-09T10:00:00Z",
  "id": "art_d4e5f6a1b2c3...",
  "kind": "doc",
  "status": "active",
  "title": "Architecture Decision Record: FRM Migration"
}
```

### Create a repo artifact

Track a repository as an artifact.

**Request:**

```json
{
  "kind": "repo",
  "tags": [
    "source-code",
    "go"
  ],
  "title": "Taskschmiede Main Repository",
  "url": "https://github.com/QuestFinTech/taskschmiede"
}
```

**Response:**

```json
{
  "created_at": "2026-02-09T10:00:00Z",
  "id": "art_e5f6a1b2c3d4...",
  "kind": "repo",
  "status": "active",
  "title": "Taskschmiede Main Repository"
}
```

## Related Tools

- [`ts.art.get`](../ts.art.get/)
- [`ts.art.list`](../ts.art.list/)
- [`ts.art.update`](../ts.art.update/)

