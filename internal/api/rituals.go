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

	"github.com/QuestFinTech/taskschmiede/internal/security"
	"github.com/QuestFinTech/taskschmiede/internal/storage"
)

// CreateRitual creates a new ritual and returns its map representation.
func (a *API) CreateRitual(ctx context.Context, name, description, prompt, origin, createdBy, endeavourID, lang string, schedule, metadata map[string]interface{}) (map[string]interface{}, *APIError) {
	if apiErr := validateRitualCreate(name, prompt, description, endeavourID, metadata); apiErr != nil {
		return nil, apiErr
	}
	// RBAC: require admin access to target endeavour
	if endeavourID != "" {
		scope, apiErr := a.resolveScope(ctx)
		if apiErr != nil {
			return nil, apiErr
		}
		if apiErr := checkEndeavourAdmin(scope, endeavourID); apiErr != nil {
			return nil, apiErr
		}
	}
	metadata = scoreAndAnnotate(metadata, name, prompt, description)
	metadata = a.applyOrgAlertTerms(metadata, endeavourID, name, prompt, description)
	ritual, err := a.rtlSvc.Create(ctx, name, description, prompt, origin, createdBy, endeavourID, lang, schedule, metadata)
	if err != nil {
		return nil, errInvalidInput(err.Error())
	}
	return ritualToMap(ritual), nil
}

// GetRitual retrieves a ritual by ID and returns its map representation.
func (a *API) GetRitual(ctx context.Context, id string) (map[string]interface{}, *APIError) {
	ritual, err := a.rtlSvc.Get(ctx, id)
	if err != nil {
		if errors.Is(err, storage.ErrRitualNotFound) {
			return nil, errNotFound("ritual", "Ritual not found")
		}
		return nil, errInternal("Failed to get ritual")
	}
	// Built-in templates are readable by all authenticated users.
	if ritual.Origin != "template" {
		// RBAC: require read access to ritual's endeavour
		scope, apiErr := a.resolveScope(ctx)
		if apiErr != nil {
			return nil, apiErr
		}
		if apiErr := checkEndeavourRead(scope, ritual.EndeavourID); apiErr != nil {
			return nil, errNotFound("ritual", "Ritual not found")
		}
	}
	return ritualToMap(ritual), nil
}

// ListRituals queries rituals with the given options and returns their map representations.
func (a *API) ListRituals(ctx context.Context, opts storage.ListRitualsOpts) ([]map[string]interface{}, int, *APIError) {
	rituals, total, err := a.rtlSvc.List(ctx, opts)
	if err != nil {
		return nil, 0, errInternal("Failed to query rituals")
	}

	items := make([]map[string]interface{}, 0, len(rituals))
	for _, rtl := range rituals {
		items = append(items, ritualToMap(rtl))
	}
	return items, total, nil
}

// UpdateRitual updates a ritual and returns the updated map representation.
func (a *API) UpdateRitual(ctx context.Context, id string, fields storage.UpdateRitualFields) (map[string]interface{}, *APIError) {
	if apiErr := validateRitualUpdate(id, fields); apiErr != nil {
		return nil, apiErr
	}
	ritual, err := a.rtlSvc.Get(ctx, id)
	if err != nil {
		if errors.Is(err, storage.ErrRitualNotFound) {
			return nil, errNotFound("ritual", "Ritual not found")
		}
		return nil, errInternal("Failed to get ritual")
	}
	// Templates cannot be updated directly -- they must be forked.
	if ritual.Origin == "template" {
		return nil, errInvalidInput("Built-in templates cannot be modified. Fork the template instead.")
	}
	// RBAC: require admin access to ritual's endeavour
	scope, apiErr := a.resolveScope(ctx)
	if apiErr != nil {
		return nil, apiErr
	}
	if apiErr := checkEndeavourAdmin(scope, ritual.EndeavourID); apiErr != nil {
		return nil, errNotFound("ritual", "Ritual not found")
	}
	if fields.Name != nil || fields.Description != nil {
		fields.Metadata = scoreAndAnnotate(fields.Metadata, derefStr(fields.Name), derefStr(fields.Description))
	}
	_, err = a.rtlSvc.Update(ctx, id, fields)
	if err != nil {
		if errors.Is(err, storage.ErrRitualNotFound) {
			return nil, errNotFound("ritual", "Ritual not found")
		}
		return nil, errInvalidInput(err.Error())
	}

	ritual, err = a.rtlSvc.Get(ctx, id)
	if err != nil {
		return nil, errInternal("Failed to get updated ritual")
	}
	return ritualToMap(ritual), nil
}

// ForkRitual creates a new ritual derived from an existing one.
func (a *API) ForkRitual(ctx context.Context, sourceID, name, prompt, description, createdBy, endeavourID, lang string, schedule, metadata map[string]interface{}) (map[string]interface{}, *APIError) {
	sourceRitual, err := a.rtlSvc.Get(ctx, sourceID)
	if err != nil {
		if errors.Is(err, storage.ErrRitualNotFound) {
			return nil, errNotFound("ritual", "Source ritual not found")
		}
		return nil, errInternal("Failed to get source ritual")
	}
	scope, apiErr := a.resolveScope(ctx)
	if apiErr != nil {
		return nil, apiErr
	}
	// Built-in templates are forkable by all authenticated users.
	if sourceRitual.Origin != "template" {
		if apiErr := checkEndeavourRead(scope, sourceRitual.EndeavourID); apiErr != nil {
			return nil, errNotFound("ritual", "Source ritual not found")
		}
	}
	// Require admin access to target endeavour (if specified)
	if endeavourID != "" {
		if apiErr := checkEndeavourAdmin(scope, endeavourID); apiErr != nil {
			return nil, apiErr
		}
	}
	metadata = scoreAndAnnotate(metadata, name, prompt, description)
	var ritual *storage.Ritual
	ritual, err = a.rtlSvc.Fork(ctx, sourceID, name, prompt, description, createdBy, endeavourID, lang, schedule, metadata)
	if err != nil {
		if errors.Is(err, storage.ErrRitualNotFound) {
			return nil, errNotFound("ritual", "Source ritual not found")
		}
		return nil, errInvalidInput(err.Error())
	}
	return ritualToMap(ritual), nil
}

// GetRitualLineage returns the version chain for a ritual (oldest to newest).
func (a *API) GetRitualLineage(ctx context.Context, id string) ([]map[string]interface{}, *APIError) {
	ritual, err := a.rtlSvc.Get(ctx, id)
	if err != nil {
		if errors.Is(err, storage.ErrRitualNotFound) {
			return nil, errNotFound("ritual", "Ritual not found")
		}
		return nil, errInternal("Failed to get ritual")
	}
	// Built-in templates are readable by all authenticated users.
	if ritual.Origin != "template" {
		scope, apiErr := a.resolveScope(ctx)
		if apiErr != nil {
			return nil, apiErr
		}
		if apiErr := checkEndeavourRead(scope, ritual.EndeavourID); apiErr != nil {
			return nil, errNotFound("ritual", "Ritual not found")
		}
	}
	_ = ritual // used only for RBAC
	var rituals []*storage.Ritual
	rituals, err = a.rtlSvc.Lineage(ctx, id)
	if err != nil {
		if errors.Is(err, storage.ErrRitualNotFound) {
			return nil, errNotFound("ritual", "Ritual not found")
		}
		return nil, errInternal("Failed to get lineage")
	}

	items := make([]map[string]interface{}, 0, len(rituals))
	for _, rtl := range rituals {
		items = append(items, ritualToMap(rtl))
	}
	return items, nil
}

func (a *API) handleRitualCreate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name        string                 `json:"name"`
		Description string                 `json:"description"`
		Prompt      string                 `json:"prompt"`
		Lang        string                 `json:"lang"`
		EndeavourID string                 `json:"endeavour_id"`
		Schedule    map[string]interface{} `json:"schedule"`
		Metadata    map[string]interface{} `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Invalid JSON body")
		return
	}

	authUser := getAuthUser(r)
	createdBy := ""
	if authUser != nil {
		createdBy = authUser.UserID
	}

	result, apiErr := a.CreateRitual(r.Context(), sanitize(body.Name), sanitize(body.Description), sanitize(body.Prompt), "custom", createdBy, sanitize(body.EndeavourID), body.Lang, security.SanitizeMap(body.Schedule), security.SanitizeMap(body.Metadata))
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	writeData(w, http.StatusCreated, result)
}

func (a *API) handleRitualList(w http.ResponseWriter, r *http.Request) {
	opts := storage.ListRitualsOpts{
		EndeavourID:  queryString(r, "endeavour_id"),
		EndeavourIDs: a.resolveEndeavourIDs(r),
		Status:       queryString(r, "status"),
		Search:       queryString(r, "search"),
		Origin:       queryString(r, "origin"),
		Lang:         queryString(r, "lang"),
		Limit:        queryInt(r, "limit", 50),
		Offset:       queryInt(r, "offset", 0),
	}

	if v := queryString(r, "is_enabled"); v == "true" {
		b := true
		opts.IsEnabled = &b
	} else if v == "false" {
		b := false
		opts.IsEnabled = &b
	}

	items, total, apiErr := a.ListRituals(r.Context(), opts)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	writeList(w, items, total, opts.Limit, opts.Offset)
}

func (a *API) handleRitualGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	result, apiErr := a.GetRitual(r.Context(), id)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	writeData(w, http.StatusOK, result)
}

func (a *API) handleRitualUpdate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var body struct {
		Name        *string                `json:"name"`
		Description *string                `json:"description"`
		Schedule    map[string]interface{} `json:"schedule"`
		IsEnabled   *bool                  `json:"is_enabled"`
		Lang        *string                `json:"lang"`
		Status      *string                `json:"status"`
		Metadata    map[string]interface{} `json:"metadata"`
		EndeavourID *string                `json:"endeavour_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Invalid JSON body")
		return
	}

	fields := storage.UpdateRitualFields{
		Name:        sanitizePtr(body.Name),
		Description: sanitizePtr(body.Description),
		Schedule:    security.SanitizeMap(body.Schedule),
		IsEnabled:   body.IsEnabled,
		Lang:        body.Lang,
		Status:      body.Status,
		Metadata:    security.SanitizeMap(body.Metadata),
		EndeavourID: sanitizePtr(body.EndeavourID),
	}

	result, apiErr := a.UpdateRitual(r.Context(), id, fields)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	writeData(w, http.StatusOK, result)
}

func (a *API) handleRitualFork(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var body struct {
		Name        string                 `json:"name"`
		Prompt      string                 `json:"prompt"`
		Description string                 `json:"description"`
		Lang        string                 `json:"lang"`
		EndeavourID string                 `json:"endeavour_id"`
		Schedule    map[string]interface{} `json:"schedule"`
		Metadata    map[string]interface{} `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Invalid JSON body")
		return
	}

	authUser := getAuthUser(r)
	createdBy := ""
	if authUser != nil {
		createdBy = authUser.UserID
	}

	result, apiErr := a.ForkRitual(r.Context(), id, sanitize(body.Name), sanitize(body.Prompt), sanitize(body.Description), createdBy, sanitize(body.EndeavourID), body.Lang, security.SanitizeMap(body.Schedule), security.SanitizeMap(body.Metadata))
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	writeData(w, http.StatusCreated, result)
}

func (a *API) handleRitualLineage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	items, apiErr := a.GetRitualLineage(r.Context(), id)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	writeData(w, http.StatusOK, items)
}

func ritualToMap(r *storage.Ritual) map[string]interface{} {
	m := map[string]interface{}{
		"id":          r.ID,
		"name":        r.Name,
		"description": r.Description,
		"prompt":      r.Prompt,
		"origin":      r.Origin,
		"is_enabled":  r.IsEnabled,
		"lang":        r.Lang,
		"status":      r.Status,
		"metadata":    r.Metadata,
		"created_at":  r.CreatedAt.Format(time.RFC3339),
		"updated_at":  r.UpdatedAt.Format(time.RFC3339),
	}
	if r.Schedule != nil {
		m["schedule"] = r.Schedule
	}
	if r.EndeavourID != "" {
		m["endeavour_id"] = r.EndeavourID
	}
	if r.PredecessorID != "" {
		m["predecessor_id"] = r.PredecessorID
	}
	if r.MethodologyID != "" {
		m["methodology_id"] = r.MethodologyID
	}
	if r.CreatedBy != "" {
		m["created_by"] = r.CreatedBy
	}
	return m
}
