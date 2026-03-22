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
	"database/sql"
	"fmt"
	"time"
)

// OnboardingInjectionReview holds a post-hoc injection detection review result.
type OnboardingInjectionReview struct {
	ID                string
	AttemptID         string
	Status            string  // pending, running, completed, failed
	Provider          string  // anthropic, openai
	Model             string  // model used for review
	InjectionDetected bool    // advisory flag
	Confidence        float64 // 0.0 to 1.0
	Evidence          string  // JSON array of evidence strings
	RawResponse       string  // full LLM response
	ErrorMessage      string  // error details if failed
	Retries           int
	CreatedAt         time.Time
	CompletedAt       *time.Time
}

// CreateInjectionReview inserts a pending review for a completed interview attempt.
func (db *DB) CreateInjectionReview(attemptID string) error {
	id := generateID("irv")
	now := UTCNow().Format(time.RFC3339)

	_, err := db.Exec(
		`INSERT INTO onboarding_injection_review (id, attempt_id, status, created_at)
		 VALUES (?, ?, 'pending', ?)`,
		id, attemptID, now,
	)
	if err != nil {
		return fmt.Errorf("create injection review: %w", err)
	}
	return nil
}

// GetInjectionReview retrieves a review by ID.
func (db *DB) GetInjectionReview(id string) (*OnboardingInjectionReview, error) {
	var r OnboardingInjectionReview
	var completedAt sql.NullString
	var createdAt string
	var injectionDetected int

	err := db.QueryRow(
		`SELECT id, attempt_id, status, provider, model, injection_detected, confidence,
		        evidence, raw_response, error_message, retries, created_at, completed_at
		 FROM onboarding_injection_review WHERE id = ?`, id,
	).Scan(&r.ID, &r.AttemptID, &r.Status, &r.Provider, &r.Model, &injectionDetected,
		&r.Confidence, &r.Evidence, &r.RawResponse, &r.ErrorMessage, &r.Retries,
		&createdAt, &completedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get injection review: %w", err)
	}

	r.InjectionDetected = injectionDetected != 0
	r.CreatedAt = ParseDBTime(createdAt)
	if completedAt.Valid {
		t := ParseDBTime(completedAt.String)
		r.CompletedAt = &t
	}
	return &r, nil
}

// GetPendingInjectionReview returns the oldest pending or failed (retryable) review.
func (db *DB) GetPendingInjectionReview(maxRetries int) (*OnboardingInjectionReview, error) {
	var r OnboardingInjectionReview
	var completedAt sql.NullString
	var createdAt string
	var injectionDetected int

	err := db.QueryRow(
		`SELECT id, attempt_id, status, provider, model, injection_detected, confidence,
		        evidence, raw_response, error_message, retries, created_at, completed_at
		 FROM onboarding_injection_review
		 WHERE status IN ('pending', 'failed') AND retries < ?
		 ORDER BY created_at ASC LIMIT 1`, maxRetries,
	).Scan(&r.ID, &r.AttemptID, &r.Status, &r.Provider, &r.Model, &injectionDetected,
		&r.Confidence, &r.Evidence, &r.RawResponse, &r.ErrorMessage, &r.Retries,
		&createdAt, &completedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get pending injection review: %w", err)
	}

	r.InjectionDetected = injectionDetected != 0
	r.CreatedAt = ParseDBTime(createdAt)
	if completedAt.Valid {
		t := ParseDBTime(completedAt.String)
		r.CompletedAt = &t
	}
	return &r, nil
}

// UpdateInjectionReview updates a review's status and results.
func (db *DB) UpdateInjectionReview(id, status, provider, model string, injectionDetected bool, confidence float64, evidence, rawResponse, errorMessage string) error {
	injectedInt := 0
	if injectionDetected {
		injectedInt = 1
	}

	var completedAt *string
	if status == "completed" || status == "failed" {
		now := UTCNow().Format(time.RFC3339)
		completedAt = &now
	}

	_, err := db.Exec(
		`UPDATE onboarding_injection_review
		 SET status = ?, provider = ?, model = ?, injection_detected = ?, confidence = ?,
		     evidence = ?, raw_response = ?, error_message = ?, completed_at = ?
		 WHERE id = ?`,
		status, provider, model, injectedInt, confidence,
		evidence, rawResponse, errorMessage, completedAt, id,
	)
	if err != nil {
		return fmt.Errorf("update injection review: %w", err)
	}
	return nil
}

// SetInjectionReviewRunning marks a review as running.
func (db *DB) SetInjectionReviewRunning(id string) error {
	_, err := db.Exec(
		`UPDATE onboarding_injection_review SET status = 'running' WHERE id = ?`, id,
	)
	if err != nil {
		return fmt.Errorf("set injection review running: %w", err)
	}
	return nil
}

// IncrementInjectionReviewRetries increments the retry counter and sets status to failed.
func (db *DB) IncrementInjectionReviewRetries(id, errorMessage string) error {
	now := UTCNow().Format(time.RFC3339)
	_, err := db.Exec(
		`UPDATE onboarding_injection_review
		 SET status = 'failed', retries = retries + 1, error_message = ?, completed_at = ?
		 WHERE id = ?`,
		errorMessage, now, id,
	)
	if err != nil {
		return fmt.Errorf("increment injection review retries: %w", err)
	}
	return nil
}

// ListInjectionReviews returns reviews with optional filters.
func (db *DB) ListInjectionReviews(status string, flaggedOnly bool, limit, offset int) ([]*OnboardingInjectionReview, error) {
	query := `SELECT id, attempt_id, status, provider, model, injection_detected, confidence,
	                 evidence, raw_response, error_message, retries, created_at, completed_at
	          FROM onboarding_injection_review WHERE 1=1`
	var args []interface{}

	if status != "" {
		query += " AND status = ?"
		args = append(args, status)
	}
	if flaggedOnly {
		query += " AND injection_detected = 1"
	}

	query += " ORDER BY created_at DESC"

	if limit <= 0 {
		limit = 50
	}
	query += " LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list injection reviews: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var reviews []*OnboardingInjectionReview
	for rows.Next() {
		var r OnboardingInjectionReview
		var completedAt sql.NullString
		var createdAt string
		var injectionDetected int

		if err := rows.Scan(&r.ID, &r.AttemptID, &r.Status, &r.Provider, &r.Model,
			&injectionDetected, &r.Confidence, &r.Evidence, &r.RawResponse,
			&r.ErrorMessage, &r.Retries, &createdAt, &completedAt); err != nil {
			return nil, fmt.Errorf("scan injection review: %w", err)
		}

		r.InjectionDetected = injectionDetected != 0
		r.CreatedAt = ParseDBTime(createdAt)
		if completedAt.Valid {
			t := ParseDBTime(completedAt.String)
			r.CompletedAt = &t
		}
		reviews = append(reviews, &r)
	}
	return reviews, nil
}

// CountFlaggedReviews returns the number of reviews where injection was detected.
func (db *DB) CountFlaggedReviews() (int, error) {
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM onboarding_injection_review WHERE injection_detected = 1`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count flagged reviews: %w", err)
	}
	return count, nil
}

// CountInjectionReviewsByStatus returns counts by status.
func (db *DB) CountInjectionReviewsByStatus() (map[string]int, error) {
	rows, err := db.Query(`SELECT status, COUNT(*) FROM onboarding_injection_review GROUP BY status`)
	if err != nil {
		return nil, fmt.Errorf("count injection reviews by status: %w", err)
	}
	defer func() { _ = rows.Close() }()

	counts := make(map[string]int)
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("scan status count: %w", err)
		}
		counts[status] = count
	}
	return counts, nil
}
