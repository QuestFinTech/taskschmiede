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

// TestRBACTaskAccess verifies that a non-admin user cannot access tasks
// in endeavours they don't belong to, and CAN access tasks in their endeavours.
func TestRBACTaskAccess(t *testing.T) {
	env := newTestEnv(t)

	// Admin creates an endeavour and a task in it.
	edvID := env.createEndeavour("RBAC Task Endeavour")
	taskID := env.createTask("Secret Task", edvID)

	// Create a non-admin user with NO endeavour access.
	alice := env.createNonAdminUser("alice")

	// Alice cannot get the task (404, not 403).
	rec := env.doRequestAs(alice.Token, "GET", "/api/v1/tasks/"+taskID, nil)
	env.parseError(rec, http.StatusNotFound)

	// Alice cannot list tasks in the endeavour (gets empty list due to scope filter).
	rec = env.doRequestAs(alice.Token, "GET", "/api/v1/tasks?endeavour_id="+edvID, nil)
	items, _ := env.parseList(rec)
	if len(items) != 0 {
		t.Errorf("alice should see 0 tasks, got %d", len(items))
	}

	// Alice cannot update the task.
	rec = env.doRequestAs(alice.Token, "PATCH", "/api/v1/tasks/"+taskID, map[string]interface{}{
		"status": "active",
	})
	env.parseError(rec, http.StatusNotFound)

	// Alice cannot create a task in the endeavour.
	rec = env.doRequestAs(alice.Token, "POST", "/api/v1/tasks", map[string]interface{}{
		"title":        "Unauthorized Task",
		"endeavour_id": edvID,
	})
	env.parseError(rec, http.StatusNotFound)

	// Grant Alice member access to the endeavour.
	env.addUserToEndeavour(alice.UserID, edvID, "member")

	// Now Alice CAN get the task.
	rec = env.doRequestAs(alice.Token, "GET", "/api/v1/tasks/"+taskID, nil)
	data := env.parseData(rec)
	if data["id"] != taskID {
		t.Errorf("alice get task: id = %v, want %v", data["id"], taskID)
	}

	// Alice CAN create a task in the endeavour.
	rec = env.doRequestAs(alice.Token, "POST", "/api/v1/tasks", map[string]interface{}{
		"title":        "Alice Task",
		"endeavour_id": edvID,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("alice create task: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	// Alice (member) cannot cancel the task (requires admin or assignee).
	rec = env.doRequestAs(alice.Token, "PATCH", "/api/v1/tasks/"+taskID, map[string]interface{}{
		"status":          "canceled",
		"canceled_reason": "test",
	})
	env.parseError(rec, http.StatusForbidden)
}

// TestRBACDemandAccess verifies endeavour-based RBAC on demands.
func TestRBACDemandAccess(t *testing.T) {
	env := newTestEnv(t)

	edvID := env.createEndeavour("RBAC Demand Endeavour")

	// Admin creates a demand.
	rec := env.doRequest("POST", "/api/v1/demands", map[string]interface{}{
		"type":         "feature",
		"title":        "Secret Demand",
		"endeavour_id": edvID,
	})
	data := env.parseData(rec)
	demandID := data["id"].(string)

	bob := env.createNonAdminUser("bob")

	// Bob cannot get the demand.
	rec = env.doRequestAs(bob.Token, "GET", "/api/v1/demands/"+demandID, nil)
	env.parseError(rec, http.StatusNotFound)

	// Bob cannot update the demand.
	rec = env.doRequestAs(bob.Token, "PATCH", "/api/v1/demands/"+demandID, map[string]interface{}{
		"title": "Hacked",
	})
	env.parseError(rec, http.StatusNotFound)

	// Grant Bob member access.
	env.addUserToEndeavour(bob.UserID, edvID, "member")

	// Now Bob can get the demand.
	rec = env.doRequestAs(bob.Token, "GET", "/api/v1/demands/"+demandID, nil)
	data = env.parseData(rec)
	if data["id"] != demandID {
		t.Errorf("bob get demand: id = %v, want %v", data["id"], demandID)
	}

	// Bob (member) cannot cancel the demand (requires admin).
	rec = env.doRequestAs(bob.Token, "PATCH", "/api/v1/demands/"+demandID, map[string]interface{}{
		"status":          "canceled",
		"canceled_reason": "test",
	})
	env.parseError(rec, http.StatusForbidden)
}

// TestRBACEndeavourAccess verifies endeavour-level RBAC.
func TestRBACEndeavourAccess(t *testing.T) {
	env := newTestEnv(t)

	edvID := env.createEndeavour("RBAC Protected Endeavour")
	carol := env.createNonAdminUser("carol")
	carolAdmin := env.createNonAdminUser("carol_admin")

	// Carol cannot get the endeavour (no access).
	rec := env.doRequestAs(carol.Token, "GET", "/api/v1/endeavours/"+edvID, nil)
	env.parseError(rec, http.StatusNotFound)

	// Carol cannot update the endeavour.
	rec = env.doRequestAs(carol.Token, "PATCH", "/api/v1/endeavours/"+edvID, map[string]interface{}{
		"name": "Hacked",
	})
	env.parseError(rec, http.StatusNotFound)

	// Grant Carol member access.
	env.addUserToEndeavour(carol.UserID, edvID, "member")

	// Carol can read the endeavour now.
	rec = env.doRequestAs(carol.Token, "GET", "/api/v1/endeavours/"+edvID, nil)
	data := env.parseData(rec)
	if data["id"] != edvID {
		t.Errorf("carol get endeavour: id = %v, want %v", data["id"], edvID)
	}

	// Carol (member) cannot update the endeavour (requires admin).
	rec = env.doRequestAs(carol.Token, "PATCH", "/api/v1/endeavours/"+edvID, map[string]interface{}{
		"name": "Renamed",
	})
	env.parseError(rec, http.StatusNotFound)

	// Use a separate user with admin role to test admin-level access.
	env.addUserToEndeavour(carolAdmin.UserID, edvID, "admin")

	// carolAdmin (admin) can update the endeavour.
	rec = env.doRequestAs(carolAdmin.Token, "PATCH", "/api/v1/endeavours/"+edvID, map[string]interface{}{
		"description": "Updated by admin",
	})
	data = env.parseData(rec)
	if data["description"] != "Updated by admin" {
		t.Errorf("admin update: description = %v", data["description"])
	}

	// carolAdmin (admin) cannot archive the endeavour via regular update.
	rec = env.doRequestAs(carolAdmin.Token, "PATCH", "/api/v1/endeavours/"+edvID, map[string]interface{}{
		"status": "archived",
	})
	env.parseError(rec, http.StatusBadRequest)
}

// TestRBACOrganizationAccess verifies organization-level RBAC.
func TestRBACOrganizationAccess(t *testing.T) {
	env := newTestEnv(t)

	orgID := env.createOrganization("RBAC Org")
	dave := env.createNonAdminUser("dave")

	// Dave cannot get the org.
	rec := env.doRequestAs(dave.Token, "GET", "/api/v1/organizations/"+orgID, nil)
	env.parseError(rec, http.StatusNotFound)

	// Dave cannot list orgs (gets empty list).
	rec = env.doRequestAs(dave.Token, "GET", "/api/v1/organizations", nil)
	items, _ := env.parseList(rec)
	if len(items) != 0 {
		t.Errorf("dave should see 0 orgs, got %d", len(items))
	}

	// Add Dave's resource to the org as member.
	env.addResourceToOrg(orgID, dave.ResourceID, "member")

	// Dave can now get the org.
	rec = env.doRequestAs(dave.Token, "GET", "/api/v1/organizations/"+orgID, nil)
	data := env.parseData(rec)
	if data["id"] != orgID {
		t.Errorf("dave get org: id = %v, want %v", data["id"], orgID)
	}

	// Dave (member) cannot update the org.
	rec = env.doRequestAs(dave.Token, "PATCH", "/api/v1/organizations/"+orgID, map[string]interface{}{
		"name": "Hacked",
	})
	env.parseError(rec, http.StatusNotFound)
}

// TestRBACResourceAccess verifies resource-level RBAC (self or org admin).
func TestRBACResourceAccess(t *testing.T) {
	env := newTestEnv(t)

	// Admin creates a resource that nobody else should see.
	otherResID := env.createResource("Other Resource", "agent")

	eve := env.createNonAdminUser("eve")

	// Eve can get her own resource.
	rec := env.doRequestAs(eve.Token, "GET", "/api/v1/resources/"+eve.ResourceID, nil)
	data := env.parseData(rec)
	if data["id"] != eve.ResourceID {
		t.Errorf("eve get own resource: id = %v", data["id"])
	}

	// Eve cannot get someone else's resource.
	rec = env.doRequestAs(eve.Token, "GET", "/api/v1/resources/"+otherResID, nil)
	env.parseError(rec, http.StatusNotFound)

	// Eve can update her own resource.
	rec = env.doRequestAs(eve.Token, "PATCH", "/api/v1/resources/"+eve.ResourceID, map[string]interface{}{
		"name": "Eve Updated",
	})
	data = env.parseData(rec)
	if data["name"] != "Eve Updated" {
		t.Errorf("eve update own resource: name = %v", data["name"])
	}

	// Eve cannot update someone else's resource.
	rec = env.doRequestAs(eve.Token, "PATCH", "/api/v1/resources/"+otherResID, map[string]interface{}{
		"name": "Hacked",
	})
	env.parseError(rec, http.StatusNotFound)
}

// TestRBACCommentAccess verifies comment RBAC via parent entity's endeavour.
func TestRBACCommentAccess(t *testing.T) {
	env := newTestEnv(t)

	edvID := env.createEndeavour("RBAC Comment Endeavour")
	taskID := env.createTask("Comment Target", edvID)

	// Admin creates a comment on the task.
	rec := env.doRequest("POST", "/api/v1/comments", map[string]interface{}{
		"entity_type": "task",
		"entity_id":   taskID,
		"content":     "Admin comment",
	})
	data := env.parseData(rec)
	commentID := data["id"].(string)

	frank := env.createNonAdminUser("frank")

	// Frank cannot get the comment (no access to the task's endeavour).
	rec = env.doRequestAs(frank.Token, "GET", "/api/v1/comments/"+commentID, nil)
	env.parseError(rec, http.StatusNotFound)

	// Frank cannot create a comment on the task.
	rec = env.doRequestAs(frank.Token, "POST", "/api/v1/comments", map[string]interface{}{
		"entity_type": "task",
		"entity_id":   taskID,
		"content":     "Unauthorized comment",
	})
	env.parseError(rec, http.StatusNotFound)

	// Grant Frank member access.
	env.addUserToEndeavour(frank.UserID, edvID, "member")

	// Now Frank can get the comment.
	rec = env.doRequestAs(frank.Token, "GET", "/api/v1/comments/"+commentID, nil)
	data = env.parseData(rec)
	if data["id"] != commentID {
		t.Errorf("frank get comment: id = %v, want %v", data["id"], commentID)
	}

	// Frank can create a comment.
	rec = env.doRequestAs(frank.Token, "POST", "/api/v1/comments", map[string]interface{}{
		"entity_type": "task",
		"entity_id":   taskID,
		"content":     "Frank comment",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("frank create comment: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestRBACApprovalAccess verifies approval RBAC via parent entity's endeavour.
func TestRBACApprovalAccess(t *testing.T) {
	env := newTestEnv(t)

	edvID := env.createEndeavour("RBAC Approval Endeavour")
	taskID := env.createTask("Approval Target", edvID)

	// Admin creates an approval.
	rec := env.doRequest("POST", "/api/v1/approvals", map[string]interface{}{
		"entity_type": "task",
		"entity_id":   taskID,
		"verdict":     "approved",
		"comment":     "Looks good",
	})
	data := env.parseData(rec)
	approvalID := data["id"].(string)

	grace := env.createNonAdminUser("grace_rbac")

	// Grace cannot get the approval.
	rec = env.doRequestAs(grace.Token, "GET", "/api/v1/approvals/"+approvalID, nil)
	env.parseError(rec, http.StatusNotFound)

	// Grace cannot list approvals for the task.
	rec = env.doRequestAs(grace.Token, "GET", "/api/v1/approvals?entity_type=task&entity_id="+taskID, nil)
	env.parseError(rec, http.StatusNotFound)

	// Grant Grace member access.
	env.addUserToEndeavour(grace.UserID, edvID, "member")

	// Now Grace can get the approval.
	rec = env.doRequestAs(grace.Token, "GET", "/api/v1/approvals/"+approvalID, nil)
	data = env.parseData(rec)
	if data["id"] != approvalID {
		t.Errorf("grace get approval: id = %v, want %v", data["id"], approvalID)
	}
}

// TestRBACRitualAccess verifies ritual RBAC via endeavour.
func TestRBACRitualAccess(t *testing.T) {
	env := newTestEnv(t)

	edvID := env.createEndeavour("RBAC Ritual Endeavour")

	// Admin creates a ritual linked to the endeavour.
	rec := env.doRequest("POST", "/api/v1/rituals", map[string]interface{}{
		"name":         "Secret Ritual",
		"prompt":       "Do the thing",
		"endeavour_id": edvID,
	})
	data := env.parseData(rec)
	ritualID := data["id"].(string)

	hank := env.createNonAdminUser("hank")

	// Hank cannot get the ritual.
	rec = env.doRequestAs(hank.Token, "GET", "/api/v1/rituals/"+ritualID, nil)
	env.parseError(rec, http.StatusNotFound)

	// Grant Hank member access.
	env.addUserToEndeavour(hank.UserID, edvID, "member")

	// Hank can read the ritual.
	rec = env.doRequestAs(hank.Token, "GET", "/api/v1/rituals/"+ritualID, nil)
	data = env.parseData(rec)
	if data["id"] != ritualID {
		t.Errorf("hank get ritual: id = %v, want %v", data["id"], ritualID)
	}

	// Hank (member) cannot update the ritual (requires admin).
	rec = env.doRequestAs(hank.Token, "PATCH", "/api/v1/rituals/"+ritualID, map[string]interface{}{
		"name": "Hacked Ritual",
	})
	env.parseError(rec, http.StatusNotFound)
}

// TestRBACRelationAccess verifies that non-admin users cannot create or delete relations.
func TestRBACRelationAccess(t *testing.T) {
	env := newTestEnv(t)

	ivan := env.createNonAdminUser("ivan")

	// Ivan cannot create a relation.
	rec := env.doRequestAs(ivan.Token, "POST", "/api/v1/relations", map[string]interface{}{
		"relationship_type":  "member_of",
		"source_entity_type": "user",
		"source_entity_id":   ivan.UserID,
		"target_entity_type": "endeavour",
		"target_entity_id":   "edv_fake",
	})
	env.parseError(rec, http.StatusForbidden)
}

// TestRBACUserAccess verifies user-level RBAC (self or admin).
func TestRBACUserAccess(t *testing.T) {
	env := newTestEnv(t)

	jane := env.createNonAdminUser("jane")

	// Jane can get herself.
	rec := env.doRequestAs(jane.Token, "GET", "/api/v1/users/"+jane.UserID, nil)
	data := env.parseData(rec)
	if data["id"] != jane.UserID {
		t.Errorf("jane get self: id = %v", data["id"])
	}

	// Jane cannot get another user (the admin).
	rec = env.doRequestAs(jane.Token, "GET", "/api/v1/users/"+env.adminUserID, nil)
	env.parseError(rec, http.StatusNotFound)

	// Jane cannot list users.
	rec = env.doRequestAs(jane.Token, "GET", "/api/v1/users", nil)
	env.parseError(rec, http.StatusForbidden)
}

// TestRBACDodAccess verifies DoD RBAC for endeavour-scoped operations.
func TestRBACDodAccess(t *testing.T) {
	env := newTestEnv(t)

	edvID := env.createEndeavour("RBAC DoD Endeavour")
	taskID := env.createTask("DoD Task", edvID)

	karl := env.createNonAdminUser("karl")

	// Karl cannot create a DoD policy (admin only).
	rec := env.doRequestAs(karl.Token, "POST", "/api/v1/dod-policies", map[string]interface{}{
		"name": "Test Policy",
		"conditions": []map[string]interface{}{
			{"id": "c1", "type": "approval", "label": "Needs approval"},
		},
	})
	env.parseError(rec, http.StatusForbidden)

	// Karl cannot get DoD status for the endeavour.
	rec = env.doRequestAs(karl.Token, "GET", "/api/v1/endeavours/"+edvID+"/dod-status", nil)
	env.parseError(rec, http.StatusNotFound)

	// Karl cannot check DoD for a task.
	rec = env.doRequestAs(karl.Token, "POST", "/api/v1/tasks/"+taskID+"/dod-check", nil)
	env.parseError(rec, http.StatusNotFound)

	// Grant Karl member access.
	env.addUserToEndeavour(karl.UserID, edvID, "member")

	// Karl can now get DoD status.
	rec = env.doRequestAs(karl.Token, "GET", "/api/v1/endeavours/"+edvID+"/dod-status", nil)
	data := env.parseData(rec)
	if data["endeavour_id"] != edvID {
		t.Errorf("karl dod status: endeavour_id = %v", data["endeavour_id"])
	}

	// Karl can check DoD for the task.
	rec = env.doRequestAs(karl.Token, "POST", "/api/v1/tasks/"+taskID+"/dod-check", nil)
	data = env.parseData(rec)
	if data["result"] != "no_policy" {
		t.Errorf("karl dod check: result = %v, want no_policy", data["result"])
	}

	// Karl (member) cannot assign a DoD policy (requires admin).
	rec = env.doRequestAs(karl.Token, "POST", "/api/v1/endeavours/"+edvID+"/dod-policy", map[string]interface{}{
		"policy_id": "dod_fake",
	})
	env.parseError(rec, http.StatusNotFound)
}

// TestRBACAddEndeavourMember verifies that only endeavour admins can add members.
func TestRBACAddEndeavourMember(t *testing.T) {
	env := newTestEnv(t)

	edvID := env.createEndeavour("RBAC AddMember Endeavour")
	leeMember := env.createNonAdminUser("lee_member")
	leeAdmin := env.createNonAdminUser("lee_admin")
	mike := env.createNonAdminUser("mike_rbac")

	// leeMember has no access, cannot add a user.
	rec := env.doRequestAs(leeMember.Token, "POST", "/api/v1/endeavours/"+edvID+"/members", map[string]interface{}{
		"user_id": mike.UserID,
		"role":    "member",
	})
	env.parseError(rec, http.StatusNotFound)

	// Grant leeMember member access (not admin).
	env.addUserToEndeavour(leeMember.UserID, edvID, "member")

	// leeMember (member) still cannot add a user (requires admin).
	rec = env.doRequestAs(leeMember.Token, "POST", "/api/v1/endeavours/"+edvID+"/members", map[string]interface{}{
		"user_id": mike.UserID,
		"role":    "member",
	})
	env.parseError(rec, http.StatusNotFound)

	// Grant leeAdmin admin access.
	env.addUserToEndeavour(leeAdmin.UserID, edvID, "admin")

	// leeAdmin (admin) can add Mike to the endeavour.
	rec = env.doRequestAs(leeAdmin.Token, "POST", "/api/v1/endeavours/"+edvID+"/members", map[string]interface{}{
		"user_id": mike.UserID,
		"role":    "member",
	})
	if rec.Code < 200 || rec.Code >= 300 {
		t.Fatalf("lee_admin add member: expected 2xx, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestRBACCrossEndeavourIsolation verifies that access to one endeavour
// does NOT grant access to entities in a different endeavour.
func TestRBACCrossEndeavourIsolation(t *testing.T) {
	env := newTestEnv(t)

	edvA := env.createEndeavour("Endeavour A")
	edvB := env.createEndeavour("Endeavour B")

	taskA := env.createTask("Task in A", edvA)
	taskB := env.createTask("Task in B", edvB)

	nina := env.createNonAdminUser("nina")

	// Give Nina access to endeavour A only.
	env.addUserToEndeavour(nina.UserID, edvA, "member")

	// Nina can access task in A.
	rec := env.doRequestAs(nina.Token, "GET", "/api/v1/tasks/"+taskA, nil)
	data := env.parseData(rec)
	if data["id"] != taskA {
		t.Errorf("nina get task A: id = %v, want %v", data["id"], taskA)
	}

	// Nina cannot access task in B.
	rec = env.doRequestAs(nina.Token, "GET", "/api/v1/tasks/"+taskB, nil)
	env.parseError(rec, http.StatusNotFound)

	// Nina cannot access endeavour B directly.
	rec = env.doRequestAs(nina.Token, "GET", "/api/v1/endeavours/"+edvB, nil)
	env.parseError(rec, http.StatusNotFound)
}

// TestRBACAdminBypass verifies that master admin can access everything.
func TestRBACAdminBypass(t *testing.T) {
	env := newTestEnv(t)

	edvID := env.createEndeavour("Admin Bypass Endeavour")
	taskID := env.createTask("Admin Task", edvID)
	orgID := env.createOrganization("Admin Bypass Org")

	// Admin (master) can access everything without explicit membership.
	rec := env.doRequest("GET", "/api/v1/tasks/"+taskID, nil)
	data := env.parseData(rec)
	if data["id"] != taskID {
		t.Errorf("admin get task: id = %v", data["id"])
	}

	rec = env.doRequest("GET", "/api/v1/endeavours/"+edvID, nil)
	data = env.parseData(rec)
	if data["id"] != edvID {
		t.Errorf("admin get endeavour: id = %v", data["id"])
	}

	rec = env.doRequest("GET", "/api/v1/organizations/"+orgID, nil)
	data = env.parseData(rec)
	if data["id"] != orgID {
		t.Errorf("admin get org: id = %v", data["id"])
	}

	rec = env.doRequest("GET", "/api/v1/users/"+env.adminUserID, nil)
	data = env.parseData(rec)
	if data["id"] != env.adminUserID {
		t.Errorf("admin get self: id = %v", data["id"])
	}
}
