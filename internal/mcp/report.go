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
)

// registerReportTools registers report generation MCP tools.
func (s *Server) registerReportTools(mcpServer *mcp.Server) {
	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.rpt.generate",
			Description: "Generate a Markdown report for a task, demand, endeavour, or project",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"scope": map[string]interface{}{
						"type":        "string",
						"description": "Report scope: task, demand, endeavour, or project",
						"enum":        []string{"task", "demand", "endeavour", "project"},
					},
					"entity_id": map[string]interface{}{
						"type":        "string",
						"description": "ID of the entity to report on",
					},
				},
				"required": []string{"scope", "entity_id"},
			},
		},
		s.withSessionAuth(s.handleRptGenerate),
	)
}

// handleRptGenerate handles the ts.rpt.generate tool.
func (s *Server) handleRptGenerate(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := requireAuth(ctx); err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	args := parseArgs(req)
	scope := getString(args, "scope")
	entityID := getString(args, "entity_id")

	if scope == "" {
		return toolError("invalid_input", "scope is required (task, demand, endeavour, or project)"), nil
	}
	if entityID == "" {
		return toolError("invalid_input", "entity_id is required"), nil
	}

	result, apiErr := s.api.GenerateReport(ctx, scope, entityID)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}

	return toolSuccess(result), nil
}
