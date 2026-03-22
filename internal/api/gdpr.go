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
	"errors"
	"net/http"
	"strconv"

	"github.com/QuestFinTech/taskschmiede/internal/auth"
	"github.com/QuestFinTech/taskschmiede/internal/security"
	"github.com/QuestFinTech/taskschmiede/internal/storage"
)

// handleExportMyData handles GET /api/v1/auth/my-data (authenticated).
// Exports all user data as JSON (GDPR Article 15 - Right of Access).
func (a *API) handleExportMyData(w http.ResponseWriter, r *http.Request) {
	authUser := auth.GetAuthUser(r.Context())
	if authUser == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	backup, err := a.db.ExportUserData(authUser.UserID)
	if err != nil {
		a.logger.Error("Failed to export user data", "user_id", authUser.UserID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to export data")
		return
	}

	if a.auditSvc != nil {
		a.auditSvc.Log(&security.AuditEntry{
			Action:    "data_export",
			ActorID:   authUser.UserID,
			ActorType: "user",
			IP:        security.ExtractIP(r),
			Source:    auditSource(r),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=\"my-data.json\"")
	_ = json.NewEncoder(w).Encode(backup)
}

// handleRequestDeletion handles POST /api/v1/auth/delete-account (authenticated).
// Requests account deletion with a 30-day grace period (GDPR Article 17).
func (a *API) handleRequestDeletion(w http.ResponseWriter, r *http.Request) {
	authUser := auth.GetAuthUser(r.Context())
	if authUser == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	var body struct {
		Confirm bool `json:"confirm"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Invalid JSON body")
		return
	}
	if !body.Confirm {
		writeError(w, http.StatusBadRequest, "confirmation_required", "You must confirm the deletion request")
		return
	}

	// Get grace period from policy.
	graceDaysStr, _ := a.db.GetPolicy("retention.deletion_grace_days")
	graceDays := 30
	if g, err := strconv.Atoi(graceDaysStr); err == nil && g > 0 {
		graceDays = g
	}

	err := a.db.RequestDeletion(authUser.UserID, graceDays)
	if err != nil {
		if errors.Is(err, storage.ErrRetentionHold) {
			writeError(w, http.StatusForbidden, "retention_hold", "Account is under legal hold and cannot be deleted at this time")
			return
		}
		a.logger.Error("Failed to request deletion", "user_id", authUser.UserID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to process deletion request")
		return
	}

	if a.auditSvc != nil {
		a.auditSvc.Log(&security.AuditEntry{
			Action:    "deletion_requested",
			ActorID:   authUser.UserID,
			ActorType: "user",
			IP:        security.ExtractIP(r),
			Source:    auditSource(r),
			Metadata:  map[string]interface{}{"grace_days": graceDays},
		})
	}

	a.logger.Info("Account deletion requested", "user_id", authUser.UserID, "grace_days", graceDays)

	writeData(w, http.StatusOK, map[string]interface{}{
		"status":     "deletion_scheduled",
		"grace_days": graceDays,
		"message":    "Your account has been scheduled for deletion. You can cancel this within the grace period by logging in and visiting your profile.",
	})
}

// handleCancelDeletion handles POST /api/v1/auth/cancel-deletion (authenticated).
// Cancels a pending account deletion request.
func (a *API) handleCancelDeletion(w http.ResponseWriter, r *http.Request) {
	authUser := auth.GetAuthUser(r.Context())
	if authUser == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	if err := a.db.CancelDeletion(authUser.UserID); err != nil {
		a.logger.Error("Failed to cancel deletion", "user_id", authUser.UserID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to cancel deletion")
		return
	}

	if a.auditSvc != nil {
		a.auditSvc.Log(&security.AuditEntry{
			Action:    "deletion_cancelled",
			ActorID:   authUser.UserID,
			ActorType: "user",
			IP:        security.ExtractIP(r),
			Source:    auditSource(r),
		})
	}

	a.logger.Info("Account deletion cancelled", "user_id", authUser.UserID)

	writeData(w, http.StatusOK, map[string]interface{}{
		"status":  "deletion_cancelled",
		"message": "Your account deletion has been cancelled and your account is active again.",
	})
}

// handleDeletionStatus handles GET /api/v1/auth/deletion-status (authenticated).
// Returns the current deletion request status.
func (a *API) handleDeletionStatus(w http.ResponseWriter, r *http.Request) {
	authUser := auth.GetAuthUser(r.Context())
	if authUser == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	req, err := a.db.GetUserDeletionRequest(authUser.UserID)
	if err != nil {
		a.logger.Error("Failed to get deletion status", "user_id", authUser.UserID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get deletion status")
		return
	}

	result := map[string]interface{}{
		"pending": req.RequestedAt != nil,
	}
	if req.RequestedAt != nil {
		result["requested_at"] = req.RequestedAt.Format("2006-01-02T15:04:05Z07:00")
	}
	if req.ScheduledAt != nil {
		result["scheduled_at"] = req.ScheduledAt.Format("2006-01-02T15:04:05Z07:00")
	}

	writeData(w, http.StatusOK, result)
}
