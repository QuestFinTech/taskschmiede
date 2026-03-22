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

// registerOrganizationTools registers organization MCP tools.
func (s *Server) registerOrganizationTools(mcpServer *mcp.Server) {
	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.org.create",
			Description: "Create a new organization",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type":        "string",
						"description": "Organization name",
					},
					"description": map[string]interface{}{
						"type":        "string",
						"description": "Organization description",
					},
					"metadata": map[string]interface{}{
						"type":        "object",
						"description": "Arbitrary key-value pairs",
					},
				},
				"required": []string{"name"},
			},
		},
		s.withSessionAuth(s.handleOrgCreate),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.org.get",
			Description: "Retrieve an organization by ID",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "Organization ID",
					},
				},
				"required": []string{"id"},
			},
		},
		s.withSessionAuth(s.handleOrgGet),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.org.list",
			Description: "Query organizations with filters",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"status": map[string]interface{}{
						"type":        "string",
						"description": "Filter by status: active, inactive, archived",
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
		s.withSessionAuth(s.handleOrgList),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.org.update",
			Description: "Update organization attributes (partial update)",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "Organization ID",
					},
					"name": map[string]interface{}{
						"type":        "string",
						"description": "New name",
					},
					"description": map[string]interface{}{
						"type":        "string",
						"description": "New description",
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
		s.withSessionAuth(s.handleOrgUpdate),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.org.add_resource",
			Description: "Add a resource to an organization",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"organization_id": map[string]interface{}{
						"type":        "string",
						"description": "Organization ID",
					},
					"resource_id": map[string]interface{}{
						"type":        "string",
						"description": "Resource ID to add",
					},
					"role": map[string]interface{}{
						"type":        "string",
						"description": "Role: owner, admin, member, guest (default: member)",
					},
				},
				"required": []string{"organization_id", "resource_id"},
			},
		},
		s.withSessionAuth(s.handleOrgAddResource),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.org.add_endeavour",
			Description: "Associate an endeavour with an organization",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"organization_id": map[string]interface{}{
						"type":        "string",
						"description": "Organization ID",
					},
					"endeavour_id": map[string]interface{}{
						"type":        "string",
						"description": "Endeavour ID to associate",
					},
					"role": map[string]interface{}{
						"type":        "string",
						"description": "Role: owner, participant (default: participant)",
					},
				},
				"required": []string{"organization_id", "endeavour_id"},
			},
		},
		s.withSessionAuth(s.handleOrgAddEndeavour),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.org.archive",
			Description: "Archive an organization (cascades to endeavours and their tasks). Use confirm=false (default) for dry-run impact, confirm=true to execute.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "Organization ID",
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
		s.withSessionAuth(s.handleOrgArchive),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.org.export",
			Description: "Export all organization data (members, endeavours, tasks, demands, artifacts, rituals, comments, relations) as JSON. Requires owner role.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"organization_id": map[string]interface{}{
						"type":        "string",
						"description": "Organization ID to export",
					},
				},
				"required": []string{"organization_id"},
			},
		},
		s.withSessionAuth(s.handleOrgExport),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.org.add_member",
			Description: "Add a user to an organization (resolves user to resource internally)",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"org_id": map[string]interface{}{
						"type":        "string",
						"description": "Organization ID",
					},
					"user_id": map[string]interface{}{
						"type":        "string",
						"description": "User ID to add",
					},
					"role": map[string]interface{}{
						"type":        "string",
						"description": "Role: owner, admin, member, guest (default: member)",
					},
				},
				"required": []string{"org_id", "user_id"},
			},
		},
		s.withSessionAuth(s.handleOrgAddMember),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.org.remove_member",
			Description: "Remove a user from an organization",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"org_id": map[string]interface{}{
						"type":        "string",
						"description": "Organization ID",
					},
					"user_id": map[string]interface{}{
						"type":        "string",
						"description": "User ID to remove",
					},
				},
				"required": []string{"org_id", "user_id"},
			},
		},
		s.withSessionAuth(s.handleOrgRemoveMember),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.org.list_members",
			Description: "List members of an organization with their roles",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"org_id": map[string]interface{}{
						"type":        "string",
						"description": "Organization ID",
					},
				},
				"required": []string{"org_id"},
			},
		},
		s.withSessionAuth(s.handleOrgListMembers),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.org.set_member_role",
			Description: "Change a member's role in an organization",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"org_id": map[string]interface{}{
						"type":        "string",
						"description": "Organization ID",
					},
					"user_id": map[string]interface{}{
						"type":        "string",
						"description": "User ID",
					},
					"role": map[string]interface{}{
						"type":        "string",
						"description": "New role: owner, admin, member, guest",
					},
				},
				"required": []string{"org_id", "user_id", "role"},
			},
		},
		s.withSessionAuth(s.handleOrgSetMemberRole),
	)
}

// handleOrgCreate handles the ts.org.create tool.
func (s *Server) handleOrgCreate(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := requireAuth(ctx); err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	args := parseArgs(req)
	name := getString(args, "name")
	if name == "" {
		return toolError("invalid_input", "Name is required"), nil
	}

	var metadata map[string]interface{}
	if m, ok := args["metadata"].(map[string]interface{}); ok {
		metadata = m
	}

	result, apiErr := s.api.CreateOrganization(ctx, name, getString(args, "description"), metadata)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}
	return toolSuccess(result), nil
}

// handleOrgGet handles the ts.org.get tool.
func (s *Server) handleOrgGet(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := requireAuth(ctx); err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	id := getString(parseArgs(req), "id")
	if id == "" {
		return toolError("invalid_input", "Organization ID is required"), nil
	}

	result, apiErr := s.api.GetOrganization(ctx, id)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}
	return toolSuccess(result), nil
}

// handleOrgList handles the ts.org.list tool.
func (s *Server) handleOrgList(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := requireAuth(ctx); err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	args := parseArgs(req)
	opts := storage.ListOrganizationsOpts{
		Status: getString(args, "status"),
		Search: getString(args, "search"),
		Limit:  getInt(args, "limit", 50),
		Offset: getInt(args, "offset", 0),
	}

	items, total, apiErr := s.api.ListOrganizations(ctx, opts, false)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}

	return toolSuccess(map[string]interface{}{
		"organizations": items,
		"total":         total,
		"limit":         opts.Limit,
		"offset":        opts.Offset,
	}), nil
}

// handleOrgUpdate handles the ts.org.update tool.
func (s *Server) handleOrgUpdate(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := requireAuth(ctx); err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	args := parseArgs(req)
	id := getString(args, "id")
	if id == "" {
		return toolError("invalid_input", "Organization ID is required"), nil
	}

	var fields storage.UpdateOrganizationFields
	if v, ok := args["name"].(string); ok {
		fields.Name = &v
	}
	if v, ok := args["description"].(string); ok {
		fields.Description = &v
	}
	if m, ok := args["metadata"].(map[string]interface{}); ok {
		fields.Metadata = m
	}
	if v, ok := args["status"].(string); ok {
		fields.Status = &v
	}

	result, apiErr := s.api.UpdateOrganization(ctx, id, fields)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}
	return toolSuccess(result), nil
}

// handleOrgAddResource handles the ts.org.add_resource tool.
func (s *Server) handleOrgAddResource(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := requireAuth(ctx); err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	args := parseArgs(req)
	orgID := getString(args, "organization_id")
	resourceID := getString(args, "resource_id")
	role := getString(args, "role")

	if orgID == "" || resourceID == "" {
		return toolError("invalid_input", "organization_id and resource_id are required"), nil
	}

	if role == "" {
		role = "member"
	}

	result, apiErr := s.api.AddResourceToOrg(ctx, orgID, resourceID, role)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}
	return toolSuccess(result), nil
}

// handleOrgAddEndeavour handles the ts.org.add_endeavour tool.
func (s *Server) handleOrgAddEndeavour(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := requireAuth(ctx); err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	args := parseArgs(req)
	orgID := getString(args, "organization_id")
	endeavourID := getString(args, "endeavour_id")
	role := getString(args, "role")

	if orgID == "" || endeavourID == "" {
		return toolError("invalid_input", "organization_id and endeavour_id are required"), nil
	}

	if role == "" {
		role = "participant"
	}

	result, apiErr := s.api.AddEndeavourToOrg(ctx, orgID, endeavourID, role)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}
	return toolSuccess(result), nil
}

// handleOrgArchive handles the ts.org.archive tool.
func (s *Server) handleOrgArchive(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := requireAuth(ctx); err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	args := parseArgs(req)
	id := getString(args, "id")
	reason := getString(args, "reason")
	confirm := getBool(args, "confirm")

	if id == "" {
		return toolError("invalid_input", "Organization ID is required"), nil
	}
	if reason == "" {
		return toolError("invalid_input", "Reason is required"), nil
	}

	if !confirm {
		result, apiErr := s.api.OrgArchiveImpact(ctx, id)
		if apiErr != nil {
			return toolAPIError(apiErr), nil
		}
		result["confirm_hint"] = "Set confirm=true to execute this archive operation"
		return toolSuccess(result), nil
	}

	result, apiErr := s.api.ArchiveOrganization(ctx, id, reason)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}
	return toolSuccess(result), nil
}

// handleOrgAddMember handles the ts.org.add_member tool.
func (s *Server) handleOrgAddMember(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := requireAuth(ctx); err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	args := parseArgs(req)
	orgID := getString(args, "org_id")
	userID := getString(args, "user_id")
	role := getString(args, "role")

	if orgID == "" || userID == "" {
		return toolError("invalid_input", "org_id and user_id are required"), nil
	}

	if role == "" {
		role = "member"
	}

	result, apiErr := s.api.AddOrgMember(ctx, orgID, userID, role)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}
	return toolSuccess(result), nil
}

// handleOrgRemoveMember handles the ts.org.remove_member tool.
func (s *Server) handleOrgRemoveMember(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := requireAuth(ctx); err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	args := parseArgs(req)
	orgID := getString(args, "org_id")
	userID := getString(args, "user_id")

	if orgID == "" || userID == "" {
		return toolError("invalid_input", "org_id and user_id are required"), nil
	}

	result, apiErr := s.api.RemoveOrgMember(ctx, orgID, userID)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}
	return toolSuccess(result), nil
}

// handleOrgListMembers handles the ts.org.list_members tool.
func (s *Server) handleOrgListMembers(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := requireAuth(ctx); err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	args := parseArgs(req)
	orgID := getString(args, "org_id")

	if orgID == "" {
		return toolError("invalid_input", "org_id is required"), nil
	}

	members, apiErr := s.api.ListOrgMembers(ctx, orgID)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}

	return toolSuccess(map[string]interface{}{
		"members": members,
		"total":   len(members),
	}), nil
}

// handleOrgSetMemberRole handles the ts.org.set_member_role tool.
func (s *Server) handleOrgSetMemberRole(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := requireAuth(ctx); err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	args := parseArgs(req)
	orgID := getString(args, "org_id")
	userID := getString(args, "user_id")
	role := getString(args, "role")

	if orgID == "" || userID == "" || role == "" {
		return toolError("invalid_input", "org_id, user_id, and role are required"), nil
	}

	result, apiErr := s.api.SetOrgMemberRole(ctx, orgID, userID, role)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}
	return toolSuccess(result), nil
}

// handleOrgExport handles the ts.org.export tool.
func (s *Server) handleOrgExport(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := requireAuth(ctx); err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	args := parseArgs(req)
	id := getString(args, "organization_id")
	if id == "" {
		return toolError("invalid_input", "organization_id is required"), nil
	}

	export, apiErr := s.api.ExportOrganization(ctx, id)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}

	return toolSuccess(export), nil
}
