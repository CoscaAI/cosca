package cosca

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/CoscaAI/cosca/internal/safe"
)

// =============================================================================
// Types
// =============================================================================

// RunOption is a functional option for configuring AI orchestration runs.
type RunOption func(*runConfig)

// runConfig holds internal configuration for a run/stream request.
type runConfig struct {
	// Agent is the name of the agent to use for this run.
	Agent string
	// Provider is the name of the LLM provider to use.
	Provider string
	// Session is the conversation session_id (ETAPA 2). When empty the
	// server generates one and returns it, so the client can resume.
	Session string
	// ParentSessionID, when set, asks the server to FORK the given parent
	// session (the child inherits the parent's history and records the
	// parent_session_id in its meta for lineage).
	ParentSessionID string
}

// WithAgent sets the agent for the orchestration run.
func WithAgent(agent string) RunOption {
	return func(c *runConfig) {
		c.Agent = agent
	}
}

// WithProvider sets the AI provider for the orchestration run.
func WithProvider(provider string) RunOption {
	return func(c *runConfig) {
		c.Provider = provider
	}
}

// WithSession sets the conversation session_id (ETAPA 2). Passing the same
// id on a subsequent call resumes the persisted conversation; omit it to let
// the server generate and return a fresh session_id.
func WithSession(sessionID string) RunOption {
	return func(c *runConfig) {
		c.Session = sessionID
	}
}

// WithParentSession requests a fork: the server creates a child session whose
// history is seeded from the parent identified by parentSessionID.
func WithParentSession(parentSessionID string) RunOption {
	return func(c *runConfig) {
		c.ParentSessionID = parentSessionID
	}
}

// RunResult represents the outcome of a synchronous AI orchestration run.
type RunResult struct {
	// Response is the final text output from the agent.
	Response string `json:"response"`
	// Agent is the name of the agent that handled the request.
	Agent string `json:"agent"`
	// SkillsUsed lists the skills invoked during execution.
	SkillsUsed []string `json:"skills_used,omitempty"`
	// DurationMs is the total execution time in milliseconds.
	DurationMs int64 `json:"duration_ms"`
	// MemoryID points to the stored execution memory record.
	MemoryID string `json:"memory_id"`
	// SessionID is the conversation identity (ETAPA 2) — distinct from
	// MemoryID. Echo it back in a later call (WithSession) to resume.
	SessionID string `json:"session_id,omitempty"`
}

// StreamEvent represents a single event emitted during a streaming
// orchestration run.
type StreamEvent struct {
	// Type is the event type (thinking, response, error, done).
	Type string `json:"type"`
	// Content is the human-readable event content.
	Content string `json:"content"`
	// DurationMs is the total run duration (only present in "done" events).
	DurationMs int64 `json:"duration_ms,omitempty"`
	// SessionID is the conversation identity (ETAPA 2); present on the
	// envelope events (thinking/done) so the client can resume the chat.
	SessionID string `json:"session_id,omitempty"`
}

// runRequest is the body sent to the orchestration API.
type runRequest struct {
	Prompt          string `json:"prompt"`
	Agent           string `json:"agent,omitempty"`
	Provider        string `json:"provider,omitempty"`
	SessionID       string `json:"session_id,omitempty"`
	ParentSessionID string `json:"parent_session_id,omitempty"`
}

// =============================================================================
// OrchestrationSDK
// =============================================================================

// OrchestrationSDK provides methods for AI orchestration — executing
// prompts through AI agents and streaming responses in real time.
type OrchestrationSDK struct {
	client *Client
}

// Run executes a prompt through the AI orchestration engine and returns
// the complete response synchronously. Use opts to specify which agent
// and provider to use.
func (s *OrchestrationSDK) Run(ctx context.Context, prompt string, opts ...RunOption) (*RunResult, error) {
	if prompt == "" {
		return nil, fmt.Errorf("prompt is required")
	}

	cfg := &runConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	body := runRequest{
		Prompt:          prompt,
		Agent:           cfg.Agent,
		Provider:        cfg.Provider,
		SessionID:       cfg.Session,
		ParentSessionID: cfg.ParentSessionID,
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal run request: %w", err)
	}

	req, err := s.client.newRequest(
		ctx,
		http.MethodPost,
		"/v1/run",
		bytes.NewReader(payload),
	)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.doRequest(req)
	if err != nil {
		return nil, err
	}
	defer safe.Close(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, s.client.decodeError(resp)
	}

	var result RunResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode run result: %w", err)
	}

	return &result, nil
}

// Stream executes a prompt through the AI orchestration engine and streams
// the response back as Server-Sent Events. It returns a channel of
// StreamEvent values. The channel is closed when streaming completes or
// when the context is cancelled. The caller must consume the channel to
// avoid goroutine leaks.
func (s *OrchestrationSDK) Stream(ctx context.Context, prompt string, opts ...RunOption) (<-chan StreamEvent, error) {
	if prompt == "" {
		return nil, fmt.Errorf("prompt is required")
	}

	cfg := &runConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	body := runRequest{
		Prompt:          prompt,
		Agent:           cfg.Agent,
		Provider:        cfg.Provider,
		SessionID:       cfg.Session,
		ParentSessionID: cfg.ParentSessionID,
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal stream request: %w", err)
	}

	req, err := s.client.newRequest(
		ctx,
		http.MethodPost,
		"/v1/run/stream",
		bytes.NewReader(payload),
	)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := s.client.doRequest(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		safe.Close(resp.Body)
		return nil, s.client.decodeError(resp)
	}

	events := make(chan StreamEvent, 64)

	go func() {
		defer safe.Close(resp.Body)
		defer close(events)

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			// Check for context cancellation.
			select {
			case <-ctx.Done():
				return
			default:
			}

			line := scanner.Text()
			if line == "" || !strings.HasPrefix(line, "data: ") {
				continue
			}

			// Strip "data: " prefix.
			data := strings.TrimPrefix(line, "data: ")

			var event StreamEvent
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				continue
			}

			select {
			case events <- event:
			case <-ctx.Done():
				return
			}
		}
	}()

	return events, nil
}
