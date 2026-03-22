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

// Client sends events to the notification service. Used by the app
// server to emit events without blocking the request path.
type Client struct {
	baseURL   string
	authToken string
	client    *http.Client
	logger    *slog.Logger
}

// NewClient creates a notification service client.
// If baseURL is empty, the client operates as a no-op.
func NewClient(baseURL, authToken string, logger *slog.Logger) *Client {
	return &Client{
		baseURL:   baseURL,
		authToken: authToken,
		client:    &http.Client{Timeout: 5 * time.Second},
		logger:    logger,
	}
}

// IsConfigured returns true if a notify service URL is set.
func (c *Client) IsConfigured() bool {
	return c.baseURL != ""
}

// Send posts an event to the notification service asynchronously.
// Errors are logged but never returned to the caller.
func (c *Client) Send(event *ServiceEvent) {
	if c.baseURL == "" {
		return
	}

	go func() {
		if err := c.sendSync(event); err != nil {
			c.logger.Warn("Failed to send notification event",
				"event_type", event.Type, "error", err)
		}
	}()
}

// sendSync posts the event synchronously. Used by Send in a goroutine.
func (c *Client) sendSync(event *ServiceEvent) error {
	if event.Timestamp == "" {
		event.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}

	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	url := c.baseURL + "/notify/event"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.authToken)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("post event: %w", err)
	}
	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("notify service returned status %d", resp.StatusCode)
	}

	c.logger.Debug("Notification event sent",
		"event_type", event.Type, "severity", event.Severity)
	return nil
}
