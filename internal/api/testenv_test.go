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
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuestFinTech/taskschmiede/internal/auth"
	"github.com/QuestFinTech/taskschmiede/internal/storage"
)

// testEnv provides an isolated test environment with an in-memory database,
// a fully wired API instance, and HTTP request helpers.
type testEnv struct {
	t               *testing.T
	db              *storage.DB
	authSvc         *auth.Service
	api             *API
	handler         http.Handler
	adminToken      string
	adminUserID     string
	adminResourceID string
}

// newTestEnv creates a fresh test environment with an in-memory SQLite database,
// a master admin user, and a valid bearer token.
func newTestEnv(t *testing.T) *testEnv {
	t.Helper()

	db, err := storage.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := db.Initialize(); err != nil {
		t.Fatalf("initialize db: %v", err)
	}

	authSvc := auth.NewService(db)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Create resource for admin user.
	const resID = "res_test_admin"
	_, err = db.Exec(
		`INSERT INTO resource (id, type, name, capacity_model, metadata, status)
		 VALUES (?, 'human', 'Test Admin', 'hours_per_week', '{}', 'active')`,
		resID,
	)
	if err != nil {
		t.Fatalf("create resource: %v", err)
	}

	// Create user linked to resource with admin privileges.
	const userID = "usr_test_admin"
	_, err = db.Exec(
		`INSERT INTO user (id, name, email, resource_id, password_hash, is_admin, tier, user_type, metadata, status)
		 VALUES (?, 'Test Admin', 'admin@test.local', ?, ?, 1, 1, 'human', '{}', 'active')`,
		userID, resID, mustHashPassword(t, "TestPass123!"),
	)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	// Create bearer token.
	token, _, err := authSvc.CreateToken(context.Background(), userID, "test-token", nil)
	if err != nil {
		t.Fatalf("create token: %v", err)
	}

	a := New(&Config{
		DB:          db,
		Logger:      logger,
		AuthService: authSvc,
	})

	return &testEnv{
		t:               t,
		db:              db,
		authSvc:         authSvc,
		api:             a,
		handler:         a.Handler(),
		adminToken:      token,
		adminUserID:     userID,
		adminResourceID: resID,
	}
}

// mustHashPassword hashes a password or fails the test.
func mustHashPassword(t *testing.T, password string) string {
	t.Helper()
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	return hash
}

// doRequest performs an authenticated HTTP request using the admin token.
func (e *testEnv) doRequest(method, path string, body interface{}) *httptest.ResponseRecorder {
	e.t.Helper()
	return e.doRequestAs(e.adminToken, method, path, body)
}

// doRequestAs performs an HTTP request with the given bearer token.
func (e *testEnv) doRequestAs(token, method, path string, body interface{}) *httptest.ResponseRecorder {
	e.t.Helper()

	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			e.t.Fatalf("encode body: %v", err)
		}
	}

	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	rec := httptest.NewRecorder()
	e.handler.ServeHTTP(rec, req)
	return rec
}

// doRequestNoAuth performs an unauthenticated HTTP request.
func (e *testEnv) doRequestNoAuth(method, path string, body interface{}) *httptest.ResponseRecorder {
	e.t.Helper()
	return e.doRequestAs("", method, path, body)
}

// parseData decodes a {"data": ...} response and returns the data map.
// Fails the test if the status code is not in the 2xx range.
func (e *testEnv) parseData(rec *httptest.ResponseRecorder) map[string]interface{} {
	e.t.Helper()

	if rec.Code < 200 || rec.Code >= 300 {
		e.t.Fatalf("expected 2xx, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Data map[string]interface{} `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		e.t.Fatalf("decode response: %v", err)
	}
	return resp.Data
}

// parseList decodes a {"data": [...], "meta": {...}} response.
// Returns the data items and meta map.
func (e *testEnv) parseList(rec *httptest.ResponseRecorder) ([]interface{}, map[string]interface{}) {
	e.t.Helper()

	if rec.Code < 200 || rec.Code >= 300 {
		e.t.Fatalf("expected 2xx, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Data []interface{}          `json:"data"`
		Meta map[string]interface{} `json:"meta"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		e.t.Fatalf("decode list response: %v", err)
	}
	return resp.Data, resp.Meta
}

// parseError decodes a {"error": {...}} response and asserts the status code.
func (e *testEnv) parseError(rec *httptest.ResponseRecorder, expectedStatus int) map[string]interface{} {
	e.t.Helper()

	if rec.Code != expectedStatus {
		e.t.Fatalf("expected status %d, got %d: %s", expectedStatus, rec.Code, rec.Body.String())
	}

	var resp struct {
		Error map[string]interface{} `json:"error"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		e.t.Fatalf("decode error response: %v", err)
	}
	return resp.Error
}

// --- Factory methods ---

// createResource creates a resource via the API and returns its ID.
func (e *testEnv) createResource(name, typ string) string {
	e.t.Helper()
	rec := e.doRequest("POST", "/api/v1/resources", map[string]interface{}{
		"type":           typ,
		"name":           name,
		"capacity_model": "hours_per_week",
	})
	data := e.parseData(rec)
	return data["id"].(string)
}

// createEndeavour creates an endeavour via the API and returns its ID.
func (e *testEnv) createEndeavour(name string) string {
	e.t.Helper()
	rec := e.doRequest("POST", "/api/v1/endeavours", map[string]interface{}{
		"name":        name,
		"description": "Test endeavour",
	})
	data := e.parseData(rec)
	return data["id"].(string)
}

// createOrganization creates an organization via the API and returns its ID.
func (e *testEnv) createOrganization(name string) string {
	e.t.Helper()
	rec := e.doRequest("POST", "/api/v1/organizations", map[string]interface{}{
		"name":        name,
		"description": "Test org",
	})
	data := e.parseData(rec)
	return data["id"].(string)
}

// createTask creates a task via the API and returns its ID.
func (e *testEnv) createTask(title, endeavourID string) string {
	e.t.Helper()
	body := map[string]interface{}{"title": title}
	if endeavourID != "" {
		body["endeavour_id"] = endeavourID
	}
	rec := e.doRequest("POST", "/api/v1/tasks", body)
	data := e.parseData(rec)
	return data["id"].(string)
}

// createRitual creates a ritual via the API and returns its ID.
func (e *testEnv) createRitual(name, prompt string) string {
	e.t.Helper()
	rec := e.doRequest("POST", "/api/v1/rituals", map[string]interface{}{
		"name":   name,
		"prompt": prompt,
	})
	data := e.parseData(rec)
	return data["id"].(string)
}

// --- RBAC test helpers ---

// testUser represents a non-admin user created for RBAC testing.
type testUser struct {
	UserID     string
	ResourceID string
	Token      string
}

// createNonAdminUser creates a non-admin user with a linked resource and bearer token.
// The user has NO access to any endeavour or organization.
func (e *testEnv) createNonAdminUser(name string) testUser {
	e.t.Helper()

	resID := "res_" + name
	_, err := e.db.Exec(
		`INSERT INTO resource (id, type, name, capacity_model, metadata, status)
		 VALUES (?, 'human', ?, 'hours_per_week', '{}', 'active')`,
		resID, name,
	)
	if err != nil {
		e.t.Fatalf("create resource for %s: %v", name, err)
	}

	userID := "usr_" + name
	_, err = e.db.Exec(
		`INSERT INTO user (id, name, email, resource_id, password_hash, tier, user_type, metadata, status)
		 VALUES (?, ?, ?, ?, ?, 1, 'human', '{}', 'active')`,
		userID, name, name+"@test.local", resID, mustHashPassword(e.t, "TestPass123!"),
	)
	if err != nil {
		e.t.Fatalf("create user %s: %v", name, err)
	}

	token, _, err := e.authSvc.CreateToken(context.Background(), userID, name+"-token", nil)
	if err != nil {
		e.t.Fatalf("create token for %s: %v", name, err)
	}

	return testUser{UserID: userID, ResourceID: resID, Token: token}
}

// addUserToEndeavour grants a user access to an endeavour with the given role.
// Uses direct DB insert (admin API) to avoid RBAC chicken-and-egg issues.
func (e *testEnv) addUserToEndeavour(userID, endeavourID, role string) {
	e.t.Helper()
	if err := e.db.AddUserToEndeavour(userID, endeavourID, role); err != nil {
		e.t.Fatalf("add user %s to endeavour %s: %v", userID, endeavourID, err)
	}
}

// addResourceToOrg grants a resource membership in an organization with the given role.
// Uses direct DB insert to avoid RBAC chicken-and-egg issues.
func (e *testEnv) addResourceToOrg(orgID, resourceID, role string) {
	e.t.Helper()
	rec := e.doRequest("POST", "/api/v1/organizations/"+orgID+"/resources", map[string]interface{}{
		"resource_id": resourceID,
		"role":        role,
	})
	if rec.Code < 200 || rec.Code >= 300 {
		e.t.Fatalf("add resource %s to org %s: %d %s", resourceID, orgID, rec.Code, rec.Body.String())
	}
}
