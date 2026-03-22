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

// registerResourceTools registers resource MCP tools.
func (s *Server) registerResourceTools(mcpServer *mcp.Server) {
	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.res.create",
			Description: "Create a new resource (human, agent, service, or budget)",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"type": map[string]interface{}{
						"type":        "string",
						"description": "Resource type: human, agent, service, budget",
					},
					"name": map[string]interface{}{
						"type":        "string",
						"description": "Resource name",
					},
					"capacity_model": map[string]interface{}{
						"type":        "string",
						"description": "Capacity model: hours_per_week, tokens_per_day, always_on, budget",
					},
					"capacity_value": map[string]interface{}{
						"type":        "number",
						"description": "Amount of capacity",
					},
					"skills": map[string]interface{}{
						"type":        "array",
						"items":       map[string]interface{}{"type": "string"},
						"description": "List of skills or capabilities",
					},
					"metadata": map[string]interface{}{
						"type":        "object",
						"description": "Arbitrary key-value pairs (e.g., email, timezone, model_id)",
					},
				},
				"required": []string{"type", "name"},
			},
		},
		s.withSessionAuth(s.handleResCreate),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.res.get",
			Description: "Retrieve a resource by ID",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "Resource ID",
					},
				},
				"required": []string{"id"},
			},
		},
		s.withSessionAuth(s.handleResGet),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.res.update",
			Description: "Update resource attributes (partial update)",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "Resource ID",
					},
					"name": map[string]interface{}{
						"type":        "string",
						"description": "New name",
					},
					"capacity_model": map[string]interface{}{
						"type":        "string",
						"description": "Capacity model: hours_per_week, tokens_per_day, always_on, budget",
					},
					"capacity_value": map[string]interface{}{
						"type":        "number",
						"description": "Amount of capacity",
					},
					"skills": map[string]interface{}{
						"type":        "array",
						"items":       map[string]interface{}{"type": "string"},
						"description": "List of skills or capabilities (replaces existing)",
					},
					"metadata": map[string]interface{}{
						"type":        "object",
						"description": "Metadata to set (replaces existing)",
					},
					"status": map[string]interface{}{
						"type":        "string",
						"description": "New status: active, inactive",
					},
				},
				"required": []string{"id"},
			},
		},
		s.withSessionAuth(s.handleResUpdate),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.res.delete",
			Description: "Delete a team resource (org admin only)",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "Resource ID",
					},
				},
				"required": []string{"id"},
			},
		},
		s.withSessionAuth(s.handleResDelete),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.res.list",
			Description: "Query resources with filters",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"type": map[string]interface{}{
						"type":        "string",
						"description": "Filter by type: human, agent, service, budget",
					},
					"status": map[string]interface{}{
						"type":        "string",
						"description": "Filter by status: active, inactive",
					},
					"organization_id": map[string]interface{}{
						"type":        "string",
						"description": "Filter by organization membership",
					},
					"search": map[string]interface{}{
						"type":        "string",
						"description": "Search by name",
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
		s.withSessionAuth(s.handleResList),
	)
}

// handleResCreate handles the ts.res.create tool.
func (s *Server) handleResCreate(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := requireAuth(ctx); err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	args := parseArgs(req)
	resType := getString(args, "type")
	name := getString(args, "name")
	capacityModel := getString(args, "capacity_model")

	if resType == "" {
		return toolError("invalid_input", "Type is required"), nil
	}
	if name == "" {
		return toolError("invalid_input", "Name is required"), nil
	}

	var capacityValue *float64
	if v, ok := args["capacity_value"].(float64); ok {
		capacityValue = &v
	}

	var skills []string
	if skillsRaw, ok := args["skills"].([]interface{}); ok {
		for _, sk := range skillsRaw {
			if str, ok := sk.(string); ok {
				skills = append(skills, str)
			}
		}
	}

	var metadata map[string]interface{}
	if m, ok := args["metadata"].(map[string]interface{}); ok {
		metadata = m
	}

	result, apiErr := s.api.CreateResource(ctx, resType, name, capacityModel, capacityValue, skills, metadata)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}
	return toolSuccess(result), nil
}

// handleResGet handles the ts.res.get tool.
func (s *Server) handleResGet(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := requireAuth(ctx); err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	id := getString(parseArgs(req), "id")
	if id == "" {
		return toolError("invalid_input", "Resource ID is required"), nil
	}

	result, apiErr := s.api.GetResource(ctx, id)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}
	return toolSuccess(result), nil
}

// handleResUpdate handles the ts.res.update tool.
func (s *Server) handleResUpdate(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := requireAuth(ctx); err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	args := parseArgs(req)
	id := getString(args, "id")
	if id == "" {
		return toolError("invalid_input", "Resource ID is required"), nil
	}

	var fields storage.UpdateResourceFields
	if v, ok := args["name"].(string); ok {
		fields.Name = &v
	}
	if v, ok := args["capacity_model"].(string); ok {
		fields.CapacityModel = &v
	}
	if v, ok := args["capacity_value"].(float64); ok {
		fields.CapacityValue = &v
	}
	if skillsRaw, ok := args["skills"].([]interface{}); ok {
		skills := make([]string, 0, len(skillsRaw))
		for _, sk := range skillsRaw {
			if str, ok := sk.(string); ok {
				skills = append(skills, str)
			}
		}
		fields.Skills = skills
	}
	if m, ok := args["metadata"].(map[string]interface{}); ok {
		fields.Metadata = m
	}
	if v, ok := args["status"].(string); ok {
		fields.Status = &v
	}

	result, apiErr := s.api.UpdateResource(ctx, id, fields)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}
	return toolSuccess(result), nil
}

// handleResDelete handles the ts.res.delete tool.
func (s *Server) handleResDelete(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := requireAuth(ctx); err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	id := getString(parseArgs(req), "id")
	if id == "" {
		return toolError("invalid_input", "Resource ID is required"), nil
	}

	if apiErr := s.api.DeleteResource(ctx, id); apiErr != nil {
		return toolAPIError(apiErr), nil
	}

	return toolSuccess(map[string]interface{}{"deleted": true}), nil
}

// handleResList handles the ts.res.list tool.
func (s *Server) handleResList(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := requireAuth(ctx); err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	args := parseArgs(req)

	opts := storage.ListResourcesOpts{
		Type:           getString(args, "type"),
		Status:         getString(args, "status"),
		OrganizationID: getString(args, "organization_id"),
		Search:         getString(args, "search"),
		Limit:          getInt(args, "limit", 50),
		Offset:         getInt(args, "offset", 0),
	}

	items, total, apiErr := s.api.ListResources(ctx, opts, false)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}

	return toolSuccess(map[string]interface{}{
		"resources": items,
		"total":     total,
		"limit":     opts.Limit,
		"offset":    opts.Offset,
	}), nil
}
