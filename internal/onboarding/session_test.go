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
	"io"
	"log/slog"
	"testing"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestCreateAndGetSession(t *testing.T) {
	sm := NewSessionManager(testLogger())
	version := DefaultInterviewVersion()

	session, err := sm.CreateSession("usr_test1", "oba_test1", version)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	if session.Phase != PhaseStep0 {
		t.Errorf("expected phase step0, got %s", session.Phase)
	}
	if session.CurrentSection != 0 {
		t.Errorf("expected section 0, got %d", session.CurrentSection)
	}
	if session.SimDB == nil {
		t.Fatal("expected simulation DB to be set")
	}

	// Get by session ID
	got := sm.GetSession(session.ID)
	if got == nil {
		t.Fatal("expected to find session by ID")
	}
	if got.UserID != "usr_test1" {
		t.Errorf("expected user usr_test1, got %s", got.UserID)
	}

	// Get by user ID
	got = sm.GetSessionByUser("usr_test1")
	if got == nil {
		t.Fatal("expected to find session by user ID")
	}

	// Active count
	if sm.ActiveCount() != 1 {
		t.Errorf("expected 1 active session, got %d", sm.ActiveCount())
	}
}

func TestOneSessionPerUser(t *testing.T) {
	sm := NewSessionManager(testLogger())
	version := DefaultInterviewVersion()

	_, err := sm.CreateSession("usr_test1", "oba_test1", version)
	if err != nil {
		t.Fatalf("first session: %v", err)
	}

	// Second session for same user should fail
	_, err = sm.CreateSession("usr_test1", "oba_test2", version)
	if err == nil {
		t.Fatal("expected error for duplicate session")
	}
}

func TestEndSession(t *testing.T) {
	sm := NewSessionManager(testLogger())
	version := DefaultInterviewVersion()

	session, _ := sm.CreateSession("usr_test1", "oba_test1", version)

	sm.EndSession(session.ID)

	if sm.GetSession(session.ID) != nil {
		t.Error("expected session to be removed")
	}
	if sm.GetSessionByUser("usr_test1") != nil {
		t.Error("expected user mapping to be removed")
	}
	if sm.ActiveCount() != 0 {
		t.Errorf("expected 0 active sessions, got %d", sm.ActiveCount())
	}

	// Can create a new session for the same user after ending
	_, err := sm.CreateSession("usr_test1", "oba_test2", version)
	if err != nil {
		t.Fatalf("new session after end: %v", err)
	}
}

func TestSessionPhaseTransitions(t *testing.T) {
	sm := NewSessionManager(testLogger())
	version := DefaultInterviewVersion()

	session, _ := sm.CreateSession("usr_test1", "oba_test1", version)

	// Start in Step 0
	if session.Phase != PhaseStep0 {
		t.Errorf("expected step0, got %s", session.Phase)
	}

	// Move to sections
	session.StartSections()
	if session.Phase != PhaseSection {
		t.Errorf("expected section, got %s", session.Phase)
	}
	if session.CurrentSection != 1 {
		t.Errorf("expected section 1, got %d", session.CurrentSection)
	}

	// Advance through sections
	for i := 2; i <= version.SectionCount(); i++ {
		session.AdvanceSection()
		if session.CurrentSection != i {
			t.Errorf("expected section %d, got %d", i, session.CurrentSection)
		}
	}

	// One more advance completes the interview
	session.AdvanceSection()
	if session.Phase != PhaseComplete {
		t.Errorf("expected complete, got %s", session.Phase)
	}
}

func TestSimulationSeedData(t *testing.T) {
	sm := NewSessionManager(testLogger())
	version := DefaultInterviewVersion()

	session, err := sm.CreateSession("usr_test1", "oba_test1", version)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	// Verify seed tasks exist in simulation
	var taskCount int
	err = session.SimDB.QueryRow("SELECT COUNT(*) FROM task").Scan(&taskCount)
	if err != nil {
		t.Fatalf("count tasks: %v", err)
	}
	if taskCount != len(version.SeedData.Tasks) {
		t.Errorf("expected %d seeded tasks, got %d", len(version.SeedData.Tasks), taskCount)
	}

	// Verify "done" task count matches Section 4 expected value
	var doneCount int
	err = session.SimDB.QueryRow("SELECT COUNT(*) FROM task WHERE status = 'done'").Scan(&doneCount)
	if err != nil {
		t.Fatalf("count done tasks: %v", err)
	}
	expectedDone := version.Challenges[4].ExpectedValues["done_count"].(int)
	if doneCount != expectedDone {
		t.Errorf("expected %d done tasks, got %d", expectedDone, doneCount)
	}

	// Verify newest seed task title (before agent creates their own task)
	var newestTitle string
	err = session.SimDB.QueryRow("SELECT title FROM task ORDER BY created_at DESC LIMIT 1").Scan(&newestTitle)
	if err != nil {
		t.Fatalf("get newest title: %v", err)
	}
	if newestTitle != "Deploy monitoring dashboard" {
		t.Errorf("expected newest seed title %q, got %q", "Deploy monitoring dashboard", newestTitle)
	}

	// Verify interviewer resource exists
	var resCount int
	err = session.SimDB.QueryRow("SELECT COUNT(*) FROM resource WHERE id = ?", version.SeedData.InterviewerResourceID).Scan(&resCount)
	if err != nil {
		t.Fatalf("check resource: %v", err)
	}
	if resCount != 1 {
		t.Errorf("expected interviewer resource, got %d", resCount)
	}

	// Verify endeavour exists
	var edvName string
	err = session.SimDB.QueryRow("SELECT name FROM endeavour WHERE id = 'edv_onboarding'").Scan(&edvName)
	if err != nil {
		t.Fatalf("get endeavour: %v", err)
	}
	if edvName != version.SeedData.EndeavourName {
		t.Errorf("expected endeavour %q, got %q", version.SeedData.EndeavourName, edvName)
	}

	// Verify seed task estimates
	var totalEst float64
	err = session.SimDB.QueryRow("SELECT COALESCE(SUM(estimate), 0) FROM task").Scan(&totalEst)
	if err != nil {
		t.Fatalf("sum estimates: %v", err)
	}
	var expectedEstSum float64
	for _, task := range version.SeedData.Tasks {
		expectedEstSum += task.Estimate
	}
	if totalEst != expectedEstSum {
		t.Errorf("expected total estimate %.1f, got %.1f", expectedEstSum, totalEst)
	}

	// Verify seed demands exist
	var demandCount int
	err = session.SimDB.QueryRow("SELECT COUNT(*) FROM demand").Scan(&demandCount)
	if err != nil {
		t.Fatalf("count demands: %v", err)
	}
	if demandCount != len(version.SeedData.Demands) {
		t.Errorf("expected %d seeded demands, got %d", len(version.SeedData.Demands), demandCount)
	}

	// Verify seed demand priorities
	var mediumCount int
	err = session.SimDB.QueryRow("SELECT COUNT(*) FROM demand WHERE priority = 'medium'").Scan(&mediumCount)
	if err != nil {
		t.Fatalf("count medium demands: %v", err)
	}
	if mediumCount != 2 {
		t.Errorf("expected 2 medium-priority demands, got %d", mediumCount)
	}
}

func TestToolLog(t *testing.T) {
	log := NewToolLog()

	log.Record(ToolCallEntry{ToolName: "ts.tsk.create", Section: 1, Success: true, PayloadBytes: 100})
	log.Record(ToolCallEntry{ToolName: "ts.tsk.update", Section: 2, Success: true, PayloadBytes: 50})
	log.Record(ToolCallEntry{ToolName: "ts.tsk.list", Section: 2, Success: false, PayloadBytes: 30})

	if log.Count() != 3 {
		t.Errorf("expected 3 entries, got %d", log.Count())
	}

	if log.CountForSection(1) != 1 {
		t.Errorf("expected 1 entry for section 1, got %d", log.CountForSection(1))
	}
	if log.CountForSection(2) != 2 {
		t.Errorf("expected 2 entries for section 2, got %d", log.CountForSection(2))
	}

	s1Entries := log.ForSection(1)
	if len(s1Entries) != 1 || s1Entries[0].ToolName != "ts.tsk.create" {
		t.Errorf("unexpected section 1 entries: %+v", s1Entries)
	}

	if log.TotalPayloadBytes() != 180 {
		t.Errorf("expected 180 payload bytes, got %d", log.TotalPayloadBytes())
	}
}

func TestNormalizeText(t *testing.T) {
	// NFC normalization: e + combining accent -> precomposed
	// U+0065 (e) + U+0301 (combining acute) -> U+00E9 (e with acute)
	input := "caf\u0065\u0301"
	expected := "caf\u00e9"
	got := NormalizeText(input)
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}

	// Already NFC should be unchanged
	plain := "ready to contribute"
	if NormalizeText(plain) != plain {
		t.Errorf("expected unchanged for already-NFC text")
	}
}

func TestBudgetChecks(t *testing.T) {
	sm := NewSessionManager(testLogger())

	// Create a version with very tight budgets for testing
	version := DefaultInterviewVersion()
	version.ToolBudgetTotal = 3
	version.ToolBudgetSection = 2

	session, _ := sm.CreateSession("usr_test1", "oba_test1", version)
	session.StartSections()

	// No budget issues initially
	if err := session.CheckBudgets(); err != nil {
		t.Errorf("unexpected budget error: %v", err)
	}

	// Add tool calls to hit section limit
	session.ToolLog.Record(ToolCallEntry{Section: 1})
	session.ToolLog.Record(ToolCallEntry{Section: 1})

	err := session.CheckBudgets()
	if err == nil {
		t.Error("expected section budget error")
	}

	// Advance to section 2, section budget resets
	session.AdvanceSection()
	if err := session.CheckBudgets(); err != nil {
		t.Errorf("unexpected budget error after section advance: %v", err)
	}

	// One more call hits total budget
	session.ToolLog.Record(ToolCallEntry{Section: 2})

	err = session.CheckBudgets()
	if err == nil {
		t.Error("expected total budget error")
	}
}
