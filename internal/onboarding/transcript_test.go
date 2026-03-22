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
	"strings"
	"testing"
)

func TestExtractAgentText_Empty(t *testing.T) {
	fields := ExtractAgentText("")
	if len(fields) != 0 {
		t.Errorf("expected 0 fields from empty log, got %d", len(fields))
	}

	fields = ExtractAgentText("[]")
	if len(fields) != 0 {
		t.Errorf("expected 0 fields from empty array, got %d", len(fields))
	}
}

func TestExtractAgentText_NormalLog(t *testing.T) {
	log := `[
		{"section":1,"tool_name":"ts.tsk.create","parameters":{"title":"Fix bug","description":"Fix the critical bug in auth"},"success":true},
		{"section":1,"tool_name":"ts.cmt.create","parameters":{"content":"Added a comment"},"success":true},
		{"section":2,"tool_name":"ts.msg.send","parameters":{"subject":"Hello","content":"Message body"},"success":true},
		{"section":3,"tool_name":"ts.onboard.submit","parameters":{"done_count":"5","newest_title":"My task"},"success":true}
	]`

	fields := ExtractAgentText(log)
	if len(fields) != 7 {
		t.Fatalf("expected 7 fields, got %d", len(fields))
	}

	// Check first entry: ts.tsk.create -> title
	if fields[0].ToolName != "ts.tsk.create" || fields[0].FieldName != "title" || fields[0].Value != "Fix bug" {
		t.Errorf("unexpected first field: %+v", fields[0])
	}

	// Check section assignment
	for _, f := range fields {
		if f.ToolName == "ts.msg.send" && f.Section != 2 {
			t.Errorf("expected section 2 for ts.msg.send, got %d", f.Section)
		}
	}

	// Check ts.onboard.submit extracts all string values
	submitFields := 0
	for _, f := range fields {
		if f.ToolName == "ts.onboard.submit" {
			submitFields++
		}
	}
	if submitFields != 2 {
		t.Errorf("expected 2 submit fields, got %d", submitFields)
	}
}

func TestExtractAgentText_Truncation(t *testing.T) {
	longText := strings.Repeat("a", 3000)
	log := `[{"section":1,"tool_name":"ts.tsk.create","parameters":{"title":"` + longText + `"},"success":true}]`

	fields := ExtractAgentText(log)
	if len(fields) != 1 {
		t.Fatalf("expected 1 field, got %d", len(fields))
	}
	if len(fields[0].Value) > maxFieldLength+20 { // allow for "... [truncated]"
		t.Errorf("expected truncated value, got length %d", len(fields[0].Value))
	}
}

func TestExtractAgentText_UnknownTool(t *testing.T) {
	log := `[{"section":1,"tool_name":"ts.unknown.tool","parameters":{"title":"Test"},"success":true}]`

	fields := ExtractAgentText(log)
	if len(fields) != 0 {
		t.Errorf("expected 0 fields for unknown tool, got %d", len(fields))
	}
}

func TestExtractAgentText_InvalidJSON(t *testing.T) {
	fields := ExtractAgentText("not json")
	if len(fields) != 0 {
		t.Errorf("expected 0 fields from invalid JSON, got %d", len(fields))
	}
}

func TestBuildReviewerPrompt_Basic(t *testing.T) {
	fields := []AgentTextField{
		{Section: 1, ToolName: "ts.tsk.create", FieldName: "title", Value: "Fix bug"},
		{Section: 1, ToolName: "ts.tsk.create", FieldName: "description", Value: "Fix the auth bug"},
	}

	sections := []SectionResult{
		{Section: 1, Score: 20, MaxScore: 20, Status: "passed"},
		{Section: 2, Score: 15, MaxScore: 20, Status: "passed"},
	}

	system, user := BuildReviewerPrompt("I am Claude, an AI assistant", fields, 35, 40, "pass", sections)

	// System prompt should contain key instructions
	if !strings.Contains(system, "prompt injection") {
		t.Error("system prompt missing 'prompt injection'")
	}
	if !strings.Contains(system, "ADVISORY ONLY") {
		t.Error("system prompt missing 'ADVISORY ONLY'")
	}

	// User prompt should contain untrusted text markers
	if !strings.Contains(user, "<<<UNTRUSTED_TEXT>>>") {
		t.Error("user prompt missing UNTRUSTED_TEXT markers")
	}
	if !strings.Contains(user, "<<<END_UNTRUSTED_TEXT>>>") {
		t.Error("user prompt missing END_UNTRUSTED_TEXT markers")
	}

	// Should include score info
	if !strings.Contains(user, "35/40") {
		t.Error("user prompt missing score")
	}

	// Should include step0 text
	if !strings.Contains(user, "I am Claude") {
		t.Error("user prompt missing step0 text")
	}

	// Should include section scores
	if !strings.Contains(user, "Section 1: 20/20") {
		t.Error("user prompt missing section 1 score")
	}
}

func TestBuildReviewerPrompt_NoText(t *testing.T) {
	_, user := BuildReviewerPrompt("", nil, 0, 160, "fail", nil)

	if !strings.Contains(user, "No agent-produced text found") {
		t.Error("expected 'No agent-produced text found' for empty input")
	}
}
