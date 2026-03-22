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


package api

import (
	"context"
	"net/http"

	"github.com/QuestFinTech/taskschmiede/internal/storage"
)

// handleEndeavourExport exports all data for an endeavour as JSON.
// Requires endeavour owner role.
func (a *API) handleEndeavourExport(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	export, apiErr := a.ExportEndeavour(r.Context(), id)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	w.Header().Set("Content-Disposition", "attachment; filename=\""+id+".json\"")
	writeData(w, http.StatusOK, export)
}

// handleOrganizationExport exports all data for an organization as JSON.
// Requires organization owner role.
func (a *API) handleOrganizationExport(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	export, apiErr := a.ExportOrganization(r.Context(), id)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	w.Header().Set("Content-Disposition", "attachment; filename=\""+id+".json\"")
	writeData(w, http.StatusOK, export)
}

// ExportEndeavour is the business-logic method used by both REST and MCP.
func (a *API) ExportEndeavour(ctx context.Context, id string) (*storage.EndeavourExport, *APIError) {
	scope, apiErr := a.resolveScope(ctx)
	if apiErr != nil {
		return nil, apiErr
	}
	if apiErr := checkEndeavourOwner(scope, id); apiErr != nil {
		return nil, apiErr
	}

	export, err := a.db.ExportEndeavourData(id)
	if err != nil {
		return nil, errNotFound("endeavour", "Endeavour not found")
	}

	if a.messageDB != nil {
		msgs, dels := a.messageDB.ExportEndeavourMessages(id)
		if msgs != nil {
			export.Messages = msgs
		}
		if dels != nil {
			export.Deliveries = dels
		}
	}

	return export, nil
}

// ExportOrganization is the business-logic method used by both REST and MCP.
func (a *API) ExportOrganization(ctx context.Context, id string) (*storage.OrgExport, *APIError) {
	scope, apiErr := a.resolveScope(ctx)
	if apiErr != nil {
		return nil, apiErr
	}
	if !scope.CanOwnerOrg(id) {
		return nil, errNotFound("organization", "Organization not found")
	}

	export, err := a.db.ExportOrganizationData(id, a.messageDB)
	if err != nil {
		return nil, errNotFound("organization", "Organization not found")
	}

	return export, nil
}
