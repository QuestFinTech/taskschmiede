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


package mcp

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/QuestFinTech/taskschmiede/internal/auth"
	"github.com/QuestFinTech/taskschmiede/internal/email"
	"github.com/QuestFinTech/taskschmiede/internal/storage"
)

// Default verification timeout
const defaultVerificationTimeout = 15 * time.Minute

// registerRegistrationTools registers registration-related MCP tools.
func (s *Server) registerRegistrationTools(mcpServer *mcp.Server) {
	// ts.reg.register - Register a new agent account (requires invitation token)
	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.reg.register",
			Description: "Register a new agent account using an invitation token. A verification email will be sent to prove email capability. After receiving the code, call ts.reg.verify to complete registration.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"invitation_token": map[string]interface{}{
						"type":        "string",
						"description": "The invitation token from your human operator (required)",
					},
					"email": map[string]interface{}{
						"type":        "string",
						"description": "Email address for the new account",
					},
					"name": map[string]interface{}{
						"type":        "string",
						"description": "Display name for the account",
					},
					"password": map[string]interface{}{
						"type":        "string",
						"description": "Password for the account",
					},
				},
				"required": []string{"invitation_token", "email", "name", "password"},
			},
		},
		s.handleRegister,
	)

	// ts.reg.verify - Verify email with code
	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.reg.verify",
			Description: "Verify email with the code from the verification email",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"email": map[string]interface{}{
						"type":        "string",
						"description": "Email address being verified",
					},
					"code": map[string]interface{}{
						"type":        "string",
						"description": "Verification code from the email (format: xxx-xxx-xxx)",
					},
				},
				"required": []string{"email", "code"},
			},
		},
		s.handleVerify,
	)

	// ts.reg.resend - Resend verification email
	mcpServer.AddTool(
		&mcp.Tool{
			Name:        "ts.reg.resend",
			Description: "Resend the verification email",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"email": map[string]interface{}{
						"type":        "string",
						"description": "Email address to resend verification to",
					},
				},
				"required": []string{"email"},
			},
		},
		s.handleResend,
	)
}

// handleRegister handles agent registration via MCP. Requires an invitation
// token from a human operator. Creates a pending registration and sends a
// verification email -- the agent must prove email capability by extracting
// the code and calling ts.reg.verify.
func (s *Server) handleRegister(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := parseArgs(req)
	invToken := getString(args, "invitation_token")
	email := getString(args, "email")
	name := getString(args, "name")
	password := getString(args, "password")

	// MCP registration always creates agent accounts.
	// Human accounts are created through the web UI.
	userType := "agent"

	// Invitation token is required for MCP registration.
	if invToken == "" {
		return toolError("missing_token", "Invitation token is required. Get one from your human operator via the web console (/my-agents)."), nil
	}

	// Validate required fields
	if email == "" {
		return toolError("missing_email", "Email is required"), nil
	}
	if name == "" {
		return toolError("missing_name", "Name is required"), nil
	}
	if password == "" {
		return toolError("missing_password", "Password is required"), nil
	}

	// Validate email format
	if !isValidEmail(email) {
		return toolError("invalid_email", "Invalid email format"), nil
	}

	// Validate password strength
	if err := auth.ValidatePassword(password); err != nil {
		return toolError("weak_password", err.Error()), nil
	}

	// Validate invitation token
	inv, err := s.db.ValidateInvitationToken(invToken)
	if errors.Is(err, storage.ErrInvitationNotFound) {
		return toolError("invalid_token", "Invitation token is invalid"), nil
	}
	if errors.Is(err, storage.ErrInvitationExpired) {
		return toolError("invalid_token", "Invitation token has expired"), nil
	}
	if errors.Is(err, storage.ErrInvitationExhausted) {
		return toolError("invalid_token", "Invitation token has reached its usage limit"), nil
	}
	if errors.Is(err, storage.ErrInvitationRevoked) {
		return toolError("invalid_token", "Invitation token has been revoked"), nil
	}
	if err != nil {
		s.logger.Error("Failed to validate invitation token", "error", err)
		return toolError("internal_error", "Failed to validate invitation token"), nil
	}

	// Check agent-per-org quota before allowing registration.
	if inv.CreatedBy != "" {
		sponsorOrgID := inv.OrganizationID
		if sponsorOrgID == "" {
			sponsorOrgID = s.api.FindUserOrgID(ctx, inv.CreatedBy)
		}
		if sponsorOrgID != "" {
			if apiErr := s.api.CheckOrgQuotaForUser(ctx, inv.CreatedBy, sponsorOrgID, "agents_per_org"); apiErr != nil {
				return toolError("tier_limit", apiErr.Message), nil
			}
		}
	}

	// Check instance capacity.
	if s.api.IsAtCapacity() {
		passwordHash, err := auth.HashPassword(password)
		if err != nil {
			s.logger.Error("Failed to hash password", "error", err)
			return toolError("internal_error", "Failed to process registration"), nil
		}

		if s.db.IsEmailOnWaitlist(email) {
			pos := s.db.GetWaitlistPosition(email)
			return toolSuccess(map[string]interface{}{
				"status":   "waitlisted",
				"position": pos,
				"message":  "You are already on the waitlist. You will be notified when a slot becomes available.",
			}), nil
		}

		entry, wlErr := s.db.AddToWaitlist(email, name, passwordHash, inv.ID, userType)
		if wlErr != nil {
			s.logger.Error("Failed to add to waitlist", "error", wlErr)
			return toolError("internal_error", "Failed to process registration"), nil
		}

		// Increment invitation token usage after successful waitlist entry.
		if err := s.db.IncrementInvitationTokenUse(inv.ID); err != nil {
			s.logger.Warn("Failed to increment invitation token usage", "error", err)
		}

		pos := s.db.GetWaitlistPosition(email)
		s.logger.Info("Registration waitlisted (MCP)",
			"email", email, "waitlist_id", entry.ID, "position", pos)

		return toolSuccess(map[string]interface{}{
			"status":   "waitlisted",
			"position": pos,
			"message":  "The instance is currently at capacity. You have been added to the waitlist and will be notified via email when a slot becomes available.",
		}), nil
	}

	// When email verification is not required (trusted mode), create user directly.
	if !s.requireAgentEmailVerification {
		passwordHash, err := auth.HashPassword(password)
		if err != nil {
			s.logger.Error("Failed to hash password", "error", err)
			return toolError("internal_error", "Failed to process registration"), nil
		}

		user, token, err := s.db.CreateUserWithInvitation(email, name, passwordHash, inv.ID, userType, "", nil)
		if err != nil {
			if err.Error() == "email already registered" || err.Error() == "check email: email already exists" {
				return toolError("email_exists", "An account with this email already exists"), nil
			}
			s.logger.Error("Failed to create user", "error", err)
			return toolError("internal_error", "Failed to create registration"), nil
		}

		// Increment invitation token usage only after successful user creation.
		if incErr := s.db.IncrementInvitationTokenUse(inv.ID); incErr != nil {
			s.logger.Warn("Failed to increment invitation token usage", "error", incErr)
		}

		// When interview is also not required, mark as skipped so agent gets full access.
		onboardingStatus := "interview_pending"
		if !s.requireAgentInterview {
			onboardingStatus = "interview_skipped"
			if _, err := s.db.Exec(`UPDATE user SET onboarding_status = 'interview_skipped' WHERE id = ?`, user.ID); err != nil {
				s.logger.Error("Failed to update onboarding status", "error", err)
			}
		}

		s.logger.Info("Agent registered (email verification skipped)",
			"user_id", user.ID, "email", email, "deployment_mode", s.deploymentMode,
			"onboarding_status", onboardingStatus)

		result := map[string]interface{}{
			"status":            "active",
			"user_id":           user.ID,
			"token":             token,
			"email":             user.Email,
			"name":              user.Name,
			"onboarding_status": onboardingStatus,
		}
		if onboardingStatus == "interview_pending" {
			result["message"] = "Account created. Complete the onboarding interview to access production tools. Call ts.onboard.start_interview to begin."
		} else {
			result["message"] = "Account created with full access."
		}
		return toolSuccess(result), nil
	}

	// Email verification path: create pending user, send verification email.
	pending, err := s.db.CreatePendingUser(email, name, password, inv.ID, userType, "", defaultVerificationTimeout, nil)
	if errors.Is(err, storage.ErrEmailExists) {
		return toolError("email_exists", "An account with this email already exists"), nil
	}
	if err != nil {
		s.logger.Error("Failed to create pending user", "error", err)
		return toolError("internal_error", "Failed to create registration"), nil
	}

	// Increment invitation token usage only after successful pending user creation.
	if incErr := s.db.IncrementInvitationTokenUse(inv.ID); incErr != nil {
		s.logger.Warn("Failed to increment invitation token usage", "error", incErr)
	}

	// Send verification email
	if s.emailSender != nil {
		if err := s.sendVerificationEmail(email, name, pending.VerificationCode); err != nil {
			s.logger.Error("Failed to send verification email", "error", err)
		}
	}

	s.logger.Info("Agent registration started (pending email verification)",
		"email", email, "name", name, "invitation_token_id", inv.ID)

	return toolSuccess(map[string]interface{}{
		"status":     "pending_verification",
		"email":      email,
		"expires_in": defaultVerificationTimeout.String(),
		"message":    "A verification code has been sent to your email. Read the email, extract the code (format: xxx-xxx-xxx), and call ts.reg.verify with your email and the code to complete registration.",
	}), nil
}

// handleVerify handles email verification.
func (s *Server) handleVerify(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := parseArgs(req)
	email := getString(args, "email")
	code := getString(args, "code")

	if email == "" {
		return toolError("missing_email", "Email is required"), nil
	}
	if code == "" {
		return toolError("missing_code", "Verification code is required"), nil
	}

	// Verify and create user
	user, token, err := s.db.VerifyAndCreateUser(email, code)
	if errors.Is(err, storage.ErrPendingUserNotFound) {
		return toolError("not_found", "No pending verification for this email"), nil
	}
	if errors.Is(err, storage.ErrInvalidCode) {
		return toolError("invalid_code", "Verification code is incorrect"), nil
	}
	if errors.Is(err, storage.ErrCodeExpired) {
		return toolError("code_expired", "Verification code has expired"), nil
	}
	if err != nil {
		s.logger.Error("Failed to verify user", "error", err)
		return toolError("internal_error", "Failed to complete verification"), nil
	}

	s.logger.Info("User verified and created", "user_id", user.ID, "email", user.Email, "user_type", user.UserType)

	result := map[string]interface{}{
		"status":  "verified",
		"token":   token,
		"user_id": user.ID,
		"name":    user.Name,
		"email":   user.Email,
	}
	if user.UserType == "agent" {
		result["onboarding_status"] = "interview_pending"
		result["message"] = "Email verified. Complete the onboarding interview to access production tools. Call ts.onboard.start_interview to begin."
	}

	return toolSuccess(result), nil
}

// handleResend handles resending verification email.
func (s *Server) handleResend(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := parseArgs(req)
	email := getString(args, "email")

	if email == "" {
		return toolError("missing_email", "Email is required"), nil
	}

	// Regenerate code
	pending, err := s.db.RegeneratePendingUserCode(email, defaultVerificationTimeout)
	if errors.Is(err, storage.ErrPendingUserNotFound) {
		return toolError("not_found", "No pending verification for this email"), nil
	}
	if err != nil {
		s.logger.Error("Failed to regenerate verification code", "error", err)
		return toolError("internal_error", "Failed to regenerate verification code"), nil
	}

	// Send new verification email
	emailSent := false
	if s.emailSender != nil {
		err = s.sendVerificationEmail(email, pending.Name, pending.VerificationCode)
		if err != nil {
			s.logger.Error("Failed to send verification email", "error", err)
		} else {
			emailSent = true
		}
	}

	s.logger.Info("Verification code regenerated", "email", email, "email_sent", emailSent)

	return toolSuccess(map[string]interface{}{
		"sent":       emailSent,
		"expires_in": defaultVerificationTimeout.String(),
	}), nil
}

// sendCodeEmail sends a verification or password reset email using HTML templates
// when available, falling back to plain text.
func (s *Server) sendCodeEmail(toEmail, name, code, lang string, isReset bool) {
	if s.emailSender == nil {
		return
	}

	greeting := "Hello"
	if name != "" {
		greeting = "Hello " + name
	}

	var subject, intro, buttonText, manualNote, expiryNote, actionURL string
	if isReset {
		subject = "Taskschmiede - Password Reset"
		intro = "Your password reset code is:"
		buttonText = "Reset Password"
		manualNote = "Or enter this code manually on the reset page."
		expiryNote = fmt.Sprintf("This code will expire in %s.", defaultVerificationTimeout)
		if s.portalURL != "" {
			actionURL = s.portalURL + "/reset-password?email=" + toEmail + "&code=" + code
		}
	} else {
		subject = "Verify your Taskschmiede account"
		intro = "Your verification code is:"
		buttonText = "Verify Email"
		manualNote = "Or enter this code manually on the verification page."
		expiryNote = fmt.Sprintf("This code will expire in %s.", defaultVerificationTimeout)
		if s.portalURL != "" {
			actionURL = s.portalURL + "/verify?email=" + toEmail + "&code=" + code
		}
	}

	if svc, ok := s.emailSender.(*email.Service); ok {
		data := &email.VerificationCodeData{
			Greeting:   greeting,
			Intro:      intro,
			Code:       code,
			ExpiryNote: expiryNote,
			ActionURL:  actionURL,
			ButtonText: buttonText,
			ManualNote: manualNote,
			Closing:    "Best regards,",
			TeamName:   "Team Taskschmiede",
		}
		if err := svc.SendVerificationCode(toEmail, subject, data); err != nil {
			s.logger.Warn("Failed to send templated email", "error", err)
		}
		return
	}

	// Fallback to plain text
	body := fmt.Sprintf("%s,\n\n%s\n\n    %s\n\n%s\n\nBest regards,\nTeam Taskschmiede",
		greeting, intro, code, expiryNote)
	if err := s.emailSender.SendEmail(toEmail, subject, body); err != nil {
		s.logger.Warn("Failed to send email", "error", err)
	}
}

// sendVerificationEmail sends the verification email to the user (backward-compat wrapper).
func (s *Server) sendVerificationEmail(toEmail, name, code string) error {
	s.sendCodeEmail(toEmail, name, code, "en", false)
	return nil
}

// isValidEmail checks if an email address is valid.
func isValidEmail(email string) bool {
	// Basic email regex - checks for something@something.something
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	return emailRegex.MatchString(email)
}

