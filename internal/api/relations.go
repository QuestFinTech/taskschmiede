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
	"net/http"
	"time"

	"github.com/QuestFinTech/taskschmiede/internal/security"
	"github.com/QuestFinTech/taskschmiede/internal/storage"
)

// CreateRelation creates a new entity relation and returns its map representation.
func (a *API) CreateRelation(ctx context.Context, relationshipType, sourceEntityType, sourceEntityID, targetEntityType, targetEntityID string, metadata map[string]interface{}, createdBy string) (map[string]interface{}, *APIError) {
	if apiErr := validateRelationCreate(relationshipType, sourceEntityType, sourceEntityID, targetEntityType, targetEntityID, metadata); apiErr != nil {
		return nil, apiErr
	}
	// RBAC: master admin, or any org owner/admin.
	if apiErr := a.CheckAdmin(ctx); apiErr != nil {
		scope, scopeErr := a.resolveScope(ctx)
		if scopeErr != nil || !isAnyOrgAdmin(scope) {
			return nil, apiErr
		}
	}
	rel, err := a.relSvc.Create(ctx, relationshipType, sourceEntityType, sourceEntityID, targetEntityType, targetEntityID, metadata, createdBy)
	if err != nil {
		return nil, errInvalidInput(err.Error())
	}
	return relationToMap(rel), nil
}

// ListRelations queries relations with the given options and returns a list of map representations.
func (a *API) ListRelations(ctx context.Context, opts storage.ListRelationsOpts) ([]map[string]interface{}, int, *APIError) {
	rels, total, err := a.relSvc.List(ctx, opts)
	if err != nil {
		return nil, 0, errInternal("Failed to query relations")
	}

	items := make([]map[string]interface{}, 0, len(rels))
	for _, rel := range rels {
		items = append(items, relationToMap(rel))
	}
	return items, total, nil
}

// DeleteRelation removes a relation by ID.
func (a *API) DeleteRelation(ctx context.Context, id string) (map[string]interface{}, *APIError) {
	// RBAC: master admin, or any org owner/admin.
	if apiErr := a.CheckAdmin(ctx); apiErr != nil {
		scope, scopeErr := a.resolveScope(ctx)
		if scopeErr != nil || !isAnyOrgAdmin(scope) {
			return nil, apiErr
		}
	}
	if err := a.relSvc.Delete(ctx, id); err != nil {
		return nil, errNotFound("relation", "Relation not found")
	}
	return map[string]interface{}{"deleted": true, "id": id}, nil
}

// --- HTTP handlers (thin wrappers) ---

func (a *API) handleRelationCreate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RelationshipType string                 `json:"relationship_type"`
		SourceEntityType string                 `json:"source_entity_type"`
		SourceEntityID   string                 `json:"source_entity_id"`
		TargetEntityType string                 `json:"target_entity_type"`
		TargetEntityID   string                 `json:"target_entity_id"`
		Metadata         map[string]interface{} `json:"metadata"`
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

	result, apiErr := a.CreateRelation(r.Context(), sanitize(body.RelationshipType), sanitize(body.SourceEntityType), sanitize(body.SourceEntityID), sanitize(body.TargetEntityType), sanitize(body.TargetEntityID), security.SanitizeMap(body.Metadata), createdBy)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	writeData(w, http.StatusCreated, result)
}

func (a *API) handleRelationList(w http.ResponseWriter, r *http.Request) {
	opts := storage.ListRelationsOpts{
		SourceEntityType: queryString(r, "source_entity_type"),
		SourceEntityID:   queryString(r, "source_entity_id"),
		TargetEntityType: queryString(r, "target_entity_type"),
		TargetEntityID:   queryString(r, "target_entity_id"),
		RelationshipType: queryString(r, "relationship_type"),
		EndeavourIDs:     a.resolveEndeavourIDs(r),
		OrganizationIDs:  a.resolveOrganizationIDs(r),
		Limit:            queryInt(r, "limit", 50),
		Offset:           queryInt(r, "offset", 0),
	}

	items, total, apiErr := a.ListRelations(r.Context(), opts)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	writeList(w, items, total, opts.Limit, opts.Offset)
}

func (a *API) handleRelationDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	result, apiErr := a.DeleteRelation(r.Context(), id)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	writeData(w, http.StatusOK, result)
}

func relationToMap(rel *storage.EntityRelation) map[string]interface{} {
	m := map[string]interface{}{
		"id":                 rel.ID,
		"relationship_type":  rel.RelationshipType,
		"source_entity_type": rel.SourceEntityType,
		"source_entity_id":   rel.SourceEntityID,
		"target_entity_type": rel.TargetEntityType,
		"target_entity_id":   rel.TargetEntityID,
		"created_at":         rel.CreatedAt.Format(time.RFC3339),
	}
	if rel.Metadata != nil {
		m["metadata"] = rel.Metadata
	}
	return m
}
