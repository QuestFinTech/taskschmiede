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


package mcp

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/QuestFinTech/taskschmiede/internal/auth"
	"github.com/QuestFinTech/taskschmiede/internal/storage"
)

// handleUserCreate handles the ts.usr.create tool.
// This handler stays in the MCP layer (Tier 2) because the self-registration
// flow with org tokens + resource creation has no REST equivalent.
func (s *Server) handleUserCreate(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := parseArgs(req)
	name := getString(args, "name")
	email := getString(args, "email")
	password := getString(args, "password")
	organizationID := getString(args, "organization_id")
	orgToken := getString(args, "org_token")
	resourceID := getString(args, "resource_id")
	externalID := getString(args, "external_id")

	if name == "" || email == "" {
		return toolError("invalid_input", "Name and email are required"), nil
	}

	// Validate email format
	if !strings.Contains(email, "@") || !strings.Contains(email, ".") {
		return toolError("invalid_input", "Invalid email format"), nil
	}

	// Determine creation mode: admin or self-registration
	authUser := getAuthUser(ctx)

	var isAdminCreation bool
	var userOrgID string

	if authUser != nil {
		// Authenticated user - check if master admin
		if s.authSvc.IsMasterAdmin(ctx, authUser.UserID) {
			isAdminCreation = true
			userOrgID = organizationID
		}
	}

	if !isAdminCreation {
		// Self-registration requires org token
		if orgToken == "" || organizationID == "" {
			return toolError("unauthorized", "Admin privileges or organization token required"), nil
		}

		// Block org-token registration in open deployment mode.
		if s.deploymentMode == "open" {
			return toolError("forbidden", "Organization token registration is not available in open deployment mode. Contact an administrator."), nil
		}

		// Validate org token via consolidated invitation_token table
		_, err := s.db.ValidateOrgInvitationToken(organizationID, orgToken)
		if err != nil {
			if errors.Is(err, storage.ErrInvitationNotFound) ||
				errors.Is(err, storage.ErrInvitationExpired) ||
				errors.Is(err, storage.ErrInvitationExhausted) ||
				errors.Is(err, storage.ErrInvitationRevoked) {
				return toolError("invalid_token", "Invalid organization token"), nil
			}
			return toolError("internal_error", "Failed to validate organization token"), nil
		}
		userOrgID = organizationID

		// Self-registration requires password
		if password == "" {
			return toolError("invalid_input", "Password is required for self-registration"), nil
		}

		// Validate password strength
		if err := auth.ValidatePassword(password); err != nil {
			return toolError("invalid_input", err.Error()), nil
		}
	}

	// Check if email already exists
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM user WHERE email = ?`, email).Scan(&count)
	if err != nil {
		return toolError("internal_error", "Failed to check email"), nil
	}
	if count > 0 {
		return toolError("conflict", "Email already exists"), nil
	}

	// Generate user ID
	idBytes := make([]byte, 8)
	if _, err := rand.Read(idBytes); err != nil {
		return toolError("internal_error", "Failed to generate user ID"), nil
	}
	userID := "usr_" + hex.EncodeToString(idBytes)

	// Hash password if provided
	var passwordHash *string
	if password != "" {
		hash, err := auth.HashPassword(password)
		if err != nil {
			return toolError("internal_error", "Failed to hash password"), nil
		}
		passwordHash = &hash
	}

	// Parse metadata
	metadataJSON := "{}"
	if metadata, ok := args["metadata"].(map[string]interface{}); ok {
		jsonBytes, _ := json.Marshal(metadata)
		metadataJSON = string(jsonBytes)
	}

	// Create user
	_, err = s.db.Exec(
		`INSERT INTO user (id, name, email, resource_id, external_id, password_hash, tier, user_type, metadata, status)
		 VALUES (?, ?, ?, ?, ?, ?, 1, 'human', ?, 'active')`,
		userID, name, email, nullString(resourceID), nullString(externalID), passwordHash, metadataJSON,
	)
	if err != nil {
		s.logger.Error("Failed to create user", "error", err)
		return toolError("internal_error", "Failed to create user"), nil
	}

	// If organization specified, add user to organization
	if userOrgID != "" {
		// First, create a resource for the user if not linking to existing
		actualResourceID := resourceID
		if actualResourceID == "" {
			// Create resource for user
			resID, err := s.createResourceForUser(ctx, userID, name, email)
			if err != nil {
				s.logger.Error("Failed to create resource for user", "error", err)
			} else {
				actualResourceID = resID
				// Update user with resource_id
				if _, err := s.db.Exec(`UPDATE user SET resource_id = ? WHERE id = ?`, resID, userID); err != nil {
					s.logger.Error("Failed to link resource to user", "user_id", userID, "resource_id", resID, "error", err)
				}
			}
		}

		// Add resource to organization via entity_relation
		if actualResourceID != "" {
			err = s.db.AddResourceToOrganization(userOrgID, actualResourceID, "member")
			if err != nil && !errors.Is(err, storage.ErrResourceAlreadyInOrg) {
				s.logger.Error("Failed to add user to organization", "error", err)
			}
		}
	}

	s.logger.Info("User created", "user_id", userID, "email", email, "admin", isAdminCreation)

	return toolSuccess(map[string]interface{}{
		"id":         userID,
		"name":       name,
		"email":      email,
		"status":     "active",
		"created_at": storage.UTCNow().Format(time.RFC3339),
	}), nil
}

// handleUserGet handles the ts.usr.get tool.
// Rewired to call the API layer for consistent behavior with REST.
func (s *Server) handleUserGet(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id := getString(parseArgs(req), "id")
	if id == "" {
		return toolError("invalid_input", "User ID is required"), nil
	}

	result, apiErr := s.api.GetUser(ctx, id)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}
	return toolSuccess(result), nil
}

// handleUserUpdate handles the ts.usr.update tool.
func (s *Server) handleUserUpdate(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	authUser, err := requireAuth(ctx)
	if err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	args := parseArgs(req)
	id := getString(args, "id")
	if id == "" {
		return toolError("invalid_input", "User ID is required"), nil
	}

	// Users can only update their own profile unless they are a master admin.
	isMasterAdmin := s.authSvc.IsMasterAdmin(ctx, authUser.UserID)
	if authUser.UserID != id && !isMasterAdmin {
		return toolError("forbidden", "You can only update your own profile"), nil
	}

	var fields storage.UpdateUserFields
	if v, ok := args["name"].(string); ok {
		fields.Name = &v
	}
	if v, ok := args["email"].(string); ok {
		fields.Email = &v
	}
	if v, ok := args["lang"].(string); ok {
		fields.Lang = &v
	}
	if v, ok := args["timezone"].(string); ok {
		fields.Timezone = &v
	}
	if v, ok := args["email_copy"].(bool); ok {
		fields.EmailCopy = &v
	}
	// Privileged fields require master admin.
	if !isMasterAdmin {
		if _, ok := args["status"]; ok {
			return toolError("forbidden", "Only master admins can change user status"), nil
		}
		if _, ok := args["metadata"]; ok {
			return toolError("forbidden", "Only master admins can change user metadata"), nil
		}
	} else {
		if v, ok := args["status"].(string); ok {
			fields.Status = &v
		}
		if v, ok := args["metadata"].(map[string]interface{}); ok {
			fields.Metadata = v
		}
	}

	result, apiErr := s.api.UpdateUser(ctx, id, fields)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}
	return toolSuccess(result), nil
}

// handleUserList handles the ts.usr.list tool.
// Rewired to call the API layer for consistent behavior with REST.
func (s *Server) handleUserList(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := parseArgs(req)

	opts := storage.ListUsersOpts{
		Status:         getString(args, "status"),
		OrganizationID: getString(args, "organization_id"),
		UserType:       getString(args, "user_type"),
		Search:         getString(args, "search"),
		Limit:          getInt(args, "limit", 50),
		Offset:         getInt(args, "offset", 0),
	}

	items, total, apiErr := s.api.ListUsers(ctx, opts, false)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}

	return toolSuccess(map[string]interface{}{
		"users":  items,
		"total":  total,
		"limit":  opts.Limit,
		"offset": opts.Offset,
	}), nil
}

// createResourceForUser creates a resource entry for a user.
func (s *Server) createResourceForUser(ctx context.Context, userID, name, email string) (string, error) {
	idBytes := make([]byte, 8)
	if _, err := rand.Read(idBytes); err != nil {
		return "", err
	}
	resourceID := "res_" + hex.EncodeToString(idBytes)

	metadata := map[string]interface{}{
		"user_id": userID,
		"email":   email,
	}
	metadataJSON, _ := json.Marshal(metadata)

	_, err := s.db.Exec(
		`INSERT INTO resource (id, type, name, capacity_model, metadata, status)
		 VALUES (?, 'human', ?, 'hours_per_week', ?, 'active')`,
		resourceID, name, string(metadataJSON),
	)
	if err != nil {
		return "", err
	}

	return resourceID, nil
}

// nullString returns a sql.NullString from a string.
func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}
