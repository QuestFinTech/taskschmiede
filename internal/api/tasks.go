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
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/QuestFinTech/taskschmiede/internal/auth"
	"github.com/QuestFinTech/taskschmiede/internal/security"
	"github.com/QuestFinTech/taskschmiede/internal/storage"
)

// --- Exported business logic methods ---

// CreateTask creates a new task and returns its map representation.
// Both REST handlers and MCP handlers call this method.
func (a *API) CreateTask(ctx context.Context, title, description, endeavourID, demandID, assigneeID string, estimate *float64, dueDate *time.Time, metadata map[string]interface{}) (map[string]interface{}, *APIError) {
	if apiErr := validateTaskCreate(title, description, endeavourID, demandID, assigneeID, metadata); apiErr != nil {
		return nil, apiErr
	}
	if apiErr := a.CheckCreationVelocity(ctx); apiErr != nil {
		return nil, apiErr
	}
	// RBAC: require write access to target endeavour
	if endeavourID != "" {
		scope, apiErr := a.resolveScope(ctx)
		if apiErr != nil {
			return nil, apiErr
		}
		if apiErr := checkEndeavourWrite(scope, endeavourID); apiErr != nil {
			return nil, apiErr
		}
	}
	metadata = scoreAndAnnotate(metadata, title, description)
	metadata = a.applyOrgAlertTerms(metadata, endeavourID, title, description)
	creatorID := a.resolveCallerResourceIDSilent(ctx)
	task, err := a.tskSvc.Create(ctx, title, description, endeavourID, demandID, assigneeID, creatorID, estimate, dueDate, metadata)
	if err != nil {
		if errors.Is(err, storage.ErrEndeavourNotFound) {
			return nil, errNotFound("endeavour", "Endeavour not found")
		}
		return nil, errInvalidInput(err.Error())
	}
	if user := auth.GetAuthUser(ctx); user != nil {
		a.logEntityChange(user.UserID, "create", "task", task.ID, endeavourID, nil, nil)
	}
	return taskToMap(task), nil
}

// GetTask retrieves a single task by ID.
func (a *API) GetTask(ctx context.Context, id string) (map[string]interface{}, *APIError) {
	task, err := a.tskSvc.Get(ctx, id)
	if err != nil {
		if errors.Is(err, storage.ErrTaskNotFound) {
			return nil, errNotFound("task", "Task not found")
		}
		return nil, errInternal("Failed to get task")
	}
	// RBAC: require read access to task's endeavour
	scope, apiErr := a.resolveScope(ctx)
	if apiErr != nil {
		return nil, apiErr
	}
	if apiErr := checkEndeavourRead(scope, task.EndeavourID); apiErr != nil {
		return nil, errNotFound("task", "Task not found")
	}
	return taskToMap(task), nil
}

// ListTasks returns a paginated list of tasks matching the given options.
// The caller is responsible for setting EndeavourIDs (scope filtering)
// and resolving the "me" shorthand in AssigneeID before calling.
func (a *API) ListTasks(ctx context.Context, opts storage.ListTasksOpts) ([]map[string]interface{}, int, *APIError) {
	tasks, total, err := a.tskSvc.List(ctx, opts)
	if err != nil {
		return nil, 0, errInternal("Failed to query tasks")
	}

	items := make([]map[string]interface{}, 0, len(tasks))
	for _, t := range tasks {
		items = append(items, taskToMap(t))
	}
	return items, total, nil
}

// TaskSummary returns status counts for tasks matching the given options.
func (a *API) TaskSummary(ctx context.Context, opts storage.ListTasksOpts) (map[string]interface{}, *APIError) {
	progress, total, err := a.tskSvc.StatusCounts(ctx, opts)
	if err != nil {
		return nil, errInternal("Failed to query tasks")
	}
	return map[string]interface{}{
		"total":    total,
		"planned":  progress.Planned,
		"active":   progress.Active,
		"done":     progress.Done,
		"canceled": progress.Canceled,
	}, nil
}

// UpdateTask applies partial updates to a task and returns the updated map.
func (a *API) UpdateTask(ctx context.Context, id string, fields storage.UpdateTaskFields) (map[string]interface{}, *APIError) {
	if apiErr := validateTaskUpdate(id, fields); apiErr != nil {
		return nil, apiErr
	}
	// RBAC: fetch task to check access
	task, err := a.tskSvc.Get(ctx, id)
	if err != nil {
		if errors.Is(err, storage.ErrTaskNotFound) {
			return nil, errNotFound("task", "Task not found")
		}
		return nil, errInternal("Failed to get task")
	}
	scope, apiErr := a.resolveScope(ctx)
	if apiErr != nil {
		return nil, apiErr
	}
	if fields.Status != nil && *fields.Status == "canceled" {
		// Cancel requires admin OR assignee OR creator OR owner (or team member of owner).
		// First check basic write access (if no access at all, return not_found).
		if apiErr := checkEndeavourWrite(scope, task.EndeavourID); apiErr != nil {
			return nil, errNotFound("task", "Task not found")
		}
		// Then check the elevated cancel permission.
		callerResID := a.resolveCallerResourceIDSilent(ctx)
		isAdmin := checkEndeavourAdmin(scope, task.EndeavourID) == nil
		isAssignee := task.AssigneeID != "" && task.AssigneeID == callerResID
		isCreator := task.CreatorID != "" && task.CreatorID == callerResID
		isOwner := task.OwnerID != "" && task.OwnerID == callerResID
		isOwnerTeamMember := task.OwnerID != "" && !isOwner && a.isTeamMember(task.OwnerID, callerResID)
		if !isAssignee && !isCreator && !isOwner && !isOwnerTeamMember {
			if !isAdmin {
				return nil, errForbidden("Canceling a task requires admin role, or being the assignee, creator, or owner")
			}
		}
		// Quorum enforcement: if the task is team-owned and the team requires
		// quorum for cancel, check that enough team members have approved.
		// Endeavour admins bypass quorum.
		if !isAdmin {
			if team := a.getTeamResource(task.OwnerID); team != nil {
				if required := teamQuorumRequired(team, "cancel"); required >= 2 {
					met, current, needed, err := a.checkQuorum(ctx, "task", id, team, "cancel", callerResID)
					if err != nil {
						return nil, errInternal("Failed to check quorum")
					}
					if !met {
						return nil, errQuorumNotMet("cancel", current, needed)
					}
				}
			}
		}
	} else {
		if apiErr := checkEndeavourWrite(scope, task.EndeavourID); apiErr != nil {
			return nil, errNotFound("task", "Task not found")
		}
	}
	// Cancel reason enforcement: require a reason when canceling.
	if fields.Status != nil && *fields.Status == "canceled" {
		if fields.CanceledReason == nil || strings.TrimSpace(*fields.CanceledReason) == "" {
			return nil, &APIError{
				Code:    "invalid_input",
				Message: "canceled_reason is required when canceling a task",
				Status:  http.StatusBadRequest,
				Details: map[string]interface{}{
					"hint": `Example: {"id": "tsk_...", "status": "canceled", "canceled_reason": "No longer needed"}`,
				},
			}
		}
	}

	// Score updated text fields for injection signals.
	if fields.Title != nil || fields.Description != nil {
		fields.Metadata = scoreAndAnnotate(fields.Metadata, derefStr(fields.Title), derefStr(fields.Description))
	}

	// DoD enforcement: check conditions when transitioning to "done".
	if fields.Status != nil && *fields.Status == "done" {
		resourceID := a.resolveCallerResourceIDSilent(ctx)
		checkResult, checkErr := a.dodSvc.Check(ctx, id, resourceID)
		if checkErr == nil && checkResult != nil && checkResult.Result == "not_met" {
			// Check for override
			task, _ := a.tskSvc.Get(ctx, id)
			if task == nil || !a.dodSvc.HasOverride(task) {
				return nil, &APIError{
					Code:    "dod_not_met",
					Message: checkResult.Hint,
					Status:  http.StatusPreconditionFailed,
					Details: checkResultToMap(checkResult),
				}
			}
		}
	}

	updatedFields, err := a.tskSvc.Update(ctx, id, fields)
	if err != nil {
		if errors.Is(err, storage.ErrTaskNotFound) {
			return nil, errNotFound("task", "Task not found")
		}
		if strings.Contains(err.Error(), "invalid status transition") {
			return nil, errInvalidTransition(err.Error())
		}
		return nil, errInvalidInput(err.Error())
	}
	action := "update"
	if fields.Status != nil && *fields.Status == "canceled" {
		action = "cancel"
	}
	if user := auth.GetAuthUser(ctx); user != nil {
		a.logEntityChange(user.UserID, action, "task", id, task.EndeavourID, updatedFields, taskFieldValues(fields, updatedFields))
	}

	// Return updated task
	task, err = a.tskSvc.Get(ctx, id)
	if err != nil {
		return nil, errInternal("Failed to get updated task")
	}
	return taskToMap(task), nil
}

// --- HTTP handlers (thin wrappers) ---

func (a *API) handleTaskCreate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Title       string                 `json:"title"`
		Description string                 `json:"description"`
		EndeavourID string                 `json:"endeavour_id"`
		AssigneeID  string                 `json:"assignee_id"`
		DemandID    string                 `json:"demand_id"`
		Estimate    *float64               `json:"estimate"`
		DueDate     *string                `json:"due_date"`
		Metadata    map[string]interface{} `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Invalid JSON body")
		return
	}

	var dueDate *time.Time
	if body.DueDate != nil && *body.DueDate != "" {
		t, err := time.Parse(time.RFC3339, *body.DueDate)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_input", "Invalid due_date format (use ISO 8601)")
			return
		}
		dueDate = &t
	}

	result, apiErr := a.CreateTask(r.Context(), sanitize(body.Title), sanitize(body.Description), sanitize(body.EndeavourID), sanitize(body.DemandID), sanitize(body.AssigneeID), body.Estimate, dueDate, security.SanitizeMap(body.Metadata))
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	writeData(w, http.StatusCreated, result)
}

func (a *API) handleTaskList(w http.ResponseWriter, r *http.Request) {
	assigneeID := a.ResolveAssigneeMe(r.Context(), queryString(r, "assignee_id"))

	opts := storage.ListTasksOpts{
		Status:       queryString(r, "status"),
		EndeavourID:  queryString(r, "endeavour_id"),
		EndeavourIDs: a.resolveEndeavourIDs(r),
		AssigneeID:   assigneeID,
		Unassigned:   queryString(r, "unassigned") == "true",
		DemandID:     queryString(r, "demand_id"),
		Search:       queryString(r, "search"),
		Limit:        queryInt(r, "limit", 50),
		Offset:       queryInt(r, "offset", 0),
	}

	// Summary mode
	if queryString(r, "summary") == "true" {
		result, apiErr := a.TaskSummary(r.Context(), opts)
		if apiErr != nil {
			writeAPIError(w, apiErr)
			return
		}
		writeData(w, http.StatusOK, result)
		return
	}

	items, total, apiErr := a.ListTasks(r.Context(), opts)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	writeList(w, items, total, opts.Limit, opts.Offset)
}

func (a *API) handleTaskGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	result, apiErr := a.GetTask(r.Context(), id)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	writeData(w, http.StatusOK, result)
}

func (a *API) handleTaskUpdate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var body struct {
		Title          *string                `json:"title"`
		Description    *string                `json:"description"`
		Status         *string                `json:"status"`
		EndeavourID    *string                `json:"endeavour_id"`
		AssigneeID     *string                `json:"assignee_id"`
		OwnerID        *string                `json:"owner_id"`
		Estimate       *float64               `json:"estimate"`
		Actual         *float64               `json:"actual"`
		DueDate        *string                `json:"due_date"`
		Metadata       map[string]interface{} `json:"metadata"`
		CanceledReason *string                `json:"canceled_reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Invalid JSON body")
		return
	}

	fields := storage.UpdateTaskFields{
		Title:          sanitizePtr(body.Title),
		Description:    sanitizePtr(body.Description),
		Status:         body.Status,
		EndeavourID:    sanitizePtr(body.EndeavourID),
		AssigneeID:     sanitizePtr(body.AssigneeID),
		OwnerID:        sanitizePtr(body.OwnerID),
		Estimate:       body.Estimate,
		Actual:         body.Actual,
		DueDate:        body.DueDate,
		Metadata:       security.SanitizeMap(body.Metadata),
		CanceledReason: sanitizePtr(body.CanceledReason),
	}

	result, apiErr := a.UpdateTask(r.Context(), id, fields)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	writeData(w, http.StatusOK, result)
}

// resolveCallerResourceIDSilent returns the caller's resource ID, or "" if unavailable.
// Unlike resolveCallerResourceID, it never returns an error.
func (a *API) resolveCallerResourceIDSilent(ctx context.Context) string {
	resID, _ := a.resolveCallerResourceID(ctx)
	return resID
}

// taskToMap converts a Task to a JSON-friendly map.
func taskToMap(t *storage.Task) map[string]interface{} {
	m := map[string]interface{}{
		"id":          t.ID,
		"title":       t.Title,
		"description": t.Description,
		"status":      t.Status,
		"metadata":    t.Metadata,
		"created_at":  t.CreatedAt.Format(time.RFC3339),
		"updated_at":  t.UpdatedAt.Format(time.RFC3339),
	}
	if t.EndeavourID != "" {
		m["endeavour_id"] = t.EndeavourID
	}
	if t.EndeavourName != "" {
		m["endeavour_name"] = t.EndeavourName
	}
	if t.DemandID != "" {
		m["demand_id"] = t.DemandID
	}
	if t.AssigneeID != "" {
		m["assignee_id"] = t.AssigneeID
	}
	if t.AssigneeName != "" {
		m["assignee_name"] = t.AssigneeName
	}
	if t.CreatorID != "" {
		m["creator_id"] = t.CreatorID
	}
	if t.CreatorName != "" {
		m["creator_name"] = t.CreatorName
	}
	if t.OwnerID != "" {
		m["owner_id"] = t.OwnerID
	}
	if t.OwnerName != "" {
		m["owner_name"] = t.OwnerName
	}
	if t.Estimate != nil {
		m["estimate"] = *t.Estimate
	}
	if t.Actual != nil {
		m["actual"] = *t.Actual
	}
	if t.DueDate != nil {
		m["due_date"] = t.DueDate.Format(time.RFC3339)
	}
	if t.StartedAt != nil {
		m["started_at"] = t.StartedAt.Format(time.RFC3339)
	}
	if t.CompletedAt != nil {
		m["completed_at"] = t.CompletedAt.Format(time.RFC3339)
	}
	if t.CanceledAt != nil {
		m["canceled_at"] = t.CanceledAt.Format(time.RFC3339)
	}
	if t.CanceledReason != "" {
		m["canceled_reason"] = t.CanceledReason
	}
	return m
}
