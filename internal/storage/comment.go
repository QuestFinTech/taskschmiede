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

// Comment represents a discussion entry attached to an entity.
type Comment struct {
	ID         string
	EntityType string
	EntityID   string
	AuthorID   string
	AuthorName string // denormalized from resource table
	ReplyToID  string
	Content    string
	Metadata   map[string]interface{}
	DeletedAt  *time.Time
	EditedAt   *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// ListCommentsOpts holds filters for listing comments on an entity.
type ListCommentsOpts struct {
	EntityType   string // optional when EndeavourIDs is set
	EntityID     string // optional when EndeavourIDs is set
	AuthorID     string
	EndeavourIDs []string // RBAC: nil = no restriction; empty = no access
	Limit        int
	Offset       int
}

// UpdateCommentFields holds the fields that can be updated on a comment.
// Only non-nil fields are applied.
type UpdateCommentFields struct {
	Content  *string
	Metadata map[string]interface{}
}

// ErrCommentNotFound is returned when a comment cannot be found by its ID.
var ErrCommentNotFound = errors.New("comment not found")

// CreateComment inserts a new comment attached to an entity.
func (db *DB) CreateComment(entityType, entityID, authorID, replyToID, content string, metadata map[string]interface{}) (*Comment, error) {
	id := generateID("cmt")

	metadataJSON := "{}"
	if metadata != nil {
		b, err := json.Marshal(metadata)
		if err != nil {
			return nil, fmt.Errorf("marshal metadata: %w", err)
		}
		metadataJSON = string(b)
	}

	var replyVal *string
	if replyToID != "" {
		replyVal = &replyToID
	}

	now := UTCNow()
	nowStr := now.Format(time.RFC3339)

	_, err := db.Exec(
		`INSERT INTO comment (id, entity_type, entity_id, author_id, reply_to_id, content, metadata, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, entityType, entityID, authorID, replyVal, content, metadataJSON, nowStr, nowStr,
	)
	if err != nil {
		return nil, fmt.Errorf("insert comment: %w", err)
	}

	return &Comment{
		ID:         id,
		EntityType: entityType,
		EntityID:   entityID,
		AuthorID:   authorID,
		ReplyToID:  replyToID,
		Content:    content,
		Metadata:   metadata,
		CreatedAt:  now,
		UpdatedAt:  now,
	}, nil
}

// GetComment retrieves a comment by ID, joining resource for author name.
// Soft-deleted comments have their content redacted to "[deleted]".
func (db *DB) GetComment(id string) (*Comment, error) {
	var c Comment
	var replyToID, authorName sql.NullString
	var deletedAt, editedAt sql.NullString
	var metadataJSON string
	var createdAt, updatedAt string

	err := db.QueryRow(
		`SELECT c.id, c.entity_type, c.entity_id, c.author_id, COALESCE(u.name, r.name),
		        c.reply_to_id, c.content, c.metadata,
		        c.deleted_at, c.edited_at, c.created_at, c.updated_at
		 FROM comment c
		 LEFT JOIN resource r ON c.author_id = r.id
		 LEFT JOIN user u ON u.resource_id = c.author_id
		 WHERE c.id = ?`,
		id,
	).Scan(&c.ID, &c.EntityType, &c.EntityID, &c.AuthorID, &authorName,
		&replyToID, &c.Content, &metadataJSON,
		&deletedAt, &editedAt, &createdAt, &updatedAt)

	if err == sql.ErrNoRows {
		return nil, ErrCommentNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query comment: %w", err)
	}

	if authorName.Valid {
		c.AuthorName = authorName.String
	}
	if replyToID.Valid {
		c.ReplyToID = replyToID.String
	}
	if deletedAt.Valid {
		t := ParseDBTime(deletedAt.String)
		c.DeletedAt = &t
		c.Content = "[deleted]"
		c.Metadata = nil
	} else {
		_ = json.Unmarshal([]byte(metadataJSON), &c.Metadata)
	}
	if editedAt.Valid {
		t := ParseDBTime(editedAt.String)
		c.EditedAt = &t
	}
	c.CreatedAt = ParseDBTime(createdAt)
	c.UpdatedAt = ParseDBTime(updatedAt)

	return &c, nil
}

// ListComments returns comments for an entity, ordered chronologically (oldest first).
// Soft-deleted comments have their content redacted.
func (db *DB) ListComments(opts ListCommentsOpts) ([]*Comment, int, error) {
	// Non-nil but empty EndeavourIDs means "no access to any endeavour".
	if opts.EndeavourIDs != nil && len(opts.EndeavourIDs) == 0 {
		return nil, 0, nil
	}

	query := `SELECT c.id, c.entity_type, c.entity_id, c.author_id, COALESCE(u.name, r.name),
	                 c.reply_to_id, c.content, c.metadata,
	                 c.deleted_at, c.edited_at, c.created_at, c.updated_at
	          FROM comment c
	          LEFT JOIN resource r ON c.author_id = r.id
	          LEFT JOIN user u ON u.resource_id = c.author_id`
	countQuery := `SELECT COUNT(*) FROM comment c`

	var conditions []string
	var params []interface{}
	var countParams []interface{}

	if opts.EntityType != "" {
		conditions = append(conditions, "c.entity_type = ?")
		params = append(params, opts.EntityType)
		countParams = append(countParams, opts.EntityType)
	}
	if opts.EntityID != "" {
		conditions = append(conditions, "c.entity_id = ?")
		params = append(params, opts.EntityID)
		countParams = append(countParams, opts.EntityID)
	}
	if opts.AuthorID != "" {
		conditions = append(conditions, "c.author_id = ?")
		params = append(params, opts.AuthorID)
		countParams = append(countParams, opts.AuthorID)
	}
	if len(opts.EndeavourIDs) > 0 {
		placeholders := make([]string, len(opts.EndeavourIDs))
		for i := range opts.EndeavourIDs {
			placeholders[i] = "?"
		}
		inClause := strings.Join(placeholders, ", ")
		// Scope comments to entities belonging to accessible endeavours.
		// All entity types use entity_relation belongs_to for endeavour linkage.
		scopeSQL := `(
			(c.entity_type = 'endeavour' AND c.entity_id IN (` + inClause + `))
			OR c.entity_id IN (SELECT er2.source_entity_id FROM entity_relation er2 WHERE er2.relationship_type = 'belongs_to' AND er2.target_entity_type = 'endeavour' AND er2.target_entity_id IN (` + inClause + `))
			OR c.entity_type = 'organization'
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
	query += ` ORDER BY c.created_at ASC LIMIT ? OFFSET ?`
	params = append(params, opts.Limit, opts.Offset)

	rows, err := db.Query(query, params...)
	if err != nil {
		return nil, 0, fmt.Errorf("query comments: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var comments []*Comment
	for rows.Next() {
		var c Comment
		var replyToID, authorName sql.NullString
		var deletedAt, editedAt sql.NullString
		var metadataJSON string
		var createdAt, updatedAt string

		if err := rows.Scan(&c.ID, &c.EntityType, &c.EntityID, &c.AuthorID, &authorName,
			&replyToID, &c.Content, &metadataJSON,
			&deletedAt, &editedAt, &createdAt, &updatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan comment: %w", err)
		}

		if authorName.Valid {
			c.AuthorName = authorName.String
		}
		if replyToID.Valid {
			c.ReplyToID = replyToID.String
		}
		if deletedAt.Valid {
			t := ParseDBTime(deletedAt.String)
			c.DeletedAt = &t
			c.Content = "[deleted]"
			c.Metadata = nil
		} else {
			_ = json.Unmarshal([]byte(metadataJSON), &c.Metadata)
		}
		if editedAt.Valid {
			t := ParseDBTime(editedAt.String)
			c.EditedAt = &t
		}
		c.CreatedAt = ParseDBTime(createdAt)
		c.UpdatedAt = ParseDBTime(updatedAt)

		comments = append(comments, &c)
	}

	return comments, total, nil
}

// GetCommentReplies returns all direct replies to a comment, ordered chronologically.
func (db *DB) GetCommentReplies(parentID string) ([]*Comment, error) {
	rows, err := db.Query(
		`SELECT c.id, c.entity_type, c.entity_id, c.author_id, COALESCE(u.name, r.name),
		        c.reply_to_id, c.content, c.metadata,
		        c.deleted_at, c.edited_at, c.created_at, c.updated_at
		 FROM comment c
		 LEFT JOIN resource r ON c.author_id = r.id
		 LEFT JOIN user u ON u.resource_id = c.author_id
		 WHERE c.reply_to_id = ?
		 ORDER BY c.created_at ASC`,
		parentID,
	)
	if err != nil {
		return nil, fmt.Errorf("query comment replies: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var replies []*Comment
	for rows.Next() {
		var c Comment
		var replyToID, authorName sql.NullString
		var deletedAt, editedAt sql.NullString
		var metadataJSON string
		var createdAt, updatedAt string

		if err := rows.Scan(&c.ID, &c.EntityType, &c.EntityID, &c.AuthorID, &authorName,
			&replyToID, &c.Content, &metadataJSON,
			&deletedAt, &editedAt, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan comment reply: %w", err)
		}

		if authorName.Valid {
			c.AuthorName = authorName.String
		}
		if replyToID.Valid {
			c.ReplyToID = replyToID.String
		}
		if deletedAt.Valid {
			t := ParseDBTime(deletedAt.String)
			c.DeletedAt = &t
			c.Content = "[deleted]"
			c.Metadata = nil
		} else {
			_ = json.Unmarshal([]byte(metadataJSON), &c.Metadata)
		}
		if editedAt.Valid {
			t := ParseDBTime(editedAt.String)
			c.EditedAt = &t
		}
		c.CreatedAt = ParseDBTime(createdAt)
		c.UpdatedAt = ParseDBTime(updatedAt)

		replies = append(replies, &c)
	}

	return replies, nil
}

// UpdateComment applies partial updates to a comment. Sets edited_at on content change.
// Returns the list of updated field names.
func (db *DB) UpdateComment(id string, fields UpdateCommentFields) ([]string, error) {
	var setClauses []string
	var params []interface{}
	var updatedFields []string

	if fields.Content != nil {
		setClauses = append(setClauses, "content = ?")
		params = append(params, *fields.Content)
		updatedFields = append(updatedFields, "content")

		setClauses = append(setClauses, "edited_at = ?")
		params = append(params, UTCNow().Format(time.RFC3339))
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

	query := fmt.Sprintf("UPDATE comment SET %s WHERE id = ? AND deleted_at IS NULL", strings.Join(setClauses, ", "))
	params = append(params, id)

	result, err := db.Exec(query, params...)
	if err != nil {
		return nil, fmt.Errorf("update comment: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return nil, ErrCommentNotFound
	}

	return updatedFields, nil
}

// SoftDeleteComment sets deleted_at on a comment.
func (db *DB) SoftDeleteComment(id string) error {
	result, err := db.Exec(
		`UPDATE comment SET deleted_at = ?, updated_at = ? WHERE id = ? AND deleted_at IS NULL`,
		UTCNow().Format(time.RFC3339), UTCNow().Format(time.RFC3339), id,
	)
	if err != nil {
		return fmt.Errorf("soft delete comment: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return ErrCommentNotFound
	}

	return nil
}

// CountComments returns the number of non-deleted comments on an entity.
func (db *DB) CountComments(entityType, entityID string) (int, error) {
	var count int
	err := db.QueryRow(
		`SELECT COUNT(*) FROM comment WHERE entity_type = ? AND entity_id = ? AND deleted_at IS NULL`,
		entityType, entityID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count comments: %w", err)
	}
	return count, nil
}
