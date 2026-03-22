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


package onboarding

import (
	"sync"
	"time"
)

// ToolCallEntry records a single tool call made during an interview.
type ToolCallEntry struct {
	Timestamp    time.Time
	Section      int
	ToolName     string
	Parameters   map[string]interface{}
	Result       interface{}
	Error        string
	DurationMs   int64
	Success      bool
	PayloadBytes int64
}

// ToolLog is a thread-safe, append-only log of tool calls for an interview session.
type ToolLog struct {
	mu      sync.Mutex
	entries []ToolCallEntry
}

// NewToolLog creates a new empty tool log.
func NewToolLog() *ToolLog {
	return &ToolLog{}
}

// Record appends a tool call entry to the log.
func (l *ToolLog) Record(entry ToolCallEntry) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, entry)
}

// All returns a copy of all entries.
func (l *ToolLog) All() []ToolCallEntry {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]ToolCallEntry, len(l.entries))
	copy(out, l.entries)
	return out
}

// ForSection returns entries for a specific section.
func (l *ToolLog) ForSection(section int) []ToolCallEntry {
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []ToolCallEntry
	for _, e := range l.entries {
		if e.Section == section {
			out = append(out, e)
		}
	}
	return out
}

// Count returns the total number of entries.
func (l *ToolLog) Count() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.entries)
}

// CountForSection returns the number of entries for a specific section.
func (l *ToolLog) CountForSection(section int) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	count := 0
	for _, e := range l.entries {
		if e.Section == section {
			count++
		}
	}
	return count
}

// TotalPayloadBytes returns the sum of payload bytes across all entries.
func (l *ToolLog) TotalPayloadBytes() int64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	var total int64
	for _, e := range l.entries {
		total += e.PayloadBytes
	}
	return total
}
