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
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/QuestFinTech/taskschmiede/internal/storage"
)

// CreateRitualRun creates a new ritual run and returns its map representation.
func (a *API) CreateRitualRun(ctx context.Context, ritualID, trigger, runBy string, metadata map[string]interface{}) (map[string]interface{}, *APIError) {
	if apiErr := a.CheckCreationVelocity(ctx); apiErr != nil {
		return nil, apiErr
	}
	// RBAC: require write access to the ritual's endeavour
	ritual, err := a.rtlSvc.Get(ctx, ritualID)
	if err != nil {
		return nil, errNotFound("ritual", "Ritual not found")
	}
	scope, apiErr := a.resolveScope(ctx)
	if apiErr != nil {
		return nil, apiErr
	}
	if apiErr := checkEndeavourWrite(scope, ritual.EndeavourID); apiErr != nil {
		return nil, errNotFound("ritual", "Ritual not found")
	}
	run, err := a.rtrSvc.Create(ctx, ritualID, trigger, runBy, metadata)
	if err != nil {
		return nil, errInvalidInput(err.Error())
	}
	return ritualRunToMap(run), nil
}

// GetRitualRun retrieves a ritual run by ID and returns its map representation.
func (a *API) GetRitualRun(ctx context.Context, id string) (map[string]interface{}, *APIError) {
	run, err := a.rtrSvc.Get(ctx, id)
	if err != nil {
		if errors.Is(err, storage.ErrRitualRunNotFound) {
			return nil, errNotFound("ritual_run", "Ritual run not found")
		}
		return nil, errInternal("Failed to get ritual run")
	}
	// RBAC: require read access to the ritual's endeavour
	ritual, ritualErr := a.rtlSvc.Get(ctx, run.RitualID)
	if ritualErr != nil {
		// Ritual deleted: only master admin can see orphaned runs
		scope, apiErr := a.resolveScope(ctx)
		if apiErr != nil {
			return nil, apiErr
		}
		if !scope.IsMasterAdmin {
			return nil, errNotFound("ritual_run", "Ritual run not found")
		}
	} else {
		scope, apiErr := a.resolveScope(ctx)
		if apiErr != nil {
			return nil, apiErr
		}
		if apiErr := checkEndeavourRead(scope, ritual.EndeavourID); apiErr != nil {
			return nil, errNotFound("ritual_run", "Ritual run not found")
		}
	}
	return ritualRunToMap(run), nil
}

// ListRitualRuns queries ritual runs with the given options and returns their map representations.
func (a *API) ListRitualRuns(ctx context.Context, opts storage.ListRitualRunsOpts) ([]map[string]interface{}, int, *APIError) {
	runs, total, err := a.rtrSvc.List(ctx, opts)
	if err != nil {
		return nil, 0, errInternal("Failed to query ritual runs")
	}

	items := make([]map[string]interface{}, 0, len(runs))
	for _, run := range runs {
		items = append(items, ritualRunToMap(run))
	}
	return items, total, nil
}

// UpdateRitualRun updates a ritual run and returns the updated map representation.
func (a *API) UpdateRitualRun(ctx context.Context, id string, fields storage.UpdateRitualRunFields) (map[string]interface{}, *APIError) {
	// RBAC: require write access to the ritual's endeavour
	run, err := a.rtrSvc.Get(ctx, id)
	if err != nil {
		if errors.Is(err, storage.ErrRitualRunNotFound) {
			return nil, errNotFound("ritual_run", "Ritual run not found")
		}
		return nil, errInternal("Failed to get ritual run")
	}
	ritual, ritualErr := a.rtlSvc.Get(ctx, run.RitualID)
	if ritualErr != nil {
		scope, apiErr := a.resolveScope(ctx)
		if apiErr != nil {
			return nil, apiErr
		}
		if !scope.IsMasterAdmin {
			return nil, errNotFound("ritual_run", "Ritual run not found")
		}
	} else {
		scope, apiErr := a.resolveScope(ctx)
		if apiErr != nil {
			return nil, apiErr
		}
		if apiErr := checkEndeavourWrite(scope, ritual.EndeavourID); apiErr != nil {
			return nil, errNotFound("ritual_run", "Ritual run not found")
		}
	}
	_, err = a.rtrSvc.Update(ctx, id, fields)
	if err != nil {
		if errors.Is(err, storage.ErrRitualRunNotFound) {
			return nil, errNotFound("ritual_run", "Ritual run not found")
		}
		return nil, errInvalidInput(err.Error())
	}

	run, err = a.rtrSvc.Get(ctx, id)
	if err != nil {
		return nil, errInternal("Failed to get updated ritual run")
	}
	return ritualRunToMap(run), nil
}

func (a *API) handleRitualRunCreate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RitualID string                 `json:"ritual_id"`
		Trigger  string                 `json:"trigger"`
		Metadata map[string]interface{} `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Invalid JSON body")
		return
	}

	if body.Trigger == "" {
		body.Trigger = "manual"
	}

	authUser := getAuthUser(r)
	runBy := ""
	if authUser != nil {
		runBy = authUser.UserID
	}

	result, apiErr := a.CreateRitualRun(r.Context(), body.RitualID, body.Trigger, runBy, body.Metadata)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	writeData(w, http.StatusCreated, result)
}

func (a *API) handleRitualRunList(w http.ResponseWriter, r *http.Request) {
	opts := storage.ListRitualRunsOpts{
		RitualID:     queryString(r, "ritual_id"),
		EndeavourIDs: a.resolveEndeavourIDs(r),
		Status:       queryString(r, "status"),
		Limit:        queryInt(r, "limit", 50),
		Offset:       queryInt(r, "offset", 0),
	}

	items, total, apiErr := a.ListRitualRuns(r.Context(), opts)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	writeList(w, items, total, opts.Limit, opts.Offset)
}

func (a *API) handleRitualRunGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	result, apiErr := a.GetRitualRun(r.Context(), id)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	writeData(w, http.StatusOK, result)
}

func (a *API) handleRitualRunUpdate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var body struct {
		Status        *string                `json:"status"`
		ResultSummary *string                `json:"result_summary"`
		Effects       map[string]interface{} `json:"effects"`
		Error         map[string]interface{} `json:"error"`
		Metadata      map[string]interface{} `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Invalid JSON body")
		return
	}

	fields := storage.UpdateRitualRunFields{
		Status:        body.Status,
		ResultSummary: body.ResultSummary,
		Effects:       body.Effects,
		Error:         body.Error,
		Metadata:      body.Metadata,
	}

	result, apiErr := a.UpdateRitualRun(r.Context(), id, fields)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	writeData(w, http.StatusOK, result)
}

func ritualRunToMap(r *storage.RitualRun) map[string]interface{} {
	m := map[string]interface{}{
		"id":         r.ID,
		"ritual_id":  r.RitualID,
		"status":     r.Status,
		"trigger":    r.Trigger,
		"metadata":   r.Metadata,
		"created_at": r.CreatedAt.Format(time.RFC3339),
		"updated_at": r.UpdatedAt.Format(time.RFC3339),
	}
	if r.RunBy != "" {
		m["run_by"] = r.RunBy
	}
	if r.ResultSummary != "" {
		m["result_summary"] = r.ResultSummary
	}
	if r.Effects != nil {
		m["effects"] = r.Effects
	}
	if r.Error != nil {
		m["error"] = r.Error
	}
	if r.StartedAt != nil {
		m["started_at"] = r.StartedAt.Format(time.RFC3339)
	}
	if r.FinishedAt != nil {
		m["finished_at"] = r.FinishedAt.Format(time.RFC3339)
	}
	return m
}
