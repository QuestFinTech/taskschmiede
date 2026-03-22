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
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

// MCPSecurityConfig holds configuration for MCP-level security features.
// These are protocol-aware measures that NGINX cannot express: JSON-RPC
// validation, per-tool rate limiting, and API version management.
type MCPSecurityConfig struct {
	// ValidationEnabled enables JSON-RPC structure and method validation.
	ValidationEnabled bool

	// ToolRateLimits defines per-tool rate limits. Keys are tool names
	// (exact) or glob patterns (e.g., "ts.onboard.*"). Values define
	// the number of allowed requests per time window.
	ToolRateLimits map[string]ToolRateLimit

	// Versions is the API version manifest served at /proxy/versions.
	// If nil, a default manifest is generated.
	Versions *VersionManifest

	// RESTDeprecations maps REST API versions to deprecation messages.
	// When a request matches a deprecated version, the proxy injects
	// an X-API-Deprecation header. Empty map = no deprecations.
	RESTDeprecations map[string]string
}

// ToolRateLimit defines a rate limit for a tool or tool group.
type ToolRateLimit struct {
	Requests int           `yaml:"requests"`
	Window   time.Duration `yaml:"window"`
}

// VersionManifest describes supported API versions for REST and MCP.
type VersionManifest struct {
	REST RESTVersionInfo `json:"rest"`
	MCP  MCPVersionInfo  `json:"mcp"`
}

// RESTVersionInfo describes REST API version support.
type RESTVersionInfo struct {
	Supported  []string `json:"supported"`
	Current    string   `json:"current"`
	Deprecated []string `json:"deprecated"`
}

// MCPVersionInfo describes MCP protocol and tool version support.
type MCPVersionInfo struct {
	Protocol string       `json:"protocol"`
	Tools    ToolVersions `json:"tools"`
}

// ToolVersions describes tool-level versioning (deprecations and aliases).
type ToolVersions struct {
	Deprecated []string          `json:"deprecated"`
	Aliases    map[string]string `json:"aliases"`
}

// standardMCPMethods lists the exact MCP methods and prefixes the proxy accepts.
// Requests with methods outside this list are rejected when validation is enabled.
var standardMCPMethods = []string{
	"initialize",
	"ping",
	"tools/",
	"resources/",
	"prompts/",
	"completion/",
	"logging/",
	"notifications/",
}

// mcpSecurity handles MCP-level validation, per-tool rate limiting,
// and version management.
type mcpSecurity struct {
	cfg    MCPSecurityConfig
	logger *slog.Logger

	// Per-tool rate limit buckets: key = "toolName:actor"
	mu      sync.Mutex
	buckets map[string]*rateBucket
}

// rateBucket implements a token bucket for rate limiting.
type rateBucket struct {
	tokens     float64
	maxTokens  float64
	refillRate float64   // tokens per second
	lastRefill time.Time
}

// allow consumes one token and returns true if the request is within the rate limit.
func (b *rateBucket) allow() bool {
	now := time.Now().UTC()
	elapsed := now.Sub(b.lastRefill).Seconds()
	b.tokens = min(b.maxTokens, b.tokens+elapsed*b.refillRate)
	b.lastRefill = now

	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}

// parsedMCPMsg holds the fields extracted from a JSON-RPC message.
type parsedMCPMsg struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

// toolCallParams holds the fields extracted from tools/call params.
type toolCallParams struct {
	Name string `json:"name"`
}

// newMCPSecurity creates a new MCP security handler with the given configuration.
func newMCPSecurity(logger *slog.Logger, cfg MCPSecurityConfig) *mcpSecurity {
	return &mcpSecurity{
		cfg:     cfg,
		logger:  logger,
		buckets: make(map[string]*rateBucket),
	}
}

// securityError represents a validation or rate limit failure.
type securityError struct {
	Code      int    // JSON-RPC error code
	Message   string // human-readable error
	HTTPCode  int    // HTTP status code
	ErrorType string // "validation" or "rate_limit"
}

// WriteMCPError writes the error as a JSON-RPC response.
func (e *securityError) WriteMCPError(w http.ResponseWriter, requestID interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if e.ErrorType == "rate_limit" {
		w.Header().Set("Retry-After", "60")
	}
	w.WriteHeader(e.HTTPCode)

	id := requestID
	if id == nil {
		id = 1
	}

	errorData := map[string]interface{}{
		"type": e.ErrorType,
	}
	if e.ErrorType == "rate_limit" {
		errorData["retry_after_seconds"] = 60
	}

	resp := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"error": map[string]interface{}{
			"code":    e.Code,
			"message": e.Message,
			"data":    errorData,
		},
	}
	_ = json.NewEncoder(w).Encode(resp)
}

// Check validates a JSON-RPC message and applies per-tool rate limits.
// Returns nil on success, or a securityError on failure. The parsed message
// is returned when available (even on some errors) so the caller can
// extract the request ID for error responses.
func (s *mcpSecurity) Check(body []byte, actorKey string) (*securityError, *parsedMCPMsg) {
	var msg parsedMCPMsg
	if err := json.Unmarshal(body, &msg); err != nil {
		return &securityError{
			Code:      -32700,
			Message:   "Parse error: invalid JSON",
			HTTPCode:  http.StatusBadRequest,
			ErrorType: "validation",
		}, nil
	}

	if s.cfg.ValidationEnabled {
		if msg.JSONRPC != "2.0" {
			return &securityError{
				Code:      -32600,
				Message:   "Invalid Request: missing or wrong jsonrpc version",
				HTTPCode:  http.StatusBadRequest,
				ErrorType: "validation",
			}, &msg
		}

		if msg.Method == "" {
			return &securityError{
				Code:      -32600,
				Message:   "Invalid Request: missing method",
				HTTPCode:  http.StatusBadRequest,
				ErrorType: "validation",
			}, &msg
		}

		if !isAllowedMethod(msg.Method) {
			s.logger.Warn("Rejected unknown MCP method",
				"method", msg.Method, "actor", actorKey)
			return &securityError{
				Code:      -32601,
				Message:   fmt.Sprintf("Method not found: %s", msg.Method),
				HTTPCode:  http.StatusBadRequest,
				ErrorType: "validation",
			}, &msg
		}
	}

	// Per-tool rate limiting for tools/call requests
	if msg.Method == "tools/call" && len(msg.Params) > 0 {
		var params toolCallParams
		if json.Unmarshal(msg.Params, &params) == nil && params.Name != "" {
			if err := s.checkToolRateLimit(params.Name, actorKey); err != nil {
				return err, &msg
			}
		}
	}

	return nil, &msg
}

// isAllowedMethod checks if a method matches standard MCP methods/prefixes.
func isAllowedMethod(method string) bool {
	for _, allowed := range standardMCPMethods {
		if method == allowed || strings.HasPrefix(method, allowed) {
			return true
		}
	}
	return false
}

// checkToolRateLimit applies per-tool rate limiting using token buckets.
func (s *mcpSecurity) checkToolRateLimit(toolName, actorKey string) *securityError {
	if len(s.cfg.ToolRateLimits) == 0 {
		return nil
	}

	limit, ok := s.findToolLimit(toolName)
	if !ok {
		return nil
	}

	key := toolName + ":" + actorKey

	s.mu.Lock()
	bucket, exists := s.buckets[key]
	if !exists {
		bucket = &rateBucket{
			tokens:     float64(limit.Requests),
			maxTokens:  float64(limit.Requests),
			refillRate: float64(limit.Requests) / limit.Window.Seconds(),
			lastRefill: time.Now().UTC(),
		}
		s.buckets[key] = bucket
	}
	allowed := bucket.allow()
	s.mu.Unlock()

	if !allowed {
		s.logger.Warn("Tool rate limit exceeded",
			"tool", toolName, "actor", actorKey,
			"limit", fmt.Sprintf("%d/%s", limit.Requests, limit.Window))
		return &securityError{
			Code:      -32000,
			Message:   fmt.Sprintf("Rate limit exceeded for tool %s", toolName),
			HTTPCode:  http.StatusTooManyRequests,
			ErrorType: "rate_limit",
		}
	}

	return nil
}

// findToolLimit finds the rate limit for a tool name, supporting glob patterns.
// Exact match takes priority over glob patterns.
func (s *mcpSecurity) findToolLimit(toolName string) (ToolRateLimit, bool) {
	// Exact match first
	if limit, ok := s.cfg.ToolRateLimits[toolName]; ok {
		return limit, true
	}

	// Glob match: "ts.onboard.*" matches "ts.onboard.start_interview"
	for pattern, limit := range s.cfg.ToolRateLimits {
		if !strings.HasSuffix(pattern, ".*") {
			continue
		}
		prefix := strings.TrimSuffix(pattern, ".*")
		if strings.HasPrefix(toolName, prefix+".") {
			return limit, true
		}
	}

	return ToolRateLimit{}, false
}

// startCleanup periodically removes stale rate limit buckets.
func (s *mcpSecurity) startCleanup(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.cleanup(10 * time.Minute)
		}
	}
}

// cleanup removes buckets that haven't been used recently.
func (s *mcpSecurity) cleanup(maxAge time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().UTC().Add(-maxAge)
	removed := 0
	for key, bucket := range s.buckets {
		if bucket.lastRefill.Before(cutoff) {
			delete(s.buckets, key)
			removed++
		}
	}
	if removed > 0 {
		s.logger.Debug("Cleaned up stale rate limit buckets", "removed", removed)
	}
}

// defaultVersionManifest returns the default version manifest when none
// is configured.
func defaultVersionManifest() *VersionManifest {
	return &VersionManifest{
		REST: RESTVersionInfo{
			Supported:  []string{"v1"},
			Current:    "v1",
			Deprecated: []string{},
		},
		MCP: MCPVersionInfo{
			Protocol: "2025-06-18",
			Tools: ToolVersions{
				Deprecated: []string{},
				Aliases:    map[string]string{},
			},
		},
	}
}

// writeVersionsResponse writes the version manifest as JSON.
func (s *mcpSecurity) writeVersionsResponse(w http.ResponseWriter) {
	manifest := s.cfg.Versions
	if manifest == nil {
		manifest = defaultVersionManifest()
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(manifest)
}

// injectDeprecationHeader adds an X-API-Deprecation header if the request
// path matches a deprecated REST API version.
func (s *mcpSecurity) injectDeprecationHeader(w http.ResponseWriter, path string) {
	for version, msg := range s.cfg.RESTDeprecations {
		if strings.HasPrefix(path, "/api/"+version+"/") {
			w.Header().Set("X-API-Deprecation", msg)
			return
		}
	}
}
