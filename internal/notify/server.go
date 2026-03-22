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


package notify

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// Server is the HTTP event receiver for the notification service.
// It accepts events at POST /notify/event and provides a health
// endpoint at GET /notify/health.
type Server struct {
	dispatcher *Dispatcher
	authToken  string
	logger     *slog.Logger
	mux        *http.ServeMux
}

// NewServer creates the notification HTTP server.
func NewServer(dispatcher *Dispatcher, authToken string, logger *slog.Logger) *Server {
	s := &Server{
		dispatcher: dispatcher,
		authToken:  authToken,
		logger:     logger,
		mux:        http.NewServeMux(),
	}
	s.mux.HandleFunc("/notify/health", s.handleHealth)
	s.mux.HandleFunc("/notify/event", s.handleEvent)
	s.mux.HandleFunc("/notify/stats", s.handleStats)
	return s
}

// Handler returns the http.Handler for use with http.Server.
func (s *Server) Handler() http.Handler {
	return s.mux
}

// handleHealth responds with service status, configured channels, and delivery stats.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats := DeliveryStats{}
	if s.dispatcher.log != nil {
		stats, _ = s.dispatcher.log.Stats()
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":       "ok",
		"channels":     s.channelList(),
		"events":       stats.TotalEvents,
		"delivered":    stats.Delivered,
		"failed":       stats.Failed,
		"rate_limited": stats.RateLimited,
	})
}

// handleStats returns detailed delivery statistics (requires authentication).
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !s.authenticate(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	stats := DeliveryStats{}
	if s.dispatcher.log != nil {
		stats, _ = s.dispatcher.log.Stats()
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(stats)
}

// handleEvent accepts a ServiceEvent via POST and dispatches it asynchronously.
func (s *Server) handleEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !s.authenticate(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var event ServiceEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		http.Error(w, "invalid JSON payload", http.StatusBadRequest)
		return
	}

	if event.Type == "" {
		http.Error(w, "missing event type", http.StatusBadRequest)
		return
	}

	if event.Timestamp == "" {
		event.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}

	s.logger.Info("Event received",
		"type", event.Type,
		"severity", event.Severity,
		"entity", fmt.Sprintf("%s/%s", event.EntityType, event.EntityID),
	)

	// Dispatch asynchronously so the app server is not blocked.
	go s.dispatcher.Dispatch(&event)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status": "accepted",
	})
}

// authenticate checks the Bearer token against the configured auth token.
func (s *Server) authenticate(r *http.Request) bool {
	if s.authToken == "" {
		return true // no auth configured (dev mode)
	}

	auth := r.Header.Get("Authorization")
	if auth == "" {
		return false
	}

	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return false
	}

	return parts[1] == s.authToken
}

// channelList returns the names of configured dispatch channels.
func (s *Server) channelList() []string {
	var channels []string
	if s.dispatcher.smtp != nil {
		channels = append(channels, "smtp")
	}
	if s.dispatcher.ntfy != nil {
		channels = append(channels, "ntfy")
	}
	if s.dispatcher.webhook != nil {
		channels = append(channels, "webhook")
	}
	if len(channels) == 0 {
		channels = append(channels, "none")
	}
	return channels
}
