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
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/QuestFinTech/taskschmiede/internal/auth"
	"github.com/QuestFinTech/taskschmiede/internal/security"
	"github.com/QuestFinTech/taskschmiede/internal/storage"
)

// CreateOrganization creates a new organization and returns its map representation.
// The creating user is automatically linked as owner via their resource.
func (a *API) CreateOrganization(ctx context.Context, name, description string, metadata map[string]interface{}) (map[string]interface{}, *APIError) {
	if apiErr := validateOrganizationCreate(name, description, metadata); apiErr != nil {
		return nil, apiErr
	}
	// Tier enforcement
	if apiErr := a.CheckTierLimit(ctx, "orgs"); apiErr != nil {
		return nil, apiErr
	}

	metadata = scoreAndAnnotate(metadata, name, description)
	org, err := a.orgSvc.Create(ctx, name, description, metadata)
	if err != nil {
		return nil, errInvalidInput(err.Error())
	}

	// Auto-link creator as owner
	if authUser := auth.GetAuthUser(ctx); authUser != nil {
		user, uerr := a.usrSvc.Get(ctx, authUser.UserID)
		if uerr == nil && user.ResourceID != nil && *user.ResourceID != "" {
			if lerr := a.orgSvc.AddResource(ctx, org.ID, *user.ResourceID, "owner"); lerr != nil {
				a.logger.Warn("Failed to auto-link org creator as owner", "org_id", org.ID, "error", lerr)
			}
		}
	}

	return orgToMap(org, 0, 0), nil
}

// GetOrganization retrieves an organization by ID with member and endeavour counts.
func (a *API) GetOrganization(ctx context.Context, id string) (map[string]interface{}, *APIError) {
	// RBAC: require org membership
	scope, apiErr := a.resolveScope(ctx)
	if apiErr != nil {
		return nil, apiErr
	}
	if !scope.CanReadOrg(id) {
		return nil, errNotFound("organization", "Organization not found")
	}
	org, memberCount, edvCount, err := a.orgSvc.Get(ctx, id)
	if err != nil {
		if errors.Is(err, storage.ErrOrgNotFound) {
			return nil, errNotFound("organization", "Organization not found")
		}
		return nil, errInternal("Failed to get organization")
	}
	return orgToMap(org, memberCount, edvCount), nil
}

// ListOrganizations queries organizations with the given options and returns a list of map representations.
func (a *API) ListOrganizations(ctx context.Context, opts storage.ListOrganizationsOpts, adminMode bool) ([]map[string]interface{}, int, *APIError) {
	// RBAC: scope-filter to user's organizations
	scope, apiErr := a.resolveScope(ctx)
	if apiErr != nil {
		return nil, 0, apiErr
	}
	if !adminMode || !scope.IsMasterAdmin {
		ids := make([]string, 0, len(scope.Organizations))
		for id := range scope.Organizations {
			ids = append(ids, id)
		}
		opts.OrganizationIDs = ids
	}
	orgs, total, err := a.orgSvc.List(ctx, opts)
	if err != nil {
		return nil, 0, errInternal("Failed to query organizations")
	}

	items := make([]map[string]interface{}, 0, len(orgs))
	for _, o := range orgs {
		mc, _ := a.db.GetOrganizationMemberCount(o.ID)
		ec, _ := a.db.GetOrganizationEndeavourCount(o.ID)
		items = append(items, orgToMap(o, mc, ec))
	}
	return items, total, nil
}

// UpdateOrganization applies partial updates to an organization and returns its map representation.
func (a *API) UpdateOrganization(ctx context.Context, id string, fields storage.UpdateOrganizationFields) (map[string]interface{}, *APIError) {
	if apiErr := validateOrganizationUpdate(id, fields); apiErr != nil {
		return nil, apiErr
	}
	// RBAC: require admin
	scope, apiErr := a.resolveScope(ctx)
	if apiErr != nil {
		return nil, apiErr
	}
	if !scope.CanAdminOrg(id) {
		return nil, errNotFound("organization", "Organization not found")
	}
	if fields.Name != nil || fields.Description != nil {
		fields.Metadata = scoreAndAnnotate(fields.Metadata, derefStr(fields.Name), derefStr(fields.Description))
	}
	org, err := a.orgSvc.Update(ctx, id, fields)
	if err != nil {
		if errors.Is(err, storage.ErrOrgNotFound) {
			return nil, errNotFound("organization", "Organization not found")
		}
		return nil, errInvalidInput(err.Error())
	}

	memberCount, _ := a.db.GetOrganizationMemberCount(id)
	edvCount, _ := a.db.GetOrganizationEndeavourCount(id)

	return orgToMap(org, memberCount, edvCount), nil
}

// AddResourceToOrg adds a resource to an organization.
func (a *API) AddResourceToOrg(ctx context.Context, orgID, resourceID, role string) (map[string]interface{}, *APIError) {
	// RBAC: require org admin
	scope, apiErr := a.resolveScope(ctx)
	if apiErr != nil {
		return nil, apiErr
	}
	if !scope.CanAdminOrg(orgID) {
		return nil, errNotFound("organization", "Organization not found")
	}
	if err := a.orgSvc.AddResource(ctx, orgID, resourceID, role); err != nil {
		return nil, errInvalidInput(err.Error())
	}
	return map[string]interface{}{
		"organization_id": orgID,
		"resource_id":     resourceID,
		"role":            role,
	}, nil
}

// AddEndeavourToOrg links an endeavour to an organization.
func (a *API) AddEndeavourToOrg(ctx context.Context, orgID, endeavourID, role string) (map[string]interface{}, *APIError) {
	// RBAC: require org admin
	scope, apiErr := a.resolveScope(ctx)
	if apiErr != nil {
		return nil, apiErr
	}
	if !scope.CanAdminOrg(orgID) {
		return nil, errNotFound("organization", "Organization not found")
	}
	if err := a.orgSvc.AddEndeavour(ctx, orgID, endeavourID, role); err != nil {
		return nil, errInvalidInput(err.Error())
	}
	return map[string]interface{}{
		"organization_id": orgID,
		"endeavour_id":    endeavourID,
		"role":            role,
	}, nil
}

// OrgArchiveImpact returns a dry-run impact summary for archiving an organization.
func (a *API) OrgArchiveImpact(ctx context.Context, id string) (map[string]interface{}, *APIError) {
	// RBAC: require owner
	scope, apiErr := a.resolveScope(ctx)
	if apiErr != nil {
		return nil, apiErr
	}
	if !scope.CanOwnerOrg(id) {
		return nil, errNotFound("organization", "Organization not found")
	}

	impact, err := a.orgSvc.ArchiveImpact(ctx, id)
	if err != nil {
		if errors.Is(err, storage.ErrOrgNotFound) {
			return nil, errNotFound("organization", "Organization not found")
		}
		return nil, errInvalidInput(err.Error())
	}

	return map[string]interface{}{
		"organization_id":      id,
		"endeavours_to_archive": impact.EndeavoursToArchive,
		"total_tasks_to_cancel": impact.TotalTasksToCancel,
		"total_planned_tasks":   impact.TotalPlannedTasks,
		"total_active_tasks":    impact.TotalActiveTasks,
	}, nil
}

// ArchiveOrganization archives an organization (cascading to endeavours and tasks).
func (a *API) ArchiveOrganization(ctx context.Context, id, reason string) (map[string]interface{}, *APIError) {
	if reason == "" {
		return nil, errInvalidInput("reason is required")
	}
	// RBAC: require owner
	scope, apiErr := a.resolveScope(ctx)
	if apiErr != nil {
		return nil, apiErr
	}
	if !scope.CanOwnerOrg(id) {
		return nil, errNotFound("organization", "Organization not found")
	}

	impact, err := a.orgSvc.Archive(ctx, id, reason)
	if err != nil {
		if errors.Is(err, storage.ErrOrgNotFound) {
			return nil, errNotFound("organization", "Organization not found")
		}
		return nil, errInvalidInput(err.Error())
	}

	if user := auth.GetAuthUser(ctx); user != nil {
		a.logEntityChange(user.UserID, "archive", "organization", id, "", nil, nil)
	}

	org, memberCount, edvCount, _ := a.orgSvc.Get(ctx, id)
	if org != nil {
		result := orgToMap(org, memberCount, edvCount)
		result["endeavours_archived"] = impact.EndeavoursToArchive
		result["tasks_canceled"] = impact.TotalTasksToCancel
		return result, nil
	}
	return map[string]interface{}{
		"id":                    id,
		"status":                "archived",
		"endeavours_archived":   impact.EndeavoursToArchive,
		"tasks_canceled":        impact.TotalTasksToCancel,
	}, nil
}

// --- HTTP handlers (thin wrappers) ---

func (a *API) handleOrganizationCreate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name        string                 `json:"name"`
		Description string                 `json:"description"`
		Metadata    map[string]interface{} `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Invalid JSON body")
		return
	}

	result, apiErr := a.CreateOrganization(r.Context(), sanitize(body.Name), sanitize(body.Description), security.SanitizeMap(body.Metadata))
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	writeData(w, http.StatusCreated, result)
}

func (a *API) handleOrganizationList(w http.ResponseWriter, r *http.Request) {
	opts := storage.ListOrganizationsOpts{
		Status: queryString(r, "status"),
		Search: queryString(r, "search"),
		Limit:  queryInt(r, "limit", 50),
		Offset: queryInt(r, "offset", 0),
	}

	adminMode := queryString(r, "admin") == "true"
	items, total, apiErr := a.ListOrganizations(r.Context(), opts, adminMode)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	writeList(w, items, total, opts.Limit, opts.Offset)
}

func (a *API) handleOrganizationGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	result, apiErr := a.GetOrganization(r.Context(), id)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	writeData(w, http.StatusOK, result)
}

func (a *API) handleOrganizationUpdate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var body struct {
		Name               *string                `json:"name"`
		Description        *string                `json:"description"`
		Metadata           map[string]interface{} `json:"metadata"`
		Status             *string                `json:"status"`
		TaskschmiedEnabled *bool                  `json:"taskschmied_enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Invalid JSON body")
		return
	}

	fields := storage.UpdateOrganizationFields{
		Name:               sanitizePtr(body.Name),
		Description:        sanitizePtr(body.Description),
		Metadata:           security.SanitizeMap(body.Metadata),
		Status:             body.Status,
		TaskschmiedEnabled: body.TaskschmiedEnabled,
	}

	result, apiErr := a.UpdateOrganization(r.Context(), id, fields)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	writeData(w, http.StatusOK, result)
}

func (a *API) handleOrganizationAddResource(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var body struct {
		ResourceID string `json:"resource_id"`
		Role       string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Invalid JSON body")
		return
	}

	if body.Role == "" {
		body.Role = "member"
	}

	result, apiErr := a.AddResourceToOrg(r.Context(), id, sanitize(body.ResourceID), sanitize(body.Role))
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	writeData(w, http.StatusOK, result)
}

func (a *API) handleOrganizationAddEndeavour(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var body struct {
		EndeavourID string `json:"endeavour_id"`
		Role        string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Invalid JSON body")
		return
	}

	if body.Role == "" {
		body.Role = "participant"
	}

	result, apiErr := a.AddEndeavourToOrg(r.Context(), id, sanitize(body.EndeavourID), sanitize(body.Role))
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	writeData(w, http.StatusOK, result)
}

func (a *API) handleOrganizationArchiveImpact(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	result, apiErr := a.OrgArchiveImpact(r.Context(), id)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	writeData(w, http.StatusOK, result)
}

func (a *API) handleOrganizationArchive(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var body struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Invalid JSON body")
		return
	}

	result, apiErr := a.ArchiveOrganization(r.Context(), id, sanitize(body.Reason))
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	writeData(w, http.StatusOK, result)
}

// --- Org alert terms ---

// ListOrgAlertTerms returns alert terms for an organization.
func (a *API) ListOrgAlertTerms(ctx context.Context, orgID string) ([]map[string]interface{}, *APIError) {
	scope, apiErr := a.resolveScope(ctx)
	if apiErr != nil {
		return nil, apiErr
	}
	if !scope.CanReadOrg(orgID) {
		return nil, errNotFound("organization", "Organization not found")
	}
	terms, err := a.db.ListOrgAlertTerms(orgID)
	if err != nil {
		return nil, errInternal("Failed to list alert terms")
	}
	items := make([]map[string]interface{}, 0, len(terms))
	for _, t := range terms {
		items = append(items, map[string]interface{}{
			"id":     t.ID,
			"term":   t.Term,
			"weight": t.Weight,
		})
	}
	return items, nil
}

// UpdateOrgAlertTerms replaces all alert terms for an organization.
func (a *API) UpdateOrgAlertTerms(ctx context.Context, orgID string, terms []struct {
	Term   string `json:"term"`
	Weight int    `json:"weight"`
}) ([]map[string]interface{}, *APIError) {
	scope, apiErr := a.resolveScope(ctx)
	if apiErr != nil {
		return nil, apiErr
	}
	if !scope.CanAdminOrg(orgID) {
		return nil, errNotFound("organization", "Organization not found")
	}

	// Validate
	if len(terms) > 100 {
		return nil, errInvalidInput("Maximum 100 alert terms per organization")
	}
	seen := map[string]bool{}
	for _, t := range terms {
		term := strings.TrimSpace(t.Term)
		if term == "" {
			continue
		}
		if len(term) > 200 {
			return nil, errInvalidInput(fmt.Sprintf("Term %q exceeds 200 character limit", term[:50]+"..."))
		}
		if t.Weight < 1 || t.Weight > 10 {
			return nil, errInvalidInput(fmt.Sprintf("Weight for %q must be between 1 and 10", term))
		}
		lower := strings.ToLower(term)
		if seen[lower] {
			return nil, errInvalidInput(fmt.Sprintf("Duplicate term: %q", term))
		}
		seen[lower] = true
	}

	callerResID := a.resolveCallerResourceIDSilent(ctx)

	// Delete existing, then create new
	if err := a.db.DeleteOrgAlertTermsByOrg(orgID); err != nil {
		return nil, errInternal("Failed to clear existing alert terms")
	}

	for _, t := range terms {
		term := strings.TrimSpace(t.Term)
		if term == "" {
			continue
		}
		if _, err := a.db.CreateOrgAlertTerm(orgID, term, t.Weight, callerResID); err != nil {
			return nil, errInternal("Failed to create alert term")
		}
	}

	// Invalidate cache for this org
	orgTermCacheMu.Lock()
	for k := range orgTermCache {
		if strings.HasPrefix(k, orgID) {
			delete(orgTermCache, k)
		}
	}
	orgTermCacheMu.Unlock()

	// Return updated list
	return a.ListOrgAlertTerms(ctx, orgID)
}

func (a *API) handleOrgAlertTermsList(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	items, apiErr := a.ListOrgAlertTerms(r.Context(), orgID)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}
	writeData(w, http.StatusOK, items)
}

func (a *API) handleOrgAlertTermsUpdate(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")

	var body []struct {
		Term   string `json:"term"`
		Weight int    `json:"weight"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Expected JSON array of {term, weight} objects")
		return
	}

	items, apiErr := a.UpdateOrgAlertTerms(r.Context(), orgID, body)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}
	writeData(w, http.StatusOK, items)
}

func orgToMap(o *storage.Organization, memberCount, edvCount int) map[string]interface{} {
	m := map[string]interface{}{
		"id":                  o.ID,
		"name":                o.Name,
		"description":         o.Description,
		"status":              o.Status,
		"taskschmied_enabled": o.TaskschmiedEnabled,
		"metadata":            o.Metadata,
		"created_at":          o.CreatedAt.Format(time.RFC3339),
		"updated_at":          o.UpdatedAt.Format(time.RFC3339),
	}
	if memberCount > 0 {
		m["member_count"] = memberCount
	}
	if edvCount > 0 {
		m["endeavour_count"] = edvCount
	}
	return m
}
