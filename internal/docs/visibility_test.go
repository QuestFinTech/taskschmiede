package docs

import (
	"testing"
)

func TestInternalToolsMarked(t *testing.T) {
	registry := DefaultRegistry("test")

	// Tools that must be internal per docs/API_VISIBILITY.md
	internalTools := []string{
		"ts.tkn.create",
		"ts.tkn.verify",
		"ts.inv.create",
		"ts.inv.list",
		"ts.inv.revoke",
		"ts.onboard.start_interview",
		"ts.onboard.next_challenge",
		"ts.onboard.submit",
		"ts.onboard.complete",
		"ts.onboard.status",
		"ts.onboard.health",
		"ts.onboard.step0",
		"ts.audit.list",
		"ts.audit.my_activity",
		"ts.audit.entity_changes",
	}

	for _, name := range internalTools {
		tool := registry.Get(name)
		if tool == nil {
			t.Errorf("tool %q not found in registry", name)
			continue
		}
		if tool.Visibility != "internal" {
			t.Errorf("tool %q: Visibility = %q, want %q", name, tool.Visibility, "internal")
		}
	}
}

func TestPublicToolsNotMarkedInternal(t *testing.T) {
	registry := DefaultRegistry("test")

	// Spot-check: public tools must not be marked internal
	publicTools := []string{
		"ts.auth.login",
		"ts.auth.whoami",
		"ts.tsk.create",
		"ts.tsk.list",
		"ts.org.create",
		"ts.edv.create",
		"ts.dmd.create",
		"ts.msg.send",
		"ts.doc.list",
		"ts.doc.get",
	}

	for _, name := range publicTools {
		tool := registry.Get(name)
		if tool == nil {
			t.Errorf("tool %q not found in registry", name)
			continue
		}
		if tool.Visibility == "internal" {
			t.Errorf("tool %q should be public, but is marked internal", name)
		}
	}
}

func TestVisibilityCount(t *testing.T) {
	registry := DefaultRegistry("test")
	all := registry.All()

	internal := 0
	public := 0
	for _, tool := range all {
		if tool.Visibility == "internal" {
			internal++
		} else {
			public++
		}
	}

	if internal != 15 {
		t.Errorf("internal tools = %d, want 15", internal)
	}

	expectedPublic := len(all) - 15
	if public != expectedPublic {
		t.Errorf("public tools = %d, want %d", public, expectedPublic)
	}
}
