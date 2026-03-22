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

	"github.com/QuestFinTech/taskschmiede/internal/storage"
)

// NewPurgeHandler returns a handler that deletes old audit_log and entity_change
// records beyond configurable retention periods. Retention is read from the policy
// table on each run (no restart needed to change thresholds):
//   - purge.audit_log_days (default: 90)
//   - purge.entity_change_days (default: 180)
func NewPurgeHandler(db *storage.DB, logger *slog.Logger) Handler {
	return Handler{
		Name:     "data-purge",
		Interval: 24 * time.Hour,
		Fn:       dataPurge(db, logger),
	}
}

func dataPurge(db *storage.DB, logger *slog.Logger) func(context.Context, time.Time) error {
	return func(_ context.Context, now time.Time) error {
		auditDays := policyInt(db, "purge.audit_log_days", 90)
		entityDays := policyInt(db, "purge.entity_change_days", 180)

		auditCutoff := now.Add(-time.Duration(auditDays) * 24 * time.Hour)
		entityCutoff := now.Add(-time.Duration(entityDays) * 24 * time.Hour)

		auditDeleted, err := db.PurgeAuditLog(auditCutoff)
		if err != nil {
			logger.Warn("Failed to purge audit_log", "error", err)
		} else if auditDeleted > 0 {
			logger.Info("Purged old audit_log entries",
				"deleted", auditDeleted,
				"cutoff", auditCutoff.Format(time.RFC3339),
				"retention_days", auditDays,
			)
		}

		entityDeleted, err := db.PurgeEntityChanges(entityCutoff)
		if err != nil {
			logger.Warn("Failed to purge entity_change", "error", err)
		} else if entityDeleted > 0 {
			logger.Info("Purged old entity_change entries",
				"deleted", entityDeleted,
				"cutoff", entityCutoff.Format(time.RFC3339),
				"retention_days", entityDays,
			)
		}

		return nil
	}
}
