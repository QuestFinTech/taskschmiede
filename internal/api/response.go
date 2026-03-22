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

import (
	"encoding/json"
	"net/http"
)

// dataResponse wraps a single entity response.
type dataResponse struct {
	Data interface{} `json:"data"`
}

// listResponse wraps a list response with pagination metadata.
type listResponse struct {
	Data interface{}  `json:"data"`
	Meta listMeta     `json:"meta"`
}

// listMeta holds pagination metadata.
type listMeta struct {
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// errorBody represents the error response structure.
type errorBody struct {
	Error errorDetail `json:"error"`
}

// errorDetail holds the error code and message.
type errorDetail struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

// writeData writes a single entity JSON response.
func writeData(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(dataResponse{Data: data})
}

// writeList writes a paginated list JSON response.
func writeList(w http.ResponseWriter, data interface{}, total, limit, offset int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(listResponse{
		Data: data,
		Meta: listMeta{Total: total, Limit: limit, Offset: offset},
	})
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorBody{
		Error: errorDetail{Code: code, Message: message},
	})
}

// writeErrorWithDetails writes a JSON error response with structured details.
func writeErrorWithDetails(w http.ResponseWriter, status int, code, message string, details interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorBody{
		Error: errorDetail{Code: code, Message: message, Details: details},
	})
}
