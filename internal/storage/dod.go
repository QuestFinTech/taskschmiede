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

// DodPolicy represents a Definition of Done policy.
type DodPolicy struct {
	ID            string
	Name          string
	Description   string
	Version       int
	PredecessorID string
	Origin        string // template, custom, derived
	Conditions    []DodCondition
	Strictness    string // all, n_of
	Quorum        int
	Scope         string // task
	Status        string // active, archived
	CreatedBy     string
	Metadata      map[string]interface{}
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// DodCondition represents a single condition within a DoD policy.
type DodCondition struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"`
	Label    string                 `json:"label"`
	Params   map[string]interface{} `json:"params"`
	Required bool                   `json:"required"`
}

// DodEndorsement records a resource's endorsement of a DoD policy version.
type DodEndorsement struct {
	ID            string
	PolicyID      string
	PolicyVersion int
	ResourceID    string
	EndeavourID   string
	Status        string // active, superseded, withdrawn
	EndorsedAt    time.Time
	SupersededAt  *time.Time
	CreatedAt     time.Time
}

// ListDodPoliciesOpts holds filters for listing DoD policies.
type ListDodPoliciesOpts struct {
	Status string
	Origin string
	Scope  string
	Search string
	Limit  int
	Offset int
}

// UpdateDodPolicyFields holds the mutable fields of a DoD policy.
// Conditions, version, and strictness are changed via NewVersion, not Update.
type UpdateDodPolicyFields struct {
	Name        *string
	Description *string
	Status      *string
	Metadata    map[string]interface{}
}

// ListDodEndorsementsOpts holds filters for listing endorsements.
type ListDodEndorsementsOpts struct {
	PolicyID     string
	ResourceID   string
	EndeavourID  string
	EndeavourIDs []string // RBAC: nil = no restriction; empty = no access
	Status       string
	Limit        int
	Offset       int
}

// DoD error sentinels.
var (
	// ErrDodPolicyNotFound is returned when a DoD policy cannot be found by its ID.
	ErrDodPolicyNotFound = errors.New("dod policy not found")
	// ErrDodEndorsementNotFound is returned when a DoD endorsement cannot be found.
	ErrDodEndorsementNotFound = errors.New("dod endorsement not found")
	// ErrDodEndorsementExists is returned when an active endorsement already exists.
	ErrDodEndorsementExists = errors.New("active endorsement already exists")
)

// CreateDodPolicy creates a new DoD policy.
func (db *DB) CreateDodPolicy(name, description, origin, createdBy string, conditions []DodCondition, strictness string, quorum int, scope string, version int, predecessorID string, metadata map[string]interface{}) (*DodPolicy, error) {
	id := generateID("dod")

	condJSON, err := json.Marshal(conditions)
	if err != nil {
		return nil, fmt.Errorf("marshal conditions: %w", err)
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
	var quorumVal *int
	if quorum > 0 {
		quorumVal = &quorum
	}

	now := UTCNow()
	nowStr := now.Format(time.RFC3339)

	_, err = db.Exec(
		`INSERT INTO dod_policy (id, name, description, version, predecessor_id, origin,
		 conditions, strictness, quorum, scope, status, created_by, metadata, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'active', ?, ?, ?, ?)`,
		id, name, descVal, version, predVal, origin,
		string(condJSON), strictness, quorumVal, scope, createdBy, metadataJSON, nowStr, nowStr,
	)
	if err != nil {
		return nil, fmt.Errorf("insert dod policy: %w", err)
	}

	return &DodPolicy{
		ID:            id,
		Name:          name,
		Description:   description,
		Version:       version,
		PredecessorID: predecessorID,
		Origin:        origin,
		Conditions:    conditions,
		Strictness:    strictness,
		Quorum:        quorum,
		Scope:         scope,
		Status:        "active",
		CreatedBy:     createdBy,
		Metadata:      metadata,
		CreatedAt:     now,
		UpdatedAt:     now,
	}, nil
}

// GetDodPolicy retrieves a DoD policy by ID.
func (db *DB) GetDodPolicy(id string) (*DodPolicy, error) {
	var p DodPolicy
	var description sql.NullString
	var predecessorID sql.NullString
	var condJSON string
	var quorum sql.NullInt64
	var metadataJSON string
	var createdAt, updatedAt string

	err := db.QueryRow(
		`SELECT id, name, description, version, predecessor_id, origin,
		        conditions, strictness, quorum, scope, status, created_by,
		        metadata, created_at, updated_at
		 FROM dod_policy WHERE id = ?`, id,
	).Scan(&p.ID, &p.Name, &description, &p.Version, &predecessorID, &p.Origin,
		&condJSON, &p.Strictness, &quorum, &p.Scope, &p.Status, &p.CreatedBy,
		&metadataJSON, &createdAt, &updatedAt)

	if err == sql.ErrNoRows {
		return nil, ErrDodPolicyNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query dod policy: %w", err)
	}

	if description.Valid {
		p.Description = description.String
	}
	if predecessorID.Valid {
		p.PredecessorID = predecessorID.String
	}
	if quorum.Valid {
		p.Quorum = int(quorum.Int64)
	}
	_ = json.Unmarshal([]byte(condJSON), &p.Conditions)
	_ = json.Unmarshal([]byte(metadataJSON), &p.Metadata)
	p.CreatedAt = ParseDBTime(createdAt)
	p.UpdatedAt = ParseDBTime(updatedAt)

	return &p, nil
}

// ListDodPolicies queries DoD policies with filters.
func (db *DB) ListDodPolicies(opts ListDodPoliciesOpts) ([]*DodPolicy, int, error) {
	query := `SELECT id, name, description, version, predecessor_id, origin,
	                 conditions, strictness, quorum, scope, status, created_by,
	                 metadata, created_at, updated_at
	          FROM dod_policy`
	countQuery := `SELECT COUNT(*) FROM dod_policy`

	var conditions []string
	var params []interface{}

	if opts.Status != "" {
		conditions = append(conditions, "status = ?")
		params = append(params, opts.Status)
	}
	if opts.Origin != "" {
		conditions = append(conditions, "origin = ?")
		params = append(params, opts.Origin)
	}
	if opts.Scope != "" {
		conditions = append(conditions, "scope = ?")
		params = append(params, opts.Scope)
	}
	if opts.Search != "" {
		conditions = append(conditions, "(name LIKE ? ESCAPE '\\' OR description LIKE ? ESCAPE '\\')")
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
	query += ` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	params = append(params, opts.Limit, opts.Offset)

	rows, err := db.Query(query, params...)
	if err != nil {
		return nil, 0, fmt.Errorf("query dod policies: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var policies []*DodPolicy
	for rows.Next() {
		p, err := scanDodPolicy(rows)
		if err != nil {
			return nil, 0, err
		}
		policies = append(policies, p)
	}

	return policies, total, nil
}

// scanDodPolicy scans a single DodPolicy from a row scanner.
func scanDodPolicy(rows *sql.Rows) (*DodPolicy, error) {
	var p DodPolicy
	var description sql.NullString
	var predecessorID sql.NullString
	var condJSON string
	var quorum sql.NullInt64
	var metadataJSON string
	var createdAt, updatedAt string

	if err := rows.Scan(&p.ID, &p.Name, &description, &p.Version, &predecessorID, &p.Origin,
		&condJSON, &p.Strictness, &quorum, &p.Scope, &p.Status, &p.CreatedBy,
		&metadataJSON, &createdAt, &updatedAt); err != nil {
		return nil, fmt.Errorf("scan dod policy: %w", err)
	}

	if description.Valid {
		p.Description = description.String
	}
	if predecessorID.Valid {
		p.PredecessorID = predecessorID.String
	}
	if quorum.Valid {
		p.Quorum = int(quorum.Int64)
	}
	_ = json.Unmarshal([]byte(condJSON), &p.Conditions)
	_ = json.Unmarshal([]byte(metadataJSON), &p.Metadata)
	p.CreatedAt = ParseDBTime(createdAt)
	p.UpdatedAt = ParseDBTime(updatedAt)

	return &p, nil
}

// UpdateDodPolicy applies partial updates to a DoD policy.
func (db *DB) UpdateDodPolicy(id string, fields UpdateDodPolicyFields) ([]string, error) {
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

	setClauses = append(setClauses, "updated_at = ?")
	params = append(params, UTCNow().Format(time.RFC3339))

	query := fmt.Sprintf("UPDATE dod_policy SET %s WHERE id = ?", strings.Join(setClauses, ", "))
	params = append(params, id)

	result, err := db.Exec(query, params...)
	if err != nil {
		return nil, fmt.Errorf("update dod policy: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return nil, ErrDodPolicyNotFound
	}

	return updatedFields, nil
}

// GetDodPolicyLineage walks the predecessor chain using a recursive CTE.
// Returns the full lineage from oldest ancestor to newest descendant.
func (db *DB) GetDodPolicyLineage(id string) ([]*DodPolicy, error) {
	rows, err := db.Query(
		`WITH RECURSIVE lineage(id) AS (
		     SELECT id FROM dod_policy WHERE id = ?
		     UNION ALL
		     SELECT p.predecessor_id FROM dod_policy p
		     INNER JOIN lineage l ON l.id = p.id
		     WHERE p.predecessor_id IS NOT NULL
		 )
		 SELECT dp.id, dp.name, dp.description, dp.version, dp.predecessor_id, dp.origin,
		        dp.conditions, dp.strictness, dp.quorum, dp.scope, dp.status, dp.created_by,
		        dp.metadata, dp.created_at, dp.updated_at
		 FROM dod_policy dp
		 INNER JOIN lineage l ON l.id = dp.id
		 ORDER BY dp.version ASC`,
		id,
	)
	if err != nil {
		return nil, fmt.Errorf("query dod policy lineage: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var policies []*DodPolicy
	for rows.Next() {
		p, err := scanDodPolicy(rows)
		if err != nil {
			return nil, err
		}
		policies = append(policies, p)
	}

	if len(policies) == 0 {
		return nil, ErrDodPolicyNotFound
	}

	return policies, nil
}

// CreateDodEndorsement records a resource's endorsement of a DoD policy version.
func (db *DB) CreateDodEndorsement(policyID string, policyVersion int, resourceID, endeavourID string) (*DodEndorsement, error) {
	id := generateID("doe")
	now := UTCNow()
	nowStr := now.Format(time.RFC3339)

	_, err := db.Exec(
		`INSERT INTO dod_endorsement (id, policy_id, policy_version, resource_id, endeavour_id,
		 status, endorsed_at, created_at)
		 VALUES (?, ?, ?, ?, ?, 'active', ?, ?)`,
		id, policyID, policyVersion, resourceID, endeavourID, nowStr, nowStr,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			return nil, ErrDodEndorsementExists
		}
		return nil, fmt.Errorf("insert dod endorsement: %w", err)
	}

	return &DodEndorsement{
		ID:            id,
		PolicyID:      policyID,
		PolicyVersion: policyVersion,
		ResourceID:    resourceID,
		EndeavourID:   endeavourID,
		Status:        "active",
		EndorsedAt:    now,
		CreatedAt:     now,
	}, nil
}

// GetDodEndorsement retrieves an endorsement by ID.
func (db *DB) GetDodEndorsement(id string) (*DodEndorsement, error) {
	var e DodEndorsement
	var endorsedAt, createdAt string
	var supersededAt sql.NullString

	err := db.QueryRow(
		`SELECT id, policy_id, policy_version, resource_id, endeavour_id,
		        status, endorsed_at, superseded_at, created_at
		 FROM dod_endorsement WHERE id = ?`, id,
	).Scan(&e.ID, &e.PolicyID, &e.PolicyVersion, &e.ResourceID, &e.EndeavourID,
		&e.Status, &endorsedAt, &supersededAt, &createdAt)

	if err == sql.ErrNoRows {
		return nil, ErrDodEndorsementNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query dod endorsement: %w", err)
	}

	e.EndorsedAt = ParseDBTime(endorsedAt)
	e.CreatedAt = ParseDBTime(createdAt)
	if supersededAt.Valid {
		t := ParseDBTime(supersededAt.String)
		e.SupersededAt = &t
	}

	return &e, nil
}

// ListDodEndorsements queries endorsements with filters.
func (db *DB) ListDodEndorsements(opts ListDodEndorsementsOpts) ([]*DodEndorsement, int, error) {
	// Non-nil but empty EndeavourIDs means "no access to any endeavour".
	if opts.EndeavourIDs != nil && len(opts.EndeavourIDs) == 0 {
		return nil, 0, nil
	}

	query := `SELECT id, policy_id, policy_version, resource_id, endeavour_id,
	                 status, endorsed_at, superseded_at, created_at
	          FROM dod_endorsement`
	countQuery := `SELECT COUNT(*) FROM dod_endorsement`

	var conditions []string
	var params []interface{}

	if opts.PolicyID != "" {
		conditions = append(conditions, "policy_id = ?")
		params = append(params, opts.PolicyID)
	}
	if opts.ResourceID != "" {
		conditions = append(conditions, "resource_id = ?")
		params = append(params, opts.ResourceID)
	}
	if opts.EndeavourID != "" {
		conditions = append(conditions, "endeavour_id = ?")
		params = append(params, opts.EndeavourID)
	}
	if len(opts.EndeavourIDs) > 0 {
		placeholders := make([]string, len(opts.EndeavourIDs))
		for i := range opts.EndeavourIDs {
			placeholders[i] = "?"
			params = append(params, opts.EndeavourIDs[i])
		}
		conditions = append(conditions, "endeavour_id IN ("+strings.Join(placeholders, ", ")+")")
	}
	if opts.Status != "" {
		conditions = append(conditions, "status = ?")
		params = append(params, opts.Status)
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
		return nil, 0, fmt.Errorf("query dod endorsements: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var endorsements []*DodEndorsement
	for rows.Next() {
		var e DodEndorsement
		var endorsedAt, createdAt string
		var supersededAt sql.NullString

		if err := rows.Scan(&e.ID, &e.PolicyID, &e.PolicyVersion, &e.ResourceID, &e.EndeavourID,
			&e.Status, &endorsedAt, &supersededAt, &createdAt); err != nil {
			return nil, 0, fmt.Errorf("scan dod endorsement: %w", err)
		}

		e.EndorsedAt = ParseDBTime(endorsedAt)
		e.CreatedAt = ParseDBTime(createdAt)
		if supersededAt.Valid {
			t := ParseDBTime(supersededAt.String)
			e.SupersededAt = &t
		}

		endorsements = append(endorsements, &e)
	}

	return endorsements, total, nil
}

// SupersedeDodEndorsements marks all active endorsements for a policy as superseded.
// Used when a policy version changes.
func (db *DB) SupersedeDodEndorsements(policyID string, oldVersion int) (int64, error) {
	now := UTCNow().Format(time.RFC3339)
	result, err := db.Exec(
		`UPDATE dod_endorsement SET status = 'superseded', superseded_at = ?
		 WHERE policy_id = ? AND policy_version = ? AND status = 'active'`,
		now, policyID, oldVersion,
	)
	if err != nil {
		return 0, fmt.Errorf("supersede dod endorsements: %w", err)
	}
	affected, _ := result.RowsAffected()
	return affected, nil
}

// WithdrawDodEndorsement withdraws an endorsement by setting status to 'withdrawn'.
func (db *DB) WithdrawDodEndorsement(id string) error {
	result, err := db.Exec(
		`UPDATE dod_endorsement SET status = 'withdrawn' WHERE id = ? AND status = 'active'`,
		id,
	)
	if err != nil {
		return fmt.Errorf("withdraw dod endorsement: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return ErrDodEndorsementNotFound
	}
	return nil
}

// GetActiveEndorsement returns the active endorsement for a resource+endeavour+policy combination.
// Returns nil, nil if no active endorsement exists.
func (db *DB) GetActiveEndorsement(resourceID, endeavourID, policyID string) (*DodEndorsement, error) {
	var e DodEndorsement
	var endorsedAt, createdAt string
	var supersededAt sql.NullString

	err := db.QueryRow(
		`SELECT id, policy_id, policy_version, resource_id, endeavour_id,
		        status, endorsed_at, superseded_at, created_at
		 FROM dod_endorsement
		 WHERE resource_id = ? AND endeavour_id = ? AND policy_id = ? AND status = 'active'`,
		resourceID, endeavourID, policyID,
	).Scan(&e.ID, &e.PolicyID, &e.PolicyVersion, &e.ResourceID, &e.EndeavourID,
		&e.Status, &endorsedAt, &supersededAt, &createdAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query active endorsement: %w", err)
	}

	e.EndorsedAt = ParseDBTime(endorsedAt)
	e.CreatedAt = ParseDBTime(createdAt)
	if supersededAt.Valid {
		t := ParseDBTime(supersededAt.String)
		e.SupersededAt = &t
	}

	return &e, nil
}
