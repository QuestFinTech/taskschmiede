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
	"log/slog"
	"strings"
	"time"
)

// Demand represents what needs to be fulfilled.
type Demand struct {
	ID             string
	Type           string
	Title          string
	Description    string
	Status         string
	Priority       string
	EndeavourID    string // populated from entity_relation
	EndeavourName  string // denormalized for convenience
	CreatorID      string
	CreatorName    string // denormalized for convenience
	OwnerID        string
	OwnerName      string // denormalized for convenience
	DueDate        *time.Time
	Metadata       map[string]interface{}
	CreatedAt      time.Time
	UpdatedAt      time.Time
	FulfilledAt    *time.Time
	CanceledAt     *time.Time
	CanceledReason string
}

// ListDemandsOpts holds filters for listing demands.
type ListDemandsOpts struct {
	Status       string
	Type         string
	Priority     string
	EndeavourID  string
	EndeavourIDs []string // RBAC: nil = no restriction; empty = no access
	Search       string
	Limit        int
	Offset       int
}

// UpdateDemandFields holds the fields to update on a demand.
type UpdateDemandFields struct {
	Title          *string
	Description    *string
	Type           *string
	Status         *string
	Priority       *string
	EndeavourID    *string
	OwnerID        *string
	DueDate        *string // RFC3339 or empty to clear
	Metadata       map[string]interface{}
	CanceledReason *string
}

// DemandFulfillmentRecord holds demand-level task progress for project reports.
type DemandFulfillmentRecord struct {
	ID        string
	Title     string
	Type      string
	Priority  string
	Status    string
	TaskTotal int
	TaskDone  int
}

// ErrDemandNotFound is returned when a demand cannot be found by its ID.
var ErrDemandNotFound = errors.New("demand not found")

// CreateDemand creates a new demand. FRM-native: no direct FK columns.
// The endeavour link is created via entity_relation.
func (db *DB) CreateDemand(demandType, title, description, priority, endeavourID, creatorID string, dueDate *time.Time, metadata map[string]interface{}) (*Demand, error) {
	id := generateID("dmd")

	metadataJSON := "{}"
	if metadata != nil {
		b, err := json.Marshal(metadata)
		if err != nil {
			return nil, fmt.Errorf("marshal metadata: %w", err)
		}
		metadataJSON = string(b)
	}

	var descVal *string
	if description != "" {
		descVal = &description
	}

	var dueDateStr *string
	if dueDate != nil {
		s := dueDate.Format(time.RFC3339)
		dueDateStr = &s
	}

	var creatorVal *string
	if creatorID != "" {
		creatorVal = &creatorID
	}

	now := UTCNow()
	_, err := db.Exec(
		`INSERT INTO demand (id, type, title, description, status, priority, due_date, metadata, creator_id)
		 VALUES (?, ?, ?, ?, 'open', ?, ?, ?, ?)`,
		id, demandType, title, descVal, priority, dueDateStr, metadataJSON, creatorVal,
	)
	if err != nil {
		return nil, fmt.Errorf("insert demand: %w", err)
	}

	// Create endeavour link via entity_relation
	if endeavourID != "" {
		relID := generateID("rel")
		_, _ = db.Exec(
			`INSERT INTO entity_relation (id, relationship_type, source_entity_type, source_entity_id,
			 target_entity_type, target_entity_id, created_at)
			 VALUES (?, ?, 'demand', ?, 'endeavour', ?, ?)`,
			relID, RelBelongsTo, id, endeavourID, now.Format(time.RFC3339),
		)
	}

	return &Demand{
		ID:          id,
		Type:        demandType,
		Title:       title,
		Description: description,
		Status:      "open",
		Priority:    priority,
		EndeavourID: endeavourID,
		CreatorID:   creatorID,
		DueDate:     dueDate,
		Metadata:    metadata,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

// GetDemand retrieves a demand by ID with endeavour from entity_relation.
func (db *DB) GetDemand(id string) (*Demand, error) {
	var d Demand
	var description sql.NullString
	var dueDate, fulfilledAt, canceledAt sql.NullString
	var canceledReason sql.NullString
	var endeavourID sql.NullString
	var endeavourName sql.NullString
	var creatorID, creatorName sql.NullString
	var ownerID, ownerName sql.NullString
	var metadataJSON string
	var createdAt, updatedAt string

	err := db.QueryRow(
		`SELECT d.id, d.type, d.title, d.description, d.status, d.priority,
		        rel_edv.target_entity_id,
		        e.name,
		        d.creator_id, rc.name,
		        d.owner_id, ro.name,
		        d.due_date, d.metadata, d.created_at, d.updated_at,
		        d.fulfilled_at, d.canceled_at, d.canceled_reason
		 FROM demand d
		 LEFT JOIN entity_relation rel_edv ON rel_edv.source_entity_id = d.id
		     AND rel_edv.source_entity_type = 'demand'
		     AND rel_edv.relationship_type = 'belongs_to'
		     AND rel_edv.target_entity_type = 'endeavour'
		 LEFT JOIN endeavour e ON rel_edv.target_entity_id = e.id
		 LEFT JOIN resource rc ON d.creator_id = rc.id
		 LEFT JOIN resource ro ON d.owner_id = ro.id
		 WHERE d.id = ?`,
		id,
	).Scan(&d.ID, &d.Type, &d.Title, &description, &d.Status, &d.Priority,
		&endeavourID, &endeavourName,
		&creatorID, &creatorName, &ownerID, &ownerName,
		&dueDate, &metadataJSON, &createdAt, &updatedAt,
		&fulfilledAt, &canceledAt, &canceledReason)

	if err == sql.ErrNoRows {
		return nil, ErrDemandNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query demand: %w", err)
	}

	if description.Valid {
		d.Description = description.String
	}
	if endeavourID.Valid {
		d.EndeavourID = endeavourID.String
	}
	if endeavourName.Valid {
		d.EndeavourName = endeavourName.String
	}
	if creatorID.Valid {
		d.CreatorID = creatorID.String
	}
	if creatorName.Valid {
		d.CreatorName = creatorName.String
	}
	if ownerID.Valid {
		d.OwnerID = ownerID.String
	}
	if ownerName.Valid {
		d.OwnerName = ownerName.String
	}
	if dueDate.Valid {
		t := ParseDBTime(dueDate.String)
		d.DueDate = &t
	}
	if fulfilledAt.Valid {
		t := ParseDBTime(fulfilledAt.String)
		d.FulfilledAt = &t
	}
	if canceledAt.Valid {
		t := ParseDBTime(canceledAt.String)
		d.CanceledAt = &t
	}
	if canceledReason.Valid {
		d.CanceledReason = canceledReason.String
	}
	if metadataJSON != "" {
		if err := json.Unmarshal([]byte(metadataJSON), &d.Metadata); err != nil {
			slog.Warn("Failed to decode demand metadata", "demand_id", d.ID, "error", err)
		}
	}
	d.CreatedAt = ParseDBTime(createdAt)
	d.UpdatedAt = ParseDBTime(updatedAt)

	return &d, nil
}

// ListDemands queries demands with filters.
func (db *DB) ListDemands(opts ListDemandsOpts) ([]*Demand, int, error) {
	// Non-nil but empty EndeavourIDs means "no access to any endeavour".
	if opts.EndeavourIDs != nil && len(opts.EndeavourIDs) == 0 {
		return nil, 0, nil
	}

	query := `SELECT d.id, d.type, d.title, d.description, d.status, d.priority,
	                 rel_edv.target_entity_id,
	                 e.name,
	                 d.creator_id, rc.name,
	                 d.owner_id, ro.name,
	                 d.due_date, d.metadata, d.created_at, d.updated_at,
	                 d.fulfilled_at, d.canceled_at, d.canceled_reason
	          FROM demand d
	          LEFT JOIN entity_relation rel_edv ON rel_edv.source_entity_id = d.id
	              AND rel_edv.source_entity_type = 'demand'
	              AND rel_edv.relationship_type = 'belongs_to'
	              AND rel_edv.target_entity_type = 'endeavour'
	          LEFT JOIN endeavour e ON rel_edv.target_entity_id = e.id
	          LEFT JOIN resource rc ON d.creator_id = rc.id
	          LEFT JOIN resource ro ON d.owner_id = ro.id`
	countQuery := `SELECT COUNT(*) FROM demand d`

	var conditions []string
	var countConditions []string
	var params []interface{}
	var countParams []interface{}

	if opts.Status != "" {
		conditions = append(conditions, "d.status = ?")
		countConditions = append(countConditions, "d.status = ?")
		params = append(params, opts.Status)
		countParams = append(countParams, opts.Status)
	}
	if opts.Type != "" {
		conditions = append(conditions, "d.type = ?")
		countConditions = append(countConditions, "d.type = ?")
		params = append(params, opts.Type)
		countParams = append(countParams, opts.Type)
	}
	if opts.Priority != "" {
		conditions = append(conditions, "d.priority = ?")
		countConditions = append(countConditions, "d.priority = ?")
		params = append(params, opts.Priority)
		countParams = append(countParams, opts.Priority)
	}
	if opts.EndeavourID != "" {
		conditions = append(conditions, "rel_edv.target_entity_id = ?")
		params = append(params, opts.EndeavourID)
		countQuery += ` JOIN entity_relation cr_edv ON cr_edv.source_entity_id = d.id
		    AND cr_edv.source_entity_type = 'demand'
		    AND cr_edv.relationship_type = 'belongs_to'
		    AND cr_edv.target_entity_type = 'endeavour'`
		countConditions = append(countConditions, "cr_edv.target_entity_id = ?")
		countParams = append(countParams, opts.EndeavourID)
	}
	if len(opts.EndeavourIDs) > 0 {
		placeholders := make([]string, len(opts.EndeavourIDs))
		for i := range opts.EndeavourIDs {
			placeholders[i] = "?"
			params = append(params, opts.EndeavourIDs[i])
			countParams = append(countParams, opts.EndeavourIDs[i])
		}
		inClause := strings.Join(placeholders, ", ")
		conditions = append(conditions, "(rel_edv.target_entity_id IN ("+inClause+") OR rel_edv.target_entity_id IS NULL)")
		countQuery += ` LEFT JOIN entity_relation cr_edv2 ON cr_edv2.source_entity_id = d.id
		    AND cr_edv2.source_entity_type = 'demand'
		    AND cr_edv2.relationship_type = 'belongs_to'
		    AND cr_edv2.target_entity_type = 'endeavour'`
		countConditions = append(countConditions, "(cr_edv2.target_entity_id IN ("+inClause+") OR cr_edv2.target_entity_id IS NULL)")
	}
	if opts.Search != "" {
		conditions = append(conditions, "(d.title LIKE ? ESCAPE '\\' OR d.description LIKE ? ESCAPE '\\')")
		countConditions = append(countConditions, "(d.title LIKE ? ESCAPE '\\' OR d.description LIKE ? ESCAPE '\\')")
		searchParam := "%" + escapeLike(opts.Search, '\\') + "%"
		params = append(params, searchParam, searchParam)
		countParams = append(countParams, searchParam, searchParam)
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	if len(countConditions) > 0 {
		countQuery += " WHERE " + strings.Join(countConditions, " AND ")
	}

	var total int
	if err := db.QueryRow(countQuery, countParams...).Scan(&total); err != nil {
		slog.Warn("Failed to count demands", "error", err)
	}

	if opts.Limit <= 0 {
		opts.Limit = 50
	}
	query += ` ORDER BY d.created_at DESC LIMIT ? OFFSET ?`
	params = append(params, opts.Limit, opts.Offset)

	rows, err := db.Query(query, params...)
	if err != nil {
		return nil, 0, fmt.Errorf("query demands: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var demands []*Demand
	for rows.Next() {
		var d Demand
		var description sql.NullString
		var endeavourID sql.NullString
		var endeavourName sql.NullString
		var creatorID, creatorName, ownerID, ownerName sql.NullString
		var dueDate, fulfilledAt, canceledAt sql.NullString
		var canceledReason sql.NullString
		var metadataJSON string
		var createdAt, updatedAt string

		if err := rows.Scan(&d.ID, &d.Type, &d.Title, &description, &d.Status, &d.Priority,
			&endeavourID, &endeavourName,
			&creatorID, &creatorName, &ownerID, &ownerName,
			&dueDate, &metadataJSON, &createdAt, &updatedAt,
			&fulfilledAt, &canceledAt, &canceledReason); err != nil {
			return nil, 0, fmt.Errorf("scan demand: %w", err)
		}

		if description.Valid {
			d.Description = description.String
		}
		if endeavourID.Valid {
			d.EndeavourID = endeavourID.String
		}
		if endeavourName.Valid {
			d.EndeavourName = endeavourName.String
		}
		if creatorID.Valid {
			d.CreatorID = creatorID.String
		}
		if creatorName.Valid {
			d.CreatorName = creatorName.String
		}
		if ownerID.Valid {
			d.OwnerID = ownerID.String
		}
		if ownerName.Valid {
			d.OwnerName = ownerName.String
		}
		if dueDate.Valid {
			t := ParseDBTime(dueDate.String)
			d.DueDate = &t
		}
		if fulfilledAt.Valid {
			t := ParseDBTime(fulfilledAt.String)
			d.FulfilledAt = &t
		}
		if canceledAt.Valid {
			t := ParseDBTime(canceledAt.String)
			d.CanceledAt = &t
		}
		if canceledReason.Valid {
			d.CanceledReason = canceledReason.String
		}
		_ = json.Unmarshal([]byte(metadataJSON), &d.Metadata)
		d.CreatedAt = ParseDBTime(createdAt)
		d.UpdatedAt = ParseDBTime(updatedAt)

		demands = append(demands, &d)
	}

	return demands, total, nil
}

// UpdateDemand applies partial updates to a demand.
func (db *DB) UpdateDemand(id string, fields UpdateDemandFields) ([]string, error) {
	var setClauses []string
	var params []interface{}
	var updatedFields []string

	if fields.Title != nil {
		setClauses = append(setClauses, "title = ?")
		params = append(params, *fields.Title)
		updatedFields = append(updatedFields, "title")
	}
	if fields.Description != nil {
		setClauses = append(setClauses, "description = ?")
		params = append(params, *fields.Description)
		updatedFields = append(updatedFields, "description")
	}
	if fields.Type != nil {
		setClauses = append(setClauses, "type = ?")
		params = append(params, *fields.Type)
		updatedFields = append(updatedFields, "type")
	}
	if fields.Status != nil {
		setClauses = append(setClauses, "status = ?")
		params = append(params, *fields.Status)
		updatedFields = append(updatedFields, "status")

		now := UTCNow().Format(time.RFC3339)
		switch *fields.Status {
		case "fulfilled":
			setClauses = append(setClauses, "fulfilled_at = ?")
			params = append(params, now)
		case "canceled":
			setClauses = append(setClauses, "canceled_at = ?")
			params = append(params, now)
			if fields.CanceledReason != nil {
				setClauses = append(setClauses, "canceled_reason = ?")
				params = append(params, *fields.CanceledReason)
				updatedFields = append(updatedFields, "canceled_reason")
			}
		}
	}
	if fields.Priority != nil {
		setClauses = append(setClauses, "priority = ?")
		params = append(params, *fields.Priority)
		updatedFields = append(updatedFields, "priority")
	}
	if fields.DueDate != nil {
		if *fields.DueDate == "" {
			setClauses = append(setClauses, "due_date = NULL")
		} else {
			setClauses = append(setClauses, "due_date = ?")
			params = append(params, *fields.DueDate)
		}
		updatedFields = append(updatedFields, "due_date")
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

	// EndeavourID managed via entity_relation
	if fields.EndeavourID != nil {
		updatedFields = append(updatedFields, "endeavour_id")
	}
	if fields.OwnerID != nil {
		if *fields.OwnerID == "" {
			setClauses = append(setClauses, "owner_id = NULL")
		} else {
			setClauses = append(setClauses, "owner_id = ?")
			params = append(params, *fields.OwnerID)
		}
		updatedFields = append(updatedFields, "owner_id")
	}

	if len(setClauses) == 0 && fields.EndeavourID == nil {
		return nil, fmt.Errorf("no fields to update")
	}

	if len(setClauses) > 0 {
		query := fmt.Sprintf("UPDATE demand SET %s WHERE id = ?", strings.Join(setClauses, ", "))
		params = append(params, id)

		result, err := db.Exec(query, params...)
		if err != nil {
			return nil, fmt.Errorf("update demand: %w", err)
		}

		rowsAffected, _ := result.RowsAffected()
		if rowsAffected == 0 {
			return nil, ErrDemandNotFound
		}
	} else {
		// Verify demand exists
		var exists int
		if err := db.QueryRow(`SELECT 1 FROM demand WHERE id = ?`, id).Scan(&exists); err != nil {
			return nil, ErrDemandNotFound
		}
	}

	// Update endeavour relation
	if fields.EndeavourID != nil {
		_ = db.SetRelation(RelBelongsTo, EntityDemand, id, EntityEndeavour, *fields.EndeavourID, "")
	}

	return updatedFields, nil
}

// DemandFulfillmentByEndeavour returns demand-level task progress for an endeavour,
// ordered by priority (urgent first). Tasks link to demands via FRM 'fulfills' relationship.
func (db *DB) DemandFulfillmentByEndeavour(endeavourID string) ([]*DemandFulfillmentRecord, error) {
	query := `
		SELECT
			d.id, d.title, d.type, d.priority, d.status,
			COUNT(t.id) AS task_total,
			SUM(CASE WHEN t.status = 'done' THEN 1 ELSE 0 END) AS task_done
		FROM demand d
		JOIN entity_relation rel_edv ON rel_edv.source_entity_id = d.id
			AND rel_edv.source_entity_type = 'demand'
			AND rel_edv.relationship_type = 'belongs_to'
			AND rel_edv.target_entity_type = 'endeavour'
			AND rel_edv.target_entity_id = ?
		LEFT JOIN entity_relation rel_tsk ON rel_tsk.target_entity_id = d.id
			AND rel_tsk.target_entity_type = 'demand'
			AND rel_tsk.source_entity_type = 'task'
			AND rel_tsk.relationship_type = 'fulfills'
		LEFT JOIN task t ON t.id = rel_tsk.source_entity_id
		GROUP BY d.id
		ORDER BY CASE d.priority
			WHEN 'urgent' THEN 1 WHEN 'high' THEN 2
			WHEN 'medium' THEN 3 WHEN 'low' THEN 4
		END ASC`

	rows, err := db.Query(query, endeavourID)
	if err != nil {
		return nil, fmt.Errorf("query demand fulfillment: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var records []*DemandFulfillmentRecord
	for rows.Next() {
		var r DemandFulfillmentRecord
		if err := rows.Scan(&r.ID, &r.Title, &r.Type, &r.Priority, &r.Status, &r.TaskTotal, &r.TaskDone); err != nil {
			return nil, fmt.Errorf("scan demand fulfillment row: %w", err)
		}
		records = append(records, &r)
	}
	return records, rows.Err()
}
