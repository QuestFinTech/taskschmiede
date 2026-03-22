// Copyright 2026 Quest Financial Technologies S.a r.l.-S., Luxembourg
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
	"strconv"
	"strings"
	"time"

	"github.com/QuestFinTech/taskschmiede/internal/llmclient"
	"github.com/QuestFinTech/taskschmiede/internal/service"
	"github.com/QuestFinTech/taskschmiede/internal/storage"
)

// NewRitualExecutorHandler returns a ticker handler that evaluates scheduled
// rituals, gathers endeavour context, and sends the ritual prompt to an LLM.
// Phase A: read-only -- the LLM response is stored as a ritual run report.
func NewRitualExecutorHandler(db *storage.DB, client llmclient.Client, msgSvc *service.MessageService, logger *slog.Logger, interval time.Duration) Handler {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	return Handler{
		Name:     "ritual-executor",
		Interval: interval,
		Fn:       ritualExecutorCheck(db, client, msgSvc, logger),
	}
}

func ritualExecutorCheck(db *storage.DB, client llmclient.Client, msgSvc *service.MessageService, logger *slog.Logger) func(context.Context, time.Time) error {
	return func(ctx context.Context, now time.Time) error {
		rituals, err := db.ListScheduledRituals()
		if err != nil {
			return fmt.Errorf("ritual-executor: list scheduled rituals: %w", err)
		}
		for _, ritual := range rituals {
			processRitual(ctx, db, client, msgSvc, logger, ritual, now)
		}
		return nil
	}
}

func processRitual(ctx context.Context, db *storage.DB, client llmclient.Client, msgSvc *service.MessageService, logger *slog.Logger, ritual *storage.Ritual, now time.Time) {
	edv, since, ok := checkRitualPrereqs(db, logger, ritual, now)
	if !ok {
		return
	}

	resp, run, ok := executeRitualLLM(ctx, db, client, logger, ritual, edv, since)
	if !ok {
		return
	}

	sendRitualReport(ctx, msgSvc, logger, ritual, edv, run, resp)
}

// checkRitualPrereqs validates that the ritual should execute: endeavour is active,
// schedule is due, and the endeavour has changed since the last run.
// Returns the endeavour, the baseline time for change detection, and whether to proceed.
func checkRitualPrereqs(db *storage.DB, logger *slog.Logger, ritual *storage.Ritual, now time.Time) (*storage.Endeavour, time.Time, bool) {
	edv, err := db.GetEndeavour(ritual.EndeavourID)
	if err != nil {
		logger.Debug("ritual-executor: endeavour not found, skipping",
			"ritual", ritual.ID, "endeavour", ritual.EndeavourID)
		return nil, time.Time{}, false
	}
	if edv.Status != "active" && edv.Status != "pending" {
		return nil, time.Time{}, false
	}

	lastRun, err := db.GetLastRitualRun(ritual.ID)
	if err != nil {
		logger.Warn("ritual-executor: failed to get last run", "ritual", ritual.ID, "error", err)
		return nil, time.Time{}, false
	}

	if lastRun != nil && lastRun.Status == "running" {
		return nil, time.Time{}, false
	}

	if !isRitualDue(ritual.Schedule, lastRun, now) {
		return nil, time.Time{}, false
	}

	var since time.Time
	if lastRun != nil && lastRun.FinishedAt != nil {
		since = *lastRun.FinishedAt
	} else if lastRun != nil {
		since = lastRun.CreatedAt
	}

	if !since.IsZero() {
		changed, err := db.HasEndeavourChangedSince(ritual.EndeavourID, since)
		if err != nil {
			logger.Warn("ritual-executor: change detection failed", "ritual", ritual.ID, "error", err)
			return nil, time.Time{}, false
		}
		if !changed {
			run, err := db.CreateRitualRun(ritual.ID, "schedule", "sys_taskschmied", nil)
			if err == nil {
				skipped := "skipped"
				summary := "No changes in endeavour since last run"
				_, _ = db.UpdateRitualRun(run.ID, storage.UpdateRitualRunFields{
					Status:        &skipped,
					ResultSummary: &summary,
				})
			}
			logger.Debug("ritual-executor: skipped, no changes",
				"ritual", ritual.ID, "endeavour", ritual.EndeavourID)
			return nil, time.Time{}, false
		}
	}

	return edv, since, true
}

// executeRitualLLM gathers endeavour context, calls the LLM, and records the run.
func executeRitualLLM(ctx context.Context, db *storage.DB, client llmclient.Client, logger *slog.Logger, ritual *storage.Ritual, edv *storage.Endeavour, since time.Time) (*llmclient.Response, *storage.RitualRun, bool) {
	export, err := db.ExportEndeavourData(ritual.EndeavourID)
	if err != nil {
		logger.Warn("ritual-executor: failed to export endeavour data",
			"ritual", ritual.ID, "endeavour", ritual.EndeavourID, "error", err)
		return nil, nil, false
	}

	contextSummary := buildEndeavourContext(export, since)

	systemPrompt := fmt.Sprintf(
		"You are Taskschmied, the governance agent for Taskschmiede.\n"+
			"Execute the following ritual and produce a structured report.\n"+
			"Respond in %s. Do not take any actions -- report only.",
		ritual.Lang,
	)
	userPrompt := fmt.Sprintf("## Ritual: %s\n\n%s\n\n## Current State\n\n%s",
		ritual.Name, ritual.Prompt, contextSummary)

	run, err := db.CreateRitualRun(ritual.ID, "schedule", "sys_taskschmied", nil)
	if err != nil {
		logger.Warn("ritual-executor: failed to create run", "ritual", ritual.ID, "error", err)
		return nil, nil, false
	}

	req := &llmclient.Request{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		MaxTokens:    2048,
	}
	callStart := storage.UTCNow()
	resp, llmErr := client.Complete(ctx, req)
	callDuration := storage.UTCNow().Sub(callStart)

	if llmErr != nil {
		failed := "failed"
		errMsg := llmErr.Error()
		meta := map[string]interface{}{
			"client":      clientLabel(client),
			"duration_ms": callDuration.Milliseconds(),
		}
		_, _ = db.UpdateRitualRun(run.ID, storage.UpdateRitualRunFields{
			Status:        &failed,
			ResultSummary: &errMsg,
			Metadata:      meta,
		})
		logger.Warn("ritual-executor: LLM call failed",
			"ritual", ritual.ID, "run", run.ID, "error", llmErr)
		return nil, nil, false
	}

	usedClient := resp.UsedProvider + "/" + resp.UsedModel
	succeeded := "succeeded"
	meta := map[string]interface{}{
		"client":      usedClient,
		"duration_ms": callDuration.Milliseconds(),
	}
	if resp.TotalTokens > 0 {
		meta["total_tokens"] = resp.TotalTokens
	}
	if resp.PredictedMs > 0 {
		meta["predicted_ms"] = resp.PredictedMs
	}
	_, _ = db.UpdateRitualRun(run.ID, storage.UpdateRitualRunFields{
		Status:        &succeeded,
		ResultSummary: &resp.Content,
		Metadata:      meta,
	})
	logger.Info("ritual-executor: completed",
		"ritual", ritual.ID, "run", run.ID, "client", usedClient,
		"endeavour", ritual.EndeavourID, "response_len", len(resp.Content),
		"duration_ms", callDuration.Milliseconds())

	return resp, run, true
}

// sendRitualReport sends the LLM report as an internal message to all endeavour members.
func sendRitualReport(ctx context.Context, msgSvc *service.MessageService, logger *slog.Logger, ritual *storage.Ritual, edv *storage.Endeavour, run *storage.RitualRun, resp *llmclient.Response) {
	if msgSvc == nil || resp.Content == "" {
		return
	}

	subject := fmt.Sprintf("Ritual: %s -- %s", ritual.Name, edv.Name)
	msgMeta := map[string]interface{}{
		"ritual_id": ritual.ID,
		"run_id":    run.ID,
	}
	_, msgErr := msgSvc.Send(ctx,
		"sys_taskschmiede",
		subject,
		resp.Content,
		"info",
		"",
		"ritual_run", run.ID,
		nil,
		"endeavour", ritual.EndeavourID,
		msgMeta,
	)
	if msgErr != nil {
		logger.Warn("ritual-executor: failed to send message",
			"ritual", ritual.ID, "run", run.ID, "error", msgErr)
	} else {
		logger.Info("ritual-executor: message sent",
			"ritual", ritual.ID, "run", run.ID, "endeavour", ritual.EndeavourID)
	}
}

// isRitualDue checks whether a ritual should be executed based on its schedule
// and the last run. Manual rituals are never due via the ticker.
func isRitualDue(schedule map[string]interface{}, lastRun *storage.RitualRun, now time.Time) bool {
	schedType, _ := schedule["type"].(string)
	switch schedType {
	case "interval":
		every, _ := schedule["every"].(string)
		d, err := parseIntervalDuration(every)
		if err != nil || d <= 0 {
			return false
		}
		if lastRun == nil {
			return true // first run
		}
		var baseline time.Time
		if lastRun.FinishedAt != nil {
			baseline = *lastRun.FinishedAt
		} else {
			baseline = lastRun.CreatedAt
		}
		return now.Sub(baseline) >= d
	case "cron":
		expr, _ := schedule["expression"].(string)
		if expr == "" {
			return false
		}
		if !cronMatchesNow(expr, now) {
			return false
		}
		// Ensure we don't run twice in the same minute.
		if lastRun != nil {
			var baseline time.Time
			if lastRun.FinishedAt != nil {
				baseline = *lastRun.FinishedAt
			} else {
				baseline = lastRun.CreatedAt
			}
			if baseline.Truncate(time.Minute).Equal(now.Truncate(time.Minute)) {
				return false
			}
		}
		return true
	default:
		return false // manual or unknown
	}
}

// parseIntervalDuration parses interval strings like "30m", "2h", "1d", "2w", "13w".
func parseIntervalDuration(s string) (time.Duration, error) {
	if s == "" {
		return 0, fmt.Errorf("empty interval")
	}
	unit := s[len(s)-1]
	numStr := s[:len(s)-1]
	num, err := strconv.Atoi(numStr)
	if err != nil {
		return 0, fmt.Errorf("invalid interval number: %s", s)
	}
	switch unit {
	case 'm':
		return time.Duration(num) * time.Minute, nil
	case 'h':
		return time.Duration(num) * time.Hour, nil
	case 'd':
		return time.Duration(num) * 24 * time.Hour, nil
	case 'w':
		return time.Duration(num) * 7 * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("unknown interval unit: %c", unit)
	}
}

// cronMatchesNow checks whether the given 5-field cron expression matches the
// provided time. Supports: *, */N, N, N-M, N,M patterns.
func cronMatchesNow(expr string, now time.Time) bool {
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return false
	}
	minute := now.Minute()
	hour := now.Hour()
	dom := now.Day()
	month := int(now.Month())
	dow := int(now.Weekday()) // 0=Sunday

	return cronFieldMatches(fields[0], minute, 0, 59) &&
		cronFieldMatches(fields[1], hour, 0, 23) &&
		cronFieldMatches(fields[2], dom, 1, 31) &&
		cronFieldMatches(fields[3], month, 1, 12) &&
		cronFieldMatches(fields[4], dow, 0, 6)
}

// cronFieldMatches checks whether a single cron field matches a value.
func cronFieldMatches(field string, value, min, max int) bool {
	if field == "*" {
		return true
	}
	// Handle comma-separated values: "1,3,5"
	for _, part := range strings.Split(field, ",") {
		part = strings.TrimSpace(part)
		if cronPartMatches(part, value, min, max) {
			return true
		}
	}
	return false
}

// cronPartMatches checks a single part (no commas) against a value.
func cronPartMatches(part string, value, min, max int) bool {
	// Step value: "*/5" or "1-10/2"
	step := 1
	if idx := strings.Index(part, "/"); idx >= 0 {
		s, err := strconv.Atoi(part[idx+1:])
		if err != nil || s <= 0 {
			return false
		}
		step = s
		part = part[:idx]
	}

	// Range: "1-5"
	if idx := strings.Index(part, "-"); idx >= 0 {
		lo, err1 := strconv.Atoi(part[:idx])
		hi, err2 := strconv.Atoi(part[idx+1:])
		if err1 != nil || err2 != nil {
			return false
		}
		if value < lo || value > hi {
			return false
		}
		return (value-lo)%step == 0
	}

	// Wildcard with step: "*/5"
	if part == "*" {
		return (value-min)%step == 0
	}

	// Exact value: "5"
	n, err := strconv.Atoi(part)
	if err != nil {
		return false
	}
	return value == n
}

// buildEndeavourContext creates a concise text summary of the endeavour state
// suitable for inclusion in an LLM prompt. Keeps output under ~3K tokens.
func buildEndeavourContext(export *storage.EndeavourExport, since time.Time) string {
	var b strings.Builder
	edv := export.Endeavour

	// Header
	fmt.Fprintf(&b, "Endeavour: %s (status: %s)\n", edv.Name, edv.Status)
	if edv.Description != "" {
		desc := edv.Description
		if len(desc) > 500 {
			desc = desc[:500] + "..."
		}
		fmt.Fprintf(&b, "%s\n", desc)
	}
	b.WriteString("\n")

	// Task summary
	taskCounts := map[string]int{}
	var changedTasks []string
	for _, t := range export.Tasks {
		taskCounts[t.Status]++
		if !since.IsZero() && t.UpdatedAt.After(since) && len(changedTasks) < 10 {
			changedTasks = append(changedTasks, fmt.Sprintf("  - %s (%s, updated %s)", t.Title, t.Status, t.UpdatedAt.Format("2006-01-02 15:04")))
		}
	}
	fmt.Fprintf(&b, "Tasks (total: %d) -- planned: %d, active: %d, done: %d, canceled: %d\n",
		len(export.Tasks), taskCounts["planned"], taskCounts["active"], taskCounts["done"], taskCounts["canceled"])
	if len(changedTasks) > 0 {
		fmt.Fprintf(&b, "Changed since %s:\n", since.Format("2006-01-02 15:04"))
		for _, line := range changedTasks {
			fmt.Fprintln(&b, line)
		}
	}
	b.WriteString("\n")

	// Demand summary
	demandCounts := map[string]int{}
	var changedDemands []string
	for _, d := range export.Demands {
		demandCounts[d.Status]++
		if !since.IsZero() && d.UpdatedAt.After(since) && len(changedDemands) < 10 {
			changedDemands = append(changedDemands, fmt.Sprintf("  - %s (%s, updated %s)", d.Title, d.Status, d.UpdatedAt.Format("2006-01-02 15:04")))
		}
	}
	fmt.Fprintf(&b, "Demands (total: %d) -- open: %d, in_progress: %d, fulfilled: %d, canceled: %d\n",
		len(export.Demands), demandCounts["open"], demandCounts["in_progress"], demandCounts["fulfilled"], demandCounts["canceled"])
	if len(changedDemands) > 0 {
		fmt.Fprintf(&b, "Changed since %s:\n", since.Format("2006-01-02 15:04"))
		for _, line := range changedDemands {
			fmt.Fprintln(&b, line)
		}
	}
	b.WriteString("\n")

	// Recent comments
	var recentComments []string
	for _, c := range export.Comments {
		if !since.IsZero() && c.CreatedAt.After(since) && len(recentComments) < 5 {
			content := c.Content
			if len(content) > 200 {
				content = content[:200] + "..."
			}
			recentComments = append(recentComments, fmt.Sprintf("  - On %s/%s: \"%s\" (%s)",
				c.EntityType, c.EntityID, content, c.CreatedAt.Format("2006-01-02 15:04")))
		}
	}
	if len(recentComments) > 0 {
		b.WriteString("Recent comments:\n")
		for _, line := range recentComments {
			fmt.Fprintln(&b, line)
		}
	}

	return b.String()
}
