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

func TestTaskCRUD(t *testing.T) {
	env := newTestEnv(t)

	// Create prerequisites
	edvID := env.createEndeavour("Task Test Endeavour")
	resID := env.createResource("Task Worker", "human")

	// Create
	rec := env.doRequest("POST", "/api/v1/tasks", map[string]interface{}{
		"title":        "Implement feature X",
		"description":  "Build the thing",
		"endeavour_id": edvID,
		"assignee_id":  resID,
		"estimate":     8.0,
		"due_date":     "2026-03-01T00:00:00Z",
		"metadata":     map[string]interface{}{"priority": "high"},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	data := env.parseData(rec)
	taskID := data["id"].(string)
	if data["title"] != "Implement feature X" {
		t.Errorf("title = %v, want Implement feature X", data["title"])
	}
	if data["status"] != "planned" {
		t.Errorf("status = %v, want planned", data["status"])
	}
	if data["endeavour_id"] != edvID {
		t.Errorf("endeavour_id = %v, want %v", data["endeavour_id"], edvID)
	}
	if data["assignee_id"] != resID {
		t.Errorf("assignee_id = %v, want %v", data["assignee_id"], resID)
	}

	// Get
	rec = env.doRequest("GET", "/api/v1/tasks/"+taskID, nil)
	data = env.parseData(rec)
	if data["id"] != taskID {
		t.Errorf("get: id = %v, want %v", data["id"], taskID)
	}
	if data["assignee_name"] != "Task Worker" {
		t.Errorf("get: assignee_name = %v, want Task Worker", data["assignee_name"])
	}

	// List by endeavour
	rec = env.doRequest("GET", "/api/v1/tasks?endeavour_id="+edvID, nil)
	items, meta := env.parseList(rec)
	if int(meta["total"].(float64)) != 1 {
		t.Errorf("list by endeavour: total = %v, want 1", meta["total"])
	}
	if len(items) != 1 {
		t.Errorf("list by endeavour: items = %d, want 1", len(items))
	}

	// Summary mode
	rec = env.doRequest("GET", "/api/v1/tasks?summary=true", nil)
	data = env.parseData(rec)
	if data["planned"] != 1.0 {
		t.Errorf("summary: planned = %v, want 1", data["planned"])
	}

	// Update: planned -> active
	rec = env.doRequest("PATCH", "/api/v1/tasks/"+taskID, map[string]interface{}{
		"status": "active",
	})
	data = env.parseData(rec)
	if data["status"] != "active" {
		t.Errorf("update to active: status = %v, want active", data["status"])
	}
	if data["started_at"] == nil {
		t.Error("update to active: started_at should be set")
	}

	// Update: active -> done
	rec = env.doRequest("PATCH", "/api/v1/tasks/"+taskID, map[string]interface{}{
		"status": "done",
		"actual": 6.5,
	})
	data = env.parseData(rec)
	if data["status"] != "done" {
		t.Errorf("update to done: status = %v, want done", data["status"])
	}
	if data["completed_at"] == nil {
		t.Error("update to done: completed_at should be set")
	}
	if data["actual"] != 6.5 {
		t.Errorf("update actual = %v, want 6.5", data["actual"])
	}

	// Invalid transition: done -> planned
	rec = env.doRequest("PATCH", "/api/v1/tasks/"+taskID, map[string]interface{}{
		"status": "planned",
	})
	env.parseError(rec, http.StatusBadRequest)

	// Cancel without reason should fail
	task2ID := env.createTask("Task to cancel", edvID)
	rec = env.doRequest("PATCH", "/api/v1/tasks/"+task2ID, map[string]interface{}{
		"status": "canceled",
	})
	env.parseError(rec, http.StatusBadRequest)

	// Cancel with reason should succeed
	rec = env.doRequest("PATCH", "/api/v1/tasks/"+task2ID, map[string]interface{}{
		"status":          "canceled",
		"canceled_reason": "No longer needed",
	})
	data = env.parseData(rec)
	if data["status"] != "canceled" {
		t.Errorf("cancel: status = %v, want canceled", data["status"])
	}

	// List by status
	rec = env.doRequest("GET", "/api/v1/tasks?status=done", nil)
	items, _ = env.parseList(rec)
	if len(items) != 1 {
		t.Errorf("list done: items = %d, want 1", len(items))
	}

	// Create with missing title
	rec = env.doRequest("POST", "/api/v1/tasks", map[string]interface{}{
		"description": "no title",
	})
	env.parseError(rec, http.StatusBadRequest)

	// Get not found
	rec = env.doRequest("GET", "/api/v1/tasks/tsk_nonexistent", nil)
	env.parseError(rec, http.StatusNotFound)
}
