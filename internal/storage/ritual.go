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

// Ritual represents a stored methodology prompt (BYOM core).
type Ritual struct {
	ID            string
	Name          string
	Description   string
	Prompt        string
	Version       int
	PredecessorID string
	Origin        string
	MethodologyID string // FK to methodology table; empty = agnostic
	Schedule      map[string]interface{}
	IsEnabled     bool
	Lang          string
	Metadata      map[string]interface{}
	CreatedBy     string
	Status        string
	EndeavourID   string // populated from entity_relation (governs)
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// ListRitualsOpts holds filters for listing rituals.
type ListRitualsOpts struct {
	EndeavourID   string
	EndeavourIDs  []string // RBAC: restrict to these endeavours (empty = no restriction)
	IsEnabled     *bool
	Status        string
	Search        string
	PredecessorID string
	Origin        string
	Lang          string
	Limit         int
	Offset        int
}

// UpdateRitualFields holds the fields to update on a ritual.
// Prompt and Version are intentionally excluded -- create a new version instead.
type UpdateRitualFields struct {
	Name        *string
	Description *string
	Schedule    map[string]interface{}
	IsEnabled   *bool
	Lang        *string
	Status      *string
	Metadata    map[string]interface{}
	EndeavourID *string
}

// ErrRitualNotFound is returned when a ritual cannot be found by its ID.
var ErrRitualNotFound = errors.New("ritual not found")

// CreateRitual creates a new ritual. FRM-native: no direct FK columns for endeavour.
// The endeavour link is created via entity_relation (governs).
func (db *DB) CreateRitual(name, description, prompt, origin, createdBy, endeavourID, lang, methodologyID string, version int, predecessorID string, schedule map[string]interface{}, metadata map[string]interface{}) (*Ritual, error) {
	id := generateID("rtl")

	scheduleJSON := sql.NullString{}
	if schedule != nil {
		b, err := json.Marshal(schedule)
		if err != nil {
			return nil, fmt.Errorf("marshal schedule: %w", err)
		}
		scheduleJSON = sql.NullString{String: string(b), Valid: true}
	}

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

	var predVal *string
	if predecessorID != "" {
		predVal = &predecessorID
	}

	var createdByVal *string
	if createdBy != "" {
		createdByVal = &createdBy
	}

	if lang == "" {
		lang = "en"
	}

	var mthVal *string
	if methodologyID != "" {
		mthVal = &methodologyID
	}

	now := UTCNow()
	_, err := db.Exec(
		`INSERT INTO ritual (id, name, description, prompt, version, predecessor_id, origin,
		 methodology_id, schedule, is_enabled, lang, metadata, created_by, status)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 1, ?, ?, ?, 'active')`,
		id, name, descVal, prompt, version, predVal, origin,
		mthVal, scheduleJSON, lang, metadataJSON, createdByVal,
	)
	if err != nil {
		return nil, fmt.Errorf("insert ritual: %w", err)
	}

	// Create endeavour link via entity_relation (governs)
	if endeavourID != "" {
		relID := generateID("rel")
		_, _ = db.Exec(
			`INSERT INTO entity_relation (id, relationship_type, source_entity_type, source_entity_id,
			 target_entity_type, target_entity_id, created_at)
			 VALUES (?, ?, 'ritual', ?, 'endeavour', ?, ?)`,
			relID, RelGoverns, id, endeavourID, now.Format(time.RFC3339),
		)
	}

	return &Ritual{
		ID:            id,
		Name:          name,
		Description:   description,
		Prompt:        prompt,
		Version:       version,
		PredecessorID: predecessorID,
		Origin:        origin,
		MethodologyID: methodologyID,
		Schedule:      schedule,
		IsEnabled:     true,
		Lang:          lang,
		Metadata:      metadata,
		CreatedBy:     createdBy,
		Status:        "active",
		EndeavourID:   endeavourID,
		CreatedAt:     now,
		UpdatedAt:     now,
	}, nil
}

// GetRitual retrieves a ritual by ID with endeavour from entity_relation.
func (db *DB) GetRitual(id string) (*Ritual, error) {
	var r Ritual
	var description sql.NullString
	var predecessorID sql.NullString
	var methodologyID sql.NullString
	var scheduleJSON sql.NullString
	var createdBy sql.NullString
	var endeavourID sql.NullString
	var metadataJSON string
	var isEnabled int
	var createdAt, updatedAt string

	err := db.QueryRow(
		`SELECT r.id, r.name, r.description, r.prompt, r.version, r.predecessor_id,
		        r.origin, r.methodology_id, r.schedule, r.is_enabled, r.lang, r.metadata,
		        r.created_by, r.status,
		        rel_edv.target_entity_id,
		        r.created_at, r.updated_at
		 FROM ritual r
		 LEFT JOIN entity_relation rel_edv ON rel_edv.source_entity_id = r.id
		     AND rel_edv.source_entity_type = 'ritual'
		     AND rel_edv.relationship_type = 'governs'
		     AND rel_edv.target_entity_type = 'endeavour'
		 WHERE r.id = ?`,
		id,
	).Scan(&r.ID, &r.Name, &description, &r.Prompt, &r.Version, &predecessorID,
		&r.Origin, &methodologyID, &scheduleJSON, &isEnabled, &r.Lang, &metadataJSON,
		&createdBy, &r.Status,
		&endeavourID,
		&createdAt, &updatedAt)

	if err == sql.ErrNoRows {
		return nil, ErrRitualNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query ritual: %w", err)
	}

	if description.Valid {
		r.Description = description.String
	}
	if predecessorID.Valid {
		r.PredecessorID = predecessorID.String
	}
	if methodologyID.Valid {
		r.MethodologyID = methodologyID.String
	}
	if scheduleJSON.Valid {
		_ = json.Unmarshal([]byte(scheduleJSON.String), &r.Schedule)
	}
	if createdBy.Valid {
		r.CreatedBy = createdBy.String
	}
	if endeavourID.Valid {
		r.EndeavourID = endeavourID.String
	}
	r.IsEnabled = isEnabled == 1
	_ = json.Unmarshal([]byte(metadataJSON), &r.Metadata)
	r.CreatedAt = ParseDBTime(createdAt)
	r.UpdatedAt = ParseDBTime(updatedAt)

	return &r, nil
}

// ListRituals queries rituals with filters.
func (db *DB) ListRituals(opts ListRitualsOpts) ([]*Ritual, int, error) {
	query := `SELECT r.id, r.name, r.description, r.prompt, r.version, r.predecessor_id,
	                 r.origin, r.methodology_id, r.schedule, r.is_enabled, r.lang, r.metadata,
	                 r.created_by, r.status,
	                 rel_edv.target_entity_id,
	                 r.created_at, r.updated_at
	          FROM ritual r
	          LEFT JOIN entity_relation rel_edv ON rel_edv.source_entity_id = r.id
	              AND rel_edv.source_entity_type = 'ritual'
	              AND rel_edv.relationship_type = 'governs'
	              AND rel_edv.target_entity_type = 'endeavour'`
	countQuery := `SELECT COUNT(*) FROM ritual r`

	var conditions []string
	var countConditions []string
	var params []interface{}
	var countParams []interface{}

	if opts.Status != "" {
		conditions = append(conditions, "r.status = ?")
		countConditions = append(countConditions, "r.status = ?")
		params = append(params, opts.Status)
		countParams = append(countParams, opts.Status)
	}
	if opts.Origin != "" {
		conditions = append(conditions, "r.origin = ?")
		countConditions = append(countConditions, "r.origin = ?")
		params = append(params, opts.Origin)
		countParams = append(countParams, opts.Origin)
	}
	if opts.IsEnabled != nil {
		val := 0
		if *opts.IsEnabled {
			val = 1
		}
		conditions = append(conditions, "r.is_enabled = ?")
		countConditions = append(countConditions, "r.is_enabled = ?")
		params = append(params, val)
		countParams = append(countParams, val)
	}
	if opts.Lang != "" {
		conditions = append(conditions, "r.lang = ?")
		countConditions = append(countConditions, "r.lang = ?")
		params = append(params, opts.Lang)
		countParams = append(countParams, opts.Lang)
	}
	if opts.PredecessorID != "" {
		conditions = append(conditions, "r.predecessor_id = ?")
		countConditions = append(countConditions, "r.predecessor_id = ?")
		params = append(params, opts.PredecessorID)
		countParams = append(countParams, opts.PredecessorID)
	}
	if opts.EndeavourID != "" {
		conditions = append(conditions, "rel_edv.target_entity_id = ?")
		params = append(params, opts.EndeavourID)
		countQuery += ` JOIN entity_relation cr_edv ON cr_edv.source_entity_id = r.id
		    AND cr_edv.source_entity_type = 'ritual'
		    AND cr_edv.relationship_type = 'governs'
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
		countQuery += ` LEFT JOIN entity_relation cr_edv2 ON cr_edv2.source_entity_id = r.id
		    AND cr_edv2.source_entity_type = 'ritual'
		    AND cr_edv2.relationship_type = 'governs'
		    AND cr_edv2.target_entity_type = 'endeavour'`
		countConditions = append(countConditions, "(cr_edv2.target_entity_id IN ("+inClause+") OR cr_edv2.target_entity_id IS NULL)")
	}
	if opts.Search != "" {
		conditions = append(conditions, "(r.name LIKE ? ESCAPE '\\' OR r.description LIKE ? ESCAPE '\\')")
		countConditions = append(countConditions, "(r.name LIKE ? ESCAPE '\\' OR r.description LIKE ? ESCAPE '\\')")
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
	query += ` ORDER BY r.created_at DESC LIMIT ? OFFSET ?`
	params = append(params, opts.Limit, opts.Offset)

	rows, err := db.Query(query, params...)
	if err != nil {
		return nil, 0, fmt.Errorf("query rituals: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var rituals []*Ritual
	for rows.Next() {
		var r Ritual
		var description sql.NullString
		var predecessorID sql.NullString
		var methodologyID sql.NullString
		var scheduleJSON sql.NullString
		var createdBy sql.NullString
		var endeavourID sql.NullString
		var metadataJSON string
		var isEnabled int
		var createdAt, updatedAt string

		if err := rows.Scan(&r.ID, &r.Name, &description, &r.Prompt, &r.Version, &predecessorID,
			&r.Origin, &methodologyID, &scheduleJSON, &isEnabled, &r.Lang, &metadataJSON,
			&createdBy, &r.Status,
			&endeavourID,
			&createdAt, &updatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan ritual: %w", err)
		}

		if description.Valid {
			r.Description = description.String
		}
		if predecessorID.Valid {
			r.PredecessorID = predecessorID.String
		}
		if methodologyID.Valid {
			r.MethodologyID = methodologyID.String
		}
		if scheduleJSON.Valid {
			_ = json.Unmarshal([]byte(scheduleJSON.String), &r.Schedule)
		}
		if createdBy.Valid {
			r.CreatedBy = createdBy.String
		}
		if endeavourID.Valid {
			r.EndeavourID = endeavourID.String
		}
		r.IsEnabled = isEnabled == 1
		_ = json.Unmarshal([]byte(metadataJSON), &r.Metadata)
		r.CreatedAt = ParseDBTime(createdAt)
		r.UpdatedAt = ParseDBTime(updatedAt)

		rituals = append(rituals, &r)
	}

	return rituals, total, nil
}

// UpdateRitual applies partial updates to a ritual.
// Prompt and Version cannot be changed -- create a new version instead.
func (db *DB) UpdateRitual(id string, fields UpdateRitualFields) ([]string, error) {
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
	if fields.Schedule != nil {
		b, err := json.Marshal(fields.Schedule)
		if err != nil {
			return nil, fmt.Errorf("marshal schedule: %w", err)
		}
		setClauses = append(setClauses, "schedule = ?")
		params = append(params, string(b))
		updatedFields = append(updatedFields, "schedule")
	}
	if fields.IsEnabled != nil {
		val := 0
		if *fields.IsEnabled {
			val = 1
		}
		setClauses = append(setClauses, "is_enabled = ?")
		params = append(params, val)
		updatedFields = append(updatedFields, "is_enabled")
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

	if len(setClauses) == 0 && fields.EndeavourID == nil {
		return nil, fmt.Errorf("no fields to update")
	}

	if len(setClauses) > 0 {
		query := fmt.Sprintf("UPDATE ritual SET %s WHERE id = ?", strings.Join(setClauses, ", "))
		params = append(params, id)

		result, err := db.Exec(query, params...)
		if err != nil {
			return nil, fmt.Errorf("update ritual: %w", err)
		}

		rowsAffected, _ := result.RowsAffected()
		if rowsAffected == 0 {
			return nil, ErrRitualNotFound
		}
	} else {
		// Verify ritual exists
		var exists int
		if err := db.QueryRow(`SELECT 1 FROM ritual WHERE id = ?`, id).Scan(&exists); err != nil {
			return nil, ErrRitualNotFound
		}
	}

	// Update endeavour relation
	if fields.EndeavourID != nil {
		_ = db.SetRelation(RelGoverns, EntityRitual, id, EntityEndeavour, *fields.EndeavourID, "")
	}

	return updatedFields, nil
}

// ListScheduledRituals returns active, enabled, non-template rituals that have
// a cron or interval schedule and are linked to an endeavour. Used by the ritual
// executor ticker to find rituals that need evaluation.
func (db *DB) ListScheduledRituals() ([]*Ritual, error) {
	rows, err := db.Query(
		`SELECT r.id, r.name, r.description, r.prompt, r.version, r.predecessor_id,
		        r.origin, r.methodology_id, r.schedule, r.is_enabled, r.lang, r.metadata,
		        r.created_by, r.status,
		        rel_edv.target_entity_id,
		        r.created_at, r.updated_at
		 FROM ritual r
		 LEFT JOIN entity_relation rel_edv ON rel_edv.source_entity_id = r.id
		     AND rel_edv.source_entity_type = 'ritual'
		     AND rel_edv.relationship_type = 'governs'
		     AND rel_edv.target_entity_type = 'endeavour'
		 JOIN endeavour edv ON rel_edv.target_entity_id = edv.id
		 WHERE r.status = 'active'
		   AND r.is_enabled = 1
		   AND r.origin != 'template'
		   AND rel_edv.target_entity_id IS NOT NULL
		   AND edv.taskschmied_enabled = 1
		   AND edv.status IN ('active', 'pending')
		   AND json_extract(r.schedule, '$.type') IN ('cron', 'interval')`)
	if err != nil {
		return nil, fmt.Errorf("list scheduled rituals: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var rituals []*Ritual
	for rows.Next() {
		var r Ritual
		var description, predecessorID, methodologyID, scheduleJSON, createdBy, endeavourID sql.NullString
		var metadataJSON, createdAt, updatedAt string
		var isEnabled int
		if err := rows.Scan(
			&r.ID, &r.Name, &description, &r.Prompt, &r.Version, &predecessorID,
			&r.Origin, &methodologyID, &scheduleJSON, &isEnabled, &r.Lang, &metadataJSON,
			&createdBy, &r.Status, &endeavourID, &createdAt, &updatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan ritual: %w", err)
		}
		if description.Valid {
			r.Description = description.String
		}
		if predecessorID.Valid {
			r.PredecessorID = predecessorID.String
		}
		if methodologyID.Valid {
			r.MethodologyID = methodologyID.String
		}
		if scheduleJSON.Valid {
			_ = json.Unmarshal([]byte(scheduleJSON.String), &r.Schedule)
		}
		if createdBy.Valid {
			r.CreatedBy = createdBy.String
		}
		if endeavourID.Valid {
			r.EndeavourID = endeavourID.String
		}
		r.IsEnabled = isEnabled == 1
		_ = json.Unmarshal([]byte(metadataJSON), &r.Metadata)
		r.CreatedAt = ParseDBTime(createdAt)
		r.UpdatedAt = ParseDBTime(updatedAt)
		rituals = append(rituals, &r)
	}
	return rituals, nil
}

// HasEndeavourChangedSince checks whether any task or demand belonging to the
// endeavour has been updated since the given time. Uses LIMIT 1 for efficiency.
func (db *DB) HasEndeavourChangedSince(endeavourID string, since time.Time) (bool, error) {
	// Use space-separated format to match SQLite datetime('now') convention.
	// Task and demand updated_at use "2006-01-02 15:04:05" (no T, no Z).
	sinceStr := since.UTC().Format("2006-01-02 15:04:05")
	var exists int
	err := db.QueryRow(`
		SELECT EXISTS(
			SELECT 1 FROM entity_relation er
			JOIN task t ON er.source_entity_id = t.id
			WHERE er.source_entity_type = 'task'
			  AND er.target_entity_type = 'endeavour'
			  AND er.relationship_type = 'belongs_to'
			  AND er.target_entity_id = ?
			  AND t.updated_at > ?
			UNION ALL
			SELECT 1 FROM entity_relation er
			JOIN demand d ON er.source_entity_id = d.id
			WHERE er.source_entity_type = 'demand'
			  AND er.target_entity_type = 'endeavour'
			  AND er.relationship_type = 'belongs_to'
			  AND er.target_entity_id = ?
			  AND d.updated_at > ?
			LIMIT 1
		)`, endeavourID, sinceStr, endeavourID, sinceStr).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check endeavour changes: %w", err)
	}
	return exists == 1, nil
}

// GetRitualLineage walks the predecessor chain backward using a recursive CTE.
// Returns the full lineage from oldest ancestor to newest descendant.
func (db *DB) GetRitualLineage(id string) ([]*Ritual, error) {
	rows, err := db.Query(
		`WITH RECURSIVE lineage(id) AS (
		     -- Start from the given ritual
		     SELECT id FROM ritual WHERE id = ?
		     UNION ALL
		     -- Walk backward through predecessors
		     SELECT r.predecessor_id FROM ritual r
		     INNER JOIN lineage l ON l.id = r.id
		     WHERE r.predecessor_id IS NOT NULL
		 )
		 SELECT rt.id, rt.name, rt.description, rt.prompt, rt.version, rt.predecessor_id,
		        rt.origin, rt.methodology_id, rt.schedule, rt.is_enabled, rt.lang, rt.metadata,
		        rt.created_by, rt.status,
		        rel_edv.target_entity_id,
		        rt.created_at, rt.updated_at
		 FROM ritual rt
		 INNER JOIN lineage l ON l.id = rt.id
		 LEFT JOIN entity_relation rel_edv ON rel_edv.source_entity_id = rt.id
		     AND rel_edv.source_entity_type = 'ritual'
		     AND rel_edv.relationship_type = 'governs'
		     AND rel_edv.target_entity_type = 'endeavour'
		 ORDER BY rt.version ASC`,
		id,
	)
	if err != nil {
		return nil, fmt.Errorf("query ritual lineage: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var rituals []*Ritual
	for rows.Next() {
		var r Ritual
		var description sql.NullString
		var predecessorID sql.NullString
		var methodologyID sql.NullString
		var scheduleJSON sql.NullString
		var createdBy sql.NullString
		var endeavourID sql.NullString
		var metadataJSON string
		var isEnabled int
		var createdAt, updatedAt string

		if err := rows.Scan(&r.ID, &r.Name, &description, &r.Prompt, &r.Version, &predecessorID,
			&r.Origin, &methodologyID, &scheduleJSON, &isEnabled, &r.Lang, &metadataJSON,
			&createdBy, &r.Status,
			&endeavourID,
			&createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan ritual lineage: %w", err)
		}

		if description.Valid {
			r.Description = description.String
		}
		if predecessorID.Valid {
			r.PredecessorID = predecessorID.String
		}
		if methodologyID.Valid {
			r.MethodologyID = methodologyID.String
		}
		if scheduleJSON.Valid {
			_ = json.Unmarshal([]byte(scheduleJSON.String), &r.Schedule)
		}
		if createdBy.Valid {
			r.CreatedBy = createdBy.String
		}
		if endeavourID.Valid {
			r.EndeavourID = endeavourID.String
		}
		r.IsEnabled = isEnabled == 1
		_ = json.Unmarshal([]byte(metadataJSON), &r.Metadata)
		r.CreatedAt = ParseDBTime(createdAt)
		r.UpdatedAt = ParseDBTime(updatedAt)

		rituals = append(rituals, &r)
	}

	if len(rituals) == 0 {
		return nil, ErrRitualNotFound
	}

	return rituals, nil
}
