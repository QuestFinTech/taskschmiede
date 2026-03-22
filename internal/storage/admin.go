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


package storage

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// MasterAdmin represents the super user account.
type MasterAdmin struct {
	ID          string
	Email       string
	Name        string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	LastLoginAt *time.Time
}

// AdminSession represents an authenticated session.
type AdminSession struct {
	ID        string
	AdminID   string
	ExpiresAt time.Time
	CreatedAt time.Time
}

// Admin and session error sentinels.
var (
	// ErrAdminExists is returned when attempting to create a second master admin.
	ErrAdminExists = errors.New("master admin already exists")
	// ErrAdminNotFound is returned when the master admin record does not exist.
	ErrAdminNotFound = errors.New("master admin not found")
	// ErrInvalidPassword is returned when password verification fails.
	ErrInvalidPassword = errors.New("invalid password")
	// ErrSessionNotFound is returned when a session ID does not exist.
	ErrSessionNotFound = errors.New("session not found")
	// ErrSessionExpired is returned when a session has passed its expiry time.
	ErrSessionExpired = errors.New("session expired")
)

// HasMasterAdmin checks if any admin user exists.
func (db *DB) HasMasterAdmin() (bool, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM user WHERE is_admin = 1").Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check admin user: %w", err)
	}
	return count > 0, nil
}

// CreateMasterAdmin creates the master admin account.
func (db *DB) CreateMasterAdmin(email, name, password string) error {
	exists, err := db.HasMasterAdmin()
	if err != nil {
		return err
	}
	if exists {
		return ErrAdminExists
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	_, err = db.Exec(
		"INSERT INTO master_admin (id, email, name, password_hash) VALUES ('master', ?, ?, ?)",
		email, name, string(hash),
	)
	if err != nil {
		return fmt.Errorf("insert master admin: %w", err)
	}

	return nil
}

// AuthenticateMasterAdmin verifies credentials and returns the admin.
func (db *DB) AuthenticateMasterAdmin(email, password string) (*MasterAdmin, error) {
	var admin MasterAdmin
	var passwordHash string
	var name sql.NullString
	var createdAt, updatedAt string
	var lastLoginAt sql.NullString

	err := db.QueryRow(
		"SELECT id, email, name, password_hash, created_at, updated_at, last_login_at FROM master_admin WHERE email = ?",
		email,
	).Scan(&admin.ID, &admin.Email, &name, &passwordHash, &createdAt, &updatedAt, &lastLoginAt)

	if err == sql.ErrNoRows {
		return nil, ErrAdminNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query master admin: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)); err != nil {
		return nil, ErrInvalidPassword
	}

	// Parse fields
	if name.Valid {
		admin.Name = name.String
	}
	admin.CreatedAt = ParseDBTime(createdAt)
	admin.UpdatedAt = ParseDBTime(updatedAt)
	if lastLoginAt.Valid {
		t := ParseDBTime(lastLoginAt.String)
		admin.LastLoginAt = &t
	}

	// Update last login
	_, _ = db.Exec("UPDATE master_admin SET last_login_at = datetime('now') WHERE id = ?", admin.ID)

	return &admin, nil
}

// UpdateMasterAdminPassword updates the master admin password.
// If MCP access is enabled, the linked user's password hash is also updated.
func (db *DB) UpdateMasterAdminPassword(currentPassword, newPassword string) error {
	var passwordHash string
	var mcpUserID sql.NullString
	err := db.QueryRow("SELECT password_hash, mcp_user_id FROM master_admin WHERE id = 'master'").Scan(&passwordHash, &mcpUserID)
	if err == sql.ErrNoRows {
		return ErrAdminNotFound
	}
	if err != nil {
		return fmt.Errorf("query master admin: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(currentPassword)); err != nil {
		return ErrInvalidPassword
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	_, err = db.Exec("UPDATE master_admin SET password_hash = ? WHERE id = 'master'", string(newHash))
	if err != nil {
		return fmt.Errorf("update password: %w", err)
	}

	// Sync password to linked MCP user if enabled
	if mcpUserID.Valid && mcpUserID.String != "" {
		_, err = db.Exec("UPDATE user SET password_hash = ? WHERE id = ?", string(newHash), mcpUserID.String)
		if err != nil {
			return fmt.Errorf("sync mcp user password: %w", err)
		}
	}

	return nil
}

// GetMCPAccessEnabled returns true -- admin users have inherent MCP access.
// Kept for API compatibility; will be removed in Phase 2.
func (db *DB) GetMCPAccessEnabled() (bool, error) {
	return true, nil
}

// EnableMCPAccess is a no-op -- admin users have inherent MCP access.
// Kept for API compatibility; will be removed in Phase 2.
func (db *DB) EnableMCPAccess() error {
	return nil
}

// DisableMCPAccess is a no-op -- admin users have inherent MCP access.
// Kept for API compatibility; will be removed in Phase 2.
func (db *DB) DisableMCPAccess() error {
	return nil
}

// GetMasterAdminUserID returns the user ID of the first admin user.
// Returns empty string if no admin user exists.
func (db *DB) GetMasterAdminUserID() string {
	var userID string
	err := db.QueryRow(
		`SELECT id FROM user WHERE is_admin = 1 AND status = 'active' ORDER BY created_at ASC LIMIT 1`,
	).Scan(&userID)
	if err != nil {
		return ""
	}
	return userID
}

// CreateSession creates a new admin session.
func (db *DB) CreateSession(adminID string, duration time.Duration) (*AdminSession, error) {
	// Generate random session ID
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return nil, fmt.Errorf("generate session id: %w", err)
	}
	sessionID := hex.EncodeToString(bytes)

	now := UTCNow()
	expiresAt := now.Add(duration)

	_, err := db.Exec(
		"INSERT INTO admin_session (id, admin_id, expires_at) VALUES (?, ?, ?)",
		sessionID, adminID, expiresAt.Format(time.RFC3339),
	)
	if err != nil {
		return nil, fmt.Errorf("insert session: %w", err)
	}

	return &AdminSession{
		ID:        sessionID,
		AdminID:   adminID,
		ExpiresAt: expiresAt,
		CreatedAt: now,
	}, nil
}

// ValidateSession checks if a session is valid and returns the admin ID.
func (db *DB) ValidateSession(sessionID string) (string, error) {
	var adminID, expiresAt string
	err := db.QueryRow(
		"SELECT admin_id, expires_at FROM admin_session WHERE id = ?",
		sessionID,
	).Scan(&adminID, &expiresAt)

	if err == sql.ErrNoRows {
		return "", ErrSessionNotFound
	}
	if err != nil {
		return "", fmt.Errorf("query session: %w", err)
	}

	expires := ParseDBTime(expiresAt)
	if UTCNow().After(expires) {
		_ = db.DeleteSession(sessionID)
		return "", ErrSessionExpired
	}

	return adminID, nil
}

// DeleteSession removes a session.
func (db *DB) DeleteSession(sessionID string) error {
	_, err := db.Exec("DELETE FROM admin_session WHERE id = ?", sessionID)
	return err
}

// CleanExpiredSessions removes all expired sessions.
func (db *DB) CleanExpiredSessions() error {
	_, err := db.Exec("DELETE FROM admin_session WHERE expires_at < datetime('now')")
	return err
}
