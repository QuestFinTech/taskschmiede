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
	"encoding/json"
	"net/http"
	"testing"
)

func TestRitualCRUD(t *testing.T) {
	env := newTestEnv(t)

	// Create
	rec := env.doRequest("POST", "/api/v1/rituals", map[string]interface{}{
		"name":        "Weekly Planning",
		"description": "Shape Up style weekly planning",
		"prompt":      "Review all active tasks. Identify blockers. Prioritize for the week.",
		"schedule":    map[string]interface{}{"type": "cron", "cron": "0 9 * * 1"},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	data := env.parseData(rec)
	rtlID := data["id"].(string)
	if data["name"] != "Weekly Planning" {
		t.Errorf("name = %v, want Weekly Planning", data["name"])
	}
	if data["status"] != "active" {
		t.Errorf("status = %v, want active", data["status"])
	}
	if data["origin"] != "custom" {
		t.Errorf("origin = %v, want custom", data["origin"])
	}

	// Get
	rec = env.doRequest("GET", "/api/v1/rituals/"+rtlID, nil)
	data = env.parseData(rec)
	if data["id"] != rtlID {
		t.Errorf("get: id = %v, want %v", data["id"], rtlID)
	}
	if data["prompt"] != "Review all active tasks. Identify blockers. Prioritize for the week." {
		t.Errorf("get: prompt mismatch")
	}

	// List
	rec = env.doRequest("GET", "/api/v1/rituals", nil)
	items, meta := env.parseList(rec)
	if int(meta["total"].(float64)) < 1 {
		t.Errorf("list: total = %v, want >= 1", meta["total"])
	}
	if len(items) < 1 {
		t.Errorf("list: items = %d, want >= 1", len(items))
	}

	// List with search (use specific term to avoid matching seeded templates)
	rec = env.doRequest("GET", "/api/v1/rituals?search=Shape+Up+style", nil)
	items, _ = env.parseList(rec)
	if len(items) != 1 {
		t.Errorf("list search: items = %d, want 1", len(items))
	}

	// Update
	rec = env.doRequest("PATCH", "/api/v1/rituals/"+rtlID, map[string]interface{}{
		"name": "Weekly Planning v2",
	})
	data = env.parseData(rec)
	if data["name"] != "Weekly Planning v2" {
		t.Errorf("update: name = %v, want Weekly Planning v2", data["name"])
	}

	// Fork
	rec = env.doRequest("POST", "/api/v1/rituals/"+rtlID+"/fork", map[string]interface{}{
		"name":   "Daily Standup",
		"prompt": "Quick check: what did you do yesterday? What today? Any blockers?",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("fork: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	forkData := env.parseData(rec)
	forkID := forkData["id"].(string)
	if forkData["name"] != "Daily Standup" {
		t.Errorf("fork: name = %v, want Daily Standup", forkData["name"])
	}
	if forkData["origin"] != "fork" {
		t.Errorf("fork: origin = %v, want fork", forkData["origin"])
	}

	// Lineage (returns data as array directly)
	rec = env.doRequest("GET", "/api/v1/rituals/"+forkID+"/lineage", nil)
	if rec.Code < 200 || rec.Code >= 300 {
		t.Fatalf("lineage: expected 2xx, got %d: %s", rec.Code, rec.Body.String())
	}
	var lineageResp struct {
		Data []interface{} `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&lineageResp); err != nil {
		t.Fatalf("lineage: decode: %v", err)
	}
	if len(lineageResp.Data) < 2 {
		t.Errorf("lineage: expected at least 2 entries, got %d", len(lineageResp.Data))
	}

	// Get not found
	rec = env.doRequest("GET", "/api/v1/rituals/rtl_nonexistent", nil)
	env.parseError(rec, http.StatusNotFound)
}

func TestRitualTemplateProtection(t *testing.T) {
	env := newTestEnv(t)

	// Updating a template should fail
	rec := env.doRequest("PATCH", "/api/v1/rituals/rtl_tmpl_task_review", map[string]interface{}{
		"name": "Hacked Template",
	})
	errData := env.parseError(rec, http.StatusBadRequest)
	if errData["code"] != "invalid_input" {
		t.Errorf("expected invalid_input error, got %v", errData["code"])
	}

	// Forking a template should still work
	rec = env.doRequest("POST", "/api/v1/rituals/rtl_tmpl_task_review/fork", map[string]interface{}{
		"name": "My Custom Task Review",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("fork template: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	forkData := env.parseData(rec)
	if forkData["origin"] != "fork" {
		t.Errorf("fork: origin = %v, want fork", forkData["origin"])
	}
}

func TestRitualRunCRUD(t *testing.T) {
	env := newTestEnv(t)

	rtlID := env.createRitual("Run Test Ritual", "Do the thing")

	// Create run
	rec := env.doRequest("POST", "/api/v1/ritual-runs", map[string]interface{}{
		"ritual_id": rtlID,
		"trigger":   "manual",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create run: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	data := env.parseData(rec)
	runID := data["id"].(string)
	if data["status"] != "running" {
		t.Errorf("status = %v, want running", data["status"])
	}
	if data["ritual_id"] != rtlID {
		t.Errorf("ritual_id = %v, want %v", data["ritual_id"], rtlID)
	}

	// Get run
	rec = env.doRequest("GET", "/api/v1/ritual-runs/"+runID, nil)
	data = env.parseData(rec)
	if data["id"] != runID {
		t.Errorf("get: id = %v, want %v", data["id"], runID)
	}

	// Update run: complete it
	rec = env.doRequest("PATCH", "/api/v1/ritual-runs/"+runID, map[string]interface{}{
		"status":         "succeeded",
		"result_summary": "All tasks reviewed, 3 blockers identified",
	})
	data = env.parseData(rec)
	if data["status"] != "succeeded" {
		t.Errorf("update: status = %v, want succeeded", data["status"])
	}

	// List runs
	rec = env.doRequest("GET", "/api/v1/ritual-runs?admin=true&ritual_id="+rtlID, nil)
	items, meta := env.parseList(rec)
	if int(meta["total"].(float64)) != 1 {
		t.Errorf("list: total = %v, want 1", meta["total"])
	}
	if len(items) != 1 {
		t.Errorf("list: items = %d, want 1", len(items))
	}

	// List with status filter
	rec = env.doRequest("GET", "/api/v1/ritual-runs?admin=true&status=succeeded", nil)
	items, _ = env.parseList(rec)
	if len(items) != 1 {
		t.Errorf("list succeeded: items = %d, want 1", len(items))
	}

	// Get not found
	rec = env.doRequest("GET", "/api/v1/ritual-runs/rtr_nonexistent", nil)
	env.parseError(rec, http.StatusNotFound)
}
