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
	"log/slog"
	"time"

	"github.com/QuestFinTech/taskschmiede/internal/llmclient"
	"github.com/QuestFinTech/taskschmiede/internal/onboarding"
	"github.com/QuestFinTech/taskschmiede/internal/storage"
)

// NewInjectionReviewerHandler returns a ticker handler that processes pending
// injection reviews. It picks up one pending review per tick, calls the
// configured LLM for analysis, and stores the result.
func NewInjectionReviewerHandler(db *storage.DB, client llmclient.Client, logger *slog.Logger, maxRetries int, interval time.Duration) Handler {
	if maxRetries <= 0 {
		maxRetries = 3
	}
	if interval <= 0 {
		interval = 2 * time.Minute
	}

	return Handler{
		Name:     "injection-reviewer",
		Interval: interval,
		Fn:       injectionReviewCheck(db, client, logger, maxRetries),
	}
}

func injectionReviewCheck(db *storage.DB, client llmclient.Client, logger *slog.Logger, maxRetries int) func(context.Context, time.Time) error {
	return func(ctx context.Context, now time.Time) error {
		review, err := db.GetPendingInjectionReview(maxRetries)
		if err != nil {
			return err
		}
		if review == nil {
			return nil // nothing to process
		}

		logger.Info("injection-reviewer: processing review",
			"review_id", review.ID,
			"attempt_id", review.AttemptID,
			"retry", review.Retries,
		)

		// Mark as running
		if err := db.SetInjectionReviewRunning(review.ID); err != nil {
			return err
		}

		// Load attempt data
		attempt, err := db.GetOnboardingAttempt(review.AttemptID)
		if err != nil || attempt == nil {
			_ = db.IncrementInjectionReviewRetries(review.ID, "attempt not found")
			return nil
		}

		// Load Step 0 text
		step0Text := ""
		step0, err := db.GetStep0(review.AttemptID)
		if err == nil && step0 != nil {
			step0Text = step0.RawText
		}

		// Extract agent-produced text from tool log
		fields := onboarding.ExtractAgentText(attempt.ToolLog)

		// Build section results from attempt result JSON
		var sections []onboarding.SectionResult
		if attempt.Result != nil {
			if raw, ok := attempt.Result["sections"]; ok {
				data, _ := json.Marshal(raw)
				_ = json.Unmarshal(data, &sections)
			}
		}

		score := attempt.TotalScore
		maxScore := 0
		resultStr := ""
		if attempt.Result != nil {
			if v, ok := attempt.Result["max_score"].(float64); ok {
				maxScore = int(v)
			}
			if v, ok := attempt.Result["result"].(string); ok {
				resultStr = v
			}
		}

		// Build prompt
		systemPrompt, userPrompt := onboarding.BuildReviewerPrompt(
			step0Text, fields, score, maxScore, resultStr, sections,
		)

		// Call LLM
		resp, err := client.Complete(ctx, &llmclient.Request{
			SystemPrompt: systemPrompt,
			UserPrompt:   userPrompt,
			MaxTokens:    512,
		})
		if err != nil {
			logger.Warn("injection-reviewer: LLM call failed",
				"review_id", review.ID,
				"error", err,
			)
			_ = db.IncrementInjectionReviewRetries(review.ID, err.Error())
			return nil // don't propagate -- will retry next tick
		}

		// Parse LLM response
		detected, confidence, evidence := parseReviewResponse(resp.Content)

		evidenceJSON, _ := json.Marshal(evidence)

		if err := db.UpdateInjectionReview(
			review.ID,
			"completed",
			client.Provider(),
			client.Model(),
			detected,
			confidence,
			string(evidenceJSON),
			resp.Content,
			"",
		); err != nil {
			logger.Warn("injection-reviewer: failed to update review",
				"review_id", review.ID,
				"error", err,
			)
			return nil
		}

		if detected {
			logger.Warn("injection-reviewer: injection detected",
				"review_id", review.ID,
				"attempt_id", review.AttemptID,
				"confidence", confidence,
				"evidence_count", len(evidence),
			)
		} else {
			logger.Info("injection-reviewer: review clean",
				"review_id", review.ID,
				"attempt_id", review.AttemptID,
			)
		}

		return nil
	}
}

// parseReviewResponse extracts the structured result from the LLM's JSON response.
// On parse failure, returns no injection detected with a parse failure note.
func parseReviewResponse(content string) (detected bool, confidence float64, evidence []string) {
	// Try to extract JSON from the response (handle potential markdown wrapping)
	jsonContent := content
	if idx := findJSONStart(content); idx >= 0 {
		jsonContent = content[idx:]
		if end := findJSONEnd(jsonContent); end > 0 {
			jsonContent = jsonContent[:end+1]
		}
	}

	var result struct {
		InjectionDetected bool     `json:"injection_detected"`
		Confidence        float64  `json:"confidence"`
		Evidence          []string `json:"evidence"`
	}

	if err := json.Unmarshal([]byte(jsonContent), &result); err != nil {
		return false, 0.0, []string{"response_parse_failure"}
	}

	return result.InjectionDetected, result.Confidence, result.Evidence
}

// findJSONStart finds the first '{' in the string.
func findJSONStart(s string) int {
	for i, c := range s {
		switch c {
		case '{':
			return i
		}
	}
	return -1
}

// findJSONEnd finds the matching '}' for the first '{'.
func findJSONEnd(s string) int {
	depth := 0
	for i, c := range s {
		switch c {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}
