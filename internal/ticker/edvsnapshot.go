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

	"github.com/QuestFinTech/taskschmiede/internal/storage"
)

// NewEndeavourSnapshotHandler returns a handler that collects daily KPI snapshots
// for each active endeavour and computes weekly rollups.
func NewEndeavourSnapshotHandler(db *storage.DB, logger *slog.Logger) Handler {
	return Handler{
		Name:     "edv-snapshot",
		Interval: 24 * time.Hour,
		Fn:       edvSnapshotCollect(db, logger),
	}
}

func edvSnapshotCollect(db *storage.DB, logger *slog.Logger) func(context.Context, time.Time) error {
	return func(_ context.Context, now time.Time) error {
		edvs, _, err := db.ListEndeavours(storage.ListEndeavoursOpts{
			Status: "active",
			Limit:  500,
		})
		if err != nil {
			return fmt.Errorf("list active endeavours: %w", err)
		}
		if len(edvs) == 0 {
			return nil
		}

		today := now.Format("2006-01-02")
		monday := mondayOfWeek(now).Format("2006-01-02")
		collected := 0

		for _, edv := range edvs {
			metrics, mErr := collectEndeavourMetrics(db, edv.ID, now)
			if mErr != nil {
				logger.Warn("Failed to collect endeavour metrics",
					"endeavour_id", edv.ID,
					"error", mErr,
				)
				continue
			}

			// Upsert daily snapshot
			if err := db.UpsertEndeavourSnapshot(edv.ID, "daily", today, metrics); err != nil {
				logger.Warn("Failed to upsert daily snapshot",
					"endeavour_id", edv.ID,
					"error", err,
				)
				continue
			}

			// Weekly rollup: load this week's dailies, take latest metrics as base,
			// sum entity_changes_count across dailies.
			weeklyMetrics, wErr := buildWeeklyRollup(db, edv.ID, monday, metrics)
			if wErr != nil {
				logger.Warn("Failed to build weekly rollup",
					"endeavour_id", edv.ID,
					"error", wErr,
				)
			} else {
				if err := db.UpsertEndeavourSnapshot(edv.ID, "weekly", monday, weeklyMetrics); err != nil {
					logger.Warn("Failed to upsert weekly snapshot",
						"endeavour_id", edv.ID,
						"error", err,
					)
				}
			}

			collected++
		}

		if collected > 0 {
			logger.Info("Endeavour snapshots collected",
				"count", collected,
				"date", today,
			)
		}
		return nil
	}
}

// collectEndeavourMetrics gathers current KPI data for a single endeavour.
func collectEndeavourMetrics(db *storage.DB, edvID string, now time.Time) (map[string]interface{}, error) {
	metrics := make(map[string]interface{})

	// Task progress
	progress, err := db.GetEndeavourTaskProgress(edvID)
	if err == nil && progress != nil {
		metrics["tasks_planned"] = progress.Planned
		metrics["tasks_active"] = progress.Active
		metrics["tasks_done"] = progress.Done
		metrics["tasks_canceled"] = progress.Canceled
		metrics["tasks_total"] = progress.Planned + progress.Active + progress.Done + progress.Canceled
	}

	// Demand fulfillment
	demands, dErr := db.DemandFulfillmentByEndeavour(edvID)
	if dErr == nil {
		metrics["demands_total"] = len(demands)
		fulfilled := 0
		for _, d := range demands {
			if d.Status == "fulfilled" {
				fulfilled++
			}
		}
		metrics["demands_fulfilled"] = fulfilled
	}

	// Entity changes in last 24h
	since := now.Add(-24 * time.Hour)
	_, changeCount, ecErr := db.ListEntityChanges(storage.ListEntityChangesOpts{
		EndeavourID: edvID,
		StartTime:   &since,
		Limit:       1,
	})
	if ecErr == nil {
		metrics["entity_changes_count"] = changeCount
	}

	// Active contributors
	contributors, cErr := db.ContributorsByEndeavour(edvID)
	if cErr == nil {
		metrics["active_contributors"] = len(contributors)
	}

	// Overdue tasks
	overdue, oErr := db.OverdueTasks(edvID)
	if oErr == nil {
		metrics["overdue_count"] = len(overdue)
	}

	// Average cycle time
	avgCycle, acErr := db.AvgCycleTimeByEndeavour(edvID)
	if acErr == nil {
		metrics["avg_cycle_time_hours"] = avgCycle
	}

	return metrics, nil
}

// buildWeeklyRollup takes the latest daily metrics as base, loads this week's
// dailies to sum entity_changes_count, and adds days_collected.
func buildWeeklyRollup(db *storage.DB, edvID, mondayDate string, latestMetrics map[string]interface{}) (map[string]interface{}, error) {
	dailies, err := db.ListEndeavourSnapshots(edvID, "daily", mondayDate, 7)
	if err != nil {
		return nil, fmt.Errorf("list weekly dailies: %w", err)
	}

	// Start with latest metrics as base
	weekly := make(map[string]interface{})
	for k, v := range latestMetrics {
		weekly[k] = v
	}

	// Sum entity_changes_count across all dailies for the week
	totalChanges := 0
	for _, d := range dailies {
		totalChanges += metricInt(d.Metrics, "entity_changes_count")
	}
	weekly["weekly_entity_changes"] = totalChanges
	weekly["days_collected"] = len(dailies)

	return weekly, nil
}

// mondayOfWeek returns the Monday (ISO week start) of the week containing t.
func mondayOfWeek(t time.Time) time.Time {
	weekday := t.Weekday()
	if weekday == time.Sunday {
		weekday = 7
	}
	return t.AddDate(0, 0, -int(weekday-time.Monday)).Truncate(24 * time.Hour)
}

// metricInt extracts an integer from a metrics map, handling JSON float64 and int types.
func metricInt(m map[string]interface{}, key string) int {
	v, ok := m[key]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	default:
		return 0
	}
}
