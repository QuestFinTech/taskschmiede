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

import "net/http"

// BodyLimitConfig holds request body size limits.
type BodyLimitConfig struct {
	MaxBodySize int64 `yaml:"max-body-size"`
}

// DefaultBodyLimitConfig returns sensible defaults.
func DefaultBodyLimitConfig() BodyLimitConfig {
	return BodyLimitConfig{MaxBodySize: 1 << 20} // 1 MB
}

// BodyLimit returns middleware that limits request body size.
// Returns 413 Request Entity Too Large if the body exceeds the configured limit.
// Uses a two-layer approach:
//  1. Pre-check Content-Length header for an immediate 413 (covers most clients).
//  2. Wrap body with http.MaxBytesReader as a safety net for chunked encoding.
func BodyLimit(cfg BodyLimitConfig) func(http.Handler) http.Handler {
	maxSize := cfg.MaxBodySize
	if maxSize <= 0 {
		maxSize = DefaultBodyLimitConfig().MaxBodySize
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Layer 1: reject immediately if Content-Length exceeds limit.
			if r.ContentLength > maxSize {
				http.Error(w, "Request Entity Too Large", http.StatusRequestEntityTooLarge)
				return
			}
			// Layer 2: safety net for chunked/unknown-length bodies.
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, maxSize)
			}
			next.ServeHTTP(w, r)
		})
	}
}
