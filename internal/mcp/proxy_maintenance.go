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
	"net"
	"net/http"
	"sync"
	"time"
)

// UpstreamState represents the proxy's view of the upstream server's availability.
type UpstreamState int

const (
	// StateHealthy means the upstream is reachable and serving requests.
	StateHealthy UpstreamState = iota
	// StateMaintenance means an admin explicitly activated maintenance mode.
	StateMaintenance
	// StateError means the upstream is unreachable (detected automatically).
	StateError
)

// String returns the human-readable name for the upstream state.
func (s UpstreamState) String() string {
	switch s {
	case StateHealthy:
		return "healthy"
	case StateMaintenance:
		return "maintenance"
	case StateError:
		return "error"
	default:
		return "unknown"
	}
}

// MaintenanceConfig holds configuration for the maintenance mode subsystem.
type MaintenanceConfig struct {
	// ManagementListen is the address for the localhost-only management API.
	// Default: "127.0.0.1:9010".
	ManagementListen string

	// ManagementAPIKey is required for write operations on the management API.
	// If empty, write operations are rejected.
	ManagementAPIKey string

	// AutoDetect enables automatic error mode when upstream is unreachable.
	AutoDetect bool

	// AutoDetectGrace is how long the upstream must be unreachable before
	// entering error mode. Prevents flapping on brief network hiccups.
	// Default: 10s.
	AutoDetectGrace time.Duration

	// HealthCheckInterval is how often to poll the upstream health endpoint.
	// Default: 5s.
	HealthCheckInterval time.Duration

	// UpstreamTimeout is the timeout for non-SSE upstream requests.
	// Default: 30s.
	UpstreamTimeout time.Duration

	// UpstreamTimeoutSSE is the timeout for SSE streaming requests.
	// Default: 300s.
	UpstreamTimeoutSSE time.Duration
}

// DefaultMaintenanceConfig returns sensible defaults.
func DefaultMaintenanceConfig() MaintenanceConfig {
	return MaintenanceConfig{
		ManagementListen:    "127.0.0.1:9010",
		AutoDetect:          true,
		AutoDetectGrace:     10 * time.Second,
		HealthCheckInterval: 5 * time.Second,
		UpstreamTimeout:     30 * time.Second,
		UpstreamTimeoutSSE:  300 * time.Second,
	}
}

// maintenanceInfo holds the metadata for an active maintenance window.
type maintenanceInfo struct {
	Reason          string     `json:"reason,omitempty"`
	EstimatedReturn *time.Time `json:"estimated_return,omitempty"`
	ActivatedAt     time.Time  `json:"activated_at"`
}

// upstreamMonitor manages the upstream state machine, health checking,
// and the management API.
type upstreamMonitor struct {
	logger      *slog.Logger
	upstreamURL string
	cfg         MaintenanceConfig

	// State machine (mutex-protected)
	mu               sync.RWMutex
	state            UpstreamState
	maintenance      *maintenanceInfo // non-nil when state == StateMaintenance
	stateChangedAt   time.Time
	graceDeadline    time.Time // when grace period expires (zero = not in grace)
	lastHealthCheck  time.Time
	lastHealthStatus bool // true = healthy

	// Management API server
	mgmtServer *http.Server

	// Notification callback (set by proxy)
	onStateChange func(from, to UpstreamState, info string)
}

// newUpstreamMonitor creates a new upstream monitor.
func newUpstreamMonitor(logger *slog.Logger, upstreamURL string, cfg MaintenanceConfig) *upstreamMonitor {
	return &upstreamMonitor{
		logger:         logger,
		upstreamURL:    upstreamURL,
		cfg:            cfg,
		state:          StateHealthy,
		stateChangedAt: time.Now().UTC(),
	}
}

// Start begins the health check loop and management API server.
// It blocks until ctx is cancelled.
func (m *upstreamMonitor) Start(ctx context.Context) error {
	// Start health check loop
	go m.healthCheckLoop(ctx)

	// Start management API
	if m.cfg.ManagementListen != "" {
		go m.startManagementAPI(ctx)
	}

	<-ctx.Done()
	if m.mgmtServer != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = m.mgmtServer.Shutdown(shutdownCtx)
	}
	return nil
}

// State returns the current upstream state.
func (m *upstreamMonitor) State() UpstreamState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state
}

// IsAvailable returns true if requests should be forwarded to upstream.
func (m *upstreamMonitor) IsAvailable() bool {
	return m.State() == StateHealthy
}

// MaintenanceResponse returns the appropriate error response for the current state.
// Returns nil if the upstream is healthy (no error response needed).
func (m *upstreamMonitor) MaintenanceResponse() *maintenanceResponse {
	m.mu.RLock()
	defer m.mu.RUnlock()

	switch m.state {
	case StateHealthy:
		return nil
	case StateMaintenance:
		resp := &maintenanceResponse{
			Type:              "maintenance",
			Message:           "System maintenance in progress.",
			RetryAfterSeconds: 120,
		}
		if m.maintenance != nil {
			if m.maintenance.Reason != "" {
				resp.Reason = m.maintenance.Reason
			}
			if m.maintenance.EstimatedReturn != nil {
				eta := m.maintenance.EstimatedReturn.UTC().Format(time.RFC3339)
				resp.EstimatedReturn = eta
				resp.Message = fmt.Sprintf("System maintenance in progress. Estimated return: %s", eta)
				// Calculate retry based on ETA
				untilReturn := time.Until(*m.maintenance.EstimatedReturn)
				if untilReturn > 0 && untilReturn < 10*time.Minute {
					resp.RetryAfterSeconds = int(untilReturn.Seconds()) + 10
				}
			}
		}
		return resp
	case StateError:
		return &maintenanceResponse{
			Type:              "error",
			Message:           "System temporarily unavailable. Our team has been notified.",
			RetryAfterSeconds: 30,
		}
	default:
		return nil
	}
}

// maintenanceResponse holds the data for a maintenance/error response.
type maintenanceResponse struct {
	Type              string `json:"type"`
	Message           string `json:"message"`
	Reason            string `json:"reason,omitempty"`
	EstimatedReturn   string `json:"estimated_return,omitempty"`
	RetryAfterSeconds int    `json:"retry_after_seconds"`
}

// WriteMCPError writes a JSON-RPC error response for maintenance/error state.
func (mr *maintenanceResponse) WriteMCPError(w http.ResponseWriter, requestID interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusServiceUnavailable)

	errorData := map[string]interface{}{
		"type":                mr.Type,
		"retry_after_seconds": mr.RetryAfterSeconds,
	}
	if mr.Reason != "" {
		errorData["reason"] = mr.Reason
	}
	if mr.EstimatedReturn != "" {
		errorData["estimated_return"] = mr.EstimatedReturn
	}

	// Use the request ID if available, otherwise use null
	id := requestID
	if id == nil {
		id = 1
	}

	resp := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"error": map[string]interface{}{
			"code":    -32000,
			"message": mr.Message,
			"data":    errorData,
		},
	}
	_ = json.NewEncoder(w).Encode(resp)
}

// WriteRESTError writes a REST API error response for maintenance/error state.
func (mr *maintenanceResponse) WriteRESTError(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Retry-After", fmt.Sprintf("%d", mr.RetryAfterSeconds))
	w.WriteHeader(http.StatusServiceUnavailable)

	resp := map[string]interface{}{
		"error":               mr.Type,
		"message":             mr.Message,
		"retry_after_seconds": mr.RetryAfterSeconds,
	}
	if mr.Reason != "" {
		resp["reason"] = mr.Reason
	}
	if mr.EstimatedReturn != "" {
		resp["estimated_return"] = mr.EstimatedReturn
	}
	_ = json.NewEncoder(w).Encode(resp)
}

// --- State transitions ---

// transitionTo atomically changes the upstream state and fires the notification callback.
func (m *upstreamMonitor) transitionTo(newState UpstreamState, info string) {
	m.mu.Lock()
	oldState := m.state
	if oldState == newState {
		m.mu.Unlock()
		return
	}
	m.state = newState
	m.stateChangedAt = time.Now().UTC()
	m.graceDeadline = time.Time{} // clear grace
	m.mu.Unlock()

	m.logger.Info("Upstream state changed",
		"from", oldState.String(),
		"to", newState.String(),
		"info", info,
	)

	if m.onStateChange != nil {
		m.onStateChange(oldState, newState, info)
	}
}

// activateMaintenance enters maintenance mode with an optional reason and ETA.
func (m *upstreamMonitor) activateMaintenance(reason string, estimatedReturn *time.Time) {
	m.mu.Lock()
	m.maintenance = &maintenanceInfo{
		Reason:          reason,
		EstimatedReturn: estimatedReturn,
		ActivatedAt:     time.Now().UTC(),
	}
	m.mu.Unlock()
	m.transitionTo(StateMaintenance, reason)
}

// deactivateMaintenance exits maintenance mode and transitions to healthy.
func (m *upstreamMonitor) deactivateMaintenance() {
	m.mu.Lock()
	m.maintenance = nil
	m.mu.Unlock()
	m.transitionTo(StateHealthy, "maintenance deactivated")
}

// --- Health check loop ---

// healthCheckLoop periodically polls the upstream health endpoint.
func (m *upstreamMonitor) healthCheckLoop(ctx context.Context) {
	interval := m.cfg.HealthCheckInterval
	if interval <= 0 {
		interval = 5 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.performHealthCheck()
		}
	}
}

// performHealthCheck checks upstream health and manages state transitions.
func (m *upstreamMonitor) performHealthCheck() {
	healthy := m.checkUpstreamHealth()

	m.mu.Lock()
	m.lastHealthCheck = time.Now().UTC()
	m.lastHealthStatus = healthy
	currentState := m.state
	m.mu.Unlock()

	switch {
	case healthy && currentState == StateError:
		// Upstream recovered from error
		m.transitionTo(StateHealthy, "upstream recovered")

	case healthy && currentState == StateMaintenance:
		// Upstream is back after planned maintenance -- auto-recover
		m.transitionTo(StateHealthy, "upstream returned after maintenance")

	case !healthy && currentState == StateHealthy && m.cfg.AutoDetect:
		// Upstream just went down -- start grace period
		m.mu.Lock()
		if m.graceDeadline.IsZero() {
			m.graceDeadline = time.Now().UTC().Add(m.cfg.AutoDetectGrace)
			m.logger.Warn("Upstream health check failed, starting grace period",
				"grace_until", m.graceDeadline.Format(time.RFC3339))
		} else if time.Now().UTC().After(m.graceDeadline) {
			// Grace period expired -- enter error mode
			m.mu.Unlock()
			m.transitionTo(StateError, "upstream unreachable after grace period")
			return
		}
		m.mu.Unlock()

	case healthy && currentState == StateHealthy:
		// Clear any grace period if we recovered before it expired
		m.mu.Lock()
		m.graceDeadline = time.Time{}
		m.mu.Unlock()
	}
}

// checkUpstreamHealth returns true if the upstream health endpoint responds with 200.
func (m *upstreamMonitor) checkUpstreamHealth() bool {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(m.upstreamURL + "/mcp/health")
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// --- Management API ---

// startManagementAPI starts the localhost-only management HTTP server.
func (m *upstreamMonitor) startManagementAPI(ctx context.Context) {
	mux := http.NewServeMux()
	mux.HandleFunc("/proxy/health", m.handleMgmtHealth)
	mux.HandleFunc("/proxy/maintenance", m.handleMgmtMaintenance)

	m.mgmtServer = &http.Server{
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	ln, err := net.Listen("tcp", m.cfg.ManagementListen)
	if err != nil {
		m.logger.Error("Failed to start management API", "error", err, "listen", m.cfg.ManagementListen)
		return
	}

	m.logger.Info("Management API started", "listen", m.cfg.ManagementListen)
	if err := m.mgmtServer.Serve(ln); err != nil && err != http.ErrServerClosed {
		m.logger.Error("Management API error", "error", err)
	}
}

// handleMgmtHealth returns the management API health status.
func (m *upstreamMonitor) handleMgmtHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	m.mu.RLock()
	state := m.state
	changedAt := m.stateChangedAt
	lastCheck := m.lastHealthCheck
	lastStatus := m.lastHealthStatus
	var maint *maintenanceInfo
	if m.maintenance != nil {
		cpy := *m.maintenance
		maint = &cpy
	}
	m.mu.RUnlock()

	result := map[string]interface{}{
		"service":            "taskschmiede-proxy",
		"status":             "healthy",
		"upstream_url":       m.upstreamURL,
		"upstream_state":     state.String(),
		"state_changed_at":   changedAt.Format(time.RFC3339),
		"last_health_check":  lastCheck.Format(time.RFC3339),
		"last_health_status": lastStatus,
	}

	if maint != nil {
		result["maintenance"] = maint
	}

	// Duration in current state
	result["state_duration_seconds"] = int(time.Since(changedAt).Seconds())

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

// handleMgmtMaintenance dispatches GET and POST for the maintenance endpoint.
func (m *upstreamMonitor) handleMgmtMaintenance(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		m.handleMgmtMaintenanceGet(w, r)
	case http.MethodPost:
		m.handleMgmtMaintenancePost(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleMgmtMaintenanceGet returns the current maintenance state as JSON.
func (m *upstreamMonitor) handleMgmtMaintenanceGet(w http.ResponseWriter, _ *http.Request) {
	m.mu.RLock()
	state := m.state
	var maint *maintenanceInfo
	if m.maintenance != nil {
		cpy := *m.maintenance
		maint = &cpy
	}
	m.mu.RUnlock()

	result := map[string]interface{}{
		"state": state.String(),
	}
	if maint != nil {
		result["maintenance"] = maint
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

// handleMgmtMaintenancePost activates or deactivates maintenance mode.
func (m *upstreamMonitor) handleMgmtMaintenancePost(w http.ResponseWriter, r *http.Request) {
	// Verify API key
	if m.cfg.ManagementAPIKey != "" {
		auth := r.Header.Get("Authorization")
		expected := "Bearer " + m.cfg.ManagementAPIKey
		if auth != expected {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
	}

	var req struct {
		Enabled         bool   `json:"enabled"`
		Reason          string `json:"reason"`
		EstimatedReturn string `json:"estimated_return"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Enabled {
		var eta *time.Time
		if req.EstimatedReturn != "" {
			t, err := time.Parse(time.RFC3339, req.EstimatedReturn)
			if err != nil {
				http.Error(w, "Invalid estimated_return format (expected RFC3339)", http.StatusBadRequest)
				return
			}
			utc := t.UTC()
			eta = &utc
		}
		m.activateMaintenance(req.Reason, eta)
		m.logger.Info("Maintenance mode activated via management API",
			"reason", req.Reason,
			"estimated_return", req.EstimatedReturn)
	} else {
		m.deactivateMaintenance()
		m.logger.Info("Maintenance mode deactivated via management API")
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"state":   m.State().String(),
		"success": true,
	})
}
