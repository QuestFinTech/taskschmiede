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

func TestArtifactCRUD(t *testing.T) {
	env := newTestEnv(t)

	// Create
	rec := env.doRequest("POST", "/api/v1/artifacts", map[string]interface{}{
		"kind":    "doc",
		"title":   "Architecture Doc",
		"url":     "https://docs.example.com/arch",
		"summary": "System architecture overview",
		"tags":    []string{"architecture", "design"},
		"metadata": map[string]interface{}{
			"format": "markdown",
		},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	data := env.parseData(rec)
	artID := data["id"].(string)
	if data["kind"] != "doc" {
		t.Errorf("kind = %v, want doc", data["kind"])
	}
	if data["title"] != "Architecture Doc" {
		t.Errorf("title = %v, want Architecture Doc", data["title"])
	}
	if data["status"] != "active" {
		t.Errorf("status = %v, want active", data["status"])
	}

	// Get
	rec = env.doRequest("GET", "/api/v1/artifacts/"+artID, nil)
	data = env.parseData(rec)
	if data["id"] != artID {
		t.Errorf("get: id = %v, want %v", data["id"], artID)
	}
	if data["url"] != "https://docs.example.com/arch" {
		t.Errorf("get: url = %v, want https://docs.example.com/arch", data["url"])
	}
	if data["summary"] != "System architecture overview" {
		t.Errorf("get: summary = %v, want System architecture overview", data["summary"])
	}

	// List
	rec = env.doRequest("GET", "/api/v1/artifacts?admin=true", nil)
	items, meta := env.parseList(rec)
	if int(meta["total"].(float64)) < 1 {
		t.Errorf("list: total = %v, want >= 1", meta["total"])
	}
	if len(items) < 1 {
		t.Errorf("list: items = %d, want >= 1", len(items))
	}

	// List with kind filter
	rec = env.doRequest("GET", "/api/v1/artifacts?admin=true&kind=doc", nil)
	items, _ = env.parseList(rec)
	if len(items) != 1 {
		t.Errorf("list kind=doc: items = %d, want 1", len(items))
	}

	// List with search
	rec = env.doRequest("GET", "/api/v1/artifacts?admin=true&search=Architecture", nil)
	items, _ = env.parseList(rec)
	if len(items) != 1 {
		t.Errorf("list search: items = %d, want 1", len(items))
	}

	// Update
	rec = env.doRequest("PATCH", "/api/v1/artifacts/"+artID, map[string]interface{}{
		"title":  "Architecture Doc v2",
		"status": "archived",
	})
	data = env.parseData(rec)
	if data["title"] != "Architecture Doc v2" {
		t.Errorf("update: title = %v, want Architecture Doc v2", data["title"])
	}
	if data["status"] != "archived" {
		t.Errorf("update: status = %v, want archived", data["status"])
	}

	// Get not found
	rec = env.doRequest("GET", "/api/v1/artifacts/art_nonexistent", nil)
	env.parseError(rec, http.StatusNotFound)
}
