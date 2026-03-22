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

func TestEndeavourCRUD(t *testing.T) {
	env := newTestEnv(t)

	// Create
	rec := env.doRequest("POST", "/api/v1/endeavours", map[string]interface{}{
		"name":        "Project Alpha",
		"description": "First test endeavour",
		"goals":       []string{"ship v1", "onboard users"},
		"start_date":  "2026-01-01T00:00:00Z",
		"end_date":    "2026-06-30T00:00:00Z",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	data := env.parseData(rec)
	edvID := data["id"].(string)
	if data["name"] != "Project Alpha" {
		t.Errorf("name = %v, want Project Alpha", data["name"])
	}
	if data["status"] != "active" {
		t.Errorf("status = %v, want active", data["status"])
	}

	// Get
	rec = env.doRequest("GET", "/api/v1/endeavours/"+edvID, nil)
	data = env.parseData(rec)
	if data["id"] != edvID {
		t.Errorf("get: id = %v, want %v", data["id"], edvID)
	}
	if data["description"] != "First test endeavour" {
		t.Errorf("get: description = %v, want First test endeavour", data["description"])
	}

	// List
	rec = env.doRequest("GET", "/api/v1/endeavours", nil)
	items, meta := env.parseList(rec)
	total := int(meta["total"].(float64))
	if total < 1 {
		t.Errorf("list: total = %d, want >= 1", total)
	}
	if len(items) < 1 {
		t.Errorf("list: items = %d, want >= 1", len(items))
	}

	// Update
	rec = env.doRequest("PATCH", "/api/v1/endeavours/"+edvID, map[string]interface{}{
		"name":   "Project Alpha v2",
		"status": "completed",
	})
	data = env.parseData(rec)
	if data["name"] != "Project Alpha v2" {
		t.Errorf("update: name = %v, want Project Alpha v2", data["name"])
	}
	if data["status"] != "completed" {
		t.Errorf("update: status = %v, want completed", data["status"])
	}

	// List with status filter
	rec = env.doRequest("GET", "/api/v1/endeavours?status=completed", nil)
	items, _ = env.parseList(rec)
	if len(items) != 1 {
		t.Errorf("list completed: items = %d, want 1", len(items))
	}

	// List with search
	rec = env.doRequest("GET", "/api/v1/endeavours?search=Alpha", nil)
	items, _ = env.parseList(rec)
	if len(items) != 1 {
		t.Errorf("list search Alpha: items = %d, want 1", len(items))
	}

	// Get not found
	rec = env.doRequest("GET", "/api/v1/endeavours/edv_nonexistent", nil)
	env.parseError(rec, http.StatusNotFound)
}
