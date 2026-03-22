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
	"net/http"
	"sync"
	"sync/atomic"
)

// ConnLimitConfig holds connection limit settings.
type ConnLimitConfig struct {
	MaxGlobal int `yaml:"max-global"` // 0 = unlimited
	MaxPerIP  int `yaml:"max-per-ip"` // 0 = unlimited
}

// DefaultConnLimitConfig returns sensible defaults.
func DefaultConnLimitConfig() ConnLimitConfig {
	return ConnLimitConfig{
		MaxGlobal: 1000,
		MaxPerIP:  50,
	}
}

// ConnLimiter tracks concurrent connections and enforces limits.
type ConnLimiter struct {
	cfg    ConnLimitConfig
	logger *slog.Logger

	global atomic.Int64

	perIP   map[string]*atomic.Int64
	perIPMu sync.RWMutex
}

// NewConnLimiter creates a connection limiter.
func NewConnLimiter(cfg ConnLimitConfig, logger *slog.Logger) *ConnLimiter {
	return &ConnLimiter{
		cfg:    cfg,
		logger: logger,
		perIP:  make(map[string]*atomic.Int64),
	}
}

// GlobalCount returns the current global connection count.
func (cl *ConnLimiter) GlobalCount() int64 {
	return cl.global.Load()
}

// Middleware returns HTTP middleware that enforces connection limits.
func (cl *ConnLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := ExtractIP(r)

		// Check global limit
		if cl.cfg.MaxGlobal > 0 {
			current := cl.global.Load()
			if current >= int64(cl.cfg.MaxGlobal) {
				cl.logger.Warn("Global connection limit reached",
					"current", current,
					"limit", cl.cfg.MaxGlobal,
					"ip", ip,
				)
				http.Error(w, "Too many connections", http.StatusServiceUnavailable)
				return
			}
		}

		// Check per-IP limit
		if cl.cfg.MaxPerIP > 0 {
			counter := cl.getIPCounter(ip)
			current := counter.Load()
			if current >= int64(cl.cfg.MaxPerIP) {
				cl.logger.Warn("Per-IP connection limit reached",
					"current", current,
					"limit", cl.cfg.MaxPerIP,
					"ip", ip,
				)
				http.Error(w, "Too many connections from your IP", http.StatusServiceUnavailable)
				return
			}
		}

		// Track connection
		cl.global.Add(1)
		ipCounter := cl.getIPCounter(ip)
		ipCounter.Add(1)

		defer func() {
			cl.global.Add(-1)
			ipCounter.Add(-1)
		}()

		next.ServeHTTP(w, r)
	})
}

// getIPCounter returns the atomic counter for a given IP, creating it if needed.
func (cl *ConnLimiter) getIPCounter(ip string) *atomic.Int64 {
	cl.perIPMu.RLock()
	counter, ok := cl.perIP[ip]
	cl.perIPMu.RUnlock()
	if ok {
		return counter
	}

	cl.perIPMu.Lock()
	defer cl.perIPMu.Unlock()

	// Double-check after acquiring write lock
	if counter, ok := cl.perIP[ip]; ok {
		return counter
	}

	counter = &atomic.Int64{}
	cl.perIP[ip] = counter
	return counter
}
