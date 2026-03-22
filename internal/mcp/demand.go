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

// registerDemandTools registers demand MCP tools.
func (s *Server) registerDemandTools(mcpServer *mcp.Server) {
	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.dmd.create",
			Description: "Create a new demand (what needs to be fulfilled)",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"type": map[string]interface{}{
						"type":        "string",
						"description": "Demand type (e.g., feature, bug, goal, meeting, epic)",
					},
					"title": map[string]interface{}{
						"type":        "string",
						"description": "Demand title",
					},
					"description": map[string]interface{}{
						"type":        "string",
						"description": "Detailed description",
					},
					"priority": map[string]interface{}{
						"type":        "string",
						"description": "Priority: low, medium (default), high, urgent",
					},
					"endeavour_id": map[string]interface{}{
						"type":        "string",
						"description": "Endeavour this demand belongs to",
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
				"required": []string{"type", "title"},
			},
		},
		s.withSessionAuth(s.handleDmdCreate),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.dmd.get",
			Description: "Retrieve a demand by ID",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "Demand ID",
					},
				},
				"required": []string{"id"},
			},
		},
		s.withSessionAuth(s.handleDmdGet),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.dmd.list",
			Description: "Query demands with filters",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"status": map[string]interface{}{
						"type":        "string",
						"description": "Filter by status: open, in_progress, fulfilled, canceled",
					},
					"type": map[string]interface{}{
						"type":        "string",
						"description": "Filter by demand type",
					},
					"priority": map[string]interface{}{
						"type":        "string",
						"description": "Filter by priority: low, medium, high, urgent",
					},
					"endeavour_id": map[string]interface{}{
						"type":        "string",
						"description": "Filter by endeavour",
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
				},
			},
		},
		s.withSessionAuth(s.handleDmdList),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.dmd.update",
			Description: `Update demand attributes (partial update).

To cancel a demand, both status and canceled_reason are required:
  {"id": "dmd_...", "status": "canceled", "canceled_reason": "No longer needed"}`,
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "Demand ID",
					},
					"title": map[string]interface{}{
						"type":        "string",
						"description": "New title",
					},
					"description": map[string]interface{}{
						"type":        "string",
						"description": "New description",
					},
					"type": map[string]interface{}{
						"type":        "string",
						"description": "New demand type",
					},
					"status": map[string]interface{}{
						"type":        "string",
						"description": "New status: open, in_progress, fulfilled, canceled",
					},
					"priority": map[string]interface{}{
						"type":        "string",
						"description": "New priority: low, medium, high, urgent",
					},
					"endeavour_id": map[string]interface{}{
						"type":        "string",
						"description": "New endeavour (empty string to unlink)",
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
		s.withSessionAuth(s.handleDmdUpdate),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.dmd.cancel",
			Description: "Cancel a demand with a reason",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "Demand ID",
					},
					"reason": map[string]interface{}{
						"type":        "string",
						"description": "Reason for cancellation",
					},
				},
				"required": []string{"id", "reason"},
			},
		},
		s.withSessionAuth(s.handleDmdCancel),
	)
}

// handleDmdCreate handles the ts.dmd.create tool.
func (s *Server) handleDmdCreate(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := requireAuth(ctx); err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	args := parseArgs(req)
	demandType := getString(args, "type")
	title := getString(args, "title")

	if demandType == "" || title == "" {
		return toolError("invalid_input", "type and title are required"), nil
	}

	description := getString(args, "description")
	priority := getString(args, "priority")
	endeavourID := getString(args, "endeavour_id")

	var dueDate *time.Time
	if ds := getString(args, "due_date"); ds != "" {
		if t, err := time.Parse(time.RFC3339, ds); err == nil {
			dueDate = &t
		}
	}

	var metadata map[string]interface{}
	if m, ok := args["metadata"].(map[string]interface{}); ok {
		metadata = m
	}

	result, apiErr := s.api.CreateDemand(ctx, demandType, title, description, priority, endeavourID, dueDate, metadata)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}
	security.FrameMapFields(result, security.DemandFrameSpec)

	return toolSuccess(result), nil
}

// handleDmdGet handles the ts.dmd.get tool.
func (s *Server) handleDmdGet(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := requireAuth(ctx); err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	args := parseArgs(req)
	id := getString(args, "id")

	if id == "" {
		return toolError("invalid_input", "Demand ID is required"), nil
	}

	result, apiErr := s.api.GetDemand(ctx, id)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}
	security.FrameMapFields(result, security.DemandFrameSpec)

	return toolSuccess(result), nil
}

// handleDmdList handles the ts.dmd.list tool.
func (s *Server) handleDmdList(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := requireAuth(ctx); err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	args := parseArgs(req)

	opts := storage.ListDemandsOpts{
		EndeavourIDs: s.api.ResolveEndeavourIDs(ctx, false),
		Status:       getString(args, "status"),
		Type:         getString(args, "type"),
		Priority:     getString(args, "priority"),
		EndeavourID:  getString(args, "endeavour_id"),
		Search:       getString(args, "search"),
		Limit:        getInt(args, "limit", 50),
		Offset:       getInt(args, "offset", 0),
	}

	items, total, apiErr := s.api.ListDemands(ctx, opts)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}
	security.FrameMapSlice(items, security.DemandFrameSpec)

	return toolSuccess(map[string]interface{}{
		"demands": items,
		"total":   total,
		"limit":   opts.Limit,
		"offset":  opts.Offset,
	}), nil
}

// handleDmdUpdate handles the ts.dmd.update tool.
func (s *Server) handleDmdUpdate(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := requireAuth(ctx); err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	args := parseArgs(req)
	id := getString(args, "id")

	if id == "" {
		return toolError("invalid_input", "Demand ID is required"), nil
	}

	var fields storage.UpdateDemandFields

	if v, ok := args["title"].(string); ok {
		fields.Title = &v
	}
	if v, ok := args["description"].(string); ok {
		fields.Description = &v
	}
	if v, ok := args["type"].(string); ok {
		fields.Type = &v
	}
	if v, ok := args["status"].(string); ok {
		fields.Status = &v
	}
	if v, ok := args["priority"].(string); ok {
		fields.Priority = &v
	}
	if v, ok := args["endeavour_id"].(string); ok {
		fields.EndeavourID = &v
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

	result, apiErr := s.api.UpdateDemand(ctx, id, fields)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}
	security.FrameMapFields(result, security.DemandFrameSpec)

	return toolSuccess(result), nil
}

// handleDmdCancel handles the ts.dmd.cancel tool.
func (s *Server) handleDmdCancel(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := requireAuth(ctx); err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	args := parseArgs(req)
	id := getString(args, "id")
	reason := getString(args, "reason")

	if id == "" {
		return toolError("invalid_input", "Demand ID is required"), nil
	}
	if reason == "" {
		return toolError("invalid_input", "Cancellation reason is required"), nil
	}

	status := "canceled"
	fields := storage.UpdateDemandFields{
		Status:         &status,
		CanceledReason: &reason,
	}

	result, apiErr := s.api.UpdateDemand(ctx, id, fields)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}
	security.FrameMapFields(result, security.DemandFrameSpec)
	return toolSuccess(result), nil
}
