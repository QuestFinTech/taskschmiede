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
	cryptoRand "crypto/rand"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/QuestFinTech/taskschmiede/internal/storage"
)

// InterviewPhase represents the current phase of an interview session.
type InterviewPhase string

// Interview phase constants.
const (
	// PhaseStep0 is the initial self-description phase.
	PhaseStep0 InterviewPhase = "step0"
	// PhaseSection is the challenge section phase.
	PhaseSection InterviewPhase = "section"
	// PhaseComplete indicates the interview has finished.
	PhaseComplete InterviewPhase = "complete"
)

// InterviewSession holds all state for an in-progress interview.
type InterviewSession struct {
	mu sync.Mutex

	ID        string
	UserID    string
	AttemptID string
	Version   *InterviewVersion

	// Simulation database (in-memory SQLite)
	SimDB *storage.DB

	// Current state
	Phase          InterviewPhase
	CurrentSection int
	ToolLog        *ToolLog

	// Timing
	StartedAt       time.Time
	SectionStartedAt time.Time
	Step0StartedAt   time.Time

	// IDs created during the interview (for cross-section referencing)
	CreatedTaskID   string // Set after Section 1 task creation
	CreatedDemandID string // Set after Section 6 demand creation

	// Step 0 data
	Step0Text      string
	Step0ModelInfo map[string]interface{}
}

// TimeRemaining returns the time remaining in the total interview budget.
func (s *InterviewSession) TimeRemaining() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	elapsed := storage.UTCNow().Sub(s.StartedAt)
	remaining := s.Version.TimeoutTotal - elapsed
	if remaining < 0 {
		return 0
	}
	return remaining
}

// SectionTimeRemaining returns the time remaining in the current section.
func (s *InterviewSession) SectionTimeRemaining() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	elapsed := storage.UTCNow().Sub(s.SectionStartedAt)
	remaining := s.Version.TimeoutSection - elapsed
	if remaining < 0 {
		return 0
	}
	return remaining
}

// IsExpired returns true if the total time budget is exhausted.
func (s *InterviewSession) IsExpired() bool {
	return s.TimeRemaining() == 0
}

// IsSectionExpired returns true if the current section's time is exhausted.
func (s *InterviewSession) IsSectionExpired() bool {
	return s.SectionTimeRemaining() == 0
}

// IsStep0Expired returns true if the Step 0 timeout has elapsed.
func (s *InterviewSession) IsStep0Expired() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Phase != PhaseStep0 {
		return false
	}
	elapsed := storage.UTCNow().Sub(s.Step0StartedAt)
	return elapsed >= s.Version.TimeoutStep0
}

// TotalToolCallsUsed returns the total number of tool calls made.
func (s *InterviewSession) TotalToolCallsUsed() int {
	return s.ToolLog.Count()
}

// SectionToolCallsUsed returns the number of tool calls in the current section.
func (s *InterviewSession) SectionToolCallsUsed() int {
	return s.ToolLog.CountForSection(s.CurrentSection)
}

// CheckBudgets returns an error if any budget is exhausted.
func (s *InterviewSession) CheckBudgets() error {
	if s.IsExpired() {
		return fmt.Errorf("interview time budget exhausted (total: %s)", s.Version.TimeoutTotal)
	}
	if s.Phase == PhaseSection && s.IsSectionExpired() {
		return fmt.Errorf("section %d time budget exhausted (per-section: %s)", s.CurrentSection, s.Version.TimeoutSection)
	}
	if s.TotalToolCallsUsed() >= s.Version.ToolBudgetTotal {
		return fmt.Errorf("total tool call budget exhausted (%d/%d)", s.Version.ToolBudgetTotal, s.Version.ToolBudgetTotal)
	}
	if s.Phase == PhaseSection && s.SectionToolCallsUsed() >= s.Version.ToolBudgetSection {
		return fmt.Errorf("section %d tool call budget exhausted (%d/%d)", s.CurrentSection, s.Version.ToolBudgetSection, s.Version.ToolBudgetSection)
	}
	return nil
}

// AdvanceSection moves to the next section, or completes the interview.
func (s *InterviewSession) AdvanceSection() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.CurrentSection < s.Version.SectionCount() {
		s.CurrentSection++
		s.SectionStartedAt = storage.UTCNow()
	} else {
		s.Phase = PhaseComplete
	}
}

// StartSections transitions from Step 0 to Section 1.
func (s *InterviewSession) StartSections() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Phase = PhaseSection
	s.CurrentSection = 1
	s.SectionStartedAt = storage.UTCNow()
}

// SessionManager manages active interview sessions.
type SessionManager struct {
	mu       sync.Mutex
	sessions map[string]*InterviewSession // session ID -> session
	byUser   map[string]string            // user ID -> session ID (enforces one per user)
	logger   *slog.Logger
}

// NewSessionManager creates a new session manager.
func NewSessionManager(logger *slog.Logger) *SessionManager {
	return &SessionManager{
		sessions: make(map[string]*InterviewSession),
		byUser:   make(map[string]string),
		logger:   logger,
	}
}

// CreateSession creates a new interview session with an in-memory simulation database.
// Returns an error if the user already has an active session.
func (m *SessionManager) CreateSession(userID, attemptID string, version *InterviewVersion) (*InterviewSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Enforce one active interview per user
	if existingID, ok := m.byUser[userID]; ok {
		return nil, fmt.Errorf("user %s already has an active interview session: %s", userID, existingID)
	}

	// Create in-memory simulation database
	simDB, err := storage.Open(":memory:")
	if err != nil {
		return nil, fmt.Errorf("create simulation database: %w", err)
	}
	if err := simDB.Initialize(); err != nil {
		_ = simDB.Close()
		return nil, fmt.Errorf("initialize simulation database: %w", err)
	}

	// Seed the simulation with interview data
	if err := seedSimulation(simDB, version); err != nil {
		_ = simDB.Close()
		return nil, fmt.Errorf("seed simulation: %w", err)
	}

	sessionID := generateSessionID()
	now := storage.UTCNow()

	session := &InterviewSession{
		ID:             sessionID,
		UserID:         userID,
		AttemptID:      attemptID,
		Version:        version,
		SimDB:          simDB,
		Phase:          PhaseStep0,
		CurrentSection: 0,
		ToolLog:        NewToolLog(),
		StartedAt:      now,
		Step0StartedAt: now,
	}

	m.sessions[sessionID] = session
	m.byUser[userID] = sessionID

	m.logger.Info("interview session created",
		"session_id", sessionID,
		"user_id", userID,
		"attempt_id", attemptID,
		"version", version.Version,
	)

	return session, nil
}

// GetSession retrieves an active session by ID.
func (m *SessionManager) GetSession(sessionID string) *InterviewSession {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sessions[sessionID]
}

// GetSessionByUser retrieves an active session by user ID.
func (m *SessionManager) GetSessionByUser(userID string) *InterviewSession {
	m.mu.Lock()
	defer m.mu.Unlock()
	if sessionID, ok := m.byUser[userID]; ok {
		return m.sessions[sessionID]
	}
	return nil
}

// EndSession removes a session and closes its simulation database.
func (m *SessionManager) EndSession(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return
	}

	if session.SimDB != nil {
		_ = session.SimDB.Close()
	}

	delete(m.byUser, session.UserID)
	delete(m.sessions, sessionID)

	m.logger.Info("interview session ended",
		"session_id", sessionID,
		"user_id", session.UserID,
	)
}

// ActiveCount returns the number of active sessions.
func (m *SessionManager) ActiveCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.sessions)
}

// seedSimulation populates the in-memory database with interview data.
func seedSimulation(db *storage.DB, version *InterviewVersion) error {
	// Create the interviewer resource
	_, err := db.Exec(
		`INSERT INTO resource (id, type, name, capacity_model, metadata, status)
		 VALUES (?, 'service', 'Onboarding Interviewer', 'always_on', '{}', 'active')`,
		version.SeedData.InterviewerResourceID,
	)
	if err != nil {
		return fmt.Errorf("create interviewer resource: %w", err)
	}

	// Create a user for the interviewer (needed for messaging)
	_, err = db.Exec(
		`INSERT INTO user (id, name, email, resource_id, tier, user_type, status, onboarding_status)
		 VALUES ('usr_interviewer', 'Onboarding Interviewer', 'interviewer@onboarding.local', ?, 1, 'service', 'active', 'active')`,
		version.SeedData.InterviewerResourceID,
	)
	if err != nil {
		return fmt.Errorf("create interviewer user: %w", err)
	}

	// Create the endeavour
	_, err = db.Exec(
		`INSERT INTO endeavour (id, name, description, status)
		 VALUES ('edv_onboarding', ?, 'Onboarding assessment endeavour', 'active')`,
		version.SeedData.EndeavourName,
	)
	if err != nil {
		return fmt.Errorf("create endeavour: %w", err)
	}

	// Create message tables (not part of main schema -- MessageDB handles its own migration)
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS message (
			id          TEXT PRIMARY KEY,
			sender_id   TEXT NOT NULL,
			subject     TEXT NOT NULL DEFAULT '',
			content     TEXT NOT NULL,
			intent      TEXT NOT NULL DEFAULT 'info',
			reply_to_id TEXT,
			entity_type TEXT,
			entity_id   TEXT,
			scope_type  TEXT,
			scope_id    TEXT,
			metadata    TEXT NOT NULL DEFAULT '{}',
			created_at  TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS message_delivery (
			id           TEXT PRIMARY KEY,
			message_id   TEXT NOT NULL REFERENCES message(id),
			recipient_id TEXT NOT NULL,
			channel      TEXT NOT NULL DEFAULT 'internal',
			status       TEXT NOT NULL DEFAULT 'pending',
			delivered_at TEXT,
			read_at      TEXT,
			created_at   TEXT NOT NULL
		);
	`)
	if err != nil {
		return fmt.Errorf("create message tables: %w", err)
	}

	// Seed tasks for Section 4
	now := storage.UTCNow().Format("2006-01-02T15:04:05Z")
	for i, task := range version.SeedData.Tasks {
		taskID := fmt.Sprintf("tsk_seed_%03d", i+1)
		var estimate *float64
		if task.Estimate > 0 {
			e := task.Estimate
			estimate = &e
		}
		_, err = db.Exec(
			`INSERT INTO task (id, title, description, status, estimate, created_at)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			taskID, task.Title, task.Description, task.Status, estimate, task.CreatedAt,
		)
		if err != nil {
			return fmt.Errorf("seed task %d: %w", i+1, err)
		}
		// Link task to endeavour via entity_relation
		relID := fmt.Sprintf("rel_seed_tsk_%03d", i+1)
		_, err = db.Exec(
			`INSERT INTO entity_relation (id, relationship_type, source_entity_type, source_entity_id, target_entity_type, target_entity_id, created_at)
			 VALUES (?, 'belongs_to', 'task', ?, 'endeavour', 'edv_onboarding', ?)`,
			relID, taskID, now,
		)
		if err != nil {
			return fmt.Errorf("seed task %d relation: %w", i+1, err)
		}
	}

	// Seed demands for Sections 6-8
	for i, demand := range version.SeedData.Demands {
		demandID := fmt.Sprintf("dmd_seed_%03d", i+1)
		_, err = db.Exec(
			`INSERT INTO demand (id, type, title, description, priority, status, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))`,
			demandID, demand.Type, demand.Title, demand.Description, demand.Priority, demand.Status,
		)
		if err != nil {
			return fmt.Errorf("seed demand %d: %w", i+1, err)
		}
		// Link demand to endeavour via entity_relation
		relID := fmt.Sprintf("rel_seed_dmd_%03d", i+1)
		_, err = db.Exec(
			`INSERT INTO entity_relation (id, relationship_type, source_entity_type, source_entity_id, target_entity_type, target_entity_id, created_at)
			 VALUES (?, 'belongs_to', 'demand', ?, 'endeavour', 'edv_onboarding', ?)`,
			relID, demandID, now,
		)
		if err != nil {
			return fmt.Errorf("seed demand %d relation: %w", i+1, err)
		}
	}

	return nil
}

// generateSessionID creates a unique session identifier.
func generateSessionID() string {
	// Reuse the storage package's ID generation pattern
	b := make([]byte, 12)
	_, _ = cryptoRandRead(b)
	return fmt.Sprintf("obs_%x", b)
}

// cryptoRandRead is a variable for testing.
var cryptoRandRead = cryptoRandReadImpl

func cryptoRandReadImpl(b []byte) (int, error) {
	return cryptoRand.Read(b)
}
