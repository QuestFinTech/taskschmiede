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


package compatibility

import (
	"testing"

	"github.com/QuestFinTech/taskschmiede/internal/storage"
)

var defaultThresholds = Thresholds{
	Pass:        96,
	Distinction: 144,
	MaxScore:    160,
	Sections:    8,
}

func TestBuildEmpty(t *testing.T) {
	m := Build(nil, defaultThresholds)
	if m.Meta.TotalModels != 0 {
		t.Errorf("expected 0 models, got %d", m.Meta.TotalModels)
	}
	if m.Meta.TotalAttempts != 0 {
		t.Errorf("expected 0 attempts, got %d", m.Meta.TotalAttempts)
	}
	if len(m.Models) != 0 {
		t.Errorf("expected empty models slice, got %d", len(m.Models))
	}
	if m.Meta.MaxPossibleScore != 160 {
		t.Errorf("expected max score 160, got %d", m.Meta.MaxPossibleScore)
	}
}

func TestBuildSingleModel(t *testing.T) {
	rows := []*storage.CompatibilityRow{
		{
			AttemptID:  "oba_1",
			Status:     "passed",
			TotalScore: 150,
			RawText:    "I am Claude, running on Claude Code with Claude Opus 4.6",
			ModelInfo:  map[string]interface{}{},
			SectionScores: []storage.CompatibilitySectionScore{
				{Section: 1, Score: 20, MaxScore: 20},
				{Section: 2, Score: 20, MaxScore: 20},
				{Section: 3, Score: 18, MaxScore: 20},
			},
		},
	}

	m := Build(rows, defaultThresholds)

	if m.Meta.TotalModels != 1 {
		t.Fatalf("expected 1 model, got %d", m.Meta.TotalModels)
	}
	if m.Meta.TotalAttempts != 1 {
		t.Errorf("expected 1 attempt, got %d", m.Meta.TotalAttempts)
	}

	e := m.Models[0]
	if e.Model != "Claude Opus 4.6" {
		t.Errorf("expected model 'Claude Opus 4.6', got %q", e.Model)
	}
	if e.Provider != "anthropic" {
		t.Errorf("expected provider 'anthropic', got %q", e.Provider)
	}
	if e.Client != "Claude Code" {
		t.Errorf("expected client 'Claude Code', got %q", e.Client)
	}
	if e.BestScore != 150 {
		t.Errorf("expected best score 150, got %d", e.BestScore)
	}
	if e.MaxScore != 60 {
		t.Errorf("expected max score 60 (sum of 3 sections), got %d", e.MaxScore)
	}
	if e.Result != "pass_distinction" {
		t.Errorf("expected result 'pass_distinction', got %q", e.Result)
	}
	if e.Attempts != 1 {
		t.Errorf("expected 1 attempt, got %d", e.Attempts)
	}
	if len(e.SectionScores) != 3 {
		t.Errorf("expected 3 section scores, got %d", len(e.SectionScores))
	}
}

func TestBuildMultipleAttemptsSameModel(t *testing.T) {
	rows := []*storage.CompatibilityRow{
		{
			AttemptID:  "oba_1",
			Status:     "passed",
			TotalScore: 120,
			RawText:    "I am running ChatGPT 5.2 via Opencode",
			ModelInfo:  map[string]interface{}{},
			SectionScores: []storage.CompatibilitySectionScore{
				{Section: 1, Score: 15, MaxScore: 20},
			},
		},
		{
			AttemptID:  "oba_2",
			Status:     "passed",
			TotalScore: 156,
			RawText:    "I am running ChatGPT 5.2 via Opencode",
			ModelInfo:  map[string]interface{}{},
			SectionScores: []storage.CompatibilitySectionScore{
				{Section: 1, Score: 20, MaxScore: 20},
			},
		},
	}

	m := Build(rows, defaultThresholds)

	if m.Meta.TotalModels != 1 {
		t.Fatalf("expected 1 model, got %d", m.Meta.TotalModels)
	}
	if m.Meta.TotalAttempts != 2 {
		t.Errorf("expected 2 attempts, got %d", m.Meta.TotalAttempts)
	}

	e := m.Models[0]
	if e.BestScore != 156 {
		t.Errorf("expected best score 156, got %d", e.BestScore)
	}
	if e.Attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", e.Attempts)
	}
	if e.SectionScores[0].Score != 20 {
		t.Errorf("expected section score from best attempt (20), got %d", e.SectionScores[0].Score)
	}
}

func TestBuildSortOrder(t *testing.T) {
	rows := []*storage.CompatibilityRow{
		{
			AttemptID:  "oba_1",
			Status:     "passed",
			TotalScore: 100,
			RawText:    "I am running Qwen3-Coder-Next via Opencode",
			ModelInfo:  map[string]interface{}{},
		},
		{
			AttemptID:  "oba_2",
			Status:     "passed",
			TotalScore: 160,
			RawText:    "I am Claude, running Claude Opus 4.6 via Claude Code",
			ModelInfo:  map[string]interface{}{},
		},
		{
			AttemptID:  "oba_3",
			Status:     "passed",
			TotalScore: 160,
			RawText:    "I am running ChatGPT 5.2 via Opencode",
			ModelInfo:  map[string]interface{}{},
		},
	}

	m := Build(rows, defaultThresholds)

	if len(m.Models) != 3 {
		t.Fatalf("expected 3 models, got %d", len(m.Models))
	}
	// Same score: alphabetical by name
	if m.Models[0].Model != "ChatGPT 5.2" {
		t.Errorf("expected ChatGPT 5.2 first (alphabetical tie-break), got %q", m.Models[0].Model)
	}
	if m.Models[1].Model != "Claude Opus 4.6" {
		t.Errorf("expected Claude Opus 4.6 second, got %q", m.Models[1].Model)
	}
	// Lower score last
	if m.Models[2].Model != "Qwen3-Coder-Next" {
		t.Errorf("expected Qwen3-Coder-Next last, got %q", m.Models[2].Model)
	}
}

func TestClassifyResult(t *testing.T) {
	tests := []struct {
		score  int
		status string
		want   string
	}{
		{160, "passed", "pass_distinction"},
		{144, "passed", "pass_distinction"},
		{143, "passed", "pass"},
		{96, "passed", "pass"},
		{95, "passed", "fail"},
		{0, "passed", "fail"},
		{160, "failed", "fail"},
		{144, "failed", "fail"},
	}

	for _, tt := range tests {
		got := ClassifyResult(tt.score, tt.status, defaultThresholds)
		if got != tt.want {
			t.Errorf("ClassifyResult(%d, %q) = %q, want %q", tt.score, tt.status, got, tt.want)
		}
	}
}

func TestFormatModelName(t *testing.T) {
	tests := []struct {
		hint    string
		rawText string
		want    string
	}{
		{"", "I am Claude, running Claude Opus 4.6", "Claude Opus 4.6"},
		{"", "I am running ChatGPT 5.2", "ChatGPT 5.2"},
		{"", "GLM-4.5-Air model", "GLM-4.5-Air"},
		{"", "I use Qwen3-Coder-Next", "Qwen3-Coder-Next"},
		{"", "GLM-4.5-Air with thinking disabled", "GLM-4.5-Air (no thinking)"},
		{"", "GLM-4.5-Air nothink mode", "GLM-4.5-Air (no thinking)"},
		{"custom-model", "No known model patterns here", "Custom-Model"},
		{"", "I am TestBot, running tests", "TestBot"},
		{"", "", "Unknown"},
		{"unknown", "", "Unknown"},
	}

	for _, tt := range tests {
		got := FormatModelName(tt.hint, tt.rawText)
		if got != tt.want {
			t.Errorf("FormatModelName(%q, %q) = %q, want %q", tt.hint, tt.rawText, got, tt.want)
		}
	}
}

func TestInferProvider(t *testing.T) {
	tests := []struct {
		model string
		want  string
	}{
		{"Claude Opus 4.6", "anthropic"},
		{"ChatGPT 5.2", "openai"},
		{"GPT-4o", "openai"},
		{"Qwen3-Coder-Next", "alibaba"},
		{"Llama-4-Scout", "meta"},
		{"Granite 4.0", "ibm"},
		{"GLM-4.5-Air", "zhipu"},
		{"Grok", "xai"},
		{"Unknown Model", "--"},
	}

	for _, tt := range tests {
		got := InferProvider(tt.model)
		if got != tt.want {
			t.Errorf("InferProvider(%q) = %q, want %q", tt.model, got, tt.want)
		}
	}
}

func TestInferClient(t *testing.T) {
	tests := []struct {
		rawText string
		want    string
	}{
		{"Running on Claude Code", "Claude Code"},
		{"Using opencode client", "Opencode"},
		{"Some other client", "--"},
	}

	for _, tt := range tests {
		got := InferClient(tt.rawText)
		if got != tt.want {
			t.Errorf("InferClient(%q) = %q, want %q", tt.rawText, got, tt.want)
		}
	}
}

func TestInferActiveParams(t *testing.T) {
	tests := []struct {
		rawText string
		want    string
	}{
		{"80B parameter model", "80B"},
		{"120B quantized", "120B"},
		{"17B active parameters", "17B active"},
		{"Cloud model, no params", "--"},
	}

	for _, tt := range tests {
		got := InferActiveParams(tt.rawText)
		if got != tt.want {
			t.Errorf("InferActiveParams(%q) = %q, want %q", tt.rawText, got, tt.want)
		}
	}
}

func TestBuildMetaThresholds(t *testing.T) {
	m := Build(nil, defaultThresholds)

	if m.Meta.PassThreshold != 96 {
		t.Errorf("expected pass threshold 96, got %d", m.Meta.PassThreshold)
	}
	if m.Meta.DistinctionThreshold != 144 {
		t.Errorf("expected distinction threshold 144, got %d", m.Meta.DistinctionThreshold)
	}
	if m.Meta.SectionCount != 8 {
		t.Errorf("expected 8 sections, got %d", m.Meta.SectionCount)
	}
	if m.Meta.GeneratedAt == "" {
		t.Error("expected non-empty generated_at")
	}
}
