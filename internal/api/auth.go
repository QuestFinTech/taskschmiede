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


package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/QuestFinTech/taskschmiede/internal/auth"
	"github.com/QuestFinTech/taskschmiede/internal/email"
	"github.com/QuestFinTech/taskschmiede/internal/security"
	"github.com/QuestFinTech/taskschmiede/internal/storage"
)

// auditSource reads the X-Source header from the request and returns a
// validated source string. Defaults to "api" for direct REST calls.
func auditSource(r *http.Request) string {
	switch r.Header.Get("X-Source") {
	case "console":
		return "console"
	case "portal":
		return "portal"
	default:
		return "api"
	}
}

// handleAuthLogin handles POST /api/v1/auth/login.
func (a *API) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Invalid JSON body")
		return
	}

	if body.Email == "" || body.Password == "" {
		writeError(w, http.StatusBadRequest, "invalid_input", "Email and password are required")
		return
	}

	// Auth-specific rate limiting (per IP)
	if a.rateLimiter != nil && !a.rateLimiter.AllowAuth(security.ExtractIP(r)) {
		if a.auditSvc != nil {
			a.auditSvc.Log(&security.AuditEntry{
				Action:     security.AuditRateLimitHit,
				ActorType:  "anonymous",
				IP:         security.ExtractIP(r),
				Source:     auditSource(r),
				Method:     r.Method,
				Endpoint:   r.URL.Path,
				StatusCode: http.StatusTooManyRequests,
				Metadata:   map[string]interface{}{"tier": "auth-endpoint", "email": body.Email},
			})
		}
		w.Header().Set("Retry-After", "60")
		writeError(w, http.StatusTooManyRequests, "rate_limited", "Too many login attempts. Please wait and try again.")
		return
	}

	// Authenticate
	user, err := a.authSvc.Authenticate(r.Context(), body.Email, body.Password)
	if err != nil {
		if a.auditSvc != nil {
			a.auditSvc.Log(&security.AuditEntry{
				Action:    security.AuditLoginFailure,
				ActorType: "anonymous",
				IP:        security.ExtractIP(r),
				Source:    auditSource(r),
				Metadata:  map[string]interface{}{"email": body.Email},
			})
		}
		if errors.Is(err, auth.ErrAccountSuspended) {
			writeError(w, http.StatusForbidden, "account_suspended", "Account is suspended")
			return
		}
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "Invalid email or password")
		return
	}

	// Check if TOTP 2FA is enabled for this user.
	totpState, totpErr := a.db.GetUserTOTP(user.ID)
	if totpErr == nil && totpState.EnabledAt != nil {
		// 2FA is required. Issue a short-lived challenge token instead of a session.
		challengeToken := storePending2FA(user.ID)

		if a.auditSvc != nil {
			a.auditSvc.Log(&security.AuditEntry{
				Action:    "login_2fa_pending",
				ActorID:   user.ID,
				ActorType: "user",
				IP:        security.ExtractIP(r),
				Source:    auditSource(r),
				Metadata:  map[string]interface{}{"email": body.Email},
			})
		}

		a.logger.Info("2FA required for login", "user_id", user.ID, "email", body.Email)

		writeData(w, http.StatusOK, map[string]interface{}{
			"status":        "2fa_required",
			"pending_token": challengeToken,
			"user_id":       user.ID,
			"message":       "Two-factor authentication is required. Submit your TOTP code to POST /api/v1/auth/totp/verify.",
		})
		return
	}

	// Create token
	ttl := a.authSvc.DefaultTokenTTL(r.Context())
	exp := storage.UTCNow().Add(ttl)
	token, tokenID, err := a.authSvc.CreateToken(r.Context(), user.ID, "rest-session", &exp)
	if err != nil {
		a.logger.Error("Failed to create token", "user_id", user.ID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to create access token")
		return
	}

	if a.auditSvc != nil {
		a.auditSvc.Log(&security.AuditEntry{
			Action:    security.AuditLoginSuccess,
			ActorID:   user.ID,
			ActorType: "user",
			IP:        security.ExtractIP(r),
			Source:    auditSource(r),
			Metadata:  map[string]interface{}{"email": body.Email},
		})
	}

	a.db.IncrementLoginCount(user.ID)

	a.logger.Info("User authenticated via REST", "user_id", user.ID, "email", body.Email)

	writeData(w, http.StatusOK, map[string]interface{}{
		"token":      token,
		"token_id":   tokenID,
		"user_id":    user.ID,
		"name":       user.Name,
		"email":      user.Email,
		"expires_at": exp.Format(time.RFC3339),
	})
}

// handleAuthVerify handles POST /api/v1/auth/verify.
func (a *API) handleAuthVerify(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email string `json:"email"`
		Code  string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Invalid JSON body")
		return
	}

	if body.Email == "" || body.Code == "" {
		writeError(w, http.StatusBadRequest, "invalid_input", "Email and code are required")
		return
	}

	user, token, err := a.db.VerifyAndCreateUser(body.Email, body.Code)
	if err != nil {
		writeError(w, http.StatusBadRequest, "verification_failed", err.Error())
		return
	}

	a.logger.Info("User verified via REST", "user_id", user.ID, "email", user.Email)

	writeData(w, http.StatusOK, map[string]interface{}{
		"status":  "verified",
		"token":   token,
		"user_id": user.ID,
		"name":    user.Name,
		"email":   user.Email,
	})
}

// handleCompleteProfile handles POST /api/v1/auth/complete-profile.
// Called after email verification to collect address, consent, and create
// the auto-organization. Only users without a person record can call this.
func (a *API) handleCompleteProfile(w http.ResponseWriter, r *http.Request) {
	authUser := auth.GetAuthUser(r.Context())
	if authUser == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	user, err := a.db.GetUser(authUser.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Failed to get user")
		return
	}

	// Check if profile is already complete (person record exists).
	if person, _ := a.db.GetPersonByUserID(user.ID); person != nil {
		writeError(w, http.StatusConflict, "already_complete", "Profile is already complete")
		return
	}

	var body struct {
		Street              string `json:"street"`
		Street2             string `json:"street2"`
		PostalCode          string `json:"postal_code"`
		City                string `json:"city"`
		State               string `json:"state"`
		Country             string `json:"country"`
		CompanyRegistration string `json:"company_registration"`
		VATNumber           string `json:"vat_number"`
		AcceptTerms         bool   `json:"accept_terms"`
		AcceptPrivacy       bool   `json:"accept_privacy"`
		AcceptDPA           bool   `json:"accept_dpa"`
		AgeDeclaration      bool   `json:"age_declaration"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Invalid JSON body")
		return
	}

	if body.Street == "" || body.PostalCode == "" || body.City == "" || body.Country == "" {
		writeError(w, http.StatusBadRequest, "invalid_input", "Address fields are required (street, postal_code, city, country)")
		return
	}
	if !body.AcceptTerms || !body.AcceptPrivacy {
		writeError(w, http.StatusBadRequest, "invalid_input", "Terms and privacy acceptance are required")
		return
	}
	if !body.AgeDeclaration {
		writeError(w, http.StatusBadRequest, "invalid_input", "Age declaration is required")
		return
	}

	// Look up the pending_user identity data to get account_type, first_name, etc.
	// These were stored at registration time. Since VerifyAndCreateUser no longer
	// records identity, we read the person's name and account type from the user record
	// and the pending_user columns that were copied to the user.
	nameParts := strings.SplitN(user.Name, " ", 2)
	firstName := nameParts[0]
	lastName := ""
	if len(nameParts) > 1 {
		lastName = nameParts[1]
	}

	// Determine account type from user metadata or default to private.
	accountType := "private"
	if user.Metadata != nil {
		if at, ok := user.Metadata["account_type"]; ok {
			if s, ok := at.(string); ok {
				accountType = s
			}
		}
	}

	if accountType == "business" && !body.AcceptDPA {
		writeError(w, http.StatusBadRequest, "invalid_input", "DPA acceptance is required for business accounts")
		return
	}

	identity := &storage.RegistrationIdentity{
		AccountType:         accountType,
		FirstName:           firstName,
		LastName:            lastName,
		CompanyName:         "",
		CompanyRegistration: body.CompanyRegistration,
		VATNumber:           body.VATNumber,
		Street:              body.Street,
		Street2:             body.Street2,
		PostalCode:          body.PostalCode,
		City:                body.City,
		State:               body.State,
		Country:             body.Country,
		AcceptDPA:           body.AcceptDPA,
	}

	// Get company name from user metadata if business account.
	if accountType == "business" && user.Metadata != nil {
		if cn, ok := user.Metadata["company_name"]; ok {
			if s, ok := cn.(string); ok {
				identity.CompanyName = s
			}
		}
	}

	// Record identity (person + address + consent).
	ipAddress := r.RemoteAddr
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		ipAddress = strings.SplitN(fwd, ",", 2)[0]
	}
	userAgent := r.Header.Get("User-Agent")
	_, identErr := a.db.RecordRegistrationIdentity(user.ID, identity, ipAddress, userAgent)
	if identErr != nil {
		a.logger.Error("Failed to record registration identity", "user_id", user.ID, "error", identErr)
		writeError(w, http.StatusInternalServerError, "internal", "Failed to save profile")
		return
	}

	// Auto-create personal organization.
	orgName := user.Name
	if accountType == "business" && identity.CompanyName != "" {
		orgName = identity.CompanyName
	}
	if orgName != "" && user.ResourceID != nil && *user.ResourceID != "" {
		org, orgErr := a.db.CreateOrganization(orgName, "", nil)
		if orgErr == nil {
			_ = a.db.AddResourceToOrganization(org.ID, *user.ResourceID, "owner")
		}
	}

	a.logger.Info("Profile completed", "user_id", user.ID, "email", user.Email)

	writeData(w, http.StatusOK, map[string]interface{}{
		"status": "complete",
	})
}

// handleWaitlist checks instance capacity and, if at capacity, either confirms
// the user is already waitlisted or adds them to the waitlist. It writes the
// HTTP response and returns true if the caller should return (user was
// waitlisted). Returns false if the instance has capacity and registration
// should proceed.
func (a *API) handleWaitlist(w http.ResponseWriter, email, name, passwordHash, invTokenID, userType string) bool {
	if !a.IsAtCapacity() {
		return false
	}

	if a.db.IsEmailOnWaitlist(email) {
		pos := a.db.GetWaitlistPosition(email)
		writeData(w, http.StatusAccepted, map[string]interface{}{
			"status":   "waitlisted",
			"position": pos,
			"message":  "You are already on the waitlist. You will be notified when a slot becomes available.",
		})
		return true
	}

	entry, wlErr := a.db.AddToWaitlist(email, name, passwordHash, invTokenID, userType)
	if wlErr != nil {
		a.logger.Error("Failed to add to waitlist", "error", wlErr)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to process registration")
		return true
	}

	// Increment invitation token usage after successful waitlist entry.
	if invTokenID != "" {
		if err := a.db.IncrementInvitationTokenUse(invTokenID); err != nil {
			a.logger.Warn("Failed to increment invitation token usage", "error", err)
		}
	}

	pos := a.db.GetWaitlistPosition(email)
	a.logger.Info("Registration waitlisted (REST)",
		"email", email, "waitlist_id", entry.ID, "position", pos)

	writeData(w, http.StatusAccepted, map[string]interface{}{
		"status":   "waitlisted",
		"position": pos,
		"message":  "The instance is currently at capacity. You have been added to the waitlist and will be notified via email when a slot becomes available.",
	})
	return true
}

// autoCreatePersonalOrg creates a personal organization for a newly registered
// user and adds their resource as owner.
func (a *API) autoCreatePersonalOrg(user *storage.User, identity *storage.RegistrationIdentity) {
	if user.ResourceID == nil || *user.ResourceID == "" {
		return
	}
	orgName := user.Name
	if identity != nil && identity.AccountType == "business" && identity.CompanyName != "" {
		orgName = identity.CompanyName
	}
	if orgName == "" {
		return
	}
	org, orgErr := a.db.CreateOrganization(orgName, "", nil)
	if orgErr == nil {
		_ = a.db.AddResourceToOrganization(org.ID, *user.ResourceID, "owner")
	}
}

// registerHumanWithToken handles direct user creation for human registrations
// that include a valid invitation token (email verification skipped).
func (a *API) registerHumanWithToken(w http.ResponseWriter, body *registerBody, invTokenID string, identity *storage.RegistrationIdentity) {
	passwordHash, err := auth.HashPassword(body.Password)
	if err != nil {
		a.logger.Error("Failed to hash password", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to process registration")
		return
	}

	if a.handleWaitlist(w, body.Email, body.Name, passwordHash, invTokenID, body.UserType) {
		return
	}

	user, token, err := a.db.CreateUserWithInvitation(body.Email, body.Name, passwordHash, invTokenID, body.UserType, body.Lang, identity)
	if err != nil {
		if err.Error() == "email already registered" || err.Error() == "check email: email already exists" {
			writeError(w, http.StatusConflict, "conflict", "Email already exists")
			return
		}
		a.logger.Error("Failed to create user with invitation", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to create registration")
		return
	}

	// Increment invitation token usage only after successful user creation.
	if err := a.db.IncrementInvitationTokenUse(invTokenID); err != nil {
		a.logger.Warn("Failed to increment invitation token usage", "error", err)
	}

	a.autoCreatePersonalOrg(user, identity)

	a.logger.Info("REST registration via invitation token",
		"user_id", user.ID, "email", body.Email, "user_type", body.UserType, "invitation_token_id", invTokenID)

	writeData(w, http.StatusCreated, map[string]interface{}{
		"status":             "active",
		"user_id":            user.ID,
		"token":              token,
		"email":              user.Email,
		"name":               user.Name,
		"onboarding_status":  "interview_pending",
		"message":            "Account created successfully. Complete the onboarding interview to access production tools.",
	})
}

// registerAgentTrusted handles agent registration without email verification
// (trusted mode, when requireAgentEmailVerification is false).
func (a *API) registerAgentTrusted(w http.ResponseWriter, body *registerBody, invTokenID string, identity *storage.RegistrationIdentity) {
	passwordHash, err := auth.HashPassword(body.Password)
	if err != nil {
		a.logger.Error("Failed to hash password", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to process registration")
		return
	}

	if a.handleWaitlist(w, body.Email, body.Name, passwordHash, invTokenID, body.UserType) {
		return
	}

	user, token, err := a.db.CreateUserWithInvitation(body.Email, body.Name, passwordHash, invTokenID, body.UserType, body.Lang, identity)
	if err != nil {
		if err.Error() == "email already registered" || err.Error() == "check email: email already exists" {
			writeError(w, http.StatusConflict, "conflict", "Email already exists")
			return
		}
		a.logger.Error("Failed to create agent user", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to create registration")
		return
	}

	// Increment invitation token usage only after successful user creation.
	if err := a.db.IncrementInvitationTokenUse(invTokenID); err != nil {
		a.logger.Warn("Failed to increment invitation token usage", "error", err)
	}

	a.autoCreatePersonalOrg(user, identity)

	onboardingStatus := "interview_pending"
	if !a.requireAgentInterview {
		onboardingStatus = "interview_skipped"
		if _, err := a.db.Exec(`UPDATE user SET onboarding_status = 'interview_skipped' WHERE id = ?`, user.ID); err != nil {
			a.logger.Error("Failed to update onboarding status", "error", err)
		}
	}

	a.logger.Info("REST agent registration (email verification skipped)",
		"user_id", user.ID, "email", body.Email, "deployment_mode", a.deploymentMode,
		"onboarding_status", onboardingStatus)

	resp := map[string]interface{}{
		"status":            "active",
		"user_id":           user.ID,
		"token":             token,
		"email":             user.Email,
		"name":              user.Name,
		"onboarding_status": onboardingStatus,
	}
	if onboardingStatus == "interview_pending" {
		resp["message"] = "Account created. Complete the onboarding interview to access production tools."
	} else {
		resp["message"] = "Account created with full access."
	}
	writeData(w, http.StatusCreated, resp)
}

// registerAgentAtCapacity handles the case where an agent registration with
// invitation token arrives but the instance is at capacity. The user is added
// to the waitlist before email verification would begin.
func (a *API) registerAgentAtCapacity(w http.ResponseWriter, body *registerBody, invTokenID string) {
	passwordHash, err := auth.HashPassword(body.Password)
	if err != nil {
		a.logger.Error("Failed to hash password", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to process registration")
		return
	}

	// handleWaitlist always returns true here because the caller already
	// confirmed IsAtCapacity, but we call through the helper for consistency.
	a.handleWaitlist(w, body.Email, body.Name, passwordHash, invTokenID, body.UserType)
}

// registerWithEmailVerification handles the email verification path for
// humans without tokens and agents with tokens.
func (a *API) registerWithEmailVerification(w http.ResponseWriter, body *registerBody, invTokenID string, identity *storage.RegistrationIdentity) {
	passwordHash, err := auth.HashPassword(body.Password)
	if err != nil {
		a.logger.Error("Failed to hash password", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to process registration")
		return
	}
	if a.handleWaitlist(w, body.Email, body.Name, passwordHash, invTokenID, body.UserType) {
		return
	}

	pending, err := a.db.CreatePendingUser(body.Email, body.Name, body.Password, invTokenID, body.UserType, body.Lang, 15*time.Minute, identity)
	if err != nil {
		if err.Error() == "email already registered" {
			writeError(w, http.StatusConflict, "conflict", "Email already exists")
			return
		}
		a.logger.Error("Failed to create pending user", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to create registration")
		return
	}

	// Increment invitation token usage only after successful pending user creation.
	if invTokenID != "" {
		if err := a.db.IncrementInvitationTokenUse(invTokenID); err != nil {
			a.logger.Warn("Failed to increment invitation token usage", "error", err)
		}
	}

	if a.emailSender != nil {
		a.sendVerificationEmail(body.Email, body.Name, pending.VerificationCode, "en", false)
	}

	a.logger.Info("REST registration started", "email", body.Email, "name", body.Name, "user_type", body.UserType)

	writeData(w, http.StatusCreated, map[string]interface{}{
		"status":     "pending_verification",
		"email":      body.Email,
		"expires_in": fmt.Sprintf("%v", 15*time.Minute),
		"message":    fmt.Sprintf("A verification code has been sent to %s. Check your inbox and call POST /api/v1/auth/verify with the code to activate your account.", body.Email),
	})
}

// registerBody holds the decoded JSON body for handleAuthRegister.
type registerBody struct {
	InvitationToken string `json:"invitation_token"`
	Email           string `json:"email"`
	Name            string `json:"name"`
	Password        string `json:"password"`
	UserType        string `json:"user_type"`
	Lang            string `json:"lang"`
	// Identity fields
	AccountType         string `json:"account_type"`
	FirstName           string `json:"first_name"`
	LastName            string `json:"last_name"`
	CompanyName         string `json:"company_name"`
	CompanyRegistration string `json:"company_registration"`
	VATNumber           string `json:"vat_number"`
	Street              string `json:"street"`
	Street2             string `json:"street2"`
	PostalCode          string `json:"postal_code"`
	City                string `json:"city"`
	State               string `json:"state"`
	Country             string `json:"country"`
	AcceptTerms         bool   `json:"accept_terms"`
	AcceptPrivacy       bool   `json:"accept_privacy"`
	AcceptDPA           bool   `json:"accept_dpa"`
	AgeDeclaration      bool   `json:"age_declaration"`
}

// handleAuthRegister handles POST /api/v1/auth/register.
// Human registration without token: creates pending user with email verification.
// Human registration with token: creates user directly (token = authorization).
// Agent registration with token: creates pending user with email verification (proves email capability).
func (a *API) handleAuthRegister(w http.ResponseWriter, r *http.Request) {
	var body registerBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Invalid JSON body")
		return
	}

	if body.Email == "" || body.Name == "" || body.Password == "" {
		writeError(w, http.StatusBadRequest, "invalid_input", "Email, name, and password are required")
		return
	}

	// Auth-specific rate limiting (per IP)
	if a.rateLimiter != nil && !a.rateLimiter.AllowAuth(security.ExtractIP(r)) {
		w.Header().Set("Retry-After", "60")
		writeError(w, http.StatusTooManyRequests, "rate_limited", "Too many requests. Please wait and try again.")
		return
	}

	// Validate password
	if err := auth.ValidatePassword(body.Password); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
		return
	}

	// For human registrations, validate identity and consent fields.
	if body.UserType == "" || body.UserType == "human" {
		// Build name from first_name + last_name if provided
		if body.FirstName != "" && body.LastName != "" {
			body.Name = body.FirstName + " " + body.LastName
		}

		// Validate consent and address for token-based registration (all data upfront).
		// Self-registration (no token) uses two-phase flow: consent and address are
		// collected in the complete-profile step after email verification.
		if body.InvitationToken != "" && body.FirstName != "" {
			if !body.AcceptTerms || !body.AcceptPrivacy {
				writeError(w, http.StatusBadRequest, "consent_required", "You must accept the Terms and Conditions and Privacy Policy")
				return
			}
			if !body.AgeDeclaration {
				writeError(w, http.StatusBadRequest, "age_required", "You must confirm you are at least 16 years old")
				return
			}
			if body.AccountType == "business" && body.CompanyName == "" {
				writeError(w, http.StatusBadRequest, "invalid_input", "Company name is required for business accounts")
				return
			}
			if body.AccountType == "business" && !body.AcceptDPA {
				writeError(w, http.StatusBadRequest, "dpa_required", "Business accounts must accept the Data Processing Agreement")
				return
			}
		}
		if body.FirstName != "" {
			if body.AccountType == "" {
				body.AccountType = "private"
			}
		}
	}

	// Default user type
	if body.UserType == "" {
		body.UserType = "human"
	}
	if body.UserType != "human" && body.UserType != "agent" {
		writeError(w, http.StatusBadRequest, "invalid_input", "User type must be 'human' or 'agent'")
		return
	}

	// Agent registration requires an invitation token from a human operator.
	if body.UserType == "agent" && body.InvitationToken == "" {
		writeError(w, http.StatusBadRequest, "missing_token",
			"Agent registration requires an invitation token. Human operators can create tokens via the web console.")
		return
	}

	// Validate invitation token if provided
	var invTokenID string
	if body.InvitationToken != "" {
		inv, err := a.db.ValidateInvitationToken(body.InvitationToken)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_token", "Invalid or expired invitation token")
			return
		}
		invTokenID = inv.ID
	}

	// Check agent-per-org quota before creating user.
	if invTokenID != "" && body.UserType == "agent" {
		invDetails, invErr := a.db.GetInvitationTokenByID(invTokenID)
		if invErr == nil && invDetails.CreatedBy != "" {
			sponsorOrgID := invDetails.OrganizationID
			if sponsorOrgID == "" {
				sponsorOrgID = a.findUserOrgID(r.Context(), invDetails.CreatedBy)
			}
			if sponsorOrgID != "" {
				if apiErr := a.CheckOrgQuotaForUser(r.Context(), invDetails.CreatedBy, sponsorOrgID, "agents_per_org"); apiErr != nil {
					writeAPIError(w, apiErr)
					return
				}
			}
		}
	}

	// Build registration identity if identity fields were provided.
	var identity *storage.RegistrationIdentity
	if body.FirstName != "" {
		identity = &storage.RegistrationIdentity{
			AccountType:         body.AccountType,
			FirstName:           body.FirstName,
			LastName:            body.LastName,
			CompanyName:         body.CompanyName,
			CompanyRegistration: body.CompanyRegistration,
			VATNumber:           body.VATNumber,
			Street:              body.Street,
			Street2:             body.Street2,
			PostalCode:          body.PostalCode,
			City:                body.City,
			State:               body.State,
			Country:             body.Country,
			AcceptDPA:           body.AcceptDPA,
		}
	}

	// Block self-registration (no token) when disabled.
	if invTokenID == "" && !a.allowSelfRegistration {
		writeError(w, http.StatusForbidden, "self_registration_disabled",
			"Self-registration is not available. Contact an administrator for access.")
		return
	}

	// Route to the appropriate registration path.
	switch {
	// Human with invitation token: create user directly (skip email verification).
	case invTokenID != "" && body.UserType != "agent":
		a.registerHumanWithToken(w, &body, invTokenID, identity)

	// Agent in trusted mode (no email verification required).
	case invTokenID != "" && body.UserType == "agent" && !a.requireAgentEmailVerification:
		a.registerAgentTrusted(w, &body, invTokenID, identity)

	// Agent with invitation token but instance at capacity: waitlist before verification.
	case invTokenID != "" && body.UserType == "agent" && a.IsAtCapacity():
		a.registerAgentAtCapacity(w, &body, invTokenID)

	// Email verification path: humans without tokens and agents with tokens.
	default:
		a.registerWithEmailVerification(w, &body, invTokenID, identity)
	}
}

// Whoami returns the full context for the authenticated user, including
// tier info, limits, usage, and scope. Both REST and MCP call this.
func (a *API) Whoami(ctx context.Context) (map[string]interface{}, *APIError) {
	authUser := auth.GetAuthUser(ctx)
	if authUser == nil {
		return nil, errUnauthorized("Authentication required")
	}

	user, err := a.db.GetUser(authUser.UserID)
	if err != nil {
		return nil, errInternal("Failed to get user")
	}

	scope, _ := a.authSvc.ResolveUserScope(ctx, authUser.UserID)

	result := map[string]interface{}{
		"user_id":    user.ID,
		"name":       user.Name,
		"email":      user.Email,
		"status":     user.Status,
		"user_type":  user.UserType,
		"lang":       user.Lang,
		"timezone":   user.Timezone,
		"email_copy":   user.EmailCopy,
		"login_count":  user.LoginCount,
		"tier":         user.Tier,
		"tier_name":    a.db.TierName(user.Tier),
	}
	// Profile completion: check if person record exists.
	// Master admins (setup flow) are always considered complete.
	// When require-kyc is disabled, all profiles are considered complete.
	requireKYC, _ := a.db.GetPolicy("registration.require_kyc")
	if a.authSvc.IsMasterAdmin(ctx, authUser.UserID) || requireKYC == "false" {
		result["profile_complete"] = true
	} else {
		person, _ := a.db.GetPersonByUserID(user.ID)
		result["profile_complete"] = person != nil
	}

	// Account type from user metadata (captured at registration).
	if user.Metadata != nil {
		if at, ok := user.Metadata["account_type"]; ok {
			result["account_type"] = at
		}
	}

	if user.ResourceID != nil && *user.ResourceID != "" {
		result["resource_id"] = *user.ResourceID
	}
	if user.LastActiveAt != nil {
		result["last_active_at"] = user.LastActiveAt.Format(time.RFC3339)
	}

	// Tier limits from tier definition table.
	tierUsage := map[string]interface{}{}
	td, tdErr := a.db.GetTierDefinition(user.Tier)
	if tdErr == nil && td != nil {
		result["tier_limits"] = map[string]int{
			"max_orgs":               td.MaxOrgs,
			"max_active_endeavours":  td.MaxActiveEndeavours,
			"max_endeavours_per_org": td.MaxEndeavoursPerOrg,
			"max_agents_per_org":     td.MaxAgentsPerOrg,
			"max_teams_per_org":      td.MaxTeamsPerOrg,
			"max_creations_per_hour": td.MaxCreationsPerHour,
		}
	}
	tierUsage["orgs"] = a.countUserOwnedOrgs(ctx, user.ID)
	tierUsage["active_endeavours"] = a.countUserOwnedActiveEndeavours(ctx, user.ID)

	// Per-org usage for the user's primary org.
	orgID := a.findUserOrgID(ctx, user.ID)
	if orgID != "" {
		tierUsage["org_id"] = orgID
		tierUsage["endeavours_per_org"] = a.countOrgEndeavours(ctx, orgID)
		tierUsage["agents_per_org"] = a.countOrgAgents(ctx, orgID)
		tierUsage["teams_per_org"] = a.countOrgTeams(ctx, orgID)
	} else {
		tierUsage["endeavours_per_org"] = 0
		tierUsage["agents_per_org"] = 0
		tierUsage["teams_per_org"] = 0
	}

	tierUsage["creations_per_hour"] = a.CreationVelocityCurrent(user.ID)

	result["tier_usage"] = tierUsage

	// Check for pending consent (terms/privacy version updates).
	// When KYC is not required, consent is not collected, so skip the check.
	if requireKYC != "false" {
		pendingConsents, consentErr := a.db.GetPendingConsents(user.ID)
		if consentErr == nil && len(pendingConsents) > 0 {
			result["pending_consents"] = pendingConsents
		}
	}

	if scope != nil {
		result["is_admin"] = scope.IsMasterAdmin
		result["organizations"] = scope.Organizations
		result["endeavours"] = scope.Endeavours
	}

	// Agent monitoring data
	if user.UserType == "agent" {
		result["monitoring"] = a.getAgentMonitoring(user.ID)
	}

	// Admin-only: instance capacity and waitlist info
	if scope != nil && scope.IsMasterAdmin {
		maxUsers := a.policyInt("instance.max_active_users", 200)
		activeUsers := a.db.CountActiveUsers()
		waitlistCount := a.db.CountWaitlist()
		result["instance"] = map[string]interface{}{
			"max_active_users":    maxUsers,
			"active_users":        activeUsers,
			"at_capacity":         maxUsers > 0 && activeUsers >= maxUsers,
			"waitlist_count":      waitlistCount,
		}
	}

	return result, nil
}

// handleAuthWhoami handles GET /api/v1/auth/whoami.
func (a *API) handleAuthWhoami(w http.ResponseWriter, r *http.Request) {
	result, apiErr := a.Whoami(r.Context())
	if apiErr != nil {
		writeAPIError(w, apiErr)
		return
	}
	writeData(w, http.StatusOK, result)
}

// getAgentMonitoring returns health snapshot and Ablecon data for an agent user.
func (a *API) getAgentMonitoring(userID string) map[string]interface{} {
	monitoring := map[string]interface{}{}

	// Health snapshot (precomputed by taskgovernor ticker)
	snap, err := a.db.GetAgentHealthSnapshot(userID)
	if err == nil && snap != nil {
		monitoring["session_rate"] = snap.SessionRate
		monitoring["session_calls"] = snap.SessionCalls
		monitoring["rolling_24h_rate"] = snap.Rolling24hRate
		monitoring["rolling_24h_calls"] = snap.Rolling24hCalls
		monitoring["rolling_7d_rate"] = snap.Rolling7dRate
		monitoring["rolling_7d_calls"] = snap.Rolling7dCalls
		monitoring["health_status"] = snap.Status
	}

	// Ablecon levels
	ablecon := map[string]interface{}{}
	sysLevel, err := a.db.GetSystemAbleconLevel()
	if err == nil && sysLevel != nil {
		ablecon["system"] = map[string]interface{}{
			"level": sysLevel.Level,
			"label": ableconLabelAPI(sysLevel.Level),
		}
	}

	orgLevels, err := a.db.ListOrgAbleconLevels()
	if err == nil && len(orgLevels) > 0 {
		orgList := make([]map[string]interface{}, 0, len(orgLevels))
		for _, o := range orgLevels {
			orgList = append(orgList, map[string]interface{}{
				"org_id": o.ScopeID,
				"level":  o.Level,
				"label":  ableconLabelAPI(o.Level),
			})
		}
		ablecon["organizations"] = orgList
	}
	if len(ablecon) > 0 {
		monitoring["ablecon"] = ablecon
	}

	return monitoring
}

func ableconLabelAPI(level int) string {
	return storage.AbleconLevelLabel(level)
}

// handleAuthLogout handles POST /api/v1/auth/logout.
// Invalidates the current bearer token.
func (a *API) handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	token := auth.ExtractBearerToken(r.Header.Get("Authorization"))
	if token != "" {
		_ = a.authSvc.RevokeToken(r.Context(), token)
	}

	writeData(w, http.StatusOK, map[string]interface{}{
		"status": "logged_out",
	})
}

// handleForgotPassword handles POST /api/v1/auth/forgot-password.
// Creates a password reset code and sends it via email.
func (a *API) handleForgotPassword(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Invalid JSON body")
		return
	}

	if body.Email == "" {
		writeError(w, http.StatusBadRequest, "invalid_input", "Email is required")
		return
	}

	// Auth-specific rate limiting (per IP)
	if a.rateLimiter != nil && !a.rateLimiter.AllowAuth(security.ExtractIP(r)) {
		// Always return success to prevent email enumeration
		writeData(w, http.StatusOK, map[string]interface{}{
			"status": "reset_requested",
		})
		return
	}

	// Create password reset (returns nil if email not found)
	reset, err := a.db.CreatePasswordReset(body.Email, 15*time.Minute)
	if err != nil {
		a.logger.Warn("Password reset error", "error", err)
	}

	// Send email if reset was created
	if reset != nil && a.emailSender != nil {
		lang := a.db.GetUserLangByEmail(body.Email)
		name := a.db.GetUserNameByEmail(body.Email)
		a.sendVerificationEmail(body.Email, name, reset.Code, lang, true)
	}

	// Always return success to prevent email enumeration
	writeData(w, http.StatusOK, map[string]interface{}{
		"status": "reset_requested",
	})
}

// handleResetPassword handles POST /api/v1/auth/reset-password.
// Validates the reset code and sets a new password.
func (a *API) handleResetPassword(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email       string `json:"email"`
		Code        string `json:"code"`
		NewPassword string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Invalid JSON body")
		return
	}

	if body.Email == "" || body.Code == "" || body.NewPassword == "" {
		writeError(w, http.StatusBadRequest, "invalid_input", "Email, code, and new_password are required")
		return
	}

	if err := auth.ValidatePassword(body.NewPassword); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
		return
	}

	if err := a.db.CompletePasswordReset(body.Email, body.Code, body.NewPassword); err != nil {
		writeError(w, http.StatusBadRequest, "reset_failed", err.Error())
		return
	}

	writeData(w, http.StatusOK, map[string]interface{}{
		"status": "password_reset",
	})
}

// handleResendVerification handles POST /api/v1/auth/resend-verification.
// Resends the verification code for a pending user registration.
func (a *API) handleResendVerification(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Invalid JSON body")
		return
	}

	if body.Email == "" {
		writeError(w, http.StatusBadRequest, "invalid_input", "Email is required")
		return
	}

	newPending, err := a.db.RegeneratePendingUserCode(body.Email, 15*time.Minute)
	if err != nil {
		writeError(w, http.StatusBadRequest, "resend_failed", "No pending verification found for this email")
		return
	}

	// Send email
	if a.emailSender != nil {
		a.sendVerificationEmail(body.Email, newPending.Name, newPending.VerificationCode, "en", false)
	}

	writeData(w, http.StatusOK, map[string]interface{}{
		"status":     "code_resent",
		"email":      body.Email,
		"expires_at": newPending.ExpiresAt.Format(time.RFC3339),
		"message":    fmt.Sprintf("A new verification code has been sent to %s. Check your inbox.", body.Email),
	})
}

// handleAuthProfileUpdate handles PATCH /api/v1/auth/profile.
// Allows authenticated users to update their own name and email.
func (a *API) handleAuthProfileUpdate(w http.ResponseWriter, r *http.Request) {
	authUser := auth.GetAuthUser(r.Context())
	if authUser == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	var body struct {
		Name      *string `json:"name"`
		Email     *string `json:"email"`
		Lang      *string `json:"lang"`
		Timezone  *string `json:"timezone"`
		EmailCopy *bool   `json:"email_copy"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Invalid JSON body")
		return
	}

	fields := storage.UpdateUserFields{
		Name:      body.Name,
		Email:     body.Email,
		Lang:      body.Lang,
		Timezone:  body.Timezone,
		EmailCopy: body.EmailCopy,
	}

	if body.Name == nil && body.Email == nil && body.Lang == nil && body.Timezone == nil && body.EmailCopy == nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "At least one of name, email, lang, timezone, or email_copy must be provided")
		return
	}

	if body.Name != nil && *body.Name == "" {
		writeError(w, http.StatusBadRequest, "invalid_input", "Name cannot be empty")
		return
	}

	if body.Email != nil && *body.Email == "" {
		writeError(w, http.StatusBadRequest, "invalid_input", "Email cannot be empty")
		return
	}

	updated, err := a.db.UpdateUser(authUser.UserID, fields)
	if err != nil {
		if err == storage.ErrEmailExists {
			writeError(w, http.StatusConflict, "conflict", "Email already in use")
			return
		}
		if err == storage.ErrUserNotFound {
			writeError(w, http.StatusNotFound, "not_found", "User not found")
			return
		}
		a.logger.Error("Failed to update profile", "user_id", authUser.UserID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to update profile")
		return
	}

	if a.auditSvc != nil {
		a.auditSvc.Log(&security.AuditEntry{
			Action:    "profile_updated",
			ActorID:   authUser.UserID,
			ActorType: "user",
			IP:        security.ExtractIP(r),
			Source:    auditSource(r),
			Metadata:  map[string]interface{}{"fields": updated},
		})
	}

	writeData(w, http.StatusOK, map[string]interface{}{
		"status":         "updated",
		"updated_fields": updated,
	})
}

// handleAuthChangePassword handles POST /api/v1/auth/change-password.
// Changes the authenticated user's password. Requires current_password.
// All other sessions are invalidated on success.
func (a *API) handleAuthChangePassword(w http.ResponseWriter, r *http.Request) {
	authUser := auth.GetAuthUser(r.Context())
	if authUser == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	var body struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Invalid JSON body")
		return
	}

	if body.CurrentPassword == "" || body.NewPassword == "" {
		writeError(w, http.StatusBadRequest, "invalid_input", "current_password and new_password are required")
		return
	}

	if err := auth.ValidatePassword(body.NewPassword); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
		return
	}

	newHash, err := auth.HashPassword(body.NewPassword)
	if err != nil {
		a.logger.Error("Failed to hash password", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to process password change")
		return
	}

	if err := a.db.ChangeUserPassword(authUser.UserID, body.CurrentPassword, newHash); err != nil {
		if err == storage.ErrInvalidPassword {
			writeError(w, http.StatusBadRequest, "invalid_password", "Current password is incorrect")
			return
		}
		if err == storage.ErrUserNotFound {
			writeError(w, http.StatusNotFound, "not_found", "User not found")
			return
		}
		a.logger.Error("Failed to change password", "user_id", authUser.UserID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to change password")
		return
	}

	if a.auditSvc != nil {
		a.auditSvc.Log(&security.AuditEntry{
			Action:    "password_changed",
			ActorID:   authUser.UserID,
			ActorType: "user",
			IP:        security.ExtractIP(r),
			Source:    auditSource(r),
		})
	}

	a.logger.Info("User changed password", "user_id", authUser.UserID)

	writeData(w, http.StatusOK, map[string]interface{}{
		"status":  "password_changed",
		"message": "Password changed successfully. All other sessions have been invalidated.",
	})
}

// handleVerificationStatus handles GET /api/v1/auth/verification-status.
// Returns the status of a pending user verification.
func (a *API) handleVerificationStatus(w http.ResponseWriter, r *http.Request) {
	emailAddr := r.URL.Query().Get("email")
	if emailAddr == "" {
		writeError(w, http.StatusBadRequest, "invalid_input", "Email query parameter is required")
		return
	}

	pending, err := a.db.GetPendingUser(emailAddr)
	if err != nil || pending == nil {
		writeData(w, http.StatusOK, map[string]interface{}{
			"found": false,
		})
		return
	}

	now := storage.UTCNow()
	writeData(w, http.StatusOK, map[string]interface{}{
		"found":      true,
		"email":      pending.Email,
		"expired":    now.After(pending.ExpiresAt),
		"expires_at": pending.ExpiresAt.Format(time.RFC3339),
	})
}

// trySendVerificationEmail attempts to send a verification email and returns
// true if the email was sent successfully, false otherwise.
func (a *API) trySendVerificationEmail(toEmail, name, code, lang string, isReset bool) bool {
	if a.emailSender == nil {
		return false
	}
	return a.sendVerificationEmail(toEmail, name, code, lang, isReset)
}

// sendVerificationEmail sends a verification or password reset email using
// HTML templates when available, falling back to plain text.
// Returns true if the email was sent successfully.
func (a *API) sendVerificationEmail(toEmail, name, code, lang string, isReset bool) bool {
	t := func(key string, args ...interface{}) string {
		if a.i18n != nil {
			return a.i18n.T(lang, key, args...)
		}
		// Fallback when i18n is not initialized
		return key
	}

	greeting := t("email.greeting", name)
	if name == "" {
		greeting = t("email.greeting", "")
	}

	var subject, intro, buttonText, manualNote, expiryNote, actionURL string
	if isReset {
		subject = t("email.reset.subject")
		intro = t("email.reset.intro")
		buttonText = t("email.reset.button")
		manualNote = t("email.reset.manual_note")
		expiryNote = t("email.reset.expiry", "15 minutes")
		if a.portalURL != "" {
			actionURL = a.portalURL + "/reset-password?email=" + toEmail + "&code=" + code
		}
	} else {
		subject = t("email.verification.subject")
		intro = t("email.verification.intro")
		buttonText = t("email.verification.button")
		manualNote = t("email.verification.manual_note")
		expiryNote = t("email.verification.expiry", "15 minutes")
		if a.portalURL != "" {
			actionURL = a.portalURL + "/verify?email=" + toEmail + "&code=" + code
		}
	}

	if ts, ok := a.emailSender.(TemplatedEmailSender); ok {
		data := &email.VerificationCodeData{
			Greeting:   greeting,
			Intro:      intro,
			Code:       code,
			ExpiryNote: expiryNote,
			ActionURL:  actionURL,
			ButtonText: buttonText,
			ManualNote: manualNote,
			Closing:    t("email.closing"),
			TeamName:   t("email.team_name"),
		}
		if err := ts.SendVerificationCode(toEmail, subject, data); err != nil {
			a.logger.Warn("Failed to send templated email", "error", err)
			return false
		}
		return true
	}

	// Fallback to plain text
	body := fmt.Sprintf("%s,\n\n%s\n\n    %s\n\n%s\n\n%s\n%s",
		greeting, intro, code, expiryNote,
		t("email.closing"), t("email.team_name"))
	if err := a.emailSender.SendEmail(toEmail, subject, body); err != nil {
		a.logger.Warn("Failed to send email", "error", err)
		return false
	}
	return true
}
