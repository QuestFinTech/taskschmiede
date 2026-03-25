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
	"fmt"
	"net/http"
	"strconv"

	"github.com/QuestFinTech/taskschmiede/internal/auth"
	"github.com/QuestFinTech/taskschmiede/internal/security"
	"github.com/QuestFinTech/taskschmiede/internal/storage"
)

// getAuthUser returns the authenticated user from context.
func getAuthUser(r *http.Request) *auth.AuthUser {
	return auth.GetAuthUser(r.Context())
}

// requireAdmin checks if the authenticated user is an admin.
// Returns true if admin, false with error response written if not.
func (a *API) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	if apiErr := a.CheckAdmin(r.Context()); apiErr != nil {
		writeAPIError(w, apiErr)
		return false
	}
	return true
}

// CheckAdmin verifies the context user is a master admin (system-level).
// Org owners/admins do not qualify -- this is for system administration only.
func (a *API) CheckAdmin(ctx context.Context) *APIError {
	user := auth.GetAuthUser(ctx)
	if user == nil {
		return errUnauthorized("Authentication required")
	}
	if !a.authSvc.IsMasterAdmin(ctx, user.UserID) {
		return errForbidden("Admin privileges required")
	}
	return nil
}

// ResolveEndeavourIDs returns the list of endeavour IDs the context user
// has access to. Returns nil when adminMode is true and the user is a master
// admin (no restriction). Returns an empty slice if the user has no endeavour access.
func (a *API) ResolveEndeavourIDs(ctx context.Context, adminMode bool) []string {
	user := auth.GetAuthUser(ctx)
	if user == nil {
		return []string{}
	}

	scope, err := a.authSvc.ResolveUserScope(ctx, user.UserID)
	if err != nil {
		return []string{}
	}

	if adminMode && scope.IsMasterAdmin {
		return nil // nil = no restriction
	}

	ids := make([]string, 0, len(scope.Endeavours))
	for id := range scope.Endeavours {
		ids = append(ids, id)
	}
	return ids
}

// resolveEndeavourIDs is the HTTP convenience wrapper.
// Admin mode is activated via ?admin=true query parameter.
func (a *API) resolveEndeavourIDs(r *http.Request) []string {
	adminMode := r.URL.Query().Get("admin") == "true"
	return a.ResolveEndeavourIDs(r.Context(), adminMode)
}

// resolveOrganizationIDs returns the list of organization IDs the context user
// belongs to. Returns nil for master admins (no restriction).
func (a *API) resolveOrganizationIDs(r *http.Request) []string {
	adminMode := r.URL.Query().Get("admin") == "true"
	user := auth.GetAuthUser(r.Context())
	if user == nil {
		return []string{}
	}
	scope, err := a.authSvc.ResolveUserScope(r.Context(), user.UserID)
	if err != nil {
		return []string{}
	}
	if adminMode && scope.IsMasterAdmin {
		return nil
	}
	ids := make([]string, 0, len(scope.Organizations))
	for id := range scope.Organizations {
		ids = append(ids, id)
	}
	return ids
}

// ResolveAssigneeMe resolves the "me" shorthand to the user's resource_id.
func (a *API) ResolveAssigneeMe(ctx context.Context, assigneeID string) string {
	if assigneeID != "me" {
		return assigneeID
	}
	user := auth.GetAuthUser(ctx)
	if user == nil {
		return ""
	}
	u, err := a.usrSvc.Get(ctx, user.UserID)
	if err == nil && u.ResourceID != nil {
		return *u.ResourceID
	}
	return ""
}

// getTierDef resolves the authenticated user's tier definition.
// Returns nil for master admins (no limits apply).
func (a *API) getTierDef(ctx context.Context) (*storage.TierDefinition, *storage.User, *APIError) {
	authUser := auth.GetAuthUser(ctx)
	if authUser == nil {
		return nil, nil, errUnauthorized("Authentication required")
	}
	if a.authSvc.IsMasterAdmin(ctx, authUser.UserID) {
		return nil, nil, nil // master admin bypasses all limits
	}
	user, err := a.usrSvc.Get(ctx, authUser.UserID)
	if err != nil {
		return nil, nil, errInternal("Failed to get user")
	}
	td, err := a.db.GetTierDefinition(user.Tier)
	if err != nil {
		return nil, user, nil // no tier def = no limits
	}
	return td, user, nil
}

// CheckTierLimit verifies the authenticated user has not exceeded their tier limit
// for the given entity type ("orgs" or "active_endeavours").
// Returns nil if within limits. Master admins bypass all limits.
func (a *API) CheckTierLimit(ctx context.Context, entityType string) *APIError {
	td, _, apiErr := a.getTierDef(ctx)
	if apiErr != nil {
		return apiErr
	}
	if td == nil {
		return nil // admin or no tier def
	}

	authUser := auth.GetAuthUser(ctx)
	var limit, current int
	switch entityType {
	case "orgs":
		limit = td.MaxOrgs
		current = a.countUserOwnedOrgs(ctx, authUser.UserID)
	case "active_endeavours":
		limit = td.MaxActiveEndeavours
		current = a.countUserOwnedActiveEndeavours(ctx, authUser.UserID)
	default:
		return nil
	}

	if storage.LimitExceeded(current, limit) {
		return errTierLimit(fmt.Sprintf("%s tier limit reached: maximum %d %s", td.Name, limit, entityType))
	}
	return nil
}

// countUserOwnedOrgs counts organizations where the user's resource has role "owner".
func (a *API) countUserOwnedOrgs(ctx context.Context, userID string) int {
	user, err := a.usrSvc.Get(ctx, userID)
	if err != nil || user.ResourceID == nil || *user.ResourceID == "" {
		return 0
	}

	var count int
	err = a.db.QueryRow(
		`SELECT COUNT(*) FROM entity_relation
		 WHERE relationship_type = 'has_member'
		   AND source_entity_type = 'organization'
		   AND target_entity_type = 'resource'
		   AND target_entity_id = ?
		   AND json_extract(metadata, '$.role') = 'owner'`,
		*user.ResourceID,
	).Scan(&count)
	if err != nil {
		return 0
	}
	return count
}

// countUserOwnedActiveEndeavours counts endeavours where the user has role "owner"
// and the endeavour status is active or pending.
func (a *API) countUserOwnedActiveEndeavours(ctx context.Context, userID string) int {
	var count int
	err := a.db.QueryRow(
		`SELECT COUNT(*) FROM entity_relation er
		 JOIN endeavour e ON e.id = er.target_entity_id
		 WHERE er.relationship_type = 'member_of'
		   AND er.source_entity_type = 'user'
		   AND er.source_entity_id = ?
		   AND er.target_entity_type = 'endeavour'
		   AND json_extract(er.metadata, '$.role') = 'owner'
		   AND e.status IN ('active', 'pending')`,
		userID,
	).Scan(&count)
	if err != nil {
		return 0
	}
	return count
}

// --------------------------------------------------------------------------
// Per-Org Quota Enforcement
// --------------------------------------------------------------------------

// FindUserOrgID finds the organization ID where the user's resource has role
// "owner". Returns empty string if not found. Explorer tier users have at most 1 org.
func (a *API) FindUserOrgID(ctx context.Context, userID string) string {
	return a.findUserOrgID(ctx, userID)
}

func (a *API) findUserOrgID(ctx context.Context, userID string) string {
	user, err := a.usrSvc.Get(ctx, userID)
	if err != nil || user.ResourceID == nil || *user.ResourceID == "" {
		return ""
	}

	var orgID string
	err = a.db.QueryRow(
		`SELECT source_entity_id FROM entity_relation
		 WHERE relationship_type = 'has_member'
		   AND source_entity_type = 'organization'
		   AND target_entity_type = 'resource'
		   AND target_entity_id = ?
		   AND json_extract(metadata, '$.role') = 'owner'
		 LIMIT 1`,
		*user.ResourceID,
	).Scan(&orgID)
	if err != nil {
		return ""
	}
	return orgID
}

// countOrgEndeavours counts all endeavours linked to the org (via
// participates_in relation) that are not archived. Used for per-org
// total quota enforcement.
func (a *API) countOrgEndeavours(ctx context.Context, orgID string) int {
	var count int
	err := a.db.QueryRow(
		`SELECT COUNT(*) FROM entity_relation er
		 JOIN endeavour e ON e.id = er.target_entity_id
		 WHERE er.relationship_type = 'participates_in'
		   AND er.source_entity_type = 'organization'
		   AND er.source_entity_id = ?
		   AND er.target_entity_type = 'endeavour'
		   AND e.status NOT IN ('archived', 'completed')`,
		orgID,
	).Scan(&count)
	if err != nil {
		return 0
	}
	return count
}

// countOrgAgents counts agent-type users whose resource is a member of the org.
func (a *API) countOrgAgents(ctx context.Context, orgID string) int {
	var count int
	err := a.db.QueryRow(
		`SELECT COUNT(*) FROM entity_relation er
		 JOIN user u ON u.resource_id = er.target_entity_id
		 WHERE er.relationship_type = 'has_member'
		   AND er.source_entity_type = 'organization'
		   AND er.source_entity_id = ?
		   AND er.target_entity_type = 'resource'
		   AND u.user_type = 'agent'`,
		orgID,
	).Scan(&count)
	if err != nil {
		return 0
	}
	return count
}

// CheckOrgQuota verifies that the authenticated user's org has not exceeded the
// per-org quota for the given type ("endeavours_per_org", "agents_per_org",
// or "teams_per_org"). Returns nil if within limits. Master admins bypass.
func (a *API) CheckOrgQuota(ctx context.Context, orgID, quotaType string) *APIError {
	authUser := auth.GetAuthUser(ctx)
	if authUser == nil {
		return errUnauthorized("Authentication required")
	}
	return a.checkOrgQuotaForUser(ctx, authUser.UserID, orgID, quotaType)
}

// CheckOrgQuotaForUser verifies that the given user's org has not exceeded the
// per-org quota. Used when the auth context differs from the sponsor (e.g.,
// during agent registration where the caller is the registering agent but the
// limit comes from the sponsor's tier).
func (a *API) CheckOrgQuotaForUser(ctx context.Context, userID, orgID, quotaType string) *APIError {
	return a.checkOrgQuotaForUser(ctx, userID, orgID, quotaType)
}

func (a *API) checkOrgQuotaForUser(ctx context.Context, userID, orgID, quotaType string) *APIError {
	if a.authSvc.IsMasterAdmin(ctx, userID) {
		return nil
	}

	user, err := a.usrSvc.Get(ctx, userID)
	if err != nil {
		return errInternal("Failed to get user")
	}

	td, err := a.db.GetTierDefinition(user.Tier)
	if err != nil {
		return nil // no tier def = no limits
	}

	var limit, current int
	switch quotaType {
	case "endeavours_per_org":
		limit = td.MaxEndeavoursPerOrg
		current = a.countOrgEndeavours(ctx, orgID)
	case "agents_per_org":
		limit = td.MaxAgentsPerOrg
		current = a.countOrgAgents(ctx, orgID)
	case "teams_per_org":
		limit = td.MaxTeamsPerOrg
		current = a.countOrgTeams(ctx, orgID)
	default:
		return nil
	}

	if storage.LimitExceeded(current, limit) {
		return errTierLimit(fmt.Sprintf("%s tier limit reached: maximum %d %s", td.Name, limit, quotaType))
	}
	return nil
}

// countOrgTeams counts active team-type resources that are members of the org.
func (a *API) countOrgTeams(ctx context.Context, orgID string) int {
	var count int
	err := a.db.QueryRow(
		`SELECT COUNT(*) FROM entity_relation er
		 JOIN resource r ON r.id = er.target_entity_id
		 WHERE er.relationship_type = 'has_member'
		   AND er.source_entity_type = 'organization'
		   AND er.source_entity_id = ?
		   AND er.target_entity_type = 'resource'
		   AND r.type = 'team'
		   AND r.status = 'active'`,
		orgID,
	).Scan(&count)
	if err != nil {
		return 0
	}
	return count
}

// countPendingAgentSlots counts the total remaining uses across all active,
// non-expired invitation tokens owned by the given user. This represents
// how many more agents could register with existing tokens.
func (a *API) countPendingAgentSlots(ctx context.Context, userID string) int {
	var count int
	err := a.db.QueryRow(
		`SELECT COALESCE(SUM(max_uses - use_count), 0)
		 FROM invitation_token
		 WHERE created_by = ?
		   AND status = 'active'
		   AND (expires_at IS NULL OR expires_at > datetime('now'))
		   AND max_uses > use_count`,
		userID,
	).Scan(&count)
	if err != nil {
		return 0
	}
	return count
}

// --------------------------------------------------------------------------
// Resource Creation Velocity
// --------------------------------------------------------------------------

// CheckCreationVelocity verifies the authenticated user has not exceeded
// their per-hour entity creation limit. On success, the creation is recorded
// in the velocity tracker. Master admins bypass the check.
func (a *API) CheckCreationVelocity(ctx context.Context) *APIError {
	authUser := auth.GetAuthUser(ctx)
	if authUser == nil {
		return nil // no auth context = called from system; skip check
	}

	td, _, apiErr := a.getTierDef(ctx)
	if apiErr != nil || td == nil {
		return nil // admin, no tier def, or auth error
	}

	limit := td.MaxCreationsPerHour
	if limit < 0 {
		return nil // unlimited
	}

	count := a.velocity.record(authUser.UserID)
	if count > limit {
		return errVelocityLimit(fmt.Sprintf("Creation rate limit reached: maximum %d entities per hour for %s tier. Try again later.", limit, td.Name))
	}
	return nil
}

// CreationVelocityCurrent returns the number of entities created by the
// given user in the current hourly window. Used by Whoami.
func (a *API) CreationVelocityCurrent(userID string) int {
	return a.velocity.current(userID)
}

// queryInt reads an integer query parameter with a default value.
func queryInt(r *http.Request, name string, defaultVal int) int {
	s := r.URL.Query().Get(name)
	if s == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal
	}
	// Cap list endpoints to prevent bulk extraction.
	if name == "limit" && v > 200 {
		v = 200
	}
	return v
}

// queryString reads a string query parameter, applying input hygiene.
func queryString(r *http.Request, name string) string {
	return security.SanitizeInput(r.URL.Query().Get(name))
}

// sanitize applies input hygiene to a string value from a decoded JSON body.
func sanitize(s string) string {
	return security.SanitizeInput(s)
}

// sanitizePtr applies input hygiene to an optional string pointer from a decoded JSON body.
func sanitizePtr(s *string) *string {
	if s == nil {
		return nil
	}
	v := security.SanitizeInput(*s)
	return &v
}

// sanitizeStrings applies input hygiene to each element of a string slice.
func sanitizeStrings(ss []string) []string {
	if ss == nil {
		return nil
	}
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = security.SanitizeInput(s)
	}
	return out
}

// sanitizeStringsPtr applies input hygiene to each element of an optional string slice pointer.
func sanitizeStringsPtr(ss *[]string) *[]string {
	if ss == nil {
		return nil
	}
	out := sanitizeStrings(*ss)
	return &out
}

// --------------------------------------------------------------------------
// Per-Resource RBAC helpers (WS-1.2)
// --------------------------------------------------------------------------

// resolveScope resolves the authenticated user's scope from context.
// Returns the scope or an APIError if the user is not authenticated or
// scope resolution fails.
func (a *API) resolveScope(ctx context.Context) (*auth.UserScope, *APIError) {
	user := auth.GetAuthUser(ctx)
	if user == nil {
		return nil, errUnauthorized("Authentication required")
	}
	scope, err := a.authSvc.ResolveUserScope(ctx, user.UserID)
	if err != nil {
		return nil, errInternal("Failed to resolve user scope")
	}
	return scope, nil
}

// checkEndeavourRead verifies the user has at least viewer access to the
// given endeavour. Returns nil on success. Unscoped entities (empty
// endeavourID) require master admin access.
func checkEndeavourRead(scope *auth.UserScope, endeavourID string) *APIError {
	if scope.IsMasterAdmin {
		return nil
	}
	if endeavourID == "" {
		return errNotFound("entity", "Not found")
	}
	if !scope.CanRead(endeavourID) {
		return errNotFound("entity", "Not found")
	}
	return nil
}

// checkEndeavourWrite verifies the user has at least member access to the
// given endeavour. Returns nil on success.
func checkEndeavourWrite(scope *auth.UserScope, endeavourID string) *APIError {
	if scope.IsMasterAdmin {
		return nil
	}
	if endeavourID == "" {
		return errNotFound("entity", "Not found")
	}
	if !scope.CanWrite(endeavourID) {
		return errNotFound("entity", "Not found")
	}
	return nil
}

// checkEndeavourAdmin verifies the user has admin access to the given
// endeavour. Returns nil on success.
func checkEndeavourAdmin(scope *auth.UserScope, endeavourID string) *APIError {
	if scope.IsMasterAdmin {
		return nil
	}
	if endeavourID == "" {
		return errNotFound("entity", "Not found")
	}
	if !scope.CanAdmin(endeavourID) {
		return errNotFound("entity", "Not found")
	}
	return nil
}

// checkEndeavourOwner verifies the user has owner access to the given
// endeavour. Returns nil on success.
func checkEndeavourOwner(scope *auth.UserScope, endeavourID string) *APIError {
	if scope.IsMasterAdmin {
		return nil
	}
	if endeavourID == "" {
		return errNotFound("entity", "Not found")
	}
	if !scope.IsOwner(endeavourID) {
		return errNotFound("entity", "Not found")
	}
	return nil
}

// resolveEntityEndeavourID looks up the endeavour_id for a parent entity
// referenced by comments and approvals. Supports task, demand, endeavour,
// artifact, ritual, and organization entity types.
func (a *API) resolveEntityEndeavourID(ctx context.Context, entityType, entityID string) (string, *APIError) {
	switch entityType {
	case "task":
		t, err := a.tskSvc.Get(ctx, entityID)
		if err != nil {
			return "", errNotFound("task", "Not found")
		}
		return t.EndeavourID, nil
	case "demand":
		d, err := a.dmdSvc.Get(ctx, entityID)
		if err != nil {
			return "", errNotFound("demand", "Not found")
		}
		return d.EndeavourID, nil
	case "endeavour":
		return entityID, nil
	case "artifact":
		art, err := a.artSvc.Get(ctx, entityID)
		if err != nil {
			return "", errNotFound("artifact", "Not found")
		}
		return art.EndeavourID, nil
	case "ritual":
		r, err := a.rtlSvc.Get(ctx, entityID)
		if err != nil {
			return "", errNotFound("ritual", "Not found")
		}
		return r.EndeavourID, nil
	case "organization":
		// Organization comments use org-level access, not endeavour.
		// Return empty string; callers should use org-level checks.
		return "", nil
	default:
		return "", errNotFound("entity", "Not found")
	}
}

// isOrgAdminOfResource checks whether the user (via scope) is an admin or
// owner of any organization that the given resource belongs to.
func (a *API) isOrgAdminOfResource(scope *auth.UserScope, resourceID string) bool {
	if scope.IsMasterAdmin {
		return true
	}
	// Find all orgs this resource belongs to (organization -> has_member -> resource).
	rels, _, err := a.db.ListRelations(storage.ListRelationsOpts{
		TargetEntityID:   resourceID,
		TargetEntityType: "resource",
		RelationshipType: storage.RelHasMember,
		Limit:            100,
	})
	if err != nil {
		return false
	}
	for _, rel := range rels {
		if rel.SourceEntityType == "organization" {
			if scope.CanAdminOrg(rel.SourceEntityID) {
				return true
			}
		}
	}
	return false
}

// isAnyOrgAdmin returns true if the user has admin or owner role in at least
// one organization. Master admin is handled separately by callers.
func isAnyOrgAdmin(scope *auth.UserScope) bool {
	for _, role := range scope.Organizations {
		if role == "owner" || role == "admin" {
			return true
		}
	}
	return false
}

// isTeamMember checks whether callerResourceID is a member of the team
// identified by teamResourceID. Returns false if the resource is not a team
// or if there is no has_member relation.
func (a *API) isTeamMember(teamResourceID, callerResourceID string) bool {
	if teamResourceID == "" || callerResourceID == "" {
		return false
	}
	// Verify the resource is actually a team.
	res, err := a.resSvc.Get(context.Background(), teamResourceID)
	if err != nil || res.Type != "team" {
		return false
	}
	// Check for a has_member relation from the team to the caller.
	rels, _, err := a.db.ListRelations(storage.ListRelationsOpts{
		SourceEntityID:   teamResourceID,
		SourceEntityType: "resource",
		TargetEntityID:   callerResourceID,
		TargetEntityType: "resource",
		RelationshipType: storage.RelHasMember,
		Limit:            1,
	})
	return err == nil && len(rels) > 0
}

// getTeamResource returns the team resource for the given ID, or nil if the
// resource is not a team.
func (a *API) getTeamResource(resourceID string) *storage.Resource {
	if resourceID == "" {
		return nil
	}
	res, err := a.resSvc.Get(context.Background(), resourceID)
	if err != nil || res.Type != "team" {
		return nil
	}
	return res
}

// teamQuorumRequired returns the quorum count required for the given action
// on a team resource. It reads from the team's metadata.quorum map:
//
//	metadata.quorum.cancel   -- quorum for cancel actions
//	metadata.quorum.fulfill  -- quorum for fulfill actions
//	metadata.quorum.default  -- fallback for any action
//
// Returns 0 if no quorum is configured (meaning no quorum enforcement).
func teamQuorumRequired(team *storage.Resource, action string) int {
	if team == nil || team.Metadata == nil {
		return 0
	}
	quorumRaw, ok := team.Metadata["quorum"]
	if !ok {
		return 0
	}
	quorumMap, ok := quorumRaw.(map[string]interface{})
	if !ok {
		return 0
	}
	// Try action-specific quorum first.
	if v, ok := quorumMap[action]; ok {
		return toInt(v)
	}
	// Fall back to default.
	if v, ok := quorumMap["default"]; ok {
		return toInt(v)
	}
	return 0
}

// toInt converts a numeric interface value to int. Handles float64 (from JSON)
// and json.Number.
func toInt(v interface{}) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	case string:
		i, _ := strconv.Atoi(n)
		return i
	default:
		return 0
	}
}

// checkQuorum verifies whether enough team members have approved the given
// action on an entity. It auto-records the caller's approval and returns
// whether the quorum threshold has been met.
//
// Returns:
//   - quorumMet: true if the required number of distinct team member approvals
//     has been reached (including the caller's new approval)
//   - currentCount: total distinct approvals after recording the caller's
//   - requiredCount: the quorum threshold
//   - err: any error during the process
func (a *API) checkQuorum(ctx context.Context, entityType, entityID string, team *storage.Resource, action, callerResourceID string) (quorumMet bool, currentCount, requiredCount int, err error) {
	requiredCount = teamQuorumRequired(team, action)
	if requiredCount < 2 {
		// No meaningful quorum -- single approval is enough.
		return true, 1, requiredCount, nil
	}

	// Record the caller's approval for this action.
	role := "quorum_" + action
	_, createErr := a.aprSvc.Create(ctx, entityType, entityID, callerResourceID, role, "approved", "Quorum vote for "+action, nil)
	if createErr != nil {
		return false, 0, requiredCount, createErr
	}

	// Count distinct approved votes from team members for this action.
	approvals, _, listErr := a.aprSvc.List(ctx, storage.ListApprovalsOpts{
		EntityType: entityType,
		EntityID:   entityID,
		Verdict:    "approved",
		Role:       role,
		Limit:      100,
	})
	if listErr != nil {
		return false, 0, requiredCount, listErr
	}

	// Count distinct approvers who are team members.
	seen := make(map[string]bool)
	for _, apr := range approvals {
		if !seen[apr.ApproverID] && a.isTeamMember(team.ID, apr.ApproverID) {
			seen[apr.ApproverID] = true
		}
	}
	currentCount = len(seen)
	quorumMet = currentCount >= requiredCount
	return quorumMet, currentCount, requiredCount, nil
}

// errQuorumNotMet returns an APIError indicating that the quorum has not been
// met. The caller's approval has been recorded and the response includes
// progress information.
func errQuorumNotMet(action string, currentCount, requiredCount int) *APIError {
	return &APIError{
		Code:    "quorum_not_met",
		Message: fmt.Sprintf("Your approval for %s has been recorded. %d of %d approvals received.", action, currentCount, requiredCount),
		Status:  http.StatusPreconditionFailed,
		Details: map[string]interface{}{
			"action":         action,
			"current_count":  currentCount,
			"required_count": requiredCount,
		},
	}
}

// IsAtCapacity returns true if the instance has reached its active user cap.
func (a *API) IsAtCapacity() bool {
	maxUsers := a.policyInt("instance.max_active_users", 200)
	if maxUsers <= 0 {
		return false // unlimited
	}
	return a.db.CountActiveUsers() >= maxUsers
}

// policyInt reads an integer policy value with a fallback default.
func (a *API) policyInt(key string, defaultVal int) int {
	val, err := a.db.GetPolicy(key)
	if err != nil {
		return defaultVal
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		return defaultVal
	}
	return n
}
