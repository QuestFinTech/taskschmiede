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
	"bytes"
	"context"
	"fmt"
	"strings"
	"text/template"
	"time"

	"github.com/QuestFinTech/taskschmiede/internal/storage"
	"github.com/QuestFinTech/taskschmiede/internal/timefmt"
)

// TaskReportData holds data for task report template execution.
type TaskReportData struct {
	ID             string
	Title          string
	Status         string
	Description    string
	AssigneeName   string
	OwnerName      string
	CreatorName    string
	DueDate        string
	Estimate       string
	Actual         string
	CreatedAt      string
	UpdatedAt      string
	StartedAt      string
	CompletedAt    string
	CanceledAt     string
	CanceledReason string
	EndeavourName  string
	Generated      string
	LeadTimeDays   string
	CycleTimeDays  string
	Changes        []ChangeRow
	HasChanges     bool
	ChangeCount    int
	Comments       []CommentRow
	HasComments    bool
	CommentCount   int
}

// DemandReportData holds data for demand report template execution.
type DemandReportData struct {
	ID             string
	Title          string
	Status         string
	Type           string
	Priority       string
	Description    string
	OwnerName      string
	CreatorName    string
	DueDate        string
	CreatedAt      string
	UpdatedAt      string
	CanceledReason string
	EndeavourName  string
	Generated      string
	Tasks          []TaskRow
	TaskTotal      int
	TaskPlanned    int
	TaskActive     int
	TaskDone       int
	TaskCanceled   int
	HasTasks       bool
	Changes        []ChangeRow
	HasChanges     bool
	ChangeCount    int
}

// TaskRow holds a single task row for report tables.
type TaskRow struct {
	Num          int
	Title        string
	Status       string
	AssigneeName string
	ElapsedDays  string
}

// EndeavourReportData holds data for endeavour report template execution.
type EndeavourReportData struct {
	ID              string
	Name            string
	Status          string
	Description     string
	Timezone        string
	StartDate       string
	EndDate         string
	CreatedAt       string
	UpdatedAt       string
	CompletedAt     string
	ArchivedReason  string
	Generated       string
	TotalTasks      int
	PlannedTasks    int
	ActiveTasks     int
	DoneTasks       int
	CanceledTasks   int
	CompletionPct   int
	HasProgress     bool
	Goals           []GoalRow
	HasGoals        bool
	GoalTotal       int
	GoalOpen        int
	GoalAchieved    int
	GoalAbandoned   int
	Demands         []DemandRow
	HasDemands      bool
	DemandTotal     int
	DemandOpen      int
	DemandInProgress int
	DemandFulfilled int
	DemandCanceled  int
	Tasks             []TaskRow
	HasTasks          bool
	TaskTotal         int
	ElapsedDays       string
	AgeDays           string
	DaysSinceUpdate   string
	Contributors      []ContributorRow
	HasContributors   bool
	ContributorCount  int
	RecentChanges     []ChangeRow
	HasRecentChanges  bool
	RecentChangeCount int
	Trend             []TrendPoint
	HasTrend          bool
	TrendPeriod       string
	Rituals           []RitualSummaryRow
	HasRituals        bool
}

// GoalRow holds a single goal row for report tables.
type GoalRow struct {
	Num    int
	Title  string
	Status string
}

// DemandRow holds a single demand row for report tables.
type DemandRow struct {
	Num      int
	Title    string
	Status   string
	Type     string
	Priority string
}

// ChangeRow holds a single entity change row for report tables.
type ChangeRow struct {
	Num        int
	Date       string
	Actor      string
	Action     string
	EntityType string
	EntityName string
	Fields     string
}

// formatFieldsWithValues formats field names with their new values from metadata.
// e.g., ["status", "actual"] with {"status": "done"} becomes "status: done, actual".
func formatFieldsWithValues(fields []string, metadata map[string]interface{}) string {
	if len(fields) == 0 {
		return ""
	}
	parts := make([]string, len(fields))
	for i, f := range fields {
		if v, ok := metadata[f]; ok {
			parts[i] = fmt.Sprintf("%s: %v", f, v)
		} else {
			parts[i] = f
		}
	}
	return strings.Join(parts, ", ")
}

// CommentRow holds a single comment row for report tables.
type CommentRow struct {
	Num     int
	Date    string
	Author  string
	Content string
}

// ContributorRow holds a single contributor row for report tables.
type ContributorRow struct {
	Num          int
	Name         string
	TasksDone    int
	TasksActive  int
	ChangesCount int
}

// TrendPoint holds a single data point in a burndown/trend table.
type TrendPoint struct {
	Date           string
	TasksTotal     int
	TasksDone      int
	DemandsTotal   int
	DemandsFulfilled int
	ChangesCount   int
	OverdueCount   int
	CompletionPct  int
}

// RitualSummaryRow holds aggregated run stats for a single ritual.
type RitualSummaryRow struct {
	Num         int
	RitualName  string
	TotalRuns   int
	Succeeded   int
	Failed      int
	Skipped     int
	SuccessRate string
	LastRunDate string
}

// ActivityDayRow holds a single day's activity for project reports.
type ActivityDayRow struct {
	Date      string
	Created   int
	Updated   int
	Completed int
}

// DemandSummaryRow holds a single demand's fulfillment status for project reports.
type DemandSummaryRow struct {
	Num           int
	Title         string
	Type          string
	Priority      string
	Status        string
	TaskTotal     int
	TaskDone      int
	CompletionPct int
}

// ProjectReportData holds data for project summary report template execution.
type ProjectReportData struct {
	ID, Name, Status, Description, Timezone string
	StartDate, EndDate, Generated           string

	// High-level metrics
	AgeDays        string
	TotalTasks     int
	CompletionPct  int
	TotalDemands   int
	FulfillmentPct int

	// Activity timeline
	TimelinePeriod string // "Last 14 days"
	ActivityDays   []ActivityDayRow
	HasActivity    bool
	TotalCreated   int
	TotalUpdated   int
	TotalCompleted int

	// Demand fulfillment
	Demands    []DemandSummaryRow
	HasDemands bool

	// Contributor leaderboard
	Contributors    []ContributorRow
	HasContributors bool

	// Goal progress
	Goals           []GoalRow
	HasGoals        bool
	GoalTotal       int
	GoalAchievedPct int

	// Risks / blockers
	OverdueTasks    []TaskRow
	HasOverdueTasks bool
	StaleTasks      []TaskRow
	HasStaleTasks   bool

	// Trend and rituals
	Trend       []TrendPoint
	HasTrend    bool
	TrendPeriod string
	Rituals     []RitualSummaryRow
	HasRituals  bool
}

// tryTemplateReport attempts to generate a report using a DB template.
// Returns the markdown string and true if a template was found and executed successfully.
// Returns empty string and false if no template exists (caller should fall back to hardcoded).
func (a *API) tryTemplateReport(scope, lang string, data interface{}, tz string) (string, bool) {
	tpl, err := a.tplSvc.GetByScope(context.Background(), scope, lang)
	if err != nil {
		return "", false
	}

	funcMap := template.FuncMap{
		"fd": func(rfc3339 string) string {
			if rfc3339 == "" {
				return "-"
			}
			s := timefmt.FormatDateTime(rfc3339, tz)
			if s == "" {
				return rfc3339
			}
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
		},
		"daysBetween": func(start, end string) string {
			if start == "" || end == "" {
				return ""
			}
			s, err1 := time.Parse(time.RFC3339, start)
			e, err2 := time.Parse(time.RFC3339, end)
			if err1 != nil || err2 != nil {
				return ""
			}
			return fmt.Sprintf("%d", int(e.Sub(s).Hours()/24))
		},
		"truncate": func(s string, n int) string {
			if len(s) > n {
				return s[:n] + "..."
			}
			return s
		},
		"join": func(items []string, sep string) string {
			return strings.Join(items, sep)
		},
		"pct": func(part, total int) string {
			if total == 0 {
				return "0%"
			}
			return fmt.Sprintf("%d%%", part*100/total)
		},
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },
		"sparkline": func(values []int) string {
			if len(values) == 0 {
				return ""
			}
			chars := []rune{' ', '.', 'o', 'O', '#'}
			minV, maxV := values[0], values[0]
			for _, v := range values[1:] {
				if v < minV {
					minV = v
				}
				if v > maxV {
					maxV = v
				}
			}
			spread := maxV - minV
			var result []rune
			for _, v := range values {
				idx := 0
				if spread > 0 {
					idx = (v - minV) * (len(chars) - 1) / spread
				}
				result = append(result, chars[idx])
			}
			return string(result)
		},
		"delta": func(current, previous int) string {
			d := current - previous
			if d > 0 {
				return fmt.Sprintf("+%d", d)
			}
			if d < 0 {
				return fmt.Sprintf("%d", d)
			}
			return "0"
		},
		"avgFloat": func(values []float64) string {
			if len(values) == 0 {
				return "0.0"
			}
			sum := 0.0
			for _, v := range values {
				sum += v
			}
			return fmt.Sprintf("%.1f", sum/float64(len(values)))
		},
	}

	tmpl, err := template.New("report").Funcs(funcMap).Parse(tpl.Body)
	if err != nil {
		return "", false
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", false
	}

	return buf.String(), true
}

// buildTaskReportData assembles the data struct for a task report template.
func (a *API) buildTaskReportData(taskMap map[string]interface{}, tz string) TaskReportData {
	fd := fmtDateTZ(tz)
	data := TaskReportData{
		ID:             mapStr(taskMap, "id"),
		Title:          mapStr(taskMap, "title"),
		Status:         mapStr(taskMap, "status"),
		Description:    mapStr(taskMap, "description"),
		AssigneeName:   nameOrID(taskMap, "assignee_name", "assignee_id"),
		OwnerName:      nameOrID(taskMap, "owner_name", "owner_id"),
		CreatorName:    nameOrID(taskMap, "creator_name", "creator_id"),
		DueDate:        fd(mapStr(taskMap, "due_date")),
		CreatedAt:      fd(mapStr(taskMap, "created_at")),
		UpdatedAt:      fd(mapStr(taskMap, "updated_at")),
		StartedAt:      fd(mapStr(taskMap, "started_at")),
		CompletedAt:    fd(mapStr(taskMap, "completed_at")),
		CanceledAt:     fd(mapStr(taskMap, "canceled_at")),
		CanceledReason: mapStr(taskMap, "canceled_reason"),
		EndeavourName:  mapStr(taskMap, "endeavour_name"),
		Generated:      fd(storage.UTCNow().Format(time.RFC3339)),
	}
	if v, ok := taskMap["estimate"]; ok {
		data.Estimate = fmt.Sprintf("%.1fh", v)
	}
	if v, ok := taskMap["actual"]; ok {
		data.Actual = fmt.Sprintf("%.1fh", v)
	}

	// Time metrics
	createdAt := mapStr(taskMap, "created_at")
	startedAt := mapStr(taskMap, "started_at")
	completedAt := mapStr(taskMap, "completed_at")
	if completedAt != "" {
		data.LeadTimeDays = daysBetweenRFC3339(createdAt, completedAt)
		if startedAt != "" {
			data.CycleTimeDays = daysBetweenRFC3339(startedAt, completedAt)
		}
	}

	// Entity changes
	taskID := mapStr(taskMap, "id")
	changes, _, _ := a.db.ListEntityChanges(storage.ListEntityChangesOpts{
		EntityID: taskID,
		Limit:    20,
	})
	if len(changes) > 0 {
		nameCache := map[string]string{}
		data.HasChanges = true
		data.ChangeCount = len(changes)
		for i, ch := range changes {
			data.Changes = append(data.Changes, ChangeRow{
				Num:    i + 1,
				Date:   ch.CreatedAt.Format("2006-01-02 15:04"),
				Actor:  a.resolveActorName(ch.ActorID, nameCache),
				Action: ch.Action,
				Fields: formatFieldsWithValues(ch.Fields, ch.Metadata),
			})
		}
	}

	// Comments
	comments, _, _ := a.db.ListComments(storage.ListCommentsOpts{
		EntityType: "task",
		EntityID:   taskID,
		Limit:      20,
	})
	if len(comments) > 0 {
		data.HasComments = true
		data.CommentCount = len(comments)
		for i, c := range comments {
			content := c.Content
			if len(content) > 200 {
				content = content[:200] + "..."
			}
			data.Comments = append(data.Comments, CommentRow{
				Num:     i + 1,
				Date:    c.CreatedAt.Format("2006-01-02 15:04"),
				Author:  c.AuthorName,
				Content: content,
			})
		}
	}

	return data
}

// buildDemandReportData assembles the data struct for a demand report template.
func (a *API) buildDemandReportData(ctx context.Context, demandMap map[string]interface{}, demandID, tz string) DemandReportData {
	fd := fmtDateTZ(tz)
	data := DemandReportData{
		ID:             mapStr(demandMap, "id"),
		Title:          mapStr(demandMap, "title"),
		Status:         mapStr(demandMap, "status"),
		Type:           mapStr(demandMap, "type"),
		Priority:       mapStr(demandMap, "priority"),
		Description:    mapStr(demandMap, "description"),
		OwnerName:      nameOrID(demandMap, "owner_name", "owner_id"),
		CreatorName:    nameOrID(demandMap, "creator_name", "creator_id"),
		DueDate:        fd(mapStr(demandMap, "due_date")),
		CreatedAt:      fd(mapStr(demandMap, "created_at")),
		UpdatedAt:      fd(mapStr(demandMap, "updated_at")),
		CanceledReason: mapStr(demandMap, "canceled_reason"),
		EndeavourName:  mapStr(demandMap, "endeavour_name"),
		Generated:      fd(storage.UTCNow().Format(time.RFC3339)),
	}

	// Linked tasks
	taskOpts := storage.ListTasksOpts{
		DemandID:     demandID,
		EndeavourIDs: a.ResolveEndeavourIDs(ctx, false),
		Limit:        500,
	}
	tasks, taskTotal, taskErr := a.ListTasks(ctx, taskOpts)
	if taskErr == nil && taskTotal > 0 {
		data.HasTasks = true
		data.TaskTotal = taskTotal
		for i, tk := range tasks {
			data.Tasks = append(data.Tasks, TaskRow{
				Num:          i + 1,
				Title:        mapStr(tk, "title"),
				Status:       mapStr(tk, "status"),
				AssigneeName: nameOrID(tk, "assignee_name", "assignee_id"),
				ElapsedDays:  taskElapsedDays(mapStr(tk, "created_at"), mapStr(tk, "completed_at")),
			})
		}
		summary, _ := a.TaskSummary(ctx, taskOpts)
		if summary != nil {
			data.TaskPlanned = toInt(summary["planned"])
			data.TaskActive = toInt(summary["active"])
			data.TaskDone = toInt(summary["done"])
			data.TaskCanceled = toInt(summary["canceled"])
		}
	}

	// Entity changes
	changes, _, _ := a.db.ListEntityChanges(storage.ListEntityChangesOpts{
		EntityID: demandID,
		Limit:    20,
	})
	if len(changes) > 0 {
		nameCache := map[string]string{}
		data.HasChanges = true
		data.ChangeCount = len(changes)
		for i, ch := range changes {
			data.Changes = append(data.Changes, ChangeRow{
				Num:    i + 1,
				Date:   ch.CreatedAt.Format("2006-01-02 15:04"),
				Actor:  a.resolveActorName(ch.ActorID, nameCache),
				Action: ch.Action,
				Fields: formatFieldsWithValues(ch.Fields, ch.Metadata),
			})
		}
	}

	return data
}

// buildEndeavourReportData assembles the data struct for an endeavour report template.
func (a *API) buildEndeavourReportData(ctx context.Context, edvMap map[string]interface{}, edvID, tz string) EndeavourReportData {
	fd := fmtDateTZ(tz)
	data := EndeavourReportData{
		ID:             mapStr(edvMap, "id"),
		Name:           mapStr(edvMap, "name"),
		Status:         mapStr(edvMap, "status"),
		Description:    mapStr(edvMap, "description"),
		Timezone:       tz,
		StartDate:      fd(mapStr(edvMap, "start_date")),
		EndDate:        fd(mapStr(edvMap, "end_date")),
		CreatedAt:      fd(mapStr(edvMap, "created_at")),
		UpdatedAt:      fd(mapStr(edvMap, "updated_at")),
		CompletedAt:    fd(mapStr(edvMap, "completed_at")),
		ArchivedReason: mapStr(edvMap, "archived_reason"),
		Generated:      fd(storage.UTCNow().Format(time.RFC3339)),
	}

	// Task progress
	if progress, ok := edvMap["progress"].(map[string]interface{}); ok {
		data.HasProgress = true
		data.PlannedTasks = toInt(progress["planned"])
		data.ActiveTasks = toInt(progress["active"])
		data.DoneTasks = toInt(progress["done"])
		data.CanceledTasks = toInt(progress["canceled"])
		data.TotalTasks = data.PlannedTasks + data.ActiveTasks + data.DoneTasks + data.CanceledTasks
		if data.TotalTasks > 0 {
			data.CompletionPct = data.DoneTasks * 100 / data.TotalTasks
		}
	}

	// Goals
	if goals, ok := edvMap["goals"]; ok && goals != nil {
		if goalList, ok := goals.([]storage.Goal); ok && len(goalList) > 0 {
			data.HasGoals = true
			data.GoalTotal = len(goalList)
			for i, g := range goalList {
				data.Goals = append(data.Goals, GoalRow{
					Num:    i + 1,
					Title:  g.Title,
					Status: g.Status,
				})
				switch g.Status {
				case "achieved":
					data.GoalAchieved++
				case "abandoned":
					data.GoalAbandoned++
				default:
					data.GoalOpen++
				}
			}
		}
	}

	// Demands
	edvIDs := a.ResolveEndeavourIDs(ctx, false)
	demandOpts := storage.ListDemandsOpts{
		EndeavourID:  edvID,
		EndeavourIDs: edvIDs,
		Limit:        500,
	}
	demands, demandTotal, dmdErr := a.ListDemands(ctx, demandOpts)
	if dmdErr == nil && demandTotal > 0 {
		data.HasDemands = true
		data.DemandTotal = demandTotal
		for i, d := range demands {
			data.Demands = append(data.Demands, DemandRow{
				Num:      i + 1,
				Title:    mapStr(d, "title"),
				Status:   mapStr(d, "status"),
				Type:     mapStr(d, "type"),
				Priority: mapStr(d, "priority"),
			})
			switch mapStr(d, "status") {
			case "in_progress":
				data.DemandInProgress++
			case "fulfilled":
				data.DemandFulfilled++
			case "canceled":
				data.DemandCanceled++
			default:
				data.DemandOpen++
			}
		}
	}

	// Tasks
	taskOpts := storage.ListTasksOpts{
		EndeavourID:  edvID,
		EndeavourIDs: edvIDs,
		Limit:        500,
	}
	tasks, taskTotal, tskErr := a.ListTasks(ctx, taskOpts)
	if tskErr == nil && taskTotal > 0 {
		data.HasTasks = true
		data.TaskTotal = taskTotal
		for i, tk := range tasks {
			data.Tasks = append(data.Tasks, TaskRow{
				Num:          i + 1,
				Title:        mapStr(tk, "title"),
				Status:       mapStr(tk, "status"),
				AssigneeName: nameOrID(tk, "assignee_name", "assignee_id"),
				ElapsedDays:  taskElapsedDays(mapStr(tk, "created_at"), mapStr(tk, "completed_at")),
			})
		}
	}

	// Time metrics
	now := storage.UTCNow().Format(time.RFC3339)
	createdAt := mapStr(edvMap, "created_at")
	updatedAt := mapStr(edvMap, "updated_at")
	data.AgeDays = daysBetweenRFC3339(createdAt, now)
	data.DaysSinceUpdate = daysBetweenRFC3339(updatedAt, now)
	completedAt := mapStr(edvMap, "completed_at")
	if completedAt != "" {
		data.ElapsedDays = daysBetweenRFC3339(createdAt, completedAt)
	} else {
		data.ElapsedDays = data.AgeDays
	}

	// Contributors
	contributors, contribErr := a.db.ContributorsByEndeavour(edvID)
	if contribErr == nil && len(contributors) > 0 {
		data.HasContributors = true
		data.ContributorCount = len(contributors)
		for i, c := range contributors {
			data.Contributors = append(data.Contributors, ContributorRow{
				Num:          i + 1,
				Name:         c.ActorName,
				TasksDone:    c.TasksDone,
				TasksActive:  c.TasksActive,
				ChangesCount: c.ChangeCount,
			})
		}
	}

	// Recent changes
	recentChanges, _, _ := a.db.ListEntityChanges(storage.ListEntityChangesOpts{
		EndeavourID: edvID,
		Limit:       30,
	})
	if len(recentChanges) > 0 {
		nameCache := map[string]string{}
		entityCache := map[string]string{}
		data.HasRecentChanges = true
		data.RecentChangeCount = len(recentChanges)
		for i, ch := range recentChanges {
			data.RecentChanges = append(data.RecentChanges, ChangeRow{
				Num:        i + 1,
				Date:       ch.CreatedAt.Format("2006-01-02 15:04"),
				Actor:      a.resolveActorName(ch.ActorID, nameCache),
				Action:     ch.Action,
				EntityType: ch.EntityType,
				EntityName: a.resolveEntityName(ch.EntityType, ch.EntityID, entityCache),
				Fields:     formatFieldsWithValues(ch.Fields, ch.Metadata),
			})
		}
	}

	// Trend data
	a.populateTrendData(&data.Trend, &data.HasTrend, &data.TrendPeriod, edvID)

	// Ritual execution summaries
	a.populateRitualSummaries(&data.Rituals, &data.HasRituals, edvID, tz)

	return data
}

// buildProjectReportData assembles the data struct for a project summary report.
func (a *API) buildProjectReportData(ctx context.Context, edvMap map[string]interface{}, edvID, tz string) ProjectReportData {
	fd := fmtDateTZ(tz)
	now := storage.UTCNow()
	nowStr := now.Format(time.RFC3339)

	data := ProjectReportData{
		ID:          mapStr(edvMap, "id"),
		Name:        mapStr(edvMap, "name"),
		Status:      mapStr(edvMap, "status"),
		Description: mapStr(edvMap, "description"),
		Timezone:    tz,
		StartDate:   fd(mapStr(edvMap, "start_date")),
		EndDate:     fd(mapStr(edvMap, "end_date")),
		Generated:   fd(nowStr),
	}

	// Age
	data.AgeDays = daysBetweenRFC3339(mapStr(edvMap, "created_at"), nowStr)

	// Task progress
	if progress, ok := edvMap["progress"].(map[string]interface{}); ok {
		planned := toInt(progress["planned"])
		active := toInt(progress["active"])
		done := toInt(progress["done"])
		canceled := toInt(progress["canceled"])
		data.TotalTasks = planned + active + done + canceled
		if data.TotalTasks > 0 {
			data.CompletionPct = done * 100 / data.TotalTasks
		}
	}

	// Demand fulfillment
	demandRecords, dErr := a.db.DemandFulfillmentByEndeavour(edvID)
	if dErr == nil && len(demandRecords) > 0 {
		data.HasDemands = true
		data.TotalDemands = len(demandRecords)
		fulfilled := 0
		for i, d := range demandRecords {
			pct := 0
			if d.TaskTotal > 0 {
				pct = d.TaskDone * 100 / d.TaskTotal
			}
			data.Demands = append(data.Demands, DemandSummaryRow{
				Num:           i + 1,
				Title:         d.Title,
				Type:          d.Type,
				Priority:      d.Priority,
				Status:        d.Status,
				TaskTotal:     d.TaskTotal,
				TaskDone:      d.TaskDone,
				CompletionPct: pct,
			})
			if d.Status == "fulfilled" {
				fulfilled++
			}
		}
		if data.TotalDemands > 0 {
			data.FulfillmentPct = fulfilled * 100 / data.TotalDemands
		}
	}

	// Goals
	if goals, ok := edvMap["goals"]; ok && goals != nil {
		if goalList, ok := goals.([]storage.Goal); ok && len(goalList) > 0 {
			data.HasGoals = true
			data.GoalTotal = len(goalList)
			achieved := 0
			for i, g := range goalList {
				data.Goals = append(data.Goals, GoalRow{
					Num:    i + 1,
					Title:  g.Title,
					Status: g.Status,
				})
				if g.Status == "achieved" {
					achieved++
				}
			}
			if data.GoalTotal > 0 {
				data.GoalAchievedPct = achieved * 100 / data.GoalTotal
			}
		}
	}

	// Activity timeline (last 14 days)
	since := now.AddDate(0, 0, -14)
	activityRecords, aErr := a.db.DailyActivityByEndeavour(edvID, since)
	if aErr == nil && len(activityRecords) > 0 {
		data.HasActivity = true
		data.TimelinePeriod = "Last 14 days"
		for _, ar := range activityRecords {
			data.ActivityDays = append(data.ActivityDays, ActivityDayRow{
				Date:      ar.Date,
				Created:   ar.Created,
				Updated:   ar.Updated,
				Completed: ar.Completed,
			})
			data.TotalCreated += ar.Created
			data.TotalUpdated += ar.Updated
			data.TotalCompleted += ar.Completed
		}
	}

	// Contributors
	contributors, contribErr := a.db.ContributorsByEndeavour(edvID)
	if contribErr == nil && len(contributors) > 0 {
		data.HasContributors = true
		for i, c := range contributors {
			data.Contributors = append(data.Contributors, ContributorRow{
				Num:          i + 1,
				Name:         c.ActorName,
				TasksDone:    c.TasksDone,
				TasksActive:  c.TasksActive,
				ChangesCount: c.ChangeCount,
			})
		}
	}

	// Overdue tasks
	overdue, odErr := a.db.OverdueTasks(edvID)
	if odErr == nil && len(overdue) > 0 {
		data.HasOverdueTasks = true
		for i, t := range overdue {
			data.OverdueTasks = append(data.OverdueTasks, TaskRow{
				Num:          i + 1,
				Title:        t.Title,
				Status:       t.Status,
				AssigneeName: t.AssigneeName,
			})
		}
	}

	// Stale tasks
	stale, stErr := a.db.StaleTasks(edvID, 7)
	if stErr == nil && len(stale) > 0 {
		data.HasStaleTasks = true
		for i, t := range stale {
			data.StaleTasks = append(data.StaleTasks, TaskRow{
				Num:          i + 1,
				Title:        t.Title,
				Status:       t.Status,
				AssigneeName: t.AssigneeName,
			})
		}
	}

	// Trend data
	a.populateTrendData(&data.Trend, &data.HasTrend, &data.TrendPeriod, edvID)

	// Ritual execution summaries
	a.populateRitualSummaries(&data.Rituals, &data.HasRituals, edvID, tz)

	return data
}

// populateTrendData loads endeavour snapshots and fills trend fields.
// Tries daily snapshots first (last 30 days, needs >= 2). Falls back to weekly (last 90 days, 12 max).
// Results are returned in chronological order (oldest first).
func (a *API) populateTrendData(trend *[]TrendPoint, hasTrend *bool, trendPeriod *string, edvID string) {
	now := storage.UTCNow()

	// Try daily snapshots (last 30 days)
	since30d := now.AddDate(0, 0, -30).Format("2006-01-02")
	dailies, dErr := a.db.ListEndeavourSnapshots(edvID, "daily", since30d, 30)
	if dErr == nil && len(dailies) >= 2 {
		*hasTrend = true
		*trendPeriod = "Daily"
		// Reverse to chronological order (ListEndeavourSnapshots returns newest first)
		for i := len(dailies) - 1; i >= 0; i-- {
			*trend = append(*trend, snapshotToTrendPoint(dailies[i]))
		}
		return
	}

	// Fall back to weekly (last 90 days, max 12)
	since90d := now.AddDate(0, 0, -90).Format("2006-01-02")
	weeklies, wErr := a.db.ListEndeavourSnapshots(edvID, "weekly", since90d, 12)
	if wErr == nil && len(weeklies) >= 2 {
		*hasTrend = true
		*trendPeriod = "Weekly"
		for i := len(weeklies) - 1; i >= 0; i-- {
			*trend = append(*trend, snapshotToTrendPoint(weeklies[i]))
		}
	}
}

// populateRitualSummaries loads ritual run aggregates for an endeavour.
func (a *API) populateRitualSummaries(rituals *[]RitualSummaryRow, hasRituals *bool, edvID, tz string) {
	fd := fmtDateTZ(tz)
	summaries, err := a.db.RitualRunSummaryByEndeavour(edvID)
	if err != nil || len(summaries) == 0 {
		return
	}

	*hasRituals = true
	for i, s := range summaries {
		rate := "0%"
		if s.TotalRuns > 0 {
			rate = fmt.Sprintf("%d%%", s.Succeeded*100/s.TotalRuns)
		}
		lastRun := "-"
		if s.LastRunDate != "" {
			lastRun = fd(s.LastRunDate)
		}
		*rituals = append(*rituals, RitualSummaryRow{
			Num:         i + 1,
			RitualName:  s.RitualName,
			TotalRuns:   s.TotalRuns,
			Succeeded:   s.Succeeded,
			Failed:      s.Failed,
			Skipped:     s.Skipped,
			SuccessRate: rate,
			LastRunDate: lastRun,
		})
	}
}

// snapshotToTrendPoint converts an EndeavourSnapshot into a TrendPoint.
func snapshotToTrendPoint(s *storage.EndeavourSnapshot) TrendPoint {
	m := s.Metrics
	total := snapshotMetricInt(m, "tasks_total")
	done := snapshotMetricInt(m, "tasks_done")
	pct := 0
	if total > 0 {
		pct = done * 100 / total
	}
	return TrendPoint{
		Date:             s.SnapshotDate,
		TasksTotal:       total,
		TasksDone:        done,
		DemandsTotal:     snapshotMetricInt(m, "demands_total"),
		DemandsFulfilled: snapshotMetricInt(m, "demands_fulfilled"),
		ChangesCount:     snapshotMetricInt(m, "entity_changes_count"),
		OverdueCount:     snapshotMetricInt(m, "overdue_count"),
		CompletionPct:    pct,
	}
}

// snapshotMetricInt extracts an integer from a snapshot metrics map,
// handling JSON float64 and int types.
func snapshotMetricInt(m map[string]interface{}, key string) int {
	v, ok := m[key]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	default:
		return 0
	}
}

// resolveActorName resolves a resource ID to a display name, using the cache
// to avoid repeated lookups.
func (a *API) resolveActorName(actorID string, cache map[string]string) string {
	if actorID == "" {
		return "-"
	}
	if name, ok := cache[actorID]; ok {
		return name
	}
	// Try resource first
	res, err := a.db.GetResource(actorID)
	if err == nil && res.Name != "" {
		cache[actorID] = res.Name
		return res.Name
	}
	// Try user (actor may be a user ID, not a resource ID)
	user, err := a.db.GetUser(actorID)
	if err == nil && user.Name != "" {
		cache[actorID] = user.Name
		return user.Name
	}
	cache[actorID] = actorID
	return actorID
}

// resolveEntityName resolves an entity ID to its title/name based on type,
// using the cache to avoid repeated lookups.
func (a *API) resolveEntityName(entityType, entityID string, cache map[string]string) string {
	if entityID == "" {
		return "-"
	}
	if name, ok := cache[entityID]; ok {
		return name
	}
	var name string
	switch entityType {
	case "task":
		if t, err := a.db.GetTask(entityID); err == nil {
			name = t.Title
		}
	case "demand":
		if d, err := a.db.GetDemand(entityID); err == nil {
			name = d.Title
		}
	case "endeavour":
		if e, err := a.db.GetEndeavour(entityID); err == nil {
			name = e.Name
		}
	case "organization":
		if o, err := a.db.GetOrganization(entityID); err == nil {
			name = o.Name
		}
	case "resource":
		if r, err := a.db.GetResource(entityID); err == nil {
			name = r.Name
		}
	}
	if name == "" {
		name = entityID
	}
	cache[entityID] = name
	return name
}

// taskElapsedDays computes elapsed days for a task.
// If completed, returns days between created and completed.
// If still active, returns days between created and now.
func taskElapsedDays(createdAt, completedAt string) string {
	if createdAt == "" {
		return "-"
	}
	end := completedAt
	if end == "" {
		end = storage.UTCNow().Format(time.RFC3339)
	}
	d := daysBetweenRFC3339(createdAt, end)
	if d == "" {
		return "-"
	}
	return d + "d"
}

// daysBetweenRFC3339 computes the number of days between two RFC3339 timestamps.
// Returns "" if either is empty or unparseable.
func daysBetweenRFC3339(start, end string) string {
	if start == "" || end == "" {
		return ""
	}
	s, err1 := time.Parse(time.RFC3339, start)
	e, err2 := time.Parse(time.RFC3339, end)
	if err1 != nil || err2 != nil {
		return ""
	}
	days := int(e.Sub(s).Hours() / 24)
	return fmt.Sprintf("%d", days)
}

