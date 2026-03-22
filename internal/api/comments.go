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
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/QuestFinTech/taskschmiede/internal/auth"
	"github.com/QuestFinTech/taskschmiede/internal/security"
	"github.com/QuestFinTech/taskschmiede/internal/storage"
)

// --- Exported business logic methods ---

// CreateComment creates a comment on an entity. The author is resolved from the
// authenticated user's linked resource.
func (a *API) CreateComment(ctx context.Context, entityType, entityID, replyToID, content string, metadata map[string]interface{}) (map[string]interface{}, *APIError) {
	if apiErr := validateCommentCreate(entityType, entityID, content, replyToID, metadata); apiErr != nil {
		return nil, apiErr
	}
	if apiErr := a.CheckCreationVelocity(ctx); apiErr != nil {
		return nil, apiErr
	}
	// RBAC: require write access to parent entity's endeavour
	if entityType != "" && entityType != "organization" {
		edvID, apiErr := a.resolveEntityEndeavourID(ctx, entityType, entityID)
		if apiErr != nil {
			return nil, apiErr
		}
		scope, apiErr := a.resolveScope(ctx)
		if apiErr != nil {
			return nil, apiErr
		}
		if apiErr := checkEndeavourWrite(scope, edvID); apiErr != nil {
			return nil, errNotFound("entity", "Not found")
		}
	}
	authorID, apiErr := a.resolveCallerResourceID(ctx)
	if apiErr != nil {
		return nil, apiErr
	}

	metadata = scoreAndAnnotate(metadata, content)
	comment, err := a.cmtSvc.Create(ctx, entityType, entityID, authorID, replyToID, content, metadata)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, errNotFound("entity", err.Error())
		}
		return nil, errInvalidInput(err.Error())
	}
	return commentToMap(comment), nil
}

// GetComment retrieves a comment by ID, including its direct replies.
func (a *API) GetComment(ctx context.Context, id string) (map[string]interface{}, *APIError) {
	comment, replies, err := a.cmtSvc.GetWithReplies(ctx, id)
	if err != nil {
		if errors.Is(err, storage.ErrCommentNotFound) {
			return nil, errNotFound("comment", "Comment not found")
		}
		return nil, errInternal("Failed to get comment")
	}
	// RBAC: require read access to parent entity's endeavour
	if comment.EntityType != "organization" {
		edvID, apiErr := a.resolveEntityEndeavourID(ctx, comment.EntityType, comment.EntityID)
		if apiErr != nil {
			return nil, errNotFound("comment", "Comment not found")
		}
		scope, apiErr := a.resolveScope(ctx)
		if apiErr != nil {
			return nil, apiErr
		}
		if apiErr := checkEndeavourRead(scope, edvID); apiErr != nil {
			return nil, errNotFound("comment", "Comment not found")
		}
	}

	result := commentToMap(comment)
	replyMaps := make([]map[string]interface{}, 0, len(replies))
	for _, r := range replies {
		replyMaps = append(replyMaps, commentToMap(r))
	}
	result["replies"] = replyMaps

	return result, nil
}

// ListComments returns a paginated list of comments for an entity.
func (a *API) ListComments(ctx context.Context, opts storage.ListCommentsOpts) ([]map[string]interface{}, int, *APIError) {
	// RBAC: require read access to parent entity's endeavour
	if opts.EntityType != "" && opts.EntityID != "" && opts.EntityType != "organization" {
		edvID, apiErr := a.resolveEntityEndeavourID(ctx, opts.EntityType, opts.EntityID)
		if apiErr != nil {
			return nil, 0, apiErr
		}
		scope, apiErr := a.resolveScope(ctx)
		if apiErr != nil {
			return nil, 0, apiErr
		}
		if apiErr := checkEndeavourRead(scope, edvID); apiErr != nil {
			return nil, 0, errNotFound("entity", "Not found")
		}
	}
	// When no entity filter is provided, scope by accessible endeavours
	// to prevent cross-tenant data access. EndeavourIDs nil = no restriction (admin).
	if opts.EntityType == "" || opts.EntityID == "" {
		if opts.EndeavourIDs == nil {
			adminMode := false
			opts.EndeavourIDs = a.ResolveEndeavourIDs(ctx, adminMode)
		}
	}
	comments, total, err := a.cmtSvc.List(ctx, opts)
	if err != nil {
		return nil, 0, errInvalidInput(err.Error())
	}

	items := make([]map[string]interface{}, 0, len(comments))
	for _, c := range comments {
		items = append(items, commentToMap(c))
	}
	return items, total, nil
}

// UpdateComment updates a comment (owner-only).
func (a *API) UpdateComment(ctx context.Context, id string, fields storage.UpdateCommentFields) (map[string]interface{}, *APIError) {
	if apiErr := validateCommentUpdate(id, fields.Content, fields.Metadata); apiErr != nil {
		return nil, apiErr
	}
	if fields.Content != nil {
		fields.Metadata = scoreAndAnnotate(fields.Metadata, *fields.Content)
	}

	callerResourceID, apiErr := a.resolveCallerResourceID(ctx)
	if apiErr != nil {
		return nil, apiErr
	}

	_, err := a.cmtSvc.Update(ctx, id, callerResourceID, fields)
	if err != nil {
		if errors.Is(err, storage.ErrCommentNotFound) {
			return nil, errNotFound("comment", "Comment not found")
		}
		if strings.Contains(err.Error(), "only the author") {
			return nil, errForbidden(err.Error())
		}
		return nil, errInvalidInput(err.Error())
	}

	comment, err := a.cmtSvc.Get(ctx, id)
	if err != nil {
		return nil, errInternal("Failed to get updated comment")
	}
	return commentToMap(comment), nil
}

// DeleteComment soft-deletes a comment (owner-only).
func (a *API) DeleteComment(ctx context.Context, id string) (map[string]interface{}, *APIError) {
	callerResourceID, apiErr := a.resolveCallerResourceID(ctx)
	if apiErr != nil {
		return nil, apiErr
	}

	err := a.cmtSvc.Delete(ctx, id, callerResourceID)
	if err != nil {
		if errors.Is(err, storage.ErrCommentNotFound) {
			return nil, errNotFound("comment", "Comment not found")
		}
		if strings.Contains(err.Error(), "only the author") {
			return nil, errForbidden(err.Error())
		}
		return nil, errInvalidInput(err.Error())
	}

	return map[string]interface{}{"deleted": true, "id": id}, nil
}

// --- Helpers ---

// resolveCallerResourceID gets the authenticated user's resource_id.
func (a *API) resolveCallerResourceID(ctx context.Context) (string, *APIError) {
	authUser := auth.GetAuthUser(ctx)
	if authUser == nil {
		return "", errUnauthorized("Authentication required")
	}

	user, err := a.usrSvc.Get(ctx, authUser.UserID)
	if err != nil {
		return "", errInternal("Failed to resolve user")
	}

	if user.ResourceID == nil || *user.ResourceID == "" {
		return "", errInvalidInput("User has no linked resource; join an organization first")
	}

	return *user.ResourceID, nil
}

func commentToMap(c *storage.Comment) map[string]interface{} {
	m := map[string]interface{}{
		"id":          c.ID,
		"entity_type": c.EntityType,
		"entity_id":   c.EntityID,
		"author_id":   c.AuthorID,
		"content":     c.Content,
		"metadata":    c.Metadata,
		"created_at":  c.CreatedAt.Format(time.RFC3339),
		"updated_at":  c.UpdatedAt.Format(time.RFC3339),
	}
	if c.AuthorName != "" {
		m["author_name"] = c.AuthorName
	}
	if c.ReplyToID != "" {
		m["reply_to_id"] = c.ReplyToID
	}
	if c.EditedAt != nil {
		m["edited_at"] = c.EditedAt.Format(time.RFC3339)
	}
	if c.DeletedAt != nil {
		m["deleted_at"] = c.DeletedAt.Format(time.RFC3339)
	}
	return m
}

// --- HTTP handlers (thin wrappers) ---

func (a *API) handleCommentCreate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		EntityType string                 `json:"entity_type"`
		EntityID   string                 `json:"entity_id"`
		Content    string                 `json:"content"`
		ReplyToID  string                 `json:"reply_to_id"`
		Metadata   map[string]interface{} `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Invalid JSON body")
		return
	}

	result, apiErr := a.CreateComment(r.Context(), sanitize(body.EntityType), sanitize(body.EntityID), sanitize(body.ReplyToID), sanitize(body.Content), security.SanitizeMap(body.Metadata))
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	writeData(w, http.StatusCreated, result)
}

func (a *API) handleCommentList(w http.ResponseWriter, r *http.Request) {
	opts := storage.ListCommentsOpts{
		EntityType: queryString(r, "entity_type"),
		EntityID:   queryString(r, "entity_id"),
		AuthorID:   queryString(r, "author_id"),
		Limit:      queryInt(r, "limit", 50),
		Offset:     queryInt(r, "offset", 0),
	}

	items, total, apiErr := a.ListComments(r.Context(), opts)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	writeList(w, items, total, opts.Limit, opts.Offset)
}

func (a *API) handleCommentGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	result, apiErr := a.GetComment(r.Context(), id)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	writeData(w, http.StatusOK, result)
}

func (a *API) handleCommentUpdate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var body struct {
		Content  *string                `json:"content"`
		Metadata map[string]interface{} `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Invalid JSON body")
		return
	}

	fields := storage.UpdateCommentFields{
		Content:  sanitizePtr(body.Content),
		Metadata: security.SanitizeMap(body.Metadata),
	}

	result, apiErr := a.UpdateComment(r.Context(), id, fields)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	writeData(w, http.StatusOK, result)
}

func (a *API) handleCommentDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	result, apiErr := a.DeleteComment(r.Context(), id)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	writeData(w, http.StatusOK, result)
}
