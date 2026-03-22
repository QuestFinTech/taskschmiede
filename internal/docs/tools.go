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


package docs

// DefaultRegistry returns the registry with all Taskschmiede tool documentation.
func DefaultRegistry(version string) *Registry {
	r := NewRegistry(version, "https://docs.taskschmiede.dev")

	// Register all tools
	registerAuthTools(r)
	registerTokenTools(r)
	registerInvitationTools(r)
	registerRegistrationTools(r)
	registerUserTools(r)
	registerOrganizationTools(r)
	registerEndeavourTools(r)
	registerTaskTools(r)
	registerRelationshipTools(r)
	registerResourceTools(r)
	registerDemandTools(r)
	registerRelationTools(r)
	registerArtifactTools(r)
	registerRitualTools(r)
	registerRitualRunTools(r)
	registerTemplateTools(r)
	registerReportTools(r)
	registerCommentTools(r)
	registerApprovalTools(r)
	registerDodTools(r)
	registerMessageTools(r)
	registerAuditTools(r)
	registerOnboardingDocsTools(r)
	registerDocTools(r)

	// Register embedded content docs (guides and workflows from Hugo Markdown files)
	RegisterEmbeddedDocs(r)

	return r
}

func registerAuthTools(r *Registry) {
	r.Register(&ToolDoc{
		Name:     "ts.auth.login",
		Category: "auth",
		Summary:  "Authenticate with email and password to get an access token.",
		Description: `Authenticates a user with their email and password credentials.
On success, returns an access token that can be used for subsequent API calls.

The token should be included in the Authorization header as a Bearer token:
Authorization: Bearer <token>

Tokens are single-use session tokens by default. For long-lived API tokens,
use ts.tkn.create after authentication.`,
		Parameters: []ParamDoc{
			{
				Name:        "email",
				Type:        "string",
				Description: "User's email address",
				Required:    true,
				Example:     "agent@example.com",
			},
			{
				Name:        "password",
				Type:        "string",
				Description: "User's password",
				Required:    true,
				Example:     "SecurePassword123!",
			},
		},
		RequiredParams: []string{"email", "password"},
		Returns: ReturnDoc{
			Description: "Returns a session token and user information on success.",
			Schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"token":      map[string]interface{}{"type": "string"},
					"user_id":    map[string]interface{}{"type": "string"},
					"expires_at": map[string]interface{}{"type": "string", "format": "date-time"},
				},
			},
			Example: map[string]interface{}{
				"token":      "ts_abc123def456...",
				"user_id":    "usr_01H8X9...",
				"expires_at": "2026-02-05T10:30:00Z",
			},
		},
		Errors: []ErrorDoc{
			{Code: "invalid_input", Description: "Email and password are required"},
			{Code: "rate_limited", Description: "Too many login attempts"},
			{Code: "invalid_credentials", Description: "Email or password is incorrect"},
		},
		Examples: []ExampleDoc{
			{
				Title:       "Basic login",
				Description: "Authenticate to get a session token.",
				Input: map[string]interface{}{
					"email":    "agent@example.com",
					"password": "SecurePassword123!",
				},
				Output: map[string]interface{}{
					"token":      "ts_abc123def456...",
					"user_id":    "usr_01H8X9ABCDEF",
					"expires_at": "2026-02-05T10:30:00Z",
				},
			},
		},
		RequiresAuth: false,
		RelatedTools: []string{"ts.tkn.verify", "ts.tkn.create"},
		Since:        "v0.1.0",
	})

	r.Register(&ToolDoc{
		Name:     "ts.auth.whoami",
		Category: "auth",
		Summary:  "Get the current user's profile, tier, limits, usage, and scope.",
		Description: `Returns detailed information about the authenticated user including their
profile, tier, usage limits, and scope. Uses session authentication --
no parameters required.`,
		Parameters:     []ParamDoc{},
		RequiredParams: []string{},
		Returns: ReturnDoc{
			Description: "Returns the authenticated user's profile and limits.",
			Schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"user_id": map[string]interface{}{"type": "string"},
					"email":   map[string]interface{}{"type": "string"},
					"name":    map[string]interface{}{"type": "string"},
					"tier":    map[string]interface{}{"type": "string"},
				},
			},
			Example: map[string]interface{}{
				"user_id": "usr_01H8X9ABCDEF",
				"email":   "agent@example.com",
				"name":    "Agent Smith",
				"tier":    "admin",
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
		},
		Examples: []ExampleDoc{
			{
				Title:       "Get current user",
				Description: "Check who you are authenticated as.",
				Input:       map[string]interface{}{},
				Output: map[string]interface{}{
					"user_id": "usr_01H8X9ABCDEF",
					"email":   "agent@example.com",
					"name":    "Agent Smith",
					"tier":    "admin",
				},
			},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.auth.login"},
		Since:        "v0.1.0",
	})

	r.Register(&ToolDoc{
		Name:     "ts.auth.forgot_password",
		Category: "auth",
		Summary:  "Request a password reset code via email.",
		Description: `Initiates the password reset flow. If the email address is associated with
an account, a reset code is sent via email. The response is identical
regardless of whether the account exists (prevents email enumeration).

When email is not configured, the reset code is returned directly in
the response for development convenience.`,
		Parameters: []ParamDoc{
			{
				Name:        "email",
				Type:        "string",
				Description: "Email address of the account to reset",
				Required:    true,
				Example:     "agent@example.com",
			},
		},
		RequiredParams: []string{"email"},
		Returns: ReturnDoc{
			Description: "Returns a confirmation that the reset was requested.",
			Schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"status":     map[string]interface{}{"type": "string"},
					"expires_in": map[string]interface{}{"type": "string"},
					"note":       map[string]interface{}{"type": "string"},
				},
			},
			Example: map[string]interface{}{
				"status":     "reset_requested",
				"expires_in": "15m0s",
				"note":       "If an account exists with that email, a reset code has been sent.",
			},
		},
		Errors: []ErrorDoc{
			{Code: "invalid_input", Description: "Email is required"},
			{Code: "rate_limited", Description: "Too many requests for this email"},
		},
		Examples: []ExampleDoc{
			{
				Title:       "Request password reset",
				Description: "Initiate a password reset for an account.",
				Input: map[string]interface{}{
					"email": "agent@example.com",
				},
				Output: map[string]interface{}{
					"status":     "reset_requested",
					"expires_in": "15m0s",
					"note":       "If an account exists with that email, a reset code has been sent.",
				},
			},
		},
		RequiresAuth: false,
		RelatedTools: []string{"ts.auth.reset_password", "ts.auth.login"},
		Since:        "v0.3.0",
	})

	r.Register(&ToolDoc{
		Name:     "ts.auth.reset_password",
		Category: "auth",
		Summary:  "Complete a password reset with the emailed code.",
		Description: `Validates the reset code and sets a new password. On success, all
existing sessions and tokens for the user are invalidated.

The new password must meet the same requirements as registration:
12+ characters, at least one uppercase, lowercase, digit, and special character.`,
		Parameters: []ParamDoc{
			{
				Name:        "email",
				Type:        "string",
				Description: "Email address of the account",
				Required:    true,
				Example:     "agent@example.com",
			},
			{
				Name:        "code",
				Type:        "string",
				Description: "Reset code from the email (format: xxx-xxx-xxx)",
				Required:    true,
				Example:     "abc-def-ghi",
			},
			{
				Name:        "new_password",
				Type:        "string",
				Description: "New password (12+ chars, mixed case, digit, special)",
				Required:    true,
				Example:     "NewSecurePassword123!",
			},
		},
		RequiredParams: []string{"email", "code", "new_password"},
		Returns: ReturnDoc{
			Description: "Returns confirmation that the password was reset.",
			Schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"status": map[string]interface{}{"type": "string"},
					"note":   map[string]interface{}{"type": "string"},
				},
			},
			Example: map[string]interface{}{
				"status": "password_reset",
				"note":   "Password has been changed. All existing sessions have been invalidated.",
			},
		},
		Errors: []ErrorDoc{
			{Code: "invalid_input", Description: "Email, code, and new_password are required"},
			{Code: "invalid_input", Description: "Password does not meet requirements"},
			{Code: "reset_failed", Description: "Invalid or expired reset code"},
		},
		Examples: []ExampleDoc{
			{
				Title:       "Complete password reset",
				Description: "Use the emailed code to set a new password.",
				Input: map[string]interface{}{
					"email":        "agent@example.com",
					"code":         "abc-def-ghi",
					"new_password": "NewSecurePassword123!",
				},
				Output: map[string]interface{}{
					"status": "password_reset",
					"note":   "Password has been changed. All existing sessions have been invalidated.",
				},
			},
		},
		RequiresAuth: false,
		RelatedTools: []string{"ts.auth.forgot_password", "ts.auth.login"},
		Since:        "v0.3.0",
	})

	r.Register(&ToolDoc{
		Name:     "ts.auth.update_profile",
		Category: "auth",
		Summary:  "Update your own profile fields.",
		Description: `Allows authenticated users to update their own profile. Only modifies
the caller's own user record -- cannot update other users. Use ts.usr.update
for admin-level user modifications.`,
		Parameters: []ParamDoc{
			{Name: "name", Type: "string", Description: "New display name", Required: false, Example: "Claude v2"},
			{Name: "lang", Type: "string", Description: "Language code (e.g., 'en', 'de', 'fr')", Required: false, Example: "en"},
			{Name: "timezone", Type: "string", Description: "IANA timezone (e.g., 'Europe/Berlin')", Required: false, Example: "Europe/Luxembourg"},
			{Name: "email_copy", Type: "boolean", Description: "Enable/disable email copies of messages", Required: false},
		},
		RequiredParams: []string{},
		Returns: ReturnDoc{
			Description: "Returns the updated user profile.",
			Example: map[string]interface{}{
				"user_id":  "usr_01H8X9...",
				"name":     "Claude v2",
				"lang":     "en",
				"timezone": "Europe/Luxembourg",
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "invalid_input", Description: "At least one field must be provided"},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.auth.whoami", "ts.usr.update"},
		Since:        "v0.3.7",
	})
}

func registerTokenTools(r *Registry) {
	r.Register(&ToolDoc{
		Name:     "ts.tkn.verify",
		Category: "auth",
		Summary:  "Verify a token and return associated user info.",
		Description: `Validates an access token and returns information about the authenticated user.

Use this to:
- Check if a token is still valid
- Get user information for the token holder
- Verify token permissions before performing actions`,
		Parameters: []ParamDoc{
			{
				Name:        "token",
				Type:        "string",
				Description: "The access token to verify",
				Required:    true,
				Example:     "ts_abc123def456...",
			},
		},
		RequiredParams: []string{"token"},
		Returns: ReturnDoc{
			Description: "Returns user information if the token is valid.",
			Schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"valid":      map[string]interface{}{"type": "boolean"},
					"user_id":    map[string]interface{}{"type": "string"},
					"email":      map[string]interface{}{"type": "string"},
					"name":       map[string]interface{}{"type": "string"},
					"expires_at": map[string]interface{}{"type": "string", "format": "date-time"},
				},
			},
			Example: map[string]interface{}{
				"valid":      true,
				"user_id":    "usr_01H8X9ABCDEF",
				"email":      "agent@example.com",
				"name":       "Agent Smith",
				"expires_at": "2026-02-05T10:30:00Z",
			},
		},
		Errors: []ErrorDoc{
			{Code: "invalid_input", Description: "Token parameter is required"},
		},
		Examples: []ExampleDoc{
			{
				Title:       "Verify a token",
				Description: "Check if a token is valid and get user info.",
				Input: map[string]interface{}{
					"token": "ts_abc123def456...",
				},
				Output: map[string]interface{}{
					"valid":      true,
					"user_id":    "usr_01H8X9ABCDEF",
					"email":      "agent@example.com",
					"name":       "Agent Smith",
					"expires_at": "2026-02-05T10:30:00Z",
				},
			},
		},
		RequiresAuth: false,
		RelatedTools: []string{"ts.auth.login", "ts.tkn.create"},
		Since:        "v0.1.0",
		Visibility:   "internal",
	})

	r.Register(&ToolDoc{
		Name:     "ts.tkn.create",
		Category: "auth",
		Summary:  "Create an API access token for a user.",
		Description: `Creates a new API access token for authenticated access.

Unlike session tokens from ts.auth.login, API tokens:
- Can have custom expiration times (or never expire)
- Have a display name for identification
- Are suitable for automation and CI/CD

Only authenticated users can create tokens. By default, tokens are created
for the authenticated user. Admins can create tokens for other users.`,
		Parameters: []ParamDoc{
			{
				Name:        "user_id",
				Type:        "string",
				Description: "User ID to create token for (defaults to authenticated user)",
				Required:    false,
				Example:     "usr_01H8X9ABCDEF",
			},
			{
				Name:        "name",
				Type:        "string",
				Description: "Display name for the token (e.g., 'CLI', 'CI/CD')",
				Required:    false,
				Default:     "API Token",
				Example:     "My Agent Token",
			},
			{
				Name:        "expires_at",
				Type:        "string",
				Description: "ISO 8601 expiration datetime (null = never expires)",
				Required:    false,
				Example:     "2026-12-31T23:59:59Z",
			},
		},
		RequiredParams: []string{},
		Returns: ReturnDoc{
			Description: "Returns the new token. Store it securely - it cannot be retrieved again.",
			Schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"token":      map[string]interface{}{"type": "string"},
					"token_id":   map[string]interface{}{"type": "string"},
					"name":       map[string]interface{}{"type": "string"},
					"expires_at": map[string]interface{}{"type": "string", "format": "date-time"},
				},
			},
			Example: map[string]interface{}{
				"token":      "ts_xyz789ghi012...",
				"token_id":   "tkn_01H8X9GHIJKL",
				"name":       "My Agent Token",
				"expires_at": "2026-12-31T23:59:59Z",
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "unauthorized", Description: "Cannot create tokens for other users"},
		},
		Examples: []ExampleDoc{
			{
				Title:       "Create a non-expiring token",
				Description: "Create an API token that never expires.",
				Input: map[string]interface{}{
					"name": "CI/CD Pipeline",
				},
				Output: map[string]interface{}{
					"token":      "ts_xyz789ghi012...",
					"token_id":   "tkn_01H8X9GHIJKL",
					"name":       "CI/CD Pipeline",
					"expires_at": nil,
				},
			},
			{
				Title:       "Create a time-limited token",
				Description: "Create a token that expires at a specific time.",
				Input: map[string]interface{}{
					"name":       "Temporary Access",
					"expires_at": "2026-03-01T00:00:00Z",
				},
				Output: map[string]interface{}{
					"token":      "ts_xyz789ghi012...",
					"token_id":   "tkn_01H8X9MNOPQ",
					"name":       "Temporary Access",
					"expires_at": "2026-03-01T00:00:00Z",
				},
			},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.auth.login", "ts.tkn.verify"},
		Since:        "v0.1.0",
		Visibility:   "internal",
	})
}

func registerUserTools(r *Registry) {
	r.Register(&ToolDoc{
		Name:     "ts.usr.create",
		Category: "user",
		Summary:  "Create a new user account.",
		Description: `Creates a new user in the system. There are two paths for user creation:

**Admin creation**: An authenticated admin can directly create users for any organization.

**Self-registration**: A user can register themselves using an organization's registration token.
This requires:
- organization_id: The organization to join
- org_token: A valid registration token for that organization
- password: The user's chosen password

Self-registration is useful for onboarding new team members or agents.`,
		Parameters: []ParamDoc{
			{
				Name:        "name",
				Type:        "string",
				Description: "User's display name",
				Required:    true,
				Example:     "Agent Smith",
			},
			{
				Name:        "email",
				Type:        "string",
				Description: "User's email address (must be unique)",
				Required:    true,
				Example:     "smith@example.com",
			},
			{
				Name:        "password",
				Type:        "string",
				Description: "User's password (required for self-registration)",
				Required:    false,
				Example:     "SecurePassword123!",
			},
			{
				Name:        "organization_id",
				Type:        "string",
				Description: "Organization to add user to (required for self-registration)",
				Required:    false,
				Example:     "org_01H8X9ABCDEF",
			},
			{
				Name:        "org_token",
				Type:        "string",
				Description: "Organization registration token (for self-registration)",
				Required:    false,
				Example:     "ort_xyz789...",
			},
			{
				Name:        "resource_id",
				Type:        "string",
				Description: "Link to existing resource (optional)",
				Required:    false,
			},
			{
				Name:        "external_id",
				Type:        "string",
				Description: "External system ID for SSO integration (optional)",
				Required:    false,
			},
			{
				Name:        "metadata",
				Type:        "object",
				Description: "Arbitrary key-value pairs (optional)",
				Required:    false,
				Example:     map[string]interface{}{"department": "engineering"},
			},
		},
		RequiredParams: []string{"name", "email"},
		Returns: ReturnDoc{
			Description: "Returns the created user object.",
			Schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id":         map[string]interface{}{"type": "string"},
					"name":       map[string]interface{}{"type": "string"},
					"email":      map[string]interface{}{"type": "string"},
					"status":     map[string]interface{}{"type": "string"},
					"created_at": map[string]interface{}{"type": "string", "format": "date-time"},
				},
			},
			Example: map[string]interface{}{
				"id":         "usr_01H8X9GHIJKL",
				"name":       "Agent Smith",
				"email":      "smith@example.com",
				"status":     "active",
				"created_at": "2026-02-04T15:30:00Z",
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No valid authentication and no org_token provided"},
			{Code: "invalid_token", Description: "Organization token is invalid or expired"},
			{Code: "email_exists", Description: "A user with this email already exists"},
			{Code: "invalid_email", Description: "Email format is invalid"},
			{Code: "weak_password", Description: "Password does not meet requirements"},
		},
		Examples: []ExampleDoc{
			{
				Title:       "Self-registration with org token",
				Description: "Register a new user using an organization's registration token.",
				Input: map[string]interface{}{
					"name":            "Agent Smith",
					"email":           "smith@example.com",
					"password":        "SecurePassword123!",
					"organization_id": "org_01H8X9ABCDEF",
					"org_token":       "ort_xyz789...",
				},
				Output: map[string]interface{}{
					"id":         "usr_01H8X9GHIJKL",
					"name":       "Agent Smith",
					"email":      "smith@example.com",
					"status":     "active",
					"created_at": "2026-02-04T15:30:00Z",
				},
			},
			{
				Title:       "Admin creates user",
				Description: "An authenticated admin creates a user directly.",
				Input: map[string]interface{}{
					"name":  "New Agent",
					"email": "newagent@example.com",
					"metadata": map[string]interface{}{
						"role": "analyst",
					},
				},
				Output: map[string]interface{}{
					"id":         "usr_01H8X9MNOPQ",
					"name":       "New Agent",
					"email":      "newagent@example.com",
					"status":     "active",
					"created_at": "2026-02-04T15:30:00Z",
				},
			},
		},
		RequiresAuth: false, // Can use org_token instead
		RelatedTools: []string{"ts.usr.get", "ts.usr.list"},
		Since:        "v0.1.0",
	})

	r.Register(&ToolDoc{
		Name:     "ts.usr.get",
		Category: "user",
		Summary:  "Retrieve a user by ID.",
		Description: `Retrieves detailed information about a specific user.

Requires authentication. Users can always retrieve their own information.
Admins can retrieve any user's information.`,
		Parameters: []ParamDoc{
			{
				Name:        "id",
				Type:        "string",
				Description: "User ID to retrieve",
				Required:    true,
				Example:     "usr_01H8X9ABCDEF",
			},
		},
		RequiredParams: []string{"id"},
		Returns: ReturnDoc{
			Description: "Returns the user object if found.",
			Schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id":          map[string]interface{}{"type": "string"},
					"name":        map[string]interface{}{"type": "string"},
					"email":       map[string]interface{}{"type": "string"},
					"status":      map[string]interface{}{"type": "string"},
					"resource_id": map[string]interface{}{"type": "string"},
					"external_id": map[string]interface{}{"type": "string"},
					"metadata":    map[string]interface{}{"type": "object"},
					"created_at":  map[string]interface{}{"type": "string", "format": "date-time"},
					"updated_at":  map[string]interface{}{"type": "string", "format": "date-time"},
				},
			},
			Example: map[string]interface{}{
				"id":         "usr_01H8X9ABCDEF",
				"name":       "Agent Smith",
				"email":      "smith@example.com",
				"status":     "active",
				"metadata":   map[string]interface{}{"department": "engineering"},
				"created_at": "2026-02-04T15:30:00Z",
				"updated_at": "2026-02-04T15:30:00Z",
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "not_found", Description: "User with this ID does not exist"},
			{Code: "unauthorized", Description: "Not allowed to view this user"},
		},
		Examples: []ExampleDoc{
			{
				Title: "Get user by ID",
				Input: map[string]interface{}{
					"id": "usr_01H8X9ABCDEF",
				},
				Output: map[string]interface{}{
					"id":         "usr_01H8X9ABCDEF",
					"name":       "Agent Smith",
					"email":      "smith@example.com",
					"status":     "active",
					"created_at": "2026-02-04T15:30:00Z",
					"updated_at": "2026-02-04T15:30:00Z",
				},
			},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.usr.create", "ts.usr.list"},
		Since:        "v0.1.0",
	})

	r.Register(&ToolDoc{
		Name:     "ts.usr.list",
		Category: "user",
		Summary:  "Query users with filters.",
		Description: `Lists users with optional filtering and pagination.

Requires authentication. Results are filtered based on the caller's permissions:
- Regular users see users in their organizations
- Admins can see all users`,
		Parameters: []ParamDoc{
			{
				Name:        "status",
				Type:        "string",
				Description: "Filter by status: active, inactive, suspended",
				Required:    false,
				Example:     "active",
			},
			{
				Name:        "organization_id",
				Type:        "string",
				Description: "Filter by organization membership",
				Required:    false,
				Example:     "org_01H8X9ABCDEF",
			},
			{
				Name:        "search",
				Type:        "string",
				Description: "Search by name or email (partial match)",
				Required:    false,
				Example:     "smith",
			},
			{
				Name:        "limit",
				Type:        "integer",
				Description: "Maximum number of results to return",
				Required:    false,
				Default:     50,
				Example:     20,
			},
			{
				Name:        "offset",
				Type:        "integer",
				Description: "Number of results to skip (for pagination)",
				Required:    false,
				Default:     0,
				Example:     20,
			},
		},
		RequiredParams: []string{},
		Returns: ReturnDoc{
			Description: "Returns a list of users matching the filters.",
			Schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"users": map[string]interface{}{
						"type":  "array",
						"items": map[string]interface{}{"$ref": "#/definitions/User"},
					},
					"total":  map[string]interface{}{"type": "integer"},
					"limit":  map[string]interface{}{"type": "integer"},
					"offset": map[string]interface{}{"type": "integer"},
				},
			},
			Example: map[string]interface{}{
				"users": []map[string]interface{}{
					{
						"id":     "usr_01H8X9ABCDEF",
						"name":   "Agent Smith",
						"email":  "smith@example.com",
						"status": "active",
					},
					{
						"id":     "usr_01H8X9GHIJKL",
						"name":   "Agent Jones",
						"email":  "jones@example.com",
						"status": "active",
					},
				},
				"total":  42,
				"limit":  50,
				"offset": 0,
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
		},
		Examples: []ExampleDoc{
			{
				Title:       "List active users",
				Description: "Get all active users.",
				Input: map[string]interface{}{
					"status": "active",
				},
				Output: map[string]interface{}{
					"users": []map[string]interface{}{
						{
							"id":     "usr_01H8X9ABCDEF",
							"name":   "Agent Smith",
							"email":  "smith@example.com",
							"status": "active",
						},
					},
					"total":  1,
					"limit":  50,
					"offset": 0,
				},
			},
			{
				Title:       "Search users",
				Description: "Search for users by name or email.",
				Input: map[string]interface{}{
					"search": "smith",
					"limit":  10,
				},
				Output: map[string]interface{}{
					"users": []map[string]interface{}{
						{
							"id":     "usr_01H8X9ABCDEF",
							"name":   "Agent Smith",
							"email":  "smith@example.com",
							"status": "active",
						},
					},
					"total":  1,
					"limit":  10,
					"offset": 0,
				},
			},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.usr.create", "ts.usr.get"},
		Since:        "v0.1.0",
	})

	r.Register(&ToolDoc{
		Name:     "ts.usr.update",
		Category: "user",
		Summary:  "Update user attributes (admin only).",
		Description: `Updates a user's profile fields. Requires admin privileges.
For self-service profile updates, use ts.auth.update_profile instead.`,
		Parameters: []ParamDoc{
			{Name: "id", Type: "string", Description: "User ID", Required: true, Example: "usr_476931df38eb2662..."},
			{Name: "name", Type: "string", Description: "New name", Required: false, Example: "Updated Agent"},
			{Name: "email", Type: "string", Description: "New email", Required: false, Example: "updated@example.com"},
			{Name: "status", Type: "string", Description: "New status: active, inactive, suspended", Required: false, Example: "active"},
			{Name: "lang", Type: "string", Description: "Language code", Required: false, Example: "de"},
			{Name: "timezone", Type: "string", Description: "IANA timezone", Required: false, Example: "Europe/Berlin"},
			{Name: "email_copy", Type: "boolean", Description: "Enable/disable email copies", Required: false},
			{Name: "metadata", Type: "object", Description: "Custom key-value pairs", Required: false},
		},
		RequiredParams: []string{"id"},
		Returns: ReturnDoc{
			Description: "Returns the updated user.",
			Example: map[string]interface{}{
				"id":     "usr_476931df38eb2662...",
				"name":   "Updated Agent",
				"status": "active",
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "not_found", Description: "User not found"},
			{Code: "forbidden", Description: "Admin privileges required"},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.usr.get", "ts.usr.list", "ts.auth.update_profile"},
		Since:        "v0.3.7",
	})
}

func registerInvitationTools(r *Registry) {
	r.Register(&ToolDoc{
		Name:     "ts.inv.create",
		Category: "invitation",
		Summary:  "Create an invitation token for agent self-registration.",
		Description: `Creates an invitation token that allows an agent to register itself.

Only master admins can create invitation tokens. Tokens can have:
- An expiration date
- A maximum number of uses
- An optional name for identification

Share the token with the agent through a secure channel. The agent
will use it with ts.reg.register to create its account.`,
		Parameters: []ParamDoc{
			{
				Name:        "name",
				Type:        "string",
				Description: "Display name for the token (for admin reference)",
				Required:    false,
				Example:     "Research Agent Token",
			},
			{
				Name:        "expires_at",
				Type:        "string",
				Description: "ISO 8601 expiration datetime",
				Required:    false,
				Example:     "2026-02-11T00:00:00Z",
			},
			{
				Name:        "max_uses",
				Type:        "integer",
				Description: "Maximum number of registrations allowed (null = unlimited)",
				Required:    false,
				Default:     1,
				Example:     1,
			},
		},
		RequiredParams: []string{},
		Returns: ReturnDoc{
			Description: "Returns the created invitation token.",
			Schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"token":      map[string]interface{}{"type": "string"},
					"name":       map[string]interface{}{"type": "string"},
					"expires_at": map[string]interface{}{"type": "string", "format": "date-time"},
					"max_uses":   map[string]interface{}{"type": "integer"},
					"uses":       map[string]interface{}{"type": "integer"},
					"created_at": map[string]interface{}{"type": "string", "format": "date-time"},
				},
			},
			Example: map[string]interface{}{
				"token":      "inv_abc123xyz789...",
				"name":       "Research Agent Token",
				"expires_at": "2026-02-11T00:00:00Z",
				"max_uses":   1,
				"uses":       0,
				"created_at": "2026-02-04T19:00:00Z",
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "no_admin", Description: "Only master admins can create invitation tokens"},
		},
		Examples: []ExampleDoc{
			{
				Title:       "Create single-use token",
				Description: "Create a token that can be used once and expires in 7 days.",
				Input: map[string]interface{}{
					"name":       "Agent Alpha Token",
					"expires_at": "2026-02-11T00:00:00Z",
					"max_uses":   1,
				},
				Output: map[string]interface{}{
					"token":      "inv_abc123xyz789...",
					"name":       "Agent Alpha Token",
					"expires_at": "2026-02-11T00:00:00Z",
					"max_uses":   1,
					"uses":       0,
					"created_at": "2026-02-04T19:00:00Z",
				},
			},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.inv.list", "ts.inv.revoke", "ts.reg.register"},
		Since:        "v0.2.0",
		Visibility:   "internal",
	})

	r.Register(&ToolDoc{
		Name:     "ts.inv.list",
		Category: "invitation",
		Summary:  "List invitation tokens.",
		Description: `Lists all invitation tokens created by the admin.

Returns both active and expired tokens. Use the status filter to
see only active tokens.`,
		Parameters: []ParamDoc{
			{
				Name:        "status",
				Type:        "string",
				Description: "Filter by status: active, expired, exhausted",
				Required:    false,
				Example:     "active",
			},
		},
		RequiredParams: []string{},
		Returns: ReturnDoc{
			Description: "Returns a list of invitation tokens.",
			Example: map[string]interface{}{
				"tokens": []map[string]interface{}{
					{
						"token":      "inv_abc123...",
						"name":       "Agent Alpha Token",
						"status":     "active",
						"expires_at": "2026-02-11T00:00:00Z",
						"max_uses":   1,
						"uses":       0,
					},
				},
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "no_admin", Description: "Only master admins can list invitation tokens"},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.inv.create", "ts.inv.revoke"},
		Since:        "v0.2.0",
		Visibility:   "internal",
	})

	r.Register(&ToolDoc{
		Name:     "ts.inv.revoke",
		Category: "invitation",
		Summary:  "Revoke an invitation token.",
		Description: `Revokes an invitation token so it can no longer be used.

Use this to invalidate a token that was compromised or is no longer needed.`,
		Parameters: []ParamDoc{
			{
				Name:        "token",
				Type:        "string",
				Description: "The invitation token to revoke",
				Required:    true,
				Example:     "inv_abc123xyz789...",
			},
		},
		RequiredParams: []string{"token"},
		Returns: ReturnDoc{
			Description: "Returns success confirmation.",
			Example: map[string]interface{}{
				"revoked": true,
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "no_admin", Description: "Only master admins can revoke tokens"},
			{Code: "not_found", Description: "Token does not exist"},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.inv.create", "ts.inv.list"},
		Since:        "v0.2.0",
		Visibility:   "internal",
	})
}

func registerRegistrationTools(r *Registry) {
	r.Register(&ToolDoc{
		Name:     "ts.reg.register",
		Category: "registration",
		Summary:  "Register a new agent account using an invitation token.",
		Description: `Registers a new agent account using an invitation token.

This is the self-registration endpoint for agents. MCP registration
always creates agent accounts. Human accounts are created through
the web UI. After calling this, the system sends a verification email.
The agent must then call ts.reg.verify with the code from the email
to complete registration.

Password requirements:
- Minimum 12 characters
- At least one uppercase letter
- At least one lowercase letter
- At least one digit
- At least one special character`,
		Parameters: []ParamDoc{
			{
				Name:        "invitation_token",
				Type:        "string",
				Description: "The invitation token from an admin",
				Required:    true,
				Example:     "inv_abc123xyz789...",
			},
			{
				Name:        "email",
				Type:        "string",
				Description: "Email address for the new account",
				Required:    true,
				Example:     "agent@example.com",
			},
			{
				Name:        "name",
				Type:        "string",
				Description: "Display name for the account",
				Required:    true,
				Example:     "Research Agent",
			},
			{
				Name:        "password",
				Type:        "string",
				Description: "Password for the account (must meet requirements)",
				Required:    true,
				Example:     "SecurePassword123!",
			},
		},
		RequiredParams: []string{"invitation_token", "email", "name", "password"},
		Returns: ReturnDoc{
			Description: "Returns registration status. A verification email is sent.",
			Example: map[string]interface{}{
				"status":            "pending_verification",
				"email":             "agent@example.com",
				"verification_sent": true,
				"expires_in":        "15m",
			},
		},
		Errors: []ErrorDoc{
			{Code: "invalid_token", Description: "Invitation token is invalid, expired, or exhausted"},
			{Code: "email_exists", Description: "An account with this email already exists"},
			{Code: "invalid_email", Description: "Email format is invalid"},
			{Code: "weak_password", Description: "Password does not meet requirements"},
		},
		Examples: []ExampleDoc{
			{
				Title:       "Register with invitation token",
				Description: "Agent registers itself using the token provided by admin.",
				Input: map[string]interface{}{
					"invitation_token": "inv_abc123xyz789...",
					"email":            "agent@example.com",
					"name":             "Research Agent",
					"password":         "SecurePassword123!",
				},
				Output: map[string]interface{}{
					"status":            "pending_verification",
					"email":             "agent@example.com",
					"verification_sent": true,
					"expires_in":        "15m",
				},
			},
		},
		RequiresAuth: false,
		RelatedTools: []string{"ts.reg.verify", "ts.reg.resend", "ts.inv.create"},
		Since:        "v0.2.0",
	})

	r.Register(&ToolDoc{
		Name:     "ts.reg.verify",
		Category: "registration",
		Summary:  "Verify email with the code from the verification email.",
		Description: `Completes registration by verifying the email address.

After calling ts.reg.register, the agent receives an email with a
verification code in the format xxx-xxx-xxx (lowercase alphanumeric).
Submit that code here to complete registration.

On success, returns an auth token that can be used immediately.`,
		Parameters: []ParamDoc{
			{
				Name:        "email",
				Type:        "string",
				Description: "Email address being verified",
				Required:    true,
				Example:     "agent@example.com",
			},
			{
				Name:        "code",
				Type:        "string",
				Description: "Verification code from the email (format: xxx-xxx-xxx)",
				Required:    true,
				Example:     "abc-def-ghi",
			},
		},
		RequiredParams: []string{"email", "code"},
		Returns: ReturnDoc{
			Description: "Returns auth token on successful verification.",
			Example: map[string]interface{}{
				"status":  "verified",
				"token":   "ts_xyz789abc...",
				"user_id": "usr_01H8X9ABCDEF",
				"name":    "Research Agent",
				"email":   "agent@example.com",
			},
		},
		Errors: []ErrorDoc{
			{Code: "invalid_code", Description: "Verification code is incorrect"},
			{Code: "code_expired", Description: "Verification code has expired"},
			{Code: "not_found", Description: "No pending verification for this email"},
		},
		Examples: []ExampleDoc{
			{
				Title:       "Verify email",
				Description: "Submit the verification code received via email.",
				Input: map[string]interface{}{
					"email": "agent@example.com",
					"code":  "abc-def-ghi",
				},
				Output: map[string]interface{}{
					"status":  "verified",
					"token":   "ts_xyz789abc...",
					"user_id": "usr_01H8X9ABCDEF",
					"name":    "Research Agent",
					"email":   "agent@example.com",
				},
			},
		},
		RequiresAuth: false,
		RelatedTools: []string{"ts.reg.register", "ts.reg.resend"},
		Since:        "v0.2.0",
	})

	r.Register(&ToolDoc{
		Name:     "ts.reg.resend",
		Category: "registration",
		Summary:  "Resend the verification email.",
		Description: `Requests a new verification code if the original expired or was lost.

The previous code is invalidated and a new one is sent. The new code
expires in 15 minutes.`,
		Parameters: []ParamDoc{
			{
				Name:        "email",
				Type:        "string",
				Description: "Email address to resend verification to",
				Required:    true,
				Example:     "agent@example.com",
			},
		},
		RequiredParams: []string{"email"},
		Returns: ReturnDoc{
			Description: "Returns confirmation that a new code was sent.",
			Example: map[string]interface{}{
				"sent":       true,
				"expires_in": "15m",
			},
		},
		Errors: []ErrorDoc{
			{Code: "missing_email", Description: "Email is required"},
			{Code: "not_found", Description: "No pending verification for this email"},
		},
		Examples: []ExampleDoc{
			{
				Title:       "Resend verification code",
				Description: "Request a new code if the original expired.",
				Input: map[string]interface{}{
					"email": "agent@example.com",
				},
				Output: map[string]interface{}{
					"sent":       true,
					"expires_in": "15m",
				},
			},
		},
		RequiresAuth: false,
		RelatedTools: []string{"ts.reg.register", "ts.reg.verify"},
		Since:        "v0.2.0",
	})
}

func registerOrganizationTools(r *Registry) {
	r.Register(&ToolDoc{
		Name:     "ts.org.create",
		Category: "organization",
		Summary:  "Create a new organization.",
		Description: `Creates a new organization in Taskschmiede.

Organizations are the top-level grouping for resources and endeavours.
They represent teams, companies, or any group that works together.`,
		Parameters: []ParamDoc{
			{
				Name:        "name",
				Type:        "string",
				Description: "Organization name",
				Required:    true,
				Example:     "Quest Financial Technologies",
			},
			{
				Name:        "description",
				Type:        "string",
				Description: "Organization description",
				Required:    false,
				Example:     "Software and consulting for financial technology",
			},
			{
				Name:        "metadata",
				Type:        "object",
				Description: "Arbitrary key-value pairs",
				Required:    false,
				Example:     map[string]interface{}{"industry": "fintech"},
			},
		},
		RequiredParams: []string{"name"},
		Returns: ReturnDoc{
			Description: "Returns the created organization summary.",
			Schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id":         map[string]interface{}{"type": "string"},
					"name":       map[string]interface{}{"type": "string"},
					"status":     map[string]interface{}{"type": "string"},
					"created_at": map[string]interface{}{"type": "string", "format": "date-time"},
				},
			},
			Example: map[string]interface{}{
				"id":         "org_1d9cb149497656c7...",
				"name":       "Quest Financial Technologies",
				"status":     "active",
				"created_at": "2026-02-06T13:36:24Z",
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "invalid_input", Description: "Name is required"},
		},
		Examples: []ExampleDoc{
			{
				Title:       "Create an organization",
				Description: "Create a new organization for a team.",
				Input: map[string]interface{}{
					"name":        "Quest Financial Technologies",
					"description": "Software and consulting for financial technology",
				},
				Output: map[string]interface{}{
					"id":         "org_1d9cb149497656c7...",
					"name":       "Quest Financial Technologies",
					"status":     "active",
					"created_at": "2026-02-06T13:36:24Z",
				},
			},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.org.get", "ts.org.list", "ts.org.add_resource", "ts.org.add_endeavour"},
		Since:        "v0.2.0",
	})

	r.Register(&ToolDoc{
		Name:     "ts.org.get",
		Category: "organization",
		Summary:  "Retrieve an organization by ID.",
		Description: `Retrieves detailed information about a specific organization,
including member count and endeavour count.`,
		Parameters: []ParamDoc{
			{
				Name:        "id",
				Type:        "string",
				Description: "Organization ID",
				Required:    true,
				Example:     "org_1d9cb149497656c7...",
			},
		},
		RequiredParams: []string{"id"},
		Returns: ReturnDoc{
			Description: "Returns the organization with member and endeavour counts.",
			Schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id":              map[string]interface{}{"type": "string"},
					"name":            map[string]interface{}{"type": "string"},
					"description":     map[string]interface{}{"type": "string"},
					"status":          map[string]interface{}{"type": "string"},
					"metadata":        map[string]interface{}{"type": "object"},
					"member_count":    map[string]interface{}{"type": "integer"},
					"endeavour_count": map[string]interface{}{"type": "integer"},
					"created_at":      map[string]interface{}{"type": "string", "format": "date-time"},
					"updated_at":      map[string]interface{}{"type": "string", "format": "date-time"},
				},
			},
			Example: map[string]interface{}{
				"id":              "org_1d9cb149497656c7...",
				"name":            "Quest Financial Technologies",
				"description":     "Software and consulting for financial technology",
				"status":          "active",
				"metadata":        map[string]interface{}{},
				"member_count":    3,
				"endeavour_count": 1,
				"created_at":      "2026-02-06T13:36:24Z",
				"updated_at":      "2026-02-06T13:36:24Z",
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "not_found", Description: "Organization with this ID does not exist"},
		},
		Examples: []ExampleDoc{
			{
				Title: "Get organization by ID",
				Input: map[string]interface{}{
					"id": "org_1d9cb149497656c7...",
				},
				Output: map[string]interface{}{
					"id":              "org_1d9cb149497656c7...",
					"name":            "Quest Financial Technologies",
					"member_count":    3,
					"endeavour_count": 1,
					"status":          "active",
					"created_at":      "2026-02-06T13:36:24Z",
					"updated_at":      "2026-02-06T13:36:24Z",
				},
			},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.org.create", "ts.org.list"},
		Since:        "v0.2.0",
	})

	r.Register(&ToolDoc{
		Name:     "ts.org.list",
		Category: "organization",
		Summary:  "Query organizations with filters.",
		Description: `Lists organizations with optional filtering and pagination.

Results include basic organization info. Use ts.org.get for full details
including member and endeavour counts.`,
		Parameters: []ParamDoc{
			{
				Name:        "status",
				Type:        "string",
				Description: "Filter by status: active, inactive, archived",
				Required:    false,
				Example:     "active",
			},
			{
				Name:        "search",
				Type:        "string",
				Description: "Search by name (partial match)",
				Required:    false,
				Example:     "Quest",
			},
			{
				Name:        "limit",
				Type:        "integer",
				Description: "Maximum number of results to return",
				Required:    false,
				Default:     50,
				Example:     20,
			},
			{
				Name:        "offset",
				Type:        "integer",
				Description: "Number of results to skip (for pagination)",
				Required:    false,
				Default:     0,
			},
		},
		RequiredParams: []string{},
		Returns: ReturnDoc{
			Description: "Returns a paginated list of organizations.",
			Example: map[string]interface{}{
				"organizations": []map[string]interface{}{
					{
						"id":          "org_1d9cb149497656c7...",
						"name":        "Quest Financial Technologies",
						"description": "Software and consulting for financial technology",
						"status":      "active",
						"created_at":  "2026-02-06T13:36:24Z",
						"updated_at":  "2026-02-06T13:36:24Z",
					},
				},
				"total":  1,
				"limit":  50,
				"offset": 0,
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.org.create", "ts.org.get"},
		Since:        "v0.2.0",
	})

	r.Register(&ToolDoc{
		Name:     "ts.org.update",
		Category: "organization",
		Summary:  "Update organization attributes (partial update).",
		Description: `Updates an organization's name, description, status, or metadata.
Only provided fields are changed.`,
		Parameters: []ParamDoc{
			{
				Name:        "id",
				Type:        "string",
				Description: "Organization ID",
				Required:    true,
				Example:     "org_1d9cb149497656c7",
			},
			{
				Name:        "name",
				Type:        "string",
				Description: "New name",
				Required:    false,
			},
			{
				Name:        "description",
				Type:        "string",
				Description: "New description",
				Required:    false,
			},
			{
				Name:        "status",
				Type:        "string",
				Description: "New status: active, inactive, archived",
				Required:    false,
				Example:     "active",
			},
			{
				Name:        "metadata",
				Type:        "object",
				Description: "Metadata to set (replaces existing)",
				Required:    false,
			},
		},
		RequiredParams: []string{"id"},
		Returns: ReturnDoc{
			Description: "Returns the updated organization.",
			Example: map[string]interface{}{
				"id":         "org_1d9cb149497656c7",
				"name":       "Updated Org Name",
				"status":     "active",
				"updated_at": "2026-02-12T10:00:00Z",
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "not_found", Description: "Organization with this ID does not exist"},
			{Code: "invalid_input", Description: "No fields to update"},
		},
		Examples: []ExampleDoc{
			{
				Title:       "Update organization name",
				Description: "Change the name of an organization.",
				Input: map[string]interface{}{
					"id":   "org_1d9cb149497656c7",
					"name": "Updated Org Name",
				},
				Output: map[string]interface{}{
					"id":         "org_1d9cb149497656c7",
					"name":       "Updated Org Name",
					"updated_at": "2026-02-12T10:00:00Z",
				},
			},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.org.get", "ts.org.list"},
		Since:        "v0.3.0",
	})

	r.Register(&ToolDoc{
		Name:     "ts.org.archive",
		Category: "organization",
		Summary:  "Archive an organization with cascade.",
		Description: `Archives an organization and cascades to all associated endeavours and tasks.

Use confirm=false (default) for a dry-run that shows what would be archived.
Use confirm=true to execute the archive operation.`,
		Parameters: []ParamDoc{
			{Name: "id", Type: "string", Description: "Organization ID", Required: true, Example: "org_1d9cb149497656c7..."},
			{Name: "reason", Type: "string", Description: "Reason for archiving", Required: true, Example: "Project completed"},
			{Name: "confirm", Type: "boolean", Description: "Execute the archive (false = dry-run)", Required: false, Default: false},
		},
		RequiredParams: []string{"id", "reason"},
		Returns: ReturnDoc{
			Description: "Returns the archive result or dry-run summary.",
			Example: map[string]interface{}{
				"id":       "org_1d9cb149497656c7...",
				"archived": true,
				"reason":   "Project completed",
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "not_found", Description: "Organization not found"},
			{Code: "forbidden", Description: "Admin privileges required"},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.org.get", "ts.org.update"},
		Since:        "v0.3.7",
	})

	r.Register(&ToolDoc{
		Name:     "ts.org.export",
		Category: "organization",
		Summary:  "Export all organization data as JSON.",
		Description: `Exports the complete organization data including members, endeavours,
tasks, and configuration as a JSON document.`,
		Parameters: []ParamDoc{
			{Name: "organization_id", Type: "string", Description: "Organization ID", Required: true, Example: "org_1d9cb149497656c7..."},
		},
		RequiredParams: []string{"organization_id"},
		Returns: ReturnDoc{
			Description: "Returns the full organization data as JSON.",
			Example: map[string]interface{}{
				"organization": map[string]interface{}{"id": "org_1d9cb149497656c7...", "name": "Quest Financial Technologies"},
				"members":      []map[string]interface{}{},
				"endeavours":   []map[string]interface{}{},
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "not_found", Description: "Organization not found"},
			{Code: "forbidden", Description: "Admin privileges required"},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.org.get"},
		Since:        "v0.3.7",
	})

	r.Register(&ToolDoc{
		Name:     "ts.org.add_member",
		Category: "organization",
		Summary:  "Add a user to an organization.",
		Description: `Adds a user to an organization with a specified role. Resolves user ID
to resource ID internally. Use ts.org.add_resource for non-user resources.`,
		Parameters: []ParamDoc{
			{Name: "org_id", Type: "string", Description: "Organization ID", Required: true, Example: "org_1d9cb149497656c7..."},
			{Name: "user_id", Type: "string", Description: "User ID to add", Required: true, Example: "usr_476931df38eb2662..."},
			{Name: "role", Type: "string", Description: "Role: owner, admin, member, guest", Required: false, Default: "member", Example: "member"},
		},
		RequiredParams: []string{"org_id", "user_id"},
		Returns: ReturnDoc{
			Description: "Returns the membership confirmation.",
			Example: map[string]interface{}{
				"org_id":    "org_1d9cb149497656c7...",
				"user_id":   "usr_476931df38eb2662...",
				"role":      "member",
				"joined_at": "2026-02-06T13:36:43Z",
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "not_found", Description: "Organization or user not found"},
			{Code: "already_member", Description: "User is already a member of this organization"},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.org.remove_member", "ts.org.list_members", "ts.org.add_resource"},
		Since:        "v0.3.7",
	})

	r.Register(&ToolDoc{
		Name:     "ts.org.remove_member",
		Category: "organization",
		Summary:  "Remove a user from an organization.",
		Description: `Removes a user's membership from an organization.`,
		Parameters: []ParamDoc{
			{Name: "org_id", Type: "string", Description: "Organization ID", Required: true, Example: "org_1d9cb149497656c7..."},
			{Name: "user_id", Type: "string", Description: "User ID to remove", Required: true, Example: "usr_476931df38eb2662..."},
		},
		RequiredParams: []string{"org_id", "user_id"},
		Returns: ReturnDoc{
			Description: "Returns confirmation of removal.",
			Example: map[string]interface{}{
				"org_id":  "org_1d9cb149497656c7...",
				"user_id": "usr_476931df38eb2662...",
				"removed": true,
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "not_found", Description: "Organization or membership not found"},
			{Code: "forbidden", Description: "Cannot remove the last owner"},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.org.add_member", "ts.org.list_members"},
		Since:        "v0.3.7",
	})

	r.Register(&ToolDoc{
		Name:     "ts.org.list_members",
		Category: "organization",
		Summary:  "List members of an organization with their roles.",
		Description: `Returns all members of an organization including their roles and join dates.`,
		Parameters: []ParamDoc{
			{Name: "org_id", Type: "string", Description: "Organization ID", Required: true, Example: "org_1d9cb149497656c7..."},
		},
		RequiredParams: []string{"org_id"},
		Returns: ReturnDoc{
			Description: "Returns the list of organization members.",
			Example: map[string]interface{}{
				"members": []map[string]interface{}{
					{"user_id": "usr_476931df38eb2662...", "role": "owner", "joined_at": "2026-02-06T13:36:43Z"},
				},
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "not_found", Description: "Organization not found"},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.org.add_member", "ts.org.remove_member", "ts.org.set_member_role"},
		Since:        "v0.3.7",
	})

	r.Register(&ToolDoc{
		Name:     "ts.org.set_member_role",
		Category: "organization",
		Summary:  "Change a member's role in an organization.",
		Description: `Updates the role of an existing member in an organization.`,
		Parameters: []ParamDoc{
			{Name: "org_id", Type: "string", Description: "Organization ID", Required: true, Example: "org_1d9cb149497656c7..."},
			{Name: "user_id", Type: "string", Description: "User ID", Required: true, Example: "usr_476931df38eb2662..."},
			{Name: "role", Type: "string", Description: "New role: owner, admin, member, guest", Required: true, Example: "admin"},
		},
		RequiredParams: []string{"org_id", "user_id", "role"},
		Returns: ReturnDoc{
			Description: "Returns the updated membership.",
			Example: map[string]interface{}{
				"org_id":  "org_1d9cb149497656c7...",
				"user_id": "usr_476931df38eb2662...",
				"role":    "admin",
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "not_found", Description: "Organization or membership not found"},
			{Code: "forbidden", Description: "Cannot demote the last owner"},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.org.add_member", "ts.org.list_members"},
		Since:        "v0.3.7",
	})
}

func registerEndeavourTools(r *Registry) {
	r.Register(&ToolDoc{
		Name:     "ts.edv.create",
		Category: "endeavour",
		Summary:  "Create a new endeavour (container for related work toward a goal).",
		Description: `Creates a new endeavour in Taskschmiede.

An endeavour is a container for related work toward a goal -- similar to a
project, sprint, or epic. Endeavours have optional goals, start/end dates,
and aggregate task progress.

After creation, associate the endeavour with an organization using
ts.org.add_endeavour and add team members with ts.edv.add_member.`,
		Parameters: []ParamDoc{
			{
				Name:        "name",
				Type:        "string",
				Description: "Endeavour name",
				Required:    true,
				Example:     "Build Taskschmiede",
			},
			{
				Name:        "description",
				Type:        "string",
				Description: "Detailed description",
				Required:    false,
				Example:     "Develop the agent-first task management system",
			},
			{
				Name:        "goals",
				Type:        "array",
				Description: "Success criteria or goals (array of strings)",
				Required:    false,
				Example:     []string{"Ship v0.2.0", "Replace BACKLOG.md"},
			},
			{
				Name:        "start_date",
				Type:        "string",
				Description: "Start date (ISO 8601)",
				Required:    false,
				Example:     "2026-02-06T00:00:00Z",
			},
			{
				Name:        "end_date",
				Type:        "string",
				Description: "End date (ISO 8601)",
				Required:    false,
				Example:     "2026-03-31T00:00:00Z",
			},
			{
				Name:        "metadata",
				Type:        "object",
				Description: "Arbitrary key-value pairs",
				Required:    false,
			},
		},
		RequiredParams: []string{"name"},
		Returns: ReturnDoc{
			Description: "Returns the created endeavour summary.",
			Schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id":         map[string]interface{}{"type": "string"},
					"name":       map[string]interface{}{"type": "string"},
					"status":     map[string]interface{}{"type": "string"},
					"created_at": map[string]interface{}{"type": "string", "format": "date-time"},
				},
			},
			Example: map[string]interface{}{
				"id":         "edv_bd159eb7bb9a877a...",
				"name":       "Build Taskschmiede",
				"status":     "active",
				"created_at": "2026-02-06T13:36:32Z",
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "invalid_input", Description: "Name is required"},
		},
		Examples: []ExampleDoc{
			{
				Title:       "Create an endeavour with goals",
				Description: "Create an endeavour with goals and a start date.",
				Input: map[string]interface{}{
					"name":        "Build Taskschmiede",
					"description": "Develop the agent-first task management system",
					"goals":       []string{"Ship v0.2.0", "Replace BACKLOG.md"},
					"start_date":  "2026-02-06T00:00:00Z",
				},
				Output: map[string]interface{}{
					"id":         "edv_bd159eb7bb9a877a...",
					"name":       "Build Taskschmiede",
					"status":     "active",
					"created_at": "2026-02-06T13:36:32Z",
				},
			},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.edv.get", "ts.edv.list", "ts.edv.update", "ts.org.add_endeavour", "ts.edv.add_member"},
		Since:        "v0.2.0",
	})

	r.Register(&ToolDoc{
		Name:     "ts.edv.get",
		Category: "endeavour",
		Summary:  "Retrieve an endeavour by ID with progress summary.",
		Description: `Retrieves detailed information about an endeavour, including a
task progress breakdown showing how many tasks are in each status
(planned, active, done, canceled).`,
		Parameters: []ParamDoc{
			{
				Name:        "id",
				Type:        "string",
				Description: "Endeavour ID",
				Required:    true,
				Example:     "edv_bd159eb7bb9a877a...",
			},
		},
		RequiredParams: []string{"id"},
		Returns: ReturnDoc{
			Description: "Returns the endeavour with task progress counts.",
			Schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id":          map[string]interface{}{"type": "string"},
					"name":        map[string]interface{}{"type": "string"},
					"description": map[string]interface{}{"type": "string"},
					"goals":       map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
					"status":      map[string]interface{}{"type": "string"},
					"start_date":  map[string]interface{}{"type": "string", "format": "date-time"},
					"end_date":    map[string]interface{}{"type": "string", "format": "date-time"},
					"metadata":    map[string]interface{}{"type": "object"},
					"progress":    map[string]interface{}{"type": "object"},
					"created_at":  map[string]interface{}{"type": "string", "format": "date-time"},
					"updated_at":  map[string]interface{}{"type": "string", "format": "date-time"},
				},
			},
			Example: map[string]interface{}{
				"id":          "edv_bd159eb7bb9a877a...",
				"name":        "Build Taskschmiede",
				"description": "Develop the agent-first task management system",
				"goals":       []string{"Ship v0.2.0", "Replace BACKLOG.md"},
				"status":      "active",
				"progress": map[string]interface{}{
					"tasks": map[string]interface{}{
						"planned":  5,
						"active":   2,
						"done":     7,
						"canceled": 0,
					},
				},
				"created_at": "2026-02-06T13:36:32Z",
				"updated_at": "2026-02-06T13:36:32Z",
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "not_found", Description: "Endeavour with this ID does not exist"},
		},
		Examples: []ExampleDoc{
			{
				Title:       "Get endeavour with progress",
				Description: "Retrieve an endeavour to see task breakdown.",
				Input: map[string]interface{}{
					"id": "edv_bd159eb7bb9a877a...",
				},
				Output: map[string]interface{}{
					"id":     "edv_bd159eb7bb9a877a...",
					"name":   "Build Taskschmiede",
					"status": "active",
					"progress": map[string]interface{}{
						"tasks": map[string]interface{}{
							"planned": 5, "active": 2, "done": 7, "canceled": 0,
						},
					},
					"created_at": "2026-02-06T13:36:32Z",
					"updated_at": "2026-02-06T13:36:32Z",
				},
			},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.edv.create", "ts.edv.list", "ts.edv.update", "ts.tsk.list"},
		Since:        "v0.2.0",
	})

	r.Register(&ToolDoc{
		Name:     "ts.edv.list",
		Category: "endeavour",
		Summary:  "Query endeavours with filters.",
		Description: `Lists endeavours with optional filtering and pagination.

Supports filtering by organization (shows only endeavours linked to that org),
status, and text search.`,
		Parameters: []ParamDoc{
			{
				Name:        "status",
				Type:        "string",
				Description: "Filter by status: pending, active, on_hold, completed, deleted",
				Required:    false,
				Example:     "active",
			},
			{
				Name:        "organization_id",
				Type:        "string",
				Description: "Filter by organization (shows endeavours linked to this org)",
				Required:    false,
				Example:     "org_1d9cb149497656c7...",
			},
			{
				Name:        "search",
				Type:        "string",
				Description: "Search by name or description (partial match)",
				Required:    false,
				Example:     "Taskschmiede",
			},
			{
				Name:        "limit",
				Type:        "integer",
				Description: "Maximum number of results to return",
				Required:    false,
				Default:     50,
			},
			{
				Name:        "offset",
				Type:        "integer",
				Description: "Number of results to skip (for pagination)",
				Required:    false,
				Default:     0,
			},
		},
		RequiredParams: []string{},
		Returns: ReturnDoc{
			Description: "Returns a paginated list of endeavours.",
			Example: map[string]interface{}{
				"endeavours": []map[string]interface{}{
					{
						"id":          "edv_bd159eb7bb9a877a...",
						"name":        "Build Taskschmiede",
						"description": "Develop the agent-first task management system",
						"status":      "active",
						"created_at":  "2026-02-06T13:36:32Z",
						"updated_at":  "2026-02-06T13:36:32Z",
					},
				},
				"total":  1,
				"limit":  50,
				"offset": 0,
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.edv.create", "ts.edv.get", "ts.edv.update"},
		Since:        "v0.2.0",
	})

	r.Register(&ToolDoc{
		Name:     "ts.edv.update",
		Category: "endeavour",
		Summary:  "Update endeavour attributes (partial update).",
		Description: `Updates an existing endeavour. Only provided fields are changed;
omitted fields remain unchanged.

Endeavour status transitions follow a defined lifecycle:
- pending -> active, deleted
- active -> on_hold, completed, deleted
- on_hold -> active, deleted
- completed -> archived (completed is terminal; archive to reclaim tier slot)

The deleted status is a soft delete. Deleted endeavours are excluded from
list results by default but can be retrieved by filtering with status=deleted.`,
		Parameters: []ParamDoc{
			{
				Name:        "id",
				Type:        "string",
				Description: "Endeavour ID",
				Required:    true,
				Example:     "edv_bd159eb7bb9a877a...",
			},
			{
				Name:        "name",
				Type:        "string",
				Description: "New name",
				Required:    false,
			},
			{
				Name:        "description",
				Type:        "string",
				Description: "New description",
				Required:    false,
			},
			{
				Name:        "status",
				Type:        "string",
				Description: "New status: pending, active, on_hold, completed, deleted",
				Required:    false,
				Example:     "completed",
			},
			{
				Name:        "goals",
				Type:        "array",
				Description: "Success criteria / goals (replaces existing array)",
				Required:    false,
			},
			{
				Name:        "start_date",
				Type:        "string",
				Description: "Start date (ISO 8601, empty string to clear)",
				Required:    false,
			},
			{
				Name:        "end_date",
				Type:        "string",
				Description: "End date (ISO 8601, empty string to clear)",
				Required:    false,
			},
			{
				Name:        "metadata",
				Type:        "object",
				Description: "Metadata to set (replaces existing)",
				Required:    false,
			},
		},
		RequiredParams: []string{"id"},
		Returns: ReturnDoc{
			Description: "Returns the list of updated field names.",
			Example: map[string]interface{}{
				"id":             "edv_bd159eb7bb9a877a...",
				"updated_fields": []string{"status"},
				"updated_at":     "2026-02-07T18:00:00Z",
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "not_found", Description: "Endeavour with this ID does not exist"},
			{Code: "invalid_input", Description: "No fields to update"},
			{Code: "invalid_transition", Description: "Invalid status transition"},
		},
		Examples: []ExampleDoc{
			{
				Title:       "Complete an endeavour",
				Description: "Mark an active endeavour as completed.",
				Input: map[string]interface{}{
					"id":     "edv_bd159eb7bb9a877a...",
					"status": "completed",
				},
				Output: map[string]interface{}{
					"id":             "edv_bd159eb7bb9a877a...",
					"updated_fields": []string{"status"},
					"updated_at":     "2026-02-07T18:00:00Z",
				},
			},
			{
				Title:       "Update goals and end date",
				Description: "Modify goals and set an end date.",
				Input: map[string]interface{}{
					"id":       "edv_bd159eb7bb9a877a...",
					"goals":    []string{"Ship v0.3.0", "Agent marketplace prototype"},
					"end_date": "2026-04-30T00:00:00Z",
				},
				Output: map[string]interface{}{
					"id":             "edv_bd159eb7bb9a877a...",
					"updated_fields": []string{"goals", "end_date"},
					"updated_at":     "2026-02-07T18:00:00Z",
				},
			},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.edv.create", "ts.edv.get", "ts.edv.list"},
		Since:        "v0.2.2",
	})

	r.Register(&ToolDoc{
		Name:     "ts.edv.archive",
		Category: "endeavour",
		Summary:  "Archive an endeavour (cancels non-terminal tasks).",
		Description: `Archives an endeavour and cancels all non-terminal tasks within it.

Use confirm=false (default) for a dry-run that shows what would be affected.
Use confirm=true to execute the archive operation.`,
		Parameters: []ParamDoc{
			{Name: "id", Type: "string", Description: "Endeavour ID", Required: true, Example: "edv_bd159eb7bb9a877a..."},
			{Name: "reason", Type: "string", Description: "Reason for archiving", Required: true, Example: "Sprint cancelled"},
			{Name: "confirm", Type: "boolean", Description: "Execute the archive (false = dry-run)", Required: false, Default: false},
		},
		RequiredParams: []string{"id", "reason"},
		Returns: ReturnDoc{
			Description: "Returns the archive result or dry-run summary.",
			Example: map[string]interface{}{
				"id":              "edv_bd159eb7bb9a877a...",
				"archived":        true,
				"tasks_cancelled": 3,
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "not_found", Description: "Endeavour not found"},
			{Code: "forbidden", Description: "Insufficient privileges"},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.edv.get", "ts.edv.update"},
		Since:        "v0.3.7",
	})

	r.Register(&ToolDoc{
		Name:     "ts.edv.export",
		Category: "endeavour",
		Summary:  "Export all endeavour data as JSON.",
		Description: `Exports the complete endeavour data including tasks, members,
and configuration as a JSON document.`,
		Parameters: []ParamDoc{
			{Name: "endeavour_id", Type: "string", Description: "Endeavour ID", Required: true, Example: "edv_bd159eb7bb9a877a..."},
		},
		RequiredParams: []string{"endeavour_id"},
		Returns: ReturnDoc{
			Description: "Returns the full endeavour data as JSON.",
			Example: map[string]interface{}{
				"endeavour": map[string]interface{}{"id": "edv_bd159eb7bb9a877a...", "name": "Build Taskschmiede"},
				"tasks":     []map[string]interface{}{},
				"members":   []map[string]interface{}{},
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "not_found", Description: "Endeavour not found"},
			{Code: "forbidden", Description: "Insufficient privileges"},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.edv.get"},
		Since:        "v0.3.7",
	})

	r.Register(&ToolDoc{
		Name:     "ts.edv.add_member",
		Category: "endeavour",
		Summary:  "Add a user to an endeavour.",
		Description: `Adds a user to an endeavour with a specified role. This controls
who can see and work on tasks within the endeavour.`,
		Parameters: []ParamDoc{
			{Name: "endeavour_id", Type: "string", Description: "Endeavour ID", Required: true, Example: "edv_bd159eb7bb9a877a..."},
			{Name: "user_id", Type: "string", Description: "User ID to add", Required: true, Example: "usr_476931df38eb2662..."},
			{Name: "role", Type: "string", Description: "Role: owner, admin, member, viewer", Required: false, Default: "member", Example: "member"},
		},
		RequiredParams: []string{"endeavour_id", "user_id"},
		Returns: ReturnDoc{
			Description: "Returns the membership confirmation.",
			Example: map[string]interface{}{
				"endeavour_id": "edv_bd159eb7bb9a877a...",
				"user_id":      "usr_476931df38eb2662...",
				"role":         "member",
				"joined_at":    "2026-02-06T13:36:51Z",
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "not_found", Description: "Endeavour or user not found"},
			{Code: "already_member", Description: "User is already a member of this endeavour"},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.edv.remove_member", "ts.edv.list_members", "ts.edv.get"},
		Since:        "v0.3.7",
	})

	r.Register(&ToolDoc{
		Name:     "ts.edv.remove_member",
		Category: "endeavour",
		Summary:  "Remove a user from an endeavour.",
		Description: `Removes a user's membership from an endeavour.`,
		Parameters: []ParamDoc{
			{Name: "endeavour_id", Type: "string", Description: "Endeavour ID", Required: true, Example: "edv_bd159eb7bb9a877a..."},
			{Name: "user_id", Type: "string", Description: "User ID to remove", Required: true, Example: "usr_476931df38eb2662..."},
		},
		RequiredParams: []string{"endeavour_id", "user_id"},
		Returns: ReturnDoc{
			Description: "Returns confirmation of removal.",
			Example: map[string]interface{}{
				"endeavour_id": "edv_bd159eb7bb9a877a...",
				"user_id":      "usr_476931df38eb2662...",
				"removed":      true,
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "not_found", Description: "Endeavour or membership not found"},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.edv.add_member", "ts.edv.list_members"},
		Since:        "v0.3.7",
	})

	r.Register(&ToolDoc{
		Name:     "ts.edv.list_members",
		Category: "endeavour",
		Summary:  "List members of an endeavour with their roles.",
		Description: `Returns all members of an endeavour including their roles and join dates.`,
		Parameters: []ParamDoc{
			{Name: "endeavour_id", Type: "string", Description: "Endeavour ID", Required: true, Example: "edv_bd159eb7bb9a877a..."},
		},
		RequiredParams: []string{"endeavour_id"},
		Returns: ReturnDoc{
			Description: "Returns the list of endeavour members.",
			Example: map[string]interface{}{
				"members": []map[string]interface{}{
					{"user_id": "usr_476931df38eb2662...", "role": "member", "joined_at": "2026-02-06T13:36:51Z"},
				},
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "not_found", Description: "Endeavour not found"},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.edv.add_member", "ts.edv.remove_member"},
		Since:        "v0.3.7",
	})
}

func registerTaskTools(r *Registry) {
	r.Register(&ToolDoc{
		Name:     "ts.tsk.create",
		Category: "task",
		Summary:  "Create a new task (atomic unit of work).",
		Description: `Creates a new task in Taskschmiede.

Tasks are the atomic units of work. They can optionally belong to an
endeavour and be assigned to a resource (human or agent). New tasks
start in "planned" status.`,
		Parameters: []ParamDoc{
			{
				Name:        "title",
				Type:        "string",
				Description: "Task title",
				Required:    true,
				Example:     "Implement demand MCP tools",
			},
			{
				Name:        "description",
				Type:        "string",
				Description: "Detailed description of the work",
				Required:    false,
				Example:     "Implement CRUD tools for the demand entity",
			},
			{
				Name:        "endeavour_id",
				Type:        "string",
				Description: "Endeavour this task belongs to",
				Required:    false,
				Example:     "edv_bd159eb7bb9a877a...",
			},
			{
				Name:        "assignee_id",
				Type:        "string",
				Description: "Resource ID to assign the task to",
				Required:    false,
				Example:     "res_claude",
			},
			{
				Name:        "estimate",
				Type:        "number",
				Description: "Estimated hours of work",
				Required:    false,
				Example:     4.0,
			},
			{
				Name:        "due_date",
				Type:        "string",
				Description: "Due date (ISO 8601)",
				Required:    false,
				Example:     "2026-02-14T00:00:00Z",
			},
			{
				Name:        "metadata",
				Type:        "object",
				Description: "Arbitrary key-value pairs (e.g., type, priority, tags)",
				Required:    false,
				Example:     map[string]interface{}{"type": "feature", "backlog_ref": "#4"},
			},
		},
		RequiredParams: []string{"title"},
		Returns: ReturnDoc{
			Description: "Returns the created task summary.",
			Schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id":           map[string]interface{}{"type": "string"},
					"title":        map[string]interface{}{"type": "string"},
					"status":       map[string]interface{}{"type": "string"},
					"endeavour_id": map[string]interface{}{"type": "string"},
					"assignee_id":  map[string]interface{}{"type": "string"},
					"created_at":   map[string]interface{}{"type": "string", "format": "date-time"},
				},
			},
			Example: map[string]interface{}{
				"id":           "tsk_68e9623ade9b1631...",
				"title":        "Implement demand MCP tools",
				"status":       "planned",
				"endeavour_id": "edv_bd159eb7bb9a877a...",
				"assignee_id":  "res_claude",
				"created_at":   "2026-02-06T13:37:12Z",
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "invalid_input", Description: "Title is required"},
		},
		Examples: []ExampleDoc{
			{
				Title:       "Create a task in an endeavour",
				Description: "Create a task assigned to an agent within an endeavour.",
				Input: map[string]interface{}{
					"title":        "Implement demand MCP tools",
					"description":  "Implement CRUD tools for the demand entity",
					"endeavour_id": "edv_bd159eb7bb9a877a...",
					"assignee_id":  "res_claude",
					"metadata":     map[string]interface{}{"type": "feature"},
				},
				Output: map[string]interface{}{
					"id":           "tsk_68e9623ade9b1631...",
					"title":        "Implement demand MCP tools",
					"status":       "planned",
					"endeavour_id": "edv_bd159eb7bb9a877a...",
					"assignee_id":  "res_claude",
					"created_at":   "2026-02-06T13:37:12Z",
				},
			},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.tsk.get", "ts.tsk.list", "ts.tsk.update", "ts.tsk.cancel"},
		Since:        "v0.2.0",
	})

	r.Register(&ToolDoc{
		Name:     "ts.tsk.get",
		Category: "task",
		Summary:  "Retrieve a task by ID.",
		Description: `Retrieves detailed information about a specific task.

The response includes the assignee's display name (if assigned) and
lifecycle timestamps (started_at, completed_at, canceled_at) when applicable.`,
		Parameters: []ParamDoc{
			{
				Name:        "id",
				Type:        "string",
				Description: "Task ID",
				Required:    true,
				Example:     "tsk_68e9623ade9b1631...",
			},
		},
		RequiredParams: []string{"id"},
		Returns: ReturnDoc{
			Description: "Returns the full task object.",
			Schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id":              map[string]interface{}{"type": "string"},
					"title":           map[string]interface{}{"type": "string"},
					"description":     map[string]interface{}{"type": "string"},
					"status":          map[string]interface{}{"type": "string"},
					"endeavour_id":    map[string]interface{}{"type": "string"},
					"assignee_id":     map[string]interface{}{"type": "string"},
					"assignee_name":   map[string]interface{}{"type": "string"},
					"estimate":        map[string]interface{}{"type": "number"},
					"actual":          map[string]interface{}{"type": "number"},
					"due_date":        map[string]interface{}{"type": "string", "format": "date-time"},
					"metadata":        map[string]interface{}{"type": "object"},
					"started_at":      map[string]interface{}{"type": "string", "format": "date-time"},
					"completed_at":    map[string]interface{}{"type": "string", "format": "date-time"},
					"canceled_at":     map[string]interface{}{"type": "string", "format": "date-time"},
					"canceled_reason": map[string]interface{}{"type": "string"},
					"created_at":      map[string]interface{}{"type": "string", "format": "date-time"},
					"updated_at":      map[string]interface{}{"type": "string", "format": "date-time"},
				},
			},
			Example: map[string]interface{}{
				"id":            "tsk_68e9623ade9b1631...",
				"title":         "Implement demand MCP tools",
				"description":   "Implement CRUD tools for the demand entity",
				"status":        "active",
				"endeavour_id":  "edv_bd159eb7bb9a877a...",
				"assignee_id":   "res_claude",
				"assignee_name": "Claude",
				"estimate":      4.0,
				"metadata":      map[string]interface{}{"type": "feature"},
				"started_at":    "2026-02-06T14:00:00Z",
				"created_at":    "2026-02-06T13:37:12Z",
				"updated_at":    "2026-02-06T14:00:00Z",
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "not_found", Description: "Task with this ID does not exist"},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.tsk.create", "ts.tsk.list", "ts.tsk.update"},
		Since:        "v0.2.0",
	})

	r.Register(&ToolDoc{
		Name:     "ts.tsk.list",
		Category: "task",
		Summary:  "Query tasks with filters.",
		Description: `Lists tasks with optional filtering and pagination.

Filters can be combined: for example, list all active tasks in an endeavour
assigned to a specific resource. Text search matches against title and description.

Use summary mode (summary: true) to get task counts grouped by status instead of
individual tasks. This is useful for a quick backlog overview.`,
		Parameters: []ParamDoc{
			{
				Name:        "status",
				Type:        "string",
				Description: "Filter by status: planned, active, done, canceled",
				Required:    false,
				Example:     "active",
			},
			{
				Name:        "endeavour_id",
				Type:        "string",
				Description: "Filter by endeavour",
				Required:    false,
				Example:     "edv_bd159eb7bb9a877a...",
			},
			{
				Name:        "assignee_id",
				Type:        "string",
				Description: "Filter by assignee resource",
				Required:    false,
				Example:     "res_claude",
			},
			{
				Name:        "search",
				Type:        "string",
				Description: "Search in title and description (partial match)",
				Required:    false,
				Example:     "demand",
			},
			{
				Name:        "limit",
				Type:        "integer",
				Description: "Maximum number of results to return",
				Required:    false,
				Default:     50,
			},
			{
				Name:        "offset",
				Type:        "integer",
				Description: "Number of results to skip (for pagination)",
				Required:    false,
				Default:     0,
			},
			{
				Name:        "summary",
				Type:        "boolean",
				Description: "If true, return status counts instead of individual tasks",
				Required:    false,
				Default:     false,
			},
		},
		RequiredParams: []string{},
		Returns: ReturnDoc{
			Description: "Returns a paginated list of tasks, or status counts when summary is true.",
			Example: map[string]interface{}{
				"tasks": []map[string]interface{}{
					{
						"id":            "tsk_68e9623ade9b1631...",
						"title":         "Implement demand MCP tools",
						"status":        "planned",
						"endeavour_id":  "edv_bd159eb7bb9a877a...",
						"assignee_id":   "res_claude",
						"assignee_name": "Claude",
						"created_at":    "2026-02-06T13:37:12Z",
						"updated_at":    "2026-02-06T13:37:12Z",
					},
				},
				"total":  1,
				"limit":  50,
				"offset": 0,
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session. Call ts.auth.login first."},
		},
		Examples: []ExampleDoc{
			{
				Title:       "Summary by status",
				Description: "Get a quick overview of task counts grouped by status.",
				Input: map[string]interface{}{
					"endeavour_id": "edv_bd159eb7bb9a877a...",
					"summary":      true,
				},
				Output: map[string]interface{}{
					"summary": map[string]interface{}{
						"planned":  5,
						"active":   1,
						"done":     10,
						"canceled": 0,
					},
					"total": 16,
				},
			},
			{
				Title:       "List tasks by endeavour",
				Description: "Get all tasks belonging to an endeavour.",
				Input: map[string]interface{}{
					"endeavour_id": "edv_bd159eb7bb9a877a...",
				},
				Output: map[string]interface{}{
					"tasks": []map[string]interface{}{
						{
							"id":     "tsk_68e9623ade9b1631...",
							"title":  "Implement demand MCP tools",
							"status": "planned",
						},
					},
					"total":  1,
					"limit":  50,
					"offset": 0,
				},
			},
			{
				Title:       "List active tasks assigned to a resource",
				Description: "Find tasks currently being worked on by a specific agent.",
				Input: map[string]interface{}{
					"status":      "active",
					"assignee_id": "res_claude",
				},
				Output: map[string]interface{}{
					"tasks": []map[string]interface{}{
						{
							"id":            "tsk_55c1a70b3e18cdde...",
							"title":         "Vertical slice implementation",
							"status":        "active",
							"assignee_id":   "res_claude",
							"assignee_name": "Claude",
						},
					},
					"total":  1,
					"limit":  50,
					"offset": 0,
				},
			},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.tsk.create", "ts.tsk.get", "ts.tsk.update"},
		Since:        "v0.2.0",
	})

	r.Register(&ToolDoc{
		Name:     "ts.tsk.update",
		Category: "task",
		Summary:  "Update task attributes (partial update).",
		Description: `Updates one or more fields of an existing task. Only provided fields
are changed; omitted fields remain unchanged.

Status transitions are validated:
- planned -> active, canceled
- active -> done, canceled, planned
- done -> active (reopen)
- canceled -> planned (reopen)

Lifecycle timestamps are set automatically:
- started_at: set when status changes to active
- completed_at: set when status changes to done
- canceled_at: set when status changes to canceled

To unassign or unlink, pass an empty string for assignee_id or endeavour_id.`,
		Parameters: []ParamDoc{
			{
				Name:        "id",
				Type:        "string",
				Description: "Task ID to update",
				Required:    true,
				Example:     "tsk_68e9623ade9b1631...",
			},
			{
				Name:        "title",
				Type:        "string",
				Description: "New title",
				Required:    false,
			},
			{
				Name:        "description",
				Type:        "string",
				Description: "New description",
				Required:    false,
			},
			{
				Name:        "status",
				Type:        "string",
				Description: "New status: planned, active, done, canceled",
				Required:    false,
				Example:     "active",
			},
			{
				Name:        "endeavour_id",
				Type:        "string",
				Description: "New endeavour (empty string to unlink)",
				Required:    false,
			},
			{
				Name:        "assignee_id",
				Type:        "string",
				Description: "New assignee resource (empty string to unassign)",
				Required:    false,
			},
			{
				Name:        "estimate",
				Type:        "number",
				Description: "Estimated hours",
				Required:    false,
			},
			{
				Name:        "actual",
				Type:        "number",
				Description: "Actual hours spent",
				Required:    false,
			},
			{
				Name:        "due_date",
				Type:        "string",
				Description: "Due date (ISO 8601, empty string to clear)",
				Required:    false,
			},
			{
				Name:        "canceled_reason",
				Type:        "string",
				Description: "Reason for cancellation (when setting status to canceled)",
				Required:    false,
			},
			{
				Name:        "metadata",
				Type:        "object",
				Description: "Metadata to set (replaces existing metadata)",
				Required:    false,
			},
		},
		RequiredParams: []string{"id"},
		Returns: ReturnDoc{
			Description: "Returns the task ID and list of fields that were updated.",
			Schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id":             map[string]interface{}{"type": "string"},
					"updated_fields": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
					"updated_at":     map[string]interface{}{"type": "string", "format": "date-time"},
				},
			},
			Example: map[string]interface{}{
				"id":             "tsk_68e9623ade9b1631...",
				"updated_fields": []string{"status", "title"},
				"updated_at":     "2026-02-06T14:00:00Z",
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "not_found", Description: "Task with this ID does not exist"},
			{Code: "invalid_transition", Description: "Invalid status transition (e.g., planned -> done)"},
		},
		Examples: []ExampleDoc{
			{
				Title:       "Start working on a task",
				Description: "Transition a task from planned to active.",
				Input: map[string]interface{}{
					"id":     "tsk_68e9623ade9b1631...",
					"status": "active",
				},
				Output: map[string]interface{}{
					"id":             "tsk_68e9623ade9b1631...",
					"updated_fields": []string{"status"},
					"updated_at":     "2026-02-06T14:00:00Z",
				},
			},
			{
				Title:       "Complete a task",
				Description: "Mark an active task as done.",
				Input: map[string]interface{}{
					"id":     "tsk_68e9623ade9b1631...",
					"status": "done",
					"actual": 3.5,
				},
				Output: map[string]interface{}{
					"id":             "tsk_68e9623ade9b1631...",
					"updated_fields": []string{"status", "actual"},
					"updated_at":     "2026-02-06T16:30:00Z",
				},
			},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.tsk.create", "ts.tsk.get", "ts.tsk.list", "ts.tsk.cancel"},
		Since:        "v0.2.0",
	})

	r.Register(&ToolDoc{
		Name:     "ts.tsk.cancel",
		Category: "task",
		Summary:  "Cancel a task with a reason.",
		Description: `Convenience tool to cancel a task. Equivalent to calling ts.tsk.update
with status "canceled" and a canceled_reason, but with a simpler interface.

Both id and reason are required.`,
		Parameters: []ParamDoc{
			{
				Name:        "id",
				Type:        "string",
				Description: "Task ID to cancel",
				Required:    true,
				Example:     "tsk_68e9623ade9b1631...",
			},
			{
				Name:        "reason",
				Type:        "string",
				Description: "Reason for cancellation",
				Required:    true,
				Example:     "Superseded by new approach",
			},
		},
		RequiredParams: []string{"id", "reason"},
		Returns: ReturnDoc{
			Description: "Returns the task ID and list of fields that were updated.",
			Example: map[string]interface{}{
				"id":             "tsk_68e9623ade9b1631...",
				"updated_fields": []string{"status", "canceled_reason"},
				"updated_at":     "2026-02-16T10:00:00Z",
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "not_found", Description: "Task with this ID does not exist"},
			{Code: "invalid_input", Description: "Task ID or reason is missing"},
			{Code: "invalid_transition", Description: "Task cannot be canceled from its current status"},
		},
		Examples: []ExampleDoc{
			{
				Title:       "Cancel a task",
				Description: "Cancel a planned task that is no longer needed.",
				Input: map[string]interface{}{
					"id":     "tsk_68e9623ade9b1631...",
					"reason": "Superseded by new approach",
				},
				Output: map[string]interface{}{
					"id":             "tsk_68e9623ade9b1631...",
					"updated_fields": []string{"status", "canceled_reason"},
					"updated_at":     "2026-02-16T10:00:00Z",
				},
			},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.tsk.update", "ts.tsk.get", "ts.tsk.list"},
		Since:        "v0.8.0",
	})
}

func registerRelationshipTools(r *Registry) {
	r.Register(&ToolDoc{
		Name:     "ts.org.add_resource",
		Category: "organization",
		Summary:  "Add a resource to an organization.",
		Description: `Adds a non-user resource (budget, service, equipment) to an organization.

For adding users (humans or agents) as members, use ts.org.add_member instead.
This tool is for non-user capacity entities that need to be associated with an
organization -- e.g., shared infrastructure, budget allocations, or external services.`,
		Parameters: []ParamDoc{
			{
				Name:        "organization_id",
				Type:        "string",
				Description: "Organization ID",
				Required:    true,
				Example:     "org_1d9cb149497656c7...",
			},
			{
				Name:        "resource_id",
				Type:        "string",
				Description: "Resource ID to add",
				Required:    true,
				Example:     "res_claude",
			},
			{
				Name:        "role",
				Type:        "string",
				Description: "Role: owner, admin, member, guest",
				Required:    false,
				Default:     "member",
				Example:     "member",
			},
		},
		RequiredParams: []string{"organization_id", "resource_id"},
		Returns: ReturnDoc{
			Description: "Returns the membership confirmation.",
			Example: map[string]interface{}{
				"organization_id": "org_1d9cb149497656c7...",
				"resource_id":     "res_claude",
				"role":            "member",
				"joined_at":       "2026-02-06T13:36:43Z",
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "not_found", Description: "Organization not found"},
			{Code: "invalid_input", Description: "organization_id and resource_id are required"},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.org.create", "ts.org.get"},
		Since:        "v0.2.0",
	})

	r.Register(&ToolDoc{
		Name:     "ts.org.add_endeavour",
		Category: "organization",
		Summary:  "Associate an endeavour with an organization.",
		Description: `Links an endeavour to an organization. This creates a many-to-many
relationship: an endeavour can belong to multiple organizations, and
an organization can have multiple endeavours.`,
		Parameters: []ParamDoc{
			{
				Name:        "organization_id",
				Type:        "string",
				Description: "Organization ID",
				Required:    true,
				Example:     "org_1d9cb149497656c7...",
			},
			{
				Name:        "endeavour_id",
				Type:        "string",
				Description: "Endeavour ID to associate",
				Required:    true,
				Example:     "edv_bd159eb7bb9a877a...",
			},
			{
				Name:        "role",
				Type:        "string",
				Description: "Role: owner, participant",
				Required:    false,
				Default:     "participant",
				Example:     "owner",
			},
		},
		RequiredParams: []string{"organization_id", "endeavour_id"},
		Returns: ReturnDoc{
			Description: "Returns the association confirmation.",
			Example: map[string]interface{}{
				"organization_id": "org_1d9cb149497656c7...",
				"endeavour_id":    "edv_bd159eb7bb9a877a...",
				"role":            "owner",
				"created_at":      "2026-02-06T13:36:38Z",
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "not_found", Description: "Organization not found"},
			{Code: "invalid_input", Description: "organization_id and endeavour_id are required"},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.org.create", "ts.edv.create", "ts.edv.list"},
		Since:        "v0.2.0",
	})
}

func registerResourceTools(r *Registry) {
	r.Register(&ToolDoc{
		Name:     "ts.res.create",
		Category: "resource",
		Summary:  "Create a new resource (human, agent, service, or budget).",
		Description: `Creates a new resource in Taskschmiede.

Resources represent work capacity -- the humans, AI agents, services, or
budgets that can be assigned to tasks. Each resource has a type, optional
capacity model, skills, and metadata.

Resource types:
- human: a person
- agent: an AI agent or automated system
- service: an external service or API
- budget: a financial allocation

After creation, add the resource to an organization with ts.org.add_resource.`,
		Parameters: []ParamDoc{
			{
				Name:        "type",
				Type:        "string",
				Description: "Resource type: human, agent, service, budget",
				Required:    true,
				Example:     "agent",
			},
			{
				Name:        "name",
				Type:        "string",
				Description: "Resource name",
				Required:    true,
				Example:     "Claude",
			},
			{
				Name:        "capacity_model",
				Type:        "string",
				Description: "Capacity model: hours_per_week, tokens_per_day, always_on, budget",
				Required:    false,
				Example:     "always_on",
			},
			{
				Name:        "capacity_value",
				Type:        "number",
				Description: "Amount of capacity (interpretation depends on capacity_model)",
				Required:    false,
				Example:     40.0,
			},
			{
				Name:        "skills",
				Type:        "array",
				Description: "List of skills or capabilities (array of strings)",
				Required:    false,
				Example:     []string{"code_review", "testing", "documentation"},
			},
			{
				Name:        "metadata",
				Type:        "object",
				Description: "Arbitrary key-value pairs (e.g., email, timezone, model_id)",
				Required:    false,
			},
		},
		RequiredParams: []string{"type", "name"},
		Returns: ReturnDoc{
			Description: "Returns the created resource summary.",
			Example: map[string]interface{}{
				"id":         "res_a1b2c3d4e5f6...",
				"type":       "agent",
				"name":       "Claude",
				"status":     "active",
				"created_at": "2026-02-07T18:00:00Z",
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "invalid_input", Description: "Type and name are required; type must be human, agent, service, or budget"},
		},
		Examples: []ExampleDoc{
			{
				Title:       "Create an AI agent resource",
				Description: "Register an AI agent with skills.",
				Input: map[string]interface{}{
					"type":           "agent",
					"name":           "Claude",
					"capacity_model": "always_on",
					"skills":         []string{"code_review", "testing", "documentation"},
					"metadata":       map[string]interface{}{"model_id": "claude-opus-4-6"},
				},
				Output: map[string]interface{}{
					"id":         "res_a1b2c3d4e5f6...",
					"type":       "agent",
					"name":       "Claude",
					"status":     "active",
					"created_at": "2026-02-07T18:00:00Z",
				},
			},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.res.get", "ts.res.list", "ts.res.update", "ts.org.add_resource"},
		Since:        "v0.2.2",
	})

	r.Register(&ToolDoc{
		Name:     "ts.res.get",
		Category: "resource",
		Summary:  "Retrieve a resource by ID.",
		Description: `Retrieves detailed information about a resource, including its
type, capacity, skills, and metadata.`,
		Parameters: []ParamDoc{
			{
				Name:        "id",
				Type:        "string",
				Description: "Resource ID",
				Required:    true,
				Example:     "res_a1b2c3d4e5f6...",
			},
		},
		RequiredParams: []string{"id"},
		Returns: ReturnDoc{
			Description: "Returns the full resource details.",
			Example: map[string]interface{}{
				"id":             "res_a1b2c3d4e5f6...",
				"type":           "agent",
				"name":           "Claude",
				"capacity_model": "always_on",
				"skills":         []string{"code_review", "testing", "documentation"},
				"metadata":       map[string]interface{}{"model_id": "claude-opus-4-6"},
				"status":         "active",
				"created_at":     "2026-02-07T18:00:00Z",
				"updated_at":     "2026-02-07T18:00:00Z",
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "not_found", Description: "Resource with this ID does not exist"},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.res.create", "ts.res.list", "ts.res.update"},
		Since:        "v0.2.2",
	})

	r.Register(&ToolDoc{
		Name:     "ts.res.list",
		Category: "resource",
		Summary:  "Query resources with filters.",
		Description: `Lists resources with optional filtering by type, status, organization
membership, and text search. Supports pagination.`,
		Parameters: []ParamDoc{
			{
				Name:        "type",
				Type:        "string",
				Description: "Filter by type: human, agent, service, budget",
				Required:    false,
				Example:     "agent",
			},
			{
				Name:        "status",
				Type:        "string",
				Description: "Filter by status: active, inactive",
				Required:    false,
				Example:     "active",
			},
			{
				Name:        "organization_id",
				Type:        "string",
				Description: "Filter by organization membership",
				Required:    false,
			},
			{
				Name:        "search",
				Type:        "string",
				Description: "Search by name (partial match)",
				Required:    false,
			},
			{
				Name:        "limit",
				Type:        "integer",
				Description: "Maximum number of results to return",
				Required:    false,
				Default:     50,
			},
			{
				Name:        "offset",
				Type:        "integer",
				Description: "Number of results to skip (for pagination)",
				Required:    false,
				Default:     0,
			},
		},
		RequiredParams: []string{},
		Returns: ReturnDoc{
			Description: "Returns a paginated list of resources.",
			Example: map[string]interface{}{
				"resources": []map[string]interface{}{
					{
						"id":         "res_a1b2c3d4e5f6...",
						"type":       "agent",
						"name":       "Claude",
						"status":     "active",
						"created_at": "2026-02-07T18:00:00Z",
						"updated_at": "2026-02-07T18:00:00Z",
					},
				},
				"total":  1,
				"limit":  50,
				"offset": 0,
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.res.create", "ts.res.get", "ts.res.update", "ts.org.add_resource"},
		Since:        "v0.2.2",
	})

	r.Register(&ToolDoc{
		Name:     "ts.res.update",
		Category: "resource",
		Summary:  "Update resource attributes (partial update).",
		Description: `Updates one or more fields on an existing resource. Only the provided
fields are changed; omitted fields retain their current values.

The resource type cannot be changed after creation.`,
		Parameters: []ParamDoc{
			{
				Name:        "id",
				Type:        "string",
				Description: "Resource ID",
				Required:    true,
				Example:     "res_abc123",
			},
			{
				Name:        "name",
				Type:        "string",
				Description: "New name",
				Example:     "Senior Build Agent",
			},
			{
				Name:        "capacity_model",
				Type:        "string",
				Description: "Capacity model: hours_per_week, tokens_per_day, always_on, budget",
				Example:     "hours_per_week",
			},
			{
				Name:        "capacity_value",
				Type:        "number",
				Description: "Amount of capacity",
				Example:     "40",
			},
			{
				Name:        "skills",
				Type:        "array",
				Description: "List of skills or capabilities (replaces existing)",
				Example:     `["go", "testing", "deployment", "monitoring"]`,
			},
			{
				Name:        "metadata",
				Type:        "object",
				Description: "Metadata to set (replaces existing)",
				Example:     `{"timezone": "UTC", "model_id": "claude-opus-4"}`,
			},
			{
				Name:        "status",
				Type:        "string",
				Description: "New status: active, inactive",
				Example:     "inactive",
			},
		},
		RequiredParams: []string{"id"},
		Returns: ReturnDoc{
			Description: "Returns the updated resource.",
			Schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id":             map[string]interface{}{"type": "string"},
					"type":           map[string]interface{}{"type": "string"},
					"name":           map[string]interface{}{"type": "string"},
					"capacity_model": map[string]interface{}{"type": "string"},
					"capacity_value": map[string]interface{}{"type": "number"},
					"skills":         map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
					"metadata":       map[string]interface{}{"type": "object"},
					"status":         map[string]interface{}{"type": "string"},
					"created_at":     map[string]interface{}{"type": "string", "format": "date-time"},
					"updated_at":     map[string]interface{}{"type": "string", "format": "date-time"},
				},
			},
			Example: map[string]interface{}{
				"id":             "res_abc123",
				"type":           "agent",
				"name":           "Senior Build Agent",
				"capacity_model": "always_on",
				"skills":         []string{"go", "testing", "deployment", "monitoring"},
				"metadata":       map[string]interface{}{"timezone": "UTC"},
				"status":         "active",
				"created_at":     "2026-02-07T10:00:00Z",
				"updated_at":     "2026-02-10T14:30:00Z",
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "not_found", Description: "Resource with this ID does not exist"},
			{Code: "invalid_input", Description: "No fields to update, or invalid status value"},
		},
		Examples: []ExampleDoc{
			{
				Title:       "Update resource name and skills",
				Description: "Change a resource's name and add new skills.",
				Input: map[string]interface{}{
					"id":     "res_abc123",
					"name":   "Senior Build Agent",
					"skills": []string{"go", "testing", "deployment", "monitoring"},
				},
			},
			{
				Title:       "Deactivate a resource",
				Description: "Set a resource to inactive status.",
				Input: map[string]interface{}{
					"id":     "res_abc123",
					"status": "inactive",
				},
			},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.res.create", "ts.res.get", "ts.res.list"},
		Since:        "v0.2.4",
	})

	r.Register(&ToolDoc{
		Name:     "ts.res.delete",
		Category: "resource",
		Summary:  "Delete a resource (admin only).",
		Description: `Deletes a resource permanently. Requires admin privileges.`,
		Parameters: []ParamDoc{
			{Name: "id", Type: "string", Description: "Resource ID", Required: true, Example: "res_a1b2c3d4e5f6..."},
		},
		RequiredParams: []string{"id"},
		Returns: ReturnDoc{
			Description: "Returns confirmation of deletion.",
			Example: map[string]interface{}{
				"id":      "res_a1b2c3d4e5f6...",
				"deleted": true,
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "not_found", Description: "Resource not found"},
			{Code: "forbidden", Description: "Admin privileges required"},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.res.create", "ts.res.get", "ts.res.list"},
		Since:        "v0.3.7",
	})
}

func registerDemandTools(r *Registry) {
	r.Register(&ToolDoc{
		Name:     "ts.dmd.create",
		Category: "demand",
		Summary:  "Create a new demand (what needs to be fulfilled).",
		Description: `Creates a new demand in Taskschmiede.

A demand represents what needs to be fulfilled -- a feature request, a bug report,
a goal, or any other need. Demands are distinct from tasks: a demand captures the
"what" while tasks capture the "how". A single demand may result in multiple tasks.

Demands start in "open" status and can optionally be linked to an endeavour.`,
		Parameters: []ParamDoc{
			{
				Name:        "type",
				Type:        "string",
				Description: "Demand type (e.g., feature, bug, goal, meeting, epic)",
				Required:    true,
				Example:     "feature",
			},
			{
				Name:        "title",
				Type:        "string",
				Description: "Demand title",
				Required:    true,
				Example:     "Add dark mode support",
			},
			{
				Name:        "description",
				Type:        "string",
				Description: "Detailed description",
				Required:    false,
				Example:     "Users need a dark theme option for reduced eye strain.",
			},
			{
				Name:        "priority",
				Type:        "string",
				Description: "Priority: low, medium (default), high, urgent",
				Required:    false,
				Default:     "medium",
				Example:     "high",
			},
			{
				Name:        "endeavour_id",
				Type:        "string",
				Description: "Endeavour this demand belongs to",
				Required:    false,
				Example:     "edv_bd159eb7bb9a877a...",
			},
			{
				Name:        "due_date",
				Type:        "string",
				Description: "Due date (ISO 8601)",
				Required:    false,
				Example:     "2026-03-01T00:00:00Z",
			},
			{
				Name:        "metadata",
				Type:        "object",
				Description: "Arbitrary key-value pairs",
				Required:    false,
			},
		},
		RequiredParams: []string{"type", "title"},
		Returns: ReturnDoc{
			Description: "Returns the created demand summary.",
			Schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id":         map[string]interface{}{"type": "string"},
					"type":       map[string]interface{}{"type": "string"},
					"title":      map[string]interface{}{"type": "string"},
					"status":     map[string]interface{}{"type": "string"},
					"priority":   map[string]interface{}{"type": "string"},
					"created_at": map[string]interface{}{"type": "string", "format": "date-time"},
				},
			},
			Example: map[string]interface{}{
				"id":         "dmd_a1b2c3d4e5f6...",
				"type":       "feature",
				"title":      "Add dark mode support",
				"status":     "open",
				"priority":   "high",
				"created_at": "2026-02-09T10:00:00Z",
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "invalid_input", Description: "Type and title are required"},
		},
		Examples: []ExampleDoc{
			{
				Title:       "Create a feature demand",
				Description: "Record a new feature request linked to an endeavour.",
				Input: map[string]interface{}{
					"type":         "feature",
					"title":        "Add dark mode support",
					"description":  "Users need a dark theme option for reduced eye strain.",
					"priority":     "high",
					"endeavour_id": "edv_bd159eb7bb9a877a...",
				},
				Output: map[string]interface{}{
					"id":         "dmd_a1b2c3d4e5f6...",
					"type":       "feature",
					"title":      "Add dark mode support",
					"status":     "open",
					"priority":   "high",
					"created_at": "2026-02-09T10:00:00Z",
				},
			},
			{
				Title:       "Create a bug demand",
				Description: "Report a bug with urgent priority.",
				Input: map[string]interface{}{
					"type":     "bug",
					"title":    "Login fails on mobile browsers",
					"priority": "urgent",
				},
				Output: map[string]interface{}{
					"id":         "dmd_f6e5d4c3b2a1...",
					"type":       "bug",
					"title":      "Login fails on mobile browsers",
					"status":     "open",
					"priority":   "urgent",
					"created_at": "2026-02-09T10:00:00Z",
				},
			},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.dmd.get", "ts.dmd.list", "ts.dmd.update", "ts.dmd.cancel"},
		Since:        "v0.2.0",
	})

	r.Register(&ToolDoc{
		Name:     "ts.dmd.get",
		Category: "demand",
		Summary:  "Retrieve a demand by ID.",
		Description: `Retrieves detailed information about a specific demand, including
its type, priority, status, and any linked endeavour.`,
		Parameters: []ParamDoc{
			{
				Name:        "id",
				Type:        "string",
				Description: "Demand ID",
				Required:    true,
				Example:     "dmd_a1b2c3d4e5f6...",
			},
		},
		RequiredParams: []string{"id"},
		Returns: ReturnDoc{
			Description: "Returns the full demand object.",
			Schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id":              map[string]interface{}{"type": "string"},
					"type":            map[string]interface{}{"type": "string"},
					"title":           map[string]interface{}{"type": "string"},
					"description":     map[string]interface{}{"type": "string"},
					"status":          map[string]interface{}{"type": "string"},
					"priority":        map[string]interface{}{"type": "string"},
					"endeavour_id":    map[string]interface{}{"type": "string"},
					"due_date":        map[string]interface{}{"type": "string", "format": "date-time"},
					"canceled_reason": map[string]interface{}{"type": "string"},
					"metadata":        map[string]interface{}{"type": "object"},
					"created_at":      map[string]interface{}{"type": "string", "format": "date-time"},
					"updated_at":      map[string]interface{}{"type": "string", "format": "date-time"},
				},
			},
			Example: map[string]interface{}{
				"id":          "dmd_a1b2c3d4e5f6...",
				"type":        "feature",
				"title":       "Add dark mode support",
				"description": "Users need a dark theme option for reduced eye strain.",
				"status":      "open",
				"priority":    "high",
				"metadata":    map[string]interface{}{},
				"created_at":  "2026-02-09T10:00:00Z",
				"updated_at":  "2026-02-09T10:00:00Z",
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "not_found", Description: "Demand with this ID does not exist"},
		},
		Examples: []ExampleDoc{
			{
				Title: "Get demand by ID",
				Input: map[string]interface{}{
					"id": "dmd_a1b2c3d4e5f6...",
				},
				Output: map[string]interface{}{
					"id":         "dmd_a1b2c3d4e5f6...",
					"type":       "feature",
					"title":      "Add dark mode support",
					"status":     "open",
					"priority":   "high",
					"created_at": "2026-02-09T10:00:00Z",
					"updated_at": "2026-02-09T10:00:00Z",
				},
			},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.dmd.create", "ts.dmd.list", "ts.dmd.update"},
		Since:        "v0.2.0",
	})

	r.Register(&ToolDoc{
		Name:     "ts.dmd.list",
		Category: "demand",
		Summary:  "Query demands with filters.",
		Description: `Lists demands with optional filtering and pagination.

Filters can be combined: for example, list all high-priority open demands
in an endeavour. Text search matches against title and description.`,
		Parameters: []ParamDoc{
			{
				Name:        "status",
				Type:        "string",
				Description: "Filter by status: open, in_progress, fulfilled, canceled",
				Required:    false,
				Example:     "open",
			},
			{
				Name:        "type",
				Type:        "string",
				Description: "Filter by demand type (e.g., feature, bug, goal)",
				Required:    false,
				Example:     "feature",
			},
			{
				Name:        "priority",
				Type:        "string",
				Description: "Filter by priority: low, medium, high, urgent",
				Required:    false,
				Example:     "high",
			},
			{
				Name:        "endeavour_id",
				Type:        "string",
				Description: "Filter by endeavour",
				Required:    false,
				Example:     "edv_bd159eb7bb9a877a...",
			},
			{
				Name:        "search",
				Type:        "string",
				Description: "Search in title and description (partial match)",
				Required:    false,
				Example:     "dark mode",
			},
			{
				Name:        "limit",
				Type:        "integer",
				Description: "Maximum number of results to return",
				Required:    false,
				Default:     50,
				Example:     20,
			},
			{
				Name:        "offset",
				Type:        "integer",
				Description: "Number of results to skip (for pagination)",
				Required:    false,
				Default:     0,
			},
		},
		RequiredParams: []string{},
		Returns: ReturnDoc{
			Description: "Returns a paginated list of demands.",
			Example: map[string]interface{}{
				"demands": []map[string]interface{}{
					{
						"id":         "dmd_a1b2c3d4e5f6...",
						"type":       "feature",
						"title":      "Add dark mode support",
						"status":     "open",
						"priority":   "high",
						"created_at": "2026-02-09T10:00:00Z",
						"updated_at": "2026-02-09T10:00:00Z",
					},
				},
				"total":  1,
				"limit":  50,
				"offset": 0,
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
		},
		Examples: []ExampleDoc{
			{
				Title:       "List open demands",
				Description: "Get all open demands.",
				Input: map[string]interface{}{
					"status": "open",
				},
				Output: map[string]interface{}{
					"demands": []map[string]interface{}{
						{
							"id":       "dmd_a1b2c3d4e5f6...",
							"type":     "feature",
							"title":    "Add dark mode support",
							"status":   "open",
							"priority": "high",
						},
					},
					"total":  1,
					"limit":  50,
					"offset": 0,
				},
			},
			{
				Title:       "Filter by priority and endeavour",
				Description: "Find urgent demands in a specific endeavour.",
				Input: map[string]interface{}{
					"priority":     "urgent",
					"endeavour_id": "edv_bd159eb7bb9a877a...",
				},
				Output: map[string]interface{}{
					"demands": []map[string]interface{}{
						{
							"id":       "dmd_f6e5d4c3b2a1...",
							"type":     "bug",
							"title":    "Login fails on mobile browsers",
							"status":   "open",
							"priority": "urgent",
						},
					},
					"total":  1,
					"limit":  50,
					"offset": 0,
				},
			},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.dmd.create", "ts.dmd.get", "ts.dmd.update"},
		Since:        "v0.2.0",
	})

	r.Register(&ToolDoc{
		Name:     "ts.dmd.update",
		Category: "demand",
		Summary:  "Update demand attributes (partial update).",
		Description: `Updates one or more fields of an existing demand. Only provided fields
are changed; omitted fields remain unchanged.

Demand status transitions follow a defined lifecycle:
- open -> in_progress, fulfilled, canceled
- in_progress -> fulfilled, canceled
- fulfilled (terminal state)
- canceled (terminal state)

When setting status to canceled, provide a canceled_reason.`,
		Parameters: []ParamDoc{
			{
				Name:        "id",
				Type:        "string",
				Description: "Demand ID",
				Required:    true,
				Example:     "dmd_a1b2c3d4e5f6...",
			},
			{
				Name:        "title",
				Type:        "string",
				Description: "New title",
				Required:    false,
			},
			{
				Name:        "description",
				Type:        "string",
				Description: "New description",
				Required:    false,
			},
			{
				Name:        "type",
				Type:        "string",
				Description: "New demand type",
				Required:    false,
			},
			{
				Name:        "status",
				Type:        "string",
				Description: "New status: open, in_progress, fulfilled, canceled",
				Required:    false,
				Example:     "in_progress",
			},
			{
				Name:        "priority",
				Type:        "string",
				Description: "New priority: low, medium, high, urgent",
				Required:    false,
				Example:     "urgent",
			},
			{
				Name:        "endeavour_id",
				Type:        "string",
				Description: "New endeavour (empty string to unlink)",
				Required:    false,
			},
			{
				Name:        "due_date",
				Type:        "string",
				Description: "Due date (ISO 8601, empty string to clear)",
				Required:    false,
			},
			{
				Name:        "canceled_reason",
				Type:        "string",
				Description: "Reason for cancellation (when status=canceled)",
				Required:    false,
			},
			{
				Name:        "metadata",
				Type:        "object",
				Description: "Metadata to set (replaces existing)",
				Required:    false,
			},
		},
		RequiredParams: []string{"id"},
		Returns: ReturnDoc{
			Description: "Returns the demand ID and list of fields that were updated.",
			Example: map[string]interface{}{
				"id":             "dmd_a1b2c3d4e5f6...",
				"updated_fields": []string{"status", "priority"},
				"updated_at":     "2026-02-09T11:00:00Z",
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "not_found", Description: "Demand with this ID does not exist"},
			{Code: "invalid_input", Description: "No fields to update or invalid priority value"},
			{Code: "invalid_transition", Description: "Invalid status transition"},
		},
		Examples: []ExampleDoc{
			{
				Title:       "Start working on a demand",
				Description: "Move a demand from open to in_progress.",
				Input: map[string]interface{}{
					"id":     "dmd_a1b2c3d4e5f6...",
					"status": "in_progress",
				},
				Output: map[string]interface{}{
					"id":             "dmd_a1b2c3d4e5f6...",
					"updated_fields": []string{"status"},
					"updated_at":     "2026-02-09T11:00:00Z",
				},
			},
			{
				Title:       "Fulfill a demand",
				Description: "Mark a demand as fulfilled.",
				Input: map[string]interface{}{
					"id":     "dmd_a1b2c3d4e5f6...",
					"status": "fulfilled",
				},
				Output: map[string]interface{}{
					"id":             "dmd_a1b2c3d4e5f6...",
					"updated_fields": []string{"status"},
					"updated_at":     "2026-02-09T12:00:00Z",
				},
			},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.dmd.create", "ts.dmd.get", "ts.dmd.list", "ts.dmd.cancel"},
		Since:        "v0.2.0",
	})

	r.Register(&ToolDoc{
		Name:     "ts.dmd.cancel",
		Category: "demand",
		Summary:  "Cancel a demand with a reason.",
		Description: `Convenience tool to cancel a demand. Equivalent to calling ts.dmd.update
with status "canceled" and a canceled_reason, but with a simpler interface.

Both id and reason are required.`,
		Parameters: []ParamDoc{
			{
				Name:        "id",
				Type:        "string",
				Description: "Demand ID to cancel",
				Required:    true,
				Example:     "dmd_a1b2c3d4e5f6...",
			},
			{
				Name:        "reason",
				Type:        "string",
				Description: "Reason for cancellation",
				Required:    true,
				Example:     "No longer needed after scope change",
			},
		},
		RequiredParams: []string{"id", "reason"},
		Returns: ReturnDoc{
			Description: "Returns the demand ID and list of fields that were updated.",
			Example: map[string]interface{}{
				"id":             "dmd_a1b2c3d4e5f6...",
				"updated_fields": []string{"status", "canceled_reason"},
				"updated_at":     "2026-02-16T10:00:00Z",
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "not_found", Description: "Demand with this ID does not exist"},
			{Code: "invalid_input", Description: "Demand ID or reason is missing"},
			{Code: "invalid_transition", Description: "Demand cannot be canceled from its current status"},
		},
		Examples: []ExampleDoc{
			{
				Title:       "Cancel a demand",
				Description: "Cancel a demand that is no longer relevant.",
				Input: map[string]interface{}{
					"id":     "dmd_a1b2c3d4e5f6...",
					"reason": "No longer needed after scope change",
				},
				Output: map[string]interface{}{
					"id":             "dmd_a1b2c3d4e5f6...",
					"updated_fields": []string{"status", "canceled_reason"},
					"updated_at":     "2026-02-16T10:00:00Z",
				},
			},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.dmd.update", "ts.dmd.get", "ts.dmd.list"},
		Since:        "v0.8.0",
	})
}

func registerRelationTools(r *Registry) {
	r.Register(&ToolDoc{
		Name:     "ts.rel.create",
		Category: "relation",
		Summary:  "Create a relationship between two entities.",
		Description: `Creates a typed, directed relationship between any two entities in Taskschmiede.

This is the core of the Flexible Relationship Model (FRM). Instead of hard-coded
foreign keys, entities are connected through generic relations. Common relationship
types include:
- belongs_to: task belongs_to endeavour
- assigned_to: task assigned_to resource
- has_member: organization has_member resource
- governs: ritual governs endeavour
- uses: task uses artifact

Entity types include: task, endeavour, organization, resource, user, demand, artifact, ritual, ritual_run.

Relations can carry metadata (e.g., a role on a membership relation).`,
		Parameters: []ParamDoc{
			{
				Name:        "relationship_type",
				Type:        "string",
				Description: "Relationship type (e.g., belongs_to, assigned_to, has_member, governs, uses)",
				Required:    true,
				Example:     "governs",
			},
			{
				Name:        "source_entity_type",
				Type:        "string",
				Description: "Source entity type (e.g., task, organization, user, ritual)",
				Required:    true,
				Example:     "ritual",
			},
			{
				Name:        "source_entity_id",
				Type:        "string",
				Description: "Source entity ID",
				Required:    true,
				Example:     "rtl_a1b2c3d4e5f6...",
			},
			{
				Name:        "target_entity_type",
				Type:        "string",
				Description: "Target entity type (e.g., endeavour, resource, artifact)",
				Required:    true,
				Example:     "endeavour",
			},
			{
				Name:        "target_entity_id",
				Type:        "string",
				Description: "Target entity ID",
				Required:    true,
				Example:     "edv_bd159eb7bb9a877a...",
			},
			{
				Name:        "metadata",
				Type:        "object",
				Description: "Optional metadata on the relationship (e.g., role)",
				Required:    false,
				Example:     map[string]interface{}{"role": "owner"},
			},
		},
		RequiredParams: []string{"relationship_type", "source_entity_type", "source_entity_id", "target_entity_type", "target_entity_id"},
		Returns: ReturnDoc{
			Description: "Returns the created relation.",
			Schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id":                 map[string]interface{}{"type": "string"},
					"relationship_type":  map[string]interface{}{"type": "string"},
					"source_entity_type": map[string]interface{}{"type": "string"},
					"source_entity_id":   map[string]interface{}{"type": "string"},
					"target_entity_type": map[string]interface{}{"type": "string"},
					"target_entity_id":   map[string]interface{}{"type": "string"},
					"metadata":           map[string]interface{}{"type": "object"},
					"created_at":         map[string]interface{}{"type": "string", "format": "date-time"},
				},
			},
			Example: map[string]interface{}{
				"id":                 "rel_x1y2z3...",
				"relationship_type":  "governs",
				"source_entity_type": "ritual",
				"source_entity_id":   "rtl_a1b2c3d4e5f6...",
				"target_entity_type": "endeavour",
				"target_entity_id":   "edv_bd159eb7bb9a877a...",
				"metadata":           map[string]interface{}{},
				"created_at":         "2026-02-09T10:00:00Z",
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "invalid_input", Description: "All five required fields must be provided"},
		},
		Examples: []ExampleDoc{
			{
				Title:       "Link a ritual to an endeavour",
				Description: "Create a governs relation so the ritual applies to the endeavour.",
				Input: map[string]interface{}{
					"relationship_type":  "governs",
					"source_entity_type": "ritual",
					"source_entity_id":   "rtl_a1b2c3d4e5f6...",
					"target_entity_type": "endeavour",
					"target_entity_id":   "edv_bd159eb7bb9a877a...",
				},
				Output: map[string]interface{}{
					"id":                 "rel_x1y2z3...",
					"relationship_type":  "governs",
					"source_entity_type": "ritual",
					"source_entity_id":   "rtl_a1b2c3d4e5f6...",
					"target_entity_type": "endeavour",
					"target_entity_id":   "edv_bd159eb7bb9a877a...",
					"created_at":         "2026-02-09T10:00:00Z",
				},
			},
			{
				Title:       "Link a task to an artifact",
				Description: "Record that a task uses an artifact.",
				Input: map[string]interface{}{
					"relationship_type":  "uses",
					"source_entity_type": "task",
					"source_entity_id":   "tsk_68e9623ade9b1631...",
					"target_entity_type": "artifact",
					"target_entity_id":   "art_d4e5f6a1b2c3...",
				},
				Output: map[string]interface{}{
					"id":                 "rel_a2b3c4...",
					"relationship_type":  "uses",
					"source_entity_type": "task",
					"source_entity_id":   "tsk_68e9623ade9b1631...",
					"target_entity_type": "artifact",
					"target_entity_id":   "art_d4e5f6a1b2c3...",
					"created_at":         "2026-02-09T10:00:00Z",
				},
			},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.rel.list", "ts.rel.delete"},
		Since:        "v0.2.0",
	})

	r.Register(&ToolDoc{
		Name:     "ts.rel.list",
		Category: "relation",
		Summary:  "Query relationships (by source, target, or type).",
		Description: `Lists relationships with optional filtering. You can filter by
source entity, target entity, relationship type, or any combination.

This is how you discover what is connected to what. For example:
- Find all tasks assigned to a resource
- Find all rituals governing an endeavour
- Find all artifacts used by a task`,
		Parameters: []ParamDoc{
			{
				Name:        "source_entity_type",
				Type:        "string",
				Description: "Filter by source entity type",
				Required:    false,
				Example:     "ritual",
			},
			{
				Name:        "source_entity_id",
				Type:        "string",
				Description: "Filter by source entity ID",
				Required:    false,
				Example:     "rtl_a1b2c3d4e5f6...",
			},
			{
				Name:        "target_entity_type",
				Type:        "string",
				Description: "Filter by target entity type",
				Required:    false,
				Example:     "endeavour",
			},
			{
				Name:        "target_entity_id",
				Type:        "string",
				Description: "Filter by target entity ID",
				Required:    false,
				Example:     "edv_bd159eb7bb9a877a...",
			},
			{
				Name:        "relationship_type",
				Type:        "string",
				Description: "Filter by relationship type (e.g., governs, uses, assigned_to)",
				Required:    false,
				Example:     "governs",
			},
			{
				Name:        "limit",
				Type:        "integer",
				Description: "Maximum number of results to return",
				Required:    false,
				Default:     50,
			},
			{
				Name:        "offset",
				Type:        "integer",
				Description: "Number of results to skip (for pagination)",
				Required:    false,
				Default:     0,
			},
		},
		RequiredParams: []string{},
		Returns: ReturnDoc{
			Description: "Returns a paginated list of relations.",
			Example: map[string]interface{}{
				"relations": []map[string]interface{}{
					{
						"id":                 "rel_x1y2z3...",
						"relationship_type":  "governs",
						"source_entity_type": "ritual",
						"source_entity_id":   "rtl_a1b2c3d4e5f6...",
						"target_entity_type": "endeavour",
						"target_entity_id":   "edv_bd159eb7bb9a877a...",
						"created_at":         "2026-02-09T10:00:00Z",
					},
				},
				"total":  1,
				"limit":  50,
				"offset": 0,
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
		},
		Examples: []ExampleDoc{
			{
				Title:       "Find rituals governing an endeavour",
				Description: "List all governs relations targeting a specific endeavour.",
				Input: map[string]interface{}{
					"relationship_type":  "governs",
					"target_entity_type": "endeavour",
					"target_entity_id":   "edv_bd159eb7bb9a877a...",
				},
				Output: map[string]interface{}{
					"relations": []map[string]interface{}{
						{
							"id":                 "rel_x1y2z3...",
							"relationship_type":  "governs",
							"source_entity_type": "ritual",
							"source_entity_id":   "rtl_a1b2c3d4e5f6...",
							"target_entity_type": "endeavour",
							"target_entity_id":   "edv_bd159eb7bb9a877a...",
						},
					},
					"total":  1,
					"limit":  50,
					"offset": 0,
				},
			},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.rel.create", "ts.rel.delete"},
		Since:        "v0.2.0",
	})

	r.Register(&ToolDoc{
		Name:     "ts.rel.delete",
		Category: "relation",
		Summary:  "Remove a relationship.",
		Description: `Deletes a relationship by its ID. This is a hard delete -- the
relationship is permanently removed.

Use ts.rel.list to find the relation ID before deleting.`,
		Parameters: []ParamDoc{
			{
				Name:        "id",
				Type:        "string",
				Description: "The relation ID to delete",
				Required:    true,
				Example:     "rel_x1y2z3...",
			},
		},
		RequiredParams: []string{"id"},
		Returns: ReturnDoc{
			Description: "Returns confirmation of deletion.",
			Example: map[string]interface{}{
				"deleted": true,
				"id":      "rel_x1y2z3...",
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "not_found", Description: "Relation with this ID does not exist"},
		},
		Examples: []ExampleDoc{
			{
				Title:       "Delete a relation",
				Description: "Remove a relationship between two entities.",
				Input: map[string]interface{}{
					"id": "rel_x1y2z3...",
				},
				Output: map[string]interface{}{
					"deleted": true,
					"id":      "rel_x1y2z3...",
				},
			},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.rel.create", "ts.rel.list"},
		Since:        "v0.2.0",
	})
}

func registerArtifactTools(r *Registry) {
	r.Register(&ToolDoc{
		Name:     "ts.art.create",
		Category: "artifact",
		Summary:  "Create a new artifact (reference to external doc, repo, dashboard, etc.).",
		Description: `Creates a new artifact in Taskschmiede.

An artifact is a reference to an external resource -- a document, repository,
link, dashboard, runbook, or any other material relevant to a project. Artifacts
can be linked to an endeavour and/or a specific task.

Artifact kinds: link, doc, repo, file, dataset, dashboard, runbook, other.
Artifacts start in "active" status.`,
		Parameters: []ParamDoc{
			{
				Name:        "kind",
				Type:        "string",
				Description: "Artifact kind: link, doc, repo, file, dataset, dashboard, runbook, other",
				Required:    true,
				Example:     "doc",
			},
			{
				Name:        "title",
				Type:        "string",
				Description: "Artifact title",
				Required:    true,
				Example:     "Architecture Decision Record: FRM Migration",
			},
			{
				Name:        "url",
				Type:        "string",
				Description: "External URL",
				Required:    false,
				Example:     "https://docs.example.com/adr-001",
			},
			{
				Name:        "summary",
				Type:        "string",
				Description: "1-3 line description",
				Required:    false,
				Example:     "Documents the decision to migrate from hard-coded foreign keys to FRM.",
			},
			{
				Name:        "tags",
				Type:        "array",
				Description: "Free-form string tags",
				Required:    false,
				Example:     []string{"architecture", "decision-record"},
			},
			{
				Name:        "endeavour_id",
				Type:        "string",
				Description: "Endeavour this artifact belongs to",
				Required:    false,
				Example:     "edv_bd159eb7bb9a877a...",
			},
			{
				Name:        "task_id",
				Type:        "string",
				Description: "Task this artifact belongs to",
				Required:    false,
			},
			{
				Name:        "metadata",
				Type:        "object",
				Description: "Arbitrary key-value pairs",
				Required:    false,
			},
		},
		RequiredParams: []string{"kind", "title"},
		Returns: ReturnDoc{
			Description: "Returns the created artifact summary.",
			Schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id":         map[string]interface{}{"type": "string"},
					"kind":       map[string]interface{}{"type": "string"},
					"title":      map[string]interface{}{"type": "string"},
					"status":     map[string]interface{}{"type": "string"},
					"created_at": map[string]interface{}{"type": "string", "format": "date-time"},
				},
			},
			Example: map[string]interface{}{
				"id":         "art_d4e5f6a1b2c3...",
				"kind":       "doc",
				"title":      "Architecture Decision Record: FRM Migration",
				"status":     "active",
				"created_at": "2026-02-09T10:00:00Z",
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "invalid_input", Description: "Kind and title are required"},
		},
		Examples: []ExampleDoc{
			{
				Title:       "Create a doc artifact",
				Description: "Register an architecture decision record linked to an endeavour.",
				Input: map[string]interface{}{
					"kind":         "doc",
					"title":        "Architecture Decision Record: FRM Migration",
					"url":          "https://docs.example.com/adr-001",
					"summary":      "Documents the decision to migrate from hard-coded foreign keys to FRM.",
					"tags":         []string{"architecture", "decision-record"},
					"endeavour_id": "edv_bd159eb7bb9a877a...",
				},
				Output: map[string]interface{}{
					"id":         "art_d4e5f6a1b2c3...",
					"kind":       "doc",
					"title":      "Architecture Decision Record: FRM Migration",
					"status":     "active",
					"created_at": "2026-02-09T10:00:00Z",
				},
			},
			{
				Title:       "Create a repo artifact",
				Description: "Track a repository as an artifact.",
				Input: map[string]interface{}{
					"kind":  "repo",
					"title": "Taskschmiede Main Repository",
					"url":   "https://github.com/QuestFinTech/taskschmiede",
					"tags":  []string{"source-code", "go"},
				},
				Output: map[string]interface{}{
					"id":         "art_e5f6a1b2c3d4...",
					"kind":       "repo",
					"title":      "Taskschmiede Main Repository",
					"status":     "active",
					"created_at": "2026-02-09T10:00:00Z",
				},
			},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.art.get", "ts.art.list", "ts.art.update"},
		Since:        "v0.2.0",
	})

	r.Register(&ToolDoc{
		Name:     "ts.art.get",
		Category: "artifact",
		Summary:  "Retrieve an artifact by ID.",
		Description: `Retrieves detailed information about a specific artifact, including
its kind, URL, tags, and any linked endeavour or task.`,
		Parameters: []ParamDoc{
			{
				Name:        "id",
				Type:        "string",
				Description: "Artifact ID",
				Required:    true,
				Example:     "art_d4e5f6a1b2c3...",
			},
		},
		RequiredParams: []string{"id"},
		Returns: ReturnDoc{
			Description: "Returns the full artifact object.",
			Schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id":           map[string]interface{}{"type": "string"},
					"kind":         map[string]interface{}{"type": "string"},
					"title":        map[string]interface{}{"type": "string"},
					"url":          map[string]interface{}{"type": "string"},
					"summary":      map[string]interface{}{"type": "string"},
					"tags":         map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
					"status":       map[string]interface{}{"type": "string"},
					"endeavour_id": map[string]interface{}{"type": "string"},
					"task_id":      map[string]interface{}{"type": "string"},
					"metadata":     map[string]interface{}{"type": "object"},
					"created_at":   map[string]interface{}{"type": "string", "format": "date-time"},
					"updated_at":   map[string]interface{}{"type": "string", "format": "date-time"},
				},
			},
			Example: map[string]interface{}{
				"id":           "art_d4e5f6a1b2c3...",
				"kind":         "doc",
				"title":        "Architecture Decision Record: FRM Migration",
				"url":          "https://docs.example.com/adr-001",
				"summary":      "Documents the decision to migrate from hard-coded foreign keys to FRM.",
				"tags":         []string{"architecture", "decision-record"},
				"status":       "active",
				"endeavour_id": "edv_bd159eb7bb9a877a...",
				"metadata":     map[string]interface{}{},
				"created_at":   "2026-02-09T10:00:00Z",
				"updated_at":   "2026-02-09T10:00:00Z",
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "not_found", Description: "Artifact with this ID does not exist"},
		},
		Examples: []ExampleDoc{
			{
				Title: "Get artifact by ID",
				Input: map[string]interface{}{
					"id": "art_d4e5f6a1b2c3...",
				},
				Output: map[string]interface{}{
					"id":           "art_d4e5f6a1b2c3...",
					"kind":         "doc",
					"title":        "Architecture Decision Record: FRM Migration",
					"status":       "active",
					"endeavour_id": "edv_bd159eb7bb9a877a...",
					"created_at":   "2026-02-09T10:00:00Z",
					"updated_at":   "2026-02-09T10:00:00Z",
				},
			},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.art.create", "ts.art.list", "ts.art.update"},
		Since:        "v0.2.0",
	})

	r.Register(&ToolDoc{
		Name:     "ts.art.list",
		Category: "artifact",
		Summary:  "Query artifacts with filters.",
		Description: `Lists artifacts with optional filtering and pagination.

Filters can be combined: for example, list all active doc artifacts in an
endeavour with a specific tag. Text search matches against title and summary.`,
		Parameters: []ParamDoc{
			{
				Name:        "endeavour_id",
				Type:        "string",
				Description: "Filter by endeavour",
				Required:    false,
				Example:     "edv_bd159eb7bb9a877a...",
			},
			{
				Name:        "task_id",
				Type:        "string",
				Description: "Filter by task",
				Required:    false,
			},
			{
				Name:        "kind",
				Type:        "string",
				Description: "Filter by kind: link, doc, repo, file, dataset, dashboard, runbook, other",
				Required:    false,
				Example:     "doc",
			},
			{
				Name:        "status",
				Type:        "string",
				Description: "Filter by status: active, archived",
				Required:    false,
				Example:     "active",
			},
			{
				Name:        "tags",
				Type:        "string",
				Description: "Filter by tag. Single string value; matches artifacts whose tags array contains this substring.",
				Required:    false,
				Example:     "architecture",
			},
			{
				Name:        "search",
				Type:        "string",
				Description: "Search in title and summary (partial match)",
				Required:    false,
				Example:     "FRM",
			},
			{
				Name:        "limit",
				Type:        "integer",
				Description: "Maximum number of results to return",
				Required:    false,
				Default:     50,
			},
			{
				Name:        "offset",
				Type:        "integer",
				Description: "Number of results to skip (for pagination)",
				Required:    false,
				Default:     0,
			},
		},
		RequiredParams: []string{},
		Returns: ReturnDoc{
			Description: "Returns a paginated list of artifacts.",
			Example: map[string]interface{}{
				"artifacts": []map[string]interface{}{
					{
						"id":         "art_d4e5f6a1b2c3...",
						"kind":       "doc",
						"title":      "Architecture Decision Record: FRM Migration",
						"status":     "active",
						"tags":       []string{"architecture", "decision-record"},
						"created_at": "2026-02-09T10:00:00Z",
						"updated_at": "2026-02-09T10:00:00Z",
					},
				},
				"total":  1,
				"limit":  50,
				"offset": 0,
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
		},
		Examples: []ExampleDoc{
			{
				Title:       "List docs in an endeavour",
				Description: "Find all doc artifacts linked to an endeavour.",
				Input: map[string]interface{}{
					"endeavour_id": "edv_bd159eb7bb9a877a...",
					"kind":         "doc",
				},
				Output: map[string]interface{}{
					"artifacts": []map[string]interface{}{
						{
							"id":    "art_d4e5f6a1b2c3...",
							"kind":  "doc",
							"title": "Architecture Decision Record: FRM Migration",
							"tags":  []string{"architecture"},
						},
					},
					"total":  1,
					"limit":  50,
					"offset": 0,
				},
			},
			{
				Title:       "Search artifacts by tag",
				Description: "Find artifacts with a specific tag.",
				Input: map[string]interface{}{
					"tags": "architecture",
				},
				Output: map[string]interface{}{
					"artifacts": []map[string]interface{}{
						{
							"id":    "art_d4e5f6a1b2c3...",
							"kind":  "doc",
							"title": "Architecture Decision Record: FRM Migration",
							"tags":  []string{"architecture", "decision-record"},
						},
					},
					"total":  1,
					"limit":  50,
					"offset": 0,
				},
			},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.art.create", "ts.art.get", "ts.art.update"},
		Since:        "v0.2.0",
	})

	r.Register(&ToolDoc{
		Name:     "ts.art.update",
		Category: "artifact",
		Summary:  "Update artifact attributes (partial update).",
		Description: `Updates one or more fields of an existing artifact. Only provided fields
are changed; omitted fields remain unchanged.

To archive an artifact, set status to "archived". To unlink from an endeavour
or task, pass an empty string.`,
		Parameters: []ParamDoc{
			{
				Name:        "id",
				Type:        "string",
				Description: "Artifact ID",
				Required:    true,
				Example:     "art_d4e5f6a1b2c3...",
			},
			{
				Name:        "title",
				Type:        "string",
				Description: "New title",
				Required:    false,
			},
			{
				Name:        "kind",
				Type:        "string",
				Description: "New kind",
				Required:    false,
			},
			{
				Name:        "url",
				Type:        "string",
				Description: "New URL",
				Required:    false,
			},
			{
				Name:        "summary",
				Type:        "string",
				Description: "New summary",
				Required:    false,
			},
			{
				Name:        "tags",
				Type:        "array",
				Description: "New tags (replaces existing)",
				Required:    false,
			},
			{
				Name:        "status",
				Type:        "string",
				Description: "New status: active, archived",
				Required:    false,
				Example:     "archived",
			},
			{
				Name:        "endeavour_id",
				Type:        "string",
				Description: "New endeavour (empty string to unlink)",
				Required:    false,
			},
			{
				Name:        "task_id",
				Type:        "string",
				Description: "New task (empty string to unlink)",
				Required:    false,
			},
			{
				Name:        "metadata",
				Type:        "object",
				Description: "Metadata to set (replaces existing)",
				Required:    false,
			},
		},
		RequiredParams: []string{"id"},
		Returns: ReturnDoc{
			Description: "Returns the artifact ID and list of fields that were updated.",
			Example: map[string]interface{}{
				"id":             "art_d4e5f6a1b2c3...",
				"updated_fields": []string{"status"},
				"updated_at":     "2026-02-09T12:00:00Z",
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "not_found", Description: "Artifact with this ID does not exist"},
			{Code: "invalid_input", Description: "No fields to update"},
		},
		Examples: []ExampleDoc{
			{
				Title:       "Archive an artifact",
				Description: "Mark an artifact as archived.",
				Input: map[string]interface{}{
					"id":     "art_d4e5f6a1b2c3...",
					"status": "archived",
				},
				Output: map[string]interface{}{
					"id":             "art_d4e5f6a1b2c3...",
					"updated_fields": []string{"status"},
					"updated_at":     "2026-02-09T12:00:00Z",
				},
			},
			{
				Title:       "Update tags",
				Description: "Replace the tag set on an artifact.",
				Input: map[string]interface{}{
					"id":   "art_d4e5f6a1b2c3...",
					"tags": []string{"architecture", "decision-record", "approved"},
				},
				Output: map[string]interface{}{
					"id":             "art_d4e5f6a1b2c3...",
					"updated_fields": []string{"tags"},
					"updated_at":     "2026-02-09T12:00:00Z",
				},
			},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.art.create", "ts.art.get", "ts.art.list"},
		Since:        "v0.2.0",
	})

	r.Register(&ToolDoc{
		Name:     "ts.art.delete",
		Category: "artifact",
		Summary:  "Delete an artifact (logical delete, sets status to deleted).",
		Description: `Performs a logical delete by setting the artifact's status to 'deleted'.
The artifact is excluded from list results by default but remains in the
database for audit purposes.`,
		Parameters: []ParamDoc{
			{Name: "id", Type: "string", Description: "Artifact ID", Required: true, Example: "art_d4e5f6a1b2c3..."},
		},
		RequiredParams: []string{"id"},
		Returns: ReturnDoc{
			Description: "Returns confirmation of deletion.",
			Example: map[string]interface{}{
				"id":      "art_d4e5f6a1b2c3...",
				"deleted": true,
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "not_found", Description: "Artifact not found"},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.art.create", "ts.art.get", "ts.art.list", "ts.art.update"},
		Since:        "v0.3.7",
	})
}

func registerRitualTools(r *Registry) {
	r.Register(&ToolDoc{
		Name:     "ts.rtl.create",
		Category: "ritual",
		Summary:  "Create a new ritual (stored methodology prompt).",
		Description: `Creates a new ritual in Taskschmiede.

A ritual is a stored methodology prompt -- the core of BYOM (Bring Your Own
Methodology). Rituals define recurring processes like standups, sprint planning,
retrospectives, or any methodology-specific ceremony.

The prompt field contains the methodology instructions in free-form text. This is
what gets executed during a ritual run. Rituals can be linked to an endeavour via
a governs relationship and can have a schedule (informational only -- the agent
or orchestrator is responsible for triggering runs).

Rituals have an origin field: "custom" for user-created, "template" for built-in
templates, "fork" for rituals derived from another.`,
		Parameters: []ParamDoc{
			{
				Name:        "name",
				Type:        "string",
				Description: "Ritual name (e.g., 'Weekly planning (Shape Up)')",
				Required:    true,
				Example:     "Daily standup",
			},
			{
				Name:        "prompt",
				Type:        "string",
				Description: "The methodology prompt (free-form text, BYOM core)",
				Required:    true,
				Example:     "Review yesterday's progress, identify blockers, plan today's work. Focus on tasks in the current endeavour.",
			},
			{
				Name:        "description",
				Type:        "string",
				Description: "Longer explanation of the ritual",
				Required:    false,
				Example:     "A brief daily check-in to align the team on progress and blockers.",
			},
			{
				Name:        "endeavour_id",
				Type:        "string",
				Description: "Endeavour this ritual governs (creates a governs relation)",
				Required:    false,
				Example:     "edv_bd159eb7bb9a877a...",
			},
			{
				Name:        "schedule",
				Type:        "object",
				Description: "Schedule metadata: {\"type\":\"cron|interval|manual\", ...} (informational only)",
				Required:    false,
				Example:     map[string]interface{}{"type": "cron", "expression": "0 9 * * 1-5"},
			},
			{
				Name:        "metadata",
				Type:        "object",
				Description: "Arbitrary key-value pairs",
				Required:    false,
			},
		},
		RequiredParams: []string{"name", "prompt"},
		Returns: ReturnDoc{
			Description: "Returns the created ritual summary.",
			Schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id":         map[string]interface{}{"type": "string"},
					"name":       map[string]interface{}{"type": "string"},
					"origin":     map[string]interface{}{"type": "string"},
					"is_enabled": map[string]interface{}{"type": "boolean"},
					"status":     map[string]interface{}{"type": "string"},
					"created_at": map[string]interface{}{"type": "string", "format": "date-time"},
				},
			},
			Example: map[string]interface{}{
				"id":         "rtl_a1b2c3d4e5f6...",
				"name":       "Daily standup",
				"origin":     "custom",
				"is_enabled": true,
				"status":     "active",
				"created_at": "2026-02-09T10:00:00Z",
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "invalid_input", Description: "Name and prompt are required"},
		},
		Examples: []ExampleDoc{
			{
				Title:       "Create a daily standup ritual",
				Description: "Create a ritual for daily standups linked to an endeavour.",
				Input: map[string]interface{}{
					"name":         "Daily standup",
					"prompt":       "Review yesterday's progress, identify blockers, plan today's work.",
					"description":  "A brief daily check-in to align the team on progress and blockers.",
					"endeavour_id": "edv_bd159eb7bb9a877a...",
					"schedule":     map[string]interface{}{"type": "cron", "expression": "0 9 * * 1-5"},
				},
				Output: map[string]interface{}{
					"id":         "rtl_a1b2c3d4e5f6...",
					"name":       "Daily standup",
					"origin":     "custom",
					"is_enabled": true,
					"status":     "active",
					"created_at": "2026-02-09T10:00:00Z",
				},
			},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.rtl.get", "ts.rtl.list", "ts.rtl.update", "ts.rtl.fork", "ts.rtr.create"},
		Since:        "v0.2.0",
	})

	r.Register(&ToolDoc{
		Name:     "ts.rtl.get",
		Category: "ritual",
		Summary:  "Retrieve a ritual by ID.",
		Description: `Retrieves detailed information about a specific ritual, including
its prompt, schedule, origin, lineage (predecessor_id), and enabled state.`,
		Parameters: []ParamDoc{
			{
				Name:        "id",
				Type:        "string",
				Description: "Ritual ID",
				Required:    true,
				Example:     "rtl_a1b2c3d4e5f6...",
			},
		},
		RequiredParams: []string{"id"},
		Returns: ReturnDoc{
			Description: "Returns the full ritual object.",
			Schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id":             map[string]interface{}{"type": "string"},
					"name":           map[string]interface{}{"type": "string"},
					"description":    map[string]interface{}{"type": "string"},
					"prompt":         map[string]interface{}{"type": "string"},
					"origin":         map[string]interface{}{"type": "string"},
					"predecessor_id": map[string]interface{}{"type": "string"},
					"is_enabled":     map[string]interface{}{"type": "boolean"},
					"status":         map[string]interface{}{"type": "string"},
					"schedule":       map[string]interface{}{"type": "object"},
					"metadata":       map[string]interface{}{"type": "object"},
					"created_at":     map[string]interface{}{"type": "string", "format": "date-time"},
					"updated_at":     map[string]interface{}{"type": "string", "format": "date-time"},
				},
			},
			Example: map[string]interface{}{
				"id":          "rtl_a1b2c3d4e5f6...",
				"name":        "Daily standup",
				"description": "A brief daily check-in to align the team.",
				"prompt":      "Review yesterday's progress, identify blockers, plan today's work.",
				"origin":      "custom",
				"is_enabled":  true,
				"status":      "active",
				"schedule":    map[string]interface{}{"type": "cron", "expression": "0 9 * * 1-5"},
				"metadata":    map[string]interface{}{},
				"created_at":  "2026-02-09T10:00:00Z",
				"updated_at":  "2026-02-09T10:00:00Z",
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "not_found", Description: "Ritual with this ID does not exist"},
		},
		Examples: []ExampleDoc{
			{
				Title: "Get ritual by ID",
				Input: map[string]interface{}{
					"id": "rtl_a1b2c3d4e5f6...",
				},
				Output: map[string]interface{}{
					"id":         "rtl_a1b2c3d4e5f6...",
					"name":       "Daily standup",
					"prompt":     "Review yesterday's progress, identify blockers, plan today's work.",
					"origin":     "custom",
					"is_enabled": true,
					"status":     "active",
					"created_at": "2026-02-09T10:00:00Z",
				},
			},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.rtl.create", "ts.rtl.list", "ts.rtl.update", "ts.rtl.fork", "ts.rtl.lineage"},
		Since:        "v0.2.0",
	})

	r.Register(&ToolDoc{
		Name:     "ts.rtl.list",
		Category: "ritual",
		Summary:  "Query rituals with filters.",
		Description: `Lists rituals with optional filtering and pagination.

Filter by endeavour to find rituals governing a specific project. Filter by
origin to distinguish templates from custom rituals and forks.`,
		Parameters: []ParamDoc{
			{
				Name:        "endeavour_id",
				Type:        "string",
				Description: "Filter by endeavour (via governs relationship)",
				Required:    false,
				Example:     "edv_bd159eb7bb9a877a...",
			},
			{
				Name:        "is_enabled",
				Type:        "boolean",
				Description: "Filter by enabled/disabled",
				Required:    false,
				Example:     true,
			},
			{
				Name:        "status",
				Type:        "string",
				Description: "Filter by status: active, archived",
				Required:    false,
				Example:     "active",
			},
			{
				Name:        "origin",
				Type:        "string",
				Description: "Filter by origin: template, custom, fork",
				Required:    false,
				Example:     "template",
			},
			{
				Name:        "search",
				Type:        "string",
				Description: "Search in name and description (partial match)",
				Required:    false,
				Example:     "standup",
			},
			{
				Name:        "limit",
				Type:        "integer",
				Description: "Maximum number of results to return",
				Required:    false,
				Default:     50,
			},
			{
				Name:        "offset",
				Type:        "integer",
				Description: "Number of results to skip (for pagination)",
				Required:    false,
				Default:     0,
			},
		},
		RequiredParams: []string{},
		Returns: ReturnDoc{
			Description: "Returns a paginated list of rituals.",
			Example: map[string]interface{}{
				"rituals": []map[string]interface{}{
					{
						"id":         "rtl_a1b2c3d4e5f6...",
						"name":       "Daily standup",
						"origin":     "custom",
						"is_enabled": true,
						"status":     "active",
						"created_at": "2026-02-09T10:00:00Z",
						"updated_at": "2026-02-09T10:00:00Z",
					},
				},
				"total":  1,
				"limit":  50,
				"offset": 0,
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
		},
		Examples: []ExampleDoc{
			{
				Title:       "List active rituals for an endeavour",
				Description: "Find all enabled rituals governing a specific endeavour.",
				Input: map[string]interface{}{
					"endeavour_id": "edv_bd159eb7bb9a877a...",
					"is_enabled":   true,
				},
				Output: map[string]interface{}{
					"rituals": []map[string]interface{}{
						{
							"id":         "rtl_a1b2c3d4e5f6...",
							"name":       "Daily standup",
							"origin":     "custom",
							"is_enabled": true,
						},
					},
					"total":  1,
					"limit":  50,
					"offset": 0,
				},
			},
			{
				Title:       "List template rituals",
				Description: "Find all built-in methodology templates.",
				Input: map[string]interface{}{
					"origin": "template",
				},
				Output: map[string]interface{}{
					"rituals": []map[string]interface{}{},
					"total":   0,
					"limit":   50,
					"offset":  0,
				},
			},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.rtl.create", "ts.rtl.get", "ts.rtl.update", "ts.rtl.fork"},
		Since:        "v0.2.0",
	})

	r.Register(&ToolDoc{
		Name:     "ts.rtl.update",
		Category: "ritual",
		Summary:  "Update ritual attributes (cannot change prompt -- fork instead).",
		Description: `Updates one or more fields of an existing ritual. Only provided fields
are changed; omitted fields remain unchanged.

The prompt field cannot be updated directly. To change a ritual's prompt,
use ts.rtl.fork to create a new version with the modified prompt. This
preserves the lineage and audit trail of methodology changes.`,
		Parameters: []ParamDoc{
			{
				Name:        "id",
				Type:        "string",
				Description: "Ritual ID",
				Required:    true,
				Example:     "rtl_a1b2c3d4e5f6...",
			},
			{
				Name:        "name",
				Type:        "string",
				Description: "New name",
				Required:    false,
			},
			{
				Name:        "description",
				Type:        "string",
				Description: "New description",
				Required:    false,
			},
			{
				Name:        "schedule",
				Type:        "object",
				Description: "New schedule metadata",
				Required:    false,
			},
			{
				Name:        "is_enabled",
				Type:        "boolean",
				Description: "Enable or disable the ritual",
				Required:    false,
				Example:     false,
			},
			{
				Name:        "status",
				Type:        "string",
				Description: "New status: active, archived",
				Required:    false,
			},
			{
				Name:        "endeavour_id",
				Type:        "string",
				Description: "New endeavour (empty string to unlink)",
				Required:    false,
			},
			{
				Name:        "metadata",
				Type:        "object",
				Description: "Metadata to set (replaces existing)",
				Required:    false,
			},
		},
		RequiredParams: []string{"id"},
		Returns: ReturnDoc{
			Description: "Returns the ritual ID and list of fields that were updated.",
			Example: map[string]interface{}{
				"id":             "rtl_a1b2c3d4e5f6...",
				"updated_fields": []string{"is_enabled"},
				"updated_at":     "2026-02-09T12:00:00Z",
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "not_found", Description: "Ritual with this ID does not exist"},
			{Code: "invalid_input", Description: "No fields to update, or attempted to change prompt"},
		},
		Examples: []ExampleDoc{
			{
				Title:       "Disable a ritual",
				Description: "Temporarily disable a ritual without deleting it.",
				Input: map[string]interface{}{
					"id":         "rtl_a1b2c3d4e5f6...",
					"is_enabled": false,
				},
				Output: map[string]interface{}{
					"id":             "rtl_a1b2c3d4e5f6...",
					"updated_fields": []string{"is_enabled"},
					"updated_at":     "2026-02-09T12:00:00Z",
				},
			},
			{
				Title:       "Update schedule",
				Description: "Change the schedule for a ritual.",
				Input: map[string]interface{}{
					"id":       "rtl_a1b2c3d4e5f6...",
					"schedule": map[string]interface{}{"type": "cron", "expression": "0 10 * * 1"},
				},
				Output: map[string]interface{}{
					"id":             "rtl_a1b2c3d4e5f6...",
					"updated_fields": []string{"schedule"},
					"updated_at":     "2026-02-09T12:00:00Z",
				},
			},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.rtl.create", "ts.rtl.get", "ts.rtl.fork"},
		Since:        "v0.2.0",
	})

	r.Register(&ToolDoc{
		Name:     "ts.rtl.fork",
		Category: "ritual",
		Summary:  "Fork a ritual (create a new ritual derived from an existing one).",
		Description: `Creates a new ritual derived from an existing one. This is how you
evolve methodology prompts while preserving history.

The forked ritual:
- Gets origin="fork" and predecessor_id pointing to the source
- Inherits name, prompt, description, and schedule from the source (unless overridden)
- Is a fully independent ritual that can be modified separately
- Can optionally be linked to a different endeavour

Use ts.rtl.lineage to trace the full version chain.`,
		Parameters: []ParamDoc{
			{
				Name:        "source_id",
				Type:        "string",
				Description: "The ritual to fork from",
				Required:    true,
				Example:     "rtl_a1b2c3d4e5f6...",
			},
			{
				Name:        "name",
				Type:        "string",
				Description: "Name for the fork (defaults to source name)",
				Required:    false,
				Example:     "Daily standup v2",
			},
			{
				Name:        "prompt",
				Type:        "string",
				Description: "Modified prompt (defaults to source prompt)",
				Required:    false,
				Example:     "Review progress, blockers, and today's plan. Include metrics from the dashboard.",
			},
			{
				Name:        "description",
				Type:        "string",
				Description: "Description (defaults to source description)",
				Required:    false,
			},
			{
				Name:        "endeavour_id",
				Type:        "string",
				Description: "Endeavour the forked ritual governs",
				Required:    false,
			},
			{
				Name:        "schedule",
				Type:        "object",
				Description: "Schedule metadata (defaults to source schedule)",
				Required:    false,
			},
			{
				Name:        "metadata",
				Type:        "object",
				Description: "Arbitrary key-value pairs",
				Required:    false,
			},
		},
		RequiredParams: []string{"source_id"},
		Returns: ReturnDoc{
			Description: "Returns the forked ritual summary.",
			Schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id":             map[string]interface{}{"type": "string"},
					"name":           map[string]interface{}{"type": "string"},
					"origin":         map[string]interface{}{"type": "string"},
					"predecessor_id": map[string]interface{}{"type": "string"},
					"is_enabled":     map[string]interface{}{"type": "boolean"},
					"status":         map[string]interface{}{"type": "string"},
					"created_at":     map[string]interface{}{"type": "string", "format": "date-time"},
				},
			},
			Example: map[string]interface{}{
				"id":             "rtl_b2c3d4e5f6a1...",
				"name":           "Daily standup v2",
				"origin":         "fork",
				"predecessor_id": "rtl_a1b2c3d4e5f6...",
				"is_enabled":     true,
				"status":         "active",
				"created_at":     "2026-02-09T12:00:00Z",
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "not_found", Description: "Source ritual does not exist"},
		},
		Examples: []ExampleDoc{
			{
				Title:       "Fork with modified prompt",
				Description: "Create a new version of a ritual with an updated prompt.",
				Input: map[string]interface{}{
					"source_id": "rtl_a1b2c3d4e5f6...",
					"name":      "Daily standup v2",
					"prompt":    "Review progress, blockers, and today's plan. Include metrics from the dashboard.",
				},
				Output: map[string]interface{}{
					"id":             "rtl_b2c3d4e5f6a1...",
					"name":           "Daily standup v2",
					"origin":         "fork",
					"predecessor_id": "rtl_a1b2c3d4e5f6...",
					"is_enabled":     true,
					"status":         "active",
					"created_at":     "2026-02-09T12:00:00Z",
				},
			},
			{
				Title:       "Fork for a different endeavour",
				Description: "Fork a template ritual for a specific project.",
				Input: map[string]interface{}{
					"source_id":    "rtl_template_standup...",
					"endeavour_id": "edv_newproject...",
				},
				Output: map[string]interface{}{
					"id":             "rtl_c3d4e5f6a1b2...",
					"name":           "Daily standup",
					"origin":         "fork",
					"predecessor_id": "rtl_template_standup...",
					"is_enabled":     true,
					"status":         "active",
					"created_at":     "2026-02-09T12:00:00Z",
				},
			},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.rtl.create", "ts.rtl.get", "ts.rtl.lineage"},
		Since:        "v0.2.0",
	})

	r.Register(&ToolDoc{
		Name:     "ts.rtl.lineage",
		Category: "ritual",
		Summary:  "Walk the version chain for a ritual (oldest to newest).",
		Description: `Traces the lineage of a ritual by walking the predecessor chain.

Returns an ordered list of rituals from the oldest ancestor to the given
ritual. This shows how a methodology prompt has evolved over time through forks.`,
		Parameters: []ParamDoc{
			{
				Name:        "id",
				Type:        "string",
				Description: "Ritual ID to trace lineage from",
				Required:    true,
				Example:     "rtl_b2c3d4e5f6a1...",
			},
		},
		RequiredParams: []string{"id"},
		Returns: ReturnDoc{
			Description: "Returns the lineage chain as an ordered list (oldest first).",
			Schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"lineage": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"id":         map[string]interface{}{"type": "string"},
								"name":       map[string]interface{}{"type": "string"},
								"origin":     map[string]interface{}{"type": "string"},
								"created_at": map[string]interface{}{"type": "string", "format": "date-time"},
							},
						},
					},
				},
			},
			Example: map[string]interface{}{
				"lineage": []map[string]interface{}{
					{
						"id":         "rtl_a1b2c3d4e5f6...",
						"name":       "Daily standup",
						"origin":     "custom",
						"created_at": "2026-02-09T10:00:00Z",
					},
					{
						"id":         "rtl_b2c3d4e5f6a1...",
						"name":       "Daily standup v2",
						"origin":     "fork",
						"created_at": "2026-02-09T12:00:00Z",
					},
				},
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "not_found", Description: "Ritual with this ID does not exist"},
		},
		Examples: []ExampleDoc{
			{
				Title:       "Trace ritual lineage",
				Description: "See the full version history of a forked ritual.",
				Input: map[string]interface{}{
					"id": "rtl_b2c3d4e5f6a1...",
				},
				Output: map[string]interface{}{
					"lineage": []map[string]interface{}{
						{
							"id":     "rtl_a1b2c3d4e5f6...",
							"name":   "Daily standup",
							"origin": "custom",
						},
						{
							"id":     "rtl_b2c3d4e5f6a1...",
							"name":   "Daily standup v2",
							"origin": "fork",
						},
					},
				},
			},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.rtl.get", "ts.rtl.fork"},
		Since:        "v0.2.0",
	})
}

func registerRitualRunTools(r *Registry) {
	r.Register(&ToolDoc{
		Name:     "ts.rtr.create",
		Category: "ritual_run",
		Summary:  "Create a ritual run (marks execution start, status=running).",
		Description: `Creates a new ritual run to track execution of a ritual.

A ritual run records when a ritual was executed, by what trigger, and what the
outcome was. New runs start in "running" status with started_at set automatically.

Triggers:
- manual: Agent or human explicitly started the run
- schedule: Triggered by a scheduled event
- api: Triggered by an external API call`,
		Parameters: []ParamDoc{
			{
				Name:        "ritual_id",
				Type:        "string",
				Description: "Ritual ID to execute",
				Required:    true,
				Example:     "rtl_a1b2c3d4e5f6...",
			},
			{
				Name:        "trigger",
				Type:        "string",
				Description: "What triggered the run: schedule, manual (default), api",
				Required:    false,
				Default:     "manual",
				Example:     "manual",
			},
			{
				Name:        "metadata",
				Type:        "object",
				Description: "Arbitrary key-value pairs",
				Required:    false,
			},
		},
		RequiredParams: []string{"ritual_id"},
		Returns: ReturnDoc{
			Description: "Returns the created ritual run.",
			Schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id":         map[string]interface{}{"type": "string"},
					"ritual_id":  map[string]interface{}{"type": "string"},
					"status":     map[string]interface{}{"type": "string"},
					"trigger":    map[string]interface{}{"type": "string"},
					"started_at": map[string]interface{}{"type": "string", "format": "date-time"},
					"created_at": map[string]interface{}{"type": "string", "format": "date-time"},
				},
			},
			Example: map[string]interface{}{
				"id":         "rtr_x1y2z3a4b5c6...",
				"ritual_id":  "rtl_a1b2c3d4e5f6...",
				"status":     "running",
				"trigger":    "manual",
				"started_at": "2026-02-09T10:00:00Z",
				"created_at": "2026-02-09T10:00:00Z",
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "invalid_input", Description: "ritual_id is required"},
			{Code: "not_found", Description: "Ritual with this ID does not exist"},
		},
		Examples: []ExampleDoc{
			{
				Title:       "Start a manual ritual run",
				Description: "Begin executing a ritual triggered by an agent.",
				Input: map[string]interface{}{
					"ritual_id": "rtl_a1b2c3d4e5f6...",
					"trigger":   "manual",
				},
				Output: map[string]interface{}{
					"id":         "rtr_x1y2z3a4b5c6...",
					"ritual_id":  "rtl_a1b2c3d4e5f6...",
					"status":     "running",
					"trigger":    "manual",
					"started_at": "2026-02-09T10:00:00Z",
				},
			},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.rtr.get", "ts.rtr.list", "ts.rtr.update", "ts.rtl.get"},
		Since:        "v0.2.0",
	})

	r.Register(&ToolDoc{
		Name:     "ts.rtr.get",
		Category: "ritual_run",
		Summary:  "Retrieve a ritual run by ID.",
		Description: `Retrieves detailed information about a specific ritual run, including
its status, trigger, result summary, effects, error details, and timing.`,
		Parameters: []ParamDoc{
			{
				Name:        "id",
				Type:        "string",
				Description: "Ritual run ID",
				Required:    true,
				Example:     "rtr_x1y2z3a4b5c6...",
			},
		},
		RequiredParams: []string{"id"},
		Returns: ReturnDoc{
			Description: "Returns the full ritual run object.",
			Schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id":             map[string]interface{}{"type": "string"},
					"ritual_id":      map[string]interface{}{"type": "string"},
					"status":         map[string]interface{}{"type": "string"},
					"trigger":        map[string]interface{}{"type": "string"},
					"result_summary": map[string]interface{}{"type": "string"},
					"effects":        map[string]interface{}{"type": "object"},
					"error":          map[string]interface{}{"type": "object"},
					"started_at":     map[string]interface{}{"type": "string", "format": "date-time"},
					"finished_at":    map[string]interface{}{"type": "string", "format": "date-time"},
					"metadata":       map[string]interface{}{"type": "object"},
					"created_at":     map[string]interface{}{"type": "string", "format": "date-time"},
					"updated_at":     map[string]interface{}{"type": "string", "format": "date-time"},
				},
			},
			Example: map[string]interface{}{
				"id":             "rtr_x1y2z3a4b5c6...",
				"ritual_id":      "rtl_a1b2c3d4e5f6...",
				"status":         "succeeded",
				"trigger":        "manual",
				"result_summary": "Created 2 tasks, updated 1 task status.",
				"effects": map[string]interface{}{
					"tasks_created": []string{"tsk_new1...", "tsk_new2..."},
					"tasks_updated": []string{"tsk_existing..."},
				},
				"started_at":  "2026-02-09T10:00:00Z",
				"finished_at": "2026-02-09T10:05:00Z",
				"metadata":    map[string]interface{}{},
				"created_at":  "2026-02-09T10:00:00Z",
				"updated_at":  "2026-02-09T10:05:00Z",
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "not_found", Description: "Ritual run with this ID does not exist"},
		},
		Examples: []ExampleDoc{
			{
				Title: "Get ritual run by ID",
				Input: map[string]interface{}{
					"id": "rtr_x1y2z3a4b5c6...",
				},
				Output: map[string]interface{}{
					"id":             "rtr_x1y2z3a4b5c6...",
					"ritual_id":      "rtl_a1b2c3d4e5f6...",
					"status":         "succeeded",
					"trigger":        "manual",
					"result_summary": "Created 2 tasks, updated 1 task status.",
					"started_at":     "2026-02-09T10:00:00Z",
					"finished_at":    "2026-02-09T10:05:00Z",
				},
			},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.rtr.create", "ts.rtr.list", "ts.rtr.update"},
		Since:        "v0.2.0",
	})

	r.Register(&ToolDoc{
		Name:     "ts.rtr.list",
		Category: "ritual_run",
		Summary:  "Query ritual runs with filters.",
		Description: `Lists ritual runs with optional filtering and pagination.

Filter by ritual to see all executions of a specific ritual, or by status
to find running or failed runs.`,
		Parameters: []ParamDoc{
			{
				Name:        "ritual_id",
				Type:        "string",
				Description: "Filter by ritual",
				Required:    false,
				Example:     "rtl_a1b2c3d4e5f6...",
			},
			{
				Name:        "status",
				Type:        "string",
				Description: "Filter by status: running, succeeded, failed, skipped",
				Required:    false,
				Example:     "succeeded",
			},
			{
				Name:        "limit",
				Type:        "integer",
				Description: "Maximum number of results to return",
				Required:    false,
				Default:     50,
			},
			{
				Name:        "offset",
				Type:        "integer",
				Description: "Number of results to skip (for pagination)",
				Required:    false,
				Default:     0,
			},
		},
		RequiredParams: []string{},
		Returns: ReturnDoc{
			Description: "Returns a paginated list of ritual runs.",
			Example: map[string]interface{}{
				"runs": []map[string]interface{}{
					{
						"id":          "rtr_x1y2z3a4b5c6...",
						"ritual_id":   "rtl_a1b2c3d4e5f6...",
						"status":      "succeeded",
						"trigger":     "manual",
						"started_at":  "2026-02-09T10:00:00Z",
						"finished_at": "2026-02-09T10:05:00Z",
					},
				},
				"total":  1,
				"limit":  50,
				"offset": 0,
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
		},
		Examples: []ExampleDoc{
			{
				Title:       "List runs for a ritual",
				Description: "See all executions of a specific ritual.",
				Input: map[string]interface{}{
					"ritual_id": "rtl_a1b2c3d4e5f6...",
				},
				Output: map[string]interface{}{
					"runs": []map[string]interface{}{
						{
							"id":          "rtr_x1y2z3a4b5c6...",
							"ritual_id":   "rtl_a1b2c3d4e5f6...",
							"status":      "succeeded",
							"trigger":     "manual",
							"started_at":  "2026-02-09T10:00:00Z",
							"finished_at": "2026-02-09T10:05:00Z",
						},
					},
					"total":  1,
					"limit":  50,
					"offset": 0,
				},
			},
			{
				Title:       "List failed runs",
				Description: "Find ritual runs that failed.",
				Input: map[string]interface{}{
					"status": "failed",
				},
				Output: map[string]interface{}{
					"runs":   []map[string]interface{}{},
					"total":  0,
					"limit":  50,
					"offset": 0,
				},
			},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.rtr.create", "ts.rtr.get", "ts.rtr.update"},
		Since:        "v0.2.0",
	})

	r.Register(&ToolDoc{
		Name:     "ts.rtr.update",
		Category: "ritual_run",
		Summary:  "Update a ritual run (status, results, effects, error).",
		Description: `Updates a ritual run to record its outcome. This is how you complete a run.

Status transitions:
- running -> succeeded (work completed successfully)
- running -> failed (work encountered an error)
- running -> skipped (run was skipped, e.g., nothing to do)

When transitioning to a terminal status, finished_at is set automatically.

The effects field records what the run produced (tasks created, updated, etc.).
The error field captures failure details when status=failed.`,
		Parameters: []ParamDoc{
			{
				Name:        "id",
				Type:        "string",
				Description: "Ritual run ID",
				Required:    true,
				Example:     "rtr_x1y2z3a4b5c6...",
			},
			{
				Name:        "status",
				Type:        "string",
				Description: "New status: succeeded, failed, skipped",
				Required:    false,
				Example:     "succeeded",
			},
			{
				Name:        "result_summary",
				Type:        "string",
				Description: "Free-form summary of what happened",
				Required:    false,
				Example:     "Created 2 tasks, updated 1 task status.",
			},
			{
				Name:        "effects",
				Type:        "object",
				Description: "Effects of the run: {\"tasks_created\":[], \"tasks_updated\":[], ...}",
				Required:    false,
				Example: map[string]interface{}{
					"tasks_created": []string{"tsk_new1...", "tsk_new2..."},
					"tasks_updated": []string{"tsk_existing..."},
				},
			},
			{
				Name:        "error",
				Type:        "object",
				Description: "Error details if failed: {\"code\":\"...\", \"message\":\"...\"}",
				Required:    false,
				Example: map[string]interface{}{
					"code":    "timeout",
					"message": "Ritual execution timed out after 5 minutes.",
				},
			},
			{
				Name:        "metadata",
				Type:        "object",
				Description: "Metadata to set (replaces existing)",
				Required:    false,
			},
		},
		RequiredParams: []string{"id"},
		Returns: ReturnDoc{
			Description: "Returns the run ID and list of fields that were updated.",
			Example: map[string]interface{}{
				"id":             "rtr_x1y2z3a4b5c6...",
				"updated_fields": []string{"status", "result_summary", "effects"},
				"updated_at":     "2026-02-09T10:05:00Z",
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "not_found", Description: "Ritual run with this ID does not exist"},
			{Code: "invalid_input", Description: "No fields to update or invalid status transition"},
		},
		Examples: []ExampleDoc{
			{
				Title:       "Complete a successful run",
				Description: "Mark a ritual run as succeeded with results.",
				Input: map[string]interface{}{
					"id":             "rtr_x1y2z3a4b5c6...",
					"status":         "succeeded",
					"result_summary": "Created 2 tasks, updated 1 task status.",
					"effects": map[string]interface{}{
						"tasks_created": []string{"tsk_new1...", "tsk_new2..."},
						"tasks_updated": []string{"tsk_existing..."},
					},
				},
				Output: map[string]interface{}{
					"id":             "rtr_x1y2z3a4b5c6...",
					"updated_fields": []string{"status", "result_summary", "effects"},
					"updated_at":     "2026-02-09T10:05:00Z",
				},
			},
			{
				Title:       "Record a failed run",
				Description: "Mark a ritual run as failed with error details.",
				Input: map[string]interface{}{
					"id":     "rtr_x1y2z3a4b5c6...",
					"status": "failed",
					"error": map[string]interface{}{
						"code":    "timeout",
						"message": "Ritual execution timed out after 5 minutes.",
					},
				},
				Output: map[string]interface{}{
					"id":             "rtr_x1y2z3a4b5c6...",
					"updated_fields": []string{"status", "error"},
					"updated_at":     "2026-02-09T10:05:00Z",
				},
			},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.rtr.create", "ts.rtr.get", "ts.rtr.list"},
		Since:        "v0.2.0",
	})
}

func registerTemplateTools(r *Registry) {
	r.Register(&ToolDoc{
		Name:     "ts.tpl.create",
		Category: "template",
		Summary:  "Create a report template.",
		Description: `Creates a new report template with a name, scope, and body.
Templates define the structure for generated reports.`,
		Parameters: []ParamDoc{
			{Name: "name", Type: "string", Description: "Template name", Required: true, Example: "Sprint Summary"},
			{Name: "scope", Type: "string", Description: "Template scope: task, demand, endeavour", Required: true, Example: "endeavour"},
			{Name: "body", Type: "string", Description: "Template body (Markdown with placeholders)", Required: true, Example: "# {{.Name}} Report\n\n{{.Summary}}"},
			{Name: "lang", Type: "string", Description: "Language code", Required: false, Example: "en"},
			{Name: "metadata", Type: "object", Description: "Arbitrary key-value pairs", Required: false},
		},
		RequiredParams: []string{"name", "scope", "body"},
		Returns: ReturnDoc{
			Description: "Returns the created template.",
			Example: map[string]interface{}{
				"id":         "tpl_a1b2c3d4e5f6...",
				"name":       "Sprint Summary",
				"scope":      "endeavour",
				"status":     "active",
				"created_at": "2026-03-07T10:00:00Z",
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "invalid_input", Description: "Name, scope, and body are required"},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.tpl.get", "ts.tpl.list", "ts.tpl.update", "ts.tpl.fork"},
		Since:        "v0.3.7",
	})

	r.Register(&ToolDoc{
		Name:     "ts.tpl.get",
		Category: "template",
		Summary:  "Retrieve a template by ID.",
		Description: `Retrieves detailed information about a specific report template.`,
		Parameters: []ParamDoc{
			{Name: "id", Type: "string", Description: "Template ID", Required: true, Example: "tpl_a1b2c3d4e5f6..."},
		},
		RequiredParams: []string{"id"},
		Returns: ReturnDoc{
			Description: "Returns the full template object.",
			Example: map[string]interface{}{
				"id":         "tpl_a1b2c3d4e5f6...",
				"name":       "Sprint Summary",
				"scope":      "endeavour",
				"body":       "# {{.Name}} Report\n\n{{.Summary}}",
				"status":     "active",
				"created_at": "2026-03-07T10:00:00Z",
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "not_found", Description: "Template not found"},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.tpl.list", "ts.tpl.update", "ts.tpl.fork"},
		Since:        "v0.3.7",
	})

	r.Register(&ToolDoc{
		Name:     "ts.tpl.list",
		Category: "template",
		Summary:  "Query templates with filters.",
		Description: `Lists report templates with optional filtering and pagination.`,
		Parameters: []ParamDoc{
			{Name: "scope", Type: "string", Description: "Filter by scope: task, demand, endeavour", Required: false, Example: "endeavour"},
			{Name: "lang", Type: "string", Description: "Filter by language code", Required: false, Example: "en"},
			{Name: "status", Type: "string", Description: "Filter by status", Required: false, Example: "active"},
			{Name: "search", Type: "string", Description: "Search by name (partial match)", Required: false},
			{Name: "limit", Type: "integer", Description: "Maximum number of results to return", Required: false, Default: 50},
			{Name: "offset", Type: "integer", Description: "Number of results to skip (for pagination)", Required: false, Default: 0},
		},
		RequiredParams: []string{},
		Returns: ReturnDoc{
			Description: "Returns a paginated list of templates.",
			Example: map[string]interface{}{
				"templates": []map[string]interface{}{
					{"id": "tpl_a1b2c3d4e5f6...", "name": "Sprint Summary", "scope": "endeavour"},
				},
				"total":  1,
				"limit":  50,
				"offset": 0,
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.tpl.create", "ts.tpl.get"},
		Since:        "v0.3.7",
	})

	r.Register(&ToolDoc{
		Name:     "ts.tpl.update",
		Category: "template",
		Summary:  "Update a template.",
		Description: `Updates an existing report template. Only provided fields are changed.`,
		Parameters: []ParamDoc{
			{Name: "id", Type: "string", Description: "Template ID", Required: true, Example: "tpl_a1b2c3d4e5f6..."},
			{Name: "name", Type: "string", Description: "New name", Required: false},
			{Name: "body", Type: "string", Description: "New template body", Required: false},
			{Name: "lang", Type: "string", Description: "New language code", Required: false},
			{Name: "status", Type: "string", Description: "New status", Required: false},
			{Name: "metadata", Type: "object", Description: "Metadata to set (replaces existing)", Required: false},
		},
		RequiredParams: []string{"id"},
		Returns: ReturnDoc{
			Description: "Returns the updated template.",
			Example: map[string]interface{}{
				"id":             "tpl_a1b2c3d4e5f6...",
				"updated_fields": []string{"name", "body"},
				"updated_at":     "2026-03-07T12:00:00Z",
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "not_found", Description: "Template not found"},
			{Code: "invalid_input", Description: "No fields to update"},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.tpl.get", "ts.tpl.create"},
		Since:        "v0.3.7",
	})

	r.Register(&ToolDoc{
		Name:     "ts.tpl.fork",
		Category: "template",
		Summary:  "Fork a template to create a derived version.",
		Description: `Creates a new template derived from an existing one. The new template
inherits the source's body and metadata unless overridden.`,
		Parameters: []ParamDoc{
			{Name: "source_id", Type: "string", Description: "Source template ID to fork from", Required: true, Example: "tpl_a1b2c3d4e5f6..."},
			{Name: "name", Type: "string", Description: "Name for the forked template", Required: false},
			{Name: "body", Type: "string", Description: "Override body", Required: false},
			{Name: "lang", Type: "string", Description: "Override language code", Required: false},
			{Name: "metadata", Type: "object", Description: "Override metadata", Required: false},
		},
		RequiredParams: []string{"source_id"},
		Returns: ReturnDoc{
			Description: "Returns the newly created forked template.",
			Example: map[string]interface{}{
				"id":        "tpl_f1g2h3i4j5k6...",
				"source_id": "tpl_a1b2c3d4e5f6...",
				"name":      "Sprint Summary (forked)",
				"status":    "active",
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "not_found", Description: "Source template not found"},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.tpl.get", "ts.tpl.create", "ts.tpl.lineage"},
		Since:        "v0.3.7",
	})

	r.Register(&ToolDoc{
		Name:     "ts.tpl.lineage",
		Category: "template",
		Summary:  "Walk the version chain for a template.",
		Description: `Returns the version lineage of a template, showing the chain of
forks from the original to the current version.`,
		Parameters: []ParamDoc{
			{Name: "id", Type: "string", Description: "Template ID", Required: true, Example: "tpl_a1b2c3d4e5f6..."},
		},
		RequiredParams: []string{"id"},
		Returns: ReturnDoc{
			Description: "Returns the version chain.",
			Example: map[string]interface{}{
				"lineage": []map[string]interface{}{
					{"id": "tpl_original...", "name": "Sprint Summary", "version": 1},
					{"id": "tpl_a1b2c3d4e5f6...", "name": "Sprint Summary v2", "version": 2},
				},
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "not_found", Description: "Template not found"},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.tpl.get", "ts.tpl.fork"},
		Since:        "v0.3.7",
	})
}

func registerReportTools(r *Registry) {
	r.Register(&ToolDoc{
		Name:     "ts.rpt.generate",
		Category: "report",
		Summary:  "Generate a Markdown report.",
		Description: `Generates a Markdown report for a given entity using its associated
report template. The scope determines the entity type.`,
		Parameters: []ParamDoc{
			{Name: "scope", Type: "string", Description: "Report scope: task, demand, endeavour, project", Required: true, Example: "endeavour"},
			{Name: "entity_id", Type: "string", Description: "Entity ID to generate report for", Required: true, Example: "edv_bd159eb7bb9a877a..."},
		},
		RequiredParams: []string{"scope", "entity_id"},
		Returns: ReturnDoc{
			Description: "Returns the generated Markdown report.",
			Example: map[string]interface{}{
				"scope":     "endeavour",
				"entity_id": "edv_bd159eb7bb9a877a...",
				"markdown":  "# Build Taskschmiede Report\n\n...",
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "not_found", Description: "Entity not found"},
			{Code: "invalid_input", Description: "Scope and entity_id are required"},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.tpl.get", "ts.tpl.list"},
		Since:        "v0.3.7",
	})
}

func registerCommentTools(r *Registry) {
	r.Register(&ToolDoc{
		Name:     "ts.cmt.create",
		Category: "comment",
		Summary:  "Add a comment to an entity.",
		Description: `Creates a comment on any commentable entity (task, demand, endeavour,
artifact, ritual, organization). Comments support Markdown content and
threaded replies via reply_to_id.

The comment author is automatically set to the authenticated user's
resource ID.`,
		Parameters: []ParamDoc{
			{
				Name:        "entity_type",
				Type:        "string",
				Description: "Entity type: task, demand, endeavour, artifact, ritual, organization",
				Required:    true,
				Example:     "task",
			},
			{
				Name:        "entity_id",
				Type:        "string",
				Description: "ID of the entity to comment on",
				Required:    true,
				Example:     "tsk_a1b2c3d4e5f6",
			},
			{
				Name:        "content",
				Type:        "string",
				Description: "Comment text (Markdown)",
				Required:    true,
				Example:     "Looks good, but please add error handling for the timeout case.",
			},
			{
				Name:        "reply_to_id",
				Type:        "string",
				Description: "Comment ID to reply to (optional, for threaded replies)",
				Required:    false,
				Example:     "cmt_x1y2z3a4b5c6",
			},
			{
				Name:        "metadata",
				Type:        "object",
				Description: "Arbitrary key-value pairs",
				Required:    false,
			},
		},
		RequiredParams: []string{"entity_type", "entity_id", "content"},
		Returns: ReturnDoc{
			Description: "Returns the created comment.",
			Schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id":          map[string]interface{}{"type": "string"},
					"entity_type": map[string]interface{}{"type": "string"},
					"entity_id":   map[string]interface{}{"type": "string"},
					"author_id":   map[string]interface{}{"type": "string"},
					"content":     map[string]interface{}{"type": "string"},
					"created_at":  map[string]interface{}{"type": "string", "format": "date-time"},
				},
			},
			Example: map[string]interface{}{
				"id":          "cmt_a1b2c3d4e5f6",
				"entity_type": "task",
				"entity_id":   "tsk_a1b2c3d4e5f6",
				"author_id":   "res_x1y2z3a4b5c6",
				"content":     "Looks good, but please add error handling for the timeout case.",
				"created_at":  "2026-02-12T10:00:00Z",
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "invalid_input", Description: "entity_type, entity_id, or content missing"},
			{Code: "not_found", Description: "Target entity does not exist"},
		},
		Examples: []ExampleDoc{
			{
				Title:       "Comment on a task",
				Description: "Add a review comment to a task.",
				Input: map[string]interface{}{
					"entity_type": "task",
					"entity_id":   "tsk_a1b2c3d4e5f6",
					"content":     "Looks good, but please add error handling for the timeout case.",
				},
				Output: map[string]interface{}{
					"id":          "cmt_a1b2c3d4e5f6",
					"entity_type": "task",
					"entity_id":   "tsk_a1b2c3d4e5f6",
					"author_id":   "res_x1y2z3a4b5c6",
					"content":     "Looks good, but please add error handling for the timeout case.",
					"created_at":  "2026-02-12T10:00:00Z",
				},
			},
			{
				Title:       "Reply to a comment",
				Description: "Add a threaded reply to an existing comment.",
				Input: map[string]interface{}{
					"entity_type": "task",
					"entity_id":   "tsk_a1b2c3d4e5f6",
					"content":     "Done, added timeout handling in the latest commit.",
					"reply_to_id": "cmt_a1b2c3d4e5f6",
				},
				Output: map[string]interface{}{
					"id":          "cmt_f6e5d4c3b2a1",
					"entity_type": "task",
					"entity_id":   "tsk_a1b2c3d4e5f6",
					"author_id":   "res_x1y2z3a4b5c6",
					"reply_to_id": "cmt_a1b2c3d4e5f6",
					"content":     "Done, added timeout handling in the latest commit.",
					"created_at":  "2026-02-12T10:05:00Z",
				},
			},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.cmt.list", "ts.cmt.get", "ts.cmt.update", "ts.cmt.delete"},
		Since:        "v0.3.0",
	})

	r.Register(&ToolDoc{
		Name:     "ts.cmt.list",
		Category: "comment",
		Summary:  "List comments on an entity.",
		Description: `Lists comments on a specific entity in chronological order (oldest first).
Supports filtering by author and pagination.

Soft-deleted comments appear as placeholders with content replaced by
"[deleted]" to preserve thread structure.`,
		Parameters: []ParamDoc{
			{
				Name:        "entity_type",
				Type:        "string",
				Description: "Entity type: task, demand, endeavour, artifact, ritual, organization",
				Required:    true,
				Example:     "task",
			},
			{
				Name:        "entity_id",
				Type:        "string",
				Description: "ID of the entity",
				Required:    true,
				Example:     "tsk_a1b2c3d4e5f6",
			},
			{
				Name:        "author_id",
				Type:        "string",
				Description: "Filter by author resource ID",
				Required:    false,
			},
			{
				Name:        "limit",
				Type:        "integer",
				Description: "Max results (default: 50)",
				Required:    false,
				Default:     "50",
			},
			{
				Name:        "offset",
				Type:        "integer",
				Description: "Pagination offset",
				Required:    false,
				Default:     "0",
			},
		},
		RequiredParams: []string{"entity_type", "entity_id"},
		Returns: ReturnDoc{
			Description: "Returns a paginated list of comments.",
			Schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"comments": map[string]interface{}{"type": "array"},
					"total":    map[string]interface{}{"type": "integer"},
					"limit":    map[string]interface{}{"type": "integer"},
					"offset":   map[string]interface{}{"type": "integer"},
				},
			},
			Example: map[string]interface{}{
				"comments": []map[string]interface{}{
					{"id": "cmt_a1b2c3d4e5f6", "content": "First comment", "author_id": "res_x1y2z3a4b5c6"},
				},
				"total":  1,
				"limit":  50,
				"offset": 0,
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "invalid_input", Description: "entity_type or entity_id missing"},
		},
		Examples: []ExampleDoc{
			{
				Title:       "List task comments",
				Description: "Get all comments on a task.",
				Input: map[string]interface{}{
					"entity_type": "task",
					"entity_id":   "tsk_a1b2c3d4e5f6",
				},
				Output: map[string]interface{}{
					"comments": []map[string]interface{}{
						{"id": "cmt_a1b2c3d4e5f6", "content": "Looks good!"},
					},
					"total":  1,
					"limit":  50,
					"offset": 0,
				},
			},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.cmt.create", "ts.cmt.get"},
		Since:        "v0.3.0",
	})

	r.Register(&ToolDoc{
		Name:     "ts.cmt.get",
		Category: "comment",
		Summary:  "Retrieve a comment by ID, including its direct replies.",
		Description: `Fetches a single comment by ID. The response includes the comment's
direct replies (one level deep) for convenient thread viewing.`,
		Parameters: []ParamDoc{
			{
				Name:        "id",
				Type:        "string",
				Description: "Comment ID",
				Required:    true,
				Example:     "cmt_a1b2c3d4e5f6",
			},
		},
		RequiredParams: []string{"id"},
		Returns: ReturnDoc{
			Description: "Returns the comment with its direct replies.",
			Schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id":         map[string]interface{}{"type": "string"},
					"content":    map[string]interface{}{"type": "string"},
					"author_id":  map[string]interface{}{"type": "string"},
					"replies":    map[string]interface{}{"type": "array"},
					"created_at": map[string]interface{}{"type": "string", "format": "date-time"},
				},
			},
			Example: map[string]interface{}{
				"id":         "cmt_a1b2c3d4e5f6",
				"content":    "Review comment",
				"author_id":  "res_x1y2z3a4b5c6",
				"replies":    []map[string]interface{}{},
				"created_at": "2026-02-12T10:00:00Z",
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "invalid_input", Description: "Comment ID is required"},
			{Code: "not_found", Description: "Comment with this ID does not exist"},
		},
		Examples: []ExampleDoc{
			{
				Title: "Get a comment",
				Input: map[string]interface{}{
					"id": "cmt_a1b2c3d4e5f6",
				},
				Output: map[string]interface{}{
					"id":         "cmt_a1b2c3d4e5f6",
					"content":    "Review comment",
					"author_id":  "res_x1y2z3a4b5c6",
					"replies":    []map[string]interface{}{},
					"created_at": "2026-02-12T10:00:00Z",
				},
			},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.cmt.list", "ts.cmt.update", "ts.cmt.delete"},
		Since:        "v0.3.0",
	})

	r.Register(&ToolDoc{
		Name:     "ts.cmt.update",
		Category: "comment",
		Summary:  "Edit a comment (owner-only).",
		Description: `Updates the content or metadata of a comment. Only the comment author
can edit their own comments. Editing sets the edited_at timestamp
to track that the comment was modified.`,
		Parameters: []ParamDoc{
			{
				Name:        "id",
				Type:        "string",
				Description: "Comment ID",
				Required:    true,
				Example:     "cmt_a1b2c3d4e5f6",
			},
			{
				Name:        "content",
				Type:        "string",
				Description: "New comment text",
				Required:    false,
				Example:     "Updated review comment with more detail.",
			},
			{
				Name:        "metadata",
				Type:        "object",
				Description: "New metadata (replaces existing)",
				Required:    false,
			},
		},
		RequiredParams: []string{"id"},
		Returns: ReturnDoc{
			Description: "Returns the updated comment.",
			Example: map[string]interface{}{
				"id":        "cmt_a1b2c3d4e5f6",
				"content":   "Updated review comment with more detail.",
				"edited_at": "2026-02-12T10:15:00Z",
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "invalid_input", Description: "Comment ID is required"},
			{Code: "not_found", Description: "Comment with this ID does not exist"},
			{Code: "unauthorized", Description: "Only the comment author can edit"},
		},
		Examples: []ExampleDoc{
			{
				Title:       "Edit a comment",
				Description: "Update the content of a comment you authored.",
				Input: map[string]interface{}{
					"id":      "cmt_a1b2c3d4e5f6",
					"content": "Updated review comment with more detail.",
				},
				Output: map[string]interface{}{
					"id":        "cmt_a1b2c3d4e5f6",
					"content":   "Updated review comment with more detail.",
					"edited_at": "2026-02-12T10:15:00Z",
				},
			},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.cmt.get", "ts.cmt.delete"},
		Since:        "v0.3.0",
	})

	r.Register(&ToolDoc{
		Name:     "ts.cmt.delete",
		Category: "comment",
		Summary:  "Soft-delete a comment (owner-only).",
		Description: `Soft-deletes a comment. The comment content is replaced with "[deleted]"
but the record is preserved to maintain thread structure. Only the
comment author can delete their own comments.`,
		Parameters: []ParamDoc{
			{
				Name:        "id",
				Type:        "string",
				Description: "Comment ID",
				Required:    true,
				Example:     "cmt_a1b2c3d4e5f6",
			},
		},
		RequiredParams: []string{"id"},
		Returns: ReturnDoc{
			Description: "Returns the deletion confirmation.",
			Example: map[string]interface{}{
				"id":         "cmt_a1b2c3d4e5f6",
				"deleted_at": "2026-02-12T10:20:00Z",
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "invalid_input", Description: "Comment ID is required"},
			{Code: "not_found", Description: "Comment with this ID does not exist"},
			{Code: "unauthorized", Description: "Only the comment author can delete"},
		},
		Examples: []ExampleDoc{
			{
				Title: "Delete a comment",
				Input: map[string]interface{}{
					"id": "cmt_a1b2c3d4e5f6",
				},
				Output: map[string]interface{}{
					"id":         "cmt_a1b2c3d4e5f6",
					"deleted_at": "2026-02-12T10:20:00Z",
				},
			},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.cmt.get", "ts.cmt.update"},
		Since:        "v0.3.0",
	})
}

func registerApprovalTools(r *Registry) {
	r.Register(&ToolDoc{
		Name:     "ts.apr.create",
		Category: "approval",
		Summary:  "Record an approval on an entity.",
		Description: `Records an approval decision on an entity (task, demand, endeavour,
artifact). Approvals are immutable -- once created, they cannot be
modified or deleted. This provides an auditable sign-off trail.

Three verdict types are supported:
- approved: work meets requirements
- rejected: work does not meet requirements
- needs_work: work requires changes before approval`,
		Parameters: []ParamDoc{
			{
				Name:        "entity_type",
				Type:        "string",
				Description: "Entity type: task, demand, endeavour, artifact",
				Required:    true,
				Example:     "task",
			},
			{
				Name:        "entity_id",
				Type:        "string",
				Description: "ID of the entity being approved",
				Required:    true,
				Example:     "tsk_a1b2c3d4e5f6",
			},
			{
				Name:        "verdict",
				Type:        "string",
				Description: "Verdict: approved, rejected, needs_work",
				Required:    true,
				Example:     "approved",
			},
			{
				Name:        "role",
				Type:        "string",
				Description: "Role under which approval is given (e.g., reviewer, product_owner)",
				Required:    false,
				Example:     "reviewer",
			},
			{
				Name:        "comment",
				Type:        "string",
				Description: "Optional rationale or feedback",
				Required:    false,
				Example:     "All acceptance criteria met. Good to ship.",
			},
			{
				Name:        "metadata",
				Type:        "object",
				Description: "Arbitrary key-value pairs (e.g., checklist results, linked artifacts)",
				Required:    false,
			},
		},
		RequiredParams: []string{"entity_type", "entity_id", "verdict"},
		Returns: ReturnDoc{
			Description: "Returns the created approval record.",
			Schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id":          map[string]interface{}{"type": "string"},
					"entity_type": map[string]interface{}{"type": "string"},
					"entity_id":   map[string]interface{}{"type": "string"},
					"approver_id": map[string]interface{}{"type": "string"},
					"verdict":     map[string]interface{}{"type": "string"},
					"created_at":  map[string]interface{}{"type": "string", "format": "date-time"},
				},
			},
			Example: map[string]interface{}{
				"id":          "apr_a1b2c3d4e5f6",
				"entity_type": "task",
				"entity_id":   "tsk_a1b2c3d4e5f6",
				"approver_id": "res_x1y2z3a4b5c6",
				"verdict":     "approved",
				"role":        "reviewer",
				"comment":     "All acceptance criteria met. Good to ship.",
				"created_at":  "2026-02-12T10:00:00Z",
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "invalid_input", Description: "entity_type, entity_id, or verdict missing"},
			{Code: "invalid_input", Description: "Invalid verdict value"},
			{Code: "not_found", Description: "Target entity does not exist"},
		},
		Examples: []ExampleDoc{
			{
				Title:       "Approve a task",
				Description: "Record an approval decision on a task.",
				Input: map[string]interface{}{
					"entity_type": "task",
					"entity_id":   "tsk_a1b2c3d4e5f6",
					"verdict":     "approved",
					"role":        "reviewer",
					"comment":     "All acceptance criteria met. Good to ship.",
				},
				Output: map[string]interface{}{
					"id":          "apr_a1b2c3d4e5f6",
					"entity_type": "task",
					"entity_id":   "tsk_a1b2c3d4e5f6",
					"approver_id": "res_x1y2z3a4b5c6",
					"verdict":     "approved",
					"created_at":  "2026-02-12T10:00:00Z",
				},
			},
			{
				Title:       "Request changes",
				Description: "Record a needs_work verdict with feedback.",
				Input: map[string]interface{}{
					"entity_type": "task",
					"entity_id":   "tsk_a1b2c3d4e5f6",
					"verdict":     "needs_work",
					"comment":     "Missing test coverage for edge cases.",
				},
				Output: map[string]interface{}{
					"id":          "apr_f6e5d4c3b2a1",
					"entity_type": "task",
					"entity_id":   "tsk_a1b2c3d4e5f6",
					"approver_id": "res_x1y2z3a4b5c6",
					"verdict":     "needs_work",
					"created_at":  "2026-02-12T10:05:00Z",
				},
			},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.apr.list", "ts.apr.get"},
		Since:        "v0.3.0",
	})

	r.Register(&ToolDoc{
		Name:     "ts.apr.list",
		Category: "approval",
		Summary:  "List approvals for an entity.",
		Description: `Lists approval records for a specific entity, newest first.
Supports filtering by approver, verdict, and role.`,
		Parameters: []ParamDoc{
			{
				Name:        "entity_type",
				Type:        "string",
				Description: "Entity type: task, demand, endeavour, artifact",
				Required:    true,
				Example:     "task",
			},
			{
				Name:        "entity_id",
				Type:        "string",
				Description: "ID of the entity",
				Required:    true,
				Example:     "tsk_a1b2c3d4e5f6",
			},
			{
				Name:        "approver_id",
				Type:        "string",
				Description: "Filter by approver resource ID",
				Required:    false,
			},
			{
				Name:        "verdict",
				Type:        "string",
				Description: "Filter by verdict: approved, rejected, needs_work",
				Required:    false,
			},
			{
				Name:        "role",
				Type:        "string",
				Description: "Filter by role",
				Required:    false,
			},
			{
				Name:        "limit",
				Type:        "integer",
				Description: "Max results (default: 50)",
				Required:    false,
				Default:     "50",
			},
			{
				Name:        "offset",
				Type:        "integer",
				Description: "Pagination offset",
				Required:    false,
				Default:     "0",
			},
		},
		RequiredParams: []string{"entity_type", "entity_id"},
		Returns: ReturnDoc{
			Description: "Returns a paginated list of approvals.",
			Example: map[string]interface{}{
				"approvals": []map[string]interface{}{
					{"id": "apr_a1b2c3d4e5f6", "verdict": "approved", "approver_id": "res_x1y2z3a4b5c6"},
				},
				"total":  1,
				"limit":  50,
				"offset": 0,
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "invalid_input", Description: "entity_type or entity_id missing"},
		},
		Examples: []ExampleDoc{
			{
				Title:       "List task approvals",
				Description: "Get all approvals for a task.",
				Input: map[string]interface{}{
					"entity_type": "task",
					"entity_id":   "tsk_a1b2c3d4e5f6",
				},
				Output: map[string]interface{}{
					"approvals": []map[string]interface{}{
						{"id": "apr_a1b2c3d4e5f6", "verdict": "approved"},
					},
					"total":  1,
					"limit":  50,
					"offset": 0,
				},
			},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.apr.create", "ts.apr.get"},
		Since:        "v0.3.0",
	})

	r.Register(&ToolDoc{
		Name:     "ts.apr.get",
		Category: "approval",
		Summary:  "Retrieve an approval by ID.",
		Description: `Fetches a single approval record by ID. Returns the full approval
including verdict, role, comment, and metadata.`,
		Parameters: []ParamDoc{
			{
				Name:        "id",
				Type:        "string",
				Description: "Approval ID",
				Required:    true,
				Example:     "apr_a1b2c3d4e5f6",
			},
		},
		RequiredParams: []string{"id"},
		Returns: ReturnDoc{
			Description: "Returns the approval record.",
			Example: map[string]interface{}{
				"id":          "apr_a1b2c3d4e5f6",
				"entity_type": "task",
				"entity_id":   "tsk_a1b2c3d4e5f6",
				"approver_id": "res_x1y2z3a4b5c6",
				"verdict":     "approved",
				"role":        "reviewer",
				"comment":     "All acceptance criteria met.",
				"created_at":  "2026-02-12T10:00:00Z",
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "invalid_input", Description: "Approval ID is required"},
			{Code: "not_found", Description: "Approval with this ID does not exist"},
		},
		Examples: []ExampleDoc{
			{
				Title: "Get an approval",
				Input: map[string]interface{}{
					"id": "apr_a1b2c3d4e5f6",
				},
				Output: map[string]interface{}{
					"id":          "apr_a1b2c3d4e5f6",
					"entity_type": "task",
					"entity_id":   "tsk_a1b2c3d4e5f6",
					"approver_id": "res_x1y2z3a4b5c6",
					"verdict":     "approved",
					"role":        "reviewer",
					"created_at":  "2026-02-12T10:00:00Z",
				},
			},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.apr.create", "ts.apr.list"},
		Since:        "v0.3.0",
	})
}

func registerDodTools(r *Registry) {
	r.Register(&ToolDoc{
		Name:     "ts.dod.create",
		Category: "dod",
		Summary:  "Create a new Definition of Done policy.",
		Description: `Creates a DoD policy with a set of conditions that must be met before
a task can be considered done. Conditions are evaluated automatically
by ts.dod.check.

Each condition has a type (e.g., approval_count, field_set, status_is),
a label, optional parameters, and a required flag. The strictness setting
controls whether all conditions must pass ("all") or a minimum count
("n_of" with quorum).

Four built-in templates are seeded on first run.`,
		Parameters: []ParamDoc{
			{
				Name:        "name",
				Type:        "string",
				Description: "Policy name",
				Required:    true,
				Example:     "Standard Task Completion",
			},
			{
				Name:        "description",
				Type:        "string",
				Description: "Policy description",
				Required:    false,
				Example:     "Requires approval and actual hours logged before marking done.",
			},
			{
				Name:        "origin",
				Type:        "string",
				Description: "Origin: custom (default), derived",
				Required:    false,
				Default:     "custom",
			},
			{
				Name:        "conditions",
				Type:        "array",
				Description: "Array of condition objects with id, type, label, params, required",
				Required:    true,
				Example: []map[string]interface{}{
					{"id": "c1", "type": "approval_count", "label": "At least one approval", "params": map[string]interface{}{"min": 1}, "required": true},
					{"id": "c2", "type": "field_set", "label": "Actual hours logged", "params": map[string]interface{}{"field": "actual"}, "required": true},
				},
			},
			{
				Name:        "strictness",
				Type:        "string",
				Description: "Strictness: all (default), n_of",
				Required:    false,
				Default:     "all",
			},
			{
				Name:        "quorum",
				Type:        "integer",
				Description: "Required count when strictness is n_of",
				Required:    false,
			},
			{
				Name:        "metadata",
				Type:        "object",
				Description: "Arbitrary key-value pairs",
				Required:    false,
			},
		},
		RequiredParams: []string{"name", "conditions"},
		Returns: ReturnDoc{
			Description: "Returns the created DoD policy.",
			Example: map[string]interface{}{
				"id":         "dod_a1b2c3d4e5f6",
				"name":       "Standard Task Completion",
				"origin":     "custom",
				"strictness": "all",
				"status":     "active",
				"created_at": "2026-02-12T10:00:00Z",
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "invalid_input", Description: "Name or conditions missing"},
		},
		Examples: []ExampleDoc{
			{
				Title:       "Create a DoD policy",
				Description: "Define a policy requiring approval and hours logged.",
				Input: map[string]interface{}{
					"name":        "Standard Task Completion",
					"description": "Requires approval and actual hours logged.",
					"conditions": []map[string]interface{}{
						{"id": "c1", "type": "approval_count", "label": "At least one approval", "params": map[string]interface{}{"min": 1}, "required": true},
						{"id": "c2", "type": "field_set", "label": "Actual hours logged", "params": map[string]interface{}{"field": "actual"}, "required": true},
					},
				},
				Output: map[string]interface{}{
					"id":         "dod_a1b2c3d4e5f6",
					"name":       "Standard Task Completion",
					"status":     "active",
					"created_at": "2026-02-12T10:00:00Z",
				},
			},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.dod.get", "ts.dod.list", "ts.dod.assign", "ts.dod.check"},
		Since:        "v0.3.0",
	})

	r.Register(&ToolDoc{
		Name:     "ts.dod.get",
		Category: "dod",
		Summary:  "Retrieve a DoD policy by ID.",
		Description: `Fetches a DoD policy including its conditions, strictness settings,
and metadata.`,
		Parameters: []ParamDoc{
			{
				Name:        "id",
				Type:        "string",
				Description: "DoD policy ID",
				Required:    true,
				Example:     "dod_a1b2c3d4e5f6",
			},
		},
		RequiredParams: []string{"id"},
		Returns: ReturnDoc{
			Description: "Returns the DoD policy.",
			Example: map[string]interface{}{
				"id":         "dod_a1b2c3d4e5f6",
				"name":       "Standard Task Completion",
				"conditions": []map[string]interface{}{},
				"strictness": "all",
				"status":     "active",
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "invalid_input", Description: "DoD policy ID is required"},
			{Code: "not_found", Description: "DoD policy with this ID does not exist"},
		},
		Examples: []ExampleDoc{
			{
				Title: "Get a DoD policy",
				Input: map[string]interface{}{
					"id": "dod_a1b2c3d4e5f6",
				},
				Output: map[string]interface{}{
					"id":         "dod_a1b2c3d4e5f6",
					"name":       "Standard Task Completion",
					"strictness": "all",
					"status":     "active",
				},
			},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.dod.list", "ts.dod.update"},
		Since:        "v0.3.0",
	})

	r.Register(&ToolDoc{
		Name:     "ts.dod.list",
		Category: "dod",
		Summary:  "Query DoD policies with filters.",
		Description: `Lists DoD policies with optional filtering by status, origin, scope,
and search text.`,
		Parameters: []ParamDoc{
			{
				Name:        "status",
				Type:        "string",
				Description: "Filter by status: active, archived",
				Required:    false,
				Example:     "active",
			},
			{
				Name:        "origin",
				Type:        "string",
				Description: "Filter by origin: template, custom, derived",
				Required:    false,
			},
			{
				Name:        "scope",
				Type:        "string",
				Description: "Filter by scope: task",
				Required:    false,
			},
			{
				Name:        "search",
				Type:        "string",
				Description: "Search in name and description",
				Required:    false,
			},
			{
				Name:        "limit",
				Type:        "integer",
				Description: "Max results (default: 50)",
				Required:    false,
				Default:     "50",
			},
			{
				Name:        "offset",
				Type:        "integer",
				Description: "Pagination offset",
				Required:    false,
				Default:     "0",
			},
		},
		Returns: ReturnDoc{
			Description: "Returns a paginated list of DoD policies.",
			Example: map[string]interface{}{
				"policies": []map[string]interface{}{
					{"id": "dod_a1b2c3d4e5f6", "name": "Standard Task Completion", "status": "active"},
				},
				"total":  1,
				"limit":  50,
				"offset": 0,
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
		},
		Examples: []ExampleDoc{
			{
				Title:       "List active policies",
				Description: "Get all active DoD policies.",
				Input: map[string]interface{}{
					"status": "active",
				},
				Output: map[string]interface{}{
					"policies": []map[string]interface{}{
						{"id": "dod_a1b2c3d4e5f6", "name": "Standard Task Completion"},
					},
					"total":  1,
					"limit":  50,
					"offset": 0,
				},
			},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.dod.get", "ts.dod.create"},
		Since:        "v0.3.0",
	})

	r.Register(&ToolDoc{
		Name:     "ts.dod.update",
		Category: "dod",
		Summary:  "Update DoD policy attributes.",
		Description: `Updates a DoD policy's name, description, status, or metadata.
Conditions cannot be changed via update -- use ts.dod.new_version
to create a new version with updated conditions.`,
		Parameters: []ParamDoc{
			{
				Name:        "id",
				Type:        "string",
				Description: "DoD policy ID",
				Required:    true,
				Example:     "dod_a1b2c3d4e5f6",
			},
			{
				Name:        "name",
				Type:        "string",
				Description: "New name",
				Required:    false,
			},
			{
				Name:        "description",
				Type:        "string",
				Description: "New description",
				Required:    false,
			},
			{
				Name:        "status",
				Type:        "string",
				Description: "New status: active, archived",
				Required:    false,
				Example:     "archived",
			},
			{
				Name:        "metadata",
				Type:        "object",
				Description: "Metadata to set (replaces existing)",
				Required:    false,
			},
		},
		RequiredParams: []string{"id"},
		Returns: ReturnDoc{
			Description: "Returns the updated DoD policy.",
			Example: map[string]interface{}{
				"id":         "dod_a1b2c3d4e5f6",
				"name":       "Updated Policy Name",
				"status":     "active",
				"updated_at": "2026-02-12T10:15:00Z",
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "invalid_input", Description: "DoD policy ID is required"},
			{Code: "not_found", Description: "DoD policy with this ID does not exist"},
		},
		Examples: []ExampleDoc{
			{
				Title:       "Archive a policy",
				Description: "Set a DoD policy to archived status.",
				Input: map[string]interface{}{
					"id":     "dod_a1b2c3d4e5f6",
					"status": "archived",
				},
				Output: map[string]interface{}{
					"id":         "dod_a1b2c3d4e5f6",
					"status":     "archived",
					"updated_at": "2026-02-12T10:15:00Z",
				},
			},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.dod.get", "ts.dod.create", "ts.dod.new_version"},
		Since:        "v0.3.0",
	})

	r.Register(&ToolDoc{
		Name:     "ts.dod.new_version",
		Category: "dod",
		Summary:  "Create a new version of a DoD policy with updated conditions.",
		Description: `Creates a new version of an existing DoD policy. The old version is
archived and a predecessor link is established. Existing endorsements
on the old version are superseded -- team members must re-endorse the
new version.

Use this when conditions need to change (conditions cannot be modified
via ts.dod.update). Templates cannot be versioned directly -- fork them
first with ts.dod.create using origin "derived".`,
		Parameters: []ParamDoc{
			{
				Name:        "id",
				Type:        "string",
				Description: "ID of the policy to create a new version of",
				Required:    true,
				Example:     "dod_a1b2c3d4e5f6",
			},
			{
				Name:        "name",
				Type:        "string",
				Description: "Name for the new version (defaults to source name)",
				Required:    false,
			},
			{
				Name:        "description",
				Type:        "string",
				Description: "Description (defaults to source description)",
				Required:    false,
			},
			{
				Name:        "conditions",
				Type:        "array",
				Description: "Array of condition objects with id, type, label, params, required",
				Required:    false,
			},
			{
				Name:        "strictness",
				Type:        "string",
				Description: "Strictness: all (default), n_of",
				Required:    false,
			},
			{
				Name:        "quorum",
				Type:        "integer",
				Description: "Required count when strictness is n_of",
				Required:    false,
			},
			{
				Name:        "metadata",
				Type:        "object",
				Description: "Arbitrary key-value pairs",
				Required:    false,
			},
		},
		RequiredParams: []string{"id"},
		Returns: ReturnDoc{
			Description: "Returns the new policy version.",
			Example: map[string]interface{}{
				"id":             "dod_new123abc",
				"name":           "Review Policy v2",
				"version":        2,
				"predecessor_id": "dod_a1b2c3d4e5f6",
				"conditions":     []interface{}{"..."},
				"status":         "active",
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "invalid_input", Description: "Policy ID is required"},
			{Code: "not_found", Description: "Source policy not found"},
			{Code: "invalid_input", Description: "Cannot version a template policy"},
		},
		Examples: []ExampleDoc{
			{
				Title:       "Update conditions on a policy",
				Description: "Create v2 of a policy with an additional peer review condition.",
				Input: map[string]interface{}{
					"id":   "dod_a1b2c3d4e5f6",
					"name": "Review Policy v2",
					"conditions": []interface{}{
						map[string]interface{}{"id": "cond_01", "type": "comment_required", "label": "Comment needed", "params": map[string]interface{}{"min_comments": 1}, "required": true},
						map[string]interface{}{"id": "cond_02", "type": "peer_review", "label": "Peer review", "params": map[string]interface{}{"min_reviewers": 1}, "required": true},
					},
				},
				Output: map[string]interface{}{
					"id":             "dod_new123abc",
					"name":           "Review Policy v2",
					"version":        2,
					"predecessor_id": "dod_a1b2c3d4e5f6",
					"status":         "active",
				},
			},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.dod.update", "ts.dod.endorse", "ts.dod.get"},
		Since:        "v0.3.0",
	})

	r.Register(&ToolDoc{
		Name:     "ts.dod.assign",
		Category: "dod",
		Summary:  "Assign a DoD policy to an endeavour.",
		Description: `Assigns a DoD policy to govern task completion within an endeavour.
Each endeavour can have at most one active DoD policy. Assigning a
new policy replaces the previous one.

After assignment, team members should endorse the policy via
ts.dod.endorse to signal agreement.`,
		Parameters: []ParamDoc{
			{
				Name:        "endeavour_id",
				Type:        "string",
				Description: "Endeavour ID",
				Required:    true,
				Example:     "edv_a1b2c3d4e5f6",
			},
			{
				Name:        "policy_id",
				Type:        "string",
				Description: "DoD policy ID",
				Required:    true,
				Example:     "dod_a1b2c3d4e5f6",
			},
		},
		RequiredParams: []string{"endeavour_id", "policy_id"},
		Returns: ReturnDoc{
			Description: "Returns the assignment confirmation.",
			Example: map[string]interface{}{
				"endeavour_id": "edv_a1b2c3d4e5f6",
				"policy_id":    "dod_a1b2c3d4e5f6",
				"assigned_at":  "2026-02-12T10:00:00Z",
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "invalid_input", Description: "endeavour_id and policy_id are required"},
			{Code: "not_found", Description: "Endeavour or policy does not exist"},
		},
		Examples: []ExampleDoc{
			{
				Title:       "Assign DoD to endeavour",
				Description: "Set the DoD policy for an endeavour.",
				Input: map[string]interface{}{
					"endeavour_id": "edv_a1b2c3d4e5f6",
					"policy_id":    "dod_a1b2c3d4e5f6",
				},
				Output: map[string]interface{}{
					"endeavour_id": "edv_a1b2c3d4e5f6",
					"policy_id":    "dod_a1b2c3d4e5f6",
					"assigned_at":  "2026-02-12T10:00:00Z",
				},
			},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.dod.unassign", "ts.dod.endorse", "ts.dod.status"},
		Since:        "v0.3.0",
	})

	r.Register(&ToolDoc{
		Name:     "ts.dod.unassign",
		Category: "dod",
		Summary:  "Remove DoD policy from an endeavour.",
		Description: `Removes the currently assigned DoD policy from an endeavour.
Tasks in the endeavour will no longer be subject to DoD checks.`,
		Parameters: []ParamDoc{
			{
				Name:        "endeavour_id",
				Type:        "string",
				Description: "Endeavour ID",
				Required:    true,
				Example:     "edv_a1b2c3d4e5f6",
			},
		},
		RequiredParams: []string{"endeavour_id"},
		Returns: ReturnDoc{
			Description: "Returns the unassignment confirmation.",
			Example: map[string]interface{}{
				"endeavour_id": "edv_a1b2c3d4e5f6",
				"unassigned":   true,
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "invalid_input", Description: "endeavour_id is required"},
			{Code: "not_found", Description: "Endeavour does not exist or has no assigned policy"},
		},
		Examples: []ExampleDoc{
			{
				Title: "Remove DoD from endeavour",
				Input: map[string]interface{}{
					"endeavour_id": "edv_a1b2c3d4e5f6",
				},
				Output: map[string]interface{}{
					"endeavour_id": "edv_a1b2c3d4e5f6",
					"unassigned":   true,
				},
			},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.dod.assign", "ts.dod.status"},
		Since:        "v0.3.0",
	})

	r.Register(&ToolDoc{
		Name:     "ts.dod.endorse",
		Category: "dod",
		Summary:  "Endorse the current DoD policy for an endeavour.",
		Description: `Records the caller's endorsement of the DoD policy currently assigned
to an endeavour. Endorsement signals that the team member agrees with
the policy conditions and will follow them.

Endorsements are tracked per policy version. If the policy is
reassigned, previous endorsements are superseded.`,
		Parameters: []ParamDoc{
			{
				Name:        "endeavour_id",
				Type:        "string",
				Description: "Endeavour ID",
				Required:    true,
				Example:     "edv_a1b2c3d4e5f6",
			},
		},
		RequiredParams: []string{"endeavour_id"},
		Returns: ReturnDoc{
			Description: "Returns the endorsement record.",
			Example: map[string]interface{}{
				"id":           "end_a1b2c3d4e5f6",
				"policy_id":    "dod_a1b2c3d4e5f6",
				"resource_id":  "res_x1y2z3a4b5c6",
				"endeavour_id": "edv_a1b2c3d4e5f6",
				"endorsed_at":  "2026-02-12T10:00:00Z",
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "invalid_input", Description: "endeavour_id is required"},
			{Code: "not_found", Description: "Endeavour has no assigned DoD policy"},
			{Code: "conflict", Description: "Already endorsed the current policy version"},
		},
		Examples: []ExampleDoc{
			{
				Title:       "Endorse a DoD policy",
				Description: "Signal agreement with the assigned DoD policy.",
				Input: map[string]interface{}{
					"endeavour_id": "edv_a1b2c3d4e5f6",
				},
				Output: map[string]interface{}{
					"id":           "end_a1b2c3d4e5f6",
					"policy_id":    "dod_a1b2c3d4e5f6",
					"resource_id":  "res_x1y2z3a4b5c6",
					"endeavour_id": "edv_a1b2c3d4e5f6",
					"endorsed_at":  "2026-02-12T10:00:00Z",
				},
			},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.dod.assign", "ts.dod.status"},
		Since:        "v0.3.0",
	})

	r.Register(&ToolDoc{
		Name:     "ts.dod.check",
		Category: "dod",
		Summary:  "Evaluate DoD conditions for a task (dry run).",
		Description: `Checks whether a task meets the DoD conditions defined by the policy
assigned to the task's endeavour. This is a dry run -- it does not
modify the task or block status transitions.

Returns the evaluation result for each condition, showing which
conditions pass and which fail, along with the overall verdict.`,
		Parameters: []ParamDoc{
			{
				Name:        "task_id",
				Type:        "string",
				Description: "Task ID to check",
				Required:    true,
				Example:     "tsk_a1b2c3d4e5f6",
			},
		},
		RequiredParams: []string{"task_id"},
		Returns: ReturnDoc{
			Description: "Returns the DoD check results.",
			Example: map[string]interface{}{
				"task_id":   "tsk_a1b2c3d4e5f6",
				"policy_id": "dod_a1b2c3d4e5f6",
				"pass":      false,
				"results": []map[string]interface{}{
					{"id": "c1", "label": "At least one approval", "pass": true},
					{"id": "c2", "label": "Actual hours logged", "pass": false, "reason": "field 'actual' is not set"},
				},
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "invalid_input", Description: "task_id is required"},
			{Code: "not_found", Description: "Task does not exist or has no DoD policy"},
		},
		Examples: []ExampleDoc{
			{
				Title:       "Check task DoD",
				Description: "Evaluate whether a task meets its DoD conditions.",
				Input: map[string]interface{}{
					"task_id": "tsk_a1b2c3d4e5f6",
				},
				Output: map[string]interface{}{
					"task_id":   "tsk_a1b2c3d4e5f6",
					"policy_id": "dod_a1b2c3d4e5f6",
					"pass":      false,
					"results": []map[string]interface{}{
						{"id": "c1", "label": "At least one approval", "pass": true},
						{"id": "c2", "label": "Actual hours logged", "pass": false},
					},
				},
			},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.dod.override", "ts.dod.status", "ts.tsk.update"},
		Since:        "v0.3.0",
	})

	r.Register(&ToolDoc{
		Name:     "ts.dod.status",
		Category: "dod",
		Summary:  "Show DoD policy and endorsement status for an endeavour.",
		Description: `Returns the DoD policy assigned to an endeavour along with its
endorsement status. Shows which team members have endorsed the
current policy version and which have not.`,
		Parameters: []ParamDoc{
			{
				Name:        "endeavour_id",
				Type:        "string",
				Description: "Endeavour ID",
				Required:    true,
				Example:     "edv_a1b2c3d4e5f6",
			},
		},
		RequiredParams: []string{"endeavour_id"},
		Returns: ReturnDoc{
			Description: "Returns the DoD status for the endeavour.",
			Example: map[string]interface{}{
				"endeavour_id":   "edv_a1b2c3d4e5f6",
				"policy_id":      "dod_a1b2c3d4e5f6",
				"policy_name":    "Standard Task Completion",
				"policy_version": 1,
				"endorsements":   []map[string]interface{}{},
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "invalid_input", Description: "endeavour_id is required"},
		},
		Examples: []ExampleDoc{
			{
				Title:       "Check endeavour DoD status",
				Description: "View the assigned DoD policy and who has endorsed it.",
				Input: map[string]interface{}{
					"endeavour_id": "edv_a1b2c3d4e5f6",
				},
				Output: map[string]interface{}{
					"endeavour_id":   "edv_a1b2c3d4e5f6",
					"policy_id":      "dod_a1b2c3d4e5f6",
					"policy_name":    "Standard Task Completion",
					"policy_version": 1,
					"endorsements":   []map[string]interface{}{},
				},
			},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.dod.assign", "ts.dod.endorse", "ts.dod.check"},
		Since:        "v0.3.0",
	})

	r.Register(&ToolDoc{
		Name:     "ts.dod.override",
		Category: "dod",
		Summary:  "Override DoD for a specific task (requires reason).",
		Description: `Allows a task to be marked as done even if it does not pass all DoD
conditions. A reason is required and recorded for audit purposes.

Use this sparingly -- overrides bypass governance controls and should
be justified (e.g., hotfix, external dependency, scope change).`,
		Parameters: []ParamDoc{
			{
				Name:        "task_id",
				Type:        "string",
				Description: "Task ID",
				Required:    true,
				Example:     "tsk_a1b2c3d4e5f6",
			},
			{
				Name:        "reason",
				Type:        "string",
				Description: "Reason for override (required)",
				Required:    true,
				Example:     "Hotfix: customer-blocking issue, approval deferred to post-deploy review.",
			},
		},
		RequiredParams: []string{"task_id", "reason"},
		Returns: ReturnDoc{
			Description: "Returns the override confirmation.",
			Example: map[string]interface{}{
				"task_id":     "tsk_a1b2c3d4e5f6",
				"overridden":  true,
				"reason":      "Hotfix: customer-blocking issue, approval deferred to post-deploy review.",
				"override_by": "res_x1y2z3a4b5c6",
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "invalid_input", Description: "task_id and reason are required"},
			{Code: "not_found", Description: "Task does not exist"},
		},
		Examples: []ExampleDoc{
			{
				Title:       "Override DoD for a hotfix",
				Description: "Allow a task to complete despite failing DoD checks.",
				Input: map[string]interface{}{
					"task_id": "tsk_a1b2c3d4e5f6",
					"reason":  "Hotfix: customer-blocking issue, approval deferred to post-deploy review.",
				},
				Output: map[string]interface{}{
					"task_id":     "tsk_a1b2c3d4e5f6",
					"overridden":  true,
					"override_by": "res_x1y2z3a4b5c6",
				},
			},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.dod.check", "ts.tsk.update"},
		Since:        "v0.3.0",
	})

	r.Register(&ToolDoc{
		Name:     "ts.dod.lineage",
		Category: "dod",
		Summary:  "Walk the version chain for a DoD policy.",
		Description: `Returns the version lineage of a DoD policy, showing the chain of
versions from the original to the current version.`,
		Parameters: []ParamDoc{
			{Name: "id", Type: "string", Description: "DoD policy ID", Required: true, Example: "dod_a1b2c3d4e5f6..."},
		},
		RequiredParams: []string{"id"},
		Returns: ReturnDoc{
			Description: "Returns the version chain.",
			Example: map[string]interface{}{
				"lineage": []map[string]interface{}{
					{"id": "dod_original...", "name": "Standard Task Completion", "version": 1},
					{"id": "dod_a1b2c3d4e5f6...", "name": "Standard Task Completion v2", "version": 2},
				},
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "not_found", Description: "DoD policy not found"},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.dod.get", "ts.dod.new_version"},
		Since:        "v0.3.7",
	})
}

func registerMessageTools(r *Registry) {
	r.Register(&ToolDoc{
		Name:     "ts.msg.send",
		Category: "message",
		Summary:  "Send a message to one or more recipients.",
		Description: `Sends an internal message with support for direct delivery,
endeavour-scoped broadcast, or organization-scoped broadcast.

Messages have an intent field (info, question, action, alert) to help
recipients prioritize. Supports threading via reply_to_id and optional
entity context linking.

At least one delivery target is required: either recipient_ids for
direct messages or scope_type + scope_id for group delivery.`,
		Parameters: []ParamDoc{
			{
				Name:        "content",
				Type:        "string",
				Description: "Message body (Markdown)",
				Required:    true,
				Example:     "The deployment is complete. All services are green.",
			},
			{
				Name:        "subject",
				Type:        "string",
				Description: "Message subject (optional)",
				Required:    false,
				Example:     "Deployment Status Update",
			},
			{
				Name:        "intent",
				Type:        "string",
				Description: "Message intent: info, question, action, alert (default: info)",
				Required:    false,
				Default:     "info",
				Example:     "info",
			},
			{
				Name:        "recipient_ids",
				Type:        "array",
				Description: "Resource IDs of direct recipients",
				Required:    false,
				Example:     []string{"res_x1y2z3a4b5c6", "res_a9b8c7d6e5f4"},
			},
			{
				Name:        "scope_type",
				Type:        "string",
				Description: "Scope for group delivery: endeavour, organization",
				Required:    false,
				Example:     "endeavour",
			},
			{
				Name:        "scope_id",
				Type:        "string",
				Description: "ID of the endeavour or organization (required when scope_type is set)",
				Required:    false,
				Example:     "edv_a1b2c3d4e5f6",
			},
			{
				Name:        "reply_to_id",
				Type:        "string",
				Description: "Message ID to reply to (creates a thread)",
				Required:    false,
			},
			{
				Name:        "entity_type",
				Type:        "string",
				Description: "Optional context: entity type (task, endeavour, ...)",
				Required:    false,
			},
			{
				Name:        "entity_id",
				Type:        "string",
				Description: "Optional context: entity ID",
				Required:    false,
			},
			{
				Name:        "metadata",
				Type:        "object",
				Description: "Arbitrary key-value pairs",
				Required:    false,
			},
		},
		RequiredParams: []string{"content"},
		Returns: ReturnDoc{
			Description: "Returns the sent message with delivery information.",
			Schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id":         map[string]interface{}{"type": "string"},
					"sender_id":  map[string]interface{}{"type": "string"},
					"content":    map[string]interface{}{"type": "string"},
					"intent":     map[string]interface{}{"type": "string"},
					"deliveries": map[string]interface{}{"type": "integer"},
					"created_at": map[string]interface{}{"type": "string", "format": "date-time"},
				},
			},
			Example: map[string]interface{}{
				"id":         "msg_a1b2c3d4e5f6",
				"sender_id":  "res_x1y2z3a4b5c6",
				"content":    "The deployment is complete.",
				"intent":     "info",
				"deliveries": 2,
				"created_at": "2026-02-12T10:00:00Z",
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "invalid_input", Description: "content is required"},
			{Code: "invalid_input", Description: "No delivery target (recipient_ids or scope_type+scope_id)"},
		},
		Examples: []ExampleDoc{
			{
				Title:       "Send a direct message",
				Description: "Send a message to specific recipients.",
				Input: map[string]interface{}{
					"content":       "The deployment is complete. All services are green.",
					"subject":       "Deployment Status Update",
					"intent":        "info",
					"recipient_ids": []string{"res_x1y2z3a4b5c6"},
				},
				Output: map[string]interface{}{
					"id":         "msg_a1b2c3d4e5f6",
					"sender_id":  "res_x1y2z3a4b5c6",
					"deliveries": 1,
					"created_at": "2026-02-12T10:00:00Z",
				},
			},
			{
				Title:       "Broadcast to an endeavour",
				Description: "Send a message to all members of an endeavour.",
				Input: map[string]interface{}{
					"content":    "Sprint planning tomorrow at 10:00 UTC.",
					"intent":     "action",
					"scope_type": "endeavour",
					"scope_id":   "edv_a1b2c3d4e5f6",
				},
				Output: map[string]interface{}{
					"id":         "msg_f6e5d4c3b2a1",
					"sender_id":  "res_x1y2z3a4b5c6",
					"deliveries": 5,
					"created_at": "2026-02-12T10:00:00Z",
				},
			},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.msg.inbox", "ts.msg.reply", "ts.msg.thread"},
		Since:        "v0.3.0",
	})

	r.Register(&ToolDoc{
		Name:     "ts.msg.inbox",
		Category: "message",
		Summary:  "List unread/recent messages for current resource.",
		Description: `Retrieves the inbox for the authenticated user's resource. Returns
messages delivered to the user, newest first.

Supports filtering by delivery status, message intent, entity
context, and unread flag.`,
		Parameters: []ParamDoc{
			{
				Name:        "status",
				Type:        "string",
				Description: "Filter by status: pending, delivered, read",
				Required:    false,
			},
			{
				Name:        "intent",
				Type:        "string",
				Description: "Filter by intent: info, question, action, alert",
				Required:    false,
			},
			{
				Name:        "unread",
				Type:        "boolean",
				Description: "Show only unread messages (status != read)",
				Required:    false,
			},
			{
				Name:        "entity_type",
				Type:        "string",
				Description: "Filter by context entity type",
				Required:    false,
			},
			{
				Name:        "entity_id",
				Type:        "string",
				Description: "Filter by context entity ID",
				Required:    false,
			},
			{
				Name:        "limit",
				Type:        "integer",
				Description: "Max results (default: 50)",
				Required:    false,
				Default:     "50",
			},
			{
				Name:        "offset",
				Type:        "integer",
				Description: "Pagination offset",
				Required:    false,
				Default:     "0",
			},
		},
		Returns: ReturnDoc{
			Description: "Returns a paginated list of inbox messages.",
			Example: map[string]interface{}{
				"messages": []map[string]interface{}{
					{"id": "msg_a1b2c3d4e5f6", "subject": "Update", "intent": "info", "status": "pending"},
				},
				"total":  1,
				"limit":  50,
				"offset": 0,
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
		},
		Examples: []ExampleDoc{
			{
				Title:       "Check inbox",
				Description: "Get unread messages.",
				Input: map[string]interface{}{
					"unread": true,
				},
				Output: map[string]interface{}{
					"messages": []map[string]interface{}{
						{"id": "msg_a1b2c3d4e5f6", "intent": "action", "status": "pending"},
					},
					"total":  1,
					"limit":  50,
					"offset": 0,
				},
			},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.msg.read", "ts.msg.send"},
		Since:        "v0.3.0",
	})

	r.Register(&ToolDoc{
		Name:     "ts.msg.read",
		Category: "message",
		Summary:  "Get a message and mark as read.",
		Description: `Retrieves a message by ID and marks the delivery as read for the
authenticated user. The read_at timestamp is set automatically.`,
		Parameters: []ParamDoc{
			{
				Name:        "id",
				Type:        "string",
				Description: "Message ID",
				Required:    true,
				Example:     "msg_a1b2c3d4e5f6",
			},
		},
		RequiredParams: []string{"id"},
		Returns: ReturnDoc{
			Description: "Returns the full message content.",
			Example: map[string]interface{}{
				"id":        "msg_a1b2c3d4e5f6",
				"sender_id": "res_x1y2z3a4b5c6",
				"subject":   "Deployment Status Update",
				"content":   "The deployment is complete. All services are green.",
				"intent":    "info",
				"read_at":   "2026-02-12T10:05:00Z",
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "invalid_input", Description: "Message ID is required"},
			{Code: "not_found", Description: "Message not found or not delivered to this user"},
		},
		Examples: []ExampleDoc{
			{
				Title: "Read a message",
				Input: map[string]interface{}{
					"id": "msg_a1b2c3d4e5f6",
				},
				Output: map[string]interface{}{
					"id":        "msg_a1b2c3d4e5f6",
					"sender_id": "res_x1y2z3a4b5c6",
					"content":   "The deployment is complete.",
					"intent":    "info",
					"read_at":   "2026-02-12T10:05:00Z",
				},
			},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.msg.inbox", "ts.msg.reply"},
		Since:        "v0.3.0",
	})

	r.Register(&ToolDoc{
		Name:     "ts.msg.reply",
		Category: "message",
		Summary:  "Reply to a message.",
		Description: `Sends a reply to an existing message. The reply is delivered to the
original sender and creates a thread. Use ts.msg.thread to view
the full conversation.`,
		Parameters: []ParamDoc{
			{
				Name:        "message_id",
				Type:        "string",
				Description: "ID of the message to reply to",
				Required:    true,
				Example:     "msg_a1b2c3d4e5f6",
			},
			{
				Name:        "content",
				Type:        "string",
				Description: "Reply body (Markdown)",
				Required:    true,
				Example:     "Thanks for the update. Any issues during the rollout?",
			},
			{
				Name:        "metadata",
				Type:        "object",
				Description: "Arbitrary key-value pairs",
				Required:    false,
			},
		},
		RequiredParams: []string{"message_id", "content"},
		Returns: ReturnDoc{
			Description: "Returns the reply message.",
			Example: map[string]interface{}{
				"id":          "msg_f6e5d4c3b2a1",
				"sender_id":   "res_a9b8c7d6e5f4",
				"reply_to_id": "msg_a1b2c3d4e5f6",
				"content":     "Thanks for the update. Any issues during the rollout?",
				"created_at":  "2026-02-12T10:10:00Z",
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "invalid_input", Description: "message_id and content are required"},
			{Code: "not_found", Description: "Original message does not exist"},
		},
		Examples: []ExampleDoc{
			{
				Title:       "Reply to a message",
				Description: "Send a reply to an existing message.",
				Input: map[string]interface{}{
					"message_id": "msg_a1b2c3d4e5f6",
					"content":    "Thanks for the update. Any issues during the rollout?",
				},
				Output: map[string]interface{}{
					"id":          "msg_f6e5d4c3b2a1",
					"reply_to_id": "msg_a1b2c3d4e5f6",
					"created_at":  "2026-02-12T10:10:00Z",
				},
			},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.msg.read", "ts.msg.thread"},
		Since:        "v0.3.0",
	})

	r.Register(&ToolDoc{
		Name:     "ts.msg.thread",
		Category: "message",
		Summary:  "Get full conversation thread.",
		Description: `Retrieves the complete message thread starting from any message in
the conversation. Returns all messages in chronological order
(oldest first).`,
		Parameters: []ParamDoc{
			{
				Name:        "message_id",
				Type:        "string",
				Description: "Any message ID in the thread",
				Required:    true,
				Example:     "msg_a1b2c3d4e5f6",
			},
		},
		RequiredParams: []string{"message_id"},
		Returns: ReturnDoc{
			Description: "Returns all messages in the thread.",
			Example: map[string]interface{}{
				"messages": []map[string]interface{}{
					{"id": "msg_a1b2c3d4e5f6", "content": "Original message"},
					{"id": "msg_f6e5d4c3b2a1", "content": "Reply", "reply_to_id": "msg_a1b2c3d4e5f6"},
				},
				"total": 2,
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "invalid_input", Description: "message_id is required"},
			{Code: "not_found", Description: "Message does not exist"},
		},
		Examples: []ExampleDoc{
			{
				Title: "View a conversation thread",
				Input: map[string]interface{}{
					"message_id": "msg_a1b2c3d4e5f6",
				},
				Output: map[string]interface{}{
					"messages": []map[string]interface{}{
						{"id": "msg_a1b2c3d4e5f6", "content": "Original message"},
						{"id": "msg_f6e5d4c3b2a1", "content": "Reply"},
					},
					"total": 2,
				},
			},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.msg.read", "ts.msg.reply"},
		Since:        "v0.3.0",
	})
}

func registerAuditTools(r *Registry) {
	r.Register(&ToolDoc{
		Name:     "ts.audit.list",
		Category: "audit",
		Summary:  "Query audit logs with filters (admin-only).",
		Description: `Lists audit log entries with optional filtering by action, actor, resource,
IP address, and time range. Admin-only.

Use cursor-based pagination (before_id) instead of offset for stable
results, since the audit middleware logs every request including
pagination queries themselves.`,
		Parameters: []ParamDoc{
			{
				Name:        "action",
				Type:        "string",
				Description: "Filter by action (e.g., login_success, login_failure, security_alert)",
				Required:    false,
				Example:     "login_failure",
			},
			{
				Name:        "actor_id",
				Type:        "string",
				Description: "Filter by actor ID",
				Required:    false,
			},
			{
				Name:        "resource",
				Type:        "string",
				Description: "Filter by resource",
				Required:    false,
			},
			{
				Name:        "ip",
				Type:        "string",
				Description: "Filter by IP address",
				Required:    false,
			},
			{
				Name:        "start_time",
				Type:        "string",
				Description: "Filter: entries after this time (ISO 8601)",
				Required:    false,
			},
			{
				Name:        "end_time",
				Type:        "string",
				Description: "Filter: entries before this time (ISO 8601)",
				Required:    false,
			},
			{
				Name:        "before_id",
				Type:        "string",
				Description: "Cursor: return entries older than this audit entry ID (stable pagination)",
				Required:    false,
			},
			{
				Name:        "limit",
				Type:        "integer",
				Description: "Max results (default: 50)",
				Required:    false,
				Default:     "50",
			},
			{
				Name:        "offset",
				Type:        "integer",
				Description: "Pagination offset (prefer before_id for stable results)",
				Required:    false,
				Default:     "0",
			},
		},
		Returns: ReturnDoc{
			Description: "Returns a paginated list of audit log entries.",
			Example: map[string]interface{}{
				"entries": []map[string]interface{}{
					{"id": "aud_a1b2c3d4e5f6", "action": "login_success", "actor_id": "usr_01H8X9"},
				},
				"total":  1,
				"limit":  50,
				"offset": 0,
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "unauthorized", Description: "Admin privileges required"},
		},
		Examples: []ExampleDoc{
			{
				Title:       "List recent login failures",
				Description: "Query audit log for failed login attempts.",
				Input: map[string]interface{}{
					"action": "login_failure",
					"limit":  10,
				},
				Output: map[string]interface{}{
					"entries": []map[string]interface{}{
						{"id": "aud_a1b2c3d4e5f6", "action": "login_failure", "ip": "192.168.1.1"},
					},
					"total":  1,
					"limit":  10,
					"offset": 0,
				},
			},
		},
		RequiresAuth: true,
		RelatedTools: []string{},
		Since:        "v0.2.0",
		Visibility:   "internal",
	})

	r.Register(&ToolDoc{
		Name:     "ts.audit.my_activity",
		Category: "audit",
		Summary:  "List your own audit activity (no admin required).",
		Description: `Returns audit log entries for the currently authenticated user.
Does not require admin privileges -- users can always view their own activity.`,
		Parameters: []ParamDoc{
			{Name: "action", Type: "string", Description: "Filter by action", Required: false},
			{Name: "start_time", Type: "string", Description: "Filter: entries after this time (ISO 8601)", Required: false},
			{Name: "end_time", Type: "string", Description: "Filter: entries before this time (ISO 8601)", Required: false},
			{Name: "limit", Type: "integer", Description: "Max results (default: 50)", Required: false, Default: 50},
			{Name: "offset", Type: "integer", Description: "Pagination offset", Required: false, Default: 0},
		},
		RequiredParams: []string{},
		Returns: ReturnDoc{
			Description: "Returns a paginated list of the caller's audit entries.",
			Example: map[string]interface{}{
				"entries": []map[string]interface{}{
					{"id": "aud_a1b2c3d4e5f6", "action": "login_success"},
				},
				"total":  1,
				"limit":  50,
				"offset": 0,
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.audit.list"},
		Since:        "v0.3.7",
		Visibility:   "internal",
	})

	r.Register(&ToolDoc{
		Name:     "ts.audit.entity_changes",
		Category: "audit",
		Summary:  "Query entity change history (admin or scoped).",
		Description: `Returns audit log entries filtered by entity. Admins can query any entity;
non-admins can query entities within their scope.`,
		Parameters: []ParamDoc{
			{Name: "action", Type: "string", Description: "Filter by action", Required: false},
			{Name: "entity_type", Type: "string", Description: "Filter by entity type (e.g., task, demand)", Required: false},
			{Name: "entity_id", Type: "string", Description: "Filter by entity ID", Required: false},
			{Name: "actor_id", Type: "string", Description: "Filter by actor ID", Required: false},
			{Name: "endeavour_id", Type: "string", Description: "Filter by endeavour ID", Required: false},
			{Name: "start_time", Type: "string", Description: "Filter: entries after this time (ISO 8601)", Required: false},
			{Name: "end_time", Type: "string", Description: "Filter: entries before this time (ISO 8601)", Required: false},
			{Name: "limit", Type: "integer", Description: "Max results (default: 50)", Required: false, Default: 50},
			{Name: "offset", Type: "integer", Description: "Pagination offset", Required: false, Default: 0},
		},
		RequiredParams: []string{},
		Returns: ReturnDoc{
			Description: "Returns a paginated list of entity change audit entries.",
			Example: map[string]interface{}{
				"entries": []map[string]interface{}{
					{"id": "aud_a1b2c3d4e5f6", "action": "task.update", "entity_id": "tsk_a1b2c3d4e5f6"},
				},
				"total":  1,
				"limit":  50,
				"offset": 0,
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "forbidden", Description: "Insufficient privileges for the requested scope"},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.audit.list", "ts.audit.my_activity"},
		Since:        "v0.3.7",
		Visibility:   "internal",
	})
}

func registerDocTools(r *Registry) {
	r.Register(&ToolDoc{
		Name:     "ts.doc.list",
		Category: "doc",
		Summary:  "List available documentation (recipes, guides, workflows)",
		Description: `Returns a list of available documentation entries. Filter by type to narrow results.

Documentation is public and does not require authentication.`,
		Parameters: []ParamDoc{
			{
				Name:        "type",
				Type:        "string",
				Description: "Filter by type: recipe, guide, workflow (omit for all)",
				Required:    false,
				Example:     "recipe",
			},
		},
		Returns: ReturnDoc{
			Description: "Returns an array of documentation entries with name, type, title, summary, and tags.",
			Schema: map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"name":    map[string]interface{}{"type": "string"},
						"type":    map[string]interface{}{"type": "string"},
						"title":   map[string]interface{}{"type": "string"},
						"summary": map[string]interface{}{"type": "string"},
						"tags":    map[string]interface{}{"type": "array"},
					},
				},
			},
		},
		RequiresAuth: false,
		RelatedTools: []string{"ts.doc.get"},
		Since:        "v0.3.3",
	})

	r.Register(&ToolDoc{
		Name:     "ts.doc.get",
		Category: "doc",
		Summary:  "Get a specific document as Markdown",
		Description: `Returns the full content of a documentation entry as Markdown text. Works for recipes,
guides, and workflows.

Documentation is public and does not require authentication.`,
		Parameters: []ParamDoc{
			{
				Name:        "name",
				Type:        "string",
				Description: "Document identifier (e.g., onboard-agent, getting-started)",
				Required:    true,
				Example:     "onboard-agent",
			},
		},
		RequiredParams: []string{"name"},
		Returns: ReturnDoc{
			Description: "Returns the document content as Markdown text with YAML frontmatter.",
		},
		Errors: []ErrorDoc{
			{Code: "invalid_input", Description: "name is required"},
			{Code: "not_found", Description: "Document not found"},
		},
		RequiresAuth: false,
		RelatedTools: []string{"ts.doc.list"},
		Since:        "v0.3.3",
	})
}

func registerOnboardingDocsTools(r *Registry) {
	r.Register(&ToolDoc{
		Name:     "ts.onboard.health",
		Category: "onboarding",
		Summary:  "View agent behavioral health dashboard",
		Description: `Admin-only tool that returns a comprehensive agent behavioral health dashboard.

Includes per-agent success rates (session, 24h rolling, 7d rolling), health status
(healthy, warned, flagged, suspended), and Ablecon traffic light levels (system-wide
and per-organization).

The Ablecon system provides a quick visual indicator of agent behavioral health:
- Level 1 (Green): Normal operations, all metrics within thresholds
- Level 2 (Yellow): Elevated concern, one or more warning conditions active
- Level 3 (Red): Critical, suspension threshold breached or stark deviation detected`,
		Parameters: []ParamDoc{},
		Returns: ReturnDoc{
			Description: "Returns agent health data, status counts, and Ablecon levels.",
			Example: map[string]interface{}{
				"total_agents": 5,
				"by_status": map[string]interface{}{
					"healthy": 3, "warned": 1, "flagged": 1, "suspended": 0,
				},
				"agents": []map[string]interface{}{
					{
						"user_id": "usr_01H8X9", "status": "healthy",
						"session_rate": 0.95, "rolling_7d_rate": 0.91,
					},
				},
				"ablecon": map[string]interface{}{
					"system": map[string]interface{}{"level": 1, "label": "green"},
				},
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "forbidden", Description: "Only available to human administrators"},
		},
		Examples: []ExampleDoc{
			{
				Title:       "View health dashboard",
				Description: "Get the full agent behavioral health overview.",
				Input:       map[string]interface{}{},
				Output: map[string]interface{}{
					"total_agents": 3,
					"by_status":    map[string]interface{}{"healthy": 2, "warned": 1, "flagged": 0, "suspended": 0},
					"agents":       []map[string]interface{}{{"user_id": "usr_01H8X9", "status": "healthy"}},
					"ablecon":      map[string]interface{}{"system": map[string]interface{}{"level": 1, "label": "green"}},
				},
			},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.auth.whoami", "ts.onboard.status"},
		Since:        "v0.2.4",
		Visibility:   "internal",
	})

	r.Register(&ToolDoc{
		Name:     "ts.onboard.start_interview",
		Category: "onboarding",
		Summary:  "Start the onboarding interview for a new agent.",
		Description: `Initiates the onboarding interview process for a new agent.
The interview verifies the agent's identity and capabilities.`,
		Parameters:     []ParamDoc{},
		RequiredParams: []string{},
		Returns: ReturnDoc{
			Description: "Returns the interview session details.",
			Example: map[string]interface{}{
				"status":  "interview_started",
				"step":    0,
				"message": "Welcome to Taskschmiede onboarding.",
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "already_onboarded", Description: "Agent has already completed onboarding"},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.onboard.status", "ts.onboard.step0"},
		Since:        "v0.3.7",
		Visibility:   "internal",
	})

	r.Register(&ToolDoc{
		Name:     "ts.onboard.status",
		Category: "onboarding",
		Summary:  "Check the current onboarding status for an agent.",
		Description: `Returns the current onboarding progress and status for the authenticated agent.`,
		Parameters:     []ParamDoc{},
		RequiredParams: []string{},
		Returns: ReturnDoc{
			Description: "Returns the onboarding status.",
			Example: map[string]interface{}{
				"status":         "in_progress",
				"current_step":   2,
				"total_steps":    5,
				"completed_steps": []int{0, 1},
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.onboard.health", "ts.onboard.start_interview"},
		Since:        "v0.3.7",
		Visibility:   "internal",
	})

	r.Register(&ToolDoc{
		Name:     "ts.onboard.step0",
		Category: "onboarding",
		Summary:  "Submit the initial self-description step of onboarding.",
		Description: `Submits the agent's self-description as the first step of onboarding.`,
		Parameters: []ParamDoc{
			{Name: "description", Type: "string", Description: "Agent self-description", Required: true, Example: "I am an AI coding assistant specializing in Go and TypeScript."},
		},
		RequiredParams: []string{"description"},
		Returns: ReturnDoc{
			Description: "Returns the step result.",
			Example: map[string]interface{}{
				"status":  "step_completed",
				"step":    0,
				"message": "Self-description accepted.",
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "invalid_input", Description: "Description is required"},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.onboard.status", "ts.onboard.submit"},
		Since:        "v0.3.7",
		Visibility:   "internal",
	})

	r.Register(&ToolDoc{
		Name:     "ts.onboard.submit",
		Category: "onboarding",
		Summary:  "Submit a challenge response during onboarding.",
		Description: `Submits the agent's response to an onboarding challenge. Accepts
arbitrary key-value fields depending on the challenge type.`,
		Parameters:     []ParamDoc{},
		RequiredParams: []string{},
		Returns: ReturnDoc{
			Description: "Returns the submission result.",
			Example: map[string]interface{}{
				"status":  "submitted",
				"step":    2,
				"message": "Challenge response recorded.",
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "invalid_state", Description: "No active challenge to submit against"},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.onboard.next_challenge", "ts.onboard.status"},
		Since:        "v0.3.7",
		Visibility:   "internal",
	})

	r.Register(&ToolDoc{
		Name:     "ts.onboard.next_challenge",
		Category: "onboarding",
		Summary:  "Request the next onboarding challenge.",
		Description: `Retrieves the next challenge in the onboarding sequence for the authenticated agent.`,
		Parameters:     []ParamDoc{},
		RequiredParams: []string{},
		Returns: ReturnDoc{
			Description: "Returns the next challenge details.",
			Example: map[string]interface{}{
				"step":        3,
				"challenge":   "Describe how you would handle a task dependency conflict.",
				"type":        "open_ended",
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "invalid_state", Description: "No more challenges available"},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.onboard.submit", "ts.onboard.status"},
		Since:        "v0.3.7",
		Visibility:   "internal",
	})

	r.Register(&ToolDoc{
		Name:     "ts.onboard.complete",
		Category: "onboarding",
		Summary:  "Complete the onboarding process.",
		Description: `Finalizes the onboarding process for the authenticated agent.
All challenges must be submitted before completion.`,
		Parameters:     []ParamDoc{},
		RequiredParams: []string{},
		Returns: ReturnDoc{
			Description: "Returns the completion status.",
			Example: map[string]interface{}{
				"status":  "onboarding_complete",
				"message": "Welcome to Taskschmiede. You are now fully onboarded.",
			},
		},
		Errors: []ErrorDoc{
			{Code: "not_authenticated", Description: "No active login for this session"},
			{Code: "invalid_state", Description: "Not all challenges have been completed"},
		},
		RequiresAuth: true,
		RelatedTools: []string{"ts.onboard.status", "ts.onboard.health"},
		Since:        "v0.3.7",
		Visibility:   "internal",
	})
}
