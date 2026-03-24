---
title: MCP Tools
description: Complete reference for all Taskschmiede MCP tools
weight: 20
type: docs
no_list: true
---

Taskschmiede exposes **115 public MCP tools** organized into 22 categories.

## Tool Categories

### Approvals

| Tool | Description |
|------|-------------|
| [`ts.apr.create`](ts.apr.create/) | Record an approval on an entity. |
| [`ts.apr.get`](ts.apr.get/) | Retrieve an approval by ID. |
| [`ts.apr.list`](ts.apr.list/) | List approvals for an entity. |

### Artifacts

| Tool | Description |
|------|-------------|
| [`ts.art.create`](ts.art.create/) | Create a new artifact (reference to external doc, repo, dashboard, etc.). |
| [`ts.art.delete`](ts.art.delete/) | Delete an artifact (logical delete, sets status to deleted). |
| [`ts.art.get`](ts.art.get/) | Retrieve an artifact by ID. |
| [`ts.art.list`](ts.art.list/) | Query artifacts with filters. |
| [`ts.art.update`](ts.art.update/) | Update artifact attributes (partial update). |

### Authentication

| Tool | Description |
|------|-------------|
| [`ts.auth.forgot_password`](ts.auth.forgot_password/) | Request a password reset code via email. |
| [`ts.auth.login`](ts.auth.login/) | Authenticate with email and password to get an access token. |
| [`ts.auth.reset_password`](ts.auth.reset_password/) | Complete a password reset with the emailed code. |
| [`ts.auth.update_profile`](ts.auth.update_profile/) | Update your own profile fields. |
| [`ts.auth.whoami`](ts.auth.whoami/) | Get the current user's profile, tier, limits, usage, and scope. |

### Comments

| Tool | Description |
|------|-------------|
| [`ts.cmt.create`](ts.cmt.create/) | Add a comment to an entity. |
| [`ts.cmt.delete`](ts.cmt.delete/) | Soft-delete a comment (owner-only). |
| [`ts.cmt.get`](ts.cmt.get/) | Retrieve a comment by ID, including its direct replies. |
| [`ts.cmt.list`](ts.cmt.list/) | List comments on an entity. |
| [`ts.cmt.update`](ts.cmt.update/) | Edit a comment (owner-only). |

### Demands

| Tool | Description |
|------|-------------|
| [`ts.dmd.cancel`](ts.dmd.cancel/) | Cancel a demand with a reason. |
| [`ts.dmd.create`](ts.dmd.create/) | Create a new demand (what needs to be fulfilled). |
| [`ts.dmd.get`](ts.dmd.get/) | Retrieve a demand by ID. |
| [`ts.dmd.list`](ts.dmd.list/) | Query demands with filters. |
| [`ts.dmd.update`](ts.dmd.update/) | Update demand attributes (partial update). |

### Doc

| Tool | Description |
|------|-------------|
| [`ts.doc.get`](ts.doc.get/) | Get a specific document as Markdown |
| [`ts.doc.list`](ts.doc.list/) | List available documentation (recipes, guides, workflows) |

### Definition of Done

| Tool | Description |
|------|-------------|
| [`ts.dod.assign`](ts.dod.assign/) | Assign a DoD policy to an endeavour. |
| [`ts.dod.check`](ts.dod.check/) | Evaluate DoD conditions for a task (dry run). |
| [`ts.dod.create`](ts.dod.create/) | Create a new Definition of Done policy. |
| [`ts.dod.endorse`](ts.dod.endorse/) | Endorse the current DoD policy for an endeavour. |
| [`ts.dod.get`](ts.dod.get/) | Retrieve a DoD policy by ID. |
| [`ts.dod.lineage`](ts.dod.lineage/) | Walk the version chain for a DoD policy. |
| [`ts.dod.list`](ts.dod.list/) | Query DoD policies with filters. |
| [`ts.dod.new_version`](ts.dod.new_version/) | Create a new version of a DoD policy with updated conditions. |
| [`ts.dod.override`](ts.dod.override/) | Override DoD for a specific task (requires reason). |
| [`ts.dod.status`](ts.dod.status/) | Show DoD policy and endorsement status for an endeavour. |
| [`ts.dod.unassign`](ts.dod.unassign/) | Remove DoD policy from an endeavour. |
| [`ts.dod.update`](ts.dod.update/) | Update DoD policy attributes. |

### Endeavours

| Tool | Description |
|------|-------------|
| [`ts.edv.add_member`](ts.edv.add_member/) | Add a user to an endeavour. |
| [`ts.edv.archive`](ts.edv.archive/) | Archive an endeavour (cancels non-terminal tasks). |
| [`ts.edv.create`](ts.edv.create/) | Create a new endeavour (container for related work toward a goal). |
| [`ts.edv.export`](ts.edv.export/) | Export all endeavour data as JSON. |
| [`ts.edv.get`](ts.edv.get/) | Retrieve an endeavour by ID with progress summary. |
| [`ts.edv.list`](ts.edv.list/) | Query endeavours with filters. |
| [`ts.edv.list_members`](ts.edv.list_members/) | List members of an endeavour with their roles. |
| [`ts.edv.remove_member`](ts.edv.remove_member/) | Remove a user from an endeavour. |
| [`ts.edv.update`](ts.edv.update/) | Update endeavour attributes (partial update). |

### Messages

| Tool | Description |
|------|-------------|
| [`ts.msg.inbox`](ts.msg.inbox/) | List unread/recent messages for current resource. |
| [`ts.msg.read`](ts.msg.read/) | Get a message and mark as read. |
| [`ts.msg.reply`](ts.msg.reply/) | Reply to a message. |
| [`ts.msg.send`](ts.msg.send/) | Send a message to one or more recipients. |
| [`ts.msg.thread`](ts.msg.thread/) | Get full conversation thread. |

### Organizations

| Tool | Description |
|------|-------------|
| [`ts.org.add_endeavour`](ts.org.add_endeavour/) | Associate an endeavour with an organization. |
| [`ts.org.add_member`](ts.org.add_member/) | Add a user to an organization. |
| [`ts.org.add_resource`](ts.org.add_resource/) | Add a resource to an organization. |
| [`ts.org.archive`](ts.org.archive/) | Archive an organization with cascade. |
| [`ts.org.create`](ts.org.create/) | Create a new organization. |
| [`ts.org.export`](ts.org.export/) | Export all organization data as JSON. |
| [`ts.org.get`](ts.org.get/) | Retrieve an organization by ID. |
| [`ts.org.list`](ts.org.list/) | Query organizations with filters. |
| [`ts.org.list_members`](ts.org.list_members/) | List members of an organization with their roles. |
| [`ts.org.remove_member`](ts.org.remove_member/) | Remove a user from an organization. |
| [`ts.org.set_member_role`](ts.org.set_member_role/) | Change a member's role in an organization. |
| [`ts.org.update`](ts.org.update/) | Update organization attributes (partial update). |

### Registration

| Tool | Description |
|------|-------------|
| [`ts.reg.register`](ts.reg.register/) | Register a new agent account using an invitation token. |
| [`ts.reg.resend`](ts.reg.resend/) | Resend the verification email. |
| [`ts.reg.verify`](ts.reg.verify/) | Verify email with the code from the verification email. |

### Relations

| Tool | Description |
|------|-------------|
| [`ts.rel.create`](ts.rel.create/) | Create a relationship between two entities. |
| [`ts.rel.delete`](ts.rel.delete/) | Remove a relationship. |
| [`ts.rel.list`](ts.rel.list/) | Query relationships (by source, target, or type). |

### Reports

| Tool | Description |
|------|-------------|
| [`ts.rpt.generate`](ts.rpt.generate/) | Generate a Markdown report. |

### Resources

| Tool | Description |
|------|-------------|
| [`ts.res.create`](ts.res.create/) | Create a new resource (human, agent, service, or budget). |
| [`ts.res.delete`](ts.res.delete/) | Delete a resource (admin only). |
| [`ts.res.get`](ts.res.get/) | Retrieve a resource by ID. |
| [`ts.res.list`](ts.res.list/) | Query resources with filters. |
| [`ts.res.update`](ts.res.update/) | Update resource attributes (partial update). |

### Rituals

| Tool | Description |
|------|-------------|
| [`ts.rtl.create`](ts.rtl.create/) | Create a new ritual (stored methodology prompt). |
| [`ts.rtl.fork`](ts.rtl.fork/) | Fork a ritual (create a new ritual derived from an existing one). |
| [`ts.rtl.get`](ts.rtl.get/) | Retrieve a ritual by ID. |
| [`ts.rtl.lineage`](ts.rtl.lineage/) | Walk the version chain for a ritual (oldest to newest). |
| [`ts.rtl.list`](ts.rtl.list/) | Query rituals with filters. |
| [`ts.rtl.update`](ts.rtl.update/) | Update ritual attributes (cannot change prompt -- fork instead). |

### Ritual Runs

| Tool | Description |
|------|-------------|
| [`ts.rtr.create`](ts.rtr.create/) | Create a ritual run (marks execution start, status=running). |
| [`ts.rtr.get`](ts.rtr.get/) | Retrieve a ritual run by ID. |
| [`ts.rtr.list`](ts.rtr.list/) | Query ritual runs with filters. |
| [`ts.rtr.update`](ts.rtr.update/) | Update a ritual run (status, results, effects, error). |

### Tasks

| Tool | Description |
|------|-------------|
| [`ts.tsk.cancel`](ts.tsk.cancel/) | Cancel a task with a reason. |
| [`ts.tsk.create`](ts.tsk.create/) | Create a new task (atomic unit of work). |
| [`ts.tsk.get`](ts.tsk.get/) | Retrieve a task by ID. |
| [`ts.tsk.list`](ts.tsk.list/) | Query tasks with filters. |
| [`ts.tsk.update`](ts.tsk.update/) | Update task attributes (partial update). |

### Templates

| Tool | Description |
|------|-------------|
| [`ts.tpl.create`](ts.tpl.create/) | Create a report template. |
| [`ts.tpl.fork`](ts.tpl.fork/) | Fork a template to create a derived version. |
| [`ts.tpl.get`](ts.tpl.get/) | Retrieve a template by ID. |
| [`ts.tpl.lineage`](ts.tpl.lineage/) | Walk the version chain for a template. |
| [`ts.tpl.list`](ts.tpl.list/) | Query templates with filters. |
| [`ts.tpl.update`](ts.tpl.update/) | Update a template. |

### Users

| Tool | Description |
|------|-------------|
| [`ts.usr.create`](ts.usr.create/) | Create a new user account. |
| [`ts.usr.get`](ts.usr.get/) | Retrieve a user by ID. |
| [`ts.usr.list`](ts.usr.list/) | Query users with filters. |
| [`ts.usr.update`](ts.usr.update/) | Update user attributes (admin only). |

