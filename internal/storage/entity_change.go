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
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// EntityChangeEntry holds the data for inserting an entity change row.
type EntityChangeEntry struct {
	ActorID     string
	Action      string
	EntityType  string
	EntityID    string
	EndeavourID string
	Fields      []string
	Metadata    map[string]interface{}
}

// EntityChangeRecord represents a stored entity change entry.
type EntityChangeRecord struct {
	ID          string                 `json:"id"`
	ActorID     string                 `json:"actor_id"`
	Action      string                 `json:"action"`
	EntityType  string                 `json:"entity_type"`
	EntityID    string                 `json:"entity_id"`
	EndeavourID string                 `json:"endeavour_id"`
	Fields      []string               `json:"fields"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
}

// ListEntityChangesOpts holds filters for querying entity changes.
type ListEntityChangesOpts struct {
	Action       string
	EntityType   string
	EntityID     string
	ActorID      string
	EndeavourID  string
	EndeavourIDs []string // nil = no restriction (master admin), empty = no access
	StartTime    *time.Time
	EndTime      *time.Time
	Limit        int
	Offset       int
}

// CreateEntityChangeBatch inserts multiple entity change entries in a transaction.
func (db *DB) CreateEntityChangeBatch(entries []*EntityChangeEntry) error {
	if len(entries) == 0 {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	stmt, err := tx.Prepare(`INSERT INTO entity_change
		(id, actor_id, action, entity_type, entity_id, endeavour_id, fields, metadata, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare statement: %w", err)
	}
	defer stmt.Close() //nolint:errcheck

	now := UTCNow().Format(time.RFC3339)
	for _, e := range entries {
		id := generateID("ech")

		fieldsJSON := "[]"
		if len(e.Fields) > 0 {
			if b, err := json.Marshal(e.Fields); err == nil {
				fieldsJSON = string(b)
			}
		}

		metadataJSON := "{}"
		if e.Metadata != nil {
			if b, err := json.Marshal(e.Metadata); err == nil {
				metadataJSON = string(b)
			}
		}

		_, err := stmt.Exec(
			id, e.ActorID, e.Action, e.EntityType, e.EntityID,
			e.EndeavourID, fieldsJSON, metadataJSON, now,
		)
		if err != nil {
			return fmt.Errorf("insert entity change entry: %w", err)
		}
	}

	return tx.Commit()
}

// ListEntityChanges queries entity change entries with filters.
func (db *DB) ListEntityChanges(opts ListEntityChangesOpts) ([]*EntityChangeRecord, int, error) {
	// Empty EndeavourIDs slice (not nil) means no access.
	if opts.EndeavourIDs != nil && len(opts.EndeavourIDs) == 0 {
		return nil, 0, nil
	}

	var conditions []string
	var args []interface{}

	if opts.Action != "" {
		conditions = append(conditions, "action = ?")
		args = append(args, opts.Action)
	}
	if opts.EntityType != "" {
		conditions = append(conditions, "entity_type = ?")
		args = append(args, opts.EntityType)
	}
	if opts.EntityID != "" {
		conditions = append(conditions, "entity_id = ?")
		args = append(args, opts.EntityID)
	}
	if opts.ActorID != "" {
		conditions = append(conditions, "actor_id = ?")
		args = append(args, opts.ActorID)
	}
	if opts.EndeavourID != "" {
		conditions = append(conditions, "endeavour_id = ?")
		args = append(args, opts.EndeavourID)
	}
	if opts.StartTime != nil {
		conditions = append(conditions, "created_at >= ?")
		args = append(args, opts.StartTime.UTC().Format(time.RFC3339))
	}
	if opts.EndTime != nil {
		conditions = append(conditions, "created_at <= ?")
		args = append(args, opts.EndTime.UTC().Format(time.RFC3339))
	}

	// Scope restriction: only show changes for the user's endeavours.
	// nil = no restriction (master admin). Non-nil = filter to those IDs.
	if opts.EndeavourIDs != nil {
		placeholders := make([]string, len(opts.EndeavourIDs))
		for i, id := range opts.EndeavourIDs {
			placeholders[i] = "?"
			args = append(args, id)
		}
		conditions = append(conditions, "endeavour_id IN ("+strings.Join(placeholders, ",")+")")
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = " WHERE " + strings.Join(conditions, " AND ")
	}

	// Count total
	countQuery := "SELECT COUNT(*) FROM entity_change" + whereClause
	var total int
	if err := db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count entity changes: %w", err)
	}

	// Query with pagination
	limit := opts.Limit
	if limit <= 0 {
		limit = 50
	}
	offset := opts.Offset
	if offset < 0 {
		offset = 0
	}

	query := "SELECT id, actor_id, action, entity_type, entity_id, endeavour_id, fields, metadata, created_at FROM entity_change" +
		whereClause + " ORDER BY rowid DESC LIMIT ? OFFSET ?"
	queryArgs := append(args, limit, offset) //nolint:gocritic

	rows, err := db.Query(query, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("query entity changes: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var records []*EntityChangeRecord
	for rows.Next() {
		var r EntityChangeRecord
		var fieldsStr, metadataStr, createdAtStr string

		err := rows.Scan(
			&r.ID, &r.ActorID, &r.Action, &r.EntityType, &r.EntityID,
			&r.EndeavourID, &fieldsStr, &metadataStr, &createdAtStr,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("scan entity change row: %w", err)
		}

		r.CreatedAt = ParseDBTime(createdAtStr)

		if fieldsStr != "" && fieldsStr != "[]" {
			_ = json.Unmarshal([]byte(fieldsStr), &r.Fields)
		}
		if r.Fields == nil {
			r.Fields = []string{}
		}

		if metadataStr != "" && metadataStr != "{}" {
			_ = json.Unmarshal([]byte(metadataStr), &r.Metadata)
		}

		records = append(records, &r)
	}

	return records, total, rows.Err()
}

// DailyActivityRecord holds aggregated daily activity for an endeavour.
type DailyActivityRecord struct {
	Date      string // YYYY-MM-DD
	Created   int
	Updated   int
	Completed int
}

// DailyActivityByEndeavour returns daily create/update/complete counts for an endeavour
// since the given time, ordered by date ascending.
func (db *DB) DailyActivityByEndeavour(endeavourID string, since time.Time) ([]*DailyActivityRecord, error) {
	query := `
		SELECT
			DATE(ec.created_at) AS day,
			SUM(CASE WHEN ec.action = 'create' THEN 1 ELSE 0 END) AS created,
			SUM(CASE WHEN ec.action = 'update' THEN 1 ELSE 0 END) AS updated,
			SUM(CASE WHEN ec.action = 'update' AND ec.entity_type = 'task'
			         AND ec.fields LIKE '%status%' THEN 1 ELSE 0 END) AS completed
		FROM entity_change ec
		WHERE ec.endeavour_id = ?
		  AND ec.created_at >= ?
		GROUP BY DATE(ec.created_at)
		ORDER BY day ASC`

	rows, err := db.Query(query, endeavourID, since.UTC().Format(time.RFC3339))
	if err != nil {
		return nil, fmt.Errorf("query daily activity: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var records []*DailyActivityRecord
	for rows.Next() {
		var r DailyActivityRecord
		if err := rows.Scan(&r.Date, &r.Created, &r.Updated, &r.Completed); err != nil {
			return nil, fmt.Errorf("scan daily activity row: %w", err)
		}
		records = append(records, &r)
	}
	return records, rows.Err()
}

// ContributorRecord holds aggregated contributor data for an endeavour.
type ContributorRecord struct {
	ActorID     string
	ActorName   string
	ChangeCount int
	TasksDone   int
	TasksActive int
}

// ContributorsByEndeavour returns contributor stats for an endeavour,
// aggregating entity changes and task assignments.
func (db *DB) ContributorsByEndeavour(endeavourID string) ([]*ContributorRecord, error) {
	query := `
		SELECT
			ec.actor_id,
			COALESCE(r.name, u.name, ec.actor_id) AS actor_name,
			COUNT(DISTINCT ec.id) AS change_count,
			COALESCE(td.tasks_done, 0) AS tasks_done,
			COALESCE(td.tasks_active, 0) AS tasks_active
		FROM entity_change ec
		LEFT JOIN resource r ON r.id = ec.actor_id
		LEFT JOIN user u ON u.id = ec.actor_id
		LEFT JOIN (
			SELECT
				COALESCE(u2.id, rel_asg.target_entity_id) AS actor_id,
				SUM(CASE WHEN t.status = 'done' THEN 1 ELSE 0 END) AS tasks_done,
				SUM(CASE WHEN t.status = 'active' THEN 1 ELSE 0 END) AS tasks_active
			FROM task t
			JOIN entity_relation rel_edv ON rel_edv.source_entity_id = t.id
				AND rel_edv.source_entity_type = 'task'
				AND rel_edv.relationship_type = 'belongs_to'
				AND rel_edv.target_entity_type = 'endeavour'
				AND rel_edv.target_entity_id = ?
			JOIN entity_relation rel_asg ON rel_asg.source_entity_id = t.id
				AND rel_asg.source_entity_type = 'task'
				AND rel_asg.relationship_type = 'assigned_to'
				AND rel_asg.target_entity_type = 'resource'
			LEFT JOIN user u2 ON u2.resource_id = rel_asg.target_entity_id
			GROUP BY COALESCE(u2.id, rel_asg.target_entity_id)
		) td ON td.actor_id = ec.actor_id
		WHERE ec.endeavour_id = ?
		GROUP BY ec.actor_id
		ORDER BY change_count DESC
		LIMIT 20`

	rows, err := db.Query(query, endeavourID, endeavourID)
	if err != nil {
		return nil, fmt.Errorf("query contributors by endeavour: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var records []*ContributorRecord
	for rows.Next() {
		var r ContributorRecord
		if err := rows.Scan(&r.ActorID, &r.ActorName, &r.ChangeCount, &r.TasksDone, &r.TasksActive); err != nil {
			return nil, fmt.Errorf("scan contributor row: %w", err)
		}
		records = append(records, &r)
	}
	return records, rows.Err()
}
