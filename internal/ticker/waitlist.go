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
	"fmt"
	"log/slog"
	"time"

	"github.com/QuestFinTech/taskschmiede/internal/email"
	"github.com/QuestFinTech/taskschmiede/internal/storage"
)

// NewWaitlistHandler returns a handler that processes the registration waitlist.
// It runs every 30 minutes and:
// 1. Expires overdue notifications (entries that were notified but didn't register in time).
// 2. Checks for available capacity and promotes waiting entries to real accounts.
// 3. Sends notification emails to promoted users.
func NewWaitlistHandler(db *storage.DB, emailSender EmailSender, logger *slog.Logger) Handler {
	return Handler{
		Name:     "waitlist-processor",
		Interval: 30 * time.Minute,
		Fn:       processWaitlist(db, emailSender, logger),
	}
}

func processWaitlist(db *storage.DB, emailSender EmailSender, logger *slog.Logger) func(context.Context, time.Time) error {
	return func(_ context.Context, _ time.Time) error {
		// Phase 1: Expire overdue notifications.
		expired, err := db.ExpireWaitlistNotifications()
		if err != nil {
			logger.Error("Waitlist: failed to expire notifications", "error", err)
		} else if expired > 0 {
			logger.Info("Waitlist: expired overdue notifications", "count", expired)
		}

		// Phase 2: Check capacity and promote waiting entries.
		maxActiveUsers := policyInt(db, "instance.max_active_users", 200)
		if maxActiveUsers <= 0 {
			return nil // no capacity limit configured
		}

		activeCount := db.CountActiveUsers()
		available := maxActiveUsers - activeCount
		if available <= 0 {
			return nil
		}

		notificationDays := policyInt(db, "waitlist.notification_window_days", 7)

		entries, err := db.PopWaitlist(available, notificationDays)
		if err != nil {
			return fmt.Errorf("waitlist pop: %w", err)
		}

		if len(entries) == 0 {
			return nil
		}

		logger.Info("Waitlist: promoting entries",
			"count", len(entries),
			"available_slots", available,
		)

		for _, entry := range entries {
			// Create the user account from waitlist data.
			user, token, createErr := db.CreateUserWithInvitation(
				entry.Email, entry.Name, entry.PasswordHash,
				entry.InvitationTokenID, entry.UserType, "", nil,
			)
			if createErr != nil {
				logger.Error("Waitlist: failed to create user",
					"email", entry.Email, "waitlist_id", entry.ID, "error", createErr)
				continue
			}

			if err := db.MarkWaitlistCreated(entry.ID); err != nil {
				logger.Error("Waitlist: failed to mark entry as created",
					"waitlist_id", entry.ID, "error", err)
			}

			logger.Info("Waitlist: user account created",
				"user_id", user.ID,
				"email", entry.Email,
				"token_prefix", token[:8]+"...",
				"waitlist_id", entry.ID,
			)

			// Send welcome email with their login token.
			if emailSender != nil && entry.Email != "" {
				subject := "Taskschmiede: Your account is ready"
				if svc, ok := emailSender.(*email.Service); ok {
					data := &email.WaitlistWelcomeData{
						Greeting:  fmt.Sprintf("Hello %s", entry.Name),
						Message:   "Good news -- a slot has opened up on Taskschmiede and your account is now active.",
						Token:     token,
						TokenNote: "This token will not be shown again. Store it securely.",
						NextSteps: "You can use this token to authenticate with the Taskschmiede API. If you are an agent, call ts.onboard.start_interview to begin the onboarding process.",
						Closing:   "Best regards,",
						TeamName:  "Team Taskschmiede",
					}
					if err := svc.SendWaitlistWelcome(entry.Email, subject, data); err != nil {
						logger.Error("Waitlist: failed to send welcome email",
							"email", entry.Email, "error", err)
					}
				} else {
					body := fmt.Sprintf(
						"Hello %s,\n\n"+
							"Good news -- a slot has opened up on Taskschmiede and your account is now active.\n\n"+
							"Your API token: %s\n\n"+
							"You can use this token to authenticate with the Taskschmiede API.\n"+
							"If you are an agent, call ts.onboard.start_interview to begin the onboarding process.\n\n"+
							"This token will not be shown again. Store it securely.\n\n"+
							"-- Taskschmiede",
						entry.Name, token,
					)
					if err := emailSender.SendEmail(entry.Email, subject, body); err != nil {
						logger.Error("Waitlist: failed to send welcome email",
							"email", entry.Email, "error", err)
					}
				}
			}
		}

		return nil
	}
}
