// Package agentbridge implements the Cosca AgentBridge — the control plane
// that lets the Kernel supervise coding agents (OpenCode, Claude Code,
// Codex) as first-class sessions.
//
// Architecture reference: BRIDGE_ARCHITECTURE.md (based on the Vercel AI SDK
// harness pattern). This package is the FOUNDATION — the session lifecycle
// with the harness trinomial (detach/stop/suspendTurn), the bridge monitor
// (attach → rerun → replay), and the event stream the dashboard renders.
//
// The actual WebSocket bridge to OpenCode is phase 2; this package models
// the domain so the dashboard and the bridge can be built on top of it.
package agentbridge

import (
	"errors"
	"sync"
	"time"
)

// SessionStatus is the lifecycle state of an agent session.
type SessionStatus string

const (
	// StatusCreated: session created, no turn started yet.
	StatusCreated SessionStatus = "created"
	// StatusInTurn: a prompt turn is actively running.
	StatusInTurn SessionStatus = "in_turn"
	// StatusIdle: between turns, ready for the next prompt.
	StatusIdle SessionStatus = "idle"
	// StatusSuspended: detached/suspended (time-slice boundary), resumable.
	StatusSuspended SessionStatus = "suspended"
	// StatusStopped: stopped and persisted, not destroyed.
	StatusStopped SessionStatus = "stopped"
)

// Session models one agent session. It carries the harness-v1 trinomial:
// detach (park with resume coords), stop (persist + halt), destroy (clean up).
type Session struct {
	ID          string        `json:"id"`
	Agent       string        `json:"agent"`    // e.g. "opencode", "claude-code"
	Model       string        `json:"model"`    // e.g. "deepseek-v4-flash"
	Provider    string        `json:"provider"` // e.g. "deepseek"
	Status      SessionStatus `json:"status"`
	LastEventID int64         `json:"last_event_seq"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`

	// Resume holds the coordinates needed to re-attach a suspended session
	// (the bridge port, token, and last-seen event id).
	Resume *ResumeCoords `json:"resume,omitempty"`
}

// ResumeCoords are the durable resume coordinates of a detached session.
type ResumeCoords struct {
	BridgePort    int    `json:"bridge_port"`
	BridgeToken   string `json:"bridge_token"` // never logged; masked in UI
	LastSeenEvent int64  `json:"last_seen_event"`
	SandboxID     string `json:"sandbox_id"`
}

// ErrSessionNotFound is returned when an operation targets a missing session.
var ErrSessionNotFound = errors.New("session not found")

// ErrInvalidTransition is returned when a lifecycle transition is invalid
// (e.g. detach on a session that is already stopped).
var ErrInvalidTransition = errors.New("invalid session state transition")

// Store keeps all sessions in memory, goroutine-safe.
type Store struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	seq      int64 // global event sequence counter
}

// NewStore creates an empty session store.
func NewStore() *Store {
	return &Store{sessions: map[string]*Session{}}
}

// Create registers a new session in StatusCreated.
func (s *Store) Create(id, agent, model, provider string) *Session {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	sess := &Session{
		ID:        id,
		Agent:     agent,
		Model:     model,
		Provider:  provider,
		Status:    StatusCreated,
		CreatedAt: now,
		UpdatedAt: now,
	}
	s.sessions[id] = sess
	return sess
}

// Get returns a copy of a session.
func (s *Store) Get(id string) (*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sess, ok := s.sessions[id]
	if !ok {
		return nil, ErrSessionNotFound
	}
	cp := *sess
	return &cp, nil
}

// List returns a copy of all sessions, newest first.
func (s *Store) List() []Session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Session, 0, len(s.sessions))
	for _, sess := range s.sessions {
		out = append(out, *sess)
	}
	// Sort by CreatedAt descending (newest first).
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].CreatedAt.After(out[j-1].CreatedAt); j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// update applies fn to a session under lock. Returns ErrSessionNotFound if
// the session does not exist.
func (s *Store) update(id string, fn func(*Session) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[id]
	if !ok {
		return ErrSessionNotFound
	}
	if err := fn(sess); err != nil {
		return err
	}
	sess.UpdatedAt = time.Now().UTC()
	return nil
}

// StartTurn transitions a session to StatusInTurn.
func (s *Store) StartTurn(id string) error {
	return s.update(id, func(sess *Session) error {
		if sess.Status != StatusCreated && sess.Status != StatusIdle && sess.Status != StatusStopped {
			return ErrInvalidTransition
		}
		sess.Status = StatusInTurn
		return nil
	})
}

// EndTurn transitions a session back to StatusIdle.
func (s *Store) EndTurn(id string) error {
	return s.update(id, func(sess *Session) error {
		if sess.Status != StatusInTurn {
			return ErrInvalidTransition
		}
		sess.Status = StatusIdle
		return nil
	})
}

// Detach parks a session with resume coordinates (harness doDetach).
func (s *Store) Detach(id string, coords *ResumeCoords) error {
	return s.update(id, func(sess *Session) error {
		if sess.Status == StatusSuspended || sess.Status == StatusStopped {
			return ErrInvalidTransition
		}
		sess.Status = StatusSuspended
		sess.Resume = coords
		return nil
	})
}

// Resume re-attaches a suspended session (attach → rerun → replay).
func (s *Store) Resume(id string) error {
	return s.update(id, func(sess *Session) error {
		if sess.Status != StatusSuspended {
			return ErrInvalidTransition
		}
		sess.Status = StatusIdle
		return nil
	})
}

// Stop persists and halts a session (harness doStop) without destroying it.
func (s *Store) Stop(id string) error {
	return s.update(id, func(sess *Session) error {
		if sess.Status == StatusStopped || sess.Status == StatusSuspended {
			return ErrInvalidTransition
		}
		sess.Status = StatusStopped
		return nil
	})
}

// Destroy removes a session entirely (harness doDestroy).
func (s *Store) Destroy(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.sessions[id]; !ok {
		return ErrSessionNotFound
	}
	delete(s.sessions, id)
	return nil
}

// NextSeq returns the next global event sequence number (atomic-ish under
// the store lock; callers should not hold the lock while using it).
func (s *Store) NextSeq() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seq++
	return s.seq
}

// Count returns the number of tracked sessions.
func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.sessions)
}
