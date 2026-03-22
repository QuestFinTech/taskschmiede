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

// Template represents a report/document template (Go text/template syntax).
type Template struct {
	ID            string
	Name          string
	Type          string // report, etc.
	Scope         string // task, demand, endeavour
	Lang          string
	Body          string
	Version       int
	PredecessorID string
	Metadata      map[string]interface{}
	CreatedBy     string
	Status        string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// ListTemplatesOpts holds filters for listing templates.
type ListTemplatesOpts struct {
	Scope  string
	Lang   string
	Status string
	Search string
	Limit  int
	Offset int
}

// UpdateTemplateFields holds the fields to update on a template.
type UpdateTemplateFields struct {
	Name     *string
	Body     *string
	Lang     *string
	Metadata map[string]interface{}
	Status   *string
}

// ErrTemplateNotFound is returned when a template cannot be found by its ID.
var ErrTemplateNotFound = errors.New("template not found")

// CreateTemplate creates a new template.
func (db *DB) CreateTemplate(name, tplType, scope, lang, body, createdBy string, version int, predecessorID string, metadata map[string]interface{}) (*Template, error) {
	id := generateID("tpl")

	if lang == "" {
		lang = "en"
	}

	metadataJSON := "{}"
	if metadata != nil {
		b, err := json.Marshal(metadata)
		if err != nil {
			return nil, fmt.Errorf("marshal metadata: %w", err)
		}
		metadataJSON = string(b)
	}

	var predVal *string
	if predecessorID != "" {
		predVal = &predecessorID
	}

	var createdByVal *string
	if createdBy != "" {
		createdByVal = &createdBy
	}

	now := UTCNow()
	_, err := db.Exec(
		`INSERT INTO template (id, name, type, scope, lang, body, version, predecessor_id,
		 metadata, created_by, status)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'active')`,
		id, name, tplType, scope, lang, body, version, predVal,
		metadataJSON, createdByVal,
	)
	if err != nil {
		return nil, fmt.Errorf("insert template: %w", err)
	}

	return &Template{
		ID:            id,
		Name:          name,
		Type:          tplType,
		Scope:         scope,
		Lang:          lang,
		Body:          body,
		Version:       version,
		PredecessorID: predecessorID,
		Metadata:      metadata,
		CreatedBy:     createdBy,
		Status:        "active",
		CreatedAt:     now,
		UpdatedAt:     now,
	}, nil
}

// GetTemplate retrieves a template by ID.
func (db *DB) GetTemplate(id string) (*Template, error) {
	var t Template
	var predecessorID sql.NullString
	var createdBy sql.NullString
	var metadataJSON string
	var createdAt, updatedAt string

	err := db.QueryRow(
		`SELECT id, name, type, scope, lang, body, version, predecessor_id,
		        metadata, created_by, status, created_at, updated_at
		 FROM template WHERE id = ?`, id,
	).Scan(&t.ID, &t.Name, &t.Type, &t.Scope, &t.Lang, &t.Body, &t.Version, &predecessorID,
		&metadataJSON, &createdBy, &t.Status, &createdAt, &updatedAt)

	if err == sql.ErrNoRows {
		return nil, ErrTemplateNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query template: %w", err)
	}

	if predecessorID.Valid {
		t.PredecessorID = predecessorID.String
	}
	if createdBy.Valid {
		t.CreatedBy = createdBy.String
	}
	_ = json.Unmarshal([]byte(metadataJSON), &t.Metadata)
	t.CreatedAt = ParseDBTime(createdAt)
	t.UpdatedAt = ParseDBTime(updatedAt)

	return &t, nil
}

// GetTemplateByScope retrieves the active template for a given scope and language.
// Falls back to "en" if no template exists for the requested language.
func (db *DB) GetTemplateByScope(scope, lang string) (*Template, error) {
	var t Template
	var predecessorID sql.NullString
	var createdBy sql.NullString
	var metadataJSON string
	var createdAt, updatedAt string

	err := db.QueryRow(
		`SELECT id, name, type, scope, lang, body, version, predecessor_id,
		        metadata, created_by, status, created_at, updated_at
		 FROM template WHERE scope = ? AND lang = ? AND status = 'active'`, scope, lang,
	).Scan(&t.ID, &t.Name, &t.Type, &t.Scope, &t.Lang, &t.Body, &t.Version, &predecessorID,
		&metadataJSON, &createdBy, &t.Status, &createdAt, &updatedAt)

	if err == sql.ErrNoRows && lang != "en" {
		// Fallback to English
		err = db.QueryRow(
			`SELECT id, name, type, scope, lang, body, version, predecessor_id,
			        metadata, created_by, status, created_at, updated_at
			 FROM template WHERE scope = ? AND lang = 'en' AND status = 'active'`, scope,
		).Scan(&t.ID, &t.Name, &t.Type, &t.Scope, &t.Lang, &t.Body, &t.Version, &predecessorID,
			&metadataJSON, &createdBy, &t.Status, &createdAt, &updatedAt)
	}

	if err == sql.ErrNoRows {
		return nil, ErrTemplateNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query template by scope: %w", err)
	}

	if predecessorID.Valid {
		t.PredecessorID = predecessorID.String
	}
	if createdBy.Valid {
		t.CreatedBy = createdBy.String
	}
	_ = json.Unmarshal([]byte(metadataJSON), &t.Metadata)
	t.CreatedAt = ParseDBTime(createdAt)
	t.UpdatedAt = ParseDBTime(updatedAt)

	return &t, nil
}

// ListTemplates queries templates with filters.
func (db *DB) ListTemplates(opts ListTemplatesOpts) ([]*Template, int, error) {
	query := `SELECT id, name, type, scope, lang, body, version, predecessor_id,
	                 metadata, created_by, status, created_at, updated_at
	          FROM template`
	countQuery := `SELECT COUNT(*) FROM template`

	var conditions []string
	var params []interface{}

	if opts.Scope != "" {
		conditions = append(conditions, "scope = ?")
		params = append(params, opts.Scope)
	}
	if opts.Lang != "" {
		conditions = append(conditions, "lang = ?")
		params = append(params, opts.Lang)
	}
	if opts.Status != "" {
		conditions = append(conditions, "status = ?")
		params = append(params, opts.Status)
	}
	if opts.Search != "" {
		conditions = append(conditions, "(name LIKE ? ESCAPE '\\' OR body LIKE ? ESCAPE '\\')")
		searchParam := "%" + escapeLike(opts.Search, '\\') + "%"
		params = append(params, searchParam, searchParam)
	}

	if len(conditions) > 0 {
		where := " WHERE " + strings.Join(conditions, " AND ")
		query += where
		countQuery += where
	}

	var total int
	if err := db.QueryRow(countQuery, params...).Scan(&total); err != nil {
		slog.Warn("Failed to count templates", "error", err)
	}

	if opts.Limit <= 0 {
		opts.Limit = 50
	}
	query += ` ORDER BY scope ASC, lang ASC LIMIT ? OFFSET ?`
	params = append(params, opts.Limit, opts.Offset)

	rows, err := db.Query(query, params...)
	if err != nil {
		return nil, 0, fmt.Errorf("query templates: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var templates []*Template
	for rows.Next() {
		var t Template
		var predecessorID sql.NullString
		var createdBy sql.NullString
		var metadataJSON string
		var createdAt, updatedAt string

		if err := rows.Scan(&t.ID, &t.Name, &t.Type, &t.Scope, &t.Lang, &t.Body, &t.Version, &predecessorID,
			&metadataJSON, &createdBy, &t.Status, &createdAt, &updatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan template: %w", err)
		}

		if predecessorID.Valid {
			t.PredecessorID = predecessorID.String
		}
		if createdBy.Valid {
			t.CreatedBy = createdBy.String
		}
		_ = json.Unmarshal([]byte(metadataJSON), &t.Metadata)
		t.CreatedAt = ParseDBTime(createdAt)
		t.UpdatedAt = ParseDBTime(updatedAt)

		templates = append(templates, &t)
	}

	return templates, total, nil
}

// UpdateTemplate applies partial updates to a template.
func (db *DB) UpdateTemplate(id string, fields UpdateTemplateFields) ([]string, error) {
	var setClauses []string
	var params []interface{}
	var updatedFields []string

	if fields.Name != nil {
		setClauses = append(setClauses, "name = ?")
		params = append(params, *fields.Name)
		updatedFields = append(updatedFields, "name")
	}
	if fields.Body != nil {
		setClauses = append(setClauses, "body = ?")
		params = append(params, *fields.Body)
		updatedFields = append(updatedFields, "body")
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

	if len(setClauses) == 0 {
		return nil, fmt.Errorf("no fields to update")
	}

	// Bump version on content changes (name or body).
	if fields.Name != nil || fields.Body != nil {
		setClauses = append(setClauses, "version = version + 1")
		updatedFields = append(updatedFields, "version")
	}

	query := fmt.Sprintf("UPDATE template SET %s WHERE id = ?", strings.Join(setClauses, ", "))
	params = append(params, id)

	result, err := db.Exec(query, params...)
	if err != nil {
		return nil, fmt.Errorf("update template: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return nil, ErrTemplateNotFound
	}

	return updatedFields, nil
}

// ForkTemplate creates a new template derived from an existing one.
func (db *DB) ForkTemplate(sourceID, name, body, lang, createdBy string, metadata map[string]interface{}) (*Template, error) {
	source, err := db.GetTemplate(sourceID)
	if err != nil {
		return nil, err
	}

	if name == "" {
		name = source.Name
	}
	if body == "" {
		body = source.Body
	}
	if lang == "" {
		lang = source.Lang
	}

	// Archive the source if same scope+lang (unique constraint).
	// Scope is always inherited from the source, so only check lang.
	if lang == source.Lang {
		archived := "archived"
		_, _ = db.UpdateTemplate(sourceID, UpdateTemplateFields{Status: &archived})
	}

	return db.CreateTemplate(name, source.Type, source.Scope, lang, body, createdBy, source.Version+1, sourceID, metadata)
}

// GetTemplateLineage walks the predecessor chain backward using a recursive CTE.
func (db *DB) GetTemplateLineage(id string) ([]*Template, error) {
	rows, err := db.Query(
		`WITH RECURSIVE lineage(id) AS (
		     SELECT id FROM template WHERE id = ?
		     UNION ALL
		     SELECT t.predecessor_id FROM template t
		     INNER JOIN lineage l ON l.id = t.id
		     WHERE t.predecessor_id IS NOT NULL
		 )
		 SELECT tpl.id, tpl.name, tpl.type, tpl.scope, tpl.lang, tpl.body, tpl.version, tpl.predecessor_id,
		        tpl.metadata, tpl.created_by, tpl.status, tpl.created_at, tpl.updated_at
		 FROM template tpl
		 INNER JOIN lineage l ON l.id = tpl.id
		 ORDER BY tpl.version ASC`, id,
	)
	if err != nil {
		return nil, fmt.Errorf("query template lineage: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var templates []*Template
	for rows.Next() {
		var t Template
		var predecessorID sql.NullString
		var createdBy sql.NullString
		var metadataJSON string
		var createdAt, updatedAt string

		if err := rows.Scan(&t.ID, &t.Name, &t.Type, &t.Scope, &t.Lang, &t.Body, &t.Version, &predecessorID,
			&metadataJSON, &createdBy, &t.Status, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan template lineage: %w", err)
		}

		if predecessorID.Valid {
			t.PredecessorID = predecessorID.String
		}
		if createdBy.Valid {
			t.CreatedBy = createdBy.String
		}
		_ = json.Unmarshal([]byte(metadataJSON), &t.Metadata)
		t.CreatedAt = ParseDBTime(createdAt)
		t.UpdatedAt = ParseDBTime(updatedAt)

		templates = append(templates, &t)
	}

	if len(templates) == 0 {
		return nil, ErrTemplateNotFound
	}

	return templates, nil
}
