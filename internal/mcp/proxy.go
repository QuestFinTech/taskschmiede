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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuestFinTech/taskschmiede/internal/notify"
	"github.com/QuestFinTech/taskschmiede/internal/security"
)

// Proxy is an MCP proxy that sits between clients and the upstream server.
// It provides:
// - Protocol translation: old HTTP+SSE clients can talk to new Streamable HTTP backend
// - Session recovery: transparent re-initialization when upstream restarts
// - Logging of all MCP traffic
// - Stable connection for clients
//
// The proxy serves both transport versions to clients:
// - /mcp (new Streamable HTTP) - proxied with session recovery
// - /mcp/sse + /mcp/message (old HTTP+SSE) - translated to new transport
type Proxy struct {
	upstreamURL string
	listenAddr  string
	logger      *slog.Logger

	// Connection tracking for old transport clients (HTTP+SSE)
	clients     map[string]*proxyClient
	clientsMu   sync.RWMutex
	clientIDSeq atomic.Int64

	// Session tracking for new transport clients (Streamable HTTP).
	// Maps client-facing session IDs to current upstream session IDs.
	// When upstream restarts and old sessions are invalidated, the proxy
	// re-initializes and updates the mapping so clients stay connected.
	streamSessions   map[string]string
	streamSessionsMu sync.RWMutex

	// Traffic logging
	logTraffic     bool
	trafficLogFile string
	trafficLog     *os.File
	trafficLogMu   sync.Mutex

	// REST reverse proxy
	restProxy *httputil.ReverseProxy

	// Security middleware
	rateLimiter  *security.RateLimiter
	connLimiter  *security.ConnLimiter
	headersCfg   security.HeadersConfig
	bodyLimitCfg security.BodyLimitConfig
	corsOrigins  []string

	// Maintenance mode (production)
	monitor       *upstreamMonitor
	httpClient    *http.Client
	sseHTTPClient *http.Client

	// MCP-level security (validation, per-tool rate limiting, versioning)
	mcpSec *mcpSecurity
}

// proxyClient represents a connected old-transport client.
type proxyClient struct {
	id      string
	writer  http.ResponseWriter
	flusher http.Flusher
	done    chan struct{}

	// Upstream session (new transport)
	upstreamSessionID string
	upstreamMu        sync.Mutex

	// Init replay: captured initialize request for replay after upstream reconnect
	lastInitializeBody []byte
	initialized        bool
}

// ProxyConfig holds configuration for the MCP proxy.
type ProxyConfig struct {
	UpstreamURL     string
	ListenAddr      string
	LogTraffic      bool
	TrafficLogFile  string // path to traffic log file (default: taskschmiede-mcp-traffic.log)
	RateLimiter     *security.RateLimiter
	ConnLimiter     *security.ConnLimiter
	HeadersConfig   security.HeadersConfig
	BodyLimitConfig security.BodyLimitConfig
	CORSOrigins       []string          // allowed CORS origins; empty = no CORS headers sent
	MaintenanceConfig *MaintenanceConfig   // nil = no maintenance mode
	Notifier          *notify.Notifier     // optional state change notifier
	MCPSecurityConfig *MCPSecurityConfig   // nil = no MCP-level security
}

// NewProxy creates a new MCP proxy.
func NewProxy(logger *slog.Logger, cfg *ProxyConfig) *Proxy {
	// Build REST reverse proxy targeting the upstream app server.
	upstreamURL, _ := url.Parse(cfg.UpstreamURL)
	restRP := &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(upstreamURL)
			// Preserve the original path (e.g. /api/v1/tasks).
			r.Out.URL.Path = r.In.URL.Path
			r.Out.URL.RawQuery = r.In.URL.RawQuery
			r.Out.Host = upstreamURL.Host
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			logger.Error("REST proxy upstream error", "error", err, "path", r.URL.Path)
			http.Error(w, "Upstream unavailable", http.StatusBadGateway)
		},
	}

	// HTTP clients for upstream requests.
	// When maintenance config is set (production), use configured timeouts.
	// Otherwise keep default behavior (no timeout) for development.
	httpClient := http.DefaultClient
	sseHTTPClient := http.DefaultClient
	if cfg.MaintenanceConfig != nil {
		timeout := cfg.MaintenanceConfig.UpstreamTimeout
		if timeout <= 0 {
			timeout = 30 * time.Second
		}
		httpClient = &http.Client{Timeout: timeout}

		sseTimeout := cfg.MaintenanceConfig.UpstreamTimeoutSSE
		if sseTimeout <= 0 {
			sseTimeout = 300 * time.Second
		}
		sseHTTPClient = &http.Client{Timeout: sseTimeout}
	}

	// Upstream monitor for maintenance mode (production).
	var monitor *upstreamMonitor
	if cfg.MaintenanceConfig != nil {
		monitor = newUpstreamMonitor(logger, cfg.UpstreamURL, *cfg.MaintenanceConfig)
	}

	// MCP-level security (validation, per-tool rate limiting, versioning).
	var mcpSec *mcpSecurity
	if cfg.MCPSecurityConfig != nil {
		mcpSec = newMCPSecurity(logger, *cfg.MCPSecurityConfig)
	}

	// Wire state change notifications.
	if monitor != nil && cfg.Notifier != nil && cfg.Notifier.HasChannels() {
		notifier := cfg.Notifier
		monitor.onStateChange = func(from, to UpstreamState, info string) {
			notifier.Send(notify.Event{
				Service:   "taskschmiede-proxy",
				Type:      "state_change",
				Summary:   fmt.Sprintf("[Taskschmiede] Upstream: %s -> %s", from.String(), to.String()),
				Detail:    info,
				Timestamp: time.Now().UTC(),
				Fields: map[string]string{
					"from_state": from.String(),
					"to_state":   to.String(),
				},
			})
		}
	}

	return &Proxy{
		upstreamURL:    cfg.UpstreamURL,
		listenAddr:     cfg.ListenAddr,
		logger:         logger,
		clients:        make(map[string]*proxyClient),
		streamSessions: make(map[string]string),
		restProxy:      restRP,
		logTraffic:     cfg.LogTraffic,
		trafficLogFile: cfg.TrafficLogFile,
		rateLimiter:    cfg.RateLimiter,
		connLimiter:    cfg.ConnLimiter,
		headersCfg:     cfg.HeadersConfig,
		bodyLimitCfg:   cfg.BodyLimitConfig,
		corsOrigins:    cfg.CORSOrigins,
		monitor:        monitor,
		httpClient:     httpClient,
		sseHTTPClient:  sseHTTPClient,
		mcpSec:         mcpSec,
	}
}

// Start starts the proxy server. It blocks until ctx is cancelled or the
// server encounters a fatal error. When ctx is cancelled, the server shuts
// down gracefully.
func (p *Proxy) Start(ctx context.Context) error {
	// Open traffic log file
	if p.logTraffic {
		logPath := p.trafficLogFile
		if logPath == "" {
			logPath = "taskschmiede-mcp-traffic.log"
		}
		f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
		if err != nil {
			return fmt.Errorf("open traffic log %s: %w", logPath, err)
		}
		p.trafficLog = f
		p.logger.Info("Traffic logging to file", "path", logPath)
	}

	// Start MCP security cleanup (rate limit bucket eviction)
	if p.mcpSec != nil {
		go p.mcpSec.startCleanup(ctx)
		p.logger.Info("MCP security enabled",
			"validation", p.mcpSec.cfg.ValidationEnabled,
			"tool_rate_limits", len(p.mcpSec.cfg.ToolRateLimits))
	}

	// Start upstream monitor (maintenance mode, health checking, management API)
	if p.monitor != nil {
		go func() {
			if err := p.monitor.Start(ctx); err != nil {
				p.logger.Error("Upstream monitor error", "error", err)
			}
		}()
		p.logger.Info("Upstream monitor started",
			"auto_detect", p.monitor.cfg.AutoDetect,
			"management_listen", p.monitor.cfg.ManagementListen)
	}

	mux := http.NewServeMux()

	// New Streamable HTTP transport (MCP 2025-06-18) - proxy to upstream
	mux.HandleFunc("/mcp", p.handleStreamableHTTP)

	// Old HTTP+SSE transport (MCP 2024-11-05) - translate to new transport
	mux.HandleFunc("/mcp/sse", p.handleSSE)
	mux.HandleFunc("/mcp/message", p.handleMessage)

	// REST API reverse proxy - forwards /api/* to upstream app server
	mux.HandleFunc("/api/", p.handleREST)

	// Health and discovery endpoints
	mux.HandleFunc("/mcp/health", p.handleHealth)
	mux.HandleFunc("/proxy/health", p.handleProxyHealth)
	mux.HandleFunc("/proxy/versions", p.handleVersions)

	// Build middleware chain: SecurityHeaders -> BodyLimit -> ConnLimit -> RateLimit -> mux
	// No audit middleware -- proxy has no DB.
	middlewares := []func(http.Handler) http.Handler{
		security.SecurityHeaders(p.headersCfg),
		security.BodyLimit(p.bodyLimitCfg),
	}
	if p.connLimiter != nil {
		middlewares = append(middlewares, p.connLimiter.Middleware)
	}
	if p.rateLimiter != nil {
		middlewares = append(middlewares, p.rateLimiter.Middleware)
	}

	handler := security.Chain(middlewares...)(mux)

	server := &http.Server{
		Addr:           p.listenAddr,
		Handler:        handler,
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   120 * time.Second, // SSE streaming needs long writes
		IdleTimeout:    120 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1 MB
	}

	// Graceful shutdown on context cancellation
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	p.logger.Info("MCP proxy starting",
		"listen", p.listenAddr,
		"upstream", p.upstreamURL,
		"log_traffic", p.logTraffic,
	)

	err := server.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

// setCORSOrigin sets the Access-Control-Allow-Origin header if the request's
// Origin matches the configured whitelist. If no origins are configured, no
// CORS header is sent (secure default for non-browser MCP clients).
func (p *Proxy) setCORSOrigin(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	if origin == "" || len(p.corsOrigins) == 0 {
		return
	}
	for _, allowed := range p.corsOrigins {
		if allowed == "*" || strings.EqualFold(allowed, origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			return
		}
	}
}

// checkMaintenanceMCP checks if maintenance mode is active and writes an MCP
// JSON-RPC error response if so. Returns true if the request was handled.
func (p *Proxy) checkMaintenanceMCP(w http.ResponseWriter, body []byte) bool {
	if p.monitor == nil {
		return false
	}
	resp := p.monitor.MaintenanceResponse()
	if resp == nil {
		return false
	}
	var requestID interface{}
	if len(body) > 0 {
		var msg struct {
			ID interface{} `json:"id"`
		}
		if json.Unmarshal(body, &msg) == nil {
			requestID = msg.ID
		}
	}
	resp.WriteMCPError(w, requestID)
	return true
}

// checkMaintenanceREST checks if maintenance mode is active and writes a REST
// error response if so. Returns true if the request was handled.
func (p *Proxy) checkMaintenanceREST(w http.ResponseWriter) bool {
	if p.monitor == nil {
		return false
	}
	resp := p.monitor.MaintenanceResponse()
	if resp == nil {
		return false
	}
	resp.WriteRESTError(w)
	return true
}

// handleStreamableHTTP proxies new Streamable HTTP transport to upstream with
// automatic session recovery. When the upstream server restarts and client sessions
// become invalid, the proxy transparently re-initializes and retries the request.
func (p *Proxy) handleStreamableHTTP(w http.ResponseWriter, r *http.Request) {
	upstreamURL := p.upstreamURL + "/mcp"

	// Buffer POST body for potential retry after session recovery.
	var bodyBytes []byte
	if r.Body != nil && r.Method == http.MethodPost {
		var err error
		bodyBytes, err = io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request", http.StatusBadRequest)
			return
		}
	}

	// Check maintenance mode before forwarding
	if p.checkMaintenanceMCP(w, bodyBytes) {
		return
	}

	clientSessionID := r.Header.Get("Mcp-Session-Id")

	// MCP security checks (validation + per-tool rate limiting)
	if p.mcpSec != nil && r.Method == http.MethodPost && len(bodyBytes) > 0 {
		actorKey := clientSessionID
		if actorKey == "" {
			actorKey = r.RemoteAddr
		}
		if secErr, parsed := p.mcpSec.Check(bodyBytes, actorKey); secErr != nil {
			var reqID interface{}
			if parsed != nil {
				reqID = parsed.ID
			}
			secErr.WriteMCPError(w, reqID)
			return
		}
	}

	// Log request
	if p.logTraffic && r.Method == http.MethodPost && len(bodyBytes) > 0 {
		p.logEvent("client->upstream (streamable)", clientSessionID, string(bodyBytes))
	}

	// Resolve session: if upstream restarted previously, the client's session ID
	// may map to a newer upstream session ID.
	upstreamSessionID := p.resolveStreamSession(clientSessionID)

	// Forward to upstream
	resp, err := p.forwardStreamable(r.Method, upstreamURL, bodyBytes, upstreamSessionID, r.Header)
	if err != nil {
		p.logger.Error("Upstream request failed", "error", err)
		http.Error(w, "Upstream error", http.StatusBadGateway)
		return
	}

	// Session recovery: if upstream returned 404 (session not found) and we had
	// a session, re-initialize with upstream and retry the original request.
	if resp.StatusCode == http.StatusNotFound && clientSessionID != "" && r.Method == http.MethodPost {
		errBody, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()

		if p.logTraffic {
			p.logEvent("upstream->proxy (session expired)", clientSessionID, string(errBody))
		}

		p.logger.Info("Upstream session expired, recovering",
			"client_session", clientSessionID,
			"upstream_session", upstreamSessionID)

		newSessionID, initErr := p.initUpstreamSession("session-recovery")
		if initErr != nil {
			p.logger.Error("Session recovery failed", "error", initErr)
			http.Error(w, "Session recovery failed", http.StatusBadGateway)
			return
		}

		// Update the mapping so future requests with the old session ID
		// are routed to the new upstream session.
		p.streamSessionsMu.Lock()
		p.streamSessions[clientSessionID] = newSessionID
		p.streamSessionsMu.Unlock()

		p.logger.Info("Session recovered",
			"client_session", clientSessionID,
			"new_upstream_session", newSessionID)

		// Retry the original request with the new session.
		resp, err = p.forwardStreamable(r.Method, upstreamURL, bodyBytes, newSessionID, r.Header)
		if err != nil {
			p.logger.Error("Retry after session recovery failed", "error", err)
			http.Error(w, "Upstream error", http.StatusBadGateway)
			return
		}
	}
	defer func() { _ = resp.Body.Close() }()

	// Copy response headers
	for _, h := range []string{"Content-Type", "Mcp-Session-Id", "Cache-Control"} {
		if v := resp.Header.Get(h); v != "" {
			w.Header().Set(h, v)
		}
	}

	// Handle SSE streaming response
	if strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
		w.Header().Set("Connection", "keep-alive")
		p.setCORSOrigin(w, r)
		w.WriteHeader(resp.StatusCode)

		flusher, ok := w.(http.Flusher)
		if !ok {
			return
		}

		sessionTag := resp.Header.Get("Mcp-Session-Id")
		buf := make([]byte, 4096)
		for {
			n, err := resp.Body.Read(buf)
			if n > 0 {
				_, _ = w.Write(buf[:n])
				flusher.Flush()
				if p.logTraffic {
					p.logEvent("upstream->client (streamable)", sessionTag, string(buf[:n]))
				}
			}
			if err != nil {
				break
			}
		}
		return
	}

	// Regular JSON response
	w.WriteHeader(resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	_, _ = w.Write(body)

	if p.logTraffic && len(body) > 0 {
		p.logEvent("upstream->client (streamable)", resp.Header.Get("Mcp-Session-Id"), string(body))
	}
}

// forwardStreamable sends a request to the upstream Streamable HTTP endpoint.
// It copies relevant client headers and sets the upstream session ID.
func (p *Proxy) forwardStreamable(method, url string, body []byte, sessionID string, clientHeaders http.Header) (*http.Response, error) {
	var bodyReader io.Reader
	if len(body) > 0 {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, err
	}

	// Copy relevant headers from the client request.
	for _, h := range []string{"Content-Type", "Accept", "Authorization", "Last-Event-ID"} {
		if v := clientHeaders.Get(h); v != "" {
			req.Header.Set(h, v)
		}
	}

	// Set the upstream session ID (may differ from client's after recovery).
	if sessionID != "" {
		req.Header.Set("Mcp-Session-Id", sessionID)
	}

	return p.sseHTTPClient.Do(req)
}

// handleSSE handles SSE connections from old-transport clients.
// It creates a client session and sends the endpoint event.
// The actual communication happens via handleMessage.
func (p *Proxy) handleSSE(w http.ResponseWriter, r *http.Request) {
	// Check maintenance mode before accepting new SSE connections
	if p.checkMaintenanceMCP(w, nil) {
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	p.setCORSOrigin(w, r)

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	// Create client
	clientID := fmt.Sprintf("client_%d", p.clientIDSeq.Add(1))
	client := &proxyClient{
		id:      clientID,
		writer:  w,
		flusher: flusher,
		done:    make(chan struct{}),
	}

	p.clientsMu.Lock()
	p.clients[clientID] = client
	p.clientsMu.Unlock()

	p.logger.Info("Old transport client connected", "client_id", clientID, "remote", r.RemoteAddr)

	// Send endpoint event to client (old transport expects this)
	endpointURL := fmt.Sprintf("http://%s/mcp/message?client_id=%s", r.Host, clientID)
	_, _ = fmt.Fprintf(w, "event: endpoint\ndata: %s\n\n", endpointURL)
	flusher.Flush()

	// Wait for client disconnect
	<-r.Context().Done()

	p.clientsMu.Lock()
	delete(p.clients, clientID)
	p.clientsMu.Unlock()

	close(client.done)
	p.logger.Info("Old transport client disconnected", "client_id", clientID)
}

// initUpstreamSession creates a new initialized session with the upstream server.
// It sends the initialize request followed by notifications/initialized, returning
// the session ID assigned by the upstream. This is the shared init logic used by
// both transport paths (SSE auto-init and Streamable HTTP session recovery).
func (p *Proxy) initUpstreamSession(logTag string) (string, error) {
	upstreamURL := p.upstreamURL + "/mcp"

	// Step 1: Send initialize request to establish upstream session.
	initBody := `{"jsonrpc":"2.0","id":-1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"taskschmiede-proxy","version":"1.0.0"}}}`

	if p.logTraffic {
		p.logEvent("proxy->upstream ("+logTag+")", "", initBody)
	}

	req, err := http.NewRequest("POST", upstreamURL, strings.NewReader(initBody))
	if err != nil {
		return "", fmt.Errorf("create init request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("upstream init request: %w", err)
	}
	respBody, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	if p.logTraffic {
		p.logEvent("upstream->proxy ("+logTag+")", "", string(respBody))
	}

	sessionID := resp.Header.Get("Mcp-Session-Id")
	if sessionID == "" {
		return "", fmt.Errorf("upstream did not return session ID")
	}

	// Step 2: Send notifications/initialized to complete the handshake.
	notifyBody := `{"jsonrpc":"2.0","method":"notifications/initialized"}`

	if p.logTraffic {
		p.logEvent("proxy->upstream ("+logTag+")", "", notifyBody)
	}

	req2, err := http.NewRequest("POST", upstreamURL, strings.NewReader(notifyBody))
	if err != nil {
		return "", fmt.Errorf("create init notification: %w", err)
	}
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Accept", "application/json, text/event-stream")
	req2.Header.Set("Mcp-Session-Id", sessionID)

	resp2, err := p.httpClient.Do(req2)
	if err != nil {
		return "", fmt.Errorf("upstream init notification: %w", err)
	}
	resp2Body, _ := io.ReadAll(resp2.Body)
	_ = resp2.Body.Close()

	if p.logTraffic && len(resp2Body) > 0 {
		p.logEvent("upstream->proxy ("+logTag+")", "", string(resp2Body))
	}

	return sessionID, nil
}

// ensureUpstreamInitialized checks if an old-transport client has an active
// upstream session. If not, it creates one via initUpstreamSession. This handles
// clients that reconnect to the proxy without re-sending the initialization sequence.
func (p *Proxy) ensureUpstreamInitialized(client *proxyClient) error {
	client.upstreamMu.Lock()
	defer client.upstreamMu.Unlock()

	if client.initialized {
		return nil
	}

	p.logger.Info("Auto-initializing upstream session", "client_id", client.id)

	sessionID, err := p.initUpstreamSession("auto-init")
	if err != nil {
		return err
	}

	client.upstreamSessionID = sessionID
	client.initialized = true

	p.logger.Info("Auto-initialized upstream session", "client_id", client.id, "session_id", sessionID)
	return nil
}

// resolveStreamSession maps a client-facing session ID to the current upstream
// session ID. If no mapping exists, the client session ID is used directly
// (passthrough for sessions that haven't needed recovery).
func (p *Proxy) resolveStreamSession(clientSessionID string) string {
	if clientSessionID == "" {
		return ""
	}
	p.streamSessionsMu.RLock()
	defer p.streamSessionsMu.RUnlock()
	if upstream, ok := p.streamSessions[clientSessionID]; ok {
		return upstream
	}
	return clientSessionID
}

// handleMessage handles messages from old-transport clients.
// It translates to new Streamable HTTP transport when talking to upstream.
func (p *Proxy) handleMessage(w http.ResponseWriter, r *http.Request) {
	clientID := r.URL.Query().Get("client_id")

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}

	// Check maintenance mode before forwarding
	if p.checkMaintenanceMCP(w, body) {
		return
	}

	// MCP security checks (validation + per-tool rate limiting)
	if p.mcpSec != nil {
		actorKey := clientID
		if actorKey == "" {
			actorKey = r.RemoteAddr
		}
		if secErr, parsed := p.mcpSec.Check(body, actorKey); secErr != nil {
			var reqID interface{}
			if parsed != nil {
				reqID = parsed.ID
			}
			secErr.WriteMCPError(w, reqID)
			return
		}
	}

	// Log request
	if p.logTraffic {
		p.logEvent("client->upstream (old)", clientID, string(body))
	}

	// Find client
	p.clientsMu.RLock()
	client, ok := p.clients[clientID]
	p.clientsMu.RUnlock()

	if !ok {
		http.Error(w, "Client not found", http.StatusNotFound)
		return
	}

	// Parse method from JSON-RPC message
	var rpcMsg struct {
		Method string `json:"method"`
	}
	_ = json.Unmarshal(body, &rpcMsg)
	isInitialize := rpcMsg.Method == "initialize"
	isInitNotification := rpcMsg.Method == "notifications/initialized"

	if isInitialize {
		client.upstreamMu.Lock()
		client.lastInitializeBody = make([]byte, len(body))
		copy(client.lastInitializeBody, body)
		client.upstreamMu.Unlock()
	}

	// Auto-initialize upstream session for clients that skipped initialization.
	// This handles reconnecting clients (e.g., Claude Code, Opencode) that open a
	// new SSE connection but don't re-send initialize because they consider
	// themselves already initialized from a previous connection.
	if !isInitialize && !isInitNotification {
		if err := p.ensureUpstreamInitialized(client); err != nil {
			p.logger.Error("Auto-initialization failed", "error", err, "client_id", clientID)
			http.Error(w, "Upstream initialization failed", http.StatusBadGateway)
			return
		}
	}

	// Build upstream request (new Streamable HTTP transport)
	upstreamURL := p.upstreamURL + "/mcp"
	req, err := http.NewRequest("POST", upstreamURL, bytes.NewReader(body))
	if err != nil {
		http.Error(w, "Failed to create request", http.StatusInternalServerError)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	// Forward auth header
	if auth := r.Header.Get("Authorization"); auth != "" {
		req.Header.Set("Authorization", auth)
	}

	// Add session ID for non-initialize requests
	client.upstreamMu.Lock()
	sessionID := client.upstreamSessionID
	client.upstreamMu.Unlock()

	if sessionID != "" {
		req.Header.Set("Mcp-Session-Id", sessionID)
	}

	// Send to upstream
	resp, err := p.httpClient.Do(req)
	if err != nil {
		p.logger.Error("Failed to forward message", "error", err, "client_id", clientID)
		http.Error(w, "Upstream error", http.StatusBadGateway)
		return
	}

	// Session recovery: if upstream returned 404 (session not found) and this is
	// not an initialize request, re-initialize with upstream and retry.
	if resp.StatusCode == http.StatusNotFound && !isInitialize && !isInitNotification {
		errBody, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()

		if p.logTraffic {
			p.logEvent("upstream->proxy (old, session expired)", clientID, string(errBody))
		}

		p.logger.Info("Upstream session expired (old transport), recovering",
			"client_id", clientID,
			"old_session", sessionID)

		newSessionID, initErr := p.initUpstreamSession("old-session-recovery")
		if initErr != nil {
			p.logger.Error("Session recovery failed (old transport)", "error", initErr, "client_id", clientID)
			http.Error(w, "Upstream session recovery failed", http.StatusBadGateway)
			return
		}

		client.upstreamMu.Lock()
		client.upstreamSessionID = newSessionID
		client.initialized = true
		client.upstreamMu.Unlock()

		p.logger.Info("Session recovered (old transport)",
			"client_id", clientID,
			"new_session", newSessionID)

		// Retry the original request with the new session.
		retryReq, err := http.NewRequest("POST", upstreamURL, bytes.NewReader(body))
		if err != nil {
			http.Error(w, "Failed to create retry request", http.StatusInternalServerError)
			return
		}
		retryReq.Header.Set("Content-Type", "application/json")
		retryReq.Header.Set("Accept", "application/json, text/event-stream")
		if auth := r.Header.Get("Authorization"); auth != "" {
			retryReq.Header.Set("Authorization", auth)
		}
		retryReq.Header.Set("Mcp-Session-Id", newSessionID)

		resp, err = p.httpClient.Do(retryReq)
		if err != nil {
			p.logger.Error("Retry after session recovery failed (old transport)", "error", err, "client_id", clientID)
			http.Error(w, "Upstream error", http.StatusBadGateway)
			return
		}
	}
	defer func() { _ = resp.Body.Close() }()

	// Capture session ID from initialize response
	if isInitialize {
		if newSessionID := resp.Header.Get("Mcp-Session-Id"); newSessionID != "" {
			client.upstreamMu.Lock()
			client.upstreamSessionID = newSessionID
			client.initialized = true
			client.upstreamMu.Unlock()
			p.logger.Info("Upstream session created", "client_id", clientID, "session_id", newSessionID)
		}
	}

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "Failed to read response", http.StatusBadGateway)
		return
	}

	// Log response
	if p.logTraffic {
		p.logEvent("upstream->proxy (old)", clientID, string(respBody))
	}

	// Extract JSON from SSE response if needed
	// Upstream may return text/event-stream with "event: message\ndata: {...}"
	jsonData := respBody
	if strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
		jsonData = extractSSEData(respBody)
	}

	// Old transport: POST returns 202 Accepted, response comes via SSE
	w.WriteHeader(http.StatusAccepted)

	// Push response to client's SSE connection
	if len(jsonData) > 0 && client.writer != nil && client.flusher != nil {
		sseEvent := fmt.Sprintf("event: message\ndata: %s\n\n", string(jsonData))
		_, _ = fmt.Fprint(client.writer, sseEvent)
		client.flusher.Flush()

		if p.logTraffic {
			p.logEvent("proxy->client (old SSE)", clientID, sseEvent)
		}
	}
}

// extractSSEData extracts the JSON data from an SSE response body.
// SSE format: "event: message\ndata: {...}\n\n"
func extractSSEData(body []byte) []byte {
	lines := strings.Split(string(body), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "data: ") {
			return []byte(strings.TrimPrefix(line, "data: "))
		}
	}
	return body // fallback: return as-is
}

// handleHealth proxies health check to upstream.
func (p *Proxy) handleHealth(w http.ResponseWriter, r *http.Request) {
	resp, err := p.httpClient.Get(p.upstreamURL + "/mcp/health")
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":   "unhealthy",
			"upstream": "disconnected",
			"error":    err.Error(),
		})
		return
	}
	defer func() { _ = resp.Body.Close() }()

	// Forward upstream response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

// handleREST forwards REST API requests to the upstream app server.
func (p *Proxy) handleREST(w http.ResponseWriter, r *http.Request) {
	// Check maintenance mode before forwarding
	if p.checkMaintenanceREST(w) {
		return
	}

	// Inject REST API deprecation headers (if configured)
	if p.mcpSec != nil {
		p.mcpSec.injectDeprecationHeader(w, r.URL.Path)
	}

	if p.logTraffic {
		p.logEvent("client->upstream (REST)", r.RemoteAddr,
			fmt.Sprintf("%s %s", r.Method, r.URL.Path))
	}

	// Use a response recorder to log the status code.
	rw := &restResponseWriter{ResponseWriter: w}
	p.restProxy.ServeHTTP(rw, r)

	if p.logTraffic {
		p.logEvent("upstream->client (REST)", r.RemoteAddr,
			fmt.Sprintf("%d %s %s", rw.statusCode, r.Method, r.URL.Path))
	}
}

// restResponseWriter wraps http.ResponseWriter to capture the status code for logging.
type restResponseWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

// WriteHeader captures the status code and delegates to the underlying writer.
func (rw *restResponseWriter) WriteHeader(code int) {
	if !rw.written {
		rw.statusCode = code
		rw.written = true
	}
	rw.ResponseWriter.WriteHeader(code)
}

// Write delegates to the underlying writer and records the implicit 200 status.
func (rw *restResponseWriter) Write(b []byte) (int, error) {
	if !rw.written {
		rw.statusCode = http.StatusOK
		rw.written = true
	}
	return rw.ResponseWriter.Write(b)
}

// Unwrap returns the underlying ResponseWriter for http.Flusher compatibility.
func (rw *restResponseWriter) Unwrap() http.ResponseWriter {
	return rw.ResponseWriter
}

// handleProxyHealth returns the proxy's own health status.
func (p *Proxy) handleProxyHealth(w http.ResponseWriter, r *http.Request) {
	p.clientsMu.RLock()
	clientCount := len(p.clients)
	p.clientsMu.RUnlock()

	// Check MCP upstream
	var mcpStatus string
	resp, err := p.httpClient.Get(p.upstreamURL + "/mcp/health")
	if err != nil {
		mcpStatus = "disconnected"
	} else {
		_ = resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			mcpStatus = "connected"
		} else {
			mcpStatus = "unhealthy"
		}
	}

	// Check REST upstream
	var restStatus string
	resp, err = p.httpClient.Get(p.upstreamURL + "/api/v1/kpi/current")
	if err != nil {
		restStatus = "disconnected"
	} else {
		_ = resp.Body.Close()
		// Any response (even 401/403) means REST is reachable
		restStatus = "connected"
	}

	result := map[string]interface{}{
		"status":            "healthy",
		"service":           "taskschmiede-proxy",
		"mcp_clients":       clientCount,
		"upstream_url":      p.upstreamURL,
		"mcp_upstream":      mcpStatus,
		"rest_upstream":     restStatus,
		"log_traffic":       p.logTraffic,
		"backend_transport": "streamable-http",
	}

	if p.connLimiter != nil {
		result["active_connections"] = p.connLimiter.GlobalCount()
	}

	// Include upstream monitor state when maintenance mode is configured
	if p.monitor != nil {
		p.monitor.mu.RLock()
		result["upstream_state"] = p.monitor.state.String()
		result["state_changed_at"] = p.monitor.stateChangedAt.Format(time.RFC3339)
		result["last_health_check"] = p.monitor.lastHealthCheck.Format(time.RFC3339)
		result["last_health_status"] = p.monitor.lastHealthStatus
		if p.monitor.maintenance != nil {
			result["maintenance"] = p.monitor.maintenance
		}
		p.monitor.mu.RUnlock()
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

// handleVersions returns the API version manifest.
func (p *Proxy) handleVersions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if p.mcpSec != nil {
		p.mcpSec.writeVersionsResponse(w)
		return
	}
	// Default response when MCP security is not configured
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(defaultVersionManifest())
}

// sensitiveFieldPattern matches JSON keys that contain sensitive data.
// Matches: "password", "new_password", "token", "invitation_token", "org_token", "secret", "code"
var sensitiveFieldPattern = regexp.MustCompile(
	`(?i)"(password|new_password|token|invitation_token|org_token|secret|code)"\s*:\s*"([^"]*)"`,
)

// redactSensitiveFields replaces sensitive values in JSON-RPC traffic data.
// Passwords and secrets are fully masked. Tokens show first/last 4 chars if long enough.
func redactSensitiveFields(data string) string {
	return sensitiveFieldPattern.ReplaceAllStringFunc(data, func(match string) string {
		parts := sensitiveFieldPattern.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}
		key := parts[1]
		value := parts[2]
		keyLower := strings.ToLower(key)

		// Tokens: show first/last 4 chars if >= 12 chars, otherwise mask entirely
		if strings.Contains(keyLower, "token") {
			if len(value) >= 12 {
				return fmt.Sprintf(`"%s": "%s...%s"`, key, value[:4], value[len(value)-4:])
			}
			return fmt.Sprintf(`"%s": "********"`, key)
		}

		// Passwords, secrets, codes: mask entirely
		return fmt.Sprintf(`"%s": "********"`, key)
	})
}

// logEvent logs an MCP event to the traffic log file and slog.
func (p *Proxy) logEvent(direction, clientID, data string) {
	// Redact sensitive fields before logging
	data = redactSensitiveFields(data)

	// Pretty print JSON if possible
	var prettyData string
	var jsonObj interface{}
	if err := json.Unmarshal([]byte(data), &jsonObj); err == nil {
		if pretty, err := json.MarshalIndent(jsonObj, "", "  "); err == nil {
			prettyData = string(pretty)
		} else {
			prettyData = data
		}
	} else {
		prettyData = strings.TrimSpace(data)
	}

	// Write to traffic log file
	if p.trafficLog != nil {
		timestamp := time.Now().UTC().Format("2006-01-02 15:04:05.000")
		p.trafficLogMu.Lock()
		_, _ = fmt.Fprintf(p.trafficLog, "%s [%s] %s\n%s\n\n", timestamp, clientID, direction, prettyData)
		p.trafficLogMu.Unlock()
	}

	// Also log via slog for console output
	p.logger.Debug("MCP traffic",
		"direction", direction,
		"client_id", clientID,
		"data", prettyData,
	)
}
