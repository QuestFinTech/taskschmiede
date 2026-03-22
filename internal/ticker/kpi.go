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
	"os"
	"path/filepath"
	"time"

	"github.com/QuestFinTech/taskschmiede/internal/storage"
)

// NewKPIHandler returns a handler that collects system metrics and writes
// them to the database and JSON window files.
func NewKPIHandler(db *storage.DB, outputDir string) Handler {
	return Handler{
		Name:     "kpi",
		Interval: 1 * time.Minute,
		Fn:       kpiCollect(db, outputDir),
	}
}

// NewKPIHandlerWithInterval returns a KPI handler with a custom interval.
func NewKPIHandlerWithInterval(db *storage.DB, outputDir string, interval time.Duration) Handler {
	h := NewKPIHandler(db, outputDir)
	h.Interval = interval
	return h
}

// kpiCollect returns the function that collects KPI metrics.
func kpiCollect(db *storage.DB, outputDir string) func(context.Context, time.Time) error {
	return func(_ context.Context, now time.Time) error {
		data, err := collectMetrics(db, now)
		if err != nil {
			return fmt.Errorf("collect metrics: %w", err)
		}

		// Store in database
		snap := &storage.KPISnapshot{
			ID:        generateKPIID(),
			Timestamp: now.Format(time.RFC3339),
			Data:      data,
			CreatedAt: now.Format(time.RFC3339),
		}
		if err := db.InsertKPISnapshot(snap); err != nil {
			return fmt.Errorf("insert snapshot: %w", err)
		}

		// Write JSON window files
		if outputDir != "" {
			if err := writeWindowFiles(db, outputDir, now); err != nil {
				return fmt.Errorf("write window files: %w", err)
			}
		}

		return nil
	}
}

// collectMetrics gathers all KPI data from the database.
func collectMetrics(db *storage.DB, now time.Time) (map[string]interface{}, error) {
	data := make(map[string]interface{})
	data["timestamp"] = now.Format(time.RFC3339)

	// Entity counts
	stats, err := db.GetStats()
	if err != nil {
		return nil, fmt.Errorf("get stats: %w", err)
	}
	data["entities"] = map[string]int{
		"organizations": stats.Organizations,
		"users":         stats.Users,
		"endeavours":    stats.Endeavours,
		"demands":       stats.Demands,
		"tasks":         stats.Tasks,
		"resources":     stats.Resources,
		"artifacts":     stats.Artifacts,
		"rituals":       stats.Rituals,
		"ritual_runs":   stats.RitualRuns,
		"relations":     stats.Relations,
	}

	// Task status distribution
	taskCounts, _, err := db.TaskStatusCounts(storage.ListTasksOpts{})
	if err == nil {
		data["tasks"] = map[string]int{
			"planned":  taskCounts.Planned,
			"active":   taskCounts.Active,
			"done":     taskCounts.Done,
			"canceled": taskCounts.Canceled,
		}
	}

	// Demand status distribution
	demandCounts, err := db.DemandStatusCounts()
	if err == nil {
		data["demands"] = demandCounts
	}

	// User status distribution
	userCounts, err := db.UserStatusCounts()
	if err == nil {
		data["users"] = userCounts
	}

	// Security metrics (last hour)
	auditCounts, err := db.AuditCountsSince(now.Add(-1 * time.Hour))
	if err == nil {
		securityData := map[string]int{
			"login_success_1h":     auditCounts["login_success"],
			"login_failure_1h":     auditCounts["login_failure"],
			"rate_limit_hits_1h":   auditCounts["rate_limit_hit"],
			"permission_denied_1h": auditCounts["permission_denied"],
		}
		if activeCount, sessionErr := db.ActiveSessionCount(); sessionErr == nil {
			securityData["active_sessions"] = activeCount
		}
		data["security"] = securityData
	}

	return data, nil
}

// windowDef defines a JSON window file.
type windowDef struct {
	filename string
	duration time.Duration
	limit    int
}

var windows = []windowDef{
	{"1h.json", 1 * time.Hour, 60},
	{"4h.json", 4 * time.Hour, 48},
	{"1d.json", 24 * time.Hour, 96},
	{"7d.json", 7 * 24 * time.Hour, 168},
}

// writeWindowFiles writes KPI JSON files for each time window.
func writeWindowFiles(db *storage.DB, outputDir string, now time.Time) error {
	if err := os.MkdirAll(outputDir, 0750); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	// Write current.json (single object)
	latest, err := db.LatestKPISnapshot()
	if err != nil {
		return fmt.Errorf("get latest snapshot: %w", err)
	}
	if latest != nil {
		if err := writeJSONFile(filepath.Join(outputDir, "current.json"), latest.Data); err != nil {
			return fmt.Errorf("write current.json: %w", err)
		}
	}

	// Write window files (arrays of snapshots)
	for _, w := range windows {
		since := now.Add(-w.duration).Format(time.RFC3339)
		snapshots, _, err := db.ListKPISnapshots(since, "", w.limit)
		if err != nil {
			continue
		}

		// Extract just the data objects with timestamps
		var entries []map[string]interface{}
		for _, s := range snapshots {
			entry := s.Data
			if entry == nil {
				entry = make(map[string]interface{})
			}
			entry["timestamp"] = s.Timestamp
			entries = append(entries, entry)
		}

		if err := writeJSONFile(filepath.Join(outputDir, w.filename), entries); err != nil {
			continue
		}
	}

	return nil
}

// writeJSONFile writes data to a file atomically (write temp, then rename).
func writeJSONFile(path string, data interface{}) error {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0600); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename temp file: %w", err)
	}
	return nil
}

// generateKPIID creates a unique ID for a KPI snapshot.
func generateKPIID() string {
	return fmt.Sprintf("kpi_%s", storage.UTCNow().Format("20060102T150405"))
}
