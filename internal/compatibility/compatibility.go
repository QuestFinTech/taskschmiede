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


// Package compatibility provides LLM compatibility matrix aggregation
// from onboarding interview results.
package compatibility

import (
	"sort"
	"strings"
	"time"

	"github.com/QuestFinTech/taskschmiede/internal/storage"
)

// Thresholds holds the scoring boundaries for result classification.
type Thresholds struct {
	Pass        int
	Distinction int
	MaxScore    int
	Sections    int
}

// SectionScore holds a single section's result.
type SectionScore struct {
	Section  int `json:"section"`
	Score    int `json:"score"`
	MaxScore int `json:"max_score"`
}

// ModelEntry holds aggregated results for one model.
type ModelEntry struct {
	Model         string         `json:"model"`
	Provider      string         `json:"provider"`
	ActiveParams  string         `json:"active_params"`
	Client        string         `json:"client"`
	BestScore     int            `json:"best_score"`
	MaxScore      int            `json:"max_score"`
	Result        string         `json:"result"`
	Attempts      int            `json:"attempts"`
	SectionScores []SectionScore `json:"section_scores"`
}

// Meta holds response metadata.
type Meta struct {
	TotalModels          int    `json:"total_models"`
	TotalAttempts        int    `json:"total_attempts"`
	MaxPossibleScore     int    `json:"max_possible_score"`
	SectionCount         int    `json:"section_count"`
	PassThreshold        int    `json:"pass_threshold"`
	DistinctionThreshold int    `json:"distinction_threshold"`
	GeneratedAt          string `json:"generated_at"`
}

// Matrix is the top-level compatibility response.
type Matrix struct {
	Models []ModelEntry `json:"models"`
	Meta   Meta         `json:"meta"`
}

// Build aggregates raw interview rows into a compatibility matrix.
// Rows are grouped by resolved model name; the best score per model is kept.
func Build(rows []*storage.CompatibilityRow, t Thresholds) *Matrix {
	best := make(map[string]*ModelEntry)

	for _, r := range rows {
		hint, _ := r.ModelInfo["model_hint"].(string)
		provider, _ := r.ModelInfo["provider_hint"].(string)

		modelName := FormatModelName(hint, r.RawText)
		if provider == "" {
			provider = InferProvider(modelName)
		}

		existing, ok := best[modelName]
		if !ok {
			entry := &ModelEntry{
				Model:        modelName,
				Provider:     provider,
				Client:       InferClient(r.RawText),
				ActiveParams: InferActiveParams(r.RawText),
				BestScore:    r.TotalScore,
				Result:       ClassifyResult(r.TotalScore, r.Status, t),
				Attempts:     1,
			}
			for _, s := range r.SectionScores {
				entry.SectionScores = append(entry.SectionScores, SectionScore{
					Section:  s.Section,
					Score:    s.Score,
					MaxScore: s.MaxScore,
				})
			}
			entry.MaxScore = sumMaxScores(entry.SectionScores)
			best[modelName] = entry
		} else {
			existing.Attempts++
			if r.TotalScore > existing.BestScore {
				existing.BestScore = r.TotalScore
				existing.Result = ClassifyResult(r.TotalScore, r.Status, t)
				existing.SectionScores = nil
				for _, s := range r.SectionScores {
					existing.SectionScores = append(existing.SectionScores, SectionScore{
						Section:  s.Section,
						Score:    s.Score,
						MaxScore: s.MaxScore,
					})
				}
				existing.MaxScore = sumMaxScores(existing.SectionScores)
			}
		}
	}

	// Convert to sorted slice: best score descending, then name ascending
	models := make([]ModelEntry, 0, len(best))
	for _, e := range best {
		models = append(models, *e)
	}
	sort.Slice(models, func(i, j int) bool {
		if models[i].BestScore != models[j].BestScore {
			return models[i].BestScore > models[j].BestScore
		}
		return models[i].Model < models[j].Model
	})

	return &Matrix{
		Models: models,
		Meta: Meta{
			TotalModels:          len(models),
			TotalAttempts:        len(rows),
			MaxPossibleScore:     t.MaxScore,
			SectionCount:         t.Sections,
			PassThreshold:        t.Pass,
			DistinctionThreshold: t.Distinction,
			GeneratedAt:          time.Now().UTC().Format(time.RFC3339),
		},
	}
}

func sumMaxScores(scores []SectionScore) int {
	total := 0
	for _, s := range scores {
		total += s.MaxScore
	}
	return total
}

// FormatModelName produces a display name from the model hint and raw step0 text.
func FormatModelName(hint, rawText string) string {
	lower := strings.ToLower(rawText)

	patterns := []struct {
		keyword string
		name    string
	}{
		{"claude opus 4.6", "Claude Opus 4.6"},
		{"claude opus 4", "Claude Opus 4"},
		{"claude sonnet", "Claude Sonnet"},
		{"chatgpt 5.2", "ChatGPT 5.2"},
		{"chatgpt 5", "ChatGPT 5"},
		{"chatgpt", "ChatGPT"},
		{"gpt-4o", "GPT-4o"},
		{"gpt-4", "GPT-4"},
		{"glm-4.5-air", "GLM-4.5-Air"},
		{"qwen3-coder-next", "Qwen3-Coder-Next"},
		{"qwen3-coder", "Qwen3-Coder"},
		{"qwen2.5-coder", "Qwen2.5-Coder"},
		{"qwen", "Qwen"},
		{"llama-4-scout", "Llama-4-Scout"},
		{"llama 4 scout", "Llama-4-Scout"},
		{"llama", "Llama"},
		{"granite 4.0", "Granite 4.0"},
		{"granite", "Granite"},
		{"deepseek", "DeepSeek"},
		{"gemini", "Gemini"},
		{"mistral", "Mistral"},
		{"grok", "Grok"},
	}

	for _, p := range patterns {
		if strings.Contains(lower, p.keyword) {
			name := p.name
			if strings.Contains(lower, "thinking disabled") || strings.Contains(lower, "nothink") {
				name += " (no thinking)"
			}
			return name
		}
	}

	// Fallback to hint with capitalization
	if hint != "" && hint != "unknown" {
		return strings.Title(hint) //nolint:staticcheck
	}

	// Last resort: extract agent name from "I am <Name>" pattern
	if name := extractAgentName(rawText); name != "" {
		return name
	}

	return "Unknown"
}

// extractAgentName pulls the agent name from "I am <Name>" in the step0 text.
func extractAgentName(rawText string) string {
	lower := strings.ToLower(rawText)
	prefixes := []string{"i am ", "my name is "}
	for _, prefix := range prefixes {
		idx := strings.Index(lower, prefix)
		if idx < 0 {
			continue
		}
		start := idx + len(prefix)
		rest := rawText[start:]
		end := len(rest)
		for _, sep := range []string{",", ".", "\n", " running", " and "} {
			if i := strings.Index(rest, sep); i >= 0 && i < end {
				end = i
			}
		}
		name := strings.TrimSpace(rest[:end])
		if name != "" {
			return name
		}
	}
	return ""
}

// InferProvider guesses the provider from a resolved model name.
func InferProvider(modelName string) string {
	lower := strings.ToLower(modelName)
	providerMap := []struct {
		keyword  string
		provider string
	}{
		{"claude", "anthropic"},
		{"chatgpt", "openai"},
		{"gpt-", "openai"},
		{"qwen", "alibaba"},
		{"llama", "meta"},
		{"granite", "ibm"},
		{"gemini", "google"},
		{"mistral", "mistral"},
		{"deepseek", "deepseek"},
		{"glm", "zhipu"},
		{"grok", "xai"},
	}
	for _, p := range providerMap {
		if strings.Contains(lower, p.keyword) {
			return p.provider
		}
	}
	return "--"
}

// InferClient guesses the MCP client from the step0 self-description.
func InferClient(rawText string) string {
	lower := strings.ToLower(rawText)
	if strings.Contains(lower, "claude code") {
		return "Claude Code"
	}
	if strings.Contains(lower, "opencode") {
		return "Opencode"
	}
	return "--"
}

// InferActiveParams extracts active parameter count from self-description text.
func InferActiveParams(rawText string) string {
	lower := strings.ToLower(rawText)

	patterns := []struct {
		keyword string
		params  string
	}{
		{"80b", "80B"},
		{"120b", "120B"},
		{"110b", "110B"},
		{"17b active", "17B active"},
		{"17b", "17B"},
		{"32b", "32B"},
		{"7b", "7B"},
	}

	for _, p := range patterns {
		if strings.Contains(lower, p.keyword) {
			return p.params
		}
	}
	return "--"
}

// ClassifyResult maps score and status to a result label.
func ClassifyResult(score int, status string, t Thresholds) string {
	if status == "failed" {
		return "fail"
	}
	if score >= t.Distinction {
		return "pass_distinction"
	}
	if score >= t.Pass {
		return "pass"
	}
	return "fail"
}
