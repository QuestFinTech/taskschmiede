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
	"encoding/json"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/QuestFinTech/taskschmiede/internal/onboarding"
	"github.com/QuestFinTech/taskschmiede/internal/storage"
)

// registerOnboardingTools registers the production onboarding MCP tools.
// These tools run on the main MCP endpoint and manage the interview lifecycle.
func (s *Server) registerOnboardingTools(mcpServer *mcp.Server) {
	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.onboard.start_interview",
			Description: "Start an onboarding interview. Requires the user to have interview_pending status. Returns the interview session ID and endpoint URL.",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		s.withSessionAuth(s.handleOnboardStartInterview),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.onboard.status",
			Description: "Get the current onboarding status, attempt history, and cooldown info for the authenticated user.",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		s.withSessionAuth(s.handleOnboardStatus),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.onboard.step0",
			Description: "Submit Step 0 self-description. This is the unscored first step of the interview where you describe yourself, your model, and your capabilities.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"description": map[string]interface{}{
						"type":        "string",
						"description": "Your self-description including name, model, and capabilities",
					},
				},
				"required": []string{"description"},
			},
		},
		s.withSessionAuth(s.handleOnboardStep0),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.onboard.next_challenge",
			Description: "Get the current interview challenge. Returns the challenge text for the current section, or the interview result if complete.",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		s.withSessionAuth(s.handleOnboardNextChallenge),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.onboard.complete",
			Description: "Signal that you have completed all challenges. Triggers evaluation and returns your interview result.",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		s.withSessionAuth(s.handleOnboardComplete),
	)

	// ts.onboard.submit is an interview-only tool. During an active interview,
	// checkOnboardingGate routes it to the simulation handler before this
	// production handler runs. We register it here so MCP clients can discover
	// the tool in the tool list.
	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.onboard.submit",
			Description: "Submit structured answers for the current interview section. Pass the key-value pairs requested by the challenge (e.g., done_count, newest_title, total_estimate). Only available during an active interview.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"done_count":          map[string]interface{}{"type": "integer", "description": "Number of tasks in done status"},
					"newest_title":        map[string]interface{}{"type": "string", "description": "Title of the most recently created task"},
					"total_estimate":      map[string]interface{}{"type": "number", "description": "Sum of all task estimates"},
					"planned_count":       map[string]interface{}{"type": "integer", "description": "Number of tasks in planned status"},
					"high_priority_demand": map[string]interface{}{"type": "string", "description": "Title of the highest-priority demand"},
					"linked_task_count":    map[string]interface{}{"type": "integer", "description": "Number of tasks linked to a demand"},
					"responsible_party":    map[string]interface{}{"type": "string", "description": "Who is accountable for sub-agent traffic through your credentials"},
					"rate_limit_response":  map[string]interface{}{"type": "string", "description": "What to do when you hit a rate limit"},
					"monitoring_intent":    map[string]interface{}{"type": "string", "description": "Is behavioral monitoring intended to punish or protect"},
					"citizenship_pledge":   map[string]interface{}{"type": "string", "description": "Your commitment to responsible platform usage"},
				},
				"additionalProperties": true,
			},
		},
		s.withSessionAuth(s.handleOnboardSubmitFallback),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.onboard.health",
			Description: "Admin-only tool: view agent behavioral health dashboard including per-agent success rates, health statuses, and Ablecon traffic light levels.",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		s.withSessionAuth(s.handleOnboardHealth),
	)
}

// handleOnboardSubmitFallback is the production-side handler for ts.onboard.submit.
// During an active interview, checkOnboardingGate routes this tool to the simulation
// before this handler runs. This only executes if called outside an interview context.
func (s *Server) handleOnboardSubmitFallback(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return toolError("no_interview", "ts.onboard.submit is only available during an active interview."), nil
}

// handleOnboardHealth returns the agent behavioral health dashboard (admin only).
func (s *Server) handleOnboardHealth(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	user, err := requireAuth(ctx)
	if err != nil {
		return toolError("unauthorized", err.Error()), nil
	}
	if user.UserType != "human" || !s.authSvc.IsMasterAdmin(ctx, user.UserID) {
		return toolError("forbidden", "This tool is only available to human administrators."), nil
	}

	// All agent health snapshots
	allSnapshots, err := s.db.ListAgentHealthSnapshots("")
	if err != nil {
		return toolError("internal_error", "Failed to list health snapshots"), nil
	}

	// Count by status
	counts := map[string]int{"healthy": 0, "warned": 0, "flagged": 0, "suspended": 0}
	agents := make([]map[string]interface{}, 0, len(allSnapshots))
	for _, snap := range allSnapshots {
		counts[snap.Status]++
		agent := map[string]interface{}{
			"user_id":          snap.UserID,
			"status":           snap.Status,
			"session_calls":    snap.SessionCalls,
			"rolling_24h_calls": snap.Rolling24hCalls,
			"rolling_7d_calls": snap.Rolling7dCalls,
			"last_checked_at":  snap.LastCheckedAt.Format(time.RFC3339),
		}
		if snap.SessionRate != nil {
			agent["session_rate"] = *snap.SessionRate
		}
		if snap.Rolling24hRate != nil {
			agent["rolling_24h_rate"] = *snap.Rolling24hRate
		}
		if snap.Rolling7dRate != nil {
			agent["rolling_7d_rate"] = *snap.Rolling7dRate
		}
		agents = append(agents, agent)
	}

	// Ablecon levels
	ablecon := map[string]interface{}{}
	sysLevel, err := s.db.GetSystemAbleconLevel()
	if err == nil && sysLevel != nil {
		ablecon["system"] = map[string]interface{}{
			"level":  sysLevel.Level,
			"label":  ableconLabelMCP(sysLevel.Level),
			"reason": sysLevel.Reason,
		}
	}

	orgLevels, err := s.db.ListOrgAbleconLevels()
	if err == nil && len(orgLevels) > 0 {
		orgList := make([]map[string]interface{}, 0, len(orgLevels))
		for _, o := range orgLevels {
			orgList = append(orgList, map[string]interface{}{
				"org_id": o.ScopeID,
				"level":  o.Level,
				"label":  ableconLabelMCP(o.Level),
				"reason": o.Reason,
			})
		}
		ablecon["organizations"] = orgList
	}

	result := map[string]interface{}{
		"total_agents": len(allSnapshots),
		"by_status":    counts,
		"agents":       agents,
		"ablecon":      ablecon,
	}

	// Add injection review summary
	flaggedCount, _ := s.db.CountFlaggedReviews()
	statusCounts, _ := s.db.CountInjectionReviewsByStatus()
	injectionInfo := map[string]interface{}{
		"total_flagged": flaggedCount,
		"by_status":     statusCounts,
	}

	// Include recent flagged reviews
	flagged, _ := s.db.ListInjectionReviews("", true, 10, 0)
	if len(flagged) > 0 {
		flaggedList := make([]map[string]interface{}, 0, len(flagged))
		for _, r := range flagged {
			entry := map[string]interface{}{
				"review_id":  r.ID,
				"attempt_id": r.AttemptID,
				"confidence": r.Confidence,
				"evidence":   r.Evidence,
				"provider":   r.Provider,
				"model":      r.Model,
			}
			flaggedList = append(flaggedList, entry)
		}
		injectionInfo["recent_flagged"] = flaggedList
	}
	result["injection_reviews"] = injectionInfo

	return toolSuccess(result), nil
}

// ableconLabelMCP returns the human-readable label for an Ablecon traffic light level.
func ableconLabelMCP(level int) string {
	return storage.AbleconLevelLabel(level)
}

// handleOnboardStartInterview starts a new interview session.
func (s *Server) handleOnboardStartInterview(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	user, err := requireAuth(ctx)
	if err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	// Check onboarding status
	status, err := s.db.GetUserOnboardingStatus(user.UserID)
	if err != nil {
		s.logger.Error("failed to get onboarding status", "user_id", user.UserID, "error", err)
		return toolError("internal_error", "Failed to check onboarding status"), nil
	}

	if status == "active" {
		return toolError("already_active", "You are already fully onboarded."), nil
	}
	if status != "interview_pending" && status != "cooldown" {
		return toolError("invalid_state", fmt.Sprintf("Cannot start interview in state: %s", status)), nil
	}

	// Check cooldown
	cooldown, err := s.db.GetCooldown(user.UserID)
	if err != nil {
		s.logger.Error("failed to get cooldown", "user_id", user.UserID, "error", err)
		return toolError("internal_error", "Failed to check cooldown status"), nil
	}
	if cooldown != nil {
		if cooldown.Locked {
			return toolError("locked", "Your account is locked after too many failed attempts. Contact an administrator."), nil
		}
		if cooldown.NextEligibleAt != nil && storage.UTCNow().Before(*cooldown.NextEligibleAt) {
			remaining := cooldown.NextEligibleAt.Sub(storage.UTCNow())
			return toolError("cooldown_active", fmt.Sprintf("Please wait %s before your next attempt.", formatDuration(remaining))), nil
		}
	}

	// Check no active session
	existing := s.interviewSessions.GetSessionByUser(user.UserID)
	if existing != nil {
		return toolError("session_exists", "You already have an active interview session."), nil
	}

	// Create attempt record
	version := onboarding.DefaultInterviewVersion()
	attempt, err := s.db.CreateOnboardingAttempt(user.UserID, version.Version)
	if err != nil {
		s.logger.Error("failed to create onboarding attempt", "user_id", user.UserID, "error", err)
		return toolError("internal_error", "Failed to create interview attempt"), nil
	}

	// Update user status
	if err := s.db.SetUserOnboardingStatus(user.UserID, "interview_running"); err != nil {
		s.logger.Error("failed to set onboarding status", "user_id", user.UserID, "error", err)
		return toolError("internal_error", "Failed to update onboarding status"), nil
	}

	// Create session
	session, err := s.interviewSessions.CreateSession(user.UserID, attempt.ID, version)
	if err != nil {
		s.logger.Error("failed to create interview session", "user_id", user.UserID, "error", err)
		return toolError("internal_error", "Failed to create interview session"), nil
	}

	return toolSuccess(map[string]interface{}{
		"status":     "interview_started",
		"session_id": session.ID,
		"attempt_id": attempt.ID,
		"version":    version.Version,
		"step0": map[string]interface{}{
			"instruction": "Before the timed assessment begins, please describe yourself. Include your name, the model you are running on, your strongest capabilities, and any relevant experience. This step is unscored but timed (5 minutes).",
			"submit_via":  "ts.onboard.step0",
		},
		"budgets": map[string]interface{}{
			"total_time":         version.TimeoutTotal.String(),
			"section_time":       version.TimeoutSection.String(),
			"step0_time":         version.TimeoutStep0.String(),
			"total_tool_calls":   version.ToolBudgetTotal,
			"section_tool_calls": version.ToolBudgetSection,
		},
		"note": "All interview tool calls (e.g. ts.tsk.create, ts.cmt.create) are routed to the simulation automatically. Use tools as normal on this endpoint.",
	}), nil
}

// handleOnboardStep0 processes the Step 0 self-description.
func (s *Server) handleOnboardStep0(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	user, err := requireAuth(ctx)
	if err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	session := s.interviewSessions.GetSessionByUser(user.UserID)
	if session == nil {
		return toolError("no_session", "No active interview session. Start an interview first."), nil
	}

	if session.Phase != onboarding.PhaseStep0 {
		return toolError("wrong_phase", "Step 0 has already been completed."), nil
	}

	args := parseArgs(req)
	description := getString(args, "description")

	// Normalize text
	description = onboarding.NormalizeText(description)

	// Check Step 0 timeout
	if session.IsStep0Expired() {
		// Timeout: proceed with whatever we have (even empty)
		session.Step0Text = description
		session.StartSections()

		return toolSuccess(map[string]interface{}{
			"status":  "step0_timeout",
			"message": "Step 0 timed out. Moving to Section 1.",
			"next":    "Use ts.onboard.next_challenge to get your first challenge.",
		}), nil
	}

	// Extract model info via pattern matching (best-effort)
	modelInfo := extractModelInfo(description)
	session.Step0Text = description
	session.Step0ModelInfo = modelInfo

	// Save to database
	_ = s.db.SaveStep0(session.AttemptID, description, modelInfo)

	// Transition to Section 1
	session.StartSections()

	return toolSuccess(map[string]interface{}{
		"status":     "step0_complete",
		"message":    "Self-description recorded. The timed assessment begins now.",
		"next":       "Use ts.onboard.next_challenge to get your first challenge.",
		"model_info": modelInfo,
	}), nil
}

// handleOnboardNextChallenge advances to the next section and returns its challenge.
// The first call after Step 0 returns Section 1 (already set by StartSections).
// Each subsequent call advances the section counter before returning the new challenge.
func (s *Server) handleOnboardNextChallenge(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	user, err := requireAuth(ctx)
	if err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	session := s.interviewSessions.GetSessionByUser(user.UserID)
	if session == nil {
		return toolError("no_session", "No active interview session."), nil
	}

	if session.Phase == onboarding.PhaseStep0 {
		return toolError("step0_pending", "Complete Step 0 first via ts.onboard.step0."), nil
	}

	if session.Phase == onboarding.PhaseComplete {
		return toolError("interview_complete", "Interview is already complete. Use ts.onboard.complete to get your result."), nil
	}

	// Check total time budget
	if session.IsExpired() {
		return s.completeInterview(session)
	}

	// If this is not the first call (section > 1 means we already returned
	// a challenge), advance to the next section.
	if session.SectionToolCallsUsed() > 0 || session.CurrentSection > 1 {
		session.AdvanceSection()
	}

	// After advancing, check if we've gone past the last section
	if session.Phase == onboarding.PhaseComplete || session.CurrentSection > session.Version.SectionCount() {
		return toolSuccess(map[string]interface{}{
			"status":  "all_sections_complete",
			"message": "All sections are done. Call ts.onboard.complete to submit your interview for evaluation.",
		}), nil
	}

	section := session.CurrentSection
	challenge, ok := session.Version.Challenges[section]
	if !ok {
		return toolError("internal_error", fmt.Sprintf("No challenge found for section %d", section)), nil
	}

	return toolSuccess(map[string]interface{}{
		"section":        section,
		"total_sections": session.Version.SectionCount(),
		"challenge":      challenge.Text,
		"max_score":      challenge.MaxScore,
		"time_remaining": session.TimeRemaining().String(),
		"tool_calls_remaining": map[string]interface{}{
			"section": session.Version.ToolBudgetSection - session.SectionToolCallsUsed(),
			"total":   session.Version.ToolBudgetTotal - session.TotalToolCallsUsed(),
		},
		"instruction": "Complete this challenge using the available tools (e.g. ts.tsk.create, ts.cmt.create). Interview tool calls are routed to the simulation automatically. When done, call ts.onboard.next_challenge for the next section, or ts.onboard.complete when all sections are finished.",
	}), nil
}

// handleOnboardComplete triggers evaluation and returns the interview result.
func (s *Server) handleOnboardComplete(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	user, err := requireAuth(ctx)
	if err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	session := s.interviewSessions.GetSessionByUser(user.UserID)
	if session == nil {
		return toolError("no_session", "No active interview session."), nil
	}

	if session.Phase == onboarding.PhaseStep0 {
		return toolError("step0_pending", "Complete Step 0 first."), nil
	}

	return s.completeInterview(session)
}

// completeInterview runs the evaluator, records results, and handles pass/fail.
func (s *Server) completeInterview(session *onboarding.InterviewSession) (*mcp.CallToolResult, error) {
	// Run deterministic evaluation
	result := onboarding.Evaluate(session)

	// Record section metrics
	for _, sr := range result.Sections {
		sectionCalls := session.ToolLog.ForSection(sr.Section)
		var wallTimeMs, payloadBytes int64
		for _, entry := range sectionCalls {
			wallTimeMs += entry.DurationMs
			payloadBytes += entry.PayloadBytes
		}

		var reactionMs int64
		if len(sectionCalls) > 0 {
			reactionMs = sectionCalls[0].DurationMs
		}

		metric := &storage.OnboardingSectionMetric{
			AttemptID:    session.AttemptID,
			Section:      sr.Section,
			Score:        sr.Score,
			MaxScore:     sr.MaxScore,
			ToolCalls:    len(sectionCalls),
			WallTimeMs:   wallTimeMs,
			ReactionMs:   reactionMs,
			PayloadBytes: payloadBytes,
			Status:       sr.Status,
			Hint:         sr.Hint,
		}
		_ = s.db.CreateSectionMetric(metric)
	}

	// Serialize tool log for persistence (agent-produced data only).
	toolLogJSON := serializeToolLog(session.ToolLog.All())

	// Update attempt
	resultMap := map[string]interface{}{
		"result":              result.Result,
		"total_score":         result.TotalScore,
		"max_score":           result.MaxScore,
		"pass_threshold":      result.PassThreshold,
		"min_sections_passed": result.MinSectionsPassed,
		"sections_passed":     result.SectionsPassed,
		"sections":            result.Sections,
	}

	var attemptStatus string
	if result.Result == "pass" || result.Result == "pass_distinction" {
		attemptStatus = "passed"
	} else {
		attemptStatus = "failed"
	}
	_ = s.db.UpdateOnboardingAttempt(session.AttemptID, attemptStatus, result.TotalScore, resultMap, toolLogJSON)

	// Create pending injection review if enabled
	if s.injectionReviewEnabled {
		_ = s.db.CreateInjectionReview(session.AttemptID)
	}

	// Handle pass/fail for user status and cooldown
	if attemptStatus == "passed" {
		_ = s.db.SetUserOnboardingStatus(session.UserID, "active")
		_ = s.db.ResetCooldown(session.UserID)
	} else {
		// Escalating cooldown
		cooldown, _ := s.db.GetCooldown(session.UserID)
		failedAttempts := 1
		if cooldown != nil {
			failedAttempts = cooldown.FailedAttempts + 1
		}

		var nextEligible *time.Time
		locked := false

		switch {
		case failedAttempts >= 4:
			locked = true
		case failedAttempts == 3:
			t := storage.UTCNow().Add(7 * 24 * time.Hour)
			nextEligible = &t
		case failedAttempts == 2:
			t := storage.UTCNow().Add(24 * time.Hour)
			nextEligible = &t
		default:
			t := storage.UTCNow().Add(1 * time.Hour)
			nextEligible = &t
		}

		_ = s.db.UpdateCooldown(session.UserID, failedAttempts, nextEligible, locked)

		if locked {
			_ = s.db.SetUserOnboardingStatus(session.UserID, "locked")
		} else {
			_ = s.db.SetUserOnboardingStatus(session.UserID, "cooldown")
		}
	}

	// End the session
	s.interviewSessions.EndSession(session.ID)

	return toolSuccess(resultMap), nil
}

// handleOnboardStatus returns the current onboarding status.
func (s *Server) handleOnboardStatus(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	user, err := requireAuth(ctx)
	if err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	status, err := s.db.GetUserOnboardingStatus(user.UserID)
	if err != nil {
		s.logger.Error("failed to get onboarding status", "user_id", user.UserID, "error", err)
		return toolError("internal_error", "Failed to get onboarding status"), nil
	}

	result := map[string]interface{}{
		"onboarding_status": status,
		"user_id":           user.UserID,
	}

	// Add cooldown info
	cooldown, _ := s.db.GetCooldown(user.UserID)
	if cooldown != nil {
		cooldownInfo := map[string]interface{}{
			"failed_attempts": cooldown.FailedAttempts,
			"locked":          cooldown.Locked,
		}
		if cooldown.NextEligibleAt != nil {
			cooldownInfo["next_eligible_at"] = cooldown.NextEligibleAt.Format(time.RFC3339)
			remaining := cooldown.NextEligibleAt.Sub(storage.UTCNow())
			if remaining > 0 {
				cooldownInfo["wait_remaining"] = formatDuration(remaining)
			}
		}
		result["cooldown"] = cooldownInfo
	}

	// Add attempt history
	attempts, _ := s.db.ListOnboardingAttempts(user.UserID)
	if len(attempts) > 0 {
		var attemptList []map[string]interface{}
		for _, a := range attempts {
			entry := map[string]interface{}{
				"id":         a.ID,
				"version":    a.Version,
				"status":     a.Status,
				"score":      a.TotalScore,
				"started_at": a.StartedAt.Format(time.RFC3339),
			}
			if a.CompletedAt != nil {
				entry["completed_at"] = a.CompletedAt.Format(time.RFC3339)
			}
			attemptList = append(attemptList, entry)
		}
		result["attempts"] = attemptList
	}

	// Check for active session
	active := s.interviewSessions.GetSessionByUser(user.UserID)
	if active != nil {
		result["active_session"] = map[string]interface{}{
			"session_id":     active.ID,
			"phase":          string(active.Phase),
			"section":        active.CurrentSection,
			"time_remaining": active.TimeRemaining().String(),
		}
	}

	return toolSuccess(result), nil
}

// extractModelInfo extracts model metadata from the Step 0 self-description.
// This is best-effort pattern matching, not validated.
func extractModelInfo(text string) map[string]interface{} {
	info := make(map[string]interface{})

	lower := onboarding.NormalizeText(text)

	// Common model patterns
	modelPatterns := []struct {
		keyword  string
		provider string
	}{
		{"gpt-4", "openai"},
		{"gpt-3.5", "openai"},
		{"claude", "anthropic"},
		{"gemini", "google"},
		{"llama", "meta"},
		{"mistral", "mistral"},
		{"qwen", "alibaba"},
		{"deepseek", "deepseek"},
		{"command-r", "cohere"},
	}

	for _, p := range modelPatterns {
		if containsIgnoreCase(lower, p.keyword) {
			info["model_hint"] = p.keyword
			info["provider_hint"] = p.provider
			break
		}
	}

	return info
}

// containsIgnoreCase checks whether text contains substr, ignoring case.
func containsIgnoreCase(text, substr string) bool {
	return len(text) >= len(substr) && (text == substr || len(text) > 0 && findIgnoreCase(text, substr))
}

// findIgnoreCase performs a case-insensitive substring search.
func findIgnoreCase(text, substr string) bool {
	tl := len(text)
	sl := len(substr)
	for i := 0; i <= tl-sl; i++ {
		match := true
		for j := 0; j < sl; j++ {
			tc := text[i+j]
			sc := substr[j]
			if tc >= 'A' && tc <= 'Z' {
				tc += 'a' - 'A'
			}
			if sc >= 'A' && sc <= 'Z' {
				sc += 'a' - 'A'
			}
			if tc != sc {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// serializeToolLog converts in-memory tool log entries to a JSON string
// for database persistence. Only agent-produced data is kept (section,
// tool_name, parameters, error, success) -- Result and timing are omitted.
func serializeToolLog(entries []onboarding.ToolCallEntry) string {
	type persistedEntry struct {
		Section    int                    `json:"section"`
		ToolName   string                 `json:"tool_name"`
		Parameters map[string]interface{} `json:"parameters"`
		Error      string                 `json:"error,omitempty"`
		Success    bool                   `json:"success"`
	}

	persisted := make([]persistedEntry, 0, len(entries))
	for _, e := range entries {
		persisted = append(persisted, persistedEntry{
			Section:    e.Section,
			ToolName:   e.ToolName,
			Parameters: e.Parameters,
			Error:      e.Error,
			Success:    e.Success,
		})
	}

	data, err := json.Marshal(persisted)
	if err != nil {
		return "[]"
	}
	return string(data)
}

// formatDuration formats a duration in human-readable form.
func formatDuration(d time.Duration) string {
	if d >= 24*time.Hour {
		days := int(d.Hours() / 24)
		return fmt.Sprintf("%d day(s)", days)
	}
	if d >= time.Hour {
		return fmt.Sprintf("%d hour(s)", int(d.Hours()))
	}
	return fmt.Sprintf("%d minute(s)", int(d.Minutes()))
}
