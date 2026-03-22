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

// Approval represents a formal approval decision on an entity.
// Approvals are immutable -- once recorded, they cannot be edited or deleted.
// A new approval from the same approver on the same entity supersedes the previous one.
type Approval struct {
	ID           string
	EntityType   string
	EntityID     string
	ApproverID   string
	ApproverName string // denormalized from resource table
	Role         string
	Verdict      string
	Comment      string
	Metadata     map[string]interface{}
	CreatedAt    time.Time
}

// ListApprovalsOpts holds filters for listing approvals on an entity.
type ListApprovalsOpts struct {
	EntityType   string // optional when EndeavourIDs is set
	EntityID     string // optional when EndeavourIDs is set
	ApproverID   string
	Verdict      string
	Role         string
	EndeavourIDs []string // RBAC: nil = no restriction; empty = no access
	Limit        int
	Offset       int
}

// ErrApprovalNotFound is returned when an approval cannot be found by its ID.
var ErrApprovalNotFound = errors.New("approval not found")

// CreateApproval inserts a new approval record.
func (db *DB) CreateApproval(entityType, entityID, approverID, role, verdict, comment string, metadata map[string]interface{}) (*Approval, error) {
	id := generateID("apr")

	metadataJSON := "{}"
	if metadata != nil {
		b, err := json.Marshal(metadata)
		if err != nil {
			return nil, fmt.Errorf("marshal metadata: %w", err)
		}
		metadataJSON = string(b)
	}

	var roleVal *string
	if role != "" {
		roleVal = &role
	}

	var commentVal *string
	if comment != "" {
		commentVal = &comment
	}

	now := UTCNow()
	nowStr := now.Format(time.RFC3339)

	_, err := db.Exec(
		`INSERT INTO approval (id, entity_type, entity_id, approver_id, role, verdict, comment, metadata, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, entityType, entityID, approverID, roleVal, verdict, commentVal, metadataJSON, nowStr,
	)
	if err != nil {
		return nil, fmt.Errorf("insert approval: %w", err)
	}

	return &Approval{
		ID:         id,
		EntityType: entityType,
		EntityID:   entityID,
		ApproverID: approverID,
		Role:       role,
		Verdict:    verdict,
		Comment:    comment,
		Metadata:   metadata,
		CreatedAt:  now,
	}, nil
}

// GetApproval retrieves an approval by ID, joining resource for approver name.
func (db *DB) GetApproval(id string) (*Approval, error) {
	var a Approval
	var approverName, role, comment sql.NullString
	var metadataJSON string
	var createdAt string

	err := db.QueryRow(
		`SELECT a.id, a.entity_type, a.entity_id, a.approver_id, COALESCE(u.name, r.name),
		        a.role, a.verdict, a.comment, a.metadata, a.created_at
		 FROM approval a
		 LEFT JOIN resource r ON a.approver_id = r.id
		 LEFT JOIN user u ON u.resource_id = a.approver_id
		 WHERE a.id = ?`,
		id,
	).Scan(&a.ID, &a.EntityType, &a.EntityID, &a.ApproverID, &approverName,
		&role, &a.Verdict, &comment, &metadataJSON, &createdAt)

	if err == sql.ErrNoRows {
		return nil, ErrApprovalNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query approval: %w", err)
	}

	if approverName.Valid {
		a.ApproverName = approverName.String
	}
	if role.Valid {
		a.Role = role.String
	}
	if comment.Valid {
		a.Comment = comment.String
	}
	_ = json.Unmarshal([]byte(metadataJSON), &a.Metadata)
	a.CreatedAt = ParseDBTime(createdAt)

	return &a, nil
}

// ListApprovals returns approvals for an entity, ordered chronologically (newest first).
func (db *DB) ListApprovals(opts ListApprovalsOpts) ([]*Approval, int, error) {
	// Non-nil but empty EndeavourIDs means "no access to any endeavour".
	if opts.EndeavourIDs != nil && len(opts.EndeavourIDs) == 0 {
		return nil, 0, nil
	}

	query := `SELECT a.id, a.entity_type, a.entity_id, a.approver_id, COALESCE(u.name, r.name),
	                 a.role, a.verdict, a.comment, a.metadata, a.created_at
	          FROM approval a
	          LEFT JOIN resource r ON a.approver_id = r.id
	          LEFT JOIN user u ON u.resource_id = a.approver_id`
	countQuery := `SELECT COUNT(*) FROM approval a`

	var conditions []string
	var params []interface{}
	var countParams []interface{}

	if opts.EntityType != "" {
		conditions = append(conditions, "a.entity_type = ?")
		params = append(params, opts.EntityType)
		countParams = append(countParams, opts.EntityType)
	}
	if opts.EntityID != "" {
		conditions = append(conditions, "a.entity_id = ?")
		params = append(params, opts.EntityID)
		countParams = append(countParams, opts.EntityID)
	}
	if opts.ApproverID != "" {
		conditions = append(conditions, "a.approver_id = ?")
		params = append(params, opts.ApproverID)
		countParams = append(countParams, opts.ApproverID)
	}
	if opts.Verdict != "" {
		conditions = append(conditions, "a.verdict = ?")
		params = append(params, opts.Verdict)
		countParams = append(countParams, opts.Verdict)
	}
	if opts.Role != "" {
		conditions = append(conditions, "a.role = ?")
		params = append(params, opts.Role)
		countParams = append(countParams, opts.Role)
	}
	if len(opts.EndeavourIDs) > 0 {
		placeholders := make([]string, len(opts.EndeavourIDs))
		for i := range opts.EndeavourIDs {
			placeholders[i] = "?"
		}
		inClause := strings.Join(placeholders, ", ")
		// Scope approvals to entities belonging to accessible endeavours.
		// All entity types use entity_relation belongs_to for endeavour linkage.
		scopeSQL := `(
			(a.entity_type = 'endeavour' AND a.entity_id IN (` + inClause + `))
			OR a.entity_id IN (SELECT er2.source_entity_id FROM entity_relation er2 WHERE er2.relationship_type = 'belongs_to' AND er2.target_entity_type = 'endeavour' AND er2.target_entity_id IN (` + inClause + `))
			OR a.entity_type = 'organization'
		)`
		conditions = append(conditions, scopeSQL)
		for i := 0; i < 2; i++ {
			for _, id := range opts.EndeavourIDs {
				params = append(params, id)
				countParams = append(countParams, id)
			}
		}
	}

	where := ""
	if len(conditions) > 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}
	query += where
	countQuery += where

	var total int
	_ = db.QueryRow(countQuery, countParams...).Scan(&total)

	if opts.Limit <= 0 {
		opts.Limit = 50
	}
	query += ` ORDER BY a.created_at DESC LIMIT ? OFFSET ?`
	params = append(params, opts.Limit, opts.Offset)

	rows, err := db.Query(query, params...)
	if err != nil {
		return nil, 0, fmt.Errorf("query approvals: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var approvals []*Approval
	for rows.Next() {
		var a Approval
		var approverName, role, comment sql.NullString
		var metadataJSON string
		var createdAt string

		if err := rows.Scan(&a.ID, &a.EntityType, &a.EntityID, &a.ApproverID, &approverName,
			&role, &a.Verdict, &comment, &metadataJSON, &createdAt); err != nil {
			return nil, 0, fmt.Errorf("scan approval: %w", err)
		}

		if approverName.Valid {
			a.ApproverName = approverName.String
		}
		if role.Valid {
			a.Role = role.String
		}
		if comment.Valid {
			a.Comment = comment.String
		}
		_ = json.Unmarshal([]byte(metadataJSON), &a.Metadata)
		a.CreatedAt = ParseDBTime(createdAt)

		approvals = append(approvals, &a)
	}

	return approvals, total, nil
}
