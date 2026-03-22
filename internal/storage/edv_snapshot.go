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

// EndeavourSnapshot represents a periodic KPI snapshot for a single endeavour.
type EndeavourSnapshot struct {
	ID           string
	EndeavourID  string
	Period       string // "daily", "weekly"
	SnapshotDate string // YYYY-MM-DD
	Metrics      map[string]interface{}
	CreatedAt    string
}

// UpsertEndeavourSnapshot inserts or updates an endeavour KPI snapshot.
// Idempotent: uses the partial unique index on (endeavour_id, scope, period, snapshot_date).
func (db *DB) UpsertEndeavourSnapshot(edvID, period, snapshotDate string, metrics map[string]interface{}) error {
	metricsJSON, err := json.Marshal(metrics)
	if err != nil {
		return fmt.Errorf("marshal snapshot metrics: %w", err)
	}

	now := UTCNow().Format(time.RFC3339)
	id := generateID("kpi")

	_, err = db.Exec(`
		INSERT INTO kpi_snapshot (id, timestamp, data, created_at, endeavour_id, scope, period, snapshot_date)
		VALUES (?, ?, ?, ?, ?, 'endeavour', ?, ?)
		ON CONFLICT(endeavour_id, scope, period, snapshot_date)
		WHERE endeavour_id != ''
		DO UPDATE SET data = excluded.data, timestamp = excluded.timestamp`,
		id, now, string(metricsJSON), now, edvID, period, snapshotDate,
	)
	if err != nil {
		return fmt.Errorf("upsert endeavour snapshot: %w", err)
	}
	return nil
}

// ListEndeavourSnapshots returns snapshots for an endeavour, newest first.
// Filters by period and optionally by a minimum snapshot_date (since, YYYY-MM-DD).
// Limit defaults to 90 if <= 0.
func (db *DB) ListEndeavourSnapshots(edvID, period, since string, limit int) ([]*EndeavourSnapshot, error) {
	if limit <= 0 {
		limit = 90
	}

	var conditions []string
	var args []interface{}

	conditions = append(conditions, "endeavour_id = ?")
	args = append(args, edvID)

	conditions = append(conditions, "scope = 'endeavour'")

	conditions = append(conditions, "period = ?")
	args = append(args, period)

	if since != "" {
		conditions = append(conditions, "snapshot_date >= ?")
		args = append(args, since)
	}

	query := "SELECT id, endeavour_id, period, snapshot_date, data, created_at FROM kpi_snapshot WHERE "
	for i, c := range conditions {
		if i > 0 {
			query += " AND "
		}
		query += c
	}
	query += " ORDER BY snapshot_date DESC"
	query += fmt.Sprintf(" LIMIT %d", limit)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query endeavour snapshots: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var snapshots []*EndeavourSnapshot
	for rows.Next() {
		var s EndeavourSnapshot
		var dataJSON sql.NullString
		if err := rows.Scan(&s.ID, &s.EndeavourID, &s.Period, &s.SnapshotDate, &dataJSON, &s.CreatedAt); err != nil {
			continue
		}
		if dataJSON.Valid {
			_ = json.Unmarshal([]byte(dataJSON.String), &s.Metrics)
		}
		if s.Metrics == nil {
			s.Metrics = make(map[string]interface{})
		}
		snapshots = append(snapshots, &s)
	}
	return snapshots, rows.Err()
}
