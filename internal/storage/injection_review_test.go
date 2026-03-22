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
)

func TestInjectionReviewCRUD(t *testing.T) {
	db := openTestDB(t)
	userID := createTestUser(t, db)

	attempt, err := db.CreateOnboardingAttempt(userID, 1)
	if err != nil {
		t.Fatalf("create attempt: %v", err)
	}

	// Create review
	err = db.CreateInjectionReview(attempt.ID)
	if err != nil {
		t.Fatalf("create review: %v", err)
	}

	// Get pending
	pending, err := db.GetPendingInjectionReview(3)
	if err != nil {
		t.Fatalf("get pending: %v", err)
	}
	if pending == nil {
		t.Fatal("expected pending review")
	}
	if pending.AttemptID != attempt.ID {
		t.Errorf("expected attempt_id %s, got %s", attempt.ID, pending.AttemptID)
	}
	if pending.Status != "pending" {
		t.Errorf("expected status pending, got %s", pending.Status)
	}

	// Set running
	err = db.SetInjectionReviewRunning(pending.ID)
	if err != nil {
		t.Fatalf("set running: %v", err)
	}

	got, _ := db.GetInjectionReview(pending.ID)
	if got.Status != "running" {
		t.Errorf("expected status running, got %s", got.Status)
	}

	// Update to completed with injection detected
	err = db.UpdateInjectionReview(
		pending.ID, "completed", "openai", "gpt-4", true, 0.85,
		`["suspicious prompt injection pattern"]`, "raw response text", "",
	)
	if err != nil {
		t.Fatalf("update review: %v", err)
	}

	got, _ = db.GetInjectionReview(pending.ID)
	if got.Status != "completed" {
		t.Errorf("expected status completed, got %s", got.Status)
	}
	if !got.InjectionDetected {
		t.Error("expected injection_detected to be true")
	}
	if got.Confidence != 0.85 {
		t.Errorf("expected confidence 0.85, got %f", got.Confidence)
	}
	if got.CompletedAt == nil {
		t.Error("expected completed_at to be set")
	}
	if got.Provider != "openai" {
		t.Errorf("expected provider openai, got %s", got.Provider)
	}

	// Count flagged
	flagged, err := db.CountFlaggedReviews()
	if err != nil {
		t.Fatalf("count flagged: %v", err)
	}
	if flagged != 1 {
		t.Errorf("expected 1 flagged, got %d", flagged)
	}
}

func TestInjectionReviewRetries(t *testing.T) {
	db := openTestDB(t)
	userID := createTestUser(t, db)

	attempt, _ := db.CreateOnboardingAttempt(userID, 1)
	_ = db.CreateInjectionReview(attempt.ID)

	pending, _ := db.GetPendingInjectionReview(3)
	_ = db.SetInjectionReviewRunning(pending.ID)

	// Simulate failure with retry
	err := db.IncrementInjectionReviewRetries(pending.ID, "API timeout")
	if err != nil {
		t.Fatalf("increment retries: %v", err)
	}

	got, _ := db.GetInjectionReview(pending.ID)
	if got.Status != "failed" {
		t.Errorf("expected status failed, got %s", got.Status)
	}
	if got.Retries != 1 {
		t.Errorf("expected 1 retry, got %d", got.Retries)
	}
	if got.ErrorMessage != "API timeout" {
		t.Errorf("expected error 'API timeout', got %s", got.ErrorMessage)
	}

	// Should still be picked up as pending (retries < maxRetries)
	nextPending, _ := db.GetPendingInjectionReview(3)
	if nextPending == nil {
		t.Fatal("expected review to be retryable")
	}

	// Increment to max
	_ = db.IncrementInjectionReviewRetries(pending.ID, "timeout 2")
	_ = db.IncrementInjectionReviewRetries(pending.ID, "timeout 3")

	// Now should NOT be picked up (retries >= maxRetries)
	notPending, _ := db.GetPendingInjectionReview(3)
	if notPending != nil {
		t.Error("expected no retryable review after max retries")
	}
}

func TestInjectionReviewList(t *testing.T) {
	db := openTestDB(t)
	userID := createTestUser(t, db)

	attempt1, _ := db.CreateOnboardingAttempt(userID, 1)
	_ = db.CreateInjectionReview(attempt1.ID)

	userID2 := createTestUser(t, db)
	attempt2, _ := db.CreateOnboardingAttempt(userID2, 1)
	_ = db.CreateInjectionReview(attempt2.ID)

	// List all
	all, err := db.ListInjectionReviews("", false, 50, 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("expected 2 reviews, got %d", len(all))
	}

	// List by status
	pending, _ := db.ListInjectionReviews("pending", false, 50, 0)
	if len(pending) != 2 {
		t.Errorf("expected 2 pending, got %d", len(pending))
	}

	// List flagged only
	flagged, _ := db.ListInjectionReviews("", true, 50, 0)
	if len(flagged) != 0 {
		t.Errorf("expected 0 flagged, got %d", len(flagged))
	}

	// Status counts
	counts, err := db.CountInjectionReviewsByStatus()
	if err != nil {
		t.Fatalf("count by status: %v", err)
	}
	if counts["pending"] != 2 {
		t.Errorf("expected 2 pending in counts, got %d", counts["pending"])
	}
}
