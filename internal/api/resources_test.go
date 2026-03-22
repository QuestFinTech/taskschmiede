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

func TestResourceCRUD(t *testing.T) {
	env := newTestEnv(t)

	// Create
	rec := env.doRequest("POST", "/api/v1/resources", map[string]interface{}{
		"type":           "agent",
		"name":           "Test Agent",
		"capacity_model": "tokens_per_day",
		"capacity_value": 100000,
		"skills":         []string{"coding", "testing"},
		"metadata":       map[string]interface{}{"model": "gpt-4"},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	data := env.parseData(rec)
	resID := data["id"].(string)
	if data["type"] != "agent" {
		t.Errorf("type = %v, want agent", data["type"])
	}
	if data["name"] != "Test Agent" {
		t.Errorf("name = %v, want Test Agent", data["name"])
	}
	if data["capacity_model"] != "tokens_per_day" {
		t.Errorf("capacity_model = %v, want tokens_per_day", data["capacity_model"])
	}
	if data["status"] != "active" {
		t.Errorf("status = %v, want active", data["status"])
	}

	// Get
	rec = env.doRequest("GET", "/api/v1/resources/"+resID, nil)
	data = env.parseData(rec)
	if data["id"] != resID {
		t.Errorf("get: id = %v, want %v", data["id"], resID)
	}
	if data["name"] != "Test Agent" {
		t.Errorf("get: name = %v, want Test Agent", data["name"])
	}
	skills, ok := data["skills"].([]interface{})
	if !ok || len(skills) != 2 {
		t.Errorf("get: skills = %v, want [coding testing]", data["skills"])
	}

	// List (admin mode: includes sys_taskschmiede + admin resource + new resource)
	rec = env.doRequest("GET", "/api/v1/resources?admin=true", nil)
	items, meta := env.parseList(rec)
	total := int(meta["total"].(float64))
	if total < 3 {
		t.Errorf("list: total = %d, want >= 3", total)
	}
	if len(items) < 3 {
		t.Errorf("list: items = %d, want >= 3", len(items))
	}

	// List with type filter
	rec = env.doRequest("GET", "/api/v1/resources?admin=true&type=agent", nil)
	items, _ = env.parseList(rec)
	if len(items) != 1 {
		t.Errorf("list type=agent: items = %d, want 1", len(items))
	}

	// List with search
	rec = env.doRequest("GET", "/api/v1/resources?admin=true&search=Test+Agent", nil)
	items, _ = env.parseList(rec)
	if len(items) != 1 {
		t.Errorf("list search: items = %d, want 1", len(items))
	}

	// Update
	rec = env.doRequest("PATCH", "/api/v1/resources/"+resID, map[string]interface{}{
		"name":   "Updated Agent",
		"status": "inactive",
	})
	data = env.parseData(rec)
	if data["name"] != "Updated Agent" {
		t.Errorf("update: name = %v, want Updated Agent", data["name"])
	}
	if data["status"] != "inactive" {
		t.Errorf("update: status = %v, want inactive", data["status"])
	}

	// Get not found
	rec = env.doRequest("GET", "/api/v1/resources/res_nonexistent", nil)
	env.parseError(rec, http.StatusNotFound)

	// Create with missing required field
	rec = env.doRequest("POST", "/api/v1/resources", map[string]interface{}{
		"name": "No Type",
	})
	env.parseError(rec, http.StatusBadRequest)
}
