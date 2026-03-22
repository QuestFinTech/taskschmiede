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
	"github.com/QuestFinTech/taskschmiede/internal/service"
	"github.com/QuestFinTech/taskschmiede/internal/storage"
)

// --- Exported business logic methods ---

// CreateDodPolicy creates a new DoD policy.
func (a *API) CreateDodPolicy(ctx context.Context, name, description, origin, createdBy string, conditions []storage.DodCondition, strictness string, quorum int, metadata map[string]interface{}) (map[string]interface{}, *APIError) {
	if apiErr := validateDodCreate(name, description, metadata); apiErr != nil {
		return nil, apiErr
	}
	// RBAC: admin only for creating global DoD policies
	if apiErr := a.CheckAdmin(ctx); apiErr != nil {
		return nil, apiErr
	}
	policy, err := a.dodSvc.Create(ctx, name, description, origin, createdBy, conditions, strictness, quorum, metadata)
	if err != nil {
		return nil, errInvalidInput(err.Error())
	}
	return dodPolicyToMap(policy), nil
}

// GetDodPolicy retrieves a DoD policy by ID.
func (a *API) GetDodPolicy(ctx context.Context, id string) (map[string]interface{}, *APIError) {
	policy, err := a.dodSvc.Get(ctx, id)
	if err != nil {
		if errors.Is(err, storage.ErrDodPolicyNotFound) {
			return nil, errNotFound("dod_policy", "DoD policy not found")
		}
		return nil, errInternal("Failed to get DoD policy")
	}
	return dodPolicyToMap(policy), nil
}

// ListDodPolicies returns a paginated list of DoD policies.
func (a *API) ListDodPolicies(ctx context.Context, opts storage.ListDodPoliciesOpts) ([]map[string]interface{}, int, *APIError) {
	policies, total, err := a.dodSvc.List(ctx, opts)
	if err != nil {
		return nil, 0, errInternal("Failed to query DoD policies")
	}

	items := make([]map[string]interface{}, 0, len(policies))
	for _, p := range policies {
		items = append(items, dodPolicyToMap(p))
	}
	return items, total, nil
}

// UpdateDodPolicy updates a DoD policy's metadata fields.
func (a *API) UpdateDodPolicy(ctx context.Context, id string, fields storage.UpdateDodPolicyFields) (map[string]interface{}, *APIError) {
	if apiErr := validateDodUpdate(id, fields); apiErr != nil {
		return nil, apiErr
	}
	// RBAC: admin only for modifying global DoD policies
	if apiErr := a.CheckAdmin(ctx); apiErr != nil {
		return nil, apiErr
	}
	_, err := a.dodSvc.Update(ctx, id, fields)
	if err != nil {
		if errors.Is(err, storage.ErrDodPolicyNotFound) {
			return nil, errNotFound("dod_policy", "DoD policy not found")
		}
		return nil, errInvalidInput(err.Error())
	}

	policy, err := a.dodSvc.Get(ctx, id)
	if err != nil {
		return nil, errInternal("Failed to get updated DoD policy")
	}
	return dodPolicyToMap(policy), nil
}

// NewDodPolicyVersion creates a new version of an existing policy.
func (a *API) NewDodPolicyVersion(ctx context.Context, id, name, description string, conditions []storage.DodCondition, strictness string, quorum int, metadata map[string]interface{}, createdBy string) (map[string]interface{}, *APIError) {
	if apiErr := validateDodCreate(name, description, metadata); apiErr != nil {
		return nil, apiErr
	}
	if apiErr := validateEntityID(id, "id"); apiErr != nil {
		return nil, apiErr
	}
	// RBAC: admin only for versioning global DoD policies
	if apiErr := a.CheckAdmin(ctx); apiErr != nil {
		return nil, apiErr
	}
	policy, err := a.dodSvc.NewVersion(ctx, id, name, description, conditions, strictness, quorum, metadata, createdBy)
	if err != nil {
		if errors.Is(err, storage.ErrDodPolicyNotFound) {
			return nil, errNotFound("dod_policy", "DoD policy not found")
		}
		return nil, errInvalidInput(err.Error())
	}
	return dodPolicyToMap(policy), nil
}

// GetDodPolicyLineage returns the version chain for a policy.
func (a *API) GetDodPolicyLineage(ctx context.Context, id string) ([]map[string]interface{}, *APIError) {
	policies, err := a.dodSvc.Lineage(ctx, id)
	if err != nil {
		if errors.Is(err, storage.ErrDodPolicyNotFound) {
			return nil, errNotFound("dod_policy", "DoD policy not found")
		}
		return nil, errInternal("Failed to get DoD policy lineage")
	}

	items := make([]map[string]interface{}, 0, len(policies))
	for _, p := range policies {
		items = append(items, dodPolicyToMap(p))
	}
	return items, nil
}

// AssignDodPolicy assigns a DoD policy to an endeavour.
func (a *API) AssignDodPolicy(ctx context.Context, endeavourID, policyID, assignedBy string) (map[string]interface{}, *APIError) {
	// RBAC: require admin access to the endeavour
	scope, apiErr := a.resolveScope(ctx)
	if apiErr != nil {
		return nil, apiErr
	}
	if apiErr := checkEndeavourAdmin(scope, endeavourID); apiErr != nil {
		return nil, errNotFound("endeavour", "Endeavour not found")
	}
	err := a.dodSvc.Assign(ctx, endeavourID, policyID, assignedBy)
	if err != nil {
		if errors.Is(err, storage.ErrEndeavourNotFound) {
			return nil, errNotFound("endeavour", "Endeavour not found")
		}
		if errors.Is(err, storage.ErrDodPolicyNotFound) {
			return nil, errNotFound("dod_policy", "DoD policy not found")
		}
		return nil, errInvalidInput(err.Error())
	}
	return map[string]interface{}{
		"endeavour_id": endeavourID,
		"policy_id":    policyID,
		"status":       "assigned",
	}, nil
}

// UnassignDodPolicy removes the DoD policy from an endeavour.
func (a *API) UnassignDodPolicy(ctx context.Context, endeavourID, removedBy string) (map[string]interface{}, *APIError) {
	// RBAC: require admin access to the endeavour
	scope, apiErr := a.resolveScope(ctx)
	if apiErr != nil {
		return nil, apiErr
	}
	if apiErr := checkEndeavourAdmin(scope, endeavourID); apiErr != nil {
		return nil, errNotFound("endeavour", "Endeavour not found")
	}
	err := a.dodSvc.Unassign(ctx, endeavourID, removedBy)
	if err != nil {
		return nil, errInvalidInput(err.Error())
	}
	return map[string]interface{}{
		"endeavour_id": endeavourID,
		"status":       "unassigned",
	}, nil
}

// EndorseDodPolicy records a resource's endorsement of the current policy for an endeavour.
func (a *API) EndorseDodPolicy(ctx context.Context, resourceID, endeavourID string) (map[string]interface{}, *APIError) {
	// RBAC: require write access (member+) to the endeavour
	scope, apiErr := a.resolveScope(ctx)
	if apiErr != nil {
		return nil, apiErr
	}
	if apiErr := checkEndeavourWrite(scope, endeavourID); apiErr != nil {
		return nil, errNotFound("endeavour", "Endeavour not found")
	}
	endorsement, err := a.dodSvc.Endorse(ctx, resourceID, endeavourID)
	if err != nil {
		if errors.Is(err, storage.ErrDodEndorsementExists) {
			return nil, errConflict("Active endorsement already exists for this resource and endeavour")
		}
		return nil, errInvalidInput(err.Error())
	}
	return dodEndorsementToMap(endorsement), nil
}

// ListDodEndorsements returns endorsements with filters.
func (a *API) ListDodEndorsements(ctx context.Context, opts storage.ListDodEndorsementsOpts) ([]map[string]interface{}, int, *APIError) {
	endorsements, total, err := a.dodSvc.ListEndorsements(ctx, opts)
	if err != nil {
		return nil, 0, errInternal("Failed to query endorsements")
	}

	items := make([]map[string]interface{}, 0, len(endorsements))
	for _, e := range endorsements {
		items = append(items, dodEndorsementToMap(e))
	}
	return items, total, nil
}

// CheckDod evaluates DoD conditions for a task (dry run).
func (a *API) CheckDod(ctx context.Context, taskID, resourceID string) (map[string]interface{}, *APIError) {
	// RBAC: require read access to the task's endeavour
	task, taskErr := a.tskSvc.Get(ctx, taskID)
	if taskErr != nil {
		return nil, errNotFound("task", "Task not found")
	}
	scope, apiErr := a.resolveScope(ctx)
	if apiErr != nil {
		return nil, apiErr
	}
	if apiErr := checkEndeavourRead(scope, task.EndeavourID); apiErr != nil {
		return nil, errNotFound("task", "Task not found")
	}
	result, err := a.dodSvc.Check(ctx, taskID, resourceID)
	if err != nil {
		if errors.Is(err, storage.ErrTaskNotFound) {
			return nil, errNotFound("task", "Task not found")
		}
		return nil, errInternal("Failed to check DoD")
	}
	if result == nil {
		return map[string]interface{}{
			"result": "no_policy",
			"hint":   "No DoD policy applies to this task",
		}, nil
	}
	return checkResultToMap(result), nil
}

// OverrideDod applies a DoD override to a task.
func (a *API) OverrideDod(ctx context.Context, taskID, resourceID, reason, source string) (map[string]interface{}, *APIError) {
	// RBAC: require admin access to the task's endeavour
	task, taskErr := a.tskSvc.Get(ctx, taskID)
	if taskErr != nil {
		return nil, errNotFound("task", "Task not found")
	}
	scope, apiErr := a.resolveScope(ctx)
	if apiErr != nil {
		return nil, apiErr
	}
	if apiErr := checkEndeavourAdmin(scope, task.EndeavourID); apiErr != nil {
		return nil, errNotFound("task", "Task not found")
	}
	// Collect conditions that will be bypassed (for audit trail).
	var conditionsBypassed []string
	checkResult, _ := a.dodSvc.Check(ctx, taskID, resourceID)
	if checkResult != nil {
		for _, c := range checkResult.Conditions {
			if c.Status == "failed" {
				conditionsBypassed = append(conditionsBypassed, c.Type)
			}
		}
	}

	err := a.dodSvc.Override(ctx, taskID, resourceID, reason)
	if err != nil {
		if errors.Is(err, storage.ErrTaskNotFound) {
			return nil, errNotFound("task", "Task not found")
		}
		return nil, errInvalidInput(err.Error())
	}

	// Record in audit log per design spec.
	if a.auditSvc != nil {
		auditMeta := map[string]interface{}{
			"task_id":     taskID,
			"reason":      reason,
			"resource_id": resourceID,
		}
		if checkResult != nil {
			auditMeta["policy_id"] = checkResult.PolicyID
			auditMeta["policy_name"] = checkResult.PolicyName
		}
		if len(conditionsBypassed) > 0 {
			auditMeta["conditions_bypassed"] = conditionsBypassed
		}
		a.auditSvc.Log(&security.AuditEntry{
			Action:   security.AuditDodOverride,
			ActorID:  resourceID,
			Resource: "task:" + taskID,
			Source:   source,
			Metadata: auditMeta,
		})
	}

	return map[string]interface{}{
		"task_id":     taskID,
		"resource_id": resourceID,
		"status":      "overridden",
		"reason":      reason,
	}, nil
}

// GetDodStatus returns the DoD policy and endorsement status for an endeavour.
func (a *API) GetDodStatus(ctx context.Context, endeavourID string) (map[string]interface{}, *APIError) {
	// RBAC: require read access to the endeavour
	scope, apiErr := a.resolveScope(ctx)
	if apiErr != nil {
		return nil, apiErr
	}
	if apiErr := checkEndeavourRead(scope, endeavourID); apiErr != nil {
		return nil, errNotFound("endeavour", "Endeavour not found")
	}
	policy, err := a.dodSvc.GetEndeavourPolicy(ctx, endeavourID)
	if err != nil {
		return nil, errInternal("Failed to get DoD status")
	}

	result := map[string]interface{}{
		"endeavour_id": endeavourID,
	}

	if policy == nil {
		result["policy"] = nil
		result["endorsements"] = []interface{}{}
		return result, nil
	}

	result["policy"] = dodPolicyToMap(policy)

	endorsements, _, _ := a.dodSvc.ListEndorsements(ctx, storage.ListDodEndorsementsOpts{
		PolicyID:    policy.ID,
		EndeavourID: endeavourID,
		Status:      "active",
		Limit:       100,
	})

	endorsementItems := make([]map[string]interface{}, 0, len(endorsements))
	for _, e := range endorsements {
		endorsementItems = append(endorsementItems, dodEndorsementToMap(e))
	}
	result["endorsements"] = endorsementItems

	return result, nil
}

// --- HTTP handlers ---

func (a *API) handleDodPolicyCreate(w http.ResponseWriter, r *http.Request) {
	resourceID, apiErr := a.resolveCallerResourceID(r.Context())
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	var body struct {
		Name        string                 `json:"name"`
		Description string                 `json:"description"`
		Origin      string                 `json:"origin"`
		Conditions  []storage.DodCondition `json:"conditions"`
		Strictness  string                 `json:"strictness"`
		Quorum      int                    `json:"quorum"`
		Metadata    map[string]interface{} `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Invalid JSON body")
		return
	}

	result, apiErr := a.CreateDodPolicy(r.Context(), body.Name, body.Description, body.Origin, resourceID, body.Conditions, body.Strictness, body.Quorum, body.Metadata)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}
	writeData(w, http.StatusCreated, result)
}

func (a *API) handleDodPolicyList(w http.ResponseWriter, r *http.Request) {
	opts := storage.ListDodPoliciesOpts{
		Status: queryString(r, "status"),
		Origin: queryString(r, "origin"),
		Scope:  queryString(r, "scope"),
		Search: queryString(r, "search"),
		Limit:  queryInt(r, "limit", 50),
		Offset: queryInt(r, "offset", 0),
	}

	items, total, apiErr := a.ListDodPolicies(r.Context(), opts)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}
	writeList(w, items, total, opts.Limit, opts.Offset)
}

func (a *API) handleDodPolicyGet(w http.ResponseWriter, r *http.Request) {
	result, apiErr := a.GetDodPolicy(r.Context(), r.PathValue("id"))
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}
	writeData(w, http.StatusOK, result)
}

func (a *API) handleDodPolicyUpdate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var body struct {
		Name        *string                `json:"name"`
		Description *string                `json:"description"`
		Status      *string                `json:"status"`
		Metadata    map[string]interface{} `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Invalid JSON body")
		return
	}

	fields := storage.UpdateDodPolicyFields{
		Name:        body.Name,
		Description: body.Description,
		Status:      body.Status,
		Metadata:    body.Metadata,
	}

	result, apiErr := a.UpdateDodPolicy(r.Context(), id, fields)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}
	writeData(w, http.StatusOK, result)
}

func (a *API) handleDodPolicyNewVersion(w http.ResponseWriter, r *http.Request) {
	resourceID, apiErr := a.resolveCallerResourceID(r.Context())
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	var body struct {
		Name        string                 `json:"name"`
		Description string                 `json:"description"`
		Conditions  []storage.DodCondition `json:"conditions"`
		Strictness  string                 `json:"strictness"`
		Quorum      int                    `json:"quorum"`
		Metadata    map[string]interface{} `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Invalid JSON body")
		return
	}

	result, apiErr := a.NewDodPolicyVersion(r.Context(), r.PathValue("id"), body.Name, body.Description, body.Conditions, body.Strictness, body.Quorum, body.Metadata, resourceID)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}
	writeData(w, http.StatusCreated, result)
}

func (a *API) handleDodPolicyLineage(w http.ResponseWriter, r *http.Request) {
	items, apiErr := a.GetDodPolicyLineage(r.Context(), r.PathValue("id"))
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}
	writeData(w, http.StatusOK, items)
}

func (a *API) handleDodAssign(w http.ResponseWriter, r *http.Request) {
	resourceID, apiErr := a.resolveCallerResourceID(r.Context())
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	var body struct {
		PolicyID string `json:"policy_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Invalid JSON body")
		return
	}

	result, apiErr := a.AssignDodPolicy(r.Context(), r.PathValue("id"), body.PolicyID, resourceID)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}
	writeData(w, http.StatusOK, result)
}

func (a *API) handleDodUnassign(w http.ResponseWriter, r *http.Request) {
	resourceID, apiErr := a.resolveCallerResourceID(r.Context())
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	result, apiErr := a.UnassignDodPolicy(r.Context(), r.PathValue("id"), resourceID)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}
	writeData(w, http.StatusOK, result)
}

func (a *API) handleDodStatus(w http.ResponseWriter, r *http.Request) {
	result, apiErr := a.GetDodStatus(r.Context(), r.PathValue("id"))
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}
	writeData(w, http.StatusOK, result)
}

func (a *API) handleDodEndorse(w http.ResponseWriter, r *http.Request) {
	resourceID, apiErr := a.resolveCallerResourceID(r.Context())
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	var body struct {
		EndeavourID string `json:"endeavour_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Invalid JSON body")
		return
	}

	result, apiErr := a.EndorseDodPolicy(r.Context(), resourceID, body.EndeavourID)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}
	writeData(w, http.StatusCreated, result)
}

func (a *API) handleDodEndorsementList(w http.ResponseWriter, r *http.Request) {
	opts := storage.ListDodEndorsementsOpts{
		PolicyID:     queryString(r, "policy_id"),
		ResourceID:   queryString(r, "resource_id"),
		EndeavourID:  queryString(r, "endeavour_id"),
		EndeavourIDs: a.resolveEndeavourIDs(r),
		Status:       queryString(r, "status"),
		Limit:        queryInt(r, "limit", 50),
		Offset:       queryInt(r, "offset", 0),
	}

	items, total, apiErr := a.ListDodEndorsements(r.Context(), opts)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}
	writeList(w, items, total, opts.Limit, opts.Offset)
}

func (a *API) handleDodCheck(w http.ResponseWriter, r *http.Request) {
	resourceID, _ := a.resolveCallerResourceID(r.Context())

	result, apiErr := a.CheckDod(r.Context(), r.PathValue("id"), resourceID)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}
	writeData(w, http.StatusOK, result)
}

func (a *API) handleDodOverride(w http.ResponseWriter, r *http.Request) {
	resourceID, apiErr := a.resolveCallerResourceID(r.Context())
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	var body struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Invalid JSON body")
		return
	}

	result, apiErr := a.OverrideDod(r.Context(), r.PathValue("id"), resourceID, body.Reason, auditSource(r))
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}
	writeData(w, http.StatusOK, result)
}

// --- Mapping helpers ---

func dodPolicyToMap(p *storage.DodPolicy) map[string]interface{} {
	m := map[string]interface{}{
		"id":          p.ID,
		"name":        p.Name,
		"description": p.Description,
		"version":     p.Version,
		"origin":      p.Origin,
		"conditions":  p.Conditions,
		"strictness":  p.Strictness,
		"scope":       p.Scope,
		"status":      p.Status,
		"created_by":  p.CreatedBy,
		"metadata":    p.Metadata,
		"created_at":  p.CreatedAt.Format(time.RFC3339),
		"updated_at":  p.UpdatedAt.Format(time.RFC3339),
	}
	if p.PredecessorID != "" {
		m["predecessor_id"] = p.PredecessorID
	}
	if p.Quorum > 0 {
		m["quorum"] = p.Quorum
	}
	return m
}

func dodEndorsementToMap(e *storage.DodEndorsement) map[string]interface{} {
	m := map[string]interface{}{
		"id":             e.ID,
		"policy_id":      e.PolicyID,
		"policy_version": e.PolicyVersion,
		"resource_id":    e.ResourceID,
		"endeavour_id":   e.EndeavourID,
		"status":         e.Status,
		"endorsed_at":    e.EndorsedAt.Format(time.RFC3339),
		"created_at":     e.CreatedAt.Format(time.RFC3339),
	}
	if e.SupersededAt != nil {
		m["superseded_at"] = e.SupersededAt.Format(time.RFC3339)
	}
	return m
}

func checkResultToMap(r *service.CheckResult) map[string]interface{} {
	conditions := make([]map[string]interface{}, 0, len(r.Conditions))
	for _, c := range r.Conditions {
		cm := map[string]interface{}{
			"id":     c.ID,
			"type":   c.Type,
			"label":  c.Label,
			"status": c.Status,
		}
		if c.Detail != "" {
			cm["detail"] = c.Detail
		}
		if c.Hint != "" {
			cm["hint"] = c.Hint
		}
		conditions = append(conditions, cm)
	}

	m := map[string]interface{}{
		"policy_id":      r.PolicyID,
		"policy_name":    r.PolicyName,
		"policy_version": r.PolicyVersion,
		"result":         r.Result,
		"conditions":     conditions,
	}
	if r.Hint != "" {
		m["hint"] = r.Hint
	}
	return m
}
