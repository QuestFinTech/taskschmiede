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
	"errors"
	"fmt"
	"time"
)

// ErrRetentionHold is returned when a deletion is blocked by an active legal hold.
var ErrRetentionHold = errors.New("account is under legal hold")

// GetUserRetentionHold returns the retention hold state for a user.
func (db *DB) GetUserRetentionHold(userID string) (*UserRetentionHold, error) {
	var hold int
	var reason, holdAt, holdBy sql.NullString

	err := db.QueryRow(
		`SELECT retention_hold, retention_hold_reason, retention_hold_at, retention_hold_by
		 FROM user WHERE id = ?`,
		userID,
	).Scan(&hold, &reason, &holdAt, &holdBy)
	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get retention hold: %w", err)
	}

	result := &UserRetentionHold{
		UserID: userID,
		Active: hold == 1,
	}
	if reason.Valid {
		result.Reason = reason.String
	}
	if holdAt.Valid {
		t := ParseDBTime(holdAt.String)
		result.HoldAt = &t
	}
	if holdBy.Valid {
		result.HoldBy = holdBy.String
	}
	return result, nil
}

// SetRetentionHold places or removes a legal hold on a user account.
func (db *DB) SetRetentionHold(userID string, active bool, reason, adminUserID string) error {
	now := UTCNow()
	if active {
		_, err := db.Exec(
			`UPDATE user SET retention_hold = 1, retention_hold_reason = ?,
			 retention_hold_at = ?, retention_hold_by = ? WHERE id = ?`,
			reason, now.Format(time.RFC3339), adminUserID, userID,
		)
		if err != nil {
			return fmt.Errorf("set retention hold: %w", err)
		}
	} else {
		_, err := db.Exec(
			`UPDATE user SET retention_hold = 0, retention_hold_reason = NULL,
			 retention_hold_at = NULL, retention_hold_by = NULL WHERE id = ?`,
			userID,
		)
		if err != nil {
			return fmt.Errorf("clear retention hold: %w", err)
		}
	}
	return nil
}

// GetUserDeletionRequest returns the deletion request state for a user.
func (db *DB) GetUserDeletionRequest(userID string) (*UserDeletionRequest, error) {
	var requestedAt, scheduledAt sql.NullString

	err := db.QueryRow(
		`SELECT deletion_requested_at, deletion_scheduled_at FROM user WHERE id = ?`,
		userID,
	).Scan(&requestedAt, &scheduledAt)
	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get deletion request: %w", err)
	}

	result := &UserDeletionRequest{UserID: userID}
	if requestedAt.Valid {
		t := ParseDBTime(requestedAt.String)
		result.RequestedAt = &t
	}
	if scheduledAt.Valid {
		t := ParseDBTime(scheduledAt.String)
		result.ScheduledAt = &t
	}
	return result, nil
}

// RequestDeletion records a deletion request and schedules the deletion.
// Returns ErrRetentionHold if the account is under legal hold.
func (db *DB) RequestDeletion(userID string, graceDays int) error {
	// Check for legal hold
	hold, err := db.GetUserRetentionHold(userID)
	if err != nil {
		return err
	}
	if hold.Active {
		return ErrRetentionHold
	}

	now := UTCNow()
	scheduled := now.AddDate(0, 0, graceDays)

	_, err = db.Exec(
		`UPDATE user SET deletion_requested_at = ?, deletion_scheduled_at = ?, status = 'suspended'
		 WHERE id = ?`,
		now.Format(time.RFC3339), scheduled.Format(time.RFC3339), userID,
	)
	if err != nil {
		return fmt.Errorf("request deletion: %w", err)
	}

	// Revoke all tokens
	return db.RevokeAllUserTokens(userID)
}

// CancelDeletion cancels a pending deletion request and reactivates the account.
func (db *DB) CancelDeletion(userID string) error {
	_, err := db.Exec(
		`UPDATE user SET deletion_requested_at = NULL, deletion_scheduled_at = NULL, status = 'active'
		 WHERE id = ?`,
		userID,
	)
	if err != nil {
		return fmt.Errorf("cancel deletion: %w", err)
	}
	return nil
}

// ListScheduledDeletions returns users whose deletion is past due.
func (db *DB) ListScheduledDeletions() ([]*UserDeletionRequest, error) {
	now := UTCNow().Format(time.RFC3339)
	rows, err := db.Query(
		`SELECT id, deletion_requested_at, deletion_scheduled_at FROM user
		 WHERE deletion_scheduled_at IS NOT NULL AND deletion_scheduled_at <= ?
		   AND retention_hold = 0 AND status != 'deleted'`,
		now,
	)
	if err != nil {
		return nil, fmt.Errorf("list scheduled deletions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var results []*UserDeletionRequest
	for rows.Next() {
		var userID string
		var requestedAt, scheduledAt sql.NullString
		if err := rows.Scan(&userID, &requestedAt, &scheduledAt); err != nil {
			return nil, fmt.Errorf("scan deletion: %w", err)
		}
		req := &UserDeletionRequest{UserID: userID}
		if requestedAt.Valid {
			t := ParseDBTime(requestedAt.String)
			req.RequestedAt = &t
		}
		if scheduledAt.Valid {
			t := ParseDBTime(scheduledAt.String)
			req.ScheduledAt = &t
		}
		results = append(results, req)
	}
	return results, nil
}

// ExecuteDeletion anonymizes user data and deletes PII.
func (db *DB) ExecuteDeletion(userID string) error {
	// Delete person record (PII)
	_, _ = db.Exec(`DELETE FROM person WHERE user_id = ?`, userID)

	// Delete addresses linked to this person via entity_relation
	rows, err := db.Query(
		`SELECT target_entity_id FROM entity_relation
		 WHERE source_entity_type = 'person' AND relationship_type = 'has_address'
		   AND source_entity_id IN (SELECT id FROM person WHERE user_id = ?)`,
		userID,
	)
	if err == nil {
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var addrID string
			if rows.Scan(&addrID) == nil {
				_, _ = db.Exec(`DELETE FROM address WHERE id = ?`, addrID)
			}
		}
	}

	// Delete TOTP data
	_ = db.DisableUserTOTP(userID)

	// Delete tokens
	_, _ = db.Exec(`DELETE FROM token WHERE user_id = ?`, userID)

	// Delete recovery codes (already handled by DisableUserTOTP, but be safe)
	_, _ = db.Exec(`DELETE FROM totp_recovery WHERE user_id = ?`, userID)

	// Anonymize user record (keep for referential integrity)
	_, err = db.Exec(
		`UPDATE user SET
		 name = 'Deleted User', email = NULL, password_hash = NULL,
		 totp_secret = NULL, totp_enabled_at = NULL,
		 retention_hold = 0, retention_hold_reason = NULL,
		 retention_hold_at = NULL, retention_hold_by = NULL,
		 metadata = '{}', status = 'deleted'
		 WHERE id = ?`,
		userID,
	)
	if err != nil {
		return fmt.Errorf("anonymize user: %w", err)
	}

	return nil
}
