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

	"github.com/QuestFinTech/taskschmiede/internal/storage"
)

// CommentService handles comment business logic.
type CommentService struct {
	db     *storage.DB
	logger *slog.Logger
}

// NewCommentService creates a new CommentService.
func NewCommentService(db *storage.DB, logger *slog.Logger) *CommentService {
	return &CommentService{db: db, logger: logger}
}

// validCommentableEntities lists entity types that support comments.
var validCommentableEntities = map[string]bool{
	"task":         true,
	"demand":       true,
	"endeavour":    true,
	"artifact":     true,
	"ritual":       true,
	"organization": true,
}

// Create validates inputs and creates a comment.
func (s *CommentService) Create(ctx context.Context, entityType, entityID, authorID, replyToID, content string, metadata map[string]interface{}) (*storage.Comment, error) {
	if content == "" {
		return nil, fmt.Errorf("content is required")
	}
	if entityType == "" {
		return nil, fmt.Errorf("entity_type is required")
	}
	if entityID == "" {
		return nil, fmt.Errorf("entity_id is required")
	}
	if authorID == "" {
		return nil, fmt.Errorf("author_id is required")
	}

	if !validCommentableEntities[entityType] {
		return nil, fmt.Errorf("unsupported entity type: %s", entityType)
	}

	if err := s.entityExists(entityType, entityID); err != nil {
		return nil, fmt.Errorf("target entity not found: %w", err)
	}

	// Enforce one-level threading: if replying to a reply, flatten to the original parent.
	if replyToID != "" {
		parent, err := s.db.GetComment(replyToID)
		if err != nil {
			return nil, fmt.Errorf("reply_to comment not found: %w", err)
		}
		if parent.DeletedAt != nil {
			return nil, fmt.Errorf("cannot reply to a deleted comment")
		}
		if parent.EntityType != entityType || parent.EntityID != entityID {
			return nil, fmt.Errorf("reply_to comment belongs to a different entity")
		}
		// Flatten: if parent is itself a reply, use the parent's parent.
		if parent.ReplyToID != "" {
			replyToID = parent.ReplyToID
		}
	}

	comment, err := s.db.CreateComment(entityType, entityID, authorID, replyToID, content, metadata)
	if err != nil {
		return nil, fmt.Errorf("create comment: %w", err)
	}

	s.logger.Info("Comment created", "id", comment.ID, "entity_type", entityType, "entity_id", entityID, "author_id", authorID)
	return comment, nil
}

// Get retrieves a comment by ID.
func (s *CommentService) Get(ctx context.Context, id string) (*storage.Comment, error) {
	if id == "" {
		return nil, fmt.Errorf("id is required")
	}
	return s.db.GetComment(id)
}

// GetWithReplies retrieves a comment and its direct replies.
func (s *CommentService) GetWithReplies(ctx context.Context, id string) (*storage.Comment, []*storage.Comment, error) {
	if id == "" {
		return nil, nil, fmt.Errorf("id is required")
	}

	comment, err := s.db.GetComment(id)
	if err != nil {
		return nil, nil, err
	}

	replies, err := s.db.GetCommentReplies(id)
	if err != nil {
		return nil, nil, fmt.Errorf("get replies: %w", err)
	}

	return comment, replies, nil
}

// List queries comments for an entity.
func (s *CommentService) List(ctx context.Context, opts storage.ListCommentsOpts) ([]*storage.Comment, int, error) {
	// Entity filter is optional when EndeavourIDs scoping is active.
	if opts.EndeavourIDs == nil {
		if opts.EntityType == "" {
			return nil, 0, fmt.Errorf("entity_type is required")
		}
		if opts.EntityID == "" {
			return nil, 0, fmt.Errorf("entity_id is required")
		}
	}
	return s.db.ListComments(opts)
}

// Update validates ownership and applies updates.
func (s *CommentService) Update(ctx context.Context, id, callerResourceID string, fields storage.UpdateCommentFields) ([]string, error) {
	if id == "" {
		return nil, fmt.Errorf("id is required")
	}

	comment, err := s.db.GetComment(id)
	if err != nil {
		return nil, err
	}

	if comment.DeletedAt != nil {
		return nil, fmt.Errorf("cannot update a deleted comment")
	}

	if comment.AuthorID != callerResourceID {
		return nil, fmt.Errorf("only the author can edit a comment")
	}

	return s.db.UpdateComment(id, fields)
}

// Delete performs a soft delete (owner-only).
func (s *CommentService) Delete(ctx context.Context, id, callerResourceID string) error {
	if id == "" {
		return fmt.Errorf("id is required")
	}

	comment, err := s.db.GetComment(id)
	if err != nil {
		return err
	}

	if comment.DeletedAt != nil {
		return fmt.Errorf("comment is already deleted")
	}

	if comment.AuthorID != callerResourceID {
		return fmt.Errorf("only the author can delete a comment")
	}

	if err := s.db.SoftDeleteComment(id); err != nil {
		return fmt.Errorf("soft delete: %w", err)
	}

	s.logger.Info("Comment deleted", "id", id, "author_id", callerResourceID)
	return nil
}

// entityExists checks whether the target entity exists.
func (s *CommentService) entityExists(entityType, entityID string) error {
	switch entityType {
	case "task":
		_, err := s.db.GetTask(entityID)
		return err
	case "demand":
		_, err := s.db.GetDemand(entityID)
		return err
	case "endeavour":
		_, err := s.db.GetEndeavour(entityID)
		return err
	case "artifact":
		_, err := s.db.GetArtifact(entityID)
		return err
	case "ritual":
		_, err := s.db.GetRitual(entityID)
		return err
	case "organization":
		_, err := s.db.GetOrganization(entityID)
		return err
	default:
		return fmt.Errorf("unsupported entity type: %s", entityType)
	}
}
