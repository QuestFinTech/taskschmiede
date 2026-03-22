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
	"testing"

	"github.com/QuestFinTech/taskschmiede/internal/storage"
)

func TestAuditSummary(t *testing.T) {
	tests := []struct {
		name   string
		record *storage.AuditLogRecord
		want   string
	}{
		{
			name:   "login success",
			record: &storage.AuditLogRecord{Action: "login_success"},
			want:   "Logged in",
		},
		{
			name:   "login success via portal",
			record: &storage.AuditLogRecord{Action: "login_success", Source: "portal"},
			want:   "Logged in via Portal",
		},
		{
			name:   "login failure",
			record: &storage.AuditLogRecord{Action: "login_failure"},
			want:   "Login attempt failed",
		},
		{
			name:   "login failure via console",
			record: &storage.AuditLogRecord{Action: "login_failure", Source: "console"},
			want:   "Login attempt failed via Console",
		},
		{
			name:   "token created",
			record: &storage.AuditLogRecord{Action: "token_created"},
			want:   "Created an API token",
		},
		{
			name:   "permission denied",
			record: &storage.AuditLogRecord{Action: "permission_denied", Method: "POST", Endpoint: "/api/v1/tasks"},
			want:   "Access denied: POST /api/v1/tasks",
		},
		{
			name:   "request POST tasks",
			record: &storage.AuditLogRecord{Action: "request", Method: "POST", Endpoint: "/api/v1/tasks", StatusCode: 201},
			want:   "Created task",
		},
		{
			name:   "request GET tasks list",
			record: &storage.AuditLogRecord{Action: "request", Method: "GET", Endpoint: "/api/v1/tasks", StatusCode: 200},
			want:   "Listed tasks",
		},
		{
			name:   "request GET single task",
			record: &storage.AuditLogRecord{Action: "request", Method: "GET", Endpoint: "/api/v1/tasks/tsk_123", StatusCode: 200},
			want:   "Viewed task",
		},
		{
			name:   "request PATCH endeavour",
			record: &storage.AuditLogRecord{Action: "request", Method: "PATCH", Endpoint: "/api/v1/endeavours/edv_123", StatusCode: 200},
			want:   "Updated endeavour",
		},
		{
			name:   "request MCP",
			record: &storage.AuditLogRecord{Action: "request", Method: "POST", Endpoint: "/mcp", StatusCode: 200},
			want:   "MCP tool call (HTTP 200)",
		},
		{
			name:   "unknown action",
			record: &storage.AuditLogRecord{Action: "some_custom_action"},
			want:   "some_custom_action",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := auditSummary(tt.record)
			if got != tt.want {
				t.Errorf("auditSummary() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSingularize(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"tasks", "task"},
		{"endeavours", "endeavour"},
		{"demands", "demand"},
		{"organizations", "organization"},
		{"entries", "entry"},
		{"policies", "policy"},
		{"status", "status"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := singularize(tt.input)
			if got != tt.want {
				t.Errorf("singularize(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
