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


package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// OnboardingAttempt represents a single interview attempt.
type OnboardingAttempt struct {
	ID          string
	UserID      string
	Version     int
	Status      string // running, passed, failed, terminated
	StartedAt   time.Time
	CompletedAt *time.Time
	TotalScore  int
	Result      map[string]interface{} // JSON: detailed result breakdown
	ToolLog     string                 // JSON: serialized tool call log
	CreatedAt   time.Time
}

// OnboardingSectionMetric holds per-section performance data.
type OnboardingSectionMetric struct {
	ID           string
	AttemptID    string
	Section      int
	Score        int
	MaxScore     int
	ToolCalls    int
	WallTimeMs   int64
	ReactionMs   int64 // time from challenge to first tool call
	PayloadBytes int64
	Status       string // passed, failed, skipped, terminated
	Hint         string // feedback hint for failed sections
}

// OnboardingCooldown tracks escalating cooldown state for a user.
type OnboardingCooldown struct {
	UserID         string
	FailedAttempts int
	NextEligibleAt *time.Time
	Locked         bool
	UpdatedAt      time.Time
}

// OnboardingStep0 holds the unscored self-description data.
type OnboardingStep0 struct {
	ID          string
	AttemptID   string
	RawText     string
	ModelInfo   map[string]interface{} // extracted model metadata
	StartedAt   time.Time
	CompletedAt *time.Time
}

// CreateOnboardingAttempt inserts a new interview attempt.
func (db *DB) CreateOnboardingAttempt(userID string, version int) (*OnboardingAttempt, error) {
	id := generateID("oba")
	now := UTCNow().Format(time.RFC3339)

	_, err := db.Exec(
		`INSERT INTO onboarding_attempt (id, user_id, version, status, started_at, total_score, result, created_at)
		 VALUES (?, ?, ?, 'running', ?, 0, '{}', ?)`,
		id, userID, version, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("create onboarding attempt: %w", err)
	}

	return &OnboardingAttempt{
		ID:        id,
		UserID:    userID,
		Version:   version,
		Status:    "running",
		StartedAt: ParseDBTime(now),
		CreatedAt: ParseDBTime(now),
		Result:    map[string]interface{}{},
	}, nil
}

// GetOnboardingAttempt retrieves an attempt by ID.
func (db *DB) GetOnboardingAttempt(id string) (*OnboardingAttempt, error) {
	var a OnboardingAttempt
	var completedAt sql.NullString
	var resultJSON, startedAt, createdAt string
	var toolLog sql.NullString

	err := db.QueryRow(
		`SELECT id, user_id, version, status, started_at, completed_at, total_score, result, COALESCE(tool_log, '[]'), created_at
		 FROM onboarding_attempt WHERE id = ?`, id,
	).Scan(&a.ID, &a.UserID, &a.Version, &a.Status, &startedAt, &completedAt, &a.TotalScore, &resultJSON, &toolLog, &createdAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get onboarding attempt: %w", err)
	}

	a.StartedAt = ParseDBTime(startedAt)
	a.CreatedAt = ParseDBTime(createdAt)
	if completedAt.Valid {
		t := ParseDBTime(completedAt.String)
		a.CompletedAt = &t
	}
	if resultJSON != "" {
		_ = json.Unmarshal([]byte(resultJSON), &a.Result)
	}
	if toolLog.Valid {
		a.ToolLog = toolLog.String
	} else {
		a.ToolLog = "[]"
	}

	return &a, nil
}

// UpdateOnboardingAttempt updates an attempt's status, score, result, and tool log.
func (db *DB) UpdateOnboardingAttempt(id, status string, totalScore int, result map[string]interface{}, toolLogJSON string) error {
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal result: %w", err)
	}

	var completedAt *string
	if status == "passed" || status == "failed" || status == "terminated" {
		now := UTCNow().Format(time.RFC3339)
		completedAt = &now
	}

	if toolLogJSON == "" {
		toolLogJSON = "[]"
	}

	_, err = db.Exec(
		`UPDATE onboarding_attempt SET status = ?, total_score = ?, result = ?, tool_log = ?, completed_at = ? WHERE id = ?`,
		status, totalScore, string(resultJSON), toolLogJSON, completedAt, id,
	)
	if err != nil {
		return fmt.Errorf("update onboarding attempt: %w", err)
	}
	return nil
}

// ListOnboardingAttempts returns all attempts for a user, newest first.
func (db *DB) ListOnboardingAttempts(userID string) ([]*OnboardingAttempt, error) {
	rows, err := db.Query(
		`SELECT id, user_id, version, status, started_at, completed_at, total_score, result, COALESCE(tool_log, '[]'), created_at
		 FROM onboarding_attempt WHERE user_id = ? ORDER BY created_at DESC`, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list onboarding attempts: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var attempts []*OnboardingAttempt
	for rows.Next() {
		var a OnboardingAttempt
		var completedAt, toolLog sql.NullString
		var resultJSON, startedAt, createdAt string

		if err := rows.Scan(&a.ID, &a.UserID, &a.Version, &a.Status, &startedAt, &completedAt, &a.TotalScore, &resultJSON, &toolLog, &createdAt); err != nil {
			return nil, fmt.Errorf("scan onboarding attempt: %w", err)
		}

		a.StartedAt = ParseDBTime(startedAt)
		a.CreatedAt = ParseDBTime(createdAt)
		if completedAt.Valid {
			t := ParseDBTime(completedAt.String)
			a.CompletedAt = &t
		}
		if resultJSON != "" {
			_ = json.Unmarshal([]byte(resultJSON), &a.Result)
		}
		if toolLog.Valid {
			a.ToolLog = toolLog.String
		} else {
			a.ToolLog = "[]"
		}
		attempts = append(attempts, &a)
	}
	return attempts, nil
}

// CreateSectionMetric inserts a per-section performance record.
func (db *DB) CreateSectionMetric(m *OnboardingSectionMetric) error {
	m.ID = generateID("obm")
	_, err := db.Exec(
		`INSERT INTO onboarding_section_metric (id, attempt_id, section, score, max_score, tool_calls, wall_time_ms, reaction_ms, payload_bytes, status, hint)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		m.ID, m.AttemptID, m.Section, m.Score, m.MaxScore, m.ToolCalls, m.WallTimeMs, m.ReactionMs, m.PayloadBytes, m.Status, m.Hint,
	)
	if err != nil {
		return fmt.Errorf("create section metric: %w", err)
	}
	return nil
}

// GetSectionMetrics returns all section metrics for an attempt, ordered by section.
func (db *DB) GetSectionMetrics(attemptID string) ([]*OnboardingSectionMetric, error) {
	rows, err := db.Query(
		`SELECT id, attempt_id, section, score, max_score, tool_calls, wall_time_ms, reaction_ms, payload_bytes, status, hint
		 FROM onboarding_section_metric WHERE attempt_id = ? ORDER BY section`, attemptID,
	)
	if err != nil {
		return nil, fmt.Errorf("get section metrics: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var metrics []*OnboardingSectionMetric
	for rows.Next() {
		var m OnboardingSectionMetric
		if err := rows.Scan(&m.ID, &m.AttemptID, &m.Section, &m.Score, &m.MaxScore, &m.ToolCalls, &m.WallTimeMs, &m.ReactionMs, &m.PayloadBytes, &m.Status, &m.Hint); err != nil {
			return nil, fmt.Errorf("scan section metric: %w", err)
		}
		metrics = append(metrics, &m)
	}
	return metrics, nil
}

// GetCooldown retrieves the cooldown record for a user.
func (db *DB) GetCooldown(userID string) (*OnboardingCooldown, error) {
	var c OnboardingCooldown
	var nextEligible sql.NullString
	var updatedAt string
	var locked int

	err := db.QueryRow(
		`SELECT user_id, failed_attempts, next_eligible_at, locked, updated_at
		 FROM onboarding_cooldown WHERE user_id = ?`, userID,
	).Scan(&c.UserID, &c.FailedAttempts, &nextEligible, &locked, &updatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get cooldown: %w", err)
	}

	c.Locked = locked != 0
	c.UpdatedAt = ParseDBTime(updatedAt)
	if nextEligible.Valid {
		t := ParseDBTime(nextEligible.String)
		c.NextEligibleAt = &t
	}
	return &c, nil
}

// UpdateCooldown creates or updates the cooldown record for a user.
func (db *DB) UpdateCooldown(userID string, failedAttempts int, nextEligibleAt *time.Time, locked bool) error {
	now := UTCNow().Format(time.RFC3339)
	var eligibleStr *string
	if nextEligibleAt != nil {
		s := nextEligibleAt.UTC().Format(time.RFC3339)
		eligibleStr = &s
	}

	lockedInt := 0
	if locked {
		lockedInt = 1
	}

	_, err := db.Exec(
		`INSERT INTO onboarding_cooldown (user_id, failed_attempts, next_eligible_at, locked, updated_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(user_id) DO UPDATE SET
		   failed_attempts = excluded.failed_attempts,
		   next_eligible_at = excluded.next_eligible_at,
		   locked = excluded.locked,
		   updated_at = excluded.updated_at`,
		userID, failedAttempts, eligibleStr, lockedInt, now,
	)
	if err != nil {
		return fmt.Errorf("update cooldown: %w", err)
	}
	return nil
}

// ResetCooldown clears the cooldown record for a user (on successful interview).
func (db *DB) ResetCooldown(userID string) error {
	now := UTCNow().Format(time.RFC3339)
	_, err := db.Exec(
		`UPDATE onboarding_cooldown SET failed_attempts = 0, next_eligible_at = NULL, locked = 0, updated_at = ? WHERE user_id = ?`,
		now, userID,
	)
	if err != nil {
		return fmt.Errorf("reset cooldown: %w", err)
	}
	return nil
}

// SaveStep0 saves the Step 0 self-description data.
func (db *DB) SaveStep0(attemptID, rawText string, modelInfo map[string]interface{}) error {
	id := generateID("obs")
	now := UTCNow().Format(time.RFC3339)

	modelJSON, err := json.Marshal(modelInfo)
	if err != nil {
		return fmt.Errorf("marshal model info: %w", err)
	}

	_, err = db.Exec(
		`INSERT INTO onboarding_step0 (id, attempt_id, raw_text, model_info, started_at, completed_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		id, attemptID, rawText, string(modelJSON), now, now,
	)
	if err != nil {
		return fmt.Errorf("save step0: %w", err)
	}
	return nil
}

// GetStep0 retrieves the Step 0 data for an attempt.
func (db *DB) GetStep0(attemptID string) (*OnboardingStep0, error) {
	var s OnboardingStep0
	var completedAt sql.NullString
	var modelJSON, startedAt string

	err := db.QueryRow(
		`SELECT id, attempt_id, raw_text, model_info, started_at, completed_at
		 FROM onboarding_step0 WHERE attempt_id = ?`, attemptID,
	).Scan(&s.ID, &s.AttemptID, &s.RawText, &modelJSON, &startedAt, &completedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get step0: %w", err)
	}

	s.StartedAt = ParseDBTime(startedAt)
	if completedAt.Valid {
		t := ParseDBTime(completedAt.String)
		s.CompletedAt = &t
	}
	if modelJSON != "" {
		_ = json.Unmarshal([]byte(modelJSON), &s.ModelInfo)
	}
	return &s, nil
}

// GetUserOnboardingStatus returns the onboarding_status for a user.
// Returns "active" if the column doesn't exist (pre-migration) or if the value is empty.
func (db *DB) GetUserOnboardingStatus(userID string) (string, error) {
	var status string
	err := db.QueryRow(`SELECT COALESCE(onboarding_status, 'active') FROM user WHERE id = ?`, userID).Scan(&status)
	if err == sql.ErrNoRows {
		return "", ErrUserNotFound
	}
	if err != nil {
		return "", fmt.Errorf("get onboarding status: %w", err)
	}
	return status, nil
}

// SetUserOnboardingStatus updates the onboarding_status for a user.
func (db *DB) SetUserOnboardingStatus(userID, status string) error {
	result, err := db.Exec(`UPDATE user SET onboarding_status = ? WHERE id = ?`, status, userID)
	if err != nil {
		return fmt.Errorf("set onboarding status: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrUserNotFound
	}
	return nil
}

// TerminateRunningInterviews marks any running interview attempts as terminated.
// Called on server startup to handle interviews interrupted by server restart.
// These terminated attempts are not counted against cooldown.
func (db *DB) TerminateRunningInterviews() (int64, error) {
	now := UTCNow().Format(time.RFC3339)
	result, err := db.Exec(
		`UPDATE onboarding_attempt SET status = 'terminated', completed_at = ?,
		 result = json_set(COALESCE(result, '{}'), '$.termination_reason', 'server_restart')
		 WHERE status = 'running'`, now,
	)
	if err != nil {
		return 0, fmt.Errorf("terminate running interviews: %w", err)
	}
	return result.RowsAffected()
}

// CompatibilitySectionScore holds a single section's score for docs generation.
type CompatibilitySectionScore struct {
	Section  int
	Score    int
	MaxScore int
	Status   string
}

// CompatibilityRow is a flat row for docs generation, joining attempt + step0 + metrics.
type CompatibilityRow struct {
	AttemptID     string
	Status        string                 // passed, failed
	TotalScore    int
	ModelInfo     map[string]interface{} // from step0.model_info JSON
	RawText       string                 // step0 self-description
	SectionScores []CompatibilitySectionScore
}

// ListCompletedAttempts returns all completed (passed/failed) attempts with model info
// and per-section scores. Used by the compatibility API endpoint.
func (db *DB) ListCompletedAttempts() ([]*CompatibilityRow, error) {
	// Query attempts with step0 data
	rows, err := db.Query(
		`SELECT a.id, a.status, a.total_score,
		        COALESCE(s.model_info, '{}'), COALESCE(s.raw_text, '')
		 FROM onboarding_attempt a
		 LEFT JOIN onboarding_step0 s ON s.attempt_id = a.id
		 WHERE a.status IN ('passed', 'failed')
		 ORDER BY a.total_score DESC, a.created_at ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list completed attempts: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []*CompatibilityRow
	for rows.Next() {
		var r CompatibilityRow
		var modelJSON string

		if err := rows.Scan(&r.AttemptID, &r.Status, &r.TotalScore, &modelJSON, &r.RawText); err != nil {
			return nil, fmt.Errorf("scan completed attempt: %w", err)
		}

		r.ModelInfo = make(map[string]interface{})
		if modelJSON != "" && modelJSON != "{}" {
			_ = json.Unmarshal([]byte(modelJSON), &r.ModelInfo)
		}

		result = append(result, &r)
	}

	// Load section scores for each attempt
	for _, r := range result {
		metrics, err := db.GetSectionMetrics(r.AttemptID)
		if err != nil {
			return nil, fmt.Errorf("get section metrics for %s: %w", r.AttemptID, err)
		}
		for _, m := range metrics {
			r.SectionScores = append(r.SectionScores, CompatibilitySectionScore{
				Section:  m.Section,
				Score:    m.Score,
				MaxScore: m.MaxScore,
				Status:   m.Status,
			})
		}
	}

	return result, nil
}

// GetActiveAttempt returns the currently running attempt for a user, or nil.
func (db *DB) GetActiveAttempt(userID string) (*OnboardingAttempt, error) {
	var a OnboardingAttempt
	var startedAt, createdAt, resultJSON string

	err := db.QueryRow(
		`SELECT id, user_id, version, status, started_at, total_score, result, created_at
		 FROM onboarding_attempt WHERE user_id = ? AND status = 'running'`, userID,
	).Scan(&a.ID, &a.UserID, &a.Version, &a.Status, &startedAt, &a.TotalScore, &resultJSON, &createdAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get active attempt: %w", err)
	}

	a.StartedAt = ParseDBTime(startedAt)
	a.CreatedAt = ParseDBTime(createdAt)
	if resultJSON != "" {
		_ = json.Unmarshal([]byte(resultJSON), &a.Result)
	}
	return &a, nil
}
