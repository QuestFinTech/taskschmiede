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


package portal

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuestFinTech/taskschmiede/internal/storage"
)

// --- Admin helper types ---

// TeamMember holds resolved data about a team member for the admin UI.
type TeamMember struct {
	RelationID   string
	ResourceID   string
	ResourceName string
	ResourceType string
	Role         string
}

// userOrgInfo holds a user's organization membership for display.
type userOrgInfo struct {
	ID   string
	Name string
}

// --- Activity helpers ---

// sourceLabel returns a human-readable suffix from the audit source field.
func sourceLabel(e *AuditEntry) string {
	switch e.Source {
	case "console":
		return " via Console"
	case "portal":
		return " via Portal"
	case "mcp":
		return " via MCP"
	case "api":
		return " via API"
	case "system":
		return " (system)"
	default:
		return ""
	}
}

// activitySummary maps an audit action to human-readable text.
func activitySummary(e *AuditEntry) string {
	switch e.Action {
	case "login_success":
		return "Logged in" + sourceLabel(e)
	case "login_failure":
		return "Login attempt failed" + sourceLabel(e)
	case "token_created":
		return "Created API token"
	case "token_revoked":
		return "Revoked API token"
	case "password_changed":
		return "Changed password"
	case "profile_updated":
		return "Updated profile"
	case "password_reset_requested":
		return "Requested password reset"
	case "user_registered":
		return "Registered account"
	case "user_verified":
		return "Verified email"
	case "permission_denied":
		return fmt.Sprintf("Access denied: %s %s", e.Method, e.Endpoint)
	case "rate_limit_hit":
		return "Rate limit reached"
	case "dod_override":
		return "Overrode Definition of Done"
	case "security_alert":
		return "Security alert"
	case "invitation_created":
		return "Created invitation"
	case "invitation_revoked":
		return "Revoked invitation"
	case "agent_blocked":
		return "Blocked agent"
	case "agent_unblocked":
		return "Unblocked agent"
	case "request":
		if e.Method != "" && e.Endpoint != "" {
			return fmt.Sprintf("%s %s (%d)", e.Method, e.Endpoint, e.StatusCode)
		}
		return "API request"
	default:
		return e.Action
	}
}

// buildActorMap collects unique actor IDs from audit entries and resolves
// them to display names by fetching users and resources.
func (s *Server) buildActorMap(token string, entries []*AuditEntry) map[string]string {
	actorMap := map[string]string{}

	seen := map[string]bool{}
	for _, e := range entries {
		if e.ActorID != "" && e.ActorID != "anonymous" && e.ActorID != "system" {
			seen[e.ActorID] = true
		}
	}
	if len(seen) == 0 {
		return actorMap
	}

	users, _, _ := s.rest.ListUsers(token, "", "", 200, 0)
	for _, u := range users {
		actorMap[u.ID] = u.Name
	}

	resources, _, _ := s.rest.AdminListResources(token, "", "", "", "", 200, 0)
	for _, r := range resources {
		actorMap[r.ID] = r.Name
	}

	return actorMap
}

// --- Admin handlers ---

func (s *Server) handleAdminOverview(w http.ResponseWriter, r *http.Request) {
	token := getToken(r)
	user, _ := s.rest.Whoami(token)

	data := PageData{
		Title:       "Admin Overview",
		User:        user,
		IsAdmin:     true,
		CurrentPage: "admin_overview",
	}

	stats, _ := s.rest.AdminStats(token)
	kpi, _ := s.rest.KPICurrent(token)
	cgStats, _ := s.rest.AdminContentGuardStats(token)

	// Ablecon + Harmcon indicators
	indicators, _ := s.rest.AdminIndicatorsData(token)

	// Block signals
	blockSignals, _ := s.rest.GetAdminAgentBlockSignals(token)

	// Tier usage
	tierUsage, _ := s.rest.AdminTierUsage(token)

	// System-wide activity summary (last 24h)
	userTZ := "UTC"
	if user != nil {
		if tz, ok := user["timezone"].(string); ok && tz != "" {
			userTZ = tz
		}
	}
	now := storage.UTCNow()
	yesterday := now.Add(-24 * time.Hour)
	var activitySummary ActivitySummary
	var activityHourly []HourBucket
	activityHourlyMax := 0
	activityResp, _ := s.rest.AdminListActivity(token, "", "", "", yesterday.Format(time.RFC3339), now.Format(time.RFC3339), 1, 0)
	if activityResp != nil {
		activitySummary = activityResp.Summary
		loc, err := time.LoadLocation(userTZ)
		if err != nil {
			loc = time.UTC
		}
		localNow := now.In(loc)
		currentHour := localNow.Hour()
		activityHourly = make([]HourBucket, 24)
		for i := 0; i < 24; i++ {
			srcHour := (currentHour - i + 24) % 24
			activityHourly[i] = activityResp.Hourly[srcHour]
			activityHourly[i].Hour = srcHour
		}
		for _, b := range activityHourly {
			s := b.Logins + b.Tasks + b.Demands + b.Endeavours
			if s > activityHourlyMax {
				activityHourlyMax = s
			}
		}
	}

	data.Data = map[string]interface{}{
		"Stats":              stats,
		"KPI":                kpi,
		"CGStats":            cgStats,
		"Indicators":         indicators,
		"BlockSignals":       blockSignals,
		"TierUsage":          tierUsage,
		"ActivitySummary":    activitySummary,
		"ActivityHourly":     activityHourly,
		"ActivityHourlyMax":  activityHourlyMax,
	}

	s.render(w, r, "admin_overview.html", data)
}

// handleAdminUsers serves the adminusers portal page.
func (s *Server) handleAdminUsers(w http.ResponseWriter, r *http.Request) {
	token := getToken(r)
	user, _ := s.rest.Whoami(token)

	data := PageData{
		Title:       "Users",
		User:        user,
		IsAdmin:     true,
		CurrentPage: "admin_users",
	}

	if r.Method == http.MethodPost {
		if !validateCSRF(r) {
			s.csrfFailed(w, r, "admin_users.html")
			return
		}
		action := r.FormValue("action")
		userID := r.FormValue("user_id")
		switch action {
		case "toggle_status":
			if userID != "" {
				if myID, ok := user["user_id"].(string); ok && myID == userID {
					data.Error = s.msg(r, user, "admin.users.errors.cannot_suspend_self")
				} else {
					u, err := s.rest.GetUser(token, userID)
					if err != nil {
						data.Error = s.msg(r, user, "admin.users.errors.failed_load", err.Error())
					} else {
						newStatus := "inactive"
						if u.Status == "inactive" {
							newStatus = "active"
						}
						if _, err := s.rest.UpdateUser(token, userID, map[string]interface{}{"status": newStatus}); err != nil {
							data.Error = s.msg(r, user, "admin.users.errors.failed_update", err.Error())
						}
					}
				}
			}
		case "suspend":
			if userID != "" {
				if myID, ok := user["user_id"].(string); ok && myID == userID {
					data.Error = s.msg(r, user, "admin.users.errors.cannot_suspend_self")
				} else if _, err := s.rest.UpdateUser(token, userID, map[string]interface{}{"status": "suspended"}); err != nil {
					data.Error = s.msg(r, user, "admin.users.errors.failed_update", err.Error())
				}
			}
		case "reactivate":
			if userID != "" {
				if _, err := s.rest.UpdateUser(token, userID, map[string]interface{}{"status": "active"}); err != nil {
					data.Error = s.msg(r, user, "admin.users.errors.failed_update", err.Error())
				}
			}
		case "promote_admin":
			if userID != "" {
				// Prevent promoting yourself (already admin).
				if myID, ok := user["user_id"].(string); ok && myID == userID {
					data.Error = s.msg(r, user, "admin.users.errors.already_admin")
				} else {
					if _, err := s.rest.UpdateUser(token, userID, map[string]interface{}{"is_admin": true}); err != nil {
						data.Error = s.msg(r, user, "admin.users.errors.failed_update", err.Error())
					}
				}
			}
		case "demote_admin":
			if userID != "" {
				// Prevent demoting yourself (lockout protection).
				if myID, ok := user["user_id"].(string); ok && myID == userID {
					data.Error = s.msg(r, user, "admin.users.errors.cannot_demote_self")
				} else {
					if _, err := s.rest.UpdateUser(token, userID, map[string]interface{}{"is_admin": false}); err != nil {
						data.Error = s.msg(r, user, "admin.users.errors.failed_update", err.Error())
					}
				}
			}
		}
		// PRG: redirect back with same query params.
		if data.Error == "" {
			q := r.URL.Query()
			q.Set("search", r.FormValue("search"))
			q.Set("status", r.FormValue("filter_status"))
			q.Set("offset", r.FormValue("offset"))
			http.Redirect(w, r, "/admin/users?"+q.Encode(), http.StatusSeeOther)
			return
		}
	}

	search := r.URL.Query().Get("search")
	status := r.URL.Query().Get("status")
	limit := 50
	offset := queryIntParam(r, "offset", 0)

	users, total, err := s.rest.ListUsers(token, search, status, limit, offset)
	if err != nil {
		data.Error = s.msg(r, user, "admin.users.errors.failed_load", err.Error())
		data.Data = map[string]interface{}{
			"Users":    []*User{},
			"UserOrgs": map[string][]userOrgInfo{},
			"Total":    0,
			"Search":   search,
			"Status":   status,
			"Offset":   offset,
			"Limit":    limit,
		}
		s.render(w, r, "admin_users.html", data)
		return
	}

	// Build org membership map: resource ID -> []userOrgInfo.
	resOrgs := map[string][]userOrgInfo{}
	orgs, _, orgErr := s.rest.AdminListOrganizations(token, "", "", 200, 0)
	if orgErr == nil {
		for _, org := range orgs {
			resources, _, resErr := s.rest.AdminListResources(token, "", "", "", org.ID, 200, 0)
			if resErr != nil {
				continue
			}
			for _, res := range resources {
				resOrgs[res.ID] = append(resOrgs[res.ID], userOrgInfo{ID: org.ID, Name: org.Name})
			}
		}
	}
	userOrgs := map[string][]userOrgInfo{}
	for _, u := range users {
		if u.ResourceID != "" {
			if infos, ok := resOrgs[u.ResourceID]; ok {
				userOrgs[u.ID] = infos
			}
		}
	}

	nextOffset := offset + limit
	data.Data = map[string]interface{}{
		"Users":      users,
		"UserOrgs":   userOrgs,
		"Total":      total,
		"Search":     search,
		"Status":     status,
		"Offset":     offset,
		"Limit":      limit,
		"NextOffset": nextOffset,
		"HasNext":    nextOffset < total,
		"HasPrev":    offset > 0,
	}
	s.render(w, r, "admin_users.html", data)
}

// handleAdminUserDetail serves the adminuserdetail portal page.
func (s *Server) handleAdminUserDetail(w http.ResponseWriter, r *http.Request) {
	token := getToken(r)
	user, _ := s.rest.Whoami(token)
	id := r.PathValue("id")

	data := PageData{
		Title:       "User",
		User:        user,
		IsAdmin:     true,
		CurrentPage: "admin_users",
	}

	if r.Method == http.MethodPost {
		if !validateCSRF(r) {
			s.csrfFailed(w, r, "admin_user_detail.html")
			return
		}
		action := r.FormValue("action")
		switch action {
		case "toggle_status":
			u, err := s.rest.GetUser(token, id)
			if err != nil {
				data.Error = s.msg(r, user, "admin.users.errors.failed_load", err.Error())
				break
			}
			newStatus := "inactive"
			if u.Status == "inactive" {
				newStatus = "active"
			}
			if _, err := s.rest.UpdateUser(token, id, map[string]interface{}{"status": newStatus}); err != nil {
				data.Error = s.msg(r, user, "admin.users.errors.failed_update", err.Error())
			} else {
				data.Success = s.msg(r, user, "admin.users.success.status_updated")
			}
		}
	}

	u, err := s.rest.GetUser(token, id)
	if err != nil {
		data.Error = s.msg(r, user, "admin.users.errors.failed_load", err.Error())
		s.render(w, r, "admin_user_detail.html", data)
		return
	}

	data.Title = u.Name
	data.Data = map[string]interface{}{
		"UserData": u,
		"UserName": u.Name,
	}
	s.render(w, r, "admin_user_detail.html", data)
}

// handleAdminResources serves the adminresources portal page.
func (s *Server) handleAdminResources(w http.ResponseWriter, r *http.Request) {
	token := getToken(r)
	user, _ := s.rest.Whoami(token)

	data := PageData{
		Title:       "Resources",
		User:        user,
		IsAdmin:     true,
		CurrentPage: "admin_resources",
	}

	if r.Method == http.MethodPost {
		if !validateCSRF(r) {
			s.csrfFailed(w, r, "admin_resources.html")
			return
		}
		action := r.FormValue("action")
		resID := r.FormValue("resource_id")
		if action == "toggle_status" && resID != "" {
			res, err := s.rest.GetResource(token, resID)
			if err != nil {
				data.Error = s.msg(r, user, "admin.resources.errors.failed_load", err.Error())
			} else {
				newStatus := "inactive"
				if res.Status == "inactive" {
					newStatus = "active"
				}
				if _, err := s.rest.UpdateResource(token, resID, map[string]interface{}{"status": newStatus}); err != nil {
					data.Error = s.msg(r, user, "admin.resources.errors.failed_update", err.Error())
				}
			}
		}
		if data.Error == "" {
			q := r.URL.Query()
			q.Set("search", r.FormValue("search"))
			q.Set("type", r.FormValue("filter_type"))
			q.Set("status", r.FormValue("filter_status"))
			q.Set("offset", r.FormValue("offset"))
			http.Redirect(w, r, "/admin/resources?"+q.Encode(), http.StatusSeeOther)
			return
		}
	}

	search := r.URL.Query().Get("search")
	resType := r.URL.Query().Get("type")
	status := r.URL.Query().Get("status")
	limit := 50
	offset := queryIntParam(r, "offset", 0)

	resources, total, err := s.rest.AdminListResources(token, search, resType, status, "", limit, offset)
	if err != nil {
		data.Error = s.msg(r, user, "admin.resources.errors.failed_load", err.Error())
		data.Data = map[string]interface{}{
			"Resources": []*Resource{},
			"Total":     0,
			"Search":    search,
			"Type":      resType,
			"Status":    status,
			"Offset":    offset,
			"Limit":     limit,
		}
		s.render(w, r, "admin_resources.html", data)
		return
	}

	nextOffset := offset + limit
	data.Data = map[string]interface{}{
		"Resources":  resources,
		"Total":      total,
		"Search":     search,
		"Type":       resType,
		"Status":     status,
		"Offset":     offset,
		"Limit":      limit,
		"NextOffset": nextOffset,
		"HasNext":    nextOffset < total,
		"HasPrev":    offset > 0,
	}
	s.render(w, r, "admin_resources.html", data)
}

// handleAdminResourceDetail serves the adminresourcedetail portal page.
func (s *Server) handleAdminResourceDetail(w http.ResponseWriter, r *http.Request) {
	token := getToken(r)
	user, _ := s.rest.Whoami(token)
	id := r.PathValue("id")

	data := PageData{
		Title:       "Resource",
		User:        user,
		IsAdmin:     true,
		CurrentPage: "admin_resources",
	}

	if r.Method == http.MethodPost {
		if !validateCSRF(r) {
			s.csrfFailed(w, r, "admin_resource_detail.html")
			return
		}
		action := r.FormValue("action")
		switch action {
		case "toggle_status":
			res, err := s.rest.GetResource(token, id)
			if err != nil {
				data.Error = s.msg(r, user, "admin.resources.errors.failed_load", err.Error())
				break
			}
			newStatus := "inactive"
			if res.Status == "inactive" {
				newStatus = "active"
			}
			if _, err := s.rest.UpdateResource(token, id, map[string]interface{}{"status": newStatus}); err != nil {
				data.Error = s.msg(r, user, "admin.resources.errors.failed_update", err.Error())
			} else {
				data.Success = s.msg(r, user, "admin.resources.success.status_updated")
			}
		}
	}

	res, err := s.rest.GetResource(token, id)
	if err != nil {
		data.Error = s.msg(r, user, "admin.resources.errors.failed_load", err.Error())
		s.render(w, r, "admin_resource_detail.html", data)
		return
	}

	data.Title = res.Name
	data.Data = map[string]interface{}{
		"Resource":     res,
		"ResourceName": res.Name,
	}
	s.render(w, r, "admin_resource_detail.html", data)
}

// --- Teams (user-facing, admin actions guarded by IsAdmin) ---

// TeamEndeavour holds resolved endeavour info for a team.
type TeamEndeavour struct {
	ID   string
	Name string
}

// linkTeamToEndeavourOrg finds the org that owns an endeavour and links the team to it.
func (s *Server) linkTeamToEndeavourOrg(token, teamID, edvID string) {
	// Find org that participates_in this endeavour.
	orgRels, _, _ := s.rest.ListRelations(token, "organization", "", "endeavour", edvID, "participates_in", 1, 0)
	if len(orgRels) > 0 {
		_, _ = s.rest.CreateRelation(token, "has_member", "organization", orgRels[0].SourceEntityID, "resource", teamID, map[string]interface{}{"role": "team"})
	}
}

// unlinkTeamEndeavourAndOrg removes team->endeavour and org->team links.
func (s *Server) unlinkTeamEndeavourAndOrg(token, teamID string) {
	// Remove endeavour links.
	if existing, _, _ := s.rest.ListRelations(token, "resource", teamID, "endeavour", "", "belongs_to", 10, 0); len(existing) > 0 {
		for _, rel := range existing {
			_ = s.rest.DeleteRelation(token, rel.ID)
		}
	}
	// Remove org membership links (where role=team).
	if orgLinks, _, _ := s.rest.ListRelations(token, "organization", "", "resource", teamID, "has_member", 10, 0); len(orgLinks) > 0 {
		for _, rel := range orgLinks {
			if role, ok := rel.Metadata["role"].(string); ok && role == "team" {
				_ = s.rest.DeleteRelation(token, rel.ID)
			}
		}
	}
}

// resolveOrgRoles returns the team roles defined for an org (from org metadata).
// Falls back to default roles if none are defined.
func (s *Server) resolveOrgRoles(token, orgID string) []string {
	defaults := []string{"Architect", "Developer", "System Administrator", "Lead", "Observer"}
	if orgID == "" {
		return defaults
	}
	org, err := s.rest.GetOrganization(token, orgID)
	if err != nil || org.Metadata == nil {
		return defaults
	}
	if raw, ok := org.Metadata["team_roles"]; ok {
		if arr, ok := raw.([]interface{}); ok && len(arr) > 0 {
			roles := make([]string, 0, len(arr))
			for _, v := range arr {
				if s, ok := v.(string); ok && s != "" {
					roles = append(roles, s)
				}
			}
			if len(roles) > 0 {
				return roles
			}
		}
	}
	return defaults
}

// autoAdjustQuorum caps quorum values at the current member count.
// Called after removing a member to prevent impossible quorum requirements.
func (s *Server) autoAdjustQuorum(token, teamID string) {
	members, _, _ := s.rest.ListRelations(token, "resource", teamID, "resource", "", "has_member", 100, 0)
	memberCount := len(members)

	res, err := s.rest.GetResource(token, teamID)
	if err != nil || res.Metadata == nil {
		return
	}
	q, ok := res.Metadata["quorum"].(map[string]interface{})
	if !ok {
		return
	}

	adjusted := false
	for _, key := range []string{"default", "cancel", "fulfill"} {
		if v, ok := q[key]; ok {
			var n int
			switch val := v.(type) {
			case float64:
				n = int(val)
			case int:
				n = val
			default:
				continue
			}
			if n > memberCount {
				q[key] = memberCount
				adjusted = true
			}
		}
	}

	if adjusted {
		meta := map[string]interface{}{}
		for k, v := range res.Metadata {
			meta[k] = v
		}
		meta["quorum"] = q
		_, _ = s.rest.UpdateResource(token, teamID, map[string]interface{}{"metadata": meta})
	}
}

// handleTeams serves the teams portal page.
func (s *Server) handleTeams(w http.ResponseWriter, r *http.Request) {
	token := getToken(r)
	user, _ := s.rest.Whoami(token)
	admin := isAdmin(user)
	canManage := canManageTeams(user)
	userOrgID := userPrimaryOrgID(user)

	data := PageData{
		Title:       "Teams",
		User:        user,
		IsAdmin:     admin,
		CurrentPage: "teams",
	}

	if r.Method == http.MethodPost && canManage {
		if !validateCSRF(r) {
			s.csrfFailed(w, r, "teams.html")
			return
		}
		action := r.FormValue("action")
		if action == "create" {
			name := r.FormValue("name")
			edvID := r.FormValue("endeavour_id")
			if name != "" {
				team, err := s.rest.CreateResource(token, map[string]interface{}{
					"type": "team",
					"name": name,
				})
				if err != nil {
					data.Error = s.msg(r, user, "admin.teams.errors.failed_create", err.Error())
				} else {
					// Link team to endeavour and its org if selected.
					if edvID != "" && team != nil {
						if _, linkErr := s.rest.CreateRelation(token, "belongs_to", "resource", team.ID, "endeavour", edvID, nil); linkErr != nil {
							data.Error = s.msg(r, user, "admin.teams.errors.failed_update", linkErr.Error())
						} else {
							s.linkTeamToEndeavourOrg(token, team.ID, edvID)
						}
					} else if !admin && userOrgID != "" && team != nil {
						// Non-admin: auto-link team to user's org.
						_, _ = s.rest.CreateRelation(token, "has_member", "organization", userOrgID, "resource", team.ID, map[string]interface{}{"role": "team"})
					}
					data.Success = s.msg(r, user, "admin.teams.success.created")
				}
			}
		}
	}

	if r.URL.Query().Get("deleted") == "1" {
		data.Success = s.msg(r, user, "admin.teams.success.deleted")
	}

	search := r.URL.Query().Get("search")
	status := r.URL.Query().Get("status")
	limit := 50
	offset := queryIntParam(r, "offset", 0)

	// Admins see all teams; org admins/owners see only their org's teams.
	var teams []*Resource
	var total int
	var err error
	if admin {
		teams, total, err = s.rest.AdminListResources(token, search, "team", status, "", limit, offset)
	} else if userOrgID != "" {
		teams, total, err = s.rest.AdminListResources(token, search, "team", status, userOrgID, limit, offset)
	}
	if err != nil {
		data.Error = s.msg(r, user, "admin.teams.errors.failed_load", err.Error())
	}

	// Resolve member counts and linked endeavour for each team.
	teamsWithCounts := make([]map[string]interface{}, 0, len(teams))
	for _, t := range teams {
		members, _, _ := s.rest.ListRelations(token, "resource", t.ID, "resource", "", "has_member", 100, 0)
		entry := map[string]interface{}{
			"ID":          t.ID,
			"Name":        t.Name,
			"Status":      t.Status,
			"CreatedAt":   t.CreatedAt,
			"MemberCount": len(members),
		}
		// Resolve linked endeavour.
		if edvRels, _, _ := s.rest.ListRelations(token, "resource", t.ID, "endeavour", "", "belongs_to", 1, 0); len(edvRels) > 0 {
			if edv, err := s.rest.GetEndeavour(token, edvRels[0].TargetEntityID); err == nil {
				entry["Endeavour"] = &TeamEndeavour{ID: edv.ID, Name: edv.Name}
			}
		}
		teamsWithCounts = append(teamsWithCounts, entry)
	}

	// Load endeavours for the create form.
	var endeavours []*Endeavour
	if canManage {
		endeavours, _, _ = s.rest.AdminListEndeavours(token, "", "", 200, 0)
	}

	nextOffset := offset + limit
	data.Data = map[string]interface{}{
		"Teams":          teamsWithCounts,
		"Total":          total,
		"Search":         search,
		"Status":         status,
		"Offset":         offset,
		"Limit":          limit,
		"NextOffset":     nextOffset,
		"HasNext":        nextOffset < total,
		"HasPrev":        offset > 0,
		"Endeavours":     endeavours,
		"CanManageTeams": canManage,
	}
	s.render(w, r, "teams.html", data)
}

// handleTeamDetail serves the teamdetail portal page.
func (s *Server) handleTeamDetail(w http.ResponseWriter, r *http.Request) {
	token := getToken(r)
	user, _ := s.rest.Whoami(token)
	id := r.PathValue("id")
	admin := isAdmin(user)
	canManage := canManageTeams(user)

	data := PageData{
		Title:       "Team",
		User:        user,
		IsAdmin:     admin,
		CurrentPage: "teams",
	}

	if r.Method == http.MethodPost && canManage {
		if !validateCSRF(r) {
			s.csrfFailed(w, r, "team_detail.html")
			return
		}
		action := r.FormValue("action")
		switch action {
		case "toggle_status":
			res, err := s.rest.GetResource(token, id)
			if err != nil {
				data.Error = s.msg(r, user, "admin.teams.errors.failed_load", err.Error())
				break
			}
			newStatus := "inactive"
			if res.Status == "inactive" {
				newStatus = "active"
			}
			if _, err := s.rest.UpdateResource(token, id, map[string]interface{}{"status": newStatus}); err != nil {
				data.Error = s.msg(r, user, "admin.teams.errors.failed_update", err.Error())
			} else {
				data.Success = s.msg(r, user, "admin.teams.success.status_updated")
			}
		case "add_member":
			resourceID := r.FormValue("resource_id")
			role := r.FormValue("role")
			if role == "" {
				role = "member"
			}
			if resourceID != "" {
				meta := map[string]interface{}{"role": role}
				if _, err := s.rest.CreateRelation(token, "has_member", "resource", id, "resource", resourceID, meta); err != nil {
					data.Error = s.msg(r, user, "admin.teams.errors.failed_add_member", err.Error())
				} else {
					data.Success = s.msg(r, user, "admin.teams.success.member_added")
				}
			}
		case "remove_member":
			relID := r.FormValue("relation_id")
			if relID != "" {
				if err := s.rest.DeleteRelation(token, relID); err != nil {
					data.Error = s.msg(r, user, "admin.teams.errors.failed_remove_member", err.Error())
				} else {
					data.Success = s.msg(r, user, "admin.teams.success.member_removed")
					// Auto-adjust quorum if it exceeds new member count.
					s.autoAdjustQuorum(token, id)
				}
			}
		case "update_quorum":
			// Cap quorum values at current member count.
			memberRels, _, _ := s.rest.ListRelations(token, "resource", id, "resource", "", "has_member", 100, 0)
			maxQ := len(memberRels)
			qDefault := r.FormValue("quorum_default")
			qCancel := r.FormValue("quorum_cancel")
			qFulfill := r.FormValue("quorum_fulfill")
			quorum := map[string]interface{}{}
			if qDefault != "" {
				if n, err := strconv.Atoi(qDefault); err == nil && n > 0 {
					if n > maxQ {
						n = maxQ
					}
					quorum["default"] = n
				}
			}
			if qCancel != "" {
				if n, err := strconv.Atoi(qCancel); err == nil && n > 0 {
					if n > maxQ {
						n = maxQ
					}
					quorum["cancel"] = n
				}
			}
			if qFulfill != "" {
				if n, err := strconv.Atoi(qFulfill); err == nil && n > 0 {
					if n > maxQ {
						n = maxQ
					}
					quorum["fulfill"] = n
				}
			}
			res, err := s.rest.GetResource(token, id)
			if err != nil {
				data.Error = s.msg(r, user, "admin.teams.errors.failed_load", err.Error())
				break
			}
			meta := map[string]interface{}{}
			if res.Metadata != nil {
				for k, v := range res.Metadata {
					meta[k] = v
				}
			}
			meta["quorum"] = quorum
			if _, err := s.rest.UpdateResource(token, id, map[string]interface{}{"metadata": meta}); err != nil {
				data.Error = s.msg(r, user, "admin.teams.errors.failed_update", err.Error())
			} else {
				data.Success = s.msg(r, user, "admin.teams.success.quorum_updated")
			}
		case "link_endeavour":
			edvID := r.FormValue("endeavour_id")
			if edvID != "" {
				// Remove existing endeavour and org links first.
				s.unlinkTeamEndeavourAndOrg(token, id)
				if _, err := s.rest.CreateRelation(token, "belongs_to", "resource", id, "endeavour", edvID, nil); err != nil {
					data.Error = s.msg(r, user, "admin.teams.errors.failed_update", err.Error())
				} else {
					// Auto-link team to the endeavour's org.
					s.linkTeamToEndeavourOrg(token, id, edvID)
					data.Success = s.msg(r, user, "admin.teams.success.endeavour_linked")
				}
			}
		case "unlink_endeavour":
			s.unlinkTeamEndeavourAndOrg(token, id)
			data.Success = s.msg(r, user, "admin.teams.success.endeavour_unlinked")
		case "delete":
			if err := s.rest.DeleteResource(token, id); err != nil {
				data.Error = s.msg(r, user, "admin.teams.errors.failed_delete", err.Error())
			} else {
				http.Redirect(w, r, "/teams?deleted=1", http.StatusSeeOther)
				return
			}
		}
	}

	team, err := s.rest.GetResource(token, id)
	if err != nil {
		data.Error = s.msg(r, user, "admin.teams.errors.failed_load", err.Error())
		s.render(w, r, "team_detail.html", data)
		return
	}

	// Resolve members.
	rels, _, _ := s.rest.ListRelations(token, "resource", id, "resource", "", "has_member", 100, 0)
	members := make([]*TeamMember, 0, len(rels))
	for _, rel := range rels {
		member := &TeamMember{
			RelationID:   rel.ID,
			ResourceID:   rel.TargetEntityID,
			ResourceName: rel.TargetEntityID,
			Role:         "member",
		}
		if rel.Metadata != nil {
			if role, ok := rel.Metadata["role"].(string); ok && role != "" {
				member.Role = role
			}
		}
		if res, err := s.rest.GetResource(token, rel.TargetEntityID); err == nil {
			member.ResourceName = res.Name
			member.ResourceType = res.Type
		}
		members = append(members, member)
	}

	// Extract quorum config from metadata.
	quorum := map[string]interface{}{
		"default": 1,
		"cancel":  0,
		"fulfill": 0,
	}
	if team.Metadata != nil {
		if q, ok := team.Metadata["quorum"].(map[string]interface{}); ok {
			if v, ok := q["default"]; ok {
				quorum["default"] = v
			}
			if v, ok := q["cancel"]; ok {
				quorum["cancel"] = v
			}
			if v, ok := q["fulfill"]; ok {
				quorum["fulfill"] = v
			}
		}
	}

	// Load resources for member add dropdown.
	// Filter out team-type resources -- teams cannot be members of teams.
	var resources []*Resource
	if canManage {
		userOrgID := userPrimaryOrgID(user)
		orgFilter := ""
		if !admin && userOrgID != "" {
			orgFilter = userOrgID
		}
		allRes, _, _ := s.rest.AdminListResources(token, "", "", "active", orgFilter, 200, 0)
		for _, res := range allRes {
			if res.Type != "team" {
				resources = append(resources, res)
			}
		}
	}

	// Resolve linked endeavour.
	var teamEndeavour *TeamEndeavour
	if edvRels, _, _ := s.rest.ListRelations(token, "resource", id, "endeavour", "", "belongs_to", 1, 0); len(edvRels) > 0 {
		if edv, err := s.rest.GetEndeavour(token, edvRels[0].TargetEntityID); err == nil {
			teamEndeavour = &TeamEndeavour{ID: edv.ID, Name: edv.Name}
		}
	}

	// Load endeavours for link dropdown.
	var endeavours []*Endeavour
	if canManage {
		endeavours, _, _ = s.rest.AdminListEndeavours(token, "", "", 200, 0)
	}

	// Resolve org from the linked endeavour for role lookup.
	var orgID string
	if teamEndeavour != nil {
		if orgRels, _, _ := s.rest.ListRelations(token, "organization", "", "endeavour", teamEndeavour.ID, "participates_in", 1, 0); len(orgRels) > 0 {
			orgID = orgRels[0].SourceEntityID
		}
	}
	roles := s.resolveOrgRoles(token, orgID)

	data.Title = team.Name
	data.Data = map[string]interface{}{
		"TeamData":        team,
		"TeamName":        team.Name,
		"Members":         members,
		"Quorum":          quorum,
		"Resources":       resources,
		"TeamEndeavour":   teamEndeavour,
		"Endeavours":      endeavours,
		"Roles":           roles,
		"CanManageTeam":   canManage,
	}
	s.render(w, r, "team_detail.html", data)
}

// --- Admin: Rituals ---

func (s *Server) handleAdminRituals(w http.ResponseWriter, r *http.Request) {
	token := getToken(r)
	user, _ := s.rest.Whoami(token)

	data := PageData{
		Title:       "Templates",
		User:        user,
		IsAdmin:     true,
		CurrentPage: "admin_rituals",
	}

	search := r.URL.Query().Get("search")
	status := r.URL.Query().Get("status")
	origin := r.URL.Query().Get("origin")
	category := r.URL.Query().Get("category")
	filterLang := r.URL.Query().Get("lang")
	limit := 50
	offset := queryIntParam(r, "offset", 0)

	// Collect all available language codes (uppercase for display).
	allLangs := make([]string, 0)
	for _, lang := range s.i18n.Languages() {
		allLangs = append(allLangs, lang.Code)
	}

	var rituals []*Ritual
	var total int

	// Only fetch rituals when category is not "reporting".
	if category != "reporting" {
		var err error
		rituals, total, err = s.rest.ListRitualsFiltered(token, search, status, origin, "", filterLang, limit, offset)
		if err != nil {
			data.Error = s.msg(r, user, "errors.failed_load_rituals", err.Error())
		}
	}

	// Resolve language for each ritual via its endeavour.
	ritualLangs := map[string][]string{}
	if len(rituals) > 0 {
		// Collect unique endeavour IDs.
		edvIDs := map[string]bool{}
		for _, rit := range rituals {
			if rit.EndeavourID != "" {
				edvIDs[rit.EndeavourID] = true
			}
		}
		// Fetch endeavours to get their language.
		edvLangMap := map[string]string{}
		for edvID := range edvIDs {
			if edv, err := s.rest.GetEndeavour(token, edvID); err == nil && edv.Lang != "" {
				edvLangMap[edvID] = edv.Lang
			}
		}
		// Assign language(s) per ritual.
		// Use the ritual's own lang field first, then fall back to
		// the endeavour's language, then default to "en".
		for _, rit := range rituals {
			lang := rit.Lang
			if lang == "" {
				lang = "en"
			}
			if rit.EndeavourID != "" {
				if l, ok := edvLangMap[rit.EndeavourID]; ok {
					lang = l
				}
			}
			ritualLangs[rit.ID] = []string{lang}
		}
	}

	// Fetch DB report templates.
	var reportTemplates []*ReportTemplate
	var reportTotal int
	if category != "rituals" {
		reportTemplates, reportTotal, _ = s.rest.ListTemplates(token, "", "", "active", search, 50, 0)
	}

	nextOffset := offset + limit
	data.Data = map[string]interface{}{
		"Rituals":         rituals,
		"RitualLangs":     ritualLangs,
		"Total":           total,
		"Search":          search,
		"Status":          status,
		"Origin":          origin,
		"Category":        category,
		"Offset":          offset,
		"Limit":           limit,
		"NextOffset":      nextOffset,
		"HasNext":         category != "reporting" && nextOffset < total,
		"HasPrev":         category != "reporting" && offset > 0,
		"AllLangs":        allLangs,
		"FilterLang":      filterLang,
		"ReportTemplates": reportTemplates,
		"ReportTotal":     reportTotal,
	}
	s.render(w, r, "admin_rituals.html", data)
}

// handleAdminRitualDetail serves the adminritualdetail portal page.
func (s *Server) handleAdminRitualDetail(w http.ResponseWriter, r *http.Request) {
	token := getToken(r)
	user, _ := s.rest.Whoami(token)
	id := r.PathValue("id")

	data := PageData{
		Title:       "Ritual",
		User:        user,
		IsAdmin:     true,
		CurrentPage: "admin_rituals",
	}

	editing := false

	if r.Method == http.MethodPost {
		if !validateCSRF(r) {
			s.csrfFailed(w, r, "admin_ritual_detail.html")
			return
		}
		action := r.FormValue("action")
		switch action {
		case "toggle_enabled":
			ritual, err := s.rest.GetRitual(token, id)
			if err != nil {
				data.Error = s.msg(r, user, "errors.failed_load_rituals", err.Error())
				break
			}
			newEnabled := !ritual.IsEnabled
			if _, err := s.rest.UpdateRitual(token, id, map[string]interface{}{"is_enabled": newEnabled}); err != nil {
				data.Error = s.msg(r, user, "errors.failed_update_ritual", err.Error())
			} else {
				data.Success = s.msg(r, user, "success.ritual_updated")
			}
		case "archive":
			if _, err := s.rest.UpdateRitual(token, id, map[string]interface{}{"status": "archived"}); err != nil {
				data.Error = s.msg(r, user, "errors.failed_update_ritual", err.Error())
			} else {
				data.Success = s.msg(r, user, "success.ritual_updated")
			}
		case "trigger_run":
			if _, err := s.rest.CreateRitualRun(token, id, "manual"); err != nil {
				data.Error = s.msg(r, user, "errors.failed_trigger_run", err.Error())
			} else {
				data.Success = s.msg(r, user, "success.run_triggered")
			}
		case "edit":
			editing = true
		case "save":
			fields := map[string]interface{}{}
			if v := r.FormValue("name"); v != "" {
				fields["name"] = v
			}
			if v := r.FormValue("description"); v != "" {
				fields["description"] = v
			}
			if v := r.FormValue("lang"); v != "" {
				fields["lang"] = v
			}
			if len(fields) > 0 {
				if _, err := s.rest.UpdateRitual(token, id, fields); err != nil {
					data.Error = s.msg(r, user, "errors.failed_update_ritual", err.Error())
					editing = true
				} else {
					data.Success = s.msg(r, user, "success.ritual_updated")
				}
			}
		}
	}

	// Check for ?edit=1 query param (for edit button link)
	if r.URL.Query().Get("edit") == "1" {
		editing = true
	}

	ritual, err := s.rest.GetRitual(token, id)
	if err != nil {
		data.Error = s.msg(r, user, "errors.failed_load_rituals", err.Error())
		s.render(w, r, "admin_ritual_detail.html", data)
		return
	}

	runs, _, _ := s.rest.ListRitualRuns(token, id, "", 20, 0)
	lineage, _ := s.rest.GetRitualLineage(token, id)

	data.Title = ritual.Name
	data.Data = map[string]interface{}{
		"RitualData": ritual,
		"RitualName": ritual.Name,
		"Runs":       runs,
		"Lineage":    lineage,
		"Editing":    editing,
	}
	s.render(w, r, "admin_ritual_detail.html", data)
}

// --- Admin: Report Templates ---

func (s *Server) handleAdminTemplateDetail(w http.ResponseWriter, r *http.Request) {
	token := getToken(r)
	user, _ := s.rest.Whoami(token)
	id := r.PathValue("id")

	data := PageData{
		Title:       "Template",
		User:        user,
		IsAdmin:     true,
		CurrentPage: "admin_rituals",
	}

	editing := false

	if r.Method == http.MethodPost {
		if !validateCSRF(r) {
			s.csrfFailed(w, r, "admin_template_detail.html")
			return
		}
		action := r.FormValue("action")
		switch action {
		case "edit":
			editing = true
		case "save":
			fields := map[string]interface{}{}
			if v := r.FormValue("name"); v != "" {
				fields["name"] = v
			}
			if v := r.FormValue("body"); v != "" {
				fields["body"] = v
			}
			if len(fields) > 0 {
				if _, err := s.rest.UpdateTemplate(token, id, fields); err != nil {
					data.Error = s.msg(r, user, "errors.failed_update_template", err.Error())
					editing = true
				} else {
					data.Success = s.msg(r, user, "success.template_updated")
				}
			}
		case "archive":
			if _, err := s.rest.UpdateTemplate(token, id, map[string]interface{}{"status": "archived"}); err != nil {
				data.Error = s.msg(r, user, "errors.failed_update_template", err.Error())
			} else {
				data.Success = s.msg(r, user, "success.template_updated")
			}
		case "create_translation":
			lang := r.FormValue("lang")
			if lang == "" {
				data.Error = s.msg(r, user, "errors.failed_update_template", "language is required")
			} else {
				forked, err := s.rest.ForkTemplate(token, id, map[string]interface{}{"lang": lang})
				if err != nil {
					data.Error = s.msg(r, user, "errors.failed_update_template", err.Error())
				} else {
					http.Redirect(w, r, "/admin/templates/"+forked.ID+"?edit=1", http.StatusSeeOther)
					return
				}
			}
		}
	}

	if r.URL.Query().Get("edit") == "1" {
		editing = true
	}

	tpl, err := s.rest.GetTemplate(token, id)
	if err != nil {
		data.Error = s.msg(r, user, "errors.failed_load_template", err.Error())
		s.render(w, r, "admin_template_detail.html", data)
		return
	}

	data.Title = tpl.Name
	data.Data = map[string]interface{}{
		"Template": tpl,
		"Editing":  editing,
	}
	s.render(w, r, "admin_template_detail.html", data)
}

// --- Unified Activity (scoped, not admin-only) ---

func (s *Server) handleUnifiedActivity(w http.ResponseWriter, r *http.Request) {
	token := getToken(r)
	user, _ := s.rest.Whoami(token)

	data := PageData{
		Title:       "Activity",
		User:        user,
		CurrentPage: "entity_changes",
	}

	action := r.URL.Query().Get("action")
	entityType := r.URL.Query().Get("entity_type")
	actorID := r.URL.Query().Get("actor_id")
	startTime := r.URL.Query().Get("start_time")
	endTime := r.URL.Query().Get("end_time")

	// Convert datetime-local values to RFC3339 for the API
	startTimeAPI := datetimeLocalToRFC3339(startTime)
	endTimeAPI := datetimeLocalToRFC3339(endTime)

	// Page size: 50 (default), 100, 200
	limit := queryIntParam(r, "limit", 50)
	if limit != 50 && limit != 100 && limit != 200 {
		limit = 50
	}
	offset := queryIntParam(r, "offset", 0)

	// Export mode
	export := r.URL.Query().Get("export")
	if export == "csv" || export == "json" {
		s.handleActivityExport(w, r, token, user, action, entityType, actorID, startTimeAPI, endTimeAPI, export)
		return
	}

	resp, err := s.rest.AdminListActivity(token, action, entityType, actorID, startTimeAPI, endTimeAPI, limit, offset)
	if err != nil {
		data.Error = s.msg(r, user, "portal.activity_feed.error", err.Error())
		data.Data = map[string]interface{}{
			"Entries":     []*UnifiedActivityEntry{},
			"Summary":     ActivitySummary{},
			"Total":       0,
			"Action":      action,
			"EntityType":  entityType,
			"ActorID":     actorID,
			"StartTime":   startTime,
			"EndTime":     endTime,
			"Offset":      offset,
			"Limit":       limit,
			"ActorMap":    map[string]string{},
			"EntityMap":   map[string]string{},
			"EndeavourMap": map[string]string{},
		}
		s.render(w, r, "entity_changes.html", data)
		return
	}

	// Build name resolution maps from entity change entries
	actorMap := s.buildUnifiedActorMap(token, resp.Data, user)
	entityMap, endeavourMap := s.buildUnifiedNameMaps(token, resp.Data)

	nextOffset := offset + limit
	total := resp.Meta.Total
	data.Data = map[string]interface{}{
		"Entries":      resp.Data,
		"Summary":      resp.Summary,
		"Total":        total,
		"Action":       action,
		"EntityType":   entityType,
		"ActorID":      actorID,
		"StartTime":    startTime,
		"EndTime":      endTime,
		"Offset":       offset,
		"Limit":        limit,
		"NextOffset":   nextOffset,
		"HasNext":      nextOffset < total,
		"HasPrev":      offset > 0,
		"ActorMap":     actorMap,
		"EntityMap":    entityMap,
		"EndeavourMap": endeavourMap,
	}
	s.render(w, r, "entity_changes.html", data)
}

// handleActivityExport serves the activityexport portal page.
func (s *Server) handleActivityExport(w http.ResponseWriter, r *http.Request, token string, user map[string]interface{}, action, entityType, actorID, startTime, endTime, format string) {
	// Fetch all entries (up to 10000) for export
	resp, err := s.rest.AdminListActivity(token, action, entityType, actorID, startTime, endTime, 10000, 0)
	if err != nil {
		http.Error(w, "Failed to load activity data", http.StatusInternalServerError)
		return
	}

	// Resolve names for export
	actorMap := s.buildUnifiedActorMap(token, resp.Data, user)
	entityMap, endeavourMap := s.buildUnifiedNameMaps(token, resp.Data)

	if format == "json" {
		type exportEntry struct {
			Time       string   `json:"time"`
			Type       string   `json:"type"`
			Action     string   `json:"action"`
			Actor      string   `json:"actor"`
			Summary    string   `json:"summary"`
			EntityType string   `json:"entity_type,omitempty"`
			Entity     string   `json:"entity,omitempty"`
			Endeavour  string   `json:"endeavour,omitempty"`
			Fields     []string `json:"fields,omitempty"`
		}
		result := make([]exportEntry, len(resp.Data))
		for i, e := range resp.Data {
			actor := actorMap[e.ActorID]
			if actor == "" {
				actor = e.ActorID
			}
			entity := entityMap[e.EntityID]
			if entity == "" {
				entity = e.EntityID
			}
			edv := endeavourMap[e.EndeavourID]
			if edv == "" {
				edv = e.EndeavourID
			}
			result[i] = exportEntry{
				Time:       e.CreatedAt,
				Type:       e.Type,
				Action:     e.Action,
				Actor:      actor,
				Summary:    e.Summary,
				EntityType: e.EntityType,
				Entity:     entity,
				Endeavour:  edv,
				Fields:     e.Fields,
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", "attachment; filename=activity.json")
		_ = json.NewEncoder(w).Encode(result)
		return
	}

	// CSV export
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=activity.csv")
	_, _ = w.Write([]byte("Time,Type,Action,Actor,Summary,Entity Type,Entity,Endeavour,Fields\n"))
	for _, e := range resp.Data {
		actor := actorMap[e.ActorID]
		if actor == "" {
			actor = e.ActorID
		}
		entity := entityMap[e.EntityID]
		if entity == "" {
			entity = e.EntityID
		}
		edv := endeavourMap[e.EndeavourID]
		if edv == "" {
			edv = e.EndeavourID
		}
		fields := strings.Join(e.Fields, "; ")
		line := fmt.Sprintf("%s,%s,%s,%s,%s,%s,%s,%s,%s\n",
			csvEscape(e.CreatedAt), csvEscape(e.Type), csvEscape(e.Action),
			csvEscape(actor), csvEscape(e.Summary),
			csvEscape(e.EntityType), csvEscape(entity),
			csvEscape(edv), csvEscape(fields))
		_, _ = w.Write([]byte(line))
	}
}

// datetimeLocalToRFC3339 converts an HTML datetime-local value (e.g. "2026-03-05T07:00")
// to RFC3339 format for the API. Returns the original string if already in RFC3339 or empty.
func datetimeLocalToRFC3339(s string) string {
	if s == "" {
		return ""
	}
	// Already RFC3339
	if strings.HasSuffix(s, "Z") || strings.Contains(s, "+") {
		return s
	}
	// datetime-local: "2026-03-05T07:00" -> "2026-03-05T07:00:00Z"
	if len(s) == 16 {
		return s + ":00Z"
	}
	// "2026-03-05T07:00:00" -> "2026-03-05T07:00:00Z"
	if len(s) == 19 {
		return s + "Z"
	}
	return s
}

// csvEscape wraps a value in quotes and escapes internal quotes.
func csvEscape(s string) string {
	if s == "" {
		return ""
	}
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

func (s *Server) buildUnifiedActorMap(token string, entries []*UnifiedActivityEntry, user map[string]interface{}) map[string]string {
	actorMap := map[string]string{}

	seen := map[string]bool{}
	for _, e := range entries {
		if e.ActorID != "" && e.ActorID != "anonymous" && e.ActorID != "system" {
			seen[e.ActorID] = true
		}
	}
	if len(seen) == 0 {
		return actorMap
	}

	if user != nil {
		if id, _ := user["user_id"].(string); id != "" {
			if name, _ := user["name"].(string); name != "" {
				actorMap[id] = name
			}
		}
	}

	isAdmin, _ := user["is_admin"].(bool)
	if isAdmin {
		users, _, _ := s.rest.ListUsers(token, "", "", 200, 0)
		for _, u := range users {
			actorMap[u.ID] = u.Name
		}
		resources, _, _ := s.rest.AdminListResources(token, "", "", "", "", 200, 0)
		for _, r := range resources {
			actorMap[r.ID] = r.Name
		}
	} else {
		agents, _, _ := s.rest.ListMyAgents(token, 200, 0)
		for _, a := range agents {
			actorMap[a.ID] = a.Name
		}
		if orgs, ok := user["organizations"].(map[string]interface{}); ok {
			for orgID := range orgs {
				orgUsers, _, _ := s.rest.ListUsers(token, orgID, "", 200, 0)
				for _, u := range orgUsers {
					actorMap[u.ID] = u.Name
				}
			}
		}
	}

	return actorMap
}

func (s *Server) buildUnifiedNameMaps(token string, entries []*UnifiedActivityEntry) (entityMap, endeavourMap map[string]string) {
	entityMap = map[string]string{}
	endeavourMap = map[string]string{}

	edvIDs := map[string]bool{}
	entityIDs := map[string]string{}
	for _, e := range entries {
		if e.EndeavourID != "" {
			edvIDs[e.EndeavourID] = true
		}
		if e.EntityID != "" && e.EntityType != "" {
			entityIDs[e.EntityID] = e.EntityType
		}
	}

	if len(edvIDs) > 0 {
		edvs, _, _ := s.rest.AdminListEndeavours(token, "", "", 200, 0)
		for _, edv := range edvs {
			endeavourMap[edv.ID] = edv.Name
		}
	}

	needTasks := false
	needDemands := false
	needOrgs := false
	needResources := false
	for _, typ := range entityIDs {
		switch typ {
		case "task":
			needTasks = true
		case "demand":
			needDemands = true
		case "organization":
			needOrgs = true
		case "resource":
			needResources = true
		}
	}

	for id, typ := range entityIDs {
		if typ == "endeavour" {
			if name, ok := endeavourMap[id]; ok {
				entityMap[id] = name
			}
		}
	}

	if needTasks {
		tasks, _, _ := s.rest.ListTasks(token, "", "", "", "", "", 200, 0)
		for _, t := range tasks {
			entityMap[t.ID] = t.Title
		}
	}
	if needDemands {
		demands, _, _ := s.rest.ListDemands(token, "", "", "", "", "", 200, 0)
		for _, d := range demands {
			entityMap[d.ID] = d.Title
		}
	}
	if needOrgs {
		orgs, _, _ := s.rest.ListOrganizations(token, "", "", 200, 0)
		for _, o := range orgs {
			entityMap[o.ID] = o.Name
		}
	}
	if needResources {
		resources, _, _ := s.rest.AdminListResources(token, "", "", "", "", 200, 0)
		for _, r := range resources {
			entityMap[r.ID] = r.Name
		}
	}

	return entityMap, endeavourMap
}

// --- Admin: Audit Log ---

func (s *Server) handleAdminAudit(w http.ResponseWriter, r *http.Request) {
	token := getToken(r)
	user, _ := s.rest.Whoami(token)

	data := PageData{
		Title:       "Audit Log",
		User:        user,
		IsAdmin:     true,
		CurrentPage: "admin_audit",
	}

	action := r.URL.Query().Get("action")
	actorID := r.URL.Query().Get("actor_id")
	source := r.URL.Query().Get("source")
	showAll := r.URL.Query().Get("show_all") == "true"
	limit := 50
	offset := queryIntParam(r, "offset", 0)

	// Default: exclude routine "request" entries unless explicitly viewing them.
	excludeAction := ""
	if action == "" && !showAll {
		excludeAction = "request"
	}

	entries, total, err := s.rest.ListAuditLog(token, action, actorID, excludeAction, source, limit, offset)
	if err != nil {
		data.Error = s.msg(r, user, "errors.failed_load_audit", err.Error())
		data.Data = map[string]interface{}{
			"Entries":  []*AuditEntry{},
			"Total":    0,
			"Action":   action,
			"ActorID":  actorID,
			"Source":   source,
			"ShowAll":  showAll,
			"Offset":   offset,
			"Limit":    limit,
			"ActorMap": map[string]string{},
		}
		s.render(w, r, "admin_audit.html", data)
		return
	}

	for _, e := range entries {
		e.Summary = activitySummary(e)
	}

	actorMap := s.buildActorMap(token, entries)

	nextOffset := offset + limit
	data.Data = map[string]interface{}{
		"Entries":    entries,
		"Total":      total,
		"Action":     action,
		"ActorID":    actorID,
		"Source":     source,
		"ShowAll":    showAll,
		"Offset":     offset,
		"Limit":      limit,
		"NextOffset": nextOffset,
		"HasNext":    nextOffset < total,
		"HasPrev":    offset > 0,
		"ActorMap":   actorMap,
	}
	s.render(w, r, "admin_audit.html", data)
}

// --- Admin: Messages ---

func (s *Server) handleAdminMessages(w http.ResponseWriter, r *http.Request) {
	token := getToken(r)
	user, _ := s.rest.Whoami(token)

	data := PageData{
		Title:       "Messages",
		User:        user,
		IsAdmin:     true,
		CurrentPage: "admin_messages",
	}

	status := r.URL.Query().Get("status")
	intent := r.URL.Query().Get("intent")
	offset := queryIntParam(r, "offset", 0)
	limit := 50

	msgs, total, err := s.rest.ListMessages(token, status, intent, false, limit, offset)
	if err != nil {
		data.Error = s.msg(r, user, "errors.failed_load_messages", err.Error())
	}

	data.Data = map[string]interface{}{
		"Messages": msgs,
		"Total":    total,
		"Status":   status,
		"Intent":   intent,
		"Limit":    limit,
		"Offset":   offset,
	}

	s.render(w, r, "admin_messages.html", data)
}

// --- Admin: Settings ---

func (s *Server) handleAdminSettings(w http.ResponseWriter, r *http.Request) {
	token := getToken(r)
	user, _ := s.rest.Whoami(token)

	data := PageData{
		Title:       "Settings",
		User:        user,
		IsAdmin:     true,
		CurrentPage: "admin_settings",
	}

	if r.Method == http.MethodPost {
		if !validateCSRF(r) {
			s.csrfFailed(w, r, "admin_settings.html")
			return
		}
		action := r.FormValue("action")

		switch action {
		case "toggle_mcp":
			settings, _ := s.rest.AdminSettings(token)
			if settings != nil {
				newVal := !settings.MCPAccessEnabled
				if _, err := s.rest.UpdateAdminSettings(token, &newVal); err != nil {
					data.Error = s.msg(r, user, "errors.failed_update_mcp", err.Error())
				} else if newVal {
					data.Success = s.msg(r, user, "success.mcp_enabled")
				} else {
					data.Success = s.msg(r, user, "success.mcp_disabled")
				}
			}

		case "update_quotas":
			quotas := map[string]string{}
			// Policy fields (non-tier).
			policyFields := []string{
				"instance.max_active_users",
				"inactivity.warn_days",
				"inactivity.deactivate_days",
				"inactivity.sweep_capacity_threshold",
				"waitlist.notification_window_days",
			}
			for _, key := range policyFields {
				if val := r.FormValue(key); val != "" {
					quotas[key] = val
				}
			}
			// Tier fields (dynamic: accept any tier.N.field from the form).
			tierFields := []string{
				"max_orgs", "max_endeavours_per_org", "max_agents_per_org",
				"max_creations_per_hour", "max_active_endeavours", "max_teams_per_org",
			}
			if err := r.ParseForm(); err == nil {
				for key, vals := range r.PostForm {
					if len(vals) == 0 || vals[0] == "" {
						continue
					}
					if len(key) > 5 && key[:5] == "tier." {
						// Validate it matches tier.N.field pattern.
						for _, f := range tierFields {
							if len(key) > len(f)+6 && key[len(key)-len(f):] == f {
								quotas[key] = vals[0]
								break
							}
						}
					}
				}
			}
			if len(quotas) > 0 {
				if _, err := s.rest.UpdateAdminQuotas(token, quotas); err != nil {
					data.Error = s.msg(r, user, "errors.failed_update_quotas", err.Error())
				} else {
					data.Success = s.msg(r, user, "success.quotas_updated")
				}
			}
		}
	}

	settings, _ := s.rest.AdminSettings(token)
	mcpEnabled := false
	if settings != nil {
		mcpEnabled = settings.MCPAccessEnabled
	}
	if data.Data == nil {
		data.Data = map[string]interface{}{}
	}
	dataMap := data.Data.(map[string]interface{})
	dataMap["MCPEnabled"] = mcpEnabled
	dataMap["Version"] = s.version

	quotas, _ := s.rest.AdminQuotas(token)
	if quotas == nil {
		quotas = map[string]string{}
	}
	dataMap["Quotas"] = quotas

	tiers, _ := s.rest.AdminTiers(token)
	dataMap["Tiers"] = tiers

	data.Data = dataMap

	s.render(w, r, "admin_settings.html", data)
}

// --- Admin: Content Guard Test ---

func (s *Server) handleAdminContentGuard(w http.ResponseWriter, r *http.Request) {
	token := getToken(r)
	user, _ := s.rest.Whoami(token)

	data := PageData{
		Title:       "Content Guard",
		User:        user,
		IsAdmin:     true,
		CurrentPage: "admin_content_guard",
	}

	var testResp *ContentGuardTestResponse
	var payloadText string

	if r.Method == http.MethodPost {
		if !validateCSRF(r) {
			s.csrfFailed(w, r, "admin_content_guard.html")
			return
		}
		action := r.FormValue("action")
		switch action {
		case "update_patterns":
			s.handleAdminContentGuardPatternsUpdate(w, r, token, user, &data)
		case "dismiss":
			entityType := r.FormValue("entity_type")
			entityID := r.FormValue("entity_id")
			if entityType != "" && entityID != "" {
				if err := s.rest.AdminContentGuardDismiss(token, entityType, entityID); err != nil {
					data.Error = s.msg(r, user, "admin.content_guard.errors.dismiss_failed", err.Error())
				}
			}
			if data.Error == "" {
				http.Redirect(w, r, "/admin/content-guard", http.StatusSeeOther)
				return
			}
		case "suspend_creator":
			userID := r.FormValue("user_id")
			if userID != "" {
				if _, err := s.rest.UpdateUser(token, userID, map[string]interface{}{"status": "suspended"}); err != nil {
					data.Error = s.msg(r, user, "admin.users.errors.failed_update", err.Error())
				}
			}
			if data.Error == "" {
				http.Redirect(w, r, "/admin/content-guard", http.StatusSeeOther)
				return
			}
		default:
			payloadText = r.FormValue("payload")
			if payloadText != "" {
				res, err := s.rest.AdminContentGuardTest(token, []string{payloadText})
				if err != nil {
					data.Error = s.msg(r, user, "admin.content_guard.errors.test_failed", err.Error())
				} else {
					testResp = res
				}
			}
		}
	}

	cgStats, _ := s.rest.AdminContentGuardStats(token)
	patterns, _ := s.rest.AdminContentGuardPatterns(token)
	alerts, alertTotal, _ := s.rest.AdminContentGuardAlerts(token, 50, 0, false)

	var results []ContentGuardTestResult
	var threshold int
	if testResp != nil {
		results = testResp.Results
		threshold = testResp.Threshold
	}

	data.Data = map[string]interface{}{
		"Results":    results,
		"Payload":    payloadText,
		"CGStats":    cgStats,
		"Patterns":   patterns,
		"Threshold":  threshold,
		"Alerts":     alerts,
		"AlertTotal": alertTotal,
	}
	s.render(w, r, "admin_content_guard.html", data)
}

// handleAdminContentGuardPatternsUpdate processes the pattern update form.
func (s *Server) handleAdminContentGuardPatternsUpdate(w http.ResponseWriter, r *http.Request, token string, user map[string]interface{}, data *PageData) {
	// Parse form: each pattern has fields named pattern_enabled_<name>, pattern_weight_<name>
	if err := r.ParseForm(); err != nil {
		data.Error = "Failed to parse form"
		return
	}

	// Load current patterns to know which are builtin
	patterns, err := s.rest.AdminContentGuardPatterns(token)
	if err != nil {
		data.Error = s.msg(r, user, "admin.content_guard.errors.save_failed", err.Error())
		return
	}

	disabled := map[string]bool{}
	weightOverrides := map[string]int{}

	for _, p := range patterns {
		if !p.Builtin {
			continue
		}
		enabledKey := fmt.Sprintf("pattern_enabled_%s", p.Name)
		weightKey := fmt.Sprintf("pattern_weight_%s", p.Name)

		if r.FormValue(enabledKey) != "on" {
			disabled[p.Name] = true
		}
		if wStr := r.FormValue(weightKey); wStr != "" {
			if wt, err := strconv.Atoi(wStr); err == nil && wt >= 1 && wt <= 25 {
				weightOverrides[p.Name] = wt
			}
		}
	}

	// Process existing custom patterns: read edits, skip deletions
	var added []map[string]interface{}
	for i, p := range patterns {
		if p.Builtin {
			continue
		}
		// Check if marked for deletion
		deleteKey := fmt.Sprintf("custom_delete_%d", i)
		if r.FormValue(deleteKey) == "on" {
			continue
		}
		// Read edited values (fall back to originals)
		nameKey := fmt.Sprintf("custom_edit_name_%d", i)
		catKey := fmt.Sprintf("custom_edit_category_%d", i)
		patKey := fmt.Sprintf("custom_edit_pattern_%d", i)
		wtKey := fmt.Sprintf("custom_edit_weight_%d", i)

		name := strings.TrimSpace(r.FormValue(nameKey))
		if name == "" {
			name = p.Name
		}
		category := strings.TrimSpace(r.FormValue(catKey))
		if category == "" {
			category = p.Category
		}
		pattern := strings.TrimSpace(r.FormValue(patKey))
		if pattern == "" {
			pattern = p.Pattern
		}
		wt := p.Weight
		if wtStr := r.FormValue(wtKey); wtStr != "" {
			if v, err := strconv.Atoi(wtStr); err == nil && v >= 1 && v <= 25 {
				wt = v
			}
		}
		added = append(added, map[string]interface{}{
			"name":     name,
			"category": category,
			"pattern":  pattern,
			"weight":   wt,
		})
	}

	// Parse new custom pattern from the "add" fields
	customName := strings.TrimSpace(r.FormValue("custom_name"))
	customCategory := strings.TrimSpace(r.FormValue("custom_category"))
	customPattern := strings.TrimSpace(r.FormValue("custom_pattern"))
	customWeightStr := strings.TrimSpace(r.FormValue("custom_weight"))

	if customName != "" && customPattern != "" {
		wt := 5
		if customWeightStr != "" {
			if v, err := strconv.Atoi(customWeightStr); err == nil {
				wt = v
			}
		}
		if customCategory == "" {
			customCategory = "custom"
		}
		added = append(added, map[string]interface{}{
			"name":     customName,
			"category": customCategory,
			"pattern":  customPattern,
			"weight":   wt,
		})
	}

	overrides := map[string]interface{}{
		"disabled":         disabled,
		"weight_overrides": weightOverrides,
	}
	if len(added) > 0 {
		overrides["added"] = added
	}

	if err := s.rest.AdminContentGuardPatternsUpdate(token, overrides); err != nil {
		data.Error = s.msg(r, user, "admin.content_guard.errors.save_failed", err.Error())
		return
	}
	data.Success = s.msg(r, user, "admin.content_guard.success.patterns_updated")
}

// --- Admin: Translations ---

func (s *Server) handleAdminTranslations(w http.ResponseWriter, r *http.Request) {
	token := getToken(r)
	user, _ := s.rest.Whoami(token)

	s.render(w, r, "admin_translations.html", PageData{
		Title:       "Translations",
		User:        user,
		IsAdmin:     true,
		Data: map[string]interface{}{
			"BaseKeyCount": s.i18n.BaseKeyCount(),
			"Languages":    s.i18n.Stats(),
		},
		CurrentPage: "admin_translations",
	})
}

// --- Admin: Taskschmied ---

func (s *Server) handleAdminTaskschmied(w http.ResponseWriter, r *http.Request) {
	token := getToken(r)
	user, _ := s.rest.Whoami(token)

	data := PageData{
		Title:       "Taskschmied",
		User:        user,
		IsAdmin:     true,
		CurrentPage: "admin_taskschmied",
	}

	// Handle per-tier toggle POST.
	if r.Method == http.MethodPost {
		if !validateCSRF(r) {
			s.csrfFailed(w, r, "admin_taskschmied.html")
			return
		}
		action := r.FormValue("action")
		if action == "toggle" {
			target := r.FormValue("target")
			disabled := r.FormValue("disabled") == "true"
			if err := s.rest.AdminTaskschmiedToggle(token, target, disabled); err != nil {
				data.Error = fmt.Sprintf("Failed to toggle %s: %v", target, err)
			} else {
				state := "enabled"
				if disabled {
					state = "disabled"
				}
				label := target
				if len(label) > 0 {
					label = strings.ToUpper(label[:1]) + label[1:]
				}
				data.Success = fmt.Sprintf("%s LLM %s.", label, state)
			}
		}
	}

	// Fetch circuit breaker status from API.
	status, err := s.rest.AdminTaskschmiedStatus(token)
	if err != nil {
		data.Error = fmt.Sprintf("Failed to load Taskschmied status: %v", err)
		s.render(w, r, "admin_taskschmied.html", data)
		return
	}

	// Fetch run stats via SQL (through the REST API ritual-runs endpoint).
	limit := 25
	offset := queryIntParam(r, "offset", 0)
	statusFilter := r.URL.Query().Get("status")
	runs, total, _ := s.rest.ListRitualRuns(token, "", statusFilter, limit, offset)

	// Resolve ritual and endeavour names for display.
	type runDisplay struct {
		Run            *RitualRun
		RitualName     string
		EndeavourName  string
		Client         string
		DurationMs     int64
	}
	displays := make([]runDisplay, 0, len(runs))
	ritualCache := map[string]*Ritual{}
	edvCache := map[string]*Endeavour{}
	for _, run := range runs {
		rd := runDisplay{Run: run}
		// Resolve ritual name.
		if _, ok := ritualCache[run.RitualID]; !ok {
			if rtl, err := s.rest.GetRitual(token, run.RitualID); err == nil {
				ritualCache[run.RitualID] = rtl
			}
		}
		if rtl, ok := ritualCache[run.RitualID]; ok {
			rd.RitualName = rtl.Name
			// Resolve endeavour name.
			if _, ok := edvCache[rtl.EndeavourID]; !ok {
				if edv, err := s.rest.GetEndeavour(token, rtl.EndeavourID); err == nil {
					edvCache[rtl.EndeavourID] = edv
				}
			}
			if edv, ok := edvCache[rtl.EndeavourID]; ok {
				rd.EndeavourName = edv.Name
			}
		}
		// Extract client and duration from metadata.
		if run.Metadata != nil {
			if c, ok := run.Metadata["client"].(string); ok {
				rd.Client = c
			}
			if d, ok := run.Metadata["duration_ms"].(float64); ok {
				rd.DurationMs = int64(d)
			}
		}
		displays = append(displays, rd)
	}

	// Compute summary stats from all runs (use a separate large-limit call).
	allRuns, allTotal, _ := s.rest.ListRitualRuns(token, "", "", 10000, 0)
	var succeeded, failed, skipped int
	var totalDuration int64
	var durationCount int
	for _, run := range allRuns {
		switch run.Status {
		case "succeeded":
			succeeded++
		case "failed":
			failed++
		case "skipped":
			skipped++
		}
		if run.Metadata != nil {
			if d, ok := run.Metadata["duration_ms"].(float64); ok && d > 0 {
				totalDuration += int64(d)
				durationCount++
			}
		}
	}
	var avgDuration int64
	if durationCount > 0 {
		avgDuration = totalDuration / int64(durationCount)
	}

	data.Data = map[string]interface{}{
		"Status":        status,
		"Enabled":       status["enabled"],
		"CircuitBreaker": status["circuit_breaker"],
		"Runs":          displays,
		"RunTotal":      total,
		"AllTotal":      allTotal,
		"Succeeded":     succeeded,
		"Failed":        failed,
		"Skipped":       skipped,
		"AvgDurationMs": avgDuration,
		"Limit":         limit,
		"Offset":        offset,
		"StatusFilter":  statusFilter,
	}
	s.render(w, r, "admin_taskschmied.html", data)
}
