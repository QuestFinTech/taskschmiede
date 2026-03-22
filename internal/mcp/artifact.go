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

// registerArtifactTools registers artifact MCP tools.
func (s *Server) registerArtifactTools(mcpServer *mcp.Server) {
	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.art.create",
			Description: "Create a new artifact (reference to external doc, repo, dashboard, etc.)",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"kind": map[string]interface{}{
						"type":        "string",
						"description": "Artifact kind: link, doc, repo, file, dataset, dashboard, runbook, other",
					},
					"title": map[string]interface{}{
						"type":        "string",
						"description": "Artifact title",
					},
					"url": map[string]interface{}{
						"type":        "string",
						"description": "External URL",
					},
					"summary": map[string]interface{}{
						"type":        "string",
						"description": "1-3 line description",
					},
					"tags": map[string]interface{}{
						"type":        "array",
						"items":       map[string]interface{}{"type": "string"},
						"description": "Free-form string tags",
					},
					"endeavour_id": map[string]interface{}{
						"type":        "string",
						"description": "Endeavour this artifact belongs to",
					},
					"task_id": map[string]interface{}{
						"type":        "string",
						"description": "Task this artifact belongs to",
					},
					"metadata": map[string]interface{}{
						"type":        "object",
						"description": "Arbitrary key-value pairs",
					},
				},
				"required": []string{"kind", "title"},
			},
		},
		s.withSessionAuth(s.handleArtCreate),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.art.get",
			Description: "Retrieve an artifact by ID",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "Artifact ID",
					},
				},
				"required": []string{"id"},
			},
		},
		s.withSessionAuth(s.handleArtGet),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.art.list",
			Description: "Query artifacts with filters",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"endeavour_id": map[string]interface{}{
						"type":        "string",
						"description": "Filter by endeavour",
					},
					"task_id": map[string]interface{}{
						"type":        "string",
						"description": "Filter by task",
					},
					"kind": map[string]interface{}{
						"type":        "string",
						"description": "Filter by kind",
					},
					"status": map[string]interface{}{
						"type":        "string",
						"description": "Filter by status: active, archived",
					},
					"tags": map[string]interface{}{
						"type":        "string",
						"description": "Filter by tag (match any)",
					},
					"search": map[string]interface{}{
						"type":        "string",
						"description": "Search in title and summary",
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
		s.withSessionAuth(s.handleArtList),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.art.delete",
			Description: "Delete an artifact (sets status to deleted)",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "Artifact ID",
					},
				},
				"required": []string{"id"},
			},
		},
		s.withSessionAuth(s.handleArtDelete),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.art.update",
			Description: "Update artifact attributes (partial update)",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "Artifact ID",
					},
					"title": map[string]interface{}{
						"type":        "string",
						"description": "New title",
					},
					"kind": map[string]interface{}{
						"type":        "string",
						"description": "New kind",
					},
					"url": map[string]interface{}{
						"type":        "string",
						"description": "New URL",
					},
					"summary": map[string]interface{}{
						"type":        "string",
						"description": "New summary",
					},
					"tags": map[string]interface{}{
						"type":        "array",
						"items":       map[string]interface{}{"type": "string"},
						"description": "New tags (replaces existing)",
					},
					"status": map[string]interface{}{
						"type":        "string",
						"description": "New status: active, archived",
					},
					"endeavour_id": map[string]interface{}{
						"type":        "string",
						"description": "New endeavour (empty string to unlink)",
					},
					"task_id": map[string]interface{}{
						"type":        "string",
						"description": "New task (empty string to unlink)",
					},
					"metadata": map[string]interface{}{
						"type":        "object",
						"description": "Metadata to set (replaces existing)",
					},
				},
				"required": []string{"id"},
			},
		},
		s.withSessionAuth(s.handleArtUpdate),
	)
}

// handleArtCreate handles the ts.art.create tool.
func (s *Server) handleArtCreate(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	authUser, err := requireAuth(ctx)
	if err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	args := parseArgs(req)
	kind := getString(args, "kind")
	title := getString(args, "title")

	if kind == "" || title == "" {
		return toolError("invalid_input", "kind and title are required"), nil
	}

	url := getString(args, "url")
	summary := getString(args, "summary")
	endeavourID := getString(args, "endeavour_id")
	taskID := getString(args, "task_id")

	var tags []string
	if rawTags, ok := args["tags"].([]interface{}); ok {
		for _, t := range rawTags {
			if s, ok := t.(string); ok {
				tags = append(tags, s)
			}
		}
	}

	var metadata map[string]interface{}
	if m, ok := args["metadata"].(map[string]interface{}); ok {
		metadata = m
	}

	result, apiErr := s.api.CreateArtifact(ctx, kind, title, url, summary, tags, metadata, authUser.UserID, endeavourID, taskID)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}
	security.FrameMapFields(result, security.ArtifactFrameSpec)

	return toolSuccess(result), nil
}

// handleArtGet handles the ts.art.get tool.
func (s *Server) handleArtGet(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := requireAuth(ctx); err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	args := parseArgs(req)
	id := getString(args, "id")

	if id == "" {
		return toolError("invalid_input", "Artifact ID is required"), nil
	}

	result, apiErr := s.api.GetArtifact(ctx, id)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}
	security.FrameMapFields(result, security.ArtifactFrameSpec)

	return toolSuccess(result), nil
}

// handleArtList handles the ts.art.list tool.
func (s *Server) handleArtList(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := requireAuth(ctx); err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	args := parseArgs(req)

	opts := storage.ListArtifactsOpts{
		EndeavourIDs: s.api.ResolveEndeavourIDs(ctx, false),
		EndeavourID:  getString(args, "endeavour_id"),
		TaskID:       getString(args, "task_id"),
		Kind:         getString(args, "kind"),
		Status:       getString(args, "status"),
		Tags:         getString(args, "tags"),
		Search:       getString(args, "search"),
		Limit:        getInt(args, "limit", 50),
		Offset:       getInt(args, "offset", 0),
	}

	items, total, apiErr := s.api.ListArtifacts(ctx, opts)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}
	security.FrameMapSlice(items, security.ArtifactFrameSpec)

	return toolSuccess(map[string]interface{}{
		"artifacts": items,
		"total":     total,
		"limit":     opts.Limit,
		"offset":    opts.Offset,
	}), nil
}

// handleArtDelete handles the ts.art.delete tool.
func (s *Server) handleArtDelete(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := requireAuth(ctx); err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	args := parseArgs(req)
	id := getString(args, "id")

	if id == "" {
		return toolError("invalid_input", "Artifact ID is required"), nil
	}

	result, apiErr := s.api.DeleteArtifact(ctx, id)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}

	return toolSuccess(result), nil
}

// handleArtUpdate handles the ts.art.update tool.
func (s *Server) handleArtUpdate(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := requireAuth(ctx); err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	args := parseArgs(req)
	id := getString(args, "id")

	if id == "" {
		return toolError("invalid_input", "Artifact ID is required"), nil
	}

	var fields storage.UpdateArtifactFields

	if v, ok := args["title"].(string); ok {
		fields.Title = &v
	}
	if v, ok := args["kind"].(string); ok {
		fields.Kind = &v
	}
	if v, ok := args["url"].(string); ok {
		fields.URL = &v
	}
	if v, ok := args["summary"].(string); ok {
		fields.Summary = &v
	}
	if v, ok := args["status"].(string); ok {
		fields.Status = &v
	}
	if v, ok := args["endeavour_id"].(string); ok {
		fields.EndeavourID = &v
	}
	if v, ok := args["task_id"].(string); ok {
		fields.TaskID = &v
	}
	if v, ok := args["metadata"].(map[string]interface{}); ok {
		fields.Metadata = v
	}
	if rawTags, ok := args["tags"].([]interface{}); ok {
		var tags []string
		for _, t := range rawTags {
			if tagStr, ok := t.(string); ok {
				tags = append(tags, tagStr)
			}
		}
		fields.Tags = &tags
	}

	result, apiErr := s.api.UpdateArtifact(ctx, id, fields)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}
	security.FrameMapFields(result, security.ArtifactFrameSpec)

	return toolSuccess(result), nil
}
