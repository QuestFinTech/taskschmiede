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
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/QuestFinTech/taskschmiede/internal/storage"
)

// registerDodTools registers Definition of Done MCP tools.
func (s *Server) registerDodTools(mcpServer *mcp.Server) {
	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.dod.create",
			Description: "Create a new DoD policy",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type":        "string",
						"description": "Policy name",
					},
					"description": map[string]interface{}{
						"type":        "string",
						"description": "Policy description",
					},
					"origin": map[string]interface{}{
						"type":        "string",
						"description": "Origin: custom (default), derived",
					},
					"conditions": map[string]interface{}{
						"type":        "array",
						"description": "Array of condition objects with id, type, label, params, required",
						"items": map[string]interface{}{
							"type": "object",
						},
					},
					"strictness": map[string]interface{}{
						"type":        "string",
						"description": "Strictness: all (default), n_of",
					},
					"quorum": map[string]interface{}{
						"type":        "integer",
						"description": "Required count when strictness is n_of",
					},
					"metadata": map[string]interface{}{
						"type":        "object",
						"description": "Arbitrary key-value pairs",
					},
				},
				"required": []string{"name", "conditions"},
			},
		},
		s.withSessionAuth(s.handleDodCreate),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.dod.get",
			Description: "Retrieve a DoD policy by ID",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "DoD policy ID",
					},
				},
				"required": []string{"id"},
			},
		},
		s.withSessionAuth(s.handleDodGet),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.dod.list",
			Description: "Query DoD policies with filters",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"status": map[string]interface{}{
						"type":        "string",
						"description": "Filter by status: active, archived",
					},
					"origin": map[string]interface{}{
						"type":        "string",
						"description": "Filter by origin: template, custom, derived",
					},
					"scope": map[string]interface{}{
						"type":        "string",
						"description": "Filter by scope: task",
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
		s.withSessionAuth(s.handleDodList),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.dod.update",
			Description: "Update DoD policy attributes (name, description, status, metadata only)",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "DoD policy ID",
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
						"description": "New status: active, archived",
					},
					"metadata": map[string]interface{}{
						"type":        "object",
						"description": "Metadata to set (replaces existing)",
					},
				},
				"required": []string{"id"},
			},
		},
		s.withSessionAuth(s.handleDodUpdate),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.dod.new_version",
			Description: "Create a new version of a DoD policy with updated conditions. Supersedes endorsements on the old version.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "ID of the policy to create a new version of",
					},
					"name": map[string]interface{}{
						"type":        "string",
						"description": "Name for the new version (defaults to source name)",
					},
					"description": map[string]interface{}{
						"type":        "string",
						"description": "Description (defaults to source description)",
					},
					"conditions": map[string]interface{}{
						"type":        "array",
						"description": "Array of condition objects with id, type, label, params, required",
						"items":       map[string]interface{}{"type": "object"},
					},
					"strictness": map[string]interface{}{
						"type":        "string",
						"description": "Strictness: all (default), n_of",
					},
					"quorum": map[string]interface{}{
						"type":        "integer",
						"description": "Required count when strictness is n_of",
					},
					"metadata": map[string]interface{}{
						"type":        "object",
						"description": "Arbitrary key-value pairs",
					},
				},
				"required": []string{"id"},
			},
		},
		s.withSessionAuth(s.handleDodNewVersion),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.dod.assign",
			Description: "Assign a DoD policy to an endeavour",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"endeavour_id": map[string]interface{}{
						"type":        "string",
						"description": "Endeavour ID",
					},
					"policy_id": map[string]interface{}{
						"type":        "string",
						"description": "DoD policy ID",
					},
				},
				"required": []string{"endeavour_id", "policy_id"},
			},
		},
		s.withSessionAuth(s.handleDodAssign),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.dod.unassign",
			Description: "Remove DoD policy from an endeavour",
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
		s.withSessionAuth(s.handleDodUnassign),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.dod.endorse",
			Description: "Endorse the current DoD policy for an endeavour",
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
		s.withSessionAuth(s.handleDodEndorse),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.dod.check",
			Description: "Evaluate DoD conditions for a task (dry run, does not modify the task)",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"task_id": map[string]interface{}{
						"type":        "string",
						"description": "Task ID to check",
					},
				},
				"required": []string{"task_id"},
			},
		},
		s.withSessionAuth(s.handleDodCheck),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.dod.override",
			Description: "Override DoD for a specific task (requires reason)",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"task_id": map[string]interface{}{
						"type":        "string",
						"description": "Task ID",
					},
					"reason": map[string]interface{}{
						"type":        "string",
						"description": "Reason for override (required)",
					},
				},
				"required": []string{"task_id", "reason"},
			},
		},
		s.withSessionAuth(s.handleDodOverride),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.dod.status",
			Description: "Show DoD policy and endorsement status for an endeavour",
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
		s.withSessionAuth(s.handleDodStatus),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.dod.lineage",
			Description: "Walk the version chain for a DoD policy (oldest to newest)",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "DoD policy ID to trace lineage from",
					},
				},
				"required": []string{"id"},
			},
		},
		s.withSessionAuth(s.handleDodLineage),
	)
}

// handleDodCreate handles the ts.dod.create tool.
func (s *Server) handleDodCreate(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	resourceID, err := s.resolveCallerResourceID(ctx)
	if err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	args := parseArgs(req)
	name := getString(args, "name")
	if name == "" {
		return toolError("invalid_input", "Name is required"), nil
	}

	conditions, condErr := parseDodConditions(args)
	if condErr != nil {
		return toolError("invalid_input", condErr.Error()), nil
	}

	var metadata map[string]interface{}
	if m, ok := args["metadata"].(map[string]interface{}); ok {
		metadata = m
	}

	result, apiErr := s.api.CreateDodPolicy(ctx, name, getString(args, "description"), getString(args, "origin"), resourceID, conditions, getString(args, "strictness"), getInt(args, "quorum", 0), metadata)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}
	return toolSuccess(result), nil
}

// handleDodGet handles the ts.dod.get tool.
func (s *Server) handleDodGet(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := requireAuth(ctx); err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	id := getString(parseArgs(req), "id")
	if id == "" {
		return toolError("invalid_input", "DoD policy ID is required"), nil
	}

	result, apiErr := s.api.GetDodPolicy(ctx, id)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}
	return toolSuccess(result), nil
}

// handleDodList handles the ts.dod.list tool.
func (s *Server) handleDodList(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := requireAuth(ctx); err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	args := parseArgs(req)
	opts := storage.ListDodPoliciesOpts{
		Status: getString(args, "status"),
		Origin: getString(args, "origin"),
		Scope:  getString(args, "scope"),
		Search: getString(args, "search"),
		Limit:  getInt(args, "limit", 50),
		Offset: getInt(args, "offset", 0),
	}

	items, total, apiErr := s.api.ListDodPolicies(ctx, opts)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}

	return toolSuccess(map[string]interface{}{
		"policies": items,
		"total":    total,
		"limit":    opts.Limit,
		"offset":   opts.Offset,
	}), nil
}

// handleDodUpdate handles the ts.dod.update tool.
func (s *Server) handleDodUpdate(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := requireAuth(ctx); err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	args := parseArgs(req)
	id := getString(args, "id")
	if id == "" {
		return toolError("invalid_input", "DoD policy ID is required"), nil
	}

	var fields storage.UpdateDodPolicyFields
	if v, ok := args["name"].(string); ok {
		fields.Name = &v
	}
	if v, ok := args["description"].(string); ok {
		fields.Description = &v
	}
	if v, ok := args["status"].(string); ok {
		fields.Status = &v
	}
	if m, ok := args["metadata"].(map[string]interface{}); ok {
		fields.Metadata = m
	}

	result, apiErr := s.api.UpdateDodPolicy(ctx, id, fields)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}
	return toolSuccess(result), nil
}

// handleDodNewVersion handles the ts.dod.new_version tool.
func (s *Server) handleDodNewVersion(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	resourceID, err := s.resolveCallerResourceID(ctx)
	if err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	args := parseArgs(req)
	id := getString(args, "id")
	if id == "" {
		return toolError("invalid_input", "DoD policy ID is required"), nil
	}

	conditions, condErr := parseDodConditions(args)
	if condErr != nil {
		return toolError("invalid_input", condErr.Error()), nil
	}

	var metadata map[string]interface{}
	if m, ok := args["metadata"].(map[string]interface{}); ok {
		metadata = m
	}

	result, apiErr := s.api.NewDodPolicyVersion(ctx, id, getString(args, "name"), getString(args, "description"), conditions, getString(args, "strictness"), getInt(args, "quorum", 0), metadata, resourceID)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}
	return toolSuccess(result), nil
}

// handleDodAssign handles the ts.dod.assign tool.
func (s *Server) handleDodAssign(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	resourceID, err := s.resolveCallerResourceID(ctx)
	if err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	args := parseArgs(req)
	endeavourID := getString(args, "endeavour_id")
	policyID := getString(args, "policy_id")

	if endeavourID == "" || policyID == "" {
		return toolError("invalid_input", "endeavour_id and policy_id are required"), nil
	}

	result, apiErr := s.api.AssignDodPolicy(ctx, endeavourID, policyID, resourceID)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}
	return toolSuccess(result), nil
}

// handleDodUnassign handles the ts.dod.unassign tool.
func (s *Server) handleDodUnassign(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	resourceID, err := s.resolveCallerResourceID(ctx)
	if err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	endeavourID := getString(parseArgs(req), "endeavour_id")
	if endeavourID == "" {
		return toolError("invalid_input", "endeavour_id is required"), nil
	}

	result, apiErr := s.api.UnassignDodPolicy(ctx, endeavourID, resourceID)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}
	return toolSuccess(result), nil
}

// handleDodEndorse handles the ts.dod.endorse tool.
func (s *Server) handleDodEndorse(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	resourceID, err := s.resolveCallerResourceID(ctx)
	if err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	endeavourID := getString(parseArgs(req), "endeavour_id")
	if endeavourID == "" {
		return toolError("invalid_input", "endeavour_id is required"), nil
	}

	result, apiErr := s.api.EndorseDodPolicy(ctx, resourceID, endeavourID)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}
	return toolSuccess(result), nil
}

// handleDodCheck handles the ts.dod.check tool.
func (s *Server) handleDodCheck(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	resourceID, _ := s.resolveCallerResourceID(ctx)

	taskID := getString(parseArgs(req), "task_id")
	if taskID == "" {
		return toolError("invalid_input", "task_id is required"), nil
	}

	result, apiErr := s.api.CheckDod(ctx, taskID, resourceID)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}
	return toolSuccess(result), nil
}

// handleDodOverride handles the ts.dod.override tool.
func (s *Server) handleDodOverride(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	resourceID, err := s.resolveCallerResourceID(ctx)
	if err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	args := parseArgs(req)
	taskID := getString(args, "task_id")
	reason := getString(args, "reason")

	if taskID == "" {
		return toolError("invalid_input", "task_id is required"), nil
	}
	if reason == "" {
		return toolError("invalid_input", "reason is required for DoD override"), nil
	}

	result, apiErr := s.api.OverrideDod(ctx, taskID, resourceID, reason, "mcp")
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}
	return toolSuccess(result), nil
}

// handleDodStatus handles the ts.dod.status tool.
func (s *Server) handleDodStatus(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := requireAuth(ctx); err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	endeavourID := getString(parseArgs(req), "endeavour_id")
	if endeavourID == "" {
		return toolError("invalid_input", "endeavour_id is required"), nil
	}

	result, apiErr := s.api.GetDodStatus(ctx, endeavourID)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}
	return toolSuccess(result), nil
}

// handleDodLineage handles the ts.dod.lineage tool.
func (s *Server) handleDodLineage(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := requireAuth(ctx); err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	id := getString(parseArgs(req), "id")
	if id == "" {
		return toolError("invalid_input", "DoD policy ID is required"), nil
	}

	items, apiErr := s.api.GetDodPolicyLineage(ctx, id)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}

	return toolSuccess(map[string]interface{}{
		"lineage": items,
		"count":   len(items),
	}), nil
}

// --- helpers ---

// resolveCallerResourceID resolves the calling user's resource ID from auth context.
func (s *Server) resolveCallerResourceID(ctx context.Context) (string, error) {
	user, err := requireAuth(ctx)
	if err != nil {
		return "", err
	}

	u, apiErr := s.api.GetUser(ctx, user.UserID)
	if apiErr != nil {
		return "", fmt.Errorf("%s", apiErr.Message)
	}

	if resID, ok := u["resource_id"].(string); ok && resID != "" {
		return resID, nil
	}
	return "", nil
}

// parseDodConditions parses the conditions array from MCP tool arguments.
func parseDodConditions(args map[string]interface{}) ([]storage.DodCondition, error) {
	condRaw, ok := args["conditions"]
	if !ok {
		return nil, nil
	}

	// The MCP SDK deserializes arguments as json.RawMessage -> map[string]interface{},
	// so the conditions array arrives as []interface{}.
	condArray, ok := condRaw.([]interface{})
	if !ok {
		return nil, nil
	}

	// Re-marshal and unmarshal to get proper typed struct
	b, err := json.Marshal(condArray)
	if err != nil {
		return nil, err
	}

	var conditions []storage.DodCondition
	if err := json.Unmarshal(b, &conditions); err != nil {
		return nil, err
	}

	return conditions, nil
}
