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


package notify

import (
	"fmt"
	"log/slog"
	"strings"
)

// Dispatcher receives ServiceEvents, applies rate limiting, dispatches
// to configured channels, and logs delivery results.
type Dispatcher struct {
	logger  *slog.Logger
	log     *DeliveryLog
	limiter *RateLimiter
	smtp    *SMTPSender
	ntfy    *NtfySender
	webhook *WebhookSender
}

// DispatcherConfig holds the full notification service configuration.
type DispatcherConfig struct {
	SMTP       *SMTPConfig       `yaml:"smtp"`
	Ntfy       *NtfyConfig       `yaml:"ntfy"`
	Webhook    *WebhookConfig    `yaml:"webhook"`
	RateLimits *RateLimitConfig  `yaml:"rate-limits"`
}

// NewDispatcher creates a dispatcher with the configured channels.
// Channels with missing configuration are silently skipped.
func NewDispatcher(logger *slog.Logger, dlog *DeliveryLog, cfg DispatcherConfig) *Dispatcher {
	d := &Dispatcher{
		logger: logger,
		log:    dlog,
	}

	rlCfg := DefaultRateLimitConfig()
	if cfg.RateLimits != nil {
		rlCfg = *cfg.RateLimits
	}
	d.limiter = NewRateLimiter(rlCfg)

	if cfg.SMTP != nil && cfg.SMTP.Host != "" && len(cfg.SMTP.To) > 0 {
		d.smtp = newSMTPSender(logger, *cfg.SMTP)
		logger.Info("Notification channel configured", "channel", "smtp",
			"host", cfg.SMTP.Host, "to", strings.Join(cfg.SMTP.To, ","))
	}

	if cfg.Ntfy != nil && cfg.Ntfy.URL != "" {
		d.ntfy = newNtfySender(logger, *cfg.Ntfy)
		logger.Info("Notification channel configured", "channel", "ntfy", "url", cfg.Ntfy.URL)
	}

	if cfg.Webhook != nil && cfg.Webhook.URL != "" {
		d.webhook = newWebhookSender(logger, *cfg.Webhook)
		logger.Info("Notification channel configured", "channel", "webhook", "url", cfg.Webhook.URL)
	}

	return d
}

// HasChannels returns true if at least one dispatch channel is configured.
func (d *Dispatcher) HasChannels() bool {
	return d.smtp != nil || d.ntfy != nil || d.webhook != nil
}

// Dispatch processes a ServiceEvent: records it, checks rate limits,
// sends to all configured channels, and logs delivery outcomes.
func (d *Dispatcher) Dispatch(event *ServiceEvent) {
	eventID, err := d.log.RecordEvent(event)
	if err != nil {
		d.logger.Error("Failed to record event", "error", err, "event_type", event.Type)
		return
	}

	forced := event.IsForcedDelivery()

	d.dispatchSMTP(event, eventID, forced)
	d.dispatchNtfy(event, eventID, forced)
	d.dispatchWebhook(event, eventID, forced)
}

// dispatchSMTP sends the event via SMTP to all applicable recipients,
// respecting rate limits unless forced delivery is required.
func (d *Dispatcher) dispatchSMTP(event *ServiceEvent, eventID int64, forced bool) {
	if d.smtp == nil {
		return
	}

	for _, recipient := range d.emailRecipients(event) {
		if !forced && !d.limiter.AllowEmail(recipient) {
			d.logger.Debug("Email rate limited", "recipient", recipient, "event_type", event.Type)
			_, _ = d.log.RecordDelivery(eventID, "smtp", recipient, "rate_limited", "")
			continue
		}

		subject := fmt.Sprintf("[Taskschmiede] %s", ntfyTitle(event))
		body := formatServiceEventEmail(event)
		err := d.smtp.Send(subject, body)
		if err != nil {
			d.logger.Warn("SMTP delivery failed",
				"recipient", recipient, "event_type", event.Type, "error", err)
			_, _ = d.log.RecordDelivery(eventID, "smtp", recipient, "failed", err.Error())
		} else {
			_, _ = d.log.RecordDelivery(eventID, "smtp", recipient, "delivered", "")
		}
	}
}

// dispatchNtfy sends the event as a push notification via ntfy.
func (d *Dispatcher) dispatchNtfy(event *ServiceEvent, eventID int64, forced bool) {
	if d.ntfy == nil {
		return
	}

	if !forced && !d.limiter.AllowNtfy() {
		d.logger.Debug("Ntfy rate limited", "event_type", event.Type)
		_, _ = d.log.RecordDelivery(eventID, "ntfy", "", "rate_limited", "")
		return
	}

	err := d.ntfy.SendEvent(event)
	if err != nil {
		d.logger.Warn("Ntfy delivery failed", "event_type", event.Type, "error", err)
		_, _ = d.log.RecordDelivery(eventID, "ntfy", "", "failed", err.Error())
	} else {
		_, _ = d.log.RecordDelivery(eventID, "ntfy", "", "delivered", "")
	}
}

// dispatchWebhook sends the event to the configured webhook endpoint.
func (d *Dispatcher) dispatchWebhook(event *ServiceEvent, eventID int64, forced bool) {
	if d.webhook == nil {
		return
	}

	if !forced && !d.limiter.AllowWebhook(d.webhook.cfg.URL) {
		d.logger.Debug("Webhook rate limited", "event_type", event.Type)
		_, _ = d.log.RecordDelivery(eventID, "webhook", d.webhook.cfg.URL, "rate_limited", "")
		return
	}

	// Convert ServiceEvent to the generic Event for the existing webhook sender.
	genericEvent := Event{
		Service:   "taskschmiede",
		Type:      event.Type,
		Summary:   event.Summary,
		Timestamp: event.ParsedTimestamp(),
		Fields: map[string]string{
			"severity":    event.Severity,
			"entity_type": event.EntityType,
			"entity_id":   event.EntityID,
			"agent_id":    event.AgentID,
			"owner_id":    event.OwnerID,
			"portal_url":  event.PortalURL,
		},
	}

	err := d.webhook.Send(genericEvent)
	if err != nil {
		d.logger.Warn("Webhook delivery failed", "event_type", event.Type, "error", err)
		_, _ = d.log.RecordDelivery(eventID, "webhook", d.webhook.cfg.URL, "failed", err.Error())
	} else {
		_, _ = d.log.RecordDelivery(eventID, "webhook", d.webhook.cfg.URL, "delivered", "")
	}
}

// emailRecipients returns the list of email recipients for an event.
// If the event has explicit recipients, those are used. Otherwise
// the configured SMTP To list is used.
func (d *Dispatcher) emailRecipients(event *ServiceEvent) []string {
	if len(event.Recipients) > 0 {
		return event.Recipients
	}
	if d.smtp != nil {
		return d.smtp.cfg.To
	}
	return nil
}

// Cleanup prunes expired rate limiter entries.
func (d *Dispatcher) Cleanup() {
	d.limiter.Cleanup()
}

// formatServiceEventEmail formats a ServiceEvent into a plain text email body
// suitable for SMTP notification delivery.
func formatServiceEventEmail(event *ServiceEvent) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n", event.Summary)
	fmt.Fprintf(&b, "  Severity:  %s\n", event.Severity)
	fmt.Fprintf(&b, "  Type:      %s\n", event.Type)
	if event.EntityType != "" {
		fmt.Fprintf(&b, "  Entity:    %s/%s\n", event.EntityType, event.EntityID)
	}
	if event.AgentID != "" {
		fmt.Fprintf(&b, "  Agent:     %s\n", event.AgentID)
	}
	if event.OwnerID != "" {
		fmt.Fprintf(&b, "  Owner:     %s\n", event.OwnerID)
	}
	fmt.Fprintf(&b, "  Time:      %s\n", event.Timestamp)
	if event.PortalURL != "" {
		fmt.Fprintf(&b, "\nReview: %s\n", event.PortalURL)
	}
	fmt.Fprintf(&b, "\n---\nAutomated notification from Taskschmiede.\n")
	return b.String()
}
