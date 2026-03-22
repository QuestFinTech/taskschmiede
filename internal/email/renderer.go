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
	"bytes"
	"embed"
	"fmt"
	htmltpl "html/template"
	"net/url"
	"regexp"
	"strings"
	texttpl "text/template"
)

//go:embed templates/*.tmpl
var templateFS embed.FS

// VerificationCodeData holds data for verification code and password reset emails.
type VerificationCodeData struct {
	Greeting   string // "Hello Mike"
	Intro      string // "Your verification code is:" or "Your password reset code is:"
	Code       string // "abc-def-ghi"
	ExpiryNote string // "This code will expire in 15 minutes."
	ActionURL  string // Full URL to verify/reset page
	ButtonText string // "Verify Email" or "Reset Password"
	ManualNote string // "Or enter this code manually:"
	Closing    string // "Best regards,"
	TeamName   string // "Team Taskschmiede"
}

// WaitlistWelcomeData holds data for waitlist welcome emails.
type WaitlistWelcomeData struct {
	Greeting  string
	Message   string
	Token     string
	TokenNote string
	NextSteps string
	Closing   string
	TeamName  string
}

// InactivityData holds data for inactivity warning and deactivation emails.
type InactivityData struct {
	Greeting string
	Message  string
	Advice   string
	Closing  string
	TeamName string
}

// SupportAckData holds data for support acknowledgement emails.
type SupportAckData struct {
	Greeting    string // "Dear Mike"
	Message     string // "Thank you for contacting Taskschmiede Support."
	ReferenceID string // "SUP-20260313-0001"
	DeptNote    string // Department-specific note
	Closing     string // "Best regards,"
	TeamName    string // "Taskschmiede"
}

// SupportReplyData holds data for support reply emails.
// Body contains the complete message text including greeting, content, and
// sign-off. The template provides only the branded HTML shell, reference ID,
// and the legally required AI disclosure footer.
type SupportReplyData struct {
	Body           string   // Full reply text (used in plain text template)
	BodyParagraphs []string // Full reply text split into paragraphs (used in HTML template)
	ReferenceID    string   // "SUP-20260313-0001"
}

// Renderer parses and renders email templates.
type Renderer struct {
	htmlTemplates *htmltpl.Template
	textTemplates *texttpl.Template
}

// mdLinkRe matches markdown-style links: [text](url)
var mdLinkRe = regexp.MustCompile(`\[([^\]]+)\]\((https?://[^\s)]+)\)`)

// linkAllowedDomains lists domains permitted in outbound markdown links.
// Links to other domains are stripped to plain text (link text only).
var linkAllowedDomains = []string{
	"taskschmiede.com",
	"docs.taskschmiede.dev",
}

// isLinkAllowed checks whether a URL's host exactly matches the allowlist.
// Subdomains are not implicitly allowed -- each domain must be listed.
func isLinkAllowed(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	for _, d := range linkAllowedDomains {
		if host == d {
			return true
		}
	}
	return false
}

// mdLinksToHTML converts markdown links to HTML anchor tags.
// Input text is HTML-escaped first, then allowed links are converted.
// Links to non-allowed domains are rendered as plain text (link text only).
func mdLinksToHTML(s string) htmltpl.HTML {
	escaped := htmltpl.HTMLEscapeString(s)
	result := mdLinkRe.ReplaceAllStringFunc(escaped, func(match string) string {
		parts := mdLinkRe.FindStringSubmatch(match)
		if len(parts) != 3 {
			return match
		}
		text, href := parts[1], parts[2]
		if !isLinkAllowed(href) {
			return text
		}
		return fmt.Sprintf(`<a href="%s" style="color:#1a1a2e;">%s</a>`, href, text)
	})
	return htmltpl.HTML(result) //nolint:gosec // escaped above, only re-adding safe <a> tags
}

// mdLinksToText converts markdown links to "text (url)" for plain text emails.
// Links to non-allowed domains are rendered as plain text (link text only).
func mdLinksToText(s string) string {
	return mdLinkRe.ReplaceAllStringFunc(s, func(match string) string {
		parts := mdLinkRe.FindStringSubmatch(match)
		if len(parts) != 3 {
			return match
		}
		text, href := parts[1], parts[2]
		if !isLinkAllowed(href) {
			return text
		}
		return fmt.Sprintf("%s (%s)", text, href)
	})
}

// NewRenderer creates a new Renderer by parsing all embedded templates.
func NewRenderer() (*Renderer, error) {
	htmlFuncs := htmltpl.FuncMap{
		"mdlinks": mdLinksToHTML,
	}
	textFuncs := texttpl.FuncMap{
		"mdlinks": mdLinksToText,
	}

	htmlTpl, err := htmltpl.New("").Funcs(htmlFuncs).ParseFS(templateFS, "templates/*.html.tmpl")
	if err != nil {
		return nil, fmt.Errorf("parse HTML templates: %w", err)
	}

	textTpl, err := texttpl.New("").Funcs(textFuncs).ParseFS(templateFS, "templates/*.txt.tmpl")
	if err != nil {
		return nil, fmt.Errorf("parse text templates: %w", err)
	}

	return &Renderer{
		htmlTemplates: htmlTpl,
		textTemplates: textTpl,
	}, nil
}

// render executes a named template pair and returns an OutgoingMessage with both bodies.
func (r *Renderer) render(name string, data interface{}) (*OutgoingMessage, error) {
	var htmlBuf, textBuf bytes.Buffer

	htmlName := name + ".html.tmpl"
	textName := name + ".txt.tmpl"

	if err := r.htmlTemplates.ExecuteTemplate(&htmlBuf, htmlName, data); err != nil {
		return nil, fmt.Errorf("render HTML template %s: %w", htmlName, err)
	}

	if err := r.textTemplates.ExecuteTemplate(&textBuf, textName, data); err != nil {
		return nil, fmt.Errorf("render text template %s: %w", textName, err)
	}

	return &OutgoingMessage{
		Body:     strings.TrimSpace(textBuf.String()),
		HTMLBody: strings.TrimSpace(htmlBuf.String()),
	}, nil
}

// RenderVerificationCode renders the verification code email (used for both
// registration verification and password reset).
func (r *Renderer) RenderVerificationCode(data *VerificationCodeData) (*OutgoingMessage, error) {
	return r.render("verification_code", data)
}

// RenderWaitlistWelcome renders the waitlist welcome email.
func (r *Renderer) RenderWaitlistWelcome(data *WaitlistWelcomeData) (*OutgoingMessage, error) {
	return r.render("waitlist_welcome", data)
}

// RenderInactivityWarning renders the inactivity warning email.
func (r *Renderer) RenderInactivityWarning(data *InactivityData) (*OutgoingMessage, error) {
	return r.render("inactivity_warning", data)
}

// RenderInactivityDeactivation renders the inactivity deactivation email.
func (r *Renderer) RenderInactivityDeactivation(data *InactivityData) (*OutgoingMessage, error) {
	return r.render("inactivity_deactivation", data)
}

// RenderSupportAck renders a branded support acknowledgement email.
func (r *Renderer) RenderSupportAck(data *SupportAckData) (*OutgoingMessage, error) {
	return r.render("support_ack", data)
}

// RenderSupportReply renders a branded support reply email.
func (r *Renderer) RenderSupportReply(data *SupportReplyData) (*OutgoingMessage, error) {
	return r.render("support_reply", data)
}
