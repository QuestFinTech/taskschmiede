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
	"errors"
	"fmt"
	"strings"
	"time"
)

// Relationship type constants.
const (
	RelBelongsTo      = "belongs_to"      // task->endeavour, demand->endeavour
	RelFulfills       = "fulfills"         // task->demand
	RelAssignedTo     = "assigned_to"      // task->resource
	RelHasMember      = "has_member"       // organization->resource
	RelParticipatesIn = "participates_in"  // organization->endeavour
	RelMemberOf       = "member_of"        // user->endeavour
	RelDependsOn      = "depends_on"       // task->task
)

// Entity type constants.
const (
	EntityTask         = "task"
	EntityDemand       = "demand"
	EntityEndeavour    = "endeavour"
	EntityOrganization = "organization"
	EntityResource     = "resource"
	EntityUser         = "user"
	EntityArtifact     = "artifact"
	EntityRitual       = "ritual"
	EntityRitualRun    = "ritual_run"
)

// Additional relationship type constants for BYOM.
const (
	RelGoverns    = "governs"     // ritual->endeavour
	RelUses       = "uses"        // organization->ritual
	RelGovernedBy = "governed_by" // endeavour->dod_policy
	RelSupersedes = "supersedes"  // entity->entity (change requests)
)

// Additional entity type constants.
const (
	EntityDodPolicy = "dod_policy"
	EntityApproval  = "approval"
	EntityTemplate  = "template"
	EntityPerson    = "person"
	EntityAddress   = "address"
)

// Identity relationship type constants.
const (
	RelHasAddress = "has_address" // person->address, organization->address
)

// EntityRelation represents a relationship between two entities.
type EntityRelation struct {
	ID               string
	RelationshipType string
	SourceEntityType string
	SourceEntityID   string
	TargetEntityType string
	TargetEntityID   string
	Metadata         map[string]interface{}
	CreatedBy        string
	CreatedAt        time.Time
}

// ListRelationsOpts holds filters for listing relations.
type ListRelationsOpts struct {
	SourceEntityType string
	SourceEntityID   string
	TargetEntityType string
	TargetEntityID   string
	RelationshipType string
	EndeavourIDs     []string // RBAC: nil = no restriction; empty = no access
	OrganizationIDs  []string // RBAC: org-scoped visibility (combined with EndeavourIDs via OR)
	Limit            int
	Offset           int
}

// Relation error sentinels.
var (
	// ErrRelationNotFound is returned when a relation cannot be found by its ID.
	ErrRelationNotFound = errors.New("relation not found")
	// ErrRelationExists is returned when creating a duplicate relation.
	ErrRelationExists = errors.New("relation already exists")
)

// CreateRelation creates a new entity relation.
func (db *DB) CreateRelation(relType, srcType, srcID, tgtType, tgtID string, metadata map[string]interface{}, createdBy string) (*EntityRelation, error) {
	id := generateID("rel")

	metadataJSON := "{}"
	if metadata != nil {
		b, err := json.Marshal(metadata)
		if err != nil {
			return nil, fmt.Errorf("marshal metadata: %w", err)
		}
		metadataJSON = string(b)
	}

	var createdByVal *string
	if createdBy != "" {
		createdByVal = &createdBy
	}

	now := UTCNow()
	_, err := db.Exec(
		`INSERT INTO entity_relation (id, relationship_type, source_entity_type, source_entity_id,
		 target_entity_type, target_entity_id, metadata, created_by, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, relType, srcType, srcID, tgtType, tgtID, metadataJSON, createdByVal, now.Format(time.RFC3339),
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			return nil, ErrRelationExists
		}
		return nil, fmt.Errorf("insert relation: %w", err)
	}

	return &EntityRelation{
		ID:               id,
		RelationshipType: relType,
		SourceEntityType: srcType,
		SourceEntityID:   srcID,
		TargetEntityType: tgtType,
		TargetEntityID:   tgtID,
		Metadata:         metadata,
		CreatedBy:        createdBy,
		CreatedAt:        now,
	}, nil
}

// DeleteRelation deletes a relation by ID.
func (db *DB) DeleteRelation(id string) error {
	result, err := db.Exec(`DELETE FROM entity_relation WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete relation: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrRelationNotFound
	}
	return nil
}

// DeleteRelationByEndpoints deletes a specific relation by type and endpoints.
func (db *DB) DeleteRelationByEndpoints(relType, srcType, srcID, tgtType, tgtID string) error {
	result, err := db.Exec(
		`DELETE FROM entity_relation
		 WHERE relationship_type = ? AND source_entity_type = ? AND source_entity_id = ?
		   AND target_entity_type = ? AND target_entity_id = ?`,
		relType, srcType, srcID, tgtType, tgtID,
	)
	if err != nil {
		return fmt.Errorf("delete relation by endpoints: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrRelationNotFound
	}
	return nil
}

// ListRelations queries relations with filters.
func (db *DB) ListRelations(opts ListRelationsOpts) ([]*EntityRelation, int, error) {
	// Non-nil but empty EndeavourIDs means "no access to any endeavour".
	if opts.EndeavourIDs != nil && len(opts.EndeavourIDs) == 0 {
		return nil, 0, nil
	}

	query := `SELECT id, relationship_type, source_entity_type, source_entity_id,
	                 target_entity_type, target_entity_id, metadata, created_by, created_at
	          FROM entity_relation`
	countQuery := `SELECT COUNT(*) FROM entity_relation`

	var conditions []string
	var params []interface{}

	if opts.SourceEntityType != "" {
		conditions = append(conditions, "source_entity_type = ?")
		params = append(params, opts.SourceEntityType)
	}
	if opts.SourceEntityID != "" {
		conditions = append(conditions, "source_entity_id = ?")
		params = append(params, opts.SourceEntityID)
	}
	if opts.TargetEntityType != "" {
		conditions = append(conditions, "target_entity_type = ?")
		params = append(params, opts.TargetEntityType)
	}
	if opts.TargetEntityID != "" {
		conditions = append(conditions, "target_entity_id = ?")
		params = append(params, opts.TargetEntityID)
	}
	if opts.RelationshipType != "" {
		conditions = append(conditions, "relationship_type = ?")
		params = append(params, opts.RelationshipType)
	}
	// RBAC scope: combine endeavour and organization visibility via OR.
	var scopeParts []string
	if len(opts.EndeavourIDs) > 0 {
		placeholders := make([]string, len(opts.EndeavourIDs))
		for i := range opts.EndeavourIDs {
			placeholders[i] = "?"
		}
		inClause := strings.Join(placeholders, ", ")
		// Include relations where source or target is linked to an accessible endeavour:
		// 1. Direct endeavour reference (source or target IS an endeavour)
		// 2. Entity belongs_to an accessible endeavour
		// 3. Organization that participates_in an accessible endeavour
		scopeParts = append(scopeParts,
			`(source_entity_type = 'endeavour' AND source_entity_id IN (`+inClause+`))`,
			`(target_entity_type = 'endeavour' AND target_entity_id IN (`+inClause+`))`,
			`source_entity_id IN (SELECT er2.source_entity_id FROM entity_relation er2 WHERE er2.relationship_type = 'belongs_to' AND er2.target_entity_type = 'endeavour' AND er2.target_entity_id IN (`+inClause+`))`,
			`target_entity_id IN (SELECT er3.source_entity_id FROM entity_relation er3 WHERE er3.relationship_type = 'belongs_to' AND er3.target_entity_type = 'endeavour' AND er3.target_entity_id IN (`+inClause+`))`,
			`source_entity_id IN (SELECT er4.source_entity_id FROM entity_relation er4 WHERE er4.relationship_type = 'participates_in' AND er4.target_entity_type = 'endeavour' AND er4.target_entity_id IN (`+inClause+`))`,
			`target_entity_id IN (SELECT er5.source_entity_id FROM entity_relation er5 WHERE er5.relationship_type = 'participates_in' AND er5.target_entity_type = 'endeavour' AND er5.target_entity_id IN (`+inClause+`))`,
		)
		for i := 0; i < 6; i++ {
			for _, id := range opts.EndeavourIDs {
				params = append(params, id)
			}
		}
	}
	if len(opts.OrganizationIDs) > 0 {
		placeholders := make([]string, len(opts.OrganizationIDs))
		for i := range opts.OrganizationIDs {
			placeholders[i] = "?"
		}
		inClause := strings.Join(placeholders, ", ")
		// Include relations involving resources that are members of the user's orgs.
		scopeParts = append(scopeParts,
			`(source_entity_type = 'organization' AND source_entity_id IN (`+inClause+`))`,
			`source_entity_id IN (SELECT er6.target_entity_id FROM entity_relation er6 WHERE er6.relationship_type = 'has_member' AND er6.source_entity_type = 'organization' AND er6.source_entity_id IN (`+inClause+`))`,
			`target_entity_id IN (SELECT er7.target_entity_id FROM entity_relation er7 WHERE er7.relationship_type = 'has_member' AND er7.source_entity_type = 'organization' AND er7.source_entity_id IN (`+inClause+`))`,
		)
		for i := 0; i < 3; i++ {
			for _, id := range opts.OrganizationIDs {
				params = append(params, id)
			}
		}
	}
	if len(scopeParts) > 0 {
		conditions = append(conditions, "("+strings.Join(scopeParts, " OR ")+")")
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
		return nil, 0, fmt.Errorf("query relations: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var relations []*EntityRelation
	for rows.Next() {
		var rel EntityRelation
		var metadataJSON string
		var createdBy *string
		var createdAt string

		if err := rows.Scan(&rel.ID, &rel.RelationshipType, &rel.SourceEntityType, &rel.SourceEntityID,
			&rel.TargetEntityType, &rel.TargetEntityID, &metadataJSON, &createdBy, &createdAt); err != nil {
			return nil, 0, fmt.Errorf("scan relation: %w", err)
		}

		_ = json.Unmarshal([]byte(metadataJSON), &rel.Metadata)
		if createdBy != nil {
			rel.CreatedBy = *createdBy
		}
		rel.CreatedAt = ParseDBTime(createdAt)

		relations = append(relations, &rel)
	}

	return relations, total, nil
}

// GetRelationTargetID returns the target entity ID for a specific relation pattern.
// Returns empty string if no relation found.
func (db *DB) GetRelationTargetID(relType, srcType, srcID, tgtType string) string {
	var targetID string
	err := db.QueryRow(
		`SELECT target_entity_id FROM entity_relation
		 WHERE relationship_type = ? AND source_entity_type = ? AND source_entity_id = ?
		   AND target_entity_type = ?`,
		relType, srcType, srcID, tgtType,
	).Scan(&targetID)
	if err != nil {
		return ""
	}
	return targetID
}

// GetRelationSourceID returns the source entity ID for a specific relation pattern.
// This is the reverse of GetRelationTargetID: given a target, find the source.
// Returns empty string if no relation found.
func (db *DB) GetRelationSourceID(relType, srcType, tgtType, tgtID string) string {
	var sourceID string
	err := db.QueryRow(
		`SELECT source_entity_id FROM entity_relation
		 WHERE relationship_type = ? AND source_entity_type = ? AND target_entity_type = ?
		   AND target_entity_id = ?`,
		relType, srcType, tgtType, tgtID,
	).Scan(&sourceID)
	if err != nil {
		return ""
	}
	return sourceID
}

// SetRelation creates or replaces a single-valued relation (e.g., task belongs_to endeavour).
// Deletes any existing relation of the same type from the same source to the same target type,
// then creates the new one. If targetID is empty, just deletes.
func (db *DB) SetRelation(relType, srcType, srcID, tgtType, tgtID, createdBy string) error {
	// Delete existing
	_, _ = db.Exec(
		`DELETE FROM entity_relation
		 WHERE relationship_type = ? AND source_entity_type = ? AND source_entity_id = ?
		   AND target_entity_type = ?`,
		relType, srcType, srcID, tgtType,
	)

	if tgtID == "" {
		return nil
	}

	id := generateID("rel")
	now := UTCNow()
	var createdByVal *string
	if createdBy != "" {
		createdByVal = &createdBy
	}

	_, err := db.Exec(
		`INSERT INTO entity_relation (id, relationship_type, source_entity_type, source_entity_id,
		 target_entity_type, target_entity_id, created_by, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		id, relType, srcType, srcID, tgtType, tgtID, createdByVal, now.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("set relation: %w", err)
	}
	return nil
}
