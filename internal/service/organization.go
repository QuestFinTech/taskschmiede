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


// Package service provides the business logic layer for Taskschmiede entities
// including tasks, demands, endeavours, organizations, resources, rituals,
// messages, approvals, templates, and definitions of done. Services sit
// between MCP handlers and the storage layer.
package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/QuestFinTech/taskschmiede/internal/storage"
)

// OrganizationService handles organization business logic.
type OrganizationService struct {
	db     *storage.DB
	logger *slog.Logger
	edvSvc *EndeavourService
}

// NewOrganizationService creates a new OrganizationService.
func NewOrganizationService(db *storage.DB, logger *slog.Logger) *OrganizationService {
	return &OrganizationService{db: db, logger: logger}
}

// SetEndeavourService sets the endeavour service reference (needed for archive cascade).
func (s *OrganizationService) SetEndeavourService(edvSvc *EndeavourService) {
	s.edvSvc = edvSvc
}

// Create creates a new organization.
func (s *OrganizationService) Create(ctx context.Context, name, description string, metadata map[string]interface{}) (*storage.Organization, error) {
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}

	org, err := s.db.CreateOrganization(name, description, metadata)
	if err != nil {
		return nil, fmt.Errorf("create organization: %w", err)
	}

	s.logger.Info("Organization created", "id", org.ID, "name", name)
	return org, nil
}

// Get retrieves an organization by ID, including member and endeavour counts.
func (s *OrganizationService) Get(ctx context.Context, id string) (*storage.Organization, int, int, error) {
	if id == "" {
		return nil, 0, 0, fmt.Errorf("id is required")
	}

	org, err := s.db.GetOrganization(id)
	if err != nil {
		return nil, 0, 0, err
	}

	memberCount, _ := s.db.GetOrganizationMemberCount(id)
	endeavourCount, _ := s.db.GetOrganizationEndeavourCount(id)

	return org, memberCount, endeavourCount, nil
}

// List queries organizations with filters.
func (s *OrganizationService) List(ctx context.Context, opts storage.ListOrganizationsOpts) ([]*storage.Organization, int, error) {
	return s.db.ListOrganizations(opts)
}

// Update applies partial updates to an organization.
func (s *OrganizationService) Update(ctx context.Context, id string, fields storage.UpdateOrganizationFields) (*storage.Organization, error) {
	if id == "" {
		return nil, fmt.Errorf("id is required")
	}

	org, err := s.db.GetOrganization(id)
	if err != nil {
		return nil, err
	}

	// Read-only: archived organizations cannot be updated.
	if org.Status == "archived" {
		return nil, fmt.Errorf("cannot update archived organization")
	}

	// Validate status if provided.
	if fields.Status != nil {
		// Block "archived" via regular update -- must use Archive().
		if *fields.Status == "archived" {
			return nil, fmt.Errorf("cannot set status to archived via update; use the archive operation instead")
		}
		switch *fields.Status {
		case "active", "inactive":
			// valid
		default:
			return nil, fmt.Errorf("invalid status: %s (must be active or inactive)", *fields.Status)
		}
	}

	// Validate name is not empty if provided.
	if fields.Name != nil && *fields.Name == "" {
		return nil, fmt.Errorf("name cannot be empty")
	}

	updated, err := s.db.UpdateOrganization(id, fields)
	if err != nil {
		return nil, err
	}

	s.logger.Info("Organization updated", "id", id, "fields", updated)

	org, err = s.db.GetOrganization(id)
	if err != nil {
		return nil, fmt.Errorf("get updated organization: %w", err)
	}
	return org, nil
}

// OrgArchiveImpact represents the impact of archiving an organization.
type OrgArchiveImpact struct {
	EndeavoursToArchive int `json:"endeavours_to_archive"`
	TotalTasksToCancel  int `json:"total_tasks_to_cancel"`
	TotalPlannedTasks   int `json:"total_planned_tasks"`
	TotalActiveTasks    int `json:"total_active_tasks"`
}

// ArchiveImpact returns a dry-run summary of what archiving an organization would do.
func (s *OrganizationService) ArchiveImpact(ctx context.Context, id string) (*OrgArchiveImpact, error) {
	org, err := s.db.GetOrganization(id)
	if err != nil {
		return nil, err
	}
	if org.Status == "archived" {
		return nil, fmt.Errorf("organization is already archived")
	}

	edvIDs, err := s.db.GetOrganizationEndeavourIDs(id)
	if err != nil {
		return nil, fmt.Errorf("get linked endeavours: %w", err)
	}

	impact := &OrgArchiveImpact{}
	for _, edvID := range edvIDs {
		edv, err := s.db.GetEndeavour(edvID)
		if err != nil || edv.Status == "archived" {
			continue
		}
		impact.EndeavoursToArchive++

		progress, err := s.db.GetEndeavourTaskProgress(edvID)
		if err != nil {
			continue
		}
		impact.TotalPlannedTasks += progress.Planned
		impact.TotalActiveTasks += progress.Active
		impact.TotalTasksToCancel += progress.Planned + progress.Active
	}

	return impact, nil
}

// Archive archives an organization: archives all linked endeavours (cascading task cancellation) and sets status to archived.
func (s *OrganizationService) Archive(ctx context.Context, id, reason string) (*OrgArchiveImpact, error) {
	org, err := s.db.GetOrganization(id)
	if err != nil {
		return nil, err
	}
	if org.Status == "archived" {
		return nil, fmt.Errorf("organization is already archived")
	}

	edvIDs, err := s.db.GetOrganizationEndeavourIDs(id)
	if err != nil {
		return nil, fmt.Errorf("get linked endeavours: %w", err)
	}

	impact := &OrgArchiveImpact{}
	for _, edvID := range edvIDs {
		edv, err := s.db.GetEndeavour(edvID)
		if err != nil || edv.Status == "archived" {
			continue
		}

		canceled, err := s.edvSvc.Archive(ctx, edvID, "Organization archived: "+reason)
		if err != nil {
			return nil, fmt.Errorf("archive endeavour %s: %w", edvID, err)
		}
		impact.EndeavoursToArchive++
		impact.TotalTasksToCancel += canceled
	}

	// Set org status to archived
	archived := "archived"
	fields := storage.UpdateOrganizationFields{Status: &archived}
	if _, err := s.db.UpdateOrganization(id, fields); err != nil {
		return nil, fmt.Errorf("update organization status: %w", err)
	}

	s.logger.Info("Organization archived", "id", id, "reason", reason,
		"endeavours_archived", impact.EndeavoursToArchive, "tasks_canceled", impact.TotalTasksToCancel)
	return impact, nil
}

// AddResource adds a resource to an organization.
func (s *OrganizationService) AddResource(ctx context.Context, orgID, resourceID, role string) error {
	if orgID == "" || resourceID == "" {
		return fmt.Errorf("organization_id and resource_id are required")
	}

	// Verify org exists and is not archived
	org, err := s.db.GetOrganization(orgID)
	if err != nil {
		return err
	}
	if org.Status == "archived" {
		return fmt.Errorf("cannot add resource to archived organization")
	}

	// Verify resource exists
	if _, err := s.db.GetResource(resourceID); err != nil {
		return err
	}

	if err := s.db.AddResourceToOrganization(orgID, resourceID, role); err != nil {
		return err
	}

	s.logger.Info("Resource added to organization", "org_id", orgID, "resource_id", resourceID, "role", role)
	return nil
}

// AddEndeavour links an endeavour to an organization.
func (s *OrganizationService) AddEndeavour(ctx context.Context, orgID, endeavourID, role string) error {
	if orgID == "" || endeavourID == "" {
		return fmt.Errorf("organization_id and endeavour_id are required")
	}

	// Verify org exists and is not archived
	org, err := s.db.GetOrganization(orgID)
	if err != nil {
		return err
	}
	if org.Status == "archived" {
		return fmt.Errorf("cannot add endeavour to archived organization")
	}

	// Verify endeavour exists
	if _, err := s.db.GetEndeavour(endeavourID); err != nil {
		return err
	}

	if err := s.db.AddEndeavourToOrganization(orgID, endeavourID, role); err != nil {
		return err
	}

	s.logger.Info("Endeavour added to organization", "org_id", orgID, "endeavour_id", endeavourID, "role", role)
	return nil
}
