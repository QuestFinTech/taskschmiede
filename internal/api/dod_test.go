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

func TestDodPolicyCRUD(t *testing.T) {
	env := newTestEnv(t)

	// List should include 4 seeded templates
	rec := env.doRequest("GET", "/api/v1/dod-policies", nil)
	items, meta := env.parseList(rec)
	if int(meta["total"].(float64)) < 4 {
		t.Errorf("expected >= 4 templates, got %v", meta["total"])
	}

	// Verify template IDs exist
	templateIDs := map[string]bool{}
	for _, item := range items {
		m := item.(map[string]interface{})
		templateIDs[m["id"].(string)] = true
	}
	for _, expected := range []string{"dod_tmpl_minimal", "dod_tmpl_peer_reviewed", "dod_tmpl_full_governance", "dod_tmpl_agent_autonomous"} {
		if !templateIDs[expected] {
			t.Errorf("missing template: %s", expected)
		}
	}

	// Create custom policy
	rec = env.doRequest("POST", "/api/v1/dod-policies", map[string]interface{}{
		"name":        "Test Policy",
		"description": "For testing",
		"conditions": []map[string]interface{}{
			{
				"id":       "cond_01",
				"type":     "comment_required",
				"label":    "At least one comment",
				"params":   map[string]interface{}{"min_comments": 1},
				"required": true,
			},
		},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	data := env.parseData(rec)
	policyID := data["id"].(string)
	if data["name"] != "Test Policy" {
		t.Errorf("name = %v, want Test Policy", data["name"])
	}
	if data["origin"] != "custom" {
		t.Errorf("origin = %v, want custom", data["origin"])
	}
	if data["version"].(float64) != 1 {
		t.Errorf("version = %v, want 1", data["version"])
	}

	// Get
	rec = env.doRequest("GET", "/api/v1/dod-policies/"+policyID, nil)
	data = env.parseData(rec)
	if data["id"] != policyID {
		t.Errorf("get: id = %v, want %v", data["id"], policyID)
	}
	conds := data["conditions"].([]interface{})
	if len(conds) != 1 {
		t.Errorf("conditions count = %d, want 1", len(conds))
	}

	// Update
	rec = env.doRequest("PATCH", "/api/v1/dod-policies/"+policyID, map[string]interface{}{
		"name": "Updated Test Policy",
	})
	data = env.parseData(rec)
	if data["name"] != "Updated Test Policy" {
		t.Errorf("update: name = %v, want Updated Test Policy", data["name"])
	}

	// Get not found
	rec = env.doRequest("GET", "/api/v1/dod-policies/dod_nonexistent", nil)
	env.parseError(rec, http.StatusNotFound)

	// List with origin filter
	rec = env.doRequest("GET", "/api/v1/dod-policies?origin=template", nil)
	items, _ = env.parseList(rec)
	for _, item := range items {
		m := item.(map[string]interface{})
		if m["origin"] != "template" {
			t.Errorf("expected origin=template, got %v for %v", m["origin"], m["id"])
		}
	}
}

func TestDodTemplateProtection(t *testing.T) {
	env := newTestEnv(t)

	// Updating a template should fail
	rec := env.doRequest("PATCH", "/api/v1/dod-policies/dod_tmpl_minimal", map[string]interface{}{
		"name": "Hacked Template",
	})
	errData := env.parseError(rec, http.StatusBadRequest)
	if errData["code"] != "invalid_input" {
		t.Errorf("expected invalid_input error, got %v", errData["code"])
	}
}

func TestDodAssignment(t *testing.T) {
	env := newTestEnv(t)

	edvID := env.createEndeavour("DoD Test Endeavour")

	// Assign template policy to endeavour
	rec := env.doRequest("POST", "/api/v1/endeavours/"+edvID+"/dod-policy", map[string]interface{}{
		"policy_id": "dod_tmpl_peer_reviewed",
	})
	data := env.parseData(rec)
	if data["status"] != "assigned" {
		t.Errorf("assign status = %v, want assigned", data["status"])
	}

	// Check dod-status
	rec = env.doRequest("GET", "/api/v1/endeavours/"+edvID+"/dod-status", nil)
	data = env.parseData(rec)
	policy := data["policy"].(map[string]interface{})
	if policy["id"] != "dod_tmpl_peer_reviewed" {
		t.Errorf("status: policy id = %v, want dod_tmpl_peer_reviewed", policy["id"])
	}

	// Replace with different policy
	rec = env.doRequest("POST", "/api/v1/endeavours/"+edvID+"/dod-policy", map[string]interface{}{
		"policy_id": "dod_tmpl_minimal",
	})
	data = env.parseData(rec)
	if data["status"] != "assigned" {
		t.Errorf("reassign status = %v, want assigned", data["status"])
	}

	// Verify replacement
	rec = env.doRequest("GET", "/api/v1/endeavours/"+edvID+"/dod-status", nil)
	data = env.parseData(rec)
	policy = data["policy"].(map[string]interface{})
	if policy["id"] != "dod_tmpl_minimal" {
		t.Errorf("after replace: policy id = %v, want dod_tmpl_minimal", policy["id"])
	}

	// Unassign
	rec = env.doRequest("DELETE", "/api/v1/endeavours/"+edvID+"/dod-policy", nil)
	data = env.parseData(rec)
	if data["status"] != "unassigned" {
		t.Errorf("unassign status = %v, want unassigned", data["status"])
	}

	// Verify unassignment
	rec = env.doRequest("GET", "/api/v1/endeavours/"+edvID+"/dod-status", nil)
	data = env.parseData(rec)
	if data["policy"] != nil {
		t.Errorf("after unassign: policy should be nil, got %v", data["policy"])
	}
}

func TestDodEndorsement(t *testing.T) {
	env := newTestEnv(t)

	edvID := env.createEndeavour("Endorsement Test")

	// Assign policy
	env.doRequest("POST", "/api/v1/endeavours/"+edvID+"/dod-policy", map[string]interface{}{
		"policy_id": "dod_tmpl_minimal",
	})

	// Endorse
	rec := env.doRequest("POST", "/api/v1/dod-endorsements", map[string]interface{}{
		"endeavour_id": edvID,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("endorse: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	data := env.parseData(rec)
	if data["policy_id"] != "dod_tmpl_minimal" {
		t.Errorf("endorsement policy_id = %v, want dod_tmpl_minimal", data["policy_id"])
	}
	if data["status"] != "active" {
		t.Errorf("endorsement status = %v, want active", data["status"])
	}

	// Duplicate endorsement should fail (conflict)
	rec = env.doRequest("POST", "/api/v1/dod-endorsements", map[string]interface{}{
		"endeavour_id": edvID,
	})
	env.parseError(rec, http.StatusConflict)

	// List endorsements
	rec = env.doRequest("GET", "/api/v1/dod-endorsements?endeavour_id="+edvID, nil)
	items, _ := env.parseList(rec)
	if len(items) != 1 {
		t.Errorf("endorsement list: expected 1, got %d", len(items))
	}

	// Verify in dod-status
	rec = env.doRequest("GET", "/api/v1/endeavours/"+edvID+"/dod-status", nil)
	data = env.parseData(rec)
	endorsements := data["endorsements"].([]interface{})
	if len(endorsements) != 1 {
		t.Errorf("status endorsements: expected 1, got %d", len(endorsements))
	}
}

func TestDodEnforcement(t *testing.T) {
	env := newTestEnv(t)

	edvID := env.createEndeavour("Enforcement Test")
	taskID := env.createTask("DoD Gated Task", edvID)

	// Create a comment_required policy
	rec := env.doRequest("POST", "/api/v1/dod-policies", map[string]interface{}{
		"name": "Comment Required",
		"conditions": []map[string]interface{}{
			{
				"id":       "cond_01",
				"type":     "comment_required",
				"label":    "Must have a comment",
				"params":   map[string]interface{}{"min_comments": 1},
				"required": true,
			},
		},
	})
	data := env.parseData(rec)
	policyID := data["id"].(string)

	// Assign policy and endorse
	env.doRequest("POST", "/api/v1/endeavours/"+edvID+"/dod-policy", map[string]interface{}{
		"policy_id": policyID,
	})
	env.doRequest("POST", "/api/v1/dod-endorsements", map[string]interface{}{
		"endeavour_id": edvID,
	})

	// Activate task
	env.doRequest("PATCH", "/api/v1/tasks/"+taskID, map[string]interface{}{
		"status": "active",
	})

	// Try to close -- should be rejected (no comments)
	rec = env.doRequest("PATCH", "/api/v1/tasks/"+taskID, map[string]interface{}{
		"status": "done",
	})
	errData := env.parseError(rec, http.StatusPreconditionFailed)
	if errData["code"] != "dod_not_met" {
		t.Errorf("expected dod_not_met, got %v", errData["code"])
	}
	// Verify details are present
	if errData["details"] == nil {
		t.Error("expected details in dod_not_met error")
	}

	// Add a comment
	env.doRequest("POST", "/api/v1/comments", map[string]interface{}{
		"entity_type": "task",
		"entity_id":   taskID,
		"content":     "Work is done.",
	})

	// Now close should succeed
	rec = env.doRequest("PATCH", "/api/v1/tasks/"+taskID, map[string]interface{}{
		"status": "done",
	})
	data = env.parseData(rec)
	if data["status"] != "done" {
		t.Errorf("after comment: status = %v, want done", data["status"])
	}
}

func TestDodCheck(t *testing.T) {
	env := newTestEnv(t)

	edvID := env.createEndeavour("Check Test")
	taskID := env.createTask("Checkable Task", edvID)

	// No policy -- check returns no_policy
	rec := env.doRequest("POST", "/api/v1/tasks/"+taskID+"/dod-check", nil)
	data := env.parseData(rec)
	if data["result"] != "no_policy" {
		t.Errorf("no policy: result = %v, want no_policy", data["result"])
	}

	// Assign and endorse
	env.doRequest("POST", "/api/v1/endeavours/"+edvID+"/dod-policy", map[string]interface{}{
		"policy_id": "dod_tmpl_peer_reviewed",
	})
	env.doRequest("POST", "/api/v1/dod-endorsements", map[string]interface{}{
		"endeavour_id": edvID,
	})

	// Check should now return structured result
	rec = env.doRequest("POST", "/api/v1/tasks/"+taskID+"/dod-check", nil)
	data = env.parseData(rec)
	if data["result"] != "not_met" {
		t.Errorf("check: result = %v, want not_met", data["result"])
	}
	if data["policy_name"] != "Peer Reviewed" {
		t.Errorf("check: policy_name = %v, want Peer Reviewed", data["policy_name"])
	}
	conditions := data["conditions"].([]interface{})
	if len(conditions) != 2 {
		t.Errorf("check: expected 2 conditions, got %d", len(conditions))
	}
}

func TestDodOverride(t *testing.T) {
	env := newTestEnv(t)

	edvID := env.createEndeavour("Override Test")
	taskID := env.createTask("Override Task", edvID)

	// Assign comment_required policy, endorse, activate
	rec := env.doRequest("POST", "/api/v1/dod-policies", map[string]interface{}{
		"name": "Strict Policy",
		"conditions": []map[string]interface{}{
			{
				"id":       "cond_01",
				"type":     "comment_required",
				"label":    "Comment needed",
				"params":   map[string]interface{}{"min_comments": 1},
				"required": true,
			},
		},
	})
	policyID := env.parseData(rec)["id"].(string)

	env.doRequest("POST", "/api/v1/endeavours/"+edvID+"/dod-policy", map[string]interface{}{
		"policy_id": policyID,
	})
	env.doRequest("POST", "/api/v1/dod-endorsements", map[string]interface{}{
		"endeavour_id": edvID,
	})
	env.doRequest("PATCH", "/api/v1/tasks/"+taskID, map[string]interface{}{
		"status": "active",
	})

	// Verify it fails without override
	rec = env.doRequest("PATCH", "/api/v1/tasks/"+taskID, map[string]interface{}{
		"status": "done",
	})
	env.parseError(rec, http.StatusPreconditionFailed)

	// Apply override
	rec = env.doRequest("POST", "/api/v1/tasks/"+taskID+"/dod-override", map[string]interface{}{
		"reason": "Emergency hotfix",
	})
	data := env.parseData(rec)
	if data["status"] != "overridden" {
		t.Errorf("override status = %v, want overridden", data["status"])
	}

	// Now closing should succeed despite failing conditions
	rec = env.doRequest("PATCH", "/api/v1/tasks/"+taskID, map[string]interface{}{
		"status": "done",
	})
	data = env.parseData(rec)
	if data["status"] != "done" {
		t.Errorf("after override: status = %v, want done", data["status"])
	}
}

func TestDodPolicyNewVersion(t *testing.T) {
	env := newTestEnv(t)

	// Create a policy
	rec := env.doRequest("POST", "/api/v1/dod-policies", map[string]interface{}{
		"name": "Versioned Policy",
		"conditions": []map[string]interface{}{
			{
				"id":       "cond_01",
				"type":     "comment_required",
				"label":    "Comment needed",
				"params":   map[string]interface{}{"min_comments": 1},
				"required": true,
			},
		},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	data := env.parseData(rec)
	policyID := data["id"].(string)
	if data["version"].(float64) != 1 {
		t.Errorf("v1: version = %v, want 1", data["version"])
	}

	// Create new version with updated conditions
	rec = env.doRequest("POST", "/api/v1/dod-policies/"+policyID+"/versions", map[string]interface{}{
		"name": "Versioned Policy v2",
		"conditions": []map[string]interface{}{
			{
				"id":       "cond_01",
				"type":     "comment_required",
				"label":    "Comment needed",
				"params":   map[string]interface{}{"min_comments": 2},
				"required": true,
			},
			{
				"id":       "cond_02",
				"type":     "peer_review",
				"label":    "Peer review",
				"params":   map[string]interface{}{"min_reviewers": 1},
				"required": true,
			},
		},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("new version: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	v2 := env.parseData(rec)
	v2ID := v2["id"].(string)
	if v2["version"].(float64) != 2 {
		t.Errorf("v2: version = %v, want 2", v2["version"])
	}
	if v2["predecessor_id"] != policyID {
		t.Errorf("v2: predecessor_id = %v, want %v", v2["predecessor_id"], policyID)
	}
	if v2["name"] != "Versioned Policy v2" {
		t.Errorf("v2: name = %v, want Versioned Policy v2", v2["name"])
	}
	conds := v2["conditions"].([]interface{})
	if len(conds) != 2 {
		t.Errorf("v2: conditions count = %d, want 2", len(conds))
	}

	// Old policy should be archived
	rec = env.doRequest("GET", "/api/v1/dod-policies/"+policyID, nil)
	oldData := env.parseData(rec)
	if oldData["status"] != "archived" {
		t.Errorf("old policy status = %v, want archived", oldData["status"])
	}

	// Lineage should show both
	rec = env.doRequest("GET", "/api/v1/dod-policies/"+v2ID+"/lineage", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("lineage: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Cannot version a template
	rec = env.doRequest("POST", "/api/v1/dod-policies/dod_tmpl_minimal/versions", map[string]interface{}{
		"conditions": []map[string]interface{}{
			{"id": "cond_01", "type": "comment_required", "label": "Comment", "params": map[string]interface{}{}, "required": true},
		},
	})
	env.parseError(rec, http.StatusBadRequest)
}

func TestDodPolicyLineage(t *testing.T) {
	env := newTestEnv(t)

	// Get lineage of a template (should return just itself)
	// The lineage endpoint returns {"data": [array of policies]}, so parseData
	// gives us the outer wrapper. The data value is actually the array.
	rec := env.doRequest("GET", "/api/v1/dod-policies/dod_tmpl_minimal/lineage", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("lineage: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Lineage not found
	rec = env.doRequest("GET", "/api/v1/dod-policies/dod_nonexistent/lineage", nil)
	env.parseError(rec, http.StatusNotFound)
}
