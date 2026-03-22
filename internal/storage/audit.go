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
	"fmt"
	"strings"
	"time"
)

// AuditLogEntry holds the data for inserting an audit log row.
type AuditLogEntry struct {
	ID         string
	Action     string
	ActorID    string
	ActorType  string
	Resource   string
	Method     string
	Endpoint   string
	StatusCode int
	IP         string
	Source     string
	DurationMs int64
	Metadata   map[string]interface{}
}

// AuditLogRecord represents a stored audit log entry.
type AuditLogRecord struct {
	ID         string                 `json:"id"`
	Action     string                 `json:"action"`
	ActorID    string                 `json:"actor_id"`
	ActorType  string                 `json:"actor_type"`
	Resource   string                 `json:"resource,omitempty"`
	Method     string                 `json:"method,omitempty"`
	Endpoint   string                 `json:"endpoint,omitempty"`
	StatusCode int                    `json:"status_code,omitempty"`
	IP         string                 `json:"ip,omitempty"`
	Source     string                 `json:"source,omitempty"`
	DurationMs int64                  `json:"duration_ms,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt  time.Time              `json:"created_at"`
}

// ListAuditLogOpts holds filters for querying the audit log.
type ListAuditLogOpts struct {
	Action        string
	ExcludeAction string // Exclude entries with this action (e.g., "request")
	ActorID       string
	Resource      string
	IP            string
	Source        string
	StartTime     *time.Time
	EndTime       *time.Time
	Limit         int
	Offset        int
	BeforeID      string // Cursor: return entries with rowid < this entry's rowid
}

// CreateAuditLogBatch inserts multiple audit log entries in a transaction.
func (db *DB) CreateAuditLogBatch(entries []AuditLogEntry) error {
	if len(entries) == 0 {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	stmt, err := tx.Prepare(`INSERT INTO audit_log
		(id, action, actor_id, actor_type, resource, method, endpoint,
		 status_code, ip, source, duration_ms, metadata, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare statement: %w", err)
	}
	defer stmt.Close() //nolint:errcheck

	now := UTCNow().Format(time.RFC3339)
	for _, e := range entries {
		id := e.ID
		if id == "" {
			id = generateID("aud")
		}

		metadataJSON := "{}"
		if e.Metadata != nil {
			b, err := json.Marshal(e.Metadata)
			if err == nil {
				metadataJSON = string(b)
			}
		}

		_, err := stmt.Exec(
			id, e.Action, e.ActorID, e.ActorType, e.Resource,
			e.Method, e.Endpoint, e.StatusCode, e.IP, e.Source, e.DurationMs,
			metadataJSON, now,
		)
		if err != nil {
			return fmt.Errorf("insert audit entry: %w", err)
		}
	}

	return tx.Commit()
}

// ListAuditLog queries audit log entries with filters.
func (db *DB) ListAuditLog(opts ListAuditLogOpts) ([]*AuditLogRecord, int, error) {
	var conditions []string
	var args []interface{}

	if opts.Action != "" {
		conditions = append(conditions, "action = ?")
		args = append(args, opts.Action)
	}
	if opts.ExcludeAction != "" {
		conditions = append(conditions, "action != ?")
		args = append(args, opts.ExcludeAction)
	}
	if opts.ActorID != "" {
		conditions = append(conditions, "actor_id = ?")
		args = append(args, opts.ActorID)
	}
	if opts.Resource != "" {
		conditions = append(conditions, "resource = ?")
		args = append(args, opts.Resource)
	}
	if opts.IP != "" {
		conditions = append(conditions, "ip = ?")
		args = append(args, opts.IP)
	}
	if opts.Source != "" {
		conditions = append(conditions, "source = ?")
		args = append(args, opts.Source)
	}
	if opts.StartTime != nil {
		conditions = append(conditions, "created_at >= ?")
		args = append(args, opts.StartTime.UTC().Format(time.RFC3339))
	}
	if opts.EndTime != nil {
		conditions = append(conditions, "created_at <= ?")
		args = append(args, opts.EndTime.UTC().Format(time.RFC3339))
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = " WHERE " + strings.Join(conditions, " AND ")
	}

	// Count total (without cursor filter -- total reflects full result set)
	countQuery := "SELECT COUNT(*) FROM audit_log" + whereClause
	var total int
	if err := db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count audit log: %w", err)
	}

	// Cursor-based pagination: filter to rows older than the cursor entry.
	// This avoids the offset instability caused by new inserts between pages.
	if opts.BeforeID != "" {
		cursorCond := "rowid < (SELECT rowid FROM audit_log WHERE id = ?)"
		if len(conditions) > 0 {
			whereClause += " AND " + cursorCond
		} else {
			whereClause = " WHERE " + cursorCond
		}
		args = append(args, opts.BeforeID)
	}

	// Query with pagination
	limit := opts.Limit
	if limit <= 0 {
		limit = 50
	}
	offset := opts.Offset
	if offset < 0 {
		offset = 0
	}

	query := "SELECT id, action, actor_id, actor_type, resource, method, endpoint, status_code, ip, source, duration_ms, metadata, created_at FROM audit_log" +
		whereClause + " ORDER BY rowid DESC LIMIT ? OFFSET ?"
	queryArgs := append(args, limit, offset) //nolint:gocritic

	rows, err := db.Query(query, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("query audit log: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var records []*AuditLogRecord
	for rows.Next() {
		var r AuditLogRecord
		var metadataStr string
		var createdAtStr string
		var actorID, actorType, resource, method, endpoint, ip, source *string
		var statusCode, durationMs *int64

		err := rows.Scan(
			&r.ID, &r.Action, &actorID, &actorType, &resource,
			&method, &endpoint, &statusCode, &ip, &source, &durationMs,
			&metadataStr, &createdAtStr,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("scan audit log row: %w", err)
		}

		if actorID != nil {
			r.ActorID = *actorID
		}
		if actorType != nil {
			r.ActorType = *actorType
		}
		if resource != nil {
			r.Resource = *resource
		}
		if method != nil {
			r.Method = *method
		}
		if endpoint != nil {
			r.Endpoint = *endpoint
		}
		if statusCode != nil {
			r.StatusCode = int(*statusCode)
		}
		if ip != nil {
			r.IP = *ip
		}
		if source != nil {
			r.Source = *source
		}
		if durationMs != nil {
			r.DurationMs = *durationMs
		}

		r.CreatedAt = ParseDBTime(createdAtStr)

		if metadataStr != "" && metadataStr != "{}" {
			_ = json.Unmarshal([]byte(metadataStr), &r.Metadata)
		}

		records = append(records, &r)
	}

	return records, total, rows.Err()
}
