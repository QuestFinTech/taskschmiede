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


package ticker

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/QuestFinTech/taskschmiede/internal/service"
	"github.com/QuestFinTech/taskschmiede/internal/storage"
)

// Taskgovernor thresholds for agent health evaluation and Ablecon levels.
const (
	// Session: rate < 50% with 10+ calls -> warned
	sessionRateThreshold = 0.50
	sessionMinCalls      = 10

	// 24h rolling: rate < 60% with 20+ calls -> flagged
	rolling24hRateThreshold = 0.60
	rolling24hMinCalls      = 20

	// 7d rolling: rate < 50% with 50+ calls -> suspended
	rolling7dRateThreshold = 0.50
	rolling7dMinCalls      = 50

	// Sudden drop: 7d > 90% and session < 40% -> flagged (model swap)
	suddenDrop7dFloor     = 0.90
	suddenDropSessionCeil = 0.40

	// Ablecon system-wide thresholds (4-level DEFCON: 4=Blue, 3=Green, 2=Orange, 1=Red)
	ableconSystemOrangeRate        = 0.70 // 7d system rate < 70% -> Orange
	ableconSystemRedRate           = 0.50 // 7d system rate < 50% -> Red
	ableconFlaggedOrangePct        = 0.20 // >20% agents flagged -> Orange
	ableconFlaggedRedPct           = 0.50 // >50% agents flagged -> Red
	ableconSuspensionOrangeCount   = 3    // 3+ suspensions in 7d -> Orange
	ableconOrgDeviationOrangeDelta = 0.20 // org fail rate > system + 20pp -> Orange
	ableconOrgDeviationRedDelta    = 0.50 // org fail rate > system + 50pp -> Red
	ableconOrgSuspensionRedCount   = 3    // 3+ org suspensions in 7d -> Red
)

// NewTaskgovernorHandler returns a handler that evaluates agent health metrics,
// enforces behavioral thresholds, and computes Ablecon traffic light levels.
func NewTaskgovernorHandler(db *storage.DB, msgSvc *service.MessageService, logger *slog.Logger) Handler {
	return Handler{
		Name:     "taskgovernor",
		Interval: 5 * time.Minute,
		Fn:       taskgovernorCheck(db, msgSvc, logger),
	}
}

func taskgovernorCheck(db *storage.DB, msgSvc *service.MessageService, logger *slog.Logger) func(context.Context, time.Time) error {
	return func(ctx context.Context, now time.Time) error {
		agentIDs, err := db.ListActiveAgentUserIDs()
		if err != nil {
			return err
		}
		if len(agentIDs) == 0 {
			return nil
		}

		// Step 1: Evaluate each agent's health
		for _, userID := range agentIDs {
			evaluateAgent(db, logger, userID, now)
		}

		// Step 2: Compute Ablecon levels
		computeAblecon(ctx, db, msgSvc, logger, agentIDs, now)

		return nil
	}
}

func evaluateAgent(db *storage.DB, logger *slog.Logger, userID string, now time.Time) {
	// Compute session rate (current active session)
	sessionRate, sessionCalls, err := db.AgentCurrentSessionRate(userID)
	if err != nil {
		logger.Warn("taskgovernor: session rate query failed", "user_id", userID, "error", err)
		return
	}

	// Compute 24h rolling rate
	rate24h, calls24h, err := db.AgentRollingRate(userID, now.Add(-24*time.Hour))
	if err != nil {
		logger.Warn("taskgovernor: 24h rolling rate query failed", "user_id", userID, "error", err)
		return
	}

	// Compute 7d rolling rate
	rate7d, calls7d, err := db.AgentRollingRate(userID, now.Add(-7*24*time.Hour))
	if err != nil {
		logger.Warn("taskgovernor: 7d rolling rate query failed", "user_id", userID, "error", err)
		return
	}

	// Determine health status
	status := "healthy"

	// Check thresholds in order of severity (most severe last wins)
	if sessionCalls >= sessionMinCalls && sessionRate < sessionRateThreshold {
		status = "warned"
		logger.Warn("taskgovernor: session rate below threshold",
			"user_id", userID,
			"session_rate", sessionRate,
			"session_calls", sessionCalls,
		)
	}

	if calls24h >= rolling24hMinCalls && rate24h < rolling24hRateThreshold {
		status = "flagged"
		logger.Warn("taskgovernor: 24h rolling rate below threshold",
			"user_id", userID,
			"rolling_24h_rate", rate24h,
			"rolling_24h_calls", calls24h,
		)
	}

	// Sudden drop detection: 7d baseline > 90% but session < 40%
	if calls7d >= rolling7dMinCalls && rate7d > suddenDrop7dFloor &&
		sessionCalls >= sessionMinCalls && sessionRate < suddenDropSessionCeil {
		if status != "flagged" {
			status = "flagged"
		}
		logger.Warn("taskgovernor: sudden performance drop detected (possible model swap)",
			"user_id", userID,
			"rolling_7d_rate", rate7d,
			"session_rate", sessionRate,
		)
	}

	if calls7d >= rolling7dMinCalls && rate7d < rolling7dRateThreshold {
		status = "suspended"
		logger.Warn("taskgovernor: 7d rolling rate below threshold, suspending agent",
			"user_id", userID,
			"rolling_7d_rate", rate7d,
			"rolling_7d_calls", calls7d,
		)
	}

	// Upsert health snapshot
	snap := &storage.AgentHealthSnapshot{
		ID:              storage.GenerateID("ahs"),
		UserID:          userID,
		SessionCalls:    sessionCalls,
		Rolling24hCalls: calls24h,
		Rolling7dCalls:  calls7d,
		Status:          status,
		LastCheckedAt:   now,
		Metadata:        "{}",
	}
	if sessionCalls > 0 {
		snap.SessionRate = &sessionRate
	}
	if calls24h > 0 {
		snap.Rolling24hRate = &rate24h
	}
	if calls7d > 0 {
		snap.Rolling7dRate = &rate7d
	}

	if err := db.UpsertAgentHealthSnapshot(snap); err != nil {
		logger.Warn("taskgovernor: failed to upsert health snapshot", "user_id", userID, "error", err)
		return
	}

	// Suspend the agent if threshold breached
	if status == "suspended" {
		if err := db.SetUserOnboardingStatus(userID, "suspended"); err != nil {
			logger.Warn("taskgovernor: failed to suspend agent", "user_id", userID, "error", err)
		}
	}
}

// computeAblecon evaluates system-wide and per-org Ablecon levels.
func computeAblecon(ctx context.Context, db *storage.DB, msgSvc *service.MessageService, logger *slog.Logger, agentIDs []string, now time.Time) {
	// Get health snapshots for all agents
	snapshots, err := db.ListAgentHealthSnapshots("")
	if err != nil {
		logger.Warn("taskgovernor: failed to list health snapshots for ablecon", "error", err)
		return
	}
	if len(snapshots) == 0 {
		return
	}

	// Compute system-wide 7d rate and count flagged agents
	var totalSuccessful7d, totalCalls7d int
	flaggedCount := 0
	for _, s := range snapshots {
		if s.Rolling7dRate != nil && s.Rolling7dCalls > 0 {
			totalSuccessful7d += int(float64(s.Rolling7dCalls) * (*s.Rolling7dRate))
			totalCalls7d += s.Rolling7dCalls
		}
		if s.Status == "flagged" || s.Status == "suspended" {
			flaggedCount++
		}
	}

	systemRate7d := 0.0
	if totalCalls7d > 0 {
		systemRate7d = float64(totalSuccessful7d) / float64(totalCalls7d)
	}

	// Count recent suspensions
	suspensions7d, _ := db.CountRecentSuspensions(now.Add(-7*24*time.Hour), nil)

	// Determine system Ablecon level (DEFCON: 4=Blue, 3=Green, 2=Orange, 1=Red)
	systemLevel := 4 // Blue: normal, no signals
	reason := map[string]interface{}{
		"system_7d_rate": systemRate7d,
		"total_agents":   len(agentIDs),
		"flagged_agents": flaggedCount,
		"suspensions_7d": suspensions7d,
	}

	flaggedPct := 0.0
	if len(agentIDs) > 0 {
		flaggedPct = float64(flaggedCount) / float64(len(agentIDs))
	}

	// Any warned or flagged agent -> Green (minor signals)
	if flaggedCount > 0 {
		systemLevel = 3
	}

	// System thresholds breached -> Orange (issues detected)
	if totalCalls7d > 0 && systemRate7d < ableconSystemOrangeRate {
		systemLevel = 2
	}
	if flaggedPct > ableconFlaggedOrangePct {
		systemLevel = 2
	}
	if suspensions7d >= ableconSuspensionOrangeCount {
		systemLevel = 2
	}

	// Severe -> Red (critical)
	if totalCalls7d > 0 && systemRate7d < ableconSystemRedRate {
		systemLevel = 1
	}
	if flaggedPct > ableconFlaggedRedPct {
		systemLevel = 1
	}

	reason["level"] = systemLevel
	prevLevel, err := db.UpsertAbleconLevel("system", "", systemLevel, reason)
	if err != nil {
		logger.Warn("taskgovernor: failed to upsert system ablecon", "error", err)
	} else if prevLevel != 0 && prevLevel != systemLevel {
		levelLabel := ableconLabel(systemLevel)
		logger.Warn("taskgovernor: system ablecon level changed",
			"from", ableconLabel(prevLevel),
			"to", levelLabel,
			"rate_7d", systemRate7d,
		)
		notifyAbleconChange(ctx, db, msgSvc, logger, "system", "",
			fmt.Sprintf("System Ablecon changed to %s (level %d)", levelLabel, systemLevel), reason)
	}

	// Per-organization Ablecon
	agentsByOrg, err := db.AgentsByOrganization()
	if err != nil {
		logger.Warn("taskgovernor: failed to get agents by org", "error", err)
		return
	}

	// Build a user->snapshot lookup
	snapByUser := make(map[string]*storage.AgentHealthSnapshot)
	for _, s := range snapshots {
		snapByUser[s.UserID] = s
	}

	systemFailRate := 1.0 - systemRate7d

	for orgID, orgAgentIDs := range agentsByOrg {
		computeOrgAblecon(ctx, db, msgSvc, logger, orgID, orgAgentIDs, snapByUser, systemFailRate, now)
	}
}

func computeOrgAblecon(ctx context.Context, db *storage.DB, msgSvc *service.MessageService, logger *slog.Logger, orgID string, orgAgentIDs []string, snapByUser map[string]*storage.AgentHealthSnapshot, systemFailRate float64, now time.Time) {
	var orgSuccessful, orgCalls int
	var minRate, maxRate float64
	hasRates := false

	for _, uid := range orgAgentIDs {
		s, ok := snapByUser[uid]
		if !ok || s.Rolling7dRate == nil || s.Rolling7dCalls == 0 {
			continue
		}
		orgSuccessful += int(float64(s.Rolling7dCalls) * (*s.Rolling7dRate))
		orgCalls += s.Rolling7dCalls
		rate := *s.Rolling7dRate
		if !hasRates {
			minRate = rate
			maxRate = rate
			hasRates = true
		} else {
			if rate < minRate {
				minRate = rate
			}
			if rate > maxRate {
				maxRate = rate
			}
		}
	}

	orgLevel := 4 // Blue: normal
	reason := map[string]interface{}{"org_id": orgID, "agent_count": len(orgAgentIDs)}

	if orgCalls > 0 {
		orgRate := float64(orgSuccessful) / float64(orgCalls)
		orgFailRate := 1.0 - orgRate
		reason["org_7d_rate"] = orgRate

		// Check for any flagged/warned agents -> Green
		for _, uid := range orgAgentIDs {
			if s, ok := snapByUser[uid]; ok && (s.Status == "flagged" || s.Status == "warned") {
				if orgLevel > 3 {
					orgLevel = 3
				}
			}
		}

		if orgFailRate > systemFailRate+ableconOrgDeviationOrangeDelta {
			orgLevel = 2
		}
		if orgFailRate > systemFailRate+ableconOrgDeviationRedDelta {
			orgLevel = 1
		}
	}

	// Within-org variance: one agent >90%, another <50%
	if hasRates && maxRate > 0.90 && minRate < 0.50 {
		if orgLevel > 2 {
			orgLevel = 2
		}
		reason["variance"] = map[string]interface{}{"min": minRate, "max": maxRate}
	}

	// Org suspensions
	orgSuspensions, _ := db.CountRecentSuspensions(now.Add(-7*24*time.Hour), orgAgentIDs)
	if orgSuspensions >= ableconOrgSuspensionRedCount {
		orgLevel = 1
	}
	reason["suspensions_7d"] = orgSuspensions

	reason["level"] = orgLevel
	prevLevel, err := db.UpsertAbleconLevel("organization", orgID, orgLevel, reason)
	if err != nil {
		logger.Warn("taskgovernor: failed to upsert org ablecon", "org_id", orgID, "error", err)
	} else if prevLevel != 0 && prevLevel != orgLevel {
		levelLabel := ableconLabel(orgLevel)
		logger.Warn("taskgovernor: org ablecon level changed",
			"org_id", orgID,
			"from", ableconLabel(prevLevel),
			"to", levelLabel,
		)
		notifyAbleconChange(ctx, db, msgSvc, logger, "organization", orgID,
			fmt.Sprintf("Organization Ablecon changed to %s (level %d)", levelLabel, orgLevel), reason)
	}
}

func ableconLabel(level int) string {
	return storage.AbleconLevelLabel(level)
}

// notifyAbleconChange sends a message to the master admin about an Ablecon level change.
func notifyAbleconChange(ctx context.Context, db *storage.DB, msgSvc *service.MessageService, logger *slog.Logger, scope, scopeID, subject string, reason map[string]interface{}) {
	if msgSvc == nil {
		return
	}

	adminID := db.GetMasterAdminUserID()
	if adminID == "" {
		return
	}

	content := fmt.Sprintf("Ablecon level change detected.\nScope: %s", scope)
	if scopeID != "" {
		content += fmt.Sprintf("\nOrganization: %s", scopeID)
	}
	for k, v := range reason {
		content += fmt.Sprintf("\n%s: %v", k, v)
	}

	_, err := msgSvc.Send(ctx, adminID, subject, content, "alert", "", "", "", []string{adminID}, "", "", nil)
	if err != nil {
		logger.Warn("taskgovernor: failed to send ablecon notification", "error", err)
	}
}
