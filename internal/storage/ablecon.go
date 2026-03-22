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
	"strings"
	"time"
)

// AbleconLevel stores the precomputed traffic light level for a scope.
// Uses DEFCON-style numbering: lower = more severe.
type AbleconLevel struct {
	ID        string
	Scope     string                 // "system" or "organization"
	ScopeID   string                 // org ID or "" for system
	Level     int                    // 1=Red, 2=Orange, 3=Green, 4=Blue
	Reason    map[string]interface{} // JSON explanation of current level
	UpdatedAt time.Time
	CreatedAt time.Time
}

// AbleconLevelLabel returns the color name for a DEFCON-style level.
func AbleconLevelLabel(level int) string {
	switch level {
	case 4:
		return "blue"
	case 3:
		return "green"
	case 2:
		return "orange"
	case 1:
		return "red"
	default:
		return "unknown"
	}
}

// GetUserAbleconLevel computes the Ablecon level for a set of agent user IDs
// by querying their health snapshots. Returns nil if no agents are found.
func (db *DB) GetUserAbleconLevel(agentUserIDs []string) (*AbleconLevel, error) {
	if len(agentUserIDs) == 0 {
		return &AbleconLevel{Level: 4, Reason: map[string]interface{}{"detail": "no agents"}}, nil
	}

	// Build IN clause
	placeholders := make([]string, len(agentUserIDs))
	args := make([]interface{}, len(agentUserIDs))
	for i, id := range agentUserIDs {
		placeholders[i] = "?"
		args[i] = id
	}
	inClause := "(" + strings.Join(placeholders, ", ") + ")"

	query := `SELECT status FROM agent_health_snapshot WHERE user_id IN ` + inClause

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("get user ablecon level: %w", err)
	}
	defer func() { _ = rows.Close() }()

	total := 0
	healthy := 0
	warned := 0
	flagged := 0
	suspended := 0
	for rows.Next() {
		var status string
		if err := rows.Scan(&status); err != nil {
			return nil, fmt.Errorf("scan agent status: %w", err)
		}
		total++
		switch status {
		case "healthy":
			healthy++
		case "warned":
			warned++
		case "flagged":
			flagged++
		case "suspended":
			suspended++
		}
	}

	reason := map[string]interface{}{
		"total_agents":     total,
		"healthy_agents":   healthy,
		"warned_agents":    warned,
		"flagged_agents":   flagged,
		"suspended_agents": suspended,
	}

	level := 4 // Blue: all healthy
	if warned > 0 || flagged > 0 {
		level = 3 // Green: minor signals
	}
	if suspended > 0 || flagged >= 2 {
		level = 2 // Orange: issues detected
	}
	if suspended >= 2 || (total > 0 && float64(flagged+suspended)/float64(total) > 0.5) {
		level = 1 // Red: critical
	}

	reason["level"] = level
	return &AbleconLevel{Level: level, Reason: reason}, nil
}


// UpsertAbleconLevel creates or updates the Ablecon level for a scope.
// Returns the previous level (0 if new) so callers can detect changes.
func (db *DB) UpsertAbleconLevel(scope, scopeID string, level int, reason map[string]interface{}) (int, error) {
	// Read previous level
	prevLevel := 0
	var existing sql.NullInt64
	_ = db.QueryRow(
		`SELECT level FROM ablecon_level WHERE scope = ? AND scope_id = ?`,
		scope, scopeID,
	).Scan(&existing)
	if existing.Valid {
		prevLevel = int(existing.Int64)
	}

	reasonJSON, err := json.Marshal(reason)
	if err != nil {
		reasonJSON = []byte("{}")
	}

	nowStr := UTCNow().Format(time.RFC3339)
	id := generateID("abl")
	_, err = db.Exec(
		`INSERT INTO ablecon_level (id, scope, scope_id, level, reason, updated_at, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(scope, scope_id) DO UPDATE SET
		   level = excluded.level,
		   reason = excluded.reason,
		   updated_at = excluded.updated_at`,
		id, scope, scopeID, level, string(reasonJSON), nowStr, nowStr,
	)
	if err != nil {
		return 0, fmt.Errorf("upsert ablecon level: %w", err)
	}
	return prevLevel, nil
}

// GetAbleconLevel returns the Ablecon level for a specific scope.
func (db *DB) GetAbleconLevel(scope, scopeID string) (*AbleconLevel, error) {
	row := db.QueryRow(
		`SELECT id, scope, scope_id, level, reason, updated_at, created_at
		 FROM ablecon_level WHERE scope = ? AND scope_id = ?`,
		scope, scopeID,
	)
	return scanAbleconLevel(row)
}

// GetSystemAbleconLevel returns the system-wide Ablecon level.
func (db *DB) GetSystemAbleconLevel() (*AbleconLevel, error) {
	return db.GetAbleconLevel("system", "")
}

// ListOrgAbleconLevels returns all organization-scoped Ablecon levels.
func (db *DB) ListOrgAbleconLevels() ([]*AbleconLevel, error) {
	rows, err := db.Query(
		`SELECT id, scope, scope_id, level, reason, updated_at, created_at
		 FROM ablecon_level WHERE scope = 'organization'
		 ORDER BY level DESC, scope_id`,
	)
	if err != nil {
		return nil, fmt.Errorf("list org ablecon levels: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []*AbleconLevel
	for rows.Next() {
		var a AbleconLevel
		var reasonStr, updatedAt, createdAt string
		err := rows.Scan(&a.ID, &a.Scope, &a.ScopeID, &a.Level, &reasonStr, &updatedAt, &createdAt)
		if err != nil {
			return nil, fmt.Errorf("scan ablecon level row: %w", err)
		}
		a.UpdatedAt = ParseDBTime(updatedAt)
		a.CreatedAt = ParseDBTime(createdAt)
		a.Reason = make(map[string]interface{})
		_ = json.Unmarshal([]byte(reasonStr), &a.Reason)
		result = append(result, &a)
	}
	return result, nil
}

func scanAbleconLevel(row *sql.Row) (*AbleconLevel, error) {
	var a AbleconLevel
	var reasonStr, updatedAt, createdAt string
	err := row.Scan(&a.ID, &a.Scope, &a.ScopeID, &a.Level, &reasonStr, &updatedAt, &createdAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan ablecon level: %w", err)
	}
	a.UpdatedAt = ParseDBTime(updatedAt)
	a.CreatedAt = ParseDBTime(createdAt)
	a.Reason = make(map[string]interface{})
	_ = json.Unmarshal([]byte(reasonStr), &a.Reason)
	return &a, nil
}
