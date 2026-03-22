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

// AgentHealthSnapshot stores precomputed health metrics for an agent user.
// Computed periodically by the taskgovernor ticker.
type AgentHealthSnapshot struct {
	ID              string
	UserID          string
	SessionRate     *float64
	SessionCalls    int
	Rolling24hRate  *float64
	Rolling24hCalls int
	Rolling7dRate   *float64
	Rolling7dCalls  int
	Status          string // healthy, warned, flagged, suspended
	LastCheckedAt   time.Time
	Metadata        string
	CreatedAt       time.Time
}

// UpsertAgentHealthSnapshot creates or updates the health snapshot for an agent.
func (db *DB) UpsertAgentHealthSnapshot(snap *AgentHealthSnapshot) error {
	nowStr := snap.LastCheckedAt.Format(time.RFC3339)
	_, err := db.Exec(
		`INSERT INTO agent_health_snapshot
		 (id, user_id, session_rate, session_calls, rolling_24h_rate, rolling_24h_calls,
		  rolling_7d_rate, rolling_7d_calls, status, last_checked_at, metadata, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(user_id) DO UPDATE SET
		   session_rate = excluded.session_rate,
		   session_calls = excluded.session_calls,
		   rolling_24h_rate = excluded.rolling_24h_rate,
		   rolling_24h_calls = excluded.rolling_24h_calls,
		   rolling_7d_rate = excluded.rolling_7d_rate,
		   rolling_7d_calls = excluded.rolling_7d_calls,
		   status = excluded.status,
		   last_checked_at = excluded.last_checked_at,
		   metadata = excluded.metadata`,
		snap.ID, snap.UserID,
		snap.SessionRate, snap.SessionCalls,
		snap.Rolling24hRate, snap.Rolling24hCalls,
		snap.Rolling7dRate, snap.Rolling7dCalls,
		snap.Status, nowStr, snap.Metadata, nowStr,
	)
	if err != nil {
		return fmt.Errorf("upsert agent health snapshot: %w", err)
	}
	return nil
}

// GetAgentHealthSnapshot returns the health snapshot for a user.
func (db *DB) GetAgentHealthSnapshot(userID string) (*AgentHealthSnapshot, error) {
	row := db.QueryRow(
		`SELECT id, user_id, session_rate, session_calls,
		        rolling_24h_rate, rolling_24h_calls,
		        rolling_7d_rate, rolling_7d_calls,
		        status, last_checked_at, metadata, created_at
		 FROM agent_health_snapshot WHERE user_id = ?`,
		userID,
	)
	return scanHealthSnapshot(row)
}

// ListAgentHealthSnapshots returns snapshots, optionally filtered by status.
// Pass empty string for statusFilter to return all.
func (db *DB) ListAgentHealthSnapshots(statusFilter string) ([]*AgentHealthSnapshot, error) {
	var rows *sql.Rows
	var err error
	if statusFilter != "" {
		rows, err = db.Query(
			`SELECT id, user_id, session_rate, session_calls,
			        rolling_24h_rate, rolling_24h_calls,
			        rolling_7d_rate, rolling_7d_calls,
			        status, last_checked_at, metadata, created_at
			 FROM agent_health_snapshot WHERE status = ?
			 ORDER BY last_checked_at DESC`,
			statusFilter,
		)
	} else {
		rows, err = db.Query(
			`SELECT id, user_id, session_rate, session_calls,
			        rolling_24h_rate, rolling_24h_calls,
			        rolling_7d_rate, rolling_7d_calls,
			        status, last_checked_at, metadata, created_at
			 FROM agent_health_snapshot
			 ORDER BY last_checked_at DESC`,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("list agent health snapshots: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []*AgentHealthSnapshot
	for rows.Next() {
		var s AgentHealthSnapshot
		var sessionRate, r24hRate, r7dRate sql.NullFloat64
		var lastCheckedAt, createdAt string
		err := rows.Scan(
			&s.ID, &s.UserID, &sessionRate, &s.SessionCalls,
			&r24hRate, &s.Rolling24hCalls,
			&r7dRate, &s.Rolling7dCalls,
			&s.Status, &lastCheckedAt, &s.Metadata, &createdAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan health snapshot row: %w", err)
		}
		s.LastCheckedAt = ParseDBTime(lastCheckedAt)
		s.CreatedAt = ParseDBTime(createdAt)
		if sessionRate.Valid {
			s.SessionRate = &sessionRate.Float64
		}
		if r24hRate.Valid {
			s.Rolling24hRate = &r24hRate.Float64
		}
		if r7dRate.Valid {
			s.Rolling7dRate = &r7dRate.Float64
		}
		result = append(result, &s)
	}
	return result, nil
}

// ListActiveAgentUserIDs returns user IDs of agents with onboarding_status = 'active'.
func (db *DB) ListActiveAgentUserIDs() ([]string, error) {
	rows, err := db.Query(
		`SELECT id FROM user WHERE user_type = 'agent' AND onboarding_status = 'active'`,
	)
	if err != nil {
		return nil, fmt.Errorf("list active agent user IDs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan agent user ID: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// AgentsByOrganization returns a map of org ID -> list of agent user IDs.
// Only includes active agents that belong to at least one organization.
func (db *DB) AgentsByOrganization() (map[string][]string, error) {
	rows, err := db.Query(
		`SELECT er.source_entity_id, u.id
		 FROM user u
		 JOIN resource r ON u.resource_id = r.id
		 JOIN entity_relation er ON er.target_entity_id = r.id
		   AND er.target_entity_type = 'resource'
		   AND er.source_entity_type = 'organization'
		   AND er.relationship_type = 'has_member'
		 WHERE u.user_type = 'agent' AND u.onboarding_status = 'active'`,
	)
	if err != nil {
		return nil, fmt.Errorf("agents by organization: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := make(map[string][]string)
	for rows.Next() {
		var orgID, userID string
		if err := rows.Scan(&orgID, &userID); err != nil {
			return nil, fmt.Errorf("scan agent org row: %w", err)
		}
		result[orgID] = append(result[orgID], userID)
	}
	return result, nil
}

// CountRecentSuspensions counts agents suspended in the given time window.
// If orgAgentIDs is non-nil, only counts those agents.
func (db *DB) CountRecentSuspensions(since time.Time, orgAgentIDs []string) (int, error) {
	sinceStr := since.Format(time.RFC3339)
	if len(orgAgentIDs) == 0 {
		var count int
		err := db.QueryRow(
			`SELECT COUNT(*) FROM agent_health_snapshot
			 WHERE status = 'suspended' AND last_checked_at >= ?`,
			sinceStr,
		).Scan(&count)
		return count, err
	}

	// Build placeholders for IN clause
	query := `SELECT COUNT(*) FROM agent_health_snapshot
	          WHERE status = 'suspended' AND last_checked_at >= ? AND user_id IN (`
	args := []interface{}{sinceStr}
	for i, id := range orgAgentIDs {
		if i > 0 {
			query += ","
		}
		query += "?"
		args = append(args, id)
	}
	query += ")"
	var count int
	err := db.QueryRow(query, args...).Scan(&count)
	return count, err
}

func scanHealthSnapshot(row *sql.Row) (*AgentHealthSnapshot, error) {
	var s AgentHealthSnapshot
	var sessionRate, r24hRate, r7dRate sql.NullFloat64
	var lastCheckedAt, createdAt string
	err := row.Scan(
		&s.ID, &s.UserID, &sessionRate, &s.SessionCalls,
		&r24hRate, &s.Rolling24hCalls,
		&r7dRate, &s.Rolling7dCalls,
		&s.Status, &lastCheckedAt, &s.Metadata, &createdAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan health snapshot: %w", err)
	}
	s.LastCheckedAt = ParseDBTime(lastCheckedAt)
	s.CreatedAt = ParseDBTime(createdAt)
	if sessionRate.Valid {
		s.SessionRate = &sessionRate.Float64
	}
	if r24hRate.Valid {
		s.Rolling24hRate = &r24hRate.Float64
	}
	if r7dRate.Valid {
		s.Rolling7dRate = &r7dRate.Float64
	}
	return &s, nil
}
