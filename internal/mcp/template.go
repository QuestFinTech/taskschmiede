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

// registerTemplateTools registers template MCP tools.
func (s *Server) registerTemplateTools(mcpServer *mcp.Server) {
	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.tpl.create",
			Description: "Create a new report template (Go text/template syntax, outputs Markdown)",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type":        "string",
						"description": "Template name (e.g., 'Task Report')",
					},
					"template_type": map[string]interface{}{
						"type":        "string",
						"description": "Template type (e.g., 'report')",
					},
					"scope": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"task", "demand", "endeavour"},
						"description": "Entity type this template generates reports for",
					},
					"lang": map[string]interface{}{
						"type":        "string",
						"description": "Language code (e.g., en, de, fr). Defaults to en.",
					},
					"body": map[string]interface{}{
						"type":        "string",
						"description": "Template body using Go text/template syntax with Markdown output",
					},
					"metadata": map[string]interface{}{
						"type":        "object",
						"description": "Arbitrary key-value pairs",
					},
				},
				"required": []string{"name", "scope", "body"},
			},
		},
		s.withSessionAuth(s.handleTplCreate),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.tpl.get",
			Description: "Retrieve a template by ID",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "Template ID",
					},
				},
				"required": []string{"id"},
			},
		},
		s.withSessionAuth(s.handleTplGet),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.tpl.list",
			Description: "Query templates with filters",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"scope": map[string]interface{}{
						"type":        "string",
						"description": "Filter by scope: task, demand, endeavour",
					},
					"lang": map[string]interface{}{
						"type":        "string",
						"description": "Filter by language code",
					},
					"status": map[string]interface{}{
						"type":        "string",
						"description": "Filter by status: active, archived",
					},
					"search": map[string]interface{}{
						"type":        "string",
						"description": "Search in name and body",
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
		s.withSessionAuth(s.handleTplList),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.tpl.update",
			Description: "Update a template (name, body, lang, status)",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "Template ID",
					},
					"name": map[string]interface{}{
						"type":        "string",
						"description": "New name",
					},
					"body": map[string]interface{}{
						"type":        "string",
						"description": "New template body",
					},
					"lang": map[string]interface{}{
						"type":        "string",
						"description": "New language code",
					},
					"status": map[string]interface{}{
						"type":        "string",
						"description": "New status: active, archived",
					},
					"metadata": map[string]interface{}{
						"type":        "object",
						"description": "Metadata to set",
					},
				},
				"required": []string{"id"},
			},
		},
		s.withSessionAuth(s.handleTplUpdate),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.tpl.fork",
			Description: "Fork a template (create a new version derived from an existing one)",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"source_id": map[string]interface{}{
						"type":        "string",
						"description": "The template to fork from",
					},
					"name": map[string]interface{}{
						"type":        "string",
						"description": "Name for the fork (defaults to source name)",
					},
					"body": map[string]interface{}{
						"type":        "string",
						"description": "Modified body (defaults to source body)",
					},
					"lang": map[string]interface{}{
						"type":        "string",
						"description": "Language code (defaults to source lang)",
					},
					"metadata": map[string]interface{}{
						"type":        "object",
						"description": "Arbitrary key-value pairs",
					},
				},
				"required": []string{"source_id"},
			},
		},
		s.withSessionAuth(s.handleTplFork),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.tpl.lineage",
			Description: "Walk the version chain for a template (oldest to newest)",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "Template ID to trace lineage from",
					},
				},
				"required": []string{"id"},
			},
		},
		s.withSessionAuth(s.handleTplLineage),
	)
}

// handleTplCreate handles the ts.tpl.create tool.
func (s *Server) handleTplCreate(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	authUser, err := requireAuth(ctx)
	if err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	args := parseArgs(req)
	name := getString(args, "name")
	scope := getString(args, "scope")
	body := getString(args, "body")

	if name == "" || scope == "" || body == "" {
		return toolError("invalid_input", "name, scope, and body are required"), nil
	}

	var metadata map[string]interface{}
	if m, ok := args["metadata"].(map[string]interface{}); ok {
		metadata = m
	}

	result, apiErr := s.api.CreateTemplate(ctx, name, getString(args, "template_type"), scope, getString(args, "lang"), body, authUser.UserID, metadata)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}
	return toolSuccess(result), nil
}

// handleTplGet handles the ts.tpl.get tool.
func (s *Server) handleTplGet(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := requireAuth(ctx); err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	id := getString(parseArgs(req), "id")
	if id == "" {
		return toolError("invalid_input", "Template ID is required"), nil
	}

	result, apiErr := s.api.GetTemplate(ctx, id)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}
	return toolSuccess(result), nil
}

// handleTplList handles the ts.tpl.list tool.
func (s *Server) handleTplList(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := requireAuth(ctx); err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	args := parseArgs(req)
	opts := storage.ListTemplatesOpts{
		Scope:  getString(args, "scope"),
		Lang:   getString(args, "lang"),
		Status: getString(args, "status"),
		Search: getString(args, "search"),
		Limit:  getInt(args, "limit", 50),
		Offset: getInt(args, "offset", 0),
	}

	items, total, apiErr := s.api.ListTemplates(ctx, opts)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}

	return toolSuccess(map[string]interface{}{
		"templates": items,
		"total":     total,
		"limit":     opts.Limit,
		"offset":    opts.Offset,
	}), nil
}

// handleTplUpdate handles the ts.tpl.update tool.
func (s *Server) handleTplUpdate(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := requireAuth(ctx); err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	args := parseArgs(req)
	id := getString(args, "id")
	if id == "" {
		return toolError("invalid_input", "Template ID is required"), nil
	}

	var fields storage.UpdateTemplateFields
	if v, ok := args["name"].(string); ok {
		fields.Name = &v
	}
	if v, ok := args["body"].(string); ok {
		fields.Body = &v
	}
	if v, ok := args["lang"].(string); ok {
		fields.Lang = &v
	}
	if v, ok := args["status"].(string); ok {
		fields.Status = &v
	}
	if v, ok := args["metadata"].(map[string]interface{}); ok {
		fields.Metadata = v
	}

	result, apiErr := s.api.UpdateTemplate(ctx, id, fields)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}
	return toolSuccess(result), nil
}

// handleTplFork handles the ts.tpl.fork tool.
func (s *Server) handleTplFork(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	authUser, err := requireAuth(ctx)
	if err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	args := parseArgs(req)
	sourceID := getString(args, "source_id")
	if sourceID == "" {
		return toolError("invalid_input", "source_id is required"), nil
	}

	var metadata map[string]interface{}
	if m, ok := args["metadata"].(map[string]interface{}); ok {
		metadata = m
	}

	result, apiErr := s.api.ForkTemplate(ctx, sourceID, getString(args, "name"), getString(args, "body"), getString(args, "lang"), authUser.UserID, metadata)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}
	return toolSuccess(result), nil
}

// handleTplLineage handles the ts.tpl.lineage tool.
func (s *Server) handleTplLineage(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := requireAuth(ctx); err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	id := getString(parseArgs(req), "id")
	if id == "" {
		return toolError("invalid_input", "Template ID is required"), nil
	}

	items, apiErr := s.api.GetTemplateLineage(ctx, id)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}

	return toolSuccess(map[string]interface{}{
		"lineage": items,
		"count":   len(items),
	}), nil
}
