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


// Package notify provides lightweight notification dispatch for operational
// events. It supports generic webhooks (Slack, Discord, Teams, Mattermost,
// Ntfy, Gotify, or any HTTP endpoint) and minimal SMTP for critical alerts.
//
// This package is intentionally independent of the app's email package
// (internal/email) so it can be used from the proxy, which must send
// notifications even when the app server is down.
package notify

import (
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// Event represents a notification event.
type Event struct {
	Service   string            // originating service (e.g. "taskschmiede-proxy")
	Type      string            // event type (e.g. "state_change")
	Summary   string            // human-readable one-liner (used as subject/text)
	Detail    string            // additional context
	Timestamp time.Time         // when the event occurred (UTC)
	Fields    map[string]string // arbitrary key-value pairs for structured data
}

// Config holds configuration for all notification channels.
type Config struct {
	Webhook *WebhookConfig
	SMTP    *SMTPConfig
}

// Notifier dispatches events to all configured notification channels.
// Send calls are asynchronous -- errors are logged but do not block.
type Notifier struct {
	logger  *slog.Logger
	webhook *WebhookSender
	smtp    *SMTPSender
}

// New creates a Notifier with the configured channels. Channels with
// incomplete configuration (e.g. empty URL or host) are silently skipped.
func New(logger *slog.Logger, cfg Config) *Notifier {
	n := &Notifier{logger: logger}
	if cfg.Webhook != nil && cfg.Webhook.URL != "" {
		n.webhook = newWebhookSender(logger, *cfg.Webhook)
		logger.Info("Notification channel configured", "channel", "webhook", "url", cfg.Webhook.URL)
	}
	if cfg.SMTP != nil && cfg.SMTP.Host != "" && len(cfg.SMTP.To) > 0 {
		n.smtp = newSMTPSender(logger, *cfg.SMTP)
		logger.Info("Notification channel configured", "channel", "smtp",
			"host", cfg.SMTP.Host, "to", strings.Join(cfg.SMTP.To, ","))
	}
	return n
}

// HasChannels returns true if at least one notification channel is configured.
func (n *Notifier) HasChannels() bool {
	return n.webhook != nil || n.smtp != nil
}

// Send dispatches an event to all configured channels asynchronously.
// Errors are logged but do not block the caller.
func (n *Notifier) Send(event Event) {
	if n.webhook != nil {
		go func() {
			if err := n.webhook.Send(event); err != nil {
				n.logger.Error("Webhook notification failed",
					"error", err, "event_type", event.Type)
			}
		}()
	}
	if n.smtp != nil {
		go func() {
			subject := event.Summary
			body := FormatEmailBody(event)
			if err := n.smtp.Send(subject, body); err != nil {
				n.logger.Error("SMTP notification failed",
					"error", err, "event_type", event.Type)
			}
		}()
	}
}

// FormatEmailBody formats an event into a plain text email body.
func FormatEmailBody(event Event) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n", event.Summary)
	fmt.Fprintf(&b, "  Service:   %s\n", event.Service)
	fmt.Fprintf(&b, "  Event:     %s\n", event.Type)
	if event.Detail != "" {
		fmt.Fprintf(&b, "  Detail:    %s\n", event.Detail)
	}
	fmt.Fprintf(&b, "  Time:      %s\n", event.Timestamp.Format(time.RFC3339))
	if len(event.Fields) > 0 {
		fmt.Fprintf(&b, "\n")
		for k, v := range event.Fields {
			fmt.Fprintf(&b, "  %s: %s\n", k, v)
		}
	}
	fmt.Fprintf(&b, "\n---\nAutomated notification from %s.\n", event.Service)
	return b.String()
}
