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

	"github.com/QuestFinTech/taskschmiede/internal/security"
	"github.com/QuestFinTech/taskschmiede/internal/storage"
)

// --- Exported business logic methods ---

// SendMessage sends a message from the authenticated user.
func (a *API) SendMessage(ctx context.Context, subject, content, intent, replyToID, entityType, entityID string, recipientIDs []string, scopeType, scopeID string, metadata map[string]interface{}) (map[string]interface{}, *APIError) {
	if apiErr := validateMessageSend(subject, content, intent, entityType, entityID, scopeType, scopeID, replyToID, recipientIDs, metadata); apiErr != nil {
		return nil, apiErr
	}
	if apiErr := a.CheckCreationVelocity(ctx); apiErr != nil {
		return nil, apiErr
	}
	if a.msgSvc == nil {
		return nil, errInternal("Messaging is not configured")
	}

	metadata = scoreAndAnnotate(metadata, subject, content)

	senderID, apiErr := a.resolveCallerResourceID(ctx)
	if apiErr != nil {
		return nil, apiErr
	}

	msg, err := a.msgSvc.Send(ctx, senderID, subject, content, intent, replyToID,
		entityType, entityID, recipientIDs, scopeType, scopeID, metadata)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, errNotFound("message", err.Error())
		}
		return nil, errInvalidInput(err.Error())
	}

	result := messageToMap(msg)
	result["sender_name"] = a.msgSvc.ResolveSenderName(senderID)
	return result, nil
}

// GetInbox returns the inbox for the authenticated user.
func (a *API) GetInbox(ctx context.Context, opts storage.ListInboxOpts) ([]map[string]interface{}, int, *APIError) {
	if a.msgSvc == nil {
		return nil, 0, errInternal("Messaging is not configured")
	}

	recipientID, apiErr := a.resolveCallerResourceID(ctx)
	if apiErr != nil {
		return nil, 0, apiErr
	}

	items, total, err := a.msgSvc.Inbox(ctx, recipientID, opts)
	if err != nil {
		return nil, 0, errInvalidInput(err.Error())
	}

	results := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		m := inboxItemToMap(item)
		m["sender_name"] = a.msgSvc.ResolveSenderName(item.SenderID)
		results = append(results, m)
	}
	return results, total, nil
}

// ReadMessage retrieves a message and marks it as read for the caller.
func (a *API) ReadMessage(ctx context.Context, id string) (map[string]interface{}, *APIError) {
	if a.msgSvc == nil {
		return nil, errInternal("Messaging is not configured")
	}

	recipientID, apiErr := a.resolveCallerResourceID(ctx)
	if apiErr != nil {
		return nil, apiErr
	}

	msg, delivery, err := a.msgSvc.Read(ctx, id, recipientID)
	if err != nil {
		if errors.Is(err, storage.ErrMessageNotFound) {
			return nil, errNotFound("message", "Message not found")
		}
		if errors.Is(err, storage.ErrDeliveryNotFound) {
			return nil, errNotFound("delivery", "No delivery found for this message and recipient")
		}
		return nil, errInvalidInput(err.Error())
	}

	result := messageToMap(msg)
	result["sender_name"] = a.msgSvc.ResolveSenderName(msg.SenderID)
	result["delivery"] = deliveryToMap(delivery)
	return result, nil
}

// ReplyToMessage creates a reply to an existing message.
func (a *API) ReplyToMessage(ctx context.Context, messageID, content string, metadata map[string]interface{}) (map[string]interface{}, *APIError) {
	if apiErr := validateEntityID(messageID, "message_id"); apiErr != nil {
		return nil, apiErr
	}
	if err := security.ValidateStringField(content, "content", security.MaxContentLen); err != nil {
		return nil, validationErr(err)
	}
	if err := security.ValidateMetadata(metadata); err != nil {
		return nil, validationErr(err)
	}
	if apiErr := a.CheckCreationVelocity(ctx); apiErr != nil {
		return nil, apiErr
	}
	metadata = scoreAndAnnotate(metadata, content)

	if a.msgSvc == nil {
		return nil, errInternal("Messaging is not configured")
	}

	senderID, apiErr := a.resolveCallerResourceID(ctx)
	if apiErr != nil {
		return nil, apiErr
	}

	msg, err := a.msgSvc.Reply(ctx, senderID, messageID, content, metadata)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, errNotFound("message", err.Error())
		}
		return nil, errInvalidInput(err.Error())
	}

	result := messageToMap(msg)
	result["sender_name"] = a.msgSvc.ResolveSenderName(senderID)
	return result, nil
}

// GetThread retrieves all messages in a conversation thread.
func (a *API) GetThread(ctx context.Context, messageID string) ([]map[string]interface{}, *APIError) {
	if a.msgSvc == nil {
		return nil, errInternal("Messaging is not configured")
	}

	// RBAC: verify caller is a participant (sender or recipient) in the thread
	callerResID, apiErr := a.resolveCallerResourceID(ctx)
	if apiErr != nil {
		return nil, apiErr
	}

	messages, err := a.msgSvc.Thread(ctx, messageID)
	if err != nil {
		if errors.Is(err, storage.ErrMessageNotFound) {
			return nil, errNotFound("message", "Message not found")
		}
		return nil, errInternal("Failed to get thread")
	}

	// Check if caller is participant in at least one message
	scope, apiErr := a.resolveScope(ctx)
	if apiErr != nil {
		return nil, apiErr
	}
	if !scope.IsMasterAdmin {
		isParticipant := false
		for _, msg := range messages {
			if msg.SenderID == callerResID {
				isParticipant = true
				break
			}
		}
		if !isParticipant {
			// Check if caller has any delivery in the thread
			for _, msg := range messages {
				if a.msgSvc.IsRecipient(ctx, msg.ID, callerResID) {
					isParticipant = true
					break
				}
			}
		}
		if !isParticipant {
			return nil, errNotFound("message", "Message not found")
		}
	}

	results := make([]map[string]interface{}, 0, len(messages))
	for _, msg := range messages {
		m := messageToMap(msg)
		m["sender_name"] = a.msgSvc.ResolveSenderName(msg.SenderID)
		results = append(results, m)
	}
	return results, nil
}

// --- Helpers ---

func messageToMap(m *storage.Message) map[string]interface{} {
	result := map[string]interface{}{
		"id":         m.ID,
		"sender_id":  m.SenderID,
		"subject":    m.Subject,
		"content":    m.Content,
		"intent":     m.Intent,
		"metadata":   m.Metadata,
		"created_at": m.CreatedAt.Format(time.RFC3339),
	}
	if m.SenderName != "" {
		result["sender_name"] = m.SenderName
	}
	if m.ReplyToID != "" {
		result["reply_to_id"] = m.ReplyToID
	}
	if m.EntityType != "" {
		result["entity_type"] = m.EntityType
	}
	if m.EntityID != "" {
		result["entity_id"] = m.EntityID
	}
	if m.ScopeType != "" {
		result["scope_type"] = m.ScopeType
	}
	if m.ScopeID != "" {
		result["scope_id"] = m.ScopeID
	}
	return result
}

func deliveryToMap(d *storage.MessageDelivery) map[string]interface{} {
	result := map[string]interface{}{
		"id":           d.ID,
		"message_id":   d.MessageID,
		"recipient_id": d.RecipientID,
		"channel":      d.Channel,
		"status":       d.Status,
		"created_at":   d.CreatedAt.Format(time.RFC3339),
	}
	if d.DeliveredAt != nil {
		result["delivered_at"] = d.DeliveredAt.Format(time.RFC3339)
	}
	if d.ReadAt != nil {
		result["read_at"] = d.ReadAt.Format(time.RFC3339)
	}
	return result
}

func inboxItemToMap(item *storage.InboxItem) map[string]interface{} {
	result := messageToMap(&item.Message)
	result["delivery_id"] = item.DeliveryID
	result["channel"] = item.Channel
	result["status"] = item.Status
	if item.ReadAt != nil {
		result["read_at"] = item.ReadAt.Format(time.RFC3339)
	}
	return result
}

// --- HTTP handlers (thin wrappers) ---

func (a *API) handleMessageSend(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Subject      string                 `json:"subject"`
		Content      string                 `json:"content"`
		Intent       string                 `json:"intent"`
		ReplyToID    string                 `json:"reply_to_id"`
		EntityType   string                 `json:"entity_type"`
		EntityID     string                 `json:"entity_id"`
		RecipientIDs []string               `json:"recipient_ids"`
		ScopeType    string                 `json:"scope_type"`
		ScopeID      string                 `json:"scope_id"`
		Metadata     map[string]interface{} `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Invalid JSON body")
		return
	}

	result, apiErr := a.SendMessage(r.Context(), sanitize(body.Subject), sanitize(body.Content), sanitize(body.Intent),
		sanitize(body.ReplyToID), sanitize(body.EntityType), sanitize(body.EntityID), sanitizeStrings(body.RecipientIDs),
		sanitize(body.ScopeType), sanitize(body.ScopeID), security.SanitizeMap(body.Metadata))
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	writeData(w, http.StatusCreated, result)
}

func (a *API) handleMessageInbox(w http.ResponseWriter, r *http.Request) {
	opts := storage.ListInboxOpts{
		Status:     queryString(r, "status"),
		Intent:     queryString(r, "intent"),
		EntityType: queryString(r, "entity_type"),
		EntityID:   queryString(r, "entity_id"),
		Unread:     queryString(r, "unread") == "true",
		Limit:      queryInt(r, "limit", 50),
		Offset:     queryInt(r, "offset", 0),
	}

	items, total, apiErr := a.GetInbox(r.Context(), opts)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	writeList(w, items, total, opts.Limit, opts.Offset)
}

func (a *API) handleMessageGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	result, apiErr := a.ReadMessage(r.Context(), id)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	writeData(w, http.StatusOK, result)
}

func (a *API) handleMessageRead(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	result, apiErr := a.ReadMessage(r.Context(), id)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	writeData(w, http.StatusOK, result)
}

func (a *API) handleMessageReply(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var body struct {
		Content  string                 `json:"content"`
		Metadata map[string]interface{} `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Invalid JSON body")
		return
	}

	result, apiErr := a.ReplyToMessage(r.Context(), id, sanitize(body.Content), security.SanitizeMap(body.Metadata))
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	writeData(w, http.StatusCreated, result)
}

func (a *API) handleMessageThread(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	messages, apiErr := a.GetThread(r.Context(), id)
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}

	writeData(w, http.StatusOK, map[string]interface{}{
		"messages": messages,
		"total":    len(messages),
	})
}
