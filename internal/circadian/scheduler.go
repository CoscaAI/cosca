// Package circadian implements the Cosca Operational Rest Cycle (ORC).
//
// This file implements the ORC scheduler: the component that drives a
// circadian.Engine along its idle-based state machine, opens the ORC window
// when the engine has been quiet long enough, runs the rest cycle exactly
// once per window and re-arms the cycle once the engine returns to the awake
// state. Nomenclature: "rest cycle", never "the AI sleeps".
package circadian

import (
	"context"
	"runtime/debug"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// Scheduler drives a single circadian.Engine on a fixed tick. It proposes
// state changes from the idle clock, runs the ORC maintenance pipeline once
// per sleep window and re-arms the gate after the engine wakes. It is safe
// for concurrent use.
type Scheduler struct {
	engine   *Engine
	coscaDir string
	interval time.Duration
	done     chan struct{}
	mu       sync.Mutex
	running  bool
	lastORC  *ORCResult
	ranORC   bool
}

// NewScheduler returns a scheduler driving engine against coscaDir with the
// default evaluation interval of 30 seconds.
func NewScheduler(engine *Engine, coscaDir string) *Scheduler {
	return &Scheduler{
		engine:   engine,
		coscaDir: coscaDir,
		interval: 30 * time.Second,
	}
}

// Start launches the evaluation loop. Each tick:
//
//  1. evaluates the proposed state from the idle clock and transitions the
//     engine, walking the transition matrix (awake -> idle -> resting ->
//     sleeping);
//  2. while the engine is in the ORC window and the rest cycle has not run
//     in this window, runs RunORC once — the cycle is never repeated inside
//     the same sleep window;
//  3. when the engine returns to the awake state (for example because the
//     Don interacted again and RecordActivity reset the idle clock), re-arms
//     the ORC gate so the next window runs a fresh cycle.
//
// The loop exits when ctx is cancelled or Stop is called. Starting an already
// running scheduler is a no-op.
func (s *Scheduler) Start(ctx context.Context) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	done := make(chan struct{})
	s.done = done
	s.mu.Unlock()

	ticker := time.NewTicker(s.interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-done:
				return
			case <-ticker.C:
				s.tickGuarded(ctx)
			}
		}
	}()
}

// tickGuarded runs a single scheduler pass inside a panic boundary so a
// panic in Evaluate or RunORC never kills the daemon: the panic is logged
// with a stack trace and the evaluation loop continues on the next tick.
func (s *Scheduler) tickGuarded(ctx context.Context) {
	defer func() {
		if rec := recover(); rec != nil {
			log.Error().
				Interface("panic", rec).
				Str("stack", string(debug.Stack())).
				Msg("circadian scheduler tick panic recovered")
		}
	}()
	s.tick(ctx)
}

// Stop halts the evaluation loop and stops the ticker. It is safe to call
// multiple times and from any goroutine.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running {
		return
	}
	s.running = false
	close(s.done)
}

// LastORC returns the most recent rest cycle result, or nil if the ORC has
// not run yet.
func (s *Scheduler) LastORC() *ORCResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastORC
}

// ran reports whether the ORC already ran in the current sleep window.
func (s *Scheduler) ran() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ranORC
}

// tick performs a single scheduler pass.
func (s *Scheduler) tick(ctx context.Context) {
	e := s.engine

	// ROBUSTEZ (2026-09-09): sem engine o scheduler não tem o que avaliar.
	// Antes, `e.Evaluate()` panicava (nil deref) em todo tick — recuperado no
	// tickGuarded (o daemon não morria) mas poluía de erro e escondia o estado.
	// Agora o pass é um no-op seguro: o daemon aguarda o engine ser injetado.
	if e == nil {
		return
	}

	// 1. Walk the state machine toward the idle clock's proposal.
	if proposed := e.Evaluate(); proposed != e.State() {
		_ = e.TransitionTo(proposed, "idle evaluation")
	}

	// 2. ORC window: run the rest cycle exactly once per sleep window.
	s.mu.Lock()
	inWindow := e.State() == StateSleeping && !s.ranORC && e.CanRun(ActivityORC)
	s.mu.Unlock()
	if inWindow {
		result, _ := RunORC(ctx, s.coscaDir)
		s.mu.Lock()
		if result != nil {
			s.lastORC = result
		}
		s.ranORC = true
		s.mu.Unlock()
	}

	// 3. The engine returned to the awake state (RecordActivity reset the
	//    idle clock): arm the ORC gate again for the next sleep window.
	s.mu.Lock()
	if e.State() == StateAwake && s.ranORC {
		s.ranORC = false
	}
	s.mu.Unlock()
}
