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
	"time"

	"github.com/QuestFinTech/taskschmiede/internal/storage"
)

// Valid demand status transitions.
var validDemandTransitions = map[string][]string{
	"open":        {"in_progress", "canceled"},
	"in_progress": {"fulfilled", "canceled", "open"},
	"fulfilled":   {"open"},     // reopen
	"canceled":    {"open"},     // reopen
}

// Valid demand priorities.
var validPriorities = map[string]bool{
	"low":    true,
	"medium": true,
	"high":   true,
	"urgent": true,
}

// DemandService handles demand business logic.
type DemandService struct {
	db     *storage.DB
	logger *slog.Logger
}

// NewDemandService creates a new DemandService.
func NewDemandService(db *storage.DB, logger *slog.Logger) *DemandService {
	return &DemandService{db: db, logger: logger}
}

// Create creates a new demand.
func (s *DemandService) Create(ctx context.Context, demandType, title, description, priority, endeavourID, creatorID string, dueDate *time.Time, metadata map[string]interface{}) (*storage.Demand, error) {
	if title == "" {
		return nil, fmt.Errorf("title is required")
	}
	if demandType == "" {
		return nil, fmt.Errorf("type is required")
	}

	if priority == "" {
		priority = "medium"
	}
	if !validPriorities[priority] {
		return nil, fmt.Errorf("invalid priority: %s (must be low, medium, high, or urgent)", priority)
	}

	// Verify endeavour exists if provided
	if endeavourID != "" {
		if _, err := s.db.GetEndeavour(endeavourID); err != nil {
			return nil, storage.ErrEndeavourNotFound
		}
	}

	demand, err := s.db.CreateDemand(demandType, title, description, priority, endeavourID, creatorID, dueDate, metadata)
	if err != nil {
		return nil, fmt.Errorf("create demand: %w", err)
	}

	s.logger.Info("Demand created", "id", demand.ID, "type", demandType, "title", title, "endeavour_id", endeavourID)
	return demand, nil
}

// Get retrieves a demand by ID.
func (s *DemandService) Get(ctx context.Context, id string) (*storage.Demand, error) {
	if id == "" {
		return nil, fmt.Errorf("id is required")
	}

	return s.db.GetDemand(id)
}

// List queries demands with filters.
func (s *DemandService) List(ctx context.Context, opts storage.ListDemandsOpts) ([]*storage.Demand, int, error) {
	return s.db.ListDemands(opts)
}

// Update applies partial updates to a demand with status transition and priority validation.
func (s *DemandService) Update(ctx context.Context, id string, fields storage.UpdateDemandFields) ([]string, error) {
	if id == "" {
		return nil, fmt.Errorf("id is required")
	}

	// Validate status transition if status is being changed
	if fields.Status != nil {
		demand, err := s.db.GetDemand(id)
		if err != nil {
			return nil, err
		}

		newStatus := *fields.Status
		allowed := validDemandTransitions[demand.Status]
		valid := false
		for _, s := range allowed {
			if s == newStatus {
				valid = true
				break
			}
		}
		if !valid {
			return nil, fmt.Errorf("invalid status transition: %s -> %s", demand.Status, newStatus)
		}
	}

	// Validate priority if being changed
	if fields.Priority != nil {
		if !validPriorities[*fields.Priority] {
			return nil, fmt.Errorf("invalid priority: %s (must be low, medium, high, or urgent)", *fields.Priority)
		}
	}

	// Verify endeavour exists if being changed
	if fields.EndeavourID != nil && *fields.EndeavourID != "" {
		if _, err := s.db.GetEndeavour(*fields.EndeavourID); err != nil {
			return nil, storage.ErrEndeavourNotFound
		}
	}

	updatedFields, err := s.db.UpdateDemand(id, fields)
	if err != nil {
		return nil, err
	}

	s.logger.Info("Demand updated", "id", id, "fields", updatedFields)
	return updatedFields, nil
}
