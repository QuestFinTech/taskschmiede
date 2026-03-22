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

func TestAuthLoginLogout(t *testing.T) {
	env := newTestEnv(t)

	// Whoami with valid token
	rec := env.doRequest("GET", "/api/v1/auth/whoami", nil)
	data := env.parseData(rec)
	if data["user_id"] != env.adminUserID {
		t.Errorf("whoami user_id = %v, want %v", data["user_id"], env.adminUserID)
	}
	if data["email"] != "admin@test.local" {
		t.Errorf("whoami email = %v, want admin@test.local", data["email"])
	}
	if data["resource_id"] != env.adminResourceID {
		t.Errorf("whoami resource_id = %v, want %v", data["resource_id"], env.adminResourceID)
	}

	// Login with correct credentials
	rec = env.doRequestNoAuth("POST", "/api/v1/auth/login", map[string]interface{}{
		"email":    "admin@test.local",
		"password": "TestPass123!",
	})
	loginData := env.parseData(rec)
	loginToken, ok := loginData["token"].(string)
	if !ok || loginToken == "" {
		t.Fatal("login did not return a token")
	}
	if loginData["user_id"] != env.adminUserID {
		t.Errorf("login user_id = %v, want %v", loginData["user_id"], env.adminUserID)
	}

	// Whoami with login token
	rec = env.doRequestAs(loginToken, "GET", "/api/v1/auth/whoami", nil)
	data = env.parseData(rec)
	if data["user_id"] != env.adminUserID {
		t.Errorf("whoami after login: user_id = %v, want %v", data["user_id"], env.adminUserID)
	}

	// Logout
	rec = env.doRequestAs(loginToken, "POST", "/api/v1/auth/logout", nil)
	data = env.parseData(rec)
	if data["status"] != "logged_out" {
		t.Errorf("logout status = %v, want logged_out", data["status"])
	}

	// Whoami after logout should fail
	rec = env.doRequestAs(loginToken, "GET", "/api/v1/auth/whoami", nil)
	env.parseError(rec, http.StatusUnauthorized)

	// Login with wrong password
	rec = env.doRequestNoAuth("POST", "/api/v1/auth/login", map[string]interface{}{
		"email":    "admin@test.local",
		"password": "WrongPass123!",
	})
	env.parseError(rec, http.StatusUnauthorized)
}

func TestAuthUnauthorized(t *testing.T) {
	env := newTestEnv(t)

	endpoints := []struct {
		method string
		path   string
	}{
		{"GET", "/api/v1/auth/whoami"},
		{"POST", "/api/v1/auth/logout"},
		{"GET", "/api/v1/tasks"},
		{"POST", "/api/v1/tasks"},
		{"GET", "/api/v1/endeavours"},
		{"POST", "/api/v1/resources"},
		{"GET", "/api/v1/organizations"},
		{"POST", "/api/v1/comments"},
		{"POST", "/api/v1/approvals"},
		{"GET", "/api/v1/relations"},
	}

	for _, ep := range endpoints {
		t.Run(ep.method+" "+ep.path, func(t *testing.T) {
			rec := env.doRequestNoAuth(ep.method, ep.path, nil)
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("expected 401 for %s %s, got %d", ep.method, ep.path, rec.Code)
			}
		})
	}
}
