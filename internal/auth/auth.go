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


// Package auth provides shared authentication and authorization functions
// used by the MCP server, REST API, and web UI.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/QuestFinTech/taskschmiede/internal/storage"
	"golang.org/x/crypto/bcrypt"
)

// contextKey is a type for context keys.
type contextKey string

const (
	// AuthUserKey is the context key for the authenticated user.
	AuthUserKey contextKey = "auth_user"

	// UserScopeKey is the context key for the resolved user scope (RBAC).
	UserScopeKey contextKey = "user_scope"
)

// AuthUser represents an authenticated user from the context.
type AuthUser struct {
	UserID    string
	TokenID   string
	UserType  string // "human" or "agent"
	ExpiresAt *time.Time
	CreatedAt *time.Time // session creation time (for absolute lifetime enforcement)
}

// MaxSessionLifetime is the hard upper bound for any MCP session,
// regardless of sliding-window renewal. After this duration the session
// is force-expired and the client must re-authenticate.
const MaxSessionLifetime = 24 * time.Hour

// Service provides authentication operations backed by the database.
type Service struct {
	db *storage.DB
}

// NewService creates a new auth Service.
func NewService(db *storage.DB) *Service {
	return &Service{db: db}
}

// ErrAccountSuspended is returned when a suspended user tries to authenticate.
var ErrAccountSuspended = fmt.Errorf("account suspended")

// ErrAccountBlocked is returned when a sponsor-blocked agent tries to authenticate.
var ErrAccountBlocked = fmt.Errorf("account blocked by sponsor")

// Authenticate verifies email/password and returns the user.
func (s *Service) Authenticate(ctx context.Context, email, password string) (*storage.User, error) {
	var user storage.User
	var passwordHash sql.NullString
	var name sql.NullString
	var resourceID sql.NullString
	var status string

	err := s.db.QueryRow(
		`SELECT id, email, name, password_hash, COALESCE(user_type, 'human'), resource_id, status FROM user WHERE email = ?`,
		email,
	).Scan(&user.ID, &user.Email, &name, &passwordHash, &user.UserType, &resourceID, &status)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("query user: %w", err)
	}

	if status == "suspended" {
		return nil, ErrAccountSuspended
	}
	if status == "blocked" {
		return nil, ErrAccountBlocked
	}
	if status != "active" {
		return nil, fmt.Errorf("user not found")
	}

	if !passwordHash.Valid || passwordHash.String == "" {
		return nil, fmt.Errorf("password authentication not available")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash.String), []byte(password)); err != nil {
		return nil, fmt.Errorf("invalid password")
	}

	if name.Valid {
		user.Name = name.String
	}
	if resourceID.Valid {
		user.ResourceID = &resourceID.String
	}

	return &user, nil
}

// CreateToken creates a new access token for a user.
// Returns the plaintext token (only available at creation time), the token ID, and any error.
func (s *Service) CreateToken(ctx context.Context, userID, name string, expiresAt *time.Time) (string, string, error) {
	// Generate random token (32 bytes = 64 hex chars)
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", "", fmt.Errorf("generate token: %w", err)
	}
	token := hex.EncodeToString(tokenBytes)

	// Hash the token for storage
	hash := sha256.Sum256([]byte(token))
	tokenHash := hex.EncodeToString(hash[:])

	// Generate token ID
	idBytes := make([]byte, 8)
	if _, err := rand.Read(idBytes); err != nil {
		return "", "", fmt.Errorf("generate token id: %w", err)
	}
	tokenID := "tkn_" + hex.EncodeToString(idBytes)

	// Insert token
	var expiresAtStr *string
	if expiresAt != nil {
		s := expiresAt.Format(time.RFC3339)
		expiresAtStr = &s
	}

	_, err := s.db.Exec(
		`INSERT INTO token (id, user_id, token_hash, name, expires_at) VALUES (?, ?, ?, ?, ?)`,
		tokenID, userID, tokenHash, name, expiresAtStr,
	)
	if err != nil {
		return "", "", fmt.Errorf("insert token: %w", err)
	}

	return token, tokenID, nil
}

// VerifyToken validates a token and returns the associated auth user.
func (s *Service) VerifyToken(ctx context.Context, token string) (*AuthUser, error) {
	hash := sha256.Sum256([]byte(token))
	tokenHash := hex.EncodeToString(hash[:])

	var tokenID, userID, userType, userStatus string
	var expiresAt sql.NullString
	var revokedAt sql.NullString

	err := s.db.QueryRow(
		`SELECT t.id, t.user_id, t.expires_at, t.revoked_at, COALESCE(u.user_type, 'human'), u.status
		 FROM token t JOIN user u ON t.user_id = u.id
		 WHERE t.token_hash = ?`,
		tokenHash,
	).Scan(&tokenID, &userID, &expiresAt, &revokedAt, &userType, &userStatus)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("invalid_token")
	}
	if err != nil {
		return nil, fmt.Errorf("query token: %w", err)
	}

	if revokedAt.Valid {
		return nil, fmt.Errorf("token_revoked")
	}

	if userStatus == "suspended" {
		return nil, ErrAccountSuspended
	}
	if userStatus == "blocked" {
		return nil, ErrAccountBlocked
	}

	var expTime *time.Time
	if expiresAt.Valid {
		t := storage.ParseDBTime(expiresAt.String)
		if !t.IsZero() {
			expTime = &t
			if storage.UTCNow().After(t) {
				return nil, fmt.Errorf("token_expired")
			}
		}
	}

	// Update last_used_at
	_, _ = s.db.Exec(`UPDATE token SET last_used_at = datetime('now') WHERE id = ?`, tokenID)

	return &AuthUser{
		UserID:    userID,
		TokenID:   tokenID,
		UserType:  userType,
		ExpiresAt: expTime,
	}, nil
}

// RevokeToken marks a token as revoked by setting revoked_at.
// The token is identified by its plaintext value (hashed for lookup).
func (s *Service) RevokeToken(ctx context.Context, token string) error {
	hash := sha256.Sum256([]byte(token))
	tokenHash := hex.EncodeToString(hash[:])

	result, err := s.db.Exec(
		`UPDATE token SET revoked_at = datetime('now') WHERE token_hash = ? AND revoked_at IS NULL`,
		tokenHash,
	)
	if err != nil {
		return fmt.Errorf("revoke token: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("token not found or already revoked")
	}

	return nil
}

// IsTokenValid checks whether a token is still valid (not revoked, not expired)
// by looking it up by ID. This is a lightweight check for session cache validation.
func (s *Service) IsTokenValid(ctx context.Context, tokenID string) bool {
	var revokedAt sql.NullString
	var expiresAt sql.NullString

	err := s.db.QueryRow(
		`SELECT expires_at, revoked_at FROM token WHERE id = ?`,
		tokenID,
	).Scan(&expiresAt, &revokedAt)
	if err != nil {
		return false
	}
	if revokedAt.Valid {
		return false
	}
	if expiresAt.Valid {
		t := storage.ParseDBTime(expiresAt.String)
		if !t.IsZero() && storage.UTCNow().After(t) {
			return false
		}
	}
	return true
}

// IsAdmin checks if a user has admin privileges (master admin or org admin).
func (s *Service) IsAdmin(ctx context.Context, userID string) (bool, error) {
	if s.IsMasterAdmin(ctx, userID) {
		return true, nil
	}

	// Check if user has admin role in any organization (via entity_relation)
	var count int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM entity_relation er
		 JOIN user u ON u.resource_id = er.target_entity_id
		 WHERE er.source_entity_type = 'organization'
		   AND er.relationship_type = 'has_member'
		   AND er.target_entity_type = 'resource'
		   AND u.id = ?
		   AND json_extract(er.metadata, '$.role') IN ('owner', 'admin')`,
		userID,
	).Scan(&count)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// IsMasterAdmin checks if a user is a system admin.
func (s *Service) IsMasterAdmin(ctx context.Context, userID string) bool {
	var count int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM user WHERE id = ? AND is_admin = 1 AND status = 'active'`,
		userID,
	).Scan(&count)
	if err != nil {
		return false
	}
	return count > 0
}

// DefaultTokenTTL reads the token.default_ttl policy from the database.
// Returns the default TTL of 8h if the policy is not set.
func (s *Service) DefaultTokenTTL(ctx context.Context) time.Duration {
	const defaultTTL = 8 * time.Hour
	raw, err := s.db.GetPolicy("token.default_ttl")
	if err != nil {
		return defaultTTL
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil {
		return defaultTTL
	}
	return parsed
}

// GetAuthUser returns the authenticated user from context, if any.
func GetAuthUser(ctx context.Context) *AuthUser {
	if user, ok := ctx.Value(AuthUserKey).(*AuthUser); ok {
		return user
	}
	return nil
}

// RequireAuth returns an error if no user is authenticated in the context.
func RequireAuth(ctx context.Context) (*AuthUser, error) {
	user := GetAuthUser(ctx)
	if user == nil {
		return nil, fmt.Errorf("authentication required")
	}
	return user, nil
}

// WithAuthUser returns a new context with the given AuthUser.
func WithAuthUser(ctx context.Context, user *AuthUser) context.Context {
	return context.WithValue(ctx, AuthUserKey, user)
}

// ExtractBearerToken extracts the bearer token from an Authorization header value.
// Returns empty string if the header is missing or not a Bearer token.
func ExtractBearerToken(authHeader string) string {
	const prefix = "Bearer "
	if len(authHeader) > len(prefix) && authHeader[:len(prefix)] == prefix {
		return authHeader[len(prefix):]
	}
	return ""
}

// UserScope represents the resolved permissions for an authenticated user.
// Populated once per request by RBAC middleware and stored in context.
type UserScope struct {
	UserID        string
	IsMasterAdmin bool
	// Endeavours maps endeavour_id -> role (owner, admin, member, viewer).
	Endeavours map[string]string
	// Organizations maps org_id -> role (owner, admin, member, guest).
	Organizations map[string]string
}

// CanRead checks if the user has at least viewer access to an endeavour.
func (s *UserScope) CanRead(endeavourID string) bool {
	if s.IsMasterAdmin {
		return true
	}
	_, ok := s.Endeavours[endeavourID]
	return ok
}

// CanWrite checks if the user has at least member access to an endeavour.
func (s *UserScope) CanWrite(endeavourID string) bool {
	if s.IsMasterAdmin {
		return true
	}
	role, ok := s.Endeavours[endeavourID]
	if !ok {
		return false
	}
	return role == "owner" || role == "admin" || role == "member"
}

// CanAdmin checks if the user has admin or owner access to an endeavour.
func (s *UserScope) CanAdmin(endeavourID string) bool {
	if s.IsMasterAdmin {
		return true
	}
	role, ok := s.Endeavours[endeavourID]
	if !ok {
		return false
	}
	return role == "owner" || role == "admin"
}

// ReadableEndeavourIDs returns all endeavour IDs the user can read.
func (s *UserScope) ReadableEndeavourIDs() []string {
	ids := make([]string, 0, len(s.Endeavours))
	for id := range s.Endeavours {
		ids = append(ids, id)
	}
	return ids
}

// OrgIDs returns all organization IDs the user belongs to (any role).
func (s *UserScope) OrgIDs() []string {
	ids := make([]string, 0, len(s.Organizations))
	for id := range s.Organizations {
		ids = append(ids, id)
	}
	return ids
}

// CanReadOrg checks if the user has at least guest access to an organization.
func (s *UserScope) CanReadOrg(orgID string) bool {
	if s.IsMasterAdmin {
		return true
	}
	_, ok := s.Organizations[orgID]
	return ok
}

// CanWriteOrg checks if the user has at least member access to an organization.
func (s *UserScope) CanWriteOrg(orgID string) bool {
	if s.IsMasterAdmin {
		return true
	}
	role, ok := s.Organizations[orgID]
	if !ok {
		return false
	}
	return role == "owner" || role == "admin" || role == "member"
}

// CanAdminOrg checks if the user has admin or owner access to an organization.
func (s *UserScope) CanAdminOrg(orgID string) bool {
	if s.IsMasterAdmin {
		return true
	}
	role, ok := s.Organizations[orgID]
	if !ok {
		return false
	}
	return role == "owner" || role == "admin"
}

// CanOwnerOrg checks if the user has owner access to an organization.
func (s *UserScope) CanOwnerOrg(orgID string) bool {
	if s.IsMasterAdmin {
		return true
	}
	role, ok := s.Organizations[orgID]
	return ok && role == "owner"
}

// IsOwner checks if the user has owner role in the endeavour.
func (s *UserScope) IsOwner(endeavourID string) bool {
	if s.IsMasterAdmin {
		return true
	}
	role, ok := s.Endeavours[endeavourID]
	return ok && role == "owner"
}

// ResolveUserScope queries the database to build the full permission set for a user.
func (svc *Service) ResolveUserScope(ctx context.Context, userID string) (*UserScope, error) {
	scope := &UserScope{
		UserID:        userID,
		Endeavours:    make(map[string]string),
		Organizations: make(map[string]string),
	}

	// Check master admin (still populate memberships below for scoped views)
	scope.IsMasterAdmin = svc.IsMasterAdmin(ctx, userID)

	// Direct user -> endeavour memberships (via entity_relation member_of)
	rows, err := svc.db.Query(
		`SELECT target_entity_id, json_extract(metadata, '$.role')
		 FROM entity_relation
		 WHERE source_entity_type = 'user'
		   AND source_entity_id = ?
		   AND target_entity_type = 'endeavour'
		   AND relationship_type = 'member_of'`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("query user endeavours: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var edvID string
		var role sql.NullString
		if err := rows.Scan(&edvID, &role); err != nil {
			continue
		}
		r := "member"
		if role.Valid && role.String != "" {
			r = role.String
		}
		scope.Endeavours[edvID] = r
	}

	// Organization memberships (via user.resource_id -> entity_relation has_member)
	rows2, err := svc.db.Query(
		`SELECT er.source_entity_id, json_extract(er.metadata, '$.role')
		 FROM entity_relation er
		 JOIN user u ON u.resource_id = er.target_entity_id
		 WHERE u.id = ?
		   AND er.source_entity_type = 'organization'
		   AND er.relationship_type = 'has_member'
		   AND er.target_entity_type = 'resource'`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("query user organizations: %w", err)
	}
	defer func() { _ = rows2.Close() }()
	for rows2.Next() {
		var orgID string
		var role sql.NullString
		if err := rows2.Scan(&orgID, &role); err != nil {
			continue
		}
		r := "member"
		if role.Valid && role.String != "" {
			r = role.String
		}
		scope.Organizations[orgID] = r
	}

	// Expand org membership to endeavour access:
	// For each org, find endeavours the org participates in.
	for orgID, orgRole := range scope.Organizations {
		rows3, err := svc.db.Query(
			`SELECT target_entity_id FROM entity_relation
			 WHERE source_entity_type = 'organization' AND source_entity_id = ?
			   AND relationship_type = 'participates_in'
			   AND target_entity_type = 'endeavour'`,
			orgID,
		)
		if err != nil {
			continue
		}
		for rows3.Next() {
			var edvID string
			if err := rows3.Scan(&edvID); err != nil {
				continue
			}
			// Direct membership takes precedence over org-derived access
			if _, exists := scope.Endeavours[edvID]; !exists {
				// Map org role to endeavour role
				edvRole := orgRoleToEndeavourRole(orgRole)
				scope.Endeavours[edvID] = edvRole
			}
		}
		_ = rows3.Close()
	}

	return scope, nil
}

// orgRoleToEndeavourRole maps an organization role to the equivalent endeavour role.
func orgRoleToEndeavourRole(orgRole string) string {
	switch orgRole {
	case "owner":
		return "admin"
	case "admin":
		return "admin"
	case "member":
		return "member"
	case "guest":
		return "viewer"
	default:
		return "viewer"
	}
}

// GetUserScope returns the resolved user scope from context, if any.
func GetUserScope(ctx context.Context) *UserScope {
	if scope, ok := ctx.Value(UserScopeKey).(*UserScope); ok {
		return scope
	}
	return nil
}

// WithUserScope returns a new context with the given UserScope.
func WithUserScope(ctx context.Context, scope *UserScope) context.Context {
	return context.WithValue(ctx, UserScopeKey, scope)
}

// ValidatePassword checks password strength requirements.
func ValidatePassword(password string) error {
	if len(password) < 12 {
		return fmt.Errorf("password must be at least 12 characters")
	}
	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, ch := range password {
		switch {
		case ch >= 'A' && ch <= 'Z':
			hasUpper = true
		case ch >= 'a' && ch <= 'z':
			hasLower = true
		case ch >= '0' && ch <= '9':
			hasDigit = true
		default:
			hasSpecial = true
		}
	}
	if !hasUpper {
		return fmt.Errorf("password must contain at least one uppercase letter")
	}
	if !hasLower {
		return fmt.Errorf("password must contain at least one lowercase letter")
	}
	if !hasDigit {
		return fmt.Errorf("password must contain at least one digit")
	}
	if !hasSpecial {
		return fmt.Errorf("password must contain at least one special character")
	}
	return nil
}

// HashPassword generates a bcrypt hash of the given password.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

// GetUserOrganizations returns the organizations the user belongs to.
func (s *Service) GetUserOrganizations(ctx context.Context, userID string) ([]map[string]interface{}, error) {
	rows, err := s.db.Query(
		`SELECT o.id, o.name, er.metadata, er.created_at
		 FROM organization o
		 JOIN entity_relation er ON o.id = er.source_entity_id
		     AND er.source_entity_type = 'organization'
		     AND er.relationship_type = 'has_member'
		     AND er.target_entity_type = 'resource'
		 JOIN user u ON u.resource_id = er.target_entity_id
		 WHERE u.id = ? AND o.status = 'active'`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var orgs []map[string]interface{}
	for rows.Next() {
		var id, name, metadataJSON, createdAt string
		if err := rows.Scan(&id, &name, &metadataJSON, &createdAt); err != nil {
			continue
		}
		role := "member"
		var meta map[string]interface{}
		if err := json.Unmarshal([]byte(metadataJSON), &meta); err == nil {
			if r, ok := meta["role"].(string); ok {
				role = r
			}
		}
		orgs = append(orgs, map[string]interface{}{
			"id":        id,
			"name":      name,
			"role":      role,
			"joined_at": createdAt,
		})
	}

	return orgs, nil
}
