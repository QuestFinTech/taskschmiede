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


// Package ticker provides a periodic task runner with handler registration.
// Handlers run at configurable intervals, checked each tick of the base loop.
package ticker

import (
	"context"
	"log/slog"
	"time"

	"github.com/QuestFinTech/taskschmiede/internal/storage"
)

// Handler defines a periodic task that runs at a fixed interval.
type Handler struct {
	Name     string
	Interval time.Duration
	Fn       func(ctx context.Context, now time.Time) error
}

// handlerState tracks per-handler execution state.
type handlerState struct {
	handler Handler
	lastRun time.Time
}

// Ticker runs registered handlers at their configured intervals.
type Ticker struct {
	logger   *slog.Logger
	handlers []handlerState
	interval time.Duration
}

// New creates a Ticker with the given base tick interval.
// The interval controls how often the ticker checks if handlers are due.
func New(logger *slog.Logger, interval time.Duration) *Ticker {
	if interval <= 0 {
		interval = time.Second
	}
	return &Ticker{
		logger:   logger,
		interval: interval,
	}
}

// Register adds a handler to the ticker. Must be called before Run.
func (t *Ticker) Register(h Handler) {
	t.handlers = append(t.handlers, handlerState{handler: h})
	t.logger.Info("Ticker handler registered",
		"handler", h.Name,
		"interval", h.Interval,
	)
}

// Run starts the ticker loop. It blocks until ctx is canceled.
func (t *Ticker) Run(ctx context.Context) {
	tick := time.NewTicker(t.interval)
	defer tick.Stop()

	t.logger.Info("Ticker started",
		"interval", t.interval,
		"handlers", len(t.handlers),
	)

	for {
		select {
		case <-ctx.Done():
			t.logger.Info("Ticker stopped")
			return
		case now := <-tick.C:
			now = now.UTC()
			for i := range t.handlers {
				hs := &t.handlers[i]
				if storage.UTCNow().Sub(hs.lastRun) >= hs.handler.Interval {
					if err := hs.handler.Fn(ctx, now); err != nil {
						t.logger.Warn("Ticker handler error",
							"handler", hs.handler.Name,
							"error", err,
						)
					} else {
						t.logger.Debug("Ticker handler executed",
							"handler", hs.handler.Name,
						)
					}
					hs.lastRun = storage.UTCNow()
				}
			}
		}
	}
}
