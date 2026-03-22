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

// ResourceService handles resource business logic.
type ResourceService struct {
	db     *storage.DB
	logger *slog.Logger
}

// NewResourceService creates a new ResourceService.
func NewResourceService(db *storage.DB, logger *slog.Logger) *ResourceService {
	return &ResourceService{db: db, logger: logger}
}

// Create creates a new resource.
func (s *ResourceService) Create(ctx context.Context, resType, name, capacityModel string, capacityValue *float64, skills []string, metadata map[string]interface{}) (*storage.Resource, error) {
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if resType == "" {
		return nil, fmt.Errorf("type is required")
	}

	validTypes := map[string]bool{"human": true, "agent": true, "service": true, "budget": true, "team": true}
	if !validTypes[resType] {
		return nil, fmt.Errorf("invalid resource type: %s (must be human, agent, service, budget, or team)", resType)
	}

	res, err := s.db.CreateResource(resType, name, capacityModel, capacityValue, skills, metadata)
	if err != nil {
		return nil, fmt.Errorf("create resource: %w", err)
	}

	s.logger.Info("Resource created", "id", res.ID, "type", resType, "name", name)
	return res, nil
}

// Get retrieves a resource by ID.
func (s *ResourceService) Get(ctx context.Context, id string) (*storage.Resource, error) {
	if id == "" {
		return nil, fmt.Errorf("id is required")
	}

	return s.db.GetResource(id)
}

// Delete hard-deletes a resource and all its relations.
// Only team-type resources may be deleted; other types must be deactivated.
func (s *ResourceService) Delete(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("id is required")
	}
	res, err := s.db.GetResource(id)
	if err != nil {
		return err
	}
	if res.Type != "team" {
		return fmt.Errorf("only team resources can be deleted; deactivate other resource types instead")
	}
	if err := s.db.DeleteResource(id); err != nil {
		return err
	}
	s.logger.Info("Resource deleted", "id", id, "type", res.Type, "name", res.Name)
	return nil
}

// List queries resources with filters.
func (s *ResourceService) List(ctx context.Context, opts storage.ListResourcesOpts) ([]*storage.Resource, int, error) {
	return s.db.ListResources(opts)
}

// Update applies partial updates to a resource.
func (s *ResourceService) Update(ctx context.Context, id string, fields storage.UpdateResourceFields) ([]string, error) {
	if id == "" {
		return nil, fmt.Errorf("id is required")
	}

	if fields.Status != nil {
		validStatuses := map[string]bool{"active": true, "inactive": true}
		if !validStatuses[*fields.Status] {
			return nil, fmt.Errorf("invalid status: %s (must be active or inactive)", *fields.Status)
		}
	}

	updatedFields, err := s.db.UpdateResource(id, fields)
	if err != nil {
		return nil, err
	}

	s.logger.Info("Resource updated", "id", id, "fields", updatedFields)
	return updatedFields, nil
}
