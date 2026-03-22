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

// Valid template scopes.
var validTemplateScopes = map[string]bool{
	"task":      true,
	"demand":    true,
	"endeavour": true,
}

// TemplateService handles template business logic.
type TemplateService struct {
	db     *storage.DB
	logger *slog.Logger
}

// NewTemplateService creates a new TemplateService.
func NewTemplateService(db *storage.DB, logger *slog.Logger) *TemplateService {
	return &TemplateService{db: db, logger: logger}
}

// Create creates a new template.
func (s *TemplateService) Create(ctx context.Context, name, tplType, scope, lang, body, createdBy string, metadata map[string]interface{}) (*storage.Template, error) {
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if body == "" {
		return nil, fmt.Errorf("body is required")
	}
	if !validTemplateScopes[scope] {
		return nil, fmt.Errorf("invalid scope: %s (must be task, demand, or endeavour)", scope)
	}
	if lang == "" {
		lang = "en"
	}

	tpl, err := s.db.CreateTemplate(name, tplType, scope, lang, body, createdBy, 1, "", metadata)
	if err != nil {
		return nil, fmt.Errorf("create template: %w", err)
	}

	s.logger.Info("Template created", "id", tpl.ID, "name", name, "type", tplType, "scope", scope, "lang", lang)
	return tpl, nil
}

// Get retrieves a template by ID.
func (s *TemplateService) Get(ctx context.Context, id string) (*storage.Template, error) {
	if id == "" {
		return nil, fmt.Errorf("id is required")
	}
	return s.db.GetTemplate(id)
}

// GetByScope retrieves the active template for a scope+lang combination.
func (s *TemplateService) GetByScope(ctx context.Context, scope, lang string) (*storage.Template, error) {
	if scope == "" {
		return nil, fmt.Errorf("scope is required")
	}
	return s.db.GetTemplateByScope(scope, lang)
}

// List queries templates with filters.
func (s *TemplateService) List(ctx context.Context, opts storage.ListTemplatesOpts) ([]*storage.Template, int, error) {
	return s.db.ListTemplates(opts)
}

// Update applies partial updates to a template.
func (s *TemplateService) Update(ctx context.Context, id string, fields storage.UpdateTemplateFields) ([]string, error) {
	if id == "" {
		return nil, fmt.Errorf("id is required")
	}
	updatedFields, err := s.db.UpdateTemplate(id, fields)
	if err != nil {
		return nil, err
	}
	s.logger.Info("Template updated", "id", id, "fields", updatedFields)
	return updatedFields, nil
}

// Fork creates a new template version derived from an existing one.
func (s *TemplateService) Fork(ctx context.Context, sourceID, name, body, lang, createdBy string, metadata map[string]interface{}) (*storage.Template, error) {
	if sourceID == "" {
		return nil, fmt.Errorf("source_id is required")
	}

	tpl, err := s.db.ForkTemplate(sourceID, name, body, lang, createdBy, metadata)
	if err != nil {
		return nil, fmt.Errorf("fork template: %w", err)
	}

	s.logger.Info("Template forked", "id", tpl.ID, "source_id", sourceID, "name", tpl.Name)
	return tpl, nil
}

// Lineage returns the full predecessor chain for a template.
func (s *TemplateService) Lineage(ctx context.Context, id string) ([]*storage.Template, error) {
	if id == "" {
		return nil, fmt.Errorf("id is required")
	}
	return s.db.GetTemplateLineage(id)
}
