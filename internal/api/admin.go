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
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuestFinTech/taskschmiede/internal/auth"
	"github.com/QuestFinTech/taskschmiede/internal/security"
	"github.com/QuestFinTech/taskschmiede/internal/storage"
)

// handleCapacity returns seat availability for the default tier.
// Public endpoint -- no authentication required.
// Used by the SaaS website to display remaining Explorer seats.
func (a *API) handleCapacity(w http.ResponseWriter, r *http.Request) {
	maxUsers := a.policyInt("instance.max_active_users", 200)
	activeUsers := a.db.CountActiveUsers()

	writeData(w, http.StatusOK, map[string]interface{}{
		"active_users":     activeUsers,
		"max_active_users": maxUsers,
		"at_capacity":      maxUsers > 0 && activeUsers >= maxUsers,
	})
}

// handleSignupInterest records a signup interest from the SaaS website.
// Public endpoint -- no authentication required.
func (a *API) handleSignupInterest(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email        string `json:"email"`
		Usecase      string `json:"usecase"`
		UsecaseOther string `json:"usecase_other"`
		Source       string `json:"source"`
		SourceOther  string `json:"source_other"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Invalid JSON body")
		return
	}
	if req.Email == "" {
		writeError(w, http.StatusBadRequest, "invalid_input", "Email is required")
		return
	}

	ip := r.RemoteAddr
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		ip = fwd
	}

	if err := a.db.RecordSignupInterest(req.Email, req.Usecase, req.UsecaseOther, req.Source, req.SourceOther, ip); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Could not record interest")
		return
	}

	writeData(w, http.StatusCreated, map[string]interface{}{"status": "recorded"})
}

// handleInstanceInfo returns non-sensitive deployment settings.
// Public endpoint -- no authentication required.
func (a *API) handleInstanceInfo(w http.ResponseWriter, r *http.Request) {
	registrationOpen := true
	defaultTierID := a.db.DefaultTierID()
	if tier, err := a.db.GetTierDefinition(defaultTierID); err == nil && tier.MaxUsers > 0 {
		var count int
		_ = a.db.QueryRow(`SELECT COUNT(*) FROM user WHERE tier = ? AND is_admin = 0`, defaultTierID).Scan(&count)
		if count >= tier.MaxUsers {
			registrationOpen = false
		}
	}

	writeData(w, http.StatusOK, map[string]interface{}{
		"deployment_mode":         a.deploymentMode,
		"allow_self_registration": a.allowSelfRegistration,
		"registration_open":       registrationOpen,
	})
}

// setupPhase computes the current setup wizard phase from database state.
//
//	"account"   -- no master admin, no pending admin
//	"verify"    -- pending admin exists (unverified)
//	"configure" -- master admin exists, setup.complete != "true"
//	"done"      -- setup.complete == "true"
func (a *API) setupPhase() string {
	hasAdmin, _ := a.db.HasMasterAdmin()
	if !hasAdmin {
		pending, _ := a.db.GetPendingAdmin()
		if pending != nil && storage.UTCNow().Before(pending.ExpiresAt) {
			return "verify"
		}
		return "account"
	}
	if v, err := a.db.GetPolicy("setup.complete"); err == nil && v == "true" {
		return "done"
	}
	return "configure"
}

// handleSetupStatus handles GET /api/v1/admin/setup/status.
// Returns whether the system needs initial setup and the current wizard phase.
func (a *API) handleSetupStatus(w http.ResponseWriter, r *http.Request) {
	phase := a.setupPhase()

	result := map[string]interface{}{
		"needs_setup": phase != "done",
		"phase":       phase,
	}

	if phase == "verify" {
		pending, _ := a.db.GetPendingAdmin()
		if pending != nil {
			result["pending_verification"] = true
			result["email"] = pending.Email
			result["expires_at"] = pending.ExpiresAt.Format(time.RFC3339)
		}
	}

	writeData(w, http.StatusOK, result)
}

// handleSetupConfigure handles POST /api/v1/admin/setup/configure.
// Stores system timezone and default language, then marks setup as complete.
func (a *API) handleSetupConfigure(w http.ResponseWriter, r *http.Request) {
	phase := a.setupPhase()
	if phase == "done" {
		writeError(w, http.StatusConflict, "already_setup", "System is already set up")
		return
	}
	if phase != "configure" {
		writeError(w, http.StatusBadRequest, "invalid_phase", "Setup is not in the configure phase")
		return
	}

	var body struct {
		Timezone        string `json:"timezone"`
		DefaultLanguage string `json:"default_language"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Invalid JSON body")
		return
	}

	if body.Timezone == "" || body.DefaultLanguage == "" {
		writeError(w, http.StatusBadRequest, "invalid_input", "Timezone and default language are required")
		return
	}

	// Validate timezone.
	if _, err := time.LoadLocation(body.Timezone); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", fmt.Sprintf("Invalid timezone: %s", body.Timezone))
		return
	}

	// Store settings and mark setup complete.
	if err := a.db.SetPolicy("system.timezone", body.Timezone); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to save timezone")
		return
	}
	if err := a.db.SetPolicy("system.default_language", body.DefaultLanguage); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to save default language")
		return
	}
	if err := a.db.SetPolicy("setup.complete", "true"); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to complete setup")
		return
	}

	writeData(w, http.StatusOK, map[string]interface{}{
		"status":           "configured",
		"timezone":         body.Timezone,
		"default_language": body.DefaultLanguage,
	})
}

// handleSetupCreate handles POST /api/v1/admin/setup.
// Creates the master admin pending registration and sends verification email.
func (a *API) handleSetupCreate(w http.ResponseWriter, r *http.Request) {
	hasAdmin, _ := a.db.HasMasterAdmin()
	if hasAdmin {
		writeError(w, http.StatusConflict, "already_setup", "System is already set up")
		return
	}

	var body struct {
		Email    string `json:"email"`
		Name     string `json:"name"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Invalid JSON body")
		return
	}

	if body.Email == "" || body.Name == "" || body.Password == "" {
		writeError(w, http.StatusBadRequest, "invalid_input", "Email, name, and password are required")
		return
	}

	if err := auth.ValidatePassword(body.Password); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
		return
	}

	pending, err := a.db.CreatePendingAdmin(body.Email, body.Name, body.Password, 15*time.Minute)
	if err != nil {
		a.logger.Error("Failed to create pending admin", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to create registration")
		return
	}

	// Send verification email (uses HTML template when available).
	emailSent := false
	if a.emailSender != nil {
		emailSent = a.trySendVerificationEmail(body.Email, body.Name, pending.VerificationCode, "en", false)
	}

	resp := map[string]interface{}{
		"status":     "pending_verification",
		"email":      pending.Email,
		"expires_at": pending.ExpiresAt.Format(time.RFC3339),
		"email_sent": emailSent,
	}
	if emailSent {
		resp["message"] = fmt.Sprintf("A verification code has been sent to %s. Check your inbox.", pending.Email)
	} else {
		// Include the code in the response so the portal can display it.
		// This handles the case where email is not configured during initial setup.
		resp["verification_code"] = pending.VerificationCode
		resp["message"] = "Email is not configured. Use the verification code shown on screen to continue."
	}

	writeData(w, http.StatusCreated, resp)
}

// handleSetupVerify handles POST /api/v1/admin/setup/verify.
// Verifies the master admin setup code and activates the admin account.
func (a *API) handleSetupVerify(w http.ResponseWriter, r *http.Request) {
	hasAdmin, _ := a.db.HasMasterAdmin()
	if hasAdmin {
		writeError(w, http.StatusConflict, "already_setup", "System is already set up")
		return
	}

	var body struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Invalid JSON body")
		return
	}

	if body.Code == "" {
		writeError(w, http.StatusBadRequest, "invalid_input", "Verification code is required")
		return
	}

	if err := a.db.VerifyAndActivateAdmin(body.Code); err != nil {
		writeError(w, http.StatusBadRequest, "verification_failed", err.Error())
		return
	}

	writeData(w, http.StatusOK, map[string]interface{}{
		"status": "activated",
	})
}

// handleSetupResend handles POST /api/v1/admin/setup/resend.
// Regenerates the verification code and sends a new email.
func (a *API) handleSetupResend(w http.ResponseWriter, r *http.Request) {
	hasAdmin, _ := a.db.HasMasterAdmin()
	if hasAdmin {
		writeError(w, http.StatusConflict, "already_setup", "System is already set up")
		return
	}

	newPending, err := a.db.RegeneratePendingAdminCode(15 * time.Minute)
	if err != nil {
		writeError(w, http.StatusBadRequest, "resend_failed", "No pending registration found")
		return
	}

	// Send verification email (uses HTML template when available)
	if a.emailSender != nil {
		a.sendVerificationEmail(newPending.Email, newPending.Name, newPending.VerificationCode, "en", false)
	}

	writeData(w, http.StatusOK, map[string]interface{}{
		"status":     "code_resent",
		"email":      newPending.Email,
		"expires_at": newPending.ExpiresAt.Format(time.RFC3339),
		"message":    fmt.Sprintf("A new verification code has been sent to %s. Check your inbox.", newPending.Email),
	})
}

// handleInvitationList handles GET /api/v1/invitations.
func (a *API) handleInvitationList(w http.ResponseWriter, r *http.Request) {
	if !a.requireAdmin(w, r) {
		return
	}

	tokens, err := a.db.ListInvitationTokens("", "")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list tokens")
		return
	}

	result := make([]map[string]interface{}, len(tokens))
	for i, t := range tokens {
		result[i] = map[string]interface{}{
			"id":         t.ID,
			"name":       t.Name,
			"token":      t.Token,
			"scope":      t.Scope,
			"max_uses":   t.MaxUses,
			"use_count":  t.Uses,
			"status":     storage.GetTokenStatus(t),
			"created_at": t.CreatedAt.Format(time.RFC3339),
		}
		if t.ExpiresAt != nil {
			result[i]["expires_at"] = t.ExpiresAt.Format(time.RFC3339)
		}
	}

	writeData(w, http.StatusOK, result)
}

// handleInvitationCreate handles POST /api/v1/invitations.
func (a *API) handleInvitationCreate(w http.ResponseWriter, r *http.Request) {
	if !a.requireAdmin(w, r) {
		return
	}

	var body struct {
		Name     string `json:"name"`
		MaxUses  int    `json:"max_uses"`
		ExpireAt string `json:"expires_at"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Invalid JSON body")
		return
	}

	if body.Name == "" {
		body.Name = "Unnamed token"
	}
	if body.MaxUses <= 0 {
		body.MaxUses = 1
	}

	var expiresAt *time.Time
	if body.ExpireAt != "" {
		t, err := time.Parse(time.RFC3339, body.ExpireAt)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_input", "Invalid expires_at format (use RFC3339)")
			return
		}
		expiresAt = &t
	}

	authUser := auth.GetAuthUser(r.Context())
	createdBy := ""
	if authUser != nil {
		createdBy = authUser.UserID
	}

	inv, err := a.db.CreateInvitationToken(body.Name, "system", "", "", &body.MaxUses, expiresAt, createdBy)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to create token")
		return
	}

	writeData(w, http.StatusCreated, map[string]interface{}{
		"id":         inv.ID,
		"name":       inv.Name,
		"token":      inv.Token,
		"scope":      inv.Scope,
		"max_uses":   inv.MaxUses,
		"created_at": inv.CreatedAt.Format(time.RFC3339),
	})
}

// handleInvitationRevoke handles DELETE /api/v1/invitations/{id}.
func (a *API) handleInvitationRevoke(w http.ResponseWriter, r *http.Request) {
	if !a.requireAdmin(w, r) {
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "invalid_input", "Token ID is required")
		return
	}

	if err := a.db.RevokeInvitationTokenByID(id); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "Token not found")
		return
	}

	writeData(w, http.StatusOK, map[string]interface{}{
		"status": "revoked",
	})
}

// ---------------------------------------------------------------------------
// Agent Token endpoints (any authenticated human user)
// ---------------------------------------------------------------------------

// handleAgentTokenList handles GET /api/v1/agent-tokens.
// Returns invitation tokens created by the authenticated user.
func (a *API) handleAgentTokenList(w http.ResponseWriter, r *http.Request) {
	authUser := getAuthUser(r)
	if authUser == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	tokens, err := a.db.ListInvitationTokens("", "", authUser.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list tokens")
		return
	}

	result := make([]map[string]interface{}, len(tokens))
	for i, t := range tokens {
		result[i] = map[string]interface{}{
			"id":        t.ID,
			"name":      t.Name,
			"max_uses":  t.MaxUses,
			"use_count": t.Uses,
			"status":    storage.GetTokenStatus(t),
			"created_at": t.CreatedAt.Format(time.RFC3339),
		}
		if t.ExpiresAt != nil {
			result[i]["expires_at"] = t.ExpiresAt.Format(time.RFC3339)
		}
	}

	writeData(w, http.StatusOK, result)
}

// handleAgentTokenCreate handles POST /api/v1/agent-tokens.
// Creates an agent invitation token owned by the authenticated user.
func (a *API) handleAgentTokenCreate(w http.ResponseWriter, r *http.Request) {
	authUser := getAuthUser(r)
	if authUser == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	// Agent token creation requires org admin or master admin privileges.
	isAdmin, err := a.authSvc.IsAdmin(r.Context(), authUser.UserID)
	if err != nil {
		a.logger.Error("Failed to check admin status", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to check permissions")
		return
	}
	if !isAdmin {
		writeError(w, http.StatusForbidden, "forbidden",
			"Agent token creation requires admin privileges (organization admin or master admin)")
		return
	}

	var body struct {
		Name     string `json:"name"`
		MaxUses  int    `json:"max_uses"`
		ExpireAt string `json:"expires_at"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Invalid JSON body")
		return
	}

	if body.Name == "" {
		body.Name = "Agent token"
	}
	if body.MaxUses <= 0 {
		body.MaxUses = 1
	}

	// Check agent slot availability: current agents + pending token slots must not
	// exceed the tier's max_agents_per_org limit.
	orgID := a.findUserOrgID(r.Context(), authUser.UserID)
	if orgID != "" {
		td, _, apiErr := a.getTierDef(r.Context())
		if apiErr != nil {
			writeAPIError(w, apiErr)
			return
		}
		if td != nil && td.MaxAgentsPerOrg >= 0 {
			currentAgents := a.countOrgAgents(r.Context(), orgID)
			pendingSlots := a.countPendingAgentSlots(r.Context(), authUser.UserID)
			remaining := td.MaxAgentsPerOrg - currentAgents - pendingSlots
			if remaining <= 0 {
				writeError(w, http.StatusForbidden, "tier_limit",
					fmt.Sprintf("No agent slots remaining. %d/%d agents registered, %d pending token slots. Revoke unused tokens or upgrade your tier.",
						currentAgents, td.MaxAgentsPerOrg, pendingSlots))
				return
			}
			// Cap max_uses to remaining slots.
			if body.MaxUses > remaining {
				body.MaxUses = remaining
			}
		}
	}

	// Agent tokens expire after the configured TTL (default 30m).
	// If the caller requests a longer window (or no expiry), cap it.
	ttl := a.agentTokenTTL
	if ttl <= 0 {
		ttl = 30 * time.Minute
	}
	maxExpiry := time.Now().UTC().Add(ttl)
	expiresAt := &maxExpiry
	if body.ExpireAt != "" {
		t, err := time.Parse(time.RFC3339, body.ExpireAt)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_input", "Invalid expires_at format (use RFC3339)")
			return
		}
		if t.Before(maxExpiry) {
			expiresAt = &t
		}
	}

	inv, err := a.db.CreateInvitationToken(body.Name, "system", "", "", &body.MaxUses, expiresAt, authUser.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to create token")
		return
	}

	resp := map[string]interface{}{
		"id":         inv.ID,
		"name":       inv.Name,
		"token":      inv.Token,
		"max_uses":   inv.MaxUses,
		"created_at": inv.CreatedAt.Format(time.RFC3339),
	}
	if inv.ExpiresAt != nil {
		resp["expires_at"] = inv.ExpiresAt.Format(time.RFC3339)
	}
	writeData(w, http.StatusCreated, resp)
}

// handleAgentTokenRevoke handles DELETE /api/v1/agent-tokens/{id}.
// Revokes an agent invitation token owned by the authenticated user.
func (a *API) handleAgentTokenRevoke(w http.ResponseWriter, r *http.Request) {
	authUser := getAuthUser(r)
	if authUser == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "invalid_input", "Token ID is required")
		return
	}

	// Ownership check: only the creator can revoke their own tokens.
	// Master admins use the /api/v1/invitations endpoint instead.
	token, err := a.db.GetInvitationTokenByID(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "Token not found")
		return
	}
	if token.CreatedBy != authUser.UserID {
		writeError(w, http.StatusNotFound, "not_found", "Token not found")
		return
	}

	if err := a.db.RevokeInvitationTokenByID(id); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "Token not found or already revoked")
		return
	}

	writeData(w, http.StatusOK, map[string]interface{}{
		"status": "revoked",
	})
}

// handleAdminSettings handles GET /api/v1/admin/settings.
func (a *API) handleAdminSettings(w http.ResponseWriter, r *http.Request) {
	if !a.requireAdmin(w, r) {
		return
	}

	mcpEnabled, _ := a.db.GetMCPAccessEnabled()

	writeData(w, http.StatusOK, map[string]interface{}{
		"mcp_access_enabled": mcpEnabled,
	})
}

// handleAdminSettingsUpdate handles PATCH /api/v1/admin/settings.
func (a *API) handleAdminSettingsUpdate(w http.ResponseWriter, r *http.Request) {
	if !a.requireAdmin(w, r) {
		return
	}

	var body struct {
		MCPAccessEnabled *bool `json:"mcp_access_enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Invalid JSON body")
		return
	}

	if body.MCPAccessEnabled != nil {
		if *body.MCPAccessEnabled {
			if err := a.db.EnableMCPAccess(); err != nil {
				writeError(w, http.StatusInternalServerError, "internal_error", "Failed to enable MCP access")
				return
			}
		} else {
			if err := a.db.DisableMCPAccess(); err != nil {
				writeError(w, http.StatusInternalServerError, "internal_error", "Failed to disable MCP access")
				return
			}
		}
	}

	mcpEnabled, _ := a.db.GetMCPAccessEnabled()
	writeData(w, http.StatusOK, map[string]interface{}{
		"mcp_access_enabled": mcpEnabled,
	})
}

// handleAdminPassword handles POST /api/v1/admin/password.
func (a *API) handleAdminPassword(w http.ResponseWriter, r *http.Request) {
	if !a.requireAdmin(w, r) {
		return
	}

	authUser := getAuthUser(r)
	if authUser == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	var body struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Invalid JSON body")
		return
	}

	if body.CurrentPassword == "" || body.NewPassword == "" {
		writeError(w, http.StatusBadRequest, "invalid_input", "Current and new passwords are required")
		return
	}

	if err := auth.ValidatePassword(body.NewPassword); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
		return
	}

	newHash, err := auth.HashPassword(body.NewPassword)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to hash password")
		return
	}

	if err := a.db.ChangeUserPassword(authUser.UserID, body.CurrentPassword, newHash); err != nil {
		if err == storage.ErrInvalidPassword {
			writeError(w, http.StatusUnauthorized, "invalid_credentials", "Current password is incorrect")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to update password")
		return
	}

	writeData(w, http.StatusOK, map[string]interface{}{
		"status": "password_updated",
	})
}

// handleAdminStats handles GET /api/v1/admin/stats.
func (a *API) handleAdminStats(w http.ResponseWriter, r *http.Request) {
	if !a.requireAdmin(w, r) {
		return
	}

	stats, err := a.db.GetStats()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get stats")
		return
	}

	writeData(w, http.StatusOK, map[string]interface{}{
		"organizations": stats.Organizations,
		"users":         stats.Users,
		"master_admins": stats.MasterAdmins,
		"endeavours":    stats.Endeavours,
		"tasks":         stats.Tasks,
		"resources":     stats.Resources,
		"artifacts":     stats.Artifacts,
		"rituals":       stats.Rituals,
		"ritual_runs":   stats.RitualRuns,
		"relations":     stats.Relations,
		"demands":       stats.Demands,
	})
}

// handleAdminIndicators handles GET /api/v1/admin/indicators.
// Returns system-wide Ablecon and Harmcon levels plus per-org Ablecon breakdown.
func (a *API) handleAdminIndicators(w http.ResponseWriter, r *http.Request) {
	if !a.requireAdmin(w, r) {
		return
	}

	sysAblecon, _ := a.db.GetSystemAbleconLevel()
	orgAbleconsRaw, _ := a.db.ListOrgAbleconLevels()
	sysHarmcon, _ := a.db.GetSystemHarmconLevel()

	ableconData := map[string]interface{}{"level": 4, "label": "blue"}
	if sysAblecon != nil {
		ableconData["level"] = sysAblecon.Level
		ableconData["label"] = storage.AbleconLevelLabel(sysAblecon.Level)
		ableconData["reason"] = sysAblecon.Reason
	}

	harmconData := map[string]interface{}{"level": 4, "label": "blue"}
	if sysHarmcon != nil {
		harmconData["level"] = sysHarmcon.Level
		harmconData["label"] = storage.AbleconLevelLabel(sysHarmcon.Level)
		harmconData["high_count"] = sysHarmcon.HighCount
		harmconData["medium_count"] = sysHarmcon.MediumCount
		harmconData["low_count"] = sysHarmcon.LowCount
	}

	var orgAblecon []map[string]interface{}
	for _, o := range orgAbleconsRaw {
		orgName := o.ScopeID
		// Try to resolve org name
		org, err := a.db.GetOrganization(o.ScopeID)
		if err == nil && org != nil {
			orgName = org.Name
		}
		orgAblecon = append(orgAblecon, map[string]interface{}{
			"org_id":   o.ScopeID,
			"org_name": orgName,
			"level":    o.Level,
			"label":    storage.AbleconLevelLabel(o.Level),
		})
	}

	writeData(w, http.StatusOK, map[string]interface{}{
		"ablecon":     ableconData,
		"harmcon":     harmconData,
		"org_ablecon": orgAblecon,
	})
}

// handleAdminContentGuardPatterns handles GET /api/v1/admin/content-guard/patterns.
// Returns all patterns (builtin + custom) with current override state.
func (a *API) handleAdminContentGuardPatterns(w http.ResponseWriter, r *http.Request) {
	if !a.requireAdmin(w, r) {
		return
	}

	builtins := security.ListBuiltinPatterns()
	overrides, _ := a.db.GetSystemPatternOverrides()

	type patternInfo struct {
		Name     string `json:"name"`
		Category string `json:"category"`
		Pattern  string `json:"pattern"`
		Weight   int    `json:"weight"`
		Builtin  bool   `json:"builtin"`
		Enabled  bool   `json:"enabled"`
	}

	patterns := make([]patternInfo, 0, len(builtins))
	for _, b := range builtins {
		weight := b.Weight
		enabled := true
		if overrides != nil {
			if overrides.Disabled[b.Name] {
				enabled = false
			}
			if w, ok := overrides.WeightOverrides[b.Name]; ok {
				weight = w
			}
		}
		patterns = append(patterns, patternInfo{
			Name:     b.Name,
			Category: b.Category,
			Pattern:  b.Pattern,
			Weight:   weight,
			Builtin:  true,
			Enabled:  enabled,
		})
	}

	// Append custom patterns
	if overrides != nil {
		for _, cp := range overrides.Added {
			patterns = append(patterns, patternInfo{
				Name:     cp.Name,
				Category: cp.Category,
				Pattern:  cp.Pattern,
				Weight:   cp.Weight,
				Builtin:  false,
				Enabled:  true,
			})
		}
	}

	writeData(w, http.StatusOK, patterns)
}

// handleAdminContentGuardPatternsUpdate handles PATCH /api/v1/admin/content-guard/patterns.
// Saves pattern overrides to the policy table and applies them live.
func (a *API) handleAdminContentGuardPatternsUpdate(w http.ResponseWriter, r *http.Request) {
	if !a.requireAdmin(w, r) {
		return
	}

	var body storage.SystemPatternOverrides
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Invalid JSON body")
		return
	}

	// Validate custom patterns
	if len(body.Added) > 50 {
		writeError(w, http.StatusBadRequest, "invalid_input", "Maximum 50 custom patterns allowed")
		return
	}
	secAdded := make([]security.CustomPattern, len(body.Added))
	for i, cp := range body.Added {
		sp := security.CustomPattern{
			Name:     cp.Name,
			Category: cp.Category,
			Pattern:  cp.Pattern,
			Weight:   cp.Weight,
		}
		if err := security.ValidateCustomPattern(sp); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_input", fmt.Sprintf("Invalid custom pattern %q: %s", cp.Name, err.Error()))
			return
		}
		secAdded[i] = sp
	}

	// Validate weight overrides
	for name, wt := range body.WeightOverrides {
		if wt < 1 || wt > 25 {
			writeError(w, http.StatusBadRequest, "invalid_input", fmt.Sprintf("Weight for %q must be between 1 and 25", name))
			return
		}
	}

	// Save to DB
	if err := a.db.SetSystemPatternOverrides(&body); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to save pattern overrides")
		return
	}

	// Apply live
	security.SetPatternOverrides(&security.PatternOverrides{
		Disabled:        body.Disabled,
		WeightOverrides: body.WeightOverrides,
		Added:           secAdded,
	})

	// Audit
	if a.auditSvc != nil {
		authUser := getAuthUser(r)
		if authUser != nil {
			a.auditSvc.Log(&security.AuditEntry{
				Action:   "content_guard_patterns_updated",
				ActorID:  authUser.UserID,
				Resource: "content-guard",
				Source:   auditSource(r),
			})
		}
	}

	writeData(w, http.StatusOK, map[string]interface{}{"status": "updated"})
}

// handleAdminContentGuardStats handles GET /api/v1/admin/content-guard/stats.
func (a *API) handleAdminContentGuardStats(w http.ResponseWriter, r *http.Request) {
	if !a.requireAdmin(w, r) {
		return
	}

	stats, err := a.db.GetContentGuardStats()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get content guard stats")
		return
	}

	writeData(w, http.StatusOK, stats)
}

// handleAdminContentGuardTest handles POST /api/v1/admin/content-guard/test.
// Dry-run endpoint: scores payloads without persisting anything.
func (a *API) handleAdminContentGuardTest(w http.ResponseWriter, r *http.Request) {
	if !a.requireAdmin(w, r) {
		return
	}

	var body struct {
		Payloads []string `json:"payloads"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Invalid JSON body")
		return
	}

	if len(body.Payloads) == 0 {
		writeError(w, http.StatusBadRequest, "invalid_input", "At least one payload is required")
		return
	}
	if len(body.Payloads) > 20 {
		writeError(w, http.StatusBadRequest, "invalid_input", "Maximum 20 payloads per request")
		return
	}

	type testResult struct {
		Text    string   `json:"text"`
		Score   int      `json:"score"`
		Signals []string `json:"signals"`
	}

	results := make([]testResult, len(body.Payloads))
	for i, payload := range body.Payloads {
		hs := security.ScoreContent(payload)
		results[i] = testResult{
			Text:    payload,
			Score:   hs.Score,
			Signals: hs.Signals,
		}
		if results[i].Signals == nil {
			results[i].Signals = []string{}
		}
	}

	writeData(w, http.StatusOK, map[string]interface{}{
		"results":   results,
		"threshold": contentGuardScoreThreshold,
	})
}

// handleAdminContentGuardAlerts handles GET /api/v1/admin/content-guard/alerts.
// Returns system-wide flagged entities ordered by harm score (descending).
func (a *API) handleAdminContentGuardAlerts(w http.ResponseWriter, r *http.Request) {
	if !a.requireAdmin(w, r) {
		return
	}

	limit := queryInt(r, "limit", 50)
	offset := queryInt(r, "offset", 0)
	includeDismissed := queryString(r, "include_dismissed") == "true"

	alerts, total, err := a.db.ListContentAlertsSystemWide(1, limit, offset, includeDismissed)
	if err != nil {
		a.logger.Error("Failed to list system-wide alerts", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list alerts")
		return
	}

	writeList(w, alerts, total, limit, offset)
}

// handleAdminContentGuardDismiss handles POST /api/v1/admin/content-guard/dismiss.
// Marks a flagged entity as dismissed (false positive).
func (a *API) handleAdminContentGuardDismiss(w http.ResponseWriter, r *http.Request) {
	if !a.requireAdmin(w, r) {
		return
	}

	var body struct {
		EntityType string `json:"entity_type"`
		EntityID   string `json:"entity_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Invalid JSON body")
		return
	}

	if body.EntityType == "" || body.EntityID == "" {
		writeError(w, http.StatusBadRequest, "invalid_input", "entity_type and entity_id are required")
		return
	}

	if err := a.db.DismissContentAlert(body.EntityType, body.EntityID); err != nil {
		a.logger.Error("Failed to dismiss content alert", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to dismiss alert")
		return
	}

	writeData(w, http.StatusOK, map[string]interface{}{"status": "dismissed"})
}

// GetUsageStats returns usage and capacity metrics for the instance.
// Admin-only: returns instance-wide metrics plus per-org breakdowns.
func (a *API) GetUsageStats(ctx context.Context) (map[string]interface{}, *APIError) {
	if apiErr := a.CheckAdmin(ctx); apiErr != nil {
		return nil, apiErr
	}

	maxUsers := a.policyInt("instance.max_active_users", 200)
	activeUsers := a.db.CountActiveUsers()
	waitlistCount := a.db.CountWaitlist()

	// User breakdown by status.
	userBreakdown := a.db.CountUsersByStatus()

	// User breakdown by type.
	userTypeBreakdown := a.db.CountUsersByType()

	// Per-org usage.
	orgUsage := a.buildOrgUsage(ctx)

	result := map[string]interface{}{
		"instance": map[string]interface{}{
			"max_active_users": maxUsers,
			"active_users":     activeUsers,
			"at_capacity":      maxUsers > 0 && activeUsers >= maxUsers,
			"waitlist_count":   waitlistCount,
			"capacity_pct":     capacityPct(activeUsers, maxUsers),
		},
		"users_by_status": userBreakdown,
		"users_by_type":   userTypeBreakdown,
		"org_usage":       orgUsage,
	}

	return result, nil
}

func capacityPct(active, max int) float64 {
	if max <= 0 {
		return 0
	}
	pct := float64(active) / float64(max) * 100
	if pct > 100 {
		pct = 100
	}
	return pct
}

func (a *API) buildOrgUsage(ctx context.Context) []map[string]interface{} {
	orgs, _, _ := a.db.ListOrganizations(storage.ListOrganizationsOpts{
		Status: "active",
		Limit:  100,
	})

	var result []map[string]interface{}
	for _, org := range orgs {
		edvCount := a.countOrgEndeavours(ctx, org.ID)
		agentCount := a.countOrgAgents(ctx, org.ID)

		// Count total members of the org.
		memberCount := 0
		rels, _, _ := a.db.ListRelations(storage.ListRelationsOpts{
			SourceEntityType: "organization",
			SourceEntityID:   org.ID,
			RelationshipType: "has_member",
			Limit:            1000,
		})
		memberCount = len(rels)

		// Quota limits for this org (use the owner's tier).
		maxEdv := -1
		maxAgents := -1
		ownerTier := a.findOrgOwnerTier(org.ID)
		if ownerTier > 0 {
			if td, err := a.db.GetTierDefinition(ownerTier); err == nil {
				maxEdv = td.MaxEndeavoursPerOrg
				maxAgents = td.MaxAgentsPerOrg
			}
		}

		entry := map[string]interface{}{
			"org_id":              org.ID,
			"org_name":            org.Name,
			"members":             memberCount,
			"agents":              agentCount,
			"endeavours":          edvCount,
			"max_endeavours":      maxEdv,
			"max_agents":          maxAgents,
		}
		result = append(result, entry)
	}
	return result
}

// findOrgOwnerTier returns the tier of the org's owner.
func (a *API) findOrgOwnerTier(orgID string) int {
	// Find the owner resource via has_member with role=owner.
	rels, _, _ := a.db.ListRelations(storage.ListRelationsOpts{
		SourceEntityType: "organization",
		SourceEntityID:   orgID,
		RelationshipType: "has_member",
		Limit:            100,
	})
	for _, rel := range rels {
		if role, ok := rel.Metadata["role"].(string); ok && role == "owner" {
			// Find the user with this resource_id.
			users, _, _ := a.db.ListUsers(storage.ListUsersOpts{Limit: 1000})
			for _, u := range users {
				if u.ResourceID != nil && *u.ResourceID == rel.TargetEntityID {
					return u.Tier
				}
			}
		}
	}
	return 0
}

// handleAdminUsage handles GET /api/v1/admin/usage.
func (a *API) handleAdminUsage(w http.ResponseWriter, r *http.Request) {
	result, apiErr := a.GetUsageStats(r.Context())
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}
	writeData(w, http.StatusOK, result)
}

// quotaKeys defines the non-tier policy keys exposed for admin quota configuration.
// Tier quotas are read from/written to the tier_definition table.
var quotaKeys = []string{
	"instance.max_active_users",
	"inactivity.warn_days",
	"inactivity.deactivate_days",
	"inactivity.sweep_capacity_threshold",
	"waitlist.notification_window_days",
}

// tierQuotaFields lists the tier_definition columns exposed as tier.N.field keys.
var tierQuotaFields = []string{
	"max_users",
	"max_orgs",
	"max_endeavours_per_org",
	"max_agents_per_org",
	"max_creations_per_hour",
	"max_active_endeavours",
	"max_teams_per_org",
}

// handleAdminQuotas handles GET /api/v1/admin/quotas.
func (a *API) handleAdminQuotas(w http.ResponseWriter, r *http.Request) {
	if apiErr := a.CheckAdmin(r.Context()); apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	result := map[string]string{}

	// Non-tier policy keys.
	for _, key := range quotaKeys {
		if v, err := a.db.GetPolicy(key); err == nil {
			result[key] = v
		}
	}

	// Flatten tier_definition rows into tier.N.field keys.
	tiers, err := a.db.ListTierDefinitions()
	if err == nil {
		for _, td := range tiers {
			result[fmt.Sprintf("tier.%d.max_users", td.ID)] = strconv.Itoa(td.MaxUsers)
			result[fmt.Sprintf("tier.%d.max_orgs", td.ID)] = strconv.Itoa(td.MaxOrgs)
			result[fmt.Sprintf("tier.%d.max_endeavours_per_org", td.ID)] = strconv.Itoa(td.MaxEndeavoursPerOrg)
			result[fmt.Sprintf("tier.%d.max_agents_per_org", td.ID)] = strconv.Itoa(td.MaxAgentsPerOrg)
			result[fmt.Sprintf("tier.%d.max_creations_per_hour", td.ID)] = strconv.Itoa(td.MaxCreationsPerHour)
			result[fmt.Sprintf("tier.%d.max_active_endeavours", td.ID)] = strconv.Itoa(td.MaxActiveEndeavours)
			result[fmt.Sprintf("tier.%d.max_teams_per_org", td.ID)] = strconv.Itoa(td.MaxTeamsPerOrg)
		}
	}

	writeData(w, http.StatusOK, result)
}

// handleAdminQuotasUpdate handles PATCH /api/v1/admin/quotas.
func (a *API) handleAdminQuotasUpdate(w http.ResponseWriter, r *http.Request) {
	if apiErr := a.CheckAdmin(r.Context()); apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	var body map[string]string
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Invalid JSON body")
		return
	}

	// Build sets of allowed keys.
	allowedPolicy := map[string]bool{}
	for _, k := range quotaKeys {
		allowedPolicy[k] = true
	}
	allowedTierField := map[string]bool{}
	for _, f := range tierQuotaFields {
		allowedTierField[f] = true
	}

	// Group tier updates by tier ID.
	tierUpdates := map[int]map[string]interface{}{}
	updated := map[string]string{}

	for key, value := range body {
		// Check for tier.N.field pattern.
		if strings.HasPrefix(key, "tier.") {
			parts := strings.SplitN(key, ".", 3)
			if len(parts) != 3 {
				writeError(w, http.StatusBadRequest, "invalid_input", fmt.Sprintf("Unknown quota key: %s", key))
				return
			}
			tierID, err := strconv.Atoi(parts[1])
			if err != nil || tierID < 1 {
				writeError(w, http.StatusBadRequest, "invalid_input", fmt.Sprintf("Invalid tier ID in key: %s", key))
				return
			}
			field := parts[2]
			if !allowedTierField[field] {
				writeError(w, http.StatusBadRequest, "invalid_input", fmt.Sprintf("Unknown tier field: %s", field))
				return
			}
			intVal, err := strconv.Atoi(value)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid_input", fmt.Sprintf("Value for %s must be an integer", key))
				return
			}
			if tierUpdates[tierID] == nil {
				tierUpdates[tierID] = map[string]interface{}{}
			}
			tierUpdates[tierID][field] = intVal
			updated[key] = value
			continue
		}

		// Non-tier policy key.
		if !allowedPolicy[key] {
			writeError(w, http.StatusBadRequest, "invalid_input", fmt.Sprintf("Unknown quota key: %s", key))
			return
		}
		if err := a.db.SetPolicy(key, value); err != nil {
			writeError(w, http.StatusInternalServerError, "internal", fmt.Sprintf("Failed to update %s", key))
			return
		}
		updated[key] = value
	}

	// Apply tier definition updates (invalidates cache per tier).
	for tierID, fields := range tierUpdates {
		if err := a.db.UpdateTierDefinition(tierID, fields); err != nil {
			writeError(w, http.StatusInternalServerError, "internal", fmt.Sprintf("Failed to update tier %d: %s", tierID, err))
			return
		}
	}

	writeData(w, http.StatusOK, updated)
}

// handleAdminAgentBlockSignals handles GET /api/v1/admin/agent-block-signals.
// Returns aggregated block signal data per sponsor (sponsors with blocked agents).
func (a *API) handleAdminAgentBlockSignals(w http.ResponseWriter, r *http.Request) {
	if !a.requireAdmin(w, r) {
		return
	}

	signals, err := a.db.GetAgentBlockSignals()
	if err != nil {
		a.logger.Error("Failed to get agent block signals", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get block signals")
		return
	}

	result := make([]map[string]interface{}, len(signals))
	for i, s := range signals {
		result[i] = map[string]interface{}{
			"sponsor_user_id": s.SponsorUserID,
			"sponsor_name":    s.SponsorName,
			"sponsor_email":   s.SponsorEmail,
			"total_agents":    s.TotalAgents,
			"blocked_count":   s.BlockedCount,
		}
	}

	writeData(w, http.StatusOK, result)
}

// handleAdminTierUsage handles GET /api/v1/admin/tier-usage.
// Returns per-tier entity counts for the admin dashboard.
// handleAdminTiers returns all tier definitions.
func (a *API) handleAdminTiers(w http.ResponseWriter, r *http.Request) {
	if !a.requireAdmin(w, r) {
		return
	}

	tiers, err := a.db.ListTierDefinitions()
	if err != nil {
		a.logger.Error("Failed to list tier definitions", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list tiers")
		return
	}

	writeData(w, http.StatusOK, tiers)
}

func (a *API) handleAdminTierUsage(w http.ResponseWriter, r *http.Request) {
	if !a.requireAdmin(w, r) {
		return
	}

	summary, err := a.db.GetTierUsageSummary()
	if err != nil {
		a.logger.Error("Failed to get tier usage summary", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get tier usage")
		return
	}

	writeData(w, http.StatusOK, summary)
}

// SetTaskschmiedStatusFunc registers a callback that returns the current
// Taskschmied status (circuit breaker stats, model info). Called from main
// after the ResilientClient is created.
func (a *API) SetTaskschmiedStatusFunc(fn func() map[string]interface{}) {
	a.taskschmiedStatusFn = fn
}

// SetTaskschmiedToggleFunc registers a callback to enable/disable Taskschmied
// LLM tiers independently. Target is "primary" or "fallback".
func (a *API) SetTaskschmiedToggleFunc(fn func(target string, disabled bool)) {
	a.taskschmiedToggleFn = fn
}

// handleAdminTaskschmiedToggle handles POST /api/v1/admin/taskschmied/toggle.
// Body: {"target": "primary"|"fallback", "disabled": true|false}
func (a *API) handleAdminTaskschmiedToggle(w http.ResponseWriter, r *http.Request) {
	if !a.requireAdmin(w, r) {
		return
	}
	if a.taskschmiedToggleFn == nil {
		writeError(w, http.StatusBadRequest, "not_configured", "Taskschmied is not configured")
		return
	}

	var body struct {
		Target   string `json:"target"`
		Disabled bool   `json:"disabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Invalid JSON body")
		return
	}
	if body.Target != "primary" && body.Target != "fallback" {
		writeError(w, http.StatusBadRequest, "invalid_input", "Target must be 'primary' or 'fallback'")
		return
	}

	a.taskschmiedToggleFn(body.Target, body.Disabled)
	writeData(w, http.StatusOK, map[string]interface{}{"target": body.Target, "disabled": body.Disabled})
}

// handleAdminTaskschmiedStatus handles GET /api/v1/admin/taskschmied/status.
func (a *API) handleAdminTaskschmiedStatus(w http.ResponseWriter, r *http.Request) {
	if !a.requireAdmin(w, r) {
		return
	}

	result := map[string]interface{}{
		"enabled": a.taskschmiedStatusFn != nil,
	}
	if a.taskschmiedStatusFn != nil {
		for k, v := range a.taskschmiedStatusFn() {
			result[k] = v
		}
	}

	writeData(w, http.StatusOK, result)
}
