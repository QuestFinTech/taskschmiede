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


package api

import (
	"sync"
	"time"

	"github.com/QuestFinTech/taskschmiede/internal/storage"
)

// velocityTracker tracks entity creation counts per user within a rolling
// hourly window. It provides a soft cap on how many entities an account
// can create per hour, protecting the platform from runaway creation by
// agent swarms or misbehaving clients.
type velocityTracker struct {
	mu      sync.Mutex
	buckets map[string]*velocityBucket
	window  time.Duration
	stop    chan struct{}
}

type velocityBucket struct {
	count       int
	windowStart time.Time
	lastAccess  time.Time
}

func newVelocityTracker() *velocityTracker {
	vt := &velocityTracker{
		buckets: make(map[string]*velocityBucket),
		window:  time.Hour,
		stop:    make(chan struct{}),
	}
	go vt.cleanupLoop()
	return vt
}

// record increments the creation count for the given user and returns the
// current count (after increment). If the window has expired, the counter
// resets before incrementing.
func (vt *velocityTracker) record(userID string) int {
	vt.mu.Lock()
	defer vt.mu.Unlock()

	now := storage.UTCNow()
	b, ok := vt.buckets[userID]
	if !ok {
		b = &velocityBucket{windowStart: now}
		vt.buckets[userID] = b
	}

	// Reset if window has expired.
	if now.Sub(b.windowStart) >= vt.window {
		b.count = 0
		b.windowStart = now
	}

	b.count++
	b.lastAccess = now
	return b.count
}

// current returns the creation count for the given user within the current
// window, without incrementing.
func (vt *velocityTracker) current(userID string) int {
	vt.mu.Lock()
	defer vt.mu.Unlock()

	b, ok := vt.buckets[userID]
	if !ok {
		return 0
	}

	now := storage.UTCNow()
	if now.Sub(b.windowStart) >= vt.window {
		return 0
	}
	return b.count
}

// cleanupLoop removes stale buckets every 10 minutes.
func (vt *velocityTracker) cleanupLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			vt.cleanup()
		case <-vt.stop:
			return
		}
	}
}

// cleanup removes buckets that have been idle for more than 2x the window
// duration.
func (vt *velocityTracker) cleanup() {
	vt.mu.Lock()
	defer vt.mu.Unlock()

	cutoff := storage.UTCNow().Add(-2 * vt.window)
	for id, b := range vt.buckets {
		if b.lastAccess.Before(cutoff) {
			delete(vt.buckets, id)
		}
	}
}

// Close stops the background cleanup goroutine.
func (vt *velocityTracker) Close() {
	close(vt.stop)
}
