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
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/QuestFinTech/taskschmiede/internal/storage"
)

// ErrorClass represents the classification of a tool call result for monitoring.
type ErrorClass int

const (
	ClassSuccess ErrorClass = iota
	ClassClient             // counted in success rate denominator
	ClassAuth               // excluded from rate (infrastructure)
	ClassServer             // excluded from rate (not agent's fault)
)

// MetricTracker records per-session tool call metrics for agent behavioral
// monitoring. It lazily creates session metric rows on first production tool
// call and classifies each result to increment the appropriate counter.
type MetricTracker struct {
	db     *storage.DB
	logger *slog.Logger

	// sessions maps MCP session IDs to metric row IDs.
	sessions map[string]string
	mu       sync.Mutex

	// recentDeletions tracks recently deleted entities for the 404 grace period.
	// Key: "entityType:entityID", value: deletion timestamp.
	recentDeletions   map[string]time.Time
	recentDeletionsMu sync.Mutex
}

const deletionGracePeriod = 60 * time.Second

// NewMetricTracker creates a new metric tracker.
func NewMetricTracker(db *storage.DB, logger *slog.Logger) *MetricTracker {
	return &MetricTracker{
		db:              db,
		logger:          logger,
		sessions:        make(map[string]string),
		recentDeletions: make(map[string]time.Time),
	}
}

// Record classifies a tool call result and increments the appropriate counter
// for the agent's session metric.
func (t *MetricTracker) Record(userID, sessionID, toolName string, result *gomcp.CallToolResult, handlerErr error) {
	if sessionID == "" {
		return
	}

	class := classifyResult(result, handlerErr, t)

	metricID, err := t.getOrCreateMetricID(userID, sessionID)
	if err != nil {
		t.logger.Warn("failed to get/create session metric", "user_id", userID, "error", err)
		return
	}

	switch class {
	case ClassSuccess:
		err = t.db.IncrementMetricSuccess(metricID)
	case ClassClient:
		err = t.db.IncrementMetricClientError(metricID)
	case ClassAuth:
		err = t.db.IncrementMetricAuthError(metricID)
	case ClassServer:
		err = t.db.IncrementMetricServerError(metricID)
	}
	if err != nil {
		t.logger.Warn("failed to increment metric", "metric_id", metricID, "class", class, "error", err)
	}
}

// RecordDeletion records that an entity was just deleted, for the 404 grace period.
func (t *MetricTracker) RecordDeletion(entityType, entityID string) {
	t.recentDeletionsMu.Lock()
	t.recentDeletions[entityType+":"+entityID] = storage.UTCNow()
	t.recentDeletionsMu.Unlock()
}

// CleanExpiredDeletions removes deletion entries older than the grace period.
func (t *MetricTracker) CleanExpiredDeletions() {
	t.recentDeletionsMu.Lock()
	defer t.recentDeletionsMu.Unlock()
	cutoff := storage.UTCNow().Add(-deletionGracePeriod)
	for key, ts := range t.recentDeletions {
		if ts.Before(cutoff) {
			delete(t.recentDeletions, key)
		}
	}
}

// isRecentlyDeleted checks if an entity was deleted within the grace period.
func (t *MetricTracker) isRecentlyDeleted(entityType, entityID string) bool {
	t.recentDeletionsMu.Lock()
	defer t.recentDeletionsMu.Unlock()
	ts, ok := t.recentDeletions[entityType+":"+entityID]
	if !ok {
		return false
	}
	return storage.UTCNow().Sub(ts) < deletionGracePeriod
}

// EndSession marks the session metric row as ended.
func (t *MetricTracker) EndSession(sessionID string) {
	t.mu.Lock()
	metricID, ok := t.sessions[sessionID]
	if ok {
		delete(t.sessions, sessionID)
	}
	t.mu.Unlock()

	if ok {
		if err := t.db.EndSessionMetric(metricID); err != nil {
			t.logger.Warn("failed to end session metric", "session_id", sessionID, "error", err)
		}
	}
}

// getOrCreateMetricID returns the metric row ID for a session, creating one if needed.
func (t *MetricTracker) getOrCreateMetricID(userID, sessionID string) (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if id, ok := t.sessions[sessionID]; ok {
		return id, nil
	}

	// Check DB for existing row (server restart recovery)
	existing, err := t.db.GetSessionMetricBySession(sessionID)
	if err != nil {
		return "", err
	}
	if existing != nil {
		t.sessions[sessionID] = existing.ID
		return existing.ID, nil
	}

	// Create new
	metric, err := t.db.CreateSessionMetric(userID, sessionID)
	if err != nil {
		return "", err
	}
	t.sessions[sessionID] = metric.ID
	return metric.ID, nil
}

// classifyResult inspects a CallToolResult to determine the error class.
func classifyResult(result *gomcp.CallToolResult, handlerErr error, tracker *MetricTracker) ErrorClass {
	// Go-level error (handler panic, transport issue)
	if handlerErr != nil {
		return ClassServer
	}

	// Nil result shouldn't happen but treat as server error
	if result == nil {
		return ClassServer
	}

	// Success
	if !result.IsError {
		return ClassSuccess
	}

	// Error result: extract the code from the JSON payload
	code := extractErrorCode(result)

	switch code {
	case "unauthorized", "session_expired", "not_authenticated", "token_revoked":
		return ClassAuth
	case "internal_error":
		return ClassServer
	case "not_found":
		// 404 grace period: not counted against the agent if the entity was
		// recently deleted (e.g., agent deletes a task then references it).
		entityType, entityID := extractErrorEntity(result)
		if entityType != "" && entityID != "" && tracker.isRecentlyDeleted(entityType, entityID) {
			return ClassServer
		}
		return ClassClient
	default:
		return ClassClient
	}
}

// extractErrorCode parses the JSON error code from a CallToolResult.
// Expected format: {"error":{"code":"...","message":"..."}}
func extractErrorCode(result *gomcp.CallToolResult) string {
	if len(result.Content) == 0 {
		return ""
	}
	tc, ok := result.Content[0].(*gomcp.TextContent)
	if !ok {
		return ""
	}

	var envelope struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(tc.Text), &envelope); err != nil {
		return ""
	}
	return envelope.Error.Code
}

// extractErrorEntity parses optional entity_type and entity_id from a not_found error.
// Expected format: {"error":{"code":"not_found",...,"entity_type":"task","entity_id":"tsk_123"}}
func extractErrorEntity(result *gomcp.CallToolResult) (string, string) {
	if len(result.Content) == 0 {
		return "", ""
	}
	tc, ok := result.Content[0].(*gomcp.TextContent)
	if !ok {
		return "", ""
	}

	var envelope struct {
		Error struct {
			EntityType string `json:"entity_type"`
			EntityID   string `json:"entity_id"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(tc.Text), &envelope); err != nil {
		return "", ""
	}
	return envelope.Error.EntityType, envelope.Error.EntityID
}
