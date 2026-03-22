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
	"testing"
)

func TestDemandCRUD(t *testing.T) {
	env := newTestEnv(t)

	edvID := env.createEndeavour("Demand Test Endeavour")

	// Create
	rec := env.doRequest("POST", "/api/v1/demands", map[string]interface{}{
		"type":         "feature",
		"title":        "Add dark mode",
		"description":  "Users want dark mode",
		"priority":     "high",
		"endeavour_id": edvID,
		"due_date":     "2026-04-01T00:00:00Z",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	data := env.parseData(rec)
	dmdID := data["id"].(string)
	if data["type"] != "feature" {
		t.Errorf("type = %v, want feature", data["type"])
	}
	if data["title"] != "Add dark mode" {
		t.Errorf("title = %v, want Add dark mode", data["title"])
	}
	if data["status"] != "open" {
		t.Errorf("status = %v, want open", data["status"])
	}
	if data["priority"] != "high" {
		t.Errorf("priority = %v, want high", data["priority"])
	}

	// Get
	rec = env.doRequest("GET", "/api/v1/demands/"+dmdID, nil)
	data = env.parseData(rec)
	if data["id"] != dmdID {
		t.Errorf("get: id = %v, want %v", data["id"], dmdID)
	}
	if data["endeavour_id"] != edvID {
		t.Errorf("get: endeavour_id = %v, want %v", data["endeavour_id"], edvID)
	}

	// List
	rec = env.doRequest("GET", "/api/v1/demands", nil)
	items, meta := env.parseList(rec)
	if int(meta["total"].(float64)) < 1 {
		t.Errorf("list: total = %v, want >= 1", meta["total"])
	}
	if len(items) < 1 {
		t.Errorf("list: items = %d, want >= 1", len(items))
	}

	// List with filters
	rec = env.doRequest("GET", "/api/v1/demands?priority=high", nil)
	items, _ = env.parseList(rec)
	if len(items) != 1 {
		t.Errorf("list priority=high: items = %d, want 1", len(items))
	}

	rec = env.doRequest("GET", "/api/v1/demands?type=feature", nil)
	items, _ = env.parseList(rec)
	if len(items) != 1 {
		t.Errorf("list type=feature: items = %d, want 1", len(items))
	}

	// Update: open -> in_progress
	rec = env.doRequest("PATCH", "/api/v1/demands/"+dmdID, map[string]interface{}{
		"status": "in_progress",
	})
	data = env.parseData(rec)
	if data["status"] != "in_progress" {
		t.Errorf("update to in_progress: status = %v, want in_progress", data["status"])
	}

	// Update: in_progress -> fulfilled
	rec = env.doRequest("PATCH", "/api/v1/demands/"+dmdID, map[string]interface{}{
		"status": "fulfilled",
	})
	data = env.parseData(rec)
	if data["status"] != "fulfilled" {
		t.Errorf("update to fulfilled: status = %v, want fulfilled", data["status"])
	}

	// Create a second demand and cancel it
	rec = env.doRequest("POST", "/api/v1/demands", map[string]interface{}{
		"type":  "bug",
		"title": "Bug to cancel",
	})
	data = env.parseData(rec)
	dmd2ID := data["id"].(string)

	// Cancel without reason should fail
	rec = env.doRequest("PATCH", "/api/v1/demands/"+dmd2ID, map[string]interface{}{
		"status": "canceled",
	})
	env.parseError(rec, http.StatusBadRequest)

	// Cancel with reason should succeed
	rec = env.doRequest("PATCH", "/api/v1/demands/"+dmd2ID, map[string]interface{}{
		"status":          "canceled",
		"canceled_reason": "Duplicate",
	})
	data = env.parseData(rec)
	if data["status"] != "canceled" {
		t.Errorf("cancel: status = %v, want canceled", data["status"])
	}

	// List by status
	rec = env.doRequest("GET", "/api/v1/demands?status=fulfilled", nil)
	items, _ = env.parseList(rec)
	if len(items) != 1 {
		t.Errorf("list fulfilled: items = %d, want 1", len(items))
	}

	// Get not found
	rec = env.doRequest("GET", "/api/v1/demands/dmd_nonexistent", nil)
	env.parseError(rec, http.StatusNotFound)
}
