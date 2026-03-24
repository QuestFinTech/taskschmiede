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

// CreateDemand creates a new demand and returns its map representation.
func (a *API) CreateDemand(ctx context.Context, typ, title, description, priority, endeavourID string, dueDate *time.Time, metadata map[string]interface{}) (map[string]interface{}, *APIError) {
	if apiErr := validateDemandCreate(typ, title, description, priority, endeavourID, metadata); apiErr != nil {
		return nil, apiErr
	}
	if apiErr := a.CheckCreationVelocity(ctx); apiErr != nil {
		return nil, apiErr
	}
	// RBAC: require write access to target endeavour
	scope, apiErr := a.resolveScope(ctx)
	if apiErr != nil {
		return nil, apiErr
	}
	if apiErr := checkEndeavourWrite(scope, endeavourID); apiErr != nil {
		return nil, apiErr
	}
	metadata = scoreAndAnnotate(metadata, title, description)
	metadata = a.applyOrgAlertTerms(metadata, endeavourID, title, description)
	creatorID := a.resolveCallerResourceIDSilent(ctx)
	demand, err := a.dmdSvc.Create(ctx, typ, title, description, priority, endeavourID, creatorID, dueDate, metadata)
	if err != nil {
		return nil, errInvalidInput(err.Error())
	}
	if user := auth.GetAuthUser(ctx); user != nil {
		a.logEntityChange(user.UserID, "create", "demand", demand.ID, endeavourID, nil, nil)
	}
	return demandToMap(demand), nil
}

// GetDemand retrieves a demand by ID and returns its map representation.
func (a *API) GetDemand(ctx context.Context, id string) (map[string]interface{}, *APIError) {
	demand, err := a.dmdSvc.Get(ctx, id)
	if err != nil {
		if errors.Is(err, storage.ErrDemandNotFound) {
			return nil, errNotFound("demand", "Demand not found")
		}
		return nil, errInternal("Failed to get demand")
	}
	// RBAC: require read access to demand's endeavour
	scope, apiErr := a.resolveScope(ctx)
	if apiErr != nil {
		return nil, apiErr
	}
	if apiErr := checkEndeavourRead(scope, demand.EndeavourID); apiErr != nil {
		return nil, errNotFound("demand", "Demand not found")
	}
	return demandToMap(demand), nil
}

// ListDemands queries demands with the given options and returns a list of map representations.
func (a *API) ListDemands(ctx context.Context, opts storage.ListDemandsOpts) ([]map[string]interface{}, int, *APIError) {
	demands, total, err := a.dmdSvc.List(ctx, opts)
	if err != nil {
		return nil, 0, errInternal("Failed to query demands")
	}

	items := make([]map[string]interface{}, 0, len(demands))
	for _, d := range demands {
		items = append(items, demandToMap(d))
	}
	return items, total, nil
}

// UpdateDemand applies partial updates to a demand and returns the updated map representation.
func (a *API) UpdateDemand(ctx context.Context, id string, fields storage.UpdateDemandFields) (map[string]interface{}, *APIError) {
	if apiErr := validateDemandUpdate(id, fields); apiErr != nil {
		return nil, apiErr
	}
	// RBAC: fetch demand to check access
	demand, err := a.dmdSvc.Get(ctx, id)
	if err != nil {
		if errors.Is(err, storage.ErrDemandNotFound) {
			return nil, errNotFound("demand", "Demand not found")
		}
		return nil, errInternal("Failed to get demand")
	}
	scope, apiErr := a.resolveScope(ctx)
	if apiErr != nil {
		return nil, apiErr
	}
	isHighStakesAction := fields.Status != nil && (*fields.Status == "canceled" || *fields.Status == "fulfilled")
	if isHighStakesAction {
		action := *fields.Status // "canceled" or "fulfilled"
		quorumAction := action
		if quorumAction == "canceled" {
			quorumAction = "cancel"
		} else {
			quorumAction = "fulfill"
		}
		// Cancel/fulfill requires admin OR creator OR owner (or team member of owner).
		// First check basic write access (if no access at all, return not_found).
		if apiErr := checkEndeavourWrite(scope, demand.EndeavourID); apiErr != nil {
			return nil, errNotFound("demand", "Demand not found")
		}
		// Then check the elevated permission.
		callerResID := a.resolveCallerResourceIDSilent(ctx)
		isAdmin := checkEndeavourAdmin(scope, demand.EndeavourID) == nil
		isCreator := demand.CreatorID != "" && demand.CreatorID == callerResID
		isOwner := demand.OwnerID != "" && demand.OwnerID == callerResID
		isOwnerTeamMember := demand.OwnerID != "" && !isOwner && a.isTeamMember(demand.OwnerID, callerResID)
		if !isCreator && !isOwner && !isOwnerTeamMember {
			if !isAdmin {
				verb := "Canceling"
				if quorumAction == "fulfill" {
					verb = "Fulfilling"
				}
				return nil, errForbidden(verb + " a demand requires admin role, or being the creator or owner")
			}
		}
		// Quorum enforcement: if the demand is team-owned and the team requires
		// quorum for this action, check that enough team members have approved.
		// Endeavour admins bypass quorum.
		if !isAdmin {
			if team := a.getTeamResource(demand.OwnerID); team != nil {
				if required := teamQuorumRequired(team, quorumAction); required >= 2 {
					met, current, needed, err := a.checkQuorum(ctx, "demand", id, team, quorumAction, callerResID)
					if err != nil {
						return nil, errInternal("Failed to check quorum")
					}
					if !met {
						return nil, errQuorumNotMet(quorumAction, current, needed)
					}
				}
			}
		}
	} else {
		if apiErr := checkEndeavourWrite(scope, demand.EndeavourID); apiErr != nil {
			return nil, errNotFound("demand", "Demand not found")
		}
	}
	// Score updated text fields for injection signals.
	if fields.Title != nil || fields.Description != nil {
		fields.Metadata = scoreAndAnnotate(fields.Metadata, derefStr(fields.Title), derefStr(fields.Description))
	}

	// Cancel reason enforcement: require a reason when canceling.
	if fields.Status != nil && *fields.Status == "canceled" {
		if fields.CanceledReason == nil || strings.TrimSpace(*fields.CanceledReason) == "" {
			return nil, &APIError{
				Code:    "invalid_input",
				Message: "canceled_reason is required when canceling a demand",
				Status:  http.StatusBadRequest,
				Details: map[string]interface{}{
					"hint": `Example: {"id": "dmd_...", "status": "canceled", "canceled_reason": "No longer needed"}`,
				},
			}
		}
	}

	updatedFields, err := a.dmdSvc.Update(ctx, id, fields)
	if err != nil {
		if errors.Is(err, storage.ErrDemandNotFound) {
			return nil, errNotFound("demand", "Demand not found")
		}
		return nil, errInvalidInput(err.Error())
	}
	action := "update"
	if fields.Status != nil && *fields.Status == "canceled" {
		action = "cancel"
	}
	if user := auth.GetAuthUser(ctx); user != nil {
		a.logEntityChange(user.UserID, action, "demand", id, demand.EndeavourID, updatedFields, demandFieldValues(fields, updatedFields))
	}

	// Auto-complete check: trigger when a demand is fulfilled.
	if fields.Status != nil && *fields.Status == "fulfilled" && demand.EndeavourID != "" {
		a.tryAutoComplete(ctx, demand.EndeavourID)
	}

	demand, err = a.dmdSvc.Get(ctx, id)
	if err != nil {
		return nil, errInternal("Failed to get updated demand")
	}
	return demandToMap(demand), nil
}

// --- HTTP handlers (thin wrappers) ---

func (a *API) handleDemandCreate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Type        string                 `json:"type"`
		Title       string                 `json:"title"`
		Description string                 `json:"description"`
		Priority    string                 `json:"priority"`
		EndeavourID string                 `json:"endeavour_id"`
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
			writeError(w, http.StatusBadRequest, "invalid_input", "Invalid due_date format")
			return
		}
		dueDate = &t
	}

	result, apiErr := a.CreateDemand(r.Context(), sanitize(body.Type), sanitize(body.Title), sanitize(body.Description), sanitize(body.Priority), sanitize(body.EndeavourID), dueDate, security.SanitizeMap(body.Metadata))
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	writeData(w, http.StatusCreated, result)
}

func (a *API) handleDemandList(w http.ResponseWriter, r *http.Request) {
	opts := storage.ListDemandsOpts{
		EndeavourIDs: a.resolveEndeavourIDs(r),
		Status:       queryString(r, "status"),
		Type:         queryString(r, "type"),
		Priority:     queryString(r, "priority"),
		EndeavourID:  queryString(r, "endeavour_id"),
		Search:       queryString(r, "search"),
		Limit:        queryInt(r, "limit", 50),
		Offset:       queryInt(r, "offset", 0),
	}

	items, total, apiErr := a.ListDemands(r.Context(), opts)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	writeList(w, items, total, opts.Limit, opts.Offset)
}

func (a *API) handleDemandGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	result, apiErr := a.GetDemand(r.Context(), id)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	writeData(w, http.StatusOK, result)
}

func (a *API) handleDemandUpdate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var body struct {
		Title          *string                `json:"title"`
		Description    *string                `json:"description"`
		Type           *string                `json:"type"`
		Status         *string                `json:"status"`
		Priority       *string                `json:"priority"`
		EndeavourID    *string                `json:"endeavour_id"`
		OwnerID        *string                `json:"owner_id"`
		DueDate        *string                `json:"due_date"`
		Metadata       map[string]interface{} `json:"metadata"`
		CanceledReason *string                `json:"canceled_reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Invalid JSON body")
		return
	}

	fields := storage.UpdateDemandFields{
		Title:          sanitizePtr(body.Title),
		Description:    sanitizePtr(body.Description),
		Type:           sanitizePtr(body.Type),
		Status:         body.Status,
		Priority:       body.Priority,
		EndeavourID:    sanitizePtr(body.EndeavourID),
		OwnerID:        sanitizePtr(body.OwnerID),
		DueDate:        body.DueDate,
		Metadata:       security.SanitizeMap(body.Metadata),
		CanceledReason: sanitizePtr(body.CanceledReason),
	}

	result, apiErr := a.UpdateDemand(r.Context(), id, fields)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	writeData(w, http.StatusOK, result)
}

func demandToMap(d *storage.Demand) map[string]interface{} {
	m := map[string]interface{}{
		"id":          d.ID,
		"type":        d.Type,
		"title":       d.Title,
		"description": d.Description,
		"status":      d.Status,
		"priority":    d.Priority,
		"metadata":    d.Metadata,
		"created_at":  d.CreatedAt.Format(time.RFC3339),
		"updated_at":  d.UpdatedAt.Format(time.RFC3339),
	}
	if d.EndeavourID != "" {
		m["endeavour_id"] = d.EndeavourID
	}
	if d.EndeavourName != "" {
		m["endeavour_name"] = d.EndeavourName
	}
	if d.CreatorID != "" {
		m["creator_id"] = d.CreatorID
	}
	if d.CreatorName != "" {
		m["creator_name"] = d.CreatorName
	}
	if d.OwnerID != "" {
		m["owner_id"] = d.OwnerID
	}
	if d.OwnerName != "" {
		m["owner_name"] = d.OwnerName
	}
	if d.DueDate != nil {
		m["due_date"] = d.DueDate.Format(time.RFC3339)
	}
	if d.CanceledReason != "" {
		m["canceled_reason"] = d.CanceledReason
	}
	return m
}
