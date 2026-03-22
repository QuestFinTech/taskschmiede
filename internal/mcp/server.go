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


// Package mcp provides the MCP server implementation for Taskschmiede.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/QuestFinTech/taskschmiede/internal/api"
	"github.com/QuestFinTech/taskschmiede/internal/auth"
	"github.com/QuestFinTech/taskschmiede/internal/docs"
	"github.com/QuestFinTech/taskschmiede/internal/onboarding"
	"github.com/QuestFinTech/taskschmiede/internal/security"
	"github.com/QuestFinTech/taskschmiede/internal/service"
	"github.com/QuestFinTech/taskschmiede/internal/storage"
)

// EmailSender interface for sending emails.
type EmailSender interface {
	SendEmail(to, subject, body string) error
}

// Server represents the MCP server.
type Server struct {
	db          *storage.DB
	logger      *slog.Logger
	mcpServer   *mcp.Server
	addr        string
	emailSender EmailSender
	authSvc     *auth.Service

	// api provides the shared business logic layer.
	// Entity handlers (tasks, demands, etc.) call api.Method() instead of
	// service layer directly. This ensures a single code path and RBAC.
	api *api.API

	// Security middleware
	auditSvc     *security.AuditService
	rateLimiter  *security.RateLimiter
	headersCfg   security.HeadersConfig
	bodyLimitCfg security.BodyLimitConfig

	// CORS origins for the REST API
	corsOrigins []string

	// sessionAuth maps MCP session IDs to authenticated users.
	// The MCP SDK's Streamable HTTP transport does not propagate per-request
	// HTTP context to tool handlers, so auth from the HTTP middleware is lost.
	// After ts.auth.login succeeds, we store the AuthUser here keyed by session ID
	// and inject it into context via withSessionAuth before tool handlers run.
	sessionAuth    map[string]*auth.AuthUser
	sessionAuthMu  sync.RWMutex
	sessionTimeout time.Duration

	// Interview server for agent onboarding (isolated simulation endpoint).
	interviewServer   *onboarding.InterviewServer
	interviewSessions *onboarding.SessionManager

	// Metric tracker for agent behavioral monitoring (Taskgovernor).
	metricTracker *MetricTracker

	// injectionReviewEnabled controls whether a pending injection review
	// is created when an interview completes. Set from config.
	injectionReviewEnabled bool

	// portalURL is the base URL for the portal (used in email links).
	portalURL string

	// version is the build version (e.g. "v0.3.2"), passed from main.
	version string

	// docsRegistry holds documentation for MCP doc tools.
	docsRegistry *docs.Registry

	// Deployment mode and onboarding gates.
	deploymentMode                string // "open" or "trusted"
	allowSelfRegistration         bool
	requireAgentEmailVerification bool
	requireAgentInterview         bool
}

// Config holds MCP server configuration.
type Config struct {
	Address                string
	EmailSender            EmailSender
	SessionTimeout         time.Duration
	AuditService           *security.AuditService
	EntityChangeWriter     *security.EntityChangeDBWriter
	RateLimiter            *security.RateLimiter
	HeadersConfig          security.HeadersConfig
	BodyLimitConfig        security.BodyLimitConfig
	CORSOrigins            []string
	MsgService             *service.MessageService
	MessageDB              *storage.MessageDB
	InjectionReviewEnabled bool
	AgentTokenTTL          time.Duration
	PortalURL              string
	Version                string

	// Deployment mode: "open" (default) or "trusted".
	DeploymentMode        string
	AllowSelfRegistration bool
	RequireAgentEmailVerification bool
	RequireAgentInterview         bool
}

// NewServer creates a new MCP server.
func NewServer(db *storage.DB, logger *slog.Logger, cfg *Config) (*Server, error) {
	authSvc := auth.NewService(db)

	// Create the shared API instance. Both REST handlers and MCP handlers
	// call into this same layer for business logic and RBAC.
	// Adapt MCP EmailSender to API EmailSender (same interface, different packages).
	var apiEmailSender api.EmailSender
	if cfg.EmailSender != nil {
		apiEmailSender = cfg.EmailSender
	}

	restAPI := api.New(&api.Config{
		DB:                 db,
		Logger:             logger,
		AuthService:        authSvc,
		AuditService:       cfg.AuditService,
		EntityChangeWriter: cfg.EntityChangeWriter,
		EmailSender:        apiEmailSender,
		CORSOrigins:        cfg.CORSOrigins,
		MsgService:         cfg.MsgService,
		MessageDB:          cfg.MessageDB,
		AgentTokenTTL:      cfg.AgentTokenTTL,
		PortalURL:          cfg.PortalURL,
		RateLimiter:        cfg.RateLimiter,
		DeploymentMode:                cfg.DeploymentMode,
		AllowSelfRegistration:         cfg.AllowSelfRegistration,
		RequireAgentEmailVerification: cfg.RequireAgentEmailVerification,
		RequireAgentInterview:         cfg.RequireAgentInterview,
	})

	s := &Server{
		db:             db,
		logger:         logger,
		addr:           cfg.Address,
		emailSender:    cfg.EmailSender,
		authSvc:        authSvc,
		api:            restAPI,
		auditSvc:       cfg.AuditService,
		rateLimiter:    cfg.RateLimiter,
		headersCfg:     cfg.HeadersConfig,
		bodyLimitCfg:   cfg.BodyLimitConfig,
		corsOrigins:    cfg.CORSOrigins,
		sessionAuth:            make(map[string]*auth.AuthUser),
		sessionTimeout:         cfg.SessionTimeout,
		metricTracker:          NewMetricTracker(db, logger),
		injectionReviewEnabled: cfg.InjectionReviewEnabled,
		portalURL:              cfg.PortalURL,
		version:                cfg.Version,
		deploymentMode:                cfg.DeploymentMode,
		allowSelfRegistration:         cfg.AllowSelfRegistration,
		requireAgentEmailVerification: cfg.RequireAgentEmailVerification,
		requireAgentInterview:         cfg.RequireAgentInterview,
	}

	// Initialize docs registry for MCP doc tools
	s.docsRegistry = docs.DefaultRegistry(cfg.Version)

	// Initialize interview session manager and server for agent onboarding
	s.interviewSessions = onboarding.NewSessionManager(logger)
	s.interviewServer = onboarding.NewInterviewServer(db, logger, authSvc, s.interviewSessions)

	// Terminate any interviews that were running when the server last stopped
	if count, err := db.TerminateRunningInterviews(); err != nil {
		logger.Warn("failed to terminate running interviews", "error", err)
	} else if count > 0 {
		logger.Info("terminated interviews from previous server run", "count", count)
	}

	// Start session cleanup goroutine
	go s.cleanupExpiredSessions()

	// Create MCP server with implementation info
	ver := s.version
	if ver == "" {
		ver = "0.0.0-dev"
	}
	impl := &mcp.Implementation{
		Name:    "taskschmiede",
		Title:   "Taskschmiede MCP Server",
		Version: ver,
	}

	opts := &mcp.ServerOptions{
		Logger: logger,
	}

	mcpServer := mcp.NewServer(impl, opts)

	// Register tools
	s.registerTools(mcpServer)

	s.mcpServer = mcpServer
	return s, nil
}

// registerTools registers all MCP tools.
func (s *Server) registerTools(mcpServer *mcp.Server) {
	// Register invitation and registration tools
	s.registerInvitationTools(mcpServer)
	s.registerRegistrationTools(mcpServer)

	// Register entity tools (vertical slice)
	s.registerOrganizationTools(mcpServer)
	s.registerEndeavourTools(mcpServer)
	s.registerTaskTools(mcpServer)
	s.registerResourceTools(mcpServer)

	// FRM tools
	s.registerRelationTools(mcpServer)
	s.registerDemandTools(mcpServer)

	// BYOM tools
	s.registerArtifactTools(mcpServer)
	s.registerRitualTools(mcpServer)
	s.registerRitualRunTools(mcpServer)

	// Collaboration tools
	s.registerCommentTools(mcpServer)
	s.registerApprovalTools(mcpServer)
	s.registerMessageTools(mcpServer)

	// Governance tools
	s.registerDodTools(mcpServer)

	// Monitoring tools
	s.registerAuditTools(mcpServer)
	s.registerEntityChangeTools(mcpServer)

	// Onboarding tools
	s.registerOnboardingTools(mcpServer)

	// Template tools
	s.registerTemplateTools(mcpServer)

	// Report tools
	s.registerReportTools(mcpServer)

	// Documentation tools
	s.registerDocTools(mcpServer)

	// Authentication: ts.auth.login
	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.auth.login",
			Description: "Authenticate with email and password to get an access token",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"email": map[string]interface{}{
						"type":        "string",
						"description": "User's email address",
					},
					"password": map[string]interface{}{
						"type":        "string",
						"description": "User's password",
					},
				},
				"required": []string{"email", "password"},
			},
		},
		s.handleAuthLogin,
	)

	// Password reset: ts.auth.forgot_password
	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.auth.forgot_password",
			Description: "Request a password reset code. Sends a code via email (or returns it directly if email is not configured).",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"email": map[string]interface{}{
						"type":        "string",
						"description": "Email address of the account to reset",
					},
				},
				"required": []string{"email"},
			},
		},
		s.handleForgotPassword,
	)

	// Password reset: ts.auth.reset_password
	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.auth.reset_password",
			Description: "Complete a password reset using the code from ts.auth.forgot_password",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"email": map[string]interface{}{
						"type":        "string",
						"description": "Email address of the account",
					},
					"code": map[string]interface{}{
						"type":        "string",
						"description": "Reset code (format: xxx-xxx-xxx)",
					},
					"new_password": map[string]interface{}{
						"type":        "string",
						"description": "New password (min 12 chars, must include upper, lower, digit, special)",
					},
				},
				"required": []string{"email", "code", "new_password"},
			},
		},
		s.handleResetPassword,
	)

	// Token verification: ts.tkn.verify
	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.tkn.verify",
			Description: "Verify a token and return associated user info",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"token": map[string]interface{}{
						"type":        "string",
						"description": "The token to verify",
					},
				},
				"required": []string{"token"},
			},
		},
		s.handleTokenVerify,
	)

	// Token creation: ts.tkn.create
	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.tkn.create",
			Description: "Create an API access token for a user (requires authentication)",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"user_id": map[string]interface{}{
						"type":        "string",
						"description": "User ID to create token for (defaults to authenticated user)",
					},
					"name": map[string]interface{}{
						"type":        "string",
						"description": "Display name for the token (e.g., 'CLI', 'CI/CD')",
					},
					"expires_at": map[string]interface{}{
						"type":        "string",
						"description": "ISO 8601 expiration datetime (null = never expires)",
					},
				},
			},
		},
		s.withSessionAuth(s.handleTokenCreate),
	)

	// ts.auth.whoami - Get current user context
	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.auth.whoami",
			Description: "Get the current user's profile, tier, limits, usage, and scope",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		s.withSessionAuth(s.handleAuthWhoami),
	)

	// ts.auth.update_profile - Update own profile
	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.auth.update_profile",
			Description: "Update the current user's own profile (name, lang, timezone, email_copy)",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type":        "string",
						"description": "New display name",
					},
					"lang": map[string]interface{}{
						"type":        "string",
						"description": "Language code (e.g., en, de, fr)",
					},
					"timezone": map[string]interface{}{
						"type":        "string",
						"description": "Timezone (e.g., Europe/Berlin)",
					},
					"email_copy": map[string]interface{}{
						"type":        "boolean",
						"description": "Receive external email copies of internal messages",
					},
				},
			},
		},
		s.withSessionAuth(s.handleAuthUpdateProfile),
	)

	// User creation: ts.usr.create
	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.usr.create",
			Description: "Create a new user. Requires admin privileges or a valid organization token for self-registration.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type":        "string",
						"description": "User's display name",
					},
					"email": map[string]interface{}{
						"type":        "string",
						"description": "User's email address (unique)",
					},
					"password": map[string]interface{}{
						"type":        "string",
						"description": "User's password (for self-registration)",
					},
					"organization_id": map[string]interface{}{
						"type":        "string",
						"description": "Organization to add user to (required for self-registration)",
					},
					"org_token": map[string]interface{}{
						"type":        "string",
						"description": "Organization registration token (for self-registration)",
					},
					"resource_id": map[string]interface{}{
						"type":        "string",
						"description": "Link to existing resource (optional)",
					},
					"external_id": map[string]interface{}{
						"type":        "string",
						"description": "External system ID for SSO (optional)",
					},
					"metadata": map[string]interface{}{
						"type":        "object",
						"description": "Arbitrary key-value pairs (optional)",
					},
				},
				"required": []string{"name", "email"},
			},
		},
		s.withSessionAuth(s.handleUserCreate),
	)

	// User get: ts.usr.get
	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.usr.get",
			Description: "Retrieve a user by ID",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "User ID",
					},
				},
				"required": []string{"id"},
			},
		},
		s.withSessionAuth(s.handleUserGet),
	)

	// User list: ts.usr.list
	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.usr.list",
			Description: "Query users with filters",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"status": map[string]interface{}{
						"type":        "string",
						"description": "Filter by status: active, inactive, suspended",
					},
					"organization_id": map[string]interface{}{
						"type":        "string",
						"description": "Filter by organization membership",
					},
					"user_type": map[string]interface{}{
						"type":        "string",
						"description": "Filter by type: human, agent",
					},
					"search": map[string]interface{}{
						"type":        "string",
						"description": "Search by name/email",
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
		s.withSessionAuth(s.handleUserList),
	)

	// User update: ts.usr.update
	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.usr.update",
			Description: "Update a user (admin only)",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "User ID",
					},
					"name": map[string]interface{}{
						"type":        "string",
						"description": "New display name",
					},
					"email": map[string]interface{}{
						"type":        "string",
						"description": "New email address",
					},
					"status": map[string]interface{}{
						"type":        "string",
						"description": "New status: active, inactive, suspended",
					},
					"lang": map[string]interface{}{
						"type":        "string",
						"description": "Language code (e.g., en, de, fr)",
					},
					"timezone": map[string]interface{}{
						"type":        "string",
						"description": "Timezone (e.g., Europe/Berlin)",
					},
					"email_copy": map[string]interface{}{
						"type":        "boolean",
						"description": "Receive external email copies of internal messages",
					},
					"metadata": map[string]interface{}{
						"type":        "object",
						"description": "Arbitrary key-value pairs",
					},
				},
				"required": []string{"id"},
			},
		},
		s.withSessionAuth(s.handleUserUpdate),
	)
}

// SetTaskschmiedStatusFunc registers a callback that returns the current
// Taskschmied circuit breaker status for the admin API endpoint.
func (s *Server) SetTaskschmiedStatusFunc(fn func() map[string]interface{}) {
	s.api.SetTaskschmiedStatusFunc(fn)
}

// SetTaskschmiedToggleFunc registers a callback to enable/disable Taskschmied
// LLM tiers independently from the admin API.
func (s *Server) SetTaskschmiedToggleFunc(fn func(target string, disabled bool)) {
	s.api.SetTaskschmiedToggleFunc(fn)
}

// Start starts the MCP server.
func (s *Server) Start() error {
	// Create Streamable HTTP handler (MCP 2025-06-18 transport)
	streamableHandler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return s.mcpServer
	}, &mcp.StreamableHTTPOptions{
		Logger: s.logger,
	})

	// Create HTTP server with routes
	mux := http.NewServeMux()

	// Streamable HTTP endpoint (new transport)
	mux.Handle("/mcp", s.authMiddleware(streamableHandler))
	mux.HandleFunc("/mcp/health", s.handleHealth)

	// REST API -- uses the same API instance as MCP handlers
	mux.Handle("/api/", s.api.Handler())

	// Build middleware chain: SecurityHeaders -> BodyLimit -> RateLimit -> AuditMiddleware -> mux
	actorExtractor := func(r *http.Request) (string, string) {
		if user := getAuthUser(r.Context()); user != nil {
			return user.UserID, "user"
		}
		return "", ""
	}

	middlewares := []func(http.Handler) http.Handler{
		security.SecurityHeaders(s.headersCfg),
		security.BodyLimit(s.bodyLimitCfg),
	}
	if s.rateLimiter != nil {
		middlewares = append(middlewares, s.rateLimiter.Middleware)
	}
	middlewares = append(middlewares, security.AuditMiddleware(s.auditSvc, actorExtractor))

	handler := security.Chain(middlewares...)(mux)

	server := &http.Server{
		Addr:           s.addr,
		Handler:        handler,
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   120 * time.Second, // SSE streaming needs long writes
		IdleTimeout:    120 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1 MB
	}

	s.logger.Info("MCP server starting", "address", s.addr, "transport", "streamable-http", "rest_api", "/api/v1/")
	return server.ListenAndServe()
}

// authMiddleware extracts and validates the Bearer token from the request.
// It adds the authenticated user to the request context if valid.
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := auth.ExtractBearerToken(r.Header.Get("Authorization"))
		if token != "" {
			if user, err := s.authSvc.VerifyToken(r.Context(), token); err == nil {
				r = r.WithContext(auth.WithAuthUser(r.Context(), user))
			}
		}
		next.ServeHTTP(w, r)
	})
}

// handleHealth returns a health check response.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	ver := s.version
	if ver == "" {
		ver = "0.0.0-dev"
	}
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "healthy",
		"service": "taskschmiede-mcp",
		"version": ver,
	})
}

// getAuthUser returns the authenticated user from context, if any.
func getAuthUser(ctx context.Context) *auth.AuthUser {
	return auth.GetAuthUser(ctx)
}

// requireAuth returns an error if no user is authenticated.
func requireAuth(ctx context.Context) (*auth.AuthUser, error) {
	return auth.RequireAuth(ctx)
}

// requireAdmin checks that the caller is authenticated and is a master admin
// (system-level). Org admins/owners do not qualify. Returns a tool error result if not.
func (s *Server) requireAdmin(ctx context.Context) (*mcp.CallToolResult, *auth.AuthUser) {
	user := getAuthUser(ctx)
	if user == nil {
		return toolError("not_authenticated", "Authentication required"), nil
	}
	if !s.authSvc.IsMasterAdmin(ctx, user.UserID) {
		return toolError("forbidden", "Admin privileges required"), nil
	}
	return nil, user
}

// withSessionAuth wraps a tool handler to inject the authenticated user from
// the session auth map into context. This bridges the gap where the MCP SDK's
// Streamable HTTP transport does not propagate HTTP request context to tool
// handlers -- after ts.auth.login stores auth in the session map, this wrapper
// makes it available to requireAuth via the standard context mechanism.
func (s *Server) withSessionAuth(handler func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error)) func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// If context already has auth (e.g. from HTTP middleware), pass through.
		if user := getAuthUser(ctx); user != nil {
			s.db.TouchUserActivity(user.UserID)
			if result, handled := s.checkOnboardingGate(ctx, user, req); handled {
				return result, nil
			}
			result, err := handler(ctx, req)
			s.recordAgentMetric(user, user.TokenID, req, result, err)
			return result, err
		}

		// Look up auth by session ID.
		if req.Session != nil {
			sessionID := req.Session.ID()
			if sessionID != "" {
				s.sessionAuthMu.Lock()
				user, ok := s.sessionAuth[sessionID]
				if ok {
					now := storage.UTCNow()

					// Enforce absolute session lifetime (24h hard cap)
					if user.CreatedAt != nil && now.After(user.CreatedAt.Add(auth.MaxSessionLifetime)) {
						delete(s.sessionAuth, sessionID)
						s.sessionAuthMu.Unlock()
						return toolError("session_expired", "Session has reached the maximum lifetime (24h). Call ts.auth.login to re-authenticate."), nil
					}

					// Check if session has expired (sliding window)
					if user.ExpiresAt != nil && now.After(*user.ExpiresAt) {
						delete(s.sessionAuth, sessionID)
						s.sessionAuthMu.Unlock()
						return toolError("session_expired", "Session has expired due to inactivity. Call ts.auth.login to re-authenticate."), nil
					}
					// Verify the underlying token is still valid in the database
					// (catches revocations from password reset, manual revoke, etc.)
					if user.TokenID != "" && !s.authSvc.IsTokenValid(ctx, user.TokenID) {
						delete(s.sessionAuth, sessionID)
						s.sessionAuthMu.Unlock()
						return toolError("session_expired", "Token has been revoked. Call ts.auth.login to re-authenticate."), nil
					}
					// Sliding window: bump expiry on every successful access
					if s.sessionTimeout > 0 {
						exp := storage.UTCNow().Add(s.sessionTimeout)
						user.ExpiresAt = &exp
					}
					s.sessionAuthMu.Unlock()

					// Per-session rate limiting (60 req/min default)
					if s.rateLimiter != nil && !s.rateLimiter.AllowSession(sessionID) {
						return toolError("rate_limited", "Too many requests. Please slow down."), nil
					}

					s.db.TouchUserActivity(user.UserID)

					if result, handled := s.checkOnboardingGate(ctx, user, req); handled {
						return result, nil
					}

					ctx = auth.WithAuthUser(ctx, user)
					result, err := handler(ctx, req)
					s.recordAgentMetric(user, sessionID, req, result, err)
					return result, err
				}
				s.sessionAuthMu.Unlock()
			}
		}

		// No auth found in context or session map.
		return toolError("not_authenticated", "No active login for this session. Call ts.auth.login first."), nil
	}
}

// recordAgentMetric records a tool call result in the metric tracker for agent
// users making production tool calls. Skips non-agent users and exempt tools.
func (s *Server) recordAgentMetric(user *auth.AuthUser, sessionID string, req *mcp.CallToolRequest, result *mcp.CallToolResult, err error) {
	if user.UserType != "agent" || sessionID == "" {
		return
	}
	toolName := ""
	if req.Params != nil {
		toolName = req.Params.Name
	}
	if isOnboardingExemptTool(toolName) {
		return
	}
	s.metricTracker.Record(user.UserID, sessionID, toolName, result, err)
}

// checkOnboardingGate verifies that agent users have completed onboarding
// before allowing access to production tools. Returns (nil, false) to allow
// the call through to the production handler, or (result, true) when the
// gate has handled the call (either by routing it to the interview simulation
// or by returning a blocking error).
func (s *Server) checkOnboardingGate(ctx context.Context, user *auth.AuthUser, req *mcp.CallToolRequest) (*mcp.CallToolResult, bool) {
	// Only gate agent users
	if user.UserType != "agent" {
		return nil, false
	}

	toolName := ""
	if req.Params != nil {
		toolName = req.Params.Name
	}

	// Look up onboarding status
	status, err := s.db.GetUserOnboardingStatus(user.UserID)
	if err != nil {
		s.logger.Warn("failed to check onboarding status", "user_id", user.UserID, "error", err)
		return nil, false // fail open on DB errors to avoid blocking legitimate users
	}

	if status == "active" || status == "interview_skipped" {
		return nil, false
	}

	// For interview_running agents, route allowed tools to the simulation
	// BEFORE checking exempt prefixes (ts.onboard.submit is both exempt
	// and an interview tool -- it must go to the simulation during interviews).
	if status == "interview_running" && s.interviewServer != nil {
		session := s.interviewSessions.GetSessionByUser(user.UserID)
		if session != nil && session.Version.IsToolAllowed(toolName) {
			result, callErr := s.interviewServer.HandleToolCall(
				auth.WithAuthUser(ctx, user), req)
			if callErr != nil {
				return toolError("interview_error", callErr.Error()), true
			}
			return result, true
		}
		// Not an interview-allowed tool: let exempt tools through,
		// block everything else below.
	}

	// Exempt tools pass through regardless of onboarding status
	if isOnboardingExemptTool(toolName) {
		return nil, false
	}

	switch status {
	case "interview_pending":
		return toolError("onboarding_required", "You must complete the onboarding interview before using production tools. Call ts.onboard.start_interview to begin."), true
	case "interview_running":
		return toolError("onboarding_in_progress", "Complete your onboarding interview before using production tools. Interview tool calls are routed automatically."), true
	case "cooldown":
		return toolError("onboarding_cooldown", "You are in a cooldown period after a failed interview. Call ts.onboard.status for details."), true
	case "locked":
		return toolError("onboarding_locked", "Your account is locked after too many failed interview attempts. Contact an administrator."), true
	case "suspended":
		return toolError("onboarding_suspended",
			"Your access has been suspended due to sustained performance degradation. "+
				"Contact an administrator to arrange re-assessment."), true
	case "blocked":
		return toolError("account_blocked",
			"Your account has been blocked by your sponsor. "+
				"Contact your sponsor to resolve this."), true
	default:
		return toolError("onboarding_required", fmt.Sprintf("Unexpected onboarding status: %s. Contact an administrator.", status)), true
	}
}

// isOnboardingExemptTool returns true for tools that should work regardless
// of onboarding status (auth, registration, invitation, onboarding tools).
func isOnboardingExemptTool(name string) bool {
	if len(name) < 3 {
		return false
	}
	// ts.auth.*, ts.onboard.*, ts.reg.*, ts.inv.*
	prefixes := []string{"ts.auth.", "ts.onboard.", "ts.reg.", "ts.inv."}
	for _, p := range prefixes {
		if len(name) >= len(p) && name[:len(p)] == p {
			return true
		}
	}
	return false
}

// toolError creates an error response for MCP tools.
func toolError(code, message string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: fmt.Sprintf(`{"error":{"code":"%s","message":"%s"}}`, code, message),
			},
		},
		IsError: true,
	}
}

// toolAPIError converts an APIError to an MCP tool error response.
// If the error has structured Details, they are included in the JSON response.
func toolAPIError(e *api.APIError) *mcp.CallToolResult {
	if e.Details != nil {
		return toolAPIErrorWithDetails(e)
	}
	return toolError(e.Code, e.Message)
}

// toolAPIErrorWithDetails creates an MCP error response that includes structured details.
func toolAPIErrorWithDetails(e *api.APIError) *mcp.CallToolResult {
	resp := map[string]interface{}{
		"error": map[string]interface{}{
			"code":    e.Code,
			"message": e.Message,
			"details": e.Details,
		},
	}
	jsonData, err := json.Marshal(resp)
	if err != nil {
		return toolError(e.Code, e.Message)
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(jsonData)},
		},
		IsError: true,
	}
}

// toolSuccess creates a success response for MCP tools.
func toolSuccess(data interface{}) *mcp.CallToolResult {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return toolError("internal_error", "Failed to encode response")
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: string(jsonData),
			},
		},
	}
}

// cleanupExpiredSessions periodically removes expired entries from the session auth map.
func (s *Server) cleanupExpiredSessions() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		now := storage.UTCNow()
		s.sessionAuthMu.Lock()
		for id, user := range s.sessionAuth {
			expired := user.ExpiresAt != nil && now.After(*user.ExpiresAt)
			absoluteExpired := user.CreatedAt != nil && now.After(user.CreatedAt.Add(auth.MaxSessionLifetime))
			if expired || absoluteExpired {
				delete(s.sessionAuth, id)
				s.logger.Debug("Cleaned up expired session", "session_id", id)
			}
		}
		s.sessionAuthMu.Unlock()
	}
}

// parseArgs parses tool arguments from the raw request.
func parseArgs(req *mcp.CallToolRequest) map[string]interface{} {
	if req.Params == nil || req.Params.Arguments == nil {
		return make(map[string]interface{})
	}

	// Arguments is json.RawMessage, unmarshal to map
	var args map[string]interface{}
	if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
		return make(map[string]interface{})
	}
	return args
}

// getString gets a string value from args, applying input hygiene (NFC normalize,
// strip zero-width chars, strip control chars, trim whitespace).
func getString(args map[string]interface{}, key string) string {
	if v, ok := args[key].(string); ok {
		return security.SanitizeInput(v)
	}
	return ""
}

// getInt gets an integer value from args.
func getInt(args map[string]interface{}, key string, defaultVal int) int {
	if v, ok := args[key].(float64); ok {
		return int(v)
	}
	if v, ok := args[key].(int); ok {
		return v
	}
	return defaultVal
}

// getBool gets a boolean value from args.
func getBool(args map[string]interface{}, key string) bool {
	if v, ok := args[key].(bool); ok {
		return v
	}
	return false
}
