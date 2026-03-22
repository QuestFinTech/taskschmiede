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
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// WebhookConfig holds configuration for a generic webhook endpoint.
// The payload format is compatible with Slack, Discord, Mattermost, Teams,
// Ntfy, Gotify, and any endpoint that accepts JSON POST requests.
type WebhookConfig struct {
	URL     string            `yaml:"url"`
	Headers map[string]string `yaml:"headers"` // e.g. {"Authorization": "Bearer xxx"}
}

// WebhookSender sends notifications via HTTP POST to a webhook URL.
type WebhookSender struct {
	cfg    WebhookConfig
	client *http.Client
	logger *slog.Logger
}

// newWebhookSender creates a WebhookSender with the given configuration and a default HTTP client.
func newWebhookSender(logger *slog.Logger, cfg WebhookConfig) *WebhookSender {
	return &WebhookSender{
		cfg:    cfg,
		client: &http.Client{Timeout: 10 * time.Second},
		logger: logger,
	}
}

// Send posts an event as JSON to the configured webhook URL.
// The payload includes a "text" field for Slack/Mattermost compatibility
// plus structured fields for custom receivers.
func (w *WebhookSender) Send(event Event) error {
	payload := map[string]interface{}{
		"text":       event.Summary,
		"service":    event.Service,
		"event_type": event.Type,
		"detail":     event.Detail,
		"timestamp":  event.Timestamp.Format(time.RFC3339),
	}
	for k, v := range event.Fields {
		payload[k] = v
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal webhook payload: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.cfg.URL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range w.cfg.Headers {
		req.Header.Set(k, v)
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("send webhook: %w", err)
	}
	_ = resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	w.logger.Debug("Webhook notification sent", "url", w.cfg.URL, "status", resp.StatusCode)
	return nil
}
