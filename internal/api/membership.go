// Copyright 2026 Quest Financial Technologies S.a r.l.-S., Luxembourg
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

	"github.com/QuestFinTech/taskschmiede/internal/storage"
)

// ---------------------------------------------------------------------------
// Organization membership (has_member: organization -> resource, role in metadata)
// ---------------------------------------------------------------------------

// AddOrgMember adds a user to an organization with the given role.
// Resolves user ID to resource ID internally.
func (a *API) AddOrgMember(ctx context.Context, orgID, userID, role string) (map[string]interface{}, *APIError) {
	if orgID == "" || userID == "" {
		return nil, errInvalidInput("organization_id and user_id are required")
	}
	if role == "" {
		role = "member"
	}

	// RBAC: require admin access to org
	scope, apiErr := a.resolveScope(ctx)
	if apiErr != nil {
		return nil, apiErr
	}
	if !scope.CanAdminOrg(orgID) {
		return nil, errNotFound("organization", "Organization not found")
	}

	// Resolve user -> resource
	user, err := a.usrSvc.Get(ctx, userID)
	if err != nil {
		if errors.Is(err, storage.ErrUserNotFound) {
			return nil, errNotFound("user", "User not found")
		}
		return nil, errInternal("Failed to get user")
	}
	if user.ResourceID == nil || *user.ResourceID == "" {
		return nil, errInvalidInput("User has no linked resource")
	}

	if err := a.orgSvc.AddResource(ctx, orgID, *user.ResourceID, role); err != nil {
		return nil, errInvalidInput(err.Error())
	}

	return map[string]interface{}{
		"organization_id": orgID,
		"user_id":         userID,
		"resource_id":     *user.ResourceID,
		"role":            role,
	}, nil
}

// RemoveOrgMember removes a user from an organization.
func (a *API) RemoveOrgMember(ctx context.Context, orgID, userID string) (map[string]interface{}, *APIError) {
	if orgID == "" || userID == "" {
		return nil, errInvalidInput("organization_id and user_id are required")
	}

	// RBAC: require admin access to org
	scope, apiErr := a.resolveScope(ctx)
	if apiErr != nil {
		return nil, apiErr
	}
	if !scope.CanAdminOrg(orgID) {
		return nil, errNotFound("organization", "Organization not found")
	}

	// Resolve user -> resource
	user, err := a.usrSvc.Get(ctx, userID)
	if err != nil {
		if errors.Is(err, storage.ErrUserNotFound) {
			return nil, errNotFound("user", "User not found")
		}
		return nil, errInternal("Failed to get user")
	}
	if user.ResourceID == nil || *user.ResourceID == "" {
		return nil, errNotFound("member", "User is not a member (no linked resource)")
	}

	// Delete the has_member relation by endpoints
	if err := a.db.DeleteRelationByEndpoints(
		storage.RelHasMember,
		storage.EntityOrganization, orgID,
		storage.EntityResource, *user.ResourceID,
	); err != nil {
		if errors.Is(err, storage.ErrRelationNotFound) {
			return nil, errNotFound("member", "User is not a member of this organization")
		}
		return nil, errInternal("Failed to remove member")
	}

	return map[string]interface{}{
		"removed":         true,
		"organization_id": orgID,
		"user_id":         userID,
	}, nil
}

// ListOrgMembers lists members of an organization with their roles.
func (a *API) ListOrgMembers(ctx context.Context, orgID string) ([]map[string]interface{}, *APIError) {
	if orgID == "" {
		return nil, errInvalidInput("organization_id is required")
	}

	// RBAC: require membership in org (any role)
	scope, apiErr := a.resolveScope(ctx)
	if apiErr != nil {
		return nil, apiErr
	}
	if !scope.CanReadOrg(orgID) {
		return nil, errNotFound("organization", "Organization not found")
	}

	// List has_member relations where source=organization/orgID
	rels, _, err := a.db.ListRelations(storage.ListRelationsOpts{
		RelationshipType: storage.RelHasMember,
		SourceEntityType: storage.EntityOrganization,
		SourceEntityID:   orgID,
		Limit:            1000,
	})
	if err != nil {
		return nil, errInternal("Failed to list members")
	}

	members := make([]map[string]interface{}, 0, len(rels))
	for _, rel := range rels {
		resourceID := rel.TargetEntityID
		role := ""
		if rel.Metadata != nil {
			if r, ok := rel.Metadata["role"].(string); ok {
				role = r
			}
		}

		// Resolve resource ID back to user
		user, err := a.db.GetUserByResourceID(resourceID)
		if err != nil {
			// Resource exists but no linked user -- include with resource info only
			members = append(members, map[string]interface{}{
				"resource_id": resourceID,
				"role":        role,
			})
			continue
		}

		members = append(members, map[string]interface{}{
			"user_id":     user.ID,
			"name":        user.Name,
			"email":       user.Email,
			"role":        role,
			"resource_id": resourceID,
		})
	}

	return members, nil
}

// SetOrgMemberRole changes a member's role in an organization.
func (a *API) SetOrgMemberRole(ctx context.Context, orgID, userID, role string) (map[string]interface{}, *APIError) {
	if orgID == "" || userID == "" || role == "" {
		return nil, errInvalidInput("organization_id, user_id, and role are required")
	}

	// RBAC: require admin access to org
	scope, apiErr := a.resolveScope(ctx)
	if apiErr != nil {
		return nil, apiErr
	}
	if !scope.CanAdminOrg(orgID) {
		return nil, errNotFound("organization", "Organization not found")
	}

	// Resolve user -> resource
	user, err := a.usrSvc.Get(ctx, userID)
	if err != nil {
		if errors.Is(err, storage.ErrUserNotFound) {
			return nil, errNotFound("user", "User not found")
		}
		return nil, errInternal("Failed to get user")
	}
	if user.ResourceID == nil || *user.ResourceID == "" {
		return nil, errNotFound("member", "User is not a member (no linked resource)")
	}

	// Delete the existing has_member relation
	if err := a.db.DeleteRelationByEndpoints(
		storage.RelHasMember,
		storage.EntityOrganization, orgID,
		storage.EntityResource, *user.ResourceID,
	); err != nil {
		if errors.Is(err, storage.ErrRelationNotFound) {
			return nil, errNotFound("member", "User is not a member of this organization")
		}
		return nil, errInternal("Failed to update member role")
	}

	// Create new relation with updated role
	metadata := map[string]interface{}{"role": role}
	if _, err := a.db.CreateRelation(
		storage.RelHasMember,
		storage.EntityOrganization, orgID,
		storage.EntityResource, *user.ResourceID,
		metadata, "",
	); err != nil {
		return nil, errInternal("Failed to update member role")
	}

	return map[string]interface{}{
		"organization_id": orgID,
		"user_id":         userID,
		"resource_id":     *user.ResourceID,
		"role":            role,
	}, nil
}

// ---------------------------------------------------------------------------
// Endeavour membership (member_of: user -> endeavour, role in metadata)
// ---------------------------------------------------------------------------

// AddEndeavourMember adds a user to an endeavour.
func (a *API) AddEndeavourMember(ctx context.Context, endeavourID, userID, role string) (map[string]interface{}, *APIError) {
	if endeavourID == "" || userID == "" {
		return nil, errInvalidInput("endeavour_id and user_id are required")
	}
	if role == "" {
		role = "member"
	}

	// RBAC: require endeavour admin access
	scope, apiErr := a.resolveScope(ctx)
	if apiErr != nil {
		return nil, apiErr
	}
	if apiErr := checkEndeavourAdmin(scope, endeavourID); apiErr != nil {
		return nil, errNotFound("endeavour", "Endeavour not found")
	}

	if err := a.edvSvc.AddUser(ctx, userID, endeavourID, role); err != nil {
		return nil, errInvalidInput(err.Error())
	}

	return map[string]interface{}{
		"endeavour_id": endeavourID,
		"user_id":      userID,
		"role":         role,
	}, nil
}

// RemoveEndeavourMember removes a user from an endeavour.
func (a *API) RemoveEndeavourMember(ctx context.Context, endeavourID, userID string) (map[string]interface{}, *APIError) {
	if endeavourID == "" || userID == "" {
		return nil, errInvalidInput("endeavour_id and user_id are required")
	}

	// RBAC: require endeavour admin access
	scope, apiErr := a.resolveScope(ctx)
	if apiErr != nil {
		return nil, apiErr
	}
	if apiErr := checkEndeavourAdmin(scope, endeavourID); apiErr != nil {
		return nil, errNotFound("endeavour", "Endeavour not found")
	}

	// Delete the member_of relation by endpoints
	if err := a.db.DeleteRelationByEndpoints(
		storage.RelMemberOf,
		storage.EntityUser, userID,
		storage.EntityEndeavour, endeavourID,
	); err != nil {
		if errors.Is(err, storage.ErrRelationNotFound) {
			return nil, errNotFound("member", "User is not a member of this endeavour")
		}
		return nil, errInternal("Failed to remove member")
	}

	return map[string]interface{}{
		"removed":      true,
		"endeavour_id": endeavourID,
		"user_id":      userID,
	}, nil
}

// ListEndeavourMembers lists members of an endeavour.
func (a *API) ListEndeavourMembers(ctx context.Context, endeavourID string) ([]map[string]interface{}, *APIError) {
	if endeavourID == "" {
		return nil, errInvalidInput("endeavour_id is required")
	}

	// RBAC: require membership in endeavour (any role)
	scope, apiErr := a.resolveScope(ctx)
	if apiErr != nil {
		return nil, apiErr
	}
	if apiErr := checkEndeavourRead(scope, endeavourID); apiErr != nil {
		return nil, errNotFound("endeavour", "Endeavour not found")
	}

	// List member_of relations where target=endeavour/endeavourID
	rels, _, err := a.db.ListRelations(storage.ListRelationsOpts{
		RelationshipType: storage.RelMemberOf,
		TargetEntityType: storage.EntityEndeavour,
		TargetEntityID:   endeavourID,
		Limit:            1000,
	})
	if err != nil {
		return nil, errInternal("Failed to list members")
	}

	members := make([]map[string]interface{}, 0, len(rels))
	for _, rel := range rels {
		userID := rel.SourceEntityID
		role := ""
		if rel.Metadata != nil {
			if r, ok := rel.Metadata["role"].(string); ok {
				role = r
			}
		}

		// Look up user details
		user, err := a.usrSvc.Get(ctx, userID)
		if err != nil {
			// User may have been deleted -- include with ID only
			members = append(members, map[string]interface{}{
				"user_id": userID,
				"role":    role,
			})
			continue
		}

		members = append(members, map[string]interface{}{
			"user_id": user.ID,
			"name":    user.Name,
			"email":   user.Email,
			"role":    role,
		})
	}

	return members, nil
}

// ---------------------------------------------------------------------------
// HTTP handlers (thin wrappers)
// ---------------------------------------------------------------------------

func (a *API) handleOrgAddMember(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")

	var body struct {
		UserID string `json:"user_id"`
		Role   string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Invalid JSON body")
		return
	}

	if body.Role == "" {
		body.Role = "member"
	}

	result, apiErr := a.AddOrgMember(r.Context(), orgID, sanitize(body.UserID), sanitize(body.Role))
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	writeData(w, http.StatusCreated, result)
}

func (a *API) handleOrgRemoveMember(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	userID := r.PathValue("user_id")

	result, apiErr := a.RemoveOrgMember(r.Context(), orgID, userID)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	writeData(w, http.StatusOK, result)
}

func (a *API) handleOrgListMembers(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")

	members, apiErr := a.ListOrgMembers(r.Context(), orgID)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	writeData(w, http.StatusOK, map[string]interface{}{
		"members": members,
		"total":   len(members),
	})
}

func (a *API) handleOrgSetMemberRole(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("id")
	userID := r.PathValue("user_id")

	var body struct {
		Role string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Invalid JSON body")
		return
	}

	result, apiErr := a.SetOrgMemberRole(r.Context(), orgID, userID, sanitize(body.Role))
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	writeData(w, http.StatusOK, result)
}

func (a *API) handleEdvAddMember(w http.ResponseWriter, r *http.Request) {
	edvID := r.PathValue("id")

	var body struct {
		UserID string `json:"user_id"`
		Role   string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Invalid JSON body")
		return
	}

	if body.Role == "" {
		body.Role = "member"
	}

	result, apiErr := a.AddEndeavourMember(r.Context(), edvID, sanitize(body.UserID), sanitize(body.Role))
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	writeData(w, http.StatusCreated, result)
}

func (a *API) handleEdvRemoveMember(w http.ResponseWriter, r *http.Request) {
	edvID := r.PathValue("id")
	userID := r.PathValue("user_id")

	result, apiErr := a.RemoveEndeavourMember(r.Context(), edvID, userID)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	writeData(w, http.StatusOK, result)
}

func (a *API) handleEdvListMembers(w http.ResponseWriter, r *http.Request) {
	edvID := r.PathValue("id")

	members, apiErr := a.ListEndeavourMembers(r.Context(), edvID)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	writeData(w, http.StatusOK, map[string]interface{}{
		"members": members,
		"total":   len(members),
	})
}
