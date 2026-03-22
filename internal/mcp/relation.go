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

// registerRelationTools registers entity relation MCP tools.
func (s *Server) registerRelationTools(mcpServer *mcp.Server) {
	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.rel.create",
			Description: "Create a relationship between two entities",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"relationship_type": map[string]interface{}{
						"type":        "string",
						"description": "Relationship type (e.g., belongs_to, assigned_to, has_member, governs, uses)",
					},
					"source_entity_type": map[string]interface{}{
						"type":        "string",
						"description": "Source entity type (e.g., task, organization, user)",
					},
					"source_entity_id": map[string]interface{}{
						"type":        "string",
						"description": "Source entity ID",
					},
					"target_entity_type": map[string]interface{}{
						"type":        "string",
						"description": "Target entity type (e.g., endeavour, resource, ritual)",
					},
					"target_entity_id": map[string]interface{}{
						"type":        "string",
						"description": "Target entity ID",
					},
					"metadata": map[string]interface{}{
						"type":        "object",
						"description": "Optional metadata on the relationship (e.g., role)",
					},
				},
				"required": []string{"relationship_type", "source_entity_type", "source_entity_id", "target_entity_type", "target_entity_id"},
			},
		},
		s.withSessionAuth(s.handleRelCreate),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.rel.list",
			Description: "Query relationships (by source, target, or type)",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"source_entity_type": map[string]interface{}{
						"type":        "string",
						"description": "Filter by source entity type",
					},
					"source_entity_id": map[string]interface{}{
						"type":        "string",
						"description": "Filter by source entity ID",
					},
					"target_entity_type": map[string]interface{}{
						"type":        "string",
						"description": "Filter by target entity type",
					},
					"target_entity_id": map[string]interface{}{
						"type":        "string",
						"description": "Filter by target entity ID",
					},
					"relationship_type": map[string]interface{}{
						"type":        "string",
						"description": "Filter by relationship type",
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
		s.withSessionAuth(s.handleRelList),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.rel.delete",
			Description: "Remove a relationship",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "The relation ID to delete",
					},
				},
				"required": []string{"id"},
			},
		},
		s.withSessionAuth(s.handleRelDelete),
	)
}

// handleRelCreate handles the ts.rel.create tool.
func (s *Server) handleRelCreate(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	authUser, err := requireAuth(ctx)
	if err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	args := parseArgs(req)
	relType := getString(args, "relationship_type")
	srcType := getString(args, "source_entity_type")
	srcID := getString(args, "source_entity_id")
	tgtType := getString(args, "target_entity_type")
	tgtID := getString(args, "target_entity_id")

	if relType == "" || srcType == "" || srcID == "" || tgtType == "" || tgtID == "" {
		return toolError("invalid_input", "relationship_type, source_entity_type, source_entity_id, target_entity_type, and target_entity_id are required"), nil
	}

	var metadata map[string]interface{}
	if m, ok := args["metadata"].(map[string]interface{}); ok {
		metadata = m
	}

	result, apiErr := s.api.CreateRelation(ctx, relType, srcType, srcID, tgtType, tgtID, metadata, authUser.UserID)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}

	return toolSuccess(result), nil
}

// handleRelList handles the ts.rel.list tool.
func (s *Server) handleRelList(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := requireAuth(ctx); err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	args := parseArgs(req)

	opts := storage.ListRelationsOpts{
		SourceEntityType: getString(args, "source_entity_type"),
		SourceEntityID:   getString(args, "source_entity_id"),
		TargetEntityType: getString(args, "target_entity_type"),
		TargetEntityID:   getString(args, "target_entity_id"),
		RelationshipType: getString(args, "relationship_type"),
		Limit:            getInt(args, "limit", 50),
		Offset:           getInt(args, "offset", 0),
	}

	items, total, apiErr := s.api.ListRelations(ctx, opts)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}

	return toolSuccess(map[string]interface{}{
		"relations": items,
		"total":     total,
		"limit":     opts.Limit,
		"offset":    opts.Offset,
	}), nil
}

// handleRelDelete handles the ts.rel.delete tool.
func (s *Server) handleRelDelete(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := requireAuth(ctx); err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	args := parseArgs(req)
	id := getString(args, "id")

	if id == "" {
		return toolError("invalid_input", "Relation ID is required"), nil
	}

	result, apiErr := s.api.DeleteRelation(ctx, id)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}

	return toolSuccess(result), nil
}
