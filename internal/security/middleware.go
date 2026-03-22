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
	"net/http"
	"time"
)

// responseInterceptor captures the status code from ResponseWriter.
type responseInterceptor struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

func (ri *responseInterceptor) WriteHeader(code int) {
	if !ri.written {
		ri.statusCode = code
		ri.written = true
	}
	ri.ResponseWriter.WriteHeader(code)
}

func (ri *responseInterceptor) Write(b []byte) (int, error) {
	if !ri.written {
		ri.statusCode = http.StatusOK
		ri.written = true
	}
	return ri.ResponseWriter.Write(b)
}

// Unwrap returns the underlying ResponseWriter for http.ResponseController compatibility.
func (ri *responseInterceptor) Unwrap() http.ResponseWriter {
	return ri.ResponseWriter
}

// AuditMiddleware returns middleware that logs request/response metadata to the audit service.
// actorExtractor is a function that extracts (actorID, actorType) from the request context.
// Pass nil to skip actor extraction (all entries will be logged as anonymous).
func AuditMiddleware(audit *AuditService, actorExtractor func(*http.Request) (string, string)) func(http.Handler) http.Handler {
	if audit == nil {
		return func(next http.Handler) http.Handler { return next }
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now().UTC()

			interceptor := &responseInterceptor{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			next.ServeHTTP(interceptor, r)

			duration := time.Since(start)

			actorID := "anonymous"
			actorType := "anonymous"
			if actorExtractor != nil {
				id, typ := actorExtractor(r)
				if id != "" {
					actorID = id
				}
				if typ != "" {
					actorType = typ
				}
			}

			audit.Log(&AuditEntry{
				Action:     AuditRequest,
				ActorID:    actorID,
				ActorType:  actorType,
				Method:     r.Method,
				Endpoint:   r.URL.Path,
				StatusCode: interceptor.statusCode,
				IP:         ExtractIP(r),
				Source:     sourceFromRequest(r),
				Duration:   duration,
			})
		})
	}
}

// sourceFromRequest reads the X-Source header and validates it.
// Returns "console", "portal", or defaults to "api".
func sourceFromRequest(r *http.Request) string {
	switch r.Header.Get("X-Source") {
	case "console":
		return "console"
	case "portal":
		return "portal"
	default:
		return "api"
	}
}

// Chain composes middleware in order: the first argument wraps outermost.
// Chain(a, b, c)(handler) == a(b(c(handler)))
func Chain(middlewares ...func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	return func(final http.Handler) http.Handler {
		for i := len(middlewares) - 1; i >= 0; i-- {
			final = middlewares[i](final)
		}
		return final
	}
}
