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

// CreateResource creates a new resource and returns its map representation.
func (a *API) CreateResource(ctx context.Context, typ, name, capacityModel string, capacityValue *float64, skills []string, metadata map[string]interface{}) (map[string]interface{}, *APIError) {
	if apiErr := validateResourceCreate(typ, name, capacityModel, skills, metadata); apiErr != nil {
		return nil, apiErr
	}

	// RBAC: only master admin or org admin/owner can create resources.
	scope, apiErr := a.resolveScope(ctx)
	if apiErr != nil {
		return nil, apiErr
	}
	if !scope.IsMasterAdmin && !isAnyOrgAdmin(scope) {
		return nil, errForbidden("Only organization admins can create resources")
	}

	if apiErr := a.CheckCreationVelocity(ctx); apiErr != nil {
		return nil, apiErr
	}

	// Enforce teams_per_org quota for team-type resources.
	if typ == "team" {
		authUser := auth.GetAuthUser(ctx)
		if authUser != nil {
			orgID := a.findUserOrgID(ctx, authUser.UserID)
			if orgID != "" {
				if apiErr := a.CheckOrgQuota(ctx, orgID, "teams_per_org"); apiErr != nil {
					return nil, apiErr
				}
			}
		}
	}

	res, err := a.resSvc.Create(ctx, typ, name, capacityModel, capacityValue, skills, metadata)
	if err != nil {
		return nil, errInvalidInput(err.Error())
	}
	return resourceToMap(res), nil
}

// GetResource retrieves a resource by ID and returns its map representation.
func (a *API) GetResource(ctx context.Context, id string) (map[string]interface{}, *APIError) {
	res, err := a.resSvc.Get(ctx, id)
	if err != nil {
		if errors.Is(err, storage.ErrResourceNotFound) {
			return nil, errNotFound("resource", "Resource not found")
		}
		return nil, errInternal("Failed to get resource")
	}
	// RBAC: self or org admin
	scope, apiErr := a.resolveScope(ctx)
	if apiErr != nil {
		return nil, apiErr
	}
	callerResID := a.resolveCallerResourceIDSilent(ctx)
	if callerResID != id && !a.isOrgAdminOfResource(scope, id) {
		return nil, errNotFound("resource", "Resource not found")
	}
	return resourceToMap(res), nil
}

// UpdateResource applies partial updates to a resource and returns the updated map.
func (a *API) UpdateResource(ctx context.Context, id string, fields storage.UpdateResourceFields) (map[string]interface{}, *APIError) {
	if apiErr := validateResourceUpdate(id, fields); apiErr != nil {
		return nil, apiErr
	}
	// RBAC: self or org admin
	scope, apiErr := a.resolveScope(ctx)
	if apiErr != nil {
		return nil, apiErr
	}
	callerResID := a.resolveCallerResourceIDSilent(ctx)
	if callerResID != id && !a.isOrgAdminOfResource(scope, id) {
		return nil, errNotFound("resource", "Resource not found")
	}
	_, err := a.resSvc.Update(ctx, id, fields)
	if err != nil {
		if errors.Is(err, storage.ErrResourceNotFound) {
			return nil, errNotFound("resource", "Resource not found")
		}
		return nil, errInvalidInput(err.Error())
	}

	res, err := a.resSvc.Get(ctx, id)
	if err != nil {
		return nil, errInternal("Failed to get updated resource")
	}
	return resourceToMap(res), nil
}

// ListResources queries resources with the given options and returns a list of map representations.
func (a *API) ListResources(ctx context.Context, opts storage.ListResourcesOpts, adminMode bool) ([]map[string]interface{}, int, *APIError) {
	// RBAC: scope resource list to caller's visibility.
	// Admin mode allows master admins to see all resources.
	// Regular mode scopes to the caller's own resource and their org members.
	scope, apiErr := a.resolveScope(ctx)
	if apiErr != nil {
		return nil, 0, apiErr
	}
	if !adminMode || !scope.IsMasterAdmin {
		callerResID := a.resolveCallerResourceIDSilent(ctx)
		orgIDs := scope.OrgIDs()
		if len(orgIDs) == 0 && callerResID == "" {
			return []map[string]interface{}{}, 0, nil
		}
		opts.VisibleToResourceID = callerResID
		opts.VisibleToOrgIDs = orgIDs
	}

	resources, total, err := a.resSvc.List(ctx, opts)
	if err != nil {
		return nil, 0, errInternal("Failed to query resources")
	}

	items := make([]map[string]interface{}, 0, len(resources))
	for _, res := range resources {
		items = append(items, resourceToMap(res))
	}
	return items, total, nil
}

// --- HTTP handlers (thin wrappers) ---

func (a *API) handleResourceCreate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Type          string                 `json:"type"`
		Name          string                 `json:"name"`
		CapacityModel string                 `json:"capacity_model"`
		CapacityValue *float64               `json:"capacity_value"`
		Skills        []string               `json:"skills"`
		Metadata      map[string]interface{} `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Invalid JSON body")
		return
	}

	result, apiErr := a.CreateResource(r.Context(), sanitize(body.Type), sanitize(body.Name), sanitize(body.CapacityModel), body.CapacityValue, sanitizeStrings(body.Skills), security.SanitizeMap(body.Metadata))
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	writeData(w, http.StatusCreated, result)
}

func (a *API) handleResourceList(w http.ResponseWriter, r *http.Request) {
	opts := storage.ListResourcesOpts{
		Type:           queryString(r, "type"),
		Status:         queryString(r, "status"),
		OrganizationID: queryString(r, "organization_id"),
		Search:         queryString(r, "search"),
		Limit:          queryInt(r, "limit", 50),
		Offset:         queryInt(r, "offset", 0),
	}

	adminMode := queryString(r, "admin") == "true"
	items, total, apiErr := a.ListResources(r.Context(), opts, adminMode)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	writeList(w, items, total, opts.Limit, opts.Offset)
}

func (a *API) handleResourceGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	result, apiErr := a.GetResource(r.Context(), id)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	writeData(w, http.StatusOK, result)
}

func (a *API) handleResourceUpdate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var body struct {
		Name          *string                `json:"name"`
		CapacityModel *string                `json:"capacity_model"`
		CapacityValue *float64               `json:"capacity_value"`
		Skills        []string               `json:"skills"`
		Metadata      map[string]interface{} `json:"metadata"`
		Status        *string                `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Invalid JSON body")
		return
	}

	fields := storage.UpdateResourceFields{
		Name:          sanitizePtr(body.Name),
		CapacityModel: sanitizePtr(body.CapacityModel),
		CapacityValue: body.CapacityValue,
		Skills:        sanitizeStrings(body.Skills),
		Metadata:      security.SanitizeMap(body.Metadata),
		Status:        body.Status,
	}

	result, apiErr := a.UpdateResource(r.Context(), id, fields)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	writeData(w, http.StatusOK, result)
}

// DeleteResource deletes a team resource.
func (a *API) DeleteResource(ctx context.Context, id string) *APIError {
	// RBAC: org admin only.
	scope, apiErr := a.resolveScope(ctx)
	if apiErr != nil {
		return apiErr
	}
	if !a.isOrgAdminOfResource(scope, id) {
		return errForbidden("Only organization admins can delete team resources")
	}
	if err := a.resSvc.Delete(ctx, id); err != nil {
		if errors.Is(err, storage.ErrResourceNotFound) {
			return errNotFound("resource", "Resource not found")
		}
		return errInvalidInput(err.Error())
	}
	return nil
}

func (a *API) handleResourceDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if apiErr := a.DeleteResource(r.Context(), id); apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}
	writeData(w, http.StatusOK, map[string]interface{}{"deleted": true})
}

func resourceToMap(r *storage.Resource) map[string]interface{} {
	m := map[string]interface{}{
		"id":         r.ID,
		"type":       r.Type,
		"name":       r.Name,
		"status":     r.Status,
		"metadata":   r.Metadata,
		"created_at": r.CreatedAt.Format(time.RFC3339),
		"updated_at": r.UpdatedAt.Format(time.RFC3339),
	}
	if r.CapacityModel != "" {
		m["capacity_model"] = r.CapacityModel
	}
	if r.CapacityValue != nil {
		m["capacity_value"] = *r.CapacityValue
	}
	if len(r.Skills) > 0 {
		m["skills"] = r.Skills
	}
	return m
}
