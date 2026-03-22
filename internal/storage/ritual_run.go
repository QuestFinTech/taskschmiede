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
	"errors"
	"fmt"
	"strings"
	"time"
)

// RitualRun represents a single execution of a ritual.
type RitualRun struct {
	ID            string
	RitualID      string
	Status        string // running, succeeded, failed, skipped
	Trigger       string // schedule, manual, api
	RunBy         string
	ResultSummary string
	Effects       map[string]interface{}
	Error         map[string]interface{}
	Metadata      map[string]interface{}
	StartedAt     *time.Time
	FinishedAt    *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// ListRitualRunsOpts holds filters for listing ritual runs.
type ListRitualRunsOpts struct {
	RitualID     string
	EndeavourIDs []string // RBAC: nil = no restriction; empty = no access
	Status       string
	Limit        int
	Offset       int
}

// UpdateRitualRunFields holds the fields to update on a ritual run.
// Only non-nil fields are applied.
type UpdateRitualRunFields struct {
	Status        *string
	ResultSummary *string
	Effects       map[string]interface{}
	Error         map[string]interface{}
	Metadata      map[string]interface{}
	FinishedAt    *string // RFC3339 or empty to clear
}

// ErrRitualRunNotFound is returned when a ritual run cannot be found by its ID.
var ErrRitualRunNotFound = errors.New("ritual run not found")

// CreateRitualRun creates a new ritual run with status=running and started_at=now.
// The ritual link uses a direct FK (ritual_id column), not FRM entity_relation.
func (db *DB) CreateRitualRun(ritualID, trigger, runBy string, metadata map[string]interface{}) (*RitualRun, error) {
	id := generateID("rtr")

	metadataJSON := "{}"
	if metadata != nil {
		b, err := json.Marshal(metadata)
		if err != nil {
			return nil, fmt.Errorf("marshal metadata: %w", err)
		}
		metadataJSON = string(b)
	}

	var runByVal *string
	if runBy != "" {
		runByVal = &runBy
	}

	now := UTCNow()
	nowStr := now.Format(time.RFC3339)

	_, err := db.Exec(
		`INSERT INTO ritual_run (id, ritual_id, status, "trigger", run_by, metadata, started_at)
		 VALUES (?, ?, 'running', ?, ?, ?, ?)`,
		id, ritualID, trigger, runByVal, metadataJSON, nowStr,
	)
	if err != nil {
		return nil, fmt.Errorf("insert ritual_run: %w", err)
	}

	startedAt := now
	return &RitualRun{
		ID:        id,
		RitualID:  ritualID,
		Status:    "running",
		Trigger:   trigger,
		RunBy:     runBy,
		Metadata:  metadata,
		StartedAt: &startedAt,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// GetRitualRun retrieves a ritual run by ID.
func (db *DB) GetRitualRun(id string) (*RitualRun, error) {
	var r RitualRun
	var runBy sql.NullString
	var resultSummary sql.NullString
	var effectsJSON, metadataJSON string
	var errorJSON sql.NullString
	var startedAt, finishedAt sql.NullString
	var createdAt, updatedAt string

	err := db.QueryRow(
		`SELECT id, ritual_id, status, "trigger", run_by, result_summary,
		        effects, error, metadata, started_at, finished_at, created_at, updated_at
		 FROM ritual_run
		 WHERE id = ?`,
		id,
	).Scan(&r.ID, &r.RitualID, &r.Status, &r.Trigger, &runBy, &resultSummary,
		&effectsJSON, &errorJSON, &metadataJSON, &startedAt, &finishedAt,
		&createdAt, &updatedAt)

	if err == sql.ErrNoRows {
		return nil, ErrRitualRunNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query ritual_run: %w", err)
	}

	if runBy.Valid {
		r.RunBy = runBy.String
	}
	if resultSummary.Valid {
		r.ResultSummary = resultSummary.String
	}
	if startedAt.Valid {
		t := ParseDBTime(startedAt.String)
		r.StartedAt = &t
	}
	if finishedAt.Valid {
		t := ParseDBTime(finishedAt.String)
		r.FinishedAt = &t
	}
	_ = json.Unmarshal([]byte(effectsJSON), &r.Effects)
	if errorJSON.Valid {
		_ = json.Unmarshal([]byte(errorJSON.String), &r.Error)
	}
	_ = json.Unmarshal([]byte(metadataJSON), &r.Metadata)
	r.CreatedAt = ParseDBTime(createdAt)
	r.UpdatedAt = ParseDBTime(updatedAt)

	return &r, nil
}

// GetLastRitualRun returns the most recent ritual run for a given ritual,
// regardless of status. Returns nil (not an error) if no runs exist yet.
func (db *DB) GetLastRitualRun(ritualID string) (*RitualRun, error) {
	var r RitualRun
	var runBy sql.NullString
	var resultSummary sql.NullString
	var effectsJSON, metadataJSON string
	var errorJSON sql.NullString
	var startedAt, finishedAt sql.NullString
	var createdAt, updatedAt string

	err := db.QueryRow(
		`SELECT id, ritual_id, status, "trigger", run_by, result_summary,
		        effects, error, metadata, started_at, finished_at, created_at, updated_at
		 FROM ritual_run
		 WHERE ritual_id = ?
		 ORDER BY created_at DESC
		 LIMIT 1`,
		ritualID,
	).Scan(&r.ID, &r.RitualID, &r.Status, &r.Trigger, &runBy, &resultSummary,
		&effectsJSON, &errorJSON, &metadataJSON, &startedAt, &finishedAt,
		&createdAt, &updatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query last ritual_run: %w", err)
	}

	if runBy.Valid {
		r.RunBy = runBy.String
	}
	if resultSummary.Valid {
		r.ResultSummary = resultSummary.String
	}
	if startedAt.Valid {
		t := ParseDBTime(startedAt.String)
		r.StartedAt = &t
	}
	if finishedAt.Valid {
		t := ParseDBTime(finishedAt.String)
		r.FinishedAt = &t
	}
	_ = json.Unmarshal([]byte(effectsJSON), &r.Effects)
	if errorJSON.Valid {
		_ = json.Unmarshal([]byte(errorJSON.String), &r.Error)
	}
	_ = json.Unmarshal([]byte(metadataJSON), &r.Metadata)
	r.CreatedAt = ParseDBTime(createdAt)
	r.UpdatedAt = ParseDBTime(updatedAt)

	return &r, nil
}

// ListRitualRuns queries ritual runs with filters.
func (db *DB) ListRitualRuns(opts ListRitualRunsOpts) ([]*RitualRun, int, error) {
	// Non-nil but empty EndeavourIDs means "no access to any endeavour".
	if opts.EndeavourIDs != nil && len(opts.EndeavourIDs) == 0 {
		return nil, 0, nil
	}

	query := `SELECT id, ritual_id, status, "trigger", run_by, result_summary,
	                 effects, error, metadata, started_at, finished_at, created_at, updated_at
	          FROM ritual_run`
	countQuery := `SELECT COUNT(*) FROM ritual_run`

	var conditions []string
	var params []interface{}

	if opts.RitualID != "" {
		conditions = append(conditions, "ritual_id = ?")
		params = append(params, opts.RitualID)
	}
	if opts.Status != "" {
		conditions = append(conditions, "status = ?")
		params = append(params, opts.Status)
	}
	if len(opts.EndeavourIDs) > 0 {
		placeholders := make([]string, len(opts.EndeavourIDs))
		for i := range opts.EndeavourIDs {
			placeholders[i] = "?"
			params = append(params, opts.EndeavourIDs[i])
		}
		inClause := strings.Join(placeholders, ", ")
		// Left join through the ritual's "governs" relation to filter by endeavour.
		// LEFT JOIN so that runs of unlinked rituals remain visible.
		edvJoin := ` LEFT JOIN entity_relation rr_edv ON rr_edv.source_entity_id = ritual_run.ritual_id
		    AND rr_edv.source_entity_type = 'ritual'
		    AND rr_edv.relationship_type = 'governs'
		    AND rr_edv.target_entity_type = 'endeavour'`
		query = `SELECT ritual_run.id, ritual_run.ritual_id, ritual_run.status, ritual_run."trigger",
		                ritual_run.run_by, ritual_run.result_summary,
		                ritual_run.effects, ritual_run.error, ritual_run.metadata,
		                ritual_run.started_at, ritual_run.finished_at,
		                ritual_run.created_at, ritual_run.updated_at
		         FROM ritual_run` + edvJoin
		countQuery = `SELECT COUNT(*) FROM ritual_run` + edvJoin
		conditions = append(conditions, "(rr_edv.target_entity_id IN ("+inClause+") OR rr_edv.target_entity_id IS NULL)")
	}

	if len(conditions) > 0 {
		where := " WHERE " + strings.Join(conditions, " AND ")
		query += where
		countQuery += where
	}

	var total int
	_ = db.QueryRow(countQuery, params...).Scan(&total)

	if opts.Limit <= 0 {
		opts.Limit = 50
	}
	query += ` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	params = append(params, opts.Limit, opts.Offset)

	rows, err := db.Query(query, params...)
	if err != nil {
		return nil, 0, fmt.Errorf("query ritual_runs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var runs []*RitualRun
	for rows.Next() {
		var r RitualRun
		var runBy sql.NullString
		var resultSummary sql.NullString
		var effectsJSON, metadataJSON string
		var errorJSON sql.NullString
		var startedAt, finishedAt sql.NullString
		var createdAt, updatedAt string

		if err := rows.Scan(&r.ID, &r.RitualID, &r.Status, &r.Trigger, &runBy, &resultSummary,
			&effectsJSON, &errorJSON, &metadataJSON, &startedAt, &finishedAt,
			&createdAt, &updatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan ritual_run: %w", err)
		}

		if runBy.Valid {
			r.RunBy = runBy.String
		}
		if resultSummary.Valid {
			r.ResultSummary = resultSummary.String
		}
		if startedAt.Valid {
			t := ParseDBTime(startedAt.String)
			r.StartedAt = &t
		}
		if finishedAt.Valid {
			t := ParseDBTime(finishedAt.String)
			r.FinishedAt = &t
		}
		_ = json.Unmarshal([]byte(effectsJSON), &r.Effects)
		if errorJSON.Valid {
			_ = json.Unmarshal([]byte(errorJSON.String), &r.Error)
		}
		_ = json.Unmarshal([]byte(metadataJSON), &r.Metadata)
		r.CreatedAt = ParseDBTime(createdAt)
		r.UpdatedAt = ParseDBTime(updatedAt)

		runs = append(runs, &r)
	}

	return runs, total, nil
}

// UpdateRitualRun applies partial updates to a ritual run.
func (db *DB) UpdateRitualRun(id string, fields UpdateRitualRunFields) ([]string, error) {
	var setClauses []string
	var params []interface{}
	var updatedFields []string

	if fields.Status != nil {
		setClauses = append(setClauses, "status = ?")
		params = append(params, *fields.Status)
		updatedFields = append(updatedFields, "status")

		// Auto-set finished_at when transitioning to a terminal status
		switch *fields.Status {
		case "succeeded", "failed", "skipped":
			if fields.FinishedAt == nil {
				nowStr := UTCNow().Format(time.RFC3339)
				setClauses = append(setClauses, "finished_at = ?")
				params = append(params, nowStr)
				updatedFields = append(updatedFields, "finished_at")
			}
		}
	}
	if fields.ResultSummary != nil {
		setClauses = append(setClauses, "result_summary = ?")
		params = append(params, *fields.ResultSummary)
		updatedFields = append(updatedFields, "result_summary")
	}
	if fields.Effects != nil {
		b, err := json.Marshal(fields.Effects)
		if err != nil {
			return nil, fmt.Errorf("marshal effects: %w", err)
		}
		setClauses = append(setClauses, "effects = ?")
		params = append(params, string(b))
		updatedFields = append(updatedFields, "effects")
	}
	if fields.Error != nil {
		b, err := json.Marshal(fields.Error)
		if err != nil {
			return nil, fmt.Errorf("marshal error: %w", err)
		}
		setClauses = append(setClauses, "error = ?")
		params = append(params, string(b))
		updatedFields = append(updatedFields, "error")
	}
	if fields.Metadata != nil {
		b, err := json.Marshal(fields.Metadata)
		if err != nil {
			return nil, fmt.Errorf("marshal metadata: %w", err)
		}
		setClauses = append(setClauses, "metadata = ?")
		params = append(params, string(b))
		updatedFields = append(updatedFields, "metadata")
	}
	if fields.FinishedAt != nil {
		if *fields.FinishedAt == "" {
			setClauses = append(setClauses, "finished_at = NULL")
		} else {
			setClauses = append(setClauses, "finished_at = ?")
			params = append(params, *fields.FinishedAt)
		}
		updatedFields = append(updatedFields, "finished_at")
	}

	if len(setClauses) == 0 {
		return nil, fmt.Errorf("no fields to update")
	}

	query := fmt.Sprintf("UPDATE ritual_run SET %s WHERE id = ?", strings.Join(setClauses, ", "))
	params = append(params, id)

	result, err := db.Exec(query, params...)
	if err != nil {
		return nil, fmt.Errorf("update ritual_run: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return nil, ErrRitualRunNotFound
	}

	return updatedFields, nil
}

// RitualRunSummary holds aggregated run statistics for a single ritual.
type RitualRunSummary struct {
	RitualID   string
	RitualName string
	TotalRuns  int
	Succeeded  int
	Failed     int
	Skipped    int
	LastRunDate string // RFC3339 or empty
}

// RitualRunSummaryByEndeavour returns per-ritual run aggregates for all active
// rituals that govern an endeavour. Rituals with zero runs are included.
func (db *DB) RitualRunSummaryByEndeavour(endeavourID string) ([]*RitualRunSummary, error) {
	query := `
		SELECT
			r.id AS ritual_id,
			r.name AS ritual_name,
			COUNT(rr.id) AS total_runs,
			SUM(CASE WHEN rr.status = 'succeeded' THEN 1 ELSE 0 END) AS succeeded,
			SUM(CASE WHEN rr.status = 'failed' THEN 1 ELSE 0 END) AS failed,
			SUM(CASE WHEN rr.status = 'skipped' THEN 1 ELSE 0 END) AS skipped,
			MAX(rr.created_at) AS last_run
		FROM ritual r
		JOIN entity_relation rel_gov ON rel_gov.source_entity_id = r.id
			AND rel_gov.source_entity_type = 'ritual'
			AND rel_gov.relationship_type = 'governs'
			AND rel_gov.target_entity_type = 'endeavour'
			AND rel_gov.target_entity_id = ?
		LEFT JOIN ritual_run rr ON rr.ritual_id = r.id
		WHERE r.status = 'active'
		GROUP BY r.id
		ORDER BY r.name ASC`

	rows, err := db.Query(query, endeavourID)
	if err != nil {
		return nil, fmt.Errorf("query ritual run summary: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var summaries []*RitualRunSummary
	for rows.Next() {
		var s RitualRunSummary
		var lastRun sql.NullString
		if err := rows.Scan(&s.RitualID, &s.RitualName, &s.TotalRuns,
			&s.Succeeded, &s.Failed, &s.Skipped, &lastRun); err != nil {
			return nil, fmt.Errorf("scan ritual run summary: %w", err)
		}
		if lastRun.Valid {
			s.LastRunDate = lastRun.String
		}
		summaries = append(summaries, &s)
	}
	return summaries, rows.Err()
}
