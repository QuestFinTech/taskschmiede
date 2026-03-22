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
	"fmt"
	"net/http"
	"time"

	"github.com/QuestFinTech/taskschmiede/internal/auth"
	"github.com/QuestFinTech/taskschmiede/internal/storage"
)

// handleOnboardingStatus handles GET /api/v1/onboarding/status.
// Returns the authenticated user's onboarding status, cooldown, and attempt history.
func (a *API) handleOnboardingStatus(w http.ResponseWriter, r *http.Request) {
	user := auth.GetAuthUser(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	status, err := a.db.GetUserOnboardingStatus(user.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get onboarding status")
		return
	}

	result := map[string]interface{}{
		"onboarding_status": status,
		"user_id":           user.UserID,
	}

	// Add cooldown info
	cooldown, _ := a.db.GetCooldown(user.UserID)
	if cooldown != nil {
		cooldownInfo := map[string]interface{}{
			"failed_attempts": cooldown.FailedAttempts,
			"locked":          cooldown.Locked,
		}
		if cooldown.NextEligibleAt != nil {
			cooldownInfo["next_eligible_at"] = cooldown.NextEligibleAt.Format(time.RFC3339)
			remaining := cooldown.NextEligibleAt.Sub(storage.UTCNow())
			if remaining > 0 {
				cooldownInfo["wait_remaining"] = formatOnboardingDuration(remaining)
			}
		}
		result["cooldown"] = cooldownInfo
	}

	// Add attempt history
	attempts, _ := a.db.ListOnboardingAttempts(user.UserID)
	if len(attempts) > 0 {
		var attemptList []map[string]interface{}
		for _, att := range attempts {
			entry := map[string]interface{}{
				"id":         att.ID,
				"version":    att.Version,
				"status":     att.Status,
				"score":      att.TotalScore,
				"started_at": att.StartedAt.Format(time.RFC3339),
			}
			if att.CompletedAt != nil {
				entry["completed_at"] = att.CompletedAt.Format(time.RFC3339)
			}
			attemptList = append(attemptList, entry)
		}
		result["attempts"] = attemptList
	}

	writeData(w, http.StatusOK, result)
}

// formatOnboardingDuration formats a duration in human-readable form.
func formatOnboardingDuration(d time.Duration) string {
	if d >= 24*time.Hour {
		days := int(d.Hours() / 24)
		return fmt.Sprintf("%d day(s)", days)
	}
	if d >= time.Hour {
		return fmt.Sprintf("%d hour(s)", int(d.Hours()))
	}
	return fmt.Sprintf("%d minute(s)", int(d.Minutes()))
}
