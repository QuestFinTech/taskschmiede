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


package notify

import (
	"sync"
	"time"
)

// RateLimitConfig holds per-channel rate limit settings.
type RateLimitConfig struct {
	EmailPerRecipient5Min int `yaml:"email-per-recipient-5min"` // default 1
	NtfyPerTopic1Min     int `yaml:"ntfy-per-topic-1min"`      // default 5
	WebhookPerURL1Min    int `yaml:"webhook-per-url-1min"`     // default 10
}

// DefaultRateLimitConfig returns rate limits matching the design doc defaults.
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		EmailPerRecipient5Min: 1,
		NtfyPerTopic1Min:     5,
		WebhookPerURL1Min:    10,
	}
}

// RateLimiter enforces per-key rate limits using a sliding window.
// Each key (e.g. "email:alice@example.com") tracks timestamps of
// recent attempts and rejects new ones that exceed the configured limit.
type RateLimiter struct {
	mu      sync.Mutex
	windows map[string][]time.Time
	config  RateLimitConfig
}

// NewRateLimiter creates a rate limiter with the given configuration.
func NewRateLimiter(cfg RateLimitConfig) *RateLimiter {
	if cfg.EmailPerRecipient5Min <= 0 {
		cfg.EmailPerRecipient5Min = 1
	}
	if cfg.NtfyPerTopic1Min <= 0 {
		cfg.NtfyPerTopic1Min = 5
	}
	if cfg.WebhookPerURL1Min <= 0 {
		cfg.WebhookPerURL1Min = 10
	}
	return &RateLimiter{
		windows: make(map[string][]time.Time),
		config:  cfg,
	}
}

// AllowEmail checks whether an email to this recipient is allowed.
func (rl *RateLimiter) AllowEmail(recipient string) bool {
	return rl.allow("email:"+recipient, 5*time.Minute, rl.config.EmailPerRecipient5Min)
}

// AllowNtfy checks whether an ntfy message to this topic is allowed.
func (rl *RateLimiter) AllowNtfy() bool {
	return rl.allow("ntfy", 1*time.Minute, rl.config.NtfyPerTopic1Min)
}

// AllowWebhook checks whether a webhook call to this URL is allowed.
func (rl *RateLimiter) AllowWebhook(url string) bool {
	return rl.allow("webhook:"+url, 1*time.Minute, rl.config.WebhookPerURL1Min)
}

// allow is the core sliding window check. Returns true if the action is
// within limits, and records the timestamp if allowed.
func (rl *RateLimiter) allow(key string, window time.Duration, maxCount int) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now().UTC()
	cutoff := now.Add(-window)

	// Prune expired entries.
	timestamps := rl.windows[key]
	pruned := timestamps[:0]
	for _, ts := range timestamps {
		if ts.After(cutoff) {
			pruned = append(pruned, ts)
		}
	}

	if len(pruned) >= maxCount {
		rl.windows[key] = pruned
		return false
	}

	rl.windows[key] = append(pruned, now)
	return true
}

// Cleanup removes expired entries from all keys. Call periodically
// to prevent memory growth.
func (rl *RateLimiter) Cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	cutoff := time.Now().UTC().Add(-10 * time.Minute)
	for key, timestamps := range rl.windows {
		pruned := timestamps[:0]
		for _, ts := range timestamps {
			if ts.After(cutoff) {
				pruned = append(pruned, ts)
			}
		}
		if len(pruned) == 0 {
			delete(rl.windows, key)
		} else {
			rl.windows[key] = pruned
		}
	}
}
