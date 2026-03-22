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

// KPISnapshot represents a point-in-time collection of system metrics.
type KPISnapshot struct {
	ID        string
	Timestamp string
	Data      map[string]interface{}
	CreatedAt string
}

// InsertKPISnapshot stores a KPI snapshot in the database.
func (db *DB) InsertKPISnapshot(snap *KPISnapshot) error {
	dataJSON, err := json.Marshal(snap.Data)
	if err != nil {
		return fmt.Errorf("marshal kpi data: %w", err)
	}

	_, err = db.Exec(
		`INSERT INTO kpi_snapshot (id, timestamp, data, created_at) VALUES (?, ?, ?, ?)`,
		snap.ID, snap.Timestamp, string(dataJSON), snap.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert kpi_snapshot: %w", err)
	}
	return nil
}

// LatestKPISnapshot returns the most recent KPI snapshot.
func (db *DB) LatestKPISnapshot() (*KPISnapshot, error) {
	var snap KPISnapshot
	var dataJSON string

	err := db.QueryRow(
		`SELECT id, timestamp, data, created_at FROM kpi_snapshot ORDER BY timestamp DESC LIMIT 1`,
	).Scan(&snap.ID, &snap.Timestamp, &dataJSON, &snap.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query latest kpi_snapshot: %w", err)
	}

	if err := json.Unmarshal([]byte(dataJSON), &snap.Data); err != nil {
		return nil, fmt.Errorf("unmarshal kpi data: %w", err)
	}
	return &snap, nil
}

// ListKPISnapshots returns snapshots within a time range and the total
// count of matching snapshots (before LIMIT is applied).
// since and until are RFC3339 timestamps. If empty, no bound is applied.
// limit <= 0 means no limit.
func (db *DB) ListKPISnapshots(since, until string, limit int) ([]*KPISnapshot, int, error) {
	var conditions []string
	var args []interface{}

	if since != "" {
		conditions = append(conditions, "timestamp >= ?")
		args = append(args, since)
	}
	if until != "" {
		conditions = append(conditions, "timestamp <= ?")
		args = append(args, until)
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = " WHERE " + conditions[0]
		for _, c := range conditions[1:] {
			whereClause += " AND " + c
		}
	}

	// Get total count
	var total int
	countQuery := `SELECT COUNT(*) FROM kpi_snapshot` + whereClause
	if err := db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count kpi_snapshots: %w", err)
	}

	// Get data
	query := `SELECT id, timestamp, data, created_at FROM kpi_snapshot` + whereClause
	query += " ORDER BY timestamp DESC"
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query kpi_snapshots: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var snapshots []*KPISnapshot
	for rows.Next() {
		var snap KPISnapshot
		var dataJSON string
		if err := rows.Scan(&snap.ID, &snap.Timestamp, &dataJSON, &snap.CreatedAt); err != nil {
			continue
		}
		if err := json.Unmarshal([]byte(dataJSON), &snap.Data); err != nil {
			continue
		}
		snapshots = append(snapshots, &snap)
	}
	return snapshots, total, nil
}

// CompactKPISnapshots applies time-based compaction to KPI snapshots:
// - Last 1 hour: keep all (1-minute granularity)
// - 1h to 24h: keep 5-minute granularity
// - 1d to 30d: keep 1-hour granularity
// - Older than 30d: delete
func (db *DB) CompactKPISnapshots() error {
	now := UTCNow()

	// Delete system snapshots older than 30 days (preserve endeavour snapshots)
	cutoff30d := now.Add(-30 * 24 * time.Hour).Format(time.RFC3339)
	if _, err := db.Exec(`DELETE FROM kpi_snapshot WHERE timestamp < ? AND scope = 'system'`, cutoff30d); err != nil {
		return fmt.Errorf("delete old kpi snapshots: %w", err)
	}

	// Compact 1d-30d range to 1-hour granularity
	cutoff1d := now.Add(-24 * time.Hour).Format(time.RFC3339)
	if err := db.compactKPIRange(cutoff30d, cutoff1d, 60); err != nil {
		return fmt.Errorf("compact 1d-30d: %w", err)
	}

	// Compact 1h-24h range to 5-minute granularity
	cutoff1h := now.Add(-1 * time.Hour).Format(time.RFC3339)
	if err := db.compactKPIRange(cutoff1d, cutoff1h, 5); err != nil {
		return fmt.Errorf("compact 1h-24h: %w", err)
	}

	return nil
}

// compactKPIRange keeps only one snapshot per minuteGranularity-minute window
// within the given time range. Keeps the earliest snapshot in each window.
func (db *DB) compactKPIRange(since, until string, minuteGranularity int) error {
	// Get all system snapshot IDs and timestamps in the range (skip endeavour snapshots)
	rows, err := db.Query(
		`SELECT id, timestamp FROM kpi_snapshot WHERE timestamp >= ? AND timestamp < ? AND scope = 'system' ORDER BY timestamp ASC`,
		since, until,
	)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()

	type entry struct {
		id        string
		timestamp time.Time
	}
	var entries []entry
	for rows.Next() {
		var e entry
		var ts string
		if err := rows.Scan(&e.id, &ts); err != nil {
			continue
		}
		e.timestamp = ParseDBTime(ts)
		if !e.timestamp.IsZero() {
			entries = append(entries, e)
		}
	}

	if len(entries) <= 1 {
		return nil
	}

	// Group by time window and collect IDs to delete
	var toDelete []interface{}
	granularity := time.Duration(minuteGranularity) * time.Minute

	var currentWindowStart time.Time
	for _, e := range entries {
		windowStart := e.timestamp.Truncate(granularity)
		if windowStart.Equal(currentWindowStart) {
			// Same window as a previous entry -- mark for deletion
			toDelete = append(toDelete, e.id)
		} else {
			// New window -- keep this entry
			currentWindowStart = windowStart
		}
	}

	// Delete in batches
	for i := 0; i < len(toDelete); i += 100 {
		end := i + 100
		if end > len(toDelete) {
			end = len(toDelete)
		}
		batch := toDelete[i:end]
		placeholders := "?"
		for j := 1; j < len(batch); j++ {
			placeholders += ",?"
		}
		_, err := db.Exec(
			fmt.Sprintf(`DELETE FROM kpi_snapshot WHERE id IN (%s)`, placeholders),
			batch...,
		)
		if err != nil {
			return fmt.Errorf("delete compacted snapshots: %w", err)
		}
	}

	return nil
}

// DemandStatusCounts returns counts of demands grouped by status.
func (db *DB) DemandStatusCounts() (map[string]int, error) {
	rows, err := db.Query(`SELECT status, COUNT(*) FROM demand GROUP BY status`)
	if err != nil {
		return nil, fmt.Errorf("query demand status counts: %w", err)
	}
	defer func() { _ = rows.Close() }()

	counts := make(map[string]int)
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			continue
		}
		counts[status] = count
	}
	return counts, nil
}

// UserStatusCounts returns counts of users grouped by status.
func (db *DB) UserStatusCounts() (map[string]int, error) {
	rows, err := db.Query(`SELECT status, COUNT(*) FROM user GROUP BY status`)
	if err != nil {
		return nil, fmt.Errorf("query user status counts: %w", err)
	}
	defer func() { _ = rows.Close() }()

	counts := make(map[string]int)
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			continue
		}
		counts[status] = count
	}
	return counts, nil
}

// AuditCountsSince returns counts of audit log actions since the given time.
// Returns a map of action -> count (e.g., "login_success" -> 15).
func (db *DB) AuditCountsSince(since time.Time) (map[string]int, error) {
	sinceStr := since.Format(time.RFC3339)
	rows, err := db.Query(
		`SELECT action, COUNT(*) FROM audit_log WHERE created_at >= ? GROUP BY action`,
		sinceStr,
	)
	if err != nil {
		return nil, fmt.Errorf("query audit counts: %w", err)
	}
	defer func() { _ = rows.Close() }()

	counts := make(map[string]int)
	for rows.Next() {
		var action string
		var count int
		if err := rows.Scan(&action, &count); err != nil {
			continue
		}
		counts[action] = count
	}
	return counts, nil
}

// ActiveSessionCount returns the number of non-expired, non-revoked API tokens.
func (db *DB) ActiveSessionCount() (int, error) {
	var count int
	err := db.QueryRow(
		`SELECT COUNT(*) FROM token
		 WHERE revoked_at IS NULL
		   AND (expires_at IS NULL OR expires_at > ?)`,
		UTCNow().Format(time.RFC3339),
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count active sessions: %w", err)
	}
	return count, nil
}

// AuditCountsByIPSince returns per-IP counts of a specific audit action since the given time.
func (db *DB) AuditCountsByIPSince(action string, since time.Time) (map[string]int, error) {
	sinceStr := since.Format(time.RFC3339)
	rows, err := db.Query(
		`SELECT ip, COUNT(*) FROM audit_log
		 WHERE action = ? AND created_at >= ? AND ip != ''
		 GROUP BY ip`,
		action, sinceStr,
	)
	if err != nil {
		return nil, fmt.Errorf("query audit counts by ip: %w", err)
	}
	defer func() { _ = rows.Close() }()

	counts := make(map[string]int)
	for rows.Next() {
		var ip string
		var count int
		if err := rows.Scan(&ip, &count); err != nil {
			continue
		}
		counts[ip] = count
	}
	return counts, nil
}
