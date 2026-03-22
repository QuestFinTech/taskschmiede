package main

import (
	"strings"
	"testing"
)

func TestIsInternalAPIPath(t *testing.T) {
	tests := []struct {
		path     string
		internal bool
	}{
		// Internal paths
		{"/api/v1/admin/setup", true},
		{"/api/v1/admin/setup/status", true},
		{"/api/v1/admin/setup/verify", true},
		{"/api/v1/admin/settings", true},
		{"/api/v1/admin/quotas", true},
		{"/api/v1/admin/stats", true},
		{"/api/v1/admin/password", true},
		{"/api/v1/admin/content-guard/stats", true},
		{"/api/v1/admin/agent-block-signals", true},
		{"/api/v1/audit", true},
		{"/api/v1/audit/my-activity", true},
		{"/api/v1/entity-changes", true},
		{"/api/v1/kpi/current", true},
		{"/api/v1/kpi/history", true},
		{"/api/v1/agent-tokens", true},
		{"/api/v1/agent-tokens/{id}", true},
		{"/api/v1/invitations", true},
		{"/api/v1/invitations/{id}", true},
		{"/api/v1/onboarding/status", true},
		{"/api/v1/my-agents", true},
		{"/api/v1/my-agents/{id}", true},
		{"/api/v1/my-alerts", true},
		{"/api/v1/my-alerts/stats", true},
		{"/api/v1/my-indicators", true},
		{"/api/v1/auth/verification-status", true},
		{"/api/v1/compatibility", true},
		{"/api/v1/activity", true},

		// Public paths
		{"/api/v1/tasks", false},
		{"/api/v1/tasks/{id}", false},
		{"/api/v1/auth/login", false},
		{"/api/v1/auth/whoami", false},
		{"/api/v1/organizations", false},
		{"/api/v1/organizations/{id}", false},
		{"/api/v1/endeavours", false},
		{"/api/v1/demands", false},
		{"/api/v1/resources", false},
		{"/api/v1/comments", false},
		{"/api/v1/messages/inbox", false},
		{"/api/v1/instance/info", false},
	}

	for _, tt := range tests {
		got := isInternalAPIPath(tt.path)
		if got != tt.internal {
			t.Errorf("isInternalAPIPath(%q) = %v, want %v", tt.path, got, tt.internal)
		}
	}
}

func TestFilterInternalOpenAPIPaths(t *testing.T) {
	spec := `openapi: "3.1.0"
info:
  title: Test API
  version: "1.0"
paths:
  /api/v1/tasks:
    get:
      summary: List tasks
  /api/v1/tasks/{id}:
    get:
      summary: Get task
  /api/v1/admin/setup:
    post:
      summary: Setup admin
  /api/v1/audit:
    get:
      summary: List audit logs
  /api/v1/organizations:
    get:
      summary: List organizations
  /api/v1/invitations:
    post:
      summary: Create invitation
`

	filtered, removed, err := filterInternalOpenAPIPaths([]byte(spec))
	if err != nil {
		t.Fatalf("filterInternalOpenAPIPaths: %v", err)
	}

	if removed != 3 {
		t.Errorf("removed = %d, want 3", removed)
	}

	result := string(filtered)

	// Public paths should be present
	if !strings.Contains(result, "/api/v1/tasks:") {
		t.Error("filtered spec should contain /api/v1/tasks")
	}
	if !strings.Contains(result, "/api/v1/organizations:") {
		t.Error("filtered spec should contain /api/v1/organizations")
	}

	// Internal paths should be absent
	if strings.Contains(result, "/api/v1/admin/setup:") {
		t.Error("filtered spec should not contain /api/v1/admin/setup")
	}
	if strings.Contains(result, "/api/v1/audit:") {
		t.Error("filtered spec should not contain /api/v1/audit")
	}
	if strings.Contains(result, "/api/v1/invitations:") {
		t.Error("filtered spec should not contain /api/v1/invitations")
	}
}

func TestFilterInternalOpenAPIPaths_PreservesStructure(t *testing.T) {
	spec := `openapi: "3.1.0"
info:
  title: Test API
  version: "1.0"
paths:
  /api/v1/tasks:
    get:
      summary: List tasks
      responses:
        "200":
          description: Task list
components:
  schemas:
    Task:
      type: object
`

	filtered, removed, err := filterInternalOpenAPIPaths([]byte(spec))
	if err != nil {
		t.Fatalf("filterInternalOpenAPIPaths: %v", err)
	}

	if removed != 0 {
		t.Errorf("removed = %d, want 0 (no internal paths)", removed)
	}

	result := string(filtered)
	if !strings.Contains(result, "components:") {
		t.Error("filtered spec should preserve components section")
	}
	if !strings.Contains(result, "Task:") {
		t.Error("filtered spec should preserve schema definitions")
	}
}
