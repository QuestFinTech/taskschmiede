---
title: "ts.doc.list"
description: "List available documentation (recipes, guides, workflows)"
category: "doc"
requires_auth: false
since: "v0.3.3"
type: docs
---

List available documentation (recipes, guides, workflows)

## Description

Returns a list of available documentation entries. Filter by type to narrow results.

Documentation is public and does not require authentication.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `type` | string |  |  | Filter by type: recipe, guide, workflow (omit for all) |

## Response

Returns an array of documentation entries with name, type, title, summary, and tags.

## Related Tools

- [`ts.doc.get`](../ts.doc.get/)

