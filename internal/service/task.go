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

// Valid task status transitions.
var validTransitions = map[string][]string{
	"planned":  {"active", "canceled"},
	"active":   {"done", "canceled", "planned"},
	"done":     {"active"},     // reopen
	"canceled": {"planned"},    // reopen
}

// TaskService handles task business logic.
type TaskService struct {
	db     *storage.DB
	logger *slog.Logger
}

// NewTaskService creates a new TaskService.
func NewTaskService(db *storage.DB, logger *slog.Logger) *TaskService {
	return &TaskService{db: db, logger: logger}
}

// Create creates a new task.
func (s *TaskService) Create(ctx context.Context, title, description, endeavourID, demandID, assigneeID, creatorID string, estimate *float64, dueDate *time.Time, metadata map[string]interface{}) (*storage.Task, error) {
	if title == "" {
		return nil, fmt.Errorf("title is required")
	}

	// Verify endeavour exists if provided and is not archived
	if endeavourID != "" {
		edv, err := s.db.GetEndeavour(endeavourID)
		if err != nil {
			return nil, storage.ErrEndeavourNotFound
		}
		if edv.Status == "archived" {
			return nil, fmt.Errorf("cannot create task in archived endeavour")
		}
	}

	task, err := s.db.CreateTask(title, description, endeavourID, demandID, assigneeID, creatorID, estimate, dueDate, metadata)
	if err != nil {
		return nil, fmt.Errorf("create task: %w", err)
	}

	s.logger.Info("Task created", "id", task.ID, "title", title, "endeavour_id", endeavourID)
	return task, nil
}

// Get retrieves a task by ID.
func (s *TaskService) Get(ctx context.Context, id string) (*storage.Task, error) {
	if id == "" {
		return nil, fmt.Errorf("id is required")
	}

	return s.db.GetTask(id)
}

// List queries tasks with filters.
func (s *TaskService) List(ctx context.Context, opts storage.ListTasksOpts) ([]*storage.Task, int, error) {
	return s.db.ListTasks(opts)
}

// StatusCounts returns task counts grouped by status.
func (s *TaskService) StatusCounts(ctx context.Context, opts storage.ListTasksOpts) (*storage.TaskProgress, int, error) {
	return s.db.TaskStatusCounts(opts)
}

// Update applies partial updates to a task with status transition validation.
func (s *TaskService) Update(ctx context.Context, id string, fields storage.UpdateTaskFields) ([]string, error) {
	if id == "" {
		return nil, fmt.Errorf("id is required")
	}

	task, err := s.db.GetTask(id)
	if err != nil {
		return nil, err
	}

	// Block updates to tasks in archived endeavours.
	if task.EndeavourID != "" {
		edv, err := s.db.GetEndeavour(task.EndeavourID)
		if err == nil && edv.Status == "archived" {
			return nil, fmt.Errorf("cannot update task in archived endeavour")
		}
	}

	// Validate status transition if status is being changed
	if fields.Status != nil {
		newStatus := *fields.Status
		allowed := validTransitions[task.Status]
		valid := false
		for _, s := range allowed {
			if s == newStatus {
				valid = true
				break
			}
		}
		if !valid {
			return nil, fmt.Errorf("invalid status transition: %s -> %s", task.Status, newStatus)
		}
	}

	updatedFields, err := s.db.UpdateTask(id, fields)
	if err != nil {
		return nil, err
	}

	s.logger.Info("Task updated", "id", id, "fields", updatedFields)
	return updatedFields, nil
}
