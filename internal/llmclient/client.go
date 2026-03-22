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


// Package llmclient provides a minimal LLM client abstraction for single-shot
// completions. It supports Anthropic Messages API and OpenAI-compatible Chat
// Completions API via direct HTTP calls (no SDK dependency).
package llmclient

import (
	"context"
	"fmt"
	"time"
)

// Request represents a single-shot completion request.
type Request struct {
	SystemPrompt string
	UserPrompt   string
	MaxTokens    int
}

// Response represents a completion response.
type Response struct {
	Content      string
	TotalTokens  int     // from usage.total_tokens (0 if not reported)
	PredictedMs  float64 // from timings.predicted_ms (0 if not reported)
	UsedProvider string  // provider that served this response (set by concrete clients)
	UsedModel    string  // model that served this response (set by concrete clients)
}

// Client is the interface for LLM providers.
type Client interface {
	Complete(ctx context.Context, req *Request) (*Response, error)
	Provider() string
	Model() string
}

// Config holds the configuration for creating an LLM client.
type Config struct {
	Provider        string        // "anthropic" or "openai"
	Model           string        // e.g., "claude-sonnet-4-5-20250929" or "gpt-4"
	APIKey          string        // API key
	APIURL          string        // Base URL (optional, for custom endpoints)
	Timeout         time.Duration // HTTP timeout
	MaxTokens       int           // Default max tokens if not specified in request
	Temperature     *float64      // Sampling temperature (nil = server default)
	ReasoningEffort string        // Reasoning effort: "low", "medium", "high" (empty = server default)
	ReasoningTokens int           // Max reasoning/thinking tokens budget (0 = server default)
}

// NewClient creates a new LLM client based on the provider configuration.
func NewClient(cfg Config) (Client, error) {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 60 * time.Second
	}
	if cfg.MaxTokens <= 0 {
		cfg.MaxTokens = 1024
	}

	switch cfg.Provider {
	case "anthropic":
		return newAnthropicClient(cfg), nil
	case "openai":
		return newOpenAIClient(cfg), nil
	default:
		return nil, fmt.Errorf("unsupported LLM provider: %q (supported: anthropic, openai)", cfg.Provider)
	}
}
