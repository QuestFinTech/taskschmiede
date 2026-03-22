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
	"time"

	"golang.org/x/crypto/bcrypt"
)

// GetUserTOTP returns the TOTP state for a user.
func (db *DB) GetUserTOTP(userID string) (*UserTOTP, error) {
	var secret sql.NullString
	var enabledAt sql.NullString

	err := db.QueryRow(
		`SELECT totp_secret, totp_enabled_at FROM user WHERE id = ?`,
		userID,
	).Scan(&secret, &enabledAt)
	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get user totp: %w", err)
	}

	result := &UserTOTP{UserID: userID}
	if secret.Valid {
		result.Secret = secret.String
	}
	if enabledAt.Valid {
		t := ParseDBTime(enabledAt.String)
		result.EnabledAt = &t
	}
	return result, nil
}

// SetUserTOTPSecret stores the TOTP secret for a user (setup phase, not yet enabled).
func (db *DB) SetUserTOTPSecret(userID, secret string) error {
	_, err := db.Exec(
		`UPDATE user SET totp_secret = ? WHERE id = ?`,
		secret, userID,
	)
	if err != nil {
		return fmt.Errorf("set totp secret: %w", err)
	}
	return nil
}

// EnableUserTOTP marks TOTP as enabled for a user.
func (db *DB) EnableUserTOTP(userID string) error {
	now := UTCNow()
	_, err := db.Exec(
		`UPDATE user SET totp_enabled_at = ? WHERE id = ?`,
		now.Format(time.RFC3339), userID,
	)
	if err != nil {
		return fmt.Errorf("enable totp: %w", err)
	}
	return nil
}

// DisableUserTOTP clears TOTP secret and enabled timestamp, and deletes recovery codes.
func (db *DB) DisableUserTOTP(userID string) error {
	_, err := db.Exec(
		`UPDATE user SET totp_secret = NULL, totp_enabled_at = NULL WHERE id = ?`,
		userID,
	)
	if err != nil {
		return fmt.Errorf("disable totp: %w", err)
	}

	_, err = db.Exec(`DELETE FROM totp_recovery WHERE user_id = ?`, userID)
	if err != nil {
		return fmt.Errorf("delete recovery codes: %w", err)
	}
	return nil
}

// CreateTOTPRecoveryCodes generates and stores recovery codes for a user.
// Returns the plaintext codes (displayed once to the user).
func (db *DB) CreateTOTPRecoveryCodes(userID string, count int) ([]string, error) {
	// Delete existing codes first
	_, err := db.Exec(`DELETE FROM totp_recovery WHERE user_id = ?`, userID)
	if err != nil {
		return nil, fmt.Errorf("delete old recovery codes: %w", err)
	}

	codes := make([]string, count)
	now := UTCNow().Format(time.RFC3339)

	for i := 0; i < count; i++ {
		// Generate 8-character alphanumeric code
		b := make([]byte, 4)
		mustRandBytes(b)
		code := fmt.Sprintf("%x", b) // 8 hex characters

		hash, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
		if err != nil {
			return nil, fmt.Errorf("hash recovery code: %w", err)
		}

		id := generateID("trc")
		_, err = db.Exec(
			`INSERT INTO totp_recovery (id, user_id, code_hash, created_at) VALUES (?, ?, ?, ?)`,
			id, userID, string(hash), now,
		)
		if err != nil {
			return nil, fmt.Errorf("insert recovery code: %w", err)
		}

		codes[i] = code
	}

	return codes, nil
}

// VerifyTOTPRecoveryCode checks a recovery code and marks it as used if valid.
// Returns true if the code was valid and unused.
func (db *DB) VerifyTOTPRecoveryCode(userID, code string) (bool, error) {
	rows, err := db.Query(
		`SELECT id, code_hash FROM totp_recovery WHERE user_id = ? AND used_at IS NULL`,
		userID,
	)
	if err != nil {
		return false, fmt.Errorf("query recovery codes: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var id, codeHash string
		if err := rows.Scan(&id, &codeHash); err != nil {
			return false, fmt.Errorf("scan recovery code: %w", err)
		}

		if bcrypt.CompareHashAndPassword([]byte(codeHash), []byte(code)) == nil {
			// Mark as used
			now := UTCNow().Format(time.RFC3339)
			_, err = db.Exec(`UPDATE totp_recovery SET used_at = ? WHERE id = ?`, now, id)
			if err != nil {
				return false, fmt.Errorf("mark recovery code used: %w", err)
			}
			return true, nil
		}
	}

	return false, nil
}

// CountUnusedRecoveryCodes returns the number of unused recovery codes for a user.
func (db *DB) CountUnusedRecoveryCodes(userID string) (int, error) {
	var count int
	err := db.QueryRow(
		`SELECT COUNT(*) FROM totp_recovery WHERE user_id = ? AND used_at IS NULL`,
		userID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count recovery codes: %w", err)
	}
	return count, nil
}
