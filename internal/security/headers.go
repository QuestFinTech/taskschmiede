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

// HeadersConfig holds security header configuration.
type HeadersConfig struct {
	HSTSEnabled    bool   `yaml:"hsts-enabled"`
	HSTSMaxAge     int    `yaml:"hsts-max-age"`
	CSPPolicy      string `yaml:"csp-policy"`
	FrameOptions   string `yaml:"frame-options"`
	ReferrerPolicy string `yaml:"referrer-policy"`
}

// DefaultHeadersConfig returns sensible defaults.
func DefaultHeadersConfig() HeadersConfig {
	return HeadersConfig{
		HSTSEnabled:    true,
		HSTSMaxAge:     31536000,
		CSPPolicy:      "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:",
		FrameOptions:   "DENY",
		ReferrerPolicy: "strict-origin-when-cross-origin",
	}
}

// SecurityHeaders returns middleware that sets security headers on all responses.
func SecurityHeaders(cfg HeadersConfig) func(http.Handler) http.Handler {
	csp := cfg.CSPPolicy
	if csp == "" {
		csp = DefaultHeadersConfig().CSPPolicy
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// CSP is application-aware (knows about inline scripts, data: URIs, etc.)
			// and is NOT set by the nginx reverse proxy, so it belongs here.
			w.Header().Set("Content-Security-Policy", csp)

			// X-Content-Type-Options, X-Frame-Options, Referrer-Policy, and
			// Permissions-Policy are set by nginx (snippets/security-headers.conf)
			// on all server blocks. Do not duplicate them here -- duplicate
			// headers confuse browsers and complicate debugging.

			if cfg.HSTSEnabled && cfg.HSTSMaxAge > 0 {
				w.Header().Set("Strict-Transport-Security",
					"max-age="+http.StatusText(0)[:0]+itoa(cfg.HSTSMaxAge)+"; includeSubDomains")
			}

			next.ServeHTTP(w, r)
		})
	}
}

// itoa converts an int to a string without importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	pos := len(buf)
	negative := n < 0
	if negative {
		n = -n
	}
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	if negative {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
