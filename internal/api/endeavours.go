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
	"time"

	"github.com/QuestFinTech/taskschmiede/internal/auth"
	"github.com/QuestFinTech/taskschmiede/internal/security"
	"github.com/QuestFinTech/taskschmiede/internal/storage"
)

// CreateEndeavour creates a new endeavour and returns its map representation.
// The creating user is automatically linked as owner.
func (a *API) CreateEndeavour(ctx context.Context, name, description string, goals []storage.Goal, startDate, endDate *time.Time, metadata map[string]interface{}) (map[string]interface{}, *APIError) {
	if apiErr := validateEndeavourCreate(name, description, goals, metadata); apiErr != nil {
		return nil, apiErr
	}
	// Tier enforcement: check both per-org and per-user limits.
	authUser := auth.GetAuthUser(ctx)
	if authUser != nil {
		orgID := a.findUserOrgID(ctx, authUser.UserID)
		if orgID != "" {
			if apiErr := a.CheckOrgQuota(ctx, orgID, "endeavours_per_org"); apiErr != nil {
				return nil, apiErr
			}
		}
		// Active endeavour limit (user-level, across all orgs).
		if apiErr := a.CheckTierLimit(ctx, "active_endeavours"); apiErr != nil {
			return nil, apiErr
		}
	}

	metadata = scoreAndAnnotate(metadata, name, description)
	edv, err := a.edvSvc.Create(ctx, name, description, goals, startDate, endDate, metadata)
	if err != nil {
		return nil, errInvalidInput(err.Error())
	}

	// Auto-link creator as owner
	if authUser := auth.GetAuthUser(ctx); authUser != nil {
		if lerr := a.edvSvc.AddUser(ctx, authUser.UserID, edv.ID, "owner"); lerr != nil {
			a.logger.Warn("Failed to auto-link endeavour creator as owner", "edv_id", edv.ID, "error", lerr)
		}
		// Auto-link endeavour to creator's org (for per-org quota counting).
		if orgID := a.findUserOrgID(ctx, authUser.UserID); orgID != "" {
			if lerr := a.orgSvc.AddEndeavour(ctx, orgID, edv.ID, "participant"); lerr != nil {
				a.logger.Warn("Failed to auto-link endeavour to org", "edv_id", edv.ID, "org_id", orgID, "error", lerr)
			}
		}
		a.logEntityChange(authUser.UserID, "create", "endeavour", edv.ID, edv.ID, nil, nil)
	}

	return endeavourToMap(edv, nil), nil
}

// GetEndeavour retrieves an endeavour by ID with task progress and returns its map representation.
func (a *API) GetEndeavour(ctx context.Context, id string) (map[string]interface{}, *APIError) {
	// RBAC: require read access
	scope, apiErr := a.resolveScope(ctx)
	if apiErr != nil {
		return nil, apiErr
	}
	if apiErr := checkEndeavourRead(scope, id); apiErr != nil {
		return nil, errNotFound("endeavour", "Endeavour not found")
	}
	edv, progress, err := a.edvSvc.Get(ctx, id)
	if err != nil {
		if errors.Is(err, storage.ErrEndeavourNotFound) {
			return nil, errNotFound("endeavour", "Endeavour not found")
		}
		return nil, errInternal("Failed to get endeavour")
	}
	return endeavourToMap(edv, progress), nil
}

// ListEndeavours queries endeavours with the given options and returns a list of map representations.
func (a *API) ListEndeavours(ctx context.Context, opts storage.ListEndeavoursOpts) ([]map[string]interface{}, int, *APIError) {
	edvs, total, err := a.edvSvc.List(ctx, opts)
	if err != nil {
		return nil, 0, errInternal("Failed to query endeavours")
	}

	items := make([]map[string]interface{}, 0, len(edvs))
	for _, e := range edvs {
		items = append(items, endeavourToMap(e, nil))
	}
	return items, total, nil
}

// UpdateEndeavour applies partial updates to an endeavour and returns the updated map representation.
func (a *API) UpdateEndeavour(ctx context.Context, id string, fields storage.UpdateEndeavourFields) (map[string]interface{}, *APIError) {
	if apiErr := validateEndeavourUpdate(id, fields); apiErr != nil {
		return nil, apiErr
	}
	// RBAC: require admin
	scope, apiErr := a.resolveScope(ctx)
	if apiErr != nil {
		return nil, apiErr
	}
	if apiErr := checkEndeavourAdmin(scope, id); apiErr != nil {
		return nil, errNotFound("endeavour", "Endeavour not found")
	}
	if fields.Name != nil || fields.Description != nil {
		fields.Metadata = scoreAndAnnotate(fields.Metadata, derefStr(fields.Name), derefStr(fields.Description))
	}
	updatedFields, err := a.edvSvc.Update(ctx, id, fields)
	if err != nil {
		if errors.Is(err, storage.ErrEndeavourNotFound) {
			return nil, errNotFound("endeavour", "Endeavour not found")
		}
		return nil, errInvalidInput(err.Error())
	}
	if user := auth.GetAuthUser(ctx); user != nil {
		a.logEntityChange(user.UserID, "update", "endeavour", id, id, updatedFields, endeavourFieldValues(fields, updatedFields))
	}

	// Auto-complete check: trigger when goals are updated.
	if fields.Goals != nil {
		a.tryAutoComplete(ctx, id)
	}

	edv, progress, err := a.edvSvc.Get(ctx, id)
	if err != nil {
		return nil, errInternal("Failed to get updated endeavour")
	}
	return endeavourToMap(edv, progress), nil
}

// tryAutoComplete checks whether an endeavour meets all auto-completion
// conditions and, if so, transitions it to completed. Best-effort: errors
// are logged but do not propagate.
func (a *API) tryAutoComplete(ctx context.Context, endeavourID string) {
	ready, err := a.db.CheckAutoComplete(endeavourID)
	if err != nil {
		a.logger.Warn("Auto-complete check failed", "endeavour_id", endeavourID, "error", err)
		return
	}
	if !ready {
		return
	}

	completed := "completed"
	_, err = a.edvSvc.Update(ctx, endeavourID, storage.UpdateEndeavourFields{Status: &completed})
	if err != nil {
		a.logger.Warn("Auto-complete update failed", "endeavour_id", endeavourID, "error", err)
		return
	}

	if user := auth.GetAuthUser(ctx); user != nil {
		a.logEntityChange(user.UserID, "update", "endeavour", endeavourID, endeavourID, []string{"status"}, map[string]interface{}{"status": "completed"})
	}
	a.logger.Info("Endeavour auto-completed", "endeavour_id", endeavourID)
}

// EndeavourArchiveImpact returns a dry-run impact summary for archiving an endeavour.
func (a *API) EndeavourArchiveImpact(ctx context.Context, id string) (map[string]interface{}, *APIError) {
	// RBAC: require owner
	scope, apiErr := a.resolveScope(ctx)
	if apiErr != nil {
		return nil, apiErr
	}
	if apiErr := checkEndeavourOwner(scope, id); apiErr != nil {
		return nil, errNotFound("endeavour", "Endeavour not found")
	}

	impact, err := a.edvSvc.ArchiveImpact(ctx, id)
	if err != nil {
		if errors.Is(err, storage.ErrEndeavourNotFound) {
			return nil, errNotFound("endeavour", "Endeavour not found")
		}
		return nil, errInvalidInput(err.Error())
	}

	return map[string]interface{}{
		"endeavour_id":    id,
		"planned_tasks":   impact.PlannedTasks,
		"active_tasks":    impact.ActiveTasks,
		"tasks_to_cancel": impact.TasksToCancel,
		"done_tasks":      impact.DoneTasks,
		"canceled_tasks":  impact.CanceledTasks,
	}, nil
}

// ArchiveEndeavour archives an endeavour (cancels non-terminal tasks and sets status to archived).
func (a *API) ArchiveEndeavour(ctx context.Context, id, reason string) (map[string]interface{}, *APIError) {
	if reason == "" {
		return nil, errInvalidInput("reason is required")
	}
	// RBAC: require owner
	scope, apiErr := a.resolveScope(ctx)
	if apiErr != nil {
		return nil, apiErr
	}
	if apiErr := checkEndeavourOwner(scope, id); apiErr != nil {
		return nil, errNotFound("endeavour", "Endeavour not found")
	}

	canceled, err := a.edvSvc.Archive(ctx, id, reason)
	if err != nil {
		if errors.Is(err, storage.ErrEndeavourNotFound) {
			return nil, errNotFound("endeavour", "Endeavour not found")
		}
		return nil, errInvalidInput(err.Error())
	}

	if user := auth.GetAuthUser(ctx); user != nil {
		a.logEntityChange(user.UserID, "archive", "endeavour", id, id, nil, nil)
	}

	edv, progress, _ := a.edvSvc.Get(ctx, id)
	if edv != nil {
		result := endeavourToMap(edv, progress)
		result["tasks_canceled"] = canceled
		return result, nil
	}
	return map[string]interface{}{
		"id":              id,
		"status":          "archived",
		"tasks_canceled":  canceled,
	}, nil
}

// --- HTTP handlers (thin wrappers) ---

func (a *API) handleEndeavourCreate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name        string                 `json:"name"`
		Description string                 `json:"description"`
		Goals       json.RawMessage        `json:"goals"`
		StartDate   *string                `json:"start_date"`
		EndDate     *string                `json:"end_date"`
		Metadata    map[string]interface{} `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Invalid JSON body")
		return
	}

	goals := parseGoalsJSON(body.Goals)

	var startDate, endDate *time.Time
	if body.StartDate != nil && *body.StartDate != "" {
		t, err := time.Parse(time.RFC3339, *body.StartDate)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_input", "Invalid start_date format")
			return
		}
		startDate = &t
	}
	if body.EndDate != nil && *body.EndDate != "" {
		t, err := time.Parse(time.RFC3339, *body.EndDate)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_input", "Invalid end_date format")
			return
		}
		endDate = &t
	}

	result, apiErr := a.CreateEndeavour(r.Context(), sanitize(body.Name), sanitize(body.Description), goals, startDate, endDate, security.SanitizeMap(body.Metadata))
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	writeData(w, http.StatusCreated, result)
}

func (a *API) handleEndeavourList(w http.ResponseWriter, r *http.Request) {
	opts := storage.ListEndeavoursOpts{
		Status:         queryString(r, "status"),
		OrganizationID: queryString(r, "organization_id"),
		Search:         queryString(r, "search"),
		Limit:          queryInt(r, "limit", 50),
		Offset:         queryInt(r, "offset", 0),
		EndeavourIDs:   a.resolveEndeavourIDs(r),
	}

	items, total, apiErr := a.ListEndeavours(r.Context(), opts)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	writeList(w, items, total, opts.Limit, opts.Offset)
}

func (a *API) handleEndeavourGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	result, apiErr := a.GetEndeavour(r.Context(), id)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	writeData(w, http.StatusOK, result)
}

func (a *API) handleEndeavourUpdate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var body struct {
		Name               *string                `json:"name"`
		Description        *string                `json:"description"`
		Status             *string                `json:"status"`
		Timezone           *string                `json:"timezone"`
		Lang               *string                `json:"lang"`
		Goals              json.RawMessage        `json:"goals"`
		StartDate          *string                `json:"start_date"`
		EndDate            *string                `json:"end_date"`
		TaskschmiedEnabled *bool                  `json:"taskschmied_enabled"`
		Metadata           map[string]interface{} `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Invalid JSON body")
		return
	}

	fields := storage.UpdateEndeavourFields{
		Name:               sanitizePtr(body.Name),
		Description:        sanitizePtr(body.Description),
		Status:             body.Status,
		Timezone:           body.Timezone,
		Lang:               body.Lang,
		StartDate:          body.StartDate,
		EndDate:            body.EndDate,
		TaskschmiedEnabled: body.TaskschmiedEnabled,
		Metadata:           security.SanitizeMap(body.Metadata),
	}
	if body.Goals != nil {
		fields.Goals = parseGoalsJSON(body.Goals)
		if fields.Goals == nil {
			fields.Goals = []storage.Goal{} // explicit empty = clear
		}
	}

	result, apiErr := a.UpdateEndeavour(r.Context(), id, fields)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	writeData(w, http.StatusOK, result)
}

// parseGoalsJSON parses a JSON goals field that can be either an array of
// strings (legacy) or an array of Goal objects (new format).
func parseGoalsJSON(raw json.RawMessage) []storage.Goal {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}

	// Try new format first (array of objects)
	var goals []storage.Goal
	if err := json.Unmarshal(raw, &goals); err == nil && len(goals) > 0 && goals[0].Title != "" {
		// Sanitize
		for i := range goals {
			goals[i].Title = security.SanitizeInput(goals[i].Title)
			goals[i].LinkedEntityType = security.SanitizeInput(goals[i].LinkedEntityType)
			goals[i].LinkedEntityID = security.SanitizeInput(goals[i].LinkedEntityID)
		}
		return goals
	}

	// Fall back to legacy string array
	var strings []string
	if err := json.Unmarshal(raw, &strings); err == nil {
		goals = make([]storage.Goal, len(strings))
		for i, s := range strings {
			goals[i] = storage.Goal{
				Title: security.SanitizeInput(s),
			}
		}
		return goals
	}

	return nil
}

func (a *API) handleEndeavourArchiveImpact(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	result, apiErr := a.EndeavourArchiveImpact(r.Context(), id)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	writeData(w, http.StatusOK, result)
}

func (a *API) handleEndeavourArchive(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var body struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Invalid JSON body")
		return
	}

	result, apiErr := a.ArchiveEndeavour(r.Context(), id, sanitize(body.Reason))
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	writeData(w, http.StatusOK, result)
}

func endeavourToMap(e *storage.Endeavour, progress *storage.TaskProgress) map[string]interface{} {
	m := map[string]interface{}{
		"id":                   e.ID,
		"name":                 e.Name,
		"description":          e.Description,
		"status":               e.Status,
		"timezone":             e.Timezone,
		"lang":                 e.Lang,
		"goals":                e.Goals,
		"taskschmied_enabled":  e.TaskschmiedEnabled,
		"metadata":             e.Metadata,
		"created_at":           e.CreatedAt.Format(time.RFC3339),
		"updated_at":           e.UpdatedAt.Format(time.RFC3339),
	}
	if e.StartDate != nil {
		m["start_date"] = e.StartDate.Format(time.RFC3339)
	}
	if e.EndDate != nil {
		m["end_date"] = e.EndDate.Format(time.RFC3339)
	}
	if e.CompletedAt != nil {
		m["completed_at"] = e.CompletedAt.Format(time.RFC3339)
	}
	if e.ArchivedAt != nil {
		m["archived_at"] = e.ArchivedAt.Format(time.RFC3339)
	}
	if e.ArchivedReason != "" {
		m["archived_reason"] = e.ArchivedReason
	}
	if progress != nil {
		m["progress"] = map[string]interface{}{
			"planned":  progress.Planned,
			"active":   progress.Active,
			"done":     progress.Done,
			"canceled": progress.Canceled,
		}
	}
	return m
}
