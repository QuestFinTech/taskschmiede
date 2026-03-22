-- TaskSchmiede Database Schema (v0.5.0)
-- SQLite with WAL mode for concurrent access
--
-- Design principles:
-- - No hard deletes (use status fields)
-- - All timestamps in UTC
-- - JSON for flexible metadata
-- - Foreign keys enforced

PRAGMA foreign_keys = ON;
PRAGMA journal_mode = WAL;

--------------------------------------------------------------------------------
-- CORE ENTITIES
--------------------------------------------------------------------------------

-- User: Identity and authentication
CREATE TABLE IF NOT EXISTS user (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    email TEXT UNIQUE,
    password_hash TEXT,                        -- bcrypt hash (NULL for SSO-only users)
    resource_id TEXT REFERENCES resource(id),  -- Links to work capacity (optional)
    external_id TEXT,                          -- Foreign user mgmt system ID (SSO, OIDC, etc.)
    invitation_token_id TEXT REFERENCES invitation_token(id), -- Token used to register (ownership chain)
    status TEXT NOT NULL DEFAULT 'active',     -- active, inactive, suspended
    tier INTEGER NOT NULL DEFAULT 1,           -- Tier level (1=explorer, 2=professional, 3=enterprise)
    user_type TEXT NOT NULL DEFAULT 'human',   -- human, agent
    onboarding_status TEXT NOT NULL DEFAULT 'active', -- active, completed, failed
    lang TEXT NOT NULL DEFAULT 'en',           -- ISO language code (e.g., en, de, fr)
    timezone TEXT NOT NULL DEFAULT 'UTC',      -- IANA timezone (e.g., Europe/Berlin)
    is_admin INTEGER NOT NULL DEFAULT 0,      -- 1 = system admin (replaces master_admin table)
    login_count INTEGER NOT NULL DEFAULT 0,  -- number of successful logins
    email_copy INTEGER NOT NULL DEFAULT 0,    -- 1 = receive external email copies of internal messages
    last_active_at TEXT,                       -- Last authenticated API/MCP activity (UTC)
    totp_secret TEXT,                          -- TOTP secret for 2FA (NULL = not enabled)
    totp_enabled_at TEXT,                      -- When 2FA was activated (UTC)
    retention_hold INTEGER NOT NULL DEFAULT 0, -- 1 = legal hold active
    retention_hold_reason TEXT,                -- Internal note (not shown to user)
    retention_hold_at TEXT,                    -- When hold was placed (UTC)
    retention_hold_by TEXT,                    -- User ID of admin who placed hold
    deletion_requested_at TEXT,               -- When user requested deletion (UTC)
    deletion_scheduled_at TEXT,               -- When deletion will execute (UTC)
    metadata TEXT DEFAULT '{}',                -- JSON object
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_user_email ON user(email);
CREATE INDEX IF NOT EXISTS idx_user_resource ON user(resource_id);
CREATE INDEX IF NOT EXISTS idx_user_external ON user(external_id);
CREATE INDEX IF NOT EXISTS idx_user_status ON user(status);

-- Organization: Group of resources and endeavours
CREATE TABLE IF NOT EXISTS organization (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    status TEXT NOT NULL DEFAULT 'active', -- active, inactive, archived
    taskschmied_enabled INTEGER NOT NULL DEFAULT 0, -- 1 = Taskschmied intelligence enabled
    metadata TEXT DEFAULT '{}',            -- JSON object (billing, settings, etc.)
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_organization_status ON organization(status);

-- Endeavour: Container for related work toward a goal
CREATE TABLE IF NOT EXISTS endeavour (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    goals TEXT,                           -- JSON array of goal strings
    status TEXT NOT NULL DEFAULT 'active', -- pending, active, on_hold, completed, archived
    timezone TEXT NOT NULL DEFAULT 'UTC', -- IANA timezone (e.g., Europe/Berlin)
    lang TEXT NOT NULL DEFAULT 'en',     -- Language code (e.g., en, de, fr)
    taskschmied_enabled INTEGER NOT NULL DEFAULT 0, -- 1 = Taskschmied intelligence enabled
    start_date TEXT,                      -- ISO 8601
    end_date TEXT,                        -- ISO 8601
    completed_at TEXT,                    -- When status became completed (UTC)
    archived_at TEXT,                     -- When status became archived (UTC)
    archived_reason TEXT,                 -- Why it was archived
    metadata TEXT DEFAULT '{}',           -- JSON object
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_endeavour_status ON endeavour(status);
CREATE INDEX IF NOT EXISTS idx_endeavour_dates ON endeavour(start_date, end_date);

-- Demand: What needs to be fulfilled
CREATE TABLE IF NOT EXISTS demand (
    id TEXT PRIMARY KEY,
    type TEXT NOT NULL,                   -- feature, bug, goal, meeting, epic, etc.
    title TEXT NOT NULL,
    description TEXT,
    status TEXT NOT NULL DEFAULT 'open',  -- open, in_progress, fulfilled, canceled
    priority TEXT NOT NULL DEFAULT 'medium', -- low, medium, high, urgent
    creator_id TEXT,                       -- Resource ID of creator
    owner_id TEXT,                         -- Resource ID of owner
    due_date TEXT,                        -- ISO 8601
    metadata TEXT DEFAULT '{}',           -- JSON object
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now')),
    fulfilled_at TEXT,                    -- When status became fulfilled
    canceled_at TEXT,                     -- When status became canceled
    canceled_reason TEXT                  -- Why it was canceled
);

CREATE INDEX IF NOT EXISTS idx_demand_status ON demand(status);
CREATE INDEX IF NOT EXISTS idx_demand_type ON demand(type);
CREATE INDEX IF NOT EXISTS idx_demand_priority ON demand(priority);
CREATE INDEX IF NOT EXISTS idx_demand_due_date ON demand(due_date);
CREATE INDEX IF NOT EXISTS idx_demand_creator ON demand(creator_id);
CREATE INDEX IF NOT EXISTS idx_demand_owner ON demand(owner_id);

-- Resource: Who or what can do work
CREATE TABLE IF NOT EXISTS resource (
    id TEXT PRIMARY KEY,
    type TEXT NOT NULL,                   -- human, agent, service, budget
    name TEXT NOT NULL,
    capacity_model TEXT,                  -- hours_per_week, tokens_per_day, always_on, budget
    capacity_value REAL,                  -- Amount of capacity
    skills TEXT DEFAULT '[]',             -- JSON array of skill strings
    metadata TEXT DEFAULT '{}',           -- JSON object (e.g., email, timezone, model_id)
    status TEXT NOT NULL DEFAULT 'active', -- active, inactive
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_resource_type ON resource(type);
CREATE INDEX IF NOT EXISTS idx_resource_status ON resource(status);

-- Task: Atomic unit of work
CREATE TABLE IF NOT EXISTS task (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    description TEXT,
    status TEXT NOT NULL DEFAULT 'planned', -- planned, active, done, canceled
    estimate REAL,                        -- Estimated hours/units
    actual REAL,                          -- Actual hours/units spent
    creator_id TEXT,                       -- Resource ID of creator
    owner_id TEXT,                         -- Resource ID of owner
    due_date TEXT,                        -- ISO 8601
    metadata TEXT DEFAULT '{}',           -- JSON object
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now')),
    started_at TEXT,                      -- When work began
    completed_at TEXT,                    -- When status became done
    canceled_at TEXT,                     -- When status became canceled
    canceled_reason TEXT                  -- Why it was canceled
);

CREATE INDEX IF NOT EXISTS idx_task_status ON task(status);
CREATE INDEX IF NOT EXISTS idx_task_due_date ON task(due_date);
CREATE INDEX IF NOT EXISTS idx_task_creator ON task(creator_id);
CREATE INDEX IF NOT EXISTS idx_task_owner ON task(owner_id);

-- Token: API access credentials
CREATE TABLE IF NOT EXISTS token (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES user(id),
    token_hash TEXT NOT NULL UNIQUE,           -- SHA-256 hash of token (never store plaintext)
    name TEXT,                                 -- Display name ("CLI token", "CI/CD", etc.)
    expires_at TEXT,                           -- ISO 8601, NULL = never expires
    last_used_at TEXT,
    revoked_at TEXT,                           -- Soft revocation timestamp
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_token_user ON token(user_id);
CREATE INDEX IF NOT EXISTS idx_token_hash ON token(token_hash);


--------------------------------------------------------------------------------
-- FLEXIBLE RELATIONSHIP MODEL (FRM)
--------------------------------------------------------------------------------

-- Generic entity relationship table (replaces purpose-specific join tables)
CREATE TABLE IF NOT EXISTS entity_relation (
    id                  TEXT PRIMARY KEY,
    relationship_type   TEXT NOT NULL,
    source_entity_type  TEXT NOT NULL,
    source_entity_id    TEXT NOT NULL,
    target_entity_type  TEXT NOT NULL,
    target_entity_id    TEXT NOT NULL,
    metadata            TEXT DEFAULT '{}',
    created_by          TEXT,
    created_at          TEXT NOT NULL,
    UNIQUE(relationship_type, source_entity_id, source_entity_type,
           target_entity_id, target_entity_type)
);

CREATE INDEX IF NOT EXISTS idx_rel_source ON entity_relation(source_entity_type, source_entity_id);
CREATE INDEX IF NOT EXISTS idx_rel_target ON entity_relation(target_entity_type, target_entity_id);
CREATE INDEX IF NOT EXISTS idx_rel_type ON entity_relation(relationship_type);

--------------------------------------------------------------------------------
-- BYOM (Bring Your Own Methodology)
--------------------------------------------------------------------------------

-- Artifact: Reference to external things (docs, repos, dashboards, specs)
-- FRM-native: scoped to endeavour or task via entity_relation (belongs_to)
CREATE TABLE IF NOT EXISTS artifact (
    id              TEXT PRIMARY KEY,            -- art_<hex>
    kind            TEXT NOT NULL,               -- link, doc, repo, file, dataset, dashboard, runbook, other
    title           TEXT NOT NULL,
    url             TEXT,                        -- External URL (the common case)
    summary         TEXT,                        -- 1-3 line description
    tags            TEXT DEFAULT '[]',           -- JSON array of string tags
    metadata        TEXT DEFAULT '{}',           -- JSON object
    created_by      TEXT,                        -- res_ or usr_ of creator
    status          TEXT NOT NULL DEFAULT 'active', -- active, archived
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at      TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_artifact_kind ON artifact(kind);
CREATE INDEX IF NOT EXISTS idx_artifact_status ON artifact(status);

-- Methodology: Named methodology that rituals can belong to.
-- Enforces one methodology per endeavour (methodology-agnostic rituals are always allowed).
CREATE TABLE IF NOT EXISTS methodology (
    id              TEXT PRIMARY KEY,            -- mth_<name>
    name            TEXT NOT NULL,               -- Display name (e.g., "Kanban")
    description     TEXT,                        -- Brief explanation
    created_at      TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Ritual: Stored methodology prompt (BYOM core)
-- FRM-native: linked to endeavours/orgs via entity_relation (governs, uses)
CREATE TABLE IF NOT EXISTS ritual (
    id              TEXT PRIMARY KEY,            -- rtl_<hex>
    name            TEXT NOT NULL,               -- e.g. "Weekly planning (Shape Up)"
    description     TEXT,
    prompt          TEXT NOT NULL,               -- The methodology prompt (free-form text)
    version         INTEGER NOT NULL DEFAULT 1,  -- Version within a lineage
    predecessor_id  TEXT REFERENCES ritual(id),  -- rtl_ of the ritual this was derived from
    origin          TEXT NOT NULL DEFAULT 'custom', -- template, custom, fork
    methodology_id  TEXT REFERENCES methodology(id), -- NULL = methodology-agnostic
    schedule        TEXT,                        -- JSON: {"type":"cron|interval|manual", ...}
    is_enabled      INTEGER NOT NULL DEFAULT 1,  -- 0 = disabled, 1 = enabled
    lang            TEXT NOT NULL DEFAULT 'en',  -- Language code (e.g., en, de, fr)
    metadata        TEXT DEFAULT '{}',           -- JSON object
    created_by      TEXT,                        -- usr_ of creator
    status          TEXT NOT NULL DEFAULT 'active', -- active, archived
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at      TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Template: Report and document templates (Go text/template syntax)
-- Separate from rituals: no schedules, no runs, scoped to entity types
CREATE TABLE IF NOT EXISTS template (
    id              TEXT PRIMARY KEY,            -- tpl_<hex>
    name            TEXT NOT NULL,               -- e.g. "Task Report"
    type            TEXT NOT NULL DEFAULT '',     -- report, etc.
    scope           TEXT NOT NULL,               -- task, demand, endeavour
    lang            TEXT NOT NULL DEFAULT 'en',  -- Language code
    body            TEXT NOT NULL,               -- Go text/template Markdown body
    version         INTEGER NOT NULL DEFAULT 1,  -- Version within a lineage
    predecessor_id  TEXT REFERENCES template(id), -- tpl_ of predecessor
    metadata        TEXT DEFAULT '{}',           -- JSON object
    created_by      TEXT,                        -- usr_ of creator
    status          TEXT NOT NULL DEFAULT 'active', -- active, archived
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at      TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(scope, lang, status)                  -- One active template per scope+lang
);

CREATE INDEX IF NOT EXISTS idx_ritual_predecessor ON ritual(predecessor_id);
CREATE INDEX IF NOT EXISTS idx_ritual_origin ON ritual(origin);
CREATE INDEX IF NOT EXISTS idx_ritual_status ON ritual(status);

CREATE INDEX IF NOT EXISTS idx_template_scope ON template(scope);
CREATE INDEX IF NOT EXISTS idx_template_lang ON template(lang);
CREATE INDEX IF NOT EXISTS idx_template_status ON template(status);
CREATE INDEX IF NOT EXISTS idx_template_predecessor ON template(predecessor_id);

-- Ritual Run: Audit record of a single ritual execution
-- Direct ritual_id FK (tight 1:N identity link)
CREATE TABLE IF NOT EXISTS ritual_run (
    id              TEXT PRIMARY KEY,            -- rtr_<hex>
    ritual_id       TEXT NOT NULL REFERENCES ritual(id),
    status          TEXT NOT NULL DEFAULT 'running', -- running, succeeded, failed, skipped
    trigger         TEXT NOT NULL DEFAULT 'manual',  -- schedule, manual, api
    run_by          TEXT,                        -- res_ or usr_ of the runner
    result_summary  TEXT,                        -- Free-form summary of what happened
    effects         TEXT DEFAULT '{}',           -- JSON: {"tasks_created":[],"tasks_updated":[],...}
    error           TEXT,                        -- JSON: {"code":"...","message":"..."} if failed
    metadata        TEXT DEFAULT '{}',           -- JSON object
    started_at      TEXT,                        -- UTC
    finished_at     TEXT,                        -- UTC
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at      TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_ritual_run_ritual ON ritual_run(ritual_id);
CREATE INDEX IF NOT EXISTS idx_ritual_run_status ON ritual_run(status);

--------------------------------------------------------------------------------
-- COMMENTS & APPROVALS
--------------------------------------------------------------------------------

-- Comment: Entity discussions
CREATE TABLE IF NOT EXISTS comment (
    id          TEXT PRIMARY KEY,
    entity_type TEXT NOT NULL,
    entity_id   TEXT NOT NULL,
    author_id   TEXT NOT NULL,
    reply_to_id TEXT REFERENCES comment(id),
    content     TEXT NOT NULL,
    metadata    TEXT DEFAULT '{}',
    deleted_at  TEXT,
    edited_at   TEXT,
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_comment_entity ON comment(entity_type, entity_id, created_at);
CREATE INDEX IF NOT EXISTS idx_comment_author ON comment(author_id);
CREATE INDEX IF NOT EXISTS idx_comment_reply_to ON comment(reply_to_id);

-- Approval: Formal approval decisions
CREATE TABLE IF NOT EXISTS approval (
    id          TEXT PRIMARY KEY,
    entity_type TEXT NOT NULL,
    entity_id   TEXT NOT NULL,
    approver_id TEXT NOT NULL,
    role        TEXT,
    verdict     TEXT NOT NULL,
    comment     TEXT,
    metadata    TEXT DEFAULT '{}',
    created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_approval_entity ON approval(entity_type, entity_id, verdict);
CREATE INDEX IF NOT EXISTS idx_approval_approver ON approval(approver_id);

--------------------------------------------------------------------------------
-- DEFINITION OF DONE (DoD)
--------------------------------------------------------------------------------

-- DoD Policy: Completion criteria policies
CREATE TABLE IF NOT EXISTS dod_policy (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    description     TEXT,
    version         INTEGER NOT NULL DEFAULT 1,
    predecessor_id  TEXT REFERENCES dod_policy(id),
    origin          TEXT NOT NULL DEFAULT 'custom',
    conditions      TEXT NOT NULL DEFAULT '[]',
    strictness      TEXT NOT NULL DEFAULT 'all',
    quorum          INTEGER,
    scope           TEXT NOT NULL DEFAULT 'task',
    status          TEXT NOT NULL DEFAULT 'active',
    created_by      TEXT NOT NULL,
    metadata        TEXT DEFAULT '{}',
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at      TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_dod_policy_status ON dod_policy(status);
CREATE INDEX IF NOT EXISTS idx_dod_policy_origin ON dod_policy(origin);
CREATE INDEX IF NOT EXISTS idx_dod_policy_predecessor ON dod_policy(predecessor_id);

-- DoD Endorsement: Links a DoD policy to a resource+endeavour
CREATE TABLE IF NOT EXISTS dod_endorsement (
    id              TEXT PRIMARY KEY,
    policy_id       TEXT NOT NULL REFERENCES dod_policy(id),
    policy_version  INTEGER NOT NULL,
    resource_id     TEXT NOT NULL,
    endeavour_id    TEXT NOT NULL,
    status          TEXT NOT NULL DEFAULT 'active',
    endorsed_at     TEXT NOT NULL,
    superseded_at   TEXT,
    created_at      TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_dod_endorsement_policy ON dod_endorsement(policy_id);
CREATE INDEX IF NOT EXISTS idx_dod_endorsement_resource ON dod_endorsement(resource_id);
CREATE INDEX IF NOT EXISTS idx_dod_endorsement_endeavour ON dod_endorsement(endeavour_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_dod_endorsement_active
    ON dod_endorsement(resource_id, endeavour_id, policy_id) WHERE status = 'active';

--------------------------------------------------------------------------------
-- KPI & METRICS
--------------------------------------------------------------------------------

-- KPI Snapshot: Point-in-time metrics snapshots
CREATE TABLE IF NOT EXISTS kpi_snapshot (
    id            TEXT PRIMARY KEY,
    timestamp     TEXT NOT NULL,
    data          TEXT NOT NULL,
    endeavour_id  TEXT NOT NULL DEFAULT '',
    scope         TEXT NOT NULL DEFAULT 'system',
    period        TEXT NOT NULL DEFAULT '',
    snapshot_date TEXT NOT NULL DEFAULT '',
    created_at    TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_kpi_snapshot_ts ON kpi_snapshot(timestamp);
CREATE UNIQUE INDEX IF NOT EXISTS idx_kpi_snapshot_edv_unique
    ON kpi_snapshot(endeavour_id, scope, period, snapshot_date)
    WHERE endeavour_id != '';
CREATE INDEX IF NOT EXISTS idx_kpi_snapshot_edv_period
    ON kpi_snapshot(endeavour_id, period, snapshot_date);

--------------------------------------------------------------------------------
-- ACTIVITY & HISTORY
--------------------------------------------------------------------------------

-- Activity log for all entities
CREATE TABLE IF NOT EXISTS activity (
    id TEXT PRIMARY KEY,
    entity_type TEXT NOT NULL,            -- demand, task, endeavour, resource, organization, user
    entity_id TEXT NOT NULL,
    actor_id TEXT REFERENCES resource(id), -- Who did it (null for system)
    action TEXT NOT NULL,                 -- comment, status_change, assignment, creation, update
    content TEXT,                         -- Comment text or change description
    old_value TEXT,                       -- Previous value (for changes)
    new_value TEXT,                       -- New value (for changes)
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_activity_entity ON activity(entity_type, entity_id);
CREATE INDEX IF NOT EXISTS idx_activity_actor ON activity(actor_id);
CREATE INDEX IF NOT EXISTS idx_activity_created ON activity(created_at);
CREATE INDEX IF NOT EXISTS idx_activity_action ON activity(action);

-- Entity change: Structured change tracking for reports and audits
CREATE TABLE IF NOT EXISTS entity_change (
    id TEXT PRIMARY KEY,
    actor_id TEXT NOT NULL,
    action TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    entity_id TEXT NOT NULL,
    endeavour_id TEXT NOT NULL DEFAULT '',
    fields TEXT DEFAULT '[]',
    metadata TEXT DEFAULT '{}',
    created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_entity_change_endeavour ON entity_change(endeavour_id);
CREATE INDEX IF NOT EXISTS idx_entity_change_actor ON entity_change(actor_id);
CREATE INDEX IF NOT EXISTS idx_entity_change_entity ON entity_change(entity_type, entity_id);
CREATE INDEX IF NOT EXISTS idx_entity_change_created ON entity_change(created_at);
CREATE INDEX IF NOT EXISTS idx_entity_change_action ON entity_change(action);

--------------------------------------------------------------------------------
-- AUDIT LOG (security events)
--------------------------------------------------------------------------------

-- Audit log for security events (separate from activity which tracks entity changes)
CREATE TABLE IF NOT EXISTS audit_log (
    id          TEXT PRIMARY KEY,
    action      TEXT NOT NULL,                          -- login_success, login_failure, rate_limit_hit, etc.
    actor_id    TEXT,                                   -- user ID, 'anonymous', or 'system'
    actor_type  TEXT NOT NULL DEFAULT 'anonymous',      -- user, admin, system, anonymous
    resource    TEXT,                                   -- entity_type:entity_id
    method      TEXT,                                   -- HTTP method
    endpoint    TEXT,                                   -- request path
    status_code INTEGER,                               -- HTTP response status
    ip          TEXT,                                   -- client IP
    source      TEXT DEFAULT '',                        -- origin: console, portal, mcp, api, system
    duration_ms INTEGER,                               -- request processing time in milliseconds
    metadata    TEXT DEFAULT '{}',                      -- JSON object
    created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_audit_log_action ON audit_log(action);
CREATE INDEX IF NOT EXISTS idx_audit_log_actor ON audit_log(actor_id);
CREATE INDEX IF NOT EXISTS idx_audit_log_resource ON audit_log(resource);
CREATE INDEX IF NOT EXISTS idx_audit_log_created ON audit_log(created_at);
CREATE INDEX IF NOT EXISTS idx_audit_log_ip ON audit_log(ip);
CREATE INDEX IF NOT EXISTS idx_audit_log_source ON audit_log(source);

--------------------------------------------------------------------------------
-- LABELS & TAGGING
--------------------------------------------------------------------------------

-- Label definitions
CREATE TABLE IF NOT EXISTS label (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    color TEXT,                           -- Hex color (#FF5733)
    description TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Entity-label associations
CREATE TABLE IF NOT EXISTS entity_label (
    entity_type TEXT NOT NULL,            -- demand, task, endeavour, organization
    entity_id TEXT NOT NULL,
    label_id TEXT NOT NULL REFERENCES label(id),
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (entity_type, entity_id, label_id)
);

CREATE INDEX IF NOT EXISTS idx_entity_label_label ON entity_label(label_id);

--------------------------------------------------------------------------------
-- EXTERNAL LINKS
--------------------------------------------------------------------------------

-- Links to external systems (GitHub, etc.)
CREATE TABLE IF NOT EXISTS external_link (
    id TEXT PRIMARY KEY,
    entity_type TEXT NOT NULL,            -- demand, task
    entity_id TEXT NOT NULL,
    system TEXT NOT NULL,                 -- github, gitlab, jira, etc.
    external_id TEXT NOT NULL,            -- Issue number, PR number, etc.
    url TEXT NOT NULL,                    -- Full URL
    sync_status TEXT DEFAULT 'linked',    -- linked, syncing, error
    last_synced_at TEXT,
    metadata TEXT DEFAULT '{}',           -- System-specific data
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (entity_type, entity_id, system, external_id)
);

CREATE INDEX IF NOT EXISTS idx_external_link_entity ON external_link(entity_type, entity_id);
CREATE INDEX IF NOT EXISTS idx_external_link_system ON external_link(system, external_id);

--------------------------------------------------------------------------------
-- VIEWS (Saved filters)
--------------------------------------------------------------------------------

-- Saved views/filters
CREATE TABLE IF NOT EXISTS saved_view (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    entity_type TEXT NOT NULL,            -- demand, task, endeavour, organization
    filters TEXT NOT NULL DEFAULT '{}',   -- JSON filter definition
    owner_id TEXT REFERENCES resource(id), -- Who created it (null = global)
    is_default INTEGER DEFAULT 0,         -- Is this the default view?
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_saved_view_owner ON saved_view(owner_id);
CREATE INDEX IF NOT EXISTS idx_saved_view_entity ON saved_view(entity_type);

--------------------------------------------------------------------------------
-- MASTER ADMIN (Super User)
--------------------------------------------------------------------------------

-- Master admin for system administration via Web UI
-- Only one master admin can exist (enforced by application)
CREATE TABLE IF NOT EXISTS master_admin (
    id TEXT PRIMARY KEY DEFAULT 'master',
    email TEXT NOT NULL UNIQUE,
    name TEXT,                             -- Optional display name
    password_hash TEXT NOT NULL,           -- bcrypt hash
    mcp_user_id TEXT REFERENCES user(id),  -- Linked user for MCP access (NULL = disabled)
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now')),
    last_login_at TEXT
);

-- Web UI sessions for master admin
CREATE TABLE IF NOT EXISTS admin_session (
    id TEXT PRIMARY KEY,                   -- Session token
    admin_id TEXT NOT NULL REFERENCES master_admin(id),
    expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_admin_session_expires ON admin_session(expires_at);

-- Pending admin registration (before email verification)
CREATE TABLE IF NOT EXISTS pending_admin (
    id TEXT PRIMARY KEY DEFAULT 'pending',
    email TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    password_hash TEXT NOT NULL,           -- bcrypt hash
    verification_code TEXT NOT NULL,       -- Format: xxx-xxx-xxx (lowercase alphanumeric)
    expires_at TEXT NOT NULL,              -- When the verification code expires
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Verification codes for password reset
CREATE TABLE IF NOT EXISTS password_reset (
    id TEXT PRIMARY KEY,
    email TEXT NOT NULL,
    code TEXT NOT NULL,                    -- Format: xxx-xxx-xxx (lowercase alphanumeric)
    expires_at TEXT NOT NULL,
    used_at TEXT,                          -- NULL until code is used
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_password_reset_email ON password_reset(email);
CREATE INDEX IF NOT EXISTS idx_password_reset_code ON password_reset(code);

-- Invitation tokens for self-registration (system-wide and org-scoped)
CREATE TABLE IF NOT EXISTS invitation_token (
    id TEXT PRIMARY KEY,
    token_hash TEXT NOT NULL UNIQUE,           -- SHA-256 hash of token
    name TEXT,                                 -- Display name for admin reference
    scope TEXT NOT NULL DEFAULT 'system',      -- 'system' (master admin) or 'organization' (org admin)
    organization_id TEXT REFERENCES organization(id), -- NULL for system scope
    role TEXT,                                 -- Role granted on use (for org tokens: member, admin, etc.)
    max_uses INTEGER DEFAULT 1,                -- Max registrations allowed (NULL = unlimited)
    uses INTEGER DEFAULT 0,                    -- Current number of uses
    expires_at TEXT,                           -- ISO 8601, NULL = never expires
    revoked_at TEXT,                           -- Soft revocation timestamp
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    created_by TEXT REFERENCES user(id)
);

CREATE INDEX IF NOT EXISTS idx_invitation_token_hash ON invitation_token(token_hash);
CREATE INDEX IF NOT EXISTS idx_invitation_token_org ON invitation_token(organization_id);

-- Pending user registration (before email verification)
CREATE TABLE IF NOT EXISTS pending_user (
    id TEXT PRIMARY KEY,
    email TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    password_hash TEXT NOT NULL,               -- bcrypt hash
    invitation_token_id TEXT REFERENCES invitation_token(id),
    verification_code TEXT NOT NULL,           -- Format: xxx-xxx-xxx
    expires_at TEXT NOT NULL,                  -- When the verification code expires
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    user_type TEXT NOT NULL DEFAULT 'human',    -- human or agent
    lang TEXT NOT NULL DEFAULT 'en',            -- ISO language code
    account_type TEXT NOT NULL DEFAULT 'private', -- private or business
    first_name TEXT,
    last_name TEXT,
    company_name TEXT,
    company_registration TEXT,
    vat_number TEXT,
    street TEXT,
    street2 TEXT,
    postal_code TEXT,
    city TEXT,
    state TEXT,
    country TEXT,
    accept_dpa INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_pending_user_email ON pending_user(email);
CREATE INDEX IF NOT EXISTS idx_pending_user_code ON pending_user(verification_code);

-- Pending email change (verified before applying)
CREATE TABLE IF NOT EXISTS pending_email_change (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES user(id),
    new_email TEXT NOT NULL,
    verification_code TEXT NOT NULL,       -- Format: xxx-xxx-xxx
    expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_pending_email_change_user ON pending_email_change(user_id);

--------------------------------------------------------------------------------
-- WAITLIST (registration queue when instance is at capacity)
--------------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS waitlist (
    id TEXT PRIMARY KEY,
    email TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    invitation_token_id TEXT REFERENCES invitation_token(id),
    user_type TEXT NOT NULL DEFAULT 'human',    -- human, agent
    status TEXT NOT NULL DEFAULT 'waiting',     -- waiting, notified, expired, created
    notified_at TEXT,
    expires_at TEXT,                            -- deadline to register after notification
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_waitlist_status ON waitlist(status);
CREATE INDEX IF NOT EXISTS idx_waitlist_email ON waitlist(email);

--------------------------------------------------------------------------------
-- IDENTITY AND LEGAL
--------------------------------------------------------------------------------

-- Person: Natural person behind a user account (1:1 with user)
CREATE TABLE IF NOT EXISTS person (
    id TEXT PRIMARY KEY,                      -- per_<hex>
    user_id TEXT NOT NULL UNIQUE REFERENCES user(id),
    first_name TEXT NOT NULL,
    middle_names TEXT,                        -- Space-separated, optional
    last_name TEXT NOT NULL,
    phone TEXT,                               -- E.164 format recommended
    country TEXT,                             -- ISO 3166-1 alpha-2 (e.g., LU, DE)
    language TEXT,                            -- BCP 47 (e.g., en, de)
    account_type TEXT NOT NULL DEFAULT 'private', -- private, business
    company_name TEXT,                        -- Required if account_type = business
    company_registration TEXT,               -- Trade register number, VAT ID (free text)
    metadata TEXT DEFAULT '{}',
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_person_user ON person(user_id);
CREATE INDEX IF NOT EXISTS idx_person_account_type ON person(account_type);

-- Address: Generic postal address, linked via entity_relation
CREATE TABLE IF NOT EXISTS address (
    id TEXT PRIMARY KEY,                      -- adr_<hex>
    label TEXT,                               -- e.g., Legal address, Billing, Shipping
    street TEXT,
    street2 TEXT,
    city TEXT,
    state TEXT,                               -- State, province, or region
    postal_code TEXT,
    country TEXT NOT NULL,                    -- ISO 3166-1 alpha-2
    metadata TEXT DEFAULT '{}',
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Consent: Legal consent records (terms, privacy, DPA, age declaration)
CREATE TABLE IF NOT EXISTS consent (
    id TEXT PRIMARY KEY,                      -- con_<hex>
    user_id TEXT NOT NULL REFERENCES user(id),
    document_type TEXT NOT NULL,              -- terms, privacy, dpa, age_declaration
    document_version TEXT NOT NULL,           -- Semantic version (e.g., 1.0.0)
    document_url TEXT,                        -- URL of the document at time of acceptance
    accepted_at TEXT NOT NULL,               -- UTC timestamp
    ip_address TEXT,                          -- IP at time of acceptance
    user_agent TEXT,                          -- Browser/client UA string
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_consent_user ON consent(user_id);
CREATE INDEX IF NOT EXISTS idx_consent_user_type ON consent(user_id, document_type);

-- TOTP recovery codes
CREATE TABLE IF NOT EXISTS totp_recovery (
    id TEXT PRIMARY KEY,                      -- trc_<hex>
    user_id TEXT NOT NULL REFERENCES user(id),
    code_hash TEXT NOT NULL,                  -- bcrypt hash of recovery code
    used_at TEXT,                             -- NULL until used, then UTC timestamp
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_totp_recovery_user ON totp_recovery(user_id);

--------------------------------------------------------------------------------
-- ONBOARDING
--------------------------------------------------------------------------------

-- Onboarding attempt: Tracks a single onboarding run for a user
CREATE TABLE IF NOT EXISTS onboarding_attempt (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES user(id),
    version     INTEGER NOT NULL,
    status      TEXT NOT NULL DEFAULT 'running',
    started_at  TEXT NOT NULL,
    completed_at TEXT,
    total_score INTEGER NOT NULL DEFAULT 0,
    result      TEXT DEFAULT '{}',
    tool_log    TEXT DEFAULT '[]',
    created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_onboarding_attempt_user ON onboarding_attempt(user_id);
CREATE INDEX IF NOT EXISTS idx_onboarding_attempt_status ON onboarding_attempt(status);

-- Onboarding section metric: Per-section scores and timing
CREATE TABLE IF NOT EXISTS onboarding_section_metric (
    id           TEXT PRIMARY KEY,
    attempt_id   TEXT NOT NULL REFERENCES onboarding_attempt(id),
    section      INTEGER NOT NULL,
    score        INTEGER NOT NULL DEFAULT 0,
    max_score    INTEGER NOT NULL DEFAULT 20,
    tool_calls   INTEGER NOT NULL DEFAULT 0,
    wall_time_ms INTEGER NOT NULL DEFAULT 0,
    reaction_ms  INTEGER NOT NULL DEFAULT 0,
    payload_bytes INTEGER NOT NULL DEFAULT 0,
    status       TEXT NOT NULL DEFAULT 'pending',
    hint         TEXT
);

CREATE INDEX IF NOT EXISTS idx_onboarding_metric_attempt ON onboarding_section_metric(attempt_id, section);

-- Onboarding cooldown: Rate limiting for onboarding attempts
CREATE TABLE IF NOT EXISTS onboarding_cooldown (
    user_id         TEXT PRIMARY KEY REFERENCES user(id),
    failed_attempts INTEGER NOT NULL DEFAULT 0,
    next_eligible_at TEXT,
    locked          INTEGER NOT NULL DEFAULT 0,
    updated_at      TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Onboarding step 0: Initial interview data
CREATE TABLE IF NOT EXISTS onboarding_step0 (
    id          TEXT PRIMARY KEY,
    attempt_id  TEXT NOT NULL REFERENCES onboarding_attempt(id),
    raw_text    TEXT NOT NULL DEFAULT '',
    model_info  TEXT DEFAULT '{}',
    started_at  TEXT NOT NULL,
    completed_at TEXT
);

CREATE INDEX IF NOT EXISTS idx_onboarding_step0_attempt ON onboarding_step0(attempt_id);

-- Onboarding injection review: LLM-based prompt injection detection
CREATE TABLE IF NOT EXISTS onboarding_injection_review (
    id                 TEXT PRIMARY KEY,
    attempt_id         TEXT NOT NULL REFERENCES onboarding_attempt(id),
    status             TEXT NOT NULL DEFAULT 'pending',
    provider           TEXT NOT NULL DEFAULT '',
    model              TEXT NOT NULL DEFAULT '',
    injection_detected INTEGER NOT NULL DEFAULT 0,
    confidence         REAL NOT NULL DEFAULT 0.0,
    evidence           TEXT DEFAULT '[]',
    raw_response       TEXT DEFAULT '',
    error_message      TEXT DEFAULT '',
    retries            INTEGER NOT NULL DEFAULT 0,
    created_at         TEXT NOT NULL,
    completed_at       TEXT
);

CREATE INDEX IF NOT EXISTS idx_injection_review_attempt ON onboarding_injection_review(attempt_id);
CREATE INDEX IF NOT EXISTS idx_injection_review_status ON onboarding_injection_review(status);

--------------------------------------------------------------------------------
-- AGENT HEALTH & SESSION METRICS
--------------------------------------------------------------------------------

-- Agent session metric: Per-session tool call statistics
CREATE TABLE IF NOT EXISTS agent_session_metric (
    id            TEXT PRIMARY KEY,
    user_id       TEXT NOT NULL,
    session_id    TEXT NOT NULL,
    started_at    TEXT NOT NULL,
    ended_at      TEXT,
    tool_calls    INTEGER NOT NULL DEFAULT 0,
    successful    INTEGER NOT NULL DEFAULT 0,
    client_errors INTEGER NOT NULL DEFAULT 0,
    auth_errors   INTEGER NOT NULL DEFAULT 0,
    server_errors INTEGER NOT NULL DEFAULT 0,
    success_rate  REAL,
    metadata      TEXT DEFAULT '{}',
    created_at    TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_asm_user ON agent_session_metric(user_id);
CREATE INDEX IF NOT EXISTS idx_asm_session ON agent_session_metric(session_id);
CREATE INDEX IF NOT EXISTS idx_asm_started ON agent_session_metric(started_at);

-- Agent health snapshot: Rolling health indicators per user
CREATE TABLE IF NOT EXISTS agent_health_snapshot (
    id                TEXT PRIMARY KEY,
    user_id           TEXT NOT NULL UNIQUE,
    session_rate      REAL,
    session_calls     INTEGER NOT NULL DEFAULT 0,
    rolling_24h_rate  REAL,
    rolling_24h_calls INTEGER NOT NULL DEFAULT 0,
    rolling_7d_rate   REAL,
    rolling_7d_calls  INTEGER NOT NULL DEFAULT 0,
    status            TEXT NOT NULL DEFAULT 'healthy',
    last_checked_at   TEXT NOT NULL,
    metadata          TEXT DEFAULT '{}',
    created_at        TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_ahs_status ON agent_health_snapshot(status);

--------------------------------------------------------------------------------
-- ABLECON (Alert Level)
--------------------------------------------------------------------------------

-- Ablecon level: DEFCON-style alert levels per scope
CREATE TABLE IF NOT EXISTS ablecon_level (
    id         TEXT PRIMARY KEY,
    scope      TEXT NOT NULL,
    scope_id   TEXT NOT NULL DEFAULT '',
    level      INTEGER NOT NULL DEFAULT 1,
    reason     TEXT DEFAULT '{}',
    updated_at TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(scope, scope_id)
);

--------------------------------------------------------------------------------
-- ORGANIZATION ALERT TERMS
--------------------------------------------------------------------------------

-- Org alert terms: Keywords that trigger alerts for an organization
CREATE TABLE IF NOT EXISTS org_alert_terms (
    id TEXT PRIMARY KEY,
    organization_id TEXT NOT NULL REFERENCES organization(id),
    term TEXT NOT NULL,
    weight INTEGER NOT NULL DEFAULT 5,
    created_by TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(organization_id, term)
);

CREATE INDEX IF NOT EXISTS idx_org_alert_terms_org ON org_alert_terms(organization_id);

--------------------------------------------------------------------------------
-- TIER DEFINITIONS
--------------------------------------------------------------------------------

-- Tier definition: Per-tier quota and limit configuration
CREATE TABLE IF NOT EXISTS tier_definition (
    id                     INTEGER PRIMARY KEY,
    name                   TEXT    NOT NULL UNIQUE,
    max_orgs               INTEGER NOT NULL DEFAULT 1,
    max_agents_per_org     INTEGER NOT NULL DEFAULT 5,
    max_endeavours_per_org INTEGER NOT NULL DEFAULT 3,
    max_active_endeavours  INTEGER NOT NULL DEFAULT 1,
    max_teams_per_org      INTEGER NOT NULL DEFAULT 5,
    max_creations_per_hour INTEGER NOT NULL DEFAULT 60,
    max_users              INTEGER NOT NULL DEFAULT -1,
    created_at             TEXT    NOT NULL,
    updated_at             TEXT    NOT NULL
);

--------------------------------------------------------------------------------
-- SYSTEM POLICIES
--------------------------------------------------------------------------------

-- System policies (configurable defaults, managed by master admin)
CREATE TABLE IF NOT EXISTS policy (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    description TEXT,
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Signup interest (no foreign keys, analytics only)
CREATE TABLE IF NOT EXISTS signup_interest (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    email TEXT NOT NULL,
    usecase TEXT NOT NULL DEFAULT '',
    usecase_other TEXT NOT NULL DEFAULT '',
    source TEXT NOT NULL DEFAULT '',
    source_other TEXT NOT NULL DEFAULT '',
    ip TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

--------------------------------------------------------------------------------
-- TRIGGERS
--------------------------------------------------------------------------------

-- Update timestamps on modification
CREATE TRIGGER IF NOT EXISTS update_master_admin_timestamp
    AFTER UPDATE ON master_admin
    BEGIN
        UPDATE master_admin SET updated_at = datetime('now') WHERE id = NEW.id;
    END;

CREATE TRIGGER IF NOT EXISTS update_endeavour_timestamp
    AFTER UPDATE ON endeavour
    BEGIN
        UPDATE endeavour SET updated_at = datetime('now') WHERE id = NEW.id;
    END;

CREATE TRIGGER IF NOT EXISTS update_demand_timestamp
    AFTER UPDATE ON demand
    BEGIN
        UPDATE demand SET updated_at = datetime('now') WHERE id = NEW.id;
    END;

CREATE TRIGGER IF NOT EXISTS update_task_timestamp
    AFTER UPDATE ON task
    BEGIN
        UPDATE task SET updated_at = datetime('now') WHERE id = NEW.id;
    END;

CREATE TRIGGER IF NOT EXISTS update_resource_timestamp
    AFTER UPDATE ON resource
    BEGIN
        UPDATE resource SET updated_at = datetime('now') WHERE id = NEW.id;
    END;

CREATE TRIGGER IF NOT EXISTS update_organization_timestamp
    AFTER UPDATE ON organization
    BEGIN
        UPDATE organization SET updated_at = datetime('now') WHERE id = NEW.id;
    END;

CREATE TRIGGER IF NOT EXISTS update_user_timestamp
    AFTER UPDATE ON user
    BEGIN
        UPDATE user SET updated_at = datetime('now') WHERE id = NEW.id;
    END;

CREATE TRIGGER IF NOT EXISTS update_artifact_timestamp
    AFTER UPDATE ON artifact
    BEGIN
        UPDATE artifact SET updated_at = datetime('now') WHERE id = NEW.id;
    END;

CREATE TRIGGER IF NOT EXISTS update_ritual_timestamp
    AFTER UPDATE ON ritual
    BEGIN
        UPDATE ritual SET updated_at = datetime('now') WHERE id = NEW.id;
    END;

CREATE TRIGGER IF NOT EXISTS update_template_timestamp
    AFTER UPDATE ON template
    BEGIN
        UPDATE template SET updated_at = datetime('now') WHERE id = NEW.id;
    END;

CREATE TRIGGER IF NOT EXISTS update_ritual_run_timestamp
    AFTER UPDATE ON ritual_run
    BEGIN
        UPDATE ritual_run SET updated_at = datetime('now') WHERE id = NEW.id;
    END;

CREATE TRIGGER IF NOT EXISTS update_person_timestamp
    AFTER UPDATE ON person
    BEGIN
        UPDATE person SET updated_at = datetime('now') WHERE id = NEW.id;
    END;

CREATE TRIGGER IF NOT EXISTS update_address_timestamp
    AFTER UPDATE ON address
    BEGIN
        UPDATE address SET updated_at = datetime('now') WHERE id = NEW.id;
    END;

CREATE TRIGGER IF NOT EXISTS update_policy_timestamp
    AFTER UPDATE ON policy
    BEGIN
        UPDATE policy SET updated_at = datetime('now') WHERE key = NEW.key;
    END;

--------------------------------------------------------------------------------
-- INITIAL DATA
--------------------------------------------------------------------------------

-- Default labels
INSERT OR IGNORE INTO label (id, name, color, description) VALUES
    ('lbl_urgent', 'urgent', '#DC2626', 'Requires immediate attention'),
    ('lbl_blocked', 'blocked', '#F59E0B', 'Waiting on external dependency'),
    ('lbl_tech_debt', 'tech-debt', '#6366F1', 'Technical debt to address'),
    ('lbl_documentation', 'documentation', '#10B981', 'Documentation work'),
    ('lbl_bug', 'bug', '#EF4444', 'Bug or defect'),
    ('lbl_enhancement', 'enhancement', '#3B82F6', 'Improvement to existing feature'),
    ('lbl_research', 'research', '#8B5CF6', 'Investigation or research task');

-- System resource (for automated actions)
INSERT OR IGNORE INTO resource (id, type, name, capacity_model, metadata) VALUES
    ('sys_taskschmiede', 'service', 'TaskSchmiede System', 'always_on', '{"description": "System actor for automated actions"}');

-- Default policies
INSERT OR IGNORE INTO policy (key, value, description) VALUES
    ('token.default_ttl', '8h', 'Default time-to-live for login tokens (Go duration). 8h = 8 hours.');

-- Tier quotas are managed in the tier_definition table (not in policy).
INSERT OR IGNORE INTO policy (key, value, description) VALUES
    ('inactivity.warn_days', '14', 'Days of inactivity before warning email is sent'),
    ('inactivity.deactivate_days', '21', 'Days of inactivity before account is deactivated'),
    ('instance.max_active_users', '200', 'Maximum number of active users on this instance'),
    ('inactivity.sweep_capacity_threshold', '0.8', 'Sweep only runs when active user count exceeds this fraction of max_active_users'),
    ('waitlist.notification_window_days', '7', 'Days a notified waitlist entry has to complete registration');

-- Legal and retention policies
INSERT OR IGNORE INTO policy (key, value, description) VALUES
    ('legal.terms_version', '1.0.0', 'Current required Terms and Conditions version'),
    ('legal.privacy_version', '1.0.0', 'Current required Privacy Policy version'),
    ('legal.dpa_version', '1.0.0', 'Current required DPA version (business accounts)'),
    ('retention.deletion_grace_days', '30', 'Days before account deletion executes after request'),
    ('retention.account_closure_days', '365', 'Days to retain account data after closure'),
    ('retention.consent_days', '1095', 'Days to retain consent/compliance records (3 years)'),
    ('retention.support_days', '730', 'Days to retain support records after closure (24 months)'),
    ('retention.login_history_days', '365', 'Days to retain login IP/UA data');
