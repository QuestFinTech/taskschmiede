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

	"github.com/QuestFinTech/taskschmiede/internal/auth"
	"github.com/QuestFinTech/taskschmiede/internal/storage"
)

// UserService handles user business logic.
type UserService struct {
	db     *storage.DB
	logger *slog.Logger
}

// NewUserService creates a new UserService.
func NewUserService(db *storage.DB, logger *slog.Logger) *UserService {
	return &UserService{db: db, logger: logger}
}

// Get retrieves a user by ID.
func (s *UserService) Get(ctx context.Context, id string) (*storage.User, error) {
	if id == "" {
		return nil, fmt.Errorf("id is required")
	}
	return s.db.GetUser(id)
}

// List queries users with filters.
func (s *UserService) List(ctx context.Context, opts storage.ListUsersOpts) ([]*storage.User, int, error) {
	return s.db.ListUsers(opts)
}

// Create creates a new user with password hashing and validation.
func (s *UserService) Create(ctx context.Context, name, email, password string, resourceID, externalID *string, tier int, userType string, metadata map[string]interface{}) (*storage.User, error) {
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if email == "" {
		return nil, fmt.Errorf("email is required")
	}

	var passwordHash string
	if password != "" {
		if err := auth.ValidatePassword(password); err != nil {
			return nil, err
		}
		hash, err := auth.HashPassword(password)
		if err != nil {
			return nil, fmt.Errorf("hash password: %w", err)
		}
		passwordHash = hash
	}

	user, err := s.db.CreateUser(name, email, passwordHash, resourceID, externalID, tier, userType, "", metadata)
	if err != nil {
		return nil, err
	}

	s.logger.Info("User created", "id", user.ID, "email", email)
	return user, nil
}

// Update applies partial updates to a user.
func (s *UserService) Update(ctx context.Context, id string, fields storage.UpdateUserFields) ([]string, error) {
	if id == "" {
		return nil, fmt.Errorf("id is required")
	}

	updatedFields, err := s.db.UpdateUser(id, fields)
	if err != nil {
		return nil, err
	}

	s.logger.Info("User updated", "id", id, "fields", updatedFields)
	return updatedFields, nil
}
