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


package onboarding

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/QuestFinTech/taskschmiede/internal/storage"
)

// handleSimTskCreate handles ts.tsk.create in the interview simulation.
func (s *InterviewServer) handleSimTskCreate(_ context.Context, req *mcp.CallToolRequest, session *InterviewSession) (*mcp.CallToolResult, error) {
	args := interviewParseArgs(req)
	title := interviewGetString(args, "title")
	if title == "" {
		return interviewToolError("invalid_input", "Title is required"), nil
	}

	id := fmt.Sprintf("tsk_%s", generateShortID())
	now := storage.UTCNow().Format(time.RFC3339)

	description := interviewGetString(args, "description")
	endeavourID := interviewGetString(args, "endeavour_id")
	assigneeID := interviewGetString(args, "assignee_id")
	demandID := interviewGetString(args, "demand_id")
	dueDate := interviewGetString(args, "due_date")

	var estimate *float64
	if v, ok := interviewGetFloat(args, "estimate"); ok {
		estimate = &v
	}

	var dd *string
	if dueDate != "" {
		dd = &dueDate
	}

	_, err := session.SimDB.Exec(
		`INSERT INTO task (id, title, description, status, estimate, due_date, created_at, updated_at)
		 VALUES (?, ?, ?, 'planned', ?, ?, ?, ?)`,
		id, title, description, estimate, dd, now, now,
	)
	if err != nil {
		return interviewToolError("internal_error", fmt.Sprintf("Failed to create task: %v", err)), nil
	}

	// Create entity_relation rows for FK-like links
	if endeavourID != "" {
		_, _ = session.SimDB.Exec(
			`INSERT INTO entity_relation (id, relationship_type, source_entity_type, source_entity_id, target_entity_type, target_entity_id, created_at)
			 VALUES (?, 'belongs_to', 'task', ?, 'endeavour', ?, ?)`,
			fmt.Sprintf("rel_%s", generateShortID()), id, endeavourID, now,
		)
	}
	if assigneeID != "" {
		_, _ = session.SimDB.Exec(
			`INSERT INTO entity_relation (id, relationship_type, source_entity_type, source_entity_id, target_entity_type, target_entity_id, created_at)
			 VALUES (?, 'assigned_to', 'task', ?, 'resource', ?, ?)`,
			fmt.Sprintf("rel_%s", generateShortID()), id, assigneeID, now,
		)
	}
	if demandID != "" {
		_, _ = session.SimDB.Exec(
			`INSERT INTO entity_relation (id, relationship_type, source_entity_type, source_entity_id, target_entity_type, target_entity_id, created_at)
			 VALUES (?, 'fulfills', 'task', ?, 'demand', ?, ?)`,
			fmt.Sprintf("rel_%s", generateShortID()), id, demandID, now,
		)
	}

	// Track the created task ID for cross-section reference
	session.mu.Lock()
	if session.CreatedTaskID == "" {
		session.CreatedTaskID = id
	}
	session.mu.Unlock()

	result := map[string]interface{}{
		"id":          id,
		"title":       title,
		"description": description,
		"status":      "planned",
		"created_at":  now,
	}
	if estimate != nil {
		result["estimate"] = *estimate
	}
	if endeavourID != "" {
		result["endeavour_id"] = endeavourID
	}

	return interviewToolSuccess(result), nil
}

// handleSimTskUpdate handles ts.tsk.update in the interview simulation.
func (s *InterviewServer) handleSimTskUpdate(_ context.Context, req *mcp.CallToolRequest, session *InterviewSession) (*mcp.CallToolResult, error) {
	args := interviewParseArgs(req)
	id := interviewGetString(args, "id")
	if id == "" {
		return interviewToolError("invalid_input", "Task ID is required"), nil
	}

	// Check task exists
	var currentStatus string
	err := session.SimDB.QueryRow("SELECT status FROM task WHERE id = ?", id).Scan(&currentStatus)
	if err == sql.ErrNoRows {
		return interviewToolError("not_found", fmt.Sprintf("Task %s not found", id)), nil
	}
	if err != nil {
		return interviewToolError("internal_error", err.Error()), nil
	}

	// Handle cancel reason enforcement (mirrors production behavior exactly)
	newStatus := interviewGetString(args, "status")
	if newStatus == "canceled" {
		canceledReason := interviewGetString(args, "canceled_reason")
		if strings.TrimSpace(canceledReason) == "" {
			return interviewToolErrorWithDetails("invalid_input", "canceled_reason is required when canceling a task", map[string]interface{}{
				"hint": `Example: {"id": "tsk_...", "status": "canceled", "canceled_reason": "No longer needed"}`,
			}), nil
		}
	}

	// Build UPDATE statement
	sets := []string{}
	vals := []interface{}{}

	if v := interviewGetString(args, "title"); v != "" {
		sets = append(sets, "title = ?")
		vals = append(vals, v)
	}
	if v := interviewGetString(args, "description"); v != "" {
		sets = append(sets, "description = ?")
		vals = append(vals, v)
	}
	if newStatus != "" {
		sets = append(sets, "status = ?")
		vals = append(vals, newStatus)
		if newStatus == "active" {
			sets = append(sets, "started_at = ?")
			vals = append(vals, storage.UTCNow().Format(time.RFC3339))
		}
		if newStatus == "done" {
			sets = append(sets, "completed_at = ?")
			vals = append(vals, storage.UTCNow().Format(time.RFC3339))
		}
		if newStatus == "canceled" {
			sets = append(sets, "canceled_at = ?")
			vals = append(vals, storage.UTCNow().Format(time.RFC3339))
			reason := interviewGetString(args, "canceled_reason")
			sets = append(sets, "canceled_reason = ?")
			vals = append(vals, reason)
		}
	}
	if v, ok := interviewGetFloat(args, "estimate"); ok {
		sets = append(sets, "estimate = ?")
		vals = append(vals, v)
	}
	if v, ok := interviewGetFloat(args, "actual"); ok {
		sets = append(sets, "actual = ?")
		vals = append(vals, v)
	}

	// Handle FK-like fields via entity_relation (after UPDATE)
	endeavourID := interviewGetString(args, "endeavour_id")
	demandID := interviewGetString(args, "demand_id")

	if len(sets) == 0 && endeavourID == "" && demandID == "" {
		return interviewToolError("invalid_input", "No fields to update"), nil
	}

	if len(sets) > 0 {
		sets = append(sets, "updated_at = ?")
		vals = append(vals, storage.UTCNow().Format(time.RFC3339))
		vals = append(vals, id)

		query := fmt.Sprintf("UPDATE task SET %s WHERE id = ?", strings.Join(sets, ", "))
		_, err = session.SimDB.Exec(query, vals...)
		if err != nil {
			return interviewToolError("internal_error", err.Error()), nil
		}
	}

	now := storage.UTCNow().Format(time.RFC3339)

	// Update entity_relation for FK-like fields (delete + insert pattern)
	if endeavourID != "" {
		_, _ = session.SimDB.Exec(
			`DELETE FROM entity_relation WHERE source_entity_type = 'task' AND source_entity_id = ? AND relationship_type = 'belongs_to' AND target_entity_type = 'endeavour'`, id)
		_, _ = session.SimDB.Exec(
			`INSERT INTO entity_relation (id, relationship_type, source_entity_type, source_entity_id, target_entity_type, target_entity_id, created_at)
			 VALUES (?, 'belongs_to', 'task', ?, 'endeavour', ?, ?)`,
			fmt.Sprintf("rel_%s", generateShortID()), id, endeavourID, now,
		)
	}
	if demandID != "" {
		_, _ = session.SimDB.Exec(
			`DELETE FROM entity_relation WHERE source_entity_type = 'task' AND source_entity_id = ? AND relationship_type = 'fulfills' AND target_entity_type = 'demand'`, id)
		_, _ = session.SimDB.Exec(
			`INSERT INTO entity_relation (id, relationship_type, source_entity_type, source_entity_id, target_entity_type, target_entity_id, created_at)
			 VALUES (?, 'fulfills', 'task', ?, 'demand', ?, ?)`,
			fmt.Sprintf("rel_%s", generateShortID()), id, demandID, now,
		)
	}

	// Return updated task
	return s.fetchTask(session.SimDB, id)
}

// handleSimTskCancel handles ts.tsk.cancel in the interview simulation.
func (s *InterviewServer) handleSimTskCancel(_ context.Context, req *mcp.CallToolRequest, session *InterviewSession) (*mcp.CallToolResult, error) {
	args := interviewParseArgs(req)
	id := interviewGetString(args, "id")
	if id == "" {
		return interviewToolError("invalid_input", "Task ID is required"), nil
	}

	reason := interviewGetString(args, "reason")
	if strings.TrimSpace(reason) == "" {
		return interviewToolError("invalid_input", "Cancellation reason is required"), nil
	}

	// Check task exists
	var currentStatus string
	err := session.SimDB.QueryRow("SELECT status FROM task WHERE id = ?", id).Scan(&currentStatus)
	if err == sql.ErrNoRows {
		return interviewToolError("not_found", fmt.Sprintf("Task %s not found", id)), nil
	}
	if err != nil {
		return interviewToolError("internal_error", err.Error()), nil
	}

	now := storage.UTCNow().Format(time.RFC3339)
	_, err = session.SimDB.Exec(
		"UPDATE task SET status = 'canceled', canceled_reason = ?, canceled_at = ?, updated_at = ? WHERE id = ?",
		reason, now, now, id,
	)
	if err != nil {
		return interviewToolError("internal_error", err.Error()), nil
	}

	return s.fetchTask(session.SimDB, id)
}

// handleSimTskList handles ts.tsk.list in the interview simulation.
func (s *InterviewServer) handleSimTskList(_ context.Context, req *mcp.CallToolRequest, session *InterviewSession) (*mcp.CallToolResult, error) {
	args := interviewParseArgs(req)
	status := interviewGetString(args, "status")
	search := interviewGetString(args, "search")

	query := `SELECT t.id, t.title, COALESCE(t.description, ''), t.status, COALESCE(t.estimate, 0),
		 (SELECT er.target_entity_id FROM entity_relation er
		  WHERE er.source_entity_type = 'task' AND er.source_entity_id = t.id
		  AND er.relationship_type = 'fulfills' AND er.target_entity_type = 'demand' LIMIT 1) AS demand_id,
		 t.created_at FROM task t WHERE 1=1`
	var vals []interface{}

	if status != "" {
		query += " AND t.status = ?"
		vals = append(vals, status)
	}
	if search != "" {
		query += " AND (t.title LIKE ? OR t.description LIKE ?)"
		vals = append(vals, "%"+search+"%", "%"+search+"%")
	}

	query += " ORDER BY t.created_at DESC"

	rows, err := session.SimDB.Query(query, vals...)
	if err != nil {
		return interviewToolError("internal_error", err.Error()), nil
	}
	defer func() { _ = rows.Close() }()

	var tasks []map[string]interface{}
	for rows.Next() {
		var id, title, description, taskStatus, createdAt string
		var estimate float64
		var demandID sql.NullString
		if err := rows.Scan(&id, &title, &description, &taskStatus, &estimate, &demandID, &createdAt); err != nil {
			return interviewToolError("internal_error", err.Error()), nil
		}
		task := map[string]interface{}{
			"id":          id,
			"title":       title,
			"description": description,
			"status":      taskStatus,
			"created_at":  createdAt,
		}
		if estimate > 0 {
			task["estimate"] = estimate
		}
		if demandID.Valid {
			task["demand_id"] = demandID.String
		}
		tasks = append(tasks, task)
	}

	if tasks == nil {
		tasks = []map[string]interface{}{}
	}

	return interviewToolSuccess(map[string]interface{}{
		"data": tasks,
		"meta": map[string]interface{}{
			"total":  len(tasks),
			"limit":  50,
			"offset": 0,
		},
	}), nil
}

// handleSimTskGet handles ts.tsk.get in the interview simulation.
func (s *InterviewServer) handleSimTskGet(_ context.Context, req *mcp.CallToolRequest, session *InterviewSession) (*mcp.CallToolResult, error) {
	args := interviewParseArgs(req)
	id := interviewGetString(args, "id")
	if id == "" {
		return interviewToolError("invalid_input", "Task ID is required"), nil
	}
	return s.fetchTask(session.SimDB, id)
}

// handleSimCmtCreate handles ts.cmt.create in the interview simulation.
func (s *InterviewServer) handleSimCmtCreate(_ context.Context, req *mcp.CallToolRequest, session *InterviewSession) (*mcp.CallToolResult, error) {
	args := interviewParseArgs(req)
	entityType := interviewGetString(args, "entity_type")
	entityID := interviewGetString(args, "entity_id")
	content := interviewGetString(args, "content")

	if entityType == "" || entityID == "" || content == "" {
		return interviewToolError("invalid_input", "entity_type, entity_id, and content are required"), nil
	}

	id := fmt.Sprintf("cmt_%s", generateShortID())
	now := storage.UTCNow().Format(time.RFC3339)

	_, err := session.SimDB.Exec(
		`INSERT INTO comment (id, entity_type, entity_id, content, author_id, created_at, updated_at)
		 VALUES (?, ?, ?, ?, 'res_agent', ?, ?)`,
		id, entityType, entityID, content, now, now,
	)
	if err != nil {
		return interviewToolError("internal_error", err.Error()), nil
	}

	return interviewToolSuccess(map[string]interface{}{
		"id":          id,
		"entity_type": entityType,
		"entity_id":   entityID,
		"content":     content,
		"created_at":  now,
	}), nil
}

// handleSimMsgSend handles ts.msg.send in the interview simulation.
func (s *InterviewServer) handleSimMsgSend(_ context.Context, req *mcp.CallToolRequest, session *InterviewSession) (*mcp.CallToolResult, error) {
	args := interviewParseArgs(req)
	content := interviewGetString(args, "content")
	if content == "" {
		return interviewToolError("invalid_input", "Content is required"), nil
	}

	subject := interviewGetString(args, "subject")

	id := fmt.Sprintf("msg_%s", generateShortID())
	now := storage.UTCNow().Format(time.RFC3339)

	// Store message in simulation DB
	_, err := session.SimDB.Exec(
		`INSERT INTO message (id, sender_id, subject, content, created_at)
		 VALUES (?, 'res_agent', ?, ?, ?)`,
		id, subject, content, now,
	)
	if err != nil {
		return interviewToolError("internal_error", err.Error()), nil
	}

	return interviewToolSuccess(map[string]interface{}{
		"id":         id,
		"content":    content,
		"subject":    subject,
		"status":     "delivered",
		"created_at": now,
	}), nil
}

// handleOnboardSubmit handles ts.onboard.submit for structured answer submission.
func (s *InterviewServer) handleOnboardSubmit(_ context.Context, req *mcp.CallToolRequest, session *InterviewSession) (*mcp.CallToolResult, error) {
	args := interviewParseArgs(req)

	// Just acknowledge the submission -- the evaluator reads the tool log
	return interviewToolSuccess(map[string]interface{}{
		"status":  "accepted",
		"message": "Your answers have been recorded.",
		"data":    args,
	}), nil
}

// fetchTask reads a task from the given DB and returns it as a tool result.
func (s *InterviewServer) fetchTask(db *storage.DB, id string) (*mcp.CallToolResult, error) {
	var title, description, taskStatus, createdAt, updatedAt string
	var estimate float64
	var endeavourID, assigneeID, demandID, canceledReason sql.NullString

	err := db.QueryRow(
		`SELECT t.id, t.title, COALESCE(t.description, ''), t.status, COALESCE(t.estimate, 0),
		 (SELECT er.target_entity_id FROM entity_relation er
		  WHERE er.source_entity_type = 'task' AND er.source_entity_id = t.id
		  AND er.relationship_type = 'belongs_to' AND er.target_entity_type = 'endeavour' LIMIT 1),
		 (SELECT er.target_entity_id FROM entity_relation er
		  WHERE er.source_entity_type = 'task' AND er.source_entity_id = t.id
		  AND er.relationship_type = 'assigned_to' AND er.target_entity_type = 'resource' LIMIT 1),
		 (SELECT er.target_entity_id FROM entity_relation er
		  WHERE er.source_entity_type = 'task' AND er.source_entity_id = t.id
		  AND er.relationship_type = 'fulfills' AND er.target_entity_type = 'demand' LIMIT 1),
		 t.canceled_reason, t.created_at, COALESCE(t.updated_at, t.created_at)
		 FROM task t WHERE t.id = ?`, id,
	).Scan(&id, &title, &description, &taskStatus, &estimate,
		&endeavourID, &assigneeID, &demandID, &canceledReason, &createdAt, &updatedAt)

	if err == sql.ErrNoRows {
		return interviewToolError("not_found", fmt.Sprintf("Task %s not found", id)), nil
	}
	if err != nil {
		return interviewToolError("internal_error", err.Error()), nil
	}

	result := map[string]interface{}{
		"id":          id,
		"title":       title,
		"description": description,
		"status":      taskStatus,
		"created_at":  createdAt,
		"updated_at":  updatedAt,
	}
	if estimate > 0 {
		result["estimate"] = estimate
	}
	if endeavourID.Valid {
		result["endeavour_id"] = endeavourID.String
	}
	if assigneeID.Valid {
		result["assignee_id"] = assigneeID.String
	}
	if demandID.Valid {
		result["demand_id"] = demandID.String
	}
	if canceledReason.Valid {
		result["canceled_reason"] = canceledReason.String
	}

	return interviewToolSuccess(result), nil
}

// handleSimDmdCreate handles ts.dmd.create in the interview simulation.
func (s *InterviewServer) handleSimDmdCreate(_ context.Context, req *mcp.CallToolRequest, session *InterviewSession) (*mcp.CallToolResult, error) {
	args := interviewParseArgs(req)
	dmdType := interviewGetString(args, "type")
	title := interviewGetString(args, "title")
	if dmdType == "" || title == "" {
		return interviewToolError("invalid_input", "Type and title are required"), nil
	}

	id := fmt.Sprintf("dmd_%s", generateShortID())
	now := storage.UTCNow().Format(time.RFC3339)

	description := interviewGetString(args, "description")
	priority := interviewGetString(args, "priority")
	if priority == "" {
		priority = "medium"
	}
	endeavourID := interviewGetString(args, "endeavour_id")
	dueDate := interviewGetString(args, "due_date")

	var dd *string
	if dueDate != "" {
		dd = &dueDate
	}

	_, err := session.SimDB.Exec(
		`INSERT INTO demand (id, type, title, description, priority, status, due_date, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, 'open', ?, ?, ?)`,
		id, dmdType, title, description, priority, dd, now, now,
	)
	if err != nil {
		return interviewToolError("internal_error", fmt.Sprintf("Failed to create demand: %v", err)), nil
	}

	// Create entity_relation for endeavour link
	if endeavourID != "" {
		_, _ = session.SimDB.Exec(
			`INSERT INTO entity_relation (id, relationship_type, source_entity_type, source_entity_id, target_entity_type, target_entity_id, created_at)
			 VALUES (?, 'belongs_to', 'demand', ?, 'endeavour', ?, ?)`,
			fmt.Sprintf("rel_%s", generateShortID()), id, endeavourID, now,
		)
	}

	// Track the created demand ID for cross-section reference
	session.mu.Lock()
	if session.CreatedDemandID == "" {
		session.CreatedDemandID = id
	}
	session.mu.Unlock()

	result := map[string]interface{}{
		"id":         id,
		"type":       dmdType,
		"title":      title,
		"priority":   priority,
		"status":     "open",
		"created_at": now,
	}
	if description != "" {
		result["description"] = description
	}
	if endeavourID != "" {
		result["endeavour_id"] = endeavourID
	}

	return interviewToolSuccess(result), nil
}

// handleSimDmdList handles ts.dmd.list in the interview simulation.
func (s *InterviewServer) handleSimDmdList(_ context.Context, req *mcp.CallToolRequest, session *InterviewSession) (*mcp.CallToolResult, error) {
	args := interviewParseArgs(req)
	status := interviewGetString(args, "status")
	dmdType := interviewGetString(args, "type")
	priority := interviewGetString(args, "priority")
	endeavourID := interviewGetString(args, "endeavour_id")
	search := interviewGetString(args, "search")

	query := `SELECT d.id, d.type, d.title, COALESCE(d.description, ''), d.priority, d.status,
		 (SELECT er.target_entity_id FROM entity_relation er
		  WHERE er.source_entity_type = 'demand' AND er.source_entity_id = d.id
		  AND er.relationship_type = 'belongs_to' AND er.target_entity_type = 'endeavour' LIMIT 1) AS endeavour_id,
		 d.created_at FROM demand d WHERE 1=1`
	var vals []interface{}

	if status != "" {
		query += " AND d.status = ?"
		vals = append(vals, status)
	}
	if dmdType != "" {
		query += " AND d.type = ?"
		vals = append(vals, dmdType)
	}
	if priority != "" {
		query += " AND d.priority = ?"
		vals = append(vals, priority)
	}
	if endeavourID != "" {
		query += ` AND EXISTS (SELECT 1 FROM entity_relation er
		  WHERE er.source_entity_type = 'demand' AND er.source_entity_id = d.id
		  AND er.relationship_type = 'belongs_to' AND er.target_entity_type = 'endeavour'
		  AND er.target_entity_id = ?)`
		vals = append(vals, endeavourID)
	}
	if search != "" {
		query += " AND (d.title LIKE ? OR d.description LIKE ?)"
		vals = append(vals, "%"+search+"%", "%"+search+"%")
	}

	query += " ORDER BY d.created_at DESC"

	rows, err := session.SimDB.Query(query, vals...)
	if err != nil {
		return interviewToolError("internal_error", err.Error()), nil
	}
	defer func() { _ = rows.Close() }()

	var demands []map[string]interface{}
	for rows.Next() {
		var id, dType, title, description, prio, dStatus, createdAt string
		var edvID sql.NullString
		if err := rows.Scan(&id, &dType, &title, &description, &prio, &dStatus, &edvID, &createdAt); err != nil {
			return interviewToolError("internal_error", err.Error()), nil
		}
		demand := map[string]interface{}{
			"id":         id,
			"type":       dType,
			"title":      title,
			"priority":   prio,
			"status":     dStatus,
			"created_at": createdAt,
		}
		if description != "" {
			demand["description"] = description
		}
		if edvID.Valid {
			demand["endeavour_id"] = edvID.String
		}
		demands = append(demands, demand)
	}

	if demands == nil {
		demands = []map[string]interface{}{}
	}

	return interviewToolSuccess(map[string]interface{}{
		"data": demands,
		"meta": map[string]interface{}{
			"total":  len(demands),
			"limit":  50,
			"offset": 0,
		},
	}), nil
}

// handleSimRelCreate handles ts.rel.create in the interview simulation.
func (s *InterviewServer) handleSimRelCreate(_ context.Context, req *mcp.CallToolRequest, session *InterviewSession) (*mcp.CallToolResult, error) {
	args := interviewParseArgs(req)
	relType := interviewGetString(args, "relationship_type")
	srcType := interviewGetString(args, "source_entity_type")
	srcID := interviewGetString(args, "source_entity_id")
	tgtType := interviewGetString(args, "target_entity_type")
	tgtID := interviewGetString(args, "target_entity_id")

	if relType == "" || srcType == "" || srcID == "" || tgtType == "" || tgtID == "" {
		return interviewToolError("invalid_input", "relationship_type, source_entity_type, source_entity_id, target_entity_type, and target_entity_id are required"), nil
	}

	id := fmt.Sprintf("rel_%s", generateShortID())
	now := storage.UTCNow().Format(time.RFC3339)

	_, err := session.SimDB.Exec(
		`INSERT INTO entity_relation (id, relationship_type, source_entity_type, source_entity_id, target_entity_type, target_entity_id, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, relType, srcType, srcID, tgtType, tgtID, now,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			return interviewToolError("conflict", "This relationship already exists"), nil
		}
		return interviewToolError("internal_error", fmt.Sprintf("Failed to create relation: %v", err)), nil
	}

	return interviewToolSuccess(map[string]interface{}{
		"id":                 id,
		"relationship_type":  relType,
		"source_entity_type": srcType,
		"source_entity_id":   srcID,
		"target_entity_type": tgtType,
		"target_entity_id":   tgtID,
		"created_at":         now,
	}), nil
}

// handleSimRelList handles ts.rel.list in the interview simulation.
func (s *InterviewServer) handleSimRelList(_ context.Context, req *mcp.CallToolRequest, session *InterviewSession) (*mcp.CallToolResult, error) {
	args := interviewParseArgs(req)

	query := "SELECT id, relationship_type, source_entity_type, source_entity_id, target_entity_type, target_entity_id, created_at FROM entity_relation WHERE 1=1"
	var vals []interface{}

	if v := interviewGetString(args, "relationship_type"); v != "" {
		query += " AND relationship_type = ?"
		vals = append(vals, v)
	}
	if v := interviewGetString(args, "source_entity_type"); v != "" {
		query += " AND source_entity_type = ?"
		vals = append(vals, v)
	}
	if v := interviewGetString(args, "source_entity_id"); v != "" {
		query += " AND source_entity_id = ?"
		vals = append(vals, v)
	}
	if v := interviewGetString(args, "target_entity_type"); v != "" {
		query += " AND target_entity_type = ?"
		vals = append(vals, v)
	}
	if v := interviewGetString(args, "target_entity_id"); v != "" {
		query += " AND target_entity_id = ?"
		vals = append(vals, v)
	}

	query += " ORDER BY created_at DESC"

	rows, err := session.SimDB.Query(query, vals...)
	if err != nil {
		return interviewToolError("internal_error", err.Error()), nil
	}
	defer func() { _ = rows.Close() }()

	var relations []map[string]interface{}
	for rows.Next() {
		var id, relType, srcType, srcID, tgtType, tgtID, createdAt string
		if err := rows.Scan(&id, &relType, &srcType, &srcID, &tgtType, &tgtID, &createdAt); err != nil {
			return interviewToolError("internal_error", err.Error()), nil
		}
		relations = append(relations, map[string]interface{}{
			"id":                 id,
			"relationship_type":  relType,
			"source_entity_type": srcType,
			"source_entity_id":   srcID,
			"target_entity_type": tgtType,
			"target_entity_id":   tgtID,
			"created_at":         createdAt,
		})
	}

	if relations == nil {
		relations = []map[string]interface{}{}
	}

	return interviewToolSuccess(map[string]interface{}{
		"relations": relations,
		"total":     len(relations),
		"limit":     50,
		"offset":    0,
	}), nil
}

// generateShortID generates a short hex ID for simulation entities.
func generateShortID() string {
	b := make([]byte, 8)
	_, _ = cryptoRandRead(b)
	return fmt.Sprintf("%x", b)
}
