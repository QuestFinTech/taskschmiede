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


package llmclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAnthropicClient(t *testing.T) {
	// Mock Anthropic API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify headers
		if r.Header.Get("x-api-key") != "test-key" {
			t.Error("missing or wrong x-api-key header")
		}
		if r.Header.Get("anthropic-version") != "2023-06-01" {
			t.Error("missing or wrong anthropic-version header")
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Error("missing Content-Type header")
		}

		// Parse request body
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if body["model"] != "claude-sonnet-4-5-20250929" {
			t.Errorf("unexpected model: %v", body["model"])
		}

		// Return mock response
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"content": []map[string]interface{}{
				{"type": "text", "text": `{"injection_detected": false, "confidence": 0.0, "evidence": []}`},
			},
		})
	}))
	defer server.Close()

	client, err := NewClient(Config{
		Provider:  "anthropic",
		Model:     "claude-sonnet-4-5-20250929",
		APIKey:    "test-key",
		APIURL:    server.URL,
		Timeout:   5 * time.Second,
		MaxTokens: 512,
	})
	if err != nil {
		t.Fatalf("create client: %v", err)
	}

	if client.Provider() != "anthropic" {
		t.Errorf("expected provider 'anthropic', got %s", client.Provider())
	}

	resp, err := client.Complete(context.Background(), &Request{
		SystemPrompt: "You are a test reviewer.",
		UserPrompt:   "Analyze this text.",
		MaxTokens:    256,
	})
	if err != nil {
		t.Fatalf("complete: %v", err)
	}

	if resp.Content == "" {
		t.Error("expected non-empty response content")
	}
}

func TestOpenAIClient(t *testing.T) {
	// Mock OpenAI API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Error("missing or wrong Authorization header")
		}

		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}

		// Verify system message is first
		messages := body["messages"].([]interface{})
		first := messages[0].(map[string]interface{})
		if first["role"] != "system" {
			t.Errorf("expected system role first, got %s", first["role"])
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"content": `{"injection_detected": true, "confidence": 0.9, "evidence": ["test"]}`,
					},
				},
			},
		})
	}))
	defer server.Close()

	client, err := NewClient(Config{
		Provider:  "openai",
		Model:     "gpt-4",
		APIKey:    "test-key",
		APIURL:    server.URL,
		Timeout:   5 * time.Second,
		MaxTokens: 512,
	})
	if err != nil {
		t.Fatalf("create client: %v", err)
	}

	if client.Provider() != "openai" {
		t.Errorf("expected provider 'openai', got %s", client.Provider())
	}
	if client.Model() != "gpt-4" {
		t.Errorf("expected model 'gpt-4', got %s", client.Model())
	}

	resp, err := client.Complete(context.Background(), &Request{
		SystemPrompt: "You are a test reviewer.",
		UserPrompt:   "Analyze this text.",
	})
	if err != nil {
		t.Fatalf("complete: %v", err)
	}

	if resp.Content == "" {
		t.Error("expected non-empty response content")
	}
}

func TestOpenAIClient_NoAuth(t *testing.T) {
	// Test that requests work without API key (for local models)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Error("expected no Authorization header when API key is empty")
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]interface{}{"content": "ok"}},
			},
		})
	}))
	defer server.Close()

	client, err := NewClient(Config{
		Provider: "openai",
		Model:    "local-model",
		APIKey:   "", // no key
		APIURL:   server.URL,
		Timeout:  5 * time.Second,
	})
	if err != nil {
		t.Fatalf("create client: %v", err)
	}

	resp, err := client.Complete(context.Background(), &Request{
		UserPrompt: "test",
	})
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if resp.Content != "ok" {
		t.Errorf("expected 'ok', got %s", resp.Content)
	}
}

func TestAnthropicClient_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error": {"message": "rate limited"}}`))
	}))
	defer server.Close()

	client, _ := NewClient(Config{
		Provider: "anthropic",
		Model:    "claude-sonnet-4-5-20250929",
		APIKey:   "test-key",
		APIURL:   server.URL,
		Timeout:  5 * time.Second,
	})

	_, err := client.Complete(context.Background(), &Request{
		UserPrompt: "test",
	})
	if err == nil {
		t.Fatal("expected error for 429 response")
	}
}

func TestUnsupportedProvider(t *testing.T) {
	_, err := NewClient(Config{
		Provider: "gemini",
		Model:    "gemini-pro",
	})
	if err == nil {
		t.Fatal("expected error for unsupported provider")
	}
}
