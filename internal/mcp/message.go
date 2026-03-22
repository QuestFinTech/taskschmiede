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

// registerMessageTools registers messaging MCP tools.
func (s *Server) registerMessageTools(mcpServer *mcp.Server) {
	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.msg.send",
			Description: "Send a message to one or more recipients (direct, endeavour, or organization scope)",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"content": map[string]interface{}{
						"type":        "string",
						"description": "Message body (Markdown)",
					},
					"subject": map[string]interface{}{
						"type":        "string",
						"description": "Message subject (optional)",
					},
					"intent": map[string]interface{}{
						"type":        "string",
						"description": "Message intent: info, question, action, alert (default: info)",
					},
					"recipient_ids": map[string]interface{}{
						"type":        "array",
						"items":       map[string]interface{}{"type": "string"},
						"description": "Resource IDs of direct recipients",
					},
					"scope_type": map[string]interface{}{
						"type":        "string",
						"description": "Scope for group delivery: endeavour, organization",
					},
					"scope_id": map[string]interface{}{
						"type":        "string",
						"description": "ID of the endeavour or organization (required when scope_type is set)",
					},
					"reply_to_id": map[string]interface{}{
						"type":        "string",
						"description": "Message ID to reply to (creates a thread)",
					},
					"entity_type": map[string]interface{}{
						"type":        "string",
						"description": "Optional context: entity type (task, endeavour, ...)",
					},
					"entity_id": map[string]interface{}{
						"type":        "string",
						"description": "Optional context: entity ID",
					},
					"metadata": map[string]interface{}{
						"type":        "object",
						"description": "Arbitrary key-value pairs",
					},
				},
				"required": []string{"content"},
			},
		},
		s.withSessionAuth(s.handleMsgSend),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.msg.inbox",
			Description: "List unread/recent messages for current resource",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"status": map[string]interface{}{
						"type":        "string",
						"description": "Filter by status: pending, delivered, read",
					},
					"intent": map[string]interface{}{
						"type":        "string",
						"description": "Filter by intent: info, question, action, alert",
					},
					"unread": map[string]interface{}{
						"type":        "boolean",
						"description": "Show only unread messages (status != read)",
					},
					"entity_type": map[string]interface{}{
						"type":        "string",
						"description": "Filter by context entity type",
					},
					"entity_id": map[string]interface{}{
						"type":        "string",
						"description": "Filter by context entity ID",
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
		s.withSessionAuth(s.handleMsgInbox),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.msg.read",
			Description: "Get a message and mark as read",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "Message ID",
					},
				},
				"required": []string{"id"},
			},
		},
		s.withSessionAuth(s.handleMsgRead),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.msg.reply",
			Description: "Reply to a message",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"message_id": map[string]interface{}{
						"type":        "string",
						"description": "ID of the message to reply to",
					},
					"content": map[string]interface{}{
						"type":        "string",
						"description": "Reply body (Markdown)",
					},
					"metadata": map[string]interface{}{
						"type":        "object",
						"description": "Arbitrary key-value pairs",
					},
				},
				"required": []string{"message_id", "content"},
			},
		},
		s.withSessionAuth(s.handleMsgReply),
	)

	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.msg.thread",
			Description: "Get full conversation thread",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"message_id": map[string]interface{}{
						"type":        "string",
						"description": "Any message ID in the thread",
					},
				},
				"required": []string{"message_id"},
			},
		},
		s.withSessionAuth(s.handleMsgThread),
	)
}

// handleMsgSend handles the ts.msg.send tool.
func (s *Server) handleMsgSend(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := requireAuth(ctx); err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	args := parseArgs(req)

	content := getString(args, "content")
	if content == "" {
		return toolError("invalid_input", "content is required"), nil
	}

	// Parse recipient_ids array
	var recipientIDs []string
	if rids, ok := args["recipient_ids"].([]interface{}); ok {
		for _, rid := range rids {
			if s, ok := rid.(string); ok && s != "" {
				recipientIDs = append(recipientIDs, s)
			}
		}
	}

	var metadata map[string]interface{}
	if m, ok := args["metadata"].(map[string]interface{}); ok {
		metadata = m
	}

	result, apiErr := s.api.SendMessage(ctx,
		getString(args, "subject"),
		content,
		getString(args, "intent"),
		getString(args, "reply_to_id"),
		getString(args, "entity_type"),
		getString(args, "entity_id"),
		recipientIDs,
		getString(args, "scope_type"),
		getString(args, "scope_id"),
		metadata,
	)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}
	security.FrameMapFields(result, security.MessageFrameSpec)
	return toolSuccess(result), nil
}

// handleMsgInbox handles the ts.msg.inbox tool.
func (s *Server) handleMsgInbox(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := requireAuth(ctx); err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	args := parseArgs(req)

	opts := storage.ListInboxOpts{
		Status:     getString(args, "status"),
		Intent:     getString(args, "intent"),
		EntityType: getString(args, "entity_type"),
		EntityID:   getString(args, "entity_id"),
		Limit:      getInt(args, "limit", 50),
		Offset:     getInt(args, "offset", 0),
	}
	if v, ok := args["unread"].(bool); ok {
		opts.Unread = v
	}

	items, total, apiErr := s.api.GetInbox(ctx, opts)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}
	security.FrameMapSlice(items, security.MessageFrameSpec)

	return toolSuccess(map[string]interface{}{
		"messages": items,
		"total":    total,
		"limit":    opts.Limit,
		"offset":   opts.Offset,
	}), nil
}

// handleMsgRead handles the ts.msg.read tool.
func (s *Server) handleMsgRead(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := requireAuth(ctx); err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	id := getString(parseArgs(req), "id")
	if id == "" {
		return toolError("invalid_input", "Message ID is required"), nil
	}

	result, apiErr := s.api.ReadMessage(ctx, id)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}
	security.FrameMapFields(result, security.MessageFrameSpec)
	return toolSuccess(result), nil
}

// handleMsgReply handles the ts.msg.reply tool.
func (s *Server) handleMsgReply(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := requireAuth(ctx); err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	args := parseArgs(req)

	messageID := getString(args, "message_id")
	if messageID == "" {
		return toolError("invalid_input", "message_id is required"), nil
	}
	content := getString(args, "content")
	if content == "" {
		return toolError("invalid_input", "content is required"), nil
	}

	var metadata map[string]interface{}
	if m, ok := args["metadata"].(map[string]interface{}); ok {
		metadata = m
	}

	result, apiErr := s.api.ReplyToMessage(ctx, messageID, content, metadata)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}
	security.FrameMapFields(result, security.MessageFrameSpec)
	return toolSuccess(result), nil
}

// handleMsgThread handles the ts.msg.thread tool.
func (s *Server) handleMsgThread(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, err := requireAuth(ctx); err != nil {
		return toolError("unauthorized", err.Error()), nil
	}

	messageID := getString(parseArgs(req), "message_id")
	if messageID == "" {
		return toolError("invalid_input", "message_id is required"), nil
	}

	result, apiErr := s.api.GetThread(ctx, messageID)
	if apiErr != nil {
		return toolAPIError(apiErr), nil
	}
	security.FrameMapSlice(result, security.MessageFrameSpec)
	return toolSuccess(map[string]interface{}{
		"messages": result,
		"total":    len(result),
	}), nil
}
