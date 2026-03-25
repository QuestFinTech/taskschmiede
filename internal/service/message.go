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


package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/QuestFinTech/taskschmiede/internal/storage"
)

// MessageService handles messaging business logic.
// It uses a separate MessageDB for messages and the main DB for
// resource/relation lookups (scope expansion, sender name resolution).
type MessageService struct {
	mdb    *storage.MessageDB
	mainDB *storage.DB
	logger *slog.Logger
}

// NewMessageService creates a new MessageService.
func NewMessageService(mdb *storage.MessageDB, mainDB *storage.DB, logger *slog.Logger) *MessageService {
	return &MessageService{mdb: mdb, mainDB: mainDB, logger: logger}
}

// validIntents lists the allowed message intent values.
var validIntents = map[string]bool{
	"info":     true,
	"question": true,
	"action":   true,
	"alert":    true,
}

// validScopeTypes lists the allowed scope types.
var validScopeTypes = map[string]bool{
	"endeavour":    true,
	"organization": true,
}

// Send validates, creates a message, expands scope to recipients, and creates delivery records.
func (s *MessageService) Send(ctx context.Context, senderID, subject, content, intent, replyToID, entityType, entityID string, recipientIDs []string, scopeType, scopeID string, metadata map[string]interface{}) (*storage.Message, error) {
	// Validate required fields
	if senderID == "" {
		return nil, fmt.Errorf("sender_id is required")
	}
	if content == "" {
		return nil, fmt.Errorf("content is required")
	}

	// Default intent
	if intent == "" {
		intent = "info"
	}
	if !validIntents[intent] {
		return nil, fmt.Errorf("invalid intent: %s (must be info, question, action, or alert)", intent)
	}

	// Must have either explicit recipients or a scope
	if len(recipientIDs) == 0 && scopeType == "" {
		return nil, fmt.Errorf("at least one recipient_id or a scope (scope_type + scope_id) is required")
	}

	// Validate scope
	if scopeType != "" {
		if !validScopeTypes[scopeType] {
			return nil, fmt.Errorf("invalid scope_type: %s (must be endeavour or organization)", scopeType)
		}
		if scopeID == "" {
			return nil, fmt.Errorf("scope_id is required when scope_type is set")
		}
	}

	// Validate reply-to exists
	if replyToID != "" {
		if _, err := s.mdb.GetMessage(replyToID); err != nil {
			return nil, fmt.Errorf("reply_to message not found: %w", err)
		}
	}

	// Resolve recipient IDs: accept both res_ (resource) and usr_ (user) prefixes.
	// User IDs are resolved to their linked resource ID.
	resolvedRecipients := make([]string, 0, len(recipientIDs))
	for _, rid := range recipientIDs {
		switch {
		case strings.HasPrefix(rid, "res_"):
			if _, err := s.mainDB.GetResource(rid); err != nil {
				return nil, fmt.Errorf("recipient resource not found: %s", rid)
			}
			resolvedRecipients = append(resolvedRecipients, rid)
		case strings.HasPrefix(rid, "usr_"):
			user, err := s.mainDB.GetUser(rid)
			if err != nil {
				return nil, fmt.Errorf("recipient user not found: %s", rid)
			}
			if user.ResourceID == nil || *user.ResourceID == "" {
				return nil, fmt.Errorf("recipient user %s has no linked resource", rid)
			}
			resolvedRecipients = append(resolvedRecipients, *user.ResourceID)
		default:
			return nil, fmt.Errorf("invalid recipient ID %q: must be a resource (res_...) or user (usr_...) ID", rid)
		}
	}

	// Expand scope to individual recipients.
	// Explicit recipients are kept as-is (allows self-sends for confirmations).
	// Sender is only excluded from scope expansion (group delivery).
	allRecipients := make(map[string]bool)
	for _, rid := range resolvedRecipients {
		allRecipients[rid] = true
	}

	if scopeType != "" {
		expanded, err := s.expandScope(scopeType, scopeID)
		if err != nil {
			return nil, fmt.Errorf("expand scope: %w", err)
		}
		for _, rid := range expanded {
			if rid != senderID { // exclude sender from scope expansion
				allRecipients[rid] = true
			}
		}
	}

	if len(allRecipients) == 0 {
		return nil, fmt.Errorf("no recipients: provide recipient_ids (res_...) for direct messages, or scope_type and scope_id for group delivery")
	}

	// Create message
	msg, err := s.mdb.CreateMessage(senderID, subject, content, intent, replyToID,
		entityType, entityID, scopeType, scopeID, metadata)
	if err != nil {
		return nil, fmt.Errorf("create message: %w", err)
	}

	// Determine delivery channel per recipient and create deliveries
	targets := make([]storage.DeliveryTarget, 0, len(allRecipients))
	for rid := range allRecipients {
		channel := s.resolveChannel(rid)
		targets = append(targets, storage.DeliveryTarget{RecipientID: rid, Channel: channel})
	}

	if err := s.mdb.CreateDeliveryBatch(msg.ID, targets); err != nil {
		return nil, fmt.Errorf("create deliveries: %w", err)
	}

	// For internal deliveries, check if the recipient's user has email_copy enabled.
	// If so, stamp copy_email on the delivery so the intercom bridge sends an email copy.
	for _, t := range targets {
		if t.Channel != "internal" {
			continue
		}
		s.stampCopyEmail(msg.ID, t.RecipientID)
	}

	s.logger.Info("Message sent",
		"id", msg.ID,
		"sender_id", senderID,
		"recipients", len(targets),
		"intent", intent,
		"scope_type", scopeType,
	)

	return msg, nil
}

// Inbox returns messages delivered to a recipient.
func (s *MessageService) Inbox(ctx context.Context, recipientID string, opts storage.ListInboxOpts) ([]*storage.InboxItem, int, error) {
	if recipientID == "" {
		return nil, 0, fmt.Errorf("recipient_id is required")
	}
	return s.mdb.ListInbox(recipientID, opts)
}

// Read retrieves a message and marks its delivery as "read" for the given recipient.
func (s *MessageService) Read(ctx context.Context, messageID, recipientID string) (*storage.Message, *storage.MessageDelivery, error) {
	if messageID == "" {
		return nil, nil, fmt.Errorf("message_id is required")
	}
	if recipientID == "" {
		return nil, nil, fmt.Errorf("recipient_id is required")
	}

	msg, err := s.mdb.GetMessage(messageID)
	if err != nil {
		return nil, nil, err
	}

	delivery, err := s.mdb.GetDeliveryByRecipient(messageID, recipientID)
	if err != nil {
		return nil, nil, fmt.Errorf("delivery not found for this recipient: %w", err)
	}

	// Auto-mark as read if not already
	if delivery.Status == "pending" || delivery.Status == "delivered" || delivery.Status == "copied" {
		if err := s.mdb.MarkRead(delivery.ID); err != nil {
			slog.Warn("Failed to auto-mark message as read", "delivery_id", delivery.ID, "error", err)
		}
		// Refresh delivery to get updated timestamps
		if refreshed, err := s.mdb.GetDelivery(delivery.ID); err == nil {
			delivery = refreshed
		}
	}

	return msg, delivery, nil
}

// Reply creates a reply to an existing message.
func (s *MessageService) Reply(ctx context.Context, senderID, messageID, content string, metadata map[string]interface{}) (*storage.Message, error) {
	if senderID == "" {
		return nil, fmt.Errorf("sender_id is required")
	}
	if messageID == "" {
		return nil, fmt.Errorf("message_id is required")
	}
	if content == "" {
		return nil, fmt.Errorf("content is required")
	}

	// Get original message
	original, err := s.mdb.GetMessage(messageID)
	if err != nil {
		return nil, fmt.Errorf("original message not found: %w", err)
	}

	// Inherit context from original
	entityType := original.EntityType
	entityID := original.EntityID

	// Determine reply recipients:
	// If the original had a scope, reply goes to all original recipients.
	// Otherwise, reply goes to the original sender (1:1).
	var recipientIDs []string
	if original.ScopeType != "" {
		// Group reply: re-expand scope, but also include original sender
		expanded, err := s.expandScope(original.ScopeType, original.ScopeID)
		if err != nil {
			return nil, fmt.Errorf("expand scope for reply: %w", err)
		}
		recipientSet := make(map[string]bool)
		for _, rid := range expanded {
			if rid != senderID {
				recipientSet[rid] = true
			}
		}
		// Include original sender if not the current sender
		if original.SenderID != senderID {
			recipientSet[original.SenderID] = true
		}
		for rid := range recipientSet {
			recipientIDs = append(recipientIDs, rid)
		}
	} else {
		// 1:1 reply: send to original sender
		if original.SenderID == senderID {
			return nil, fmt.Errorf("cannot reply to your own message in a direct conversation")
		}
		recipientIDs = []string{original.SenderID}
	}

	// Use the same intent as original by default (info for replies)
	return s.Send(ctx, senderID, original.Subject, content, "info",
		messageID, entityType, entityID, recipientIDs, "", "", metadata)
}

// Thread retrieves all messages in a conversation thread.
func (s *MessageService) Thread(ctx context.Context, messageID string) ([]*storage.Message, error) {
	if messageID == "" {
		return nil, fmt.Errorf("message_id is required")
	}
	return s.mdb.GetThread(messageID)
}

// ResolveSenderName looks up a sender's display name from the main DB.
func (s *MessageService) ResolveSenderName(senderID string) string {
	// Try user table first (prefer user.name over resource.name)
	var name string
	err := s.mainDB.QueryRow(
		`SELECT COALESCE(u.name, r.name) FROM resource r
		 LEFT JOIN user u ON u.resource_id = r.id
		 WHERE r.id = ?`, senderID,
	).Scan(&name)
	if err == nil && name != "" {
		return name
	}
	return senderID
}

// IsRecipient checks if a resource has a delivery for the given message.
func (s *MessageService) IsRecipient(ctx context.Context, messageID, recipientID string) bool {
	_, err := s.mdb.GetDeliveryByRecipient(messageID, recipientID)
	return err == nil
}

// expandScope resolves a scope (endeavour or organization) to a list of member resource IDs.
func (s *MessageService) expandScope(scopeType, scopeID string) ([]string, error) {
	var rels []*storage.EntityRelation
	var err error

	switch scopeType {
	case "endeavour":
		// Find resources that are members of this endeavour via FRM relations.
		// Look for: resource -> member_of -> endeavour
		// and: endeavour -> has_member -> resource
		rels, _, err = s.mainDB.ListRelations(storage.ListRelationsOpts{
			TargetEntityID:   scopeID,
			TargetEntityType: "endeavour",
			RelationshipType: storage.RelMemberOf,
			Limit:            1000,
		})
		if err != nil {
			return nil, fmt.Errorf("list endeavour members (member_of): %w", err)
		}

		// Also check has_member direction
		rels2, _, err := s.mainDB.ListRelations(storage.ListRelationsOpts{
			SourceEntityID:   scopeID,
			SourceEntityType: "endeavour",
			RelationshipType: storage.RelHasMember,
			Limit:            1000,
		})
		if err != nil {
			return nil, fmt.Errorf("list endeavour members (has_member): %w", err)
		}

		seen := make(map[string]bool)
		var resourceIDs []string
		for _, r := range rels {
			if r.SourceEntityType == "resource" && !seen[r.SourceEntityID] {
				seen[r.SourceEntityID] = true
				resourceIDs = append(resourceIDs, r.SourceEntityID)
			}
			if r.SourceEntityType == "user" {
				// Resolve user -> resource
				rid := s.resolveUserResource(r.SourceEntityID)
				if rid != "" && !seen[rid] {
					seen[rid] = true
					resourceIDs = append(resourceIDs, rid)
				}
			}
		}
		for _, r := range rels2 {
			if r.TargetEntityType == "resource" && !seen[r.TargetEntityID] {
				seen[r.TargetEntityID] = true
				resourceIDs = append(resourceIDs, r.TargetEntityID)
			}
		}
		return resourceIDs, nil

	case "organization":
		// Find resources that are members of this organization.
		// organization -> has_member -> resource
		rels, _, err = s.mainDB.ListRelations(storage.ListRelationsOpts{
			SourceEntityID:   scopeID,
			SourceEntityType: "organization",
			RelationshipType: storage.RelHasMember,
			Limit:            1000,
		})
		if err != nil {
			return nil, fmt.Errorf("list organization members: %w", err)
		}

		seen := make(map[string]bool)
		var resourceIDs []string
		for _, r := range rels {
			if r.TargetEntityType == "resource" && !seen[r.TargetEntityID] {
				seen[r.TargetEntityID] = true
				resourceIDs = append(resourceIDs, r.TargetEntityID)
			}
		}
		return resourceIDs, nil
	}

	return nil, fmt.Errorf("unsupported scope type: %s", scopeType)
}

// resolveChannel determines the delivery channel for a recipient.
// Checks resource metadata for delivery.channels preference.
// Default: "internal" for all resource types.
func (s *MessageService) resolveChannel(recipientID string) string {
	res, err := s.mainDB.GetResource(recipientID)
	if err != nil {
		return "internal"
	}

	if delivery, ok := res.Metadata["delivery"].(map[string]interface{}); ok {
		if channels, ok := delivery["channels"].([]interface{}); ok {
			for _, ch := range channels {
				if chStr, ok := ch.(string); ok && chStr == "email" {
					// Check if email is configured
					if emailAddr, ok := delivery["email"].(string); ok && emailAddr != "" {
						return "email"
					}
				}
			}
		}
	}

	return "internal"
}

// stampCopyEmail checks if the recipient's user has email_copy enabled,
// and if so, sets copy_email on the delivery record.
func (s *MessageService) stampCopyEmail(messageID, recipientID string) {
	// Look up user by resource_id
	user, err := s.lookupUserByResource(recipientID)
	if err != nil || user == nil {
		return
	}
	if !user.EmailCopy || user.Email == "" {
		return
	}
	// Find the delivery for this message+recipient and set copy_email
	delivery, err := s.mdb.GetDeliveryByRecipient(messageID, recipientID)
	if err != nil {
		return
	}
	if err := s.mdb.SetCopyEmail(delivery.ID, user.Email); err != nil {
		s.logger.Warn("Failed to set copy_email on delivery",
			"delivery_id", delivery.ID, "email", user.Email, "error", err)
	}
}

// lookupUserByResource finds the user associated with a resource_id.
func (s *MessageService) lookupUserByResource(resourceID string) (*storage.User, error) {
	var userID string
	err := s.mainDB.QueryRow(`SELECT id FROM user WHERE resource_id = ? AND status = 'active'`, resourceID).Scan(&userID)
	if err != nil {
		return nil, err
	}
	return s.mainDB.GetUser(userID)
}

// resolveUserResource gets the resource_id for a user ID.
func (s *MessageService) resolveUserResource(userID string) string {
	var resourceID *string
	err := s.mainDB.QueryRow(`SELECT resource_id FROM user WHERE id = ?`, userID).Scan(&resourceID)
	if err != nil || resourceID == nil {
		return ""
	}
	return *resourceID
}
