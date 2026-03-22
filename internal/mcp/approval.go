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

// registerApprovalTools registers approval MCP tools.
func (s *Server) registerApprovalTools(mcpServer *mcp.Server) {
	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.apr.create",
			Description: "Record an approval on an entity (task, demand, endeavour, artifact). Approvals are immutable.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"entity_type": map[string]interface{}{
						"type":        "string",
						"description": "Entity type: task, demand, endeavour, artifact",
					},
					"entity_id": map[string]interface{}{
						"type":        "string",
						"description": "ID of the entity being approved",
					},
					"verdict": map[string]interface{}{
						"type":        "string",
						"description": "Verdict: approved, rejected, needs_work",
					},
					"role": map[string]interface{}{
						"type":        "string",
						"description": "Role under which approval is given (e.g., reviewer, product_owner)",
					},
					"comment": map[string]interface{}{
						"type":        "string",
						"description": "Optional rationale or feedback",
					},
					"metadata": map[string]interface{}{
						"type":        "object",
						"description": "Arbitrary key-value pairs (e.g., checklist results, linked artifacts)",
					},
				},
				"required": []string{"entity_type", "entity_id", "verdict"},
			},
		},
		s.withSessionAuth(s.handleAprCreate),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.apr.list",
			Description: "List approvals for an entity (newest first)",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"entity_type": map[string]interface{}{
						"type":        "string",
						"description": "Entity type: task, demand, endeavour, artifact",
					},
					"entity_id": map[string]interface{}{
						"type":        "string",
						"description": "ID of the entity",
					},
					"approver_id": map[string]interface{}{
						"type":        "string",
						"description": "Filter by approver resource ID",
					},
					"verdict": map[string]interface{}{
						"type":        "string",
						"description": "Filter by verdict: approved, rejected, needs_work",
					},
					"role": map[string]interface{}{
						"type":        "string",
						"description": "Filter by role",
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
		s.withSessionAuth(s.handleAprList),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.apr.get",
			Description: "Retrieve an approval by ID",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "Approval ID",
					},
				},
				"required": []string{"id"},
			},
		},
		s.withSessionAuth(s.handleAprGet),
	)
}

// handleAprCreate handles the ts.apr.create tool.
func (s *Server) handleAprCreate(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
	verdict := getString(args, "verdict")
	if verdict == "" {
		return toolError("invalid_input", "verdict is required"), nil
	}

	var metadata map[string]interface{}
	if m, ok := args["metadata"].(map[string]interface{}); ok {
		metadata = m
	}

	result, apiErr := s.api.CreateApproval(ctx, entityType, entityID,
		getString(args, "role"), verdict, getString(args, "comment"), metadata)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}
	return toolSuccess(result), nil
}

// handleAprList handles the ts.apr.list tool.
func (s *Server) handleAprList(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

	opts := storage.ListApprovalsOpts{
		EntityType: entityType,
		EntityID:   entityID,
		ApproverID: getString(args, "approver_id"),
		Verdict:    getString(args, "verdict"),
		Role:       getString(args, "role"),
		Limit:      getInt(args, "limit", 50),
		Offset:     getInt(args, "offset", 0),
	}

	items, total, apiErr := s.api.ListApprovals(ctx, opts)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}

	return toolSuccess(map[string]interface{}{
		"approvals": items,
		"total":     total,
		"limit":     opts.Limit,
		"offset":    opts.Offset,
	}), nil
}

// handleAprGet handles the ts.apr.get tool.
func (s *Server) handleAprGet(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := requireAuth(ctx); err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	id := getString(parseArgs(req), "id")
	if id == "" {
		return toolError("invalid_input", "Approval ID is required"), nil
	}

	result, apiErr := s.api.GetApproval(ctx, id)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}
	return toolSuccess(result), nil
}
