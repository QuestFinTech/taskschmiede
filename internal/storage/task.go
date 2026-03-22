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
	"math"
	"strings"
	"time"

)

// Task represents an atomic unit of work.
type Task struct {
	ID             string
	Title          string
	Description    string
	Status         string
	EndeavourID    string
	EndeavourName  string // denormalized for convenience
	DemandID       string
	AssigneeID     string
	AssigneeName   string // denormalized for convenience
	CreatorID      string
	CreatorName    string // denormalized for convenience
	OwnerID        string
	OwnerName      string // denormalized for convenience
	Estimate       *float64
	Actual         *float64
	DueDate        *time.Time
	Metadata       map[string]interface{}
	CreatedAt      time.Time
	UpdatedAt      time.Time
	StartedAt      *time.Time
	CompletedAt    *time.Time
	CanceledAt     *time.Time
	CanceledReason string
}

// ListTasksOpts holds filters for listing tasks.
type ListTasksOpts struct {
	Status       string
	EndeavourID  string
	EndeavourIDs []string // RBAC: nil = no restriction; empty = no access
	AssigneeID   string
	Unassigned   bool // Filter tasks with no assignee
	DemandID     string
	Search       string
	Limit        int
	Offset       int
}

// UpdateTaskFields holds the fields to update on a task.
// Only non-nil fields are applied.
type UpdateTaskFields struct {
	Title          *string
	Description    *string
	Status         *string
	EndeavourID    *string
	DemandID       *string
	AssigneeID     *string
	OwnerID        *string
	Estimate       *float64
	Actual         *float64
	DueDate        *string // RFC3339 or empty to clear
	Metadata       map[string]interface{}
	CanceledReason *string
}

// ErrTaskNotFound is returned when a task cannot be found by its ID.
var ErrTaskNotFound = errors.New("task not found")

// CreateTask creates a new task. Relationships (endeavour, demand, assignee) are
// stored in entity_relation rather than direct FK columns.
func (db *DB) CreateTask(title, description, endeavourID, demandID, assigneeID, creatorID string, estimate *float64, dueDate *time.Time, metadata map[string]interface{}) (*Task, error) {
	id := generateID("tsk")

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
		`INSERT INTO task (id, title, description, estimate, due_date, metadata, status, creator_id)
		 VALUES (?, ?, ?, ?, ?, ?, 'planned', ?)`,
		id, title, descVal, estimate, dueDateStr, metadataJSON, creatorVal,
	)
	if err != nil {
		return nil, fmt.Errorf("insert task: %w", err)
	}

	// Create entity_relations for the relationship fields
	nowStr := now.Format(time.RFC3339)
	if endeavourID != "" {
		relID := generateID("rel")
		_, _ = db.Exec(
			`INSERT INTO entity_relation (id, relationship_type, source_entity_type, source_entity_id,
			 target_entity_type, target_entity_id, created_at)
			 VALUES (?, ?, 'task', ?, 'endeavour', ?, ?)`,
			relID, RelBelongsTo, id, endeavourID, nowStr,
		)
	}
	if demandID != "" {
		relID := generateID("rel")
		_, _ = db.Exec(
			`INSERT INTO entity_relation (id, relationship_type, source_entity_type, source_entity_id,
			 target_entity_type, target_entity_id, created_at)
			 VALUES (?, ?, 'task', ?, 'demand', ?, ?)`,
			relID, RelFulfills, id, demandID, nowStr,
		)
	}
	if assigneeID != "" {
		relID := generateID("rel")
		_, _ = db.Exec(
			`INSERT INTO entity_relation (id, relationship_type, source_entity_type, source_entity_id,
			 target_entity_type, target_entity_id, created_at)
			 VALUES (?, ?, 'task', ?, 'resource', ?, ?)`,
			relID, RelAssignedTo, id, assigneeID, nowStr,
		)
	}

	return &Task{
		ID:          id,
		Title:       title,
		Description: description,
		Status:      "planned",
		EndeavourID: endeavourID,
		DemandID:    demandID,
		AssigneeID:  assigneeID,
		CreatorID:   creatorID,
		Estimate:    estimate,
		DueDate:     dueDate,
		Metadata:    metadata,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

// GetTask retrieves a task by ID, including relationships from entity_relation.
func (db *DB) GetTask(id string) (*Task, error) {
	var task Task
	var description sql.NullString
	var assigneeName sql.NullString
	var endeavourName sql.NullString
	var creatorID, creatorName sql.NullString
	var ownerID, ownerName sql.NullString
	var estimate, actual sql.NullFloat64
	var dueDate, startedAt, completedAt, canceledAt sql.NullString
	var canceledReason sql.NullString
	var metadataJSON string
	var createdAt, updatedAt string
	var endeavourID, demandID, assigneeID sql.NullString

	err := db.QueryRow(
		`SELECT t.id, t.title, t.description, t.status,
		        rel_edv.target_entity_id,
		        e.name,
		        rel_dmd.target_entity_id,
		        rel_asg.target_entity_id,
		        r.name,
		        t.creator_id, rc.name,
		        t.owner_id, ro.name,
		        t.estimate, t.actual, t.due_date,
		        t.metadata, t.created_at, t.updated_at, t.started_at, t.completed_at,
		        t.canceled_at, t.canceled_reason
		 FROM task t
		 LEFT JOIN entity_relation rel_edv ON rel_edv.source_entity_id = t.id
		     AND rel_edv.source_entity_type = 'task'
		     AND rel_edv.relationship_type = 'belongs_to'
		     AND rel_edv.target_entity_type = 'endeavour'
		 LEFT JOIN endeavour e ON rel_edv.target_entity_id = e.id
		 LEFT JOIN entity_relation rel_dmd ON rel_dmd.source_entity_id = t.id
		     AND rel_dmd.source_entity_type = 'task'
		     AND rel_dmd.relationship_type = 'fulfills'
		     AND rel_dmd.target_entity_type = 'demand'
		 LEFT JOIN entity_relation rel_asg ON rel_asg.source_entity_id = t.id
		     AND rel_asg.source_entity_type = 'task'
		     AND rel_asg.relationship_type = 'assigned_to'
		     AND rel_asg.target_entity_type = 'resource'
		 LEFT JOIN resource r ON rel_asg.target_entity_id = r.id
		 LEFT JOIN resource rc ON t.creator_id = rc.id
		 LEFT JOIN resource ro ON t.owner_id = ro.id
		 WHERE t.id = ?`,
		id,
	).Scan(&task.ID, &task.Title, &description, &task.Status,
		&endeavourID, &endeavourName, &demandID, &assigneeID, &assigneeName,
		&creatorID, &creatorName, &ownerID, &ownerName,
		&estimate, &actual, &dueDate,
		&metadataJSON, &createdAt, &updatedAt, &startedAt, &completedAt,
		&canceledAt, &canceledReason)

	if err == sql.ErrNoRows {
		return nil, ErrTaskNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query task: %w", err)
	}

	if description.Valid {
		task.Description = description.String
	}
	if endeavourID.Valid {
		task.EndeavourID = endeavourID.String
	}
	if endeavourName.Valid {
		task.EndeavourName = endeavourName.String
	}
	if demandID.Valid {
		task.DemandID = demandID.String
	}
	if assigneeID.Valid {
		task.AssigneeID = assigneeID.String
	}
	if assigneeName.Valid {
		task.AssigneeName = assigneeName.String
	}
	if creatorID.Valid {
		task.CreatorID = creatorID.String
	}
	if creatorName.Valid {
		task.CreatorName = creatorName.String
	}
	if ownerID.Valid {
		task.OwnerID = ownerID.String
	}
	if ownerName.Valid {
		task.OwnerName = ownerName.String
	}
	if estimate.Valid {
		task.Estimate = &estimate.Float64
	}
	if actual.Valid {
		task.Actual = &actual.Float64
	}
	if dueDate.Valid {
		t := ParseDBTime(dueDate.String)
		task.DueDate = &t
	}
	if startedAt.Valid {
		t := ParseDBTime(startedAt.String)
		task.StartedAt = &t
	}
	if completedAt.Valid {
		t := ParseDBTime(completedAt.String)
		task.CompletedAt = &t
	}
	if canceledAt.Valid {
		t := ParseDBTime(canceledAt.String)
		task.CanceledAt = &t
	}
	if canceledReason.Valid {
		task.CanceledReason = canceledReason.String
	}
	_ = json.Unmarshal([]byte(metadataJSON), &task.Metadata)
	task.CreatedAt = ParseDBTime(createdAt)
	task.UpdatedAt = ParseDBTime(updatedAt)

	return &task, nil
}

// ListTasks queries tasks with filters. Relationships resolved via entity_relation.
func (db *DB) ListTasks(opts ListTasksOpts) ([]*Task, int, error) {
	// Empty EndeavourIDs = user has no endeavour access, return nothing.
	if opts.EndeavourIDs != nil && len(opts.EndeavourIDs) == 0 {
		return nil, 0, nil
	}
	query := `SELECT t.id, t.title, t.description, t.status,
	                 rel_edv.target_entity_id,
	                 e.name,
	                 rel_dmd.target_entity_id,
	                 rel_asg.target_entity_id,
	                 r.name,
	                 t.creator_id, rc.name,
	                 t.owner_id, ro.name,
	                 t.estimate, t.actual, t.due_date,
	                 t.metadata, t.created_at, t.updated_at, t.started_at, t.completed_at,
	                 t.canceled_at, t.canceled_reason
	          FROM task t
	          LEFT JOIN entity_relation rel_edv ON rel_edv.source_entity_id = t.id
	              AND rel_edv.source_entity_type = 'task'
	              AND rel_edv.relationship_type = 'belongs_to'
	              AND rel_edv.target_entity_type = 'endeavour'
	          LEFT JOIN endeavour e ON rel_edv.target_entity_id = e.id
	          LEFT JOIN entity_relation rel_dmd ON rel_dmd.source_entity_id = t.id
	              AND rel_dmd.source_entity_type = 'task'
	              AND rel_dmd.relationship_type = 'fulfills'
	              AND rel_dmd.target_entity_type = 'demand'
	          LEFT JOIN entity_relation rel_asg ON rel_asg.source_entity_id = t.id
	              AND rel_asg.source_entity_type = 'task'
	              AND rel_asg.relationship_type = 'assigned_to'
	              AND rel_asg.target_entity_type = 'resource'
	          LEFT JOIN resource r ON rel_asg.target_entity_id = r.id
	          LEFT JOIN resource rc ON t.creator_id = rc.id
	          LEFT JOIN resource ro ON t.owner_id = ro.id`
	countQuery := `SELECT COUNT(*) FROM task t`

	var conditions []string
	var countConditions []string
	var params []interface{}
	var countParams []interface{}

	if opts.Status != "" {
		conditions = append(conditions, "t.status = ?")
		countConditions = append(countConditions, "t.status = ?")
		params = append(params, opts.Status)
		countParams = append(countParams, opts.Status)
	}

	if opts.EndeavourID != "" {
		conditions = append(conditions, "rel_edv.target_entity_id = ?")
		params = append(params, opts.EndeavourID)
		// Count query needs its own JOIN
		countQuery += ` JOIN entity_relation cr_edv ON cr_edv.source_entity_id = t.id
		    AND cr_edv.source_entity_type = 'task'
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
		countQuery += ` LEFT JOIN entity_relation cr_edv2 ON cr_edv2.source_entity_id = t.id
		    AND cr_edv2.source_entity_type = 'task'
		    AND cr_edv2.relationship_type = 'belongs_to'
		    AND cr_edv2.target_entity_type = 'endeavour'`
		countConditions = append(countConditions, "(cr_edv2.target_entity_id IN ("+inClause+") OR cr_edv2.target_entity_id IS NULL)")
	}

	if opts.Unassigned {
		conditions = append(conditions, "rel_asg.target_entity_id IS NULL")
		countQuery += ` LEFT JOIN entity_relation cr_unasg ON cr_unasg.source_entity_id = t.id
		    AND cr_unasg.source_entity_type = 'task'
		    AND cr_unasg.relationship_type = 'assigned_to'
		    AND cr_unasg.target_entity_type = 'resource'`
		countConditions = append(countConditions, "cr_unasg.target_entity_id IS NULL")
	}

	if opts.AssigneeID != "" {
		conditions = append(conditions, "rel_asg.target_entity_id = ?")
		params = append(params, opts.AssigneeID)
		countQuery += ` JOIN entity_relation cr_asg ON cr_asg.source_entity_id = t.id
		    AND cr_asg.source_entity_type = 'task'
		    AND cr_asg.relationship_type = 'assigned_to'
		    AND cr_asg.target_entity_type = 'resource'`
		countConditions = append(countConditions, "cr_asg.target_entity_id = ?")
		countParams = append(countParams, opts.AssigneeID)
	}

	if opts.DemandID != "" {
		conditions = append(conditions, "rel_dmd.target_entity_id = ?")
		params = append(params, opts.DemandID)
		countQuery += ` JOIN entity_relation cr_dmd ON cr_dmd.source_entity_id = t.id
		    AND cr_dmd.source_entity_type = 'task'
		    AND cr_dmd.relationship_type = 'fulfills'
		    AND cr_dmd.target_entity_type = 'demand'`
		countConditions = append(countConditions, "cr_dmd.target_entity_id = ?")
		countParams = append(countParams, opts.DemandID)
	}

	if opts.Search != "" {
		conditions = append(conditions, "(t.title LIKE ? ESCAPE '\\' OR t.description LIKE ? ESCAPE '\\')")
		countConditions = append(countConditions, "(t.title LIKE ? ESCAPE '\\' OR t.description LIKE ? ESCAPE '\\')")
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
	_ = db.QueryRow(countQuery, countParams...).Scan(&total)

	if opts.Limit <= 0 {
		opts.Limit = 50
	}
	query += ` ORDER BY t.created_at DESC LIMIT ? OFFSET ?`
	params = append(params, opts.Limit, opts.Offset)

	rows, err := db.Query(query, params...)
	if err != nil {
		return nil, 0, fmt.Errorf("query tasks: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var tasks []*Task
	for rows.Next() {
		var task Task
		var description sql.NullString
		var endeavourID, demandID, assigneeID sql.NullString
		var endeavourName, assigneeName sql.NullString
		var creatorID, creatorName, ownerID, ownerName sql.NullString
		var estimate, actual sql.NullFloat64
		var dueDate, startedAt, completedAt, canceledAt sql.NullString
		var canceledReason sql.NullString
		var metadataJSON string
		var createdAt, updatedAt string

		if err := rows.Scan(&task.ID, &task.Title, &description, &task.Status,
			&endeavourID, &endeavourName, &demandID, &assigneeID, &assigneeName,
			&creatorID, &creatorName, &ownerID, &ownerName,
			&estimate, &actual, &dueDate,
			&metadataJSON, &createdAt, &updatedAt, &startedAt, &completedAt,
			&canceledAt, &canceledReason); err != nil {
			return nil, 0, fmt.Errorf("scan task: %w", err)
		}

		if description.Valid {
			task.Description = description.String
		}
		if endeavourID.Valid {
			task.EndeavourID = endeavourID.String
		}
		if endeavourName.Valid {
			task.EndeavourName = endeavourName.String
		}
		if demandID.Valid {
			task.DemandID = demandID.String
		}
		if assigneeID.Valid {
			task.AssigneeID = assigneeID.String
		}
		if assigneeName.Valid {
			task.AssigneeName = assigneeName.String
		}
		if creatorID.Valid {
			task.CreatorID = creatorID.String
		}
		if creatorName.Valid {
			task.CreatorName = creatorName.String
		}
		if ownerID.Valid {
			task.OwnerID = ownerID.String
		}
		if ownerName.Valid {
			task.OwnerName = ownerName.String
		}
		if estimate.Valid {
			task.Estimate = &estimate.Float64
		}
		if actual.Valid {
			task.Actual = &actual.Float64
		}
		if dueDate.Valid {
			t := ParseDBTime(dueDate.String)
			task.DueDate = &t
		}
		if startedAt.Valid {
			t := ParseDBTime(startedAt.String)
			task.StartedAt = &t
		}
		if completedAt.Valid {
			t := ParseDBTime(completedAt.String)
			task.CompletedAt = &t
		}
		if canceledAt.Valid {
			t := ParseDBTime(canceledAt.String)
			task.CanceledAt = &t
		}
		if canceledReason.Valid {
			task.CanceledReason = canceledReason.String
		}
		_ = json.Unmarshal([]byte(metadataJSON), &task.Metadata)
		task.CreatedAt = ParseDBTime(createdAt)
		task.UpdatedAt = ParseDBTime(updatedAt)

		tasks = append(tasks, &task)
	}

	return tasks, total, nil
}

// TaskStatusCounts returns task counts grouped by status, applying the same
// filters as ListTasks (except limit/offset which don't apply to aggregation).
func (db *DB) TaskStatusCounts(opts ListTasksOpts) (*TaskProgress, int, error) {
	if opts.EndeavourIDs != nil && len(opts.EndeavourIDs) == 0 {
		return &TaskProgress{}, 0, nil
	}
	query := `SELECT t.status, COUNT(*) FROM task t`

	var conditions []string
	var params []interface{}

	if opts.Status != "" {
		conditions = append(conditions, "t.status = ?")
		params = append(params, opts.Status)
	}
	if opts.EndeavourID != "" {
		query += ` JOIN entity_relation cr_edv ON cr_edv.source_entity_id = t.id
		    AND cr_edv.source_entity_type = 'task'
		    AND cr_edv.relationship_type = 'belongs_to'
		    AND cr_edv.target_entity_type = 'endeavour'`
		conditions = append(conditions, "cr_edv.target_entity_id = ?")
		params = append(params, opts.EndeavourID)
	}
	if opts.AssigneeID != "" {
		query += ` JOIN entity_relation cr_asg ON cr_asg.source_entity_id = t.id
		    AND cr_asg.source_entity_type = 'task'
		    AND cr_asg.relationship_type = 'assigned_to'
		    AND cr_asg.target_entity_type = 'resource'`
		conditions = append(conditions, "cr_asg.target_entity_id = ?")
		params = append(params, opts.AssigneeID)
	}
	if opts.DemandID != "" {
		query += ` JOIN entity_relation cr_dmd ON cr_dmd.source_entity_id = t.id
		    AND cr_dmd.source_entity_type = 'task'
		    AND cr_dmd.relationship_type = 'fulfills'
		    AND cr_dmd.target_entity_type = 'demand'`
		conditions = append(conditions, "cr_dmd.target_entity_id = ?")
		params = append(params, opts.DemandID)
	}
	if opts.Search != "" {
		conditions = append(conditions, "(t.title LIKE ? ESCAPE '\\' OR t.description LIKE ? ESCAPE '\\')")
		searchParam := "%" + escapeLike(opts.Search, '\\') + "%"
		params = append(params, searchParam, searchParam)
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " GROUP BY t.status"

	rows, err := db.Query(query, params...)
	if err != nil {
		return nil, 0, fmt.Errorf("query task status counts: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var progress TaskProgress
	total := 0
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			continue
		}
		total += count
		switch status {
		case "planned":
			progress.Planned = count
		case "active":
			progress.Active = count
		case "done":
			progress.Done = count
		case "canceled":
			progress.Canceled = count
		}
	}

	return &progress, total, nil
}

// OverdueTasks returns tasks with a past due date that are not done or canceled,
// scoped to an endeavour via FRM join. Limit 20, ordered by due_date ASC.
func (db *DB) OverdueTasks(endeavourID string) ([]*Task, error) {
	now := UTCNow().Format(time.RFC3339)
	query := `
		SELECT t.id, t.title, t.status, t.due_date, t.updated_at,
		       COALESCE(r.name, rel_asg.target_entity_id, '') AS assignee_name
		FROM task t
		JOIN entity_relation rel_edv ON rel_edv.source_entity_id = t.id
			AND rel_edv.source_entity_type = 'task'
			AND rel_edv.relationship_type = 'belongs_to'
			AND rel_edv.target_entity_type = 'endeavour'
			AND rel_edv.target_entity_id = ?
		LEFT JOIN entity_relation rel_asg ON rel_asg.source_entity_id = t.id
			AND rel_asg.source_entity_type = 'task'
			AND rel_asg.relationship_type = 'assigned_to'
			AND rel_asg.target_entity_type = 'resource'
		LEFT JOIN resource r ON rel_asg.target_entity_id = r.id
		WHERE t.due_date < ? AND t.status NOT IN ('done', 'canceled')
		ORDER BY t.due_date ASC
		LIMIT 20`

	rows, err := db.Query(query, endeavourID, now)
	if err != nil {
		return nil, fmt.Errorf("query overdue tasks: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var tasks []*Task
	for rows.Next() {
		var t Task
		var dueDateStr, updatedAtStr, assigneeName sql.NullString
		if err := rows.Scan(&t.ID, &t.Title, &t.Status, &dueDateStr, &updatedAtStr, &assigneeName); err != nil {
			return nil, fmt.Errorf("scan overdue task row: %w", err)
		}
		if dueDateStr.Valid && dueDateStr.String != "" {
			dd := ParseDBTime(dueDateStr.String)
			t.DueDate = &dd
		}
		if updatedAtStr.Valid && updatedAtStr.String != "" {
			t.UpdatedAt = ParseDBTime(updatedAtStr.String)
		}
		if assigneeName.Valid {
			t.AssigneeName = assigneeName.String
		}
		tasks = append(tasks, &t)
	}
	return tasks, rows.Err()
}

// StaleTasks returns active tasks with no updates in the given number of days,
// scoped to an endeavour via FRM join. Limit 20, ordered by updated_at ASC (most stale first).
func (db *DB) StaleTasks(endeavourID string, staleDays int) ([]*Task, error) {
	cutoff := UTCNow().AddDate(0, 0, -staleDays).Format(time.RFC3339)
	query := `
		SELECT t.id, t.title, t.status, t.due_date, t.updated_at,
		       COALESCE(r.name, rel_asg.target_entity_id, '') AS assignee_name
		FROM task t
		JOIN entity_relation rel_edv ON rel_edv.source_entity_id = t.id
			AND rel_edv.source_entity_type = 'task'
			AND rel_edv.relationship_type = 'belongs_to'
			AND rel_edv.target_entity_type = 'endeavour'
			AND rel_edv.target_entity_id = ?
		LEFT JOIN entity_relation rel_asg ON rel_asg.source_entity_id = t.id
			AND rel_asg.source_entity_type = 'task'
			AND rel_asg.relationship_type = 'assigned_to'
			AND rel_asg.target_entity_type = 'resource'
		LEFT JOIN resource r ON rel_asg.target_entity_id = r.id
		WHERE t.status = 'active' AND t.updated_at < ?
		ORDER BY t.updated_at ASC
		LIMIT 20`

	rows, err := db.Query(query, endeavourID, cutoff)
	if err != nil {
		return nil, fmt.Errorf("query stale tasks: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var tasks []*Task
	for rows.Next() {
		var t Task
		var dueDateStr, updatedAtStr, assigneeName sql.NullString
		if err := rows.Scan(&t.ID, &t.Title, &t.Status, &dueDateStr, &updatedAtStr, &assigneeName); err != nil {
			return nil, fmt.Errorf("scan stale task row: %w", err)
		}
		if dueDateStr.Valid && dueDateStr.String != "" {
			dd := ParseDBTime(dueDateStr.String)
			t.DueDate = &dd
		}
		if updatedAtStr.Valid && updatedAtStr.String != "" {
			t.UpdatedAt = ParseDBTime(updatedAtStr.String)
		}
		if assigneeName.Valid {
			t.AssigneeName = assigneeName.String
		}
		tasks = append(tasks, &t)
	}
	return tasks, rows.Err()
}

// AvgCycleTimeByEndeavour returns the average cycle time in hours for done tasks
// in an endeavour. Cycle time is started_at -> completed_at. Returns 0 if no data.
func (db *DB) AvgCycleTimeByEndeavour(endeavourID string) (float64, error) {
	query := `
		SELECT AVG((julianday(t.completed_at) - julianday(t.started_at)) * 24)
		FROM task t
		JOIN entity_relation rel_edv ON rel_edv.source_entity_id = t.id
			AND rel_edv.source_entity_type = 'task'
			AND rel_edv.relationship_type = 'belongs_to'
			AND rel_edv.target_entity_type = 'endeavour'
			AND rel_edv.target_entity_id = ?
		WHERE t.status = 'done'
			AND t.started_at IS NOT NULL AND t.started_at != ''
			AND t.completed_at IS NOT NULL AND t.completed_at != ''`

	var avg sql.NullFloat64
	if err := db.QueryRow(query, endeavourID).Scan(&avg); err != nil {
		return 0, fmt.Errorf("query avg cycle time: %w", err)
	}
	if !avg.Valid {
		return 0, nil
	}
	return math.Round(avg.Float64*10) / 10, nil
}

// BulkCancelTasksByEndeavour cancels all planned/active tasks linked to an endeavour.
// Returns the count of affected rows.
func (db *DB) BulkCancelTasksByEndeavour(endeavourID, reason string) (int, error) {
	now := UTCNow().Format(time.RFC3339)
	result, err := db.Exec(
		`UPDATE task SET status = 'canceled', canceled_at = ?, canceled_reason = ?, updated_at = ?
		 WHERE id IN (
		     SELECT er.source_entity_id FROM entity_relation er
		     WHERE er.relationship_type = ? AND er.source_entity_type = 'task'
		       AND er.target_entity_type = 'endeavour' AND er.target_entity_id = ?
		 ) AND status IN ('planned', 'active')`,
		now, reason, now, RelBelongsTo, endeavourID,
	)
	if err != nil {
		return 0, fmt.Errorf("bulk cancel tasks: %w", err)
	}
	count, _ := result.RowsAffected()
	return int(count), nil
}

// UpdateTask applies partial updates to a task. Returns the list of updated field names.
func (db *DB) UpdateTask(id string, fields UpdateTaskFields) ([]string, error) {
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
	if fields.Status != nil {
		setClauses = append(setClauses, "status = ?")
		params = append(params, *fields.Status)
		updatedFields = append(updatedFields, "status")

		now := UTCNow().Format(time.RFC3339)
		switch *fields.Status {
		case "active":
			setClauses = append(setClauses, "started_at = COALESCE(started_at, ?)")
			params = append(params, now)
		case "done":
			setClauses = append(setClauses, "completed_at = ?")
			params = append(params, now)
			// Auto-compute actual hours from started_at if not explicitly provided
			if fields.Actual == nil {
				var startedAtStr *string
				_ = db.QueryRow(`SELECT started_at FROM task WHERE id = ?`, id).Scan(&startedAtStr)
				if startedAtStr != nil {
					startedAt := ParseDBTime(*startedAtStr)
					if !startedAt.IsZero() {
						completedAt, _ := time.Parse(time.RFC3339, now)
						hours := math.Round(completedAt.Sub(startedAt).Hours()*10) / 10
						setClauses = append(setClauses, "actual = ?")
						params = append(params, hours)
					}
				}
			}
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
	// EndeavourID, DemandID, and AssigneeID are managed via entity_relation (handled after SQL update)
	if fields.EndeavourID != nil {
		updatedFields = append(updatedFields, "endeavour_id")
	}
	if fields.DemandID != nil {
		updatedFields = append(updatedFields, "demand_id")
	}
	if fields.AssigneeID != nil {
		updatedFields = append(updatedFields, "assignee_id")
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
	if fields.Estimate != nil {
		setClauses = append(setClauses, "estimate = ?")
		params = append(params, *fields.Estimate)
		updatedFields = append(updatedFields, "estimate")
	}
	if fields.Actual != nil {
		setClauses = append(setClauses, "actual = ?")
		params = append(params, *fields.Actual)
		updatedFields = append(updatedFields, "actual")
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

	// If only relation fields changed (no SQL columns), we still need to verify the task exists
	if len(setClauses) == 0 {
		// Check if we have relation-only updates
		if fields.EndeavourID == nil && fields.DemandID == nil && fields.AssigneeID == nil && fields.OwnerID == nil {
			return nil, fmt.Errorf("no fields to update")
		}
		// Verify task exists
		var exists int
		if err := db.QueryRow(`SELECT 1 FROM task WHERE id = ?`, id).Scan(&exists); err != nil {
			return nil, ErrTaskNotFound
		}
	} else {
		query := fmt.Sprintf("UPDATE task SET %s WHERE id = ?", strings.Join(setClauses, ", "))
		params = append(params, id)

		result, err := db.Exec(query, params...)
		if err != nil {
			return nil, fmt.Errorf("update task: %w", err)
		}

		rowsAffected, _ := result.RowsAffected()
		if rowsAffected == 0 {
			return nil, ErrTaskNotFound
		}
	}

	// Update entity_relations for endeavour, demand, and assignee
	if fields.EndeavourID != nil {
		_ = db.SetRelation(RelBelongsTo, EntityTask, id, EntityEndeavour, *fields.EndeavourID, "")
	}
	if fields.DemandID != nil {
		_ = db.SetRelation(RelFulfills, EntityTask, id, EntityDemand, *fields.DemandID, "")
	}
	if fields.AssigneeID != nil {
		_ = db.SetRelation(RelAssignedTo, EntityTask, id, EntityResource, *fields.AssigneeID, "")
	}

	return updatedFields, nil
}
