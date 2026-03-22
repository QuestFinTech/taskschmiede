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

// ArtifactService handles artifact business logic.
type ArtifactService struct {
	db     *storage.DB
	logger *slog.Logger
}

// NewArtifactService creates a new ArtifactService.
func NewArtifactService(db *storage.DB, logger *slog.Logger) *ArtifactService {
	return &ArtifactService{db: db, logger: logger}
}

// Create creates a new artifact.
func (s *ArtifactService) Create(ctx context.Context, kind, title, url, summary string, tags []string, metadata map[string]interface{}, createdBy, endeavourID, taskID string) (*storage.Artifact, error) {
	if title == "" {
		return nil, fmt.Errorf("title is required")
	}
	if kind == "" {
		return nil, fmt.Errorf("kind is required")
	}

	// Verify endeavour exists if provided
	if endeavourID != "" {
		if _, err := s.db.GetEndeavour(endeavourID); err != nil {
			return nil, storage.ErrEndeavourNotFound
		}
	}

	art, err := s.db.CreateArtifact(kind, title, url, summary, tags, metadata, createdBy, endeavourID, taskID)
	if err != nil {
		return nil, fmt.Errorf("create artifact: %w", err)
	}

	s.logger.Info("Artifact created", "id", art.ID, "kind", kind, "title", title)
	return art, nil
}

// Get retrieves an artifact by ID.
func (s *ArtifactService) Get(ctx context.Context, id string) (*storage.Artifact, error) {
	if id == "" {
		return nil, fmt.Errorf("id is required")
	}

	return s.db.GetArtifact(id)
}

// List queries artifacts with filters.
func (s *ArtifactService) List(ctx context.Context, opts storage.ListArtifactsOpts) ([]*storage.Artifact, int, error) {
	return s.db.ListArtifacts(opts)
}

// Update applies partial updates to an artifact.
func (s *ArtifactService) Update(ctx context.Context, id string, fields storage.UpdateArtifactFields) ([]string, error) {
	if id == "" {
		return nil, fmt.Errorf("id is required")
	}

	// Verify endeavour exists if being changed
	if fields.EndeavourID != nil && *fields.EndeavourID != "" {
		if _, err := s.db.GetEndeavour(*fields.EndeavourID); err != nil {
			return nil, storage.ErrEndeavourNotFound
		}
	}

	updatedFields, err := s.db.UpdateArtifact(id, fields)
	if err != nil {
		return nil, err
	}

	s.logger.Info("Artifact updated", "id", id, "fields", updatedFields)
	return updatedFields, nil
}
