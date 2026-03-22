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
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// InvitationToken represents an invitation for self-registration.
// Scope determines whether it is system-wide or organization-scoped.
type InvitationToken struct {
	ID             string
	Token          string // Only set on creation, never stored
	Name           string
	Scope          string // "system" or "organization"
	OrganizationID string // Set when Scope is "organization"
	Role           string // Role granted on use (for org tokens: member, admin, etc.)
	MaxUses        *int
	Uses           int
	ExpiresAt      *time.Time
	RevokedAt      *time.Time
	CreatedAt      time.Time
	CreatedBy      string
}

// PendingUser represents a user registration awaiting email verification.
type PendingUser struct {
	ID                string
	Email             string
	Name              string
	UserType          string
	Lang              string
	InvitationTokenID string
	VerificationCode  string
	ExpiresAt         time.Time
	CreatedAt         time.Time
}

// Invitation and registration error sentinels.
var (
	// ErrInvitationNotFound is returned when an invitation token cannot be found.
	ErrInvitationNotFound = errors.New("invitation token not found")
	// ErrInvitationExpired is returned when an invitation token has passed its expiry.
	ErrInvitationExpired = errors.New("invitation token has expired")
	// ErrInvitationExhausted is returned when an invitation token has no remaining uses.
	ErrInvitationExhausted = errors.New("invitation token has reached max uses")
	// ErrInvitationRevoked is returned when an invitation token has been revoked.
	ErrInvitationRevoked = errors.New("invitation token has been revoked")
	// ErrEmailExists is returned when a registration email is already in use.
	ErrEmailExists = errors.New("email already registered")
	// ErrPendingUserNotFound is returned when no pending registration exists for an email.
	ErrPendingUserNotFound = errors.New("no pending registration for this email")
	// ErrInvalidCode is returned when a verification code does not match.
	ErrInvalidCode = errors.New("invalid verification code")
	// ErrCodeExpired is returned when a verification code has passed its expiry.
	ErrCodeExpired = errors.New("verification code has expired")
)

// mustRandBytes fills b with cryptographically secure random bytes.
// Panics on failure -- a broken CSPRNG means all tokens and IDs are predictable.
func mustRandBytes(b []byte) {
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand: " + err.Error())
	}
}

// generateToken creates a random token with prefix.
func generateToken(prefix string) string {
	bytes := make([]byte, 24)
	mustRandBytes(bytes)
	return prefix + "_" + hex.EncodeToString(bytes)
}

// hashToken creates a SHA-256 hash of a token.
func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// generateID creates a random ID with prefix.
func generateID(prefix string) string {
	bytes := make([]byte, 12)
	mustRandBytes(bytes)
	return prefix + "_" + hex.EncodeToString(bytes)
}

// GenerateID creates a random ID with the given prefix, for use by other packages.
func GenerateID(prefix string) string {
	return generateID(prefix)
}

// CreateInvitationToken creates a new invitation token.
// Scope defaults to "system" if empty. For org-scoped tokens, provide organizationID and optionally role.
func (db *DB) CreateInvitationToken(name, scope, organizationID, role string, maxUses *int, expiresAt *time.Time, createdBy string) (*InvitationToken, error) {
	if scope == "" {
		scope = "system"
	}

	id := generateID("inv")
	token := generateToken("inv")
	tokenHash := hashToken(token)

	var expiresAtStr *string
	if expiresAt != nil {
		s := expiresAt.Format(time.RFC3339)
		expiresAtStr = &s
	}

	var orgID *string
	if organizationID != "" {
		orgID = &organizationID
	}
	var roleVal *string
	if role != "" {
		roleVal = &role
	}
	var createdByVal *string
	if createdBy != "" {
		createdByVal = &createdBy
	}

	_, err := db.Exec(
		`INSERT INTO invitation_token (id, token_hash, name, scope, organization_id, role, max_uses, expires_at, created_by)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, tokenHash, name, scope, orgID, roleVal, maxUses, expiresAtStr, createdByVal,
	)
	if err != nil {
		return nil, fmt.Errorf("insert invitation token: %w", err)
	}

	return &InvitationToken{
		ID:             id,
		Token:          token, // Return plaintext token only on creation
		Name:           name,
		Scope:          scope,
		OrganizationID: organizationID,
		Role:           role,
		MaxUses:        maxUses,
		Uses:           0,
		ExpiresAt:      expiresAt,
		CreatedAt:      UTCNow(),
		CreatedBy:      createdBy,
	}, nil
}

// ValidateInvitationToken checks if a token is valid and can be used.
func (db *DB) ValidateInvitationToken(token string) (*InvitationToken, error) {
	tokenHash := hashToken(token)

	var inv InvitationToken
	var name, orgID, role sql.NullString
	var maxUses sql.NullInt64
	var expiresAt, revokedAt sql.NullString
	var createdAt string
	var createdBy sql.NullString

	err := db.QueryRow(
		`SELECT id, name, scope, organization_id, role, max_uses, uses, expires_at, revoked_at, created_at, created_by
		 FROM invitation_token WHERE token_hash = ?`,
		tokenHash,
	).Scan(&inv.ID, &name, &inv.Scope, &orgID, &role, &maxUses, &inv.Uses, &expiresAt, &revokedAt, &createdAt, &createdBy)

	if err == sql.ErrNoRows {
		return nil, ErrInvitationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query invitation token: %w", err)
	}

	// Parse fields
	if name.Valid {
		inv.Name = name.String
	}
	if orgID.Valid {
		inv.OrganizationID = orgID.String
	}
	if role.Valid {
		inv.Role = role.String
	}
	if maxUses.Valid {
		m := int(maxUses.Int64)
		inv.MaxUses = &m
	}
	if expiresAt.Valid {
		t := ParseDBTime(expiresAt.String)
		inv.ExpiresAt = &t
	}
	if revokedAt.Valid {
		t := ParseDBTime(revokedAt.String)
		inv.RevokedAt = &t
	}
	inv.CreatedAt = ParseDBTime(createdAt)
	if createdBy.Valid {
		inv.CreatedBy = createdBy.String
	}

	// Check if revoked
	if inv.RevokedAt != nil {
		return nil, ErrInvitationRevoked
	}

	// Check if expired
	if inv.ExpiresAt != nil && UTCNow().After(*inv.ExpiresAt) {
		return nil, ErrInvitationExpired
	}

	// Check if exhausted
	if inv.MaxUses != nil && inv.Uses >= *inv.MaxUses {
		return nil, ErrInvitationExhausted
	}

	return &inv, nil
}

// ValidateOrgInvitationToken validates an organization-scoped invitation token.
// It checks that the token exists, is scoped to the given organization, and is active.
func (db *DB) ValidateOrgInvitationToken(orgID, token string) (*InvitationToken, error) {
	tokenHash := hashToken(token)

	var inv InvitationToken
	var name, dbOrgID, role sql.NullString
	var maxUses sql.NullInt64
	var expiresAt, revokedAt sql.NullString
	var createdAt string
	var createdBy sql.NullString

	err := db.QueryRow(
		`SELECT id, name, scope, organization_id, role, max_uses, uses, expires_at, revoked_at, created_at, created_by
		 FROM invitation_token
		 WHERE token_hash = ? AND scope = 'organization' AND organization_id = ?`,
		tokenHash, orgID,
	).Scan(&inv.ID, &name, &inv.Scope, &dbOrgID, &role, &maxUses, &inv.Uses, &expiresAt, &revokedAt, &createdAt, &createdBy)

	if err == sql.ErrNoRows {
		return nil, ErrInvitationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query org invitation token: %w", err)
	}

	// Parse fields
	if name.Valid {
		inv.Name = name.String
	}
	if dbOrgID.Valid {
		inv.OrganizationID = dbOrgID.String
	}
	if role.Valid {
		inv.Role = role.String
	}
	if maxUses.Valid {
		m := int(maxUses.Int64)
		inv.MaxUses = &m
	}
	if expiresAt.Valid {
		t := ParseDBTime(expiresAt.String)
		inv.ExpiresAt = &t
	}
	if revokedAt.Valid {
		t := ParseDBTime(revokedAt.String)
		inv.RevokedAt = &t
	}
	inv.CreatedAt = ParseDBTime(createdAt)
	if createdBy.Valid {
		inv.CreatedBy = createdBy.String
	}

	// Check if revoked
	if inv.RevokedAt != nil {
		return nil, ErrInvitationRevoked
	}

	// Check if expired
	if inv.ExpiresAt != nil && UTCNow().After(*inv.ExpiresAt) {
		return nil, ErrInvitationExpired
	}

	// Check if exhausted
	if inv.MaxUses != nil && inv.Uses >= *inv.MaxUses {
		return nil, ErrInvitationExhausted
	}

	return &inv, nil
}

// IncrementInvitationTokenUse increments the usage counter.
func (db *DB) IncrementInvitationTokenUse(id string) error {
	_, err := db.Exec("UPDATE invitation_token SET uses = uses + 1 WHERE id = ?", id)
	return err
}

// ListInvitationTokens lists invitation tokens with optional status, org, and creator filters.
func (db *DB) ListInvitationTokens(status, organizationID string, createdBy ...string) ([]*InvitationToken, error) {
	query := `SELECT id, name, scope, organization_id, role, max_uses, uses, expires_at, revoked_at, created_at, created_by
			  FROM invitation_token`

	var conditions []string
	var params []interface{}

	if organizationID != "" {
		conditions = append(conditions, "organization_id = ?")
		params = append(params, organizationID)
	}
	if len(createdBy) > 0 && createdBy[0] != "" {
		conditions = append(conditions, "created_by = ?")
		params = append(params, createdBy[0])
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	query += " ORDER BY created_at DESC"

	rows, err := db.Query(query, params...)
	if err != nil {
		return nil, fmt.Errorf("query invitation tokens: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var tokens []*InvitationToken
	for rows.Next() {
		var inv InvitationToken
		var name, orgID, role sql.NullString
		var maxUses sql.NullInt64
		var expiresAt, revokedAt sql.NullString
		var createdAt string
		var createdBy sql.NullString

		err := rows.Scan(&inv.ID, &name, &inv.Scope, &orgID, &role, &maxUses, &inv.Uses, &expiresAt, &revokedAt, &createdAt, &createdBy)
		if err != nil {
			return nil, fmt.Errorf("scan invitation token: %w", err)
		}

		if name.Valid {
			inv.Name = name.String
		}
		if orgID.Valid {
			inv.OrganizationID = orgID.String
		}
		if role.Valid {
			inv.Role = role.String
		}
		if maxUses.Valid {
			m := int(maxUses.Int64)
			inv.MaxUses = &m
		}
		if expiresAt.Valid {
			t := ParseDBTime(expiresAt.String)
			inv.ExpiresAt = &t
		}
		if revokedAt.Valid {
			t := ParseDBTime(revokedAt.String)
			inv.RevokedAt = &t
		}
		inv.CreatedAt = ParseDBTime(createdAt)
		if createdBy.Valid {
			inv.CreatedBy = createdBy.String
		}

		// Filter by status if specified
		if status != "" {
			tokenStatus := GetTokenStatus(&inv)
			if tokenStatus != status {
				continue
			}
		}

		tokens = append(tokens, &inv)
	}

	return tokens, nil
}

// GetTokenStatus determines the status of an invitation token.
func GetTokenStatus(inv *InvitationToken) string {
	if inv.RevokedAt != nil {
		return "revoked"
	}
	if inv.ExpiresAt != nil && UTCNow().After(*inv.ExpiresAt) {
		return "expired"
	}
	if inv.MaxUses != nil && inv.Uses >= *inv.MaxUses {
		return "exhausted"
	}
	return "active"
}

// GetInvitationTokenByID retrieves an invitation token by its database ID.
func (db *DB) GetInvitationTokenByID(id string) (*InvitationToken, error) {
	var inv InvitationToken
	var name, orgID, role sql.NullString
	var maxUses sql.NullInt64
	var expiresAt, revokedAt sql.NullString
	var createdAt string
	var createdBy sql.NullString

	err := db.QueryRow(
		`SELECT id, name, scope, organization_id, role, max_uses, uses, expires_at, revoked_at, created_at, created_by
		 FROM invitation_token WHERE id = ?`, id,
	).Scan(&inv.ID, &name, &inv.Scope, &orgID, &role, &maxUses, &inv.Uses, &expiresAt, &revokedAt, &createdAt, &createdBy)
	if err != nil {
		return nil, ErrInvitationNotFound
	}

	if name.Valid {
		inv.Name = name.String
	}
	if orgID.Valid {
		inv.OrganizationID = orgID.String
	}
	if role.Valid {
		inv.Role = role.String
	}
	if maxUses.Valid {
		m := int(maxUses.Int64)
		inv.MaxUses = &m
	}
	if expiresAt.Valid {
		t := ParseDBTime(expiresAt.String)
		inv.ExpiresAt = &t
	}
	if revokedAt.Valid {
		t := ParseDBTime(revokedAt.String)
		inv.RevokedAt = &t
	}
	inv.CreatedAt = ParseDBTime(createdAt)
	if createdBy.Valid {
		inv.CreatedBy = createdBy.String
	}

	return &inv, nil
}

// RevokeInvitationToken revokes an invitation token.
func (db *DB) RevokeInvitationToken(token string) error {
	tokenHash := hashToken(token)

	result, err := db.Exec(
		"UPDATE invitation_token SET revoked_at = datetime('now') WHERE token_hash = ? AND revoked_at IS NULL",
		tokenHash,
	)
	if err != nil {
		return fmt.Errorf("revoke invitation token: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrInvitationNotFound
	}

	return nil
}

// RevokeInvitationTokenByID revokes an invitation token by its ID.
func (db *DB) RevokeInvitationTokenByID(id string) error {
	result, err := db.Exec(
		"UPDATE invitation_token SET revoked_at = datetime('now') WHERE id = ? AND revoked_at IS NULL",
		id,
	)
	if err != nil {
		return fmt.Errorf("revoke invitation token: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrInvitationNotFound
	}

	return nil
}

// CreatePendingUser creates a pending user registration.
func (db *DB) CreatePendingUser(email, name, password, invitationTokenID, userType, lang string, timeout time.Duration, identity *RegistrationIdentity) (*PendingUser, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	if userType == "" {
		userType = "human"
	}
	if lang == "" {
		lang = "en"
	}

	// Check if email already exists as a user
	var exists int
	err := db.QueryRow("SELECT COUNT(*) FROM user WHERE email = ?", email).Scan(&exists)
	if err != nil {
		return nil, fmt.Errorf("check email: %w", err)
	}
	if exists > 0 {
		return nil, ErrEmailExists
	}

	// Hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	id := generateID("pnd")
	code := generateCode()
	now := UTCNow()
	expiresAt := now.Add(timeout)

	// Use NULL for empty invitation_token_id (open registration)
	var invTokenVal *string
	if invitationTokenID != "" {
		invTokenVal = &invitationTokenID
	}

	// Delete any existing pending registration for this email
	_, _ = db.Exec("DELETE FROM pending_user WHERE email = ?", email)

	_, err = db.Exec(
		`INSERT INTO pending_user (id, email, name, password_hash, invitation_token_id, user_type, lang, verification_code, expires_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, email, name, string(hash), invTokenVal, userType, lang, code, expiresAt.Format(time.RFC3339),
	)
	if err != nil {
		return nil, fmt.Errorf("insert pending user: %w", err)
	}

	// Update identity fields if provided
	if identity != nil {
		_, err = db.Exec(
			`UPDATE pending_user SET account_type = ?, first_name = ?, last_name = ?, company_name = ?, company_registration = ?, vat_number = ?, street = ?, street2 = ?, postal_code = ?, city = ?, state = ?, country = ?, accept_dpa = ? WHERE id = ?`,
			identity.AccountType, identity.FirstName, identity.LastName, identity.CompanyName,
			identity.CompanyRegistration, identity.VATNumber,
			identity.Street, identity.Street2, identity.PostalCode, identity.City, identity.State,
			identity.Country, identity.AcceptDPA, id,
		)
		if err != nil {
			return nil, fmt.Errorf("set pending user identity: %w", err)
		}
	}

	return &PendingUser{
		ID:                id,
		Email:             email,
		Name:              name,
		UserType:          userType,
		Lang:              lang,
		InvitationTokenID: invitationTokenID,
		VerificationCode:  code,
		ExpiresAt:         expiresAt,
		CreatedAt:         now,
	}, nil
}

// GetPendingUser retrieves a pending user by email.
func (db *DB) GetPendingUser(email string) (*PendingUser, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	var p PendingUser
	var invTokenID sql.NullString
	var expiresAt, createdAt string

	err := db.QueryRow(
		`SELECT id, email, name, invitation_token_id, user_type, lang, verification_code, expires_at, created_at
		 FROM pending_user WHERE email = ?`,
		email,
	).Scan(&p.ID, &p.Email, &p.Name, &invTokenID, &p.UserType, &p.Lang, &p.VerificationCode, &expiresAt, &createdAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query pending user: %w", err)
	}

	if invTokenID.Valid {
		p.InvitationTokenID = invTokenID.String
	}
	p.ExpiresAt = ParseDBTime(expiresAt)
	p.CreatedAt = ParseDBTime(createdAt)

	return &p, nil
}

// VerifyAndCreateUser verifies the code and creates the user account.
func (db *DB) VerifyAndCreateUser(email, code string) (*User, string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	code = strings.ToLower(strings.TrimSpace(code))

	// Get pending user
	var id, name, passwordHash, storedCode, expiresAt, userType, lang string
	var invTokenID sql.NullString
	var accountType, firstName, lastName, companyName, countryCol sql.NullString
	var companyRegistration, vatNumber, street, street2, postalCode, city, state sql.NullString
	var acceptDPA int

	err := db.QueryRow(
		`SELECT id, name, password_hash, invitation_token_id, user_type, lang, verification_code, expires_at,
		        account_type, first_name, last_name, company_name, country,
		        COALESCE(company_registration, ''), COALESCE(vat_number, ''),
		        COALESCE(street, ''), COALESCE(street2, ''), COALESCE(postal_code, ''),
		        COALESCE(city, ''), COALESCE(state, ''), COALESCE(accept_dpa, 0)
		 FROM pending_user WHERE email = ?`,
		email,
	).Scan(&id, &name, &passwordHash, &invTokenID, &userType, &lang, &storedCode, &expiresAt,
		&accountType, &firstName, &lastName, &companyName, &countryCol,
		&companyRegistration, &vatNumber, &street, &street2, &postalCode, &city, &state, &acceptDPA)

	if err == sql.ErrNoRows {
		return nil, "", ErrPendingUserNotFound
	}
	if err != nil {
		return nil, "", fmt.Errorf("query pending user: %w", err)
	}

	// Check code
	if storedCode != code {
		return nil, "", ErrInvalidCode
	}

	// Check expiry
	expires := ParseDBTime(expiresAt)
	if UTCNow().After(expires) {
		return nil, "", ErrCodeExpired
	}

	if userType == "" {
		userType = "human"
	}
	if lang == "" {
		lang = "en"
	}

	// Create user with the configured default tier.
	// Agent users start with interview_pending; humans start active.
	onboardingStatus := "active"
	if userType == "agent" {
		onboardingStatus = "interview_pending"
	}

	defaultTier := db.DefaultTierID()
	userID := generateID("usr")
	_, err = db.Exec(
		`INSERT INTO user (id, name, email, password_hash, invitation_token_id, tier, user_type, lang, status, onboarding_status)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'active', ?)`,
		userID, name, email, passwordHash, invTokenID, defaultTier, userType, lang, onboardingStatus,
	)
	if err != nil {
		return nil, "", fmt.Errorf("create user: %w", err)
	}

	// Create auth token for the new user
	token := generateToken("ts")
	tokenHash := hashToken(token)
	tokenID := generateID("tkn")

	_, err = db.Exec(
		`INSERT INTO token (id, user_id, token_hash, name)
		 VALUES (?, ?, ?, 'Registration Token')`,
		tokenID, userID, tokenHash,
	)
	if err != nil {
		return nil, "", fmt.Errorf("create token: %w", err)
	}

	// Create a resource for the user so they can author comments and messages.
	// Include delivery metadata so intercom can relay messages as email.
	resMeta := map[string]interface{}{
		"delivery": map[string]interface{}{
			"email":    email,
			"channels": []string{"email"},
		},
	}
	res, err := db.CreateResource(userType, name, "", nil, nil, resMeta)
	if err != nil {
		return nil, "", fmt.Errorf("create resource for user: %w", err)
	}

	// Link resource to user
	_, err = db.Exec(`UPDATE user SET resource_id = ? WHERE id = ?`, res.ID, userID)
	if err != nil {
		return nil, "", fmt.Errorf("link resource to user: %w", err)
	}

	// Auto-link agent to sponsor's organization (best-effort, same as CreateUserWithInvitation).
	if userType == "agent" && invTokenID.Valid && invTokenID.String != "" {
		db.autoLinkAgentToOrg(invTokenID.String, res.ID)
	}

	// Store account_type and company_name in user metadata for later use.
	meta := map[string]interface{}{}
	if accountType.Valid && accountType.String != "" {
		meta["account_type"] = accountType.String
	}
	if companyName.Valid && companyName.String != "" {
		meta["company_name"] = companyName.String
	}
	if len(meta) > 0 {
		if _, err := db.UpdateUser(userID, UpdateUserFields{Metadata: meta}); err != nil {
			slog.Warn("Failed to store user metadata during registration", "user_id", userID, "error", err)
		}
	}

	// When KYC is not required, create the auto-org immediately.
	// When KYC is required, identity recording and org creation are
	// deferred to the complete-profile step (phase 2 of registration).
	requireKYC, _ := db.GetPolicy("registration.require_kyc")
	if requireKYC == "false" && name != "" {
		orgName := name
		if accountType.Valid && accountType.String == "business" && companyName.Valid && companyName.String != "" {
			orgName = companyName.String
		}
		org, orgErr := db.CreateOrganization(orgName, "", nil)
		if orgErr != nil {
			slog.Warn("Failed to create auto-org during registration", "user_id", userID, "error", orgErr)
		} else if err := db.AddResourceToOrganization(org.ID, res.ID, "owner"); err != nil {
			slog.Warn("Failed to add resource to auto-org", "user_id", userID, "org_id", org.ID, "error", err)
		}
	}

	// Delete pending registration
	if _, err := db.Exec("DELETE FROM pending_user WHERE email = ?", email); err != nil {
		slog.Warn("Failed to clean up pending registration", "email", email, "error", err)
	}

	user := &User{
		ID:         userID,
		Name:       name,
		Email:      email,
		ResourceID: &res.ID,
		Status:     "active",
		Tier:       defaultTier,
		UserType:   userType,
		Lang:       lang,
	}

	return user, token, nil
}

// User struct is defined in user.go.

// CreateUserWithInvitation creates a user directly using a validated invitation token,
// bypassing the pending_user/email-verification flow. The invitation token serves as
// proof of authorization from a human operator.
func (db *DB) CreateUserWithInvitation(email, name, passwordHash, invTokenID, userType, lang string, identity *RegistrationIdentity) (*User, string, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	// Check email uniqueness against both users and pending registrations
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM user WHERE email = ?`, email).Scan(&count); err != nil {
		return nil, "", fmt.Errorf("check email: %w", err)
	}
	if count > 0 {
		return nil, "", ErrEmailExists
	}

	if userType == "" {
		userType = "agent"
	}
	if lang == "" {
		lang = "en"
	}

	// Agent users start with interview_pending; humans start active.
	onboardingStatus := "active"
	if userType == "agent" {
		onboardingStatus = "interview_pending"
	}

	defaultTier := db.DefaultTierID()
	userID := generateID("usr")
	_, err := db.Exec(
		`INSERT INTO user (id, name, email, password_hash, invitation_token_id, tier, user_type, lang, status, onboarding_status)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'active', ?)`,
		userID, name, email, passwordHash, invTokenID, defaultTier, userType, lang, onboardingStatus,
	)
	if err != nil {
		return nil, "", fmt.Errorf("create user: %w", err)
	}

	// Create auth token
	token := generateToken("ts")
	tokenHash := hashToken(token)
	tokenID := generateID("tkn")

	_, err = db.Exec(
		`INSERT INTO token (id, user_id, token_hash, name)
		 VALUES (?, ?, ?, 'Registration Token')`,
		tokenID, userID, tokenHash,
	)
	if err != nil {
		return nil, "", fmt.Errorf("create token: %w", err)
	}

	// Create a resource for the user so they can author comments and messages.
	// Include delivery metadata so intercom can relay messages as email.
	resMeta := map[string]interface{}{
		"delivery": map[string]interface{}{
			"email":    email,
			"channels": []string{"email"},
		},
	}
	res, err := db.CreateResource(userType, name, "", nil, nil, resMeta)
	if err != nil {
		return nil, "", fmt.Errorf("create resource for user: %w", err)
	}

	// Link resource to user
	_, err = db.Exec(`UPDATE user SET resource_id = ? WHERE id = ?`, res.ID, userID)
	if err != nil {
		return nil, "", fmt.Errorf("link resource to user: %w", err)
	}

	// Delete any stale pending registration for this email
	if _, err := db.Exec("DELETE FROM pending_user WHERE email = ?", email); err != nil {
		slog.Warn("Failed to clean up pending registration", "email", email, "error", err)
	}

	// Auto-link agent resource to sponsor's organization.
	if userType == "agent" {
		db.autoLinkAgentToOrg(invTokenID, res.ID)
	}

	// Create person and consent records if identity data was provided.
	if identity != nil {
		if _, err := db.RecordRegistrationIdentity(userID, identity, "", ""); err != nil {
			slog.Warn("Failed to record registration identity", "user_id", userID, "error", err)
		}
	}

	user := &User{
		ID:         userID,
		Name:       name,
		Email:      email,
		ResourceID: &res.ID,
		Status:     "active",
		Tier:       defaultTier,
		UserType:   userType,
		Lang:       lang,
	}

	return user, token, nil
}

// autoLinkAgentToOrg finds the sponsor's org from the invitation token and
// adds the agent's resource as a member. Best-effort; failures are logged.
func (db *DB) autoLinkAgentToOrg(invTokenID, resourceID string) {
	inv, err := db.GetInvitationTokenByID(invTokenID)
	if err != nil {
		return
	}

	orgID := inv.OrganizationID
	if orgID == "" && inv.CreatedBy != "" {
		// Find sponsor's org: resource with "owner" role in an organization.
		var sponsorResID string
		err := db.QueryRow(`SELECT resource_id FROM user WHERE id = ?`, inv.CreatedBy).Scan(&sponsorResID)
		if err != nil || sponsorResID == "" {
			return
		}
		err = db.QueryRow(
			`SELECT source_entity_id FROM entity_relation
			 WHERE relationship_type = 'has_member'
			   AND source_entity_type = 'organization'
			   AND target_entity_type = 'resource'
			   AND target_entity_id = ?
			   AND json_extract(metadata, '$.role') = 'owner'
			 LIMIT 1`,
			sponsorResID,
		).Scan(&orgID)
		if err != nil || orgID == "" {
			return
		}
	}

	if orgID != "" {
		if err := db.AddResourceToOrganization(orgID, resourceID, "member"); err != nil {
			slog.Warn("Failed to auto-link agent to sponsor org", "org_id", orgID, "resource_id", resourceID, "error", err)
		}
	}
}

// RegeneratePendingUserCode creates a new verification code for existing pending user.
func (db *DB) RegeneratePendingUserCode(email string, timeout time.Duration) (*PendingUser, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	// Check if pending user exists
	pending, err := db.GetPendingUser(email)
	if err != nil {
		return nil, err
	}
	if pending == nil {
		return nil, ErrPendingUserNotFound
	}

	// Generate new code
	code := generateCode()
	expiresAt := UTCNow().Add(timeout)

	// Update the code and expiry
	_, err = db.Exec(
		`UPDATE pending_user SET verification_code = ?, expires_at = ? WHERE email = ?`,
		code, expiresAt.Format(time.RFC3339), email,
	)
	if err != nil {
		return nil, fmt.Errorf("update verification code: %w", err)
	}

	pending.VerificationCode = code
	pending.ExpiresAt = expiresAt

	return pending, nil
}

// CleanExpiredPendingUsers removes expired pending user registrations.
func (db *DB) CleanExpiredPendingUsers() error {
	_, err := db.Exec("DELETE FROM pending_user WHERE expires_at < datetime('now')")
	return err
}
