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
	"sync"
	"sync/atomic"
	"time"
)

// Circuit breaker states used by ResilientClient.
const (
	stateClosed   int32 = 0 // normal operation: primary is healthy
	stateOpen     int32 = 1 // primary has failed: requests go to fallback
	stateHalfOpen int32 = 2 // cooldown elapsed: probing primary
	stateDisabled int32 = 3 // maintenance mode: all LLM calls skipped
)

// ErrDisabled is returned when the circuit breaker is in maintenance mode.
var ErrDisabled = fmt.Errorf("LLM calls disabled (maintenance mode)")

// ResilientStats holds circuit breaker metrics for reporting.
type ResilientStats struct {
	State                string  `json:"state"`
	PrimaryProvider      string  `json:"primary_provider"`
	PrimaryModel         string  `json:"primary_model"`
	PrimaryURL           string  `json:"primary_url,omitempty"`
	PrimaryStatus        string  `json:"primary_status"` // "in operation", "unavailable", "probing"
	FallbackProvider     string  `json:"fallback_provider,omitempty"`
	FallbackModel        string  `json:"fallback_model,omitempty"`
	FallbackURL          string  `json:"fallback_url,omitempty"`
	FallbackStatus       string  `json:"fallback_status,omitempty"` // "standing by", "in operation", "unavailable"
	GoHeuristicsStatus   string  `json:"go_heuristics_status"` // "standing by", "in operation"
	ConsecutiveFailures  int     `json:"consecutive_failures"`
	TotalPrimary         int64   `json:"total_primary"`
	TotalFallback        int64   `json:"total_fallback"`
	TotalFailures        int64   `json:"total_failures"`
	LastSuccess          string  `json:"last_success,omitempty"`
	LastTransition       string  `json:"last_transition,omitempty"`
	CooldownEnds         string  `json:"cooldown_ends,omitempty"`
}

// ResilientClient wraps a primary and optional fallback LLM client with
// circuit breaker logic. It implements the Client interface.
type ResilientClient struct {
	primary  Client
	fallback Client // nil = no fallback

	state          int32 // atomic: stateClosed, stateOpen, stateHalfOpen, stateDisabled
	failures       int32 // atomic: consecutive failure count
	lastSuccess    int64 // atomic: unix timestamp
	lastTransition int64 // atomic: unix timestamp
	openAt         int64 // atomic: when circuit opened
	mu             sync.Mutex

	primaryDisabled  int32 // atomic: 1 = admin disabled primary
	fallbackDisabled int32 // atomic: 1 = admin disabled fallback

	failThreshold  int
	cooldownPeriod time.Duration

	totalPrimary  int64 // atomic counter
	totalFallback int64 // atomic counter
	totalFailures int64 // atomic counter
}

// ResilientOption configures a ResilientClient.
type ResilientOption func(*ResilientClient)

// WithFailThreshold sets the number of consecutive failures before the
// circuit opens. Default is 3.
func WithFailThreshold(n int) ResilientOption {
	return func(rc *ResilientClient) {
		if n > 0 {
			rc.failThreshold = n
		}
	}
}

// WithCooldownPeriod sets how long the circuit stays open before
// transitioning to half-open. Default is 2 minutes.
func WithCooldownPeriod(d time.Duration) ResilientOption {
	return func(rc *ResilientClient) {
		if d > 0 {
			rc.cooldownPeriod = d
		}
	}
}

// NewResilientClient creates a circuit-breaker-wrapped client that routes
// requests to primary when healthy and falls back when the circuit is open.
func NewResilientClient(primary, fallback Client, opts ...ResilientOption) *ResilientClient {
	rc := &ResilientClient{
		primary:        primary,
		fallback:       fallback,
		failThreshold:  3,
		cooldownPeriod: 2 * time.Minute,
	}
	for _, opt := range opts {
		opt(rc)
	}
	return rc
}

// SetPrimaryDisabled toggles the primary LLM on or off. When disabled,
// requests go directly to the fallback. If both are disabled, returns ErrDisabled.
func (rc *ResilientClient) SetPrimaryDisabled(disabled bool) {
	if disabled {
		atomic.StoreInt32(&rc.primaryDisabled, 1)
	} else {
		atomic.StoreInt32(&rc.primaryDisabled, 0)
		// Reset circuit breaker state when re-enabling.
		rc.mu.Lock()
		atomic.StoreInt32(&rc.state, stateClosed)
		atomic.StoreInt32(&rc.failures, 0)
		rc.mu.Unlock()
	}
	atomic.StoreInt64(&rc.lastTransition, time.Now().UTC().Unix())
}

// SetFallbackDisabled toggles the fallback LLM on or off.
func (rc *ResilientClient) SetFallbackDisabled(disabled bool) {
	if disabled {
		atomic.StoreInt32(&rc.fallbackDisabled, 1)
	} else {
		atomic.StoreInt32(&rc.fallbackDisabled, 0)
	}
	atomic.StoreInt64(&rc.lastTransition, time.Now().UTC().Unix())
}

// IsPrimaryDisabled returns whether the primary LLM is admin-disabled.
func (rc *ResilientClient) IsPrimaryDisabled() bool {
	return atomic.LoadInt32(&rc.primaryDisabled) == 1
}

// IsFallbackDisabled returns whether the fallback LLM is admin-disabled.
func (rc *ResilientClient) IsFallbackDisabled() bool {
	return atomic.LoadInt32(&rc.fallbackDisabled) == 1
}

// Complete routes the request based on circuit state and admin overrides.
func (rc *ResilientClient) Complete(ctx context.Context, req *Request) (*Response, error) {
	pDisabled := atomic.LoadInt32(&rc.primaryDisabled) == 1
	fDisabled := atomic.LoadInt32(&rc.fallbackDisabled) == 1

	// Both disabled = maintenance mode.
	if pDisabled && (fDisabled || rc.fallback == nil) {
		return nil, ErrDisabled
	}

	// Primary admin-disabled: go straight to fallback.
	if pDisabled {
		return rc.callFallback(ctx, req)
	}

	state := atomic.LoadInt32(&rc.state)

	switch state {
	case stateClosed:
		return rc.callPrimaryWithFallback(ctx, req, fDisabled)

	case stateOpen:
		// Check if cooldown has elapsed for half-open probe.
		openAt := atomic.LoadInt64(&rc.openAt)
		if time.Now().UTC().Unix()-openAt >= int64(rc.cooldownPeriod.Seconds()) {
			rc.transitionTo(stateHalfOpen)
			return rc.callPrimaryWithFallback(ctx, req, fDisabled)
		}
		if fDisabled {
			return nil, ErrDisabled
		}
		return rc.callFallback(ctx, req)

	case stateHalfOpen:
		return rc.callPrimaryWithFallback(ctx, req, fDisabled)

	default:
		return rc.callPrimaryWithFallback(ctx, req, fDisabled)
	}
}

// callPrimaryWithFallback calls primary and falls back unless fallback is disabled.
func (rc *ResilientClient) callPrimaryWithFallback(ctx context.Context, req *Request, fbDisabled bool) (*Response, error) {
	atomic.AddInt64(&rc.totalPrimary, 1)
	resp, err := rc.primary.Complete(ctx, req)
	if err != nil {
		rc.recordFailure()
		if !fbDisabled && rc.fallback != nil {
			return rc.callFallback(ctx, req)
		}
		return nil, err
	}
	rc.recordSuccess()
	return resp, nil
}

// State returns the current circuit state as a string.
func (rc *ResilientClient) State() string {
	pDisabled := atomic.LoadInt32(&rc.primaryDisabled) == 1
	fDisabled := atomic.LoadInt32(&rc.fallbackDisabled) == 1
	if pDisabled && (fDisabled || rc.fallback == nil) {
		return "disabled"
	}
	switch atomic.LoadInt32(&rc.state) {
	case stateOpen:
		return "open"
	case stateHalfOpen:
		return "half-open"
	default:
		return "closed"
	}
}

// Stats returns all circuit breaker counters for admin reporting.
func (rc *ResilientClient) Stats() ResilientStats {
	currentState := atomic.LoadInt32(&rc.state)
	s := ResilientStats{
		State:               rc.State(),
		PrimaryProvider:     rc.primary.Provider(),
		PrimaryModel:        rc.primary.Model(),
		ConsecutiveFailures: int(atomic.LoadInt32(&rc.failures)),
		TotalPrimary:        atomic.LoadInt64(&rc.totalPrimary),
		TotalFallback:       atomic.LoadInt64(&rc.totalFallback),
		TotalFailures:       atomic.LoadInt64(&rc.totalFailures),
	}
	// Derive operational status from circuit state and admin overrides.
	pDisabled := atomic.LoadInt32(&rc.primaryDisabled) == 1
	fDisabled := atomic.LoadInt32(&rc.fallbackDisabled) == 1

	if pDisabled {
		s.PrimaryStatus = "disabled"
	} else {
		switch currentState {
		case stateClosed:
			s.PrimaryStatus = "in operation"
		case stateOpen:
			s.PrimaryStatus = "unavailable"
		case stateHalfOpen:
			s.PrimaryStatus = "probing"
		default:
			s.PrimaryStatus = "in operation"
		}
	}

	if rc.fallback != nil {
		if fDisabled {
			s.FallbackStatus = "disabled"
		} else if pDisabled || currentState == stateOpen {
			s.FallbackStatus = "in operation"
		} else {
			s.FallbackStatus = "standing by"
		}
	}

	// Go heuristics are in operation when both LLMs are unavailable.
	bothDown := pDisabled && (fDisabled || rc.fallback == nil)
	if bothDown {
		s.GoHeuristicsStatus = "in operation"
	} else {
		s.GoHeuristicsStatus = "standing by"
	}
	if rc.fallback != nil {
		s.FallbackProvider = rc.fallback.Provider()
		s.FallbackModel = rc.fallback.Model()
	}
	if ts := atomic.LoadInt64(&rc.lastSuccess); ts > 0 {
		s.LastSuccess = time.Unix(ts, 0).UTC().Format(time.RFC3339)
	}
	if ts := atomic.LoadInt64(&rc.lastTransition); ts > 0 {
		s.LastTransition = time.Unix(ts, 0).UTC().Format(time.RFC3339)
	}
	if currentState == stateOpen {
		openAt := atomic.LoadInt64(&rc.openAt)
		cooldownEnds := time.Unix(openAt, 0).UTC().Add(rc.cooldownPeriod)
		s.CooldownEnds = cooldownEnds.Format(time.RFC3339)
	}
	return s
}

// Provider delegates to the current active client's provider.
func (rc *ResilientClient) Provider() string {
	if atomic.LoadInt32(&rc.state) == stateOpen && rc.fallback != nil {
		return rc.fallback.Provider()
	}
	return rc.primary.Provider()
}

// Model delegates to the current active client's model.
func (rc *ResilientClient) Model() string {
	if atomic.LoadInt32(&rc.state) == stateOpen && rc.fallback != nil {
		return rc.fallback.Model()
	}
	return rc.primary.Model()
}

// callFallback calls the fallback client, or returns an error if none is configured.
func (rc *ResilientClient) callFallback(ctx context.Context, req *Request) (*Response, error) {
	if rc.fallback == nil {
		return nil, fmt.Errorf("circuit open and no fallback configured")
	}
	atomic.AddInt64(&rc.totalFallback, 1)
	resp, err := rc.fallback.Complete(ctx, req)
	if err != nil {
		atomic.AddInt64(&rc.totalFailures, 1)
		return nil, err
	}
	return resp, nil
}

// recordFailure increments the failure counter and potentially opens the circuit.
func (rc *ResilientClient) recordFailure() {
	atomic.AddInt64(&rc.totalFailures, 1)
	newCount := atomic.AddInt32(&rc.failures, 1)
	if int(newCount) >= rc.failThreshold {
		rc.transitionTo(stateOpen)
	}
}

// recordSuccess resets failure count and closes the circuit if half-open.
func (rc *ResilientClient) recordSuccess() {
	atomic.StoreInt32(&rc.failures, 0)
	atomic.StoreInt64(&rc.lastSuccess, time.Now().UTC().Unix())
	state := atomic.LoadInt32(&rc.state)
	if state == stateHalfOpen {
		rc.transitionTo(stateClosed)
	}
}

// transitionTo changes the circuit state with mutual exclusion.
func (rc *ResilientClient) transitionTo(newState int32) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	old := atomic.LoadInt32(&rc.state)
	if old == newState {
		return
	}
	atomic.StoreInt32(&rc.state, newState)
	atomic.StoreInt64(&rc.lastTransition, time.Now().UTC().Unix())
	if newState == stateOpen {
		atomic.StoreInt64(&rc.openAt, time.Now().UTC().Unix())
	}
}

var _ Client = (*ResilientClient)(nil)
