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
	"log/slog"
	"strings"
	"time"
)

// ContentScoringItem represents an entity queued for LLM content scoring.
type ContentScoringItem struct {
	EntityType string // "task", "demand", "comment", etc.
	EntityID   string
	Text       string // concatenated text fields
	HarmScore  int    // current heuristic score
	Retries    int    // number of previous LLM scoring attempts
}

// contentTable describes an entity table with text fields to score.
type contentTable struct {
	Table      string
	TextFields []string // columns to concatenate for scoring
}

// contentTables lists all entity tables that use scoreAndAnnotate.
// Message is excluded because it lives in a separate database (MessageDB).
var contentTables = []contentTable{
	{"task", []string{"title", "description"}},
	{"demand", []string{"title", "description"}},
	{"endeavour", []string{"name", "description"}},
	{"comment", []string{"content"}},
	{"artifact", []string{"title", "summary"}},
	{"ritual", []string{"name", "description"}},
	{"organization", []string{"name", "description"}},
}

// GetPendingContentForScoring returns entities queued for LLM content scoring.
// It scans all entity tables for metadata containing harm_score_llm_status = "pending"
// or "error" (for retries, up to maxRetries). Returns at most limit items.
func (db *DB) GetPendingContentForScoring(limit, maxRetries int) ([]ContentScoringItem, error) {
	var items []ContentScoringItem

	for _, ct := range contentTables {
		if len(items) >= limit {
			break
		}

		// Build COALESCE expression for text fields
		var textParts []string
		for _, f := range ct.TextFields {
			textParts = append(textParts, fmt.Sprintf("COALESCE(%s, '')", f))
		}
		textExpr := strings.Join(textParts, " || '\\n' || ")

		remaining := limit - len(items)
		query := fmt.Sprintf(`
			SELECT id, %s,
				CAST(COALESCE(json_extract(metadata, '$.harm_score'), 0) AS INTEGER),
				CAST(COALESCE(json_extract(metadata, '$.harm_score_llm_retries'), 0) AS INTEGER)
			FROM %s
			WHERE (json_extract(metadata, '$.harm_score_llm_status') = 'pending'
				OR (json_extract(metadata, '$.harm_score_llm_status') = 'error'
					AND CAST(COALESCE(json_extract(metadata, '$.harm_score_llm_retries'), 0) AS INTEGER) < ?))
			LIMIT ?`,
			textExpr, ct.Table)

		rows, err := db.Query(query, maxRetries, remaining)
		if err != nil {
			// Table might not exist yet (e.g., messages on older DBs) -- skip.
			continue
		}

		for rows.Next() {
			var item ContentScoringItem
			if err := rows.Scan(&item.EntityID, &item.Text, &item.HarmScore, &item.Retries); err != nil {
				_ = rows.Close()
				return nil, fmt.Errorf("scan %s content scoring item: %w", ct.Table, err)
			}
			item.EntityType = ct.Table
			item.Text = strings.TrimSpace(item.Text)
			if item.Text != "" {
				items = append(items, item)
			}
		}
		_ = rows.Close()
	}

	return items, nil
}

// UpdateContentHarmScore writes LLM scoring results to entity metadata.
func (db *DB) UpdateContentHarmScore(entityType, entityID string, llmScore int, confidence float64, categories []string, status string) error {
	// Validate entity type against known tables.
	valid := false
	for _, ct := range contentTables {
		if ct.Table == entityType {
			valid = true
			break
		}
	}
	if !valid {
		return fmt.Errorf("unknown entity type for content scoring: %s", entityType)
	}

	catsJSON, _ := json.Marshal(categories)

	query := fmt.Sprintf(`
		UPDATE %s SET
			metadata = json_set(
				COALESCE(metadata, '{}'),
				'$.harm_score_llm', ?,
				'$.harm_score_llm_confidence', ?,
				'$.harm_score_llm_categories', json(?),
				'$.harm_score_llm_status', ?
			),
			updated_at = datetime('now')
		WHERE id = ?`,
		entityType)

	result, err := db.Exec(query, llmScore, confidence, string(catsJSON), status, entityID)
	if err != nil {
		return fmt.Errorf("update %s content harm score: %w", entityType, err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("entity not found: %s/%s", entityType, entityID)
	}
	return nil
}

// IncrementContentHarmRetries marks a failed LLM scoring attempt and increments the retry counter.
func (db *DB) IncrementContentHarmRetries(entityType, entityID, errMsg string) error {
	valid := false
	for _, ct := range contentTables {
		if ct.Table == entityType {
			valid = true
			break
		}
	}
	if !valid {
		return fmt.Errorf("unknown entity type: %s", entityType)
	}

	query := fmt.Sprintf(`
		UPDATE %s SET
			metadata = json_set(
				COALESCE(metadata, '{}'),
				'$.harm_score_llm_status', 'error',
				'$.harm_score_llm_retries', CAST(COALESCE(json_extract(metadata, '$.harm_score_llm_retries'), 0) AS INTEGER) + 1,
				'$.harm_score_llm_error', ?
			),
			updated_at = datetime('now')
		WHERE id = ?`,
		entityType)

	_, err := db.Exec(query, errMsg, entityID)
	return err
}

// MarkContentHarmFailed sets the LLM scoring status to "failed" (no more retries).
func (db *DB) MarkContentHarmFailed(entityType, entityID string) error {
	valid := false
	for _, ct := range contentTables {
		if ct.Table == entityType {
			valid = true
			break
		}
	}
	if !valid {
		return fmt.Errorf("unknown entity type: %s", entityType)
	}

	query := fmt.Sprintf(`
		UPDATE %s SET
			metadata = json_set(
				COALESCE(metadata, '{}'),
				'$.harm_score_llm_status', 'failed'
			),
			updated_at = datetime('now')
		WHERE id = ?`,
		entityType)

	_, err := db.Exec(query, entityID)
	return err
}

// ContentGuardStats holds system-wide content guard statistics (admin view).
type ContentGuardStats struct {
	TotalScanned int            `json:"total_scanned"`
	Clean        int            `json:"clean"`
	Low          int            `json:"low"`
	Medium       int            `json:"medium"`
	High         int            `json:"high"`
	ByEntityType map[string]int `json:"by_entity_type"`
	LLMPending   int            `json:"llm_pending"`
	LLMCompleted int            `json:"llm_completed"`
	LLMError     int            `json:"llm_error"`
	LLMFailed    int            `json:"llm_failed"`
}

// GetContentGuardStats returns system-wide content guard statistics.
// It scans all entity tables and aggregates harm_score distributions and LLM queue status.
func (db *DB) GetContentGuardStats() (*ContentGuardStats, error) {
	stats := &ContentGuardStats{
		ByEntityType: make(map[string]int),
	}

	for _, ct := range contentTables {
		query := fmt.Sprintf(`
			SELECT
				COUNT(*) AS total,
				SUM(CASE WHEN CAST(COALESCE(json_extract(metadata, '$.harm_score'), 0) AS INTEGER) BETWEEN 1 AND 39 THEN 1 ELSE 0 END) AS low,
				SUM(CASE WHEN CAST(COALESCE(json_extract(metadata, '$.harm_score'), 0) AS INTEGER) BETWEEN 40 AND 69 THEN 1 ELSE 0 END) AS medium,
				SUM(CASE WHEN CAST(COALESCE(json_extract(metadata, '$.harm_score'), 0) AS INTEGER) >= 70 THEN 1 ELSE 0 END) AS high,
				SUM(CASE WHEN json_extract(metadata, '$.harm_score_llm_status') = 'pending' THEN 1 ELSE 0 END) AS llm_pending,
				SUM(CASE WHEN json_extract(metadata, '$.harm_score_llm_status') = 'completed' THEN 1 ELSE 0 END) AS llm_completed,
				SUM(CASE WHEN json_extract(metadata, '$.harm_score_llm_status') = 'error' THEN 1 ELSE 0 END) AS llm_error,
				SUM(CASE WHEN json_extract(metadata, '$.harm_score_llm_status') = 'failed' THEN 1 ELSE 0 END) AS llm_failed
			FROM %s`, ct.Table)

		var total, low, medium, high, llmPending, llmCompleted, llmError, llmFailed int
		err := db.QueryRow(query).Scan(&total, &low, &medium, &high, &llmPending, &llmCompleted, &llmError, &llmFailed)
		if err != nil {
			continue // table might not exist
		}

		stats.TotalScanned += total
		stats.Low += low
		stats.Medium += medium
		stats.High += high
		stats.LLMPending += llmPending
		stats.LLMCompleted += llmCompleted
		stats.LLMError += llmError
		stats.LLMFailed += llmFailed
		if total > 0 {
			stats.ByEntityType[ct.Table] = total
		}
	}

	stats.Clean = stats.TotalScanned - stats.Low - stats.Medium - stats.High
	return stats, nil
}

// ContentGuardUserStats holds user-scoped content guard statistics.
type ContentGuardUserStats struct {
	TotalScanned int `json:"total_scanned"`
	Clean        int `json:"clean"`
	Flagged      int `json:"flagged"`
}

// GetContentGuardStatsByAgents returns content guard statistics scoped to specific agents.
// Uses the same table/join pattern as ListContentAlertsByAgents.
func (db *DB) GetContentGuardStatsByAgents(agentUserIDs, agentResourceIDs []string) (*ContentGuardUserStats, error) {
	stats := &ContentGuardUserStats{}

	if len(agentUserIDs) == 0 && len(agentResourceIDs) == 0 {
		return stats, nil
	}

	var unions []string
	var params []interface{}

	for _, at := range alertTables {
		ids := resolveAgentIDs(at, agentUserIDs, agentResourceIDs)
		if len(ids) == 0 {
			continue
		}

		inClause, inParams := buildInClause(ids)

		q := fmt.Sprintf(
			`SELECT
				COUNT(*) AS total,
				SUM(CASE WHEN CAST(COALESCE(json_extract(metadata, '$.harm_score'), 0) AS INTEGER) >= 1 THEN 1 ELSE 0 END) AS flagged
			FROM %s
			WHERE %s IN (%s)`,
			at.Table, at.CreatorCol, inClause)

		unions = append(unions, q)
		params = append(params, inParams...)
	}

	// Task via entity_relation join.
	if len(agentResourceIDs) > 0 {
		inClause, inParams := buildInClause(agentResourceIDs)

		q := fmt.Sprintf(
			`SELECT
				COUNT(*) AS total,
				SUM(CASE WHEN CAST(COALESCE(json_extract(t.metadata, '$.harm_score'), 0) AS INTEGER) >= 1 THEN 1 ELSE 0 END) AS flagged
			FROM task t
			JOIN entity_relation er ON er.source_entity_id = t.id
				AND er.source_entity_type = 'task'
				AND er.relationship_type = 'assigned_to'
				AND er.target_entity_type = 'resource'
			WHERE er.target_entity_id IN (%s)`,
			inClause)

		unions = append(unions, q)
		params = append(params, inParams...)
	}

	if len(unions) == 0 {
		return stats, nil
	}

	// Wrap each subquery and sum across all.
	query := "SELECT SUM(total), SUM(flagged) FROM (" +
		strings.Join(unions, " UNION ALL ") + ")"

	var total, flagged int
	if err := db.QueryRow(query, params...).Scan(&total, &flagged); err != nil {
		return stats, nil
	}

	stats.TotalScanned = total
	stats.Flagged = flagged
	stats.Clean = total - flagged
	return stats, nil
}

// ContentAlert represents a flagged content item for the my-alerts view.
type ContentAlert struct {
	EntityType string `json:"entity_type"`
	EntityID   string `json:"entity_id"`
	Title      string `json:"title"`
	HarmScore  int    `json:"harm_score"`
	CreatedAt  string `json:"created_at"`
}

// alertableTable describes a content table with a creator column for alert filtering.
type alertableTable struct {
	Table       string
	TitleField  string
	CreatorCol  string // column that holds the creator/assignee ID
	CreatorType string // "resource" or "user" -- what ID format the creator column uses
}

// alertTables lists tables that can be filtered by agent creator/assignee.
// Task is excluded because assignee is tracked via entity_relation, not a direct column.
// Message is excluded because it lives in a separate database (MessageDB).
var alertTables = []alertableTable{
	{"comment", "content", "author_id", "resource"},
	{"artifact", "title", "created_by", "resource"},
	{"ritual", "name", "created_by", "user"},
}

// validContentEntityTypes maps entity type names to their database table names
// for content guard operations (dismiss, escalate).
var validContentEntityTypes = map[string]string{
	"task":     "task",
	"demand":   "demand",
	"comment":  "comment",
	"artifact": "artifact",
	"ritual":   "ritual",
}

// resolveAgentIDs returns the appropriate ID slice for a given alertable table.
func resolveAgentIDs(at alertableTable, agentUserIDs, agentResourceIDs []string) []string {
	if at.CreatorType == "resource" {
		return agentResourceIDs
	}
	return agentUserIDs
}

// buildInClause generates a SQL IN clause with placeholders and corresponding params.
func buildInClause(ids []string) (string, []interface{}) {
	placeholders := make([]string, len(ids))
	params := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		params[i] = id
	}
	return strings.Join(placeholders, ", "), params
}

// ListContentAlertsByAgents returns flagged entities (harm_score >= threshold)
// created by or assigned to the given agent IDs. agentUserIDs and agentResourceIDs
// are the user-table IDs and resource-table IDs of the agents respectively.
func (db *DB) ListContentAlertsByAgents(agentUserIDs, agentResourceIDs []string, threshold, limit, offset int) ([]ContentAlert, int, error) {
	if len(agentUserIDs) == 0 && len(agentResourceIDs) == 0 {
		return nil, 0, nil
	}
	if limit <= 0 {
		limit = 50
	}

	// Build per-table subqueries, collecting params in order.
	var unions []string
	var params []interface{}

	for _, at := range alertTables {
		ids := resolveAgentIDs(at, agentUserIDs, agentResourceIDs)
		if len(ids) == 0 {
			continue
		}

		inClause, inParams := buildInClause(ids)

		q := fmt.Sprintf(
			`SELECT '%s' AS entity_type, id, %s AS title,
				CAST(COALESCE(json_extract(metadata, '$.harm_score'), 0) AS INTEGER) AS harm_score,
				created_at
			FROM %s
			WHERE %s IN (%s)
				AND CAST(COALESCE(json_extract(metadata, '$.harm_score'), 0) AS INTEGER) >= ?`,
			at.Table, at.TitleField, at.Table, at.CreatorCol, inClause)

		unions = append(unions, q)
		params = append(params, inParams...)
		params = append(params, threshold)
	}

	// Task assignments use entity_relation (FRM), not a direct column.
	if len(agentResourceIDs) > 0 {
		inClause, inParams := buildInClause(agentResourceIDs)

		q := fmt.Sprintf(
			`SELECT 'task' AS entity_type, t.id, t.title AS title,
				CAST(COALESCE(json_extract(t.metadata, '$.harm_score'), 0) AS INTEGER) AS harm_score,
				t.created_at
			FROM task t
			JOIN entity_relation er ON er.source_entity_id = t.id
				AND er.source_entity_type = 'task'
				AND er.relationship_type = 'assigned_to'
				AND er.target_entity_type = 'resource'
			WHERE er.target_entity_id IN (%s)
				AND CAST(COALESCE(json_extract(t.metadata, '$.harm_score'), 0) AS INTEGER) >= ?`,
			inClause)

		unions = append(unions, q)
		params = append(params, inParams...)
		params = append(params, threshold)
	}

	if len(unions) == 0 {
		return nil, 0, nil
	}

	baseUnion := strings.Join(unions, " UNION ALL ")

	// Count total using a wrapping query (duplicate params for the count query).
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM (%s)", baseUnion)
	var total int
	// Use a copy of params for count (same params, no LIMIT/OFFSET).
	countParams := make([]interface{}, len(params))
	copy(countParams, params)
	if err := db.QueryRow(countQuery, countParams...).Scan(&total); err != nil {
		slog.Warn("Failed to count content alerts", "error", err)
	}

	// Data query with ORDER BY + pagination.
	dataQuery := baseUnion + " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	params = append(params, limit, offset)

	rows, err := db.Query(dataQuery, params...)
	if err != nil {
		return nil, 0, fmt.Errorf("query content alerts: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var alerts []ContentAlert
	for rows.Next() {
		var a ContentAlert
		if err := rows.Scan(&a.EntityType, &a.EntityID, &a.Title, &a.HarmScore, &a.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan content alert: %w", err)
		}
		alerts = append(alerts, a)
	}

	return alerts, total, nil
}

// SystemContentAlert extends ContentAlert with creator and dismissed info.
type SystemContentAlert struct {
	EntityType string `json:"entity_type"`
	EntityID   string `json:"entity_id"`
	Title      string `json:"title"`
	HarmScore  int    `json:"harm_score"`
	CreatedAt  string `json:"created_at"`
	CreatorID  string `json:"creator_id"`  // resource or user ID of the creator
	UserID     string `json:"user_id"`     // user ID resolved from resource_id (for suspend)
	Dismissed  bool   `json:"dismissed"`
}

// ListContentAlertsSystemWide returns all flagged entities system-wide.
func (db *DB) ListContentAlertsSystemWide(threshold, limit, offset int, includeDismissed bool) ([]SystemContentAlert, int, error) {
	if limit <= 0 {
		limit = 50
	}

	dismissFilter := ""
	if !includeDismissed {
		dismissFilter = " AND COALESCE(json_extract(t0.metadata, '$.harm_dismissed'), 0) != 1"
	}

	var unions []string
	var params []interface{}

	for _, at := range alertTables {
		// Join to user table to resolve resource_id -> user_id.
		var userJoin, userIDExpr string
		if at.CreatorType == "resource" {
			userJoin = fmt.Sprintf(" LEFT JOIN user u ON u.resource_id = t0.%s", at.CreatorCol)
			userIDExpr = "COALESCE(u.id, '')"
		} else {
			userJoin = ""
			userIDExpr = fmt.Sprintf("t0.%s", at.CreatorCol)
		}

		q := fmt.Sprintf(
			`SELECT '%s' AS entity_type, t0.id, t0.%s AS title,
				CAST(COALESCE(json_extract(t0.metadata, '$.harm_score'), 0) AS INTEGER) AS harm_score,
				t0.created_at,
				t0.%s AS creator_id,
				%s AS user_id,
				COALESCE(json_extract(t0.metadata, '$.harm_dismissed'), 0) AS dismissed
			FROM %s t0%s
			WHERE CAST(COALESCE(json_extract(t0.metadata, '$.harm_score'), 0) AS INTEGER) >= ?%s`,
			at.Table, at.TitleField, at.CreatorCol, userIDExpr, at.Table, userJoin, dismissFilter)

		unions = append(unions, q)
		params = append(params, threshold)
	}

	// Tasks: creator is the assignee via entity_relation.
	taskDismissFilter := ""
	if !includeDismissed {
		taskDismissFilter = " AND COALESCE(json_extract(t0.metadata, '$.harm_dismissed'), 0) != 1"
	}
	taskQ := fmt.Sprintf(
		`SELECT 'task' AS entity_type, t0.id, t0.title AS title,
			CAST(COALESCE(json_extract(t0.metadata, '$.harm_score'), 0) AS INTEGER) AS harm_score,
			t0.created_at,
			COALESCE(er.target_entity_id, '') AS creator_id,
			COALESCE(u.id, '') AS user_id,
			COALESCE(json_extract(t0.metadata, '$.harm_dismissed'), 0) AS dismissed
		FROM task t0
		LEFT JOIN entity_relation er ON er.source_entity_id = t0.id
			AND er.source_entity_type = 'task'
			AND er.relationship_type = 'assigned_to'
			AND er.target_entity_type = 'resource'
		LEFT JOIN user u ON u.resource_id = er.target_entity_id
		WHERE CAST(COALESCE(json_extract(t0.metadata, '$.harm_score'), 0) AS INTEGER) >= ?%s`,
		taskDismissFilter)
	unions = append(unions, taskQ)
	params = append(params, threshold)

	// Demands: owner_id is a resource ID.
	demandDismissFilter := ""
	if !includeDismissed {
		demandDismissFilter = " AND COALESCE(json_extract(t0.metadata, '$.harm_dismissed'), 0) != 1"
	}
	demandQ := fmt.Sprintf(
		`SELECT 'demand' AS entity_type, t0.id, t0.title AS title,
			CAST(COALESCE(json_extract(t0.metadata, '$.harm_score'), 0) AS INTEGER) AS harm_score,
			t0.created_at,
			COALESCE(t0.owner_id, '') AS creator_id,
			COALESCE(u.id, '') AS user_id,
			COALESCE(json_extract(t0.metadata, '$.harm_dismissed'), 0) AS dismissed
		FROM demand t0
		LEFT JOIN user u ON u.resource_id = t0.owner_id
		WHERE CAST(COALESCE(json_extract(t0.metadata, '$.harm_score'), 0) AS INTEGER) >= ?%s`,
		demandDismissFilter)
	unions = append(unions, demandQ)
	params = append(params, threshold)

	if len(unions) == 0 {
		return nil, 0, nil
	}

	baseUnion := strings.Join(unions, " UNION ALL ")

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM (%s)", baseUnion)
	countParams := make([]interface{}, len(params))
	copy(countParams, params)
	var total int
	if err := db.QueryRow(countQuery, countParams...).Scan(&total); err != nil {
		slog.Warn("Failed to count content review queue", "error", err)
	}

	dataQuery := baseUnion + " ORDER BY harm_score DESC, created_at DESC LIMIT ? OFFSET ?"
	params = append(params, limit, offset)

	rows, err := db.Query(dataQuery, params...)
	if err != nil {
		return nil, 0, fmt.Errorf("query system content alerts: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var alerts []SystemContentAlert
	for rows.Next() {
		var a SystemContentAlert
		var dismissed int
		if err := rows.Scan(&a.EntityType, &a.EntityID, &a.Title, &a.HarmScore, &a.CreatedAt, &a.CreatorID, &a.UserID, &dismissed); err != nil {
			return nil, 0, fmt.Errorf("scan system content alert: %w", err)
		}
		a.Dismissed = dismissed == 1
		alerts = append(alerts, a)
	}

	return alerts, total, nil
}

// DismissContentAlert sets harm_dismissed=true in the metadata of a flagged entity.
func (db *DB) DismissContentAlert(entityType, entityID string) error {
	table, ok := validContentEntityTypes[entityType]
	if !ok {
		return fmt.Errorf("unsupported entity type: %s", entityType)
	}

	query := fmt.Sprintf(
		`UPDATE %s SET metadata = json_set(COALESCE(metadata, '{}'), '$.harm_dismissed', 1), updated_at = ? WHERE id = ?`,
		table)

	_, err := db.Exec(query, UTCNow().Format(time.RFC3339), entityID)
	return err
}

// HarmconLevel represents the computed Harmcon (Content Harm Condition) level.
type HarmconLevel struct {
	Level       int // 1=Red, 2=Orange, 3=Green, 4=Blue
	HighCount   int // harm_score >= 70 in 24h
	MediumCount int // harm_score 40-69 in 24h
	LowCount    int // harm_score 1-39 in 24h
}

// computeHarmconLevel applies thresholds to determine the DEFCON-style level.
func computeHarmconLevel(high, medium, low int) int {
	if high >= 3 || medium >= 10 {
		return 1 // Red: significant harmful content
	}
	if high >= 1 || medium >= 3 {
		return 2 // Orange: content issues detected
	}
	if low > 0 {
		return 3 // Green: minor content flags
	}
	return 4 // Blue: no signals detected
}

// GetSystemHarmconLevel computes the system-wide Harmcon level by counting
// flagged entities across all content tables in the last 24 hours.
func (db *DB) GetSystemHarmconLevel() (*HarmconLevel, error) {
	h := &HarmconLevel{}

	for _, ct := range contentTables {
		query := fmt.Sprintf(`
			SELECT
				SUM(CASE WHEN CAST(COALESCE(json_extract(metadata, '$.harm_score'), 0) AS INTEGER) >= 70 THEN 1 ELSE 0 END),
				SUM(CASE WHEN CAST(COALESCE(json_extract(metadata, '$.harm_score'), 0) AS INTEGER) BETWEEN 40 AND 69 THEN 1 ELSE 0 END),
				SUM(CASE WHEN CAST(COALESCE(json_extract(metadata, '$.harm_score'), 0) AS INTEGER) BETWEEN 1 AND 39 THEN 1 ELSE 0 END)
			FROM %s
			WHERE updated_at >= datetime('now', '-24 hours')`, ct.Table)

		var high, medium, low int
		err := db.QueryRow(query).Scan(&high, &medium, &low)
		if err != nil {
			continue // table might not exist
		}
		h.HighCount += high
		h.MediumCount += medium
		h.LowCount += low
	}

	h.Level = computeHarmconLevel(h.HighCount, h.MediumCount, h.LowCount)
	return h, nil
}

// GetUserHarmconLevel computes the Harmcon level scoped to a user's agents.
// Uses the same alertTables + task entity_relation JOIN pattern as GetContentGuardStatsByAgents.
func (db *DB) GetUserHarmconLevel(agentUserIDs, agentResourceIDs []string) (*HarmconLevel, error) {
	h := &HarmconLevel{}

	if len(agentUserIDs) == 0 && len(agentResourceIDs) == 0 {
		h.Level = 4
		return h, nil
	}

	var unions []string
	var params []interface{}

	for _, at := range alertTables {
		ids := resolveAgentIDs(at, agentUserIDs, agentResourceIDs)
		if len(ids) == 0 {
			continue
		}

		inClause, inParams := buildInClause(ids)

		q := fmt.Sprintf(
			`SELECT
				COALESCE(SUM(CASE WHEN CAST(COALESCE(json_extract(metadata, '$.harm_score'), 0) AS INTEGER) >= 70 THEN 1 ELSE 0 END), 0) AS h,
				COALESCE(SUM(CASE WHEN CAST(COALESCE(json_extract(metadata, '$.harm_score'), 0) AS INTEGER) BETWEEN 40 AND 69 THEN 1 ELSE 0 END), 0) AS m,
				COALESCE(SUM(CASE WHEN CAST(COALESCE(json_extract(metadata, '$.harm_score'), 0) AS INTEGER) BETWEEN 1 AND 39 THEN 1 ELSE 0 END), 0) AS l
			FROM %s
			WHERE %s IN (%s) AND updated_at >= datetime('now', '-24 hours')`,
			at.Table, at.CreatorCol, inClause)

		unions = append(unions, q)
		params = append(params, inParams...)
	}

	// Task via entity_relation join.
	if len(agentResourceIDs) > 0 {
		inClause, inParams := buildInClause(agentResourceIDs)

		q := fmt.Sprintf(
			`SELECT
				COALESCE(SUM(CASE WHEN CAST(COALESCE(json_extract(t.metadata, '$.harm_score'), 0) AS INTEGER) >= 70 THEN 1 ELSE 0 END), 0) AS h,
				COALESCE(SUM(CASE WHEN CAST(COALESCE(json_extract(t.metadata, '$.harm_score'), 0) AS INTEGER) BETWEEN 40 AND 69 THEN 1 ELSE 0 END), 0) AS m,
				COALESCE(SUM(CASE WHEN CAST(COALESCE(json_extract(t.metadata, '$.harm_score'), 0) AS INTEGER) BETWEEN 1 AND 39 THEN 1 ELSE 0 END), 0) AS l
			FROM task t
			JOIN entity_relation er ON er.source_entity_id = t.id
				AND er.source_entity_type = 'task'
				AND er.relationship_type = 'assigned_to'
				AND er.target_entity_type = 'resource'
			WHERE er.target_entity_id IN (%s) AND t.updated_at >= datetime('now', '-24 hours')`,
			inClause)

		unions = append(unions, q)
		params = append(params, inParams...)
	}

	if len(unions) == 0 {
		h.Level = 4
		return h, nil
	}

	// Each subquery returns (h, m, l). UNION ALL and SUM across rows.
	query := "SELECT COALESCE(SUM(h), 0), COALESCE(SUM(m), 0), COALESCE(SUM(l), 0) FROM (" +
		strings.Join(unions, " UNION ALL ") + ")"

	var high, medium, low int
	if err := db.QueryRow(query, params...).Scan(&high, &medium, &low); err != nil {
		h.Level = 4
		return h, nil
	}

	h.HighCount = high
	h.MediumCount = medium
	h.LowCount = low
	h.Level = computeHarmconLevel(high, medium, low)
	return h, nil
}

// ---------------------------------------------------------------------------
// System Pattern Overrides (stored in policy table)
// ---------------------------------------------------------------------------

// PolicyKeySystemPatterns is the policy key for admin pattern overrides.
const PolicyKeySystemPatterns = "content-guard.system-patterns"

// CustomPatternEntry is a storage-layer representation of a custom scoring pattern.
// Mirrors security.CustomPattern without importing that package (avoids import cycle).
type CustomPatternEntry struct {
	Name     string `json:"name"`
	Category string `json:"category"`
	Pattern  string `json:"pattern"`
	Weight   int    `json:"weight"`
}

// SystemPatternOverrides holds admin-configurable scoring overrides.
type SystemPatternOverrides struct {
	Disabled        map[string]bool   `json:"disabled,omitempty"`
	WeightOverrides map[string]int    `json:"weight_overrides,omitempty"`
	Added           []CustomPatternEntry `json:"added,omitempty"`
}

// GetSystemPatternOverrides loads pattern overrides from the policy table.
// Returns nil (no error) when no overrides are stored.
func (db *DB) GetSystemPatternOverrides() (*SystemPatternOverrides, error) {
	raw, err := db.GetPolicy(PolicyKeySystemPatterns)
	if err != nil || raw == "" {
		return nil, nil
	}
	var o SystemPatternOverrides
	if err := json.Unmarshal([]byte(raw), &o); err != nil {
		return nil, fmt.Errorf("unmarshal system pattern overrides: %w", err)
	}
	return &o, nil
}

// SetSystemPatternOverrides saves pattern overrides to the policy table.
func (db *DB) SetSystemPatternOverrides(o *SystemPatternOverrides) error {
	b, err := json.Marshal(o)
	if err != nil {
		return fmt.Errorf("marshal system pattern overrides: %w", err)
	}
	return db.SetPolicy(PolicyKeySystemPatterns, string(b))
}

// ---------------------------------------------------------------------------
// Org Alert Terms (per-organization additive scoring terms)
// ---------------------------------------------------------------------------

// OrgAlertTerm represents a per-organization content alert term.
type OrgAlertTerm struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organization_id"`
	Term           string    `json:"term"`
	Weight         int       `json:"weight"`
	CreatedBy      string    `json:"created_by"`
	CreatedAt      time.Time `json:"created_at"`
}

// CreateOrgAlertTerm adds an alert term for an organization.
func (db *DB) CreateOrgAlertTerm(orgID, term string, weight int, createdBy string) (*OrgAlertTerm, error) {
	id := generateID("oat")
	now := UTCNow()
	_, err := db.Exec(
		`INSERT INTO org_alert_terms (id, organization_id, term, weight, created_by, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		id, orgID, term, weight, createdBy, now.Format(time.RFC3339),
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			return nil, fmt.Errorf("term already exists for this organization")
		}
		return nil, fmt.Errorf("insert org alert term: %w", err)
	}
	return &OrgAlertTerm{
		ID:             id,
		OrganizationID: orgID,
		Term:           term,
		Weight:         weight,
		CreatedBy:      createdBy,
		CreatedAt:      now,
	}, nil
}

// ListOrgAlertTerms returns all alert terms for an organization.
func (db *DB) ListOrgAlertTerms(orgID string) ([]*OrgAlertTerm, error) {
	rows, err := db.Query(
		`SELECT id, organization_id, term, weight, created_by, created_at
		 FROM org_alert_terms WHERE organization_id = ? ORDER BY term ASC`,
		orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("query org alert terms: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var terms []*OrgAlertTerm
	for rows.Next() {
		var t OrgAlertTerm
		var createdAt string
		if err := rows.Scan(&t.ID, &t.OrganizationID, &t.Term, &t.Weight, &t.CreatedBy, &createdAt); err != nil {
			return nil, fmt.Errorf("scan org alert term: %w", err)
		}
		t.CreatedAt = ParseDBTime(createdAt)
		terms = append(terms, &t)
	}
	return terms, nil
}

// DeleteOrgAlertTerm removes a single alert term.
func (db *DB) DeleteOrgAlertTerm(id, orgID string) error {
	result, err := db.Exec(
		`DELETE FROM org_alert_terms WHERE id = ? AND organization_id = ?`,
		id, orgID,
	)
	if err != nil {
		return fmt.Errorf("delete org alert term: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("alert term not found")
	}
	return nil
}

// DeleteOrgAlertTermsByOrg removes all alert terms for an organization.
func (db *DB) DeleteOrgAlertTermsByOrg(orgID string) error {
	_, err := db.Exec(`DELETE FROM org_alert_terms WHERE organization_id = ?`, orgID)
	return err
}

// CountOrgAlertTerms returns the number of alert terms for an organization.
func (db *DB) CountOrgAlertTerms(orgID string) (int, error) {
	var count int
	err := db.QueryRow(
		`SELECT COUNT(*) FROM org_alert_terms WHERE organization_id = ?`,
		orgID,
	).Scan(&count)
	return count, err
}

// ---------------------------------------------------------------------------
// Escalation Support (automated enforcement)
// ---------------------------------------------------------------------------

// EscalationCandidate represents an entity eligible for automated escalation.
type EscalationCandidate struct {
	EntityType string
	EntityID   string
	HarmScore  int
	UserID     string // user ID of the creator (resolved from resource_id)
}

// ListUnescalatedEntities returns entities with harm_score >= threshold
// that haven't been marked as escalated yet (harm_escalated not set in metadata).
func (db *DB) ListUnescalatedEntities(threshold, limit int) ([]EscalationCandidate, error) {
	if limit <= 0 {
		limit = 50
	}

	var unions []string
	var params []interface{}

	for _, at := range alertTables {
		var userJoin, userIDExpr string
		if at.CreatorType == "resource" {
			userJoin = fmt.Sprintf(" LEFT JOIN user u ON u.resource_id = t0.%s", at.CreatorCol)
			userIDExpr = "COALESCE(u.id, '')"
		} else {
			userJoin = ""
			userIDExpr = fmt.Sprintf("COALESCE(t0.%s, '')", at.CreatorCol)
		}

		q := fmt.Sprintf(
			`SELECT '%s' AS entity_type, t0.id,
				CAST(COALESCE(json_extract(t0.metadata, '$.harm_score'), 0) AS INTEGER) AS harm_score,
				%s AS user_id
			FROM %s t0%s
			WHERE CAST(COALESCE(json_extract(t0.metadata, '$.harm_score'), 0) AS INTEGER) >= ?
				AND COALESCE(json_extract(t0.metadata, '$.harm_escalated'), 0) != 1`,
			at.Table, userIDExpr, at.Table, userJoin)

		unions = append(unions, q)
		params = append(params, threshold)
	}

	// Tasks: assignee via entity_relation.
	taskQ := `SELECT 'task' AS entity_type, t0.id,
			CAST(COALESCE(json_extract(t0.metadata, '$.harm_score'), 0) AS INTEGER) AS harm_score,
			COALESCE(u.id, '') AS user_id
		FROM task t0
		LEFT JOIN entity_relation er ON er.source_entity_id = t0.id
			AND er.source_entity_type = 'task'
			AND er.relationship_type = 'assigned_to'
			AND er.target_entity_type = 'resource'
		LEFT JOIN user u ON u.resource_id = er.target_entity_id
		WHERE CAST(COALESCE(json_extract(t0.metadata, '$.harm_score'), 0) AS INTEGER) >= ?
			AND COALESCE(json_extract(t0.metadata, '$.harm_escalated'), 0) != 1`
	unions = append(unions, taskQ)
	params = append(params, threshold)

	// Demands: owner_id is a resource ID.
	demandQ := `SELECT 'demand' AS entity_type, t0.id,
			CAST(COALESCE(json_extract(t0.metadata, '$.harm_score'), 0) AS INTEGER) AS harm_score,
			COALESCE(u.id, '') AS user_id
		FROM demand t0
		LEFT JOIN user u ON u.resource_id = t0.owner_id
		WHERE CAST(COALESCE(json_extract(t0.metadata, '$.harm_score'), 0) AS INTEGER) >= ?
			AND COALESCE(json_extract(t0.metadata, '$.harm_escalated'), 0) != 1`
	unions = append(unions, demandQ)
	params = append(params, threshold)

	query := strings.Join(unions, " UNION ALL ") + " ORDER BY harm_score DESC LIMIT ?"
	params = append(params, limit)

	rows, err := db.Query(query, params...)
	if err != nil {
		return nil, fmt.Errorf("query unescalated entities: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var candidates []EscalationCandidate
	for rows.Next() {
		var c EscalationCandidate
		if err := rows.Scan(&c.EntityType, &c.EntityID, &c.HarmScore, &c.UserID); err != nil {
			return nil, fmt.Errorf("scan escalation candidate: %w", err)
		}
		candidates = append(candidates, c)
	}

	return candidates, nil
}

// MarkEntityEscalated sets harm_escalated=1 in the entity's metadata.
func (db *DB) MarkEntityEscalated(entityType, entityID string) error {
	table, ok := validContentEntityTypes[entityType]
	if !ok {
		return fmt.Errorf("unsupported entity type for escalation: %s", entityType)
	}

	query := fmt.Sprintf(
		`UPDATE %s SET metadata = json_set(COALESCE(metadata, '{}'), '$.harm_escalated', 1), updated_at = ? WHERE id = ?`,
		table)
	_, err := db.Exec(query, UTCNow().Format(time.RFC3339), entityID)
	return err
}

// CreatorHarmCounts holds aggregated severity counts for a content creator within a time window.
type CreatorHarmCounts struct {
	UserID      string
	HighCount   int // harm_score >= 70
	MediumCount int // harm_score 40-69
}

// GetCreatorHarmCounts returns per-creator (user ID) harm counts within a time window.
// Only includes entities created since the given time with harm_score >= 40.
func (db *DB) GetCreatorHarmCounts(since time.Time) ([]CreatorHarmCounts, error) {
	sinceStr := since.Format(time.RFC3339)

	var subqueries []string
	var params []interface{}

	for _, at := range alertTables {
		var userIDExpr, userJoin string
		if at.CreatorType == "resource" {
			userJoin = fmt.Sprintf(" LEFT JOIN user u ON u.resource_id = t0.%s", at.CreatorCol)
			userIDExpr = "COALESCE(u.id, '')"
		} else {
			userJoin = ""
			userIDExpr = fmt.Sprintf("COALESCE(t0.%s, '')", at.CreatorCol)
		}

		q := fmt.Sprintf(
			`SELECT %s AS user_id,
				CAST(COALESCE(json_extract(t0.metadata, '$.harm_score'), 0) AS INTEGER) AS score
			FROM %s t0%s
			WHERE CAST(COALESCE(json_extract(t0.metadata, '$.harm_score'), 0) AS INTEGER) >= 40
				AND t0.created_at >= ?`,
			userIDExpr, at.Table, userJoin)

		subqueries = append(subqueries, q)
		params = append(params, sinceStr)
	}

	// Tasks via entity_relation.
	taskQ := `SELECT COALESCE(u.id, '') AS user_id,
			CAST(COALESCE(json_extract(t0.metadata, '$.harm_score'), 0) AS INTEGER) AS score
		FROM task t0
		LEFT JOIN entity_relation er ON er.source_entity_id = t0.id
			AND er.source_entity_type = 'task'
			AND er.relationship_type = 'assigned_to'
			AND er.target_entity_type = 'resource'
		LEFT JOIN user u ON u.resource_id = er.target_entity_id
		WHERE CAST(COALESCE(json_extract(t0.metadata, '$.harm_score'), 0) AS INTEGER) >= 40
			AND t0.created_at >= ?`
	subqueries = append(subqueries, taskQ)
	params = append(params, sinceStr)

	// Demands: owner_id is a resource ID.
	demandQ := `SELECT COALESCE(u.id, '') AS user_id,
			CAST(COALESCE(json_extract(t0.metadata, '$.harm_score'), 0) AS INTEGER) AS score
		FROM demand t0
		LEFT JOIN user u ON u.resource_id = t0.owner_id
		WHERE CAST(COALESCE(json_extract(t0.metadata, '$.harm_score'), 0) AS INTEGER) >= 40
			AND t0.created_at >= ?`
	subqueries = append(subqueries, demandQ)
	params = append(params, sinceStr)

	baseUnion := strings.Join(subqueries, " UNION ALL ")
	query := fmt.Sprintf(`
		SELECT user_id,
			SUM(CASE WHEN score >= 70 THEN 1 ELSE 0 END) AS high_count,
			SUM(CASE WHEN score BETWEEN 40 AND 69 THEN 1 ELSE 0 END) AS medium_count
		FROM (%s)
		WHERE user_id != ''
		GROUP BY user_id
		HAVING SUM(CASE WHEN score >= 70 THEN 1 ELSE 0 END) > 0
			OR SUM(CASE WHEN score BETWEEN 40 AND 69 THEN 1 ELSE 0 END) > 0`,
		baseUnion)

	rows, err := db.Query(query, params...)
	if err != nil {
		return nil, fmt.Errorf("query creator harm counts: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var counts []CreatorHarmCounts
	for rows.Next() {
		var c CreatorHarmCounts
		if err := rows.Scan(&c.UserID, &c.HighCount, &c.MediumCount); err != nil {
			return nil, fmt.Errorf("scan creator harm counts: %w", err)
		}
		counts = append(counts, c)
	}

	return counts, nil
}
