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


package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/QuestFinTech/taskschmiede/internal/storage"
)

// Valid ritual run status transitions.
var validRunTransitions = map[string][]string{
	"running": {"succeeded", "failed", "skipped"},
}

// Valid ritual run triggers.
var validRunTriggers = map[string]bool{
	"schedule": true,
	"manual":   true,
	"api":      true,
}

// RitualRunService handles ritual run business logic.
type RitualRunService struct {
	db     *storage.DB
	logger *slog.Logger
}

// NewRitualRunService creates a new RitualRunService.
func NewRitualRunService(db *storage.DB, logger *slog.Logger) *RitualRunService {
	return &RitualRunService{db: db, logger: logger}
}

// Create creates a new ritual run (marks execution start).
func (s *RitualRunService) Create(ctx context.Context, ritualID, trigger, runBy string, metadata map[string]interface{}) (*storage.RitualRun, error) {
	if ritualID == "" {
		return nil, fmt.Errorf("ritual_id is required")
	}

	// Verify ritual exists
	if _, err := s.db.GetRitual(ritualID); err != nil {
		return nil, storage.ErrRitualNotFound
	}

	if trigger == "" {
		trigger = "manual"
	}
	if !validRunTriggers[trigger] {
		return nil, fmt.Errorf("invalid trigger: %s (must be schedule, manual, or api)", trigger)
	}

	run, err := s.db.CreateRitualRun(ritualID, trigger, runBy, metadata)
	if err != nil {
		return nil, fmt.Errorf("create ritual run: %w", err)
	}

	s.logger.Info("Ritual run created", "id", run.ID, "ritual_id", ritualID, "trigger", trigger)
	return run, nil
}

// Get retrieves a ritual run by ID.
func (s *RitualRunService) Get(ctx context.Context, id string) (*storage.RitualRun, error) {
	if id == "" {
		return nil, fmt.Errorf("id is required")
	}

	return s.db.GetRitualRun(id)
}

// List queries ritual runs with filters.
func (s *RitualRunService) List(ctx context.Context, opts storage.ListRitualRunsOpts) ([]*storage.RitualRun, int, error) {
	return s.db.ListRitualRuns(opts)
}

// Update applies partial updates to a ritual run with status transition validation.
func (s *RitualRunService) Update(ctx context.Context, id string, fields storage.UpdateRitualRunFields) ([]string, error) {
	if id == "" {
		return nil, fmt.Errorf("id is required")
	}

	// Validate status transition if status is being changed
	if fields.Status != nil {
		run, err := s.db.GetRitualRun(id)
		if err != nil {
			return nil, err
		}

		newStatus := *fields.Status
		allowed := validRunTransitions[run.Status]
		valid := false
		for _, s := range allowed {
			if s == newStatus {
				valid = true
				break
			}
		}
		if !valid {
			return nil, fmt.Errorf("invalid status transition: %s -> %s", run.Status, newStatus)
		}
	}

	updatedFields, err := s.db.UpdateRitualRun(id, fields)
	if err != nil {
		return nil, err
	}

	s.logger.Info("Ritual run updated", "id", id, "fields", updatedFields)
	return updatedFields, nil
}
