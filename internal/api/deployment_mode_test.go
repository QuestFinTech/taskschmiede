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
	"context"
	"io"
	"log/slog"
	"net/http"
	"testing"

	"github.com/QuestFinTech/taskschmiede/internal/auth"
	"github.com/QuestFinTech/taskschmiede/internal/storage"
)

// newTestEnvWithMode creates a test environment with deployment mode settings.
func newTestEnvWithMode(t *testing.T, mode string, allowSelfReg, requireEmailVerify, requireInterview bool) *testEnv {
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
		DB:                            db,
		Logger:                        logger,
		AuthService:                   authSvc,
		DeploymentMode:                mode,
		AllowSelfRegistration:         allowSelfReg,
		RequireAgentEmailVerification: requireEmailVerify,
		RequireAgentInterview:         requireInterview,
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

// --- Instance Info ---

func TestInstanceInfoPublic(t *testing.T) {
	env := newTestEnvWithMode(t, "open", true, true, true)

	rec := env.doRequestNoAuth("GET", "/api/v1/instance/info", nil)
	data := env.parseData(rec)

	if data["deployment_mode"] != "open" {
		t.Errorf("deployment_mode = %v, want open", data["deployment_mode"])
	}
	if data["allow_self_registration"] != true {
		t.Errorf("allow_self_registration = %v, want true", data["allow_self_registration"])
	}
}

func TestInstanceInfoTrustedMode(t *testing.T) {
	env := newTestEnvWithMode(t, "trusted", false, false, false)

	rec := env.doRequestNoAuth("GET", "/api/v1/instance/info", nil)
	data := env.parseData(rec)

	if data["deployment_mode"] != "trusted" {
		t.Errorf("deployment_mode = %v, want trusted", data["deployment_mode"])
	}
	if data["allow_self_registration"] != false {
		t.Errorf("allow_self_registration = %v, want false", data["allow_self_registration"])
	}
}

// --- Self-Registration Gate ---

func TestSelfRegistrationBlocked(t *testing.T) {
	env := newTestEnvWithMode(t, "open", false, true, true)

	rec := env.doRequestNoAuth("POST", "/api/v1/auth/register", map[string]interface{}{
		"email":    "newuser@example.com",
		"name":     "New User",
		"password": "SecureP@ss1!xyz",
	})

	errData := env.parseError(rec, http.StatusForbidden)
	if errData["code"] != "self_registration_disabled" {
		t.Errorf("error code = %v, want self_registration_disabled", errData["code"])
	}
}

func TestSelfRegistrationAllowed(t *testing.T) {
	env := newTestEnvWithMode(t, "open", true, true, true)

	rec := env.doRequestNoAuth("POST", "/api/v1/auth/register", map[string]interface{}{
		"email":    "newuser@example.com",
		"name":     "New User",
		"password": "SecureP@ss1!xyz",
	})

	// Should succeed (pending verification) -- status 201 or 200.
	// Email sending may fail in test (no SMTP), but the response should not be 403.
	if rec.Code == http.StatusForbidden {
		t.Errorf("self-registration should be allowed, got 403")
	}
}

// --- Agent Token Creation: Admin-Only ---

func TestAgentTokenCreateRequiresAdmin(t *testing.T) {
	env := newTestEnvWithMode(t, "open", true, true, true)

	// Create a non-admin user.
	nonAdmin := env.createNonAdminUser("nonadmin1")

	rec := env.doRequestAs(nonAdmin.Token, "POST", "/api/v1/agent-tokens", map[string]interface{}{
		"name": "Test Agent Token",
	})

	errData := env.parseError(rec, http.StatusForbidden)
	if errData["code"] != "forbidden" {
		t.Errorf("error code = %v, want forbidden", errData["code"])
	}
}

func TestAgentTokenCreateAdminAllowed(t *testing.T) {
	env := newTestEnvWithMode(t, "open", true, true, true)

	rec := env.doRequest("POST", "/api/v1/agent-tokens", map[string]interface{}{
		"name": "Test Agent Token",
	})

	data := env.parseData(rec)
	if data["id"] == nil || data["id"] == "" {
		t.Error("expected agent token to be created with an id")
	}
	if data["token"] == nil || data["token"] == "" {
		t.Error("expected agent token to include the token value")
	}
}

// --- Master Admin Promotion/Demotion ---

func TestPromoteUserToAdmin(t *testing.T) {
	env := newTestEnvWithMode(t, "open", true, true, true)

	// Create a non-admin user.
	target := env.createNonAdminUser("promotee")

	// Admin promotes the user.
	isAdmin := true
	rec := env.doRequest("PATCH", "/api/v1/users/"+target.UserID, map[string]interface{}{
		"is_admin": isAdmin,
	})

	data := env.parseData(rec)
	if data["is_admin"] != true {
		t.Errorf("is_admin = %v, want true after promotion", data["is_admin"])
	}
}

func TestPromoteRequiresMasterAdmin(t *testing.T) {
	env := newTestEnvWithMode(t, "open", true, true, true)

	nonAdmin := env.createNonAdminUser("nonadmin2")
	target := env.createNonAdminUser("target2")

	// Non-admin tries to promote -- should fail.
	rec := env.doRequestAs(nonAdmin.Token, "PATCH", "/api/v1/users/"+target.UserID, map[string]interface{}{
		"is_admin": true,
	})

	errData := env.parseError(rec, http.StatusForbidden)
	if errData["code"] != "forbidden" {
		t.Errorf("error code = %v, want forbidden", errData["code"])
	}
}

func TestSelfDemotionBlocked(t *testing.T) {
	env := newTestEnvWithMode(t, "open", true, true, true)

	// Admin tries to demote themselves.
	rec := env.doRequest("PATCH", "/api/v1/users/"+env.adminUserID, map[string]interface{}{
		"is_admin": false,
	})

	errData := env.parseError(rec, http.StatusBadRequest)
	if errData["code"] != "invalid_input" {
		t.Errorf("error code = %v, want invalid_input", errData["code"])
	}
}

func TestDemoteOtherAdmin(t *testing.T) {
	env := newTestEnvWithMode(t, "open", true, true, true)

	// Create a user and promote them first.
	target := env.createNonAdminUser("demotee")
	rec := env.doRequest("PATCH", "/api/v1/users/"+target.UserID, map[string]interface{}{
		"is_admin": true,
	})
	env.parseData(rec) // verify success

	// Now demote them.
	rec = env.doRequest("PATCH", "/api/v1/users/"+target.UserID, map[string]interface{}{
		"is_admin": false,
	})

	data := env.parseData(rec)
	if data["is_admin"] != false {
		t.Errorf("is_admin = %v, want false after demotion", data["is_admin"])
	}
}
