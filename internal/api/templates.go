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

	"github.com/QuestFinTech/taskschmiede/internal/storage"
)

// CreateTemplate creates a new template and returns its map representation.
func (a *API) CreateTemplate(ctx context.Context, name, tplType, scope, lang, body, createdBy string, metadata map[string]interface{}) (map[string]interface{}, *APIError) {
	if name == "" {
		return nil, errInvalidInput("name is required")
	}
	if body == "" {
		return nil, errInvalidInput("body is required")
	}

	tpl, err := a.tplSvc.Create(ctx, name, tplType, scope, lang, body, createdBy, metadata)
	if err != nil {
		return nil, errInvalidInput(err.Error())
	}
	return templateToMap(tpl), nil
}

// GetTemplate retrieves a template by ID.
func (a *API) GetTemplate(ctx context.Context, id string) (map[string]interface{}, *APIError) {
	tpl, err := a.tplSvc.Get(ctx, id)
	if err != nil {
		if errors.Is(err, storage.ErrTemplateNotFound) {
			return nil, errNotFound("template", "Template not found")
		}
		return nil, errInternal("Failed to get template")
	}
	return templateToMap(tpl), nil
}

// ListTemplates queries templates with filters.
func (a *API) ListTemplates(ctx context.Context, opts storage.ListTemplatesOpts) ([]map[string]interface{}, int, *APIError) {
	templates, total, err := a.tplSvc.List(ctx, opts)
	if err != nil {
		return nil, 0, errInternal("Failed to query templates")
	}
	items := make([]map[string]interface{}, 0, len(templates))
	for _, tpl := range templates {
		items = append(items, templateToMap(tpl))
	}
	return items, total, nil
}

// UpdateTemplate updates a template and returns the updated map.
func (a *API) UpdateTemplate(ctx context.Context, id string, fields storage.UpdateTemplateFields) (map[string]interface{}, *APIError) {
	_, err := a.tplSvc.Update(ctx, id, fields)
	if err != nil {
		if errors.Is(err, storage.ErrTemplateNotFound) {
			return nil, errNotFound("template", "Template not found")
		}
		return nil, errInvalidInput(err.Error())
	}
	tpl, err := a.tplSvc.Get(ctx, id)
	if err != nil {
		return nil, errInternal("Failed to get updated template")
	}
	return templateToMap(tpl), nil
}

// ForkTemplate creates a new template version from an existing one.
func (a *API) ForkTemplate(ctx context.Context, sourceID, name, body, lang, createdBy string, metadata map[string]interface{}) (map[string]interface{}, *APIError) {
	tpl, err := a.tplSvc.Fork(ctx, sourceID, name, body, lang, createdBy, metadata)
	if err != nil {
		if errors.Is(err, storage.ErrTemplateNotFound) {
			return nil, errNotFound("template", "Source template not found")
		}
		return nil, errInvalidInput(err.Error())
	}
	return templateToMap(tpl), nil
}

// GetTemplateLineage returns the version chain for a template.
func (a *API) GetTemplateLineage(ctx context.Context, id string) ([]map[string]interface{}, *APIError) {
	templates, err := a.tplSvc.Lineage(ctx, id)
	if err != nil {
		if errors.Is(err, storage.ErrTemplateNotFound) {
			return nil, errNotFound("template", "Template not found")
		}
		return nil, errInternal("Failed to get lineage")
	}
	items := make([]map[string]interface{}, 0, len(templates))
	for _, tpl := range templates {
		items = append(items, templateToMap(tpl))
	}
	return items, nil
}

// --- REST handlers ---

func (a *API) handleTemplateCreate(w http.ResponseWriter, r *http.Request) {
	if !a.requireAdmin(w, r) {
		return
	}

	var body struct {
		Name     string                 `json:"name"`
		Type     string                 `json:"type"`
		Scope    string                 `json:"scope"`
		Lang     string                 `json:"lang"`
		Body     string                 `json:"body"`
		Metadata map[string]interface{} `json:"metadata"`
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

	result, apiErr := a.CreateTemplate(r.Context(), sanitize(body.Name), body.Type, body.Scope, body.Lang, body.Body, createdBy, body.Metadata)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}
	writeData(w, http.StatusCreated, result)
}

func (a *API) handleTemplateList(w http.ResponseWriter, r *http.Request) {
	opts := storage.ListTemplatesOpts{
		Scope:  queryString(r, "scope"),
		Lang:   queryString(r, "lang"),
		Status: queryString(r, "status"),
		Search: queryString(r, "search"),
		Limit:  queryInt(r, "limit", 50),
		Offset: queryInt(r, "offset", 0),
	}

	items, total, apiErr := a.ListTemplates(r.Context(), opts)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}
	writeList(w, items, total, opts.Limit, opts.Offset)
}

func (a *API) handleTemplateGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	result, apiErr := a.GetTemplate(r.Context(), id)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}
	writeData(w, http.StatusOK, result)
}

func (a *API) handleTemplateUpdate(w http.ResponseWriter, r *http.Request) {
	if !a.requireAdmin(w, r) {
		return
	}

	id := r.PathValue("id")

	var body struct {
		Name     *string                `json:"name"`
		Body     *string                `json:"body"`
		Lang     *string                `json:"lang"`
		Status   *string                `json:"status"`
		Metadata map[string]interface{} `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Invalid JSON body")
		return
	}

	fields := storage.UpdateTemplateFields{
		Name:     sanitizePtr(body.Name),
		Body:     body.Body,
		Lang:     body.Lang,
		Status:   body.Status,
		Metadata: body.Metadata,
	}

	result, apiErr := a.UpdateTemplate(r.Context(), id, fields)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}
	writeData(w, http.StatusOK, result)
}

func (a *API) handleTemplateFork(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var body struct {
		Name     string                 `json:"name"`
		Body     string                 `json:"body"`
		Lang     string                 `json:"lang"`
		Metadata map[string]interface{} `json:"metadata"`
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

	result, apiErr := a.ForkTemplate(r.Context(), id, sanitize(body.Name), body.Body, body.Lang, createdBy, body.Metadata)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}
	writeData(w, http.StatusCreated, result)
}

func (a *API) handleTemplateLineage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	items, apiErr := a.GetTemplateLineage(r.Context(), id)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}
	writeData(w, http.StatusOK, items)
}

func templateToMap(t *storage.Template) map[string]interface{} {
	m := map[string]interface{}{
		"id":         t.ID,
		"name":       t.Name,
		"type":       t.Type,
		"scope":      t.Scope,
		"lang":       t.Lang,
		"body":       t.Body,
		"version":    t.Version,
		"status":     t.Status,
		"metadata":   t.Metadata,
		"created_at": t.CreatedAt.Format(time.RFC3339),
		"updated_at": t.UpdatedAt.Format(time.RFC3339),
	}
	if t.PredecessorID != "" {
		m["predecessor_id"] = t.PredecessorID
	}
	if t.CreatedBy != "" {
		m["created_by"] = t.CreatedBy
	}
	return m
}
