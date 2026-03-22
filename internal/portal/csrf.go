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


package portal

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
)

const csrfCookieName = "portal_csrf"
const csrfFieldName = "csrf_token"

// generateCSRFToken creates a cryptographically random CSRF token.
func generateCSRFToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return base64.URLEncoding.EncodeToString(b)
}

// setCSRFToken generates a token, sets it as a cookie, and returns it for
// embedding in a template hidden field. Uses double-submit cookie pattern.
func (s *Server) setCSRFToken(w http.ResponseWriter) string {
	token := generateCSRFToken()
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.secure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   3600,
	})
	return token
}

// validateCSRF checks that the POST form field matches the CSRF cookie.
func validateCSRF(r *http.Request) bool {
	cookie, err := r.Cookie(csrfCookieName)
	if err != nil || cookie.Value == "" {
		return false
	}
	formToken := r.FormValue(csrfFieldName)
	if formToken == "" {
		return false
	}
	return formToken == cookie.Value
}
