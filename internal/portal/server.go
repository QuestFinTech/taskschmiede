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


// Package portal provides the member portal web UI for Taskschmiede.
// The portal is a standalone HTTP server that communicates with the
// Taskschmiede REST API (no direct database access). It serves the
// public-facing member portal at my.taskschmiede.dev.
package portal

import (
	"bytes"
	"context"
	"crypto/rand"
	"embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/QuestFinTech/taskschmiede/internal/i18n"
	"github.com/QuestFinTech/taskschmiede/internal/static"
	"github.com/QuestFinTech/taskschmiede/internal/storage"
	"github.com/QuestFinTech/taskschmiede/internal/timefmt"
)

// emailRegex validates email format requiring a proper TLD.
var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?)*\.[a-zA-Z]{2,}$`)

// validatePassword checks if password meets security requirements.
// Returns an i18n key on failure, or "" on success.
func validatePassword(password string) string {
	if len(password) < 12 {
		return "errors.password_too_short"
	}
	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, c := range password {
		switch {
		case unicode.IsUpper(c):
			hasUpper = true
		case unicode.IsLower(c):
			hasLower = true
		case unicode.IsDigit(c):
			hasDigit = true
		case unicode.IsPunct(c) || unicode.IsSymbol(c):
			hasSpecial = true
		}
	}
	if !hasUpper {
		return "errors.password_no_upper"
	}
	if !hasLower {
		return "errors.password_no_lower"
	}
	if !hasDigit {
		return "errors.password_no_digit"
	}
	if !hasSpecial {
		return "errors.password_no_special"
	}
	return ""
}

//go:embed templates/*.html
var templatesFS embed.FS

const cookieName = "portal_token"

// authRateLimiter provides per-IP rate limiting for authentication endpoints.
// It uses a simple sliding window counter: max 5 attempts per minute per IP.
type authRateLimiter struct {
	mu      sync.Mutex
	entries map[string]*authRateEntry
}

type authRateEntry struct {
	attempts []time.Time
}

func newAuthRateLimiter() *authRateLimiter {
	rl := &authRateLimiter{entries: make(map[string]*authRateEntry)}
	go rl.cleanup()
	return rl
}

// allow returns true if the IP is within the rate limit (5 attempts per 60s).
func (rl *authRateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := storage.UTCNow()
	cutoff := now.Add(-60 * time.Second)

	entry, ok := rl.entries[ip]
	if !ok {
		entry = &authRateEntry{}
		rl.entries[ip] = entry
	}

	// Remove expired attempts
	valid := entry.attempts[:0]
	for _, t := range entry.attempts {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	entry.attempts = valid

	if len(entry.attempts) >= 5 {
		return false
	}
	entry.attempts = append(entry.attempts, now)
	return true
}

func (rl *authRateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		rl.mu.Lock()
		cutoff := storage.UTCNow().Add(-2 * time.Minute)
		for ip, entry := range rl.entries {
			if len(entry.attempts) == 0 || entry.attempts[len(entry.attempts)-1].Before(cutoff) {
				delete(rl.entries, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// extractClientIP gets the client IP from the request.
// Prefers X-Real-IP (set by NGINX to $remote_addr, not spoofable)
// over X-Forwarded-For (which can be forged by clients).
func extractClientIP(r *http.Request) string {
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if idx := strings.IndexByte(xff, ','); idx != -1 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// Server handles the member portal web UI.
type Server struct {
	rest        *RESTClient
	templates   map[string]*template.Template
	addr        string
	supportURL    string // support agent base URL for /api/case
	supportAPIKey string // API key for authenticating portal-to-support requests
	secure        bool
	showAbout   bool
	version     string
	i18n        *i18n.Bundle
	authLimiter *authRateLimiter
}

// ServerConfig holds configuration for the portal server.
type ServerConfig struct {
	ListenAddr      string
	APIURL          string
	SupportAgentURL    string // e.g. http://localhost:9002
	SupportAgentAPIKey string // shared secret for portal-to-support auth
	Secure             bool
	Version         string
	ShowAbout       bool
}

// NewServer creates a new portal server backed by the REST API.
func NewServer(cfg *ServerConfig) (*Server, error) {
	bundle, err := i18n.New()
	if err != nil {
		return nil, fmt.Errorf("i18n: %w", err)
	}

	templates := make(map[string]*template.Template)

	pages := []string{
		"login.html",
		"register.html",
		"verify.html",
		"forgot_password.html",
		"reset_password.html",
		"setup_configure.html",
		"dashboard.html",
		"profile.html",
		"agents.html",
		"agent_detail.html",
		"orgs.html",
		"org_detail.html",
		"endeavours.html",
		"endeavour_detail.html",
		"rituals.html",
		"ritual_detail.html",
		"tasks.html",
		"task_detail.html",
		"demands.html",
		"demand_detail.html",
		"activity.html",
		"alerts.html",
		"usage.html",
		"messages.html",
		"teams.html",
		"team_detail.html",
		"admin_overview.html",
		"admin_users.html",
		"admin_user_detail.html",
		"admin_resources.html",
		"admin_resource_detail.html",
		"admin_rituals.html",
		"admin_ritual_detail.html",
		"admin_template_detail.html",
		"admin_messages.html",
		"admin_audit.html",
		"entity_changes.html",
		"admin_settings.html",
		"admin_translations.html",
		"admin_content_guard.html",
		"admin_taskschmied.html",
		"report_view.html",
		"about.html",
		"support.html",
		"login_2fa.html",
		"accept_terms.html",
		"complete_profile.html",
		"privacy_data.html",
	}

	funcMap := template.FuncMap{
		"fmtFloat": func(format string, v interface{}) string {
			switch f := v.(type) {
			case *float64:
				if f == nil {
					return ""
				}
				return fmt.Sprintf(format, *f)
			case float64:
				return fmt.Sprintf(format, f)
			default:
				return fmt.Sprintf("%v", v)
			}
		},
		"t": func(lang, key string, args ...interface{}) string {
			return bundle.T(lang, key, args...)
		},
		"tHTML": func(lang, key string, args ...interface{}) template.HTML {
			return template.HTML(bundle.T(lang, key, args...))
		},
		"tp": func(lang, key string, count int) string {
			return bundle.Tp(lang, key, count)
		},
		"fmtDateTime": timefmt.FormatDateTime,
		"fmtDate":     timefmt.FormatDate,
		"fmtTime":     timefmt.FormatTime,
		"truncDate": func(s string) string {
			if len(s) >= 10 {
				return s[:10]
			}
			return s
		},
		"upper":    strings.ToUpper,
		"firstName": func(name string) string {
			if i := strings.IndexByte(name, ' '); i > 0 {
				return name[:i]
			}
			return name
		},
		"joinLines": func(lines []string) string { return strings.Join(lines, "\n") },
		"fmtLimit": func(v interface{}) string {
			switch n := v.(type) {
			case float64:
				if n < 0 {
					return "unlimited"
				}
				return fmt.Sprintf("%.0f", n)
			case int:
				if n < 0 {
					return "unlimited"
				}
				return fmt.Sprintf("%d", n)
			default:
				return fmt.Sprintf("%v", v)
			}
		},
		"tierPct": func(usage, limit interface{}) int {
			toInt := func(v interface{}) int {
				switch n := v.(type) {
				case float64:
					return int(n)
				case int:
					return n
				default:
					return 0
				}
			}
			u := toInt(usage)
			l := toInt(limit)
			if l <= 0 {
				if u > 0 {
					return 10 // show a sliver for unlimited
				}
				return 0
			}
			pct := (u * 100) / l
			if pct > 100 {
				pct = 100
			}
			return pct
		},
		"add": func(a, b int) int {
			return a + b
		},
		"sub": func(a, b int) int {
			r := a - b
			if r < 0 {
				r = 0
			}
			return r
		},
		"sparkPct": func(value, max, height int) int {
			if max <= 0 {
				return 0
			}
			px := (value * height) / max
			if px < 1 && value > 0 {
				px = 1
			}
			return px
		},
		"metaVal": func(meta map[string]interface{}, key string) string {
			if meta == nil {
				return ""
			}
			v, ok := meta[key]
			if !ok {
				return ""
			}
			return fmt.Sprintf("%v", v)
		},
		"fmtSchedule": func(sched map[string]interface{}) string {
			if sched == nil {
				return ""
			}
			typ, _ := sched["type"].(string)
			if typ == "manual" {
				return "Manual"
			}
			if typ == "interval" {
				every, _ := sched["every"].(string)
				on, _ := sched["on"].(string)
				if every == "" {
					return "Interval"
				}
				// Expand unit abbreviations for display.
				result := "Every " + every
				for _, pair := range [][2]string{{"m", " minutes"}, {"h", " hours"}, {"d", " days"}, {"w", " weeks"}} {
					if strings.HasSuffix(every, pair[0]) {
						num := strings.TrimSuffix(every, pair[0])
						if num != "" {
							result = "Every " + num + pair[1]
							break
						}
					}
				}
				if on != "" {
					result += " on " + strings.ToUpper(on[:1]) + on[1:]
				}
				return result
			}
			expr, _ := sched["expression"].(string)
			if expr == "" {
				return typ
			}
			desc := cronDesc(expr)
			if desc != "" {
				return desc + " (" + expr + ")"
			}
			return expr
		},
		"fmtMethodology": func(id string) string {
			// Strip mth_ prefix and format: mth_design_sprint -> Design Sprint
			name := strings.TrimPrefix(id, "mth_")
			name = strings.ReplaceAll(name, "_", " ")
			// Title case
			words := strings.Fields(name)
			for i, w := range words {
				if len(w) > 0 {
					words[i] = strings.ToUpper(w[:1]) + w[1:]
				}
			}
			return strings.Join(words, " ")
		},
	}

	for _, page := range pages {
		tmpl, err := template.New("").Funcs(funcMap).ParseFS(templatesFS, "templates/base.html", "templates/nav.html", "templates/"+page)
		if err != nil {
			return nil, fmt.Errorf("parse template %s: %w", page, err)
		}
		templates[page] = tmpl
	}

	supportURL := cfg.SupportAgentURL

	return &Server{
		rest:        NewRESTClient(cfg.APIURL),
		templates:   templates,
		addr:        cfg.ListenAddr,
		supportURL:    supportURL,
		supportAPIKey: cfg.SupportAgentAPIKey,
		secure:        cfg.Secure,
		showAbout:   cfg.ShowAbout,
		version:     cfg.Version,
		i18n:        bundle,
		authLimiter: newAuthRateLimiter(),
	}, nil
}

// maxBytesMiddleware limits request body size to 1 MB for all requests.
func maxBytesMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		next.ServeHTTP(w, r)
	})
}

// contextKey is an unexported type for context keys in this package.
type contextKey string

// nonceKey is the context key for the CSP nonce.
const nonceKey contextKey = "csp-nonce"

// generateNonce creates a cryptographically random base64-encoded nonce.
func generateNonce() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return base64.StdEncoding.EncodeToString(b)
}

// getNonce retrieves the CSP nonce from the request context.
func getNonce(r *http.Request) string {
	if v, ok := r.Context().Value(nonceKey).(string); ok {
		return v
	}
	return ""
}

// securityHeadersMiddleware adds HSTS, CSP, and cross-origin isolation headers.
// Generates a per-request nonce for inline script execution.
func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nonce := generateNonce()
		ctx := context.WithValue(r.Context(), nonceKey, nonce)
		r = r.WithContext(ctx)

		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; "+
				"script-src 'self' 'nonce-"+nonce+"'; "+
				"style-src 'self' 'nonce-"+nonce+"'; "+
				"img-src 'self' data:; "+
				"font-src 'self'; "+
				"connect-src 'self'; "+
				"object-src 'none'; "+
				"base-uri 'self'; "+
				"form-action 'self'; "+
				"frame-ancestors 'none'")
		w.Header().Set("Cross-Origin-Embedder-Policy", "credentialless")
		w.Header().Set("Cross-Origin-Opener-Policy", "same-origin")
		w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
		next.ServeHTTP(w, r)
	})
}

// Start starts the portal server.
func (s *Server) Start() error {
	mux := http.NewServeMux()

	// Static assets (favicon, icons)
	static.RegisterHandlers(mux)

	// Public routes
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/setup", s.handleSetup)
	mux.HandleFunc("/login", s.handleLogin)
	mux.HandleFunc("/login/2fa", s.handleLogin2FA)
	mux.HandleFunc("/register", s.handleRegister)
	mux.HandleFunc("/verify", s.handleVerify)
	mux.HandleFunc("/logout", s.handleLogout)
	mux.HandleFunc("/forgot-password", s.handleForgotPassword)
	mux.HandleFunc("/reset-password", s.handleResetPassword)

	// Authenticated routes
	mux.HandleFunc("/complete-profile", s.requireAuth(s.handleCompleteProfile))
	mux.HandleFunc("/accept-terms", s.requireAuth(s.handleAcceptTerms))
	mux.HandleFunc("/privacy-data", s.requireAuth(s.handlePrivacyData))
	mux.HandleFunc("/privacy-data/export", s.requireAuth(s.handleDataExport))
	mux.HandleFunc("/dashboard", s.requireAuth(s.handleDashboard))
	mux.HandleFunc("/profile", s.requireAuth(s.handleProfile))
	mux.HandleFunc("/agents", s.requireAuth(s.handleAgents))
	mux.HandleFunc("/agents/{id}", s.requireAuth(s.handleAgentDetail))
	mux.HandleFunc("/organizations", s.requireAuth(s.handleOrganizations))
	mux.HandleFunc("/organizations/{id}", s.requireAuth(s.handleOrganizationDetail))
	mux.HandleFunc("/organizations/{id}/export", s.requireAuth(s.handleOrganizationExport))
	mux.HandleFunc("/endeavours", s.requireAuth(s.handleEndeavours))
	mux.HandleFunc("/endeavours/{id}", s.requireAuth(s.handleEndeavourDetail))
	mux.HandleFunc("/endeavours/{id}/export", s.requireAuth(s.handleEndeavourExport))
	mux.HandleFunc("/rituals", s.requireAuth(s.handleRituals))
	mux.HandleFunc("/rituals/{id}", s.requireAuth(s.handleRitualDetail))
	mux.HandleFunc("/tasks", s.requireAuth(s.handleTasks))
	mux.HandleFunc("/tasks/{id}", s.requireAuth(s.handleTaskDetail))
	mux.HandleFunc("/demands", s.requireAuth(s.handleDemands))
	mux.HandleFunc("/demands/{id}", s.requireAuth(s.handleDemandDetail))
	mux.HandleFunc("/activity", s.requireAuth(s.handleActivity))
	mux.HandleFunc("/alerts", s.requireAuth(s.handleAlerts))
	mux.HandleFunc("/usage", s.requireAuth(s.handleUsage))
	mux.HandleFunc("/messages", s.requireAuth(s.handleMessages))
	mux.HandleFunc("/messages/{id}", s.requireAuth(s.handleMessageThread))
	mux.HandleFunc("/teams", s.requireAuth(s.handleTeams))
	mux.HandleFunc("/teams/{id}", s.requireAuth(s.handleTeamDetail))
	mux.HandleFunc("/entity-changes", s.requireAuth(s.handleUnifiedActivity))

	// Support
	mux.HandleFunc("/support", s.requireAuth(s.handleSupport))

	// About
	mux.HandleFunc("/about", s.requireAuth(s.handleAbout))

	// Reports
	mux.HandleFunc("/reports/{scope}/{id}", s.requireAuth(s.handleReport))

	// KPI data (JSON proxy, admin-only)
	mux.HandleFunc("/kpi/current.json", s.requireAdminAuth(s.handleKPICurrent))
	mux.HandleFunc("/kpi/history.json", s.requireAdminAuth(s.handleKPIHistory))

	// Admin routes (protected by requireAdminAuth)
	mux.HandleFunc("/admin/overview", s.requireAdminAuth(s.handleAdminOverview))
	mux.HandleFunc("/admin/users", s.requireAdminAuth(s.handleAdminUsers))
	mux.HandleFunc("/admin/users/{id}", s.requireAdminAuth(s.handleAdminUserDetail))
	mux.HandleFunc("/admin/resources", s.requireAdminAuth(s.handleAdminResources))
	mux.HandleFunc("/admin/resources/{id}", s.requireAdminAuth(s.handleAdminResourceDetail))
	mux.HandleFunc("/admin/rituals", s.requireAdminAuth(s.handleAdminRituals))
	mux.HandleFunc("/admin/rituals/{id}", s.requireAdminAuth(s.handleAdminRitualDetail))
	mux.HandleFunc("/admin/templates/{id}", s.requireAdminAuth(s.handleAdminTemplateDetail))
	mux.HandleFunc("/admin/messages", s.requireAdminAuth(s.handleAdminMessages))
	mux.HandleFunc("/admin/audit", s.requireAdminAuth(s.handleAdminAudit))
	mux.HandleFunc("/admin/settings", s.requireAdminAuth(s.handleAdminSettings))
	mux.HandleFunc("/admin/translations", s.requireAdminAuth(s.handleAdminTranslations))
	mux.HandleFunc("/admin/content-guard", s.requireAdminAuth(s.handleAdminContentGuard))
	mux.HandleFunc("/admin/taskschmied", s.requireAdminAuth(s.handleAdminTaskschmied))

	server := &http.Server{
		Addr:           s.addr,
		Handler:        securityHeadersMiddleware(maxBytesMiddleware(mux)),
		ReadTimeout:    15 * time.Second,
		WriteTimeout:   30 * time.Second,
		IdleTimeout:    60 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	slog.Info("Starting member portal", "address", s.addr)
	return server.ListenAndServe()
}

// --- Auth helpers ---

func getToken(r *http.Request) string {
	cookie, err := r.Cookie(cookieName)
	if err != nil || cookie.Value == "" {
		return ""
	}
	return cookie.Value
}

// setToken is a helper for the portal server.
func (s *Server) setToken(w http.ResponseWriter, token string, expiresAt time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		Secure:   s.secure,
		SameSite: http.SameSiteStrictMode,
	})
}

func clearToken(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})
}

// requireAuth is a helper for the portal server.
func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := getToken(r)
		if token == "" {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		user, err := s.rest.Whoami(token)
		if err != nil {
			clearToken(w)
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		// Check for profile completion (two-phase registration).
		// Users who verified email but haven't completed their profile
		// are redirected to /complete-profile.
		if r.URL.Path != "/complete-profile" && r.URL.Path != "/accept-terms" && r.URL.Path != "/logout" {
			if pc, ok := user["profile_complete"]; ok {
				if complete, ok := pc.(bool); ok && !complete {
					http.Redirect(w, r, "/complete-profile", http.StatusSeeOther)
					return
				}
			}
		}
		// Check for pending consent (terms/privacy version updates).
		// Redirect to accept-terms page unless already on it.
		if r.URL.Path != "/accept-terms" && r.URL.Path != "/complete-profile" {
			if pc, ok := user["pending_consents"]; ok {
				if pending, ok := pc.([]interface{}); ok && len(pending) > 0 {
					http.Redirect(w, r, "/accept-terms", http.StatusSeeOther)
					return
				}
			}
		}
		next(w, r)
	}
}

// requireAdminAuth wraps a handler with authentication + master admin check.
// Returns 404 to non-admins (avoids leaking information about admin features).
func (s *Server) requireAdminAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := getToken(r)
		if token == "" {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		user, err := s.rest.Whoami(token)
		if err != nil {
			clearToken(w)
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		if !isAdmin(user) {
			http.NotFound(w, r)
			return
		}
		next(w, r)
	}
}

// isAdmin checks if a user (from Whoami) has master admin privileges.
func isAdmin(user map[string]interface{}) bool {
	if user == nil {
		return false
	}
	isMA, ok := user["is_admin"].(bool)
	return ok && isMA
}

// userOrgRole returns the user's role in their first org where they are
// owner or admin. Returns the orgID and role, or empty strings if none.
func userOrgAdminRole(user map[string]interface{}) (orgID, role string) {
	if user == nil {
		return "", ""
	}
	orgs, ok := user["organizations"].(map[string]interface{})
	if !ok {
		return "", ""
	}
	for id, r := range orgs {
		rs, ok := r.(string)
		if !ok {
			continue
		}
		if rs == "owner" || rs == "admin" {
			return id, rs
		}
	}
	return "", ""
}

// canManageTeams returns true if the user is admin or an org admin/owner.
func canManageTeams(user map[string]interface{}) bool {
	if isAdmin(user) {
		return true
	}
	orgID, _ := userOrgAdminRole(user)
	return orgID != ""
}

// userPrimaryOrgID returns the first org ID where the user has any role.
func userPrimaryOrgID(user map[string]interface{}) string {
	if user == nil {
		return ""
	}
	orgs, ok := user["organizations"].(map[string]interface{})
	if !ok {
		return ""
	}
	for id := range orgs {
		return id
	}
	return ""
}

// queryIntParam extracts an integer query parameter with a default value.
func queryIntParam(r *http.Request, name string, defaultVal int) int {
	v := r.URL.Query().Get(name)
	if v == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return defaultVal
	}
	return n
}

// --- Rendering ---

// PageData contains common data for all pages.
type PageData struct {
	Title          string
	Error          string
	Success        string
	Data           interface{}
	CSRFToken      string
	Nonce          string // CSP nonce for inline scripts
	User           map[string]interface{}
	IsAdmin           bool
	IsEndeavourAdmin  bool // true if user is admin/owner of any endeavour
	EmailCopy         bool
	Lang           string
	Dir            string
	Timezone       string
	Languages      []i18n.Language
	CurrentPage    string
	AlertBadge     int // unresolved alert count for nav badge
	HighAlertCount int // high-severity alerts for persistent banner
	ShowAbout      bool
	ShowSupport    bool
}

// render is a helper for the portal server.
func (s *Server) render(w http.ResponseWriter, r *http.Request, name string, data PageData) {
	tmpl, ok := s.templates[name]
	if !ok {
		log.Printf("Template not found: %s", name)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if data.Nonce == "" {
		data.Nonce = getNonce(r)
	}
	if data.Lang == "" {
		// Check user record first (avoids extra API call for authenticated pages).
		if data.User != nil {
			if lang, ok := data.User["lang"].(string); ok && lang != "" && s.i18n.HasLanguage(lang) {
				data.Lang = lang
			}
		}
		if data.Lang == "" {
			data.Lang = s.resolveLang(r)
		}
	}
	if data.Timezone == "" && data.User != nil {
		if tz, ok := data.User["timezone"].(string); ok && tz != "" {
			data.Timezone = tz
		}
	}
	if data.Timezone == "" {
		data.Timezone = "UTC"
	}
	if data.User != nil {
		if ec, ok := data.User["email_copy"].(bool); ok {
			data.EmailCopy = ec
		}
	}
	// Set IsAdmin from user data (Whoami returns is_admin at top level).
	if data.User != nil && !data.IsAdmin {
		if isMA, ok := data.User["is_admin"].(bool); ok && isMA {
			data.IsAdmin = true
		}
	}
	// Set IsEndeavourAdmin: true if user is admin/owner of any endeavour.
	if data.User != nil && !data.IsEndeavourAdmin {
		if data.IsAdmin {
			data.IsEndeavourAdmin = true
		} else if edvs, ok := data.User["endeavours"].(map[string]interface{}); ok {
			for _, role := range edvs {
				if r, ok := role.(string); ok && (r == "admin" || r == "owner") {
					data.IsEndeavourAdmin = true
					break
				}
			}
		}
	}
	// Fetch alert stats for nav badge and persistent banner.
	if data.User != nil {
		if token := getToken(r); token != "" {
			if stats, err := s.rest.MyAlertStats(token); err == nil && stats != nil {
				data.AlertBadge = stats.Flagged
				// Fetch high-severity count from full alert list (threshold 70).
				_, highTotal, _ := s.rest.ListMyAlerts(token, 70, 0, 0)
				data.HighAlertCount = highTotal
			}
		}
	}
	data.ShowAbout = s.showAbout
	data.ShowSupport = s.supportURL != ""
	data.Dir = s.i18n.Dir(data.Lang)
	data.Languages = s.i18n.Languages()
	data.CSRFToken = s.setCSRFToken(w)
	if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// resolveLang determines the language for unauthenticated pages.
// For authenticated pages, render() reads lang from data.User directly.
// Fallback: Accept-Language header > "en".
func (s *Server) resolveLang(r *http.Request) string {
	if lang := s.i18n.MatchAcceptLanguage(r.Header.Get("Accept-Language")); lang != "" {
		return lang
	}
	return "en"
}

// msg translates a server-side message using the user's language preference.
// For authenticated pages pass the user map from Whoami; for public pages pass nil.
func (s *Server) msg(r *http.Request, user map[string]interface{}, key string, args ...interface{}) string {
	lang := ""
	if user != nil {
		if l, ok := user["lang"].(string); ok && l != "" && s.i18n.HasLanguage(l) {
			lang = l
		}
	}
	if lang == "" {
		lang = s.resolveLang(r)
	}
	return s.i18n.T(lang, key, args...)
}

func (s *Server) csrfFailed(w http.ResponseWriter, r *http.Request, page string) {
	w.WriteHeader(http.StatusForbidden)
	s.render(w, r, page, PageData{Title: "Error", Error: s.msg(r, nil, "errors.invalid_form")})
}

// --- Handlers ---

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	// Check if setup is needed (first-run wizard).
	status, err := s.rest.SetupStatus()
	if err == nil && status.NeedsSetup {
		http.Redirect(w, r, "/setup", http.StatusSeeOther)
		return
	}

	token := getToken(r)
	if token != "" {
		if _, err := s.rest.Whoami(token); err == nil {
			http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
			return
		}
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// handleSetup serves the setup portal page.
func (s *Server) handleSetup(w http.ResponseWriter, r *http.Request) {
	status, _ := s.rest.SetupStatus()
	phase := ""
	if status != nil {
		phase = status.Phase
	}

	switch phase {
	case "done":
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	case "verify":
		http.Redirect(w, r, "/verify?setup=1&email="+url.QueryEscape(status.Email), http.StatusSeeOther)
	case "configure":
		s.handleSetupConfigure(w, r)
	default:
		http.Redirect(w, r, "/register?setup=1", http.StatusSeeOther)
	}
}


// handleSetupConfigure serves the setup configure portal page.
func (s *Server) handleSetupConfigure(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		if !validateCSRF(r) {
			s.csrfFailed(w, r, "setup_configure.html")
			return
		}

		timezone := r.FormValue("timezone")
		defaultLanguage := r.FormValue("default_language")

		formData := map[string]interface{}{
			"Timezone":        timezone,
			"DefaultLanguage": defaultLanguage,
			"Languages":       s.i18n.Languages(),
		}

		if timezone == "" || defaultLanguage == "" {
			s.render(w, r, "setup_configure.html", PageData{Title: "Configure", Error: s.msg(r, nil, "errors.required_fields"), Data: formData})
			return
		}

		if err := s.rest.SetupConfigure(timezone, defaultLanguage); err != nil {
			s.render(w, r, "setup_configure.html", PageData{Title: "Configure", Error: err.Error(), Data: formData})
			return
		}

		http.Redirect(w, r, "/setup", http.StatusSeeOther)
		return
	}

	data := map[string]interface{}{
		"Languages":       s.i18n.Languages(),
		"DefaultLanguage": "en",
	}
	s.render(w, r, "setup_configure.html", PageData{Title: "Configure", Data: data})
}

// handleLogin serves the login portal page.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	// Redirect to setup if no admin exists yet.
	if r.Method == http.MethodGet {
		if status, err := s.rest.SetupStatus(); err == nil && status.NeedsSetup {
			http.Redirect(w, r, "/setup", http.StatusSeeOther)
			return
		}
	}

	data := PageData{Title: "Sign In"}
	if r.URL.Query().Get("reset") == "success" {
		data.Success = s.msg(r, nil, "success.password_reset")
	}
	switch r.URL.Query().Get("msg") {
	case "registration_disabled":
		data.Error = s.msg(r, nil, "errors.registration_disabled")
	case "registration_full":
		data.Error = s.msg(r, nil, "errors.registration_full")
	}

	if r.Method == http.MethodPost {
		if !validateCSRF(r) {
			s.csrfFailed(w, r, "login.html")
			return
		}

		ip := extractClientIP(r)
		if !s.authLimiter.allow(ip) {
			data.Error = s.msg(r, nil, "errors.too_many_attempts")
			w.Header().Set("Retry-After", "60")
			s.render(w, r, "login.html", data)
			return
		}

		emailAddr := r.FormValue("email")
		password := r.FormValue("password")

		result, err := s.rest.Login(emailAddr, password)
		if err != nil {
			if restErr, ok := err.(*RESTError); ok && restErr.Code == "account_suspended" {
				data.Error = s.msg(r, nil, "errors.account_suspended")
			} else {
				data.Error = s.msg(r, nil, "errors.invalid_credentials")
			}
			s.render(w, r, "login.html", data)
			return
		}

		// Check if 2FA is required.
		if result.Status == "2fa_required" {
			http.SetCookie(w, &http.Cookie{
				Name:     "pending_2fa",
				Value:    result.PendingToken,
				Path:     "/login/2fa",
				HttpOnly: true,
				SameSite: http.SameSiteStrictMode,
				Secure:   s.secure,
				MaxAge:   300,
			})
			http.Redirect(w, r, "/login/2fa", http.StatusSeeOther)
			return
		}

		expiresAt, _ := time.Parse(time.RFC3339, result.ExpiresAt)
		s.setToken(w, result.Token, expiresAt)
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
		return
	}

	s.render(w, r, "login.html", data)
}

// handleLogin2FA serves the 2FA login portal page.
func (s *Server) handleLogin2FA(w http.ResponseWriter, r *http.Request) {
	// Read the pending token from the HttpOnly cookie.
	pendingToken := ""
	if c, err := r.Cookie("pending_2fa"); err == nil {
		pendingToken = c.Value
	}

	data := PageData{
		Title: "Two-Factor Authentication",
	}

	if r.Method == http.MethodPost {
		if !validateCSRF(r) {
			s.csrfFailed(w, r, "login_2fa.html")
			return
		}

		code := r.FormValue("code")
		if code == "" || pendingToken == "" {
			data.Error = s.msg(r, nil, "errors.required_fields")
			s.render(w, r, "login_2fa.html", data)
			return
		}

		result, err := s.rest.VerifyTOTP(pendingToken, code)
		if err != nil {
			data.Error = s.msg(r, nil, "errors.invalid_2fa_code")
			s.render(w, r, "login_2fa.html", data)
			return
		}

		// Clear the pending 2FA cookie.
		http.SetCookie(w, &http.Cookie{
			Name:     "pending_2fa",
			Value:    "",
			Path:     "/login/2fa",
			HttpOnly: true,
			MaxAge:   -1,
		})

		expiresAt, _ := time.Parse(time.RFC3339, result.ExpiresAt)
		s.setToken(w, result.Token, expiresAt)
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
		return
	}

	if pendingToken == "" {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	s.render(w, r, "login_2fa.html", data)
}

// handleRegister serves the register portal page.
func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	setupMode := r.URL.Query().Get("setup") == "1"

	// Check if self-registration is allowed (skip during setup).
	if !setupMode {
		if info, err := s.rest.GetInstanceInfo(); err == nil {
			if !info.AllowSelfRegistration {
				http.Redirect(w, r, "/login?msg=registration_disabled", http.StatusSeeOther)
				return
			}
			if !info.RegistrationOpen {
				http.Redirect(w, r, "/login?msg=registration_full", http.StatusSeeOther)
				return
			}
		}
	}

	renderPage := func(pd PageData) {
		if setupMode {
			if m, ok := pd.Data.(map[string]interface{}); ok {
				m["SetupMode"] = true
			} else if pd.Data == nil {
				pd.Data = map[string]interface{}{"SetupMode": true}
			}
		}
		pd.Languages = s.i18n.Languages()
		s.render(w, r, "register.html", pd)
	}

	if r.Method == http.MethodPost {
		if !validateCSRF(r) {
			s.csrfFailed(w, r, "register.html")
			return
		}

		ip := extractClientIP(r)
		if !s.authLimiter.allow(ip) {
			renderPage(PageData{Title: "Create Account", Error: s.msg(r, nil, "errors.too_many_attempts")})
			return
		}

		firstName := r.FormValue("first_name")
		lastName := r.FormValue("last_name")
		emailAddr := r.FormValue("email")
		password := r.FormValue("password")
		confirmPassword := r.FormValue("confirm_password")
		accountType := r.FormValue("account_type")
		companyName := r.FormValue("company_name")

		if accountType == "" {
			accountType = "private"
		}
		name := firstName + " " + lastName

		formData := map[string]interface{}{
			"FirstName":   firstName,
			"LastName":    lastName,
			"Email":       emailAddr,
			"AccountType": accountType,
			"CompanyName": companyName,
		}

		if firstName == "" || lastName == "" || emailAddr == "" || password == "" {
			renderPage(PageData{Title: "Create Account", Error: s.msg(r, nil, "errors.required_fields"), Data: formData})
			return
		}
		if !emailRegex.MatchString(emailAddr) {
			renderPage(PageData{Title: "Create Account", Error: s.msg(r, nil, "errors.invalid_email"), Data: formData})
			return
		}
		if password != confirmPassword {
			renderPage(PageData{Title: "Create Account", Error: s.msg(r, nil, "errors.passwords_not_match"), Data: formData})
			return
		}
		if errKey := validatePassword(password); errKey != "" {
			renderPage(PageData{Title: "Create Account", Error: s.msg(r, nil, errKey), Data: formData})
			return
		}
		if accountType == "business" && companyName == "" {
			renderPage(PageData{Title: "Create Account", Error: s.msg(r, nil, "errors.company_required"), Data: formData})
			return
		}

		lang := r.FormValue("lang")

		if setupMode {
			// During setup, use the admin setup endpoint which creates the
			// pending admin account and sends the verification email.
			result, setupErr := s.rest.SetupCreate(emailAddr, name, password, accountType, companyName)
			if setupErr != nil {
				renderPage(PageData{Title: "Create Account", Error: setupErr.Error(), Data: formData})
				return
			}
			verifyURL := "/verify?setup=1&email=" + url.QueryEscape(emailAddr)
			if !result.EmailSent && result.VerificationCode != "" {
				// Email not configured -- pass code via query param so verify page can display it.
				verifyURL += "&fallback_code=" + url.QueryEscape(result.VerificationCode)
			}
			http.Redirect(w, r, verifyURL, http.StatusSeeOther)
			return
		}

		opts := &RegisterOpts{
			AccountType: accountType,
			FirstName:   firstName,
			LastName:    lastName,
			CompanyName: companyName,
		}
		_, regErr := s.rest.Register(emailAddr, name, password, lang, opts)
		if regErr != nil {
			renderPage(PageData{Title: "Create Account", Error: regErr.Error(), Data: formData})
			return
		}

		http.Redirect(w, r, "/verify?email="+url.QueryEscape(emailAddr), http.StatusSeeOther)
		return
	}

	// Pre-fill email from query parameter (e.g. from /join page).
	formData := map[string]interface{}{}
	if emailParam := r.URL.Query().Get("email"); emailParam != "" {
		formData["Email"] = emailParam
	}
	renderPage(PageData{Title: "Create Account", Data: formData})
}

// handleVerify serves the verify portal page.
func (s *Server) handleVerify(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Referrer-Policy", "no-referrer")

	setupMode := r.URL.Query().Get("setup") == "1"
	if r.Method == http.MethodPost && r.FormValue("setup") == "1" {
		setupMode = true
	}

	// Auto-detect setup mode: if setup is not yet fully complete, treat this
	// as a setup verification even without the setup=1 param.
	// This handles the email link (no setup=1) and stale tabs (already verified).
	if !setupMode {
		ss, ssErr := s.rest.SetupStatus()
		if ssErr == nil && ss != nil {
			switch ss.Phase {
			case "verify":
				// Setup is pending verification -- treat as setup mode
				// (email link from setup doesn't include setup=1).
				setupMode = true
			case "configure":
				http.Redirect(w, r, "/setup", http.StatusSeeOther)
				return
			case "done":
				// Setup is complete. Only redirect bare /verify visits to /login.
				// When an email param is present, fall through to handle regular
				// user verification (registration, email change).
				if r.URL.Query().Get("email") == "" {
					http.Redirect(w, r, "/login", http.StatusSeeOther)
					return
				}
			}
		}
	}

	emailAddr := r.URL.Query().Get("email")
	if emailAddr == "" && r.Method == http.MethodPost {
		emailAddr = r.FormValue("email")
	}

	if emailAddr == "" {
		s.render(w, r, "verify.html", PageData{
			Title: "Verify Email",
			Data: map[string]interface{}{
				"Lookup": true,
			},
		})
		return
	}

	// In setup mode, get expiry info from SetupStatus; otherwise use VerificationStatus.
	var foundEmail string
	var expiresAtStr string
	var expired bool
	var found bool

	if setupMode {
		ss, ssErr := s.rest.SetupStatus()
		if ssErr == nil && ss.PendingVerification {
			found = true
			foundEmail = ss.Email
			expiresAtStr = ss.ExpiresAt
		}
	} else {
		vs, vsErr := s.rest.VerificationStatus(emailAddr)
		if vsErr == nil && vs.Found {
			found = true
			foundEmail = vs.Email
			expiresAtStr = vs.ExpiresAt
			expired = vs.Expired
		}
	}

	if !found {
		// If setup already advanced past verify (e.g., verified in another tab),
		// redirect instead of showing an error.
		if setupMode {
			ss2, _ := s.rest.SetupStatus()
			if ss2 != nil && (ss2.Phase == "configure" || ss2.Phase == "done") {
				http.Redirect(w, r, "/setup", http.StatusSeeOther)
				return
			}
		}
		s.render(w, r, "verify.html", PageData{
			Title: "Verify Email",
			Error: s.msg(r, nil, "errors.no_pending_verification"),
			Data: map[string]interface{}{
				"Email":         emailAddr,
				"ExpiresIn":     "0m 00s",
				"ExpiresAtUnix": 0,
				"SetupMode":     setupMode,
			},
		})
		return
	}

	expiresAt, _ := time.Parse(time.RFC3339, expiresAtStr)
	now := time.Now().UTC()
	expiresIn := expiresAt.Sub(now)
	if expiresIn < 0 {
		expiresIn = 0
	}

	// If email was not configured, the fallback code is passed via query param.
	fallbackCode := r.URL.Query().Get("fallback_code")

	data := PageData{
		Title: "Verify Email",
		Data: map[string]interface{}{
			"Email":         foundEmail,
			"ExpiresIn":     fmt.Sprintf("%dm %02ds", int(expiresIn.Minutes()), int(expiresIn.Seconds())%60),
			"ExpiresAtUnix": expiresAt.Unix(),
			"SetupMode":     setupMode,
			"FallbackCode":  fallbackCode,
		},
	}

	if expired {
		data.Error = s.msg(r, nil, "errors.code_expired")
	}

	code := r.URL.Query().Get("code")
	if code == "" && r.Method == http.MethodPost {
		code = r.FormValue("code")
	}

	// Handle resend.
	if r.Method == http.MethodPost && r.FormValue("action") == "resend" {
		if !validateCSRF(r) {
			s.csrfFailed(w, r, "verify.html")
			return
		}
		if setupMode {
			result, resendErr := s.rest.SetupResend()
			if resendErr != nil {
				data.Error = s.msg(r, nil, "errors.failed_resend_code")
				s.render(w, r, "verify.html", data)
				return
			}
			if result != nil {
				newExpiresAt, _ := time.Parse(time.RFC3339, result.ExpiresAt)
				newExpiresIn := newExpiresAt.Sub(time.Now().UTC())
				data.Data = map[string]interface{}{
					"Email":         result.Email,
					"ExpiresIn":     fmt.Sprintf("%dm %02ds", int(newExpiresIn.Minutes()), int(newExpiresIn.Seconds())%60),
					"ExpiresAtUnix": newExpiresAt.Unix(),
					"SetupMode":     true,
				}
			}
		} else {
			if resendErr := s.rest.ResendVerification(emailAddr); resendErr != nil {
				data.Error = s.msg(r, nil, "errors.failed_resend_code")
				s.render(w, r, "verify.html", data)
				return
			}
			newVS, _ := s.rest.VerificationStatus(emailAddr)
			if newVS != nil && newVS.Found {
				newExpiresAt, _ := time.Parse(time.RFC3339, newVS.ExpiresAt)
				newExpiresIn := newExpiresAt.Sub(time.Now().UTC())
				data.Data = map[string]interface{}{
					"Email":         newVS.Email,
					"ExpiresIn":     fmt.Sprintf("%dm %02ds", int(newExpiresIn.Minutes()), int(newExpiresIn.Seconds())%60),
					"ExpiresAtUnix": newExpiresAt.Unix(),
				}
			}
		}
		data.Success = s.msg(r, nil, "success.new_code_sent")
		s.render(w, r, "verify.html", data)
		return
	}

	if code != "" {
		if r.Method == http.MethodPost && !validateCSRF(r) {
			// On CSRF failure, redirect to a fresh page rather than showing
			// an error -- the user likely has a stale tab open.
			freshURL := "/verify?email=" + url.QueryEscape(emailAddr)
			if setupMode {
				freshURL += "&setup=1"
			}
			http.Redirect(w, r, freshURL, http.StatusSeeOther)
			return
		}

		if setupMode {
			// Check if setup already completed (e.g., verified in another tab).
			ss, _ := s.rest.SetupStatus()
			if ss != nil && (ss.Phase == "configure" || ss.Phase == "done") {
				http.Redirect(w, r, "/setup", http.StatusSeeOther)
				return
			}

			// Use admin setup verify endpoint which auto-promotes to admin.
			verifyErr := s.rest.SetupVerify(code)
			if verifyErr != nil {
				data.Error = verifyErr.Error()
				s.render(w, r, "verify.html", data)
				return
			}
			// Redirect to /setup which will now show the configure phase.
			http.Redirect(w, r, "/setup", http.StatusSeeOther)
			return
		}

		result, verifyErr := s.rest.VerifyUser(emailAddr, code)
		if verifyErr != nil {
			data.Error = verifyErr.Error()
			s.render(w, r, "verify.html", data)
			return
		}

		// Auto-login: set session cookie and redirect to profile completion.
		if result != nil && result.Token != "" {
			s.setToken(w, result.Token, time.Now().UTC().Add(24*time.Hour))
			http.Redirect(w, r, "/complete-profile", http.StatusSeeOther)
			return
		}

		// Fallback if no token (should not happen).
		s.render(w, r, "verify.html", PageData{
			Title:   "Verify Email",
			Success: s.msg(r, nil, "success.welcome_verified", emailAddr),
			Data: map[string]interface{}{
				"Email":    emailAddr,
				"Verified": true,
			},
		})
		return
	}

	s.render(w, r, "verify.html", data)
}

// handleCompleteProfile serves the complete profile portal page.
func (s *Server) handleCompleteProfile(w http.ResponseWriter, r *http.Request) {
	token := getToken(r)
	user, _ := s.rest.Whoami(token)

	// If profile is already complete, redirect to dashboard.
	if pc, ok := user["profile_complete"]; ok {
		if complete, ok := pc.(bool); ok && complete {
			http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
			return
		}
	}

	// Determine account type from user data (captured at registration).
	accountType := "private"
	if ut, ok := user["account_type"]; ok {
		if s, ok := ut.(string); ok && s != "" {
			accountType = s
		}
	}

	if r.Method == http.MethodPost {
		if !validateCSRF(r) {
			s.csrfFailed(w, r, "complete_profile.html")
			return
		}

		street := r.FormValue("street")
		street2 := r.FormValue("street2")
		postalCode := r.FormValue("postal_code")
		city := r.FormValue("city")
		state := r.FormValue("state")
		country := r.FormValue("country")
		companyRegistration := r.FormValue("company_registration")
		vatNumber := r.FormValue("vat_number")
		acceptTerms := r.FormValue("accept_terms") == "on"
		acceptPrivacy := r.FormValue("accept_privacy") == "on"
		acceptDPA := r.FormValue("accept_dpa") == "on"
		ageDeclaration := r.FormValue("age_declaration") == "on"

		formData := map[string]interface{}{
			"AccountType":         accountType,
			"Street":              street,
			"Street2":             street2,
			"PostalCode":          postalCode,
			"City":                city,
			"State":               state,
			"Country":             country,
			"CompanyRegistration": companyRegistration,
			"VATNumber":           vatNumber,
		}

		if street == "" || postalCode == "" || city == "" || country == "" {
			s.render(w, r, "complete_profile.html", PageData{Title: "Complete Your Profile", User: user, Error: s.msg(r, user, "errors.address_required"), Data: formData})
			return
		}
		if !acceptTerms || !acceptPrivacy {
			s.render(w, r, "complete_profile.html", PageData{Title: "Complete Your Profile", User: user, Error: s.msg(r, user, "errors.consent_required"), Data: formData})
			return
		}
		if !ageDeclaration {
			s.render(w, r, "complete_profile.html", PageData{Title: "Complete Your Profile", User: user, Error: s.msg(r, user, "errors.age_required"), Data: formData})
			return
		}
		if accountType == "business" && !acceptDPA {
			s.render(w, r, "complete_profile.html", PageData{Title: "Complete Your Profile", User: user, Error: s.msg(r, user, "errors.dpa_required"), Data: formData})
			return
		}

		opts := &CompleteProfileOpts{
			Street:              street,
			Street2:             street2,
			PostalCode:          postalCode,
			City:                city,
			State:               state,
			Country:             country,
			CompanyRegistration: companyRegistration,
			VATNumber:           vatNumber,
			AcceptTerms:         acceptTerms,
			AcceptPrivacy:       acceptPrivacy,
			AcceptDPA:           acceptDPA,
			AgeDeclaration:      ageDeclaration,
		}
		if err := s.rest.CompleteProfile(token, opts); err != nil {
			s.render(w, r, "complete_profile.html", PageData{Title: "Complete Your Profile", User: user, Error: err.Error(), Data: formData})
			return
		}

		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
		return
	}

	data := map[string]interface{}{
		"AccountType": accountType,
	}
	s.render(w, r, "complete_profile.html", PageData{Title: "Complete Your Profile", User: user, Data: data})
}

// handleLogout serves the logout portal page.
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	token := getToken(r)
	if token != "" {
		_ = s.rest.Logout(token)
	}
	clearToken(w)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// handleForgotPassword serves the forgot password portal page.
func (s *Server) handleForgotPassword(w http.ResponseWriter, r *http.Request) {
	data := PageData{Title: "Forgot Password"}

	if r.Method == http.MethodPost {
		if !validateCSRF(r) {
			s.csrfFailed(w, r, "forgot_password.html")
			return
		}

		ip := extractClientIP(r)
		if !s.authLimiter.allow(ip) {
			data.Error = s.msg(r, nil, "errors.too_many_attempts")
			w.Header().Set("Retry-After", "60")
			s.render(w, r, "forgot_password.html", data)
			return
		}

		emailAddr := r.FormValue("email")
		if emailAddr == "" {
			data.Error = s.msg(r, nil, "errors.email_required")
			s.render(w, r, "forgot_password.html", data)
			return
		}
		if !emailRegex.MatchString(emailAddr) {
			data.Error = s.msg(r, nil, "errors.invalid_email")
			s.render(w, r, "forgot_password.html", data)
			return
		}
		_ = s.rest.ForgotPassword(emailAddr)
		http.Redirect(w, r, "/reset-password?email="+url.QueryEscape(emailAddr), http.StatusSeeOther)
		return
	}

	s.render(w, r, "forgot_password.html", data)
}

// handleResetPassword serves the reset password portal page.
func (s *Server) handleResetPassword(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Referrer-Policy", "no-referrer")

	emailAddr := r.URL.Query().Get("email")
	code := r.URL.Query().Get("code")

	if r.Method == http.MethodPost {
		if !validateCSRF(r) {
			s.csrfFailed(w, r, "reset_password.html")
			return
		}
		if emailAddr == "" {
			emailAddr = r.FormValue("email")
		}
		if code == "" {
			code = r.FormValue("code")
		}
	}

	data := PageData{
		Title: "Reset Password",
		Data: map[string]interface{}{
			"Email": emailAddr,
			"Code":  code,
		},
	}

	if emailAddr == "" && code == "" {
		http.Redirect(w, r, "/forgot-password", http.StatusSeeOther)
		return
	}
	if emailAddr == "" || code == "" {
		s.render(w, r, "reset_password.html", data)
		return
	}

	if r.Method == http.MethodPost && r.FormValue("action") == "reset" {
		ip := extractClientIP(r)
		if !s.authLimiter.allow(ip) {
			data.Error = s.msg(r, nil, "errors.too_many_attempts")
			w.Header().Set("Retry-After", "60")
			s.render(w, r, "reset_password.html", data)
			return
		}

		newPassword := r.FormValue("new_password")
		confirmPassword := r.FormValue("confirm_password")

		if newPassword != confirmPassword {
			data.Error = s.msg(r, nil, "errors.passwords_not_match")
			data.Data = map[string]interface{}{"Email": emailAddr, "Code": code, "ShowForm": true}
			s.render(w, r, "reset_password.html", data)
			return
		}
		if errKey := validatePassword(newPassword); errKey != "" {
			data.Error = s.msg(r, nil, errKey)
			data.Data = map[string]interface{}{"Email": emailAddr, "Code": code, "ShowForm": true}
			s.render(w, r, "reset_password.html", data)
			return
		}
		if err := s.rest.ResetPassword(emailAddr, code, newPassword); err != nil {
			data.Error = s.msg(r, nil, "errors.failed_reset_password", err.Error())
			data.Data = map[string]interface{}{"Email": emailAddr, "Code": code, "ShowForm": true}
			s.render(w, r, "reset_password.html", data)
			return
		}
		http.Redirect(w, r, "/login?reset=success", http.StatusSeeOther)
		return
	}

	data.Data = map[string]interface{}{"Email": emailAddr, "Code": code, "ShowForm": true}
	s.render(w, r, "reset_password.html", data)
}

// handleAcceptTerms serves the accept terms portal page.
func (s *Server) handleAcceptTerms(w http.ResponseWriter, r *http.Request) {
	token := getToken(r)
	user, _ := s.rest.Whoami(token)

	data := PageData{
		Title: "Accept Updated Terms",
		User:  user,
	}

	if r.Method == http.MethodPost {
		if !validateCSRF(r) {
			s.csrfFailed(w, r, "accept_terms.html")
			return
		}

		acceptTerms := r.FormValue("accept_terms") == "on"
		acceptPrivacy := r.FormValue("accept_privacy") == "on"
		acceptDPA := r.FormValue("accept_dpa") == "on"

		if !acceptTerms || !acceptPrivacy {
			data.Error = s.msg(r, user, "errors.consent_required")
			s.render(w, r, "accept_terms.html", data)
			return
		}

		if err := s.rest.AcceptConsent(token, acceptDPA); err != nil {
			data.Error = s.msg(r, user, "errors.consent_failed")
			s.render(w, r, "accept_terms.html", data)
			return
		}

		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
		return
	}

	s.render(w, r, "accept_terms.html", data)
}

// handlePrivacyData serves the privacy data portal page.
func (s *Server) handlePrivacyData(w http.ResponseWriter, r *http.Request) {
	token := getToken(r)
	user, _ := s.rest.Whoami(token)

	data := PageData{
		Title:       "Privacy & Data",
		User:        user,
		CurrentPage: "profile",
	}

	if r.Method == http.MethodPost {
		if !validateCSRF(r) {
			s.csrfFailed(w, r, "privacy_data.html")
			return
		}

		action := r.FormValue("action")
		switch action {
		case "request_deletion":
			if err := s.rest.RequestDeletion(token); err != nil {
				data.Error = s.msg(r, user, "errors.deletion_failed")
			} else {
				data.Success = s.msg(r, user, "success.deletion_requested")
			}

		case "cancel_deletion":
			if err := s.rest.CancelDeletion(token); err != nil {
				data.Error = s.msg(r, user, "errors.cancel_deletion_failed")
			} else {
				data.Success = s.msg(r, user, "success.deletion_cancelled")
			}
		}
	}

	// Fetch deletion status.
	delStatus, _ := s.rest.DeletionStatus(token)
	if delStatus != nil {
		data.Data = map[string]interface{}{
			"DeletionPending":   delStatus.Pending,
			"DeletionScheduled": delStatus.ScheduledAt,
		}
	}

	s.render(w, r, "privacy_data.html", data)
}

// handleDataExport serves the data export portal page.
func (s *Server) handleDataExport(w http.ResponseWriter, r *http.Request) {
	token := getToken(r)

	exportData, err := s.rest.ExportMyData(token)
	if err != nil {
		http.Error(w, "Failed to export data", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", `attachment; filename="my-data.json"`)
	_, _ = w.Write(exportData)
}

// handleDashboard serves the dashboard portal page.
func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	token := getToken(r)
	user, _ := s.rest.Whoami(token)

	data := map[string]interface{}{}
	if user != nil {
		data["User"] = user
	}

	// Agent health summary
	agents, agentTotal, _ := s.rest.ListMyAgents(token, 50, 0)
	data["Agents"] = agents
	data["AgentTotal"] = agentTotal

	// Recent content guard alerts (last 5)
	alerts, alertTotal, _ := s.rest.ListMyAlerts(token, 1, 5, 0)
	data["RecentAlerts"] = alerts
	data["AlertTotal"] = alertTotal

	// Unread message count + recent messages for dashboard widget
	data["UnreadCount"] = s.rest.UnreadMessageCount(token)
	recentMsgs, _, _ := s.rest.ListInbox(token, "", false, 3, 0)
	data["RecentMessages"] = recentMsgs

	// Activity summary (last 24h from unified activity endpoint)
	userTZ := "UTC"
	if user != nil {
		if tz, ok := user["timezone"].(string); ok && tz != "" {
			userTZ = tz
		}
	}
	now := storage.UTCNow()
	yesterday := now.Add(-24 * time.Hour)
	activityResp, _ := s.rest.ListActivity(token, "", "", "", yesterday.Format(time.RFC3339), now.Format(time.RFC3339), 1, 0)
	if activityResp != nil {
		data["ActivitySummary"] = activityResp.Summary
		// Reorder hourly buckets: current local hour on left, counting backwards
		loc, err := time.LoadLocation(userTZ)
		if err != nil {
			loc = time.UTC
		}
		localNow := now.In(loc)
		currentHour := localNow.Hour()
		reordered := make([]HourBucket, 24)
		for i := 0; i < 24; i++ {
			srcHour := (currentHour - i + 24) % 24
			reordered[i] = activityResp.Hourly[srcHour]
			reordered[i].Hour = srcHour
		}
		data["ActivityHourly"] = reordered
		maxH := 0
		for _, b := range reordered {
			s := b.Logins + b.Tasks + b.Demands + b.Endeavours
			if s > maxH {
				maxH = s
			}
		}
		data["ActivityHourlyMax"] = maxH
	} else {
		data["ActivitySummary"] = ActivitySummary{}
		data["ActivityHourly"] = []HourBucket{}
		data["ActivityHourlyMax"] = 0
	}

	// Ablecon + Harmcon indicators
	indicators, _ := s.rest.MyIndicators(token)
	data["Indicators"] = indicators

	// User's organizations and endeavours for quick access
	orgs, _, _ := s.rest.ListOrganizations(token, "", "active", 50, 0)
	data["Organizations"] = orgs
	endeavours, _, _ := s.rest.ListEndeavours(token, "", "", 50, 0)
	data["Endeavours"] = endeavours

	s.render(w, r, "dashboard.html", PageData{
		Title:       "Dashboard",
		User:        user,
		Data:        data,
		CurrentPage: "dashboard",
	})
}

// handleProfile serves the profile portal page.
func (s *Server) handleProfile(w http.ResponseWriter, r *http.Request) {
	token := getToken(r)
	user, _ := s.rest.Whoami(token)

	data := PageData{
		Title:       "Profile",
		User:        user,
		Data:        map[string]interface{}{"User": user},
		CurrentPage: "profile",
	}

	if r.Method == http.MethodPost {
		if !validateCSRF(r) {
			s.csrfFailed(w, r, "profile.html")
			return
		}
		action := r.FormValue("action")

		switch action {
		case "update_profile":
			firstName := strings.TrimSpace(r.FormValue("first_name"))
			middleNames := strings.TrimSpace(r.FormValue("middle_names"))
			lastName := strings.TrimSpace(r.FormValue("last_name"))
			lang := r.FormValue("lang")
			timezone := r.FormValue("timezone")
			emailCopy := r.FormValue("email_copy") == "on"
			phone := strings.TrimSpace(r.FormValue("phone"))
			country := r.FormValue("country")
			accountType := r.FormValue("account_type")
			companyName := strings.TrimSpace(r.FormValue("company_name"))
			companyReg := strings.TrimSpace(r.FormValue("company_registration"))
			street := strings.TrimSpace(r.FormValue("street"))
			street2 := strings.TrimSpace(r.FormValue("street2"))
			city := strings.TrimSpace(r.FormValue("city"))
			state := strings.TrimSpace(r.FormValue("state"))
			postalCode := strings.TrimSpace(r.FormValue("postal_code"))

			if firstName == "" || lastName == "" {
				data.Error = s.msg(r, user, "errors.name_required")
				s.render(w, r, "profile.html", data)
				return
			}
			// Construct full name for User record.
			name := firstName
			if middleNames != "" {
				name += " " + middleNames
			}
			name += " " + lastName

			userFields := map[string]interface{}{"name": name, "email_copy": emailCopy}
			if lang != "" && s.i18n.HasLanguage(lang) {
				userFields["lang"] = lang
			}
			if timezone != "" {
				userFields["timezone"] = timezone
			}
			if err := s.rest.UpdateProfile(token, userFields); err != nil {
				data.Error = s.msg(r, user, "errors.failed_update_profile", err.Error())
				s.render(w, r, "profile.html", data)
				return
			}
			// Update Person record (name + all personal info + address).
			personFields := map[string]interface{}{
				"first_name":           firstName,
				"middle_names":         middleNames,
				"last_name":            lastName,
				"phone":                phone,
				"country":              country,
				"account_type":         accountType,
				"company_name":         companyName,
				"company_registration": companyReg,
				"street":               street,
				"street2":              street2,
				"city":                 city,
				"state":                state,
				"postal_code":          postalCode,
			}
			_ = s.rest.UpdateMyPerson(token, personFields)
			// Refresh user data
			user, _ = s.rest.Whoami(token)
			data.User = user
			data.Data = map[string]interface{}{"User": user}
			data.Success = s.msg(r, user, "success.profile_updated")

		case "update_email":
			emailAddr := strings.TrimSpace(r.FormValue("email"))
			if emailAddr == "" || !emailRegex.MatchString(emailAddr) {
				data.Error = s.msg(r, user, "errors.invalid_email")
				s.render(w, r, "profile.html", data)
				return
			}
			if err := s.rest.RequestEmailChange(token, emailAddr); err != nil {
				data.Error = s.msg(r, user, "errors.failed_update_profile", err.Error())
				s.render(w, r, "profile.html", data)
				return
			}
			data.Data.(map[string]interface{})["EmailChangePending"] = emailAddr
			data.Success = s.msg(r, user, "success.email_verification_sent")

		case "verify_email":
			code := strings.TrimSpace(r.FormValue("email_code"))
			if code == "" {
				data.Error = s.msg(r, user, "errors.required_fields")
				s.render(w, r, "profile.html", data)
				return
			}
			if err := s.rest.VerifyEmailChange(token, code); err != nil {
				data.Error = s.msg(r, user, "errors.invalid_verification_code")
				s.render(w, r, "profile.html", data)
				return
			}
			user, _ = s.rest.Whoami(token)
			data.User = user
			data.Data = map[string]interface{}{"User": user}
			data.Success = s.msg(r, user, "success.email_updated")

		case "cancel_email_change":
			_ = s.rest.CancelEmailChange(token)

		case "change_password":
			currentPassword := r.FormValue("current_password")
			newPassword := r.FormValue("new_password")
			confirmPassword := r.FormValue("confirm_password")

			if newPassword != confirmPassword {
				data.Error = s.msg(r, user, "errors.new_passwords_not_match")
				s.render(w, r, "profile.html", data)
				return
			}
			if errKey := validatePassword(newPassword); errKey != "" {
				data.Error = s.msg(r, user, errKey)
				s.render(w, r, "profile.html", data)
				return
			}
			if err := s.rest.ChangePassword(token, currentPassword, newPassword); err != nil {
				if restErr, ok := err.(*RESTError); ok && restErr.Code == "invalid_credentials" {
					data.Error = s.msg(r, user, "errors.current_password_incorrect")
				} else {
					data.Error = s.msg(r, user, "errors.failed_update_password")
				}
				s.render(w, r, "profile.html", data)
				return
			}
			data.Success = s.msg(r, user, "success.password_updated")

		case "totp_setup":
			setupResult, err := s.rest.TOTPSetup(token)
			if err != nil {
				data.Error = s.msg(r, user, "errors.totp_setup_failed")
				s.render(w, r, "profile.html", data)
				return
			}
			data.Data.(map[string]interface{})["TOTPSetup"] = setupResult
			data.Data.(map[string]interface{})["TOTPPhase"] = "verify"

		case "totp_enable":
			code := r.FormValue("totp_code")
			if code == "" {
				data.Error = s.msg(r, user, "errors.required_fields")
				s.render(w, r, "profile.html", data)
				return
			}
			enableResult, err := s.rest.TOTPEnable(token, code)
			if err != nil {
				data.Error = s.msg(r, user, "errors.invalid_totp_code")
				s.render(w, r, "profile.html", data)
				return
			}
			data.Data.(map[string]interface{})["RecoveryCodes"] = enableResult.RecoveryCodes
			data.Data.(map[string]interface{})["TOTPPhase"] = "recovery"
			data.Success = s.msg(r, user, "success.totp_enabled")

		case "totp_disable":
			code := r.FormValue("totp_code")
			if code == "" {
				data.Error = s.msg(r, user, "errors.required_fields")
				s.render(w, r, "profile.html", data)
				return
			}
			if err := s.rest.TOTPDisable(token, code); err != nil {
				data.Error = s.msg(r, user, "errors.invalid_totp_code")
				s.render(w, r, "profile.html", data)
				return
			}
			data.Success = s.msg(r, user, "success.totp_disabled")

		}
	}

	// Fetch TOTP status for the profile page.
	totpStatus, _ := s.rest.TOTPStatus(token)
	if data.Data == nil {
		data.Data = map[string]interface{}{"User": user}
	}
	if totpStatus != nil {
		data.Data.(map[string]interface{})["TOTPEnabled"] = totpStatus.Enabled
		data.Data.(map[string]interface{})["TOTPRecoveryRemaining"] = totpStatus.RecoveryCodesRemaining
	}

	// Fetch person data for the profile page.
	// Always provide a Person object so the template renders the section.
	person, _ := s.rest.GetMyPerson(token)
	if person == nil {
		person = &PersonResult{}
	}
	data.Data.(map[string]interface{})["Person"] = person

	// Check for pending email change (persists across page reloads).
	if pending, newEmail, err := s.rest.GetPendingEmailChange(token); err == nil && pending {
		data.Data.(map[string]interface{})["EmailChangePending"] = newEmail
	}

	// Handle email verification link (?email_code=CODE).
	if emailCode := r.URL.Query().Get("email_code"); emailCode != "" {
		if err := s.rest.VerifyEmailChange(token, emailCode); err != nil {
			data.Error = s.msg(r, user, "errors.invalid_verification_code")
		} else {
			user, _ = s.rest.Whoami(token)
			data.User = user
			data.Data.(map[string]interface{})["User"] = user
			delete(data.Data.(map[string]interface{}), "EmailChangePending")
			data.Success = s.msg(r, user, "success.email_updated")
		}
	}

	// Fetch consent history for the profile page.
	consents, _ := s.rest.ListMyConsents(token)
	if consents != nil {
		data.Data.(map[string]interface{})["Consents"] = consents
	}

	s.render(w, r, "profile.html", data)
}

// handleAgents serves the agents portal page.
func (s *Server) handleAgents(w http.ResponseWriter, r *http.Request) {
	token := getToken(r)
	user, _ := s.rest.Whoami(token)
	data := PageData{Title: "Agents", User: user, CurrentPage: "agents"}

	if r.Method == http.MethodPost {
		if !validateCSRF(r) {
			s.csrfFailed(w, r, "agents.html")
			return
		}
		action := r.FormValue("action")

		switch action {
		case "create":
			name := r.FormValue("name")
			if name == "" {
				name = "Agent token"
			}
			maxUses := 1
			if v := r.FormValue("max_uses"); v != "" {
				if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
					maxUses = parsed
				}
			}
			var expiresAtStr string
			if v := r.FormValue("expires_at"); v != "" {
				if t, err := time.ParseInLocation("2006-01-02T15:04", v, time.Local); err == nil {
					expiresAtStr = t.UTC().Format(time.RFC3339)
				}
			}
			inv, err := s.rest.CreateAgentToken(token, name, maxUses, expiresAtStr)
			if err != nil {
				data.Error = s.msg(r, user, "errors.failed_create_token", err.Error())
			} else {
				data.Success = s.msg(r, user, "success.token_created", inv.Token)
			}

		case "revoke":
			tokenID := r.FormValue("token_id")
			if tokenID == "" {
				data.Error = s.msg(r, user, "errors.token_id_required")
			} else if err := s.rest.RevokeAgentToken(token, tokenID); err != nil {
				data.Error = s.msg(r, user, "errors.failed_revoke_token", err.Error())
			} else {
				data.Success = s.msg(r, user, "success.token_revoked")
			}

		case "block":
			agentID := r.FormValue("agent_id")
			blocked := "blocked"
			if err := s.rest.UpdateMyAgent(token, agentID, nil, &blocked); err != nil {
				data.Error = s.msg(r, user, "portal.agent_detail.errors.block_failed", err.Error())
			} else {
				data.Success = s.msg(r, user, "portal.agent_detail.block_success")
			}

		case "unblock":
			agentID := r.FormValue("agent_id")
			active := "active"
			if err := s.rest.UpdateMyAgent(token, agentID, nil, &active); err != nil {
				data.Error = s.msg(r, user, "portal.agent_detail.errors.unblock_failed", err.Error())
			} else {
				data.Success = s.msg(r, user, "portal.agent_detail.unblock_success")
			}
		}
	}

	tokens, err := s.rest.ListAgentTokens(token)
	if err != nil {
		data.Error = s.msg(r, user, "errors.failed_list_tokens", err.Error())
	}

	type tokenWithStatus struct {
		Token  *InvitationToken
		Status string
	}
	tokenList := make([]tokenWithStatus, len(tokens))
	for i, t := range tokens {
		tokenList[i] = tokenWithStatus{Token: t, Status: t.Status}
	}

	agents, _, agentErr := s.rest.ListMyAgents(token, 50, 0)
	if agentErr != nil && data.Error == "" {
		data.Error = s.msg(r, user, "errors.failed_list_agents", agentErr.Error())
	}

	// Token counts and filter
	tokenFilter := r.URL.Query().Get("token_status")
	var activeCount, usedCount, expiredCount int
	for _, t := range tokenList {
		switch t.Status {
		case "active":
			activeCount++
		case "exhausted":
			usedCount++
		case "expired", "revoked":
			expiredCount++
		}
	}
	var filteredTokens []tokenWithStatus
	if tokenFilter == "" {
		filteredTokens = tokenList
	} else {
		for _, t := range tokenList {
			match := false
			switch tokenFilter {
			case "active":
				match = t.Status == "active"
			case "used":
				match = t.Status == "exhausted"
			case "expired":
				match = t.Status == "expired" || t.Status == "revoked"
			}
			if match {
				filteredTokens = append(filteredTokens, t)
			}
		}
	}

	if data.Data == nil {
		data.Data = map[string]interface{}{}
	}
	dataMap := data.Data.(map[string]interface{})
	dataMap["Tokens"] = filteredTokens
	dataMap["TokenFilter"] = tokenFilter
	dataMap["TokenActiveCount"] = activeCount
	dataMap["TokenUsedCount"] = usedCount
	dataMap["TokenExpiredCount"] = expiredCount
	dataMap["TokenTotal"] = len(tokenList)
	dataMap["Agents"] = agents
	dataMap["AgentCount"] = len(agents)
	data.Data = dataMap

	s.render(w, r, "agents.html", data)
}

// handleAgentDetail serves the agent detail portal page.
func (s *Server) handleAgentDetail(w http.ResponseWriter, r *http.Request) {
	token := getToken(r)
	user, _ := s.rest.Whoami(token)
	agentID := r.PathValue("id")

	data := PageData{Title: "Agent Detail", User: user, CurrentPage: "agents"}

	// Handle update
	if r.Method == http.MethodPost {
		if !validateCSRF(r) {
			s.csrfFailed(w, r, "agent_detail.html")
			return
		}
		action := r.FormValue("action")
		switch action {
		case "update":
			name := r.FormValue("name")
			status := r.FormValue("status")
			var namePtr, statusPtr *string
			if name != "" {
				namePtr = &name
			}
			if status != "" {
				statusPtr = &status
			}
			if err := s.rest.UpdateMyAgent(token, agentID, namePtr, statusPtr); err != nil {
				data.Error = s.msg(r, user, "errors.failed_update_agent", err.Error())
			} else {
				data.Success = s.msg(r, user, "success.agent_updated")
			}
		case "block":
			blocked := "blocked"
			if err := s.rest.UpdateMyAgent(token, agentID, nil, &blocked); err != nil {
				data.Error = s.msg(r, user, "portal.agent_detail.errors.block_failed", err.Error())
			} else {
				data.Success = s.msg(r, user, "portal.agent_detail.block_success")
			}
		case "unblock":
			active := "active"
			if err := s.rest.UpdateMyAgent(token, agentID, nil, &active); err != nil {
				data.Error = s.msg(r, user, "portal.agent_detail.errors.unblock_failed", err.Error())
			} else {
				data.Success = s.msg(r, user, "portal.agent_detail.unblock_success")
			}
		}
	}

	// Load agent detail
	agent, err := s.rest.GetMyAgent(token, agentID)
	if err != nil {
		data.Error = s.msg(r, user, "errors.agent_not_found")
		data.Data = map[string]interface{}{}
		s.render(w, r, "agent_detail.html", data)
		return
	}

	dataMap := map[string]interface{}{
		"Agent":     agent,
		"AgentName": agent.Name,
	}

	// Load onboarding
	onboarding, _ := s.rest.GetMyAgentOnboarding(token, agentID)
	if onboarding != nil {
		dataMap["Onboarding"] = onboarding
	}

	// Load recent activity
	activity, _, _ := s.rest.ListMyAgentActivity(token, agentID, 20, 0)
	if activity != nil {
		dataMap["Activity"] = activity
	}

	data.Data = dataMap
	s.render(w, r, "agent_detail.html", data)
}

// handleOrganizations serves the organizations portal page.
func (s *Server) handleOrganizations(w http.ResponseWriter, r *http.Request) {
	token := getToken(r)
	user, _ := s.rest.Whoami(token)
	data := PageData{Title: "Organizations", User: user, CurrentPage: "organizations"}

	if r.Method == http.MethodPost {
		if !validateCSRF(r) {
			s.csrfFailed(w, r, "orgs.html")
			return
		}
		action := r.FormValue("action")
		if action == "create" {
			name := r.FormValue("name")
			description := r.FormValue("description")
			if name == "" {
				data.Error = s.msg(r, user, "errors.name_required")
			} else {
				_, err := s.rest.CreateOrganization(token, name, description)
				if err != nil {
					data.Error = s.msg(r, user, "errors.failed_create_org", err.Error())
				} else {
					data.Success = s.msg(r, user, "success.organization_created")
				}
			}
		}
	}

	search := r.URL.Query().Get("search")
	status := r.URL.Query().Get("status")
	offset := queryIntParam(r, "offset", 0)
	limit := 50

	orgs, total, err := s.rest.ListOrganizations(token, search, status, limit, offset)
	if err != nil && data.Error == "" {
		data.Error = s.msg(r, user, "errors.failed_list_orgs", err.Error())
	}

	data.Data = map[string]interface{}{
		"Organizations": orgs,
		"Total":         total,
		"Limit":         limit,
		"Offset":        offset,
		"Search":        search,
		"Status":        status,
	}

	s.render(w, r, "orgs.html", data)
}

// handleOrganizationDetail serves the organization detail portal page.
func (s *Server) handleOrganizationDetail(w http.ResponseWriter, r *http.Request) {
	token := getToken(r)
	user, _ := s.rest.Whoami(token)
	orgID := r.PathValue("id")

	data := PageData{Title: "Organization Detail", User: user, CurrentPage: "organizations"}

	if r.Method == http.MethodPost {
		if !validateCSRF(r) {
			s.csrfFailed(w, r, "org_detail.html")
			return
		}
		action := r.FormValue("action")
		switch action {
		case "update":
			fields := map[string]interface{}{}
			if name := r.FormValue("name"); name != "" {
				fields["name"] = name
			}
			if status := r.FormValue("status"); status != "" {
				fields["status"] = status
			}
			if len(fields) > 0 {
				if _, err := s.rest.UpdateOrganization(token, orgID, fields); err != nil {
					data.Error = s.msg(r, user, "errors.failed_update_org", err.Error())
				} else {
					data.Success = s.msg(r, user, "success.organization_updated")
				}
			}
		case "update_roles":
			rolesRaw := r.FormValue("team_roles")
			var roles []string
			for _, line := range strings.Split(rolesRaw, "\n") {
				role := strings.TrimSpace(line)
				if role != "" {
					roles = append(roles, role)
				}
			}
			// Merge into existing metadata.
			org, err := s.rest.GetOrganization(token, orgID)
			if err != nil {
				data.Error = s.msg(r, user, "errors.failed_update_org", err.Error())
			} else {
				meta := map[string]interface{}{}
				if org.Metadata != nil {
					for k, v := range org.Metadata {
						meta[k] = v
					}
				}
				meta["team_roles"] = roles
				if _, err := s.rest.UpdateOrganization(token, orgID, map[string]interface{}{"metadata": meta}); err != nil {
					data.Error = s.msg(r, user, "errors.failed_update_org", err.Error())
				} else {
					data.Success = s.msg(r, user, "portal.org_detail.success.roles_updated")
				}
			}
		case "update_alert_terms":
			termsRaw := r.FormValue("alert_terms")
			weightStr := r.FormValue("alert_weight")
			weight := 5
			if weightStr != "" {
				if v, err := strconv.Atoi(weightStr); err == nil && v >= 1 && v <= 10 {
					weight = v
				}
			}
			var terms []map[string]interface{}
			for _, line := range strings.Split(termsRaw, "\n") {
				term := strings.TrimSpace(line)
				if term != "" {
					terms = append(terms, map[string]interface{}{"term": term, "weight": weight})
				}
			}
			if err := s.rest.UpdateOrgAlertTerms(token, orgID, terms); err != nil {
				data.Error = s.msg(r, user, "portal.org_detail.errors.alert_terms_failed", err.Error())
			} else {
				data.Success = s.msg(r, user, "portal.org_detail.success.alert_terms_updated")
			}
		case "add_member":
			resourceID := r.FormValue("resource_id")
			role := r.FormValue("role")
			if role == "" {
				role = "member"
			}
			if resourceID != "" {
				meta := map[string]interface{}{"role": role}
				if _, err := s.rest.CreateRelation(token, "has_member", "organization", orgID, "resource", resourceID, meta); err != nil {
					data.Error = s.msg(r, user, "portal.org_detail.errors.failed_add_member", err.Error())
				} else {
					data.Success = s.msg(r, user, "portal.org_detail.success.member_added")
				}
			}
		case "remove_member":
			relID := r.FormValue("relation_id")
			if relID != "" {
				if err := s.rest.DeleteRelation(token, relID); err != nil {
					data.Error = s.msg(r, user, "portal.org_detail.errors.failed_remove_member", err.Error())
				} else {
					data.Success = s.msg(r, user, "portal.org_detail.success.member_removed")
				}
			}
		case "change_role":
			relID := r.FormValue("relation_id")
			resourceID := r.FormValue("resource_id")
			newRole := r.FormValue("role")
			if relID != "" && resourceID != "" && newRole != "" {
				// Delete old relation and create new one with updated role.
				if err := s.rest.DeleteRelation(token, relID); err != nil {
					data.Error = s.msg(r, user, "portal.org_detail.errors.failed_change_role", err.Error())
				} else {
					meta := map[string]interface{}{"role": newRole}
					if _, err := s.rest.CreateRelation(token, "has_member", "organization", orgID, "resource", resourceID, meta); err != nil {
						data.Error = s.msg(r, user, "portal.org_detail.errors.failed_change_role", err.Error())
					} else {
						data.Success = s.msg(r, user, "portal.org_detail.success.role_changed")
					}
				}
			}
		case "archive":
			reason := r.FormValue("reason")
			if reason == "" {
				data.Error = s.msg(r, user, "errors.archive_reason_required")
			} else {
				if _, err := s.rest.ArchiveOrganization(token, orgID, reason); err != nil {
					data.Error = s.msg(r, user, "errors.failed_archive_org", err.Error())
				} else {
					data.Success = s.msg(r, user, "success.organization_archived")
				}
			}
		}
	}

	org, err := s.rest.GetOrganization(token, orgID)
	if err != nil {
		data.Error = s.msg(r, user, "errors.org_not_found")
		data.Data = map[string]interface{}{}
		s.render(w, r, "org_detail.html", data)
		return
	}

	dataMap := map[string]interface{}{
		"Organization":     org,
		"OrganizationName": org.Name,
	}

	// Load members as relations to get roles and relation IDs.
	type OrgMember struct {
		RelationID   string
		ResourceID   string
		ResourceName string
		ResourceType string
		Role         string
	}
	memberRels, _, _ := s.rest.ListRelations(token, "organization", orgID, "resource", "", "has_member", 100, 0)
	members := make([]*OrgMember, 0, len(memberRels))
	for _, rel := range memberRels {
		m := &OrgMember{
			RelationID:   rel.ID,
			ResourceID:   rel.TargetEntityID,
			ResourceName: rel.TargetEntityID,
			Role:         "member",
		}
		if rel.Metadata != nil {
			if role, ok := rel.Metadata["role"].(string); ok && role != "" {
				m.Role = role
			}
		}
		if res, err := s.rest.GetResource(token, rel.TargetEntityID); err == nil {
			m.ResourceName = res.Name
			m.ResourceType = res.Type
		}
		members = append(members, m)
	}
	dataMap["Members"] = members

	// Determine if user can manage this org (admin or org owner/admin).
	canManageOrg := isAdmin(user)
	if !canManageOrg {
		_, role := userOrgAdminRole(user)
		if role == "owner" || role == "admin" {
			canManageOrg = true
		}
	}
	dataMap["CanManageOrg"] = canManageOrg

	// Load available resources for member add dropdown.
	if canManageOrg {
		allRes, _, _ := s.rest.AdminListResources(token, "", "", "active", orgID, 200, 0)
		// Exclude resources that are already members.
		memberIDs := map[string]bool{}
		for _, m := range members {
			memberIDs[m.ResourceID] = true
		}
		var available []*Resource
		for _, res := range allRes {
			if !memberIDs[res.ID] && res.Type != "team" {
				available = append(available, res)
			}
		}
		dataMap["AvailableResources"] = available
		// Org member roles.
		dataMap["OrgRoles"] = []string{"owner", "admin", "member", "guest"}
	}

	// Load endeavours in this organization
	edvs, _, _ := s.rest.ListEndeavours(token, orgID, "", 20, 0)
	dataMap["Endeavours"] = edvs

	// Extract team roles from org metadata for display.
	var teamRoles []string
	if org.Metadata != nil {
		if raw, ok := org.Metadata["team_roles"]; ok {
			if arr, ok := raw.([]interface{}); ok {
				for _, v := range arr {
					if s, ok := v.(string); ok && s != "" {
						teamRoles = append(teamRoles, s)
					}
				}
			}
		}
	}
	dataMap["TeamRoles"] = teamRoles

	// Load alert terms
	alertTerms, _ := s.rest.ListOrgAlertTerms(token, orgID)
	dataMap["AlertTerms"] = alertTerms

	data.Data = dataMap
	s.render(w, r, "org_detail.html", data)
}

// handleEndeavours serves the endeavours portal page.
func (s *Server) handleEndeavours(w http.ResponseWriter, r *http.Request) {
	token := getToken(r)
	user, _ := s.rest.Whoami(token)
	data := PageData{Title: "Endeavours", User: user, CurrentPage: "endeavours"}

	if r.Method == http.MethodPost {
		if !validateCSRF(r) {
			s.csrfFailed(w, r, "endeavours.html")
			return
		}
		action := r.FormValue("action")
		if action == "create" {
			name := r.FormValue("name")
			description := r.FormValue("description")
			if name == "" {
				data.Error = s.msg(r, user, "errors.name_required")
			} else {
				_, err := s.rest.CreateEndeavour(token, name, description)
				if err != nil {
					data.Error = s.msg(r, user, "errors.failed_create_endeavour", err.Error())
				} else {
					data.Success = s.msg(r, user, "success.endeavour_created")
				}
			}
		}
	}

	orgID := r.URL.Query().Get("organization_id")
	search := r.URL.Query().Get("search")
	edvs, _, err := s.rest.ListEndeavours(token, orgID, search, 50, 0)
	if err != nil && data.Error == "" {
		data.Error = s.msg(r, user, "errors.failed_list_endeavours", err.Error())
	}

	data.Data = map[string]interface{}{
		"Endeavours": edvs,
	}

	s.render(w, r, "endeavours.html", data)
}

// handleEndeavourDetail serves the endeavour detail portal page.
func (s *Server) handleEndeavourDetail(w http.ResponseWriter, r *http.Request) {
	token := getToken(r)
	user, _ := s.rest.Whoami(token)
	edvID := r.PathValue("id")

	data := PageData{Title: "Endeavour Detail", User: user, CurrentPage: "endeavours"}

	if r.Method == http.MethodPost {
		if !validateCSRF(r) {
			s.csrfFailed(w, r, "endeavour_detail.html")
			return
		}
		action := r.FormValue("action")
		switch action {
		case "update":
			fields := map[string]interface{}{}
			if name := r.FormValue("name"); name != "" {
				fields["name"] = name
			}
			if status := r.FormValue("status"); status != "" {
				fields["status"] = status
			}
			if v := r.FormValue("start_date"); v != "" {
				fields["start_date"] = v
			} else {
				fields["start_date"] = ""
			}
			if v := r.FormValue("end_date"); v != "" {
				fields["end_date"] = v
			} else {
				fields["end_date"] = ""
			}
			fields["taskschmied_enabled"] = r.FormValue("taskschmied_enabled") == "true"
			if len(fields) > 0 {
				if _, err := s.rest.UpdateEndeavour(token, edvID, fields); err != nil {
					data.Error = s.msg(r, user, "errors.failed_update_endeavour", err.Error())
				} else {
					data.Success = s.msg(r, user, "success.endeavour_updated")
				}
			}
		case "update_goals":
			edv, err := s.rest.GetEndeavour(token, edvID)
			if err != nil {
				data.Error = s.msg(r, user, "errors.failed_load_endeavour", err.Error())
			} else if edv.Goals != nil {
				updatedGoals := make([]map[string]interface{}, 0, len(edv.Goals))
				for _, g := range edv.Goals {
					goalID, _ := g["id"].(string)
					title, _ := g["title"].(string)
					newStatus := r.FormValue("goal_status_" + goalID)
					if newStatus == "" {
						newStatus = "open"
					}
					updatedGoals = append(updatedGoals, map[string]interface{}{
						"id":     goalID,
						"title":  title,
						"status": newStatus,
					})
				}
				fields := map[string]interface{}{"goals": updatedGoals}
				if _, err := s.rest.UpdateEndeavour(token, edvID, fields); err != nil {
					data.Error = s.msg(r, user, "errors.failed_update_goals", err.Error())
				} else {
					data.Success = s.msg(r, user, "success.goals_updated")
				}
			}
		case "add_goal":
			title := r.FormValue("goal_title")
			if title == "" {
				data.Error = s.msg(r, user, "errors.goal_title_required")
			} else {
				edv, err := s.rest.GetEndeavour(token, edvID)
				if err != nil {
					data.Error = s.msg(r, user, "errors.failed_load_endeavour", err.Error())
				} else {
					goals := make([]map[string]interface{}, 0)
					if edv.Goals != nil {
						goals = append(goals, edv.Goals...)
					}
					goals = append(goals, map[string]interface{}{
						"title":  title,
						"status": "open",
					})
					fields := map[string]interface{}{"goals": goals}
					if _, err := s.rest.UpdateEndeavour(token, edvID, fields); err != nil {
						data.Error = s.msg(r, user, "errors.failed_add_goal", err.Error())
					} else {
						data.Success = s.msg(r, user, "success.goal_added")
					}
				}
			}
		case "grant_access":
			agentUserID := r.FormValue("agent_user_id")
			if agentUserID == "" {
				data.Error = s.msg(r, user, "errors.select_agent")
			} else {
				if err := s.rest.AddUserToEndeavour(token, agentUserID, edvID, "member"); err != nil {
					data.Error = s.msg(r, user, "errors.failed_grant_access", err.Error())
				} else {
					data.Success = s.msg(r, user, "success.agent_access_granted")
				}
			}
		case "revoke_access":
			revokeUserID := r.FormValue("revoke_user_id")
			if revokeUserID == "" {
				data.Error = s.msg(r, user, "errors.select_agent")
			} else {
				// Prevent revoking owners -- check membership first
				members, _ := s.rest.ListEndeavourMembers(token, edvID)
				isOwner := false
				for _, m := range members {
					if uid, _ := m["user_id"].(string); uid == revokeUserID {
						if role, _ := m["role"].(string); role == "owner" {
							isOwner = true
						}
						break
					}
				}
				if isOwner {
					data.Error = s.msg(r, user, "errors.cannot_revoke_owner")
				} else if err := s.rest.RemoveEndeavourMember(token, edvID, revokeUserID); err != nil {
					data.Error = s.msg(r, user, "errors.failed_revoke_access", err.Error())
				} else {
					data.Success = s.msg(r, user, "success.agent_access_revoked")
				}
			}
		case "archive":
			reason := r.FormValue("reason")
			if reason == "" {
				data.Error = s.msg(r, user, "errors.archive_reason_required")
			} else {
				if _, err := s.rest.ArchiveEndeavour(token, edvID, reason); err != nil {
					data.Error = s.msg(r, user, "errors.failed_archive_endeavour", err.Error())
				} else {
					data.Success = s.msg(r, user, "success.endeavour_archived")
				}
			}
		}
	}

	edv, err := s.rest.GetEndeavour(token, edvID)
	if err != nil {
		data.Error = s.msg(r, user, "errors.endeavour_not_found")
		data.Data = map[string]interface{}{}
		s.render(w, r, "endeavour_detail.html", data)
		return
	}

	dataMap := map[string]interface{}{
		"Endeavour":     edv,
		"EndeavourName": edv.Name,
	}

	// Load tasks in this endeavour
	tasks, _, _ := s.rest.ListTasks(token, edvID, "", "", "", "", 20, 0)
	dataMap["Tasks"] = tasks

	data.Data = dataMap
	s.render(w, r, "endeavour_detail.html", data)
}

// --- Rituals (user-facing) ---

func (s *Server) handleRituals(w http.ResponseWriter, r *http.Request) {
	token := getToken(r)
	user, _ := s.rest.Whoami(token)
	data := PageData{Title: "Rituals", User: user, CurrentPage: "rituals"}

	if r.Method == http.MethodPost {
		if !validateCSRF(r) {
			s.csrfFailed(w, r, "rituals.html")
			return
		}
		action := r.FormValue("action")
		if action == "fork" {
			templateID := r.FormValue("template_id")
			endeavourID := r.FormValue("endeavour_id")
			if templateID == "" || endeavourID == "" {
				data.Error = s.msg(r, user, "errors.ritual_fork_fields_required")
			} else {
				fields := map[string]interface{}{
					"endeavour_id": endeavourID,
				}
				if name := r.FormValue("name"); name != "" {
					fields["name"] = name
				}
				_, err := s.rest.ForkRitual(token, templateID, fields)
				if err != nil {
					data.Error = s.msg(r, user, "errors.failed_fork_ritual", err.Error())
				} else {
					data.Success = s.msg(r, user, "success.ritual_forked")
				}
			}
		}
	}

	search := r.URL.Query().Get("search")
	endeavourID := r.URL.Query().Get("endeavour_id")
	tab := r.URL.Query().Get("tab") // "templates" or "" (my rituals)
	filterLang := r.URL.Query().Get("lang")

	// Default template language filter to the user's profile language.
	if filterLang == "" && tab == "templates" {
		if ul, ok := user["lang"]; ok {
			if s, ok := ul.(string); ok && s != "" {
				filterLang = s
			}
		}
	}

	// Fetch user's own rituals (forks, custom -- exclude templates).
	var rituals []*Ritual
	var total int
	if tab != "templates" {
		var err error
		rituals, _, err = s.rest.ListRitualsFiltered(token, search, "active", "", endeavourID, "", 50, 0)
		if err != nil && data.Error == "" {
			data.Error = s.msg(r, user, "errors.failed_load_rituals", err.Error())
		}
		// Filter out origin=template from user's list (they see those in the templates tab).
		var filtered []*Ritual
		for _, r := range rituals {
			if r.Origin != "template" {
				filtered = append(filtered, r)
			}
		}
		rituals = filtered
		total = len(filtered)
	}

	// Fetch available templates (for fork).
	var templates []*Ritual
	var templateTotal int
	if tab == "templates" || tab == "" {
		templates, templateTotal, _ = s.rest.ListRitualsFiltered(token, search, "active", "template", "", filterLang, 50, 0)
	}

	// Fetch active endeavours for the fork dropdown (exclude completed/archived).
	allEdvs, _, _ := s.rest.ListEndeavours(token, "", "", 100, 0)
	var edvs []*Endeavour
	for _, e := range allEdvs {
		if e.Status == "active" || e.Status == "pending" {
			edvs = append(edvs, e)
		}
	}

	// Build endeavour name map for display.
	edvNames := map[string]string{}
	for _, e := range allEdvs {
		edvNames[e.ID] = e.Name
	}
	// Resolve any endeavour names referenced by rituals but not in the user's list.
	for _, rit := range rituals {
		if rit.EndeavourID != "" {
			if _, ok := edvNames[rit.EndeavourID]; !ok {
				if edv, err := s.rest.GetEndeavour(token, rit.EndeavourID); err == nil {
					edvNames[edv.ID] = edv.Name
				}
			}
		}
	}

	data.Data = map[string]interface{}{
		"Rituals":        rituals,
		"Total":          total,
		"Templates":      templates,
		"TemplateTotal":  templateTotal,
		"Endeavours":     edvs,
		"EdvNames":       edvNames,
		"Search":         search,
		"EndeavourID":    endeavourID,
		"Tab":            tab,
		"FilterLang":     filterLang,
	}
	s.render(w, r, "rituals.html", data)
}

// handleRitualDetail serves the ritual detail portal page.
func (s *Server) handleRitualDetail(w http.ResponseWriter, r *http.Request) {
	token := getToken(r)
	user, _ := s.rest.Whoami(token)
	id := r.PathValue("id")

	data := PageData{Title: "Ritual", User: user, CurrentPage: "rituals"}

	editing := false

	if r.Method == http.MethodPost {
		if !validateCSRF(r) {
			s.csrfFailed(w, r, "ritual_detail.html")
			return
		}
		action := r.FormValue("action")
		switch action {
		case "toggle_enabled":
			ritual, err := s.rest.GetRitual(token, id)
			if err != nil {
				data.Error = s.msg(r, user, "errors.failed_load_rituals", err.Error())
				break
			}
			newEnabled := !ritual.IsEnabled
			if _, err := s.rest.UpdateRitual(token, id, map[string]interface{}{"is_enabled": newEnabled}); err != nil {
				data.Error = s.msg(r, user, "errors.failed_update_ritual", err.Error())
			} else {
				data.Success = s.msg(r, user, "success.ritual_updated")
			}
		case "archive":
			if _, err := s.rest.UpdateRitual(token, id, map[string]interface{}{"status": "archived"}); err != nil {
				data.Error = s.msg(r, user, "errors.failed_update_ritual", err.Error())
			} else {
				data.Success = s.msg(r, user, "success.ritual_updated")
			}
		case "edit":
			editing = true
		case "save":
			fields := map[string]interface{}{}
			if v := r.FormValue("name"); v != "" {
				fields["name"] = v
			}
			if v := r.FormValue("description"); v != "" {
				fields["description"] = v
			}
			// Build schedule from form fields.
			if schedType := r.FormValue("schedule_type"); schedType != "" {
				sched := map[string]interface{}{"type": schedType}
				switch schedType {
				case "cron":
					if expr := r.FormValue("schedule_expression"); expr != "" {
						sched["expression"] = expr
					}
				case "interval":
					if every := r.FormValue("schedule_every"); every != "" {
						sched["every"] = every
					}
					if on := r.FormValue("schedule_on"); on != "" {
						sched["on"] = on
					}
				}
				fields["schedule"] = sched
			}
			if len(fields) > 0 {
				if _, err := s.rest.UpdateRitual(token, id, fields); err != nil {
					data.Error = s.msg(r, user, "errors.failed_update_ritual", err.Error())
					editing = true
				} else {
					data.Success = s.msg(r, user, "success.ritual_updated")
				}
			}
		case "fork":
			endeavourID := r.FormValue("endeavour_id")
			forkFields := map[string]interface{}{}
			if endeavourID != "" {
				forkFields["endeavour_id"] = endeavourID
			}
			if name := r.FormValue("name"); name != "" {
				forkFields["name"] = name
			}
			forked, err := s.rest.ForkRitual(token, id, forkFields)
			if err != nil {
				data.Error = s.msg(r, user, "errors.failed_fork_ritual", err.Error())
			} else {
				http.Redirect(w, r, "/rituals/"+forked.ID, http.StatusSeeOther)
				return
			}
		}
	}

	if r.URL.Query().Get("edit") == "1" {
		editing = true
	}

	ritual, err := s.rest.GetRitual(token, id)
	if err != nil {
		data.Error = s.msg(r, user, "errors.failed_load_rituals", err.Error())
		data.Data = map[string]interface{}{}
		s.render(w, r, "ritual_detail.html", data)
		return
	}

	// Fetch active endeavours for fork dropdown (exclude completed/archived).
	allEdvs, _, _ := s.rest.ListEndeavours(token, "", "", 100, 0)
	var edvs []*Endeavour
	for _, e := range allEdvs {
		if e.Status == "active" || e.Status == "pending" {
			edvs = append(edvs, e)
		}
	}

	data.Title = ritual.Name
	data.Data = map[string]interface{}{
		"Ritual":      ritual,
		"RitualName":  ritual.Name,
		"Editing":     editing,
		"IsTemplate":  ritual.Origin == "template",
		"Endeavours":  edvs,
	}
	s.render(w, r, "ritual_detail.html", data)
}

// handleTasks serves the tasks portal page.
func (s *Server) handleTasks(w http.ResponseWriter, r *http.Request) {
	token := getToken(r)
	user, _ := s.rest.Whoami(token)
	data := PageData{Title: "Tasks", User: user, CurrentPage: "tasks"}

	if r.Method == http.MethodPost {
		if !validateCSRF(r) {
			s.csrfFailed(w, r, "tasks.html")
			return
		}
		action := r.FormValue("action")
		if action == "create" {
			title := r.FormValue("title")
			description := r.FormValue("description")
			if title == "" {
				data.Error = s.msg(r, user, "errors.title_required")
			} else {
				fields := map[string]interface{}{
					"title":       title,
					"description": description,
				}
				if edvID := r.FormValue("endeavour_id"); edvID != "" {
					fields["endeavour_id"] = edvID
				}
				if dueDate := r.FormValue("due_date"); dueDate != "" {
					if t, err := time.ParseInLocation("2006-01-02T15:04", dueDate, time.Local); err == nil {
						fields["due_date"] = t.UTC().Format(time.RFC3339)
					}
				}
				if _, err := s.rest.CreateTask(token, fields); err != nil {
					data.Error = s.msg(r, user, "errors.failed_create_task", err.Error())
				} else {
					data.Success = s.msg(r, user, "success.task_created")
				}
			}
		}
	}

	// Filters
	filterEdvID := r.URL.Query().Get("endeavour_id")
	filterStatus := r.URL.Query().Get("status")
	filterSearch := r.URL.Query().Get("search")

	tasks, total, err := s.rest.ListTasks(token, filterEdvID, "", filterStatus, filterSearch, "", 50, 0)
	if err != nil && data.Error == "" {
		data.Error = s.msg(r, user, "errors.failed_list_tasks", err.Error())
	}

	// Load endeavours for the create form dropdown
	edvs, _, _ := s.rest.ListEndeavours(token, "", "", 100, 0)

	// If filtering by endeavour, resolve the name
	var filterEdvName string
	if filterEdvID != "" {
		if edv, err := s.rest.GetEndeavour(token, filterEdvID); err == nil {
			filterEdvName = edv.Name
		}
	}

	data.Data = map[string]interface{}{
		"Tasks":              tasks,
		"Total":              total,
		"Endeavours":         edvs,
		"FilterEndeavourID":  filterEdvID,
		"FilterEndeavourName": filterEdvName,
		"FilterStatus":       filterStatus,
		"FilterSearch":       filterSearch,
	}

	s.render(w, r, "tasks.html", data)
}

// handleTaskDetail serves the task detail portal page.
func (s *Server) handleTaskDetail(w http.ResponseWriter, r *http.Request) {
	token := getToken(r)
	user, _ := s.rest.Whoami(token)
	taskID := r.PathValue("id")

	data := PageData{Title: "Task Detail", User: user, CurrentPage: "tasks"}

	if r.Method == http.MethodPost {
		if !validateCSRF(r) {
			s.csrfFailed(w, r, "task_detail.html")
			return
		}
		action := r.FormValue("action")
		switch action {
		case "update_status":
			newStatus := r.FormValue("status")
			if newStatus != "" {
				if _, err := s.rest.UpdateTask(token, taskID, map[string]interface{}{"status": newStatus}); err != nil {
					data.Error = s.msg(r, user, "errors.failed_update_task", err.Error())
				} else {
					data.Success = s.msg(r, user, "success.task_updated")
				}
			}
		case "update":
			fields := map[string]interface{}{}
			if title := r.FormValue("title"); title != "" {
				fields["title"] = title
			}
			if v := r.FormValue("assignee_id"); v != "" {
				fields["assignee_id"] = v
			} else {
				fields["assignee_id"] = ""
			}
			if v := r.FormValue("owner_id"); v != "" {
				fields["owner_id"] = v
			} else {
				fields["owner_id"] = ""
			}
			if estimate := r.FormValue("estimate"); estimate != "" {
				if v, err := strconv.ParseFloat(estimate, 64); err == nil {
					fields["estimate"] = v
				}
			}
			if actual := r.FormValue("actual"); actual != "" {
				if v, err := strconv.ParseFloat(actual, 64); err == nil {
					fields["actual"] = v
				}
			}
			if v := r.FormValue("due_date"); v != "" {
				fields["due_date"] = v
			} else {
				fields["due_date"] = ""
			}
			if len(fields) > 0 {
				if _, err := s.rest.UpdateTask(token, taskID, fields); err != nil {
					data.Error = s.msg(r, user, "errors.failed_update_task", err.Error())
				} else {
					data.Success = s.msg(r, user, "success.task_updated")
				}
			}
		case "cancel":
			reason := r.FormValue("reason")
			if reason == "" {
				data.Error = s.msg(r, user, "errors.reason_required")
			} else {
				fields := map[string]interface{}{"status": "canceled", "canceled_reason": reason}
				if _, err := s.rest.UpdateTask(token, taskID, fields); err != nil {
					var restErr *RESTError
					if errors.As(err, &restErr) && restErr.Code == "quorum_not_met" {
						data.Success = restErr.Message
					} else {
						data.Error = s.msg(r, user, "errors.failed_update_task", err.Error())
					}
				} else {
					data.Success = s.msg(r, user, "success.task_updated")
				}
			}
		}
	}

	task, err := s.rest.GetTask(token, taskID)
	if err != nil {
		data.Error = s.msg(r, user, "errors.task_not_found")
		data.Data = map[string]interface{}{}
		s.render(w, r, "task_detail.html", data)
		return
	}

	// Load comments for this task
	comments, _, _ := s.rest.ListComments(token, "task", taskID, 50, 0)

	// Load resources for assignee/owner dropdowns
	resources, _, _ := s.rest.ListResources(token, "", 200, 0)

	data.Data = map[string]interface{}{
		"Task":      task,
		"TaskTitle": task.Title,
		"Comments":  comments,
		"Resources": resources,
	}

	s.render(w, r, "task_detail.html", data)
}

// handleActivity serves the activity portal page.
func (s *Server) handleActivity(w http.ResponseWriter, r *http.Request) {
	token := getToken(r)
	user, _ := s.rest.Whoami(token)

	offset := queryIntParam(r, "offset", 0)
	limit := 50

	data := PageData{Title: "Activity", User: user, CurrentPage: "activity"}

	entries, total, err := s.rest.ListMyFullActivity(token, limit, offset)
	if err != nil {
		data.Error = s.msg(r, user, "errors.failed_load_activity", err.Error())
	}

	// Build name resolution maps (same as unified activity screen)
	actorMap := s.buildUnifiedActorMap(token, entries, user)
	entityMap, endeavourMap := s.buildUnifiedNameMaps(token, entries)

	nextOffset := offset + limit
	data.Data = map[string]interface{}{
		"Entries":      entries,
		"Total":        total,
		"Limit":        limit,
		"Offset":       offset,
		"NextOffset":   nextOffset,
		"HasNext":      nextOffset < total,
		"HasPrev":      offset > 0,
		"ActorMap":     actorMap,
		"EntityMap":    entityMap,
		"EndeavourMap": endeavourMap,
	}

	s.render(w, r, "activity.html", data)
}

// handleAlerts serves the alerts portal page.
func (s *Server) handleAlerts(w http.ResponseWriter, r *http.Request) {
	token := getToken(r)
	user, _ := s.rest.Whoami(token)

	offset := queryIntParam(r, "offset", 0)
	limit := 50
	threshold := queryIntParam(r, "threshold", 1)

	data := PageData{Title: "Content Alerts", User: user, CurrentPage: "alerts"}

	alerts, total, err := s.rest.ListMyAlerts(token, threshold, limit, offset)
	if err != nil {
		data.Error = s.msg(r, user, "errors.failed_load_alerts", err.Error())
	}

	cgStats, _ := s.rest.MyAlertStats(token)

	data.Data = map[string]interface{}{
		"Alerts":  alerts,
		"Total":   total,
		"CGStats": cgStats,
	}

	s.render(w, r, "alerts.html", data)
}

// handleUsage serves the usage portal page.
func (s *Server) handleUsage(w http.ResponseWriter, r *http.Request) {
	token := getToken(r)
	user, _ := s.rest.Whoami(token)

	data := PageData{Title: "Usage", User: user, CurrentPage: "usage"}
	data.Data = map[string]interface{}{
		"User": user,
	}

	s.render(w, r, "usage.html", data)
}

// handleAbout serves the about portal page.
func (s *Server) handleAbout(w http.ResponseWriter, r *http.Request) {
	if !s.showAbout {
		http.NotFound(w, r)
		return
	}
	token := getToken(r)
	user, _ := s.rest.Whoami(token)

	// Check API health
	apiHealthy := false
	apiVersion := "-"
	health, err := s.rest.Health()
	if err == nil && health.Status == "healthy" {
		apiHealthy = true
		apiVersion = health.Version
	}

	// Determine scheme and base domain from the request host.
	// Staging/production uses subdomains (api.*, mcp.*, ntfy.*);
	// development uses localhost with different ports.
	scheme := "https"
	if !s.secure {
		scheme = "http"
	}
	reqHost := r.Host // e.g., "portal.taskschmiede.com.home.arpa" or "localhost:9090"
	apiBase := s.rest.BaseURL()

	// Health probes always hit internal (localhost) endpoints,
	// since the portal runs on the same host as all services.
	probeProxyURL := replacePort(apiBase, "9001") + "/proxy/health"
	probeNotifyURL := replacePort(apiBase, "9004") + "/notify/health"
	proxyStatus, _ := s.rest.ProbeHealth(probeProxyURL)
	notifyStatus, _ := s.rest.ProbeHealth(probeNotifyURL)

	// Display URLs: staging/production use subdomains, dev uses ports.
	var displayAPI, displayPortal, displayProxy, displayNotify string

	if strings.Contains(reqHost, ".home.arpa") {
		baseDomain := strings.TrimPrefix(reqHost, "portal.")
		displayAPI = scheme + "://api." + baseDomain
		displayPortal = scheme + "://" + reqHost
		displayProxy = scheme + "://mcp." + baseDomain
		displayNotify = ""
	} else if strings.Contains(reqHost, "taskschmiede.dev") {
		displayAPI = scheme + "://api.taskschmiede.dev"
		displayPortal = scheme + "://my.taskschmiede.dev"
		displayProxy = scheme + "://mcp.taskschmiede.dev"
		displayNotify = scheme + "://ntfy.taskschmiede.dev"
	} else {
		displayAPI = apiBase
		displayPortal = scheme + "://" + s.addr
		displayProxy = replacePort(apiBase, "9001")
		displayNotify = replacePort(apiBase, "9004")
	}

	// Resolve docs URL based on host
	docsURL := "https://docs.taskschmiede.dev/guides/"
	if strings.Contains(reqHost, ".home.arpa") {
		docsURL = "https://docs.taskschmiede.dev.home.arpa/guides/"
	}

	// Language list
	var langNames []string
	for _, l := range s.i18n.Languages() {
		langNames = append(langNames, l.Name)
	}

	info := map[string]interface{}{
		"version":       s.version,
		"api_version":   apiVersion,
		"api_url":       displayAPI,
		"api_port":      "9000",
		"api_healthy":   apiHealthy,
		"proxy_url":     displayProxy,
		"proxy_port":    "9001",
		"proxy_status":  proxyStatus,
		"notify_url":    displayNotify,
		"notify_port":   "9004",
		"notify_status": notifyStatus,
		"portal_addr":   displayPortal,
		"portal_port":   portFromAddr(s.addr),
		"secure":        s.secure,
		"languages":     strings.Join(langNames, ", "),
		"docs_url":      docsURL,
	}

	data := PageData{Title: "About", User: user, CurrentPage: "about"}
	data.Data = info
	s.render(w, r, "about.html", data)
}

// replacePort replaces the port in a URL string.
// e.g. replacePort("http://localhost:9000", "9001") -> "http://localhost:9001"
func replacePort(rawURL, newPort string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	host, _, err := net.SplitHostPort(u.Host)
	if err != nil {
		// No port in URL, append it
		u.Host = u.Host + ":" + newPort
	} else {
		u.Host = host + ":" + newPort
	}
	return u.String()
}

// portFromAddr extracts the port from an address like ":9090" or "localhost:9090".
func portFromAddr(addr string) string {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return port
}

// handleMessages serves the messages portal page.
func (s *Server) handleMessages(w http.ResponseWriter, r *http.Request) {
	token := getToken(r)
	user, _ := s.rest.Whoami(token)
	data := PageData{Title: "Messages", User: user, CurrentPage: "messages"}

	if r.Method == http.MethodPost {
		if !validateCSRF(r) {
			s.csrfFailed(w, r, "messages.html")
			return
		}
		action := r.FormValue("action")

		switch action {
		case "send":
			subject := r.FormValue("subject")
			content := r.FormValue("content")
			recipientIDs := r.FormValue("recipient_ids")
			intent := r.FormValue("intent")

			if subject == "" || content == "" || recipientIDs == "" {
				data.Error = s.msg(r, user, "errors.recipient_subject_content_required")
			} else {
				fields := map[string]interface{}{
					"subject":       subject,
					"content":       content,
					"recipient_ids": []string{recipientIDs},
					"intent":        intent,
				}
				if _, err := s.rest.SendMessage(token, fields); err != nil {
					data.Error = s.msg(r, user, "errors.failed_send_message", err.Error())
				} else {
					data.Success = s.msg(r, user, "success.message_sent")
				}
			}

		case "reply":
			msgID := r.FormValue("message_id")
			content := r.FormValue("content")
			if msgID == "" || content == "" {
				data.Error = s.msg(r, user, "errors.message_id_content_required")
			} else {
				if _, err := s.rest.ReplyMessage(token, msgID, content); err != nil {
					data.Error = s.msg(r, user, "errors.failed_send_reply", err.Error())
				} else {
					data.Success = s.msg(r, user, "success.reply_sent")
				}
			}
		}
	}

	filterStatus := r.URL.Query().Get("status")
	offset := queryIntParam(r, "offset", 0)
	limit := 50

	msgs, total, err := s.rest.ListInbox(token, filterStatus, false, limit, offset)
	if err != nil && data.Error == "" {
		data.Error = s.msg(r, user, "errors.failed_load_inbox", err.Error())
	}

	// Fetch available resources for the recipient dropdown
	resources, _, _ := s.rest.ListResources(token, "", 200, 0)

	unreadCount := s.rest.UnreadMessageCount(token)

	data.Data = map[string]interface{}{
		"View":         "inbox",
		"Messages":     msgs,
		"Total":        total,
		"UnreadCount":  unreadCount,
		"Limit":        limit,
		"Offset":       offset,
		"FilterStatus": filterStatus,
		"Resources":    resources,
	}

	s.render(w, r, "messages.html", data)
}

// handleMessageThread serves the message thread portal page.
func (s *Server) handleMessageThread(w http.ResponseWriter, r *http.Request) {
	token := getToken(r)
	user, _ := s.rest.Whoami(token)
	msgID := r.PathValue("id")

	data := PageData{Title: "Message Thread", User: user, CurrentPage: "messages"}

	// Handle reply from thread view
	if r.Method == http.MethodPost {
		if !validateCSRF(r) {
			s.csrfFailed(w, r, "messages.html")
			return
		}
		action := r.FormValue("action")
		if action == "reply" {
			content := r.FormValue("content")
			replyToID := r.FormValue("message_id")
			if content != "" && replyToID != "" {
				if _, err := s.rest.ReplyMessage(token, replyToID, content); err != nil {
					data.Error = s.msg(r, user, "errors.failed_send_reply", err.Error())
				} else {
					data.Success = s.msg(r, user, "success.reply_sent")
				}
			}
		}
	}

	// Mark as read by fetching the message
	_, _ = s.rest.GetMessage(token, msgID)

	// Load thread
	thread, err := s.rest.GetThread(token, msgID)
	if err != nil {
		data.Error = s.msg(r, user, "errors.failed_load_thread", err.Error())
	}

	data.Data = map[string]interface{}{
		"View":      "thread",
		"Thread":    thread,
		"MessageID": msgID,
	}

	s.render(w, r, "messages.html", data)
}

// --- Demands ---

func (s *Server) handleDemands(w http.ResponseWriter, r *http.Request) {
	token := getToken(r)
	user, _ := s.rest.Whoami(token)
	data := PageData{Title: "Demands", User: user, CurrentPage: "demands"}

	if r.Method == http.MethodPost {
		if !validateCSRF(r) {
			s.csrfFailed(w, r, "demands.html")
			return
		}
		action := r.FormValue("action")
		if action == "create" {
			title := r.FormValue("title")
			dtype := r.FormValue("type")
			priority := r.FormValue("priority")
			description := r.FormValue("description")
			if title == "" {
				data.Error = s.msg(r, user, "errors.title_required")
			} else {
				fields := map[string]interface{}{
					"title":       title,
					"description": description,
				}
				if dtype != "" {
					fields["type"] = dtype
				}
				if priority != "" {
					fields["priority"] = priority
				}
				if edvID := r.FormValue("endeavour_id"); edvID != "" {
					fields["endeavour_id"] = edvID
				}
				if _, err := s.rest.CreateDemand(token, fields); err != nil {
					data.Error = s.msg(r, user, "errors.failed_create_demand", err.Error())
				} else {
					data.Success = s.msg(r, user, "success.demand_created")
				}
			}
		}
	}

	search := r.URL.Query().Get("search")
	filterStatus := r.URL.Query().Get("status")
	dtype := r.URL.Query().Get("type")
	priority := r.URL.Query().Get("priority")
	offset := queryIntParam(r, "offset", 0)
	limit := 50

	demands, total, err := s.rest.ListDemands(token, search, filterStatus, dtype, priority, "", limit, offset)
	if err != nil && data.Error == "" {
		data.Error = s.msg(r, user, "errors.failed_load_demands", err.Error())
	}

	// Load endeavours for the create form dropdown
	edvs, _, _ := s.rest.ListEndeavours(token, "", "", 100, 0)

	nextOffset := offset + limit
	data.Data = map[string]interface{}{
		"Demands":      demands,
		"Total":        total,
		"Endeavours":   edvs,
		"Search":       search,
		"FilterStatus": filterStatus,
		"Type":         dtype,
		"Priority":     priority,
		"Offset":       offset,
		"Limit":        limit,
		"NextOffset":   nextOffset,
		"HasNext":      nextOffset < total,
		"HasPrev":      offset > 0,
	}

	s.render(w, r, "demands.html", data)
}

// handleDemandDetail serves the demand detail portal page.
func (s *Server) handleDemandDetail(w http.ResponseWriter, r *http.Request) {
	token := getToken(r)
	user, _ := s.rest.Whoami(token)
	demandID := r.PathValue("id")

	data := PageData{Title: "Demand Detail", User: user, CurrentPage: "demands"}

	if r.Method == http.MethodPost {
		if !validateCSRF(r) {
			s.csrfFailed(w, r, "demand_detail.html")
			return
		}
		action := r.FormValue("action")
		switch action {
		case "update":
			fields := map[string]interface{}{}
			// Fetch current state to detect changes and avoid no-op transitions
			current, _ := s.rest.GetDemand(token, demandID)
			if v := r.FormValue("title"); v != "" && (current == nil || v != current.Title) {
				fields["title"] = v
			}
			if v := r.FormValue("description"); current == nil || v != current.Description {
				fields["description"] = v
			}
			if v := r.FormValue("type"); v != "" && (current == nil || v != current.Type) {
				fields["type"] = v
			}
			if st := r.FormValue("status"); st != "" && (current == nil || st != current.Status) {
				fields["status"] = st
			}
			if pr := r.FormValue("priority"); pr != "" && (current == nil || pr != current.Priority) {
				fields["priority"] = pr
			}
			newOwner := r.FormValue("owner_id")
			if current == nil || newOwner != current.OwnerID {
				fields["owner_id"] = newOwner
			}
			if len(fields) > 0 {
				if _, err := s.rest.UpdateDemand(token, demandID, fields); err != nil {
					var restErr *RESTError
					if errors.As(err, &restErr) && restErr.Code == "quorum_not_met" {
						data.Success = restErr.Message
					} else {
						data.Error = s.msg(r, user, "errors.failed_update_demand", err.Error())
					}
				} else {
					data.Success = s.msg(r, user, "success.demand_updated")
				}
			}
		case "cancel":
			reason := r.FormValue("reason")
			if reason == "" {
				data.Error = s.msg(r, user, "errors.reason_required")
			} else {
				fields := map[string]interface{}{"status": "canceled", "canceled_reason": reason}
				if _, err := s.rest.UpdateDemand(token, demandID, fields); err != nil {
					var restErr *RESTError
					if errors.As(err, &restErr) && restErr.Code == "quorum_not_met" {
						data.Success = restErr.Message
					} else {
						data.Error = s.msg(r, user, "errors.failed_update_demand", err.Error())
					}
				} else {
					data.Success = s.msg(r, user, "success.demand_updated")
				}
			}
		}
	}

	demand, err := s.rest.GetDemand(token, demandID)
	if err != nil {
		data.Error = s.msg(r, user, "errors.demand_not_found")
		data.Data = map[string]interface{}{}
		s.render(w, r, "demand_detail.html", data)
		return
	}

	// Load linked tasks for this demand.
	linkedTasks, _, _ := s.rest.ListTasks(token, "", "", "", "", demand.ID, 50, 0)
	// Load resources for owner dropdown.
	resources, _, _ := s.rest.AdminListResources(token, "", "", "active", "", 200, 0)

	data.Title = demand.Title
	data.Data = map[string]interface{}{
		"Demand":      demand,
		"DemandTitle": demand.Title,
		"LinkedTasks": linkedTasks,
		"Resources":   resources,
	}
	s.render(w, r, "demand_detail.html", data)
}

// --- Reports ---

func (s *Server) handleReport(w http.ResponseWriter, r *http.Request) {
	token := getToken(r)
	user, _ := s.rest.Whoami(token)
	scope := r.PathValue("scope")
	id := r.PathValue("id")

	// Handle email action (POST)
	if r.Method == http.MethodPost {
		if !validateCSRF(r) {
			s.csrfFailed(w, r, "report_view.html")
			return
		}
		action := r.FormValue("action")
		if action == "email" {
			if err := s.rest.EmailReport(token, scope, id); err != nil {
				data := PageData{Title: "Report", User: user, CurrentPage: "reports"}
				data.Error = s.msg(r, user, "errors.failed_send_report", err.Error())
				s.render(w, r, "report_view.html", data)
				return
			}
			http.Redirect(w, r, fmt.Sprintf("/reports/%s/%s?sent=1", scope, id), http.StatusSeeOther)
			return
		}
	}

	// Generate report
	report, err := s.rest.GenerateReport(token, scope, id)
	if err != nil {
		data := PageData{Title: "Report", User: user, CurrentPage: "reports"}
		data.Error = s.msg(r, user, "errors.failed_generate_report", err.Error())
		s.render(w, r, "report_view.html", data)
		return
	}

	// Download as .md file
	format := r.URL.Query().Get("format")
	if format == "download" {
		filename := fmt.Sprintf("%s_%s_report.md", scope, id)
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
		_, _ = w.Write([]byte(report.Markdown))
		return
	}

	data := PageData{
		Title:       "Report: " + report.Title,
		User:        user,
		CurrentPage: "reports",
	}

	if r.URL.Query().Get("sent") == "1" {
		data.Success = s.msg(r, user, "success.report_emailed")
	}

	data.Data = map[string]interface{}{
		"Report": report,
		"Scope":  scope,
		"ID":     id,
	}
	s.render(w, r, "report_view.html", data)
}

// --- KPI JSON proxies ---

func (s *Server) handleKPICurrent(w http.ResponseWriter, r *http.Request) {
	token := getToken(r)
	data, err := s.rest.do("GET", "/api/v1/kpi/current", token, nil)
	if err != nil {
		http.Error(w, `{"error":"failed to fetch KPI data"}`, http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(data)
}

// handleKPIHistory serves the k p i history portal page.
func (s *Server) handleKPIHistory(w http.ResponseWriter, r *http.Request) {
	token := getToken(r)
	params := url.Values{}
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 1000 {
			params.Set("limit", strconv.Itoa(n))
		}
	}
	if v := r.URL.Query().Get("since"); v != "" {
		params.Set("since", v)
	}
	path := "/api/v1/kpi/history"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}
	data, err := s.rest.do("GET", path, token, nil)
	if err != nil {
		http.Error(w, `{"error":"failed to fetch KPI history"}`, http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(data)
}

// cronDesc converts a standard 5-field cron expression into a human-readable
// description, similar to crontab.guru. Supports wildcards, single values,
// lists, ranges, and step intervals.
func cronDesc(expr string) string {
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return ""
	}
	minute, hour, dom, month, dow := fields[0], fields[1], fields[2], fields[3], fields[4]

	var parts []string

	// Time description
	switch {
	case minute == "*" && hour == "*":
		parts = append(parts, "Every minute")
	case minute == "*" && hour != "*":
		parts = append(parts, "Every minute past hour "+fmtHour(hour))
	case hour == "*":
		parts = append(parts, "At minute "+minute+" of every hour")
	default:
		parts = append(parts, "At "+fmtCronTime(hour, minute))
	}

	// Day-of-month
	if dom != "*" {
		parts = append(parts, "on day "+dom+" of the month")
	}

	// Month
	if month != "*" {
		parts = append(parts, "in "+fmtMonth(month))
	}

	// Day-of-week
	if dow != "*" {
		parts = append(parts, "on "+fmtDOW(dow))
	}

	return strings.Join(parts, " ")
}

var dowNames = []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}
var monthNames = []string{"", "January", "February", "March", "April", "May", "June",
	"July", "August", "September", "October", "November", "December"}

func fmtCronTime(hour, minute string) string {
	h, err := strconv.Atoi(hour)
	if err != nil {
		return hour + ":" + minute
	}
	m, err := strconv.Atoi(minute)
	if err != nil {
		return hour + ":" + minute
	}
	return fmt.Sprintf("%02d:%02d", h, m)
}

func fmtHour(hour string) string {
	h, err := strconv.Atoi(hour)
	if err != nil {
		return hour
	}
	return fmt.Sprintf("%02d:00", h)
}

func fmtDOW(dow string) string {
	// Handle range like "1-5"
	if strings.Contains(dow, "-") {
		rangeParts := strings.SplitN(dow, "-", 2)
		from := dowName(rangeParts[0])
		to := dowName(rangeParts[1])
		return from + " through " + to
	}
	// Handle list like "1,3,5"
	if strings.Contains(dow, ",") {
		days := strings.Split(dow, ",")
		names := make([]string, len(days))
		for i, d := range days {
			names[i] = dowName(d)
		}
		if len(names) == 2 {
			return names[0] + " and " + names[1]
		}
		return strings.Join(names[:len(names)-1], ", ") + ", and " + names[len(names)-1]
	}
	return dowName(dow)
}

func dowName(s string) string {
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 || n > 6 {
		return s
	}
	return dowNames[n]
}

func fmtMonth(month string) string {
	n, err := strconv.Atoi(month)
	if err != nil || n < 1 || n > 12 {
		return month
	}
	return monthNames[n]
}

// handleOrganizationExport streams the org export JSON as a download.
func (s *Server) handleOrganizationExport(w http.ResponseWriter, r *http.Request) {
	token := getToken(r)
	id := r.PathValue("id")

	body, err := s.rest.ExportRaw(token, "/api/v1/organizations/"+id+"/export")
	if err != nil {
		http.Error(w, "Export failed", http.StatusInternalServerError)
		return
	}
	defer func() { _ = body.Close() }()

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", id+".json"))
	_, _ = io.Copy(w, body)
}

// handleEndeavourExport streams the endeavour export JSON as a download.
func (s *Server) handleEndeavourExport(w http.ResponseWriter, r *http.Request) {
	token := getToken(r)
	id := r.PathValue("id")

	body, err := s.rest.ExportRaw(token, "/api/v1/endeavours/"+id+"/export")
	if err != nil {
		http.Error(w, "Export failed", http.StatusInternalServerError)
		return
	}
	defer func() { _ = body.Close() }()

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", id+".json"))
	_, _ = io.Copy(w, body)
}

// handleSupport serves the support portal page.
func (s *Server) handleSupport(w http.ResponseWriter, r *http.Request) {
	if s.supportURL == "" {
		http.NotFound(w, r)
		return
	}
	token := getToken(r)
	user, _ := s.rest.Whoami(token)

	type supportData struct {
		User    map[string]interface{}
		CaseID  string
		Topic   string
		Subject string
		Message string
	}

	data := PageData{
		Title:       "Support",
		User:        user,
		CurrentPage: "support",
	}

	sd := supportData{User: user}

	if r.Method == http.MethodPost {
		// JSON request from the inline support panel (no CSRF -- authenticated via cookie).
		if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
			var req struct {
				Topic   string `json:"topic"`
				Subject string `json:"subject"`
				Message string `json:"message"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error":"invalid_json"}`))
				return
			}
			if strings.TrimSpace(req.Message) == "" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error":"message_required"}`))
				return
			}
			userName, userEmail := "", ""
			if user != nil {
				if n, ok := user["name"].(string); ok {
					userName = n
				}
				if e, ok := user["email"].(string); ok {
					userEmail = e
				}
			}
			payload := map[string]string{
				"topic": req.Topic, "name": userName, "email": userEmail,
				"subject": req.Subject, "message": req.Message, "source": "portal",
			}
			jsonBody, _ := json.Marshal(payload)
			fwdReq, err := http.NewRequest("POST", s.supportURL+"/api/case", bytes.NewReader(jsonBody))
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"error":"internal_error"}`))
				return
			}
			fwdReq.Header.Set("Content-Type", "application/json")
			if s.supportAPIKey != "" {
				fwdReq.Header.Set("Authorization", "Bearer "+s.supportAPIKey)
			}
			// Forward the real client IP so the support service can rate-limit per user.
			// Prefer an existing X-Forwarded-For (portal sits behind nginx).
			clientIP := r.Header.Get("X-Forwarded-For")
			if clientIP == "" {
				clientIP, _, _ = net.SplitHostPort(r.RemoteAddr)
				if clientIP == "" {
					clientIP = r.RemoteAddr
				}
			}
			fwdReq.Header.Set("X-Forwarded-For", clientIP)
			resp, err := http.DefaultClient.Do(fwdReq)
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadGateway)
				_, _ = w.Write([]byte(`{"error":"service_unavailable"}`))
				return
			}
			defer func() { _ = resp.Body.Close() }()

			// Read response to extract case_id for the internal message.
			respBody, _ := io.ReadAll(resp.Body)
			if resp.StatusCode == http.StatusCreated {
				var caseResult map[string]string
				if json.Unmarshal(respBody, &caseResult) == nil {
					if caseID := caseResult["case_id"]; caseID != "" {
						// Send internal message to the user as confirmation.
						resID, _ := user["resource_id"].(string)
						slog.Info("Support case created, sending confirmation message",
							"case_id", caseID, "resource_id", resID, "has_resource_id", resID != "")
						if resID != "" {
							msgFields := map[string]interface{}{
								"subject":       "Taskschmiede: " + caseID,
								"content":       req.Subject + "\n\n" + req.Message,
								"recipient_ids": []string{resID},
								"intent":        "info",
							}
							if _, err := s.rest.SendMessage(token, msgFields); err != nil {
								slog.Warn("Failed to send support confirmation message",
									"case_id", caseID, "resource_id", resID, "error", err)
							}
						}
					}
				}
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(resp.StatusCode)
			_, _ = w.Write(respBody)
			return
		}

		// Form POST from the fallback /support page.
		if !validateCSRF(r) {
			s.csrfFailed(w, r, "support.html")
			return
		}

		sd.Topic = r.FormValue("topic")
		sd.Subject = r.FormValue("subject")
		sd.Message = r.FormValue("message")

		if sd.Message == "" {
			data.Error = s.msg(r, user, "errors.message_required")
			data.Data = sd
			s.render(w, r, "support.html", data)
			return
		}

		if sd.Topic == "" {
			sd.Topic = "support"
		}

		userName := ""
		userEmail := ""
		if user != nil {
			if n, ok := user["name"].(string); ok {
				userName = n
			}
			if e, ok := user["email"].(string); ok {
				userEmail = e
			}
		}

		payload := map[string]string{
			"topic":   sd.Topic,
			"name":    userName,
			"email":   userEmail,
			"subject": sd.Subject,
			"message": sd.Message,
			"source":  "portal",
		}
		jsonBody, _ := json.Marshal(payload)

		fwdReq2, err := http.NewRequest("POST", s.supportURL+"/api/case", bytes.NewReader(jsonBody))
		if err != nil {
			slog.Warn("Support case request creation failed", "error", err)
			data.Error = s.msg(r, user, "errors.service_unavailable")
			data.Data = sd
			s.render(w, r, "support.html", data)
			return
		}
		fwdReq2.Header.Set("Content-Type", "application/json")
		if s.supportAPIKey != "" {
			fwdReq2.Header.Set("Authorization", "Bearer "+s.supportAPIKey)
		}
		clientIP2 := r.Header.Get("X-Forwarded-For")
		if clientIP2 == "" {
			clientIP2, _, _ = net.SplitHostPort(r.RemoteAddr)
			if clientIP2 == "" {
				clientIP2 = r.RemoteAddr
			}
		}
		fwdReq2.Header.Set("X-Forwarded-For", clientIP2)

		resp, err := http.DefaultClient.Do(fwdReq2)
		if err != nil {
			slog.Warn("Support case creation failed", "error", err)
			data.Error = s.msg(r, user, "errors.service_unavailable")
			data.Data = sd
			s.render(w, r, "support.html", data)
			return
		}
		defer func() { _ = resp.Body.Close() }()

		var result map[string]string
		_ = json.NewDecoder(resp.Body).Decode(&result)

		if resp.StatusCode != http.StatusCreated {
			data.Error = s.msg(r, user, "errors.service_unavailable")
			data.Data = sd
			s.render(w, r, "support.html", data)
			return
		}

		sd.CaseID = result["case_id"]
		data.Success = s.msg(r, user, "success.support_submitted")
		data.Data = sd
		s.render(w, r, "support.html", data)
		return
	}

	// GET: pre-fill from query params.
	sd.Topic = r.URL.Query().Get("topic")
	if sd.Topic == "" {
		sd.Topic = "support"
	}
	sd.Subject = r.URL.Query().Get("subject")
	data.Data = sd
	s.render(w, r, "support.html", data)
}
