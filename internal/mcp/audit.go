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

// registerAuditTools registers audit-related MCP tools.
func (s *Server) registerAuditTools(mcpServer *mcp.Server) {
	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.audit.list",
			Description: "Query audit logs with filters",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"action": map[string]interface{}{
						"type":        "string",
						"description": "Filter by action (e.g., login_success, login_failure, security_alert)",
					},
					"actor_id": map[string]interface{}{
						"type":        "string",
						"description": "Filter by actor ID",
					},
					"resource": map[string]interface{}{
						"type":        "string",
						"description": "Filter by resource",
					},
					"ip": map[string]interface{}{
						"type":        "string",
						"description": "Filter by IP address",
					},
					"source": map[string]interface{}{
						"type":        "string",
						"description": "Filter by source (console, portal, mcp, api, system)",
					},
					"start_time": map[string]interface{}{
						"type":        "string",
						"description": "Filter: entries after this time (ISO 8601)",
					},
					"end_time": map[string]interface{}{
						"type":        "string",
						"description": "Filter: entries before this time (ISO 8601)",
					},
					"limit": map[string]interface{}{
						"type":        "integer",
						"description": "Max results (default: 50)",
					},
					"offset": map[string]interface{}{
						"type":        "integer",
						"description": "Pagination offset",
					},
					"before_id": map[string]interface{}{
						"type":        "string",
						"description": "Cursor: return entries older than this audit entry ID (stable pagination)",
					},
				},
			},
		},
		s.withSessionAuth(s.handleAuditList),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.audit.my_activity",
			Description: "List your own audit activity history. Returns action, resource, timestamp, and a human-readable summary. No admin privileges required.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"action": map[string]interface{}{
						"type":        "string",
						"description": "Filter by action (e.g., login_success, request)",
					},
					"start_time": map[string]interface{}{
						"type":        "string",
						"description": "Filter: entries after this time (ISO 8601)",
					},
					"end_time": map[string]interface{}{
						"type":        "string",
						"description": "Filter: entries before this time (ISO 8601)",
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
		s.withSessionAuth(s.handleAuditMyActivity),
	)
}

// registerEntityChangeTools registers entity change history MCP tools.
func (s *Server) registerEntityChangeTools(mcpServer *mcp.Server) {
	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.audit.entity_changes",
			Description: "Query entity change history (task/demand/endeavour CRUD operations). Scoped: master admins see all, endeavour admins/owners see their endeavours only.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"action": map[string]interface{}{
						"type":        "string",
						"description": "Filter by action (create, update, cancel, archive, delete)",
					},
					"entity_type": map[string]interface{}{
						"type":        "string",
						"description": "Filter by entity type (task, demand, endeavour, organization, resource)",
					},
					"entity_id": map[string]interface{}{
						"type":        "string",
						"description": "Filter by entity ID",
					},
					"actor_id": map[string]interface{}{
						"type":        "string",
						"description": "Filter by actor ID (who made the change)",
					},
					"endeavour_id": map[string]interface{}{
						"type":        "string",
						"description": "Filter by endeavour ID",
					},
					"start_time": map[string]interface{}{
						"type":        "string",
						"description": "Filter: entries after this time (ISO 8601)",
					},
					"end_time": map[string]interface{}{
						"type":        "string",
						"description": "Filter: entries before this time (ISO 8601)",
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
		s.withSessionAuth(s.handleEntityChanges),
	)
}

// handleEntityChanges handles the ts.audit.entity_changes tool.
func (s *Server) handleEntityChanges(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := parseArgs(req)

	opts := storage.ListEntityChangesOpts{
		Action:      getString(args, "action"),
		EntityType:  getString(args, "entity_type"),
		EntityID:    getString(args, "entity_id"),
		ActorID:     getString(args, "actor_id"),
		EndeavourID: getString(args, "endeavour_id"),
		Limit:       getInt(args, "limit", 50),
		Offset:      getInt(args, "offset", 0),
	}

	if st := getString(args, "start_time"); st != "" {
		if t, err := time.Parse(time.RFC3339, st); err == nil {
			opts.StartTime = &t
		}
	}
	if et := getString(args, "end_time"); et != "" {
		if t, err := time.Parse(time.RFC3339, et); err == nil {
			opts.EndTime = &t
		}
	}

	entries, total, apiErr := s.api.ListEntityChanges(ctx, opts)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}

	return toolSuccess(map[string]interface{}{
		"entries": entries,
		"total":   total,
		"limit":   opts.Limit,
		"offset":  opts.Offset,
	}), nil
}

// handleAuditList handles the ts.audit.list tool.
func (s *Server) handleAuditList(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := parseArgs(req)

	opts := storage.ListAuditLogOpts{
		Action:   getString(args, "action"),
		ActorID:  getString(args, "actor_id"),
		Resource: getString(args, "resource"),
		IP:       getString(args, "ip"),
		Source:   getString(args, "source"),
		Limit:    getInt(args, "limit", 50),
		Offset:   getInt(args, "offset", 0),
		BeforeID: getString(args, "before_id"),
	}

	if st := getString(args, "start_time"); st != "" {
		if t, err := time.Parse(time.RFC3339, st); err == nil {
			opts.StartTime = &t
		}
	}
	if et := getString(args, "end_time"); et != "" {
		if t, err := time.Parse(time.RFC3339, et); err == nil {
			opts.EndTime = &t
		}
	}

	entries, total, apiErr := s.api.ListAuditLog(ctx, opts)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}

	return toolSuccess(map[string]interface{}{
		"entries": entries,
		"total":   total,
		"limit":   opts.Limit,
		"offset":  opts.Offset,
	}), nil
}

// handleAuditMyActivity handles the ts.audit.my_activity tool.
func (s *Server) handleAuditMyActivity(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := parseArgs(req)

	opts := storage.ListAuditLogOpts{
		Action: getString(args, "action"),
		Limit:  getInt(args, "limit", 50),
		Offset: getInt(args, "offset", 0),
	}

	if st := getString(args, "start_time"); st != "" {
		if t, err := time.Parse(time.RFC3339, st); err == nil {
			opts.StartTime = &t
		}
	}
	if et := getString(args, "end_time"); et != "" {
		if t, err := time.Parse(time.RFC3339, et); err == nil {
			opts.EndTime = &t
		}
	}

	entries, total, apiErr := s.api.ListMyAuditLog(ctx, opts)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}

	return toolSuccess(map[string]interface{}{
		"entries": entries,
		"total":   total,
		"limit":   opts.Limit,
		"offset":  opts.Offset,
	}), nil
}
