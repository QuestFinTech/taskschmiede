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
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// NtfyConfig holds configuration for ntfy.sh push notifications.
type NtfyConfig struct {
	URL             string `yaml:"url"`              // e.g. https://ntfy.example.com/taskschmiede
	Token           string `yaml:"token"`            // access token (optional)
	DefaultPriority int    `yaml:"default-priority"` // 1-5 (default 3)
}

// NtfySender sends push notifications via ntfy.sh using the HTTP publish API.
// ntfy uses custom headers rather than JSON body, making it distinct from
// a generic webhook.
type NtfySender struct {
	cfg    NtfyConfig
	client *http.Client
	logger *slog.Logger
}

// newNtfySender creates an NtfySender with the given configuration, clamping the
// default priority to the valid 1-5 range.
func newNtfySender(logger *slog.Logger, cfg NtfyConfig) *NtfySender {
	if cfg.DefaultPriority < 1 || cfg.DefaultPriority > 5 {
		cfg.DefaultPriority = 3
	}
	return &NtfySender{
		cfg:    cfg,
		client: &http.Client{Timeout: 10 * time.Second},
		logger: logger,
	}
}

// SendEvent publishes a ServiceEvent to the configured ntfy topic.
func (n *NtfySender) SendEvent(event *ServiceEvent) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.cfg.URL,
		bytes.NewBufferString(event.Summary))
	if err != nil {
		return fmt.Errorf("create ntfy request: %w", err)
	}

	// Title: event type in human-readable form.
	req.Header.Set("Title", ntfyTitle(event))
	req.Header.Set("Priority", fmt.Sprintf("%d", ntfyPriority(event, n.cfg.DefaultPriority)))
	req.Header.Set("Tags", ntfyTags(event))

	if event.PortalURL != "" {
		req.Header.Set("Click", event.PortalURL)
	}

	if n.cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+n.cfg.Token)
	}

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("send ntfy: %w", err)
	}
	_ = resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("ntfy returned status %d", resp.StatusCode)
	}

	n.logger.Debug("Ntfy notification sent",
		"url", n.cfg.URL, "status", resp.StatusCode, "event_type", event.Type)
	return nil
}

// ntfyTitle returns a short title for the notification.
func ntfyTitle(event *ServiceEvent) string {
	switch event.Type {
	case EventContentAlert:
		return "Content Alert"
	case EventContentSuspension:
		return "Agent Suspended"
	case EventSecurityAnomaly:
		return "Security Anomaly"
	case EventAbleconChange:
		return "Ablecon Level Change"
	case EventHarmconChange:
		return "Harmcon Level Change"
	default:
		return "Taskschmiede Alert"
	}
}

// ntfyPriority maps severity to ntfy's 1-5 priority scale.
func ntfyPriority(event *ServiceEvent, defaultPriority int) int {
	switch event.Severity {
	case SeverityCritical:
		return 5 // max/urgent
	case SeverityHigh:
		return 4 // high
	case SeverityMedium:
		return 3 // default
	case SeverityLow:
		return 2 // low
	default:
		return defaultPriority
	}
}

// ntfyTags returns comma-separated emoji tags for the notification.
func ntfyTags(event *ServiceEvent) string {
	switch event.Severity {
	case SeverityCritical:
		return "rotating_light"
	case SeverityHigh:
		return "warning"
	case SeverityMedium:
		return "orange_circle"
	default:
		return "information_source"
	}
}
