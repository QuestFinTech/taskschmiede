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
	"net/http"
	"strconv"
	"time"

	"github.com/QuestFinTech/taskschmiede/internal/auth"
)

// handleInjectionReviewList returns paginated injection reviews (admin only).
func (a *API) handleInjectionReviewList(w http.ResponseWriter, r *http.Request) {
	user := auth.GetAuthUser(r.Context())
	if user == nil || user.UserType != "human" || !a.authSvc.IsMasterAdmin(r.Context(), user.UserID) {
		writeError(w, http.StatusForbidden, "forbidden", "Admin access required")
		return
	}

	status := r.URL.Query().Get("status")
	flaggedOnly := r.URL.Query().Get("flagged") == "true"

	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}

	offset := 0
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}

	reviews, err := a.db.ListInjectionReviews(status, flaggedOnly, limit, offset)
	if err != nil {
		a.logger.Error("Failed to list injection reviews", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list reviews")
		return
	}

	items := make([]map[string]interface{}, 0, len(reviews))
	for _, r := range reviews {
		entry := map[string]interface{}{
			"id":                 r.ID,
			"attempt_id":        r.AttemptID,
			"status":            r.Status,
			"provider":          r.Provider,
			"model":             r.Model,
			"injection_detected": r.InjectionDetected,
			"confidence":        r.Confidence,
			"evidence":          r.Evidence,
			"error_message":     r.ErrorMessage,
			"retries":           r.Retries,
			"created_at":        r.CreatedAt.Format(time.RFC3339),
		}
		if r.CompletedAt != nil {
			entry["completed_at"] = r.CompletedAt.Format(time.RFC3339)
		}
		items = append(items, entry)
	}

	writeData(w, http.StatusOK, map[string]interface{}{
		"items":  items,
		"limit":  limit,
		"offset": offset,
	})
}

// handleInjectionReviewGet returns a single injection review by ID (admin only).
func (a *API) handleInjectionReviewGet(w http.ResponseWriter, r *http.Request) {
	user := auth.GetAuthUser(r.Context())
	if user == nil || user.UserType != "human" || !a.authSvc.IsMasterAdmin(r.Context(), user.UserID) {
		writeError(w, http.StatusForbidden, "forbidden", "Admin access required")
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Review ID is required")
		return
	}

	review, err := a.db.GetInjectionReview(id)
	if err != nil {
		a.logger.Error("Failed to get injection review", "id", id, "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get review")
		return
	}
	if review == nil {
		writeError(w, http.StatusNotFound, "not_found", "Review not found")
		return
	}

	result := map[string]interface{}{
		"id":                 review.ID,
		"attempt_id":        review.AttemptID,
		"status":            review.Status,
		"provider":          review.Provider,
		"model":             review.Model,
		"injection_detected": review.InjectionDetected,
		"confidence":        review.Confidence,
		"evidence":          review.Evidence,
		"raw_response":      review.RawResponse,
		"error_message":     review.ErrorMessage,
		"retries":           review.Retries,
		"created_at":        review.CreatedAt.Format(time.RFC3339),
	}
	if review.CompletedAt != nil {
		result["completed_at"] = review.CompletedAt.Format(time.RFC3339)
	}

	writeData(w, http.StatusOK, result)
}
