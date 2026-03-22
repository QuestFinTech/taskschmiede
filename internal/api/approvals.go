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

	"github.com/QuestFinTech/taskschmiede/internal/security"
	"github.com/QuestFinTech/taskschmiede/internal/storage"
)

// --- Exported business logic methods ---

// CreateApproval records an approval on an entity. The approver is resolved
// from the authenticated user's linked resource.
func (a *API) CreateApproval(ctx context.Context, entityType, entityID, role, verdict, comment string, metadata map[string]interface{}) (map[string]interface{}, *APIError) {
	if apiErr := validateApprovalCreate(entityType, entityID, verdict, role, comment, metadata); apiErr != nil {
		return nil, apiErr
	}
	if apiErr := a.CheckCreationVelocity(ctx); apiErr != nil {
		return nil, apiErr
	}
	// Required field checks (BUG-2 fix: better error messages)
	if entityType == "" {
		return nil, errInvalidInput("entity_type is required")
	}
	if entityID == "" {
		return nil, errInvalidInput("entity_id is required")
	}
	if verdict == "" {
		return nil, errInvalidInput("verdict is required")
	}

	// RBAC: require write access to parent entity's endeavour
	if entityType != "" && entityType != "organization" {
		edvID, apiErr := a.resolveEntityEndeavourID(ctx, entityType, entityID)
		if apiErr != nil {
			return nil, apiErr
		}
		scope, apiErr := a.resolveScope(ctx)
		if apiErr != nil {
			return nil, apiErr
		}
		if apiErr := checkEndeavourWrite(scope, edvID); apiErr != nil {
			return nil, errNotFound("entity", "Not found")
		}
	}
	approverID, apiErr := a.resolveCallerResourceID(ctx)
	if apiErr != nil {
		return nil, apiErr
	}

	approval, err := a.aprSvc.Create(ctx, entityType, entityID, approverID, role, verdict, comment, metadata)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, errNotFound("entity", err.Error())
		}
		return nil, errInvalidInput(err.Error())
	}
	return approvalToMap(approval), nil
}

// GetApproval retrieves an approval by ID.
func (a *API) GetApproval(ctx context.Context, id string) (map[string]interface{}, *APIError) {
	approval, err := a.aprSvc.Get(ctx, id)
	if err != nil {
		if errors.Is(err, storage.ErrApprovalNotFound) {
			return nil, errNotFound("approval", "Approval not found")
		}
		return nil, errInternal("Failed to get approval")
	}
	// RBAC: require read access to parent entity's endeavour
	if approval.EntityType != "organization" {
		edvID, apiErr := a.resolveEntityEndeavourID(ctx, approval.EntityType, approval.EntityID)
		if apiErr != nil {
			return nil, errNotFound("approval", "Approval not found")
		}
		scope, apiErr := a.resolveScope(ctx)
		if apiErr != nil {
			return nil, apiErr
		}
		if apiErr := checkEndeavourRead(scope, edvID); apiErr != nil {
			return nil, errNotFound("approval", "Approval not found")
		}
	}
	return approvalToMap(approval), nil
}

// ListApprovals returns a paginated list of approvals for an entity.
func (a *API) ListApprovals(ctx context.Context, opts storage.ListApprovalsOpts) ([]map[string]interface{}, int, *APIError) {
	// RBAC: require read access to parent entity's endeavour
	if opts.EntityType != "" && opts.EntityID != "" && opts.EntityType != "organization" {
		edvID, apiErr := a.resolveEntityEndeavourID(ctx, opts.EntityType, opts.EntityID)
		if apiErr != nil {
			return nil, 0, apiErr
		}
		scope, apiErr := a.resolveScope(ctx)
		if apiErr != nil {
			return nil, 0, apiErr
		}
		if apiErr := checkEndeavourRead(scope, edvID); apiErr != nil {
			return nil, 0, errNotFound("entity", "Not found")
		}
	}
	// When no entity filter is provided, scope by accessible endeavours
	// to prevent cross-tenant data access. EndeavourIDs nil = no restriction (admin).
	if opts.EntityType == "" || opts.EntityID == "" {
		if opts.EndeavourIDs == nil {
			adminMode := false
			opts.EndeavourIDs = a.ResolveEndeavourIDs(ctx, adminMode)
		}
	}
	approvals, total, err := a.aprSvc.List(ctx, opts)
	if err != nil {
		return nil, 0, errInvalidInput(err.Error())
	}

	items := make([]map[string]interface{}, 0, len(approvals))
	for _, apr := range approvals {
		items = append(items, approvalToMap(apr))
	}
	return items, total, nil
}

// --- Helpers ---

func approvalToMap(a *storage.Approval) map[string]interface{} {
	m := map[string]interface{}{
		"id":          a.ID,
		"entity_type": a.EntityType,
		"entity_id":   a.EntityID,
		"approver_id": a.ApproverID,
		"verdict":     a.Verdict,
		"metadata":    a.Metadata,
		"created_at":  a.CreatedAt.Format(time.RFC3339),
	}
	if a.ApproverName != "" {
		m["approver_name"] = a.ApproverName
	}
	if a.Role != "" {
		m["role"] = a.Role
	}
	if a.Comment != "" {
		m["comment"] = a.Comment
	}
	return m
}

// --- HTTP handlers (thin wrappers) ---

func (a *API) handleApprovalCreate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		EntityType string                 `json:"entity_type"`
		EntityID   string                 `json:"entity_id"`
		Role       string                 `json:"role"`
		Verdict    string                 `json:"verdict"`
		Comment    string                 `json:"comment"`
		Metadata   map[string]interface{} `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Invalid JSON body")
		return
	}

	result, apiErr := a.CreateApproval(r.Context(), sanitize(body.EntityType), sanitize(body.EntityID), sanitize(body.Role), sanitize(body.Verdict), sanitize(body.Comment), security.SanitizeMap(body.Metadata))
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	writeData(w, http.StatusCreated, result)
}

func (a *API) handleApprovalList(w http.ResponseWriter, r *http.Request) {
	opts := storage.ListApprovalsOpts{
		EntityType: queryString(r, "entity_type"),
		EntityID:   queryString(r, "entity_id"),
		ApproverID: queryString(r, "approver_id"),
		Verdict:    queryString(r, "verdict"),
		Role:       queryString(r, "role"),
		Limit:      queryInt(r, "limit", 50),
		Offset:     queryInt(r, "offset", 0),
	}

	items, total, apiErr := a.ListApprovals(r.Context(), opts)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	writeList(w, items, total, opts.Limit, opts.Offset)
}

func (a *API) handleApprovalGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	result, apiErr := a.GetApproval(r.Context(), id)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	writeData(w, http.StatusOK, result)
}
