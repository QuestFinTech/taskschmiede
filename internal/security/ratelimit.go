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


package security

import (
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// RateLimitConfig holds rate limiting configuration.
type RateLimitConfig struct {
	GlobalPerIP     RateConfig    `yaml:"global-per-ip"`
	PerSession      RateConfig    `yaml:"per-session"`
	AuthEndpoint    RateConfig    `yaml:"auth-endpoint"`
	CleanupInterval time.Duration `yaml:"cleanup-interval"`
}

// RateConfig defines a single rate limit tier.
type RateConfig struct {
	Requests int           `yaml:"requests"`
	Window   time.Duration `yaml:"window"`
	Enabled  bool          `yaml:"enabled"`
}

// DefaultRateLimitConfig returns defaults matching the security spec.
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		GlobalPerIP:     RateConfig{Requests: 120, Window: time.Minute, Enabled: true},
		PerSession:      RateConfig{Requests: 60, Window: time.Minute, Enabled: true},
		AuthEndpoint:    RateConfig{Requests: 5, Window: time.Minute, Enabled: true},
		CleanupInterval: 5 * time.Minute,
	}
}

// bucket implements a token bucket rate limiter.
type bucket struct {
	mu         sync.Mutex
	tokens     float64
	maxTokens  float64
	refillRate float64 // tokens per second
	lastRefill time.Time
	lastAccess time.Time
}

// allow checks if a request is allowed and consumes a token if so.
func (b *bucket) allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now().UTC()
	elapsed := now.Sub(b.lastRefill).Seconds()
	b.tokens += elapsed * b.refillRate
	if b.tokens > b.maxTokens {
		b.tokens = b.maxTokens
	}
	b.lastRefill = now
	b.lastAccess = now

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// newBucket creates a token bucket for the given rate config.
func newBucket(cfg RateConfig) *bucket {
	maxTokens := float64(cfg.Requests)
	refillRate := maxTokens / cfg.Window.Seconds()
	return &bucket{
		tokens:     maxTokens,
		maxTokens:  maxTokens,
		refillRate: refillRate,
		lastRefill: time.Now().UTC(),
		lastAccess: time.Now().UTC(),
	}
}

// bucketStore manages a map of buckets by key.
type bucketStore struct {
	mu      sync.RWMutex
	buckets map[string]*bucket
	cfg     RateConfig
}

// newBucketStore creates a new bucket store.
func newBucketStore(cfg RateConfig) *bucketStore {
	return &bucketStore{
		buckets: make(map[string]*bucket),
		cfg:     cfg,
	}
}

// allow checks the rate limit for the given key.
func (s *bucketStore) allow(key string) bool {
	s.mu.RLock()
	b, exists := s.buckets[key]
	s.mu.RUnlock()

	if !exists {
		s.mu.Lock()
		// Double-check after acquiring write lock
		b, exists = s.buckets[key]
		if !exists {
			b = newBucket(s.cfg)
			s.buckets[key] = b
		}
		s.mu.Unlock()
	}

	return b.allow()
}

// cleanup removes buckets not accessed within the given duration.
func (s *bucketStore) cleanup(maxAge time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().UTC().Add(-maxAge)
	for key, b := range s.buckets {
		b.mu.Lock()
		lastAccess := b.lastAccess
		b.mu.Unlock()
		if lastAccess.Before(cutoff) {
			delete(s.buckets, key)
		}
	}
}

// RateLimiter manages multiple rate limiting tiers.
type RateLimiter struct {
	config  RateLimitConfig
	logger  *slog.Logger
	audit   *AuditService
	ipStore *bucketStore
	authStore *bucketStore
	sessionStore *bucketStore
	done    chan struct{}
}

// NewRateLimiter creates a new rate limiter with background cleanup.
func NewRateLimiter(cfg RateLimitConfig, logger *slog.Logger, audit *AuditService) *RateLimiter {
	cleanupInterval := cfg.CleanupInterval
	if cleanupInterval <= 0 {
		cleanupInterval = 5 * time.Minute
	}

	rl := &RateLimiter{
		config:       cfg,
		logger:       logger,
		audit:        audit,
		ipStore:      newBucketStore(cfg.GlobalPerIP),
		authStore:    newBucketStore(cfg.AuthEndpoint),
		sessionStore: newBucketStore(cfg.PerSession),
		done:         make(chan struct{}),
	}

	go rl.cleaner(cleanupInterval)
	return rl
}

// Close stops the background cleanup goroutine.
func (rl *RateLimiter) Close() {
	close(rl.done)
}

// AllowAuth checks the auth-endpoint rate limit for the given key (e.g. email).
// Returns true if the request is allowed, false if rate limited.
// This is used by MCP tool handlers where HTTP-level middleware cannot
// distinguish login requests from other tool calls on the same route.
func (rl *RateLimiter) AllowAuth(key string) bool {
	if !rl.config.AuthEndpoint.Enabled {
		return true
	}
	return rl.authStore.allow(key)
}

// AllowSession checks the per-session rate limit for the given session key.
// Returns true if the request is allowed, false if rate limited.
func (rl *RateLimiter) AllowSession(key string) bool {
	if !rl.config.PerSession.Enabled {
		return true
	}
	return rl.sessionStore.allow(key)
}

// Middleware returns HTTP middleware that applies IP-based rate limiting.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	if !rl.config.GlobalPerIP.Enabled {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := ExtractIP(r)
		if !rl.ipStore.allow(ip) {
			rl.logger.Warn("rate limit exceeded", "ip", ip, "tier", "global")
			if rl.audit != nil {
				rl.audit.Log(&AuditEntry{
					Action:     AuditRateLimitHit,
					ActorType:  "anonymous",
					IP:         ip,
					Source:     sourceFromRequest(r),
					Method:     r.Method,
					Endpoint:   r.URL.Path,
					StatusCode: http.StatusTooManyRequests,
					Metadata:   map[string]interface{}{"tier": "global-per-ip"},
				})
			}
			w.Header().Set("Retry-After", "60")
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// AuthMiddleware returns stricter rate limiting for auth endpoints.
func (rl *RateLimiter) AuthMiddleware(next http.Handler) http.Handler {
	if !rl.config.AuthEndpoint.Enabled {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := ExtractIP(r)
		if !rl.authStore.allow(ip) {
			rl.logger.Warn("auth rate limit exceeded", "ip", ip)
			if rl.audit != nil {
				rl.audit.Log(&AuditEntry{
					Action:     AuditRateLimitHit,
					ActorType:  "anonymous",
					IP:         ip,
					Source:     sourceFromRequest(r),
					Method:     r.Method,
					Endpoint:   r.URL.Path,
					StatusCode: http.StatusTooManyRequests,
					Metadata:   map[string]interface{}{"tier": "auth-endpoint"},
				})
			}
			w.Header().Set("Retry-After", "60")
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// SessionMiddleware returns identity-based rate limiting.
// sessionKeyFunc extracts the session/identity key from the request.
func (rl *RateLimiter) SessionMiddleware(sessionKeyFunc func(*http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if !rl.config.PerSession.Enabled {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := sessionKeyFunc(r)
			if key == "" {
				// No session identity; skip session-based limiting
				next.ServeHTTP(w, r)
				return
			}
			if !rl.sessionStore.allow(key) {
				rl.logger.Warn("session rate limit exceeded", "session_key", key)
				if rl.audit != nil {
					rl.audit.Log(&AuditEntry{
						Action:     AuditRateLimitHit,
						ActorID:    key,
						ActorType:  "user",
						IP:         ExtractIP(r),
						Source:     sourceFromRequest(r),
						Method:     r.Method,
						Endpoint:   r.URL.Path,
						StatusCode: http.StatusTooManyRequests,
						Metadata:   map[string]interface{}{"tier": "per-session"},
					})
				}
				w.Header().Set("Retry-After", "60")
				http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// cleaner periodically removes stale buckets.
func (rl *RateLimiter) cleaner(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Stale = not accessed in 2x the window duration
	ipMaxAge := rl.config.GlobalPerIP.Window * 2
	authMaxAge := rl.config.AuthEndpoint.Window * 2
	sessionMaxAge := rl.config.PerSession.Window * 2

	for {
		select {
		case <-ticker.C:
			rl.ipStore.cleanup(ipMaxAge)
			rl.authStore.cleanup(authMaxAge)
			rl.sessionStore.cleanup(sessionMaxAge)
		case <-rl.done:
			return
		}
	}
}

// ExtractIP gets the client IP from the request.
// Prefers X-Real-IP (set by NGINX to $remote_addr, not spoofable)
// over X-Forwarded-For (which can be forged by clients).
func ExtractIP(r *http.Request) string {
	// Check X-Real-IP first (set by NGINX to the true remote address)
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}

	// Fall back to X-Forwarded-For
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if idx := strings.IndexByte(xff, ','); idx != -1 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}

	// Fall back to RemoteAddr (strip port)
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	// Normalize IPv6 loopback to IPv4 for readability
	if host == "::1" {
		return "127.0.0.1"
	}
	return host
}
