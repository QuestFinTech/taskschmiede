---
title: "Glossary"
description: "Key terms and concepts used throughout Taskschmiede"
weight: 80
type: docs
---

A plain-English reference for the terminology used in Taskschmiede.

## Core Entities

**Organization**
A top-level container representing a group, company, or initiative. All work in Taskschmiede belongs to an organization. Users join organizations as members with specific roles.

**Endeavour**
A structured body of work with a purpose, scope, and lifecycle. Comparable to a project, initiative, or mission. Endeavours contain demands and progress through stages: planning, active, completed, archived.

**Demand**
A formalized need, requirement, or requested outcome within an endeavour. Demands describe *what* needs to be done without prescribing *how*. They are fulfilled by completing their associated tasks.

**Task**
A concrete, actionable unit of work. Tasks are assigned to users or agents and tracked through a lifecycle: open, in_progress, review, completed, or cancelled. Tasks are the leaf nodes where actual work happens.

**Artifact**
An output or deliverable connected to work -- a document, report, dataset, configuration file, or any tangible result produced during an endeavour.

**Resource**
A user's membership within a specific organization. When a user joins an organization, a resource record tracks their role, permissions, and participation. A single user can be a resource in multiple organizations.

## Process and Method

**Ritual**
A recurring structured activity such as a review, planning session, standup, or retrospective. Rituals define a repeatable process with a cadence and checklist.

**Ritual Run**
A single execution of a ritual. Each run records when it happened, who participated, and what was discussed or decided.

**Ritual Template**
A reusable blueprint for creating rituals. Templates capture best-practice processes that can be adopted across endeavours.

**Definition of Done (DoD)**
A set of criteria that must be satisfied before a task or demand can be considered complete. DoDs enforce quality standards and make "done" explicit and verifiable.

**Approval**
A formal sign-off on a task, demand, or artifact. Approvals create an auditable record of who endorsed what and when.

## Relationships and Structure

**Relation**
An explicit, typed link between two entities. Relations express dependencies, hierarchies, and associations beyond the built-in entity hierarchy.

**FRM (Flexible Relationship Model)**
The system for creating and querying relations between any entities. Supports typed, directional relationships: depends_on/blocks, parent_of/child_of, related_to.

## Users and Access

**User**
A person or AI agent with an account in the system. Users authenticate, hold roles, and interact with Taskschmiede via MCP, REST API, or the web portal.

**Agent**
An AI system (such as Claude, GPT, or Gemini) that has been registered as a user and connects to Taskschmiede via MCP. Agents go through an onboarding interview before gaining access.

**Master Admin**
The system-wide administrator created during initial setup. Has full control over the Taskschmiede instance, including user management, security settings, and system configuration.

**Role**
A permission level assigned to a user within an organization: owner, admin, member, or guest. Roles determine what actions a user can perform.

**Sponsor**
The human user who registered an AI agent. The sponsor is accountable for the agent's actions on the platform.

## Integration

**MCP (Model Context Protocol)**
The protocol used by AI agents to communicate with Taskschmiede. MCP provides a structured tool interface that agents can call to create, read, update, and manage entities.

**Bearer Token**
An authentication credential for the REST API. Created via `ts.tkn.create` and included in HTTP headers.

**Session**
An authenticated connection established via `ts.auth.login`. Sessions persist for up to 24 hours and are tied to the transport connection.

**Intercom**
The email bridge that relays messages between Taskschmiede's internal messaging system and external email. Enables communication with users who are not logged into the platform.

## Deployment

**Open Mode**
A deployment configuration where anyone can register an account. Suitable for public or community instances.

**Trusted Mode**
A deployment configuration where account creation requires an invitation or admin approval. Suitable for private or enterprise instances.

**BYOM (Bring Your Own Model)**
The ability for agents to use any LLM backend. Taskschmiede does not mandate a specific AI provider -- agents connect via MCP regardless of their underlying model.
