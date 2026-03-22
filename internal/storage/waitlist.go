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
	"log/slog"
	"time"
)

// WaitlistEntry represents a queued registration.
type WaitlistEntry struct {
	ID                string
	Email             string
	Name              string
	PasswordHash      string
	InvitationTokenID string
	UserType          string
	Status            string // waiting, notified, expired, created
	NotifiedAt        *time.Time
	ExpiresAt         *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// AddToWaitlist queues a registration for when capacity becomes available.
func (db *DB) AddToWaitlist(email, name, passwordHash, invTokenID, userType string) (*WaitlistEntry, error) {
	id := GenerateID("wl")
	now := UTCNow()

	var invTokenPtr *string
	if invTokenID != "" {
		invTokenPtr = &invTokenID
	}

	_, err := db.Exec(
		`INSERT INTO waitlist (id, email, name, password_hash, invitation_token_id, user_type, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, 'waiting', ?, ?)`,
		id, email, name, passwordHash, invTokenPtr, userType, now.Format(time.RFC3339), now.Format(time.RFC3339),
	)
	if err != nil {
		return nil, fmt.Errorf("add to waitlist: %w", err)
	}

	return &WaitlistEntry{
		ID:                id,
		Email:             email,
		Name:              name,
		PasswordHash:      passwordHash,
		InvitationTokenID: invTokenID,
		UserType:          userType,
		Status:            "waiting",
		CreatedAt:         now,
		UpdatedAt:         now,
	}, nil
}

// GetWaitlistPosition returns the 1-based position of an email in the waitlist.
// Returns 0 if the email is not on the waitlist.
func (db *DB) GetWaitlistPosition(email string) int {
	var position int
	err := db.QueryRow(
		`SELECT COUNT(*) FROM waitlist
		 WHERE status = 'waiting' AND created_at <= (
			SELECT created_at FROM waitlist WHERE email = ? AND status = 'waiting'
		 )`,
		email,
	).Scan(&position)
	if err != nil {
		return 0
	}
	return position
}

// CountWaitlist returns the number of waiting entries.
func (db *DB) CountWaitlist() int {
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM waitlist WHERE status = 'waiting'`).Scan(&count); err != nil {
		slog.Warn("Failed to count waitlist entries", "error", err)
	}
	return count
}

// IsEmailOnWaitlist checks if an email is already on the waitlist (any status).
func (db *DB) IsEmailOnWaitlist(email string) bool {
	var count int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM waitlist WHERE email = ? AND status IN ('waiting', 'notified')`,
		email,
	).Scan(&count); err != nil {
		slog.Warn("Failed to check waitlist status", "email", email, "error", err)
	}
	return count > 0
}

// PopWaitlist returns the oldest waiting entries up to limit and marks them as notified.
func (db *DB) PopWaitlist(limit int, notificationWindowDays int) ([]*WaitlistEntry, error) {
	now := UTCNow()
	expiresAt := now.Add(time.Duration(notificationWindowDays) * 24 * time.Hour)

	rows, err := db.Query(
		`SELECT id, email, name, password_hash, COALESCE(invitation_token_id, ''), user_type, created_at
		 FROM waitlist WHERE status = 'waiting'
		 ORDER BY created_at ASC LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("pop waitlist: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var entries []*WaitlistEntry
	for rows.Next() {
		var e WaitlistEntry
		var createdAtStr string
		if err := rows.Scan(&e.ID, &e.Email, &e.Name, &e.PasswordHash, &e.InvitationTokenID, &e.UserType, &createdAtStr); err != nil {
			continue
		}
		e.CreatedAt = ParseDBTime(createdAtStr)
		e.NotifiedAt = &now
		e.ExpiresAt = &expiresAt
		e.Status = "notified"
		entries = append(entries, &e)
	}

	// Mark them as notified
	for _, e := range entries {
		_, _ = db.Exec(
			`UPDATE waitlist SET status = 'notified', notified_at = ?, expires_at = ?, updated_at = ?
			 WHERE id = ?`,
			now.Format(time.RFC3339), expiresAt.Format(time.RFC3339), now.Format(time.RFC3339), e.ID,
		)
	}

	return entries, nil
}

// MarkWaitlistCreated marks a waitlist entry as successfully registered.
func (db *DB) MarkWaitlistCreated(id string) error {
	_, err := db.Exec(
		`UPDATE waitlist SET status = 'created', updated_at = ? WHERE id = ?`,
		UTCNow().Format(time.RFC3339), id,
	)
	return err
}

// ExpireWaitlistNotifications marks notified entries as expired if past their deadline.
func (db *DB) ExpireWaitlistNotifications() (int, error) {
	now := UTCNow().Format(time.RFC3339)
	result, err := db.Exec(
		`UPDATE waitlist SET status = 'expired', updated_at = ?
		 WHERE status = 'notified' AND expires_at IS NOT NULL AND expires_at < ?`,
		now, now,
	)
	if err != nil {
		return 0, err
	}
	n, _ := result.RowsAffected()
	return int(n), nil
}

// ListWaitlist returns all waitlist entries for admin viewing.
func (db *DB) ListWaitlist(status string, limit, offset int) ([]*WaitlistEntry, error) {
	query := `SELECT id, email, name, COALESCE(invitation_token_id, ''), user_type, status,
		 notified_at, expires_at, created_at, updated_at
		 FROM waitlist`
	var args []interface{}

	if status != "" {
		query += " WHERE status = ?"
		args = append(args, status)
	}
	query += " ORDER BY created_at ASC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var entries []*WaitlistEntry
	for rows.Next() {
		e := &WaitlistEntry{}
		var invTokenID string
		var notifiedAt, expiresAt, createdAt, updatedAt sql.NullString
		if err := rows.Scan(&e.ID, &e.Email, &e.Name, &invTokenID, &e.UserType, &e.Status,
			&notifiedAt, &expiresAt, &createdAt, &updatedAt); err != nil {
			continue
		}
		e.InvitationTokenID = invTokenID
		if notifiedAt.Valid {
			t := ParseDBTime(notifiedAt.String)
			e.NotifiedAt = &t
		}
		if expiresAt.Valid {
			t := ParseDBTime(expiresAt.String)
			e.ExpiresAt = &t
		}
		if createdAt.Valid {
			e.CreatedAt = ParseDBTime(createdAt.String)
		}
		if updatedAt.Valid {
			e.UpdatedAt = ParseDBTime(updatedAt.String)
		}
		entries = append(entries, e)
	}
	return entries, nil
}

// RemoveFromWaitlist deletes a waitlist entry (admin action).
func (db *DB) RemoveFromWaitlist(id string) error {
	_, err := db.Exec(`DELETE FROM waitlist WHERE id = ?`, id)
	return err
}
