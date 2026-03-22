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


// Package security provides security middleware, input validation, audit
// logging, rate limiting, connection limiting, injection detection, and
// content framing for Taskschmiede.
package security

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/QuestFinTech/taskschmiede/internal/storage"
)

// Audit action constants for security and operational events.
const (
	AuditLoginSuccess      = "login_success"
	AuditLoginFailure      = "login_failure"
	AuditTokenCreated      = "token_created"
	AuditTokenRevoked      = "token_revoked"
	AuditPasswordChanged   = "password_changed"
	AuditPasswordReset     = "password_reset_requested"
	AuditRateLimitHit      = "rate_limit_hit"
	AuditPermissionDenied  = "permission_denied"
	AuditSessionCreated    = "session_created"
	AuditSessionExpired    = "session_expired"
	AuditInvitationCreated = "invitation_created"
	AuditInvitationRevoked = "invitation_revoked"
	AuditUserRegistered    = "user_registered"
	AuditUserVerified      = "user_verified"
	AuditRequest           = "request"
	AuditSecurityAlert     = "security_alert"

	// AuditDodOverride records a Definition of Done override.
	AuditDodOverride = "dod_override"

	// Intercom audit events.
	AuditIntercomSend             = "intercom_send"
	AuditIntercomReceive          = "intercom_receive"
	AuditIntercomRejectNoMatch    = "intercom_reject_no_match"
	AuditIntercomRejectMismatch   = "intercom_reject_mismatch"
	AuditIntercomRejectExpired    = "intercom_reject_expired"
	AuditIntercomRejectFlooded    = "intercom_reject_flooded"
	AuditIntercomRejectDuplicate  = "intercom_reject_duplicate"
	AuditIntercomAttachmentNotice = "intercom_attachment_notice"
)

// AuditEntry represents a single audit event to be logged.
type AuditEntry struct {
	Action     string
	ActorID    string
	ActorType  string
	Resource   string
	Method     string
	Endpoint   string
	StatusCode int
	IP         string
	Source     string
	Duration   time.Duration
	Metadata   map[string]interface{}
}

// AuditService handles async audit log persistence.
type AuditService struct {
	db     *storage.DB
	logger *slog.Logger
	ch     chan *AuditEntry
	done   chan struct{}
}

// NewAuditService creates a new audit service with an async write channel.
// bufferSize controls the channel buffer; entries are dropped if the buffer is full.
func NewAuditService(db *storage.DB, logger *slog.Logger, bufferSize int) *AuditService {
	if bufferSize <= 0 {
		bufferSize = 1024
	}
	s := &AuditService{
		db:     db,
		logger: logger,
		ch:     make(chan *AuditEntry, bufferSize),
		done:   make(chan struct{}),
	}
	go s.writer()
	return s
}

// Log enqueues an audit entry for async writing. Non-blocking; drops if buffer full.
func (s *AuditService) Log(entry *AuditEntry) {
	if entry == nil {
		return
	}
	select {
	case s.ch <- entry:
	default:
		s.logger.Warn("audit log buffer full, dropping entry", "action", entry.Action)
	}
}

// Close drains the buffer and shuts down the writer goroutine.
func (s *AuditService) Close() {
	close(s.ch)
	<-s.done
}

// List queries audit log entries with filters.
func (s *AuditService) List(_ context.Context, opts storage.ListAuditLogOpts) ([]*storage.AuditLogRecord, int, error) {
	return s.db.ListAuditLog(opts)
}

// writer drains the channel and batch-inserts entries into the database.
func (s *AuditService) writer() {
	defer close(s.done)

	const maxBatch = 50
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	var batch []storage.AuditLogEntry

	flush := func() {
		if len(batch) == 0 {
			return
		}
		if err := s.db.CreateAuditLogBatch(batch); err != nil {
			s.logger.Error("failed to write audit log batch", "count", len(batch), "error", err)
		}
		batch = batch[:0]
	}

	for {
		select {
		case entry, ok := <-s.ch:
			if !ok {
				// Channel closed, flush remaining
				flush()
				return
			}
			batch = append(batch, toStorageEntry(entry))
			if len(batch) >= maxBatch {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

// toStorageEntry converts an AuditEntry to a storage.AuditLogEntry.
func toStorageEntry(e *AuditEntry) storage.AuditLogEntry {
	return storage.AuditLogEntry{
		Action:     e.Action,
		ActorID:    e.ActorID,
		ActorType:  e.ActorType,
		Resource:   e.Resource,
		Method:     e.Method,
		Endpoint:   e.Endpoint,
		StatusCode: e.StatusCode,
		IP:         e.IP,
		Source:     e.Source,
		DurationMs: e.Duration.Milliseconds(),
		Metadata:   e.Metadata,
	}
}

// EntityChangeEntry represents a CRUD operation on an entity.
// Written to a separate log file from security audit events.
type EntityChangeEntry struct {
	Timestamp   string                 `json:"timestamp"`
	ActorID     string                 `json:"actor_id"`
	Action      string                 `json:"action"` // create, update, delete, cancel
	EntityType  string                 `json:"entity_type"`
	EntityID    string                 `json:"entity_id"`
	Fields      []string               `json:"fields,omitempty"`      // changed fields for updates
	EndeavourID string                 `json:"endeavour_id,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// EntityAuditLogger writes entity CRUD events to a dedicated JSON-lines file.
// This is separate from the security audit log to keep data change history
// distinct from security alerts and access events.
type EntityAuditLogger struct {
	file   *os.File
	mu     sync.Mutex
	logger *slog.Logger
}

// NewEntityAuditLogger opens (or creates) the entity audit log file.
// The file uses 0600 permissions (owner-only read/write).
func NewEntityAuditLogger(path string, logger *slog.Logger) (*EntityAuditLogger, error) {
	if path == "" {
		path = "taskschmiede-entity-audit.jsonl"
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil, fmt.Errorf("open entity audit log %s: %w", path, err)
	}
	logger.Info("Entity audit logging to file", "path", path)
	return &EntityAuditLogger{file: f, logger: logger}, nil
}

// Log writes an entity change entry to the log file.
func (l *EntityAuditLogger) Log(entry *EntityChangeEntry) {
	if entry == nil {
		return
	}
	entry.Timestamp = time.Now().UTC().Format(time.RFC3339)
	data, err := json.Marshal(entry)
	if err != nil {
		l.logger.Error("failed to marshal entity audit entry", "error", err)
		return
	}
	l.mu.Lock()
	_, _ = l.file.Write(append(data, '\n'))
	l.mu.Unlock()
}

// Close closes the underlying log file.
func (l *EntityAuditLogger) Close() error {
	return l.file.Close()
}

// EntityChangeDBWriter writes entity CRUD events to the database asynchronously.
// Same pattern as AuditService: buffered channel + batch flush goroutine.
type EntityChangeDBWriter struct {
	db     *storage.DB
	logger *slog.Logger
	ch     chan *EntityChangeEntry
	done   chan struct{}
}

// NewEntityChangeDBWriter creates a new async DB writer for entity changes.
func NewEntityChangeDBWriter(db *storage.DB, logger *slog.Logger, bufferSize int) *EntityChangeDBWriter {
	if bufferSize <= 0 {
		bufferSize = 1024
	}
	w := &EntityChangeDBWriter{
		db:     db,
		logger: logger,
		ch:     make(chan *EntityChangeEntry, bufferSize),
		done:   make(chan struct{}),
	}
	go w.writer()
	return w
}

// Log enqueues an entity change entry for async writing. Non-blocking; drops if buffer full.
func (w *EntityChangeDBWriter) Log(entry *EntityChangeEntry) {
	if entry == nil {
		return
	}
	select {
	case w.ch <- entry:
	default:
		w.logger.Warn("entity change buffer full, dropping entry", "action", entry.Action, "entity_type", entry.EntityType)
	}
}

// Close drains the buffer and shuts down the writer goroutine.
func (w *EntityChangeDBWriter) Close() {
	close(w.ch)
	<-w.done
}

// List queries entity changes with scope filtering.
func (w *EntityChangeDBWriter) List(_ context.Context, opts storage.ListEntityChangesOpts) ([]*storage.EntityChangeRecord, int, error) {
	return w.db.ListEntityChanges(opts)
}

func (w *EntityChangeDBWriter) writer() {
	defer close(w.done)

	const maxBatch = 50
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	var batch []*storage.EntityChangeEntry

	flush := func() {
		if len(batch) == 0 {
			return
		}
		if err := w.db.CreateEntityChangeBatch(batch); err != nil {
			w.logger.Error("failed to write entity change batch", "count", len(batch), "error", err)
		}
		batch = batch[:0]
	}

	for {
		select {
		case entry, ok := <-w.ch:
			if !ok {
				flush()
				return
			}
			batch = append(batch, toEntityChangeEntry(entry))
			if len(batch) >= maxBatch {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

func toEntityChangeEntry(e *EntityChangeEntry) *storage.EntityChangeEntry {
	return &storage.EntityChangeEntry{
		ActorID:     e.ActorID,
		Action:      e.Action,
		EntityType:  e.EntityType,
		EntityID:    e.EntityID,
		EndeavourID: e.EndeavourID,
		Fields:      e.Fields,
		Metadata:    e.Metadata,
	}
}
