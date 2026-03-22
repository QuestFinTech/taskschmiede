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

// RelationService handles entity relation business logic.
type RelationService struct {
	db     *storage.DB
	logger *slog.Logger
}

// NewRelationService creates a new RelationService.
func NewRelationService(db *storage.DB, logger *slog.Logger) *RelationService {
	return &RelationService{db: db, logger: logger}
}

// Create creates a new entity relation.
func (s *RelationService) Create(ctx context.Context, relType, srcType, srcID, tgtType, tgtID string, metadata map[string]interface{}, createdBy string) (*storage.EntityRelation, error) {
	if relType == "" {
		return nil, fmt.Errorf("relationship_type is required")
	}
	if srcType == "" || srcID == "" {
		return nil, fmt.Errorf("source_entity_type and source_entity_id are required")
	}
	if tgtType == "" || tgtID == "" {
		return nil, fmt.Errorf("target_entity_type and target_entity_id are required")
	}

	rel, err := s.db.CreateRelation(relType, srcType, srcID, tgtType, tgtID, metadata, createdBy)
	if err != nil {
		return nil, fmt.Errorf("create relation: %w", err)
	}

	s.logger.Info("Relation created", "id", rel.ID, "type", relType, "source", srcType+"/"+srcID, "target", tgtType+"/"+tgtID)
	return rel, nil
}

// List queries relations with filters.
func (s *RelationService) List(ctx context.Context, opts storage.ListRelationsOpts) ([]*storage.EntityRelation, int, error) {
	return s.db.ListRelations(opts)
}

// Delete removes a relation by ID.
func (s *RelationService) Delete(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("id is required")
	}

	if err := s.db.DeleteRelation(id); err != nil {
		return err
	}

	s.logger.Info("Relation deleted", "id", id)
	return nil
}
