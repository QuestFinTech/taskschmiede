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
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// User represents a registered user.
type User struct {
	ID           string
	Name         string
	Email        string
	ResourceID   *string
	ExternalID   *string
	Status       string
	IsAdmin      bool
	LoginCount   int
	Tier         int
	UserType     string
	Lang         string
	Timezone     string
	EmailCopy    bool // receive external email copies of internal messages
	LastActiveAt *time.Time
	Metadata     map[string]interface{}
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// UserTOTP holds TOTP two-factor authentication state for a user.
type UserTOTP struct {
	UserID    string
	Secret    string     // TOTP secret (empty = not enabled)
	EnabledAt *time.Time // when 2FA was activated
}

// UserRetentionHold holds legal hold state for a user.
type UserRetentionHold struct {
	UserID  string
	Active  bool
	Reason  string
	HoldAt  *time.Time
	HoldBy  string // user ID of admin who placed the hold
}

// UserDeletionRequest holds account deletion request state.
type UserDeletionRequest struct {
	UserID      string
	RequestedAt *time.Time
	ScheduledAt *time.Time // RequestedAt + grace period
}

// ListUsersOpts holds filters for listing users.
type ListUsersOpts struct {
	Status         string
	OrganizationID string
	UserType       string
	Search         string
	Limit          int
	Offset         int
}

// UpdateUserFields holds the fields to update on a user.
// Only non-nil fields are applied.
type UpdateUserFields struct {
	Name       *string
	Email      *string
	Status     *string
	ResourceID *string
	ExternalID *string
	Tier       *int
	UserType   *string
	Lang       *string
	Timezone   *string
	EmailCopy  *bool
	IsAdmin    *bool
	Metadata   map[string]interface{}
}

// TierName is available as a method on *DB -- see tier.go.

// ErrUserNotFound is returned when a user cannot be found by their ID.
var ErrUserNotFound = errors.New("user not found")

// GetUser retrieves a user by ID.
func (db *DB) GetUser(id string) (*User, error) {
	var user User
	var email, resourceID, externalID, lastActiveAt sql.NullString
	var metadataJSON string
	var createdAt, updatedAt string

	err := db.QueryRow(
		`SELECT id, name, email, resource_id, external_id, status, is_admin, login_count, tier, user_type, lang, timezone, email_copy, last_active_at, metadata, created_at, updated_at
		 FROM user WHERE id = ?`,
		id,
	).Scan(&user.ID, &user.Name, &email, &resourceID, &externalID, &user.Status, &user.IsAdmin, &user.LoginCount, &user.Tier, &user.UserType, &user.Lang, &user.Timezone, &user.EmailCopy, &lastActiveAt, &metadataJSON, &createdAt, &updatedAt)

	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}

	if email.Valid {
		user.Email = email.String
	}
	if resourceID.Valid {
		user.ResourceID = &resourceID.String
	}
	if externalID.Valid {
		user.ExternalID = &externalID.String
	}
	if lastActiveAt.Valid {
		t := ParseDBTime(lastActiveAt.String)
		user.LastActiveAt = &t
	}

	_ = json.Unmarshal([]byte(metadataJSON), &user.Metadata)
	user.CreatedAt = ParseDBTime(createdAt)
	user.UpdatedAt = ParseDBTime(updatedAt)

	return &user, nil
}

// GetUserByResourceID retrieves a user by their linked resource ID.
// Returns ErrUserNotFound if no user has the given resource_id.
func (db *DB) GetUserByResourceID(resourceID string) (*User, error) {
	var user User
	var email, resID, externalID, lastActiveAt sql.NullString
	var metadataJSON string
	var createdAt, updatedAt string

	err := db.QueryRow(
		`SELECT id, name, email, resource_id, external_id, status, is_admin, login_count, tier, user_type, lang, timezone, email_copy, last_active_at, metadata, created_at, updated_at
		 FROM user WHERE resource_id = ?`,
		resourceID,
	).Scan(&user.ID, &user.Name, &email, &resID, &externalID, &user.Status, &user.IsAdmin, &user.LoginCount, &user.Tier, &user.UserType, &user.Lang, &user.Timezone, &user.EmailCopy, &lastActiveAt, &metadataJSON, &createdAt, &updatedAt)

	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get user by resource_id: %w", err)
	}

	if email.Valid {
		user.Email = email.String
	}
	if resID.Valid {
		user.ResourceID = &resID.String
	}
	if externalID.Valid {
		user.ExternalID = &externalID.String
	}
	if lastActiveAt.Valid {
		t := ParseDBTime(lastActiveAt.String)
		user.LastActiveAt = &t
	}

	_ = json.Unmarshal([]byte(metadataJSON), &user.Metadata)
	user.CreatedAt = ParseDBTime(createdAt)
	user.UpdatedAt = ParseDBTime(updatedAt)

	return &user, nil
}

// ListUsers queries users with filters.
func (db *DB) ListUsers(opts ListUsersOpts) ([]*User, int, error) {
	query := `SELECT u.id, u.name, u.email, u.resource_id, u.external_id, u.status, u.is_admin, u.login_count, u.tier, u.user_type, u.lang, u.timezone, u.email_copy, u.last_active_at, u.metadata, u.created_at, u.updated_at
		  FROM user u`
	countQuery := `SELECT COUNT(*) FROM user u`

	var conditions []string
	var params []interface{}
	var countConditions []string
	var countParams []interface{}

	if opts.OrganizationID != "" {
		join := ` JOIN entity_relation er ON u.resource_id = er.target_entity_id
		    AND er.target_entity_type = 'resource'
		    AND er.source_entity_type = 'organization'
		    AND er.relationship_type = 'has_member'`
		query += join
		countQuery += join
		conditions = append(conditions, `er.source_entity_id = ?`)
		countConditions = append(countConditions, `er.source_entity_id = ?`)
		params = append(params, opts.OrganizationID)
		countParams = append(countParams, opts.OrganizationID)
	}

	if opts.Status != "" {
		conditions = append(conditions, `u.status = ?`)
		countConditions = append(countConditions, `u.status = ?`)
		params = append(params, opts.Status)
		countParams = append(countParams, opts.Status)
	}

	if opts.UserType != "" {
		conditions = append(conditions, `u.user_type = ?`)
		countConditions = append(countConditions, `u.user_type = ?`)
		params = append(params, opts.UserType)
		countParams = append(countParams, opts.UserType)
	}

	if opts.Search != "" {
		conditions = append(conditions, `(u.name LIKE ? ESCAPE '\' OR u.email LIKE ? ESCAPE '\')`)
		countConditions = append(countConditions, `(u.name LIKE ? ESCAPE '\' OR u.email LIKE ? ESCAPE '\')`)
		searchParam := "%" + escapeLike(opts.Search, '\\') + "%"
		params = append(params, searchParam, searchParam)
		countParams = append(countParams, searchParam, searchParam)
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	if len(countConditions) > 0 {
		countQuery += " WHERE " + strings.Join(countConditions, " AND ")
	}

	var total int
	_ = db.QueryRow(countQuery, countParams...).Scan(&total)

	if opts.Limit <= 0 {
		opts.Limit = 50
	}
	query += ` ORDER BY u.created_at DESC LIMIT ? OFFSET ?`
	params = append(params, opts.Limit, opts.Offset)

	rows, err := db.Query(query, params...)
	if err != nil {
		return nil, 0, fmt.Errorf("query users: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var users []*User
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, user)
	}

	return users, total, nil
}

// CreateUser inserts a new user.
func (db *DB) CreateUser(name, email, passwordHash string, resourceID, externalID *string, tier int, userType, lang string, metadata map[string]interface{}) (*User, error) {
	// Check email uniqueness
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM user WHERE email = ?`, email).Scan(&count); err != nil {
		return nil, fmt.Errorf("check email: %w", err)
	}
	if count > 0 {
		return nil, ErrEmailExists
	}

	if tier <= 0 {
		tier = 1
	}
	if userType == "" {
		userType = "human"
	}
	if lang == "" {
		lang = "en"
	}

	id := generateID("usr")

	metadataJSON := "{}"
	if metadata != nil {
		b, err := json.Marshal(metadata)
		if err != nil {
			return nil, fmt.Errorf("marshal metadata: %w", err)
		}
		metadataJSON = string(b)
	}

	var resID, extID sql.NullString
	if resourceID != nil && *resourceID != "" {
		resID = sql.NullString{String: *resourceID, Valid: true}
	}
	if externalID != nil && *externalID != "" {
		extID = sql.NullString{String: *externalID, Valid: true}
	}

	var pwHash *string
	if passwordHash != "" {
		pwHash = &passwordHash
	}

	_, err := db.Exec(
		`INSERT INTO user (id, name, email, resource_id, external_id, password_hash, tier, user_type, lang, metadata, status)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'active')`,
		id, name, email, resID, extID, pwHash, tier, userType, lang, metadataJSON,
	)
	if err != nil {
		return nil, fmt.Errorf("insert user: %w", err)
	}

	now := UTCNow()
	return &User{
		ID:         id,
		Name:       name,
		Email:      email,
		ResourceID: resourceID,
		ExternalID: externalID,
		Status:     "active",
		Tier:       tier,
		UserType:   userType,
		Lang:       lang,
		Metadata:   metadata,
		CreatedAt:  now,
		UpdatedAt:  now,
	}, nil
}

// UpdateUser applies partial updates to a user.
func (db *DB) UpdateUser(id string, fields UpdateUserFields) ([]string, error) {
	var setClauses []string
	var params []interface{}
	var updatedFields []string

	if fields.Name != nil {
		setClauses = append(setClauses, "name = ?")
		params = append(params, *fields.Name)
		updatedFields = append(updatedFields, "name")
	}
	if fields.Email != nil {
		// Check email uniqueness
		var count int
		if err := db.QueryRow(`SELECT COUNT(*) FROM user WHERE email = ? AND id != ?`, *fields.Email, id).Scan(&count); err != nil {
			return nil, fmt.Errorf("check email: %w", err)
		}
		if count > 0 {
			return nil, ErrEmailExists
		}
		setClauses = append(setClauses, "email = ?")
		params = append(params, *fields.Email)
		updatedFields = append(updatedFields, "email")
	}
	if fields.Status != nil {
		setClauses = append(setClauses, "status = ?")
		params = append(params, *fields.Status)
		updatedFields = append(updatedFields, "status")
	}
	if fields.ResourceID != nil {
		if *fields.ResourceID == "" {
			setClauses = append(setClauses, "resource_id = NULL")
		} else {
			setClauses = append(setClauses, "resource_id = ?")
			params = append(params, *fields.ResourceID)
		}
		updatedFields = append(updatedFields, "resource_id")
	}
	if fields.ExternalID != nil {
		if *fields.ExternalID == "" {
			setClauses = append(setClauses, "external_id = NULL")
		} else {
			setClauses = append(setClauses, "external_id = ?")
			params = append(params, *fields.ExternalID)
		}
		updatedFields = append(updatedFields, "external_id")
	}
	if fields.Tier != nil {
		setClauses = append(setClauses, "tier = ?")
		params = append(params, *fields.Tier)
		updatedFields = append(updatedFields, "tier")
	}
	if fields.UserType != nil {
		setClauses = append(setClauses, "user_type = ?")
		params = append(params, *fields.UserType)
		updatedFields = append(updatedFields, "user_type")
	}
	if fields.Lang != nil {
		setClauses = append(setClauses, "lang = ?")
		params = append(params, *fields.Lang)
		updatedFields = append(updatedFields, "lang")
	}
	if fields.Timezone != nil {
		setClauses = append(setClauses, "timezone = ?")
		params = append(params, *fields.Timezone)
		updatedFields = append(updatedFields, "timezone")
	}
	if fields.EmailCopy != nil {
		setClauses = append(setClauses, "email_copy = ?")
		params = append(params, *fields.EmailCopy)
		updatedFields = append(updatedFields, "email_copy")
	}
	if fields.IsAdmin != nil {
		setClauses = append(setClauses, "is_admin = ?")
		params = append(params, *fields.IsAdmin)
		updatedFields = append(updatedFields, "is_admin")
	}
	if fields.Metadata != nil {
		b, err := json.Marshal(fields.Metadata)
		if err != nil {
			return nil, fmt.Errorf("marshal metadata: %w", err)
		}
		setClauses = append(setClauses, "metadata = ?")
		params = append(params, string(b))
		updatedFields = append(updatedFields, "metadata")
	}

	if len(setClauses) == 0 {
		return nil, fmt.Errorf("no fields to update")
	}

	query := fmt.Sprintf("UPDATE user SET %s WHERE id = ?", strings.Join(setClauses, ", "))
	params = append(params, id)

	result, err := db.Exec(query, params...)
	if err != nil {
		return nil, fmt.Errorf("update user: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return nil, ErrUserNotFound
	}

	return updatedFields, nil
}

// ListAgentsByOwner returns agents registered via invitation tokens created by the given user.
func (db *DB) ListAgentsByOwner(ownerUserID string, limit, offset int) ([]*User, int, error) {
	if limit <= 0 {
		limit = 50
	}

	baseWhere := `WHERE u.user_type = 'agent' AND u.invitation_token_id IN (SELECT id FROM invitation_token WHERE created_by = ?)`

	var total int
	_ = db.QueryRow(`SELECT COUNT(*) FROM user u `+baseWhere, ownerUserID).Scan(&total)

	rows, err := db.Query(
		`SELECT u.id, u.name, u.email, u.resource_id, u.external_id, u.status, u.is_admin, u.login_count, u.tier, u.user_type, u.lang, u.timezone, u.email_copy, u.last_active_at, u.metadata, u.created_at, u.updated_at
		 FROM user u `+baseWhere+` ORDER BY u.created_at DESC LIMIT ? OFFSET ?`,
		ownerUserID, limit, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("query agents by owner: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var users []*User
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan agent: %w", err)
		}
		users = append(users, user)
	}

	return users, total, nil
}

// IsAgentOwnedBy checks if the given agent user was registered via an invitation token
// created by the specified owner user.
func (db *DB) IsAgentOwnedBy(agentUserID, ownerUserID string) (bool, error) {
	var count int
	err := db.QueryRow(
		`SELECT COUNT(*) FROM user u
		 JOIN invitation_token it ON u.invitation_token_id = it.id
		 WHERE u.id = ? AND u.user_type = 'agent' AND it.created_by = ?`,
		agentUserID, ownerUserID,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check agent ownership: %w", err)
	}
	return count > 0, nil
}

// ChangeUserPassword verifies the current password and updates to a new one.
// All existing tokens for the user are revoked on success.
func (db *DB) ChangeUserPassword(userID, currentPassword, newPasswordHash string) error {
	var passwordHash sql.NullString
	err := db.QueryRow(`SELECT password_hash FROM user WHERE id = ? AND status = 'active'`, userID).Scan(&passwordHash)
	if err == sql.ErrNoRows {
		return ErrUserNotFound
	}
	if err != nil {
		return fmt.Errorf("query user password: %w", err)
	}

	if !passwordHash.Valid || passwordHash.String == "" {
		return fmt.Errorf("password authentication not available for this user")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash.String), []byte(currentPassword)); err != nil {
		return ErrInvalidPassword
	}

	_, err = db.Exec(
		`UPDATE user SET password_hash = ?, updated_at = datetime('now') WHERE id = ?`,
		newPasswordHash, userID,
	)
	if err != nil {
		return fmt.Errorf("update user password: %w", err)
	}

	// Revoke all existing tokens to invalidate other sessions
	_, err = db.Exec(
		`UPDATE token SET revoked_at = datetime('now') WHERE user_id = ? AND revoked_at IS NULL`,
		userID,
	)
	if err != nil {
		return fmt.Errorf("revoke user tokens: %w", err)
	}

	return nil
}

// IncrementLoginCount atomically increments the login count for a user.
func (db *DB) IncrementLoginCount(userID string) {
	_, _ = db.Exec(`UPDATE user SET login_count = login_count + 1 WHERE id = ?`, userID)
}

// TouchUserActivity updates last_active_at for the given user. The update is
// debounced: it only writes if the stored value is NULL or older than 1 minute,
// keeping the write rate low even under heavy request traffic.
func (db *DB) TouchUserActivity(userID string) {
	now := UTCNow().Format(time.RFC3339)
	_, _ = db.Exec(
		`UPDATE user SET last_active_at = ?
		 WHERE id = ? AND (last_active_at IS NULL OR last_active_at < datetime(?, '-1 minute'))`,
		now, userID, now,
	)
}

// CountActiveUsers returns the number of users with status 'active'.
func (db *DB) CountActiveUsers() int {
	var count int
	_ = db.QueryRow(`SELECT COUNT(*) FROM user WHERE status = 'active'`).Scan(&count)
	return count
}

// CountUsersByStatus returns a map of status -> count for all users.
func (db *DB) CountUsersByStatus() map[string]int {
	result := map[string]int{}
	rows, err := db.Query(`SELECT status, COUNT(*) FROM user GROUP BY status`)
	if err != nil {
		return result
	}
	defer rows.Close() //nolint:errcheck
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err == nil {
			result[status] = count
		}
	}
	return result
}

// CountUsersByType returns a map of user_type -> count for all users.
func (db *DB) CountUsersByType() map[string]int {
	result := map[string]int{}
	rows, err := db.Query(`SELECT COALESCE(user_type, 'human'), COUNT(*) FROM user GROUP BY COALESCE(user_type, 'human')`)
	if err != nil {
		return result
	}
	defer rows.Close() //nolint:errcheck
	for rows.Next() {
		var userType string
		var count int
		if err := rows.Scan(&userType, &count); err == nil {
			result[userType] = count
		}
	}
	return result
}

// ListInactiveUsersForWarning returns active users whose last_active_at is
// older than warnThreshold but newer than deactivateThreshold, and who have
// not yet been warned (metadata.inactivity_warned_at is absent or empty).
// Excludes admin users.
func (db *DB) ListInactiveUsersForWarning(warnThreshold, deactivateThreshold time.Time) ([]*User, error) {
	warnStr := warnThreshold.Format(time.RFC3339)
	deactivateStr := deactivateThreshold.Format(time.RFC3339)

	rows, err := db.Query(
		`SELECT id, name, email, resource_id, external_id, status, is_admin, login_count, tier, user_type, lang, timezone, email_copy, last_active_at, metadata, created_at, updated_at
		 FROM user
		 WHERE status = 'active'
		   AND is_admin = 0
		   AND last_active_at IS NOT NULL
		   AND last_active_at < ?
		   AND last_active_at >= ?
		   AND (json_extract(metadata, '$.inactivity_warned_at') IS NULL
		        OR json_extract(metadata, '$.inactivity_warned_at') = '')
		 ORDER BY last_active_at ASC`,
		warnStr, deactivateStr,
	)
	if err != nil {
		return nil, fmt.Errorf("query inactive users for warning: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var users []*User
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, user)
	}
	return users, nil
}

// ListInactiveUsersForDeactivation returns active users whose last_active_at
// is older than the deactivate threshold. Excludes admin users.
func (db *DB) ListInactiveUsersForDeactivation(deactivateThreshold time.Time) ([]*User, error) {
	thresholdStr := deactivateThreshold.Format(time.RFC3339)

	rows, err := db.Query(
		`SELECT id, name, email, resource_id, external_id, status, is_admin, login_count, tier, user_type, lang, timezone, email_copy, last_active_at, metadata, created_at, updated_at
		 FROM user
		 WHERE status = 'active'
		   AND is_admin = 0
		   AND last_active_at IS NOT NULL
		   AND last_active_at < ?
		 ORDER BY last_active_at ASC`,
		thresholdStr,
	)
	if err != nil {
		return nil, fmt.Errorf("query inactive users for deactivation: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var users []*User
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, user)
	}
	return users, nil
}

// SetUserInactivityWarned marks a user as warned by setting inactivity_warned_at
// in their metadata. Preserves existing metadata keys.
func (db *DB) SetUserInactivityWarned(userID string, warnedAt time.Time) error {
	warnedStr := warnedAt.Format(time.RFC3339)
	_, err := db.Exec(
		`UPDATE user SET metadata = json_set(COALESCE(metadata, '{}'), '$.inactivity_warned_at', ?),
		                 updated_at = datetime('now')
		 WHERE id = ?`,
		warnedStr, userID,
	)
	if err != nil {
		return fmt.Errorf("set inactivity warned: %w", err)
	}
	return nil
}

// DeactivateUser sets a user's status to 'inactive'.
func (db *DB) DeactivateUser(userID string) error {
	result, err := db.Exec(
		`UPDATE user SET status = 'inactive', updated_at = datetime('now') WHERE id = ? AND status = 'active'`,
		userID,
	)
	if err != nil {
		return fmt.Errorf("deactivate user: %w", err)
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return ErrUserNotFound
	}
	return nil
}

// GetUserLangByEmail returns the language preference for a user identified by email.
// Returns "en" if the user is not found or has no language set.
func (db *DB) GetUserLangByEmail(emailAddr string) string {
	var lang sql.NullString
	err := db.QueryRow("SELECT lang FROM user WHERE email = ? AND status = 'active'", emailAddr).Scan(&lang)
	if err != nil || !lang.Valid || lang.String == "" {
		return "en"
	}
	return lang.String
}

// GetUserNameByEmail returns the display name for an active user, or empty string if not found.
func (db *DB) GetUserNameByEmail(emailAddr string) string {
	var name sql.NullString
	err := db.QueryRow("SELECT name FROM user WHERE email = ? AND status = 'active'", emailAddr).Scan(&name)
	if err != nil || !name.Valid {
		return ""
	}
	return name.String
}

// SuspendUser sets a user's status to "suspended" with a reason stored in metadata.
// No-op if the user is already suspended.
func (db *DB) SuspendUser(userID, reason string) error {
	now := UTCNow().Format(time.RFC3339)
	result, err := db.Exec(
		`UPDATE user SET
			status = 'suspended',
			metadata = json_set(COALESCE(metadata, '{}'), '$.suspended_reason', ?, '$.suspended_at', ?),
			updated_at = ?
		WHERE id = ? AND status != 'suspended'`,
		reason, now, now, userID)
	if err != nil {
		return fmt.Errorf("suspend user: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		// Already suspended or user not found -- not an error for idempotent operations.
		return nil
	}
	return nil
}

// GetAgentOwnerUserID returns the user ID of the human who created the invitation token
// used to register the given agent. Returns empty string if the chain cannot be resolved.
func (db *DB) GetAgentOwnerUserID(agentUserID string) (string, error) {
	var ownerID sql.NullString
	err := db.QueryRow(
		`SELECT it.created_by
		 FROM user u
		 JOIN invitation_token it ON u.invitation_token_id = it.id
		 WHERE u.id = ?`, agentUserID).Scan(&ownerID)
	if err != nil {
		return "", nil // cannot resolve -- not necessarily an error
	}
	if ownerID.Valid {
		return ownerID.String, nil
	}
	return "", nil
}

// IsUserSuspended returns true if the user's status is "suspended".
func (db *DB) IsUserSuspended(userID string) (bool, error) {
	var status string
	err := db.QueryRow(`SELECT status FROM user WHERE id = ?`, userID).Scan(&status)
	if err != nil {
		return false, err
	}
	return status == "suspended", nil
}

// BlockUser sets a user's status to "blocked" with a reason stored in metadata.
// Admin-suspended users cannot be blocked (suspend takes precedence).
func (db *DB) BlockUser(userID, reason string) error {
	now := UTCNow().Format(time.RFC3339)
	result, err := db.Exec(
		`UPDATE user SET
			status = 'blocked',
			metadata = json_set(COALESCE(metadata, '{}'), '$.blocked_reason', ?, '$.blocked_at', ?),
			updated_at = ?
		WHERE id = ? AND status != 'suspended'`,
		reason, now, now, userID)
	if err != nil {
		return fmt.Errorf("block user: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("user not found or is admin-suspended")
	}
	return nil
}

// UnblockUser restores a blocked user to active status and removes block metadata.
// Only works on users with status "blocked".
func (db *DB) UnblockUser(userID string) error {
	now := UTCNow().Format(time.RFC3339)
	result, err := db.Exec(
		`UPDATE user SET
			status = 'active',
			metadata = json_remove(COALESCE(metadata, '{}'), '$.blocked_reason', '$.blocked_at'),
			updated_at = ?
		WHERE id = ? AND status = 'blocked'`,
		now, userID)
	if err != nil {
		return fmt.Errorf("unblock user: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("user not found or not blocked")
	}
	return nil
}

// AgentBlockSignal aggregates blocked agent info per sponsor.
type AgentBlockSignal struct {
	SponsorUserID string
	SponsorName   string
	SponsorEmail  string
	TotalAgents   int
	BlockedCount  int
}

// GetAgentBlockSignals returns sponsors who have at least one blocked agent,
// ordered by blocked count descending.
func (db *DB) GetAgentBlockSignals() ([]AgentBlockSignal, error) {
	rows, err := db.Query(
		`SELECT
			it.created_by AS sponsor_user_id,
			sponsor.name AS sponsor_name,
			sponsor.email AS sponsor_email,
			COUNT(*) AS total_agents,
			SUM(CASE WHEN u.status = 'blocked' THEN 1 ELSE 0 END) AS blocked_count
		FROM user u
		JOIN invitation_token it ON u.invitation_token_id = it.id
		JOIN user sponsor ON it.created_by = sponsor.id
		WHERE u.user_type = 'agent'
		GROUP BY it.created_by
		HAVING blocked_count > 0
		ORDER BY blocked_count DESC`)
	if err != nil {
		return nil, fmt.Errorf("get agent block signals: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var signals []AgentBlockSignal
	for rows.Next() {
		var s AgentBlockSignal
		if err := rows.Scan(&s.SponsorUserID, &s.SponsorName, &s.SponsorEmail, &s.TotalAgents, &s.BlockedCount); err != nil {
			return nil, fmt.Errorf("scan block signal: %w", err)
		}
		signals = append(signals, s)
	}
	return signals, nil
}

// RevokeAllUserTokens revokes all active tokens for a user.
func (db *DB) RevokeAllUserTokens(userID string) error {
	_, err := db.Exec(
		`UPDATE token SET revoked_at = datetime('now') WHERE user_id = ? AND revoked_at IS NULL`,
		userID,
	)
	if err != nil {
		return fmt.Errorf("revoke all user tokens: %w", err)
	}
	return nil
}

// scanUser scans a user row from the given scanner.
func scanUser(rows *sql.Rows) (*User, error) {
	var user User
	var email, resourceID, externalID, lastActiveAt sql.NullString
	var metadataJSON string
	var createdAt, updatedAt string

	if err := rows.Scan(&user.ID, &user.Name, &email, &resourceID, &externalID, &user.Status, &user.IsAdmin, &user.LoginCount, &user.Tier, &user.UserType, &user.Lang, &user.Timezone, &user.EmailCopy, &lastActiveAt, &metadataJSON, &createdAt, &updatedAt); err != nil {
		return nil, err
	}

	if email.Valid {
		user.Email = email.String
	}
	if resourceID.Valid {
		user.ResourceID = &resourceID.String
	}
	if externalID.Valid {
		user.ExternalID = &externalID.String
	}
	if lastActiveAt.Valid {
		t := ParseDBTime(lastActiveAt.String)
		user.LastActiveAt = &t
	}

	_ = json.Unmarshal([]byte(metadataJSON), &user.Metadata)
	user.CreatedAt = ParseDBTime(createdAt)
	user.UpdatedAt = ParseDBTime(updatedAt)

	return &user, nil
}
