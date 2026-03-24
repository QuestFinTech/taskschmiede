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


package portal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// RESTClient communicates with the Taskschmiede REST API.
type RESTClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewRESTClient creates a new REST API client.
func NewRESTClient(baseURL string) *RESTClient {
	return &RESTClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// apiResponse wraps the standard REST API response format.
type apiResponse struct {
	Data  json.RawMessage `json:"data"`
	Error *apiError       `json:"error"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// RESTError is returned when the API returns an error response.
type RESTError struct {
	StatusCode int
	Code       string
	Message    string
}

// Error returns the human-readable error message.
func (e *RESTError) Error() string {
	return e.Message
}

// do executes an HTTP request and returns the parsed data field.
func (c *RESTClient) do(method, path string, token string, body interface{}) (json.RawMessage, error) {
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal body: %w", err)
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, c.baseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("X-Source", "portal")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var apiResp apiResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if apiResp.Error != nil {
		return nil, &RESTError{
			StatusCode: resp.StatusCode,
			Code:       apiResp.Error.Code,
			Message:    apiResp.Error.Message,
		}
	}

	return apiResp.Data, nil
}

// unmarshal calls do() and unmarshals the data into dst.
func (c *RESTClient) unmarshal(method, path, token string, body, dst interface{}) error {
	data, err := c.do(method, path, token, body)
	if err != nil {
		return err
	}
	if dst != nil && data != nil {
		return json.Unmarshal(data, dst)
	}
	return nil
}

// doList executes a GET and returns list data plus total count.
func (c *RESTClient) doList(path, token string) (json.RawMessage, int, error) {
	req, err := http.NewRequest("GET", c.baseURL+path, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("create request: %w", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("X-Source", "portal")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, fmt.Errorf("read response: %w", err)
	}

	var listResp struct {
		Data json.RawMessage `json:"data"`
		Meta struct {
			Total int `json:"total"`
		} `json:"meta"`
		Error *apiError `json:"error"`
	}
	if err := json.Unmarshal(body, &listResp); err != nil {
		return nil, 0, fmt.Errorf("parse response: %w", err)
	}

	if listResp.Error != nil {
		return nil, 0, &RESTError{
			StatusCode: resp.StatusCode,
			Code:       listResp.Error.Code,
			Message:    listResp.Error.Message,
		}
	}

	return listResp.Data, listResp.Meta.Total, nil
}

// --- Public endpoints (no auth) ---

// LoginResult holds login response data.
type LoginResult struct {
	Status       string `json:"status"`
	Token        string `json:"token"`
	TokenID      string `json:"token_id"`
	PendingToken string `json:"pending_token"`
	UserID       string `json:"user_id"`
	Name         string `json:"name"`
	Email        string `json:"email"`
	ExpiresAt    string `json:"expires_at"`
}

// Login authenticates a user with email and password.
func (c *RESTClient) Login(email, password string) (*LoginResult, error) {
	var result LoginResult
	body := map[string]string{"email": email, "password": password}
	if err := c.unmarshal("POST", "/api/v1/auth/login", "", body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// TOTPVerifyResult holds the response from 2FA verification.
type TOTPVerifyResult struct {
	Token     string `json:"token"`
	TokenID   string `json:"token_id"`
	UserID    string `json:"user_id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	ExpiresAt string `json:"expires_at"`
}

// VerifyTOTP completes 2FA login with a pending token and TOTP code.
func (c *RESTClient) VerifyTOTP(pendingToken, code string) (*TOTPVerifyResult, error) {
	var result TOTPVerifyResult
	body := map[string]string{"token": pendingToken, "code": code}
	if err := c.unmarshal("POST", "/api/v1/auth/totp/verify", "", body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// TOTPSetupResult holds the response from TOTP setup.
type TOTPSetupResult struct {
	Secret string `json:"secret"`
	URL    string `json:"url"`
	QRCode string `json:"qr_code"`
}

// TOTPSetup initiates TOTP setup for the authenticated user.
func (c *RESTClient) TOTPSetup(token string) (*TOTPSetupResult, error) {
	var result TOTPSetupResult
	if err := c.unmarshal("POST", "/api/v1/auth/totp/setup", token, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// TOTPEnableResult holds the response from TOTP enable.
type TOTPEnableResult struct {
	Status        string   `json:"status"`
	RecoveryCodes []string `json:"recovery_codes"`
}

// TOTPEnable validates a code and enables TOTP for the authenticated user.
func (c *RESTClient) TOTPEnable(token, code string) (*TOTPEnableResult, error) {
	var result TOTPEnableResult
	body := map[string]string{"code": code}
	if err := c.unmarshal("POST", "/api/v1/auth/totp/enable", token, body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// TOTPDisable disables TOTP for the authenticated user. Requires a valid TOTP code.
func (c *RESTClient) TOTPDisable(token, code string) error {
	body := map[string]string{"code": code}
	_, err := c.do("POST", "/api/v1/auth/totp/disable", token, body)
	return err
}

// TOTPStatusResult holds the TOTP status response.
type TOTPStatusResult struct {
	Enabled                bool        `json:"enabled"`
	EnabledAt              interface{} `json:"enabled_at"`
	RecoveryCodesRemaining int         `json:"recovery_codes_remaining"`
}

// TOTPStatus returns the TOTP status for the authenticated user.
func (c *RESTClient) TOTPStatus(token string) (*TOTPStatusResult, error) {
	var result TOTPStatusResult
	if err := c.unmarshal("GET", "/api/v1/auth/totp/status", token, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// InstanceInfo holds non-sensitive deployment settings from the API.
type InstanceInfo struct {
	DeploymentMode        string `json:"deployment_mode"`
	AllowSelfRegistration bool   `json:"allow_self_registration"`
	RegistrationOpen      bool   `json:"registration_open"`
}

// GetInstanceInfo fetches deployment settings (public, no auth).
func (c *RESTClient) GetInstanceInfo() (*InstanceInfo, error) {
	var result InstanceInfo
	if err := c.unmarshal("GET", "/api/v1/instance/info", "", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// RegisterResult holds registration response data.
type RegisterResult struct {
	Status    string `json:"status"`
	Email     string `json:"email"`
	ExpiresIn string `json:"expires_in"`
	Message   string `json:"message"`
}

// RegisterOpts holds the identity and consent fields for registration.
type RegisterOpts struct {
	AccountType         string
	FirstName           string
	LastName            string
	CompanyName         string
	CompanyRegistration string
	VATNumber           string
	Street              string
	Street2             string
	PostalCode          string
	City                string
	State               string
	Country             string
	AcceptTerms         bool
	AcceptPrivacy       bool
	AcceptDPA           bool
	AgeDeclaration      bool
}

// Register creates a new user account with the given credentials.
func (c *RESTClient) Register(email, name, password, lang string, opts *RegisterOpts) (*RegisterResult, error) {
	var result RegisterResult
	body := map[string]interface{}{
		"email":     email,
		"name":      name,
		"password":  password,
		"user_type": "human",
	}
	if lang != "" {
		body["lang"] = lang
	}
	if opts != nil {
		body["account_type"] = opts.AccountType
		body["first_name"] = opts.FirstName
		body["last_name"] = opts.LastName
		body["company_name"] = opts.CompanyName
		body["company_registration"] = opts.CompanyRegistration
		body["vat_number"] = opts.VATNumber
		body["street"] = opts.Street
		body["street2"] = opts.Street2
		body["postal_code"] = opts.PostalCode
		body["city"] = opts.City
		body["state"] = opts.State
		body["country"] = opts.Country
		body["accept_terms"] = opts.AcceptTerms
		body["accept_privacy"] = opts.AcceptPrivacy
		body["accept_dpa"] = opts.AcceptDPA
		body["age_declaration"] = opts.AgeDeclaration
	}
	if err := c.unmarshal("POST", "/api/v1/auth/register", "", body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// VerifyResult holds verification response data.
type VerifyResult struct {
	Status string `json:"status"`
	Token  string `json:"token"`
	UserID string `json:"user_id"`
	Name   string `json:"name"`
	Email  string `json:"email"`
}

// VerifyUser completes email verification with the given code.
func (c *RESTClient) VerifyUser(email, code string) (*VerifyResult, error) {
	var result VerifyResult
	body := map[string]string{"email": email, "code": code}
	if err := c.unmarshal("POST", "/api/v1/auth/verify", "", body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// CompleteProfileOpts holds the address and consent fields for profile completion.
type CompleteProfileOpts struct {
	Street              string
	Street2             string
	PostalCode          string
	City                string
	State               string
	Country             string
	CompanyRegistration string
	VATNumber           string
	AcceptTerms         bool
	AcceptPrivacy       bool
	AcceptDPA           bool
	AgeDeclaration      bool
}

// CompleteProfile submits the profile completion data (address + consent).
func (c *RESTClient) CompleteProfile(token string, opts *CompleteProfileOpts) error {
	body := map[string]interface{}{
		"street":               opts.Street,
		"street2":              opts.Street2,
		"postal_code":          opts.PostalCode,
		"city":                 opts.City,
		"state":                opts.State,
		"country":              opts.Country,
		"company_registration": opts.CompanyRegistration,
		"vat_number":           opts.VATNumber,
		"accept_terms":         opts.AcceptTerms,
		"accept_privacy":       opts.AcceptPrivacy,
		"accept_dpa":           opts.AcceptDPA,
		"age_declaration":      opts.AgeDeclaration,
	}
	_, err := c.do("POST", "/api/v1/auth/complete-profile", token, body)
	return err
}

// ResendVerification requests a new email verification code.
func (c *RESTClient) ResendVerification(email string) error {
	body := map[string]string{"email": email}
	_, err := c.do("POST", "/api/v1/auth/resend-verification", "", body)
	return err
}

// VerificationStatusResult holds verification status data.
type VerificationStatusResult struct {
	Found     bool   `json:"found"`
	Email     string `json:"email"`
	Expired   bool   `json:"expired"`
	ExpiresAt string `json:"expires_at"`
}

// VerificationStatus returns the current email verification state for a pending user.
func (c *RESTClient) VerificationStatus(email string) (*VerificationStatusResult, error) {
	var result VerificationStatusResult
	if err := c.unmarshal("GET", "/api/v1/auth/verification-status?email="+email, "", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ForgotPassword requests a password reset code for the given email.
func (c *RESTClient) ForgotPassword(email string) error {
	body := map[string]string{"email": email}
	_, err := c.do("POST", "/api/v1/auth/forgot-password", "", body)
	return err
}

// ResetPassword completes a password reset with the given code and new password.
func (c *RESTClient) ResetPassword(email, code, newPassword string) error {
	body := map[string]string{"email": email, "code": code, "new_password": newPassword}
	_, err := c.do("POST", "/api/v1/auth/reset-password", "", body)
	return err
}

// --- Authenticated endpoints ---

func (c *RESTClient) Logout(token string) error {
	_, err := c.do("POST", "/api/v1/auth/logout", token, nil)
	return err
}

// Whoami returns the authenticated user profile and scope.
func (c *RESTClient) Whoami(token string) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := c.unmarshal("GET", "/api/v1/auth/whoami", token, nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// UpdateProfile updates the authenticated user profile fields.
func (c *RESTClient) UpdateProfile(token string, fields map[string]interface{}) error {
	_, err := c.do("PATCH", "/api/v1/auth/profile", token, fields)
	return err
}

// ChangePassword changes the authenticated user password.
func (c *RESTClient) ChangePassword(token, currentPassword, newPassword string) error {
	body := map[string]string{
		"current_password": currentPassword,
		"new_password":     newPassword,
	}
	_, err := c.do("POST", "/api/v1/auth/change-password", token, body)
	return err
}

// AcceptConsent records the user's acceptance of current terms, privacy, and DPA versions.
func (c *RESTClient) AcceptConsent(token string, acceptDPA bool) error {
	body := map[string]bool{"accept_terms": true, "accept_privacy": true, "accept_dpa": acceptDPA}
	_, err := c.do("POST", "/api/v1/auth/consent", token, body)
	return err
}

// AddressResult holds address data nested in a person response.
type AddressResult struct {
	ID         string `json:"id"`
	Label      string `json:"label"`
	Street     string `json:"street"`
	Street2    string `json:"street2"`
	City       string `json:"city"`
	State      string `json:"state"`
	PostalCode string `json:"postal_code"`
	Country    string `json:"country"`
}

// PersonResult holds person record data.
type PersonResult struct {
	ID                  string         `json:"id"`
	FirstName           string         `json:"first_name"`
	MiddleNames         string         `json:"middle_names"`
	LastName            string         `json:"last_name"`
	Phone               string         `json:"phone"`
	Country             string         `json:"country"`
	Language            string         `json:"language"`
	AccountType         string         `json:"account_type"`
	CompanyName         string         `json:"company_name"`
	CompanyRegistration string         `json:"company_registration"`
	Address             *AddressResult `json:"address,omitempty"`
}

// ConsentRecord holds a consent history entry.
type ConsentRecord struct {
	ID              string `json:"id"`
	DocumentType    string `json:"document_type"`
	DocumentVersion string `json:"document_version"`
	AcceptedAt      string `json:"accepted_at"`
}

// GetMyPerson returns the authenticated user's person record.
func (c *RESTClient) GetMyPerson(token string) (*PersonResult, error) {
	var result PersonResult
	if err := c.unmarshal("GET", "/api/v1/auth/person", token, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateMyPerson updates the authenticated user's person record.
func (c *RESTClient) UpdateMyPerson(token string, fields map[string]interface{}) error {
	_, err := c.do("PATCH", "/api/v1/auth/person", token, fields)
	return err
}

// RequestEmailChange initiates an email change with verification.
func (c *RESTClient) RequestEmailChange(token, newEmail string) error {
	_, err := c.do("POST", "/api/v1/auth/email/change", token, map[string]interface{}{
		"new_email": newEmail,
	})
	return err
}

// GetPendingEmailChange checks if there's a pending email change.
func (c *RESTClient) GetPendingEmailChange(token string) (bool, string, error) {
	var result struct {
		Pending  bool   `json:"pending"`
		NewEmail string `json:"new_email"`
	}
	if err := c.unmarshal("GET", "/api/v1/auth/email/pending", token, nil, &result); err != nil {
		return false, "", err
	}
	return result.Pending, result.NewEmail, nil
}

// CancelEmailChange cancels a pending email change.
func (c *RESTClient) CancelEmailChange(token string) error {
	_, err := c.do("POST", "/api/v1/auth/email/cancel", token, nil)
	return err
}

// VerifyEmailChange confirms the email change with a verification code.
func (c *RESTClient) VerifyEmailChange(token, code string) error {
	_, err := c.do("POST", "/api/v1/auth/email/verify", token, map[string]interface{}{
		"code": code,
	})
	return err
}

// ListMyConsents returns the authenticated user's consent history.
func (c *RESTClient) ListMyConsents(token string) ([]*ConsentRecord, error) {
	var result struct {
		Items []*ConsentRecord `json:"items"`
	}
	if err := c.unmarshal("GET", "/api/v1/auth/consents", token, nil, &result); err != nil {
		return nil, err
	}
	return result.Items, nil
}

// ExportMyData downloads the user's data export as raw JSON bytes.
func (c *RESTClient) ExportMyData(token string) ([]byte, error) {
	resp, err := c.do("GET", "/api/v1/auth/my-data", token, nil)
	return resp, err
}

// RequestDeletion initiates account deletion with a grace period.
func (c *RESTClient) RequestDeletion(token string) error {
	body := map[string]bool{"confirm": true}
	_, err := c.do("POST", "/api/v1/auth/delete-account", token, body)
	return err
}

// CancelDeletion cancels a pending account deletion.
func (c *RESTClient) CancelDeletion(token string) error {
	_, err := c.do("POST", "/api/v1/auth/cancel-deletion", token, nil)
	return err
}

// DeletionStatusResult holds the deletion status response.
type DeletionStatusResult struct {
	Pending     bool   `json:"pending"`
	RequestedAt string `json:"requested_at"`
	ScheduledAt string `json:"scheduled_at"`
}

// DeletionStatus returns the account deletion status.
func (c *RESTClient) DeletionStatus(token string) (*DeletionStatusResult, error) {
	var result DeletionStatusResult
	if err := c.unmarshal("GET", "/api/v1/auth/deletion-status", token, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// InvitationToken holds invitation token data.
type InvitationToken struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Token     string     `json:"token"`
	Scope     string     `json:"scope"`
	MaxUses   *int       `json:"max_uses"`
	Uses      int        `json:"use_count"`
	Status    string     `json:"status"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt *time.Time `json:"expires_at"`
}

// ListAgentTokens returns invitation tokens created by the authenticated user.
func (c *RESTClient) ListAgentTokens(token string) ([]*InvitationToken, error) {
	var result []*InvitationToken
	if err := c.unmarshal("GET", "/api/v1/agent-tokens", token, nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// CreateAgentToken creates a new agent invitation token.
func (c *RESTClient) CreateAgentToken(token string, name string, maxUses int, expiresAt string) (*InvitationToken, error) {
	var result InvitationToken
	body := map[string]interface{}{
		"name":     name,
		"max_uses": maxUses,
	}
	if expiresAt != "" {
		body["expires_at"] = expiresAt
	}
	if err := c.unmarshal("POST", "/api/v1/agent-tokens", token, body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// RevokeAgentToken revokes an agent invitation token by ID.
func (c *RESTClient) RevokeAgentToken(token string, id string) error {
	_, err := c.do("DELETE", "/api/v1/agent-tokens/"+id, token, nil)
	return err
}

// ActivityEntry holds a simplified audit entry from the my-activity API.
type ActivityEntry struct {
	Action    string `json:"action"`
	Resource  string `json:"resource,omitempty"`
	Source    string `json:"source,omitempty"`
	Summary   string `json:"summary"`
	CreatedAt string `json:"created_at"`
}

// --- My Agents endpoints ---

// MyAgent holds agent data from the my-agents endpoints.
type MyAgent struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Email            string `json:"email"`
	Status           string `json:"status"`
	UserType         string `json:"user_type"`
	CreatedAt        string `json:"created_at"`
	HealthStatus     string `json:"health_status,omitempty"`
	OnboardingStatus string `json:"onboarding_status,omitempty"`
}

// MyAgentDetail holds full agent detail from GET /my-agents/{id}.
type MyAgentDetail struct {
	ID               string                 `json:"id"`
	Name             string                 `json:"name"`
	Email            string                 `json:"email"`
	Status           string                 `json:"status"`
	UserType         string                 `json:"user_type"`
	Tier             int                    `json:"tier"`
	TierName         string                 `json:"tier_name"`
	CreatedAt        string                 `json:"created_at"`
	UpdatedAt        string                 `json:"updated_at"`
	Health           map[string]interface{} `json:"health,omitempty"`
	OnboardingStatus string                 `json:"onboarding_status,omitempty"`
}

// MyAgentActivity holds a simplified audit entry for agent activity.
type MyAgentActivity struct {
	Action    string `json:"action"`
	Resource  string `json:"resource,omitempty"`
	Summary   string `json:"summary"`
	CreatedAt string `json:"created_at"`
}

// MyAgentOnboarding holds onboarding data for an agent.
type MyAgentOnboarding struct {
	AgentID          string                   `json:"agent_id"`
	OnboardingStatus string                   `json:"onboarding_status"`
	Cooldown         map[string]interface{}   `json:"cooldown,omitempty"`
	Attempts         []map[string]interface{} `json:"attempts,omitempty"`
}

// ListMyAgents returns agents sponsored by the authenticated user.
func (c *RESTClient) ListMyAgents(token string, limit, offset int) ([]*MyAgent, int, error) {
	path := fmt.Sprintf("/api/v1/my-agents?limit=%d&offset=%d", limit, offset)
	data, total, err := c.doList(path, token)
	if err != nil {
		return nil, 0, err
	}
	var agents []*MyAgent
	if err := json.Unmarshal(data, &agents); err != nil {
		return nil, 0, err
	}
	return agents, total, nil
}

// GetMyAgent returns details for a specific sponsored agent.
func (c *RESTClient) GetMyAgent(token, agentID string) (*MyAgentDetail, error) {
	var result MyAgentDetail
	if err := c.unmarshal("GET", "/api/v1/my-agents/"+agentID, token, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateMyAgent updates a sponsored agent name or status.
func (c *RESTClient) UpdateMyAgent(token, agentID string, name, status *string) error {
	body := map[string]interface{}{}
	if name != nil {
		body["name"] = *name
	}
	if status != nil {
		body["status"] = *status
	}
	_, err := c.do("PATCH", "/api/v1/my-agents/"+agentID, token, body)
	return err
}

// ListMyAgentActivity returns recent activity for a sponsored agent.
func (c *RESTClient) ListMyAgentActivity(token, agentID string, limit, offset int) ([]*MyAgentActivity, int, error) {
	path := fmt.Sprintf("/api/v1/my-agents/%s/activity?limit=%d&offset=%d", agentID, limit, offset)
	data, total, err := c.doList(path, token)
	if err != nil {
		return nil, 0, err
	}
	var entries []*MyAgentActivity
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, 0, err
	}
	return entries, total, nil
}

// GetMyAgentOnboarding returns onboarding status for a sponsored agent.
func (c *RESTClient) GetMyAgentOnboarding(token, agentID string) (*MyAgentOnboarding, error) {
	var result MyAgentOnboarding
	if err := c.unmarshal("GET", "/api/v1/my-agents/"+agentID+"/onboarding", token, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListMyActivity returns the authenticated user audit activity.
func (c *RESTClient) ListMyActivity(token string, limit, offset int) ([]*ActivityEntry, int, error) {
	path := fmt.Sprintf("/api/v1/audit/my-activity?limit=%d&offset=%d", limit, offset)
	data, total, err := c.doList(path, token)
	if err != nil {
		return nil, 0, err
	}
	var entries []*ActivityEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, 0, err
	}
	return entries, total, nil
}

// ListMyFullActivity returns the authenticated user entity change activity.
func (c *RESTClient) ListMyFullActivity(token string, limit, offset int) ([]*UnifiedActivityEntry, int, error) {
	path := fmt.Sprintf("/api/v1/audit/my-full-activity?limit=%d&offset=%d", limit, offset)
	data, total, err := c.doList(path, token)
	if err != nil {
		return nil, 0, err
	}
	var entries []*UnifiedActivityEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, 0, err
	}
	return entries, total, nil
}

// --- My Alerts ---

// ContentAlert holds a flagged content alert.
type ContentAlert struct {
	EntityType string `json:"entity_type"`
	EntityID   string `json:"entity_id"`
	Title      string `json:"title"`
	HarmScore  int    `json:"harm_score"`
	CreatedAt  string `json:"created_at"`
}

// ListMyAlerts returns content guard alerts for the authenticated user.
func (c *RESTClient) ListMyAlerts(token string, threshold, limit, offset int) ([]*ContentAlert, int, error) {
	path := fmt.Sprintf("/api/v1/my-alerts?threshold=%d&limit=%d&offset=%d", threshold, limit, offset)
	data, total, err := c.doList(path, token)
	if err != nil {
		return nil, 0, err
	}
	var alerts []*ContentAlert
	if err := json.Unmarshal(data, &alerts); err != nil {
		return nil, 0, err
	}
	return alerts, total, nil
}

// ContentGuardUserStats holds user-scoped content guard statistics.
type ContentGuardUserStats struct {
	TotalScanned int `json:"total_scanned"`
	Clean        int `json:"clean"`
	Flagged      int `json:"flagged"`
}

// MyAlertStats fetches content guard stats for the current user's agents.
func (c *RESTClient) MyAlertStats(token string) (*ContentGuardUserStats, error) {
	var result ContentGuardUserStats
	if err := c.unmarshal("GET", "/api/v1/my-alerts/stats", token, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ContentGuardStats holds system-wide content guard statistics (admin view).
type ContentGuardStats struct {
	TotalScanned int            `json:"total_scanned"`
	Clean        int            `json:"clean"`
	Low          int            `json:"low"`
	Medium       int            `json:"medium"`
	High         int            `json:"high"`
	ByEntityType map[string]int `json:"by_entity_type"`
	LLMPending   int            `json:"llm_pending"`
	LLMCompleted int            `json:"llm_completed"`
	LLMError     int            `json:"llm_error"`
	LLMFailed    int            `json:"llm_failed"`
}

// --- Admin Content Guard Patterns ---

// PatternInfo holds data about a scoring pattern from the admin API.
type PatternInfo struct {
	Name     string `json:"name"`
	Category string `json:"category"`
	Pattern  string `json:"pattern"`
	Weight   int    `json:"weight"`
	Builtin  bool   `json:"builtin"`
	Enabled  bool   `json:"enabled"`
}

// AdminContentGuardPatterns fetches all scoring patterns with override state.
func (c *RESTClient) AdminContentGuardPatterns(token string) ([]PatternInfo, error) {
	var result []PatternInfo
	if err := c.unmarshal("GET", "/api/v1/admin/content-guard/patterns", token, nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// AdminContentGuardPatternsUpdate saves pattern overrides.
func (c *RESTClient) AdminContentGuardPatternsUpdate(token string, overrides map[string]interface{}) error {
	_, err := c.do("PATCH", "/api/v1/admin/content-guard/patterns", token, overrides)
	return err
}

// --- Org Alert Terms ---

// OrgAlertTerm holds an organization-level alert term.
type OrgAlertTerm struct {
	ID     string `json:"id"`
	Term   string `json:"term"`
	Weight int    `json:"weight"`
}

// ListOrgAlertTerms fetches alert terms for an organization.
func (c *RESTClient) ListOrgAlertTerms(token, orgID string) ([]OrgAlertTerm, error) {
	path := fmt.Sprintf("/api/v1/organizations/%s/alert-terms", orgID)
	var result []OrgAlertTerm
	if err := c.unmarshal("GET", path, token, nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// UpdateOrgAlertTerms replaces all alert terms for an organization.
func (c *RESTClient) UpdateOrgAlertTerms(token, orgID string, terms []map[string]interface{}) error {
	path := fmt.Sprintf("/api/v1/organizations/%s/alert-terms", orgID)
	_, err := c.do("PUT", path, token, terms)
	return err
}

// AdminContentGuardStats fetches system-wide content guard statistics.
func (c *RESTClient) AdminContentGuardStats(token string) (*ContentGuardStats, error) {
	var result ContentGuardStats
	if err := c.unmarshal("GET", "/api/v1/admin/content-guard/stats", token, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// AdminTaskschmiedStatus fetches Taskschmied circuit breaker status.
func (c *RESTClient) AdminTaskschmiedStatus(token string) (map[string]interface{}, error) {
	data, err := c.do("GET", "/api/v1/admin/taskschmied/status", token, nil)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// AdminTaskschmiedToggle enables or disables a specific Taskschmied LLM tier.
// Target is "primary" or "fallback".
func (c *RESTClient) AdminTaskschmiedToggle(token, target string, disabled bool) error {
	_, err := c.do("POST", "/api/v1/admin/taskschmied/toggle", token, map[string]interface{}{
		"target":   target,
		"disabled": disabled,
	})
	return err
}

// ContentGuardTestResult holds a single test result from the dry-run scorer.
type ContentGuardTestResult struct {
	Text    string   `json:"text"`
	Score   int      `json:"score"`
	Signals []string `json:"signals"`
}

// ContentGuardTestResponse holds the full test response including threshold.
type ContentGuardTestResponse struct {
	Results   []ContentGuardTestResult
	Threshold int
}

// AdminContentGuardTest sends payloads to the dry-run scorer endpoint.
func (c *RESTClient) AdminContentGuardTest(token string, payloads []string) (*ContentGuardTestResponse, error) {
	body := map[string]interface{}{"payloads": payloads}
	var resp struct {
		Results   []ContentGuardTestResult `json:"results"`
		Threshold int                      `json:"threshold"`
	}
	if err := c.unmarshal("POST", "/api/v1/admin/content-guard/test", token, body, &resp); err != nil {
		return nil, err
	}
	return &ContentGuardTestResponse{
		Results:   resp.Results,
		Threshold: resp.Threshold,
	}, nil
}

// SystemContentAlert represents a flagged entity in the system-wide alerts view.
type SystemContentAlert struct {
	EntityType string `json:"entity_type"`
	EntityID   string `json:"entity_id"`
	Title      string `json:"title"`
	HarmScore  int    `json:"harm_score"`
	CreatedAt  string `json:"created_at"`
	CreatorID  string `json:"creator_id"`
	UserID     string `json:"user_id"`
	Dismissed  bool   `json:"dismissed"`
}

// AdminContentGuardAlerts fetches system-wide flagged entities.
func (c *RESTClient) AdminContentGuardAlerts(token string, limit, offset int, includeDismissed bool) ([]SystemContentAlert, int, error) {
	path := fmt.Sprintf("/api/v1/admin/content-guard/alerts?limit=%d&offset=%d", limit, offset)
	if includeDismissed {
		path += "&include_dismissed=true"
	}
	data, total, err := c.doList(path, token)
	if err != nil {
		return nil, 0, err
	}
	var alerts []SystemContentAlert
	if err := json.Unmarshal(data, &alerts); err != nil {
		return nil, 0, fmt.Errorf("parse alerts: %w", err)
	}
	return alerts, total, nil
}

// AdminContentGuardDismiss marks a flagged entity as dismissed.
func (c *RESTClient) AdminContentGuardDismiss(token, entityType, entityID string) error {
	body := map[string]string{"entity_type": entityType, "entity_id": entityID}
	_, err := c.do("POST", "/api/v1/admin/content-guard/dismiss", token, body)
	return err
}

// --- Organizations ---

// Organization holds organization data.
type Organization struct {
	ID             string                 `json:"id"`
	Name           string                 `json:"name"`
	Description    string                 `json:"description"`
	Status         string                 `json:"status"`
	MemberCount    int                    `json:"member_count,omitempty"`
	EndeavourCount int                    `json:"endeavour_count,omitempty"`
	Metadata       map[string]interface{} `json:"metadata"`
	CreatedAt      string                 `json:"created_at"`
	UpdatedAt      string                 `json:"updated_at"`
}

// ListOrganizations returns organizations visible to the authenticated user.
func (c *RESTClient) ListOrganizations(token string, search, status string, limit, offset int) ([]*Organization, int, error) {
	path := fmt.Sprintf("/api/v1/organizations?limit=%d&offset=%d", limit, offset)
	if search != "" {
		path += "&search=" + url.QueryEscape(search)
	}
	if status != "" {
		path += "&status=" + url.QueryEscape(status)
	}
	data, total, err := c.doList(path, token)
	if err != nil {
		return nil, 0, err
	}
	var orgs []*Organization
	if err := json.Unmarshal(data, &orgs); err != nil {
		return nil, 0, err
	}
	return orgs, total, nil
}

// GetOrganization returns an organization by ID.
func (c *RESTClient) GetOrganization(token, id string) (*Organization, error) {
	var result Organization
	if err := c.unmarshal("GET", "/api/v1/organizations/"+id, token, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// CreateOrganization creates a new organization.
func (c *RESTClient) CreateOrganization(token, name, description string) (*Organization, error) {
	var result Organization
	body := map[string]string{"name": name, "description": description}
	if err := c.unmarshal("POST", "/api/v1/organizations", token, body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateOrganization applies partial updates to an organization.
func (c *RESTClient) UpdateOrganization(token, id string, fields map[string]interface{}) (*Organization, error) {
	var result Organization
	if err := c.unmarshal("PATCH", "/api/v1/organizations/"+id, token, fields, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// --- Endeavours ---

// Endeavour holds endeavour data.
type Endeavour struct {
	ID             string                   `json:"id"`
	Name           string                   `json:"name"`
	Description    string                   `json:"description"`
	Status         string                   `json:"status"`
	Timezone       string                   `json:"timezone,omitempty"`
	Lang           string                   `json:"lang,omitempty"`
	Goals          []map[string]interface{} `json:"goals,omitempty"`
	Progress       map[string]interface{}   `json:"progress,omitempty"`
	StartDate      string                   `json:"start_date,omitempty"`
	EndDate        string                   `json:"end_date,omitempty"`
	ArchivedAt         string                   `json:"archived_at,omitempty"`
	ArchivedReason     string                   `json:"archived_reason,omitempty"`
	TaskschmiedEnabled bool                     `json:"taskschmied_enabled"`
	Metadata           map[string]interface{}   `json:"metadata"`
	CreatedAt          string                   `json:"created_at"`
	UpdatedAt          string                   `json:"updated_at"`
}

// ListEndeavours returns endeavours visible to the authenticated user.
func (c *RESTClient) ListEndeavours(token string, orgID, search string, limit, offset int) ([]*Endeavour, int, error) {
	path := fmt.Sprintf("/api/v1/endeavours?limit=%d&offset=%d", limit, offset)
	if orgID != "" {
		path += "&organization_id=" + url.QueryEscape(orgID)
	}
	if search != "" {
		path += "&search=" + url.QueryEscape(search)
	}
	data, total, err := c.doList(path, token)
	if err != nil {
		return nil, 0, err
	}
	var edvs []*Endeavour
	if err := json.Unmarshal(data, &edvs); err != nil {
		return nil, 0, err
	}
	return edvs, total, nil
}

// AdminListEndeavours returns all endeavours with admin filters.
func (c *RESTClient) AdminListEndeavours(token string, orgID, search string, limit, offset int) ([]*Endeavour, int, error) {
	path := fmt.Sprintf("/api/v1/endeavours?admin=true&limit=%d&offset=%d", limit, offset)
	if orgID != "" {
		path += "&organization_id=" + url.QueryEscape(orgID)
	}
	if search != "" {
		path += "&search=" + url.QueryEscape(search)
	}
	data, total, err := c.doList(path, token)
	if err != nil {
		return nil, 0, err
	}
	var edvs []*Endeavour
	if err := json.Unmarshal(data, &edvs); err != nil {
		return nil, 0, err
	}
	return edvs, total, nil
}

// GetEndeavour returns an endeavour by ID.
func (c *RESTClient) GetEndeavour(token, id string) (*Endeavour, error) {
	var result Endeavour
	if err := c.unmarshal("GET", "/api/v1/endeavours/"+id, token, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// CreateEndeavour creates a new endeavour.
func (c *RESTClient) CreateEndeavour(token, name, description string) (*Endeavour, error) {
	var result Endeavour
	body := map[string]string{"name": name, "description": description}
	if err := c.unmarshal("POST", "/api/v1/endeavours", token, body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateEndeavour applies partial updates to an endeavour.
func (c *RESTClient) UpdateEndeavour(token, id string, fields map[string]interface{}) (*Endeavour, error) {
	var result Endeavour
	if err := c.unmarshal("PATCH", "/api/v1/endeavours/"+id, token, fields, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ArchiveImpact holds the dry-run impact of archiving an endeavour.
type ArchiveImpact struct {
	EndeavourID  string `json:"endeavour_id"`
	PlannedTasks int    `json:"planned_tasks"`
	ActiveTasks  int    `json:"active_tasks"`
	TasksToCancel int   `json:"tasks_to_cancel"`
	DoneTasks    int    `json:"done_tasks"`
	CanceledTasks int   `json:"canceled_tasks"`
}

// GetEndeavourArchiveImpact returns a dry-run impact report for archiving an endeavour.
func (c *RESTClient) GetEndeavourArchiveImpact(token, id string) (*ArchiveImpact, error) {
	var result ArchiveImpact
	if err := c.unmarshal("GET", "/api/v1/endeavours/"+id+"/archive", token, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ArchiveEndeavour archives an endeavour and cascades to its tasks.
func (c *RESTClient) ArchiveEndeavour(token, id, reason string) (*Endeavour, error) {
	var result Endeavour
	body := map[string]string{"reason": reason}
	if err := c.unmarshal("POST", "/api/v1/endeavours/"+id+"/archive", token, body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// OrgArchiveImpact holds the dry-run impact of archiving an organization.
type OrgArchiveImpact struct {
	OrganizationID      string `json:"organization_id"`
	EndeavoursToArchive int    `json:"endeavours_to_archive"`
	TotalTasksToCancel  int    `json:"total_tasks_to_cancel"`
	TotalPlannedTasks   int    `json:"total_planned_tasks"`
	TotalActiveTasks    int    `json:"total_active_tasks"`
}

// GetOrgArchiveImpact returns a dry-run impact report for archiving an organization.
func (c *RESTClient) GetOrgArchiveImpact(token, id string) (*OrgArchiveImpact, error) {
	var result OrgArchiveImpact
	if err := c.unmarshal("GET", "/api/v1/organizations/"+id+"/archive", token, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ArchiveOrganization archives an organization and cascades to its endeavours.
func (c *RESTClient) ArchiveOrganization(token, id, reason string) (*Organization, error) {
	var result Organization
	body := map[string]string{"reason": reason}
	if err := c.unmarshal("POST", "/api/v1/organizations/"+id+"/archive", token, body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// --- Tasks ---

// PortalTask holds task data (named PortalTask to avoid conflict with task tool).
type PortalTask struct {
	ID             string                 `json:"id"`
	Title          string                 `json:"title"`
	Description    string                 `json:"description"`
	Status         string                 `json:"status"`
	EndeavourID    string                 `json:"endeavour_id,omitempty"`
	EndeavourName  string                 `json:"endeavour_name,omitempty"`
	DemandID       string                 `json:"demand_id,omitempty"`
	AssigneeID     string                 `json:"assignee_id,omitempty"`
	AssigneeName   string                 `json:"assignee_name,omitempty"`
	CreatorID      string                 `json:"creator_id,omitempty"`
	CreatorName    string                 `json:"creator_name,omitempty"`
	OwnerID        string                 `json:"owner_id,omitempty"`
	OwnerName      string                 `json:"owner_name,omitempty"`
	Estimate       *float64               `json:"estimate,omitempty"`
	Actual         *float64               `json:"actual,omitempty"`
	DueDate        string                 `json:"due_date,omitempty"`
	CanceledReason string                 `json:"canceled_reason,omitempty"`
	Metadata       map[string]interface{} `json:"metadata"`
	CreatedAt      string                 `json:"created_at"`
	UpdatedAt      string                 `json:"updated_at"`
	StartedAt      string                 `json:"started_at,omitempty"`
	CompletedAt    string                 `json:"completed_at,omitempty"`
	CanceledAt     string                 `json:"canceled_at,omitempty"`
}

// ListTasks returns tasks matching the given filters.
func (c *RESTClient) ListTasks(token string, endeavourID, assigneeID, status, search, demandID string, limit, offset int) ([]*PortalTask, int, error) {
	path := fmt.Sprintf("/api/v1/tasks?limit=%d&offset=%d", limit, offset)
	if endeavourID != "" {
		path += "&endeavour_id=" + url.QueryEscape(endeavourID)
	}
	if assigneeID != "" {
		path += "&assignee_id=" + url.QueryEscape(assigneeID)
	}
	if status != "" {
		path += "&status=" + url.QueryEscape(status)
	}
	if search != "" {
		path += "&search=" + url.QueryEscape(search)
	}
	if demandID != "" {
		path += "&demand_id=" + url.QueryEscape(demandID)
	}
	data, total, err := c.doList(path, token)
	if err != nil {
		return nil, 0, err
	}
	var tasks []*PortalTask
	if err := json.Unmarshal(data, &tasks); err != nil {
		return nil, 0, err
	}
	return tasks, total, nil
}

// GetTask returns a task by ID.
func (c *RESTClient) GetTask(token, id string) (*PortalTask, error) {
	var result PortalTask
	if err := c.unmarshal("GET", "/api/v1/tasks/"+id, token, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// CreateTask creates a new task.
func (c *RESTClient) CreateTask(token string, fields map[string]interface{}) (*PortalTask, error) {
	var result PortalTask
	if err := c.unmarshal("POST", "/api/v1/tasks", token, fields, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateTask applies partial updates to a task.
func (c *RESTClient) UpdateTask(token, id string, fields map[string]interface{}) (*PortalTask, error) {
	var result PortalTask
	if err := c.unmarshal("PATCH", "/api/v1/tasks/"+id, token, fields, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// --- Messages ---

// InboxMessage holds an inbox message item.
type InboxMessage struct {
	ID         string `json:"id"`
	SenderID   string `json:"sender_id"`
	SenderName string `json:"sender_name"`
	Subject    string `json:"subject"`
	Content    string `json:"content"`
	Intent     string `json:"intent"`
	ReplyToID  string `json:"reply_to_id,omitempty"`
	EntityType string `json:"entity_type,omitempty"`
	EntityID   string `json:"entity_id,omitempty"`
	DeliveryID string `json:"delivery_id"`
	Channel    string `json:"channel"`
	Status     string `json:"status"`
	ReadAt     string `json:"read_at,omitempty"`
	CreatedAt  string `json:"created_at"`
}

// MessageDetail holds a single message with delivery info.
type MessageDetail struct {
	ID         string                 `json:"id"`
	SenderID   string                 `json:"sender_id"`
	SenderName string                 `json:"sender_name"`
	Subject    string                 `json:"subject"`
	Content    string                 `json:"content"`
	Intent     string                 `json:"intent"`
	ReplyToID  string                 `json:"reply_to_id,omitempty"`
	EntityType string                 `json:"entity_type,omitempty"`
	EntityID   string                 `json:"entity_id,omitempty"`
	Delivery   map[string]interface{} `json:"delivery,omitempty"`
	CreatedAt  string                 `json:"created_at"`
}

// ThreadMessage holds a message in a thread.
type ThreadMessage struct {
	ID         string `json:"id"`
	SenderID   string `json:"sender_id"`
	SenderName string `json:"sender_name"`
	Subject    string `json:"subject"`
	Content    string `json:"content"`
	Intent     string `json:"intent"`
	ReplyToID  string `json:"reply_to_id,omitempty"`
	CreatedAt  string `json:"created_at"`
}

// ListInbox returns inbox messages for the authenticated user.
func (c *RESTClient) ListInbox(token string, status string, unread bool, limit, offset int) ([]*InboxMessage, int, error) {
	return c.ListMessages(token, status, "", unread, limit, offset)
}

// ListMessages returns messages matching the given filters.
func (c *RESTClient) ListMessages(token string, status, intent string, unread bool, limit, offset int) ([]*InboxMessage, int, error) {
	path := fmt.Sprintf("/api/v1/messages?limit=%d&offset=%d", limit, offset)
	if status != "" {
		path += "&status=" + url.QueryEscape(status)
	}
	if intent != "" {
		path += "&intent=" + url.QueryEscape(intent)
	}
	if unread {
		path += "&unread=true"
	}
	data, total, err := c.doList(path, token)
	if err != nil {
		return nil, 0, err
	}
	var msgs []*InboxMessage
	if err := json.Unmarshal(data, &msgs); err != nil {
		return nil, 0, err
	}
	return msgs, total, nil
}

// GetMessage returns a message by ID.
func (c *RESTClient) GetMessage(token, id string) (*MessageDetail, error) {
	var result MessageDetail
	if err := c.unmarshal("GET", "/api/v1/messages/"+id, token, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetThread returns all messages in a conversation thread.
func (c *RESTClient) GetThread(token, messageID string) ([]*ThreadMessage, error) {
	data, err := c.do("GET", "/api/v1/messages/"+messageID+"/thread", token, nil)
	if err != nil {
		return nil, err
	}
	var result struct {
		Messages []*ThreadMessage `json:"messages"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result.Messages, nil
}

// SendMessage sends a new message.
func (c *RESTClient) SendMessage(token string, fields map[string]interface{}) (*MessageDetail, error) {
	var result MessageDetail
	if err := c.unmarshal("POST", "/api/v1/messages", token, fields, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ReplyMessage replies to an existing message.
func (c *RESTClient) ReplyMessage(token, messageID, content string) (*MessageDetail, error) {
	var result MessageDetail
	body := map[string]string{"content": content}
	if err := c.unmarshal("POST", "/api/v1/messages/"+messageID+"/reply", token, body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UnreadMessageCount returns the number of unread messages for the authenticated user.
func (c *RESTClient) UnreadMessageCount(token string) int {
	_, total, err := c.ListInbox(token, "", true, 1, 0)
	if err != nil {
		return 0
	}
	return total
}

// --- Comments ---

// Comment holds comment data.
type Comment struct {
	ID         string `json:"id"`
	EntityType string `json:"entity_type"`
	EntityID   string `json:"entity_id"`
	AuthorID   string `json:"author_id"`
	AuthorName string `json:"author_name"`
	Content    string `json:"content"`
	EditedAt   string `json:"edited_at,omitempty"`
	CreatedAt  string `json:"created_at"`
}

// ListComments returns comments on an entity.
func (c *RESTClient) ListComments(token, entityType, entityID string, limit, offset int) ([]*Comment, int, error) {
	path := fmt.Sprintf("/api/v1/comments?entity_type=%s&entity_id=%s&limit=%d&offset=%d", entityType, entityID, limit, offset)
	data, total, err := c.doList(path, token)
	if err != nil {
		return nil, 0, err
	}
	var comments []*Comment
	if err := json.Unmarshal(data, &comments); err != nil {
		return nil, 0, err
	}
	return comments, total, nil
}

// --- Resources ---

// PortalResource holds a resource summary for dropdowns and lists.
type PortalResource struct {
	ID     string `json:"id"`
	Type   string `json:"type"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

// ListResources returns resources visible to the authenticated user.
func (c *RESTClient) ListResources(token string, orgID string, limit, offset int) ([]*PortalResource, int, error) {
	path := fmt.Sprintf("/api/v1/resources?status=active&limit=%d&offset=%d", limit, offset)
	if orgID != "" {
		path += "&organization_id=" + url.QueryEscape(orgID)
	}
	data, total, err := c.doList(path, token)
	if err != nil {
		return nil, 0, err
	}
	var resources []*PortalResource
	if err := json.Unmarshal(data, &resources); err != nil {
		return nil, 0, err
	}
	return resources, total, nil
}

// --- User Endeavour Access ---

func (c *RESTClient) AddUserToEndeavour(token, userID, endeavourID, role string) error {
	body := map[string]string{"user_id": userID, "role": role}
	_, err := c.do("POST", "/api/v1/endeavours/"+endeavourID+"/members", token, body)
	return err
}

// RemoveEndeavourMember removes a user from an endeavour.
func (c *RESTClient) RemoveEndeavourMember(token, endeavourID, userID string) error {
	_, err := c.do("DELETE", "/api/v1/endeavours/"+endeavourID+"/members/"+userID, token, nil)
	return err
}

// ListEndeavourMembers returns the members of an endeavour.
func (c *RESTClient) ListEndeavourMembers(token, endeavourID string) ([]map[string]interface{}, error) {
	var result struct {
		Members []map[string]interface{} `json:"members"`
	}
	if err := c.unmarshal("GET", "/api/v1/endeavours/"+endeavourID+"/members", token, nil, &result); err != nil {
		return nil, err
	}
	return result.Members, nil
}

// --- Admin: Setup ---

// SetupStatusResult holds the admin setup status.
type SetupStatusResult struct {
	NeedsSetup          bool   `json:"needs_setup"`
	Phase               string `json:"phase"`
	PendingVerification bool   `json:"pending_verification"`
	Email               string `json:"email"`
	ExpiresAt           string `json:"expires_at"`
}

// SetupStatus returns the initial setup state for the instance.
func (c *RESTClient) SetupStatus() (*SetupStatusResult, error) {
	var result SetupStatusResult
	if err := c.unmarshal("GET", "/api/v1/admin/setup/status", "", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// SetupCreateResult holds the admin setup creation result.
type SetupCreateResult struct {
	Status           string `json:"status"`
	Email            string `json:"email"`
	ExpiresAt        string `json:"expires_at"`
	Message          string `json:"message"`
	EmailSent        bool   `json:"email_sent"`
	VerificationCode string `json:"verification_code,omitempty"`
}

// SetupCreate creates the master admin account during initial setup.
func (c *RESTClient) SetupCreate(email, name, password, accountType, companyName string) (*SetupCreateResult, error) {
	var result SetupCreateResult
	body := map[string]string{"email": email, "name": name, "password": password, "account_type": accountType, "company_name": companyName}
	if err := c.unmarshal("POST", "/api/v1/admin/setup", "", body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// SetupVerify completes email verification during initial setup.
func (c *RESTClient) SetupVerify(code string) error {
	body := map[string]string{"code": code}
	_, err := c.do("POST", "/api/v1/admin/setup/verify", "", body)
	return err
}

// SetupResendResult holds the resend result.
type SetupResendResult struct {
	Status    string `json:"status"`
	Email     string `json:"email"`
	ExpiresAt string `json:"expires_at"`
	Message   string `json:"message"`
}

// SetupResend requests a new verification code during initial setup.
func (c *RESTClient) SetupResend() (*SetupResendResult, error) {
	var result SetupResendResult
	if err := c.unmarshal("POST", "/api/v1/admin/setup/resend", "", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// SetupConfigure sends system configuration (timezone, language) to complete setup.
func (c *RESTClient) SetupConfigure(timezone, defaultLanguage string) error {
	body := map[string]string{"timezone": timezone, "default_language": defaultLanguage}
	_, err := c.do("POST", "/api/v1/admin/setup/configure", "", body)
	return err
}

// --- Admin: Users ---

// User holds user data from the admin API.
type User struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Email      string `json:"email"`
	Status     string `json:"status"`
	UserType   string `json:"user_type"`
	IsAdmin    bool   `json:"is_admin"`
	Tier       int    `json:"tier"`
	TierName   string `json:"tier_name"`
	Lang       string `json:"lang"`
	ResourceID string `json:"resource_id"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

// ListUsers returns users matching the given filters (admin).
func (c *RESTClient) ListUsers(token, search, status string, limit, offset int) ([]*User, int, error) {
	path := fmt.Sprintf("/api/v1/users?admin=true&limit=%d&offset=%d", limit, offset)
	if search != "" {
		path += "&search=" + url.QueryEscape(search)
	}
	if status != "" {
		path += "&status=" + url.QueryEscape(status)
	}
	data, total, err := c.doList(path, token)
	if err != nil {
		return nil, 0, err
	}
	var users []*User
	if err := json.Unmarshal(data, &users); err != nil {
		return nil, 0, err
	}
	return users, total, nil
}

// GetUser returns a user by ID.
func (c *RESTClient) GetUser(token, id string) (*User, error) {
	var result User
	if err := c.unmarshal("GET", "/api/v1/users/"+id, token, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateUser applies partial updates to a user (admin).
func (c *RESTClient) UpdateUser(token, id string, fields map[string]interface{}) (*User, error) {
	var result User
	if err := c.unmarshal("PATCH", "/api/v1/users/"+id, token, fields, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// --- Admin: Resources (full) ---

// Resource holds full resource data for admin views.
type Resource struct {
	ID            string                 `json:"id"`
	Type          string                 `json:"type"`
	Name          string                 `json:"name"`
	Status        string                 `json:"status"`
	CapacityModel string                 `json:"capacity_model,omitempty"`
	CapacityValue *float64               `json:"capacity_value,omitempty"`
	Skills        []string               `json:"skills,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt     string                 `json:"created_at"`
	UpdatedAt     string                 `json:"updated_at"`
}

// AdminListResources returns all resources with admin filters.
func (c *RESTClient) AdminListResources(token, search, resType, status, orgID string, limit, offset int) ([]*Resource, int, error) {
	path := fmt.Sprintf("/api/v1/resources?admin=true&limit=%d&offset=%d", limit, offset)
	if search != "" {
		path += "&search=" + url.QueryEscape(search)
	}
	if resType != "" {
		path += "&type=" + url.QueryEscape(resType)
	}
	if status != "" {
		path += "&status=" + url.QueryEscape(status)
	}
	if orgID != "" {
		path += "&organization_id=" + url.QueryEscape(orgID)
	}
	data, total, err := c.doList(path, token)
	if err != nil {
		return nil, 0, err
	}
	var resources []*Resource
	if err := json.Unmarshal(data, &resources); err != nil {
		return nil, 0, err
	}
	return resources, total, nil
}

// GetResource returns a resource by ID.
func (c *RESTClient) GetResource(token, id string) (*Resource, error) {
	var result Resource
	if err := c.unmarshal("GET", "/api/v1/resources/"+id, token, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateResource applies partial updates to a resource.
func (c *RESTClient) UpdateResource(token, id string, fields map[string]interface{}) (*Resource, error) {
	var result Resource
	if err := c.unmarshal("PATCH", "/api/v1/resources/"+id, token, fields, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteResource deletes a resource by ID.
func (c *RESTClient) DeleteResource(token, id string) error {
	return c.unmarshal("DELETE", "/api/v1/resources/"+id, token, nil, nil)
}

// CreateResource creates a new resource.
func (c *RESTClient) CreateResource(token string, fields map[string]interface{}) (*Resource, error) {
	var result Resource
	if err := c.unmarshal("POST", "/api/v1/resources", token, fields, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// --- Admin: Relations ---

// Relation holds entity relation data.
type Relation struct {
	ID               string                 `json:"id"`
	RelationshipType string                 `json:"relationship_type"`
	SourceEntityType string                 `json:"source_entity_type"`
	SourceEntityID   string                 `json:"source_entity_id"`
	TargetEntityType string                 `json:"target_entity_type"`
	TargetEntityID   string                 `json:"target_entity_id"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt        string                 `json:"created_at"`
}

// ListRelations returns entity relations matching the given filters.
func (c *RESTClient) ListRelations(token, sourceType, sourceID, targetType, targetID, relType string, limit, offset int) ([]*Relation, int, error) {
	path := fmt.Sprintf("/api/v1/relations?limit=%d&offset=%d", limit, offset)
	if sourceType != "" {
		path += "&source_entity_type=" + url.QueryEscape(sourceType)
	}
	if sourceID != "" {
		path += "&source_entity_id=" + url.QueryEscape(sourceID)
	}
	if targetType != "" {
		path += "&target_entity_type=" + url.QueryEscape(targetType)
	}
	if targetID != "" {
		path += "&target_entity_id=" + url.QueryEscape(targetID)
	}
	if relType != "" {
		path += "&relationship_type=" + url.QueryEscape(relType)
	}
	data, total, err := c.doList(path, token)
	if err != nil {
		return nil, 0, err
	}
	var relations []*Relation
	if err := json.Unmarshal(data, &relations); err != nil {
		return nil, 0, err
	}
	return relations, total, nil
}

// CreateRelation creates a new entity relation.
func (c *RESTClient) CreateRelation(token, relType, srcType, srcID, tgtType, tgtID string, metadata map[string]interface{}) (*Relation, error) {
	body := map[string]interface{}{
		"relationship_type":  relType,
		"source_entity_type": srcType,
		"source_entity_id":   srcID,
		"target_entity_type": tgtType,
		"target_entity_id":   tgtID,
	}
	if metadata != nil {
		body["metadata"] = metadata
	}
	var result Relation
	if err := c.unmarshal("POST", "/api/v1/relations", token, body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteRelation removes an entity relation by ID.
func (c *RESTClient) DeleteRelation(token, id string) error {
	return c.unmarshal("DELETE", "/api/v1/relations/"+id, token, nil, nil)
}

// --- Admin: Demands ---

// Demand holds demand data.
type Demand struct {
	ID             string `json:"id"`
	Type           string `json:"type"`
	Title          string `json:"title"`
	Description    string `json:"description"`
	Status         string `json:"status"`
	Priority       string `json:"priority"`
	EndeavourID    string `json:"endeavour_id"`
	EndeavourName  string `json:"endeavour_name"`
	CreatorID      string `json:"creator_id"`
	CreatorName    string `json:"creator_name"`
	OwnerID        string `json:"owner_id"`
	OwnerName      string `json:"owner_name"`
	DueDate        string `json:"due_date"`
	CanceledReason string `json:"canceled_reason"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

// ListDemands returns demands matching the given filters.
func (c *RESTClient) ListDemands(token, search, status, dtype, priority, endeavourID string, limit, offset int) ([]*Demand, int, error) {
	path := fmt.Sprintf("/api/v1/demands?limit=%d&offset=%d", limit, offset)
	if search != "" {
		path += "&search=" + url.QueryEscape(search)
	}
	if status != "" {
		path += "&status=" + url.QueryEscape(status)
	}
	if dtype != "" {
		path += "&type=" + url.QueryEscape(dtype)
	}
	if priority != "" {
		path += "&priority=" + url.QueryEscape(priority)
	}
	if endeavourID != "" {
		path += "&endeavour_id=" + url.QueryEscape(endeavourID)
	}
	data, total, err := c.doList(path, token)
	if err != nil {
		return nil, 0, err
	}
	var demands []*Demand
	if err := json.Unmarshal(data, &demands); err != nil {
		return nil, 0, err
	}
	return demands, total, nil
}

// GetDemand returns a demand by ID.
func (c *RESTClient) GetDemand(token, id string) (*Demand, error) {
	var result Demand
	if err := c.unmarshal("GET", "/api/v1/demands/"+id, token, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateDemand applies partial updates to a demand.
func (c *RESTClient) UpdateDemand(token, id string, fields map[string]interface{}) (*Demand, error) {
	var result Demand
	if err := c.unmarshal("PATCH", "/api/v1/demands/"+id, token, fields, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// CreateDemand creates a new demand.
func (c *RESTClient) CreateDemand(token string, fields map[string]interface{}) (*Demand, error) {
	var result Demand
	if err := c.unmarshal("POST", "/api/v1/demands", token, fields, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// --- Admin: Rituals ---

// Ritual holds ritual data.
type Ritual struct {
	ID            string                 `json:"id"`
	Name          string                 `json:"name"`
	Description   string                 `json:"description"`
	Prompt        string                 `json:"prompt"`
	Origin        string                 `json:"origin"`
	IsEnabled     bool                   `json:"is_enabled"`
	Lang          string                 `json:"lang"`
	Status        string                 `json:"status"`
	MethodologyID string                 `json:"methodology_id"`
	Schedule      map[string]interface{} `json:"schedule"`
	EndeavourID   string                 `json:"endeavour_id"`
	PredecessorID string                 `json:"predecessor_id"`
	CreatedBy     string                 `json:"created_by"`
	Metadata      map[string]interface{} `json:"metadata"`
	CreatedAt     string                 `json:"created_at"`
	UpdatedAt     string                 `json:"updated_at"`
}

// RitualRun holds ritual run data.
type RitualRun struct {
	ID            string                 `json:"id"`
	RitualID      string                 `json:"ritual_id"`
	Status        string                 `json:"status"`
	Trigger       string                 `json:"trigger"`
	RunBy         string                 `json:"run_by"`
	ResultSummary string                 `json:"result_summary"`
	Effects       map[string]interface{} `json:"effects"`
	Error         map[string]interface{} `json:"error"`
	Metadata      map[string]interface{} `json:"metadata"`
	StartedAt     string                 `json:"started_at"`
	FinishedAt    string                 `json:"finished_at"`
	CreatedAt     string                 `json:"created_at"`
	UpdatedAt     string                 `json:"updated_at"`
}

// ListRituals returns rituals matching the given filters.
func (c *RESTClient) ListRituals(token, search, status, origin string, limit, offset int) ([]*Ritual, int, error) {
	return c.ListRitualsFiltered(token, search, status, origin, "", "", limit, offset)
}

// ListRitualsFiltered returns rituals matching extended filters including language.
func (c *RESTClient) ListRitualsFiltered(token, search, status, origin, endeavourID, lang string, limit, offset int) ([]*Ritual, int, error) {
	path := fmt.Sprintf("/api/v1/rituals?limit=%d&offset=%d", limit, offset)
	if search != "" {
		path += "&search=" + url.QueryEscape(search)
	}
	if status != "" {
		path += "&status=" + url.QueryEscape(status)
	}
	if origin != "" {
		path += "&origin=" + url.QueryEscape(origin)
	}
	if endeavourID != "" {
		path += "&endeavour_id=" + url.QueryEscape(endeavourID)
	}
	if lang != "" {
		path += "&lang=" + url.QueryEscape(lang)
	}
	data, total, err := c.doList(path, token)
	if err != nil {
		return nil, 0, err
	}
	var rituals []*Ritual
	if err := json.Unmarshal(data, &rituals); err != nil {
		return nil, 0, err
	}
	return rituals, total, nil
}

// GetRitual returns a ritual by ID.
func (c *RESTClient) GetRitual(token, id string) (*Ritual, error) {
	var result Ritual
	if err := c.unmarshal("GET", "/api/v1/rituals/"+id, token, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateRitual applies partial updates to a ritual.
func (c *RESTClient) UpdateRitual(token, id string, fields map[string]interface{}) (*Ritual, error) {
	var result Ritual
	if err := c.unmarshal("PATCH", "/api/v1/rituals/"+id, token, fields, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetRitualLineage returns the version chain for a ritual.
func (c *RESTClient) GetRitualLineage(token, id string) ([]*Ritual, error) {
	data, err := c.do("GET", "/api/v1/rituals/"+id+"/lineage", token, nil)
	if err != nil {
		return nil, err
	}
	var rituals []*Ritual
	if err := json.Unmarshal(data, &rituals); err != nil {
		return nil, err
	}
	return rituals, nil
}

// ListRitualRuns returns ritual runs matching the given filters.
func (c *RESTClient) ListRitualRuns(token, ritualID, status string, limit, offset int) ([]*RitualRun, int, error) {
	path := fmt.Sprintf("/api/v1/ritual-runs?limit=%d&offset=%d", limit, offset)
	if ritualID != "" {
		path += "&ritual_id=" + url.QueryEscape(ritualID)
	}
	if status != "" {
		path += "&status=" + url.QueryEscape(status)
	}
	data, total, err := c.doList(path, token)
	if err != nil {
		return nil, 0, err
	}
	var runs []*RitualRun
	if err := json.Unmarshal(data, &runs); err != nil {
		return nil, 0, err
	}
	return runs, total, nil
}

// CreateRitualRun triggers a new ritual run.
func (c *RESTClient) CreateRitualRun(token, ritualID, trigger string) (*RitualRun, error) {
	var result RitualRun
	body := map[string]string{"ritual_id": ritualID, "trigger": trigger}
	if err := c.unmarshal("POST", "/api/v1/ritual-runs", token, body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ForkRitual creates a new ritual derived from an existing one.
func (c *RESTClient) ForkRitual(token, id string, fields map[string]interface{}) (*Ritual, error) {
	var result Ritual
	if err := c.unmarshal("POST", "/api/v1/rituals/"+id+"/fork", token, fields, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// --- Admin: Report Templates ---

// ReportTemplate holds a report template (DB-stored Go text/template).
type ReportTemplate struct {
	ID            string                 `json:"id"`
	Name          string                 `json:"name"`
	Type          string                 `json:"type"`
	Scope         string                 `json:"scope"`
	Lang          string                 `json:"lang"`
	Body          string                 `json:"body"`
	Version       int                    `json:"version"`
	PredecessorID string                 `json:"predecessor_id"`
	CreatedBy     string                 `json:"created_by"`
	Status        string                 `json:"status"`
	Metadata      map[string]interface{} `json:"metadata"`
	CreatedAt     string                 `json:"created_at"`
	UpdatedAt     string                 `json:"updated_at"`
}

// ListTemplates returns report templates matching the given filters.
func (c *RESTClient) ListTemplates(token, scope, lang, status, search string, limit, offset int) ([]*ReportTemplate, int, error) {
	path := fmt.Sprintf("/api/v1/templates?limit=%d&offset=%d", limit, offset)
	if scope != "" {
		path += "&scope=" + url.QueryEscape(scope)
	}
	if lang != "" {
		path += "&lang=" + url.QueryEscape(lang)
	}
	if status != "" {
		path += "&status=" + url.QueryEscape(status)
	}
	if search != "" {
		path += "&search=" + url.QueryEscape(search)
	}
	data, total, err := c.doList(path, token)
	if err != nil {
		return nil, 0, err
	}
	var templates []*ReportTemplate
	if err := json.Unmarshal(data, &templates); err != nil {
		return nil, 0, err
	}
	return templates, total, nil
}

// GetTemplate returns a report template by ID.
func (c *RESTClient) GetTemplate(token, id string) (*ReportTemplate, error) {
	var result ReportTemplate
	if err := c.unmarshal("GET", "/api/v1/templates/"+id, token, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateTemplate applies partial updates to a report template.
func (c *RESTClient) UpdateTemplate(token, id string, fields map[string]interface{}) (*ReportTemplate, error) {
	var result ReportTemplate
	if err := c.unmarshal("PATCH", "/api/v1/templates/"+id, token, fields, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ForkTemplate creates a new template derived from an existing one.
func (c *RESTClient) ForkTemplate(token, sourceID string, fields map[string]interface{}) (*ReportTemplate, error) {
	var result ReportTemplate
	if err := c.unmarshal("POST", "/api/v1/templates/"+sourceID+"/fork", token, fields, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// --- Admin: Audit ---

// AuditEntry holds an audit log entry.
type AuditEntry struct {
	ID         string                 `json:"id"`
	Action     string                 `json:"action"`
	ActorID    string                 `json:"actor_id"`
	ActorType  string                 `json:"actor_type"`
	Resource   string                 `json:"resource"`
	Method     string                 `json:"method"`
	Endpoint   string                 `json:"endpoint"`
	StatusCode int                    `json:"status_code"`
	IP         string                 `json:"ip"`
	Source     string                 `json:"source"`
	DurationMs int64                  `json:"duration_ms"`
	Metadata   map[string]interface{} `json:"metadata"`
	CreatedAt  string                 `json:"created_at"`
	Summary    string                 `json:"-"`
}

// ListAuditLog returns audit log entries matching the given filters.
func (c *RESTClient) ListAuditLog(token, action, actorID, excludeAction, source string, limit, offset int) ([]*AuditEntry, int, error) {
	path := fmt.Sprintf("/api/v1/audit?limit=%d&offset=%d", limit, offset)
	if action != "" {
		path += "&action=" + url.QueryEscape(action)
	}
	if actorID != "" {
		path += "&actor_id=" + url.QueryEscape(actorID)
	}
	if excludeAction != "" {
		path += "&exclude_action=" + url.QueryEscape(excludeAction)
	}
	if source != "" {
		path += "&source=" + url.QueryEscape(source)
	}
	data, total, err := c.doList(path, token)
	if err != nil {
		return nil, 0, err
	}
	var entries []*AuditEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, 0, err
	}
	return entries, total, nil
}

// --- Entity Changes ---

// EntityChangeEntry holds an entity change record.
type EntityChangeEntry struct {
	ID          string                 `json:"id"`
	ActorID     string                 `json:"actor_id"`
	Action      string                 `json:"action"`
	EntityType  string                 `json:"entity_type"`
	EntityID    string                 `json:"entity_id"`
	EndeavourID string                 `json:"endeavour_id"`
	Fields      []string               `json:"fields"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt   string                 `json:"created_at"`
}

// ListEntityChanges returns entity change history entries.
func (c *RESTClient) ListEntityChanges(token, action, entityType, endeavourID, actorID string, limit, offset int) ([]*EntityChangeEntry, int, error) {
	path := fmt.Sprintf("/api/v1/entity-changes?limit=%d&offset=%d", limit, offset)
	if action != "" {
		path += "&action=" + url.QueryEscape(action)
	}
	if entityType != "" {
		path += "&entity_type=" + url.QueryEscape(entityType)
	}
	if endeavourID != "" {
		path += "&endeavour_id=" + url.QueryEscape(endeavourID)
	}
	if actorID != "" {
		path += "&actor_id=" + url.QueryEscape(actorID)
	}
	data, total, err := c.doList(path, token)
	if err != nil {
		return nil, 0, err
	}
	var entries []*EntityChangeEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, 0, err
	}
	return entries, total, nil
}

// --- Unified Activity ---

// UnifiedActivityEntry represents a merged audit/entity-change entry.
type UnifiedActivityEntry struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	Action      string                 `json:"action"`
	ActorID     string                 `json:"actor_id"`
	Summary     string                 `json:"summary"`
	EntityType  string                 `json:"entity_type,omitempty"`
	EntityID    string                 `json:"entity_id,omitempty"`
	EndeavourID string                 `json:"endeavour_id,omitempty"`
	Fields      []string               `json:"fields,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Source      string                 `json:"source,omitempty"`
	CreatedAt   string                 `json:"created_at"`
}

// ActivitySummary holds aggregated counts.
type ActivitySummary struct {
	Logins       int `json:"logins"`
	Tasks        int `json:"tasks"`
	Demands      int `json:"demands"`
	Endeavours   int `json:"endeavours"`
	Resources    int `json:"resources"`
	Orgs         int `json:"organizations"`
	Other        int `json:"other"`
	Total        int `json:"total"`
	UniqueActors int `json:"unique_actors"`
}

// HourBucket holds per-hour activity counts for sparkline visualization.
type HourBucket struct {
	Hour       int `json:"hour"`
	Logins     int `json:"logins"`
	Tasks      int `json:"tasks"`
	Demands    int `json:"demands"`
	Endeavours int `json:"endeavours"`
}

// ActivityResponse holds the response from the unified activity endpoint.
type ActivityResponse struct {
	Data    []*UnifiedActivityEntry `json:"data"`
	Summary ActivitySummary         `json:"summary"`
	Hourly  []HourBucket            `json:"hourly"`
	Meta    struct {
		Total  int `json:"total"`
		Limit  int `json:"limit"`
		Offset int `json:"offset"`
	} `json:"meta"`
}

// ListActivity returns unified activity entries for the authenticated user.
func (c *RESTClient) ListActivity(token, action, entityType, actorID, startTime, endTime string, limit, offset int) (*ActivityResponse, error) {
	return c.listActivity(token, action, entityType, actorID, startTime, endTime, limit, offset, false)
}

// AdminListActivity returns unified activity entries with admin scope.
func (c *RESTClient) AdminListActivity(token, action, entityType, actorID, startTime, endTime string, limit, offset int) (*ActivityResponse, error) {
	return c.listActivity(token, action, entityType, actorID, startTime, endTime, limit, offset, true)
}

func (c *RESTClient) listActivity(token, action, entityType, actorID, startTime, endTime string, limit, offset int, admin bool) (*ActivityResponse, error) {
	path := fmt.Sprintf("/api/v1/activity?limit=%d&offset=%d", limit, offset)
	if admin {
		path += "&admin=true"
	}
	if action != "" {
		path += "&action=" + url.QueryEscape(action)
	}
	if entityType != "" {
		path += "&entity_type=" + url.QueryEscape(entityType)
	}
	if actorID != "" {
		path += "&actor_id=" + url.QueryEscape(actorID)
	}
	if startTime != "" {
		path += "&start_time=" + url.QueryEscape(startTime)
	}
	if endTime != "" {
		path += "&end_time=" + url.QueryEscape(endTime)
	}

	req, err := http.NewRequest("GET", c.baseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("X-Source", "portal")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		var errResp struct {
			Error *apiError `json:"error"`
		}
		if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error != nil {
			return nil, &RESTError{StatusCode: resp.StatusCode, Code: errResp.Error.Code, Message: errResp.Error.Message}
		}
		return nil, &RESTError{StatusCode: resp.StatusCode, Code: "unknown", Message: "request failed"}
	}

	var result ActivityResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return &result, nil
}

// --- Admin: Stats & Settings ---

// Stats holds system-wide entity counts.
type Stats struct {
	Organizations int `json:"organizations"`
	Users         int `json:"users"`
	MasterAdmins  int `json:"master_admins"`
	Endeavours    int `json:"endeavours"`
	Demands       int `json:"demands"`
	Tasks         int `json:"tasks"`
	Resources     int `json:"resources"`
	Artifacts     int `json:"artifacts"`
	Rituals       int `json:"rituals"`
	RitualRuns    int `json:"ritual_runs"`
	Relations     int `json:"relations"`
}

// AdminStats returns system-wide statistics for the admin dashboard.
func (c *RESTClient) AdminStats(token string) (*Stats, error) {
	var result Stats
	if err := c.unmarshal("GET", "/api/v1/admin/stats", token, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// SettingsResult holds admin settings.
type SettingsResult struct {
	MCPAccessEnabled bool `json:"mcp_access_enabled"`
}

// AdminSettings returns admin-level settings.
func (c *RESTClient) AdminSettings(token string) (*SettingsResult, error) {
	var result SettingsResult
	if err := c.unmarshal("GET", "/api/v1/admin/settings", token, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateAdminSettings updates admin-level settings.
func (c *RESTClient) UpdateAdminSettings(token string, mcpEnabled *bool) (*SettingsResult, error) {
	var result SettingsResult
	body := map[string]interface{}{}
	if mcpEnabled != nil {
		body["mcp_access_enabled"] = *mcpEnabled
	}
	if err := c.unmarshal("PATCH", "/api/v1/admin/settings", token, body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// AdminQuotas returns the current quota configuration.
func (c *RESTClient) AdminQuotas(token string) (map[string]string, error) {
	var result map[string]string
	if err := c.unmarshal("GET", "/api/v1/admin/quotas", token, nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// UpdateAdminQuotas updates quota configuration values.
func (c *RESTClient) UpdateAdminQuotas(token string, quotas map[string]string) (map[string]string, error) {
	var result map[string]string
	if err := c.unmarshal("PATCH", "/api/v1/admin/quotas", token, quotas, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// --- Indicators (Ablecon/Harmcon) ---

// IndicatorLevel represents a single DEFCON-style level.
type IndicatorLevel struct {
	Level       int                    `json:"level"`
	Label       string                 `json:"label"`
	Reason      map[string]interface{} `json:"reason,omitempty"`
	HighCount   int                    `json:"high_count,omitempty"`
	MediumCount int                    `json:"medium_count,omitempty"`
	LowCount    int                    `json:"low_count,omitempty"`
}

// UserIndicators holds the user-scoped Ablecon and Harmcon levels.
type UserIndicators struct {
	Ablecon *IndicatorLevel `json:"ablecon"`
	Harmcon *IndicatorLevel `json:"harmcon"`
}

// AdminIndicators holds system-wide Ablecon/Harmcon levels plus org breakdown.
type AdminIndicators struct {
	Ablecon    *IndicatorLevel `json:"ablecon"`
	Harmcon    *IndicatorLevel `json:"harmcon"`
	OrgAblecon []OrgIndicator  `json:"org_ablecon,omitempty"`
}

// OrgIndicator holds an organization's Ablecon level.
type OrgIndicator struct {
	OrgID   string `json:"org_id"`
	OrgName string `json:"org_name"`
	Level   int    `json:"level"`
	Label   string `json:"label"`
}

// MyIndicators fetches the user-scoped Ablecon and Harmcon levels.
func (c *RESTClient) MyIndicators(token string) (*UserIndicators, error) {
	var result UserIndicators
	if err := c.unmarshal("GET", "/api/v1/my-indicators", token, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// AdminIndicatorsData fetches system-wide Ablecon/Harmcon levels plus org Ablecon.
func (c *RESTClient) AdminIndicatorsData(token string) (*AdminIndicators, error) {
	var result AdminIndicators
	if err := c.unmarshal("GET", "/api/v1/admin/indicators", token, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// --- Admin: KPI ---

// KPISnapshot holds KPI data.
type KPISnapshot struct {
	Timestamp string         `json:"timestamp"`
	Entities  map[string]int `json:"entities"`
	Tasks     map[string]int `json:"tasks"`
	Demands   map[string]int `json:"demands"`
	Users     map[string]int `json:"users"`
	Security  map[string]int `json:"security"`
}

// KPICurrent returns the latest KPI snapshot.
func (c *RESTClient) KPICurrent(token string) (*KPISnapshot, error) {
	data, err := c.do("GET", "/api/v1/kpi/current", token, nil)
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, nil
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	snap := &KPISnapshot{
		Entities: make(map[string]int),
		Tasks:    make(map[string]int),
		Demands:  make(map[string]int),
		Users:    make(map[string]int),
		Security: make(map[string]int),
	}
	if ts, ok := raw["timestamp"].(string); ok {
		snap.Timestamp = ts
	}
	parseIntMap := func(key string) map[string]int {
		m := make(map[string]int)
		if sub, ok := raw[key].(map[string]interface{}); ok {
			for k, v := range sub {
				if num, ok := v.(float64); ok {
					m[k] = int(num)
				}
			}
		}
		return m
	}
	snap.Entities = parseIntMap("entities")
	snap.Tasks = parseIntMap("tasks")
	snap.Demands = parseIntMap("demands")
	snap.Users = parseIntMap("users")
	snap.Security = parseIntMap("security")
	return snap, nil
}

// KPIHistoryTyped returns historical KPI snapshots.
func (c *RESTClient) KPIHistoryTyped(token string, limit int) ([]*KPISnapshot, error) {
	path := fmt.Sprintf("/api/v1/kpi/history?limit=%d", limit)
	data, _, err := c.doList(path, token)
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, nil
	}
	var rawItems []map[string]interface{}
	if err := json.Unmarshal(data, &rawItems); err != nil {
		return nil, err
	}
	parseIntMap := func(raw map[string]interface{}, key string) map[string]int {
		m := make(map[string]int)
		if sub, ok := raw[key].(map[string]interface{}); ok {
			for k, v := range sub {
				if num, ok := v.(float64); ok {
					m[k] = int(num)
				}
			}
		}
		return m
	}
	var snapshots []*KPISnapshot
	for _, item := range rawItems {
		snap := &KPISnapshot{
			Entities: make(map[string]int),
			Tasks:    make(map[string]int),
			Demands:  make(map[string]int),
			Users:    make(map[string]int),
			Security: make(map[string]int),
		}
		if ts, ok := item["timestamp"].(string); ok {
			snap.Timestamp = ts
		}
		snap.Entities = parseIntMap(item, "entities")
		snap.Tasks = parseIntMap(item, "tasks")
		snap.Demands = parseIntMap(item, "demands")
		snap.Users = parseIntMap(item, "users")
		snap.Security = parseIntMap(item, "security")
		snapshots = append(snapshots, snap)
	}
	return snapshots, nil
}

// --- Admin: Invitations ---

func (c *RESTClient) ListInvitations(token string) ([]*InvitationToken, error) {
	var result []*InvitationToken
	if err := c.unmarshal("GET", "/api/v1/invitations", token, nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// CreateInvitation creates a new invitation token (admin).
func (c *RESTClient) CreateInvitation(token string, name string, maxUses int, expiresAt string) (*InvitationToken, error) {
	var result InvitationToken
	body := map[string]interface{}{
		"name":     name,
		"max_uses": maxUses,
	}
	if expiresAt != "" {
		body["expires_at"] = expiresAt
	}
	if err := c.unmarshal("POST", "/api/v1/invitations", token, body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// RevokeInvitation revokes an invitation token (admin).
func (c *RESTClient) RevokeInvitation(token string, id string) error {
	_, err := c.do("DELETE", "/api/v1/invitations/"+id, token, nil)
	return err
}

// --- Admin: Organizations (with status filter) ---

func (c *RESTClient) AdminListOrganizations(token, search, status string, limit, offset int) ([]*Organization, int, error) {
	path := fmt.Sprintf("/api/v1/organizations?admin=true&limit=%d&offset=%d", limit, offset)
	if search != "" {
		path += "&search=" + url.QueryEscape(search)
	}
	if status != "" {
		path += "&status=" + url.QueryEscape(status)
	}
	data, total, err := c.doList(path, token)
	if err != nil {
		return nil, 0, err
	}
	var orgs []*Organization
	if err := json.Unmarshal(data, &orgs); err != nil {
		return nil, 0, err
	}
	return orgs, total, nil
}

// --- Reports ---

// ReportResult holds the generated report data.
type ReportResult struct {
	Scope       string `json:"scope"`
	EntityID    string `json:"entity_id"`
	Title       string `json:"title"`
	Markdown    string `json:"markdown"`
	GeneratedAt string `json:"generated_at"`
}

// GenerateReport calls the REST API to generate a Markdown report.
func (c *RESTClient) GenerateReport(token, scope, entityID string) (*ReportResult, error) {
	var result ReportResult
	path := fmt.Sprintf("/api/v1/reports/%s/%s", scope, entityID)
	if err := c.unmarshal("GET", path, token, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// EmailReport calls the REST API to generate and email a report to the caller.
func (c *RESTClient) EmailReport(token, scope, entityID string) error {
	path := fmt.Sprintf("/api/v1/reports/%s/%s/email", scope, entityID)
	var result map[string]interface{}
	return c.unmarshal("POST", path, token, nil, &result)
}

// ExportRaw fetches a raw JSON export from the REST API and returns the response body.
// The caller must close the returned ReadCloser.
func (c *RESTClient) ExportRaw(token, path string) (io.ReadCloser, error) {
	req, err := http.NewRequest("GET", c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("export failed: %s", resp.Status)
	}
	return resp.Body, nil
}

// AgentBlockSignal holds aggregated block signal data for a sponsor.
type AgentBlockSignal struct {
	SponsorUserID string `json:"sponsor_user_id"`
	SponsorName   string `json:"sponsor_name"`
	SponsorEmail  string `json:"sponsor_email"`
	TotalAgents   int    `json:"total_agents"`
	BlockedCount  int    `json:"blocked_count"`
}

// GetAdminAgentBlockSignals fetches the block signal aggregation for admins.
func (c *RESTClient) GetAdminAgentBlockSignals(token string) ([]AgentBlockSignal, error) {
	var result []AgentBlockSignal
	if err := c.unmarshal("GET", "/api/v1/admin/agent-block-signals", token, nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// TierUsageSummary holds per-tier entity counts for admin display.
type TierUsageSummary struct {
	TierID     int    `json:"tier_id"`
	TierName   string `json:"tier_name"`
	Users      int    `json:"users"`
	Orgs       int    `json:"orgs"`
	Endeavours int    `json:"endeavours"`
	Teams      int    `json:"teams"`
	Agents     int    `json:"agents"`
}

// AdminTierDefinition mirrors storage.TierDefinition for the portal.
type AdminTierDefinition struct {
	ID                  int    `json:"id"`
	Name                string `json:"name"`
	MaxUsers            int    `json:"max_users"`
	MaxOrgs             int    `json:"max_orgs"`
	MaxAgentsPerOrg     int    `json:"max_agents_per_org"`
	MaxEndeavoursPerOrg int    `json:"max_endeavours_per_org"`
	MaxActiveEndeavours int    `json:"max_active_endeavours"`
	MaxTeamsPerOrg      int    `json:"max_teams_per_org"`
	MaxCreationsPerHour int    `json:"max_creations_per_hour"`
}

// AdminTiers fetches all tier definitions.
func (c *RESTClient) AdminTiers(token string) ([]*AdminTierDefinition, error) {
	var result []*AdminTierDefinition
	if err := c.unmarshal("GET", "/api/v1/admin/tiers", token, nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// AdminTierUsage fetches the tier usage summary for the admin dashboard.
func (c *RESTClient) AdminTierUsage(token string) ([]*TierUsageSummary, error) {
	var result []*TierUsageSummary
	if err := c.unmarshal("GET", "/api/v1/admin/tier-usage", token, nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// HealthInfo holds the response from the MCP health endpoint.
type HealthInfo struct {
	Service string `json:"service"`
	Status  string `json:"status"`
	Version string `json:"version"`
}

// Health checks the MCP server health endpoint (unauthenticated).
func (c *RESTClient) Health() (*HealthInfo, error) {
	var info HealthInfo
	resp, err := c.httpClient.Get(c.baseURL + "/mcp/health")
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}
	return &info, nil
}

// BaseURL returns the configured API base URL.
func (c *RESTClient) BaseURL() string {
	return c.baseURL
}

// ProbeHealth checks an arbitrary health endpoint and returns status and version.
// Returns ("healthy", version) on success, ("unreachable", "") on failure.
func (c *RESTClient) ProbeHealth(url string) (status, version string) {
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return "unreachable", ""
	}
	defer func() { _ = resp.Body.Close() }()
	var info struct {
		Status  string `json:"status"`
		Version string `json:"version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return "unreachable", ""
	}
	if info.Status == "" || info.Status == "ok" {
		info.Status = "healthy"
	}
	return info.Status, info.Version
}
