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
	"fmt"
	"time"
)

// AgentSessionMetric tracks per-session tool call success/error counts
// for agent behavioral monitoring (Taskgovernor).
type AgentSessionMetric struct {
	ID           string
	UserID       string
	SessionID    string
	StartedAt    time.Time
	EndedAt      *time.Time
	ToolCalls    int
	Successful   int
	ClientErrors int
	AuthErrors   int
	ServerErrors int
	SuccessRate  *float64
	Metadata     string
	CreatedAt    time.Time
}

// CreateSessionMetric creates a new session metric row for an agent session.
func (db *DB) CreateSessionMetric(userID, sessionID string) (*AgentSessionMetric, error) {
	id := generateID("asm")
	now := UTCNow()
	nowStr := now.Format(time.RFC3339)
	_, err := db.Exec(
		`INSERT INTO agent_session_metric (id, user_id, session_id, started_at, created_at)
		 VALUES (?, ?, ?, ?, ?)`,
		id, userID, sessionID, nowStr, nowStr,
	)
	if err != nil {
		return nil, fmt.Errorf("create session metric: %w", err)
	}
	return &AgentSessionMetric{
		ID:        id,
		UserID:    userID,
		SessionID: sessionID,
		StartedAt: now,
		Metadata:  "{}",
		CreatedAt: now,
	}, nil
}

// GetSessionMetricBySession returns the metric row for an MCP session ID.
func (db *DB) GetSessionMetricBySession(sessionID string) (*AgentSessionMetric, error) {
	row := db.QueryRow(
		`SELECT id, user_id, session_id, started_at, ended_at,
		        tool_calls, successful, client_errors, auth_errors, server_errors,
		        success_rate, metadata, created_at
		 FROM agent_session_metric WHERE session_id = ?`,
		sessionID,
	)
	return scanSessionMetric(row)
}

// IncrementMetricSuccess atomically increments the successful counter.
func (db *DB) IncrementMetricSuccess(id string) error {
	return db.incrementMetric(id, "successful")
}

// IncrementMetricClientError atomically increments the client_errors counter.
func (db *DB) IncrementMetricClientError(id string) error {
	return db.incrementMetric(id, "client_errors")
}

// IncrementMetricAuthError atomically increments the auth_errors counter.
func (db *DB) IncrementMetricAuthError(id string) error {
	return db.incrementMetric(id, "auth_errors")
}

// IncrementMetricServerError atomically increments the server_errors counter.
func (db *DB) IncrementMetricServerError(id string) error {
	return db.incrementMetric(id, "server_errors")
}

func (db *DB) incrementMetric(id, column string) error {
	query := fmt.Sprintf(
		`UPDATE agent_session_metric
		 SET %s = %s + 1,
		     tool_calls = tool_calls + 1,
		     success_rate = CAST(successful AS REAL) / NULLIF(successful + client_errors, 0)
		 WHERE id = ?`, column, column,
	)
	// For "successful" and "client_errors", use specialized queries that account
	// for the incremented value in the rate computation.
	switch column {
	case "successful":
		query = `UPDATE agent_session_metric
		         SET successful = successful + 1,
		             tool_calls = tool_calls + 1,
		             success_rate = CAST(successful + 1 AS REAL) / NULLIF(successful + 1 + client_errors, 0)
		         WHERE id = ?`
	case "client_errors":
		query = `UPDATE agent_session_metric
		         SET client_errors = client_errors + 1,
		             tool_calls = tool_calls + 1,
		             success_rate = CAST(successful AS REAL) / NULLIF(successful + client_errors + 1, 0)
		         WHERE id = ?`
	}
	_, err := db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("increment metric %s: %w", column, err)
	}
	return nil
}

// EndSessionMetric marks a session metric as ended.
func (db *DB) EndSessionMetric(id string) error {
	nowStr := UTCNow().Format(time.RFC3339)
	_, err := db.Exec(
		`UPDATE agent_session_metric SET ended_at = ? WHERE id = ?`,
		nowStr, id,
	)
	if err != nil {
		return fmt.Errorf("end session metric: %w", err)
	}
	return nil
}

// AgentRollingRate computes the aggregate success rate for an agent since a given time.
// Returns the rate (0.0-1.0), total rated calls (successful + client_errors), and error.
func (db *DB) AgentRollingRate(userID string, since time.Time) (float64, int, error) {
	sinceStr := since.Format(time.RFC3339)
	var successful, clientErrors int
	err := db.QueryRow(
		`SELECT COALESCE(SUM(successful), 0), COALESCE(SUM(client_errors), 0)
		 FROM agent_session_metric
		 WHERE user_id = ? AND started_at >= ?`,
		userID, sinceStr,
	).Scan(&successful, &clientErrors)
	if err != nil {
		return 0, 0, fmt.Errorf("rolling rate query: %w", err)
	}
	total := successful + clientErrors
	if total == 0 {
		return 0, 0, nil
	}
	return float64(successful) / float64(total), total, nil
}

// AgentCurrentSessionRate returns the success rate and call count for the most
// recent active (not ended) session for an agent.
func (db *DB) AgentCurrentSessionRate(userID string) (float64, int, error) {
	var successRate sql.NullFloat64
	var toolCalls int
	err := db.QueryRow(
		`SELECT success_rate, tool_calls
		 FROM agent_session_metric
		 WHERE user_id = ? AND ended_at IS NULL
		 ORDER BY started_at DESC LIMIT 1`,
		userID,
	).Scan(&successRate, &toolCalls)
	if err == sql.ErrNoRows {
		return 0, 0, nil
	}
	if err != nil {
		return 0, 0, fmt.Errorf("current session rate: %w", err)
	}
	if !successRate.Valid {
		return 0, toolCalls, nil
	}
	return successRate.Float64, toolCalls, nil
}

func scanSessionMetric(row *sql.Row) (*AgentSessionMetric, error) {
	var m AgentSessionMetric
	var startedAt, createdAt string
	var endedAt sql.NullString
	var successRate sql.NullFloat64
	err := row.Scan(
		&m.ID, &m.UserID, &m.SessionID, &startedAt, &endedAt,
		&m.ToolCalls, &m.Successful, &m.ClientErrors, &m.AuthErrors, &m.ServerErrors,
		&successRate, &m.Metadata, &createdAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan session metric: %w", err)
	}
	m.StartedAt = ParseDBTime(startedAt)
	m.CreatedAt = ParseDBTime(createdAt)
	if endedAt.Valid {
		t := ParseDBTime(endedAt.String)
		m.EndedAt = &t
	}
	if successRate.Valid {
		m.SuccessRate = &successRate.Float64
	}
	return &m, nil
}
