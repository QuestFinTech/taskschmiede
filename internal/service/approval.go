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

// ApprovalService handles approval business logic.
type ApprovalService struct {
	db     *storage.DB
	logger *slog.Logger
}

// NewApprovalService creates a new ApprovalService.
func NewApprovalService(db *storage.DB, logger *slog.Logger) *ApprovalService {
	return &ApprovalService{db: db, logger: logger}
}

// validApprovableEntities lists entity types that support approvals.
var validApprovableEntities = map[string]bool{
	"task":      true,
	"demand":    true,
	"endeavour": true,
	"artifact":  true,
}

// validVerdicts lists acceptable verdict values.
var validVerdicts = map[string]bool{
	"approved":   true,
	"rejected":   true,
	"needs_work": true,
}

// Create validates inputs and creates an approval.
func (s *ApprovalService) Create(ctx context.Context, entityType, entityID, approverID, role, verdict, comment string, metadata map[string]interface{}) (*storage.Approval, error) {
	if entityType == "" {
		return nil, fmt.Errorf("entity_type is required")
	}
	if entityID == "" {
		return nil, fmt.Errorf("entity_id is required")
	}
	if approverID == "" {
		return nil, fmt.Errorf("approver_id is required")
	}
	if verdict == "" {
		return nil, fmt.Errorf("verdict is required")
	}

	if !validApprovableEntities[entityType] {
		return nil, fmt.Errorf("unsupported entity type for approvals: %s", entityType)
	}

	if !validVerdicts[verdict] {
		return nil, fmt.Errorf("invalid verdict: %s (must be approved, rejected, or needs_work)", verdict)
	}

	if err := s.entityExists(entityType, entityID); err != nil {
		return nil, fmt.Errorf("target entity not found: %w", err)
	}

	approval, err := s.db.CreateApproval(entityType, entityID, approverID, role, verdict, comment, metadata)
	if err != nil {
		return nil, fmt.Errorf("create approval: %w", err)
	}

	s.logger.Info("Approval created", "id", approval.ID, "entity_type", entityType, "entity_id", entityID, "approver_id", approverID, "verdict", verdict)
	return approval, nil
}

// Get retrieves an approval by ID.
func (s *ApprovalService) Get(ctx context.Context, id string) (*storage.Approval, error) {
	if id == "" {
		return nil, fmt.Errorf("id is required")
	}
	return s.db.GetApproval(id)
}

// List queries approvals for an entity.
func (s *ApprovalService) List(ctx context.Context, opts storage.ListApprovalsOpts) ([]*storage.Approval, int, error) {
	// Entity filter is optional when EndeavourIDs scoping is active.
	if opts.EndeavourIDs == nil {
		if opts.EntityType == "" {
			return nil, 0, fmt.Errorf("entity_type is required")
		}
		if opts.EntityID == "" {
			return nil, 0, fmt.Errorf("entity_id is required")
		}
	}
	return s.db.ListApprovals(opts)
}

// entityExists checks whether the target entity exists.
func (s *ApprovalService) entityExists(entityType, entityID string) error {
	switch entityType {
	case "task":
		_, err := s.db.GetTask(entityID)
		return err
	case "demand":
		_, err := s.db.GetDemand(entityID)
		return err
	case "endeavour":
		_, err := s.db.GetEndeavour(entityID)
		return err
	case "artifact":
		_, err := s.db.GetArtifact(entityID)
		return err
	default:
		return fmt.Errorf("unsupported entity type: %s", entityType)
	}
}
