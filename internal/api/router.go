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
	"errors"
	"net/http"
	"strings"

	"github.com/QuestFinTech/taskschmiede/internal/auth"
)

// Handler returns the http.Handler for the REST API.
// It includes CORS middleware and all route registrations.
func (a *API) Handler() http.Handler {
	mux := http.NewServeMux()

	// Auth (public)
	mux.HandleFunc("POST /api/v1/auth/login", a.handleAuthLogin)
	mux.HandleFunc("POST /api/v1/auth/register", a.handleAuthRegister)
	mux.HandleFunc("POST /api/v1/auth/verify", a.handleAuthVerify)
	mux.HandleFunc("POST /api/v1/auth/forgot-password", a.handleForgotPassword)
	mux.HandleFunc("POST /api/v1/auth/reset-password", a.handleResetPassword)
	mux.HandleFunc("POST /api/v1/auth/resend-verification", a.handleResendVerification)
	mux.HandleFunc("POST /api/v1/auth/complete-profile", a.withAuth(a.handleCompleteProfile))
	mux.HandleFunc("GET /api/v1/auth/verification-status", a.handleVerificationStatus)
	mux.HandleFunc("GET /api/v1/auth/whoami", a.withAuth(a.handleAuthWhoami))
	mux.HandleFunc("PATCH /api/v1/auth/profile", a.withAuth(a.handleAuthProfileUpdate))
	mux.HandleFunc("POST /api/v1/auth/change-password", a.withAuth(a.handleAuthChangePassword))
	mux.HandleFunc("POST /api/v1/auth/logout", a.withAuth(a.handleAuthLogout))

	// TOTP 2FA
	mux.HandleFunc("POST /api/v1/auth/totp/setup", a.withAuth(a.handleTOTPSetup))
	mux.HandleFunc("POST /api/v1/auth/totp/enable", a.withAuth(a.handleTOTPEnable))
	mux.HandleFunc("POST /api/v1/auth/totp/disable", a.withAuth(a.handleTOTPDisable))
	mux.HandleFunc("GET /api/v1/auth/totp/status", a.withAuth(a.handleTOTPStatus))
	mux.HandleFunc("POST /api/v1/auth/totp/verify", a.handleTOTPVerify)

	// Consent
	mux.HandleFunc("POST /api/v1/auth/consent", a.withAuth(a.handleAcceptConsent))

	// Person (profile enhancement)
	mux.HandleFunc("GET /api/v1/auth/person", a.withAuth(a.handleGetMyPerson))
	mux.HandleFunc("PATCH /api/v1/auth/person", a.withAuth(a.handleUpdateMyPerson))
	mux.HandleFunc("GET /api/v1/auth/consents", a.withAuth(a.handleListMyConsents))

	// Email change (with verification)
	mux.HandleFunc("POST /api/v1/auth/email/change", a.withAuth(a.handleRequestEmailChange))
	mux.HandleFunc("POST /api/v1/auth/email/verify", a.withAuth(a.handleVerifyEmailChange))
	mux.HandleFunc("GET /api/v1/auth/email/pending", a.withAuth(a.handleGetPendingEmailChange))
	mux.HandleFunc("POST /api/v1/auth/email/cancel", a.withAuth(a.handleCancelEmailChange))

	// GDPR
	mux.HandleFunc("GET /api/v1/auth/my-data", a.withAuth(a.handleExportMyData))
	mux.HandleFunc("POST /api/v1/auth/delete-account", a.withAuth(a.handleRequestDeletion))
	mux.HandleFunc("POST /api/v1/auth/cancel-deletion", a.withAuth(a.handleCancelDeletion))
	mux.HandleFunc("GET /api/v1/auth/deletion-status", a.withAuth(a.handleDeletionStatus))

	// Tasks
	mux.HandleFunc("POST /api/v1/tasks", a.withAuth(a.handleTaskCreate))
	mux.HandleFunc("GET /api/v1/tasks", a.withAuth(a.handleTaskList))
	mux.HandleFunc("GET /api/v1/tasks/{id}", a.withAuth(a.handleTaskGet))
	mux.HandleFunc("PATCH /api/v1/tasks/{id}", a.withAuth(a.handleTaskUpdate))

	// Endeavours
	mux.HandleFunc("POST /api/v1/endeavours", a.withAuth(a.handleEndeavourCreate))
	mux.HandleFunc("GET /api/v1/endeavours", a.withAuth(a.handleEndeavourList))
	mux.HandleFunc("GET /api/v1/endeavours/{id}", a.withAuth(a.handleEndeavourGet))
	mux.HandleFunc("PATCH /api/v1/endeavours/{id}", a.withAuth(a.handleEndeavourUpdate))
	mux.HandleFunc("GET /api/v1/endeavours/{id}/archive", a.withAuth(a.handleEndeavourArchiveImpact))
	mux.HandleFunc("POST /api/v1/endeavours/{id}/archive", a.withAuth(a.handleEndeavourArchive))
	mux.HandleFunc("GET /api/v1/endeavours/{id}/export", a.withAuth(a.handleEndeavourExport))
	mux.HandleFunc("POST /api/v1/endeavours/{id}/members", a.withAuth(a.handleEdvAddMember))
	mux.HandleFunc("GET /api/v1/endeavours/{id}/members", a.withAuth(a.handleEdvListMembers))
	mux.HandleFunc("DELETE /api/v1/endeavours/{id}/members/{user_id}", a.withAuth(a.handleEdvRemoveMember))

	// Organizations
	mux.HandleFunc("POST /api/v1/organizations", a.withAuth(a.handleOrganizationCreate))
	mux.HandleFunc("GET /api/v1/organizations", a.withAuth(a.handleOrganizationList))
	mux.HandleFunc("GET /api/v1/organizations/{id}", a.withAuth(a.handleOrganizationGet))
	mux.HandleFunc("PATCH /api/v1/organizations/{id}", a.withAuth(a.handleOrganizationUpdate))
	mux.HandleFunc("GET /api/v1/organizations/{id}/archive", a.withAuth(a.handleOrganizationArchiveImpact))
	mux.HandleFunc("POST /api/v1/organizations/{id}/archive", a.withAuth(a.handleOrganizationArchive))
	mux.HandleFunc("POST /api/v1/organizations/{id}/resources", a.withAuth(a.handleOrganizationAddResource))
	mux.HandleFunc("POST /api/v1/organizations/{id}/endeavours", a.withAuth(a.handleOrganizationAddEndeavour))
	mux.HandleFunc("GET /api/v1/organizations/{id}/export", a.withAuth(a.handleOrganizationExport))
	mux.HandleFunc("POST /api/v1/organizations/{id}/members", a.withAuth(a.handleOrgAddMember))
	mux.HandleFunc("GET /api/v1/organizations/{id}/members", a.withAuth(a.handleOrgListMembers))
	mux.HandleFunc("PATCH /api/v1/organizations/{id}/members/{user_id}", a.withAuth(a.handleOrgSetMemberRole))
	mux.HandleFunc("DELETE /api/v1/organizations/{id}/members/{user_id}", a.withAuth(a.handleOrgRemoveMember))
	mux.HandleFunc("GET /api/v1/organizations/{id}/alert-terms", a.withAuth(a.handleOrgAlertTermsList))
	mux.HandleFunc("PUT /api/v1/organizations/{id}/alert-terms", a.withAuth(a.handleOrgAlertTermsUpdate))

	// Resources
	mux.HandleFunc("POST /api/v1/resources", a.withAuth(a.handleResourceCreate))
	mux.HandleFunc("GET /api/v1/resources", a.withAuth(a.handleResourceList))
	mux.HandleFunc("GET /api/v1/resources/{id}", a.withAuth(a.handleResourceGet))
	mux.HandleFunc("PATCH /api/v1/resources/{id}", a.withAuth(a.handleResourceUpdate))
	mux.HandleFunc("DELETE /api/v1/resources/{id}", a.withAuth(a.handleResourceDelete))

	// Demands
	mux.HandleFunc("POST /api/v1/demands", a.withAuth(a.handleDemandCreate))
	mux.HandleFunc("GET /api/v1/demands", a.withAuth(a.handleDemandList))
	mux.HandleFunc("GET /api/v1/demands/{id}", a.withAuth(a.handleDemandGet))
	mux.HandleFunc("PATCH /api/v1/demands/{id}", a.withAuth(a.handleDemandUpdate))

	// Artifacts
	mux.HandleFunc("POST /api/v1/artifacts", a.withAuth(a.handleArtifactCreate))
	mux.HandleFunc("GET /api/v1/artifacts", a.withAuth(a.handleArtifactList))
	mux.HandleFunc("GET /api/v1/artifacts/{id}", a.withAuth(a.handleArtifactGet))
	mux.HandleFunc("PATCH /api/v1/artifacts/{id}", a.withAuth(a.handleArtifactUpdate))
	mux.HandleFunc("DELETE /api/v1/artifacts/{id}", a.withAuth(a.handleArtifactDelete))

	// Rituals
	mux.HandleFunc("POST /api/v1/rituals", a.withAuth(a.handleRitualCreate))
	mux.HandleFunc("GET /api/v1/rituals", a.withAuth(a.handleRitualList))
	mux.HandleFunc("GET /api/v1/rituals/{id}", a.withAuth(a.handleRitualGet))
	mux.HandleFunc("PATCH /api/v1/rituals/{id}", a.withAuth(a.handleRitualUpdate))
	mux.HandleFunc("POST /api/v1/rituals/{id}/fork", a.withAuth(a.handleRitualFork))
	mux.HandleFunc("GET /api/v1/rituals/{id}/lineage", a.withAuth(a.handleRitualLineage))

	// Ritual Runs
	mux.HandleFunc("POST /api/v1/ritual-runs", a.withAuth(a.handleRitualRunCreate))
	mux.HandleFunc("GET /api/v1/ritual-runs", a.withAuth(a.handleRitualRunList))
	mux.HandleFunc("GET /api/v1/ritual-runs/{id}", a.withAuth(a.handleRitualRunGet))
	mux.HandleFunc("PATCH /api/v1/ritual-runs/{id}", a.withAuth(a.handleRitualRunUpdate))

	// Comments
	mux.HandleFunc("POST /api/v1/comments", a.withAuth(a.handleCommentCreate))
	mux.HandleFunc("GET /api/v1/comments", a.withAuth(a.handleCommentList))
	mux.HandleFunc("GET /api/v1/comments/{id}", a.withAuth(a.handleCommentGet))
	mux.HandleFunc("PATCH /api/v1/comments/{id}", a.withAuth(a.handleCommentUpdate))
	mux.HandleFunc("DELETE /api/v1/comments/{id}", a.withAuth(a.handleCommentDelete))

	// Approvals
	mux.HandleFunc("POST /api/v1/approvals", a.withAuth(a.handleApprovalCreate))
	mux.HandleFunc("GET /api/v1/approvals", a.withAuth(a.handleApprovalList))
	mux.HandleFunc("GET /api/v1/approvals/{id}", a.withAuth(a.handleApprovalGet))

	// DoD Policies
	mux.HandleFunc("POST /api/v1/dod-policies", a.withAuth(a.handleDodPolicyCreate))
	mux.HandleFunc("GET /api/v1/dod-policies", a.withAuth(a.handleDodPolicyList))
	mux.HandleFunc("GET /api/v1/dod-policies/{id}", a.withAuth(a.handleDodPolicyGet))
	mux.HandleFunc("PATCH /api/v1/dod-policies/{id}", a.withAuth(a.handleDodPolicyUpdate))
	mux.HandleFunc("POST /api/v1/dod-policies/{id}/versions", a.withAuth(a.handleDodPolicyNewVersion))
	mux.HandleFunc("GET /api/v1/dod-policies/{id}/lineage", a.withAuth(a.handleDodPolicyLineage))

	// DoD Endeavour Assignment + Status
	mux.HandleFunc("POST /api/v1/endeavours/{id}/dod-policy", a.withAuth(a.handleDodAssign))
	mux.HandleFunc("DELETE /api/v1/endeavours/{id}/dod-policy", a.withAuth(a.handleDodUnassign))
	mux.HandleFunc("GET /api/v1/endeavours/{id}/dod-status", a.withAuth(a.handleDodStatus))

	// DoD Endorsements
	mux.HandleFunc("POST /api/v1/dod-endorsements", a.withAuth(a.handleDodEndorse))
	mux.HandleFunc("GET /api/v1/dod-endorsements", a.withAuth(a.handleDodEndorsementList))

	// DoD Task Operations
	mux.HandleFunc("POST /api/v1/tasks/{id}/dod-check", a.withAuth(a.handleDodCheck))
	mux.HandleFunc("POST /api/v1/tasks/{id}/dod-override", a.withAuth(a.handleDodOverride))

	// Messages
	mux.HandleFunc("POST /api/v1/messages", a.withAuth(a.handleMessageSend))
	mux.HandleFunc("GET /api/v1/messages", a.withAuth(a.handleMessageInbox))
	mux.HandleFunc("GET /api/v1/messages/{id}", a.withAuth(a.handleMessageGet))
	mux.HandleFunc("PATCH /api/v1/messages/{id}", a.withAuth(a.handleMessageRead))
	mux.HandleFunc("POST /api/v1/messages/{id}/reply", a.withAuth(a.handleMessageReply))
	mux.HandleFunc("GET /api/v1/messages/{id}/thread", a.withAuth(a.handleMessageThread))

	// Relations
	mux.HandleFunc("POST /api/v1/relations", a.withAuth(a.handleRelationCreate))
	mux.HandleFunc("GET /api/v1/relations", a.withAuth(a.handleRelationList))
	mux.HandleFunc("DELETE /api/v1/relations/{id}", a.withAuth(a.handleRelationDelete))

	// Users
	mux.HandleFunc("POST /api/v1/users", a.withAuth(a.handleUserCreate))
	mux.HandleFunc("GET /api/v1/users", a.withAuth(a.handleUserList))
	mux.HandleFunc("GET /api/v1/users/{id}", a.withAuth(a.handleUserGet))
	mux.HandleFunc("PATCH /api/v1/users/{id}", a.withAuth(a.handleUserUpdate))

	// Templates
	mux.HandleFunc("POST /api/v1/templates", a.withAuth(a.handleTemplateCreate))
	mux.HandleFunc("GET /api/v1/templates", a.withAuth(a.handleTemplateList))
	mux.HandleFunc("GET /api/v1/templates/{id}", a.withAuth(a.handleTemplateGet))
	mux.HandleFunc("PATCH /api/v1/templates/{id}", a.withAuth(a.handleTemplateUpdate))
	mux.HandleFunc("POST /api/v1/templates/{id}/fork", a.withAuth(a.handleTemplateFork))
	mux.HandleFunc("GET /api/v1/templates/{id}/lineage", a.withAuth(a.handleTemplateLineage))

	// Reports
	mux.HandleFunc("GET /api/v1/reports/{scope}/{id}", a.withAuth(a.handleReportGenerate))
	mux.HandleFunc("POST /api/v1/reports/{scope}/{id}/email", a.withAuth(a.handleReportEmail))

	// KPI (admin-only)
	mux.HandleFunc("GET /api/v1/kpi/current", a.withAuth(a.handleKPICurrent))
	mux.HandleFunc("GET /api/v1/kpi/history", a.withAuth(a.handleKPIHistory))

	// Audit
	mux.HandleFunc("GET /api/v1/audit", a.withAuth(a.handleAuditList))
	mux.HandleFunc("GET /api/v1/audit/my-activity", a.withAuth(a.handleAuditMyActivity))
	mux.HandleFunc("GET /api/v1/audit/my-full-activity", a.withAuth(a.handleMyFullActivity))
	mux.HandleFunc("GET /api/v1/entity-changes", a.withAuth(a.handleEntityChangeList))
	mux.HandleFunc("GET /api/v1/activity", a.withAuth(a.handleActivityList))

	// Onboarding
	mux.HandleFunc("GET /api/v1/onboarding/status", a.withAuth(a.handleOnboardingStatus))

	// Injection reviews (admin-only)
	mux.HandleFunc("GET /api/v1/onboarding/injection-reviews", a.withAuth(a.handleInjectionReviewList))
	mux.HandleFunc("GET /api/v1/onboarding/injection-reviews/{id}", a.withAuth(a.handleInjectionReviewGet))

	// Compatibility matrix (public)
	mux.HandleFunc("GET /api/v1/compatibility", a.handleCompatibility)

	// Instance info (public -- non-sensitive deployment settings)
	mux.HandleFunc("GET /api/v1/instance/info", a.handleInstanceInfo)

	// Seat capacity (public -- used by SaaS website for availability display)
	mux.HandleFunc("GET /api/v1/capacity", a.handleCapacity)

	// Signup interest (public -- used by SaaS website /join page)
	mux.HandleFunc("POST /api/v1/signup-interest", a.handleSignupInterest)

	// Admin setup (public -- only works when no admin exists)
	mux.HandleFunc("GET /api/v1/admin/setup/status", a.handleSetupStatus)
	mux.HandleFunc("POST /api/v1/admin/setup", a.handleSetupCreate)
	mux.HandleFunc("POST /api/v1/admin/setup/verify", a.handleSetupVerify)
	mux.HandleFunc("POST /api/v1/admin/setup/resend", a.handleSetupResend)
	mux.HandleFunc("POST /api/v1/admin/setup/configure", a.handleSetupConfigure)

	// Admin (authenticated, admin-only)
	mux.HandleFunc("GET /api/v1/admin/settings", a.withAuth(a.handleAdminSettings))
	mux.HandleFunc("PATCH /api/v1/admin/settings", a.withAuth(a.handleAdminSettingsUpdate))
	mux.HandleFunc("POST /api/v1/admin/password", a.withAuth(a.handleAdminPassword))
	mux.HandleFunc("GET /api/v1/admin/stats", a.withAuth(a.handleAdminStats))
	mux.HandleFunc("GET /api/v1/admin/usage", a.withAuth(a.handleAdminUsage))
	mux.HandleFunc("GET /api/v1/admin/quotas", a.withAuth(a.handleAdminQuotas))
	mux.HandleFunc("PATCH /api/v1/admin/quotas", a.withAuth(a.handleAdminQuotasUpdate))
	mux.HandleFunc("GET /api/v1/admin/indicators", a.withAuth(a.handleAdminIndicators))
	mux.HandleFunc("GET /api/v1/admin/content-guard/stats", a.withAuth(a.handleAdminContentGuardStats))
	mux.HandleFunc("POST /api/v1/admin/content-guard/test", a.withAuth(a.handleAdminContentGuardTest))
	mux.HandleFunc("GET /api/v1/admin/content-guard/patterns", a.withAuth(a.handleAdminContentGuardPatterns))
	mux.HandleFunc("PATCH /api/v1/admin/content-guard/patterns", a.withAuth(a.handleAdminContentGuardPatternsUpdate))
	mux.HandleFunc("GET /api/v1/admin/content-guard/alerts", a.withAuth(a.handleAdminContentGuardAlerts))
	mux.HandleFunc("POST /api/v1/admin/content-guard/dismiss", a.withAuth(a.handleAdminContentGuardDismiss))
	mux.HandleFunc("GET /api/v1/admin/agent-block-signals", a.withAuth(a.handleAdminAgentBlockSignals))
	mux.HandleFunc("GET /api/v1/admin/tiers", a.withAuth(a.handleAdminTiers))
	mux.HandleFunc("GET /api/v1/admin/tier-usage", a.withAuth(a.handleAdminTierUsage))
	mux.HandleFunc("GET /api/v1/admin/taskschmied/status", a.withAuth(a.handleAdminTaskschmiedStatus))
	mux.HandleFunc("POST /api/v1/admin/taskschmied/toggle", a.withAuth(a.handleAdminTaskschmiedToggle))

	// Agent tokens (authenticated, user-scoped)
	mux.HandleFunc("GET /api/v1/agent-tokens", a.withAuth(a.handleAgentTokenList))
	mux.HandleFunc("POST /api/v1/agent-tokens", a.withAuth(a.handleAgentTokenCreate))
	mux.HandleFunc("DELETE /api/v1/agent-tokens/{id}", a.withAuth(a.handleAgentTokenRevoke))

	// My agents (authenticated, owner-scoped)
	mux.HandleFunc("GET /api/v1/my-agents", a.withAuth(a.handleMyAgentsList))
	mux.HandleFunc("GET /api/v1/my-agents/{id}", a.withAuth(a.handleMyAgentGet))
	mux.HandleFunc("PATCH /api/v1/my-agents/{id}", a.withAuth(a.handleMyAgentUpdate))
	mux.HandleFunc("GET /api/v1/my-agents/{id}/activity", a.withAuth(a.handleMyAgentActivity))
	mux.HandleFunc("GET /api/v1/my-agents/{id}/onboarding", a.withAuth(a.handleMyAgentOnboarding))

	// My alerts and indicators (authenticated, owner-scoped)
	mux.HandleFunc("GET /api/v1/my-alerts", a.withAuth(a.handleMyAlertsList))
	mux.HandleFunc("GET /api/v1/my-alerts/stats", a.withAuth(a.handleMyAlertsStats))
	mux.HandleFunc("GET /api/v1/my-indicators", a.withAuth(a.handleMyIndicators))

	// Invitations (authenticated, admin-only)
	mux.HandleFunc("GET /api/v1/invitations", a.withAuth(a.handleInvitationList))
	mux.HandleFunc("POST /api/v1/invitations", a.withAuth(a.handleInvitationCreate))
	mux.HandleFunc("DELETE /api/v1/invitations/{id}", a.withAuth(a.handleInvitationRevoke))

	// Wrap with CORS
	return a.corsMiddleware(mux)
}

// withAuth wraps a handler function to require authentication.
// It resolves the Bearer token and injects the AuthUser into the context.
// Also touches last_active_at for activity tracking (debounced).
func (a *API) withAuth(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := auth.ExtractBearerToken(r.Header.Get("Authorization"))
		if token == "" {
			writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
			return
		}

		user, err := a.authSvc.VerifyToken(r.Context(), token)
		if err != nil {
			if errors.Is(err, auth.ErrAccountSuspended) {
				writeError(w, http.StatusForbidden, "account_suspended", "Account has been suspended by an administrator")
				return
			}
			if errors.Is(err, auth.ErrAccountBlocked) {
				writeError(w, http.StatusForbidden, "account_blocked", "Account has been blocked by sponsor")
				return
			}
			writeError(w, http.StatusUnauthorized, "unauthorized", "Invalid or expired token")
			return
		}

		a.db.TouchUserActivity(user.UserID)

		r = r.WithContext(auth.WithAuthUser(r.Context(), user))
		handler(w, r)
	}
}

// corsMiddleware adds CORS and security headers for all API responses.
func (a *API) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && a.isAllowedOrigin(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			w.Header().Set("Access-Control-Max-Age", "86400")
			w.Header().Set("Vary", "Origin")
		}

		// Security headers
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		w.Header().Set("Cross-Origin-Embedder-Policy", "credentialless")
		w.Header().Set("Cross-Origin-Opener-Policy", "same-origin")
		w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
		w.Header().Set("Cache-Control", "no-store")

		// Handle preflight
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// isAllowedOrigin checks if the origin is in the allowed list.
func (a *API) isAllowedOrigin(origin string) bool {
	for _, allowed := range a.corsOrigins {
		if allowed == "*" || strings.EqualFold(allowed, origin) {
			return true
		}
	}
	return false
}
