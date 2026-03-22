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

// Valid ritual origins.
var validRitualOrigins = map[string]bool{
	"template": true,
	"custom":   true,
	"fork":     true,
}

// RitualService handles ritual business logic.
type RitualService struct {
	db     *storage.DB
	logger *slog.Logger
}

// NewRitualService creates a new RitualService.
func NewRitualService(db *storage.DB, logger *slog.Logger) *RitualService {
	return &RitualService{db: db, logger: logger}
}

// Create creates a new ritual.
func (s *RitualService) Create(ctx context.Context, name, description, prompt, origin, createdBy, endeavourID, lang string, schedule, metadata map[string]interface{}) (*storage.Ritual, error) {
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if prompt == "" {
		return nil, fmt.Errorf("prompt is required")
	}

	if origin == "" {
		origin = "custom"
	}
	if !validRitualOrigins[origin] {
		return nil, fmt.Errorf("invalid origin: %s (must be template, custom, or fork)", origin)
	}

	// Verify endeavour exists if provided
	if endeavourID != "" {
		if _, err := s.db.GetEndeavour(endeavourID); err != nil {
			return nil, storage.ErrEndeavourNotFound
		}
	}

	ritual, err := s.db.CreateRitual(name, description, prompt, origin, createdBy, endeavourID, lang, "", 1, "", schedule, metadata)
	if err != nil {
		return nil, fmt.Errorf("create ritual: %w", err)
	}

	s.logger.Info("Ritual created", "id", ritual.ID, "name", name, "origin", origin)
	return ritual, nil
}

// Fork creates a new ritual forked from an existing one.
func (s *RitualService) Fork(ctx context.Context, sourceID, name, prompt, description, createdBy, endeavourID, lang string, schedule, metadata map[string]interface{}) (*storage.Ritual, error) {
	if sourceID == "" {
		return nil, fmt.Errorf("source_id is required")
	}

	source, err := s.db.GetRitual(sourceID)
	if err != nil {
		return nil, err
	}

	// Defaults from source
	if name == "" {
		name = source.Name
	}
	if prompt == "" {
		prompt = source.Prompt
	}
	if description == "" {
		description = source.Description
	}
	if schedule == nil {
		schedule = source.Schedule
	}
	if lang == "" {
		lang = source.Lang
	}

	// Verify endeavour exists if provided
	if endeavourID != "" {
		if _, err := s.db.GetEndeavour(endeavourID); err != nil {
			return nil, storage.ErrEndeavourNotFound
		}
	}

	// Duplicate fork check: prevent forking the same template to the same
	// endeavour when an active fork already exists.
	if endeavourID != "" {
		existing, _, err := s.db.ListRituals(storage.ListRitualsOpts{
			EndeavourID: endeavourID,
			Status:      "active",
		})
		if err != nil {
			return nil, fmt.Errorf("check duplicate fork: %w", err)
		}
		for _, r := range existing {
			if r.PredecessorID == sourceID {
				return nil, fmt.Errorf("this template is already active on this endeavour")
			}
		}
		// Methodology conflict check: if the source ritual has a methodology,
		// verify the target endeavour does not already have rituals from a
		// different methodology. Methodology-agnostic rituals are always allowed.
		if source.MethodologyID != "" {
			for _, r := range existing {
				if r.MethodologyID != "" && r.MethodologyID != source.MethodologyID {
					return nil, fmt.Errorf("methodology conflict: this endeavour already uses %s rituals", r.MethodologyID)
				}
			}
		}
	}

	// Inherit methodology from source.
	ritual, err := s.db.CreateRitual(name, description, prompt, "fork", createdBy, endeavourID, lang, source.MethodologyID, 1, sourceID, schedule, metadata)
	if err != nil {
		return nil, fmt.Errorf("fork ritual: %w", err)
	}

	s.logger.Info("Ritual forked", "id", ritual.ID, "source_id", sourceID, "name", name)
	return ritual, nil
}

// Get retrieves a ritual by ID.
func (s *RitualService) Get(ctx context.Context, id string) (*storage.Ritual, error) {
	if id == "" {
		return nil, fmt.Errorf("id is required")
	}

	return s.db.GetRitual(id)
}

// List queries rituals with filters.
func (s *RitualService) List(ctx context.Context, opts storage.ListRitualsOpts) ([]*storage.Ritual, int, error) {
	return s.db.ListRituals(opts)
}

// Update applies partial updates to a ritual.
// Cannot update prompt or version -- create a new version instead.
// Cannot update template rituals -- fork to create a customized version.
func (s *RitualService) Update(ctx context.Context, id string, fields storage.UpdateRitualFields) ([]string, error) {
	if id == "" {
		return nil, fmt.Errorf("id is required")
	}

	// Reject updates to template rituals
	ritual, err := s.db.GetRitual(id)
	if err != nil {
		return nil, err
	}
	if ritual.Origin == "template" {
		return nil, fmt.Errorf("cannot update template rituals; use ts.rtl.fork to create a customized version")
	}

	// Verify endeavour exists if being changed
	if fields.EndeavourID != nil && *fields.EndeavourID != "" {
		if _, err := s.db.GetEndeavour(*fields.EndeavourID); err != nil {
			return nil, storage.ErrEndeavourNotFound
		}
	}

	updatedFields, err := s.db.UpdateRitual(id, fields)
	if err != nil {
		return nil, err
	}

	s.logger.Info("Ritual updated", "id", id, "fields", updatedFields)
	return updatedFields, nil
}

// Lineage returns the full predecessor chain for a ritual.
func (s *RitualService) Lineage(ctx context.Context, id string) ([]*storage.Ritual, error) {
	if id == "" {
		return nil, fmt.Errorf("id is required")
	}

	return s.db.GetRitualLineage(id)
}
