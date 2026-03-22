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

func TestOrganizationCRUD(t *testing.T) {
	env := newTestEnv(t)

	// Create
	rec := env.doRequest("POST", "/api/v1/organizations", map[string]interface{}{
		"name":        "Acme Corp",
		"description": "A test organization",
		"metadata":    map[string]interface{}{"plan": "enterprise"},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	data := env.parseData(rec)
	orgID := data["id"].(string)
	if data["name"] != "Acme Corp" {
		t.Errorf("name = %v, want Acme Corp", data["name"])
	}
	if data["status"] != "active" {
		t.Errorf("status = %v, want active", data["status"])
	}

	// Get
	rec = env.doRequest("GET", "/api/v1/organizations/"+orgID, nil)
	data = env.parseData(rec)
	if data["id"] != orgID {
		t.Errorf("get: id = %v, want %v", data["id"], orgID)
	}
	if data["description"] != "A test organization" {
		t.Errorf("get: description = %v, want A test organization", data["description"])
	}

	// List
	rec = env.doRequest("GET", "/api/v1/organizations", nil)
	items, meta := env.parseList(rec)
	if int(meta["total"].(float64)) < 1 {
		t.Errorf("list: total = %v, want >= 1", meta["total"])
	}
	if len(items) < 1 {
		t.Errorf("list: items = %d, want >= 1", len(items))
	}

	// Add resource to organization
	resID := env.createResource("Org Member", "human")
	rec = env.doRequest("POST", "/api/v1/organizations/"+orgID+"/resources", map[string]interface{}{
		"resource_id": resID,
		"role":        "member",
	})
	if rec.Code != http.StatusOK && rec.Code != http.StatusCreated {
		t.Fatalf("add resource: expected 200/201, got %d: %s", rec.Code, rec.Body.String())
	}

	// Add endeavour to organization (may already be auto-linked during creation)
	edvID := env.createEndeavour("Org Endeavour")
	rec = env.doRequest("POST", "/api/v1/organizations/"+orgID+"/endeavours", map[string]interface{}{
		"endeavour_id": edvID,
		"role":         "participant",
	})
	if rec.Code != http.StatusOK && rec.Code != http.StatusCreated && rec.Code != http.StatusBadRequest {
		t.Fatalf("add endeavour: expected 200/201/400, got %d: %s", rec.Code, rec.Body.String())
	}

	// Update
	rec = env.doRequest("PATCH", "/api/v1/organizations/"+orgID, map[string]interface{}{
		"name":        "Acme Corp v2",
		"description": "Updated description",
		"status":      "inactive",
	})
	data = env.parseData(rec)
	if data["name"] != "Acme Corp v2" {
		t.Errorf("update: name = %v, want Acme Corp v2", data["name"])
	}
	if data["description"] != "Updated description" {
		t.Errorf("update: description = %v, want Updated description", data["description"])
	}
	if data["status"] != "inactive" {
		t.Errorf("update: status = %v, want inactive", data["status"])
	}

	// Update not found
	rec = env.doRequest("PATCH", "/api/v1/organizations/org_nonexistent", map[string]interface{}{
		"name": "Ghost",
	})
	env.parseError(rec, http.StatusNotFound)

	// List with search
	rec = env.doRequest("GET", "/api/v1/organizations?search=Acme+Corp", nil)
	items, _ = env.parseList(rec)
	if len(items) != 1 {
		t.Errorf("list search: items = %d, want 1", len(items))
	}

	// Get not found
	rec = env.doRequest("GET", "/api/v1/organizations/org_nonexistent", nil)
	env.parseError(rec, http.StatusNotFound)
}
