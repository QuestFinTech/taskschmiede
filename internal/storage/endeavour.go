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

// Goal represents a structured goal within an endeavour.
type Goal struct {
	ID               string `json:"id"`
	Title            string `json:"title"`
	Status           string `json:"status"` // open, achieved, abandoned
	LinkedEntityType string `json:"linked_entity_type,omitempty"`
	LinkedEntityID   string `json:"linked_entity_id,omitempty"`
}

// Endeavour represents a container for related work toward a goal.
type Endeavour struct {
	ID             string
	Name           string
	Description    string
	Goals          []Goal
	Status         string
	Timezone       string // IANA timezone (e.g., "Europe/Berlin"), defaults to "UTC"
	Lang           string // Language code (e.g., "en", "de"), defaults to "en"
	StartDate      *time.Time
	EndDate        *time.Time
	CompletedAt    *time.Time
	ArchivedAt     *time.Time
	ArchivedReason     string
	TaskschmiedEnabled bool // Opt-in: allow Taskschmied to execute rituals
	Metadata           map[string]interface{}
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// parseGoals unmarshals a JSON goals column, handling both the legacy format
// (array of strings) and the new structured format (array of Goal objects).
func parseGoals(data string) []Goal {
	if data == "" || data == "[]" || data == "null" {
		return nil
	}

	// Try new format first
	var goals []Goal
	if err := json.Unmarshal([]byte(data), &goals); err == nil && len(goals) > 0 && goals[0].Title != "" {
		return goals
	}

	// Fall back to legacy string array
	var strings []string
	if err := json.Unmarshal([]byte(data), &strings); err == nil {
		goals = make([]Goal, len(strings))
		for i, s := range strings {
			goals[i] = Goal{
				ID:     generateID("gol"),
				Title:  s,
				Status: "open",
			}
		}
		return goals
	}

	return nil
}

// ListEndeavoursOpts holds filters for listing endeavours.
type ListEndeavoursOpts struct {
	Status         string
	OrganizationID string
	EndeavourIDs   []string // RBAC: restrict to these endeavours (nil = no restriction, empty = none)
	Search         string
	Limit          int
	Offset         int
}

// TaskProgress holds task status counts for an endeavour.
type TaskProgress struct {
	Planned  int `json:"planned"`
	Active   int `json:"active"`
	Done     int `json:"done"`
	Canceled int `json:"canceled"`
}

// Endeavour error sentinels.
var (
	// ErrEndeavourNotFound is returned when an endeavour cannot be found by its ID.
	ErrEndeavourNotFound = errors.New("endeavour not found")
	// ErrUserAlreadyInEndeavour is returned when adding a user who already has access.
	ErrUserAlreadyInEndeavour = errors.New("user already has access to this endeavour")
)

// CreateEndeavour creates a new endeavour.
func (db *DB) CreateEndeavour(name, description string, goals []Goal, startDate, endDate *time.Time, metadata map[string]interface{}) (*Endeavour, error) {
	id := generateID("edv")

	// Assign IDs and default status to goals that lack them
	for i := range goals {
		if goals[i].ID == "" {
			goals[i].ID = generateID("gol")
		}
		if goals[i].Status == "" {
			goals[i].Status = "open"
		}
	}

	goalsJSON := "[]"
	if goals != nil {
		b, err := json.Marshal(goals)
		if err != nil {
			return nil, fmt.Errorf("marshal goals: %w", err)
		}
		goalsJSON = string(b)
	}

	metadataJSON := "{}"
	if metadata != nil {
		b, err := json.Marshal(metadata)
		if err != nil {
			return nil, fmt.Errorf("marshal metadata: %w", err)
		}
		metadataJSON = string(b)
	}

	var descVal, startVal, endVal *string
	if description != "" {
		descVal = &description
	}
	if startDate != nil {
		s := startDate.Format(time.RFC3339)
		startVal = &s
	}
	if endDate != nil {
		s := endDate.Format(time.RFC3339)
		endVal = &s
	}

	now := UTCNow()
	_, err := db.Exec(
		`INSERT INTO endeavour (id, name, description, goals, start_date, end_date, metadata, status)
		 VALUES (?, ?, ?, ?, ?, ?, ?, 'active')`,
		id, name, descVal, goalsJSON, startVal, endVal, metadataJSON,
	)
	if err != nil {
		return nil, fmt.Errorf("insert endeavour: %w", err)
	}

	return &Endeavour{
		ID:          id,
		Name:        name,
		Description: description,
		Goals:       goals,
		Status:      "active",
		Timezone:    "UTC",
		Lang:        "en",
		StartDate:   startDate,
		EndDate:     endDate,
		Metadata:    metadata,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

// GetEndeavour retrieves an endeavour by ID.
func (db *DB) GetEndeavour(id string) (*Endeavour, error) {
	var edv Endeavour
	var description, goalsJSON sql.NullString
	var startDate, endDate sql.NullString
	var completedAt, archivedAt, archivedReason sql.NullString
	var metadataJSON string
	var createdAt, updatedAt string

	var taskschmiedEnabled int
	err := db.QueryRow(
		`SELECT id, name, description, goals, status, timezone, lang, start_date, end_date,
		        completed_at, archived_at, archived_reason, taskschmied_enabled, metadata, created_at, updated_at
		 FROM endeavour WHERE id = ?`,
		id,
	).Scan(&edv.ID, &edv.Name, &description, &goalsJSON, &edv.Status, &edv.Timezone, &edv.Lang, &startDate, &endDate,
		&completedAt, &archivedAt, &archivedReason, &taskschmiedEnabled, &metadataJSON, &createdAt, &updatedAt)

	if err == sql.ErrNoRows {
		return nil, ErrEndeavourNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query endeavour: %w", err)
	}

	if description.Valid {
		edv.Description = description.String
	}
	if goalsJSON.Valid {
		edv.Goals = parseGoals(goalsJSON.String)
	}
	if startDate.Valid {
		t := ParseDBTime(startDate.String)
		edv.StartDate = &t
	}
	if endDate.Valid {
		t := ParseDBTime(endDate.String)
		edv.EndDate = &t
	}
	if completedAt.Valid {
		t := ParseDBTime(completedAt.String)
		edv.CompletedAt = &t
	}
	if archivedAt.Valid {
		t := ParseDBTime(archivedAt.String)
		edv.ArchivedAt = &t
	}
	if archivedReason.Valid {
		edv.ArchivedReason = archivedReason.String
	}
	edv.TaskschmiedEnabled = taskschmiedEnabled == 1
	_ = json.Unmarshal([]byte(metadataJSON), &edv.Metadata)
	edv.CreatedAt = ParseDBTime(createdAt)
	edv.UpdatedAt = ParseDBTime(updatedAt)

	return &edv, nil
}

// ListEndeavours queries endeavours with filters.
func (db *DB) ListEndeavours(opts ListEndeavoursOpts) ([]*Endeavour, int, error) {
	query := `SELECT e.id, e.name, e.description, e.goals, e.status, e.timezone, e.lang, e.start_date, e.end_date, e.completed_at, e.archived_at, e.archived_reason, e.taskschmied_enabled, e.metadata, e.created_at, e.updated_at FROM endeavour e`
	countQuery := `SELECT COUNT(*) FROM endeavour e`

	var conditions []string
	var params []interface{}

	if opts.OrganizationID != "" {
		query += ` JOIN entity_relation er ON e.id = er.target_entity_id
		    AND er.target_entity_type = 'endeavour'
		    AND er.source_entity_type = 'organization'
		    AND er.relationship_type = 'participates_in'`
		countQuery += ` JOIN entity_relation er ON e.id = er.target_entity_id
		    AND er.target_entity_type = 'endeavour'
		    AND er.source_entity_type = 'organization'
		    AND er.relationship_type = 'participates_in'`
		conditions = append(conditions, "er.source_entity_id = ?")
		params = append(params, opts.OrganizationID)
	}

	if opts.Status != "" {
		conditions = append(conditions, "e.status = ?")
		params = append(params, opts.Status)
	} else {
		// Exclude archived by default
		conditions = append(conditions, "e.status != 'archived'")
	}

	if opts.Search != "" {
		conditions = append(conditions, "(e.name LIKE ? ESCAPE '\\' OR e.description LIKE ? ESCAPE '\\')")
		searchParam := "%" + escapeLike(opts.Search, '\\') + "%"
		params = append(params, searchParam, searchParam)
	}

	if len(opts.EndeavourIDs) > 0 {
		placeholders := make([]string, len(opts.EndeavourIDs))
		for i := range opts.EndeavourIDs {
			placeholders[i] = "?"
			params = append(params, opts.EndeavourIDs[i])
		}
		inClause := strings.Join(placeholders, ", ")
		conditions = append(conditions, "e.id IN ("+inClause+")")
	} else if opts.EndeavourIDs != nil && len(opts.EndeavourIDs) == 0 {
		// Non-nil empty slice means user has no endeavour access
		conditions = append(conditions, "1=0")
	}

	if len(conditions) > 0 {
		where := " WHERE " + strings.Join(conditions, " AND ")
		query += where
		countQuery += where
	}

	// Count params mirror query params (before LIMIT/OFFSET)
	countParams := make([]interface{}, len(params))
	copy(countParams, params)

	var total int
	_ = db.QueryRow(countQuery, countParams...).Scan(&total)

	if opts.Limit <= 0 {
		opts.Limit = 50
	}
	query += ` ORDER BY e.created_at DESC LIMIT ? OFFSET ?`
	params = append(params, opts.Limit, opts.Offset)

	rows, err := db.Query(query, params...)
	if err != nil {
		return nil, 0, fmt.Errorf("query endeavours: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var edvs []*Endeavour
	for rows.Next() {
		var edv Endeavour
		var description, goalsJSON sql.NullString
		var startDate, endDate sql.NullString
		var completedAt, archivedAt, archivedReason sql.NullString
		var metadataJSON string
		var createdAt, updatedAt string

		var taskschmiedEnabled int
		if err := rows.Scan(&edv.ID, &edv.Name, &description, &goalsJSON, &edv.Status, &edv.Timezone, &edv.Lang, &startDate, &endDate, &completedAt, &archivedAt, &archivedReason, &taskschmiedEnabled, &metadataJSON, &createdAt, &updatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan endeavour: %w", err)
		}
		edv.TaskschmiedEnabled = taskschmiedEnabled == 1

		if description.Valid {
			edv.Description = description.String
		}
		if goalsJSON.Valid {
			edv.Goals = parseGoals(goalsJSON.String)
		}
		if startDate.Valid {
			t := ParseDBTime(startDate.String)
			edv.StartDate = &t
		}
		if endDate.Valid {
			t := ParseDBTime(endDate.String)
			edv.EndDate = &t
		}
		if completedAt.Valid {
			t := ParseDBTime(completedAt.String)
			edv.CompletedAt = &t
		}
		if archivedAt.Valid {
			t := ParseDBTime(archivedAt.String)
			edv.ArchivedAt = &t
		}
		if archivedReason.Valid {
			edv.ArchivedReason = archivedReason.String
		}
		_ = json.Unmarshal([]byte(metadataJSON), &edv.Metadata)
		edv.CreatedAt = ParseDBTime(createdAt)
		edv.UpdatedAt = ParseDBTime(updatedAt)

		edvs = append(edvs, &edv)
	}

	return edvs, total, nil
}

// GetEndeavourTaskProgress returns task status counts for an endeavour.
func (db *DB) GetEndeavourTaskProgress(endeavourID string) (*TaskProgress, error) {
	var progress TaskProgress

	rows, err := db.Query(
		`SELECT t.status, COUNT(*) FROM task t
		 JOIN entity_relation er ON er.source_entity_id = t.id
		     AND er.source_entity_type = 'task'
		     AND er.relationship_type = 'belongs_to'
		     AND er.target_entity_type = 'endeavour'
		 WHERE er.target_entity_id = ?
		 GROUP BY t.status`,
		endeavourID,
	)
	if err != nil {
		return &progress, fmt.Errorf("query task progress: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			continue
		}
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

	return &progress, nil
}

// UpdateEndeavourFields holds the fields to update on an endeavour.
// Only non-nil fields are applied.
type UpdateEndeavourFields struct {
	Name               *string
	Description        *string
	Status             *string
	Timezone           *string // IANA timezone (e.g., "Europe/Berlin")
	Lang               *string // Language code (e.g., "en", "de")
	ArchivedReason     *string // set when archiving
	Goals              []Goal  // nil = no change, empty = clear
	StartDate          *string // RFC3339 or empty to clear
	EndDate            *string // RFC3339 or empty to clear
	TaskschmiedEnabled *bool
	Metadata           map[string]interface{}
}

// UpdateEndeavour applies partial updates to an endeavour.
func (db *DB) UpdateEndeavour(id string, fields UpdateEndeavourFields) ([]string, error) {
	var setClauses []string
	var params []interface{}
	var updatedFields []string

	if fields.Name != nil {
		setClauses = append(setClauses, "name = ?")
		params = append(params, *fields.Name)
		updatedFields = append(updatedFields, "name")
	}
	if fields.Description != nil {
		setClauses = append(setClauses, "description = ?")
		params = append(params, *fields.Description)
		updatedFields = append(updatedFields, "description")
	}
	if fields.Timezone != nil {
		setClauses = append(setClauses, "timezone = ?")
		params = append(params, *fields.Timezone)
		updatedFields = append(updatedFields, "timezone")
	}
	if fields.Lang != nil {
		setClauses = append(setClauses, "lang = ?")
		params = append(params, *fields.Lang)
		updatedFields = append(updatedFields, "lang")
	}
	if fields.Status != nil {
		setClauses = append(setClauses, "status = ?")
		params = append(params, *fields.Status)
		updatedFields = append(updatedFields, "status")

		now := UTCNow().Format(time.RFC3339)
		switch *fields.Status {
		case "completed":
			setClauses = append(setClauses, "completed_at = ?")
			params = append(params, now)
		case "archived":
			setClauses = append(setClauses, "archived_at = ?")
			params = append(params, now)
			if fields.ArchivedReason != nil {
				setClauses = append(setClauses, "archived_reason = ?")
				params = append(params, *fields.ArchivedReason)
			}
		}
	}
	if fields.Goals != nil {
		for i := range fields.Goals {
			if fields.Goals[i].ID == "" {
				fields.Goals[i].ID = generateID("gol")
			}
			if fields.Goals[i].Status == "" {
				fields.Goals[i].Status = "open"
			}
		}
		b, err := json.Marshal(fields.Goals)
		if err != nil {
			return nil, fmt.Errorf("marshal goals: %w", err)
		}
		setClauses = append(setClauses, "goals = ?")
		params = append(params, string(b))
		updatedFields = append(updatedFields, "goals")
	}
	if fields.StartDate != nil {
		if *fields.StartDate == "" {
			setClauses = append(setClauses, "start_date = NULL")
		} else {
			setClauses = append(setClauses, "start_date = ?")
			params = append(params, *fields.StartDate)
		}
		updatedFields = append(updatedFields, "start_date")
	}
	if fields.EndDate != nil {
		if *fields.EndDate == "" {
			setClauses = append(setClauses, "end_date = NULL")
		} else {
			setClauses = append(setClauses, "end_date = ?")
			params = append(params, *fields.EndDate)
		}
		updatedFields = append(updatedFields, "end_date")
	}
	if fields.TaskschmiedEnabled != nil {
		val := 0
		if *fields.TaskschmiedEnabled {
			val = 1
		}
		setClauses = append(setClauses, "taskschmied_enabled = ?")
		params = append(params, val)
		updatedFields = append(updatedFields, "taskschmied_enabled")
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

	if len(setClauses) == 0 {
		return nil, fmt.Errorf("no fields to update")
	}

	// Always update the timestamp
	setClauses = append(setClauses, "updated_at = ?")
	params = append(params, UTCNow().Format(time.RFC3339))

	query := fmt.Sprintf("UPDATE endeavour SET %s WHERE id = ?", strings.Join(setClauses, ", "))
	params = append(params, id)

	result, err := db.Exec(query, params...)
	if err != nil {
		return nil, fmt.Errorf("update endeavour: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return nil, ErrEndeavourNotFound
	}

	return updatedFields, nil
}

// CheckAutoComplete returns true when an endeavour is eligible for automatic
// completion: status is active, metadata has auto_complete=true, all tasks are
// terminal (done/canceled), all demands are fulfilled/canceled, and all goals
// are achieved/abandoned (no open).
func (db *DB) CheckAutoComplete(endeavourID string) (bool, error) {
	// 1. Check endeavour status is active and auto_complete is enabled.
	var status, metadataJSON string
	var goalsJSON sql.NullString
	err := db.QueryRow(
		`SELECT status, metadata, goals FROM endeavour WHERE id = ?`,
		endeavourID,
	).Scan(&status, &metadataJSON, &goalsJSON)
	if err != nil {
		return false, fmt.Errorf("check auto-complete: %w", err)
	}
	if status != "active" {
		return false, nil
	}

	var metadata map[string]interface{}
	_ = json.Unmarshal([]byte(metadataJSON), &metadata)
	autoComplete, _ := metadata["auto_complete"].(bool)
	if !autoComplete {
		return false, nil
	}

	// 2. Check all goals are achieved or abandoned (no open).
	goals := parseGoals(goalsJSON.String)
	for _, g := range goals {
		if g.Status == "open" {
			return false, nil
		}
	}

	// 3. Check all tasks are terminal (done/canceled).
	progress, err := db.GetEndeavourTaskProgress(endeavourID)
	if err != nil {
		return false, err
	}
	if progress.Planned > 0 || progress.Active > 0 {
		return false, nil
	}

	// 4. Check all demands are fulfilled or canceled.
	var openDemands int
	err = db.QueryRow(
		`SELECT COUNT(*) FROM demand d
		 JOIN entity_relation er ON er.source_entity_id = d.id
		     AND er.source_entity_type = 'demand'
		     AND er.relationship_type = 'belongs_to'
		     AND er.target_entity_type = 'endeavour'
		 WHERE er.target_entity_id = ?
		   AND d.status NOT IN ('fulfilled', 'canceled')`,
		endeavourID,
	).Scan(&openDemands)
	if err != nil {
		return false, fmt.Errorf("check demands: %w", err)
	}
	if openDemands > 0 {
		return false, nil
	}

	return true, nil
}

// AddUserToEndeavour grants a user access to an endeavour.
func (db *DB) AddUserToEndeavour(userID, endeavourID, role string) error {
	if role == "" {
		role = "member"
	}

	metadata := map[string]interface{}{"role": role}
	_, err := db.CreateRelation(RelMemberOf, EntityUser, userID, EntityEndeavour, endeavourID, metadata, "")
	if err != nil {
		if errors.Is(err, ErrRelationExists) {
			return ErrUserAlreadyInEndeavour
		}
		return fmt.Errorf("add user to endeavour: %w", err)
	}
	return nil
}
