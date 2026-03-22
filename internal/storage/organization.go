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

// Organization represents an organization in the system.
type Organization struct {
	ID                 string
	Name               string
	Description        string
	Status             string
	TaskschmiedEnabled bool // Default for new endeavours in this org
	Metadata           map[string]interface{}
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// ListOrganizationsOpts holds filters for listing organizations.
type ListOrganizationsOpts struct {
	Status          string
	Search          string
	ResourceID      string   // filter by member resource
	OrganizationIDs []string // RBAC scope filter (nil = no restriction)
	Limit           int
	Offset          int
}

// Organization error sentinels.
var (
	// ErrOrgNotFound is returned when an organization cannot be found by its ID.
	ErrOrgNotFound = errors.New("organization not found")
	// ErrResourceAlreadyInOrg is returned when adding a resource that is already a member.
	ErrResourceAlreadyInOrg = errors.New("resource already in organization")
	// ErrEndeavourAlreadyInOrg is returned when linking an endeavour that is already linked.
	ErrEndeavourAlreadyInOrg = errors.New("endeavour already linked to organization")
)

// CreateOrganization creates a new organization.
func (db *DB) CreateOrganization(name, description string, metadata map[string]interface{}) (*Organization, error) {
	id := generateID("org")

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

	now := UTCNow()
	_, err := db.Exec(
		`INSERT INTO organization (id, name, description, metadata, status) VALUES (?, ?, ?, ?, 'active')`,
		id, name, descVal, metadataJSON,
	)
	if err != nil {
		return nil, fmt.Errorf("insert organization: %w", err)
	}

	return &Organization{
		ID:          id,
		Name:        name,
		Description: description,
		Status:      "active",
		Metadata:    metadata,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

// GetOrganization retrieves an organization by ID.
func (db *DB) GetOrganization(id string) (*Organization, error) {
	var org Organization
	var description sql.NullString
	var metadataJSON string
	var createdAt, updatedAt string

	var taskschmiedEnabled int
	err := db.QueryRow(
		`SELECT id, name, description, status, taskschmied_enabled, metadata, created_at, updated_at
		 FROM organization WHERE id = ?`,
		id,
	).Scan(&org.ID, &org.Name, &description, &org.Status, &taskschmiedEnabled, &metadataJSON, &createdAt, &updatedAt)

	if err == sql.ErrNoRows {
		return nil, ErrOrgNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query organization: %w", err)
	}

	if description.Valid {
		org.Description = description.String
	}
	org.TaskschmiedEnabled = taskschmiedEnabled == 1
	_ = json.Unmarshal([]byte(metadataJSON), &org.Metadata)
	org.CreatedAt = ParseDBTime(createdAt)
	org.UpdatedAt = ParseDBTime(updatedAt)

	return &org, nil
}

// ListOrganizations queries organizations with filters.
func (db *DB) ListOrganizations(opts ListOrganizationsOpts) ([]*Organization, int, error) {
	query := `SELECT o.id, o.name, o.description, o.status, o.taskschmied_enabled, o.metadata, o.created_at, o.updated_at FROM organization o`
	countQuery := `SELECT COUNT(*) FROM organization o`

	var conditions []string
	var params []interface{}

	if opts.ResourceID != "" {
		query += ` JOIN entity_relation er ON o.id = er.source_entity_id
		    AND er.source_entity_type = 'organization'
		    AND er.relationship_type = 'has_member'
		    AND er.target_entity_type = 'resource'`
		countQuery += ` JOIN entity_relation er ON o.id = er.source_entity_id
		    AND er.source_entity_type = 'organization'
		    AND er.relationship_type = 'has_member'
		    AND er.target_entity_type = 'resource'`
		conditions = append(conditions, "er.target_entity_id = ?")
		params = append(params, opts.ResourceID)
	}

	if opts.OrganizationIDs != nil {
		if len(opts.OrganizationIDs) == 0 {
			return nil, 0, nil // no access
		}
		placeholders := make([]string, len(opts.OrganizationIDs))
		for i, id := range opts.OrganizationIDs {
			placeholders[i] = "?"
			params = append(params, id)
		}
		conditions = append(conditions, "o.id IN ("+strings.Join(placeholders, ",")+")")
	}

	if opts.Status != "" {
		conditions = append(conditions, "o.status = ?")
		params = append(params, opts.Status)
	}

	if opts.Search != "" {
		conditions = append(conditions, "(o.name LIKE ? ESCAPE '\\' OR o.description LIKE ? ESCAPE '\\')")
		searchParam := "%" + escapeLike(opts.Search, '\\') + "%"
		params = append(params, searchParam, searchParam)
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
	query += ` ORDER BY o.created_at DESC LIMIT ? OFFSET ?`
	params = append(params, opts.Limit, opts.Offset)

	rows, err := db.Query(query, params...)
	if err != nil {
		return nil, 0, fmt.Errorf("query organizations: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var orgs []*Organization
	for rows.Next() {
		var org Organization
		var description sql.NullString
		var metadataJSON string
		var createdAt, updatedAt string

		var taskschmiedEnabled int
		if err := rows.Scan(&org.ID, &org.Name, &description, &org.Status, &taskschmiedEnabled, &metadataJSON, &createdAt, &updatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan organization: %w", err)
		}
		org.TaskschmiedEnabled = taskschmiedEnabled == 1

		if description.Valid {
			org.Description = description.String
		}
		_ = json.Unmarshal([]byte(metadataJSON), &org.Metadata)
		org.CreatedAt = ParseDBTime(createdAt)
		org.UpdatedAt = ParseDBTime(updatedAt)

		orgs = append(orgs, &org)
	}

	return orgs, total, nil
}

// UpdateOrganizationFields holds optional fields for partial organization updates.
type UpdateOrganizationFields struct {
	Name               *string
	Description        *string
	Metadata           map[string]interface{}
	Status             *string
	TaskschmiedEnabled *bool
}

// UpdateOrganization applies partial updates to an organization.
func (db *DB) UpdateOrganization(id string, fields UpdateOrganizationFields) ([]string, error) {
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
	if fields.TaskschmiedEnabled != nil {
		val := 0
		if *fields.TaskschmiedEnabled {
			val = 1
		}
		setClauses = append(setClauses, "taskschmied_enabled = ?")
		params = append(params, val)
		updatedFields = append(updatedFields, "taskschmied_enabled")
	}

	if len(setClauses) == 0 {
		return nil, fmt.Errorf("no fields to update")
	}

	setClauses = append(setClauses, "updated_at = datetime('now')")
	query := fmt.Sprintf("UPDATE organization SET %s WHERE id = ?", strings.Join(setClauses, ", "))
	params = append(params, id)

	result, err := db.Exec(query, params...)
	if err != nil {
		return nil, fmt.Errorf("update organization: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return nil, ErrOrgNotFound
	}

	return updatedFields, nil
}

// GetOrganizationMemberCount returns the number of resources in an organization.
func (db *DB) GetOrganizationMemberCount(orgID string) (int, error) {
	var count int
	err := db.QueryRow(
		`SELECT COUNT(*) FROM entity_relation
		 WHERE relationship_type = ? AND source_entity_type = 'organization'
		   AND source_entity_id = ? AND target_entity_type = 'resource'`,
		RelHasMember, orgID,
	).Scan(&count)
	return count, err
}

// GetOrganizationEndeavourCount returns the number of endeavours linked to an organization.
func (db *DB) GetOrganizationEndeavourCount(orgID string) (int, error) {
	var count int
	err := db.QueryRow(
		`SELECT COUNT(*) FROM entity_relation
		 WHERE relationship_type = ? AND source_entity_type = 'organization'
		   AND source_entity_id = ? AND target_entity_type = 'endeavour'`,
		RelParticipatesIn, orgID,
	).Scan(&count)
	return count, err
}

// GetOrganizationEndeavourIDs returns the IDs of endeavours linked to an organization.
func (db *DB) GetOrganizationEndeavourIDs(orgID string) ([]string, error) {
	rows, err := db.Query(
		`SELECT target_entity_id FROM entity_relation
		 WHERE relationship_type = ? AND source_entity_type = 'organization'
		   AND source_entity_id = ? AND target_entity_type = 'endeavour'`,
		RelParticipatesIn, orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("query org endeavour IDs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			continue
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// AddResourceToOrganization adds a resource to an organization with a role.
func (db *DB) AddResourceToOrganization(orgID, resourceID, role string) error {
	if role == "" {
		role = "member"
	}

	metadata := map[string]interface{}{"role": role}
	_, err := db.CreateRelation(RelHasMember, EntityOrganization, orgID, EntityResource, resourceID, metadata, "")
	if err != nil {
		if errors.Is(err, ErrRelationExists) {
			return ErrResourceAlreadyInOrg
		}
		return fmt.Errorf("add resource to organization: %w", err)
	}
	return nil
}

// AddEndeavourToOrganization links an endeavour to an organization.
func (db *DB) AddEndeavourToOrganization(orgID, endeavourID, role string) error {
	if role == "" {
		role = "participant"
	}

	metadata := map[string]interface{}{"role": role}
	_, err := db.CreateRelation(RelParticipatesIn, EntityOrganization, orgID, EntityEndeavour, endeavourID, metadata, "")
	if err != nil {
		if errors.Is(err, ErrRelationExists) {
			return ErrEndeavourAlreadyInOrg
		}
		return fmt.Errorf("add endeavour to organization: %w", err)
	}
	return nil
}
