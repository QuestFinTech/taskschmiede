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

// Resource represents a work capacity entity (human, agent, service, budget).
type Resource struct {
	ID            string
	Type          string
	Name          string
	CapacityModel string
	CapacityValue *float64
	Skills        []string
	Metadata      map[string]interface{}
	Status        string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// ListResourcesOpts holds filters for listing resources.
type ListResourcesOpts struct {
	Type           string
	Status         string
	OrganizationID string
	Search         string
	Limit          int
	Offset         int

	// RBAC visibility filters (set by API layer, not by callers).
	VisibleToResourceID string   // caller's own resource ID
	VisibleToOrgIDs     []string // org IDs the caller belongs to
}

// UpdateResourceFields holds optional fields for partial resource updates.
type UpdateResourceFields struct {
	Name          *string
	CapacityModel *string
	CapacityValue *float64
	Skills        []string               // nil = no change; empty slice = clear
	Metadata      map[string]interface{} // nil = no change; replaces existing
	Status        *string
}

// ErrResourceNotFound is returned when a resource cannot be found by its ID.
var ErrResourceNotFound = errors.New("resource not found")

// CreateResource creates a new resource.
func (db *DB) CreateResource(resType, name, capacityModel string, capacityValue *float64, skills []string, metadata map[string]interface{}) (*Resource, error) {
	id := generateID("res")

	skillsJSON := "[]"
	if skills != nil {
		b, err := json.Marshal(skills)
		if err != nil {
			return nil, fmt.Errorf("marshal skills: %w", err)
		}
		skillsJSON = string(b)
	}

	metadataJSON := "{}"
	if metadata != nil {
		b, err := json.Marshal(metadata)
		if err != nil {
			return nil, fmt.Errorf("marshal metadata: %w", err)
		}
		metadataJSON = string(b)
	}

	var capModel, capVal interface{}
	if capacityModel != "" {
		capModel = capacityModel
	}
	if capacityValue != nil {
		capVal = *capacityValue
	}

	now := UTCNow()
	_, err := db.Exec(
		`INSERT INTO resource (id, type, name, capacity_model, capacity_value, skills, metadata, status)
		 VALUES (?, ?, ?, ?, ?, ?, ?, 'active')`,
		id, resType, name, capModel, capVal, skillsJSON, metadataJSON,
	)
	if err != nil {
		return nil, fmt.Errorf("insert resource: %w", err)
	}

	return &Resource{
		ID:            id,
		Type:          resType,
		Name:          name,
		CapacityModel: capacityModel,
		CapacityValue: capacityValue,
		Skills:        skills,
		Metadata:      metadata,
		Status:        "active",
		CreatedAt:     now,
		UpdatedAt:     now,
	}, nil
}

// GetResource retrieves a resource by ID.
func (db *DB) GetResource(id string) (*Resource, error) {
	var res Resource
	var capacityModel sql.NullString
	var capacityValue sql.NullFloat64
	var skillsJSON, metadataJSON string
	var createdAt, updatedAt string

	err := db.QueryRow(
		`SELECT id, type, name, capacity_model, capacity_value, skills, metadata, status, created_at, updated_at
		 FROM resource WHERE id = ?`,
		id,
	).Scan(&res.ID, &res.Type, &res.Name, &capacityModel, &capacityValue,
		&skillsJSON, &metadataJSON, &res.Status, &createdAt, &updatedAt)

	if err == sql.ErrNoRows {
		return nil, ErrResourceNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query resource: %w", err)
	}

	if capacityModel.Valid {
		res.CapacityModel = capacityModel.String
	}
	if capacityValue.Valid {
		res.CapacityValue = &capacityValue.Float64
	}
	_ = json.Unmarshal([]byte(skillsJSON), &res.Skills)
	_ = json.Unmarshal([]byte(metadataJSON), &res.Metadata)
	res.CreatedAt = ParseDBTime(createdAt)
	res.UpdatedAt = ParseDBTime(updatedAt)

	return &res, nil
}

// DeleteResource hard-deletes a resource and all its relations.
func (db *DB) DeleteResource(id string) error {
	// Delete all relations involving this resource.
	_, _ = db.Exec(`DELETE FROM entity_relation
		WHERE (source_entity_type = 'resource' AND source_entity_id = ?)
		   OR (target_entity_type = 'resource' AND target_entity_id = ?)`, id, id)
	result, err := db.Exec(`DELETE FROM resource WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete resource: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrResourceNotFound
	}
	return nil
}

// ListResources queries resources with filters.
func (db *DB) ListResources(opts ListResourcesOpts) ([]*Resource, int, error) {
	query := `SELECT r.id, r.type, r.name, r.capacity_model, r.capacity_value, r.skills, r.metadata, r.status, r.created_at, r.updated_at FROM resource r`
	countQuery := `SELECT COUNT(*) FROM resource r`

	var conditions []string
	var params []interface{}

	if opts.OrganizationID != "" {
		query += ` JOIN entity_relation er ON r.id = er.target_entity_id
		    AND er.target_entity_type = 'resource'
		    AND er.source_entity_type = 'organization'
		    AND er.relationship_type = 'has_member'`
		countQuery += ` JOIN entity_relation er ON r.id = er.target_entity_id
		    AND er.target_entity_type = 'resource'
		    AND er.source_entity_type = 'organization'
		    AND er.relationship_type = 'has_member'`
		conditions = append(conditions, "er.source_entity_id = ?")
		params = append(params, opts.OrganizationID)
	}

	if opts.Type != "" {
		conditions = append(conditions, "r.type = ?")
		params = append(params, opts.Type)
	}

	if opts.Status != "" {
		conditions = append(conditions, "r.status = ?")
		params = append(params, opts.Status)
	}

	if opts.Search != "" {
		conditions = append(conditions, "(r.name LIKE ? ESCAPE '\\')")
		searchParam := "%" + escapeLike(opts.Search, '\\') + "%"
		params = append(params, searchParam)
	}

	// RBAC: restrict to caller's own resource + resources in their orgs.
	if opts.VisibleToResourceID != "" || len(opts.VisibleToOrgIDs) > 0 {
		var visParts []string
		if opts.VisibleToResourceID != "" {
			visParts = append(visParts, "r.id = ?")
			params = append(params, opts.VisibleToResourceID)
		}
		if len(opts.VisibleToOrgIDs) > 0 {
			placeholders := make([]string, len(opts.VisibleToOrgIDs))
			for i, oid := range opts.VisibleToOrgIDs {
				placeholders[i] = "?"
				params = append(params, oid)
			}
			visParts = append(visParts, `r.id IN (
				SELECT er2.target_entity_id FROM entity_relation er2
				WHERE er2.target_entity_type = 'resource'
				  AND er2.source_entity_type = 'organization'
				  AND er2.relationship_type = 'has_member'
				  AND er2.source_entity_id IN (`+strings.Join(placeholders, ",")+`))`)
		}
		conditions = append(conditions, "("+strings.Join(visParts, " OR ")+")")
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
	query += ` ORDER BY r.created_at DESC LIMIT ? OFFSET ?`
	params = append(params, opts.Limit, opts.Offset)

	rows, err := db.Query(query, params...)
	if err != nil {
		return nil, 0, fmt.Errorf("query resources: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var resources []*Resource
	for rows.Next() {
		var res Resource
		var capacityModel sql.NullString
		var capacityValue sql.NullFloat64
		var skillsJSON, metadataJSON string
		var createdAt, updatedAt string

		if err := rows.Scan(&res.ID, &res.Type, &res.Name, &capacityModel, &capacityValue,
			&skillsJSON, &metadataJSON, &res.Status, &createdAt, &updatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan resource: %w", err)
		}

		if capacityModel.Valid {
			res.CapacityModel = capacityModel.String
		}
		if capacityValue.Valid {
			res.CapacityValue = &capacityValue.Float64
		}
		_ = json.Unmarshal([]byte(skillsJSON), &res.Skills)
		_ = json.Unmarshal([]byte(metadataJSON), &res.Metadata)
		res.CreatedAt = ParseDBTime(createdAt)
		res.UpdatedAt = ParseDBTime(updatedAt)

		resources = append(resources, &res)
	}

	return resources, total, nil
}

// UpdateResource applies partial updates to a resource.
func (db *DB) UpdateResource(id string, fields UpdateResourceFields) ([]string, error) {
	var setClauses []string
	var params []interface{}
	var updatedFields []string

	if fields.Name != nil {
		setClauses = append(setClauses, "name = ?")
		params = append(params, *fields.Name)
		updatedFields = append(updatedFields, "name")
	}
	if fields.CapacityModel != nil {
		setClauses = append(setClauses, "capacity_model = ?")
		params = append(params, *fields.CapacityModel)
		updatedFields = append(updatedFields, "capacity_model")
	}
	if fields.CapacityValue != nil {
		setClauses = append(setClauses, "capacity_value = ?")
		params = append(params, *fields.CapacityValue)
		updatedFields = append(updatedFields, "capacity_value")
	}
	if fields.Skills != nil {
		b, err := json.Marshal(fields.Skills)
		if err != nil {
			return nil, fmt.Errorf("marshal skills: %w", err)
		}
		setClauses = append(setClauses, "skills = ?")
		params = append(params, string(b))
		updatedFields = append(updatedFields, "skills")
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

	if len(setClauses) == 0 {
		return nil, fmt.Errorf("no fields to update")
	}

	setClauses = append(setClauses, "updated_at = datetime('now')")
	query := fmt.Sprintf("UPDATE resource SET %s WHERE id = ?", strings.Join(setClauses, ", "))
	params = append(params, id)

	result, err := db.Exec(query, params...)
	if err != nil {
		return nil, fmt.Errorf("update resource: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return nil, ErrResourceNotFound
	}

	return updatedFields, nil
}
