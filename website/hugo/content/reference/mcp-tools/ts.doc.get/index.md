---
title: "ts.doc.get"
description: "Get a specific document as Markdown"
category: "doc"
requires_auth: false
since: "v0.3.3"
type: docs
---

Get a specific document as Markdown

## Description

Returns the full content of a documentation entry as Markdown text. Works for recipes,
guides, and workflows.

Documentation is public and does not require authentication.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `name` | string | Yes |  | Document identifier (e.g., onboard-agent, getting-started) |

## Response

Returns the document content as Markdown text with YAML frontmatter.

## Errors

| Code | Description |
|------|-------------|
| `invalid_input` | name is required |
| `not_found` | Document not found |

## Related Tools

- [`ts.doc.list`](../ts.doc.list/)

