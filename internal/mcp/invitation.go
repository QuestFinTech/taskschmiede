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
	"errors"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/QuestFinTech/taskschmiede/internal/storage"
)

// registerInvitationTools registers invitation-related MCP tools.
func (s *Server) registerInvitationTools(mcpServer *mcp.Server) {
	// ts.inv.create - Create invitation token
	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.inv.create",
			Description: "Create an invitation token for agent self-registration (requires admin)",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type":        "string",
						"description": "Display name for the token (for admin reference)",
					},
					"expires_at": map[string]interface{}{
						"type":        "string",
						"description": "ISO 8601 expiration datetime",
					},
					"max_uses": map[string]interface{}{
						"type":        "integer",
						"description": "Maximum number of registrations allowed (null = unlimited)",
					},
					"scope": map[string]interface{}{
						"type":        "string",
						"description": "Token scope: 'system' (default) or 'organization'",
					},
					"organization_id": map[string]interface{}{
						"type":        "string",
						"description": "Organization ID (required when scope is 'organization')",
					},
					"role": map[string]interface{}{
						"type":        "string",
						"description": "Role granted on use (for org tokens: member, admin, etc.)",
					},
				},
			},
		},
		s.withSessionAuth(s.handleInvitationCreate),
	)

	// ts.inv.list - List invitation tokens
	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.inv.list",
			Description: "List invitation tokens (requires admin)",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"status": map[string]interface{}{
						"type":        "string",
						"description": "Filter by status: active, expired, exhausted, revoked",
					},
					"organization_id": map[string]interface{}{
						"type":        "string",
						"description": "Filter by organization ID (shows only org-scoped tokens for this org)",
					},
				},
			},
		},
		s.withSessionAuth(s.handleInvitationList),
	)

	// ts.inv.revoke - Revoke invitation token
	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.inv.revoke",
			Description: "Revoke an invitation token (requires admin)",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"token": map[string]interface{}{
						"type":        "string",
						"description": "The invitation token to revoke",
					},
				},
				"required": []string{"token"},
			},
		},
		s.withSessionAuth(s.handleInvitationRevoke),
	)
}

// handleInvitationCreate creates a new invitation token.
func (s *Server) handleInvitationCreate(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Require admin privileges (master admin or org admin/owner).
	if errResult, _ := s.requireAdmin(ctx); errResult != nil {
		return errResult, nil
	}

	args := parseArgs(req)
	name := getString(args, "name")
	expiresAtStr := getString(args, "expires_at")
	maxUsesVal := args["max_uses"]
	scope := getString(args, "scope")
	organizationID := getString(args, "organization_id")
	role := getString(args, "role")

	// Default scope
	if scope == "" {
		scope = "system"
	}

	// Validate scope
	if scope != "system" && scope != "organization" {
		return toolError("invalid_scope", "Scope must be 'system' or 'organization'"), nil
	}

	// Block org-scoped invitation tokens in open deployment mode.
	if scope == "organization" && s.deploymentMode == "open" {
		return toolError("forbidden", "Organization-scoped invitation tokens are not available in open deployment mode. Use system-scoped tokens or admin creation instead."), nil
	}

	// Org-scoped tokens require organization_id
	if scope == "organization" && organizationID == "" {
		return toolError("missing_organization_id", "organization_id is required for organization-scoped tokens"), nil
	}

	// Parse expiry
	var expiresAt *time.Time
	if expiresAtStr != "" {
		t, err := time.Parse(time.RFC3339, expiresAtStr)
		if err != nil {
			return toolError("invalid_expires_at", "Invalid ISO 8601 datetime format"), nil
		}
		expiresAt = &t
	}

	// Parse max_uses
	var maxUses *int
	if maxUsesVal != nil {
		if v, ok := maxUsesVal.(float64); ok {
			m := int(v)
			maxUses = &m
		}
	} else {
		// Default to 1
		m := 1
		maxUses = &m
	}

	// Resolve the creating user ID from auth context.
	createdBy := ""
	if authUser := getAuthUser(ctx); authUser != nil {
		createdBy = authUser.UserID

		// Check agent slot quota: current agents + pending token slots must not
		// exceed the tier's max_agents_per_org limit.
		user, err := s.db.GetUser(authUser.UserID)
		if err == nil && user != nil {
			tierDef, err := s.db.GetTierDefinition(user.Tier)
			if err == nil && tierDef != nil && !user.IsAdmin && tierDef.MaxAgentsPerOrg >= 0 {
				if user.ResourceID != nil && *user.ResourceID != "" {
					orgID := s.findResourceOrgID(*user.ResourceID)
					if orgID != "" {
						currentAgents := s.countOrgAgents(orgID)
						pendingSlots := s.countPendingAgentSlots(authUser.UserID)
						remaining := tierDef.MaxAgentsPerOrg - currentAgents - pendingSlots
						if remaining <= 0 {
							return toolError("tier_limit", "No agent slots remaining. Revoke unused tokens or upgrade your tier."), nil
						}
						// Cap max_uses to remaining slots.
						if maxUses != nil && *maxUses > remaining {
							*maxUses = remaining
						}
					}
				}
			}
		}
	}

	// Create the invitation token
	inv, err := s.db.CreateInvitationToken(name, scope, organizationID, role, maxUses, expiresAt, createdBy)
	if err != nil {
		s.logger.Error("Failed to create invitation token", "error", err)
		return toolError("internal_error", "Failed to create invitation token"), nil
	}

	// Build response
	response := map[string]interface{}{
		"token":      inv.Token,
		"name":       inv.Name,
		"scope":      inv.Scope,
		"max_uses":   inv.MaxUses,
		"uses":       inv.Uses,
		"created_at": inv.CreatedAt.Format(time.RFC3339),
	}
	if inv.OrganizationID != "" {
		response["organization_id"] = inv.OrganizationID
	}
	if inv.Role != "" {
		response["role"] = inv.Role
	}
	if inv.ExpiresAt != nil {
		response["expires_at"] = inv.ExpiresAt.Format(time.RFC3339)
	}

	s.logger.Info("Invitation token created", "name", name, "scope", scope)
	return toolSuccess(response), nil
}

// handleInvitationList lists invitation tokens.
func (s *Server) handleInvitationList(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Require admin privileges (master admin or org admin/owner).
	if errResult, _ := s.requireAdmin(ctx); errResult != nil {
		return errResult, nil
	}

	args := parseArgs(req)
	status := getString(args, "status")
	organizationID := getString(args, "organization_id")

	tokens, err := s.db.ListInvitationTokens(status, organizationID)
	if err != nil {
		s.logger.Error("Failed to list invitation tokens", "error", err)
		return toolError("internal_error", "Failed to list invitation tokens"), nil
	}

	// Build response
	tokenList := make([]map[string]interface{}, 0, len(tokens))
	for _, t := range tokens {
		item := map[string]interface{}{
			"id":         t.ID,
			"name":       t.Name,
			"scope":      t.Scope,
			"max_uses":   t.MaxUses,
			"uses":       t.Uses,
			"created_at": t.CreatedAt.Format(time.RFC3339),
		}
		if t.OrganizationID != "" {
			item["organization_id"] = t.OrganizationID
		}
		if t.Role != "" {
			item["role"] = t.Role
		}
		if t.ExpiresAt != nil {
			item["expires_at"] = t.ExpiresAt.Format(time.RFC3339)
		}
		if t.RevokedAt != nil {
			item["revoked_at"] = t.RevokedAt.Format(time.RFC3339)
			item["status"] = "revoked"
		} else if t.ExpiresAt != nil && storage.UTCNow().After(*t.ExpiresAt) {
			item["status"] = "expired"
		} else if t.MaxUses != nil && t.Uses >= *t.MaxUses {
			item["status"] = "exhausted"
		} else {
			item["status"] = "active"
		}
		tokenList = append(tokenList, item)
	}

	return toolSuccess(map[string]interface{}{
		"tokens": tokenList,
	}), nil
}

// handleInvitationRevoke revokes an invitation token.
func (s *Server) handleInvitationRevoke(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Require admin privileges (master admin or org admin/owner).
	if errResult, _ := s.requireAdmin(ctx); errResult != nil {
		return errResult, nil
	}

	args := parseArgs(req)
	token := getString(args, "token")

	if token == "" {
		return toolError("missing_token", "Token is required"), nil
	}

	err := s.db.RevokeInvitationToken(token)
	if errors.Is(err, storage.ErrInvitationNotFound) {
		return toolError("not_found", "Token does not exist or already revoked"), nil
	}
	if err != nil {
		s.logger.Error("Failed to revoke invitation token", "error", err)
		return toolError("internal_error", "Failed to revoke invitation token"), nil
	}

	s.logger.Info("Invitation token revoked")
	return toolSuccess(map[string]interface{}{
		"revoked": true,
	}), nil
}

// findResourceOrgID finds the org that a resource belongs to as owner.
func (s *Server) findResourceOrgID(resourceID string) string {
	var orgID string
	err := s.db.QueryRow(
		`SELECT source_entity_id FROM entity_relation
		 WHERE relationship_type = 'has_member'
		   AND source_entity_type = 'organization'
		   AND target_entity_type = 'resource'
		   AND target_entity_id = ?
		   AND json_extract(metadata, '$.role') = 'owner'
		 LIMIT 1`,
		resourceID,
	).Scan(&orgID)
	if err != nil {
		return ""
	}
	return orgID
}

// countOrgAgents counts active agent-type resources in an org.
func (s *Server) countOrgAgents(orgID string) int {
	var count int
	_ = s.db.QueryRow(
		`SELECT COUNT(*) FROM entity_relation er
		 JOIN resource r ON r.id = er.target_entity_id
		 WHERE er.relationship_type = 'has_member'
		   AND er.source_entity_type = 'organization'
		   AND er.source_entity_id = ?
		   AND er.target_entity_type = 'resource'
		   AND r.type = 'agent'
		   AND r.status = 'active'`,
		orgID,
	).Scan(&count)
	return count
}

// countPendingAgentSlots counts remaining invitation token uses for a user.
func (s *Server) countPendingAgentSlots(userID string) int {
	var count int
	_ = s.db.QueryRow(
		`SELECT COALESCE(SUM(max_uses - uses), 0) FROM invitation_token
		 WHERE created_by = ?
		   AND revoked_at IS NULL
		   AND (expires_at IS NULL OR expires_at > datetime('now'))
		   AND (max_uses IS NULL OR uses < max_uses)`,
		userID,
	).Scan(&count)
	return count
}
