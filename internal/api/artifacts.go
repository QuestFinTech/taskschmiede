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

// CreateArtifact creates a new artifact and returns its map representation.
func (a *API) CreateArtifact(ctx context.Context, kind, title, url, summary string, tags []string, metadata map[string]interface{}, createdBy, endeavourID, taskID string) (map[string]interface{}, *APIError) {
	if apiErr := validateArtifactCreate(kind, title, url, summary, tags, endeavourID, taskID, metadata); apiErr != nil {
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
	metadata = scoreAndAnnotate(metadata, title, summary)
	metadata = a.applyOrgAlertTerms(metadata, endeavourID, title, summary)
	art, err := a.artSvc.Create(ctx, kind, title, url, summary, tags, metadata, createdBy, endeavourID, taskID)
	if err != nil {
		return nil, errInvalidInput(err.Error())
	}
	return artifactToMap(art), nil
}

// GetArtifact retrieves an artifact by ID and returns its map representation.
func (a *API) GetArtifact(ctx context.Context, id string) (map[string]interface{}, *APIError) {
	art, err := a.artSvc.Get(ctx, id)
	if err != nil {
		if errors.Is(err, storage.ErrArtifactNotFound) {
			return nil, errNotFound("artifact", "Artifact not found")
		}
		return nil, errInternal("Failed to get artifact")
	}
	// RBAC: require read access to artifact's endeavour
	scope, apiErr := a.resolveScope(ctx)
	if apiErr != nil {
		return nil, apiErr
	}
	if apiErr := checkEndeavourRead(scope, art.EndeavourID); apiErr != nil {
		return nil, errNotFound("artifact", "Artifact not found")
	}
	return artifactToMap(art), nil
}

// ListArtifacts queries artifacts with the given options and returns their map representations.
func (a *API) ListArtifacts(ctx context.Context, opts storage.ListArtifactsOpts) ([]map[string]interface{}, int, *APIError) {
	artifacts, total, err := a.artSvc.List(ctx, opts)
	if err != nil {
		return nil, 0, errInternal("Failed to query artifacts")
	}

	items := make([]map[string]interface{}, 0, len(artifacts))
	for _, art := range artifacts {
		items = append(items, artifactToMap(art))
	}
	return items, total, nil
}

// UpdateArtifact updates an artifact and returns the updated map representation.
func (a *API) UpdateArtifact(ctx context.Context, id string, fields storage.UpdateArtifactFields) (map[string]interface{}, *APIError) {
	if apiErr := validateArtifactUpdate(id, fields); apiErr != nil {
		return nil, apiErr
	}
	// RBAC: fetch artifact to check access
	art, err := a.artSvc.Get(ctx, id)
	if err != nil {
		if errors.Is(err, storage.ErrArtifactNotFound) {
			return nil, errNotFound("artifact", "Artifact not found")
		}
		return nil, errInternal("Failed to get artifact")
	}
	scope, apiErr := a.resolveScope(ctx)
	if apiErr != nil {
		return nil, apiErr
	}
	if apiErr := checkEndeavourWrite(scope, art.EndeavourID); apiErr != nil {
		return nil, errNotFound("artifact", "Artifact not found")
	}
	if fields.Title != nil || fields.Summary != nil {
		fields.Metadata = scoreAndAnnotate(fields.Metadata, derefStr(fields.Title), derefStr(fields.Summary))
	}
	_, err = a.artSvc.Update(ctx, id, fields)
	if err != nil {
		if errors.Is(err, storage.ErrArtifactNotFound) {
			return nil, errNotFound("artifact", "Artifact not found")
		}
		return nil, errInvalidInput(err.Error())
	}

	art, err = a.artSvc.Get(ctx, id)
	if err != nil {
		return nil, errInternal("Failed to get updated artifact")
	}
	return artifactToMap(art), nil
}

// DeleteArtifact sets an artifact's status to "deleted" (logical delete).
func (a *API) DeleteArtifact(ctx context.Context, id string) (map[string]interface{}, *APIError) {
	// RBAC: fetch artifact to check access
	art, err := a.artSvc.Get(ctx, id)
	if err != nil {
		if errors.Is(err, storage.ErrArtifactNotFound) {
			return nil, errNotFound("artifact", "Artifact not found")
		}
		return nil, errInternal("Failed to get artifact")
	}
	scope, apiErr := a.resolveScope(ctx)
	if apiErr != nil {
		return nil, apiErr
	}
	if apiErr := checkEndeavourWrite(scope, art.EndeavourID); apiErr != nil {
		return nil, errNotFound("artifact", "Artifact not found")
	}
	deleted := "deleted"
	fields := storage.UpdateArtifactFields{Status: &deleted}
	_, err = a.artSvc.Update(ctx, id, fields)
	if err != nil {
		if errors.Is(err, storage.ErrArtifactNotFound) {
			return nil, errNotFound("artifact", "Artifact not found")
		}
		return nil, errInvalidInput(err.Error())
	}
	return map[string]interface{}{"id": id, "status": "deleted"}, nil
}

func (a *API) handleArtifactDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	result, apiErr := a.DeleteArtifact(r.Context(), id)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}
	writeData(w, http.StatusOK, result)
}

func (a *API) handleArtifactCreate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Kind        string                 `json:"kind"`
		Title       string                 `json:"title"`
		URL         string                 `json:"url"`
		Summary     string                 `json:"summary"`
		Tags        []string               `json:"tags"`
		Metadata    map[string]interface{} `json:"metadata"`
		EndeavourID string                 `json:"endeavour_id"`
		TaskID      string                 `json:"task_id"`
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

	result, apiErr := a.CreateArtifact(r.Context(), sanitize(body.Kind), sanitize(body.Title), sanitize(body.URL), sanitize(body.Summary), sanitizeStrings(body.Tags), security.SanitizeMap(body.Metadata), createdBy, sanitize(body.EndeavourID), sanitize(body.TaskID))
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	writeData(w, http.StatusCreated, result)
}

func (a *API) handleArtifactList(w http.ResponseWriter, r *http.Request) {
	opts := storage.ListArtifactsOpts{
		EndeavourID:  queryString(r, "endeavour_id"),
		EndeavourIDs: a.resolveEndeavourIDs(r),
		TaskID:       queryString(r, "task_id"),
		Kind:         queryString(r, "kind"),
		Status:       queryString(r, "status"),
		Search:       queryString(r, "search"),
		Tags:         queryString(r, "tags"),
		Limit:        queryInt(r, "limit", 50),
		Offset:       queryInt(r, "offset", 0),
	}

	items, total, apiErr := a.ListArtifacts(r.Context(), opts)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	writeList(w, items, total, opts.Limit, opts.Offset)
}

func (a *API) handleArtifactGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	result, apiErr := a.GetArtifact(r.Context(), id)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	writeData(w, http.StatusOK, result)
}

func (a *API) handleArtifactUpdate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var body struct {
		Title       *string                `json:"title"`
		Kind        *string                `json:"kind"`
		URL         *string                `json:"url"`
		Summary     *string                `json:"summary"`
		Tags        *[]string              `json:"tags"`
		Metadata    map[string]interface{} `json:"metadata"`
		Status      *string                `json:"status"`
		EndeavourID *string                `json:"endeavour_id"`
		TaskID      *string                `json:"task_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Invalid JSON body")
		return
	}

	fields := storage.UpdateArtifactFields{
		Title:       sanitizePtr(body.Title),
		Kind:        sanitizePtr(body.Kind),
		URL:         sanitizePtr(body.URL),
		Summary:     sanitizePtr(body.Summary),
		Tags:        sanitizeStringsPtr(body.Tags),
		Metadata:    security.SanitizeMap(body.Metadata),
		Status:      body.Status,
		EndeavourID: sanitizePtr(body.EndeavourID),
		TaskID:      sanitizePtr(body.TaskID),
	}

	result, apiErr := a.UpdateArtifact(r.Context(), id, fields)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	writeData(w, http.StatusOK, result)
}

func artifactToMap(a *storage.Artifact) map[string]interface{} {
	m := map[string]interface{}{
		"id":         a.ID,
		"kind":       a.Kind,
		"title":      a.Title,
		"status":     a.Status,
		"metadata":   a.Metadata,
		"created_at": a.CreatedAt.Format(time.RFC3339),
		"updated_at": a.UpdatedAt.Format(time.RFC3339),
	}
	if a.URL != "" {
		m["url"] = a.URL
	}
	if a.Summary != "" {
		m["summary"] = a.Summary
	}
	if len(a.Tags) > 0 {
		m["tags"] = a.Tags
	}
	if a.EndeavourID != "" {
		m["endeavour_id"] = a.EndeavourID
	}
	if a.TaskID != "" {
		m["task_id"] = a.TaskID
	}
	if a.CreatedBy != "" {
		m["created_by"] = a.CreatedBy
	}
	return m
}
