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
	"github.com/QuestFinTech/taskschmiede/internal/storage"
)

// CreateUser creates a new user and returns its map representation.
// Admin check is NOT performed here -- the HTTP handler must enforce it.
func (a *API) CreateUser(ctx context.Context, name, email, password string, resourceID, externalID *string, tier int, userType string, metadata map[string]interface{}) (map[string]interface{}, *APIError) {
	user, err := a.usrSvc.Create(ctx, name, email, password, resourceID, externalID, tier, userType, metadata)
	if err != nil {
		if errors.Is(err, storage.ErrEmailExists) {
			return nil, errConflict("Email already exists")
		}
		return nil, errInvalidInput(err.Error())
	}
	return userToMap(user), nil
}

// GetUser retrieves a user by ID, including their organizations, and returns a map representation.
func (a *API) GetUser(ctx context.Context, id string) (map[string]interface{}, *APIError) {
	// RBAC: self or admin
	authUser := auth.GetAuthUser(ctx)
	if authUser == nil {
		return nil, errUnauthorized("Authentication required")
	}
	if authUser.UserID != id {
		if apiErr := a.CheckAdmin(ctx); apiErr != nil {
			return nil, errNotFound("user", "User not found")
		}
	}
	user, err := a.usrSvc.Get(ctx, id)
	if err != nil {
		if errors.Is(err, storage.ErrUserNotFound) {
			return nil, errNotFound("user", "User not found")
		}
		return nil, errInternal("Failed to get user")
	}

	result := userToMap(user)

	orgs, _ := a.authSvc.GetUserOrganizations(ctx, id)
	if orgs != nil {
		result["organizations"] = orgs
	}

	return result, nil
}

// ListUsers queries users with the given options and returns their map representations.
// Admin can list all users. Non-admin can list users scoped to their own organization.
func (a *API) ListUsers(ctx context.Context, opts storage.ListUsersOpts, adminMode bool) ([]map[string]interface{}, int, *APIError) {
	scope, apiErr := a.resolveScope(ctx)
	if apiErr != nil {
		return nil, 0, apiErr
	}
	if !adminMode || !scope.IsMasterAdmin {
		// Non-admin: require organization_id filter and caller must be a member.
		if opts.OrganizationID == "" {
			return nil, 0, errForbidden("Organization filter required (admin access needed for unscoped listing)")
		}
		if _, ok := scope.Organizations[opts.OrganizationID]; !ok {
			return nil, 0, errNotFound("organization", "Organization not found")
		}
	}
	users, total, err := a.usrSvc.List(ctx, opts)
	if err != nil {
		return nil, 0, errInternal("Failed to query users")
	}

	items := make([]map[string]interface{}, 0, len(users))
	for _, u := range users {
		items = append(items, userToMap(u))
	}
	return items, total, nil
}

// UpdateUser updates a user and returns the updated map representation.
func (a *API) UpdateUser(ctx context.Context, id string, fields storage.UpdateUserFields) (map[string]interface{}, *APIError) {
	_, err := a.usrSvc.Update(ctx, id, fields)
	if err != nil {
		if errors.Is(err, storage.ErrUserNotFound) {
			return nil, errNotFound("user", "User not found")
		}
		if errors.Is(err, storage.ErrEmailExists) {
			return nil, errConflict("Email already exists")
		}
		return nil, errInvalidInput(err.Error())
	}

	user, err := a.usrSvc.Get(ctx, id)
	if err != nil {
		return nil, errInternal("Failed to get updated user")
	}
	return userToMap(user), nil
}

// AddUserToEndeavour grants a user access to an endeavour with the given role.
func (a *API) AddUserToEndeavour(ctx context.Context, userID, endeavourID, role string) (map[string]interface{}, *APIError) {
	// RBAC: require admin access to the endeavour
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
		"user_id":      userID,
		"endeavour_id": endeavourID,
		"role":         role,
	}, nil
}

func (a *API) handleUserCreate(w http.ResponseWriter, r *http.Request) {
	if !a.requireAdmin(w, r) {
		return
	}

	var body struct {
		Name       string                 `json:"name"`
		Email      string                 `json:"email"`
		Password   string                 `json:"password"`
		ResourceID *string                `json:"resource_id"`
		ExternalID *string                `json:"external_id"`
		Metadata   map[string]interface{} `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Invalid JSON body")
		return
	}

	result, apiErr := a.CreateUser(r.Context(), body.Name, body.Email, body.Password, body.ResourceID, body.ExternalID, 1, "human", body.Metadata)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	writeData(w, http.StatusCreated, result)
}

func (a *API) handleUserList(w http.ResponseWriter, r *http.Request) {
	opts := storage.ListUsersOpts{
		Status:         queryString(r, "status"),
		OrganizationID: queryString(r, "organization_id"),
		UserType:       queryString(r, "user_type"),
		Search:         queryString(r, "search"),
		Limit:          queryInt(r, "limit", 50),
		Offset:         queryInt(r, "offset", 0),
	}

	adminMode := queryString(r, "admin") == "true"
	items, total, apiErr := a.ListUsers(r.Context(), opts, adminMode)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	writeList(w, items, total, opts.Limit, opts.Offset)
}

func (a *API) handleUserGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	result, apiErr := a.GetUser(r.Context(), id)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	writeData(w, http.StatusOK, result)
}

func (a *API) handleUserUpdate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	// Users can only update their own profile unless they are a master admin.
	authUser := auth.GetAuthUser(r.Context())
	if authUser == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	if authUser.UserID != id && !a.authSvc.IsMasterAdmin(r.Context(), authUser.UserID) {
		writeError(w, http.StatusForbidden, "forbidden", "You can only update your own profile")
		return
	}

	var body struct {
		Name       *string                `json:"name"`
		Email      *string                `json:"email"`
		Status     *string                `json:"status"`
		ResourceID *string                `json:"resource_id"`
		ExternalID *string                `json:"external_id"`
		EmailCopy  *bool                  `json:"email_copy"`
		IsAdmin    *bool                  `json:"is_admin"`
		Metadata   map[string]interface{} `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Invalid JSON body")
		return
	}

	isMasterAdmin := a.authSvc.IsMasterAdmin(r.Context(), authUser.UserID)

	// Privileged fields require master admin.
	if !isMasterAdmin {
		if body.IsAdmin != nil {
			writeError(w, http.StatusForbidden, "forbidden", "Only master admins can change admin privileges")
			return
		}
		if body.Status != nil {
			writeError(w, http.StatusForbidden, "forbidden", "Only master admins can change user status")
			return
		}
		if body.ResourceID != nil {
			writeError(w, http.StatusForbidden, "forbidden", "Only master admins can change resource links")
			return
		}
		if body.ExternalID != nil {
			writeError(w, http.StatusForbidden, "forbidden", "Only master admins can change external IDs")
			return
		}
		if body.Metadata != nil {
			writeError(w, http.StatusForbidden, "forbidden", "Only master admins can change user metadata")
			return
		}
	}

	// Prevent self-demotion (lockout protection).
	if body.IsAdmin != nil && authUser.UserID == id && !*body.IsAdmin {
		writeError(w, http.StatusBadRequest, "invalid_input", "You cannot remove your own master admin privileges")
		return
	}

	// Prevent self-suspension/deactivation (lockout protection).
	if body.Status != nil && authUser.UserID == id && *body.Status != "active" {
		writeError(w, http.StatusBadRequest, "self_action", "You cannot suspend or deactivate your own account")
		return
	}

	fields := storage.UpdateUserFields{
		Name:       body.Name,
		Email:      body.Email,
		Status:     body.Status,
		ResourceID: body.ResourceID,
		ExternalID: body.ExternalID,
		EmailCopy:  body.EmailCopy,
		IsAdmin:    body.IsAdmin,
		Metadata:   body.Metadata,
	}

	result, apiErr := a.UpdateUser(r.Context(), id, fields)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	writeData(w, http.StatusOK, result)
}

func userToMap(u *storage.User) map[string]interface{} {
	m := map[string]interface{}{
		"id":         u.ID,
		"name":       u.Name,
		"email":      u.Email,
		"status":     u.Status,
		"is_admin":     u.IsAdmin,
		"login_count":  u.LoginCount,
		"tier":         u.Tier,
		"user_type":  u.UserType,
		"lang":       u.Lang,
		"email_copy": u.EmailCopy,
		"metadata":   u.Metadata,
		"created_at": u.CreatedAt.Format(time.RFC3339),
		"updated_at": u.UpdatedAt.Format(time.RFC3339),
	}
	if u.ResourceID != nil {
		m["resource_id"] = *u.ResourceID
	}
	if u.ExternalID != nil {
		m["external_id"] = *u.ExternalID
	}
	return m
}
