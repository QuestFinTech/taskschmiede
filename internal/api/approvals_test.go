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

func TestApprovalCRUD(t *testing.T) {
	env := newTestEnv(t)

	// Create a task to approve
	taskID := env.createTask("Approvable Task", "")

	// Create approval
	rec := env.doRequest("POST", "/api/v1/approvals", map[string]interface{}{
		"entity_type": "task",
		"entity_id":   taskID,
		"verdict":     "approved",
		"role":        "reviewer",
		"comment":     "Looks good to me",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	data := env.parseData(rec)
	aprID := data["id"].(string)
	if data["entity_type"] != "task" {
		t.Errorf("entity_type = %v, want task", data["entity_type"])
	}
	if data["entity_id"] != taskID {
		t.Errorf("entity_id = %v, want %v", data["entity_id"], taskID)
	}
	if data["verdict"] != "approved" {
		t.Errorf("verdict = %v, want approved", data["verdict"])
	}
	if data["role"] != "reviewer" {
		t.Errorf("role = %v, want reviewer", data["role"])
	}
	if data["approver_id"] != env.adminResourceID {
		t.Errorf("approver_id = %v, want %v", data["approver_id"], env.adminResourceID)
	}

	// Get approval
	rec = env.doRequest("GET", "/api/v1/approvals/"+aprID, nil)
	data = env.parseData(rec)
	if data["id"] != aprID {
		t.Errorf("get: id = %v, want %v", data["id"], aprID)
	}
	if data["approver_name"] != "Test Admin" {
		t.Errorf("get: approver_name = %v, want Test Admin", data["approver_name"])
	}
	if data["comment"] != "Looks good to me" {
		t.Errorf("get: comment = %v, want Looks good to me", data["comment"])
	}

	// Create second approval (different verdict)
	rec = env.doRequest("POST", "/api/v1/approvals", map[string]interface{}{
		"entity_type": "task",
		"entity_id":   taskID,
		"verdict":     "needs_work",
		"role":        "stakeholder",
		"comment":     "Need more tests",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create second: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	// List approvals for entity
	rec = env.doRequest("GET", "/api/v1/approvals?entity_type=task&entity_id="+taskID, nil)
	items, meta := env.parseList(rec)
	if int(meta["total"].(float64)) != 2 {
		t.Errorf("list: total = %v, want 2", meta["total"])
	}
	if len(items) != 2 {
		t.Errorf("list: items = %d, want 2", len(items))
	}
	// Verify both verdicts are present
	verdicts := map[string]bool{}
	for _, item := range items {
		m := item.(map[string]interface{})
		verdicts[m["verdict"].(string)] = true
	}
	if !verdicts["approved"] || !verdicts["needs_work"] {
		t.Errorf("list: expected both verdicts, got %v", verdicts)
	}

	// List with verdict filter
	rec = env.doRequest("GET", "/api/v1/approvals?entity_type=task&entity_id="+taskID+"&verdict=approved", nil)
	items, _ = env.parseList(rec)
	if len(items) != 1 {
		t.Errorf("list verdict=approved: items = %d, want 1", len(items))
	}

	// Create with invalid verdict
	rec = env.doRequest("POST", "/api/v1/approvals", map[string]interface{}{
		"entity_type": "task",
		"entity_id":   taskID,
		"verdict":     "invalid_verdict",
	})
	env.parseError(rec, http.StatusBadRequest)

	// Create with missing verdict
	rec = env.doRequest("POST", "/api/v1/approvals", map[string]interface{}{
		"entity_type": "task",
		"entity_id":   taskID,
	})
	env.parseError(rec, http.StatusBadRequest)

	// Get not found
	rec = env.doRequest("GET", "/api/v1/approvals/apr_nonexistent", nil)
	env.parseError(rec, http.StatusNotFound)
}
