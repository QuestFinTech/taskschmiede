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
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"image/png"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pquerna/otp/totp"
	"github.com/QuestFinTech/taskschmiede/internal/auth"
	"github.com/QuestFinTech/taskschmiede/internal/security"
	"github.com/QuestFinTech/taskschmiede/internal/storage"
)

// pending2FA tracks a 2FA challenge issued after successful password
// authentication. The challenge token is returned to the client and must
// be presented together with a valid TOTP code to obtain a full session.
type pending2FA struct {
	UserID    string
	ExpiresAt time.Time
	Attempts  int32 // atomic; max 5 before challenge is invalidated
}

// maxPending2FAAttempts is the maximum number of code verification attempts
// allowed per 2FA challenge before it is invalidated.
const maxPending2FAAttempts = 5

// pending2FAStore is a process-local store of outstanding 2FA challenges.
// Entries expire after 5 minutes and are cleaned up lazily on access.
var pending2FAStore sync.Map

// storePending2FA creates a 2FA challenge for the given user and returns
// the challenge token.
func storePending2FA(userID string) string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	token := hex.EncodeToString(b)
	pending2FAStore.Store(token, &pending2FA{
		UserID:    userID,
		ExpiresAt: storage.UTCNow().Add(5 * time.Minute),
	})
	return token
}

// getPending2FA retrieves a valid 2FA challenge without consuming it.
// Returns the user ID if the token is valid, not expired, and within
// the attempt limit. Each call increments the attempt counter; if the
// limit is exceeded the challenge is deleted and the call returns false.
func getPending2FA(token string) (string, bool) {
	val, ok := pending2FAStore.Load(token)
	if !ok {
		return "", false
	}
	p := val.(*pending2FA)
	if storage.UTCNow().After(p.ExpiresAt) {
		pending2FAStore.Delete(token)
		return "", false
	}
	if atomic.AddInt32(&p.Attempts, 1) > maxPending2FAAttempts {
		pending2FAStore.Delete(token)
		return "", false
	}
	return p.UserID, true
}

// consumePending2FA removes a 2FA challenge after successful verification.
func consumePending2FA(token string) {
	pending2FAStore.Delete(token)
}

// handleTOTPSetup handles POST /api/v1/auth/totp/setup (authenticated).
// Generates a new TOTP key for the user and returns the secret, provisioning
// URL, and a base64-encoded QR code PNG.
func (a *API) handleTOTPSetup(w http.ResponseWriter, r *http.Request) {
	authUser := auth.GetAuthUser(r.Context())
	if authUser == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	// Check if TOTP is already enabled.
	totpState, err := a.db.GetUserTOTP(authUser.UserID)
	if err != nil {
		a.logger.Error("Failed to get TOTP state", "user_id", authUser.UserID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to check TOTP state")
		return
	}
	if totpState.EnabledAt != nil {
		writeError(w, http.StatusConflict, "totp_already_enabled", "TOTP is already enabled for this account")
		return
	}

	// Fetch user email for the account name.
	user, err := a.db.GetUser(authUser.UserID)
	if err != nil {
		a.logger.Error("Failed to get user", "user_id", authUser.UserID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get user details")
		return
	}

	// Generate TOTP key.
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "Taskschmiede",
		AccountName: user.Email,
	})
	if err != nil {
		a.logger.Error("Failed to generate TOTP key", "user_id", authUser.UserID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to generate TOTP key")
		return
	}

	// Store the secret (not yet enabled).
	if err := a.db.SetUserTOTPSecret(authUser.UserID, key.Secret()); err != nil {
		a.logger.Error("Failed to store TOTP secret", "user_id", authUser.UserID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to store TOTP secret")
		return
	}

	// Generate QR code as base64 PNG.
	img, err := key.Image(200, 200)
	if err != nil {
		a.logger.Error("Failed to generate QR image", "user_id", authUser.UserID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to generate QR code")
		return
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		a.logger.Error("Failed to encode QR PNG", "user_id", authUser.UserID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to encode QR code")
		return
	}
	qrBase64 := base64.StdEncoding.EncodeToString(buf.Bytes())

	writeData(w, http.StatusOK, map[string]interface{}{
		"secret":  key.Secret(),
		"url":     key.URL(),
		"qr_code": qrBase64,
	})
}

// handleTOTPEnable handles POST /api/v1/auth/totp/enable (authenticated).
// Validates a TOTP code against the stored secret and activates 2FA.
// Returns recovery codes on success.
func (a *API) handleTOTPEnable(w http.ResponseWriter, r *http.Request) {
	authUser := auth.GetAuthUser(r.Context())
	if authUser == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	var body struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Invalid JSON body")
		return
	}
	if body.Code == "" {
		writeError(w, http.StatusBadRequest, "invalid_input", "TOTP code is required")
		return
	}

	totpState, err := a.db.GetUserTOTP(authUser.UserID)
	if err != nil {
		a.logger.Error("Failed to get TOTP state", "user_id", authUser.UserID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to check TOTP state")
		return
	}
	if totpState.Secret == "" {
		writeError(w, http.StatusBadRequest, "totp_not_setup", "TOTP has not been set up. Call POST /api/v1/auth/totp/setup first.")
		return
	}
	if totpState.EnabledAt != nil {
		writeError(w, http.StatusConflict, "totp_already_enabled", "TOTP is already enabled for this account")
		return
	}

	// Validate the TOTP code.
	if !totp.Validate(body.Code, totpState.Secret) {
		writeError(w, http.StatusBadRequest, "invalid_code", "Invalid TOTP code")
		return
	}

	// Enable TOTP.
	if err := a.db.EnableUserTOTP(authUser.UserID); err != nil {
		a.logger.Error("Failed to enable TOTP", "user_id", authUser.UserID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to enable TOTP")
		return
	}

	// Generate recovery codes.
	codes, err := a.db.CreateTOTPRecoveryCodes(authUser.UserID, 10)
	if err != nil {
		a.logger.Error("Failed to create recovery codes", "user_id", authUser.UserID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to generate recovery codes")
		return
	}

	if a.auditSvc != nil {
		a.auditSvc.Log(&security.AuditEntry{
			Action:    "totp_enabled",
			ActorID:   authUser.UserID,
			ActorType: "user",
			IP:        security.ExtractIP(r),
			Source:    auditSource(r),
		})
	}

	a.logger.Info("TOTP enabled", "user_id", authUser.UserID)

	writeData(w, http.StatusOK, map[string]interface{}{
		"status":         "enabled",
		"recovery_codes": codes,
	})
}

// handleTOTPDisable handles POST /api/v1/auth/totp/disable (authenticated).
// Requires a valid TOTP code to confirm the disable action.
func (a *API) handleTOTPDisable(w http.ResponseWriter, r *http.Request) {
	authUser := auth.GetAuthUser(r.Context())
	if authUser == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	var body struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Invalid JSON body")
		return
	}
	if body.Code == "" {
		writeError(w, http.StatusBadRequest, "invalid_input", "TOTP code is required")
		return
	}

	totpState, err := a.db.GetUserTOTP(authUser.UserID)
	if err != nil {
		a.logger.Error("Failed to get TOTP state", "user_id", authUser.UserID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to check TOTP state")
		return
	}
	if totpState.EnabledAt == nil {
		writeError(w, http.StatusBadRequest, "totp_not_enabled", "TOTP is not enabled for this account")
		return
	}

	// Validate the TOTP code.
	if !totp.Validate(body.Code, totpState.Secret) {
		writeError(w, http.StatusBadRequest, "invalid_code", "Invalid TOTP code")
		return
	}

	// Disable TOTP (clears secret, enabled_at, and recovery codes).
	if err := a.db.DisableUserTOTP(authUser.UserID); err != nil {
		a.logger.Error("Failed to disable TOTP", "user_id", authUser.UserID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to disable TOTP")
		return
	}

	if a.auditSvc != nil {
		a.auditSvc.Log(&security.AuditEntry{
			Action:    "totp_disabled",
			ActorID:   authUser.UserID,
			ActorType: "user",
			IP:        security.ExtractIP(r),
			Source:    auditSource(r),
		})
	}

	a.logger.Info("TOTP disabled", "user_id", authUser.UserID)

	writeData(w, http.StatusOK, map[string]interface{}{
		"status": "disabled",
	})
}

// handleTOTPStatus handles GET /api/v1/auth/totp/status (authenticated).
// Returns whether TOTP is enabled, when it was enabled, and remaining
// recovery codes.
func (a *API) handleTOTPStatus(w http.ResponseWriter, r *http.Request) {
	authUser := auth.GetAuthUser(r.Context())
	if authUser == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	totpState, err := a.db.GetUserTOTP(authUser.UserID)
	if err != nil {
		a.logger.Error("Failed to get TOTP state", "user_id", authUser.UserID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to check TOTP state")
		return
	}

	enabled := totpState.EnabledAt != nil

	var enabledAt interface{}
	if totpState.EnabledAt != nil {
		enabledAt = totpState.EnabledAt.Format(time.RFC3339)
	}

	var remaining int
	if enabled {
		remaining, err = a.db.CountUnusedRecoveryCodes(authUser.UserID)
		if err != nil {
			a.logger.Error("Failed to count recovery codes", "user_id", authUser.UserID, "error", err)
		}
	}

	writeData(w, http.StatusOK, map[string]interface{}{
		"enabled":                  enabled,
		"enabled_at":               enabledAt,
		"recovery_codes_remaining": remaining,
	})
}

// handleTOTPVerify handles POST /api/v1/auth/totp/verify (public).
// Completes 2FA login by validating a TOTP code (or recovery code) against
// a pending 2FA challenge token. On success, returns a full session token.
func (a *API) handleTOTPVerify(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token string `json:"token"`
		Code  string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Invalid JSON body")
		return
	}
	if body.Token == "" || body.Code == "" {
		writeError(w, http.StatusBadRequest, "invalid_input", "Token and code are required")
		return
	}

	// Rate limit by IP.
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
				Metadata:   map[string]interface{}{"tier": "auth-endpoint"},
			})
		}
		w.Header().Set("Retry-After", "60")
		writeError(w, http.StatusTooManyRequests, "rate_limited", "Too many attempts. Please wait and try again.")
		return
	}

	// Look up the pending 2FA challenge (non-destructive).
	userID, ok := getPending2FA(body.Token)
	if !ok {
		if a.auditSvc != nil {
			a.auditSvc.Log(&security.AuditEntry{
				Action:    "login_2fa_failure",
				ActorType: "anonymous",
				IP:        security.ExtractIP(r),
				Source:    auditSource(r),
				Metadata:  map[string]interface{}{"reason": "invalid_or_expired_token"},
			})
		}
		writeError(w, http.StatusUnauthorized, "invalid_token", "Invalid or expired 2FA token")
		return
	}

	// Get the user's TOTP state.
	totpState, err := a.db.GetUserTOTP(userID)
	if err != nil || totpState.EnabledAt == nil {
		a.logger.Error("Failed to get TOTP state for 2FA verify", "user_id", userID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to verify 2FA")
		return
	}

	// Try TOTP code first, then recovery code.
	codeValid := totp.Validate(body.Code, totpState.Secret)
	usedRecovery := false

	if !codeValid {
		recovered, recErr := a.db.VerifyTOTPRecoveryCode(userID, body.Code)
		if recErr != nil {
			a.logger.Error("Failed to check recovery code", "user_id", userID, "error", recErr)
		}
		if recovered {
			codeValid = true
			usedRecovery = true
		}
	}

	if !codeValid {
		if a.auditSvc != nil {
			a.auditSvc.Log(&security.AuditEntry{
				Action:    "login_2fa_failure",
				ActorID:   userID,
				ActorType: "user",
				IP:        security.ExtractIP(r),
				Source:    auditSource(r),
				Metadata:  map[string]interface{}{"reason": "invalid_code"},
			})
		}
		writeError(w, http.StatusUnauthorized, "invalid_code", "Invalid TOTP or recovery code")
		return
	}

	// Code is valid -- consume the pending challenge so it cannot be reused.
	consumePending2FA(body.Token)

	// Create a full session token.
	ttl := a.authSvc.DefaultTokenTTL(r.Context())
	exp := storage.UTCNow().Add(ttl)
	token, tokenID, err := a.authSvc.CreateToken(r.Context(), userID, "rest-session", &exp)
	if err != nil {
		a.logger.Error("Failed to create token after 2FA", "user_id", userID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to create session token")
		return
	}

	// Fetch user details for the response.
	user, err := a.db.GetUser(userID)
	if err != nil {
		a.logger.Error("Failed to get user after 2FA", "user_id", userID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get user details")
		return
	}

	if a.auditSvc != nil {
		meta := map[string]interface{}{"email": user.Email}
		if usedRecovery {
			meta["method"] = "recovery_code"
		}
		a.auditSvc.Log(&security.AuditEntry{
			Action:    "login_2fa_success",
			ActorID:   userID,
			ActorType: "user",
			IP:        security.ExtractIP(r),
			Source:    auditSource(r),
			Metadata:  meta,
		})
	}

	a.db.IncrementLoginCount(userID)

	a.logger.Info("User authenticated via 2FA", "user_id", userID, "email", user.Email, "recovery_code", usedRecovery)

	writeData(w, http.StatusOK, map[string]interface{}{
		"token":      token,
		"token_id":   tokenID,
		"user_id":    userID,
		"name":       user.Name,
		"email":      user.Email,
		"expires_at": exp.Format(time.RFC3339),
	})
}
