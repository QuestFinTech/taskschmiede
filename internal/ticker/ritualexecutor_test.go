package ticker

import (
	"testing"
	"time"

	"github.com/QuestFinTech/taskschmiede/internal/storage"
)

// --- parseIntervalDuration tests ---

func TestParseIntervalDuration(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
		wantErr  bool
	}{
		{"30m", 30 * time.Minute, false},
		{"2h", 2 * time.Hour, false},
		{"1d", 24 * time.Hour, false},
		{"2w", 14 * 24 * time.Hour, false},
		{"13w", 13 * 7 * 24 * time.Hour, false},
		{"", 0, true},
		{"abc", 0, true},
		{"2x", 0, true},
	}
	for _, tt := range tests {
		d, err := parseIntervalDuration(tt.input)
		if tt.wantErr {
			if err == nil {
				t.Errorf("parseIntervalDuration(%q): expected error, got %v", tt.input, d)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseIntervalDuration(%q): unexpected error: %v", tt.input, err)
			continue
		}
		if d != tt.expected {
			t.Errorf("parseIntervalDuration(%q) = %v, want %v", tt.input, d, tt.expected)
		}
	}
}

// --- cronMatchesNow tests ---

func TestCronMatchesNow(t *testing.T) {
	// 2026-03-16 09:30 Monday (weekday=1)
	now := time.Date(2026, 3, 16, 9, 30, 0, 0, time.UTC)

	tests := []struct {
		expr    string
		matches bool
	}{
		{"* * * * *", true},           // every minute
		{"30 9 * * *", true},          // at 09:30
		{"30 9 * * 1", true},          // at 09:30 on Monday
		{"0 9 * * *", false},          // at 09:00 (not 09:30)
		{"30 10 * * *", false},        // at 10:30 (not 09:30)
		{"30 9 * * 5", false},         // at 09:30 on Friday (not Monday)
		{"30 9 16 * *", true},         // at 09:30 on day 16
		{"30 9 17 * *", false},        // at 09:30 on day 17
		{"30 9 * 3 *", true},          // at 09:30 in March
		{"30 9 * 4 *", false},         // at 09:30 in April
		{"*/5 * * * *", true},         // every 5 minutes (30 % 5 == 0)
		{"*/7 * * * *", false},        // every 7 minutes (30 % 7 != 0)
		{"0 9 * * 1-5", false},        // at 09:00 on weekdays (minute doesn't match)
		{"30 9 * * 1-5", true},        // at 09:30 on weekdays
		{"30 9 * * 0,6", false},       // at 09:30 on weekends
		{"30 9 * * 1,3,5", true},      // at 09:30 on Mon,Wed,Fri
		{"30 9 * * 2,4", false},       // at 09:30 on Tue,Thu
		{"0 9", false},                // invalid (too few fields)
		{"", false},                   // empty
	}
	for _, tt := range tests {
		got := cronMatchesNow(tt.expr, now)
		if got != tt.matches {
			t.Errorf("cronMatchesNow(%q, 09:30 Mon Mar 16) = %v, want %v", tt.expr, got, tt.matches)
		}
	}
}

// --- isRitualDue tests ---

func TestIsRitualDue_Interval(t *testing.T) {
	now := time.Date(2026, 3, 16, 10, 0, 0, 0, time.UTC)
	sched := map[string]interface{}{"type": "interval", "every": "2h"}

	// First run (no last run) -> due
	if !isRitualDue(sched, nil, now) {
		t.Error("interval: first run should be due")
	}

	// Last run 3 hours ago -> due
	finished := now.Add(-3 * time.Hour)
	lastRun := &storage.RitualRun{FinishedAt: &finished, CreatedAt: finished}
	if !isRitualDue(sched, lastRun, now) {
		t.Error("interval: 3h since last run (>2h) should be due")
	}

	// Last run 1 hour ago -> not due
	finished = now.Add(-1 * time.Hour)
	lastRun = &storage.RitualRun{FinishedAt: &finished, CreatedAt: finished}
	if isRitualDue(sched, lastRun, now) {
		t.Error("interval: 1h since last run (<2h) should not be due")
	}
}

func TestIsRitualDue_Cron(t *testing.T) {
	// 09:30 Monday
	now := time.Date(2026, 3, 16, 9, 30, 0, 0, time.UTC)
	sched := map[string]interface{}{"type": "cron", "expression": "30 9 * * 1"}

	// First run -> due
	if !isRitualDue(sched, nil, now) {
		t.Error("cron: first run matching expression should be due")
	}

	// Already ran this minute -> not due
	sameMinute := time.Date(2026, 3, 16, 9, 30, 15, 0, time.UTC)
	lastRun := &storage.RitualRun{FinishedAt: &sameMinute, CreatedAt: sameMinute}
	if isRitualDue(sched, lastRun, now) {
		t.Error("cron: already ran this minute should not be due")
	}

	// Ran yesterday -> due
	yesterday := now.Add(-24 * time.Hour)
	lastRun = &storage.RitualRun{FinishedAt: &yesterday, CreatedAt: yesterday}
	if !isRitualDue(sched, lastRun, now) {
		t.Error("cron: last ran yesterday should be due")
	}
}

func TestIsRitualDue_Manual(t *testing.T) {
	now := time.Date(2026, 3, 16, 10, 0, 0, 0, time.UTC)
	sched := map[string]interface{}{"type": "manual"}

	if isRitualDue(sched, nil, now) {
		t.Error("manual: should never be due via ticker")
	}
}

// --- NewRitualExecutorHandler tests ---

func TestNewRitualExecutorHandler_Defaults(t *testing.T) {
	h := NewRitualExecutorHandler(nil, nil, nil, nil, 0)
	if h.Name != "ritual-executor" {
		t.Errorf("expected name 'ritual-executor', got %s", h.Name)
	}
	if h.Interval != 30*time.Second {
		t.Errorf("expected default interval 30s, got %v", h.Interval)
	}
}

func TestNewRitualExecutorHandler_CustomInterval(t *testing.T) {
	h := NewRitualExecutorHandler(nil, nil, nil, nil, 15*time.Second)
	if h.Interval != 15*time.Second {
		t.Errorf("expected interval 15s, got %v", h.Interval)
	}
}
