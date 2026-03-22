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


package storage

import (
	"testing"
	"time"
)

func openTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.Initialize(); err != nil {
		t.Fatalf("initialize db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func createTestUser(t *testing.T, db *DB) string {
	t.Helper()
	id := generateID("usr")
	_, err := db.Exec(
		`INSERT INTO user (id, name, email, password_hash, tier, user_type, status, onboarding_status)
		 VALUES (?, 'Test Agent', ?, 'hash', 1, 'agent', 'active', 'interview_pending')`,
		id, id+"@test.local",
	)
	if err != nil {
		t.Fatalf("create test user: %v", err)
	}
	return id
}

func TestOnboardingAttemptCRUD(t *testing.T) {
	db := openTestDB(t)
	userID := createTestUser(t, db)

	// Create
	attempt, err := db.CreateOnboardingAttempt(userID, 1)
	if err != nil {
		t.Fatalf("create attempt: %v", err)
	}
	if attempt.Status != "running" {
		t.Errorf("expected status running, got %s", attempt.Status)
	}
	if attempt.Version != 1 {
		t.Errorf("expected version 1, got %d", attempt.Version)
	}

	// Get
	got, err := db.GetOnboardingAttempt(attempt.ID)
	if err != nil {
		t.Fatalf("get attempt: %v", err)
	}
	if got.UserID != userID {
		t.Errorf("expected user_id %s, got %s", userID, got.UserID)
	}

	// Update
	result := map[string]interface{}{"sections": []int{20, 15, 18, 12, 10}}
	err = db.UpdateOnboardingAttempt(attempt.ID, "passed", 75, result, `[{"section":1,"tool_name":"ts.tsk.create","parameters":{},"success":true}]`)
	if err != nil {
		t.Fatalf("update attempt: %v", err)
	}

	got, _ = db.GetOnboardingAttempt(attempt.ID)
	if got.Status != "passed" {
		t.Errorf("expected status passed, got %s", got.Status)
	}
	if got.TotalScore != 75 {
		t.Errorf("expected score 75, got %d", got.TotalScore)
	}
	if got.CompletedAt == nil {
		t.Error("expected completed_at to be set")
	}

	// List
	attempts, err := db.ListOnboardingAttempts(userID)
	if err != nil {
		t.Fatalf("list attempts: %v", err)
	}
	if len(attempts) != 1 {
		t.Errorf("expected 1 attempt, got %d", len(attempts))
	}

	// GetActiveAttempt (none since it's now passed)
	active, err := db.GetActiveAttempt(userID)
	if err != nil {
		t.Fatalf("get active: %v", err)
	}
	if active != nil {
		t.Error("expected no active attempt")
	}
}

func TestSectionMetrics(t *testing.T) {
	db := openTestDB(t)
	userID := createTestUser(t, db)

	attempt, _ := db.CreateOnboardingAttempt(userID, 1)

	m := &OnboardingSectionMetric{
		AttemptID:    attempt.ID,
		Section:      1,
		Score:        20,
		MaxScore:     20,
		ToolCalls:    3,
		WallTimeMs:   15000,
		ReactionMs:   2000,
		PayloadBytes: 512,
		Status:       "passed",
		Hint:         "",
	}
	if err := db.CreateSectionMetric(m); err != nil {
		t.Fatalf("create metric: %v", err)
	}
	if m.ID == "" {
		t.Error("expected ID to be set")
	}

	metrics, err := db.GetSectionMetrics(attempt.ID)
	if err != nil {
		t.Fatalf("get metrics: %v", err)
	}
	if len(metrics) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(metrics))
	}
	if metrics[0].Score != 20 {
		t.Errorf("expected score 20, got %d", metrics[0].Score)
	}
}

func TestCooldown(t *testing.T) {
	db := openTestDB(t)
	userID := createTestUser(t, db)

	// Initially no cooldown
	c, err := db.GetCooldown(userID)
	if err != nil {
		t.Fatalf("get cooldown: %v", err)
	}
	if c != nil {
		t.Error("expected no cooldown initially")
	}

	// Set cooldown
	nextEligible := UTCNow().Add(1 * time.Hour)
	err = db.UpdateCooldown(userID, 1, &nextEligible, false)
	if err != nil {
		t.Fatalf("update cooldown: %v", err)
	}

	c, _ = db.GetCooldown(userID)
	if c == nil {
		t.Fatal("expected cooldown record")
	}
	if c.FailedAttempts != 1 {
		t.Errorf("expected 1 failed attempt, got %d", c.FailedAttempts)
	}
	if c.Locked {
		t.Error("expected not locked")
	}

	// Escalate
	err = db.UpdateCooldown(userID, 4, nil, true)
	if err != nil {
		t.Fatalf("update cooldown locked: %v", err)
	}

	c, _ = db.GetCooldown(userID)
	if !c.Locked {
		t.Error("expected locked")
	}
	if c.FailedAttempts != 4 {
		t.Errorf("expected 4 failed attempts, got %d", c.FailedAttempts)
	}

	// Reset
	err = db.ResetCooldown(userID)
	if err != nil {
		t.Fatalf("reset cooldown: %v", err)
	}

	c, _ = db.GetCooldown(userID)
	if c.FailedAttempts != 0 {
		t.Errorf("expected 0 failed attempts after reset, got %d", c.FailedAttempts)
	}
}

func TestStep0(t *testing.T) {
	db := openTestDB(t)
	userID := createTestUser(t, db)

	attempt, _ := db.CreateOnboardingAttempt(userID, 1)

	modelInfo := map[string]interface{}{"model": "gpt-4", "provider": "openai"}
	err := db.SaveStep0(attempt.ID, "I am a helpful assistant", modelInfo)
	if err != nil {
		t.Fatalf("save step0: %v", err)
	}

	s, err := db.GetStep0(attempt.ID)
	if err != nil {
		t.Fatalf("get step0: %v", err)
	}
	if s.RawText != "I am a helpful assistant" {
		t.Errorf("unexpected raw text: %s", s.RawText)
	}
	if s.ModelInfo["model"] != "gpt-4" {
		t.Errorf("unexpected model info: %v", s.ModelInfo)
	}
}

func TestOnboardingStatus(t *testing.T) {
	db := openTestDB(t)
	userID := createTestUser(t, db)

	status, err := db.GetUserOnboardingStatus(userID)
	if err != nil {
		t.Fatalf("get status: %v", err)
	}
	if status != "interview_pending" {
		t.Errorf("expected interview_pending, got %s", status)
	}

	err = db.SetUserOnboardingStatus(userID, "active")
	if err != nil {
		t.Fatalf("set status: %v", err)
	}

	status, _ = db.GetUserOnboardingStatus(userID)
	if status != "active" {
		t.Errorf("expected active, got %s", status)
	}
}

func TestTerminateRunningInterviews(t *testing.T) {
	db := openTestDB(t)
	userID := createTestUser(t, db)

	// Create two running attempts
	if _, err := db.CreateOnboardingAttempt(userID, 1); err != nil {
		t.Fatalf("create attempt 1: %v", err)
	}

	userID2 := createTestUser(t, db)
	if _, err := db.CreateOnboardingAttempt(userID2, 1); err != nil {
		t.Fatalf("create attempt 2: %v", err)
	}

	// Terminate
	count, err := db.TerminateRunningInterviews()
	if err != nil {
		t.Fatalf("terminate: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 terminated, got %d", count)
	}

	// Verify both are terminated
	a1, _ := db.GetActiveAttempt(userID)
	if a1 != nil {
		t.Error("expected no active attempt for user 1")
	}

	a2, _ := db.GetActiveAttempt(userID2)
	if a2 != nil {
		t.Error("expected no active attempt for user 2")
	}
}
