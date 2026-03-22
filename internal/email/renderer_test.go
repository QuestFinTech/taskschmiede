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


package email

import (
	"html/template"
	"strings"
	"testing"
)

func TestNewRenderer(t *testing.T) {
	r, err := NewRenderer()
	if err != nil {
		t.Fatalf("NewRenderer() error: %v", err)
	}
	if r == nil {
		t.Fatal("NewRenderer() returned nil")
	}
}

func TestRenderVerificationCode(t *testing.T) {
	r, err := NewRenderer()
	if err != nil {
		t.Fatalf("NewRenderer() error: %v", err)
	}

	data := &VerificationCodeData{
		Greeting:   "Hello Mike",
		Intro:      "Your verification code is:",
		Code:       "abc-def-ghi",
		ExpiryNote: "This code will expire in 15 minutes.",
		ActionURL:  "https://portal.example.com/verify?email=mike@test.com&code=abc-def-ghi",
		ButtonText: "Verify Email",
		ManualNote: "Or enter this code manually on the verification page.",
		Closing:    "Best regards,",
		TeamName:   "Team Taskschmiede",
	}

	msg, err := r.RenderVerificationCode(data)
	if err != nil {
		t.Fatalf("RenderVerificationCode() error: %v", err)
	}

	// Check both bodies are non-empty
	if msg.Body == "" {
		t.Error("text body is empty")
	}
	if msg.HTMLBody == "" {
		t.Error("HTML body is empty")
	}

	// HTML should contain the code, a button link, and TASKSCHMIEDE branding
	if !strings.Contains(msg.HTMLBody, "abc-def-ghi") {
		t.Error("HTML body missing verification code")
	}
	if !strings.Contains(msg.HTMLBody, "Verify Email") {
		t.Error("HTML body missing button text")
	}
	if !strings.Contains(msg.HTMLBody, "https://portal.example.com/verify") {
		t.Error("HTML body missing action URL")
	}
	if !strings.Contains(msg.HTMLBody, "TASKSCHMIEDE") {
		t.Error("HTML body missing branding")
	}

	// Text body should contain the code but no HTML tags
	if !strings.Contains(msg.Body, "abc-def-ghi") {
		t.Error("text body missing verification code")
	}
	if strings.Contains(msg.Body, "<") {
		t.Error("text body contains HTML tags")
	}
}

func TestRenderVerificationCodeNoURL(t *testing.T) {
	r, err := NewRenderer()
	if err != nil {
		t.Fatalf("NewRenderer() error: %v", err)
	}

	data := &VerificationCodeData{
		Greeting:   "Hello",
		Intro:      "Your password reset code is:",
		Code:       "xyz-123-456",
		ExpiryNote: "This code will expire in 15 minutes.",
		Closing:    "Best regards,",
		TeamName:   "Team Taskschmiede",
	}

	msg, err := r.RenderVerificationCode(data)
	if err != nil {
		t.Fatalf("RenderVerificationCode() error: %v", err)
	}

	if msg.Body == "" || msg.HTMLBody == "" {
		t.Error("bodies should be non-empty")
	}

	// Without ActionURL, the button section should not appear
	if strings.Contains(msg.HTMLBody, "Reset Password") {
		t.Error("HTML body should not contain button when no ActionURL")
	}
}

func TestRenderWaitlistWelcome(t *testing.T) {
	r, err := NewRenderer()
	if err != nil {
		t.Fatalf("NewRenderer() error: %v", err)
	}

	data := &WaitlistWelcomeData{
		Greeting:  "Hello Agent",
		Message:   "Good news -- a slot has opened up.",
		Token:     "tok_secret_abc123xyz",
		TokenNote: "Store this securely.",
		NextSteps: "Call ts.onboard.start_interview to begin.",
		Closing:   "Best regards,",
		TeamName:  "Team Taskschmiede",
	}

	msg, err := r.RenderWaitlistWelcome(data)
	if err != nil {
		t.Fatalf("RenderWaitlistWelcome() error: %v", err)
	}

	if msg.Body == "" || msg.HTMLBody == "" {
		t.Error("bodies should be non-empty")
	}
	if !strings.Contains(msg.HTMLBody, "tok_secret_abc123xyz") {
		t.Error("HTML body missing token")
	}
	if !strings.Contains(msg.Body, "tok_secret_abc123xyz") {
		t.Error("text body missing token")
	}
}

func TestRenderInactivityWarning(t *testing.T) {
	r, err := NewRenderer()
	if err != nil {
		t.Fatalf("NewRenderer() error: %v", err)
	}

	data := &InactivityData{
		Greeting: "Hello Test User",
		Message:  "Your account has been inactive for 14 days.",
		Advice:   "Log in to keep your account active.",
		Closing:  "Best regards,",
		TeamName: "Team Taskschmiede",
	}

	msg, err := r.RenderInactivityWarning(data)
	if err != nil {
		t.Fatalf("RenderInactivityWarning() error: %v", err)
	}

	if msg.Body == "" || msg.HTMLBody == "" {
		t.Error("bodies should be non-empty")
	}
	// Warning template has amber accent
	if !strings.Contains(msg.HTMLBody, "#f5a623") {
		t.Error("HTML body missing warning accent color")
	}
	if !strings.Contains(msg.Body, "inactive for 14 days") {
		t.Error("text body missing inactivity message")
	}
}

func TestRenderInactivityDeactivation(t *testing.T) {
	r, err := NewRenderer()
	if err != nil {
		t.Fatalf("NewRenderer() error: %v", err)
	}

	data := &InactivityData{
		Greeting: "Hello Test User",
		Message:  "Your account has been deactivated.",
		Advice:   "You can reactivate by logging in.",
		Closing:  "Best regards,",
		TeamName: "Team Taskschmiede",
	}

	msg, err := r.RenderInactivityDeactivation(data)
	if err != nil {
		t.Fatalf("RenderInactivityDeactivation() error: %v", err)
	}

	if msg.Body == "" || msg.HTMLBody == "" {
		t.Error("bodies should be non-empty")
	}
	// Deactivation template has red accent
	if !strings.Contains(msg.HTMLBody, "#d0021b") {
		t.Error("HTML body missing deactivation accent color")
	}
	if strings.Contains(msg.Body, "<") {
		t.Error("text body contains HTML tags")
	}
}

func TestMdLinksToHTML(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want template.HTML
	}{
		{
			name: "single link",
			in:   "See [Configuration Guide](https://docs.taskschmiede.dev/guides/configuration/) for details.",
			want: `See <a href="https://docs.taskschmiede.dev/guides/configuration/" style="color:#1a1a2e;">Configuration Guide</a> for details.`,
		},
		{
			name: "multiple links",
			in:   "Read [Docs](https://docs.taskschmiede.dev) and [DPA](https://taskschmiede.com/dpa).",
			want: `Read <a href="https://docs.taskschmiede.dev" style="color:#1a1a2e;">Docs</a> and <a href="https://taskschmiede.com/dpa" style="color:#1a1a2e;">DPA</a>.`,
		},
		{
			name: "no links",
			in:   "Plain text without links.",
			want: "Plain text without links.",
		},
		{
			name: "html special chars escaped",
			in:   `Check <b>bold</b> and [Link](https://taskschmiede.com/a&b)`,
			want: `Check &lt;b&gt;bold&lt;/b&gt; and <a href="https://taskschmiede.com/a&amp;b" style="color:#1a1a2e;">Link</a>`,
		},
		{
			name: "blocked domain stripped to text",
			in:   "See [Evil](https://evil.com/phish) for details.",
			want: "See Evil for details.",
		},
		{
			name: "subdomain not implicitly allowed",
			in:   "See [Sub](https://sub.taskschmiede.com/path) link.",
			want: "See Sub link.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mdLinksToHTML(tt.in)
			if got != tt.want {
				t.Errorf("mdLinksToHTML(%q)\n got: %s\nwant: %s", tt.in, got, tt.want)
			}
		})
	}
}

func TestMdLinksToText(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "single link",
			in:   "See [Configuration Guide](https://docs.taskschmiede.dev/guides/configuration/) for details.",
			want: "See Configuration Guide (https://docs.taskschmiede.dev/guides/configuration/) for details.",
		},
		{
			name: "no links",
			in:   "Plain text.",
			want: "Plain text.",
		},
		{
			name: "blocked domain stripped",
			in:   "See [Evil](https://evil.com/phish) here.",
			want: "See Evil here.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mdLinksToText(tt.in)
			if got != tt.want {
				t.Errorf("mdLinksToText(%q)\n got: %s\nwant: %s", tt.in, got, tt.want)
			}
		})
	}
}

func TestIsLinkAllowed(t *testing.T) {
	tests := []struct {
		url     string
		allowed bool
	}{
		{"https://taskschmiede.com/dpa", true},
		{"https://taskschmiede.com/privacy#section", true},
		{"https://docs.taskschmiede.dev/guides/configuration/", true},
		{"https://evil.com/phish", false},
		{"https://sub.taskschmiede.com/path", false},
		{"https://taskschmiede.com.evil.com/fake", false},
		{"https://faketaskschmiede.com/x", false},
		{"not-a-url", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			got := isLinkAllowed(tt.url)
			if got != tt.allowed {
				t.Errorf("isLinkAllowed(%q) = %v, want %v", tt.url, got, tt.allowed)
			}
		})
	}
}

func TestRenderSupportReplyWithLinks(t *testing.T) {
	r, err := NewRenderer()
	if err != nil {
		t.Fatalf("NewRenderer() error: %v", err)
	}

	body := "Dear Mike,\n\nPlease see our [DPA](https://taskschmiede.com/dpa) for details."
	data := &SupportReplyData{
		Body:           body,
		BodyParagraphs: []string{"Dear Mike,", "Please see our [DPA](https://taskschmiede.com/dpa) for details."},
		ReferenceID:    "SUP-20260313-0001",
	}

	msg, err := r.RenderSupportReply(data)
	if err != nil {
		t.Fatalf("RenderSupportReply() error: %v", err)
	}

	// HTML should have anchor tag
	if !strings.Contains(msg.HTMLBody, `<a href="https://taskschmiede.com/dpa"`) {
		t.Error("HTML body missing anchor tag for DPA link")
	}
	if !strings.Contains(msg.HTMLBody, ">DPA</a>") {
		t.Error("HTML body missing link text")
	}

	// Text should have "DPA (url)" format
	if !strings.Contains(msg.Body, "DPA (https://taskschmiede.com/dpa)") {
		t.Error("text body missing expanded link")
	}

	// Both should have sign-off and AI disclosure
	if !strings.Contains(msg.HTMLBody, "Best regards,") {
		t.Error("HTML body missing sign-off")
	}
	if !strings.Contains(msg.HTMLBody, "AI support agent") {
		t.Error("HTML body missing AI disclosure")
	}
	if !strings.Contains(msg.Body, "Best regards,") {
		t.Error("text body missing sign-off")
	}
}
