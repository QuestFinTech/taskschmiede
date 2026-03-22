// Copyright 2026 Quest Financial Technologies S.a r.l.-S., Luxembourg
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


// Package main provides the member portal binary for Taskschmiede.
// This binary serves the public-facing member portal (my.taskschmiede.dev)
// and communicates with the Taskschmiede REST API for all data operations.
package main

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/QuestFinTech/taskschmiede/internal/portal"
)

// Version, Commit, and BuildTime are set via ldflags at build time.
var (
	Version   = "0.1.0"
	Commit    = "unknown"
	BuildTime = "unknown"
)

// envVarRegex matches ${VAR} patterns for environment variable expansion.
var envVarRegex = regexp.MustCompile(`\$\{([^}]+)\}`)

func main() {
	cfg := parseArgs()
	if cfg == nil {
		return
	}
	cfg.Version = Version

	server, err := portal.NewServer(cfg)
	if err != nil {
		log.Fatalf("Failed to create portal server: %v", err)
	}

	log.Printf("Taskschmiede Portal %s (commit %s)", Version, Commit)
	log.Printf("Listening on %s", cfg.ListenAddr)
	log.Printf("REST API:    %s", cfg.APIURL)

	if err := server.Start(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

// portalFileConfig maps the portal section of config.yaml.
type portalFileConfig struct {
	Portal struct {
		Listen             string `yaml:"listen"`
		APIURL             string `yaml:"api-url"`
		Secure             bool   `yaml:"secure"`
		ShowAbout          *bool  `yaml:"show-about"`
		SupportAgentURL    string `yaml:"support-agent-url"`
		SupportAgentAPIKey string `yaml:"support-agent-api-key"`
	} `yaml:"portal"`
}

// parseArgs parses CLI arguments and returns a portal server configuration,
// or nil if --help or --version was requested.
func parseArgs() *portal.ServerConfig {
	cfg := &portal.ServerConfig{
		ListenAddr: ":9090",
		APIURL:     "http://localhost:9000",
		ShowAbout:  true,
	}

	var configFile string

	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--config", "-c":
			if i+1 < len(args) {
				configFile = args[i+1]
				i++
			}
		case "--listen", "-l":
			if i+1 < len(args) {
				cfg.ListenAddr = args[i+1]
				i++
			}
		case "--api-url", "-a":
			if i+1 < len(args) {
				cfg.APIURL = args[i+1]
				i++
			}
		case "--secure":
			cfg.Secure = true
		case "--hide-about":
			cfg.ShowAbout = false
		case "--help", "-h":
			printUsage()
			return nil
		case "--version", "-v":
			fmt.Printf("taskschmiede-portal %s (commit %s, built %s)\n", Version, Commit, BuildTime)
			return nil
		default:
			if strings.HasPrefix(arg, "-") {
				fmt.Fprintf(os.Stderr, "Unknown option: %s\n\n", arg)
				printUsage()
				os.Exit(1)
			}
		}
	}

	// Load config file (if specified). File values are defaults;
	// explicit CLI flags above take precedence.
	if configFile != "" {
		fileCfg, err := loadConfigFile(configFile)
		if err != nil {
			log.Fatalf("Failed to load config: %v", err)
		}
		applyFileConfig(cfg, fileCfg, args)
	}

	return cfg
}

// loadConfigFile reads and parses the portal section from a config YAML file.
func loadConfigFile(path string) (*portalFileConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}
	expanded := expandEnvVars(string(data))

	var fileCfg portalFileConfig
	if err := yaml.Unmarshal([]byte(expanded), &fileCfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}
	return &fileCfg, nil
}

// applyFileConfig applies config file values where no explicit CLI flag was given.
func applyFileConfig(cfg *portal.ServerConfig, fileCfg *portalFileConfig, args []string) {
	p := fileCfg.Portal

	// Only apply file values if the corresponding CLI flag was not set.
	if p.Listen != "" && !hasFlag(args, "--listen", "-l") {
		cfg.ListenAddr = p.Listen
	}
	if p.APIURL != "" && !hasFlag(args, "--api-url", "-a") {
		cfg.APIURL = p.APIURL
	}
	if p.Secure && !hasFlag(args, "--secure") {
		cfg.Secure = true
	}
	if p.ShowAbout != nil && !hasFlag(args, "--hide-about") {
		cfg.ShowAbout = *p.ShowAbout
	}
	if p.SupportAgentURL != "" {
		cfg.SupportAgentURL = p.SupportAgentURL
	}
	if p.SupportAgentAPIKey != "" {
		cfg.SupportAgentAPIKey = p.SupportAgentAPIKey
	}
}

// hasFlag checks whether any of the given flag names appear in args.
func hasFlag(args []string, names ...string) bool {
	for _, a := range args {
		for _, n := range names {
			if a == n {
				return true
			}
		}
	}
	return false
}

// expandEnvVars replaces ${VAR} patterns with environment variable values.
func expandEnvVars(s string) string {
	return envVarRegex.ReplaceAllStringFunc(s, func(match string) string {
		varName := match[2 : len(match)-1]
		if value := os.Getenv(varName); value != "" {
			return value
		}
		return match
	})
}

// printUsage displays the portal binary usage message with all available options.
func printUsage() {
	fmt.Println("Taskschmiede Portal - Member portal (my.taskschmiede.dev)")
	fmt.Println()
	fmt.Println("Usage: taskschmiede-portal [options]")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  --config, -c <file>    Config file path (reads portal section)")
	fmt.Println("  --listen, -l <addr>    Listen address (default: :9090)")
	fmt.Println("  --api-url, -a <url>    REST API base URL (default: http://localhost:9000)")
	fmt.Println("  --secure               Enable Secure flag on cookies (use behind HTTPS)")
	fmt.Println("  --hide-about           Hide the About page and nav link")
	fmt.Println("  --version, -v          Show version")
	fmt.Println("  --help, -h             Show this help")
	fmt.Println()
	fmt.Println("CLI flags take precedence over config file values.")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  taskschmiede-portal")
	fmt.Println("  taskschmiede-portal --config config.yaml")
	fmt.Println("  taskschmiede-portal --config config.yaml --hide-about")
	fmt.Println("  taskschmiede-portal --listen :8090 --api-url http://api.example.com:9000")
	fmt.Println("  taskschmiede-portal --secure --api-url https://api.taskschmiede.dev")
}
