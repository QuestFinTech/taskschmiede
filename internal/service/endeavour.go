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

// Valid endeavour status transitions.
// Note: "archived" is NOT reachable via Update -- it requires the dedicated Archive method.
var validEdvTransitions = map[string][]string{
	"pending":   {"active"},
	"active":    {"on_hold", "completed"},
	"on_hold":   {"active"},
	"completed": {}, // completed is terminal; use archive to reclaim tier slot
}

// EndeavourService handles endeavour business logic.
type EndeavourService struct {
	db     *storage.DB
	logger *slog.Logger
}

// NewEndeavourService creates a new EndeavourService.
func NewEndeavourService(db *storage.DB, logger *slog.Logger) *EndeavourService {
	return &EndeavourService{db: db, logger: logger}
}

// Create creates a new endeavour.
func (s *EndeavourService) Create(ctx context.Context, name, description string, goals []storage.Goal, startDate, endDate *time.Time, metadata map[string]interface{}) (*storage.Endeavour, error) {
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}

	edv, err := s.db.CreateEndeavour(name, description, goals, startDate, endDate, metadata)
	if err != nil {
		return nil, fmt.Errorf("create endeavour: %w", err)
	}

	s.logger.Info("Endeavour created", "id", edv.ID, "name", name)
	return edv, nil
}

// Get retrieves an endeavour by ID with task progress.
func (s *EndeavourService) Get(ctx context.Context, id string) (*storage.Endeavour, *storage.TaskProgress, error) {
	if id == "" {
		return nil, nil, fmt.Errorf("id is required")
	}

	edv, err := s.db.GetEndeavour(id)
	if err != nil {
		return nil, nil, err
	}

	progress, _ := s.db.GetEndeavourTaskProgress(id)

	return edv, progress, nil
}

// List queries endeavours with filters.
func (s *EndeavourService) List(ctx context.Context, opts storage.ListEndeavoursOpts) ([]*storage.Endeavour, int, error) {
	return s.db.ListEndeavours(opts)
}

// Update applies partial updates to an endeavour with status transition validation.
func (s *EndeavourService) Update(ctx context.Context, id string, fields storage.UpdateEndeavourFields) ([]string, error) {
	if id == "" {
		return nil, fmt.Errorf("id is required")
	}

	edv, err := s.db.GetEndeavour(id)
	if err != nil {
		return nil, err
	}

	// Read-only: archived endeavours cannot be updated.
	if edv.Status == "archived" {
		return nil, fmt.Errorf("cannot update archived endeavour")
	}

	// Validate status transition if status is being changed
	if fields.Status != nil {
		newStatus := *fields.Status

		// Same status is a no-op -- clear the field so it's not written.
		if newStatus == edv.Status {
			fields.Status = nil
		} else {
			// Block "archived" via regular update -- must use Archive().
			if newStatus == "archived" {
				return nil, fmt.Errorf("cannot set status to archived via update; use the archive operation instead")
			}

			allowed := validEdvTransitions[edv.Status]
			valid := false
			for _, s := range allowed {
				if s == newStatus {
					valid = true
					break
				}
			}
			if !valid {
				return nil, fmt.Errorf("invalid status transition: %s -> %s", edv.Status, newStatus)
			}

			// Completing requires all tasks to be in a terminal state.
			if newStatus == "completed" {
				progress, perr := s.db.GetEndeavourTaskProgress(id)
				if perr != nil {
					return nil, fmt.Errorf("check task progress: %w", perr)
				}
				if progress.Planned > 0 || progress.Active > 0 {
					return nil, fmt.Errorf("cannot complete: %d planned and %d active tasks remain", progress.Planned, progress.Active)
				}
			}
		}
	}

	updatedFields, err := s.db.UpdateEndeavour(id, fields)
	if err != nil {
		return nil, err
	}

	s.logger.Info("Endeavour updated", "id", id, "fields", updatedFields)
	return updatedFields, nil
}

// ArchiveImpact represents the impact of archiving an endeavour.
type ArchiveImpact struct {
	PlannedTasks  int `json:"planned_tasks"`
	ActiveTasks   int `json:"active_tasks"`
	TasksToCancel int `json:"tasks_to_cancel"`
	DoneTasks     int `json:"done_tasks"`
	CanceledTasks int `json:"canceled_tasks"`
}

// ArchiveImpact returns a dry-run summary of what archiving an endeavour would do.
func (s *EndeavourService) ArchiveImpact(ctx context.Context, id string) (*ArchiveImpact, error) {
	edv, err := s.db.GetEndeavour(id)
	if err != nil {
		return nil, err
	}
	if edv.Status == "archived" {
		return nil, fmt.Errorf("endeavour is already archived")
	}

	progress, err := s.db.GetEndeavourTaskProgress(id)
	if err != nil {
		return nil, fmt.Errorf("get task progress: %w", err)
	}

	return &ArchiveImpact{
		PlannedTasks:  progress.Planned,
		ActiveTasks:   progress.Active,
		TasksToCancel: progress.Planned + progress.Active,
		DoneTasks:     progress.Done,
		CanceledTasks: progress.Canceled,
	}, nil
}

// Archive archives an endeavour: cancels all non-terminal tasks and sets status to archived.
func (s *EndeavourService) Archive(ctx context.Context, id, reason string) (int, error) {
	edv, err := s.db.GetEndeavour(id)
	if err != nil {
		return 0, err
	}
	if edv.Status == "archived" {
		return 0, fmt.Errorf("endeavour is already archived")
	}

	// Bulk cancel non-terminal tasks
	canceled, err := s.db.BulkCancelTasksByEndeavour(id, "Endeavour archived: "+reason)
	if err != nil {
		return 0, fmt.Errorf("bulk cancel tasks: %w", err)
	}

	// Set endeavour to archived
	archived := "archived"
	fields := storage.UpdateEndeavourFields{
		Status:         &archived,
		ArchivedReason: &reason,
	}
	if _, err := s.db.UpdateEndeavour(id, fields); err != nil {
		return 0, fmt.Errorf("update endeavour status: %w", err)
	}

	s.logger.Info("Endeavour archived", "id", id, "reason", reason, "tasks_canceled", canceled)
	return canceled, nil
}

// AddUser grants a user access to an endeavour.
func (s *EndeavourService) AddUser(ctx context.Context, userID, endeavourID, role string) error {
	if userID == "" || endeavourID == "" {
		return fmt.Errorf("user_id and endeavour_id are required")
	}

	// Verify user exists
	if _, err := s.db.GetUser(userID); err != nil {
		return err
	}

	// Verify endeavour exists
	if _, err := s.db.GetEndeavour(endeavourID); err != nil {
		return err
	}

	if err := s.db.AddUserToEndeavour(userID, endeavourID, role); err != nil {
		return err
	}

	s.logger.Info("User added to endeavour", "user_id", userID, "endeavour_id", endeavourID, "role", role)
	return nil
}
