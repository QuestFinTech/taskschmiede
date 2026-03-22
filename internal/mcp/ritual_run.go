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

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/QuestFinTech/taskschmiede/internal/storage"
)

// registerRitualRunTools registers ritual run MCP tools.
func (s *Server) registerRitualRunTools(mcpServer *mcp.Server) {
	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.rtr.create",
			Description: "Create a ritual run (marks execution start, status=running)",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"ritual_id": map[string]interface{}{
						"type":        "string",
						"description": "Ritual ID to execute",
					},
					"trigger": map[string]interface{}{
						"type":        "string",
						"description": "What triggered the run: schedule, manual (default), api",
					},
					"metadata": map[string]interface{}{
						"type":        "object",
						"description": "Arbitrary key-value pairs",
					},
				},
				"required": []string{"ritual_id"},
			},
		},
		s.withSessionAuth(s.handleRtrCreate),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.rtr.get",
			Description: "Retrieve a ritual run by ID",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "Ritual run ID",
					},
				},
				"required": []string{"id"},
			},
		},
		s.withSessionAuth(s.handleRtrGet),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.rtr.list",
			Description: "Query ritual runs with filters",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"ritual_id": map[string]interface{}{
						"type":        "string",
						"description": "Filter by ritual",
					},
					"status": map[string]interface{}{
						"type":        "string",
						"description": "Filter by status: running, succeeded, failed, skipped",
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
		s.withSessionAuth(s.handleRtrList),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.rtr.update",
			Description: "Update a ritual run (status, results, effects, error)",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "Ritual run ID",
					},
					"status": map[string]interface{}{
						"type":        "string",
						"description": "New status: succeeded, failed, skipped",
					},
					"result_summary": map[string]interface{}{
						"type":        "string",
						"description": "Free-form summary of what happened",
					},
					"effects": map[string]interface{}{
						"type":        "object",
						"description": "Effects of the run: {\"tasks_created\":[], \"tasks_updated\":[], ...}",
					},
					"error": map[string]interface{}{
						"type":        "object",
						"description": "Error details if failed: {\"code\":\"...\", \"message\":\"...\"}",
					},
					"metadata": map[string]interface{}{
						"type":        "object",
						"description": "Metadata to set (replaces existing)",
					},
				},
				"required": []string{"id"},
			},
		},
		s.withSessionAuth(s.handleRtrUpdate),
	)
}

// handleRtrCreate handles the ts.rtr.create tool.
func (s *Server) handleRtrCreate(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	authUser, err := requireAuth(ctx)
	if err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	args := parseArgs(req)
	ritualID := getString(args, "ritual_id")
	if ritualID == "" {
		return toolError("invalid_input", "ritual_id is required"), nil
	}

	trigger := getString(args, "trigger")
	var metadata map[string]interface{}
	if m, ok := args["metadata"].(map[string]interface{}); ok {
		metadata = m
	}

	result, apiErr := s.api.CreateRitualRun(ctx, ritualID, trigger, authUser.UserID, metadata)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}
	return toolSuccess(result), nil
}

// handleRtrGet handles the ts.rtr.get tool.
func (s *Server) handleRtrGet(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := requireAuth(ctx); err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	id := getString(parseArgs(req), "id")
	if id == "" {
		return toolError("invalid_input", "Ritual run ID is required"), nil
	}

	result, apiErr := s.api.GetRitualRun(ctx, id)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}
	return toolSuccess(result), nil
}

// handleRtrList handles the ts.rtr.list tool.
func (s *Server) handleRtrList(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := requireAuth(ctx); err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	args := parseArgs(req)

	opts := storage.ListRitualRunsOpts{
		RitualID: getString(args, "ritual_id"),
		Status:   getString(args, "status"),
		Limit:    getInt(args, "limit", 50),
		Offset:   getInt(args, "offset", 0),
	}

	items, total, apiErr := s.api.ListRitualRuns(ctx, opts)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}

	return toolSuccess(map[string]interface{}{
		"runs":   items,
		"total":  total,
		"limit":  opts.Limit,
		"offset": opts.Offset,
	}), nil
}

// handleRtrUpdate handles the ts.rtr.update tool.
func (s *Server) handleRtrUpdate(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := requireAuth(ctx); err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	args := parseArgs(req)
	id := getString(args, "id")
	if id == "" {
		return toolError("invalid_input", "Ritual run ID is required"), nil
	}

	var fields storage.UpdateRitualRunFields
	if v, ok := args["status"].(string); ok {
		fields.Status = &v
	}
	if v, ok := args["result_summary"].(string); ok {
		fields.ResultSummary = &v
	}
	if v, ok := args["effects"].(map[string]interface{}); ok {
		fields.Effects = v
	}
	if v, ok := args["error"].(map[string]interface{}); ok {
		fields.Error = v
	}
	if v, ok := args["metadata"].(map[string]interface{}); ok {
		fields.Metadata = v
	}

	result, apiErr := s.api.UpdateRitualRun(ctx, id, fields)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}
	return toolSuccess(result), nil
}
