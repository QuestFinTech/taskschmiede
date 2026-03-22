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

func TestRelationCRUD(t *testing.T) {
	env := newTestEnv(t)

	// Create prerequisites.
	// Note: createOrganization auto-links the admin user as owner (1 has_member relation).
	// Note: createEndeavour auto-links the endeavour to the user's org (1 participates_in relation).
	resID := env.createResource("Relation Test Resource", "human")
	orgID := env.createOrganization("Relation Test Org")
	edvID := env.createEndeavour("Relation Test Endeavour")

	// Create relation: org has_member resource
	rec := env.doRequest("POST", "/api/v1/relations", map[string]interface{}{
		"relationship_type":  "has_member",
		"source_entity_type": "organization",
		"source_entity_id":   orgID,
		"target_entity_type": "resource",
		"target_entity_id":   resID,
		"metadata":           map[string]interface{}{"role": "member"},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	data := env.parseData(rec)
	relID := data["id"].(string)
	if data["relationship_type"] != "has_member" {
		t.Errorf("relationship_type = %v, want has_member", data["relationship_type"])
	}

	// List by source: auto-created owner + auto-linked participates_in + has_member = 3
	rec = env.doRequest("GET", "/api/v1/relations?source_entity_type=organization&source_entity_id="+orgID, nil)
	items, meta := env.parseList(rec)
	if int(meta["total"].(float64)) != 3 {
		t.Errorf("list by source: total = %v, want 3", meta["total"])
	}
	if len(items) != 3 {
		t.Errorf("list by source: items = %d, want 3", len(items))
	}

	// Find the auto-created participates_in relation ID.
	var rel2ID string
	for _, item := range items {
		m := item.(map[string]interface{})
		if m["relationship_type"] == "participates_in" && m["target_entity_id"] == edvID {
			rel2ID = m["id"].(string)
			break
		}
	}
	if rel2ID == "" {
		t.Fatal("auto-created participates_in relation not found")
	}

	// List by target (our manually added resource)
	rec = env.doRequest("GET", "/api/v1/relations?target_entity_type=resource&target_entity_id="+resID, nil)
	items, _ = env.parseList(rec)
	if len(items) != 1 {
		t.Errorf("list by target: items = %d, want 1", len(items))
	}

	// List by relationship type
	rec = env.doRequest("GET", "/api/v1/relations?relationship_type=participates_in", nil)
	items, _ = env.parseList(rec)
	if len(items) != 1 {
		t.Errorf("list by type participates_in: items = %d, want 1", len(items))
	}

	// Delete the manually created has_member relation
	rec = env.doRequest("DELETE", "/api/v1/relations/"+relID, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// List by source: auto-created owner + participates_in = 2
	rec = env.doRequest("GET", "/api/v1/relations?source_entity_type=organization&source_entity_id="+orgID, nil)
	_, meta = env.parseList(rec)
	if int(meta["total"].(float64)) != 2 {
		t.Errorf("list after delete: total = %v, want 2", meta["total"])
	}

	// Delete participates_in relation
	rec = env.doRequest("DELETE", "/api/v1/relations/"+rel2ID, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete second: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// With scoped access (no admin=true), org relations are invisible once the org
	// no longer participates in any accessible endeavour.
	rec = env.doRequest("GET", "/api/v1/relations?source_entity_type=organization&source_entity_id="+orgID, nil)
	_, meta = env.parseList(rec)
	if int(meta["total"].(float64)) != 0 {
		t.Errorf("list after manual deletes (scoped): total = %v, want 0", meta["total"])
	}

	// With admin=true, the auto-created owner relation is still visible.
	rec = env.doRequest("GET", "/api/v1/relations?source_entity_type=organization&source_entity_id="+orgID+"&admin=true", nil)
	_, meta = env.parseList(rec)
	if int(meta["total"].(float64)) != 1 {
		t.Errorf("list after manual deletes (admin): total = %v, want 1 (auto-owner)", meta["total"])
	}
}
