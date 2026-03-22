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


package ticker

import (
	"context"
	"log/slog"
	"time"

	"github.com/QuestFinTech/taskschmiede/internal/security"
	"github.com/QuestFinTech/taskschmiede/internal/storage"
)

// Alert thresholds for security anomaly detection.
const (
	bruteForceThreshold     = 10 // login failures per IP in 5 min
	rateLimitSpikeThreshold = 50 // rate limit hits total in 5 min
	permDeniedThreshold     = 20 // permission denied events in 5 min
)

// NewAlertHandler returns a handler that checks for security anomalies.
func NewAlertHandler(db *storage.DB, logger *slog.Logger, auditSvc *security.AuditService) Handler {
	return Handler{
		Name:     "alerts",
		Interval: 5 * time.Minute,
		Fn:       alertCheck(db, logger, auditSvc),
	}
}

func alertCheck(db *storage.DB, logger *slog.Logger, auditSvc *security.AuditService) func(context.Context, time.Time) error {
	return func(_ context.Context, now time.Time) error {
		since := now.Add(-5 * time.Minute)

		// Check brute-force: >10 login failures from same IP in 5min
		loginFailsByIP, err := db.AuditCountsByIPSince(security.AuditLoginFailure, since)
		if err == nil {
			for ip, count := range loginFailsByIP {
				if count >= bruteForceThreshold {
					logger.Warn("Security alert: possible brute-force attack",
						"ip", ip,
						"login_failures", count,
						"window", "5m",
					)
					auditSvc.Log(&security.AuditEntry{
						Action:   security.AuditSecurityAlert,
						Resource: "brute_force_detected",
						IP:       ip,
						Source:   "system",
						Metadata: map[string]interface{}{
							"alert_type":     "brute_force",
							"login_failures": count,
							"window_minutes": 5,
						},
					})
				}
			}
		}

		// Aggregate counts for spike detection
		auditCounts, err := db.AuditCountsSince(since)
		if err != nil {
			return nil
		}

		// Check rate limit spike: >50 hits in 5min
		if hits := auditCounts[security.AuditRateLimitHit]; hits >= rateLimitSpikeThreshold {
			logger.Warn("Security alert: rate limit spike",
				"hits", hits,
				"window", "5m",
			)
			auditSvc.Log(&security.AuditEntry{
				Action:   security.AuditSecurityAlert,
				Resource: "rate_limit_spike",
				Source:   "system",
				Metadata: map[string]interface{}{
					"alert_type":      "rate_limit_spike",
					"rate_limit_hits": hits,
					"window_minutes":  5,
				},
			})
		}

		// Check permission denied spike: >20 in 5min
		if denied := auditCounts[security.AuditPermissionDenied]; denied >= permDeniedThreshold {
			logger.Warn("Security alert: permission denied spike",
				"count", denied,
				"window", "5m",
			)
			auditSvc.Log(&security.AuditEntry{
				Action:   security.AuditSecurityAlert,
				Resource: "permission_denied_spike",
				Source:   "system",
				Metadata: map[string]interface{}{
					"alert_type":       "permission_denied_spike",
					"permission_denied": denied,
					"window_minutes":   5,
				},
			})
		}

		return nil
	}
}
