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
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/QuestFinTech/taskschmiede/internal/email"
	"github.com/QuestFinTech/taskschmiede/internal/storage"
)

// EmailSender sends emails. It matches the interface used by the MCP and API layers.
type EmailSender interface {
	SendEmail(to, subject, body string) error
}

// InactivitySweepConfig holds optional dependencies for the inactivity sweep handler.
type InactivitySweepConfig struct {
	MsgDB     *storage.MessageDB // Optional: message DB for backup export.
	BackupDir string             // Optional: directory for user data backups.
}

// NewInactivitySweepHandler returns a handler that warns and deactivates
// users who have been inactive beyond the configured thresholds. The sweep
// only runs when the instance is at or above the capacity threshold
// (default: 80% of max_active_users).
func NewInactivitySweepHandler(db *storage.DB, emailSender EmailSender, logger *slog.Logger, cfg ...InactivitySweepConfig) Handler {
	var c InactivitySweepConfig
	if len(cfg) > 0 {
		c = cfg[0]
	}
	return Handler{
		Name:     "inactivity-sweep",
		Interval: 6 * time.Hour,
		Fn:       inactivitySweep(db, emailSender, logger, c),
	}
}

func inactivitySweep(db *storage.DB, emailSender EmailSender, logger *slog.Logger, cfg InactivitySweepConfig) func(context.Context, time.Time) error {
	return func(_ context.Context, now time.Time) error {
		// Read configuration from policy table.
		warnDays := policyInt(db, "inactivity.warn_days", 14)
		deactivateDays := policyInt(db, "inactivity.deactivate_days", 21)
		maxActiveUsers := policyInt(db, "instance.max_active_users", 200)
		capacityThreshold := policyFloat(db, "inactivity.sweep_capacity_threshold", 0.8)

		// Only sweep when at capacity threshold.
		activeCount := db.CountActiveUsers()
		threshold := int(float64(maxActiveUsers) * capacityThreshold)
		if activeCount < threshold {
			logger.Debug("Inactivity sweep skipped: below capacity threshold",
				"active_users", activeCount,
				"threshold", threshold,
				"max_active_users", maxActiveUsers,
			)
			return nil
		}

		logger.Info("Inactivity sweep running",
			"active_users", activeCount,
			"threshold", threshold,
			"warn_days", warnDays,
			"deactivate_days", deactivateDays,
		)

		warnThreshold := now.Add(-time.Duration(warnDays) * 24 * time.Hour)
		deactivateThreshold := now.Add(-time.Duration(deactivateDays) * 24 * time.Hour)

		// Phase 1: Deactivate users past the deactivate threshold.
		deactivateUsers, err := db.ListInactiveUsersForDeactivation(deactivateThreshold)
		if err != nil {
			logger.Error("Failed to list users for deactivation", "error", err)
		} else {
			for _, user := range deactivateUsers {
				// Export user data before deactivation.
				if cfg.BackupDir != "" {
					if bErr := exportUserBackup(db, cfg.MsgDB, user.ID, cfg.BackupDir, logger); bErr != nil {
						logger.Error("Failed to backup user before deactivation",
							"user_id", user.ID, "email", user.Email, "error", bErr)
					}
				}

				if err := db.DeactivateUser(user.ID); err != nil {
					logger.Error("Failed to deactivate user",
						"user_id", user.ID, "email", user.Email, "error", err)
					continue
				}
				logger.Info("User deactivated for inactivity",
					"user_id", user.ID,
					"email", user.Email,
					"last_active_at", user.LastActiveAt,
				)

				// Send deactivation notification email.
				if emailSender != nil && user.Email != "" {
					subject := "Taskschmiede: Account deactivated"
					if svc, ok := emailSender.(*email.Service); ok {
						data := &email.InactivityData{
							Greeting: fmt.Sprintf("Hello %s", user.Name),
							Message:  fmt.Sprintf("Your Taskschmiede account (%s) has been deactivated due to inactivity. Your last API activity was more than %d days ago.", user.Email, deactivateDays),
							Advice:   "Your data has been preserved. You can reactivate your account by logging in at any time, provided the instance has available capacity. If you believe this is an error, please contact the instance administrator.",
							Closing:  "Best regards,",
							TeamName: "Team Taskschmiede",
						}
						if err := svc.SendInactivityDeactivation(user.Email, subject, data); err != nil {
							logger.Error("Failed to send deactivation email",
								"user_id", user.ID, "email", user.Email, "error", err)
						}
					} else {
						body := fmt.Sprintf(
							"Hello %s,\n\n"+
								"Your Taskschmiede account (%s) has been deactivated due to inactivity.\n"+
								"Your last API activity was more than %d days ago.\n\n"+
								"Your data has been preserved. You can reactivate your account by logging in "+
								"at any time, provided the instance has available capacity.\n\n"+
								"If you believe this is an error, please contact the instance administrator.\n\n"+
								"-- Taskschmiede",
							user.Name, user.Email, deactivateDays,
						)
						if err := emailSender.SendEmail(user.Email, subject, body); err != nil {
							logger.Error("Failed to send deactivation email",
								"user_id", user.ID, "email", user.Email, "error", err)
						}
					}
				}
			}
			if len(deactivateUsers) > 0 {
				logger.Info("Inactivity deactivation complete", "count", len(deactivateUsers))
			}
		}

		// Phase 2: Warn users in the warn window (between warn and deactivate thresholds).
		warnUsers, err := db.ListInactiveUsersForWarning(warnThreshold, deactivateThreshold)
		if err != nil {
			logger.Error("Failed to list users for warning", "error", err)
		} else {
			for _, user := range warnUsers {
				daysUntilDeactivation := deactivateDays - warnDays

				// Send warning email.
				if emailSender != nil && user.Email != "" {
					subject := "Taskschmiede: Inactivity warning"
					if svc, ok := emailSender.(*email.Service); ok {
						data := &email.InactivityData{
							Greeting: fmt.Sprintf("Hello %s", user.Name),
							Message:  fmt.Sprintf("Your Taskschmiede account (%s) has been inactive for %d days. It will be deactivated in %d days unless you log in.", user.Email, warnDays, daysUntilDeactivation),
							Advice:   "To keep your account active, simply log in or make any API call.",
							Closing:  "Best regards,",
							TeamName: "Team Taskschmiede",
						}
						if err := svc.SendInactivityWarning(user.Email, subject, data); err != nil {
							logger.Error("Failed to send inactivity warning email",
								"user_id", user.ID, "email", user.Email, "error", err)
							continue
						}
					} else {
						body := fmt.Sprintf(
							"Hello %s,\n\n"+
								"Your Taskschmiede account (%s) has been inactive for %d days.\n"+
								"It will be deactivated in %d days unless you log in.\n\n"+
								"To keep your account active, simply log in or make any API call.\n\n"+
								"-- Taskschmiede",
							user.Name, user.Email, warnDays, daysUntilDeactivation,
						)
						if err := emailSender.SendEmail(user.Email, subject, body); err != nil {
							logger.Error("Failed to send inactivity warning email",
								"user_id", user.ID, "email", user.Email, "error", err)
							continue
						}
					}
				}

				// Mark as warned so we don't re-send.
				if err := db.SetUserInactivityWarned(user.ID, now); err != nil {
					logger.Error("Failed to mark user as warned",
						"user_id", user.ID, "error", err)
					continue
				}

				logger.Info("Inactivity warning sent",
					"user_id", user.ID,
					"email", user.Email,
					"last_active_at", user.LastActiveAt,
					"deactivation_in_days", daysUntilDeactivation,
				)
			}
			if len(warnUsers) > 0 {
				logger.Info("Inactivity warnings complete", "count", len(warnUsers))
			}
		}

		return nil
	}
}

// policyInt reads an integer policy value with a fallback default.
func policyInt(db *storage.DB, key string, defaultVal int) int {
	val, err := db.GetPolicy(key)
	if err != nil {
		return defaultVal
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		return defaultVal
	}
	return n
}

// exportUserBackup exports a user's data to a JSON file in the backup directory.
func exportUserBackup(db *storage.DB, msgDB *storage.MessageDB, userID, backupDir string, logger *slog.Logger) error {
	backup, err := db.ExportUserData(userID)
	if err != nil {
		return fmt.Errorf("export user data: %w", err)
	}

	// Add messages if MessageDB is available.
	if msgDB != nil && backup.Resource != nil {
		msgs, dels := msgDB.ExportUserMessages(backup.Resource.ID)
		if msgs != nil {
			backup.Messages = msgs
		}
		if dels != nil {
			backup.Deliveries = dels
		}
	}

	if err := os.MkdirAll(backupDir, 0700); err != nil {
		return fmt.Errorf("create backup dir: %w", err)
	}

	filename := fmt.Sprintf("%s_%s.json", storage.UTCNow().Format("20060102-150405"), userID)
	path := filepath.Join(backupDir, filename)

	data, err := json.MarshalIndent(backup, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal backup: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write backup file: %w", err)
	}

	logger.Info("User data backed up",
		"user_id", userID,
		"path", path,
		"size_bytes", len(data),
	)
	return nil
}

// policyFloat reads a float64 policy value with a fallback default.
func policyFloat(db *storage.DB, key string, defaultVal float64) float64 {
	val, err := db.GetPolicy(key)
	if err != nil {
		return defaultVal
	}
	f, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return defaultVal
	}
	return f
}
