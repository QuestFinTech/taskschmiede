// Copyright 2026 Quest Financial Technologies S.a r.l.-S., Luxembourg
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
	"fmt"
	"testing"
	"time"
)

// mockClient is a test helper that records calls and returns configured responses.
type mockClient struct {
	provider string
	model    string
	response *Response
	err      error
	calls    int
}

func (m *mockClient) Complete(_ context.Context, _ *Request) (*Response, error) {
	m.calls++
	return m.response, m.err
}

func (m *mockClient) Provider() string { return m.provider }
func (m *mockClient) Model() string    { return m.model }

func TestResilientClient_ClosedRoutesToPrimary(t *testing.T) {
	primary := &mockClient{
		provider: "openai",
		model:    "test-large-model",
		response: &Response{Content: "primary response"},
	}
	fallback := &mockClient{
		provider: "openai",
		model:    "phi-4-mini",
		response: &Response{Content: "fallback response"},
	}

	rc := NewResilientClient(primary, fallback)

	resp, err := rc.Complete(context.Background(), &Request{UserPrompt: "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "primary response" {
		t.Errorf("expected primary response, got %q", resp.Content)
	}
	if primary.calls != 1 {
		t.Errorf("expected 1 primary call, got %d", primary.calls)
	}
	if fallback.calls != 0 {
		t.Errorf("expected 0 fallback calls, got %d", fallback.calls)
	}
	if rc.State() != "closed" {
		t.Errorf("expected state closed, got %s", rc.State())
	}
}

func TestResilientClient_TripsAfterNFailures(t *testing.T) {
	primary := &mockClient{
		provider: "openai",
		model:    "test-large-model",
		err:      fmt.Errorf("connection refused"),
	}
	fallback := &mockClient{
		provider: "openai",
		model:    "phi-4-mini",
		response: &Response{Content: "fallback response"},
	}

	rc := NewResilientClient(primary, fallback, WithFailThreshold(3))

	for i := 0; i < 3; i++ {
		resp, err := rc.Complete(context.Background(), &Request{UserPrompt: "test"})
		if err != nil {
			t.Fatalf("expected fallback to succeed, got error: %v", err)
		}
		if resp.Content != "fallback response" {
			t.Errorf("expected fallback response, got %q", resp.Content)
		}
	}

	if rc.State() != "open" {
		t.Errorf("expected state open after 3 failures, got %s", rc.State())
	}
}

func TestResilientClient_OpenRoutesToFallback(t *testing.T) {
	primary := &mockClient{
		provider: "openai",
		model:    "test-large-model",
		err:      fmt.Errorf("connection refused"),
	}
	fallback := &mockClient{
		provider: "openai",
		model:    "phi-4-mini",
		response: &Response{Content: "fallback response"},
	}

	rc := NewResilientClient(primary, fallback, WithFailThreshold(2), WithCooldownPeriod(5*time.Minute))

	// Trip the circuit.
	for i := 0; i < 2; i++ {
		_, _ = rc.Complete(context.Background(), &Request{UserPrompt: "test"})
	}
	if rc.State() != "open" {
		t.Fatalf("expected state open, got %s", rc.State())
	}

	// Reset primary call count and make it succeed.
	primary.calls = 0
	primary.err = nil
	primary.response = &Response{Content: "primary response"}
	fallback.calls = 0

	// Next call should go to fallback (cooldown not elapsed).
	resp, err := rc.Complete(context.Background(), &Request{UserPrompt: "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "fallback response" {
		t.Errorf("expected fallback response while open, got %q", resp.Content)
	}
	if primary.calls != 0 {
		t.Errorf("expected 0 primary calls while open, got %d", primary.calls)
	}
	if fallback.calls != 1 {
		t.Errorf("expected 1 fallback call, got %d", fallback.calls)
	}
}

func TestResilientClient_HalfOpenProbesPrimary(t *testing.T) {
	primary := &mockClient{
		provider: "openai",
		model:    "test-large-model",
		err:      fmt.Errorf("connection refused"),
	}
	fallback := &mockClient{
		provider: "openai",
		model:    "phi-4-mini",
		response: &Response{Content: "fallback response"},
	}

	// Use very short cooldown for test.
	rc := NewResilientClient(primary, fallback,
		WithFailThreshold(2),
		WithCooldownPeriod(1*time.Millisecond),
	)

	// Trip the circuit.
	for i := 0; i < 2; i++ {
		_, _ = rc.Complete(context.Background(), &Request{UserPrompt: "test"})
	}
	if rc.State() != "open" {
		t.Fatalf("expected state open, got %s", rc.State())
	}

	// Wait for cooldown.
	time.Sleep(10 * time.Millisecond)

	// Primary still fails -- should fall back but transition to half-open first.
	primary.calls = 0
	fallback.calls = 0

	resp, err := rc.Complete(context.Background(), &Request{UserPrompt: "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "fallback response" {
		t.Errorf("expected fallback response on half-open probe failure, got %q", resp.Content)
	}
	if primary.calls != 1 {
		t.Errorf("expected 1 primary call (probe), got %d", primary.calls)
	}
}

func TestResilientClient_RecoveryClosesCircuit(t *testing.T) {
	callCount := 0
	primary := &mockClient{
		provider: "openai",
		model:    "test-large-model",
		err:      fmt.Errorf("connection refused"),
	}
	fallback := &mockClient{
		provider: "openai",
		model:    "phi-4-mini",
		response: &Response{Content: "fallback response"},
	}

	rc := NewResilientClient(primary, fallback,
		WithFailThreshold(2),
		WithCooldownPeriod(1*time.Millisecond),
	)

	// Trip the circuit.
	for i := 0; i < 2; i++ {
		_, _ = rc.Complete(context.Background(), &Request{UserPrompt: "test"})
		callCount++
	}

	// Wait for cooldown.
	time.Sleep(10 * time.Millisecond)

	// Now primary recovers.
	primary.err = nil
	primary.response = &Response{Content: "primary recovered"}
	primary.calls = 0

	resp, err := rc.Complete(context.Background(), &Request{UserPrompt: "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "primary recovered" {
		t.Errorf("expected primary recovered, got %q", resp.Content)
	}
	if rc.State() != "closed" {
		t.Errorf("expected state closed after recovery, got %s", rc.State())
	}
}

func TestResilientClient_Stats(t *testing.T) {
	primary := &mockClient{
		provider: "openai",
		model:    "test-large-model",
		response: &Response{Content: "ok"},
	}
	fallback := &mockClient{
		provider: "openai",
		model:    "phi-4-mini",
		response: &Response{Content: "ok"},
	}

	rc := NewResilientClient(primary, fallback)

	// Make a few successful calls.
	for i := 0; i < 3; i++ {
		_, _ = rc.Complete(context.Background(), &Request{UserPrompt: "test"})
	}

	stats := rc.Stats()
	if stats.State != "closed" {
		t.Errorf("expected state closed, got %s", stats.State)
	}
	if stats.PrimaryModel != "test-large-model" {
		t.Errorf("expected primary model test-large-model, got %s", stats.PrimaryModel)
	}
	if stats.FallbackModel != "phi-4-mini" {
		t.Errorf("expected fallback model phi-4-mini, got %s", stats.FallbackModel)
	}
	if stats.TotalPrimary != 3 {
		t.Errorf("expected 3 total primary, got %d", stats.TotalPrimary)
	}
	if stats.TotalFallback != 0 {
		t.Errorf("expected 0 total fallback, got %d", stats.TotalFallback)
	}
	if stats.TotalFailures != 0 {
		t.Errorf("expected 0 total failures, got %d", stats.TotalFailures)
	}
	if stats.ConsecutiveFailures != 0 {
		t.Errorf("expected 0 consecutive failures, got %d", stats.ConsecutiveFailures)
	}
	if stats.LastSuccess == "" {
		t.Error("expected last_success to be set")
	}
}

func TestResilientClient_NoFallback(t *testing.T) {
	primary := &mockClient{
		provider: "openai",
		model:    "test-large-model",
		err:      fmt.Errorf("connection refused"),
	}

	rc := NewResilientClient(primary, nil, WithFailThreshold(2))

	_, err := rc.Complete(context.Background(), &Request{UserPrompt: "test"})
	if err == nil {
		t.Fatal("expected error with no fallback")
	}
}

func TestResilientClient_ProviderModel(t *testing.T) {
	primary := &mockClient{provider: "openai", model: "test-large-model"}
	fallback := &mockClient{provider: "openai", model: "phi-4-mini"}

	rc := NewResilientClient(primary, fallback)

	if rc.Provider() != "openai" {
		t.Errorf("expected provider openai, got %s", rc.Provider())
	}
	if rc.Model() != "test-large-model" {
		t.Errorf("expected model test-large-model, got %s", rc.Model())
	}
}
