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

// Artifact represents a reference to an external thing (doc, repo, dashboard, spec).
type Artifact struct {
	ID          string
	Kind        string
	Title       string
	URL         string
	Summary     string
	Tags        []string
	Metadata    map[string]interface{}
	CreatedBy   string
	Status      string
	EndeavourID string // populated from entity_relation
	TaskID      string // populated from entity_relation
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ListArtifactsOpts holds filters for listing artifacts.
type ListArtifactsOpts struct {
	EndeavourID  string
	EndeavourIDs []string // RBAC: nil = no restriction; empty = no access
	TaskID       string
	Kind         string
	Status       string
	Search       string
	Tags         string
	Limit        int
	Offset       int
}

// UpdateArtifactFields holds the fields to update on an artifact.
type UpdateArtifactFields struct {
	Title       *string
	Kind        *string
	URL         *string
	Summary     *string
	Tags        *[]string
	Metadata    map[string]interface{}
	Status      *string
	EndeavourID *string
	TaskID      *string
}

// ErrArtifactNotFound is returned when an artifact cannot be found by its ID.
var ErrArtifactNotFound = errors.New("artifact not found")

// CreateArtifact creates a new artifact. FRM-native: no direct FK columns.
// The endeavour and task links are created via entity_relation.
func (db *DB) CreateArtifact(kind, title, url, summary string, tags []string, metadata map[string]interface{}, createdBy, endeavourID, taskID string) (*Artifact, error) {
	id := generateID("art")

	tagsJSON := "[]"
	if tags != nil {
		b, err := json.Marshal(tags)
		if err != nil {
			return nil, fmt.Errorf("marshal tags: %w", err)
		}
		tagsJSON = string(b)
	}

	metadataJSON := "{}"
	if metadata != nil {
		b, err := json.Marshal(metadata)
		if err != nil {
			return nil, fmt.Errorf("marshal metadata: %w", err)
		}
		metadataJSON = string(b)
	}

	var urlVal *string
	if url != "" {
		urlVal = &url
	}

	var summaryVal *string
	if summary != "" {
		summaryVal = &summary
	}

	var createdByVal *string
	if createdBy != "" {
		createdByVal = &createdBy
	}

	now := UTCNow()
	_, err := db.Exec(
		`INSERT INTO artifact (id, kind, title, url, summary, tags, metadata, created_by, status)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'active')`,
		id, kind, title, urlVal, summaryVal, tagsJSON, metadataJSON, createdByVal,
	)
	if err != nil {
		return nil, fmt.Errorf("insert artifact: %w", err)
	}

	// Create endeavour link via entity_relation
	if endeavourID != "" {
		relID := generateID("rel")
		_, _ = db.Exec(
			`INSERT INTO entity_relation (id, relationship_type, source_entity_type, source_entity_id,
			 target_entity_type, target_entity_id, created_at)
			 VALUES (?, ?, 'artifact', ?, 'endeavour', ?, ?)`,
			relID, RelBelongsTo, id, endeavourID, now.Format(time.RFC3339),
		)
	}

	// Create task link via entity_relation
	if taskID != "" {
		relID := generateID("rel")
		_, _ = db.Exec(
			`INSERT INTO entity_relation (id, relationship_type, source_entity_type, source_entity_id,
			 target_entity_type, target_entity_id, created_at)
			 VALUES (?, ?, 'artifact', ?, 'task', ?, ?)`,
			relID, RelBelongsTo, id, taskID, now.Format(time.RFC3339),
		)
	}

	if tags == nil {
		tags = []string{}
	}

	return &Artifact{
		ID:          id,
		Kind:        kind,
		Title:       title,
		URL:         url,
		Summary:     summary,
		Tags:        tags,
		Metadata:    metadata,
		CreatedBy:   createdBy,
		Status:      "active",
		EndeavourID: endeavourID,
		TaskID:      taskID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

// GetArtifact retrieves an artifact by ID with endeavour and task from entity_relation.
func (db *DB) GetArtifact(id string) (*Artifact, error) {
	var a Artifact
	var url, summary sql.NullString
	var createdBy sql.NullString
	var endeavourID, taskID sql.NullString
	var tagsJSON, metadataJSON string
	var createdAt, updatedAt string

	err := db.QueryRow(
		`SELECT a.id, a.kind, a.title, a.url, a.summary, a.tags, a.metadata,
		        a.created_by, a.status,
		        rel_edv.target_entity_id,
		        rel_tsk.target_entity_id,
		        a.created_at, a.updated_at
		 FROM artifact a
		 LEFT JOIN entity_relation rel_edv ON rel_edv.source_entity_id = a.id
		     AND rel_edv.source_entity_type = 'artifact'
		     AND rel_edv.relationship_type = 'belongs_to'
		     AND rel_edv.target_entity_type = 'endeavour'
		 LEFT JOIN entity_relation rel_tsk ON rel_tsk.source_entity_id = a.id
		     AND rel_tsk.source_entity_type = 'artifact'
		     AND rel_tsk.relationship_type = 'belongs_to'
		     AND rel_tsk.target_entity_type = 'task'
		 WHERE a.id = ?`,
		id,
	).Scan(&a.ID, &a.Kind, &a.Title, &url, &summary, &tagsJSON, &metadataJSON,
		&createdBy, &a.Status,
		&endeavourID,
		&taskID,
		&createdAt, &updatedAt)

	if err == sql.ErrNoRows {
		return nil, ErrArtifactNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query artifact: %w", err)
	}

	if url.Valid {
		a.URL = url.String
	}
	if summary.Valid {
		a.Summary = summary.String
	}
	if createdBy.Valid {
		a.CreatedBy = createdBy.String
	}
	if endeavourID.Valid {
		a.EndeavourID = endeavourID.String
	}
	if taskID.Valid {
		a.TaskID = taskID.String
	}
	_ = json.Unmarshal([]byte(tagsJSON), &a.Tags)
	if a.Tags == nil {
		a.Tags = []string{}
	}
	_ = json.Unmarshal([]byte(metadataJSON), &a.Metadata)
	a.CreatedAt = ParseDBTime(createdAt)
	a.UpdatedAt = ParseDBTime(updatedAt)

	return &a, nil
}

// ListArtifacts queries artifacts with filters.
func (db *DB) ListArtifacts(opts ListArtifactsOpts) ([]*Artifact, int, error) {
	// Non-nil but empty EndeavourIDs means "no access to any endeavour".
	if opts.EndeavourIDs != nil && len(opts.EndeavourIDs) == 0 {
		return nil, 0, nil
	}

	query := `SELECT a.id, a.kind, a.title, a.url, a.summary, a.tags, a.metadata,
	                 a.created_by, a.status,
	                 rel_edv.target_entity_id,
	                 rel_tsk.target_entity_id,
	                 a.created_at, a.updated_at
	          FROM artifact a
	          LEFT JOIN entity_relation rel_edv ON rel_edv.source_entity_id = a.id
	              AND rel_edv.source_entity_type = 'artifact'
	              AND rel_edv.relationship_type = 'belongs_to'
	              AND rel_edv.target_entity_type = 'endeavour'
	          LEFT JOIN entity_relation rel_tsk ON rel_tsk.source_entity_id = a.id
	              AND rel_tsk.source_entity_type = 'artifact'
	              AND rel_tsk.relationship_type = 'belongs_to'
	              AND rel_tsk.target_entity_type = 'task'`
	countQuery := `SELECT COUNT(*) FROM artifact a`

	var conditions []string
	var countConditions []string
	var params []interface{}
	var countParams []interface{}

	if opts.Status != "" {
		conditions = append(conditions, "a.status = ?")
		countConditions = append(countConditions, "a.status = ?")
		params = append(params, opts.Status)
		countParams = append(countParams, opts.Status)
	} else {
		conditions = append(conditions, "a.status != 'deleted'")
		countConditions = append(countConditions, "a.status != 'deleted'")
	}
	if opts.Kind != "" {
		conditions = append(conditions, "a.kind = ?")
		countConditions = append(countConditions, "a.kind = ?")
		params = append(params, opts.Kind)
		countParams = append(countParams, opts.Kind)
	}
	if opts.EndeavourID != "" {
		conditions = append(conditions, "rel_edv.target_entity_id = ?")
		params = append(params, opts.EndeavourID)
		countQuery += ` JOIN entity_relation cr_edv ON cr_edv.source_entity_id = a.id
		    AND cr_edv.source_entity_type = 'artifact'
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
		countQuery += ` LEFT JOIN entity_relation cr_edv2 ON cr_edv2.source_entity_id = a.id
		    AND cr_edv2.source_entity_type = 'artifact'
		    AND cr_edv2.relationship_type = 'belongs_to'
		    AND cr_edv2.target_entity_type = 'endeavour'`
		countConditions = append(countConditions, "(cr_edv2.target_entity_id IN ("+inClause+") OR cr_edv2.target_entity_id IS NULL)")
	}
	if opts.TaskID != "" {
		conditions = append(conditions, "rel_tsk.target_entity_id = ?")
		params = append(params, opts.TaskID)
		countQuery += ` JOIN entity_relation cr_tsk ON cr_tsk.source_entity_id = a.id
		    AND cr_tsk.source_entity_type = 'artifact'
		    AND cr_tsk.relationship_type = 'belongs_to'
		    AND cr_tsk.target_entity_type = 'task'`
		countConditions = append(countConditions, "cr_tsk.target_entity_id = ?")
		countParams = append(countParams, opts.TaskID)
	}
	if opts.Search != "" {
		conditions = append(conditions, "(a.title LIKE ? ESCAPE '\\' OR a.summary LIKE ? ESCAPE '\\')")
		countConditions = append(countConditions, "(a.title LIKE ? ESCAPE '\\' OR a.summary LIKE ? ESCAPE '\\')")
		searchParam := "%" + escapeLike(opts.Search, '\\') + "%"
		params = append(params, searchParam, searchParam)
		countParams = append(countParams, searchParam, searchParam)
	}
	if opts.Tags != "" {
		// Match any artifact whose tags JSON array contains the given tag substring
		conditions = append(conditions, "a.tags LIKE ? ESCAPE '\\'")
		countConditions = append(countConditions, "a.tags LIKE ? ESCAPE '\\'")
		tagParam := "%" + escapeLike(opts.Tags, '\\') + "%"
		params = append(params, tagParam)
		countParams = append(countParams, tagParam)
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
	query += ` ORDER BY a.created_at DESC LIMIT ? OFFSET ?`
	params = append(params, opts.Limit, opts.Offset)

	rows, err := db.Query(query, params...)
	if err != nil {
		return nil, 0, fmt.Errorf("query artifacts: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var artifacts []*Artifact
	for rows.Next() {
		var a Artifact
		var url, summary sql.NullString
		var createdBy sql.NullString
		var endeavourID, taskID sql.NullString
		var tagsJSON, metadataJSON string
		var createdAt, updatedAt string

		if err := rows.Scan(&a.ID, &a.Kind, &a.Title, &url, &summary, &tagsJSON, &metadataJSON,
			&createdBy, &a.Status,
			&endeavourID,
			&taskID,
			&createdAt, &updatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan artifact: %w", err)
		}

		if url.Valid {
			a.URL = url.String
		}
		if summary.Valid {
			a.Summary = summary.String
		}
		if createdBy.Valid {
			a.CreatedBy = createdBy.String
		}
		if endeavourID.Valid {
			a.EndeavourID = endeavourID.String
		}
		if taskID.Valid {
			a.TaskID = taskID.String
		}
		_ = json.Unmarshal([]byte(tagsJSON), &a.Tags)
		if a.Tags == nil {
			a.Tags = []string{}
		}
		_ = json.Unmarshal([]byte(metadataJSON), &a.Metadata)
		a.CreatedAt = ParseDBTime(createdAt)
		a.UpdatedAt = ParseDBTime(updatedAt)

		artifacts = append(artifacts, &a)
	}

	return artifacts, total, nil
}

// UpdateArtifact applies partial updates to an artifact.
func (db *DB) UpdateArtifact(id string, fields UpdateArtifactFields) ([]string, error) {
	var setClauses []string
	var params []interface{}
	var updatedFields []string

	if fields.Title != nil {
		setClauses = append(setClauses, "title = ?")
		params = append(params, *fields.Title)
		updatedFields = append(updatedFields, "title")
	}
	if fields.Kind != nil {
		setClauses = append(setClauses, "kind = ?")
		params = append(params, *fields.Kind)
		updatedFields = append(updatedFields, "kind")
	}
	if fields.URL != nil {
		setClauses = append(setClauses, "url = ?")
		params = append(params, *fields.URL)
		updatedFields = append(updatedFields, "url")
	}
	if fields.Summary != nil {
		setClauses = append(setClauses, "summary = ?")
		params = append(params, *fields.Summary)
		updatedFields = append(updatedFields, "summary")
	}
	if fields.Tags != nil {
		b, err := json.Marshal(*fields.Tags)
		if err != nil {
			return nil, fmt.Errorf("marshal tags: %w", err)
		}
		setClauses = append(setClauses, "tags = ?")
		params = append(params, string(b))
		updatedFields = append(updatedFields, "tags")
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
	if fields.Status != nil {
		setClauses = append(setClauses, "status = ?")
		params = append(params, *fields.Status)
		updatedFields = append(updatedFields, "status")
	}

	// EndeavourID and TaskID managed via entity_relation
	if fields.EndeavourID != nil {
		updatedFields = append(updatedFields, "endeavour_id")
	}
	if fields.TaskID != nil {
		updatedFields = append(updatedFields, "task_id")
	}

	if len(setClauses) == 0 && fields.EndeavourID == nil && fields.TaskID == nil {
		return nil, fmt.Errorf("no fields to update")
	}

	if len(setClauses) > 0 {
		query := fmt.Sprintf("UPDATE artifact SET %s WHERE id = ?", strings.Join(setClauses, ", "))
		params = append(params, id)

		result, err := db.Exec(query, params...)
		if err != nil {
			return nil, fmt.Errorf("update artifact: %w", err)
		}

		rowsAffected, _ := result.RowsAffected()
		if rowsAffected == 0 {
			return nil, ErrArtifactNotFound
		}
	} else {
		// Verify artifact exists
		var exists int
		if err := db.QueryRow(`SELECT 1 FROM artifact WHERE id = ?`, id).Scan(&exists); err != nil {
			return nil, ErrArtifactNotFound
		}
	}

	// Update endeavour relation
	if fields.EndeavourID != nil {
		_ = db.SetRelation(RelBelongsTo, EntityArtifact, id, EntityEndeavour, *fields.EndeavourID, "")
	}

	// Update task relation
	if fields.TaskID != nil {
		_ = db.SetRelation(RelBelongsTo, EntityArtifact, id, EntityTask, *fields.TaskID, "")
	}

	return updatedFields, nil
}
