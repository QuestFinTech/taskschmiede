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

	"github.com/QuestFinTech/taskschmiede/internal/auth"
	"github.com/QuestFinTech/taskschmiede/internal/storage"
)

// handleMyAlertsList handles GET /api/v1/my-alerts.
// Returns content guard alerts for entities created by or assigned to the caller's agents.
func (a *API) handleMyAlertsList(w http.ResponseWriter, r *http.Request) {
	authUser := auth.GetAuthUser(r.Context())
	if authUser == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	limit := queryInt(r, "limit", 50)
	offset := queryInt(r, "offset", 0)
	threshold := queryInt(r, "threshold", 1)

	// Get all agents owned by this user.
	agents, _, err := a.db.ListAgentsByOwner(authUser.UserID, 1000, 0)
	if err != nil {
		a.logger.Error("Failed to list agents for alerts", "user_id", authUser.UserID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get alerts")
		return
	}

	var agentUserIDs []string
	var agentResourceIDs []string
	for _, agent := range agents {
		agentUserIDs = append(agentUserIDs, agent.ID)
		if agent.ResourceID != nil && *agent.ResourceID != "" {
			agentResourceIDs = append(agentResourceIDs, *agent.ResourceID)
		}
	}

	if len(agentUserIDs) == 0 {
		writeList(w, []interface{}{}, 0, limit, offset)
		return
	}

	alerts, total, err := a.db.ListContentAlertsByAgents(agentUserIDs, agentResourceIDs, threshold, limit, offset)
	if err != nil {
		a.logger.Error("Failed to list content alerts", "user_id", authUser.UserID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get alerts")
		return
	}

	writeList(w, alerts, total, limit, offset)
}

// handleMyAlertsStats handles GET /api/v1/my-alerts/stats.
// Returns content guard statistics for entities created by or assigned to the caller's agents.
func (a *API) handleMyAlertsStats(w http.ResponseWriter, r *http.Request) {
	authUser := auth.GetAuthUser(r.Context())
	if authUser == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	agents, _, err := a.db.ListAgentsByOwner(authUser.UserID, 1000, 0)
	if err != nil {
		a.logger.Error("Failed to list agents for alert stats", "user_id", authUser.UserID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get stats")
		return
	}

	var agentUserIDs []string
	var agentResourceIDs []string
	for _, agent := range agents {
		agentUserIDs = append(agentUserIDs, agent.ID)
		if agent.ResourceID != nil && *agent.ResourceID != "" {
			agentResourceIDs = append(agentResourceIDs, *agent.ResourceID)
		}
	}

	stats, err := a.db.GetContentGuardStatsByAgents(agentUserIDs, agentResourceIDs)
	if err != nil {
		a.logger.Error("Failed to get content guard stats", "user_id", authUser.UserID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get stats")
		return
	}

	writeData(w, http.StatusOK, stats)
}

// handleMyIndicators handles GET /api/v1/my-indicators.
// Returns Ablecon and Harmcon levels scoped to the caller's agents.
func (a *API) handleMyIndicators(w http.ResponseWriter, r *http.Request) {
	authUser := auth.GetAuthUser(r.Context())
	if authUser == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	agents, _, err := a.db.ListAgentsByOwner(authUser.UserID, 1000, 0)
	if err != nil {
		a.logger.Error("Failed to list agents for indicators", "user_id", authUser.UserID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get indicators")
		return
	}

	var agentUserIDs []string
	var agentResourceIDs []string
	for _, agent := range agents {
		agentUserIDs = append(agentUserIDs, agent.ID)
		if agent.ResourceID != nil && *agent.ResourceID != "" {
			agentResourceIDs = append(agentResourceIDs, *agent.ResourceID)
		}
	}

	ablecon, err := a.db.GetUserAbleconLevel(agentUserIDs)
	if err != nil {
		a.logger.Error("Failed to get user ablecon", "user_id", authUser.UserID, "error", err)
		ablecon = &storage.AbleconLevel{Level: 4, Reason: map[string]interface{}{}}
	}

	harmcon, err := a.db.GetUserHarmconLevel(agentUserIDs, agentResourceIDs)
	if err != nil {
		a.logger.Error("Failed to get user harmcon", "user_id", authUser.UserID, "error", err)
		harmcon = &storage.HarmconLevel{Level: 4}
	}

	writeData(w, http.StatusOK, map[string]interface{}{
		"ablecon": map[string]interface{}{
			"level":  ablecon.Level,
			"label":  storage.AbleconLevelLabel(ablecon.Level),
			"reason": ablecon.Reason,
		},
		"harmcon": map[string]interface{}{
			"level":        harmcon.Level,
			"label":        storage.AbleconLevelLabel(harmcon.Level),
			"high_count":   harmcon.HighCount,
			"medium_count": harmcon.MediumCount,
			"low_count":    harmcon.LowCount,
		},
	})
}
