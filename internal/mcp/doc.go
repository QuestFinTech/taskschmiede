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
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// registerDocTools registers documentation lookup MCP tools.
func (s *Server) registerDocTools(mcpServer *mcp.Server) {
	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.doc.list",
			Description: "List available documentation (guides, workflows)",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"type": map[string]interface{}{
						"type":        "string",
						"description": "Filter by type: guide, workflow (omit for all)",
						"enum":        []string{"guide", "workflow"},
					},
				},
			},
		},
		s.handleDocList,
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.doc.get",
			Description: "Get a specific document as Markdown",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type":        "string",
						"description": "Document identifier (e.g., onboard-agent, deploy-production)",
					},
				},
				"required": []string{"name"},
			},
		},
		s.handleDocGet,
	)
}

// docListEntry is a lightweight entry returned by ts.doc.list.
type docListEntry struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Title   string `json:"title"`
	Summary string `json:"summary"`
}

// handleDocList handles the ts.doc.list tool.
func (s *Server) handleDocList(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := parseArgs(req)
	docType := getString(args, "type")

	var entries []docListEntry

	var docs []*docListEntry
	if docType == "" {
		for _, d := range s.docsRegistry.AllContentDocs() {
			docs = append(docs, &docListEntry{
				Name:    d.Name,
				Type:    d.Type,
				Title:   d.Title,
				Summary: d.Summary,
			})
		}
	} else {
		for _, d := range s.docsRegistry.ContentDocsByType(docType) {
			docs = append(docs, &docListEntry{
				Name:    d.Name,
				Type:    d.Type,
				Title:   d.Title,
				Summary: d.Summary,
			})
		}
	}

	for _, d := range docs {
		entries = append(entries, *d)
	}

	if entries == nil {
		entries = []docListEntry{}
	}

	return toolSuccess(entries), nil
}

// handleDocGet handles the ts.doc.get tool.
func (s *Server) handleDocGet(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := parseArgs(req)
	name := getString(args, "name")

	if name == "" {
		return toolError("invalid_input", "name is required"), nil
	}

	doc := s.docsRegistry.GetContentDoc(name)
	if doc == nil {
		return toolError("not_found", fmt.Sprintf("document %q not found", name)), nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: doc.Body},
		},
	}, nil
}
