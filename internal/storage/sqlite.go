// Copyright 2026 Quest Financial Technologies S.à r.l.-S., Luxembourg
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.


// Package storage provides SQLite database operations for Taskschmiede.
package storage

import (
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

// escapeLike escapes SQL LIKE wildcards (%, _) and the escape character itself
// so that user-supplied search terms are matched literally.
// The escape rune must match the ESCAPE clause in the query (e.g., ESCAPE '\\').
func escapeLike(s string, escape rune) string {
	esc := string(escape)
	s = strings.ReplaceAll(s, esc, esc+esc)
	s = strings.ReplaceAll(s, "%", esc+"%")
	s = strings.ReplaceAll(s, "_", esc+"_")
	return s
}

// schemaFS holds the embedded schema.sql file for database initialization.
//
//go:embed schema.sql
var schemaFS embed.FS

// ritualTemplatesFS holds the embedded ritual template JSON files.
//
//go:embed templates_rituals/*.json
var ritualTemplatesFS embed.FS

// DB wraps the SQLite database connection.
type DB struct {
	*sql.DB
}

// Open opens or creates a SQLite database at the given path.
func Open(path string) (*DB, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return &DB{db}, nil
}

// OpenReadOnly opens an existing SQLite database in read-only mode.
// Used by docs generation to query onboarding data without writing.
func OpenReadOnly(path string) (*DB, error) {
	db, err := sql.Open("sqlite", path+"?mode=ro")
	if err != nil {
		return nil, fmt.Errorf("open database (read-only): %w", err)
	}

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return &DB{db}, nil
}

// Initialize creates all tables if they don't exist and runs migrations.
func (db *DB) Initialize() error {
	schema, err := schemaFS.ReadFile("schema.sql")
	if err != nil {
		return fmt.Errorf("read schema: %w", err)
	}

	_, err = db.Exec(string(schema))
	if err != nil {
		return fmt.Errorf("execute schema: %w", err)
	}

	if err := db.migrate(); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	return nil
}

// migrate seeds data that does not belong in schema.sql.
// For v0.5.0 (first public release), all tables are created by schema.sql.
// This function only seeds reference data via INSERT OR IGNORE.
func (db *DB) migrate() error {
	if err := db.seedMethodologies(); err != nil {
		return fmt.Errorf("methodology seeding: %w", err)
	}
	if err := db.seedDodTemplates(); err != nil {
		return fmt.Errorf("dod template seeding: %w", err)
	}
	if err := db.seedRitualTemplates(); err != nil {
		return fmt.Errorf("ritual template seeding: %w", err)
	}
	if err := db.seedFreeTierRitualTemplates(); err != nil {
		return fmt.Errorf("free tier ritual template seeding: %w", err)
	}
	if err := db.migrateTemplateTypeColumn(); err != nil {
		return fmt.Errorf("template type column migration: %w", err)
	}
	if err := db.seedReportTemplates(); err != nil {
		return fmt.Errorf("report template seeding: %w", err)
	}
	if err := db.seedTierExplorerName(); err != nil {
		return fmt.Errorf("tier explorer name seeding: %w", err)
	}
	return nil
}

// seedMethodologies inserts the built-in methodology definitions.
func (db *DB) seedMethodologies() error {
	methodologies := []struct {
		id, name, description string
	}{
		{"mth_kanban", "Kanban", "Flow-based methodology with WIP limits and pull-based scheduling"},
		{"mth_scrum", "Scrum", "Sprint-based agile methodology with defined ceremonies"},
		{"mth_okr", "OKR", "Objectives and Key Results -- goal-setting framework"},
		{"mth_gtd", "GTD", "Getting Things Done -- personal productivity methodology"},
		{"mth_design_sprint", "Design Sprint", "Google Ventures 5-day design sprint"},
	}
	for _, m := range methodologies {
		_, _ = db.Exec(
			`INSERT OR IGNORE INTO methodology (id, name, description) VALUES (?, ?, ?)`,
			m.id, m.name, m.description,
		)
	}
	return nil
}

// seedDodTemplates inserts built-in Definition of Done policy templates.
func (db *DB) seedDodTemplates() error {
	now := UTCNow().Format("2006-01-02T15:04:05Z")
	templates := []struct {
		id, name, description, conditions string
	}{
		{
			"dod_tmpl_minimal",
			"Minimal",
			"Conscious closure -- think before you mark done",
			`[{"id":"cond_01","type":"manual_attestation","label":"I confirm this task is complete","params":{"prompt":"Confirm that the work is finished and meets the task description"},"required":true}]`,
		},
		{
			"dod_tmpl_peer_reviewed",
			"Peer Reviewed",
			"Requires peer review before closure",
			`[{"id":"cond_01","type":"peer_review","label":"At least one peer review","params":{"min_reviewers":1,"exclude_author":true},"required":true},{"id":"cond_02","type":"comment_required","label":"Completion notes","params":{"min_comments":1},"required":true}]`,
		},
		{
			"dod_tmpl_full_governance",
			"Full Governance",
			"Comprehensive completion criteria with sign-off",
			`[{"id":"cond_01","type":"peer_review","label":"Peer review","params":{"min_reviewers":2,"exclude_author":true},"required":true},{"id":"cond_02","type":"checklist_complete","label":"All checklist items done","params":{},"required":true},{"id":"cond_03","type":"tests_pass","label":"Tests pass","params":{"artifact_tag":"test-report"},"required":true},{"id":"cond_04","type":"stakeholder_sign_off","label":"Product owner sign-off","params":{"role":"product_owner"},"required":true},{"id":"cond_05","type":"comment_required","label":"Completion summary","params":{"min_comments":1},"required":true}]`,
		},
		{
			"dod_tmpl_agent_autonomous",
			"Agent Autonomous",
			"Machine-verifiable completion criteria for autonomous agents",
			`[{"id":"cond_01","type":"tests_pass","label":"Automated tests pass","params":{"artifact_tag":"test-report"},"required":true},{"id":"cond_02","type":"field_populated","label":"Result summary provided","params":{"field":"result_summary"},"required":true},{"id":"cond_03","type":"checklist_complete","label":"Acceptance criteria met","params":{},"required":true}]`,
		},
	}

	for _, t := range templates {
		_, _ = db.Exec(
			`INSERT OR IGNORE INTO dod_policy (id, name, description, origin, conditions, strictness, scope, status, created_by, metadata, created_at, updated_at)
			 VALUES (?, ?, ?, 'template', ?, 'all', 'task', 'active', 'sys_taskschmiede', '{}', ?, ?)`,
			t.id, t.name, t.description, t.conditions, now, now,
		)
	}

	return nil
}

// ritualTemplateFile is the JSON structure for embedded ritual template files.
type ritualTemplateFile struct {
	ID       string                              `json:"id"`
	Schedule json.RawMessage                     `json:"schedule"`
	Metadata json.RawMessage                     `json:"metadata"`
	Enabled  bool                                `json:"enabled"`
	Langs    map[string]ritualTemplateTranslation `json:"translations"`
}

type ritualTemplateTranslation struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Prompt      string `json:"prompt"`
}

// seedRitualTemplates inserts built-in ritual templates from embedded JSON files.
// Templates use INSERT OR IGNORE for idempotency -- safe to run on every startup.
// Each translation creates a separate ritual row with a language-suffixed ID.
// Old methodology-specific templates (v1) are archived on upgrade.
func (db *DB) seedRitualTemplates() error {
	// Ensure methodology rows exist for backward compatibility with existing forks.
	for _, m := range []struct{ id, name, desc string }{
		{"mth_kanban", "Kanban", "Flow-based methodology with WIP limits and pull-based scheduling"},
		{"mth_scrum", "Scrum", "Sprint-based agile methodology with defined ceremonies"},
		{"mth_okr", "OKR", "Objectives and Key Results -- goal-setting framework"},
		{"mth_gtd", "GTD", "Getting Things Done -- personal productivity methodology"},
		{"mth_design_sprint", "Design Sprint", "Google Ventures 5-day design sprint"},
	} {
		_, _ = db.Exec(`INSERT OR IGNORE INTO methodology (id, name, description) VALUES (?, ?, ?)`, m.id, m.name, m.desc)
	}

	// Archive v1 templates replaced by agent-centric set.
	// Forked rituals (origin='fork') are unaffected.
	v1IDs := []string{
		"rtl_tmpl_okr_weekly_checkin", "rtl_tmpl_okr_quarterly_scoring",
		"rtl_tmpl_gtd_weekly_review", "rtl_tmpl_gtd_daily_processing", "rtl_tmpl_gtd_monthly_review",
		"rtl_tmpl_design_sprint_day1", "rtl_tmpl_design_sprint_day2", "rtl_tmpl_design_sprint_day3",
		"rtl_tmpl_design_sprint_day4", "rtl_tmpl_design_sprint_day5",
		"rtl_tmpl_scrum_sprint_planning", "rtl_tmpl_scrum_daily_standup", "rtl_tmpl_scrum_retrospective",
		"rtl_tmpl_kanban_board_walk", "rtl_tmpl_kanban_replenishment",
		"rtl_tmpl_task_list", "rtl_tmpl_kanban_board", "rtl_tmpl_daily_standup", "rtl_tmpl_weekly_digest",
	}
	for _, id := range v1IDs {
		_, _ = db.Exec(`UPDATE ritual SET status = 'archived' WHERE id = ? AND origin = 'template'`, id)
	}

	// Load ritual templates from embedded JSON files.
	entries, err := ritualTemplatesFS.ReadDir("templates_rituals")
	if err != nil {
		return fmt.Errorf("read ritual templates dir: %w", err)
	}

	now := UTCNow().Format("2006-01-02T15:04:05Z")
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := ritualTemplatesFS.ReadFile("templates_rituals/" + entry.Name())
		if err != nil {
			return fmt.Errorf("read ritual template %s: %w", entry.Name(), err)
		}
		var tmpl ritualTemplateFile
		if err := json.Unmarshal(data, &tmpl); err != nil {
			return fmt.Errorf("parse ritual template %s: %w", entry.Name(), err)
		}

		schedule := string(tmpl.Schedule)
		metadata := string(tmpl.Metadata)
		enabled := 0
		if tmpl.Enabled {
			enabled = 1
		}

		for lang, tr := range tmpl.Langs {
			// English uses the base ID; other languages get a suffix.
			id := tmpl.ID
			if lang != "en" {
				id = tmpl.ID + "_" + lang
			}
			_, _ = db.Exec(
				`INSERT OR IGNORE INTO ritual (id, name, description, prompt, version,
				 predecessor_id, origin, methodology_id, schedule, is_enabled, lang, metadata,
				 created_by, status, created_at, updated_at)
				 VALUES (?, ?, ?, ?, 1, NULL, 'template', NULL, ?, ?, ?, ?, 'sys_taskschmiede',
				 'active', ?, ?)`,
				id, tr.Name, tr.Description, tr.Prompt, schedule, enabled, lang, metadata, now, now,
			)
		}
	}

	return nil
}

// seedFreeTierRitualTemplates inserts the four free-tier ritual templates
// (Task List, Kanban Board, Daily Standup, Weekly Digest).
// Idempotent: uses INSERT OR IGNORE so existing templates are not overwritten.
func (db *DB) seedFreeTierRitualTemplates() error {
	now := UTCNow().Format("2006-01-02T15:04:05Z")
	templates := []struct {
		id, name, description, prompt, schedule, metadata string
	}{
		{
			"rtl_tmpl_task_list",
			"Task List",
			"Ordered backlog with status tracking -- the simplest way to manage work",
			"Review the task list top to bottom. For each task: Is the status current? Is the priority still correct? Are there any blockers? Reorder the list based on current priorities. Archive or cancel tasks that are no longer relevant. Identify the top 3 tasks that should be worked on next. If the list has grown beyond 20 items, consider whether some should be grouped into a demand or broken down differently.",
			`{"type":"manual"}`,
			`{"category":"explorer","ceremony":"task_list_review"}`,
		},
		{
			"rtl_tmpl_kanban_board",
			"Kanban Board",
			"Visual workflow with columns (backlog, in-progress, review, done) and WIP limits",
			"Organize tasks into four columns: Backlog, In Progress, Review, and Done. Set WIP limits: no more than 3 items in In Progress and 2 items in Review at any time. Walk the board right-to-left: start from Review -- are items waiting for feedback? Move to In Progress -- are any items stuck or aging beyond 2 days? Check Backlog -- is the next priority item ready to pull? Only pull new work when capacity opens up. The goal is smooth flow, not maximum utilization. Track cycle time: how long does a typical item take from In Progress to Done?",
			`{"type":"cron","expression":"0 9 * * 1-5"}`,
			`{"category":"explorer","methodology":"kanban","ceremony":"board_management"}`,
		},
		{
			"rtl_tmpl_daily_standup",
			"Daily Standup",
			"Async daily check-in: what I did, what I will do, blockers",
			"Each team member (human or agent) answers three questions: 1) What did I complete since the last check-in? Reference specific task IDs. 2) What will I work on next? Name the task or demand. 3) Are there any blockers or things I need help with? Keep responses concise -- 2-3 sentences per question. If a blocker is raised, assign someone to resolve it and set a deadline. Review yesterday's blockers -- were they resolved? This check-in works asynchronously: participants post their update within the check-in window, no simultaneous attendance required.",
			`{"type":"cron","expression":"0 9 * * 1-5"}`,
			`{"category":"explorer","ceremony":"daily_standup"}`,
		},
		{
			"rtl_tmpl_weekly_digest",
			"Weekly Digest",
			"Auto-generated summary of the week's activity per endeavour",
			"Generate a structured summary of this week's activity. Include: Tasks completed this week (count and list). Tasks started but not yet finished. New demands created. Blockers raised and their resolution status. Key decisions made (from comments and approvals). Contributors active this week. Compare to last week: is velocity trending up, down, or stable? Highlight any tasks that have been in progress for more than 5 days. End with a 2-3 sentence narrative summary suitable for stakeholders who did not follow the daily activity.",
			`{"type":"cron","expression":"0 17 * * 5"}`,
			`{"category":"explorer","ceremony":"weekly_digest"}`,
		},
	}

	for _, t := range templates {
		if _, err := db.Exec(
			`INSERT OR IGNORE INTO ritual (id, name, description, prompt, version,
			 predecessor_id, origin, schedule, is_enabled, metadata, created_by,
			 status, created_at, updated_at)
			 VALUES (?, ?, ?, ?, 1, NULL, 'template', ?, 1, ?, 'sys_taskschmiede',
			 'active', ?, ?)`,
			t.id, t.name, t.description, t.prompt, t.schedule, t.metadata, now, now,
		); err != nil {
			return fmt.Errorf("seed free tier ritual %s: %w", t.name, err)
		}
	}

	return nil
}

// seedReportTemplates creates default English report templates if none exist.
// Idempotent: skips any scope that already has an active template for "en".
// Template bodies include the final v0.5.0 content (activity logs, time metrics,
// burndown, contributors, ritual execution, elapsed days).
func (db *DB) seedReportTemplates() error {
	seeds := []struct {
		name  string
		scope string
		body  string
	}{
		{"Task Report", "task", seedTaskTemplate},
		{"Demand Report", "demand", seedDemandTemplate},
		{"Endeavour Report", "endeavour", seedEndeavourTemplate},
		{"Project Summary", "project", seedProjectTemplate},
	}

	for _, s := range seeds {
		var count int
		err := db.QueryRow(
			"SELECT COUNT(*) FROM template WHERE scope = ? AND lang = 'en' AND status = 'active'",
			s.scope,
		).Scan(&count)
		if err != nil {
			return fmt.Errorf("check template seed %s: %w", s.scope, err)
		}
		if count > 0 {
			continue
		}
		_, err = db.CreateTemplate(s.name, "report", s.scope, "en", s.body, "", 1, "", nil)
		if err != nil {
			return fmt.Errorf("seed template %s: %w", s.scope, err)
		}
	}
	return nil
}

// migrateTemplateTypeColumn adds the type column to the template table if it
// does not exist yet, and backfills existing rows with type='report'.
func (db *DB) migrateTemplateTypeColumn() error {
	// Check if the column already exists.
	var count int
	err := db.QueryRow(
		`SELECT COUNT(*) FROM pragma_table_info('template') WHERE name = 'type'`,
	).Scan(&count)
	if err != nil {
		return fmt.Errorf("check template type column: %w", err)
	}
	if count > 0 {
		// Column exists; backfill any rows that still have an empty type.
		_, _ = db.Exec(`UPDATE template SET type = 'report' WHERE type = ''`)
		return nil
	}

	// Add the column and backfill.
	_, err = db.Exec(`ALTER TABLE template ADD COLUMN type TEXT NOT NULL DEFAULT ''`)
	if err != nil {
		return fmt.Errorf("add template type column: %w", err)
	}
	_, err = db.Exec(`UPDATE template SET type = 'report'`)
	if err != nil {
		return fmt.Errorf("backfill template type: %w", err)
	}
	return nil
}

// seedTierExplorerName ensures tier 1 is named "explorer" (not legacy "free").
func (db *DB) seedTierExplorerName() error {
	_, _ = db.Exec(
		`UPDATE tier_definition SET name = 'explorer', updated_at = ? WHERE id = 1 AND name = 'free'`,
		UTCNow().Format("2006-01-02T15:04:05Z"),
	)
	return nil
}

// ---------------------------------------------------------------------------
// Report template bodies (final v0.5.0 content)
// ---------------------------------------------------------------------------

var seedTaskTemplate = `# Task Report: {{.Title}}

**Generated:** {{.Generated}}

## Overview

| Field | Value |
|-------|-------|
| ID | {{.ID}} |
| Status | {{.Status}} |
| Assignee | {{.AssigneeName}} |
| Owner | {{.OwnerName}} |
| Creator | {{.CreatorName}} |
{{- if .DueDate}}
| Due Date | {{.DueDate}} |
{{- end}}
{{- if .Estimate}}
| Estimate | {{.Estimate}} |
{{- end}}
{{- if .Actual}}
| Actual | {{.Actual}} |
{{- end}}
| Created | {{.CreatedAt}} |
| Updated | {{.UpdatedAt}} |
{{- if .StartedAt}}
| Started | {{.StartedAt}} |
{{- end}}
{{- if .CompletedAt}}
| Completed | {{.CompletedAt}} |
{{- end}}
{{- if .CanceledAt}}
| Canceled | {{.CanceledAt}} |
{{- end}}
{{- if .Description}}

## Description

{{.Description}}
{{- end}}
{{- if .EndeavourName}}

## Context

- **Endeavour:** {{.EndeavourName}}
{{- end}}
{{- if .CanceledReason}}

## Cancellation

**Reason:** {{.CanceledReason}}
{{- end}}
{{- if .LeadTimeDays}}

## Time Metrics

| Metric | Value |
|--------|-------|
| Lead Time | {{.LeadTimeDays}} days |
{{- if .CycleTimeDays}}
| Cycle Time | {{.CycleTimeDays}} days |
{{- end}}
{{- end}}
{{- if .HasChanges}}

## Activity Log

| # | Date | Actor | Action | Fields |
|---|------|-------|--------|--------|
{{- range .Changes}}
| {{.Num}} | {{.Date}} | {{.Actor}} | {{.Action}} | {{.Fields}} |
{{- end}}
{{- end}}
{{- if .HasComments}}

## Comments

| # | Date | Author | Comment |
|---|------|--------|---------|
{{- range .Comments}}
| {{.Num}} | {{.Date}} | {{.Author}} | {{.Content}} |
{{- end}}
{{- end}}`

var seedDemandTemplate = `# Demand Report: {{.Title}}

**Generated:** {{.Generated}}

## Overview

| Field | Value |
|-------|-------|
| ID | {{.ID}} |
| Status | {{.Status}} |
| Type | {{.Type}} |
| Priority | {{.Priority}} |
| Owner | {{.OwnerName}} |
| Creator | {{.CreatorName}} |
{{- if .DueDate}}
| Due Date | {{.DueDate}} |
{{- end}}
| Created | {{.CreatedAt}} |
| Updated | {{.UpdatedAt}} |
{{- if .Description}}

## Description

{{.Description}}
{{- end}}
{{- if .EndeavourName}}

## Context

- **Endeavour:** {{.EndeavourName}}
{{- end}}
{{- if .HasTasks}}

## Linked Tasks

| # | Title | Status | Assignee | Elapsed |
|---|-------|--------|----------|---------|
{{- range .Tasks}}
| {{.Num}} | {{.Title}} | {{.Status}} | {{.AssigneeName}} | {{.ElapsedDays}} |
{{- end}}

### Task Summary

- **Total:** {{.TaskTotal}}
- **Planned:** {{.TaskPlanned}}
- **Active:** {{.TaskActive}}
- **Done:** {{.TaskDone}}
- **Canceled:** {{.TaskCanceled}}
{{- end}}
{{- if .CanceledReason}}

## Cancellation

**Reason:** {{.CanceledReason}}
{{- end}}
{{- if .HasChanges}}

## Activity Log

| # | Date | Actor | Action | Fields |
|---|------|-------|--------|--------|
{{- range .Changes}}
| {{.Num}} | {{.Date}} | {{.Actor}} | {{.Action}} | {{.Fields}} |
{{- end}}
{{- end}}`

var seedEndeavourTemplate = `# Endeavour Report: {{.Name}}

**Generated:** {{.Generated}}

## Overview

| Field | Value |
|-------|-------|
| ID | {{.ID}} |
| Status | {{.Status}} |
| Timezone | {{.Timezone}} |
{{- if .StartDate}}
| Start Date | {{.StartDate}} |
{{- end}}
{{- if .EndDate}}
| End Date | {{.EndDate}} |
{{- end}}
| Created | {{.CreatedAt}} |
{{- if .CompletedAt}}
| Completed | {{.CompletedAt}} |
{{- end}}
| Updated | {{.UpdatedAt}} |
{{- if .ElapsedDays}}
| Elapsed | {{.ElapsedDays}} days |
{{- end}}
{{- if .Description}}

## Description

{{.Description}}
{{- end}}
{{- if .HasProgress}}

## Task Progress

- **Total:** {{.TotalTasks}}
- **Planned:** {{.PlannedTasks}}
- **Active:** {{.ActiveTasks}}
- **Done:** {{.DoneTasks}}
- **Canceled:** {{.CanceledTasks}}
- **Completion:** {{.CompletionPct}}%
{{- end}}
{{- if .HasGoals}}

## Goals

| # | Goal | Status |
|---|------|--------|
{{- range .Goals}}
| {{.Num}} | {{.Title}} | {{.Status}} |
{{- end}}

### Goal Summary

- **Total:** {{.GoalTotal}}
- **Open:** {{.GoalOpen}}
- **Achieved:** {{.GoalAchieved}}
- **Abandoned:** {{.GoalAbandoned}}
{{- end}}
{{- if .HasDemands}}

## Demands

| # | Title | Status | Type | Priority |
|---|-------|--------|------|----------|
{{- range .Demands}}
| {{.Num}} | {{.Title}} | {{.Status}} | {{.Type}} | {{.Priority}} |
{{- end}}

### Demand Summary

- **Total:** {{.DemandTotal}}
- **Open:** {{.DemandOpen}}
- **In Progress:** {{.DemandInProgress}}
- **Fulfilled:** {{.DemandFulfilled}}
- **Canceled:** {{.DemandCanceled}}
{{- end}}
{{- if .HasTasks}}

## Tasks

| # | Title | Status | Assignee | Elapsed |
|---|-------|--------|----------|---------|
{{- range .Tasks}}
| {{.Num}} | {{.Title}} | {{.Status}} | {{.AssigneeName}} | {{.ElapsedDays}} |
{{- end}}
{{- end}}
{{- if .ArchivedReason}}

## Archived

**Reason:** {{.ArchivedReason}}
{{- end}}
{{- if .AgeDays}}

## Time Metrics

| Metric | Value |
|--------|-------|
| Age | {{.AgeDays}} days |
{{- if .DaysSinceUpdate}}
| Days Since Update | {{.DaysSinceUpdate}} days |
{{- end}}
{{- end}}
{{- if .HasContributors}}

## Contributors

| # | Name | Tasks Done | Tasks Active | Changes |
|---|------|------------|--------------|---------|
{{- range .Contributors}}
| {{.Num}} | {{.Name}} | {{.TasksDone}} | {{.TasksActive}} | {{.ChangesCount}} |
{{- end}}
{{- end}}
{{- if .HasRecentChanges}}

## Recent activity

| # | Date | Actor | Action | Entity type | Entity | Fields |
|---|------|-------|--------|-------------|--------|--------|
{{- range .RecentChanges}}
| {{.Num}} | {{.Date}} | {{.Actor}} | {{.Action}} | {{.EntityType}} | {{.EntityName}} | {{.Fields}} |
{{- end}}
{{- end}}
{{- if .HasTrend}}

## Burndown ({{.TrendPeriod}})

| Date | Total | Done | Remaining | Completion |
|------|-------|------|-----------|------------|
{{- range .Trend}}
| {{.Date}} | {{.TasksTotal}} | {{.TasksDone}} | {{sub .TasksTotal .TasksDone}} | {{pct .TasksDone .TasksTotal}} |
{{- end}}
{{- end}}
{{- if .HasRituals}}

## Ritual Execution

| # | Ritual | Runs | OK | Failed | Skipped | Success Rate | Last Run |
|---|--------|------|----|--------|---------|--------------|----------|
{{- range .Rituals}}
| {{.Num}} | {{.RitualName}} | {{.TotalRuns}} | {{.Succeeded}} | {{.Failed}} | {{.Skipped}} | {{.SuccessRate}} | {{.LastRunDate}} |
{{- end}}
{{- end}}`

var seedProjectTemplate = `# Project Summary: {{.Name}}

**Generated:** {{.Generated}} | **Status:** {{.Status}} | **Age:** {{.AgeDays}} days

## Progress

| Metric | Value |
|--------|-------|
| Tasks | {{.TotalTasks}} ({{.CompletionPct}}% complete) |
| Demands | {{.TotalDemands}} ({{.FulfillmentPct}}% fulfilled) |
{{- if .HasGoals}}
| Goals | {{.GoalTotal}} ({{.GoalAchievedPct}}% achieved) |
{{- end}}
{{- if .HasActivity}}

## Activity ({{.TimelinePeriod}})

| Date | Created | Updated | Completed |
|------|---------|---------|-----------|
{{- range .ActivityDays}}
| {{.Date}} | {{.Created}} | {{.Updated}} | {{.Completed}} |
{{- end}}

**Totals:** {{.TotalCreated}} created, {{.TotalUpdated}} updated, {{.TotalCompleted}} completed
{{- end}}
{{- if .HasDemands}}

## Demand Fulfillment

| # | Demand | Type | Priority | Status | Progress |
|---|--------|------|----------|--------|----------|
{{- range .Demands}}
| {{.Num}} | {{.Title}} | {{.Type}} | {{.Priority}} | {{.Status}} | {{.TaskDone}}/{{.TaskTotal}} ({{.CompletionPct}}%) |
{{- end}}
{{- end}}
{{- if .HasContributors}}

## Contributors

| # | Name | Tasks Done | Tasks Active | Changes |
|---|------|------------|--------------|---------|
{{- range .Contributors}}
| {{.Num}} | {{.Name}} | {{.TasksDone}} | {{.TasksActive}} | {{.ChangesCount}} |
{{- end}}
{{- end}}
{{- if .HasGoals}}

## Goals

| # | Goal | Status |
|---|------|--------|
{{- range .Goals}}
| {{.Num}} | {{.Title}} | {{.Status}} |
{{- end}}
{{- end}}
{{- if .HasOverdueTasks}}

## Overdue Tasks

| # | Title | Status | Assignee |
|---|-------|--------|----------|
{{- range .OverdueTasks}}
| {{.Num}} | {{.Title}} | {{.Status}} | {{.AssigneeName}} |
{{- end}}
{{- end}}
{{- if .HasStaleTasks}}

## Stale Tasks (no updates in 7+ days)

| # | Title | Status | Assignee |
|---|-------|--------|----------|
{{- range .StaleTasks}}
| {{.Num}} | {{.Title}} | {{.Status}} | {{.AssigneeName}} |
{{- end}}
{{- end}}
{{- if .HasTrend}}

## Burndown ({{.TrendPeriod}})

| Date | Total | Done | Completion | Changes | Overdue |
|------|-------|------|------------|---------|---------|
{{- range .Trend}}
| {{.Date}} | {{.TasksTotal}} | {{.TasksDone}} | {{pct .TasksDone .TasksTotal}} | {{.ChangesCount}} | {{.OverdueCount}} |
{{- end}}
{{- end}}
{{- if .HasRituals}}

## Ritual Execution

| # | Ritual | Runs | OK | Failed | Skipped | Success Rate | Last Run |
|---|--------|------|----|--------|---------|--------------|----------|
{{- range .Rituals}}
| {{.Num}} | {{.RitualName}} | {{.TotalRuns}} | {{.Succeeded}} | {{.Failed}} | {{.Skipped}} | {{.SuccessRate}} | {{.LastRunDate}} |
{{- end}}
{{- end}}`

// Stats holds counts of main entities for the system statistics overview.
type Stats struct {
	Organizations int
	Users         int
	MasterAdmins  int
	Endeavours    int
	Demands       int
	Tasks         int
	Resources     int
	Artifacts     int
	Rituals       int
	RitualRuns    int
	Relations     int
}

// GetStats returns current system statistics.
func (db *DB) GetStats() (*Stats, error) {
	stats := &Stats{}

	queries := []struct {
		query string
		dest  *int
	}{
		{"SELECT COUNT(*) FROM organization WHERE status = 'active'", &stats.Organizations},
		{"SELECT COUNT(*) FROM user WHERE status = 'active'", &stats.Users},
		{"SELECT COUNT(*) FROM user WHERE status = 'active' AND is_admin = 1", &stats.MasterAdmins},
		{"SELECT COUNT(*) FROM endeavour WHERE status = 'active'", &stats.Endeavours},
		{"SELECT COUNT(*) FROM demand WHERE status NOT IN ('canceled', 'fulfilled')", &stats.Demands},
		{"SELECT COUNT(*) FROM task WHERE status IN ('planned', 'active')", &stats.Tasks},
		{"SELECT COUNT(*) FROM resource WHERE status = 'active'", &stats.Resources},
		{"SELECT COUNT(*) FROM artifact WHERE status = 'active'", &stats.Artifacts},
		{"SELECT COUNT(*) FROM ritual WHERE status = 'active'", &stats.Rituals},
		{"SELECT COUNT(*) FROM ritual_run", &stats.RitualRuns},
		{"SELECT COUNT(*) FROM entity_relation", &stats.Relations},
	}

	for _, q := range queries {
		if err := db.QueryRow(q.query).Scan(q.dest); err != nil {
			// Table might not exist yet, default to 0
			*q.dest = 0
		}
	}

	return stats, nil
}

// GetPolicy returns the value for a policy key.
func (db *DB) GetPolicy(key string) (string, error) {
	var value string
	err := db.QueryRow(`SELECT value FROM policy WHERE key = ?`, key).Scan(&value)
	if err != nil {
		return "", fmt.Errorf("get policy %q: %w", key, err)
	}
	return value, nil
}

// ListPoliciesByPrefix returns all policies whose key starts with the given prefix.
func (db *DB) ListPoliciesByPrefix(prefix string) (map[string]string, error) {
	rows, err := db.Query(`SELECT key, value FROM policy WHERE key LIKE ?`, prefix+"%")
	if err != nil {
		return nil, fmt.Errorf("list policies by prefix %q: %w", prefix, err)
	}
	defer rows.Close() //nolint:errcheck
	result := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err == nil {
			result[k] = v
		}
	}
	return result, nil
}

// SetPolicy upserts a policy key-value pair.
func (db *DB) SetPolicy(key, value string) error {
	_, err := db.Exec(
		`INSERT INTO policy (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		key, value,
	)
	if err != nil {
		return fmt.Errorf("set policy %q: %w", key, err)
	}
	return nil
}

// DeletePolicy removes a policy key.
func (db *DB) DeletePolicy(key string) error {
	_, err := db.Exec(`DELETE FROM policy WHERE key = ?`, key)
	if err != nil {
		return fmt.Errorf("delete policy %q: %w", key, err)
	}
	return nil
}

// Close closes the database connection.
func (db *DB) Close() error {
	return db.DB.Close()
}
