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

// registerCommentTools registers comment MCP tools.
func (s *Server) registerCommentTools(mcpServer *mcp.Server) {
	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.cmt.create",
			Description: "Add a comment to an entity (task, demand, endeavour, artifact, ritual, organization)",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"entity_type": map[string]interface{}{
						"type":        "string",
						"description": "Entity type: task, demand, endeavour, artifact, ritual, organization",
					},
					"entity_id": map[string]interface{}{
						"type":        "string",
						"description": "ID of the entity to comment on",
					},
					"content": map[string]interface{}{
						"type":        "string",
						"description": "Comment text (Markdown)",
					},
					"reply_to_id": map[string]interface{}{
						"type":        "string",
						"description": "Comment ID to reply to (optional, for threaded replies)",
					},
					"metadata": map[string]interface{}{
						"type":        "object",
						"description": "Arbitrary key-value pairs",
					},
				},
				"required": []string{"entity_type", "entity_id", "content"},
			},
		},
		s.withSessionAuth(s.handleCmtCreate),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.cmt.list",
			Description: "List comments on an entity (chronological order, oldest first)",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"entity_type": map[string]interface{}{
						"type":        "string",
						"description": "Entity type: task, demand, endeavour, artifact, ritual, organization",
					},
					"entity_id": map[string]interface{}{
						"type":        "string",
						"description": "ID of the entity",
					},
					"author_id": map[string]interface{}{
						"type":        "string",
						"description": "Filter by author resource ID",
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
				"required": []string{"entity_type", "entity_id"},
			},
		},
		s.withSessionAuth(s.handleCmtList),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.cmt.get",
			Description: "Retrieve a comment by ID, including its direct replies",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "Comment ID",
					},
				},
				"required": []string{"id"},
			},
		},
		s.withSessionAuth(s.handleCmtGet),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.cmt.update",
			Description: "Edit a comment (owner-only, sets edited_at timestamp)",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "Comment ID",
					},
					"content": map[string]interface{}{
						"type":        "string",
						"description": "New comment text",
					},
					"metadata": map[string]interface{}{
						"type":        "object",
						"description": "New metadata (replaces existing)",
					},
				},
				"required": []string{"id"},
			},
		},
		s.withSessionAuth(s.handleCmtUpdate),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.cmt.delete",
			Description: "Soft-delete a comment (owner-only, shows [deleted] placeholder, preserves thread)",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "Comment ID",
					},
				},
				"required": []string{"id"},
			},
		},
		s.withSessionAuth(s.handleCmtDelete),
	)
}

// handleCmtCreate handles the ts.cmt.create tool.
func (s *Server) handleCmtCreate(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := requireAuth(ctx); err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	args := parseArgs(req)

	entityType := getString(args, "entity_type")
	if entityType == "" {
		return toolError("invalid_input", "entity_type is required"), nil
	}
	entityID := getString(args, "entity_id")
	if entityID == "" {
		return toolError("invalid_input", "entity_id is required"), nil
	}
	content := getString(args, "content")
	if content == "" {
		return toolError("invalid_input", "content is required"), nil
	}

	var metadata map[string]interface{}
	if m, ok := args["metadata"].(map[string]interface{}); ok {
		metadata = m
	}

	result, apiErr := s.api.CreateComment(ctx, entityType, entityID,
		getString(args, "reply_to_id"), content, metadata)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}
	security.FrameMapFields(result, security.CommentFrameSpec)
	return toolSuccess(result), nil
}

// handleCmtList handles the ts.cmt.list tool.
func (s *Server) handleCmtList(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := requireAuth(ctx); err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	args := parseArgs(req)

	entityType := getString(args, "entity_type")
	if entityType == "" {
		return toolError("invalid_input", "entity_type is required"), nil
	}
	entityID := getString(args, "entity_id")
	if entityID == "" {
		return toolError("invalid_input", "entity_id is required"), nil
	}

	opts := storage.ListCommentsOpts{
		EntityType: entityType,
		EntityID:   entityID,
		AuthorID:   getString(args, "author_id"),
		Limit:      getInt(args, "limit", 50),
		Offset:     getInt(args, "offset", 0),
	}

	items, total, apiErr := s.api.ListComments(ctx, opts)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}
	security.FrameMapSlice(items, security.CommentFrameSpec)

	return toolSuccess(map[string]interface{}{
		"comments": items,
		"total":    total,
		"limit":    opts.Limit,
		"offset":   opts.Offset,
	}), nil
}

// handleCmtGet handles the ts.cmt.get tool.
func (s *Server) handleCmtGet(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := requireAuth(ctx); err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	id := getString(parseArgs(req), "id")
	if id == "" {
		return toolError("invalid_input", "Comment ID is required"), nil
	}

	result, apiErr := s.api.GetComment(ctx, id)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}
	security.FrameCommentWithReplies(result)
	return toolSuccess(result), nil
}

// handleCmtUpdate handles the ts.cmt.update tool.
func (s *Server) handleCmtUpdate(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := requireAuth(ctx); err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	args := parseArgs(req)
	id := getString(args, "id")
	if id == "" {
		return toolError("invalid_input", "Comment ID is required"), nil
	}

	var fields storage.UpdateCommentFields
	if v, ok := args["content"].(string); ok {
		fields.Content = &v
	}
	if m, ok := args["metadata"].(map[string]interface{}); ok {
		fields.Metadata = m
	}

	result, apiErr := s.api.UpdateComment(ctx, id, fields)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}
	security.FrameMapFields(result, security.CommentFrameSpec)
	return toolSuccess(result), nil
}

// handleCmtDelete handles the ts.cmt.delete tool.
func (s *Server) handleCmtDelete(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := requireAuth(ctx); err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	id := getString(parseArgs(req), "id")
	if id == "" {
		return toolError("invalid_input", "Comment ID is required"), nil
	}

	result, apiErr := s.api.DeleteComment(ctx, id)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}
	return toolSuccess(result), nil
}
