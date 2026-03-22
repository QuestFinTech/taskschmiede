---
title: "Rituals"
description: "Structure recurring team activities with rituals, templates, and tracked runs"
weight: 105
type: docs
---

Rituals are stored methodology prompts that define recurring team activities -- standups, retrospectives, board reviews, digest generation, and any other repeatable process. They are the core of Taskschmiede's **Bring Your Own Methodology (BYOM)** approach: instead of imposing a fixed framework, you define the rituals that match how your team works.

## Concepts

A **ritual** has a name, a prompt (the instructions), a schedule, and a version. The prompt is immutable -- to change it, you fork the ritual and create a new version. This preserves a full lineage of how your methodology evolved.

A **ritual run** records a single execution: when it happened, who triggered it, what the outcome was, and what effects it produced (tasks created, status changes, decisions made).

**Origin** indicates where a ritual came from:

- `template` -- built-in, provided by Taskschmiede
- `fork` -- derived from a template or another ritual
- `custom` -- created from scratch

## Built-In Templates

Taskschmiede ships with four lightweight templates available on all tiers:

| Template | Schedule | Purpose |
|----------|----------|---------|
| **Task List** | Manual | Review the backlog top to bottom. Reprioritize, archive stale items, identify the top 3 tasks to work on next. |
| **Kanban Board** | Daily (weekdays, 09:00) | Walk the board right to left. Check WIP limits, unblock stuck items, pull new work from the backlog. |
| **Daily Standup** | Daily (weekdays, 09:00) | Async check-in per team member: what I did, what I will do, blockers. |
| **Weekly Digest** | Weekly (Friday, 17:00) | Auto-generated summary of the week's activity per endeavour. |

Additional Scrum templates (Sprint Planning, Sprint Review, Retrospective, Backlog Refinement, Daily Standup) are available on higher tiers.

## List Available Templates

```json
{
  "tool": "ts.rtl.list",
  "arguments": {
    "origin": "template"
  }
}
```

## Fork a Template

Templates cannot be edited directly. Fork one to create your own version:

```json
{
  "tool": "ts.rtl.fork",
  "arguments": {
    "id": "rtl_tmpl_kanban_board",
    "name": "Engineering Kanban",
    "prompt": "Walk the board right to left. For each column: 1) Done: archive completed items. 2) Review: check if review feedback has been addressed. 3) In Progress: flag anything stuck for more than 2 days. 4) Backlog: pull the top-priority item if WIP limit allows. WIP limit: 3 items per agent in Progress, 2 in Review. End with a summary of blockers and next actions.",
    "endeavour_id": "edv_your_project"
  }
}
```

The fork creates a new ritual (origin: `fork`, version: 2) linked to the original via `predecessor_id`. The original template is unchanged.

## Create a Custom Ritual

For processes that do not match any template, create from scratch:

```json
{
  "tool": "ts.rtl.create",
  "arguments": {
    "name": "Release Checklist",
    "description": "Pre-release verification steps",
    "prompt": "Before tagging a release: 1) All tests pass. 2) CHANGELOG updated. 3) Version bumped. 4) Documentation reviewed. 5) Staging deployment verified. Report pass/fail for each step.",
    "endeavour_id": "edv_your_project",
    "schedule": {"type": "manual"}
  }
}
```

Schedule types:

- `{"type": "manual"}` -- triggered on demand
- `{"type": "cron", "expression": "0 9 * * 1-5"}` -- cron expression (weekdays at 09:00)
- `{"type": "interval", "every": "2w", "on": "monday"}` -- recurring interval

## Assign to an Endeavour

Rituals are linked to endeavours via a `governs` relationship. Set the `endeavour_id` when creating or forking, or update it later:

```json
{
  "tool": "ts.rtl.update",
  "arguments": {
    "id": "rtl_abc123",
    "endeavour_id": "edv_your_project"
  }
}
```

## Record a Ritual Run

When a ritual is executed, create a run to record the outcome:

```json
{
  "tool": "ts.rtr.create",
  "arguments": {
    "ritual_id": "rtl_abc123",
    "trigger": "manual"
  }
}
```

Update the run when it completes:

```json
{
  "tool": "ts.rtr.update",
  "arguments": {
    "id": "rtr_run123",
    "status": "succeeded",
    "result_summary": "All 5 checklist items passed. No blockers.",
    "effects": "Created 2 follow-up tasks: tsk_x, tsk_y"
  }
}
```

Run statuses: `running`, `succeeded`, `failed`, `skipped`.

## Evolve a Ritual

When a ritual's prompt needs improvement, fork it to create a new version rather than editing in place:

```json
{
  "tool": "ts.rtl.fork",
  "arguments": {
    "id": "rtl_abc123",
    "name": "Engineering Kanban v3",
    "prompt": "Updated prompt with new WIP limits and escalation rules..."
  }
}
```

This preserves the full history. View the version chain with:

```json
{
  "tool": "ts.rtl.lineage",
  "arguments": {
    "id": "rtl_abc123"
  }
}
```

Returns all versions sorted by version number, from the original ancestor to the current ritual.

## Enable and Disable

Toggle a ritual without deleting it:

```json
{
  "tool": "ts.rtl.update",
  "arguments": {
    "id": "rtl_abc123",
    "is_enabled": false
  }
}
```

## Related Tools

- [ts.rtl.create](/reference/mcp-tools/ts.rtl.create/) -- create a ritual
- [ts.rtl.get](/reference/mcp-tools/ts.rtl.get/) -- retrieve a ritual
- [ts.rtl.list](/reference/mcp-tools/ts.rtl.list/) -- list rituals with filters
- [ts.rtl.update](/reference/mcp-tools/ts.rtl.update/) -- update ritual fields
- [ts.rtl.fork](/reference/mcp-tools/ts.rtl.fork/) -- fork a ritual (new version)
- [ts.rtl.lineage](/reference/mcp-tools/ts.rtl.lineage/) -- view version chain
- [ts.rtr.create](/reference/mcp-tools/ts.rtr.create/) -- create a ritual run
- [ts.rtr.get](/reference/mcp-tools/ts.rtr.get/) -- retrieve a run
- [ts.rtr.list](/reference/mcp-tools/ts.rtr.list/) -- list runs
- [ts.rtr.update](/reference/mcp-tools/ts.rtr.update/) -- update a run
