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


// Package main provides the entry point for the Taskschmiede server binary.
// Taskschmiede is a task and project management system for AI agents and humans.
package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
	_ "time/tzdata" // Embed timezone database for consistent timezone support

	"gopkg.in/yaml.v3"

	"github.com/QuestFinTech/taskschmiede/internal/api"
	"github.com/QuestFinTech/taskschmiede/internal/docs"
	"github.com/QuestFinTech/taskschmiede/internal/email"
	"github.com/QuestFinTech/taskschmiede/internal/envutil"
	"github.com/QuestFinTech/taskschmiede/internal/intercom"
	"github.com/QuestFinTech/taskschmiede/internal/llmclient"
	"github.com/QuestFinTech/taskschmiede/internal/logging"
	"github.com/QuestFinTech/taskschmiede/internal/mcp"
	"github.com/QuestFinTech/taskschmiede/internal/notify"
	"github.com/QuestFinTech/taskschmiede/internal/security"
	"github.com/QuestFinTech/taskschmiede/internal/service"
	"github.com/QuestFinTech/taskschmiede/internal/storage"
	"github.com/QuestFinTech/taskschmiede/internal/ticker"
)


// banner holds the ASCII art banner displayed on startup.
//
//go:embed banner.txt
var banner string

// tagline is the program's one-line description shown in help output.
const tagline = "Task and project management for AI agents and humans"

// Version, Commit, and BuildTime are set via ldflags at build time.
var (
	Version   = "0.1.0"
	Commit    = "unknown"
	BuildTime = "unknown"
)

// Config holds all configuration options for the Taskschmiede server.
type Config struct {
	// Database settings.
	Database DatabaseConfig `yaml:"database"`

	// Server settings.
	Server ServerConfig `yaml:"server"`

	// Logging settings.
	Log logging.LogConfig `yaml:"log"`

	// Ticker settings (periodic task runner).
	Ticker TickerConfig `yaml:"ticker"`

	// Email settings.
	Email EmailConfig `yaml:"email"`

	// Messaging settings.
	Messaging MessagingConfig `yaml:"messaging"`

	// Injection review settings.
	InjectionReview InjectionReviewConfig `yaml:"injection-review"`

	// Content guard settings (WS-4.5).
	ContentGuard ContentGuardConfig `yaml:"content-guard"`

	// Ritual executor settings (Taskschmied Phase A).
	RitualExecutor RitualExecutorConfig `yaml:"ritual-executor"`

	// Security settings.
	Security SecurityConfig `yaml:"security"`

	// Notification service client settings (J-4).
	Notify NotifyClientConfig `yaml:"notify"`

	// Instance settings (quotas applied at startup).
	Instance InstanceConfig `yaml:"instance"`

	// Tier definitions (seeded on first run; managed via admin portal afterwards).
	Tiers TierConfig `yaml:"tiers"`

	// Registration settings.
	Registration RegistrationConfig `yaml:"registration"`
}

// InstanceConfig holds instance-level quota defaults.
// Values are applied to the policy table on startup.
// A zero value means "do not override the DB value".
type InstanceConfig struct {
	MaxActiveUsers int `yaml:"max-active-users"`
}

// TierConfig holds tier system configuration.
type TierConfig struct {
	DefaultTier int             `yaml:"default-tier"`
	Definitions []TierDefConfig `yaml:"definitions"`
}

// TierDefConfig holds a single tier definition from config.yaml.
type TierDefConfig struct {
	ID                  int    `yaml:"id"`
	Name                string `yaml:"name"`
	MaxUsers            int    `yaml:"max-users"`
	MaxOrgs             int    `yaml:"max-orgs"`
	MaxAgentsPerOrg     int    `yaml:"max-agents-per-org"`
	MaxEndeavoursPerOrg int    `yaml:"max-endeavours-per-org"`
	MaxActiveEndeavours int    `yaml:"max-active-endeavours"`
	MaxTeamsPerOrg      int    `yaml:"max-teams-per-org"`
	MaxCreationsPerHour int    `yaml:"max-creations-per-hour"`
}

// RegistrationConfig holds registration flow settings.
type RegistrationConfig struct {
	RequireKYC *bool `yaml:"require-kyc"`
}

// RequireKYCEnabled returns true if KYC address collection is required.
// Defaults to true (SaaS behavior) if not set.
func (c RegistrationConfig) RequireKYCEnabled() bool {
	if c.RequireKYC == nil {
		return true
	}
	return *c.RequireKYC
}

// DatabaseConfig holds database connection settings.
type DatabaseConfig struct {
	Path string `yaml:"path"`
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	MCPPort        int           `yaml:"mcp-port"`
	SessionTimeout time.Duration `yaml:"session-timeout"`
	AgentTokenTTL  time.Duration `yaml:"agent-token-ttl"`
}

// TickerConfig holds ticker (periodic task runner) settings.
type TickerConfig struct {
	Interval time.Duration `yaml:"interval"`
	KPI      KPIConfig     `yaml:"kpi"`
}

// KPIConfig holds KPI snapshot collection settings.
type KPIConfig struct {
	Enabled   bool          `yaml:"enabled"`
	Interval  time.Duration `yaml:"interval"`
	OutputDir string        `yaml:"output-dir"`
}

// EmailConfig holds email settings with shared server config and per-account identity.
type EmailConfig struct {
	// Shared server settings
	SMTPHost   string `yaml:"smtp-host"`
	SMTPPort   int    `yaml:"smtp-port"`
	SMTPUseTLS bool   `yaml:"smtp-use-tls"`
	SMTPUseSSL bool   `yaml:"smtp-use-ssl"`
	IMAPHost   string `yaml:"imap-host"`
	IMAPPort   int    `yaml:"imap-port"`
	IMAPUseTLS bool   `yaml:"imap-use-tls"`
	IMAPUseSSL bool   `yaml:"imap-use-ssl"`

	// Per-account identity and credentials
	Support EmailAccountConfig `yaml:"support"`
	Intercom EmailAccountConfig `yaml:"intercom"`

	// Verification settings
	VerificationTimeout time.Duration `yaml:"verification-timeout"`

	// Portal URL for constructing verification/reset links in emails
	PortalURL string `yaml:"portal-url"`
}

// EmailAccountConfig holds identity and credentials for a single email account.
type EmailAccountConfig struct {
	Name     string `yaml:"name"`
	Address  string `yaml:"address"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

// AgentOnboardingConfig holds agent onboarding gate settings.
type AgentOnboardingConfig struct {
	RequireEmailVerification bool `yaml:"require-email-verification"`
	RequireInterview         bool `yaml:"require-interview"`
}

// SecurityConfig holds security settings.
type SecurityConfig struct {
	RateLimit            security.RateLimitConfig `yaml:"rate-limit"`
	ConnLimit            security.ConnLimitConfig `yaml:"conn-limit"`
	Headers              security.HeadersConfig   `yaml:"headers"`
	BodyLimit            security.BodyLimitConfig `yaml:"body-limit"`
	Audit                AuditConfig              `yaml:"audit"`
	CORSOrigins          []string                 `yaml:"cors-origins"`
	DeploymentMode       string                   `yaml:"deployment-mode"`
	AllowSelfRegistration bool                    `yaml:"allow-self-registration"`
	AgentOnboarding      AgentOnboardingConfig    `yaml:"agent-onboarding"`
}

// MessagingConfig holds messaging settings.
type MessagingConfig struct {
	DatabasePath string         `yaml:"database-path"`
	Intercom     IntercomConfig `yaml:"intercom"`
}

// IntercomConfig holds email bridge operational settings.
type IntercomConfig struct {
	Enabled          bool          `yaml:"enabled"`
	ReplyTTL         time.Duration `yaml:"reply-ttl"`
	SweepInterval    time.Duration `yaml:"sweep-interval"`
	SendInterval     time.Duration `yaml:"send-interval"`
	MaxRetries       int           `yaml:"max-retries"`
	MaxInboundPerHour int          `yaml:"max-inbound-per-hour"`
	DedupWindow      time.Duration `yaml:"dedup-window"`
}

// LLMServiceConfig holds the shared LLM provider fields used by injection
// review, content guard, and ritual executor configs.
type LLMServiceConfig struct {
	Enabled         bool          `yaml:"enabled"`
	Provider        string        `yaml:"provider"`
	Model           string        `yaml:"model"`
	APIKey          string        `yaml:"api-key"`
	APIURL          string        `yaml:"api-url"`
	Timeout         time.Duration `yaml:"timeout"`
	MaxRetries      int           `yaml:"max-retries"`
	TickerInterval  time.Duration `yaml:"ticker-interval"`
	Temperature     *float64      `yaml:"temperature"`
	ReasoningEffort string        `yaml:"reasoning-effort"`
	ReasoningTokens int           `yaml:"reasoning-tokens"`
	// Fallback LLM provider (optional -- used when primary is unavailable).
	FallbackProvider string        `yaml:"fallback-provider"`
	FallbackModel    string        `yaml:"fallback-model"`
	FallbackAPIKey   string        `yaml:"fallback-api-key"`
	FallbackAPIURL   string        `yaml:"fallback-api-url"`
	FallbackTimeout  time.Duration `yaml:"fallback-timeout"`
}

// InjectionReviewConfig holds post-hoc injection detection settings.
type InjectionReviewConfig struct {
	LLMServiceConfig `yaml:",inline"`
}

// ContentGuardConfig holds LLM-assisted content scoring settings (WS-4.5).
type ContentGuardConfig struct {
	LLMServiceConfig `yaml:",inline"`
	ScoreThreshold   int `yaml:"score-threshold"`
}

// RitualExecutorConfig holds Taskschmied ritual execution settings (Phase A).
type RitualExecutorConfig struct {
	LLMServiceConfig `yaml:",inline"`
	MaxTokens        int `yaml:"max-tokens"`
}

// NotifyClientConfig holds settings for the notification service client.
// When URL is set, the app server sends events to the notification service.
type NotifyClientConfig struct {
	URL       string `yaml:"url"`        // e.g. http://localhost:9004
	AuthToken string `yaml:"auth-token"` // shared secret
}

// AuditConfig holds audit logging settings.
type AuditConfig struct {
	BufferSize int `yaml:"buffer-size"`
}

// DefaultConfig returns the default configuration with sensible values.
func DefaultConfig() *Config {
	execDir, _ := os.Executable()
	appDir := filepath.Dir(execDir)

	return &Config{
		Database: DatabaseConfig{
			Path: filepath.Join(appDir, "taskschmiede.db"),
		},
		Server: ServerConfig{
			MCPPort:        9000,
			SessionTimeout: 2 * time.Hour,
			AgentTokenTTL:  30 * time.Minute,
		},
		Log: logging.LogConfig{
			File:  filepath.Join(appDir, "taskschmiede.log"),
			Level: "INFO",
		},
		Ticker: TickerConfig{
			Interval: 1 * time.Second,
			KPI: KPIConfig{
				Enabled:  true,
				Interval: 1 * time.Minute,
			},
		},
		Email: EmailConfig{
			SMTPPort:            465,
			SMTPUseSSL:          true,
			IMAPPort:            993,
			IMAPUseSSL:          true,
			VerificationTimeout: 15 * time.Minute,
		},
		Messaging: MessagingConfig{
			DatabasePath: "", // derived from main DB path
			Intercom: IntercomConfig{
				Enabled:          false,
				ReplyTTL:         30 * 24 * time.Hour,
				SweepInterval:    1 * time.Minute,
				SendInterval:     30 * time.Second,
				MaxRetries:       3,
				MaxInboundPerHour: 20,
				DedupWindow:      1 * time.Hour,
			},
		},
		Security: SecurityConfig{
			RateLimit:             security.DefaultRateLimitConfig(),
			ConnLimit:             security.DefaultConnLimitConfig(),
			Headers:               security.DefaultHeadersConfig(),
			BodyLimit:             security.DefaultBodyLimitConfig(),
			Audit:                 AuditConfig{BufferSize: 1024},
			DeploymentMode:        "open",
			AllowSelfRegistration: true,
			AgentOnboarding: AgentOnboardingConfig{
				RequireEmailVerification: true,
				RequireInterview:         true,
			},
		},
	}
}

// LoadConfig loads configuration from a YAML file, merging with defaults.
func LoadConfig(path string) (*Config, error) {
	cfg := DefaultConfig()

	if path == "" {
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	// Expand environment variables in the config file
	expanded := envutil.ExpandEnvVars(string(data))

	if err := yaml.Unmarshal([]byte(expanded), cfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}

	// Validate deployment mode
	switch cfg.Security.DeploymentMode {
	case "open", "trusted":
		// valid
	case "":
		cfg.Security.DeploymentMode = "open"
	default:
		return nil, fmt.Errorf("invalid security.deployment-mode %q: must be \"open\" or \"trusted\"", cfg.Security.DeploymentMode)
	}

	// In open mode, agent onboarding gates are always enforced.
	if cfg.Security.DeploymentMode == "open" {
		cfg.Security.AgentOnboarding.RequireEmailVerification = true
		cfg.Security.AgentOnboarding.RequireInterview = true
	}

	return cfg, nil
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "serve":
		if err := cmdServe(args); err != nil {
			log.Fatalf("Error: %v", err)
		}
	case "docs":
		if err := cmdDocs(args); err != nil {
			log.Fatalf("Error: %v", err)
		}
	case "version", "-v", "--version":
		printVersion()
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

// versionString returns the version with a 'v' prefix.
func versionString() string {
	v := Version
	if len(v) > 0 && v[0] != 'v' {
		v = "v" + v
	}
	return v
}

// printVersion displays version, commit, and build time.
func printVersion() {
	fmt.Printf("Taskschmiede %s\n", versionString())
	fmt.Printf("  Commit:  %s\n", Commit)
	fmt.Printf("  Built:   %s\n", BuildTime)
	fmt.Println("  Created in Luxembourg")
}

// printUsage displays the top-level usage message with available commands.
func printUsage() {
	fmt.Print(banner)
	fmt.Println(tagline)
	fmt.Printf("Version: %s\n", versionString())
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  taskschmiede <command> [options]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  serve     Start the Taskschmiede server")
	fmt.Println("  docs      Generate documentation")
	fmt.Println("  version   Show version information")
	fmt.Println("  help      Show this help message")
	fmt.Println()
	fmt.Println("Run 'taskschmiede <command> --help' for command options.")
	fmt.Println()
	fmt.Println("Configuration:")
	fmt.Println("  Configuration can be provided via command-line flags or a YAML file.")
	fmt.Println("  Command-line flags take precedence over config file values.")
	fmt.Println()
	fmt.Println("Example:")
	fmt.Println("  taskschmiede serve --port 9000 --log-level DEBUG")
	fmt.Println("  taskschmiede serve --config-file /etc/taskschmiede/config.yaml")
}

// printDocsUsage displays usage for the docs subcommand and its children.
func printDocsUsage() {
	fmt.Println("Usage: taskschmiede docs <subcommand> [options]")
	fmt.Println()
	fmt.Println("Generate documentation from embedded tool definitions.")
	fmt.Println()
	fmt.Println("Subcommands:")
	fmt.Println("  export      Export tool registry as JSON or copy OpenAPI spec (filtered)")
	fmt.Println("  hugo        Generate Hugo-compatible Markdown pages from exported data")
	fmt.Println()
	fmt.Println("Options for 'export':")
	fmt.Println("  --format <fmt>     Export format: json, openapi (default: json)")
	fmt.Println("  --output <path>    Output file or directory (default: stdout for json, ./website/hugo/static/ for openapi)")
	fmt.Println("  --help             Show this help message")
	fmt.Println()
	fmt.Println("Options for 'hugo':")
	fmt.Println("  --input <path>     Path to mcp-tools.json (default: ./website/hugo/static/mcp-tools.json)")
	fmt.Println("  --output <path>    Output directory for generated Markdown (default: ./website/hugo/content/reference/mcp-tools)")
	fmt.Println("  --help             Show this help message")
	fmt.Println()
	fmt.Println("Example:")
	fmt.Println("  taskschmiede docs export --format json --output ./website/hugo/static/mcp-tools.json")
	fmt.Println("  taskschmiede docs export --format openapi --output ./website/hugo/static/")
	fmt.Println("  taskschmiede docs hugo --input ./website/hugo/static/mcp-tools.json")
}

// cmdDocs dispatches the docs subcommand to export or hugo generation.
func cmdDocs(args []string) error {
	if len(args) == 0 {
		printDocsUsage()
		return nil
	}

	subCmd := args[0]
	subArgs := args[1:]

	switch subCmd {
	case "export":
		return cmdDocsExport(subArgs)
	case "hugo":
		return cmdDocsHugo(subArgs)
	case "help", "--help", "-h":
		printDocsUsage()
		return nil
	default:
		fmt.Fprintf(os.Stderr, "Unknown docs subcommand: %s\n\n", subCmd)
		printDocsUsage()
		os.Exit(1)
	}
	return nil
}

// cmdDocsExport exports the tool registry as JSON or a filtered OpenAPI spec.
func cmdDocsExport(args []string) error {
	var (
		format   string
		output   string
		showHelp bool
	)

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--format":
			if i+1 < len(args) {
				format = args[i+1]
				i++
			}
		case "--output":
			if i+1 < len(args) {
				output = args[i+1]
				i++
			}
		case "--help", "-h":
			showHelp = true
		}
	}

	if showHelp {
		printDocsUsage()
		return nil
	}

	if format == "" {
		format = "json"
	}

	registry := docs.DefaultRegistry(versionString())

	switch format {
	case "json":
		data, err := registry.ToJSON()
		if err != nil {
			return fmt.Errorf("export JSON: %w", err)
		}
		if output == "" || output == "-" {
			fmt.Println(string(data))
		} else {
			if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
				return fmt.Errorf("create output directory: %w", err)
			}
			if err := os.WriteFile(output, data, 0o644); err != nil {
				return fmt.Errorf("write JSON: %w", err)
			}
			fmt.Printf("Exported %d tools to %s\n", len(registry.All()), output)
		}
	case "openapi":
		// Find and copy openapi.yaml (filtering out internal endpoints)
		specPaths := []string{
			"docs/openapi.yaml",
			filepath.Join(filepath.Dir(filepath.Dir(output)), "docs", "openapi.yaml"),
		}
		var specData []byte
		var specErr error
		for _, p := range specPaths {
			specData, specErr = os.ReadFile(p)
			if specErr == nil {
				break
			}
		}
		if specErr != nil {
			return fmt.Errorf("openapi.yaml not found (searched: %v): %w", specPaths, specErr)
		}

		// Filter out internal endpoints for public docs
		filtered, removed, err := filterInternalOpenAPIPaths(specData)
		if err != nil {
			return fmt.Errorf("filter openapi spec: %w", err)
		}
		specData = filtered

		if output == "" {
			output = "./website/hugo/static/"
		}
		// If output is a directory, append filename
		info, err := os.Stat(output)
		if err == nil && info.IsDir() {
			output = filepath.Join(output, "openapi.yaml")
		} else if strings.HasSuffix(output, "/") {
			if err := os.MkdirAll(output, 0o755); err != nil {
				return fmt.Errorf("create output directory: %w", err)
			}
			output = filepath.Join(output, "openapi.yaml")
		}
		if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
			return fmt.Errorf("create output directory: %w", err)
		}
		if err := os.WriteFile(output, specData, 0o644); err != nil {
			return fmt.Errorf("write openapi.yaml: %w", err)
		}
		fmt.Printf("Exported openapi.yaml to %s (%d internal paths removed)\n", output, removed)
	default:
		return fmt.Errorf("unknown format: %s (supported: json, openapi)", format)
	}

	return nil
}

// internalAPIPrefixes lists REST API path prefixes classified as internal
// per docs/API_VISIBILITY.md. These are filtered out of the public OpenAPI spec.
var internalAPIPrefixes = []string{
	"/api/v1/admin/setup",
	"/api/v1/admin/settings",
	"/api/v1/admin/quotas",
	"/api/v1/admin/stats",
	"/api/v1/admin/usage",
	"/api/v1/admin/tier-usage",
	"/api/v1/admin/indicators",
	"/api/v1/admin/agent-block-signals",
	"/api/v1/admin/content-guard",
	"/api/v1/admin/password",
	"/api/v1/audit",
	"/api/v1/entity-changes",
	"/api/v1/kpi/",
	"/api/v1/agent-tokens",
	"/api/v1/invitations",
	"/api/v1/onboarding/",
	"/api/v1/my-agents",
	"/api/v1/my-alerts",
	"/api/v1/my-indicators",
	"/api/v1/auth/verification-status",
	"/api/v1/compatibility",
	"/api/v1/activity",
}

// filterInternalOpenAPIPaths removes internal paths from an OpenAPI spec.
// It uses the yaml.v3 Node API to preserve formatting and comments.
func filterInternalOpenAPIPaths(data []byte) ([]byte, int, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, 0, fmt.Errorf("parse YAML: %w", err)
	}

	// doc is a Document node; its first child is the root mapping
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return data, 0, nil
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return data, 0, nil
	}

	// Find the "paths" key in the root mapping
	removed := 0
	for i := 0; i < len(root.Content)-1; i += 2 {
		if root.Content[i].Value == "paths" {
			pathsNode := root.Content[i+1]
			if pathsNode.Kind != yaml.MappingNode {
				break
			}
			// Walk path keys and remove internal ones
			filtered := make([]*yaml.Node, 0, len(pathsNode.Content))
			for j := 0; j < len(pathsNode.Content)-1; j += 2 {
				pathKey := pathsNode.Content[j].Value
				if isInternalAPIPath(pathKey) {
					removed++
					continue
				}
				filtered = append(filtered, pathsNode.Content[j], pathsNode.Content[j+1])
			}
			pathsNode.Content = filtered
			break
		}
	}

	out, err := yaml.Marshal(&doc)
	if err != nil {
		return nil, 0, fmt.Errorf("marshal YAML: %w", err)
	}
	return out, removed, nil
}

// isInternalAPIPath checks if a path matches any internal prefix.
func isInternalAPIPath(path string) bool {
	for _, prefix := range internalAPIPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

// cmdDocsHugo generates Hugo-compatible Markdown pages from exported tool JSON.
func cmdDocsHugo(args []string) error {
	var (
		inputPath  string
		outputDir  string
		showHelp   bool
	)

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--input":
			if i+1 < len(args) {
				inputPath = args[i+1]
				i++
			}
		case "--output":
			if i+1 < len(args) {
				outputDir = args[i+1]
				i++
			}
		case "--help", "-h":
			showHelp = true
		}
	}

	if showHelp {
		printDocsUsage()
		return nil
	}

	if inputPath == "" {
		inputPath = "./website/hugo/static/mcp-tools.json"
	}
	if outputDir == "" {
		outputDir = "./website/hugo/content/reference/mcp-tools"
	}

	// Read the exported JSON
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", inputPath, err)
	}

	// Parse into structure
	var export struct {
		Version string          `json:"version"`
		BaseURL string          `json:"base_url"`
		Tools   []docs.ToolDoc  `json:"tools"`
	}
	if err := json.Unmarshal(data, &export); err != nil {
		return fmt.Errorf("parse JSON: %w", err)
	}

	// Clean and recreate output directory to remove stale pages
	if err := os.RemoveAll(outputDir); err != nil {
		return fmt.Errorf("clean output directory: %w", err)
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	// Group tools by category for the index page
	categories := make(map[string][]docs.ToolDoc)
	var publicCount int
	for _, t := range export.Tools {
		vis := t.Visibility
		if vis == "" {
			vis = "public"
		}
		if vis != "public" {
			continue
		}
		publicCount++
		categories[t.Category] = append(categories[t.Category], t)
	}

	// Sort category names
	catNames := make([]string, 0, len(categories))
	for c := range categories {
		catNames = append(catNames, c)
	}
	sort.Strings(catNames)

	// Generate index page
	var idx strings.Builder
	idx.WriteString("---\n")
	idx.WriteString("title: MCP Tools\n")
	idx.WriteString("description: Complete reference for all Taskschmiede MCP tools\n")
	idx.WriteString("weight: 20\n")
	idx.WriteString("type: docs\n")
	idx.WriteString("no_list: true\n")
	idx.WriteString("---\n\n")
	fmt.Fprintf(&idx, "Taskschmiede exposes **%d public MCP tools** organized into %d categories.\n\n", publicCount, len(catNames))
	idx.WriteString("## Tool Categories\n\n")

	// Category name mapping for display
	catDisplayNames := map[string]string{
		"approval": "Approvals", "artifact": "Artifacts", "audit": "Audit",
		"auth": "Authentication", "comment": "Comments", "demand": "Demands",
		"dod": "Definition of Done", "docs": "Documentation", "endeavour": "Endeavours",
		"invitation": "Invitations", "message": "Messages", "onboarding": "Onboarding",
		"organization": "Organizations", "registration": "Registration", "relation": "Relations",
		"report": "Reports", "resource": "Resources", "ritual": "Rituals",
		"ritual_run": "Ritual Runs", "task": "Tasks", "template": "Templates",
		"token": "Tokens", "user": "Users",
	}

	for _, cat := range catNames {
		tools := categories[cat]
		displayName := catDisplayNames[cat]
		if displayName == "" {
			displayName = strings.ToUpper(cat[:1]) + cat[1:]
		}
		fmt.Fprintf(&idx, "### %s\n\n", displayName)
		fmt.Fprintf(&idx, "| Tool | Description |\n")
		fmt.Fprintf(&idx, "|------|-------------|\n")
		for _, t := range tools {
			fmt.Fprintf(&idx, "| [`%s`](%s/) | %s |\n", t.Name, t.Name, t.Summary)
		}
		idx.WriteString("\n")
	}

	if err := os.WriteFile(filepath.Join(outputDir, "_index.md"), []byte(idx.String()), 0o644); err != nil {
		return fmt.Errorf("write index: %w", err)
	}

	// Generate individual tool pages
	toolCount := 0
	for _, t := range export.Tools {
		vis := t.Visibility
		if vis == "" {
			vis = "public"
		}
		if vis != "public" {
			continue
		}

		var sb strings.Builder

		// Hugo frontmatter
		sb.WriteString("---\n")
		fmt.Fprintf(&sb, "title: \"%s\"\n", t.Name)
		fmt.Fprintf(&sb, "description: \"%s\"\n", strings.ReplaceAll(t.Summary, "\"", "'"))
		fmt.Fprintf(&sb, "category: \"%s\"\n", t.Category)
		fmt.Fprintf(&sb, "requires_auth: %v\n", t.RequiresAuth)
		fmt.Fprintf(&sb, "since: \"%s\"\n", t.Since)
		sb.WriteString("type: docs\n")
		sb.WriteString("---\n\n")

		// Summary
		fmt.Fprintf(&sb, "%s\n\n", t.Summary)

		// Auth badge
		if t.RequiresAuth {
			sb.WriteString("**Requires authentication**\n\n")
		}

		// Description
		if t.Description != "" {
			sb.WriteString("## Description\n\n")
			sb.WriteString(t.Description)
			sb.WriteString("\n\n")
		}

		// Parameters
		sb.WriteString("## Parameters\n\n")
		if len(t.Parameters) == 0 {
			sb.WriteString("No parameters.\n\n")
		} else {
			sb.WriteString("| Name | Type | Required | Default | Description |\n")
			sb.WriteString("|------|------|----------|---------|-------------|\n")
			for _, p := range t.Parameters {
				req := ""
				if p.Required {
					req = "Yes"
				}
				def := ""
				if p.Default != nil {
					def = fmt.Sprintf("`%v`", p.Default)
				}
				desc := strings.ReplaceAll(p.Description, "|", "\\|")
				fmt.Fprintf(&sb, "| `%s` | %s | %s | %s | %s |\n",
					p.Name, p.Type, req, def, desc)
			}
			sb.WriteString("\n")
		}

		// Response
		sb.WriteString("## Response\n\n")
		sb.WriteString(t.Returns.Description)
		sb.WriteString("\n\n")
		if t.Returns.Example != nil {
			sb.WriteString("```json\n")
			jsonBytes, _ := json.MarshalIndent(t.Returns.Example, "", "  ")
			sb.WriteString(string(jsonBytes))
			sb.WriteString("\n```\n\n")
		}

		// Errors
		if len(t.Errors) > 0 {
			sb.WriteString("## Errors\n\n")
			sb.WriteString("| Code | Description |\n")
			sb.WriteString("|------|-------------|\n")
			for _, e := range t.Errors {
				desc := strings.ReplaceAll(e.Description, "|", "\\|")
				fmt.Fprintf(&sb, "| `%s` | %s |\n", e.Code, desc)
			}
			sb.WriteString("\n")
		}

		// Examples
		if len(t.Examples) > 0 {
			sb.WriteString("## Examples\n\n")
			for _, ex := range t.Examples {
				fmt.Fprintf(&sb, "### %s\n\n", ex.Title)
				if ex.Description != "" {
					sb.WriteString(ex.Description + "\n\n")
				}
				sb.WriteString("**Request:**\n\n")
				sb.WriteString("```json\n")
				jsonBytes, _ := json.MarshalIndent(ex.Input, "", "  ")
				sb.WriteString(string(jsonBytes))
				sb.WriteString("\n```\n\n")
				sb.WriteString("**Response:**\n\n")
				sb.WriteString("```json\n")
				jsonBytes, _ = json.MarshalIndent(ex.Output, "", "  ")
				sb.WriteString(string(jsonBytes))
				sb.WriteString("\n```\n\n")
			}
		}

		// Related tools
		if len(t.RelatedTools) > 0 {
			sb.WriteString("## Related Tools\n\n")
			for _, rt := range t.RelatedTools {
				fmt.Fprintf(&sb, "- [`%s`](../%s/)\n", rt, rt)
			}
			sb.WriteString("\n")
		}

		// Write tool page
		toolDir := filepath.Join(outputDir, t.Name)
		if err := os.MkdirAll(toolDir, 0o755); err != nil {
			return fmt.Errorf("create tool dir %s: %w", t.Name, err)
		}
		if err := os.WriteFile(filepath.Join(toolDir, "index.md"), []byte(sb.String()), 0o644); err != nil {
			return fmt.Errorf("write tool %s: %w", t.Name, err)
		}
		toolCount++
	}

	fmt.Printf("Generated %d tool pages in %s\n", toolCount, outputDir)

	// Also generate REST API pages from OpenAPI spec
	// outputDir is like ./website/hugo/content/reference/mcp-tools
	// We need ./website/hugo/static/openapi.yaml
	hugoRoot := filepath.Dir(filepath.Dir(filepath.Dir(outputDir))) // ./website/hugo
	apiSpecPath := filepath.Join(hugoRoot, "static", "openapi.yaml")
	apiOutputDir := filepath.Join(filepath.Dir(outputDir), "rest-api")
	if err := generateRESTAPIPages(apiSpecPath, apiOutputDir); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: REST API page generation skipped: %v\n", err)
	}

	return nil
}

// openAPISpec holds a parsed OpenAPI 3.x specification.
type openAPISpec struct {
	Info  openAPIInfo            `yaml:"info"`
	Tags  []openAPITag           `yaml:"tags"`
	Paths map[string]interface{} `yaml:"paths"`
	Components struct {
		Schemas    map[string]interface{} `yaml:"schemas"`
		Responses  map[string]interface{} `yaml:"responses"`
		Parameters map[string]interface{} `yaml:"parameters"`
	} `yaml:"components"`
}

// openAPIInfo holds the info block of an OpenAPI spec.
type openAPIInfo struct {
	Title       string `yaml:"title"`
	Version     string `yaml:"version"`
	Description string `yaml:"description"`
}

// openAPITag holds a single tag definition from an OpenAPI spec.
type openAPITag struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

// generateRESTAPIPages creates Hugo Markdown pages for every REST API
// endpoint group parsed from the OpenAPI specification at specPath.
func generateRESTAPIPages(specPath, outputDir string) error {
	data, err := os.ReadFile(specPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", specPath, err)
	}

	var spec openAPISpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return fmt.Errorf("parse OpenAPI spec: %w", err)
	}

	// Clean and recreate output directory to remove stale pages
	if err := os.RemoveAll(outputDir); err != nil {
		return fmt.Errorf("clean REST API output directory: %w", err)
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	// Build tag descriptions map
	tagDesc := map[string]string{}
	for _, t := range spec.Tags {
		tagDesc[t.Name] = t.Description
	}

	// Group endpoints by tag
	type endpoint struct {
		Method  string
		Path    string
		Summary string
		Desc    string
		Tag     string
		Params  []map[string]interface{}
		ReqBody map[string]interface{}
		Resp    map[string]interface{}
		Security []interface{}
	}

	tagEndpoints := map[string][]endpoint{}
	var tagOrder []string
	tagSeen := map[string]bool{}

	// Sort paths for deterministic output
	pathKeys := make([]string, 0, len(spec.Paths))
	for p := range spec.Paths {
		pathKeys = append(pathKeys, p)
	}
	sort.Strings(pathKeys)

	for _, path := range pathKeys {
		ops, ok := spec.Paths[path].(map[string]interface{})
		if !ok {
			continue
		}
		// Collect path-level parameters
		var pathParams []map[string]interface{}
		if pp, ok := ops["parameters"].([]interface{}); ok {
			for _, p := range pp {
				if pm, ok := p.(map[string]interface{}); ok {
					pathParams = append(pathParams, pm)
				}
			}
		}

		for _, method := range []string{"get", "post", "put", "patch", "delete"} {
			opRaw, ok := ops[method]
			if !ok {
				continue
			}
			op, ok := opRaw.(map[string]interface{})
			if !ok {
				continue
			}

			tag := "Other"
			if tags, ok := op["tags"].([]interface{}); ok && len(tags) > 0 {
				if t, ok := tags[0].(string); ok {
					tag = t
				}
			}

			if !tagSeen[tag] {
				tagSeen[tag] = true
				tagOrder = append(tagOrder, tag)
			}

			summary, _ := op["summary"].(string)
			desc, _ := op["description"].(string)

			// Collect operation parameters
			var params []map[string]interface{}
			params = append(params, pathParams...)
			if opParams, ok := op["parameters"].([]interface{}); ok {
				for _, p := range opParams {
					if pm, ok := p.(map[string]interface{}); ok {
						params = append(params, pm)
					}
				}
			}

			var reqBody map[string]interface{}
			if rb, ok := op["requestBody"].(map[string]interface{}); ok {
				reqBody = rb
			}

			var resp map[string]interface{}
			if r, ok := op["responses"].(map[string]interface{}); ok {
				resp = r
			}

			var security []interface{}
			if s, ok := op["security"].([]interface{}); ok {
				security = s
			}

			tagEndpoints[tag] = append(tagEndpoints[tag], endpoint{
				Method:   strings.ToUpper(method),
				Path:     path,
				Summary:  summary,
				Desc:     desc,
				Tag:      tag,
				Params:   params,
				ReqBody:  reqBody,
				Resp:     resp,
				Security: security,
			})
		}
	}

	// Generate index page
	var idx strings.Builder
	idx.WriteString("---\n")
	idx.WriteString("title: REST API\n")
	idx.WriteString("description: Complete reference for the Taskschmiede REST API\n")
	idx.WriteString("weight: 10\n")
	idx.WriteString("type: docs\n")
	idx.WriteString("no_list: true\n")
	idx.WriteString("---\n\n")

	idx.WriteString("The Taskschmiede REST API provides HTTP endpoints for all operations.\n")
	idx.WriteString("All endpoints return JSON. Timestamps are UTC in RFC 3339 format.\n\n")
	idx.WriteString("[Download OpenAPI Specification (YAML)](/openapi.yaml)\n\n")
	idx.WriteString("## Authentication\n\n")
	idx.WriteString("Most endpoints require a bearer token in the `Authorization` header:\n\n")
	idx.WriteString("```\nAuthorization: Bearer <token>\n```\n\n")
	idx.WriteString("Obtain a token by calling [POST /api/v1/auth/login](/reference/rest-api/auth/#login).\n\n")
	idx.WriteString("## Endpoint Groups\n\n")
	idx.WriteString("| Group | Description | Endpoints |\n")
	idx.WriteString("|-------|-------------|:---------:|\n")

	for _, tag := range tagOrder {
		desc := tagDesc[tag]
		slug := strings.ToLower(strings.ReplaceAll(tag, " ", "-"))
		count := len(tagEndpoints[tag])
		fmt.Fprintf(&idx, "| [%s](%s/) | %s | %d |\n", tag, slug, desc, count)
	}

	if err := os.WriteFile(filepath.Join(outputDir, "_index.md"), []byte(idx.String()), 0o644); err != nil {
		return fmt.Errorf("write index: %w", err)
	}

	// Generate per-tag pages
	pageCount := 0
	for _, tag := range tagOrder {
		eps := tagEndpoints[tag]
		slug := strings.ToLower(strings.ReplaceAll(tag, " ", "-"))

		var sb strings.Builder
		sb.WriteString("---\n")
		fmt.Fprintf(&sb, "title: \"%s\"\n", tag)
		fmt.Fprintf(&sb, "description: \"%s\"\n", strings.ReplaceAll(tagDesc[tag], "\"", "'"))
		fmt.Fprintf(&sb, "weight: %d\n", pageCount+1)
		sb.WriteString("type: docs\n")
		sb.WriteString("---\n\n")

		if tagDesc[tag] != "" {
			sb.WriteString(tagDesc[tag] + "\n\n")
		}

		sb.WriteString("## Endpoints\n\n")
		sb.WriteString("| Method | Path | Summary |\n")
		sb.WriteString("|--------|------|---------- |\n")
		for _, ep := range eps {
			anchor := strings.ToLower(strings.ReplaceAll(ep.Summary, " ", "-"))
			// Clean anchor of non-alphanumeric chars
			anchorClean := strings.Map(func(r rune) rune {
				if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
					return r
				}
				return -1
			}, anchor)
			fmt.Fprintf(&sb, "| `%s` | `%s` | [%s](#%s) |\n", ep.Method, ep.Path, ep.Summary, anchorClean)
		}
		sb.WriteString("\n---\n\n")

		// Detail for each endpoint
		for _, ep := range eps {
			fmt.Fprintf(&sb, "## %s\n\n", ep.Summary)
			fmt.Fprintf(&sb, "`%s %s`\n\n", ep.Method, ep.Path)

			// Auth requirement
			isPublic := false
			if ep.Security != nil && len(ep.Security) == 0 {
				isPublic = true
			}
			if isPublic {
				sb.WriteString("**Public** -- no authentication required.\n\n")
			} else {
				sb.WriteString("**Requires authentication.**\n\n")
			}

			if ep.Desc != "" {
				sb.WriteString(ep.Desc + "\n\n")
			}

			// Parameters
			if len(ep.Params) > 0 {
				sb.WriteString("### Parameters\n\n")
				sb.WriteString("| Name | In | Type | Required | Description |\n")
				sb.WriteString("|------|-----|------|:--------:|-------------|\n")
				for _, p := range ep.Params {
					name, _ := p["name"].(string)
					in, _ := p["in"].(string)
					desc, _ := p["description"].(string)
					required, _ := p["required"].(bool)
					ptype := "string"
					if schema, ok := p["schema"].(map[string]interface{}); ok {
						if t, ok := schema["type"].(string); ok {
							ptype = t
						}
					}
					reqStr := ""
					if required {
						reqStr = "Yes"
					}
					fmt.Fprintf(&sb, "| `%s` | %s | %s | %s | %s |\n", name, in, ptype, reqStr, desc)
				}
				sb.WriteString("\n")
			}

			// Request body
			if ep.ReqBody != nil {
				sb.WriteString("### Request Body\n\n")
				if content, ok := ep.ReqBody["content"].(map[string]interface{}); ok {
					if jsonContent, ok := content["application/json"].(map[string]interface{}); ok {
						if schema, ok := jsonContent["schema"].(map[string]interface{}); ok {
							writeSchemaTable(&sb, schema, spec.Components.Schemas)
						}
					}
				}
			}

			// Responses
			if ep.Resp != nil {
				sb.WriteString("### Responses\n\n")
				// Sort response codes
				var codes []string
				for code := range ep.Resp {
					codes = append(codes, code)
				}
				sort.Strings(codes)
				sb.WriteString("| Code | Description |\n")
				sb.WriteString("|:----:|-------------|\n")
				for _, code := range codes {
					respRaw := ep.Resp[code]
					desc := ""
					if r, ok := respRaw.(map[string]interface{}); ok {
						desc, _ = r["description"].(string)
					}
					fmt.Fprintf(&sb, "| `%s` | %s |\n", code, desc)
				}
				sb.WriteString("\n")
			}

			sb.WriteString("---\n\n")
		}

		// Write tag page
		tagDir := filepath.Join(outputDir, slug)
		if err := os.MkdirAll(tagDir, 0o755); err != nil {
			return fmt.Errorf("create tag dir %s: %w", slug, err)
		}
		if err := os.WriteFile(filepath.Join(tagDir, "index.md"), []byte(sb.String()), 0o644); err != nil {
			return fmt.Errorf("write tag %s: %w", tag, err)
		}
		pageCount++
	}

	fmt.Printf("Generated %d REST API pages in %s\n", pageCount, outputDir)
	return nil
}

// writeSchemaTable renders an OpenAPI schema as a markdown parameters table.
// It resolves $ref pointers one level deep.
func writeSchemaTable(sb *strings.Builder, schema map[string]interface{}, schemas map[string]interface{}) {
	// Resolve $ref
	if ref, ok := schema["$ref"].(string); ok {
		parts := strings.Split(ref, "/")
		schemaName := parts[len(parts)-1]
		if resolved, ok := schemas[schemaName].(map[string]interface{}); ok {
			schema = resolved
		} else {
			fmt.Fprintf(sb, "See schema: `%s`\n\n", schemaName)
			return
		}
	}

	props, ok := schema["properties"].(map[string]interface{})
	if !ok {
		return
	}

	// Check required fields
	requiredSet := map[string]bool{}
	if req, ok := schema["required"].([]interface{}); ok {
		for _, r := range req {
			if s, ok := r.(string); ok {
				requiredSet[s] = true
			}
		}
	}

	sb.WriteString("| Field | Type | Required | Description |\n")
	sb.WriteString("|-------|------|:--------:|-------------|\n")

	// Sort property names
	propNames := make([]string, 0, len(props))
	for name := range props {
		propNames = append(propNames, name)
	}
	sort.Strings(propNames)

	for _, name := range propNames {
		prop, ok := props[name].(map[string]interface{})
		if !ok {
			continue
		}
		ptype, _ := prop["type"].(string)
		if ptype == "" {
			if _, hasRef := prop["$ref"]; hasRef {
				ptype = "object"
			} else {
				ptype = "any"
			}
		}
		if format, ok := prop["format"].(string); ok && format != "" {
			ptype = ptype + " <" + format + ">"
		}
		desc, _ := prop["description"].(string)
		desc = strings.ReplaceAll(desc, "|", "\\|")
		desc = strings.ReplaceAll(desc, "\n", " ")
		reqStr := ""
		if requiredSet[name] {
			reqStr = "Yes"
		}
		fmt.Fprintf(sb, "| `%s` | %s | %s | %s |\n", name, ptype, reqStr, desc)
	}
	sb.WriteString("\n")
}

// printServeUsage displays usage for the serve subcommand.
func printServeUsage() {
	fmt.Println("Usage: taskschmiede serve [options]")
	fmt.Println()
	fmt.Println("Start the Taskschmiede server (MCP + REST API).")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  --config-file <path>    Path to YAML configuration file")
	fmt.Println("  --db <path>             Path to SQLite database (default: ./taskschmiede.db)")
	fmt.Println("  --mcp-port <number>     MCP server port (default: 9000)")
	fmt.Println("  --log-file <path>       Path to log file (default: ./taskschmiede.log)")
	fmt.Println("  --log-level <level>     Log level: DEBUG, INFO, WARN, ERROR (default: INFO)")
	fmt.Println("  --ticker-interval <dur> Ticker base interval (default: 1s)")
	fmt.Println("  --help                  Show this help message")
	fmt.Println()
	fmt.Println("The server exposes:")
	fmt.Println("  - MCP:      http://localhost:<mcp-port>/mcp")
	fmt.Println("  - REST API: http://localhost:<mcp-port>/api/v1/")
	fmt.Println()
	fmt.Println("Example config.yaml:")
	fmt.Println("  database:")
	fmt.Println("    path: /var/lib/taskschmiede/data.db")
	fmt.Println("  server:")
	fmt.Println("    mcp-port: 9000")
	fmt.Println("  log:")
	fmt.Println("    file: /var/log/taskschmiede/taskschmiede.log")
	fmt.Println("    level: INFO")
	fmt.Println("  ticker:")
	fmt.Println("    interval: 1s")
}

// initDatabase opens the database, runs migrations, and seeds tier/policy data.
func initDatabase(cfg *Config, logger *slog.Logger) (*storage.DB, error) {
	db, err := storage.Open(cfg.Database.Path)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if err := db.Initialize(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("initialize database: %w", err)
	}
	logger.Info("Database initialized", "path", cfg.Database.Path)

	// Seed tier definitions from config (first run only; no-op if tiers exist).
	tierDefs := configToTierSeeds(cfg.Tiers)
	if err := db.SeedTierDefinitions(tierDefs); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("seed tier definitions: %w", err)
	}

	// Store the default tier in the policy table.
	defaultTier := cfg.Tiers.DefaultTier
	if defaultTier <= 0 {
		defaultTier = 1
	}
	if err := db.SetPolicy("tiers.default_tier", strconv.Itoa(defaultTier)); err != nil {
		logger.Error("Failed to apply tiers.default_tier from config", "error", err)
	}

	// Store registration settings in policy table.
	requireKYC := "true"
	if !cfg.Registration.RequireKYCEnabled() {
		requireKYC = "false"
	}
	if err := db.SetPolicy("registration.require_kyc", requireKYC); err != nil {
		logger.Error("Failed to apply registration.require_kyc from config", "error", err)
	}

	// Apply instance config to policy table (config.yaml overrides DB defaults on startup).
	if cfg.Instance.MaxActiveUsers > 0 {
		if err := db.SetPolicy("instance.max_active_users", strconv.Itoa(cfg.Instance.MaxActiveUsers)); err != nil {
			logger.Error("Failed to apply instance.max_active_users from config", "error", err)
		} else {
			logger.Info("Instance config applied", "max_active_users", cfg.Instance.MaxActiveUsers)
		}
	}

	return db, nil
}

// securityServices groups the security-related services created at startup.
type securityServices struct {
	audit              *security.AuditService
	entityChangeWriter *security.EntityChangeDBWriter
	rateLimiter        *security.RateLimiter
}

// initSecurity creates the audit, entity-change, and rate-limiter services.
func initSecurity(db *storage.DB, cfg *Config, logger *slog.Logger) securityServices {
	auditSvc := security.NewAuditService(db, logger, cfg.Security.Audit.BufferSize)
	ecWriter := security.NewEntityChangeDBWriter(db, logger, cfg.Security.Audit.BufferSize)
	rl := security.NewRateLimiter(cfg.Security.RateLimit, logger, auditSvc)

	logger.Info("Security services initialized",
		"audit_buffer", cfg.Security.Audit.BufferSize,
		"rate_limit_ip", cfg.Security.RateLimit.GlobalPerIP.Enabled,
		"rate_limit_auth", cfg.Security.RateLimit.AuthEndpoint.Enabled,
	)

	return securityServices{
		audit:              auditSvc,
		entityChangeWriter: ecWriter,
		rateLimiter:        rl,
	}
}

// initMessaging opens the message database and creates the message service.
func initMessaging(db *storage.DB, cfg *Config, logger *slog.Logger) (*storage.MessageDB, *service.MessageService, string, error) {
	msgDBPath := cfg.Messaging.DatabasePath
	if msgDBPath == "" {
		msgDBPath = strings.TrimSuffix(cfg.Database.Path, ".db") + "_messages.db"
	}
	msgDB, err := storage.OpenMessageDB(msgDBPath)
	if err != nil {
		return nil, nil, "", fmt.Errorf("open message database: %w", err)
	}
	logger.Info("Message database initialized", "path", msgDBPath)

	msgSvc := service.NewMessageService(msgDB, db, logger)
	return msgDB, msgSvc, msgDBPath, nil
}

// llmClients holds optional LLM clients created during initialization.
type llmClients struct {
	ritualResilient *llmclient.ResilientClient
}

// initLLMClients creates LLM clients for injection review, content guard, and
// ritual executor, registering the corresponding ticker handlers.
func initLLMClients(
	cfg *Config,
	db *storage.DB,
	t *ticker.Ticker,
	msgSvc *service.MessageService,
	notifyClient *notify.Client,
	logger *slog.Logger,
) llmClients {
	var result llmClients

	// Injection reviewer
	if cfg.InjectionReview.Enabled {
		llmClient, err := llmclient.NewClient(llmclient.Config{
			Provider:        cfg.InjectionReview.Provider,
			Model:           cfg.InjectionReview.Model,
			APIKey:          cfg.InjectionReview.APIKey,
			APIURL:          cfg.InjectionReview.APIURL,
			Timeout:         cfg.InjectionReview.Timeout,
			MaxTokens:       512,
			Temperature:     cfg.InjectionReview.Temperature,
			ReasoningEffort: cfg.InjectionReview.ReasoningEffort,
			ReasoningTokens: cfg.InjectionReview.ReasoningTokens,
		})
		if err != nil {
			logger.Warn("Failed to create injection review LLM client", "error", err)
		} else {
			t.Register(ticker.NewInjectionReviewerHandler(
				db, llmClient, logger,
				cfg.InjectionReview.MaxRetries,
				cfg.InjectionReview.TickerInterval,
			))
			logger.Info("Injection review enabled",
				"provider", cfg.InjectionReview.Provider,
				"model", cfg.InjectionReview.Model,
			)
		}
	}

	// Content guard (WS-4.5)
	if cfg.ContentGuard.Enabled {
		cgPrimary, err := llmclient.NewClient(llmclient.Config{
			Provider:        cfg.ContentGuard.Provider,
			Model:           cfg.ContentGuard.Model,
			APIKey:          cfg.ContentGuard.APIKey,
			APIURL:          cfg.ContentGuard.APIURL,
			Timeout:         cfg.ContentGuard.Timeout,
			MaxTokens:       256,
			Temperature:     cfg.ContentGuard.Temperature,
			ReasoningEffort: cfg.ContentGuard.ReasoningEffort,
			ReasoningTokens: cfg.ContentGuard.ReasoningTokens,
		})
		if err != nil {
			logger.Warn("Failed to create content guard LLM client", "error", err)
		} else {
			// Create optional fallback client for ResilientClient.
			var cgFallback llmclient.Client
			if cfg.ContentGuard.FallbackProvider != "" {
				fbTimeout := cfg.ContentGuard.FallbackTimeout
				if fbTimeout <= 0 {
					fbTimeout = 30 * time.Second
				}
				fb, fbErr := llmclient.NewClient(llmclient.Config{
					Provider:  cfg.ContentGuard.FallbackProvider,
					Model:     cfg.ContentGuard.FallbackModel,
					APIKey:    cfg.ContentGuard.FallbackAPIKey,
					APIURL:    cfg.ContentGuard.FallbackAPIURL,
					Timeout:   fbTimeout,
					MaxTokens: 256,
				})
				if fbErr != nil {
					logger.Warn("Failed to create content guard fallback LLM client", "error", fbErr)
				} else {
					cgFallback = fb
					logger.Info("Content guard fallback configured",
						"provider", cfg.ContentGuard.FallbackProvider,
						"model", cfg.ContentGuard.FallbackModel,
					)
				}
			}

			cgClient := llmclient.NewResilientClient(cgPrimary, cgFallback)
			t.Register(ticker.NewContentGuardHandler(
				db, cgClient, msgSvc, notifyClient, logger,
				cfg.ContentGuard.MaxRetries,
				cfg.ContentGuard.TickerInterval,
			))
			api.SetContentGuardThreshold(cfg.ContentGuard.ScoreThreshold)
			// Load system pattern overrides from DB and apply.
			if spo, err := db.GetSystemPatternOverrides(); err == nil && spo != nil {
				secAdded := make([]security.CustomPattern, len(spo.Added))
				for i, cp := range spo.Added {
					secAdded[i] = security.CustomPattern{
						Name:     cp.Name,
						Category: cp.Category,
						Pattern:  cp.Pattern,
						Weight:   cp.Weight,
					}
				}
				security.SetPatternOverrides(&security.PatternOverrides{
					Disabled:        spo.Disabled,
					WeightOverrides: spo.WeightOverrides,
					Added:           secAdded,
				})
				logger.Info("Content guard pattern overrides loaded",
					"disabled", len(spo.Disabled),
					"weight_overrides", len(spo.WeightOverrides),
					"custom_patterns", len(spo.Added),
				)
			}
			logger.Info("Content guard enabled",
				"provider", cfg.ContentGuard.Provider,
				"model", cfg.ContentGuard.Model,
				"threshold", cfg.ContentGuard.ScoreThreshold,
			)
		}
	}

	// Ritual executor (Taskschmied Phase A)
	if cfg.RitualExecutor.Enabled {
		maxTokens := cfg.RitualExecutor.MaxTokens
		if maxTokens <= 0 {
			maxTokens = 2048
		}
		rePrimary, err := llmclient.NewClient(llmclient.Config{
			Provider:        cfg.RitualExecutor.Provider,
			Model:           cfg.RitualExecutor.Model,
			APIKey:          cfg.RitualExecutor.APIKey,
			APIURL:          cfg.RitualExecutor.APIURL,
			Timeout:         cfg.RitualExecutor.Timeout,
			MaxTokens:       maxTokens,
			Temperature:     cfg.RitualExecutor.Temperature,
			ReasoningEffort: cfg.RitualExecutor.ReasoningEffort,
			ReasoningTokens: cfg.RitualExecutor.ReasoningTokens,
		})
		if err != nil {
			logger.Warn("Failed to create ritual executor LLM client", "error", err)
		} else {
			var reFallback llmclient.Client
			if cfg.RitualExecutor.FallbackProvider != "" {
				fbTimeout := cfg.RitualExecutor.FallbackTimeout
				if fbTimeout <= 0 {
					fbTimeout = 30 * time.Second
				}
				fb, fbErr := llmclient.NewClient(llmclient.Config{
					Provider:  cfg.RitualExecutor.FallbackProvider,
					Model:     cfg.RitualExecutor.FallbackModel,
					APIKey:    cfg.RitualExecutor.FallbackAPIKey,
					APIURL:    cfg.RitualExecutor.FallbackAPIURL,
					Timeout:   fbTimeout,
					MaxTokens: maxTokens,
				})
				if fbErr != nil {
					logger.Warn("Failed to create ritual executor fallback LLM client", "error", fbErr)
				} else {
					reFallback = fb
					logger.Info("Ritual executor fallback configured",
						"provider", cfg.RitualExecutor.FallbackProvider,
						"model", cfg.RitualExecutor.FallbackModel,
					)
				}
			}
			result.ritualResilient = llmclient.NewResilientClient(rePrimary, reFallback)
			interval := cfg.RitualExecutor.TickerInterval
			if interval <= 0 {
				interval = 30 * time.Second
			}
			t.Register(ticker.NewRitualExecutorHandler(db, result.ritualResilient, msgSvc, logger, interval))
			logger.Info("Ritual executor enabled",
				"provider", cfg.RitualExecutor.Provider,
				"model", cfg.RitualExecutor.Model,
				"interval", interval,
			)
		}
	}

	return result
}

// initTickerHandlers registers the standard (non-LLM) ticker handlers and
// returns the ticker. LLM-related handlers are registered separately in
// initLLMClients.
func initTickerHandlers(
	cfg *Config,
	db *storage.DB,
	msgDB *storage.MessageDB,
	msgDBPath string,
	msgSvc *service.MessageService,
	auditSvc *security.AuditService,
	emailSender mcp.EmailSender,
	logger *slog.Logger,
) *ticker.Ticker {
	t := ticker.New(logger, cfg.Ticker.Interval)
	t.Register(ticker.NewCleanupHandler(db))

	if cfg.Ticker.KPI.Enabled {
		kpiDir := cfg.Ticker.KPI.OutputDir
		if kpiDir == "" {
			kpiDir = filepath.Join(filepath.Dir(cfg.Database.Path), "kpi")
		}
		t.Register(ticker.NewKPIHandlerWithInterval(db, kpiDir, cfg.Ticker.KPI.Interval))
	}

	t.Register(ticker.NewAlertHandler(db, logger, auditSvc))
	t.Register(ticker.NewTaskgovernorHandler(db, msgSvc, logger))

	// Inactivity sweep (needs email sender)
	backupDir := filepath.Join(filepath.Dir(cfg.Database.Path), "backups")
	t.Register(ticker.NewInactivitySweepHandler(db, emailSender, logger, ticker.InactivitySweepConfig{
		MsgDB:     msgDB,
		BackupDir: backupDir,
	}))

	// Waitlist processor (promotes queued registrations when capacity is available)
	t.Register(ticker.NewWaitlistHandler(db, emailSender, logger))

	// Database backup handler (daily VACUUM INTO with rotation)
	dbBackupDir := filepath.Join(filepath.Dir(cfg.Database.Path), "db-backups")
	t.Register(ticker.NewBackupHandler(logger, ticker.BackupConfig{
		DB:        db,
		MsgDB:     msgDB,
		DBPath:    cfg.Database.Path,
		MsgDBPath: msgDBPath,
		BackupDir: dbBackupDir,
	}))

	// Data purge handler (daily cleanup of old audit_log and entity_change records)
	t.Register(ticker.NewPurgeHandler(db, logger))

	// Endeavour snapshot handler (daily KPI snapshots per endeavour + weekly rollup)
	t.Register(ticker.NewEndeavourSnapshotHandler(db, logger))

	return t
}

// initEmailSender creates the SMTP client and email service using the Support
// account. Returns nil if SMTP is not configured.
func initEmailSender(cfg *Config, logger *slog.Logger) mcp.EmailSender {
	if cfg.Email.SMTPHost == "" {
		return nil
	}
	smtpCfg := &email.Config{
		Name:       cfg.Email.Support.Name,
		Address:    cfg.Email.Support.Address,
		Username:   cfg.Email.Support.Username,
		Password:   cfg.Email.Support.Password,
		SMTPHost:   cfg.Email.SMTPHost,
		SMTPPort:   cfg.Email.SMTPPort,
		SMTPUseTLS: cfg.Email.SMTPUseTLS,
		SMTPUseSSL: cfg.Email.SMTPUseSSL,
	}
	smtpClient, err := email.NewSMTPClient(smtpCfg)
	if err != nil {
		logger.Warn("Failed to create SMTP client", "error", err)
		return nil
	}
	emailSvc, err := email.NewService(smtpClient)
	if err != nil {
		logger.Warn("Failed to create email service", "error", err)
		return nil
	}
	return emailSvc
}

// initIntercom starts the email bridge (intercom) if enabled.
// Returns nil if intercom is not configured or creation fails.
func initIntercom(
	cfg *Config,
	msgSvc *service.MessageService,
	msgDB *storage.MessageDB,
	db *storage.DB,
	auditSvc *security.AuditService,
	logger *slog.Logger,
) *intercom.Intercom {
	if !cfg.Messaging.Intercom.Enabled || cfg.Email.SMTPHost == "" {
		return nil
	}

	icSMTPCfg := &email.Config{
		Name:       cfg.Email.Intercom.Name,
		Address:    cfg.Email.Intercom.Address,
		Username:   cfg.Email.Intercom.Username,
		Password:   cfg.Email.Intercom.Password,
		SMTPHost:   cfg.Email.SMTPHost,
		SMTPPort:   cfg.Email.SMTPPort,
		SMTPUseTLS: cfg.Email.SMTPUseTLS,
		SMTPUseSSL: cfg.Email.SMTPUseSSL,
	}
	icSMTPClient, err := email.NewSMTPClient(icSMTPCfg)
	if err != nil {
		logger.Warn("Failed to create SMTP client for intercom", "error", err)
		return nil
	}

	icIMAPCfg := &email.Config{
		Username:   cfg.Email.Intercom.Username,
		Password:   cfg.Email.Intercom.Password,
		IMAPHost:   cfg.Email.IMAPHost,
		IMAPPort:   cfg.Email.IMAPPort,
		IMAPUseTLS: cfg.Email.IMAPUseTLS,
		IMAPUseSSL: cfg.Email.IMAPUseSSL,
	}
	ic, err := intercom.New(icSMTPClient, icIMAPCfg, msgSvc, msgDB, db, auditSvc, logger, intercom.Config{
		Enabled:           true,
		Address:           cfg.Email.Intercom.Address,
		DisplayName:       cfg.Email.Intercom.Name,
		ReplyTTL:          cfg.Messaging.Intercom.ReplyTTL,
		SweepInterval:     cfg.Messaging.Intercom.SweepInterval,
		SendInterval:      cfg.Messaging.Intercom.SendInterval,
		MaxRetries:        cfg.Messaging.Intercom.MaxRetries,
		MaxInboundPerHour: cfg.Messaging.Intercom.MaxInboundPerHour,
		DedupWindow:       cfg.Messaging.Intercom.DedupWindow,
	})
	if err != nil {
		logger.Warn("Failed to create intercom", "error", err)
		return nil
	}

	ic.Start()
	logger.Info("Intercom started",
		"address", cfg.Email.Intercom.Address,
		"sweep_interval", cfg.Messaging.Intercom.SweepInterval,
		"send_interval", cfg.Messaging.Intercom.SendInterval,
	)
	return ic
}

// initMCPServer creates and configures the MCP server (without starting it).
func initMCPServer(
	cfg *Config,
	db *storage.DB,
	msgDB *storage.MessageDB,
	msgSvc *service.MessageService,
	sec securityServices,
	emailSender mcp.EmailSender,
	ritualRC *llmclient.ResilientClient,
	logger *slog.Logger,
) (*mcp.Server, error) {
	mcpAddr := fmt.Sprintf(":%d", cfg.Server.MCPPort)
	mcpCfg := &mcp.Config{
		Address:                mcpAddr,
		SessionTimeout:         cfg.Server.SessionTimeout,
		AuditService:           sec.audit,
		EntityChangeWriter:     sec.entityChangeWriter,
		RateLimiter:            sec.rateLimiter,
		HeadersConfig:          cfg.Security.Headers,
		BodyLimitConfig:        cfg.Security.BodyLimit,
		CORSOrigins:            cfg.Security.CORSOrigins,
		MsgService:             msgSvc,
		MessageDB:              msgDB,
		InjectionReviewEnabled: cfg.InjectionReview.Enabled,
		AgentTokenTTL:          cfg.Server.AgentTokenTTL,
		PortalURL:              cfg.Email.PortalURL,
		Version:                Version,
		DeploymentMode:                cfg.Security.DeploymentMode,
		AllowSelfRegistration:         cfg.Security.AllowSelfRegistration,
		RequireAgentEmailVerification: cfg.Security.AgentOnboarding.RequireEmailVerification,
		RequireAgentInterview:         cfg.Security.AgentOnboarding.RequireInterview,
		EmailSender:                   emailSender,
	}

	mcpServer, err := mcp.NewServer(db, logger, mcpCfg)
	if err != nil {
		return nil, fmt.Errorf("create MCP server: %w", err)
	}

	// Wire Taskschmied status and toggle endpoints.
	if ritualRC != nil {
		rc := ritualRC
		primaryURL := cfg.RitualExecutor.APIURL
		fallbackURL := cfg.RitualExecutor.FallbackAPIURL
		mcpServer.SetTaskschmiedStatusFunc(func() map[string]interface{} {
			stats := rc.Stats()
			stats.PrimaryURL = primaryURL
			stats.FallbackURL = fallbackURL
			return map[string]interface{}{
				"circuit_breaker": stats,
			}
		})
		mcpServer.SetTaskschmiedToggleFunc(func(target string, disabled bool) {
			switch target {
			case "primary":
				rc.SetPrimaryDisabled(disabled)
			case "fallback":
				rc.SetFallbackDisabled(disabled)
			}
			action := "enabled"
			if disabled {
				action = "disabled"
			}
			logger.Info("Taskschmied LLM toggled", "target", target, "action", action)
		})
	}

	return mcpServer, nil
}

// cmdServe parses flags, initializes all subsystems, and runs the server.
func cmdServe(args []string) error {
	// Parse command-line flags
	var (
		configFile     string
		dbPath         string
		mcpPort        int
		logFile        string
		logLevel       string
		tickerInterval string
		showHelp       bool
	)

	// Simple argument parsing (avoiding flag package for custom help)
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--config-file":
			if i+1 < len(args) {
				configFile = args[i+1]
				i++
			}
		case "--db":
			if i+1 < len(args) {
				dbPath = args[i+1]
				i++
			}
		case "--mcp-port":
			if i+1 < len(args) {
				_, _ = fmt.Sscanf(args[i+1], "%d", &mcpPort)
				i++
			}
		case "--log-file":
			if i+1 < len(args) {
				logFile = args[i+1]
				i++
			}
		case "--log-level":
			if i+1 < len(args) {
				logLevel = args[i+1]
				i++
			}
		case "--ticker-interval":
			if i+1 < len(args) {
				tickerInterval = args[i+1]
				i++
			}
		case "--help", "-h":
			showHelp = true
		default:
			if strings.HasPrefix(arg, "-") {
				fmt.Fprintf(os.Stderr, "Unknown option: %s\n\n", arg)
				printServeUsage()
				os.Exit(1)
			}
		}
	}

	if showHelp {
		printServeUsage()
		return nil
	}

	// Load configuration
	cfg, err := LoadConfig(configFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Override config with command-line flags
	if dbPath != "" {
		cfg.Database.Path = dbPath
	}
	if mcpPort != 0 {
		cfg.Server.MCPPort = mcpPort
	}
	if logFile != "" {
		cfg.Log.File = logFile
	}
	if logLevel != "" {
		cfg.Log.Level = logLevel
	}
	if tickerInterval != "" {
		if d, err := time.ParseDuration(tickerInterval); err == nil {
			cfg.Ticker.Interval = d
		}
	}

	// Setup logging
	logger, logFileHandle, err := logging.SetupLogging(cfg.Log, "main")
	if err != nil {
		return fmt.Errorf("setup logging: %w", err)
	}
	if logFileHandle != nil {
		defer func() { _ = logFileHandle.Close() }()
		// Print banner to log file
		_, _ = fmt.Fprint(logFileHandle, banner)
		_, _ = fmt.Fprintln(logFileHandle, tagline)
		_, _ = fmt.Fprintf(logFileHandle, "Version: %s\n", versionString())
		_, _ = fmt.Fprintln(logFileHandle)
	}

	logger.Info("Starting Taskschmiede",
		"version", versionString(),
		"mcp_port", cfg.Server.MCPPort,
		"database", cfg.Database.Path,
		"log_level", cfg.Log.Level,
		"ticker_interval", cfg.Ticker.Interval,
		"kpi_enabled", cfg.Ticker.KPI.Enabled,
		"deployment_mode", cfg.Security.DeploymentMode,
		"allow_self_registration", cfg.Security.AllowSelfRegistration,
		"require_agent_email_verification", cfg.Security.AgentOnboarding.RequireEmailVerification,
		"require_agent_interview", cfg.Security.AgentOnboarding.RequireInterview,
	)

	// Initialize database (open, migrate, seed tiers and policies)
	db, err := initDatabase(cfg, logger)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	// Create security services
	sec := initSecurity(db, cfg, logger)
	defer sec.audit.Close()
	defer sec.entityChangeWriter.Close()
	defer sec.rateLimiter.Close()

	// Open message database and create message service
	msgDB, msgSvc, msgDBPath, err := initMessaging(db, cfg, logger)
	if err != nil {
		return err
	}
	defer func() { _ = msgDB.Close() }()

	// Create notification service client (J-4)
	notifyClient := notify.NewClient(cfg.Notify.URL, cfg.Notify.AuthToken, logger)
	if notifyClient.IsConfigured() {
		logger.Info("Notification service client configured", "url", cfg.Notify.URL)
	}

	// Set up email service using Support account
	emailSender := initEmailSender(cfg, logger)

	// Register standard ticker handlers (cleanup, KPI, alerts, backups, etc.)
	t := initTickerHandlers(cfg, db, msgDB, msgDBPath, msgSvc, sec.audit, emailSender, logger)

	// Register LLM-powered ticker handlers (injection review, content guard, ritual executor)
	llm := initLLMClients(cfg, db, t, msgSvc, notifyClient, logger)

	// Create context with cancellation for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start ticker (periodic task runner)
	tickerDone := make(chan struct{})
	go func() {
		t.Run(ctx)
		close(tickerDone)
	}()

	// Start intercom (email bridge) if enabled
	ic := initIntercom(cfg, msgSvc, msgDB, db, sec.audit, logger)

	// Create and start MCP server
	mcpServer, err := initMCPServer(cfg, db, msgDB, msgSvc, sec, emailSender, llm.ritualResilient, logger)
	if err != nil {
		return err
	}

	mcpServerDone := make(chan error, 1)
	go func() {
		logger.Info("MCP server starting",
			"address", fmt.Sprintf("http://localhost:%d", cfg.Server.MCPPort),
		)
		mcpServerDone <- mcpServer.Start()
	}()

	// Wait for shutdown signal or server error
	select {
	case sig := <-sigChan:
		logger.Info("Graceful shutdown requested", "signal", sig)
	case err := <-mcpServerDone:
		if err != nil {
			logger.Error("MCP server error", "error", err)
			cancel()
			return err
		}
	}

	// Graceful shutdown
	cancel()

	// Wait for ticker to finish
	<-tickerDone
	logger.Info("Ticker stopped")

	// Stop intercom if running
	if ic != nil {
		ic.Stop()
		logger.Info("Intercom stopped")
	}

	logger.Info("Graceful shutdown completed")
	return nil
}

// configToTierSeeds converts config tier definitions to storage seed structs.
// Falls back to the 3 standard SaaS tiers if no definitions are configured.
func configToTierSeeds(tc TierConfig) []storage.TierSeedDef {
	if len(tc.Definitions) > 0 {
		seeds := make([]storage.TierSeedDef, len(tc.Definitions))
		for i, d := range tc.Definitions {
			seeds[i] = storage.TierSeedDef{
				ID:                  d.ID,
				Name:                d.Name,
				MaxUsers:            d.MaxUsers,
				MaxOrgs:             d.MaxOrgs,
				MaxAgentsPerOrg:     d.MaxAgentsPerOrg,
				MaxEndeavoursPerOrg: d.MaxEndeavoursPerOrg,
				MaxActiveEndeavours: d.MaxActiveEndeavours,
				MaxTeamsPerOrg:      d.MaxTeamsPerOrg,
				MaxCreationsPerHour: d.MaxCreationsPerHour,
			}
		}
		return seeds
	}

	// Backward compatibility: no config = seed the 3 standard SaaS tiers.
	return []storage.TierSeedDef{
		{ID: 1, Name: "explorer", MaxUsers: -1, MaxOrgs: 1, MaxAgentsPerOrg: 5, MaxEndeavoursPerOrg: 3, MaxActiveEndeavours: 1, MaxTeamsPerOrg: 5, MaxCreationsPerHour: 60},
		{ID: 2, Name: "professional", MaxUsers: -1, MaxOrgs: 5, MaxAgentsPerOrg: 25, MaxEndeavoursPerOrg: -1, MaxActiveEndeavours: 10, MaxTeamsPerOrg: 25, MaxCreationsPerHour: 300},
		{ID: 3, Name: "enterprise", MaxUsers: -1, MaxOrgs: -1, MaxAgentsPerOrg: -1, MaxEndeavoursPerOrg: -1, MaxActiveEndeavours: -1, MaxTeamsPerOrg: -1, MaxCreationsPerHour: -1},
	}
}

