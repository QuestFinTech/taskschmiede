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

	"github.com/QuestFinTech/taskschmiede/internal/auth"
	"github.com/QuestFinTech/taskschmiede/internal/security"
	"github.com/QuestFinTech/taskschmiede/internal/storage"
)

// handleAcceptConsent handles POST /api/v1/auth/consent (authenticated).
// Records that the user has accepted the current versions of terms and privacy.
func (a *API) handleAcceptConsent(w http.ResponseWriter, r *http.Request) {
	authUser := auth.GetAuthUser(r.Context())
	if authUser == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	var body struct {
		AcceptTerms   bool `json:"accept_terms"`
		AcceptPrivacy bool `json:"accept_privacy"`
		AcceptDPA     bool `json:"accept_dpa"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Invalid JSON body")
		return
	}

	if !body.AcceptTerms || !body.AcceptPrivacy {
		writeError(w, http.StatusBadRequest, "consent_required", "You must accept both Terms and Conditions and Privacy Policy")
		return
	}

	ip := security.ExtractIP(r)
	ua := r.Header.Get("User-Agent")

	// Get current required versions from policy table.
	termsVersion, _ := a.db.GetPolicy("legal.terms_version")
	if termsVersion == "" {
		termsVersion = "1.0.0"
	}
	privacyVersion, _ := a.db.GetPolicy("legal.privacy_version")
	if privacyVersion == "" {
		privacyVersion = "1.0.0"
	}

	// Record consent for terms.
	if _, err := a.db.CreateConsent(authUser.UserID, storage.ConsentTerms, termsVersion, "", ip, ua); err != nil {
		a.logger.Error("Failed to record terms consent", "user_id", authUser.UserID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to record consent")
		return
	}

	// Record consent for privacy.
	if _, err := a.db.CreateConsent(authUser.UserID, storage.ConsentPrivacy, privacyVersion, "", ip, ua); err != nil {
		a.logger.Error("Failed to record privacy consent", "user_id", authUser.UserID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to record consent")
		return
	}

	// Record consent for DPA if accepted.
	var dpaVersion string
	if body.AcceptDPA {
		dpaVersion, _ = a.db.GetPolicy("legal.dpa_version")
		if dpaVersion == "" {
			dpaVersion = "1.0.0"
		}
		if _, err := a.db.CreateConsent(authUser.UserID, storage.ConsentDPA, dpaVersion, "", ip, ua); err != nil {
			a.logger.Error("Failed to record DPA consent", "user_id", authUser.UserID, "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "Failed to record consent")
			return
		}
	}

	auditMeta := map[string]interface{}{
		"terms_version":   termsVersion,
		"privacy_version": privacyVersion,
	}
	if dpaVersion != "" {
		auditMeta["dpa_version"] = dpaVersion
	}

	if a.auditSvc != nil {
		a.auditSvc.Log(&security.AuditEntry{
			Action:    "consent_accepted",
			ActorID:   authUser.UserID,
			ActorType: "user",
			IP:        ip,
			Source:    auditSource(r),
			Metadata:  auditMeta,
		})
	}

	result := map[string]interface{}{
		"status":          "accepted",
		"terms_version":   termsVersion,
		"privacy_version": privacyVersion,
	}
	if dpaVersion != "" {
		result["dpa_version"] = dpaVersion
	}
	writeData(w, http.StatusOK, result)
}
