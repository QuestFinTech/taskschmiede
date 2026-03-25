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
	"encoding/json"
	"net/http"
	"time"

	"github.com/QuestFinTech/taskschmiede/internal/auth"
	"github.com/QuestFinTech/taskschmiede/internal/security"
	"github.com/QuestFinTech/taskschmiede/internal/storage"
)

// handleMyAgentsList handles GET /api/v1/my-agents.
// Lists agents registered via the caller's invitation tokens.
func (a *API) handleMyAgentsList(w http.ResponseWriter, r *http.Request) {
	authUser := auth.GetAuthUser(r.Context())
	if authUser == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	limit := queryInt(r, "limit", 50)
	offset := queryInt(r, "offset", 0)

	agents, total, err := a.db.ListAgentsByOwner(authUser.UserID, limit, offset)
	if err != nil {
		a.logger.Error("Failed to list agents by owner", "user_id", authUser.UserID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list agents")
		return
	}

	result := make([]map[string]interface{}, len(agents))
	for i, agent := range agents {
		entry := map[string]interface{}{
			"id":         agent.ID,
			"name":       agent.Name,
			"email":      agent.Email,
			"status":     agent.Status,
			"user_type":  agent.UserType,
			"created_at": agent.CreatedAt.Format(time.RFC3339),
		}

		// Include health snapshot if available
		snap, err := a.db.GetAgentHealthSnapshot(agent.ID)
		if err == nil && snap != nil {
			entry["health_status"] = snap.Status
		}

		// Include onboarding status
		onboardingStatus, _ := a.db.GetUserOnboardingStatus(agent.ID)
		entry["onboarding_status"] = onboardingStatus

		result[i] = entry
	}

	writeList(w, result, total, limit, offset)
}

// handleMyAgentGet handles GET /api/v1/my-agents/{id}.
// Returns detailed info for an agent owned by the caller.
func (a *API) handleMyAgentGet(w http.ResponseWriter, r *http.Request) {
	authUser := auth.GetAuthUser(r.Context())
	if authUser == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	agentID := r.PathValue("id")

	// Verify ownership
	owned, err := a.db.IsAgentOwnedBy(agentID, authUser.UserID)
	if err != nil {
		a.logger.Error("Failed to check agent ownership", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get agent")
		return
	}
	if !owned {
		writeError(w, http.StatusNotFound, "not_found", "Agent not found")
		return
	}

	agent, err := a.db.GetUser(agentID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "Agent not found")
		return
	}

	result := map[string]interface{}{
		"id":         agent.ID,
		"name":       agent.Name,
		"email":      agent.Email,
		"status":     agent.Status,
		"user_type":  agent.UserType,
		"tier":       agent.Tier,
		"tier_name":  a.db.TierName(agent.Tier),
		"created_at": agent.CreatedAt.Format(time.RFC3339),
		"updated_at": agent.UpdatedAt.Format(time.RFC3339),
	}

	// Health snapshot
	snap, err := a.db.GetAgentHealthSnapshot(agentID)
	if err == nil && snap != nil {
		result["health"] = map[string]interface{}{
			"status":           snap.Status,
			"session_rate":     snap.SessionRate,
			"session_calls":    snap.SessionCalls,
			"rolling_24h_rate": snap.Rolling24hRate,
			"rolling_24h_calls": snap.Rolling24hCalls,
			"rolling_7d_rate":  snap.Rolling7dRate,
			"rolling_7d_calls": snap.Rolling7dCalls,
			"last_checked_at":  snap.LastCheckedAt.Format(time.RFC3339),
		}
	}

	// Onboarding status
	onboardingStatus, _ := a.db.GetUserOnboardingStatus(agentID)
	result["onboarding_status"] = onboardingStatus

	writeData(w, http.StatusOK, result)
}

// handleMyAgentUpdate handles PATCH /api/v1/my-agents/{id}.
// Allows the owner to update agent name or status (enable/disable/block/unblock).
func (a *API) handleMyAgentUpdate(w http.ResponseWriter, r *http.Request) {
	authUser := auth.GetAuthUser(r.Context())
	if authUser == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	agentID := r.PathValue("id")

	// Verify ownership
	owned, err := a.db.IsAgentOwnedBy(agentID, authUser.UserID)
	if err != nil {
		a.logger.Error("Failed to check agent ownership", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to update agent")
		return
	}
	if !owned {
		writeError(w, http.StatusNotFound, "not_found", "Agent not found")
		return
	}

	var body struct {
		Name   *string `json:"name"`
		Status *string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Invalid JSON body")
		return
	}

	// Validate status if provided
	if body.Status != nil {
		switch *body.Status {
		case "active", "inactive", "blocked":
			// allowed
		default:
			writeError(w, http.StatusBadRequest, "invalid_input", "Status must be 'active', 'inactive', or 'blocked'")
			return
		}
	}

	if body.Name != nil && *body.Name == "" {
		writeError(w, http.StatusBadRequest, "invalid_input", "Name cannot be empty")
		return
	}

	if body.Name == nil && body.Status == nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "At least one of name or status must be provided")
		return
	}

	// Handle block/unblock via dedicated storage methods
	if body.Status != nil && *body.Status == "blocked" {
		if err := a.db.BlockUser(agentID, "Blocked by sponsor"); err != nil {
			a.logger.Error("Failed to block agent", "agent_id", agentID, "error", err)
			writeError(w, http.StatusConflict, "block_failed", err.Error())
			return
		}
		// Revoke all tokens so the agent is locked out immediately
		if err := a.db.RevokeAllUserTokens(agentID); err != nil {
			a.logger.Error("Failed to revoke agent tokens on block", "agent_id", agentID, "error", err)
		}
		if a.auditSvc != nil {
			a.auditSvc.Log(&security.AuditEntry{
				Action:   "agent_blocked",
				ActorID:  authUser.UserID,
				Resource: agentID,
				Source:   auditSource(r),
			})
		}
		updated := []string{"status"}
		if body.Name != nil {
			if _, err := a.db.UpdateUser(agentID, storage.UpdateUserFields{Name: body.Name}); err == nil {
				updated = append(updated, "name")
				a.syncResourceName(r.Context(), agentID, *body.Name)
			}
		}
		writeData(w, http.StatusOK, map[string]interface{}{
			"status":         "updated",
			"updated_fields": updated,
		})
		return
	}

	// Check if this is an unblock (current status is blocked, target is active)
	if body.Status != nil && *body.Status == "active" {
		agent, err := a.db.GetUser(agentID)
		if err == nil && agent.Status == "blocked" {
			if err := a.db.UnblockUser(agentID); err != nil {
				a.logger.Error("Failed to unblock agent", "agent_id", agentID, "error", err)
				writeError(w, http.StatusInternalServerError, "internal_error", "Failed to unblock agent")
				return
			}
			if a.auditSvc != nil {
				a.auditSvc.Log(&security.AuditEntry{
					Action:   "agent_unblocked",
					ActorID:  authUser.UserID,
					Resource: agentID,
					Source:   auditSource(r),
				})
			}
			updated := []string{"status"}
			if body.Name != nil {
				if _, err := a.db.UpdateUser(agentID, storage.UpdateUserFields{Name: body.Name}); err == nil {
					updated = append(updated, "name")
					a.syncResourceName(r.Context(), agentID, *body.Name)
				}
			}
			writeData(w, http.StatusOK, map[string]interface{}{
				"status":         "updated",
				"updated_fields": updated,
			})
			return
		}
	}

	// Standard update (active/inactive, name)
	fields := storage.UpdateUserFields{
		Name:   body.Name,
		Status: body.Status,
	}

	updated, err := a.db.UpdateUser(agentID, fields)
	if err != nil {
		a.logger.Error("Failed to update agent", "agent_id", agentID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to update agent")
		return
	}

	// Sync name to linked resource so recipient/member lists stay current.
	if body.Name != nil {
		a.syncResourceName(r.Context(), agentID, *body.Name)
	}

	writeData(w, http.StatusOK, map[string]interface{}{
		"status":         "updated",
		"updated_fields": updated,
	})
}

// handleMyAgentActivity handles GET /api/v1/my-agents/{id}/activity.
// Returns the agent's audit log entries (simplified view).
func (a *API) handleMyAgentActivity(w http.ResponseWriter, r *http.Request) {
	authUser := auth.GetAuthUser(r.Context())
	if authUser == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	agentID := r.PathValue("id")

	// Verify ownership
	owned, err := a.db.IsAgentOwnedBy(agentID, authUser.UserID)
	if err != nil {
		a.logger.Error("Failed to check agent ownership", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get agent activity")
		return
	}
	if !owned {
		writeError(w, http.StatusNotFound, "not_found", "Agent not found")
		return
	}

	opts := storage.ListAuditLogOpts{
		ActorID: agentID,
		Action:  queryString(r, "action"),
		Limit:   queryInt(r, "limit", 50),
		Offset:  queryInt(r, "offset", 0),
	}

	entries, total, err := a.db.ListAuditLog(opts)
	if err != nil {
		a.logger.Error("Failed to list agent activity", "agent_id", agentID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get agent activity")
		return
	}

	result := make([]AuditActivityEntry, len(entries))
	for i, e := range entries {
		result[i] = AuditActivityEntry{
			Action:    e.Action,
			Resource:  e.Resource,
			Summary:   auditSummary(e),
			CreatedAt: e.CreatedAt.Format(time.RFC3339),
		}
	}

	writeList(w, result, total, opts.Limit, opts.Offset)
}

// handleMyAgentOnboarding handles GET /api/v1/my-agents/{id}/onboarding.
// Returns the agent's onboarding status and interview attempts.
func (a *API) handleMyAgentOnboarding(w http.ResponseWriter, r *http.Request) {
	authUser := auth.GetAuthUser(r.Context())
	if authUser == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	agentID := r.PathValue("id")

	// Verify ownership
	owned, err := a.db.IsAgentOwnedBy(agentID, authUser.UserID)
	if err != nil {
		a.logger.Error("Failed to check agent ownership", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get onboarding info")
		return
	}
	if !owned {
		writeError(w, http.StatusNotFound, "not_found", "Agent not found")
		return
	}

	status, _ := a.db.GetUserOnboardingStatus(agentID)

	result := map[string]interface{}{
		"agent_id":          agentID,
		"onboarding_status": status,
	}

	// Cooldown info
	cooldown, _ := a.db.GetCooldown(agentID)
	if cooldown != nil {
		cooldownInfo := map[string]interface{}{
			"failed_attempts": cooldown.FailedAttempts,
			"locked":          cooldown.Locked,
		}
		if cooldown.NextEligibleAt != nil {
			cooldownInfo["next_eligible_at"] = cooldown.NextEligibleAt.Format(time.RFC3339)
		}
		result["cooldown"] = cooldownInfo
	}

	// Interview attempts
	attempts, _ := a.db.ListOnboardingAttempts(agentID)
	if attempts != nil {
		attemptList := make([]map[string]interface{}, len(attempts))
		for i, a := range attempts {
			entry := map[string]interface{}{
				"id":          a.ID,
				"version":     a.Version,
				"status":      a.Status,
				"total_score": a.TotalScore,
				"started_at":  a.StartedAt.Format(time.RFC3339),
				"created_at":  a.CreatedAt.Format(time.RFC3339),
			}
			if a.CompletedAt != nil {
				entry["completed_at"] = a.CompletedAt.Format(time.RFC3339)
			}
			attemptList[i] = entry
		}
		result["attempts"] = attemptList
	}

	writeData(w, http.StatusOK, result)
}
