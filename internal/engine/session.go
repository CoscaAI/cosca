package engine

import (
	"bufio"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/CoscaAI/cosca/internal/chat"
)

// ─── Defaults ──────────────────────────────────────────────────────────────────

const (
	// defaultSessionsDir is the default sessions storage directory relative to
	// the workspace root.
	defaultSessionsDir = ".cosca/sessions"
)

// ─── SessionManager ───────────────────────────────────────────────────────────

// SessionManager manages persistent chat sessions as JSONL files in
// .cosca/sessions/. Each session is stored as a single JSONL file where every
// line is a JSON object representing one of:
//
//	{"type":"meta","id":"...","model":"...","agent":"...","created_at":"...","token_usage":{...},"metadata":{...}}
//	{"type":"message","role":"user","content":"...","timestamp":"..."}
//	{"type":"usage","prompt_tokens":0,"completion_tokens":0,"total_tokens":0}
//
// The meta line appears first, followed by zero or more message and usage lines
// in chronological order.
type SessionManager struct {
	sessionsDir string
	sessions    map[string]*Session
	mu          sync.RWMutex
}

// NewSessionManager creates a SessionManager that stores session files under the
// given directory. The directory is created on first write if it does not exist.
func NewSessionManager(sessionsDir string) *SessionManager {
	return &SessionManager{
		sessionsDir: sessionsDir,
		sessions:    make(map[string]*Session),
	}
}

// NewSessionManagerDefault creates a SessionManager using the default path
// ".cosca/sessions/" relative to the current working directory.
func NewSessionManagerDefault() *SessionManager {
	return NewSessionManager(defaultSessionsDir)
}

// ─── Session CRUD ─────────────────────────────────────────────────────────────

// CreateSession creates a new session with a UUID, the current timestamp, and
// the provided model and agent identifiers. The session is stored in memory
// but NOT written to disk until SaveSession is called.
func (sm *SessionManager) CreateSession(model, agent string) *Session {
	return sm.newSession(newUUID(), model, agent, "")
}

// CreateSessionWithID creates a new session with a caller-supplied ID instead
// of a randomly generated UUID. It is used for fixed-ID sessions — e.g. the
// voice wrapper's "cosca-voice" — where the session file must be named after a
// stable identifier so conversation memory persists across invocations.
func (sm *SessionManager) CreateSessionWithID(id, model, agent string) *Session {
	return sm.newSession(id, model, agent, "")
}

// CreateSessionWithParent creates a session with a caller-supplied ID whose
// meta records parentID as its parent_session_id (fork/lineage). Used by the
// /v1/run family to support session fork: the child session's history is
// seeded from the parent and its meta links back to it so sessionindex's
// SessionLineage can traverse the fork. The session is stored in memory but
// NOT written to disk until SaveSession is called.
func (sm *SessionManager) CreateSessionWithParent(id, model, agent, parentID string) *Session {
	return sm.newSession(id, model, agent, parentID)
}

// newSession builds a Session with the given ID, model, and agent, and caches
// it in memory under that ID. It is the shared constructor used by both
// CreateSession (random UUID) and CreateSessionWithID (fixed ID). The session
// is stored in memory but NOT written to disk until SaveSession is called.
func (sm *SessionManager) newSession(id, model, agent, parentID string) *Session {
	now := time.Now().UTC()
	s := &Session{
		ID:              id,
		Model:           model,
		Agent:           agent,
		ParentSessionID: parentID,
		Messages:        make([]chat.Message, 0),
		CreatedAt:       now,
		UpdatedAt:       now,
		Metadata:        make(map[string]string),
	}

	sm.mu.Lock()
	sm.sessions[s.ID] = s
	sm.mu.Unlock()

	return s
}

// GetSession returns a session from the in-memory cache by ID. It does NOT
// attempt to load the session from disk — call LoadSession for that.
func (sm *SessionManager) GetSession(id string) (*Session, error) {
	sm.mu.RLock()
	s, ok := sm.sessions[id]
	sm.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("session %q not found in memory", id)
	}
	return s, nil
}

// AppendMessage appends a message to the session's message list and updates
// the UpdatedAt timestamp. It does NOT immediately persist to disk.
func (sm *SessionManager) AppendMessage(sessionID string, msg chat.Message) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	s, ok := sm.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session %q not found", sessionID)
	}

	s.Messages = append(s.Messages, msg)
	s.UpdatedAt = time.Now().UTC()
	return nil
}

// DeleteSession removes a session from memory and deletes its JSONL file from
// disk. It is not an error if the file does not exist.
func (sm *SessionManager) DeleteSession(sessionID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	delete(sm.sessions, sessionID)

	path := filepath.Join(sm.sessionsDir, sessionID+".jsonl")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete session file: %w", err)
	}
	return nil
}

// ─── Persistence ──────────────────────────────────────────────────────────────

// SaveSession persists the session to disk as a JSONL file. The file path is
// {sessionsDir}/{sessionID}.jsonl. The directory is created if necessary.
func (sm *SessionManager) SaveSession(sessionID string) error {
	sm.mu.Lock()
	s, ok := sm.sessions[sessionID]
	if !ok {
		sm.mu.Unlock()
		return fmt.Errorf("session %q not found", sessionID)
	}

	// Copy all fields while holding the write lock to avoid race with
	// AppendMessage (which writes s.Messages and s.UpdatedAt under the
	// same lock). Without this copy, SaveSession would read s.Messages
	// after releasing the lock, racing with concurrent AppendMessage calls.
	id := s.ID
	model := s.Model
	agent := s.Agent
	parentID := s.ParentSessionID
	createdAt := s.CreatedAt
	updatedAt := s.UpdatedAt
	usage := s.TokenUsage
	messages := make([]chat.Message, len(s.Messages))
	copy(messages, s.Messages)
	metadata := make(map[string]string, len(s.Metadata))
	for k, v := range s.Metadata {
		metadata[k] = v
	}
	sm.mu.Unlock()

	// Ensure the sessions directory exists.
	dir := sm.sessionsDir
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create sessions dir: %w", err)
	}

	path := filepath.Join(dir, sessionID+".jsonl")
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create session file: %w", err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetEscapeHTML(false)

	// ── Line 1: meta ────────────────────────────────────────────────────────
	meta := jsonlLine{
		Type:            "meta",
		ID:              id,
		Model:           model,
		Agent:           agent,
		ParentSessionID: parentID,
		CreatedAt:       createdAt,
		UpdatedAt:       updatedAt,
		Usage:           usage,
		Metadata:        metadata,
	}
	if err := enc.Encode(meta); err != nil {
		return fmt.Errorf("encode meta line: %w", err)
	}

	// ── Lines 2..N: messages ────────────────────────────────────────────────
	now := time.Now().UTC()
	for _, m := range messages {
		line := jsonlLine{
			Type:         "message",
			Role:         m.Role,
			Content:      m.Content,
			ContentParts: m.ContentParts,
			Name:         m.Name,
			ToolCalls:    m.ToolCalls,
			ToolCallID:   m.ToolCallID,
			Timestamp:    now,
		}
		if err := enc.Encode(line); err != nil {
			return fmt.Errorf("encode message line: %w", err)
		}
	}

	// ── Last line: usage summary (only if non-zero) ─────────────────────────
	if usage.TotalTokens > 0 {
		usageLine := jsonlLine{
			Type:             "usage",
			PromptTokens:     usage.PromptTokens,
			CompletionTokens: usage.CompletionTokens,
			TotalTokens:      usage.TotalTokens,
		}
		if err := enc.Encode(usageLine); err != nil {
			return fmt.Errorf("encode usage line: %w", err)
		}
	}

	return nil
}

// LoadSession reads a session from a JSONL file on disk and caches it in
// memory. If the session is already cached, it is overwritten with the
// on-disk version.
func (sm *SessionManager) LoadSession(sessionID string) (*Session, error) {
	path := filepath.Join(sm.sessionsDir, sessionID+".jsonl")
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open session file: %w", err)
	}
	defer f.Close()

	s := &Session{
		ID:       sessionID,
		Messages: make([]chat.Message, 0),
		Metadata: make(map[string]string),
	}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024) // 1 MB line buffer

	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := scanner.Bytes()

		if len(line) == 0 {
			continue
		}

		var parsed jsonlLine
		if err := json.Unmarshal(line, &parsed); err != nil {
			return nil, fmt.Errorf("parse line %d: %w", lineNo, err)
		}

		switch parsed.Type {
		case "meta":
			s.ID = parsed.ID
			s.Model = parsed.Model
			s.Agent = parsed.Agent
			s.ParentSessionID = parsed.ParentSessionID
			s.CreatedAt = parsed.CreatedAt
			s.UpdatedAt = parsed.UpdatedAt
			s.TokenUsage = parsed.Usage
			if parsed.Metadata != nil {
				s.Metadata = parsed.Metadata
			}

		case "message":
			msg := chat.Message{
				Role:         parsed.Role,
				Content:      parsed.Content,
				ContentParts: parsed.ContentParts,
				Name:         parsed.Name,
				ToolCalls:    parsed.ToolCalls,
				ToolCallID:   parsed.ToolCallID,
			}
			s.Messages = append(s.Messages, msg)

		case "usage":
			s.TokenUsage = chat.Usage{
				PromptTokens:     parsed.PromptTokens,
				CompletionTokens: parsed.CompletionTokens,
				TotalTokens:      parsed.TotalTokens,
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read session file: %w", err)
	}

	// Cache in memory.
	sm.mu.Lock()
	sm.sessions[sessionID] = s
	sm.mu.Unlock()

	return s, nil
}

// ─── Listing & Resume ─────────────────────────────────────────────────────────

// ListSessions scans the sessions directory on disk and returns all sessions
// that have valid JSONL files. Sessions are returned in order of the creation
// timestamp parsed from the meta line (most recent first).
func (sm *SessionManager) ListSessions() ([]*Session, error) {
	entries, err := os.ReadDir(sm.sessionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*Session{}, nil
		}
		return nil, fmt.Errorf("read sessions dir: %w", err)
	}

	var sessions []*Session
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}

		id := strings.TrimSuffix(entry.Name(), ".jsonl")
		s, err := sm.LoadSession(id)
		if err != nil {
			// Skip corrupt files — log would go here in production.
			continue
		}
		sessions = append(sessions, s)
	}

	// Sort by most recently updated (or created) first.
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt)
	})

	return sessions, nil
}

// ResumeLatest loads the most recently updated session from disk into memory
// and returns it. If no sessions exist it returns nil without error. The model
// parameter is currently unused but reserved for filtering by model in future.
func (sm *SessionManager) ResumeLatest(model string) (*Session, error) {
	sessions, err := sm.ListSessions()
	if err != nil {
		return nil, err
	}

	if len(sessions) == 0 {
		return nil, nil
	}

	// ListSessions already sorts most-recent-first.
	return sessions[0], nil
}

// ─── JSONL Line ───────────────────────────────────────────────────────────────

// jsonlLine is a union struct used to encode/decode individual lines in the
// JSONL session file format. The Type field discriminates the payload.
type jsonlLine struct {
	Type string `json:"type"` // "meta", "message", "usage"

	// ── Meta fields ─────────────────────────────────────────────────────
	ID              string            `json:"id,omitempty"`
	Model           string            `json:"model,omitempty"`
	Agent           string            `json:"agent,omitempty"`
	ParentSessionID string            `json:"parent_session_id,omitempty"`
	CreatedAt       time.Time         `json:"created_at,omitempty"`
	UpdatedAt       time.Time         `json:"updated_at,omitempty"`
	Usage           chat.Usage        `json:"token_usage,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty"`

	// ── Message fields ──────────────────────────────────────────────────
	Role         chat.Role          `json:"role,omitempty"`
	Content      string             `json:"content,omitempty"`
	ContentParts []chat.ContentPart `json:"content_parts,omitempty"`
	Name         string             `json:"name,omitempty"`
	ToolCalls    []chat.ToolCall    `json:"tool_calls,omitempty"`
	ToolCallID   string             `json:"tool_call_id,omitempty"`
	Timestamp    time.Time          `json:"timestamp,omitempty"`

	// ── Usage fields ────────────────────────────────────────────────────
	PromptTokens     int `json:"prompt_tokens,omitempty"`
	CompletionTokens int `json:"completion_tokens,omitempty"`
	TotalTokens      int `json:"total_tokens,omitempty"`
}

// ─── UUID v4 (no external dependencies) ───────────────────────────────────────

// newUUID generates a UUID v4 string using crypto/rand.
func newUUID() string {
	uuid := make([]byte, 16)
	_, _ = rand.Read(uuid) // crypto/rand.Read never errors on modern Linux/Go

	uuid[6] = (uuid[6] & 0x0f) | 0x40 // Version 4
	uuid[8] = (uuid[8] & 0x3f) | 0x80 // Variant 10

	return fmt.Sprintf("%x-%x-%x-%x-%x",
		uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:],
	)
}
