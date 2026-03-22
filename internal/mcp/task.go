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


package mcp

import (
	"context"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/QuestFinTech/taskschmiede/internal/security"
	"github.com/QuestFinTech/taskschmiede/internal/storage"
)

// registerTaskTools registers task MCP tools.
func (s *Server) registerTaskTools(mcpServer *mcp.Server) {
	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.tsk.create",
			Description: "Create a new task (atomic unit of work)",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"title": map[string]interface{}{
						"type":        "string",
						"description": "Task title",
					},
					"description": map[string]interface{}{
						"type":        "string",
						"description": "Detailed description",
					},
					"endeavour_id": map[string]interface{}{
						"type":        "string",
						"description": "Endeavour this task belongs to",
					},
					"demand_id": map[string]interface{}{
						"type":        "string",
						"description": "Demand this task belongs to",
					},
					"assignee_id": map[string]interface{}{
						"type":        "string",
						"description": "Resource ID to assign",
					},
					"estimate": map[string]interface{}{
						"type":        "number",
						"description": "Estimated hours",
					},
					"due_date": map[string]interface{}{
						"type":        "string",
						"description": "Due date (ISO 8601)",
					},
					"metadata": map[string]interface{}{
						"type":        "object",
						"description": "Arbitrary key-value pairs",
					},
				},
				"required": []string{"title"},
			},
		},
		s.withSessionAuth(s.handleTskCreate),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.tsk.get",
			Description: "Retrieve a task by ID",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "Task ID",
					},
				},
				"required": []string{"id"},
			},
		},
		s.withSessionAuth(s.handleTskGet),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.tsk.list",
			Description: "Query tasks with filters",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"status": map[string]interface{}{
						"type":        "string",
						"description": "Filter by status: planned, active, done, canceled",
					},
					"endeavour_id": map[string]interface{}{
						"type":        "string",
						"description": "Filter by endeavour",
					},
					"assignee_id": map[string]interface{}{
						"type":        "string",
						"description": "Filter by assignee resource",
					},
					"search": map[string]interface{}{
						"type":        "string",
						"description": "Search in title and description",
					},
					"limit": map[string]interface{}{
						"type":        "integer",
						"description": "Max results (default: 50)",
					},
					"offset": map[string]interface{}{
						"type":        "integer",
						"description": "Pagination offset",
					},
					"summary": map[string]interface{}{
						"type":        "boolean",
						"description": "If true, return status counts instead of individual tasks",
					},
				},
			},
		},
		s.withSessionAuth(s.handleTskList),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.tsk.update",
			Description: `Update task attributes (partial update).

To cancel a task, both status and canceled_reason are required:
  {"id": "tsk_...", "status": "canceled", "canceled_reason": "No longer needed"}`,
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "Task ID",
					},
					"title": map[string]interface{}{
						"type":        "string",
						"description": "New title",
					},
					"description": map[string]interface{}{
						"type":        "string",
						"description": "New description",
					},
					"status": map[string]interface{}{
						"type":        "string",
						"description": "New status: planned, active, done, canceled",
					},
					"endeavour_id": map[string]interface{}{
						"type":        "string",
						"description": "New endeavour (empty string to unlink)",
					},
					"demand_id": map[string]interface{}{
						"type":        "string",
						"description": "New demand (empty string to unlink)",
					},
					"assignee_id": map[string]interface{}{
						"type":        "string",
						"description": "New assignee resource (empty string to unassign)",
					},
					"estimate": map[string]interface{}{
						"type":        "number",
						"description": "Estimated hours",
					},
					"actual": map[string]interface{}{
						"type":        "number",
						"description": "Actual hours spent",
					},
					"due_date": map[string]interface{}{
						"type":        "string",
						"description": "Due date (ISO 8601, empty to clear)",
					},
					"owner_id": map[string]interface{}{
						"type":        "string",
						"description": "New owner resource (empty string to clear)",
					},
					"canceled_reason": map[string]interface{}{
						"type":        "string",
						"description": "Reason for cancellation (when status=canceled)",
					},
					"metadata": map[string]interface{}{
						"type":        "object",
						"description": "Metadata to set (replaces existing)",
					},
				},
				"required": []string{"id"},
			},
		},
		s.withSessionAuth(s.handleTskUpdate),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.tsk.cancel",
			Description: "Cancel a task with a reason",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "Task ID",
					},
					"reason": map[string]interface{}{
						"type":        "string",
						"description": "Reason for cancellation",
					},
				},
				"required": []string{"id", "reason"},
			},
		},
		s.withSessionAuth(s.handleTskCancel),
	)
}

// handleTskCreate handles the ts.tsk.create tool.
func (s *Server) handleTskCreate(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := requireAuth(ctx); err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	args := parseArgs(req)
	title := getString(args, "title")
	if title == "" {
		return toolError("invalid_input", "Title is required"), nil
	}

	var estimate *float64
	if v, ok := args["estimate"].(float64); ok {
		estimate = &v
	}

	var dueDate *time.Time
	if s := getString(args, "due_date"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			dueDate = &t
		}
	}

	var metadata map[string]interface{}
	if m, ok := args["metadata"].(map[string]interface{}); ok {
		metadata = m
	}

	result, apiErr := s.api.CreateTask(ctx, title, getString(args, "description"),
		getString(args, "endeavour_id"), getString(args, "demand_id"), getString(args, "assignee_id"),
		estimate, dueDate, metadata)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}
	security.FrameMapFields(result, security.TaskFrameSpec)
	return toolSuccess(result), nil
}

// handleTskGet handles the ts.tsk.get tool.
func (s *Server) handleTskGet(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := requireAuth(ctx); err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	id := getString(parseArgs(req), "id")
	if id == "" {
		return toolError("invalid_input", "Task ID is required"), nil
	}

	result, apiErr := s.api.GetTask(ctx, id)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}
	security.FrameMapFields(result, security.TaskFrameSpec)
	return toolSuccess(result), nil
}

// handleTskList handles the ts.tsk.list tool.
func (s *Server) handleTskList(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := requireAuth(ctx); err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	args := parseArgs(req)
	assigneeID := getString(args, "assignee_id")
	assigneeID = s.api.ResolveAssigneeMe(ctx, assigneeID)

	opts := storage.ListTasksOpts{
		Status:       getString(args, "status"),
		EndeavourID:  getString(args, "endeavour_id"),
		AssigneeID:   assigneeID,
		Search:       getString(args, "search"),
		Limit:        getInt(args, "limit", 50),
		Offset:       getInt(args, "offset", 0),
		EndeavourIDs: s.api.ResolveEndeavourIDs(ctx, false),
	}

	// Summary mode: return status counts instead of individual tasks.
	if summary, ok := args["summary"].(bool); ok && summary {
		result, apiErr := s.api.TaskSummary(ctx, opts)
		if apiErr != nil {
			return toolAPIError(apiErr), nil
		}
		return toolSuccess(result), nil
	}

	items, total, apiErr := s.api.ListTasks(ctx, opts)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}
	security.FrameMapSlice(items, security.TaskFrameSpec)

	return toolSuccess(map[string]interface{}{
		"tasks":  items,
		"total":  total,
		"limit":  opts.Limit,
		"offset": opts.Offset,
	}), nil
}

// handleTskUpdate handles the ts.tsk.update tool.
func (s *Server) handleTskUpdate(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := requireAuth(ctx); err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	args := parseArgs(req)
	id := getString(args, "id")
	if id == "" {
		return toolError("invalid_input", "Task ID is required"), nil
	}

	var fields storage.UpdateTaskFields
	if v, ok := args["title"].(string); ok {
		fields.Title = &v
	}
	if v, ok := args["description"].(string); ok {
		fields.Description = &v
	}
	if v, ok := args["status"].(string); ok {
		fields.Status = &v
	}
	if v, ok := args["endeavour_id"].(string); ok {
		fields.EndeavourID = &v
	}
	if v, ok := args["demand_id"].(string); ok {
		fields.DemandID = &v
	}
	if v, ok := args["assignee_id"].(string); ok {
		fields.AssigneeID = &v
	}
	if v, ok := args["estimate"].(float64); ok {
		fields.Estimate = &v
	}
	if v, ok := args["actual"].(float64); ok {
		fields.Actual = &v
	}
	if v, ok := args["due_date"].(string); ok {
		fields.DueDate = &v
	}
	if v, ok := args["owner_id"].(string); ok {
		fields.OwnerID = &v
	}
	if v, ok := args["canceled_reason"].(string); ok {
		fields.CanceledReason = &v
	}
	if v, ok := args["metadata"].(map[string]interface{}); ok {
		fields.Metadata = v
	}

	result, apiErr := s.api.UpdateTask(ctx, id, fields)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}
	security.FrameMapFields(result, security.TaskFrameSpec)
	return toolSuccess(result), nil
}

// handleTskCancel handles the ts.tsk.cancel tool.
func (s *Server) handleTskCancel(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := requireAuth(ctx); err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	args := parseArgs(req)
	id := getString(args, "id")
	reason := getString(args, "reason")

	if id == "" {
		return toolError("invalid_input", "Task ID is required"), nil
	}
	if reason == "" {
		return toolError("invalid_input", "Cancellation reason is required"), nil
	}

	status := "canceled"
	fields := storage.UpdateTaskFields{
		Status:         &status,
		CanceledReason: &reason,
	}

	result, apiErr := s.api.UpdateTask(ctx, id, fields)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}
	security.FrameMapFields(result, security.TaskFrameSpec)
	return toolSuccess(result), nil
}
