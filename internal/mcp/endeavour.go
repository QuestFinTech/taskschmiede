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
	"github.com/QuestFinTech/taskschmiede/internal/storage"
)

// registerEndeavourTools registers endeavour MCP tools.
func (s *Server) registerEndeavourTools(mcpServer *mcp.Server) {
	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.edv.create",
			Description: "Create a new endeavour (container for related work toward a goal)",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type":        "string",
						"description": "Endeavour name",
					},
					"description": map[string]interface{}{
						"type":        "string",
						"description": "Detailed description",
					},
					"goals": map[string]interface{}{
						"type":        "array",
						"items":       map[string]interface{}{"type": "object"},
						"description": "Success criteria / goals. Each goal can be a string (title only) or an object with title, status (open/achieved/abandoned), linked_entity_type, and linked_entity_id.",
					},
					"start_date": map[string]interface{}{
						"type":        "string",
						"description": "Start date (ISO 8601)",
					},
					"end_date": map[string]interface{}{
						"type":        "string",
						"description": "End date (ISO 8601)",
					},
					"timezone": map[string]interface{}{
						"type":        "string",
						"description": "IANA timezone for this endeavour (e.g., Europe/Berlin). Defaults to UTC.",
					},
					"lang": map[string]interface{}{
						"type":        "string",
						"description": "Language code (e.g., en, de, fr). Defaults to en.",
					},
					"metadata": map[string]interface{}{
						"type":        "object",
						"description": "Arbitrary key-value pairs. Set auto_complete (boolean) to true to auto-complete the endeavour when all goals are achieved/abandoned and all demands are fulfilled/canceled",
					},
				},
				"required": []string{"name"},
			},
		},
		s.withSessionAuth(s.handleEdvCreate),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.edv.get",
			Description: "Retrieve an endeavour by ID with progress summary",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "Endeavour ID",
					},
				},
				"required": []string{"id"},
			},
		},
		s.withSessionAuth(s.handleEdvGet),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.edv.list",
			Description: "Query endeavours with filters",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"status": map[string]interface{}{
						"type":        "string",
						"description": "Filter by status: pending, active, on_hold, completed, archived",
					},
					"organization_id": map[string]interface{}{
						"type":        "string",
						"description": "Filter by organization",
					},
					"search": map[string]interface{}{
						"type":        "string",
						"description": "Search by name or description",
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
		s.withSessionAuth(s.handleEdvList),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.edv.update",
			Description: "Update endeavour attributes (partial update)",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "Endeavour ID",
					},
					"name": map[string]interface{}{
						"type":        "string",
						"description": "New name",
					},
					"description": map[string]interface{}{
						"type":        "string",
						"description": "New description",
					},
					"status": map[string]interface{}{
						"type":        "string",
						"description": "New status: pending, active, on_hold, completed",
					},
					"timezone": map[string]interface{}{
						"type":        "string",
						"description": "IANA timezone (e.g., Europe/Berlin). Defaults to UTC.",
					},
					"lang": map[string]interface{}{
						"type":        "string",
						"description": "Language code (e.g., en, de, fr). Defaults to en.",
					},
					"goals": map[string]interface{}{
						"type":        "array",
						"items":       map[string]interface{}{"type": "object"},
						"description": "Goals (replaces existing). Each item can be a string (title only) or an object with id, title, status, linked_entity_type, linked_entity_id.",
					},
					"start_date": map[string]interface{}{
						"type":        "string",
						"description": "Start date (ISO 8601, empty to clear)",
					},
					"end_date": map[string]interface{}{
						"type":        "string",
						"description": "End date (ISO 8601, empty to clear)",
					},
					"metadata": map[string]interface{}{
						"type":        "object",
						"description": "Metadata to set (replaces existing). Set auto_complete (boolean) to true to auto-complete the endeavour when all goals are achieved/abandoned and all demands are fulfilled/canceled",
					},
					"taskschmied_enabled": map[string]interface{}{
						"type":        "boolean",
						"description": "Enable or disable Taskschmied governance agent for this endeavour",
					},
				},
				"required": []string{"id"},
			},
		},
		s.withSessionAuth(s.handleEdvUpdate),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.edv.archive",
			Description: "Archive an endeavour (cancels all non-terminal tasks). Use confirm=false (default) for dry-run impact, confirm=true to execute.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "Endeavour ID",
					},
					"reason": map[string]interface{}{
						"type":        "string",
						"description": "Reason for archiving",
					},
					"confirm": map[string]interface{}{
						"type":        "boolean",
						"description": "Set to true to execute archive; false (default) returns dry-run impact",
					},
				},
				"required": []string{"id", "reason"},
			},
		},
		s.withSessionAuth(s.handleEdvArchive),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.edv.export",
			Description: "Export all endeavour data (tasks, demands, artifacts, rituals, comments, relations, messages) as JSON. Requires owner role.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"endeavour_id": map[string]interface{}{
						"type":        "string",
						"description": "Endeavour ID to export",
					},
				},
				"required": []string{"endeavour_id"},
			},
		},
		s.withSessionAuth(s.handleEdvExport),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.edv.add_member",
			Description: "Add a user to an endeavour",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"endeavour_id": map[string]interface{}{
						"type":        "string",
						"description": "Endeavour ID",
					},
					"user_id": map[string]interface{}{
						"type":        "string",
						"description": "User ID to add",
					},
					"role": map[string]interface{}{
						"type":        "string",
						"description": "Role: owner, admin, member, viewer (default: member)",
					},
				},
				"required": []string{"endeavour_id", "user_id"},
			},
		},
		s.withSessionAuth(s.handleEdvAddMember),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.edv.remove_member",
			Description: "Remove a user from an endeavour",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"endeavour_id": map[string]interface{}{
						"type":        "string",
						"description": "Endeavour ID",
					},
					"user_id": map[string]interface{}{
						"type":        "string",
						"description": "User ID to remove",
					},
				},
				"required": []string{"endeavour_id", "user_id"},
			},
		},
		s.withSessionAuth(s.handleEdvRemoveMember),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.edv.list_members",
			Description: "List members of an endeavour with their roles",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"endeavour_id": map[string]interface{}{
						"type":        "string",
						"description": "Endeavour ID",
					},
				},
				"required": []string{"endeavour_id"},
			},
		},
		s.withSessionAuth(s.handleEdvListMembers),
	)
}

// parseGoalsArg parses goals from MCP tool arguments.
// Accepts both legacy format (array of strings) and new format (array of objects).
func parseGoalsArg(args map[string]interface{}) []storage.Goal {
	goalsRaw, ok := args["goals"].([]interface{})
	if !ok {
		return nil
	}

	goals := make([]storage.Goal, 0, len(goalsRaw))
	for _, g := range goalsRaw {
		switch v := g.(type) {
		case string:
			// Legacy format: plain string
			goals = append(goals, storage.Goal{Title: v})
		case map[string]interface{}:
			// New format: structured object
			goal := storage.Goal{}
			if id, ok := v["id"].(string); ok {
				goal.ID = id
			}
			if title, ok := v["title"].(string); ok {
				goal.Title = title
			}
			if status, ok := v["status"].(string); ok {
				goal.Status = status
			}
			if let, ok := v["linked_entity_type"].(string); ok {
				goal.LinkedEntityType = let
			}
			if lei, ok := v["linked_entity_id"].(string); ok {
				goal.LinkedEntityID = lei
			}
			if goal.Title != "" {
				goals = append(goals, goal)
			}
		}
	}
	return goals
}

// handleEdvCreate handles the ts.edv.create tool.
func (s *Server) handleEdvCreate(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := requireAuth(ctx); err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	args := parseArgs(req)
	name := getString(args, "name")
	if name == "" {
		return toolError("invalid_input", "Name is required"), nil
	}

	description := getString(args, "description")
	goals := parseGoalsArg(args)

	// Parse dates
	var startDate, endDate *time.Time
	if sd := getString(args, "start_date"); sd != "" {
		if t, err := time.Parse(time.RFC3339, sd); err == nil {
			startDate = &t
		}
	}
	if ed := getString(args, "end_date"); ed != "" {
		if t, err := time.Parse(time.RFC3339, ed); err == nil {
			endDate = &t
		}
	}

	var metadata map[string]interface{}
	if m, ok := args["metadata"].(map[string]interface{}); ok {
		metadata = m
	}

	result, apiErr := s.api.CreateEndeavour(ctx, name, description, goals, startDate, endDate, metadata)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}
	return toolSuccess(result), nil
}

// handleEdvGet handles the ts.edv.get tool.
func (s *Server) handleEdvGet(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := requireAuth(ctx); err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	id := getString(parseArgs(req), "id")
	if id == "" {
		return toolError("invalid_input", "Endeavour ID is required"), nil
	}

	result, apiErr := s.api.GetEndeavour(ctx, id)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}
	return toolSuccess(result), nil
}

// handleEdvList handles the ts.edv.list tool.
func (s *Server) handleEdvList(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := requireAuth(ctx); err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	args := parseArgs(req)

	opts := storage.ListEndeavoursOpts{
		Status:         getString(args, "status"),
		OrganizationID: getString(args, "organization_id"),
		Search:         getString(args, "search"),
		Limit:          getInt(args, "limit", 50),
		Offset:         getInt(args, "offset", 0),
		EndeavourIDs:   s.api.ResolveEndeavourIDs(ctx, false),
	}

	items, total, apiErr := s.api.ListEndeavours(ctx, opts)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}

	return toolSuccess(map[string]interface{}{
		"endeavours": items,
		"total":      total,
		"limit":      opts.Limit,
		"offset":     opts.Offset,
	}), nil
}

// handleEdvUpdate handles the ts.edv.update tool.
func (s *Server) handleEdvUpdate(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := requireAuth(ctx); err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	args := parseArgs(req)
	id := getString(args, "id")
	if id == "" {
		return toolError("invalid_input", "Endeavour ID is required"), nil
	}

	var fields storage.UpdateEndeavourFields
	if v, ok := args["name"].(string); ok {
		fields.Name = &v
	}
	if v, ok := args["description"].(string); ok {
		fields.Description = &v
	}
	if v, ok := args["status"].(string); ok {
		fields.Status = &v
	}
	if v, ok := args["timezone"].(string); ok {
		fields.Timezone = &v
	}
	if v, ok := args["lang"].(string); ok {
		fields.Lang = &v
	}
	if _, ok := args["goals"]; ok {
		fields.Goals = parseGoalsArg(args)
		if fields.Goals == nil {
			fields.Goals = []storage.Goal{} // explicit empty = clear
		}
	}
	if v, ok := args["start_date"].(string); ok {
		fields.StartDate = &v
	}
	if v, ok := args["end_date"].(string); ok {
		fields.EndDate = &v
	}
	if v, ok := args["metadata"].(map[string]interface{}); ok {
		fields.Metadata = v
	}
	if v, ok := args["taskschmied_enabled"].(bool); ok {
		fields.TaskschmiedEnabled = &v
	}

	result, apiErr := s.api.UpdateEndeavour(ctx, id, fields)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}
	return toolSuccess(result), nil
}

// handleEdvArchive handles the ts.edv.archive tool.
func (s *Server) handleEdvArchive(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := requireAuth(ctx); err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	args := parseArgs(req)
	id := getString(args, "id")
	reason := getString(args, "reason")
	confirm := getBool(args, "confirm")

	if id == "" {
		return toolError("invalid_input", "Endeavour ID is required"), nil
	}
	if reason == "" {
		return toolError("invalid_input", "Reason is required"), nil
	}

	if !confirm {
		result, apiErr := s.api.EndeavourArchiveImpact(ctx, id)
		if apiErr != nil {
			return toolAPIError(apiErr), nil
		}
		result["confirm_hint"] = "Set confirm=true to execute this archive operation"
		return toolSuccess(result), nil
	}

	result, apiErr := s.api.ArchiveEndeavour(ctx, id, reason)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}
	return toolSuccess(result), nil
}

// handleEdvAddMember handles the ts.edv.add_member tool.
func (s *Server) handleEdvAddMember(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := requireAuth(ctx); err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	args := parseArgs(req)
	endeavourID := getString(args, "endeavour_id")
	userID := getString(args, "user_id")
	role := getString(args, "role")

	if endeavourID == "" || userID == "" {
		return toolError("invalid_input", "endeavour_id and user_id are required"), nil
	}

	if role == "" {
		role = "member"
	}

	result, apiErr := s.api.AddEndeavourMember(ctx, endeavourID, userID, role)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}
	return toolSuccess(result), nil
}

// handleEdvRemoveMember handles the ts.edv.remove_member tool.
func (s *Server) handleEdvRemoveMember(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := requireAuth(ctx); err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	args := parseArgs(req)
	endeavourID := getString(args, "endeavour_id")
	userID := getString(args, "user_id")

	if endeavourID == "" || userID == "" {
		return toolError("invalid_input", "endeavour_id and user_id are required"), nil
	}

	result, apiErr := s.api.RemoveEndeavourMember(ctx, endeavourID, userID)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}
	return toolSuccess(result), nil
}

// handleEdvListMembers handles the ts.edv.list_members tool.
func (s *Server) handleEdvListMembers(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := requireAuth(ctx); err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	args := parseArgs(req)
	endeavourID := getString(args, "endeavour_id")

	if endeavourID == "" {
		return toolError("invalid_input", "endeavour_id is required"), nil
	}

	members, apiErr := s.api.ListEndeavourMembers(ctx, endeavourID)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}

	return toolSuccess(map[string]interface{}{
		"members": members,
		"total":   len(members),
	}), nil
}

// handleEdvExport handles the ts.edv.export tool.
func (s *Server) handleEdvExport(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := requireAuth(ctx); err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	args := parseArgs(req)
	id := getString(args, "endeavour_id")
	if id == "" {
		return toolError("invalid_input", "endeavour_id is required"), nil
	}

	export, apiErr := s.api.ExportEndeavour(ctx, id)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}

	return toolSuccess(export), nil
}
