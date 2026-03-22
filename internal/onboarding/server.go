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


package onboarding

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/QuestFinTech/taskschmiede/internal/auth"
	"github.com/QuestFinTech/taskschmiede/internal/storage"
)

// interviewHandler is a tool handler that operates on an interview session.
type interviewHandler func(context.Context, *mcp.CallToolRequest, *InterviewSession) (*mcp.CallToolResult, error)

// InterviewServer routes interview tool calls to isolated simulation handlers.
// Each interview session gets its own in-memory database, providing complete
// isolation from the production system. Tool calls arrive via HandleToolCall,
// which is invoked by the production MCP server's onboarding gate.
type InterviewServer struct {
	sessions     *SessionManager
	productionDB *storage.DB
	logger       *slog.Logger
	authSvc      *auth.Service
	handlers     map[string]interviewHandler
}

// NewInterviewServer creates a new interview server with handler dispatch.
func NewInterviewServer(productionDB *storage.DB, logger *slog.Logger, authSvc *auth.Service, sessions *SessionManager) *InterviewServer {
	s := &InterviewServer{
		sessions:     sessions,
		productionDB: productionDB,
		logger:       logger,
		authSvc:      authSvc,
	}

	s.handlers = map[string]interviewHandler{
		"ts.tsk.create":     s.handleSimTskCreate,
		"ts.tsk.update":     s.handleSimTskUpdate,
		"ts.tsk.cancel":     s.handleSimTskCancel,
		"ts.tsk.list":       s.handleSimTskList,
		"ts.tsk.get":        s.handleSimTskGet,
		"ts.cmt.create":     s.handleSimCmtCreate,
		"ts.msg.send":       s.handleSimMsgSend,
		"ts.dmd.create":     s.handleSimDmdCreate,
		"ts.dmd.list":       s.handleSimDmdList,
		"ts.rel.create":     s.handleSimRelCreate,
		"ts.rel.list":       s.handleSimRelList,
		"ts.onboard.submit": s.handleOnboardSubmit,
	}

	return s
}

// HandleToolCall routes a tool call from the production endpoint into the
// interview simulation. It finds the session by user ID, validates budgets,
// dispatches to the correct handler, and records the tool call in the log.
func (s *InterviewServer) HandleToolCall(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	toolName := ""
	if req.Params != nil {
		toolName = req.Params.Name
	}

	handler, ok := s.handlers[toolName]
	if !ok {
		return interviewToolError("tool_not_available", fmt.Sprintf("Tool %q is not available during the interview.", toolName)), nil
	}

	// Delegate to withInterviewSession for session lookup, budget checks, and tool call logging
	wrapped := s.withInterviewSession(handler)
	return wrapped(ctx, req)
}

// withInterviewSession wraps a handler to find the interview session and inject it.
// It also validates budgets and records tool calls.
func (s *InterviewServer) withInterviewSession(handler func(context.Context, *mcp.CallToolRequest, *InterviewSession) (*mcp.CallToolResult, error)) func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		user := auth.GetAuthUser(ctx)
		if user == nil {
			return interviewToolError("not_authenticated", "Authentication required"), nil
		}

		session := s.sessions.GetSessionByUser(user.UserID)
		if session == nil {
			return interviewToolError("no_session", "No active interview session. Start an interview first."), nil
		}

		if session.Phase == PhaseComplete {
			return interviewToolError("interview_complete", "Interview is already complete."), nil
		}

		if session.Phase == PhaseStep0 {
			return interviewToolError("step0_pending", "Complete Step 0 (self-description) before making tool calls."), nil
		}

		// Check tool is allowed
		toolName := ""
		if req.Params != nil {
			toolName = req.Params.Name
		}
		if !session.Version.IsToolAllowed(toolName) {
			return interviewToolError("tool_not_available", fmt.Sprintf("Tool %q is not available during the interview.", toolName)), nil
		}

		// Check budgets
		if err := session.CheckBudgets(); err != nil {
			return interviewToolError("budget_exhausted", err.Error()), nil
		}

		// Record start time
		start := storage.UTCNow()

		// Execute the handler
		result, err := handler(ctx, req, session)

		// Record the tool call
		duration := storage.UTCNow().Sub(start)
		args := interviewParseArgs(req)
		payloadBytes := int64(len(req.Params.Arguments))

		entry := ToolCallEntry{
			Timestamp:    start,
			Section:      session.CurrentSection,
			ToolName:     toolName,
			Parameters:   args,
			DurationMs:   duration.Milliseconds(),
			PayloadBytes: payloadBytes,
			Success:      err == nil && (result == nil || !result.IsError),
		}

		if result != nil {
			entry.Result = result
		}
		if err != nil {
			entry.Error = err.Error()
		} else if result != nil && result.IsError {
			entry.Success = false
			if len(result.Content) > 0 {
				if tc, ok := result.Content[0].(*mcp.TextContent); ok {
					entry.Error = tc.Text
				}
			}
		}

		session.ToolLog.Record(entry)

		return result, err
	}
}

// interviewToolError creates an error response for interview tools.
func interviewToolError(code, message string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: fmt.Sprintf(`{"error":{"code":"%s","message":"%s"}}`, code, message),
			},
		},
		IsError: true,
	}
}

// interviewToolErrorWithDetails creates an error response with structured details.
func interviewToolErrorWithDetails(code, message string, details map[string]interface{}) *mcp.CallToolResult {
	resp := map[string]interface{}{
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
			"details": details,
		},
	}
	jsonData, err := json.Marshal(resp)
	if err != nil {
		return interviewToolError(code, message)
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(jsonData)},
		},
		IsError: true,
	}
}

// interviewToolSuccess creates a success response for interview tools.
func interviewToolSuccess(data interface{}) *mcp.CallToolResult {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return interviewToolError("internal_error", "Failed to encode response")
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(jsonData)},
		},
	}
}

// interviewParseArgs parses tool arguments from the raw request.
func interviewParseArgs(req *mcp.CallToolRequest) map[string]interface{} {
	if req.Params == nil || req.Params.Arguments == nil {
		return make(map[string]interface{})
	}
	var args map[string]interface{}
	if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
		return make(map[string]interface{})
	}
	return args
}

func interviewGetString(args map[string]interface{}, key string) string {
	if v, ok := args[key].(string); ok {
		return v
	}
	return ""
}

func interviewGetFloat(args map[string]interface{}, key string) (float64, bool) {
	if v, ok := args[key].(float64); ok {
		return v, true
	}
	return 0, false
}
