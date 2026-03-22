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


// Package main provides the standalone MCP proxy binary.
//
// The proxy sits between MCP clients and the Taskschmiede server, providing
// automatic reconnection, traffic logging, and a stable endpoint so clients
// are not disrupted when the server restarts.
//
// This is a separate binary to avoid Windows file-locking issues where the
// server binary cannot be rebuilt while the proxy holds a lock on it.
package main

import (
	"context"
	_ "embed"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/QuestFinTech/taskschmiede/internal/envutil"
	"github.com/QuestFinTech/taskschmiede/internal/logging"
	"github.com/QuestFinTech/taskschmiede/internal/mcp"
	"github.com/QuestFinTech/taskschmiede/internal/notify"
	"github.com/QuestFinTech/taskschmiede/internal/security"
)

// banner holds the ASCII art banner displayed on startup.
//
//go:embed banner.txt
var banner string

// tagline is the program's one-line description shown in help output.
const tagline = "MCP Development Proxy"

// Version, Commit, and BuildTime are set via ldflags at build time.
var (
	Version   = "0.1.0"
	Commit    = "unknown"
	BuildTime = "unknown"
)

// Config holds the subset of configuration used by the proxy.
type Config struct {
	Proxy       ProxyConfig       `yaml:"proxy"`
	Log         logging.LogConfig `yaml:"log"`
	Security    SecurityConfig    `yaml:"security"`
	Maintenance MaintenanceConfig `yaml:"maintenance"`
	MCPSecurity MCPSecurityYAML   `yaml:"mcp-security"`
}

// MCPSecurityYAML holds MCP-level security settings (validation, rate limits, versioning).
type MCPSecurityYAML struct {
	Enabled          bool                           `yaml:"enabled"`
	Validation       bool                           `yaml:"validation"`
	ToolRateLimits   map[string]mcp.ToolRateLimit   `yaml:"tool-rate-limits"`
	RESTDeprecations map[string]string              `yaml:"rest-deprecations"`
	APIVersions      APIVersionsYAML                `yaml:"api-versions"`
}

// APIVersionsYAML holds REST API version configuration.
type APIVersionsYAML struct {
	Current    string   `yaml:"current"`
	Supported  []string `yaml:"supported"`
	Deprecated []string `yaml:"deprecated"`
}

// SecurityConfig holds the security settings relevant to the proxy.
type SecurityConfig struct {
	RateLimit   security.RateLimitConfig `yaml:"rate-limit"`
	ConnLimit   security.ConnLimitConfig `yaml:"conn-limit"`
	Headers     security.HeadersConfig   `yaml:"headers"`
	BodyLimit   security.BodyLimitConfig `yaml:"body-limit"`
	CORSOrigins []string                 `yaml:"cors-origins"`
}

// ProxyConfig holds MCP proxy settings.
type ProxyConfig struct {
	Listen         string `yaml:"listen"`
	Upstream       string `yaml:"upstream"`
	LogTraffic     bool   `yaml:"log-traffic"`
	TrafficLogFile string `yaml:"traffic-log-file"`
}

// MaintenanceConfig holds maintenance mode settings (production use).
// When Enabled is true, the proxy runs the upstream monitor with health
// checking, auto-detect, and a management API on a separate port.
type MaintenanceConfig struct {
	Enabled             bool               `yaml:"enabled"`
	ManagementListen    string             `yaml:"management-listen"`
	ManagementAPIKey    string             `yaml:"management-api-key"`
	AutoDetect          bool               `yaml:"auto-detect"`
	AutoDetectGrace     time.Duration      `yaml:"auto-detect-grace"`
	HealthCheckInterval time.Duration      `yaml:"health-check-interval"`
	UpstreamTimeout     time.Duration      `yaml:"upstream-timeout"`
	UpstreamTimeoutSSE  time.Duration      `yaml:"upstream-timeout-sse"`
	Notifications       NotificationConfig `yaml:"notifications"`
}

// NotificationConfig holds settings for state change notifications.
type NotificationConfig struct {
	Webhook notify.WebhookConfig `yaml:"webhook"`
	SMTP    notify.SMTPConfig    `yaml:"smtp"`
}

// DefaultConfig returns the default proxy configuration.
func DefaultConfig() *Config {
	return &Config{
		Proxy: ProxyConfig{
			Listen:     ":9001",
			Upstream:   "http://localhost:9000",
			LogTraffic: true,
		},
		Log: logging.LogConfig{
			File:  "-",
			Level: "DEBUG",
		},
		Security: SecurityConfig{
			RateLimit: security.DefaultRateLimitConfig(),
			ConnLimit: security.DefaultConnLimitConfig(),
			Headers:   security.DefaultHeadersConfig(),
			BodyLimit: security.DefaultBodyLimitConfig(),
		},
		Maintenance: MaintenanceConfig{
			Enabled:             false, // off by default (dev mode)
			ManagementListen:    "127.0.0.1:9010",
			AutoDetect:          true,
			AutoDetectGrace:     10 * time.Second,
			HealthCheckInterval: 5 * time.Second,
			UpstreamTimeout:     30 * time.Second,
			UpstreamTimeoutSSE:  300 * time.Second,
		},
		MCPSecurity: MCPSecurityYAML{
			Enabled:    false, // off by default (dev mode)
			Validation: true,
			APIVersions: APIVersionsYAML{
				Current:   "v1",
				Supported: []string{"v1"},
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

	expanded := envutil.ExpandEnvVars(string(data))

	if err := yaml.Unmarshal([]byte(expanded), cfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}

	return cfg, nil
}

// versionString returns the version with 'v' prefix.
func versionString() string {
	v := Version
	if len(v) > 0 && v[0] != 'v' {
		v = "v" + v
	}
	return v
}

// printUsage displays the proxy binary usage message with all available options.
func printUsage() {
	fmt.Print(banner)
	fmt.Println(tagline)
	fmt.Printf("Version: %s\n", versionString())
	fmt.Println()
	fmt.Println("Start a proxy for development that provides:")
	fmt.Println("  - MCP protocol proxying with auto-reconnect on upstream restart")
	fmt.Println("  - REST API reverse proxying (/api/* forwarded to upstream)")
	fmt.Println("  - Connection limits (global + per-IP)")
	fmt.Println("  - Traffic logging for both MCP and REST")
	fmt.Println("  - Stable single-port entry point for all clients")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  taskschmiede-proxy [options]")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  --config-file <path>  Path to YAML configuration file")
	fmt.Println("  --listen <addr>       Proxy listen address (default: :9001)")
	fmt.Println("  --upstream <url>      Upstream MCP server URL (default: http://localhost:9000)")
	fmt.Println("  --log-traffic         Enable detailed traffic logging (default: true)")
	fmt.Println("  --no-log-traffic      Disable traffic logging")
	fmt.Println("  --log-level <level>   Log level: DEBUG, INFO, WARN, ERROR (default: DEBUG)")
	fmt.Println("  --version             Show version information")
	fmt.Println("  --help                Show this help message")
	fmt.Println()
	fmt.Println("Example:")
	fmt.Println("  taskschmiede-proxy --config-file config.yaml")
	fmt.Println()
	fmt.Println("Clients connect to the proxy instead of Taskschmiede directly:")
	fmt.Println("  - MCP (SSE):  http://localhost:9001/mcp/sse")
	fmt.Println("  - MCP (HTTP): http://localhost:9001/mcp")
	fmt.Println("  - REST API:   http://localhost:9001/api/v1/...")
	fmt.Println("  - Health:     http://localhost:9001/proxy/health")
	fmt.Println()
	fmt.Println("When upstream restarts, the proxy maintains client connections and")
	fmt.Println("automatically reconnects to upstream when it becomes available.")
}

// printVersion displays version, commit, and build time.
func printVersion() {
	fmt.Printf("Taskschmiede MCP Proxy %s\n", versionString())
	fmt.Printf("  Commit:  %s\n", Commit)
	fmt.Printf("  Built:   %s\n", BuildTime)
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Fatalf("Error: %v", err)
	}
}

// run parses flags, loads config, and starts the proxy server.
func run(args []string) error {
	var (
		configFile  string
		listenAddr  string
		upstreamURL string
		logTraffic  *bool
		logLevel    string
		showHelp    bool
		showVersion bool
	)

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--config-file":
			if i+1 < len(args) {
				configFile = args[i+1]
				i++
			}
		case "--listen":
			if i+1 < len(args) {
				listenAddr = args[i+1]
				i++
			}
		case "--upstream":
			if i+1 < len(args) {
				upstreamURL = args[i+1]
				i++
			}
		case "--log-traffic":
			t := true
			logTraffic = &t
		case "--no-log-traffic":
			f := false
			logTraffic = &f
		case "--log-level":
			if i+1 < len(args) {
				logLevel = args[i+1]
				i++
			}
		case "--help", "-h":
			showHelp = true
		case "--version", "-v":
			showVersion = true
		default:
			if strings.HasPrefix(arg, "-") {
				fmt.Fprintf(os.Stderr, "Unknown option: %s\n\n", arg)
				printUsage()
				os.Exit(1)
			}
		}
	}

	if showHelp {
		printUsage()
		return nil
	}

	if showVersion {
		printVersion()
		return nil
	}

	// Load configuration
	cfg, err := LoadConfig(configFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Override config with command-line flags
	if listenAddr != "" {
		cfg.Proxy.Listen = listenAddr
	}
	if upstreamURL != "" {
		cfg.Proxy.Upstream = upstreamURL
	}
	if logTraffic != nil {
		cfg.Proxy.LogTraffic = *logTraffic
	}
	if logLevel != "" {
		cfg.Log.Level = logLevel
	}

	// Setup logging
	logger, _, err := logging.SetupLogging(cfg.Log, "proxy")
	if err != nil {
		return fmt.Errorf("setup logging: %w", err)
	}

	// Print banner
	fmt.Print(banner)
	fmt.Println(tagline)
	fmt.Printf("Version: %s\n", versionString())
	fmt.Println()

	// Handle shutdown signals
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Create security services
	proxyRL := security.NewRateLimiter(cfg.Security.RateLimit, logger, nil)
	defer proxyRL.Close()
	proxyCL := security.NewConnLimiter(cfg.Security.ConnLimit, logger)

	// Build proxy config
	proxyCfg := &mcp.ProxyConfig{
		UpstreamURL:     cfg.Proxy.Upstream,
		ListenAddr:      cfg.Proxy.Listen,
		LogTraffic:      cfg.Proxy.LogTraffic,
		TrafficLogFile:  cfg.Proxy.TrafficLogFile,
		RateLimiter:     proxyRL,
		ConnLimiter:     proxyCL,
		HeadersConfig:   cfg.Security.Headers,
		BodyLimitConfig: cfg.Security.BodyLimit,
		CORSOrigins:     cfg.Security.CORSOrigins,
	}

	// Enable maintenance mode (production use)
	if cfg.Maintenance.Enabled {
		proxyCfg.MaintenanceConfig = &mcp.MaintenanceConfig{
			ManagementListen:    cfg.Maintenance.ManagementListen,
			ManagementAPIKey:    cfg.Maintenance.ManagementAPIKey,
			AutoDetect:          cfg.Maintenance.AutoDetect,
			AutoDetectGrace:     cfg.Maintenance.AutoDetectGrace,
			HealthCheckInterval: cfg.Maintenance.HealthCheckInterval,
			UpstreamTimeout:     cfg.Maintenance.UpstreamTimeout,
			UpstreamTimeoutSSE:  cfg.Maintenance.UpstreamTimeoutSSE,
		}
		logger.Info("Maintenance mode enabled",
			"management_listen", cfg.Maintenance.ManagementListen,
			"auto_detect", cfg.Maintenance.AutoDetect)

		// Create state change notifier (webhook + SMTP)
		notifyCfg := notify.Config{}
		if cfg.Maintenance.Notifications.Webhook.URL != "" {
			notifyCfg.Webhook = &cfg.Maintenance.Notifications.Webhook
		}
		if cfg.Maintenance.Notifications.SMTP.Host != "" {
			notifyCfg.SMTP = &cfg.Maintenance.Notifications.SMTP
		}
		notifier := notify.New(logger, notifyCfg)
		if notifier.HasChannels() {
			proxyCfg.Notifier = notifier
		}
	}

	// Enable MCP-level security (validation, per-tool rate limiting, versioning)
	if cfg.MCPSecurity.Enabled {
		secCfg := &mcp.MCPSecurityConfig{
			ValidationEnabled: cfg.MCPSecurity.Validation,
			ToolRateLimits:    cfg.MCPSecurity.ToolRateLimits,
			RESTDeprecations:  cfg.MCPSecurity.RESTDeprecations,
		}
		// Build version manifest from config
		if cfg.MCPSecurity.APIVersions.Current != "" {
			secCfg.Versions = &mcp.VersionManifest{
				REST: mcp.RESTVersionInfo{
					Supported:  cfg.MCPSecurity.APIVersions.Supported,
					Current:    cfg.MCPSecurity.APIVersions.Current,
					Deprecated: cfg.MCPSecurity.APIVersions.Deprecated,
				},
				MCP: mcp.MCPVersionInfo{
					Protocol: "2025-06-18",
					Tools: mcp.ToolVersions{
						Deprecated: []string{},
						Aliases:    map[string]string{},
					},
				},
			}
		}
		proxyCfg.MCPSecurityConfig = secCfg
		logger.Info("MCP security enabled",
			"validation", secCfg.ValidationEnabled,
			"tool_rate_limits", len(secCfg.ToolRateLimits))
	}

	// Create and start proxy
	proxy := mcp.NewProxy(logger, proxyCfg)

	proxyDone := make(chan error, 1)
	go func() {
		proxyDone <- proxy.Start(ctx)
	}()

	// Wait for shutdown signal or error
	select {
	case sig := <-sigChan:
		logger.Info("Shutdown requested", "signal", sig)
		cancel()
	case err := <-proxyDone:
		cancel()
		if err != nil {
			// Log to both the file logger and stderr so the error is visible
			// in journalctl even when logging is directed to a file.
			logger.Error("Proxy failed", "error", err)
			fmt.Fprintf(os.Stderr, "Error: proxy error: %v\n", err)
			return fmt.Errorf("proxy error: %w", err)
		}
	case <-ctx.Done():
	}

	return nil
}

