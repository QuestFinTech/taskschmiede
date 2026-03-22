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
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/QuestFinTech/taskschmiede/internal/auth"
	"github.com/QuestFinTech/taskschmiede/internal/security"
	"github.com/QuestFinTech/taskschmiede/internal/storage"
)

// handleAuthLogin handles the ts.auth.login tool.
func (s *Server) handleAuthLogin(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := parseArgs(req)
	email := getString(args, "email")
	password := getString(args, "password")

	if email == "" || password == "" {
		return toolError("invalid_input", "Email and password are required"), nil
	}

	// Check auth rate limit (per email) before attempting authentication.
	// HTTP-level AuthMiddleware cannot be used here because all MCP tools
	// share the /mcp route -- we rate limit inside the handler instead.
	if s.rateLimiter != nil && !s.rateLimiter.AllowAuth(email) {
		s.logger.Warn("MCP auth rate limit exceeded", "email", email)
		if s.auditSvc != nil {
			s.auditSvc.Log(&security.AuditEntry{
				Action:    security.AuditRateLimitHit,
				ActorType: "anonymous",
				Source:    "mcp",
				Metadata:  map[string]interface{}{"email": email, "tier": "auth-endpoint"},
			})
		}
		return toolError("rate_limited", "Too many login attempts. Please wait and try again."), nil
	}

	// Authenticate user
	user, err := s.authSvc.Authenticate(ctx, email, password)
	if err != nil {
		s.logger.Debug("Authentication failed", "email", email, "error", err)
		if s.auditSvc != nil {
			s.auditSvc.Log(&security.AuditEntry{
				Action:    security.AuditLoginFailure,
				ActorType: "anonymous",
				Source:    "mcp",
				Metadata:  map[string]interface{}{"email": email},
			})
		}
		return toolError("invalid_credentials", "Invalid email or password"), nil
	}

	// Determine token TTL from policy
	ttl := s.authSvc.DefaultTokenTTL(ctx)
	exp := storage.UTCNow().Add(ttl)
	expiresAt := &exp

	// Create token for the user
	token, tokenID, err := s.authSvc.CreateToken(ctx, user.ID, "mcp-session", expiresAt)
	if err != nil {
		s.logger.Error("Failed to create token", "user_id", user.ID, "error", err)
		return toolError("internal_error", "Failed to create access token"), nil
	}

	// Store auth in session map so subsequent tool calls in this session
	// can access it via withSessionAuth, even though the MCP SDK does not
	// propagate the HTTP request context to tool handlers.
	var sessionExpiresAt *time.Time
	if s.sessionTimeout > 0 {
		sessExp := storage.UTCNow().Add(s.sessionTimeout)
		sessionExpiresAt = &sessExp
	}

	if req.Session != nil {
		sessionID := req.Session.ID()
		if sessionID != "" {
			now := storage.UTCNow()
			s.sessionAuthMu.Lock()
			s.sessionAuth[sessionID] = &auth.AuthUser{
				UserID:    user.ID,
				TokenID:   tokenID,
				UserType:  user.UserType,
				ExpiresAt: sessionExpiresAt,
				CreatedAt: &now,
			}
			s.sessionAuthMu.Unlock()
		}
	}

	s.db.IncrementLoginCount(user.ID)

	s.logger.Info("User authenticated via MCP", "user_id", user.ID, "email", email)

	if s.auditSvc != nil {
		s.auditSvc.Log(&security.AuditEntry{
			Action:    security.AuditLoginSuccess,
			ActorID:   user.ID,
			ActorType: "user",
			Source:    "mcp",
			Metadata:  map[string]interface{}{"email": email},
		})
	}

	result := map[string]interface{}{
		"token":    token,
		"user_id":  user.ID,
		"name":     user.Name,
		"email":    user.Email,
		"token_id": tokenID,
	}
	if user.ResourceID != nil && *user.ResourceID != "" {
		result["resource_id"] = *user.ResourceID
	}
	if s.sessionTimeout > 0 {
		result["session_timeout"] = s.sessionTimeout.String()
		result["session_note"] = fmt.Sprintf("Session expires after %s of inactivity. Each tool call resets the timer.", s.sessionTimeout)
	}

	return toolSuccess(result), nil
}

// handleAuthWhoami handles the ts.auth.whoami tool.
func (s *Server) handleAuthWhoami(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	result, apiErr := s.api.Whoami(ctx)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}
	return toolSuccess(result), nil
}

// handleAuthUpdateProfile handles the ts.auth.update_profile tool.
// Allows authenticated users to update their own profile fields.
func (s *Server) handleAuthUpdateProfile(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	authUser, err := requireAuth(ctx)
	if err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	args := parseArgs(req)

	var fields storage.UpdateUserFields
	if v, ok := args["name"].(string); ok {
		fields.Name = &v
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

	if fields.Name == nil && fields.Lang == nil && fields.Timezone == nil && fields.EmailCopy == nil {
		return toolError("invalid_input", "At least one field is required: name, lang, timezone, email_copy"), nil
	}

	result, apiErr := s.api.UpdateUser(ctx, authUser.UserID, fields)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}
	return toolSuccess(result), nil
}

// handleForgotPassword handles the ts.auth.forgot_password tool.
// Creates a password reset code and sends it via email (if configured).
// Does not require authentication (the user forgot their password).
func (s *Server) handleForgotPassword(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := parseArgs(req)
	email := getString(args, "email")

	if email == "" {
		return toolError("invalid_input", "Email is required"), nil
	}

	// Rate limit password reset requests per email
	if s.rateLimiter != nil && !s.rateLimiter.AllowAuth(email) {
		s.logger.Warn("MCP password reset rate limit exceeded", "email", email)
		return toolError("rate_limited", "Too many requests. Please wait and try again."), nil
	}

	// Create password reset (returns nil if email not found -- by design)
	reset, err := s.db.CreatePasswordReset(email, defaultVerificationTimeout)
	if err != nil {
		s.logger.Warn("Password reset error", "error", err)
	}

	// Send email if reset was created and email is configured
	if reset != nil && s.emailSender != nil {
		lang := s.db.GetUserLangByEmail(email)
		s.sendCodeEmail(email, "", reset.Code, lang, true)
	}

	// Always return success to prevent email enumeration
	result := map[string]interface{}{
		"status":     "reset_requested",
		"expires_in": defaultVerificationTimeout.String(),
		"note":       "If an account exists with that email, a reset code has been sent.",
	}

	// Include the code directly when email is not configured (development/testing)
	if reset != nil && s.emailSender == nil {
		result["code"] = reset.Code
		result["note"] = "Email not configured. Code returned directly for development use."
	}

	return toolSuccess(result), nil
}

// handleResetPassword handles the ts.auth.reset_password tool.
// Validates the reset code and sets a new password.
// Does not require authentication.
func (s *Server) handleResetPassword(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := parseArgs(req)
	email := getString(args, "email")
	code := getString(args, "code")
	newPassword := getString(args, "new_password")

	if email == "" || code == "" || newPassword == "" {
		return toolError("invalid_input", "Email, code, and new_password are required"), nil
	}

	if err := auth.ValidatePassword(newPassword); err != nil {
		return toolError("invalid_input", err.Error()), nil
	}

	if err := s.db.CompletePasswordReset(email, code, newPassword); err != nil {
		return toolError("reset_failed", err.Error()), nil
	}

	s.logger.Info("Password reset completed via MCP", "email", email)

	return toolSuccess(map[string]interface{}{
		"status": "password_reset",
		"note":   "Password has been changed. All existing sessions have been invalidated.",
	}), nil
}

// handleTokenVerify handles the ts.tkn.verify tool.
func (s *Server) handleTokenVerify(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := parseArgs(req)
	token := getString(args, "token")

	if token == "" {
		return toolError("invalid_input", "Token is required"), nil
	}

	user, err := s.authSvc.VerifyToken(ctx, token)
	if err != nil {
		return toolSuccess(map[string]interface{}{
			"valid": false,
			"error": err.Error(),
		}), nil
	}

	result := map[string]interface{}{
		"valid":   true,
		"user_id": user.UserID,
	}
	if user.ExpiresAt != nil {
		result["expires_at"] = user.ExpiresAt.Format(time.RFC3339)
	}

	return toolSuccess(result), nil
}

// handleTokenCreate handles the ts.tkn.create tool.
func (s *Server) handleTokenCreate(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Require authentication
	authUser, err := requireAuth(ctx)
	if err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	args := parseArgs(req)
	userID := getString(args, "user_id")
	name := getString(args, "name")
	expiresAtStr := getString(args, "expires_at")

	// Default to authenticated user if no user_id specified
	if userID == "" {
		userID = authUser.UserID
	}

	// Check if user has permission to create tokens for other users
	if userID != authUser.UserID {
		isAdmin, err := s.authSvc.IsAdmin(ctx, authUser.UserID)
		if err != nil || !isAdmin {
			return toolError("unauthorized", "Cannot create tokens for other users"), nil
		}
	}

	// Parse expiration
	var expiresAt *time.Time
	if expiresAtStr != "" {
		t, err := time.Parse(time.RFC3339, expiresAtStr)
		if err != nil {
			return toolError("invalid_input", "Invalid expires_at format (use ISO 8601)"), nil
		}
		expiresAt = &t
	}

	// Create token
	token, tokenID, err := s.authSvc.CreateToken(ctx, userID, name, expiresAt)
	if err != nil {
		s.logger.Error("Failed to create token", "user_id", userID, "error", err)
		return toolError("internal_error", "Failed to create token"), nil
	}

	s.logger.Info("Token created", "token_id", tokenID, "user_id", userID, "name", name)

	if s.auditSvc != nil {
		s.auditSvc.Log(&security.AuditEntry{
			Action:    security.AuditTokenCreated,
			ActorID:   authUser.UserID,
			ActorType: "user",
			Resource:  tokenID,
			Source:    "mcp",
			Metadata:  map[string]interface{}{"name": name, "for_user": userID},
		})
	}

	result := map[string]interface{}{
		"id":         tokenID,
		"token":      token,
		"name":       name,
		"created_at": storage.UTCNow().Format(time.RFC3339),
	}
	if expiresAt != nil {
		result["expires_at"] = expiresAt.Format(time.RFC3339)
	}

	return toolSuccess(result), nil
}
