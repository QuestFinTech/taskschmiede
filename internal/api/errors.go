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


package api

import "net/http"

// APIError represents a structured error returned by exported API methods.
// Both REST handlers and MCP handlers can map this to their response format.
type APIError struct {
	Code    string      // Error code (e.g., "not_found", "invalid_input")
	Message string      // Human-readable message
	Status  int         // HTTP status code (used by REST, ignored by MCP)
	Details interface{} // Optional structured details (e.g., DoD condition results)
}

// Error returns the human-readable error message.
func (e *APIError) Error() string {
	return e.Message
}

// Common error constructors.

func errNotFound(entity, message string) *APIError {
	return &APIError{Code: "not_found", Message: message, Status: http.StatusNotFound}
}

func errInvalidInput(message string) *APIError {
	return &APIError{Code: "invalid_input", Message: message, Status: http.StatusBadRequest}
}

func errConflict(message string) *APIError {
	return &APIError{Code: "conflict", Message: message, Status: http.StatusConflict}
}

func errInternal(message string) *APIError {
	return &APIError{Code: "internal_error", Message: message, Status: http.StatusInternalServerError}
}

func errUnauthorized(message string) *APIError {
	return &APIError{Code: "unauthorized", Message: message, Status: http.StatusUnauthorized}
}

func errForbidden(message string) *APIError {
	return &APIError{Code: "forbidden", Message: message, Status: http.StatusForbidden}
}

func errInvalidTransition(message string) *APIError {
	return &APIError{Code: "invalid_transition", Message: message, Status: http.StatusBadRequest}
}

func errTierLimit(message string) *APIError {
	return &APIError{Code: "tier_limit", Message: message, Status: http.StatusForbidden}
}

func errVelocityLimit(message string) *APIError {
	return &APIError{Code: "velocity_limit", Message: message, Status: http.StatusTooManyRequests}
}

// writeAPIError writes an APIError as an HTTP JSON response.
func writeAPIError(w http.ResponseWriter, e *APIError) {
	if e.Details != nil {
		writeErrorWithDetails(w, e.Status, e.Code, e.Message, e.Details)
		return
	}
	writeError(w, e.Status, e.Code, e.Message)
}
