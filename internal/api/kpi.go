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
	"net/http"
)

// handleKPICurrent returns the most recent KPI snapshot.
func (a *API) handleKPICurrent(w http.ResponseWriter, r *http.Request) {
	if !a.requireAdmin(w, r) {
		return
	}

	snap, err := a.db.LatestKPISnapshot()
	if err != nil {
		a.logger.Error("Failed to get latest KPI snapshot", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get KPI data")
		return
	}
	if snap == nil {
		writeError(w, http.StatusNotFound, "not_found", "No KPI snapshots available")
		return
	}

	// Include snapshot metadata in response
	result := snap.Data
	if result == nil {
		result = make(map[string]interface{})
	}
	result["id"] = snap.ID
	result["timestamp"] = snap.Timestamp

	writeData(w, http.StatusOK, result)
}

// handleKPIHistory returns KPI snapshots within a time range.
// Query params: since, until (RFC3339), limit (default 50).
func (a *API) handleKPIHistory(w http.ResponseWriter, r *http.Request) {
	if !a.requireAdmin(w, r) {
		return
	}

	since := queryString(r, "since")
	until := queryString(r, "until")
	limit := queryInt(r, "limit", 50)

	snapshots, total, err := a.db.ListKPISnapshots(since, until, limit)
	if err != nil {
		a.logger.Error("Failed to list KPI snapshots", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to query KPI history")
		return
	}

	// Convert to response format
	var results []map[string]interface{}
	for _, s := range snapshots {
		entry := s.Data
		if entry == nil {
			entry = make(map[string]interface{})
		}
		entry["id"] = s.ID
		entry["timestamp"] = s.Timestamp
		results = append(results, entry)
	}

	writeList(w, results, total, limit, 0)
}
