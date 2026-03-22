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

import "time"

// ServiceEvent represents an event sent from the app server to the
// notification service. The payload is intentionally pre-sanitized --
// it contains summaries, not raw user content.
type ServiceEvent struct {
	Type       string   `json:"type"`                  // content_alert, content_suspension, security_anomaly, ablecon_change, harmcon_change
	Severity   string   `json:"severity"`              // low, medium, high, critical
	Summary    string   `json:"summary"`               // human-readable one-liner
	EntityType string   `json:"entity_type,omitempty"`  // task, comment, artifact, etc.
	EntityID   string   `json:"entity_id,omitempty"`    // entity identifier
	AgentID    string   `json:"agent_id,omitempty"`     // agent user ID that triggered the event
	OwnerID    string   `json:"owner_id,omitempty"`     // agent's human owner user ID
	PortalURL  string   `json:"portal_url,omitempty"`   // deep link to the relevant page
	Timestamp  string   `json:"timestamp"`              // RFC3339 UTC
	Recipients []string `json:"recipients,omitempty"`   // explicit recipient identifiers (email addresses or user IDs)
}

// Known event types.
const (
	EventContentAlert      = "content_alert"
	EventContentSuspension = "content_suspension"
	EventSecurityAnomaly   = "security_anomaly"
	EventAbleconChange     = "ablecon_change"
	EventHarmconChange     = "harmcon_change"
)

// Known severity levels.
const (
	SeverityLow      = "low"
	SeverityMedium   = "medium"
	SeverityHigh     = "high"
	SeverityCritical = "critical"
)

// IsForcedDelivery returns true if this event type bypasses rate limiting
// and must be delivered immediately.
func (e *ServiceEvent) IsForcedDelivery() bool {
	return e.Type == EventContentSuspension || e.Severity == SeverityCritical
}

// ParsedTimestamp returns the Timestamp field as time.Time.
func (e *ServiceEvent) ParsedTimestamp() time.Time {
	t, err := time.Parse(time.RFC3339, e.Timestamp)
	if err != nil {
		return time.Now().UTC()
	}
	return t
}
