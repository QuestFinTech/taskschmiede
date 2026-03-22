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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// defaultOpenAIURL is the base URL for the OpenAI Chat Completions API.
const defaultOpenAIURL = "https://api.openai.com"

// openaiClient implements the Client interface using the OpenAI-compatible Chat Completions API.
type openaiClient struct {
	apiKey          string
	apiURL          string
	model           string
	maxTokens       int
	temperature     *float64
	reasoningEffort string
	reasoningTokens int
	client          *http.Client
}

func newOpenAIClient(cfg Config) *openaiClient {
	apiURL := cfg.APIURL
	if apiURL == "" {
		apiURL = defaultOpenAIURL
	}
	// Strip trailing slash for consistent URL building
	apiURL = strings.TrimRight(apiURL, "/")
	return &openaiClient{
		apiKey:          cfg.APIKey,
		apiURL:          apiURL,
		model:           cfg.Model,
		maxTokens:       cfg.MaxTokens,
		temperature:     cfg.Temperature,
		reasoningEffort: cfg.ReasoningEffort,
		reasoningTokens: cfg.ReasoningTokens,
		client:          &http.Client{Timeout: cfg.Timeout},
	}
}

func (c *openaiClient) Provider() string { return "openai" }
func (c *openaiClient) Model() string    { return c.model }

func (c *openaiClient) Complete(ctx context.Context, req *Request) (*Response, error) {
	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = c.maxTokens
	}

	messages := []map[string]string{}
	if req.SystemPrompt != "" {
		messages = append(messages, map[string]string{
			"role":    "system",
			"content": req.SystemPrompt,
		})
	}
	messages = append(messages, map[string]string{
		"role":    "user",
		"content": req.UserPrompt,
	})

	body := map[string]interface{}{
		"model":      c.model,
		"max_tokens": maxTokens,
		"messages":   messages,
	}
	if c.temperature != nil {
		body["temperature"] = *c.temperature
	}
	if c.reasoningEffort != "" {
		body["reasoning_effort"] = c.reasoningEffort
	}
	if c.reasoningTokens > 0 {
		body["reasoning_tokens"] = c.reasoningTokens
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// Support both /v1/chat/completions and endpoints that already include the path
	endpoint := c.apiURL + "/v1/chat/completions"
	if strings.Contains(c.apiURL, "/v1/") {
		endpoint = c.apiURL
		if !strings.HasSuffix(endpoint, "/chat/completions") {
			endpoint = strings.TrimRight(endpoint, "/") + "/chat/completions"
		}
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openai API call: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1 MB limit
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openai API error (HTTP %d): %s", resp.StatusCode, truncate(string(respBody), 500))
	}

	// Parse OpenAI Chat Completions API response
	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage *struct {
			TotalTokens int `json:"total_tokens"`
		} `json:"usage"`
		Timings *struct {
			PredictedMs float64 `json:"predicted_ms"`
		} `json:"timings"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("openai API returned no choices")
	}

	r := &Response{
		Content:      result.Choices[0].Message.Content,
		UsedProvider: c.Provider(),
		UsedModel:    c.model,
	}
	if result.Usage != nil {
		r.TotalTokens = result.Usage.TotalTokens
	}
	if result.Timings != nil {
		r.PredictedMs = result.Timings.PredictedMs
	}
	return r, nil
}

var _ Client = (*openaiClient)(nil)
