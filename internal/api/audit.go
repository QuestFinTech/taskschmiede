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
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/QuestFinTech/taskschmiede/internal/auth"
	"github.com/QuestFinTech/taskschmiede/internal/storage"
)

func (a *API) handleAuditList(w http.ResponseWriter, r *http.Request) {
	if !a.requireAdmin(w, r) {
		return
	}

	opts := storage.ListAuditLogOpts{
		Action:        queryString(r, "action"),
		ExcludeAction: queryString(r, "exclude_action"),
		ActorID:       queryString(r, "actor_id"),
		Resource:      queryString(r, "resource"),
		IP:            queryString(r, "ip"),
		Source:        queryString(r, "source"),
		Limit:         queryInt(r, "limit", 50),
		Offset:        queryInt(r, "offset", 0),
		BeforeID:      queryString(r, "before_id"),
	}

	if s := queryString(r, "start_time"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			opts.StartTime = &t
		}
	}
	if s := queryString(r, "end_time"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			opts.EndTime = &t
		}
	}

	entries, total, err := a.db.ListAuditLog(opts)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to query audit log")
		return
	}

	writeList(w, entries, total, opts.Limit, opts.Offset)
}

// handleAuditMyActivity handles GET /api/v1/audit/my-activity.
func (a *API) handleAuditMyActivity(w http.ResponseWriter, r *http.Request) {
	opts := storage.ListAuditLogOpts{
		Action: queryString(r, "action"),
		Limit:  queryInt(r, "limit", 50),
		Offset: queryInt(r, "offset", 0),
	}

	if s := queryString(r, "start_time"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			opts.StartTime = &t
		}
	}
	if s := queryString(r, "end_time"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			opts.EndTime = &t
		}
	}

	entries, total, apiErr := a.ListMyAuditLog(r.Context(), opts)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	writeList(w, entries, total, opts.Limit, opts.Offset)
}

// handleMyFullActivity handles GET /api/v1/audit/my-full-activity.
// Returns merged login events + entity changes for the current user.
// No endeavour admin check -- users can always see their own changes.
func (a *API) handleMyFullActivity(w http.ResponseWriter, r *http.Request) {
	authUser := auth.GetAuthUser(r.Context())
	if authUser == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	limit := queryInt(r, "limit", 50)
	offset := queryInt(r, "offset", 0)

	var startTime, endTime *time.Time
	if s := queryString(r, "start_time"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			startTime = &t
		}
	}
	if s := queryString(r, "end_time"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			endTime = &t
		}
	}

	// Fetch login events from audit log (actor_id = user ID)
	auditOpts := storage.ListAuditLogOpts{
		ActorID:   authUser.UserID,
		Action:    "login_success",
		StartTime: startTime,
		EndTime:   endTime,
		Limit:     offset + limit + 200,
		Offset:    0,
	}
	auditEntries, _, err := a.db.ListAuditLog(auditOpts)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to query audit log")
		return
	}

	// Fetch entity changes for this user.
	// actor_id in entity_change may be user ID or resource ID depending on source.
	// Query by user ID first; also query by resource_id if set.
	user, err := a.db.GetUser(authUser.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to look up user")
		return
	}

	ecOpts := storage.ListEntityChangesOpts{
		ActorID:   authUser.UserID,
		StartTime: startTime,
		EndTime:   endTime,
		Limit:     offset + limit + 200,
		Offset:    0,
	}
	ecEntries, _, err := a.db.ListEntityChanges(ecOpts)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to query entity changes")
		return
	}

	// Also fetch by resource_id if different from user ID
	if user.ResourceID != nil && *user.ResourceID != "" && *user.ResourceID != authUser.UserID {
		ecOpts2 := storage.ListEntityChangesOpts{
			ActorID:   *user.ResourceID,
			StartTime: startTime,
			EndTime:   endTime,
			Limit:     offset + limit + 200,
			Offset:    0,
		}
		ecEntries2, _, err2 := a.db.ListEntityChanges(ecOpts2)
		if err2 == nil {
			ecEntries = append(ecEntries, ecEntries2...)
		}
	}

	// Merge into unified entries sorted by time desc
	type entry struct {
		ID          string                 `json:"id"`
		Type        string                 `json:"type"`
		Action      string                 `json:"action"`
		ActorID     string                 `json:"actor_id"`
		Summary     string                 `json:"summary"`
		EntityType  string                 `json:"entity_type,omitempty"`
		EntityID    string                 `json:"entity_id,omitempty"`
		EndeavourID string                 `json:"endeavour_id,omitempty"`
		Fields      []string               `json:"fields,omitempty"`
		Metadata    map[string]interface{} `json:"metadata,omitempty"`
		Source      string                 `json:"source,omitempty"`
		CreatedAt   string                 `json:"created_at"`
	}
	merged := make([]entry, 0, len(auditEntries)+len(ecEntries))

	for _, e := range auditEntries {
		merged = append(merged, entry{
			ID:        e.ID,
			Type:      "login",
			Action:    e.Action,
			ActorID:   e.ActorID,
			Summary:   auditSummary(e),
			Source:    e.Source,
			CreatedAt: e.CreatedAt.Format(time.RFC3339),
		})
	}
	for _, e := range ecEntries {
		merged = append(merged, entry{
			ID:          e.ID,
			Type:        "entity_change",
			Action:      e.Action,
			ActorID:     e.ActorID,
			EntityType:  e.EntityType,
			EntityID:    e.EntityID,
			EndeavourID: e.EndeavourID,
			Fields:      e.Fields,
			Metadata:    e.Metadata,
			Summary:     e.Action + " " + e.EntityType,
			CreatedAt:   e.CreatedAt.Format(time.RFC3339),
		})
	}

	sort.Slice(merged, func(i, j int) bool {
		return merged[i].CreatedAt > merged[j].CreatedAt
	})

	total := len(merged)
	end := offset + limit
	if end > total {
		end = total
	}
	var page []entry
	if offset < total {
		page = merged[offset:end]
	}
	if page == nil {
		page = []entry{}
	}

	writeList(w, page, total, limit, offset)
}

// ListAuditLog queries the audit log with admin access control.
func (a *API) ListAuditLog(ctx context.Context, opts storage.ListAuditLogOpts) ([]*storage.AuditLogRecord, int, *APIError) {
	if apiErr := a.CheckAdmin(ctx); apiErr != nil {
		return nil, 0, apiErr
	}

	entries, total, err := a.db.ListAuditLog(opts)
	if err != nil {
		return nil, 0, errInternal("Failed to query audit log")
	}

	return entries, total, nil
}

// AuditActivityEntry is a simplified audit entry for non-admin users
// viewing their own activity history.
type AuditActivityEntry struct {
	Action    string `json:"action"`
	Resource  string `json:"resource,omitempty"`
	Source    string `json:"source,omitempty"`
	Summary   string `json:"summary"`
	CreatedAt string `json:"created_at"`
}

// ListMyAuditLog returns the caller's own audit entries in simplified form.
// No admin check -- any authenticated user can view their own activity.
func (a *API) ListMyAuditLog(ctx context.Context, opts storage.ListAuditLogOpts) ([]AuditActivityEntry, int, *APIError) {
	authUser := auth.GetAuthUser(ctx)
	if authUser == nil {
		return nil, 0, errUnauthorized("Authentication required")
	}

	// Force actor_id to the caller's user ID -- cannot browse others' activity
	opts.ActorID = authUser.UserID
	// Non-admins cannot filter by IP
	opts.IP = ""

	entries, total, err := a.db.ListAuditLog(opts)
	if err != nil {
		return nil, 0, errInternal("Failed to query audit log")
	}

	result := make([]AuditActivityEntry, len(entries))
	for i, e := range entries {
		result[i] = AuditActivityEntry{
			Action:    e.Action,
			Resource:  e.Resource,
			Source:    e.Source,
			Summary:   auditSummary(e),
			CreatedAt: e.CreatedAt.Format(time.RFC3339),
		}
	}

	return result, total, nil
}

// handleEntityChangeList handles GET /api/v1/entity-changes.
// Scoped: master admins see all, endeavour admins/owners see their endeavours only.
// Users with no admin/owner role on any endeavour get 403.
func (a *API) handleEntityChangeList(w http.ResponseWriter, r *http.Request) {
	scope, apiErr := a.resolveScope(r.Context())
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	// Determine which endeavour IDs the user can see entity changes for.
	// Master admin: nil (no restriction). Others: only endeavours where admin/owner.
	var endeavourIDs []string
	if !scope.IsMasterAdmin {
		for id, role := range scope.Endeavours {
			if role == "admin" || role == "owner" {
				endeavourIDs = append(endeavourIDs, id)
			}
		}
		if len(endeavourIDs) == 0 {
			writeError(w, http.StatusForbidden, "forbidden", "Endeavour admin privileges required")
			return
		}
	}

	opts := storage.ListEntityChangesOpts{
		Action:       queryString(r, "action"),
		EntityType:   queryString(r, "entity_type"),
		EntityID:     queryString(r, "entity_id"),
		ActorID:      queryString(r, "actor_id"),
		EndeavourID:  queryString(r, "endeavour_id"),
		EndeavourIDs: endeavourIDs,
		Limit:        queryInt(r, "limit", 50),
		Offset:       queryInt(r, "offset", 0),
	}

	if s := queryString(r, "start_time"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			opts.StartTime = &t
		}
	}
	if s := queryString(r, "end_time"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			opts.EndTime = &t
		}
	}

	// If a specific endeavour_id filter is provided, verify scope.
	if opts.EndeavourID != "" && !scope.IsMasterAdmin {
		allowed := false
		for _, id := range endeavourIDs {
			if id == opts.EndeavourID {
				allowed = true
				break
			}
		}
		if !allowed {
			writeError(w, http.StatusForbidden, "forbidden", "No access to this endeavour")
			return
		}
	}

	entries, total, err := a.db.ListEntityChanges(opts)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to query entity changes")
		return
	}

	writeList(w, entries, total, opts.Limit, opts.Offset)
}

// ListEntityChanges queries entity changes with scope-based access control.
// Used by the MCP tool handler.
func (a *API) ListEntityChanges(ctx context.Context, opts storage.ListEntityChangesOpts) ([]*storage.EntityChangeRecord, int, *APIError) {
	scope, apiErr := a.resolveScope(ctx)
	if apiErr != nil {
		return nil, 0, apiErr
	}

	if !scope.IsMasterAdmin {
		var endeavourIDs []string
		for id, role := range scope.Endeavours {
			if role == "admin" || role == "owner" {
				endeavourIDs = append(endeavourIDs, id)
			}
		}
		if len(endeavourIDs) == 0 {
			return nil, 0, errForbidden("Endeavour admin privileges required")
		}
		opts.EndeavourIDs = endeavourIDs

		// Verify specific endeavour filter is in scope.
		if opts.EndeavourID != "" {
			allowed := false
			for _, id := range endeavourIDs {
				if id == opts.EndeavourID {
					allowed = true
					break
				}
			}
			if !allowed {
				return nil, 0, errForbidden("No access to this endeavour")
			}
		}
	}

	entries, total, err := a.db.ListEntityChanges(opts)
	if err != nil {
		return nil, 0, errInternal("Failed to query entity changes")
	}

	return entries, total, nil
}

// auditSourceLabel returns a human-readable suffix from the source field.
func auditSourceLabel(source string) string {
	switch source {
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

// auditSummary generates a human-readable summary from an audit log record.
func auditSummary(e *storage.AuditLogRecord) string {
	switch e.Action {
	case "login_success":
		return "Logged in" + auditSourceLabel(e.Source)
	case "login_failure":
		return "Login attempt failed" + auditSourceLabel(e.Source)
	case "token_created":
		return "Created an API token"
	case "token_revoked":
		return "Revoked an API token"
	case "password_changed":
		return "Changed password"
	case "password_reset_requested":
		return "Requested password reset"
	case "session_created":
		return "Started a new session"
	case "session_expired":
		return "Session expired"
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
	case "intercom_send":
		return "Sent intercom message"
	case "intercom_receive":
		return "Received intercom message"
	case "request":
		return summarizeRequest(e.Method, e.Endpoint, e.StatusCode)
	default:
		return e.Action
	}
}

// summarizeRequest generates a readable summary for HTTP request audit entries.
func summarizeRequest(method, endpoint string, statusCode int) string {
	if endpoint == "" {
		return fmt.Sprintf("%s request (HTTP %d)", method, statusCode)
	}

	// Extract entity type from endpoint pattern /api/v1/{entity} or /mcp
	parts := strings.Split(strings.TrimPrefix(endpoint, "/"), "/")

	if len(parts) >= 3 && parts[0] == "api" && parts[1] == "v1" {
		entity := parts[2]
		switch method {
		case "POST":
			return fmt.Sprintf("Created %s", singularize(entity))
		case "GET":
			if len(parts) > 3 {
				return fmt.Sprintf("Viewed %s", singularize(entity))
			}
			return fmt.Sprintf("Listed %s", entity)
		case "PATCH", "PUT":
			return fmt.Sprintf("Updated %s", singularize(entity))
		case "DELETE":
			return fmt.Sprintf("Deleted %s", singularize(entity))
		}
	}

	if endpoint == "/mcp" || strings.HasPrefix(endpoint, "/mcp") {
		return fmt.Sprintf("MCP tool call (HTTP %d)", statusCode)
	}

	return fmt.Sprintf("%s %s (HTTP %d)", method, endpoint, statusCode)
}

// UnifiedActivityEntry represents a single item in the merged activity timeline.
type UnifiedActivityEntry struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`                    // "login", "work"
	Action      string                 `json:"action"`                  // e.g. "login_success", "create", "update"
	ActorID     string                 `json:"actor_id"`
	Summary     string                 `json:"summary"`
	EntityType  string                 `json:"entity_type,omitempty"`   // Only for entity changes
	EntityID    string                 `json:"entity_id,omitempty"`     // Only for entity changes
	EndeavourID string                 `json:"endeavour_id,omitempty"`  // Only for entity changes
	Fields      []string               `json:"fields,omitempty"`        // Only for entity changes
	Metadata    map[string]interface{} `json:"metadata,omitempty"`      // Only for entity changes
	Source      string                 `json:"source,omitempty"`        // Only for audit entries
	CreatedAt   string                 `json:"created_at"`
}

// ActivitySummary holds aggregated counts for the activity page header.
type ActivitySummary struct {
	Logins       int `json:"logins"`
	Tasks        int `json:"tasks"`
	Demands      int `json:"demands"`
	Endeavours   int `json:"endeavours"`
	Resources    int `json:"resources"`
	Orgs         int `json:"organizations"`
	Other        int `json:"other"`
	Total        int `json:"total"`
	UniqueActors int `json:"unique_actors"`
}

// HourBucket holds per-hour activity counts for sparkline visualization.
type HourBucket struct {
	Hour       int `json:"hour"`
	Logins     int `json:"logins"`
	Tasks      int `json:"tasks"`
	Demands    int `json:"demands"`
	Endeavours int `json:"endeavours"`
}

// handleActivityList handles GET /api/v1/activity.
// Merges audit log (login events) and entity changes into a unified timeline.
// Scoped: master admins see all, endeavour admins/owners see their entity changes + own logins.
func (a *API) handleActivityList(w http.ResponseWriter, r *http.Request) {
	scope, apiErr := a.resolveScope(r.Context())
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	limit := queryInt(r, "limit", 50)
	offset := queryInt(r, "offset", 0)
	actorID := queryString(r, "actor_id")
	entityType := queryString(r, "entity_type")
	ecAction := queryString(r, "action")

	var startTime, endTime *time.Time
	if s := queryString(r, "start_time"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			startTime = &t
		}
	}
	if s := queryString(r, "end_time"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			endTime = &t
		}
	}

	// Determine entity change scope
	adminMode := queryString(r, "admin") == "true"
	var endeavourIDs []string
	if !adminMode || !scope.IsMasterAdmin {
		for id, role := range scope.Endeavours {
			if role == "admin" || role == "owner" {
				endeavourIDs = append(endeavourIDs, id)
			}
		}
		if len(endeavourIDs) == 0 {
			// No admin/owner endeavours: return empty activity
			writeData(w, http.StatusOK, map[string]interface{}{
				"entries": []interface{}{},
				"total":   0,
				"limit":   limit,
				"offset":  offset,
				"summary": map[string]int{"total": 0, "unique_actors": 0},
				"hourly":  make([]map[string]int, 24),
			})
			return
		}
	}

	// Fetch a large window to merge and paginate in-memory.
	// We need enough to cover the requested offset+limit after merge.
	fetchLimit := offset + limit + 200 // extra buffer for merge

	// Query entity changes
	ecOpts := storage.ListEntityChangesOpts{
		Action:       ecAction,
		EntityType:   entityType,
		ActorID:      actorID,
		EndeavourIDs: endeavourIDs,
		StartTime:    startTime,
		EndTime:      endTime,
		Limit:        fetchLimit,
		Offset:       0,
	}
	ecEntries, ecTotal, err := a.db.ListEntityChanges(ecOpts)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to query entity changes")
		return
	}

	// Query audit log for login events (skip if filtering by entity_type or entity-change actions)
	var auditEntries []*storage.AuditLogRecord
	auditTotal := 0
	includeLogins := entityType == "" && (ecAction == "" || ecAction == "login")
	if includeLogins {
		auditAction := ""
		if ecAction == "login" {
			// When filtering for "login", only show login_success
			auditAction = "login_success"
		}
		auditOpts := storage.ListAuditLogOpts{
			Action:    auditAction,
			ActorID:   actorID,
			StartTime: startTime,
			EndTime:   endTime,
			Limit:     fetchLimit,
			Offset:    0,
		}
		if auditAction == "" {
			// Only include login events by default, not all audit entries
			auditOpts.Action = "login_success"
		}
		if !scope.IsMasterAdmin {
			// Non-admins only see their own logins
			authUser := auth.GetAuthUser(r.Context())
			if authUser != nil {
				auditOpts.ActorID = authUser.UserID
			}
		}
		auditEntries, auditTotal, err = a.db.ListAuditLog(auditOpts)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "Failed to query audit log")
			return
		}
	}

	// Merge into unified entries
	merged := make([]UnifiedActivityEntry, 0, len(ecEntries)+len(auditEntries))

	for _, ec := range ecEntries {
		summary := ec.Action + " " + ec.EntityType
		merged = append(merged, UnifiedActivityEntry{
			ID:          ec.ID,
			Type:        "work",
			Action:      ec.Action,
			ActorID:     ec.ActorID,
			Summary:     summary,
			EntityType:  ec.EntityType,
			EntityID:    ec.EntityID,
			EndeavourID: ec.EndeavourID,
			Fields:      ec.Fields,
			Metadata:    ec.Metadata,
			CreatedAt:   ec.CreatedAt.Format(time.RFC3339),
		})
	}

	for _, al := range auditEntries {
		merged = append(merged, UnifiedActivityEntry{
			ID:        al.ID,
			Type:      "login",
			Action:    al.Action,
			ActorID:   al.ActorID,
			Summary:   auditSummary(al),
			Source:    al.Source,
			CreatedAt: al.CreatedAt.Format(time.RFC3339),
		})
	}

	// Sort by created_at descending
	sort.Slice(merged, func(i, j int) bool {
		return merged[i].CreatedAt > merged[j].CreatedAt
	})

	// Compute summary from the full set (before pagination)
	summary := ActivitySummary{}
	actorSet := map[string]struct{}{}
	for _, e := range merged {
		if e.ActorID != "" {
			actorSet[e.ActorID] = struct{}{}
		}
		switch e.Type {
		case "login":
			summary.Logins++
		case "work":
			switch e.EntityType {
			case "task":
				summary.Tasks++
			case "demand":
				summary.Demands++
			case "endeavour":
				summary.Endeavours++
			case "resource":
				summary.Resources++
			case "organization":
				summary.Orgs++
			default:
				summary.Other++
			}
		}
	}

	total := ecTotal + auditTotal
	summary.Total = total
	summary.UniqueActors = len(actorSet)

	// Compute hourly buckets for sparkline
	hourly := make([]HourBucket, 24)
	for i := range hourly {
		hourly[i].Hour = i
	}
	for _, e := range merged {
		t, err := time.Parse(time.RFC3339, e.CreatedAt)
		if err != nil {
			continue
		}
		h := t.Hour()
		switch e.Type {
		case "login":
			hourly[h].Logins++
		case "work":
			switch e.EntityType {
			case "task":
				hourly[h].Tasks++
			case "demand":
				hourly[h].Demands++
			case "endeavour":
				hourly[h].Endeavours++
			}
		}
	}

	// Paginate
	start := offset
	if start > len(merged) {
		start = len(merged)
	}
	end := start + limit
	if end > len(merged) {
		end = len(merged)
	}
	page := merged[start:end]

	// Write response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(struct {
		Data    []UnifiedActivityEntry `json:"data"`
		Summary ActivitySummary        `json:"summary"`
		Hourly  []HourBucket           `json:"hourly"`
		Meta    listMeta               `json:"meta"`
	}{
		Data:    page,
		Summary: summary,
		Hourly:  hourly,
		Meta:    listMeta{Total: total, Limit: limit, Offset: offset},
	})
}

// singularize removes trailing 's' for simple English plurals.
func singularize(s string) string {
	if strings.HasSuffix(s, "ies") {
		return s[:len(s)-3] + "y"
	}
	if strings.HasSuffix(s, "us") || strings.HasSuffix(s, "ss") {
		return s
	}
	if strings.HasSuffix(s, "s") {
		return s[:len(s)-1]
	}
	return s
}
