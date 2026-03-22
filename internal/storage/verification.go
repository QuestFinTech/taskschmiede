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
	"database/sql"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// PendingAdmin represents a registration awaiting email verification.
type PendingAdmin struct {
	ID               string
	Email            string
	Name             string
	VerificationCode string
	ExpiresAt        time.Time
	CreatedAt        time.Time
}

// PasswordReset represents a password reset request.
type PasswordReset struct {
	ID        string
	Email     string
	Code      string
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}

// generateCode creates a verification code in format xxx-xxx-xxx (lowercase alphanumeric).
func generateCode() string {
	const chars = "0123456789abcdefghijklmnopqrstuvwxyz"
	code := make([]byte, 11) // xxx-xxx-xxx = 11 chars

	bytes := make([]byte, 9)
	mustRandBytes(bytes)

	idx := 0
	for i := 0; i < 11; i++ {
		if i == 3 || i == 7 {
			code[i] = '-'
		} else {
			code[i] = chars[bytes[idx]%byte(len(chars))]
			idx++
		}
	}

	return string(code)
}

// CreatePendingAdmin creates a pending admin registration with verification code.
func (db *DB) CreatePendingAdmin(email, name, password string, timeout time.Duration) (*PendingAdmin, error) {
	// Check if master admin already exists
	hasAdmin, err := db.HasMasterAdmin()
	if err != nil {
		return nil, err
	}
	if hasAdmin {
		return nil, ErrAdminExists
	}

	// Hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	code := generateCode()
	now := UTCNow()
	expiresAt := now.Add(timeout)

	// Delete any existing pending registration first
	_, _ = db.Exec("DELETE FROM pending_admin")

	_, err = db.Exec(
		`INSERT INTO pending_admin (id, email, name, password_hash, verification_code, expires_at)
		 VALUES ('pending', ?, ?, ?, ?, ?)`,
		email, name, string(hash), code, expiresAt.Format(time.RFC3339),
	)
	if err != nil {
		return nil, fmt.Errorf("insert pending admin: %w", err)
	}

	return &PendingAdmin{
		ID:               "pending",
		Email:            email,
		Name:             name,
		VerificationCode: code,
		ExpiresAt:        expiresAt,
		CreatedAt:        now,
	}, nil
}

// GetPendingAdmin retrieves the pending admin registration.
func (db *DB) GetPendingAdmin() (*PendingAdmin, error) {
	var p PendingAdmin
	var expiresAt, createdAt string

	err := db.QueryRow(
		`SELECT id, email, name, verification_code, expires_at, created_at
		 FROM pending_admin WHERE id = 'pending'`,
	).Scan(&p.ID, &p.Email, &p.Name, &p.VerificationCode, &expiresAt, &createdAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query pending admin: %w", err)
	}

	p.ExpiresAt = ParseDBTime(expiresAt)
	p.CreatedAt = ParseDBTime(createdAt)

	return &p, nil
}

// VerifyAndActivateAdmin verifies the code and creates the admin user.
func (db *DB) VerifyAndActivateAdmin(code string) error {
	code = strings.ToLower(strings.TrimSpace(code))

	// Get pending admin
	var email, name, passwordHash, expiresAt string
	var storedCode string

	err := db.QueryRow(
		`SELECT email, name, password_hash, verification_code, expires_at
		 FROM pending_admin WHERE id = 'pending'`,
	).Scan(&email, &name, &passwordHash, &storedCode, &expiresAt)

	if err == sql.ErrNoRows {
		return fmt.Errorf("no pending registration found")
	}
	if err != nil {
		return fmt.Errorf("query pending admin: %w", err)
	}

	// Check code
	if storedCode != code {
		return fmt.Errorf("invalid verification code")
	}

	// Check expiry
	expires := ParseDBTime(expiresAt)
	if UTCNow().After(expires) {
		return fmt.Errorf("verification code has expired")
	}

	// Create or promote admin user in the user table.
	// The user may already exist if they registered via the REST API before
	// the setup wizard was completed.
	var userID string
	defaultTier := db.DefaultTierID()

	var existingID, existingResourceID sql.NullString
	err = db.QueryRow(
		`SELECT id, resource_id FROM user WHERE email = ?`, email,
	).Scan(&existingID, &existingResourceID)

	if err == sql.ErrNoRows {
		// No existing user -- create a new one.
		userID = generateID("usr")
		_, err = db.Exec(
			`INSERT INTO user (id, name, email, password_hash, is_admin, tier, user_type, status, metadata)
			 VALUES (?, ?, ?, ?, 1, ?, 'human', 'active', '{}')`,
			userID, name, email, passwordHash, defaultTier,
		)
		if err != nil {
			return fmt.Errorf("create admin user: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("check existing user: %w", err)
	} else {
		// User exists -- promote to admin and update credentials.
		userID = existingID.String
		_, err = db.Exec(
			`UPDATE user SET is_admin = 1, name = ?, password_hash = ?, status = 'active' WHERE id = ?`,
			name, passwordHash, userID,
		)
		if err != nil {
			return fmt.Errorf("promote existing user to admin: %w", err)
		}
	}

	// Create a resource for the admin if they don't already have one.
	if !existingResourceID.Valid || existingResourceID.String == "" {
		resMeta := map[string]interface{}{
			"delivery": map[string]interface{}{
				"email":    email,
				"channels": []string{"email"},
			},
		}
		res, resErr := db.CreateResource("human", name, "", nil, nil, resMeta)
		if resErr != nil {
			return fmt.Errorf("create resource for admin: %w", resErr)
		}
		_, err = db.Exec(`UPDATE user SET resource_id = ? WHERE id = ?`, res.ID, userID)
		if err != nil {
			return fmt.Errorf("link resource to admin: %w", err)
		}
	}

	// Create Person record if one does not already exist.
	// Split the combined name into first/last for the Person record.
	_, personErr := db.GetPersonByUserID(userID)
	if personErr == ErrPersonNotFound {
		parts := strings.SplitN(name, " ", 2)
		firstName := parts[0]
		lastName := ""
		if len(parts) > 1 {
			lastName = parts[1]
		}
		_, personErr = db.CreatePerson(userID, firstName, lastName, "private", "", "")
	}
	// Log but don't fail setup if Person creation fails.
	if personErr != nil && personErr != ErrPersonNotFound {
		// Non-fatal: admin can fix name on the profile page.
		_ = personErr
	}

	// Delete pending registration
	_, _ = db.Exec("DELETE FROM pending_admin WHERE id = 'pending'")

	return nil
}

// DeletePendingAdmin removes the pending registration.
func (db *DB) DeletePendingAdmin() error {
	_, err := db.Exec("DELETE FROM pending_admin WHERE id = 'pending'")
	return err
}

// RegeneratePendingAdminCode creates a new verification code for existing pending admin.
func (db *DB) RegeneratePendingAdminCode(timeout time.Duration) (*PendingAdmin, error) {
	// Get existing pending admin
	pending, err := db.GetPendingAdmin()
	if err != nil {
		return nil, err
	}
	if pending == nil {
		return nil, fmt.Errorf("no pending registration found")
	}

	// Generate new code
	code := generateCode()
	expiresAt := UTCNow().Add(timeout)

	// Update the code and expiry
	_, err = db.Exec(
		`UPDATE pending_admin SET verification_code = ?, expires_at = ? WHERE id = 'pending'`,
		code, expiresAt.Format(time.RFC3339),
	)
	if err != nil {
		return nil, fmt.Errorf("update verification code: %w", err)
	}

	pending.VerificationCode = code
	pending.ExpiresAt = expiresAt

	return pending, nil
}

// CreatePasswordReset creates a password reset request.
func (db *DB) CreatePasswordReset(email string, timeout time.Duration) (*PasswordReset, error) {
	// Check if the email exists in the user table (active users with password auth only).
	var exists int
	err := db.QueryRow(
		"SELECT COUNT(*) FROM user WHERE email = ? AND status = 'active' AND password_hash IS NOT NULL AND password_hash != ''",
		email,
	).Scan(&exists)
	if err != nil {
		return nil, fmt.Errorf("check user email: %w", err)
	}
	if exists == 0 {
		// Don't reveal if email exists - return success but don't create reset
		return nil, nil
	}

	// Generate ID and code
	idBytes := make([]byte, 16)
	mustRandBytes(idBytes)
	id := fmt.Sprintf("%x", idBytes)

	code := generateCode()
	now := UTCNow()
	expiresAt := now.Add(timeout)

	// Delete any existing reset for this email
	_, _ = db.Exec("DELETE FROM password_reset WHERE email = ? AND used_at IS NULL", email)

	_, err = db.Exec(
		`INSERT INTO password_reset (id, email, code, expires_at)
		 VALUES (?, ?, ?, ?)`,
		id, email, code, expiresAt.Format(time.RFC3339),
	)
	if err != nil {
		return nil, fmt.Errorf("insert password reset: %w", err)
	}

	return &PasswordReset{
		ID:        id,
		Email:     email,
		Code:      code,
		ExpiresAt: expiresAt,
		CreatedAt: now,
	}, nil
}

// ValidatePasswordReset checks if a reset code is valid.
func (db *DB) ValidatePasswordReset(email, code string) (*PasswordReset, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	code = strings.ToLower(strings.TrimSpace(code))

	var r PasswordReset
	var expiresAt, createdAt string
	var usedAt sql.NullString

	err := db.QueryRow(
		`SELECT id, email, code, expires_at, used_at, created_at
		 FROM password_reset WHERE email = ? AND code = ?`,
		email, code,
	).Scan(&r.ID, &r.Email, &r.Code, &expiresAt, &usedAt, &createdAt)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("invalid reset code")
	}
	if err != nil {
		return nil, fmt.Errorf("query password reset: %w", err)
	}

	r.ExpiresAt = ParseDBTime(expiresAt)
	r.CreatedAt = ParseDBTime(createdAt)

	if usedAt.Valid {
		return nil, fmt.Errorf("reset code has already been used")
	}

	if UTCNow().After(r.ExpiresAt) {
		return nil, fmt.Errorf("reset code has expired")
	}

	return &r, nil
}

// CompletePasswordReset changes the password and marks the reset as used.
func (db *DB) CompletePasswordReset(email, code, newPassword string) error {
	// Validate the reset first
	reset, err := db.ValidatePasswordReset(email, code)
	if err != nil {
		return err
	}

	// Hash new password
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	// Update user password
	result, err := db.Exec(
		"UPDATE user SET password_hash = ?, updated_at = datetime('now') WHERE email = ? AND status = 'active' AND password_hash IS NOT NULL AND password_hash != ''",
		string(hash), email,
	)
	if err != nil {
		return fmt.Errorf("update user password: %w", err)
	}
	userRows, _ := result.RowsAffected()
	if userRows == 0 {
		return fmt.Errorf("no account found for email")
	}

	// Revoke all tokens for this user to invalidate existing sessions.
	var userID string
	if err := db.QueryRow("SELECT id FROM user WHERE email = ?", email).Scan(&userID); err == nil {
		if _, err := db.Exec(
			"UPDATE token SET revoked_at = datetime('now') WHERE user_id = ? AND revoked_at IS NULL",
			userID,
		); err != nil {
			return fmt.Errorf("revoke user tokens: %w", err)
		}
	}

	// Mark reset as used
	_, _ = db.Exec(
		"UPDATE password_reset SET used_at = datetime('now') WHERE id = ?",
		reset.ID,
	)

	return nil
}

// PendingEmailChange represents a pending email address change awaiting verification.
type PendingEmailChange struct {
	ID               string
	UserID           string
	NewEmail         string
	VerificationCode string
	ExpiresAt        time.Time
}

// CreatePendingEmailChange creates a pending email change request.
// Deletes any previous pending change for the same user.
func (db *DB) CreatePendingEmailChange(userID, newEmail string, timeout time.Duration) (*PendingEmailChange, error) {
	// Check if new email is already taken.
	var exists int
	if err := db.QueryRow("SELECT COUNT(*) FROM user WHERE email = ?", newEmail).Scan(&exists); err != nil {
		return nil, fmt.Errorf("check email: %w", err)
	}
	if exists > 0 {
		return nil, fmt.Errorf("email already in use")
	}

	// Delete any existing pending change for this user.
	_, _ = db.Exec("DELETE FROM pending_email_change WHERE user_id = ?", userID)

	id := generateID("pec")
	code := generateCode()
	now := UTCNow()
	expiresAt := now.Add(timeout)

	_, err := db.Exec(
		`INSERT INTO pending_email_change (id, user_id, new_email, verification_code, expires_at)
		 VALUES (?, ?, ?, ?, ?)`,
		id, userID, newEmail, code, expiresAt.Format(time.RFC3339),
	)
	if err != nil {
		return nil, fmt.Errorf("insert pending email change: %w", err)
	}

	return &PendingEmailChange{
		ID:               id,
		UserID:           userID,
		NewEmail:         newEmail,
		VerificationCode: code,
		ExpiresAt:        expiresAt,
	}, nil
}

// VerifyAndApplyEmailChange verifies the code and updates the user's email.
func (db *DB) VerifyAndApplyEmailChange(userID, code string) (string, error) {
	var pec PendingEmailChange
	var expiresAtStr string
	err := db.QueryRow(
		`SELECT id, user_id, new_email, verification_code, expires_at
		 FROM pending_email_change WHERE user_id = ? AND verification_code = ?`,
		userID, strings.TrimSpace(strings.ToLower(code)),
	).Scan(&pec.ID, &pec.UserID, &pec.NewEmail, &pec.VerificationCode, &expiresAtStr)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("invalid verification code")
		}
		return "", fmt.Errorf("query pending email change: %w", err)
	}

	pec.ExpiresAt = ParseDBTime(expiresAtStr)
	if UTCNow().After(pec.ExpiresAt) {
		_, _ = db.Exec("DELETE FROM pending_email_change WHERE id = ?", pec.ID)
		return "", fmt.Errorf("verification code expired")
	}

	// Apply the email change.
	_, err = db.Exec(
		"UPDATE user SET email = ?, updated_at = datetime('now') WHERE id = ?",
		pec.NewEmail, userID,
	)
	if err != nil {
		return "", fmt.Errorf("update user email: %w", err)
	}

	// Clean up.
	_, _ = db.Exec("DELETE FROM pending_email_change WHERE id = ?", pec.ID)

	return pec.NewEmail, nil
}

// GetPendingEmailChange returns the pending email change for a user, if any.
func (db *DB) GetPendingEmailChange(userID string) (*PendingEmailChange, error) {
	var pec PendingEmailChange
	var expiresAtStr string
	err := db.QueryRow(
		`SELECT id, user_id, new_email, verification_code, expires_at
		 FROM pending_email_change WHERE user_id = ? AND expires_at > datetime('now')`,
		userID,
	).Scan(&pec.ID, &pec.UserID, &pec.NewEmail, &pec.VerificationCode, &expiresAtStr)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query pending email change: %w", err)
	}
	pec.ExpiresAt = ParseDBTime(expiresAtStr)
	return &pec, nil
}

// CleanExpiredVerifications removes expired pending registrations and password resets.
func (db *DB) CleanExpiredVerifications() error {
	// Clean expired pending admins
	_, _ = db.Exec("DELETE FROM pending_admin WHERE expires_at < datetime('now')")

	// Clean expired and used password resets (keep for audit, clean after 7 days)
	_, _ = db.Exec(`DELETE FROM password_reset
			 WHERE expires_at < datetime('now', '-7 days')
			    OR (used_at IS NOT NULL AND used_at < datetime('now', '-7 days'))`)

	// Clean expired pending email changes
	_, _ = db.Exec("DELETE FROM pending_email_change WHERE expires_at < datetime('now')")

	return nil
}
