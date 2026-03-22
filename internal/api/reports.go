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


package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/QuestFinTech/taskschmiede/internal/storage"
	"github.com/QuestFinTech/taskschmiede/internal/timefmt"
)

// GenerateReport produces a Markdown report for the given scope and entity.
// Supported scopes: "task", "demand", "endeavour".
// The report language is determined by the parent endeavour's lang setting.
func (a *API) GenerateReport(ctx context.Context, scope, entityID string) (map[string]interface{}, *APIError) {
	switch scope {
	case "task":
		return a.generateTaskReport(ctx, entityID)
	case "demand":
		return a.generateDemandReport(ctx, entityID)
	case "endeavour":
		return a.generateEndeavourReport(ctx, entityID)
	case "project":
		return a.generateProjectReport(ctx, entityID)
	default:
		return nil, errInvalidInput("Invalid report scope: must be task, demand, endeavour, or project")
	}
}

func (a *API) generateTaskReport(ctx context.Context, id string) (map[string]interface{}, *APIError) {
	taskMap, apiErr := a.GetTask(ctx, id)
	if apiErr != nil {
		return nil, apiErr
	}

	edvID := mapStr(taskMap, "endeavour_id")
	tz := a.resolveEndeavourTimezone(edvID)
	lang := a.resolveEndeavourLang(edvID)
	title := mapStr(taskMap, "title")

	// Try DB template first.
	data := a.buildTaskReportData(taskMap, tz)
	if md, ok := a.tryTemplateReport("task", lang, data, tz); ok {
		return map[string]interface{}{
			"scope":        "task",
			"entity_id":    id,
			"title":        title,
			"markdown":     md,
			"generated_at": storage.UTCNow().Format(time.RFC3339),
		}, nil
	}

	// Fallback: hardcoded i18n-based generation.
	fd := fmtDateTZ(tz)
	t := a.reportT(lang)

	var b strings.Builder

	fmt.Fprintf(&b, "# %s\n\n", t("report.task.heading", title))
	fmt.Fprintf(&b, "**%s:** %s\n\n", t("report.generated"), fd(storage.UTCNow().Format(time.RFC3339)))

	// Overview table
	fmt.Fprintf(&b, "## %s\n\n", t("report.section.overview"))
	fmt.Fprintf(&b, "| %s | %s |\n|-------|-------|\n", t("report.table.field"), t("report.table.value"))
	writeRow(&b, t("report.field.id"), mapStr(taskMap, "id"))
	writeRow(&b, t("report.field.status"), mapStr(taskMap, "status"))
	writeRow(&b, t("report.field.assignee"), nameOrID(taskMap, "assignee_name", "assignee_id"))
	writeRow(&b, t("report.field.owner"), nameOrID(taskMap, "owner_name", "owner_id"))
	writeRow(&b, t("report.field.creator"), nameOrID(taskMap, "creator_name", "creator_id"))
	if v := mapStr(taskMap, "due_date"); v != "" {
		writeRow(&b, t("report.field.due_date"), fd(v))
	}
	if v, ok := taskMap["estimate"]; ok {
		writeRow(&b, t("report.field.estimate"), fmt.Sprintf("%.1fh", v))
	}
	if v, ok := taskMap["actual"]; ok {
		writeRow(&b, t("report.field.actual"), fmt.Sprintf("%.1fh", v))
	}
	writeRow(&b, t("report.field.created"), fd(mapStr(taskMap, "created_at")))
	writeRow(&b, t("report.field.updated"), fd(mapStr(taskMap, "updated_at")))
	if v := mapStr(taskMap, "started_at"); v != "" {
		writeRow(&b, t("report.field.started"), fd(v))
	}
	if v := mapStr(taskMap, "completed_at"); v != "" {
		writeRow(&b, t("report.field.completed"), fd(v))
	}
	if v := mapStr(taskMap, "canceled_at"); v != "" {
		writeRow(&b, t("report.field.canceled"), fd(v))
	}
	b.WriteString("\n")

	// Description
	if desc := mapStr(taskMap, "description"); desc != "" {
		fmt.Fprintf(&b, "## %s\n\n%s\n\n", t("report.section.description"), desc)
	}

	// Context
	edvName := mapStr(taskMap, "endeavour_name")
	if edvName != "" {
		fmt.Fprintf(&b, "## %s\n\n- **%s:** %s\n", t("report.section.context"), t("report.label.endeavour"), edvName)
	}
	if v := mapStr(taskMap, "canceled_reason"); v != "" {
		fmt.Fprintf(&b, "\n## %s\n\n**%s:** %s\n", t("report.section.cancellation"), t("report.label.reason"), v)
	}
	b.WriteString("\n")

	return map[string]interface{}{
		"scope":        "task",
		"entity_id":    id,
		"title":        title,
		"markdown":     b.String(),
		"generated_at": storage.UTCNow().Format(time.RFC3339),
	}, nil
}

func (a *API) generateDemandReport(ctx context.Context, id string) (map[string]interface{}, *APIError) {
	demandMap, apiErr := a.GetDemand(ctx, id)
	if apiErr != nil {
		return nil, apiErr
	}

	edvID := mapStr(demandMap, "endeavour_id")
	tz := a.resolveEndeavourTimezone(edvID)
	lang := a.resolveEndeavourLang(edvID)
	title := mapStr(demandMap, "title")

	// Try DB template first.
	data := a.buildDemandReportData(ctx, demandMap, id, tz)
	if md, ok := a.tryTemplateReport("demand", lang, data, tz); ok {
		return map[string]interface{}{
			"scope":        "demand",
			"entity_id":    id,
			"title":        title,
			"markdown":     md,
			"generated_at": storage.UTCNow().Format(time.RFC3339),
		}, nil
	}

	// Fallback: hardcoded i18n-based generation.
	fd := fmtDateTZ(tz)
	t := a.reportT(lang)

	var b strings.Builder

	fmt.Fprintf(&b, "# %s\n\n", t("report.demand.heading", title))
	fmt.Fprintf(&b, "**%s:** %s\n\n", t("report.generated"), fd(storage.UTCNow().Format(time.RFC3339)))

	// Overview table
	fmt.Fprintf(&b, "## %s\n\n", t("report.section.overview"))
	fmt.Fprintf(&b, "| %s | %s |\n|-------|-------|\n", t("report.table.field"), t("report.table.value"))
	writeRow(&b, t("report.field.id"), mapStr(demandMap, "id"))
	writeRow(&b, t("report.field.status"), mapStr(demandMap, "status"))
	writeRow(&b, t("report.field.type"), mapStr(demandMap, "type"))
	writeRow(&b, t("report.field.priority"), mapStr(demandMap, "priority"))
	writeRow(&b, t("report.field.owner"), nameOrID(demandMap, "owner_name", "owner_id"))
	writeRow(&b, t("report.field.creator"), nameOrID(demandMap, "creator_name", "creator_id"))
	if v := mapStr(demandMap, "due_date"); v != "" {
		writeRow(&b, t("report.field.due_date"), fd(v))
	}
	writeRow(&b, t("report.field.created"), fd(mapStr(demandMap, "created_at")))
	writeRow(&b, t("report.field.updated"), fd(mapStr(demandMap, "updated_at")))
	b.WriteString("\n")

	// Description
	if desc := mapStr(demandMap, "description"); desc != "" {
		fmt.Fprintf(&b, "## %s\n\n%s\n\n", t("report.section.description"), desc)
	}

	// Context
	edvName := mapStr(demandMap, "endeavour_name")
	if edvName != "" {
		fmt.Fprintf(&b, "## %s\n\n- **%s:** %s\n\n", t("report.section.context"), t("report.label.endeavour"), edvName)
	}

	// Linked tasks (reuse from data struct to avoid double query)
	if data.HasTasks {
		fmt.Fprintf(&b, "## %s\n\n", t("report.section.linked_tasks"))
		fmt.Fprintf(&b, "| # | %s | %s | %s |\n|---|-------|--------|----------|\n",
			t("report.field.title"), t("report.field.status"), t("report.field.assignee"))
		for _, tk := range data.Tasks {
			fmt.Fprintf(&b, "| %d | %s | %s | %s |\n",
				tk.Num, tk.Title, tk.Status, tk.AssigneeName)
		}
		b.WriteString("\n")

		fmt.Fprintf(&b, "### %s\n\n", t("report.section.task_summary"))
		fmt.Fprintf(&b, "- **%s:** %v\n", t("report.summary.total"), data.TaskTotal)
		fmt.Fprintf(&b, "- **%s:** %v\n", t("report.summary.planned"), data.TaskPlanned)
		fmt.Fprintf(&b, "- **%s:** %v\n", t("report.summary.active"), data.TaskActive)
		fmt.Fprintf(&b, "- **%s:** %v\n", t("report.summary.done"), data.TaskDone)
		fmt.Fprintf(&b, "- **%s:** %v\n", t("report.summary.canceled"), data.TaskCanceled)
		b.WriteString("\n")
	}

	if v := mapStr(demandMap, "canceled_reason"); v != "" {
		fmt.Fprintf(&b, "## %s\n\n**%s:** %s\n\n", t("report.section.cancellation"), t("report.label.reason"), v)
	}

	return map[string]interface{}{
		"scope":        "demand",
		"entity_id":    id,
		"title":        title,
		"markdown":     b.String(),
		"generated_at": storage.UTCNow().Format(time.RFC3339),
	}, nil
}

func (a *API) generateEndeavourReport(ctx context.Context, id string) (map[string]interface{}, *APIError) {
	edvMap, apiErr := a.GetEndeavour(ctx, id)
	if apiErr != nil {
		return nil, apiErr
	}

	tz := mapStr(edvMap, "timezone")
	if tz == "" {
		tz = "UTC"
	}
	lang := mapStr(edvMap, "lang")
	if lang == "" {
		lang = "en"
	}
	name := mapStr(edvMap, "name")

	// Try DB template first.
	data := a.buildEndeavourReportData(ctx, edvMap, id, tz)
	if md, ok := a.tryTemplateReport("endeavour", lang, data, tz); ok {
		return map[string]interface{}{
			"scope":        "endeavour",
			"entity_id":    id,
			"title":        name,
			"markdown":     md,
			"generated_at": storage.UTCNow().Format(time.RFC3339),
		}, nil
	}

	// Fallback: hardcoded i18n-based generation.
	fd := fmtDateTZ(tz)
	t := a.reportT(lang)

	var b strings.Builder

	fmt.Fprintf(&b, "# %s\n\n", t("report.endeavour.heading", name))
	fmt.Fprintf(&b, "**%s:** %s\n\n", t("report.generated"), fd(storage.UTCNow().Format(time.RFC3339)))

	// Overview table
	fmt.Fprintf(&b, "## %s\n\n", t("report.section.overview"))
	fmt.Fprintf(&b, "| %s | %s |\n|-------|-------|\n", t("report.table.field"), t("report.table.value"))
	writeRow(&b, t("report.field.id"), mapStr(edvMap, "id"))
	writeRow(&b, t("report.field.status"), mapStr(edvMap, "status"))
	writeRow(&b, t("report.field.timezone"), tz)
	if v := mapStr(edvMap, "start_date"); v != "" {
		writeRow(&b, t("report.field.start_date"), fd(v))
	}
	if v := mapStr(edvMap, "end_date"); v != "" {
		writeRow(&b, t("report.field.end_date"), fd(v))
	}
	writeRow(&b, t("report.field.created"), fd(mapStr(edvMap, "created_at")))
	if v := mapStr(edvMap, "completed_at"); v != "" {
		writeRow(&b, t("report.field.completed"), fd(v))
	}
	writeRow(&b, t("report.field.updated"), fd(mapStr(edvMap, "updated_at")))
	b.WriteString("\n")

	// Description
	if desc := mapStr(edvMap, "description"); desc != "" {
		fmt.Fprintf(&b, "## %s\n\n%s\n\n", t("report.section.description"), desc)
	}

	// Task progress (reuse from data struct)
	if data.HasProgress {
		fmt.Fprintf(&b, "## %s\n\n", t("report.section.task_progress"))
		fmt.Fprintf(&b, "- **%s:** %d\n", t("report.summary.total"), data.TotalTasks)
		fmt.Fprintf(&b, "- **%s:** %v\n", t("report.summary.planned"), data.PlannedTasks)
		fmt.Fprintf(&b, "- **%s:** %v\n", t("report.summary.active"), data.ActiveTasks)
		fmt.Fprintf(&b, "- **%s:** %v\n", t("report.summary.done"), data.DoneTasks)
		fmt.Fprintf(&b, "- **%s:** %v\n", t("report.summary.canceled"), data.CanceledTasks)
		if data.TotalTasks > 0 {
			fmt.Fprintf(&b, "- **%s:** %d%%\n", t("report.summary.completion"), data.CompletionPct)
		}
		b.WriteString("\n")
	}

	// Goals (reuse from data struct)
	if data.HasGoals {
		fmt.Fprintf(&b, "## %s\n\n", t("report.section.goals"))
		fmt.Fprintf(&b, "| # | %s | %s |\n|---|------|--------|\n",
			t("report.section.goals"), t("report.field.status"))
		for _, g := range data.Goals {
			fmt.Fprintf(&b, "| %d | %s | %s |\n", g.Num, g.Title, g.Status)
		}
		fmt.Fprintf(&b, "\n### %s\n\n", t("report.section.goal_summary"))
		fmt.Fprintf(&b, "- **%s:** %d\n", t("report.summary.total"), data.GoalTotal)
		fmt.Fprintf(&b, "- **%s:** %d\n", t("report.summary.open"), data.GoalOpen)
		fmt.Fprintf(&b, "- **%s:** %d\n", t("report.summary.achieved"), data.GoalAchieved)
		fmt.Fprintf(&b, "- **%s:** %d\n", t("report.summary.abandoned"), data.GoalAbandoned)
		b.WriteString("\n")
	}

	// Demands (reuse from data struct)
	if data.HasDemands {
		fmt.Fprintf(&b, "## %s\n\n", t("report.section.demands"))
		fmt.Fprintf(&b, "| # | %s | %s | %s | %s |\n|---|-------|--------|------|----------|\n",
			t("report.field.title"), t("report.field.status"), t("report.field.type"), t("report.field.priority"))
		for _, d := range data.Demands {
			fmt.Fprintf(&b, "| %d | %s | %s | %s | %s |\n",
				d.Num, d.Title, d.Status, d.Type, d.Priority)
		}
		fmt.Fprintf(&b, "\n### %s\n\n", t("report.section.demand_summary"))
		fmt.Fprintf(&b, "- **%s:** %d\n", t("report.summary.total"), data.DemandTotal)
		fmt.Fprintf(&b, "- **%s:** %d\n", t("report.summary.open"), data.DemandOpen)
		fmt.Fprintf(&b, "- **%s:** %d\n", t("report.summary.in_progress"), data.DemandInProgress)
		fmt.Fprintf(&b, "- **%s:** %d\n", t("report.summary.fulfilled"), data.DemandFulfilled)
		fmt.Fprintf(&b, "- **%s:** %d\n", t("report.summary.canceled"), data.DemandCanceled)
		b.WriteString("\n")
	}

	// Tasks (reuse from data struct)
	if data.HasTasks {
		fmt.Fprintf(&b, "## %s\n\n", t("report.section.tasks"))
		fmt.Fprintf(&b, "| # | %s | %s | %s |\n|---|-------|--------|----------|\n",
			t("report.field.title"), t("report.field.status"), t("report.field.assignee"))
		for _, tk := range data.Tasks {
			fmt.Fprintf(&b, "| %d | %s | %s | %s |\n",
				tk.Num, tk.Title, tk.Status, tk.AssigneeName)
		}
		b.WriteString("\n")
	}

	if v := mapStr(edvMap, "archived_reason"); v != "" {
		fmt.Fprintf(&b, "## %s\n\n**%s:** %s\n\n", t("report.section.archived"), t("report.label.reason"), v)
	}

	return map[string]interface{}{
		"scope":        "endeavour",
		"entity_id":    id,
		"title":        name,
		"markdown":     b.String(),
		"generated_at": storage.UTCNow().Format(time.RFC3339),
	}, nil
}

func (a *API) generateProjectReport(ctx context.Context, id string) (map[string]interface{}, *APIError) {
	edvMap, apiErr := a.GetEndeavour(ctx, id)
	if apiErr != nil {
		return nil, apiErr
	}

	tz := mapStr(edvMap, "timezone")
	if tz == "" {
		tz = "UTC"
	}
	lang := mapStr(edvMap, "lang")
	if lang == "" {
		lang = "en"
	}
	name := mapStr(edvMap, "name")

	data := a.buildProjectReportData(ctx, edvMap, id, tz)
	if md, ok := a.tryTemplateReport("project", lang, data, tz); ok {
		return map[string]interface{}{
			"scope":        "project",
			"entity_id":    id,
			"title":        name,
			"markdown":     md,
			"generated_at": storage.UTCNow().Format(time.RFC3339),
		}, nil
	}

	// No template found: return a minimal fallback.
	return nil, errInternal("No project report template found")
}

// --- REST handler ---

func (a *API) handleReportGenerate(w http.ResponseWriter, r *http.Request) {
	scope := r.PathValue("scope")
	id := r.PathValue("id")

	result, apiErr := a.GenerateReport(r.Context(), scope, id)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	writeData(w, http.StatusOK, result)
}

func (a *API) handleReportEmail(w http.ResponseWriter, r *http.Request) {
	scope := r.PathValue("scope")
	id := r.PathValue("id")

	result, apiErr := a.GenerateReport(r.Context(), scope, id)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	recipientID, apiErr := a.resolveCallerResourceID(r.Context())
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	if a.msgSvc == nil {
		writeAPIError(w, errInternal("Messaging is not configured"))
		return
	}

	title, _ := result["title"].(string)
	markdown, _ := result["markdown"].(string)

	// Resolve the report language for the email subject.
	lang := a.resolveReportLang(scope, id)
	subject := a.reportT(lang)("report.email_subject", title)

	_, err := a.msgSvc.Send(r.Context(), "sys_taskschmiede", subject, markdown, "info", "",
		"", "", []string{recipientID}, "", "", nil)
	if err != nil {
		writeAPIError(w, errInternal(err.Error()))
		return
	}

	writeData(w, http.StatusOK, map[string]interface{}{"status": "sent"})
}

// --- Helpers ---

func mapStr(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func nameOrID(m map[string]interface{}, nameKey, idKey string) string {
	if n := mapStr(m, nameKey); n != "" {
		return n
	}
	if id := mapStr(m, idKey); id != "" {
		return id
	}
	return "-"
}

func writeRow(b *strings.Builder, label, value string) {
	if value == "" {
		value = "-"
	}
	b.WriteString("| ")
	b.WriteString(label)
	b.WriteString(" | ")
	b.WriteString(value)
	b.WriteString(" |\n")
}

// fmtDateTZ returns a date-formatting closure that uses the given IANA timezone.
// The returned string always includes the timezone abbreviation.
func fmtDateTZ(tz string) func(string) string {
	return func(rfc3339 string) string {
		if rfc3339 == "" {
			return "-"
		}
		s := timefmt.FormatDateTime(rfc3339, tz)
		if s == "" {
			return rfc3339
		}
		// Append timezone abbreviation
		t, err := time.Parse(time.RFC3339, rfc3339)
		if err != nil {
			return s
		}
		loc := time.UTC
		if tz != "" && tz != "UTC" {
			if l, err := time.LoadLocation(tz); err == nil {
				loc = l
			}
		}
		abbr, _ := t.In(loc).Zone()
		return s + " " + abbr
	}
}

// reportT returns a translation closure for the given language.
// If the i18n bundle is not available, it returns the key or formats with args.
func (a *API) reportT(lang string) func(string, ...interface{}) string {
	return func(key string, args ...interface{}) string {
		if a.i18n != nil {
			return a.i18n.T(lang, key, args...)
		}
		// Fallback: return key with basic formatting
		if len(args) > 0 {
			return fmt.Sprintf(key, args...)
		}
		return key
	}
}

// resolveEndeavourTimezone looks up the timezone for an endeavour by ID.
// Returns "UTC" if the endeavour cannot be found or has no timezone set.
func (a *API) resolveEndeavourTimezone(endeavourID string) string {
	if endeavourID == "" {
		return "UTC"
	}
	edv, err := a.db.GetEndeavour(endeavourID)
	if err != nil || edv.Timezone == "" {
		return "UTC"
	}
	return edv.Timezone
}

// resolveEndeavourLang looks up the language for an endeavour by ID.
// Returns "en" if the endeavour cannot be found or has no lang set.
func (a *API) resolveEndeavourLang(endeavourID string) string {
	if endeavourID == "" {
		return "en"
	}
	edv, err := a.db.GetEndeavour(endeavourID)
	if err != nil || edv.Lang == "" {
		return "en"
	}
	return edv.Lang
}

// resolveReportLang determines the language for a report based on scope and entity.
func (a *API) resolveReportLang(scope, entityID string) string {
	switch scope {
	case "endeavour", "project":
		return a.resolveEndeavourLang(entityID)
	case "task":
		task, err := a.db.GetTask(entityID)
		if err == nil {
			return a.resolveEndeavourLang(task.EndeavourID)
		}
	case "demand":
		demand, err := a.db.GetDemand(entityID)
		if err == nil {
			return a.resolveEndeavourLang(demand.EndeavourID)
		}
	}
	return "en"
}
