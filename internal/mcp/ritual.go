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
	"github.com/QuestFinTech/taskschmiede/internal/security"
	"github.com/QuestFinTech/taskschmiede/internal/storage"
)

// registerRitualTools registers ritual MCP tools.
func (s *Server) registerRitualTools(mcpServer *mcp.Server) {
	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.rtl.create",
			Description: "Create a new ritual (stored methodology prompt)",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type":        "string",
						"description": "Ritual name (e.g., 'Weekly planning (Shape Up)')",
					},
					"prompt": map[string]interface{}{
						"type":        "string",
						"description": "The methodology prompt (free-form text, BYOM core)",
					},
					"description": map[string]interface{}{
						"type":        "string",
						"description": "Longer explanation of the ritual",
					},
					"lang": map[string]interface{}{
						"type":        "string",
						"description": "Language code (e.g., en, de, fr). Defaults to en.",
					},
					"endeavour_id": map[string]interface{}{
						"type":        "string",
						"description": "Endeavour this ritual governs",
					},
					"schedule": map[string]interface{}{
						"type":        "object",
						"description": "Schedule metadata: {\"type\":\"cron|interval|manual\", ...} (informational only)",
					},
					"metadata": map[string]interface{}{
						"type":        "object",
						"description": "Arbitrary key-value pairs",
					},
				},
				"required": []string{"name", "prompt"},
			},
		},
		s.withSessionAuth(s.handleRtlCreate),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.rtl.get",
			Description: "Retrieve a ritual by ID",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "Ritual ID",
					},
				},
				"required": []string{"id"},
			},
		},
		s.withSessionAuth(s.handleRtlGet),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.rtl.list",
			Description: "Query rituals with filters",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"endeavour_id": map[string]interface{}{
						"type":        "string",
						"description": "Filter by endeavour (via governs relationship)",
					},
					"is_enabled": map[string]interface{}{
						"type":        "boolean",
						"description": "Filter by enabled/disabled",
					},
					"status": map[string]interface{}{
						"type":        "string",
						"description": "Filter by status: active, archived",
					},
					"origin": map[string]interface{}{
						"type":        "string",
						"description": "Filter by origin: template, custom, fork",
					},
					"search": map[string]interface{}{
						"type":        "string",
						"description": "Search in name and description",
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
		s.withSessionAuth(s.handleRtlList),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.rtl.update",
			Description: "Update ritual attributes (cannot change prompt -- create new version instead)",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "Ritual ID",
					},
					"name": map[string]interface{}{
						"type":        "string",
						"description": "New name",
					},
					"description": map[string]interface{}{
						"type":        "string",
						"description": "New description",
					},
					"schedule": map[string]interface{}{
						"type":        "object",
						"description": "New schedule metadata",
					},
					"lang": map[string]interface{}{
						"type":        "string",
						"description": "Language code (e.g., en, de, fr)",
					},
					"is_enabled": map[string]interface{}{
						"type":        "boolean",
						"description": "Enable or disable the ritual",
					},
					"status": map[string]interface{}{
						"type":        "string",
						"description": "New status: active, archived",
					},
					"endeavour_id": map[string]interface{}{
						"type":        "string",
						"description": "New endeavour (empty string to unlink)",
					},
					"metadata": map[string]interface{}{
						"type":        "object",
						"description": "Metadata to set (replaces existing)",
					},
				},
				"required": []string{"id"},
			},
		},
		s.withSessionAuth(s.handleRtlUpdate),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.rtl.fork",
			Description: `Fork a ritual (create a new ritual derived from an existing one).

Example -- fork with a modified prompt:
  {"source_id": "rtl_...", "name": "My variant", "prompt": "Updated methodology..."}`,
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"source_id": map[string]interface{}{
						"type":        "string",
						"description": "The ritual to fork from",
					},
					"name": map[string]interface{}{
						"type":        "string",
						"description": "Name for the fork (defaults to source name)",
					},
					"prompt": map[string]interface{}{
						"type":        "string",
						"description": "Modified prompt (defaults to source prompt)",
					},
					"description": map[string]interface{}{
						"type":        "string",
						"description": "Description (defaults to source description)",
					},
					"endeavour_id": map[string]interface{}{
						"type":        "string",
						"description": "Endeavour the forked ritual governs",
					},
					"schedule": map[string]interface{}{
						"type":        "object",
						"description": "Schedule metadata (defaults to source schedule)",
					},
					"metadata": map[string]interface{}{
						"type":        "object",
						"description": "Arbitrary key-value pairs",
					},
				},
				"required": []string{"source_id"},
			},
		},
		s.withSessionAuth(s.handleRtlFork),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.rtl.lineage",
			Description: "Walk the version chain for a ritual (oldest to newest)",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "Ritual ID to trace lineage from",
					},
				},
				"required": []string{"id"},
			},
		},
		s.withSessionAuth(s.handleRtlLineage),
	)
}

// handleRtlCreate handles the ts.rtl.create tool.
func (s *Server) handleRtlCreate(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	authUser, err := requireAuth(ctx)
	if err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	args := parseArgs(req)
	name := getString(args, "name")
	prompt := getString(args, "prompt")

	if name == "" || prompt == "" {
		return toolError("invalid_input", "name and prompt are required"), nil
	}

	var schedule map[string]interface{}
	if m, ok := args["schedule"].(map[string]interface{}); ok {
		schedule = m
	}
	var metadata map[string]interface{}
	if m, ok := args["metadata"].(map[string]interface{}); ok {
		metadata = m
	}

	result, apiErr := s.api.CreateRitual(ctx, name, getString(args, "description"), prompt, "", authUser.UserID, getString(args, "endeavour_id"), getString(args, "lang"), schedule, metadata)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}
	security.FrameMapFields(result, security.RitualFrameSpec)
	return toolSuccess(result), nil
}

// handleRtlGet handles the ts.rtl.get tool.
func (s *Server) handleRtlGet(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := requireAuth(ctx); err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	id := getString(parseArgs(req), "id")
	if id == "" {
		return toolError("invalid_input", "Ritual ID is required"), nil
	}

	result, apiErr := s.api.GetRitual(ctx, id)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}
	security.FrameMapFields(result, security.RitualFrameSpec)
	return toolSuccess(result), nil
}

// handleRtlList handles the ts.rtl.list tool.
func (s *Server) handleRtlList(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := requireAuth(ctx); err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	args := parseArgs(req)

	opts := storage.ListRitualsOpts{
		EndeavourID:  getString(args, "endeavour_id"),
		EndeavourIDs: s.api.ResolveEndeavourIDs(ctx, false),
		Status:       getString(args, "status"),
		Origin:       getString(args, "origin"),
		Search:       getString(args, "search"),
		Limit:        getInt(args, "limit", 50),
		Offset:       getInt(args, "offset", 0),
	}

	if v, ok := args["is_enabled"].(bool); ok {
		opts.IsEnabled = &v
	}

	items, total, apiErr := s.api.ListRituals(ctx, opts)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}
	security.FrameMapSlice(items, security.RitualFrameSpec)

	return toolSuccess(map[string]interface{}{
		"rituals": items,
		"total":   total,
		"limit":   opts.Limit,
		"offset":  opts.Offset,
	}), nil
}

// handleRtlUpdate handles the ts.rtl.update tool.
func (s *Server) handleRtlUpdate(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := requireAuth(ctx); err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	args := parseArgs(req)
	id := getString(args, "id")
	if id == "" {
		return toolError("invalid_input", "Ritual ID is required"), nil
	}

	var fields storage.UpdateRitualFields
	if v, ok := args["name"].(string); ok {
		fields.Name = &v
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
	if v, ok := args["lang"].(string); ok {
		fields.Lang = &v
	}
	if v, ok := args["is_enabled"].(bool); ok {
		fields.IsEnabled = &v
	}
	if v, ok := args["schedule"].(map[string]interface{}); ok {
		fields.Schedule = v
	}
	if v, ok := args["metadata"].(map[string]interface{}); ok {
		fields.Metadata = v
	}

	result, apiErr := s.api.UpdateRitual(ctx, id, fields)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}
	security.FrameMapFields(result, security.RitualFrameSpec)
	return toolSuccess(result), nil
}

// handleRtlFork handles the ts.rtl.fork tool.
func (s *Server) handleRtlFork(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	authUser, err := requireAuth(ctx)
	if err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	args := parseArgs(req)
	sourceID := getString(args, "source_id")
	if sourceID == "" {
		return toolError("invalid_input", "source_id is required"), nil
	}

	var schedule map[string]interface{}
	if m, ok := args["schedule"].(map[string]interface{}); ok {
		schedule = m
	}
	var metadata map[string]interface{}
	if m, ok := args["metadata"].(map[string]interface{}); ok {
		metadata = m
	}

	result, apiErr := s.api.ForkRitual(ctx, sourceID, getString(args, "name"), getString(args, "prompt"), getString(args, "description"), authUser.UserID, getString(args, "endeavour_id"), getString(args, "lang"), schedule, metadata)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}
	security.FrameMapFields(result, security.RitualFrameSpec)
	return toolSuccess(result), nil
}

// handleRtlLineage handles the ts.rtl.lineage tool.
func (s *Server) handleRtlLineage(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := requireAuth(ctx); err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	id := getString(parseArgs(req), "id")
	if id == "" {
		return toolError("invalid_input", "Ritual ID is required"), nil
	}

	items, apiErr := s.api.GetRitualLineage(ctx, id)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}
	security.FrameMapSlice(items, security.RitualFrameSpec)

	return toolSuccess(map[string]interface{}{
		"lineage": items,
		"count":   len(items),
	}), nil
}
