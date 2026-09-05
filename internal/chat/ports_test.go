// Package chat provides tests for the core port interfaces and supporting types
// defined in ports.go. This file verifies interface contracts via compile-time
// checks, mock implementations, and type serialization round-trips.
package chat

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// ─── Compile-time interface checks ───────────────────────────────────────────

// Ensure all core interfaces can be satisfied by concrete mock types at compile
// time. If any interface changes incompatibly, these lines will fail to compile.

func TestAgentInterfaceCompiles(t *testing.T) {
	t.Parallel()
	var _ Agent = (*portMockAgent)(nil)
}

func TestToolInterfaceCompiles(t *testing.T) {
	t.Parallel()
	var _ Tool = (*portMockTool)(nil)
}

func TestProviderInterfaceCompiles(t *testing.T) {
	t.Parallel()
	var _ Provider = (*portMockProvider)(nil)
}

func TestMemoryInterfaceCompiles(t *testing.T) {
	t.Parallel()
	var _ Memory = (*portMockMemory)(nil)
}

func TestSandboxInterfaceCompiles(t *testing.T) {
	t.Parallel()
	var _ Sandbox = (*portMockSandbox)(nil)
}

// ─── Mock types ──────────────────────────────────────────────────────────────
// NOTE: These mocks are prefixed with "portMock" to avoid colliding with
// mock types in other files within the same package (e.g. registry_test.go).

type portMockAgent struct{}

func (m *portMockAgent) Name() string           { return "test-agent" }
func (m *portMockAgent) SystemPrompt() string   { return "test prompt" }
func (m *portMockAgent) Capabilities() []string { return []string{"test"} }
func (m *portMockAgent) Run(_ context.Context, _ AgentRequest) (*AgentResponse, error) {
	return &AgentResponse{Content: "ok"}, nil
}

type portMockTool struct{}

func (m *portMockTool) Name() string            { return "test-tool" }
func (m *portMockTool) Description() string     { return "test description" }
func (m *portMockTool) Schema() json.RawMessage { return json.RawMessage(`{"type":"object"}`) }
func (m *portMockTool) Execute(_ context.Context, _ json.RawMessage) (*ToolResult, error) {
	return &ToolResult{Output: "done", Duration: time.Second}, nil
}
func (m *portMockTool) Validate(_ json.RawMessage) error { return nil }

type portMockProvider struct{}

func (m *portMockProvider) Name() string { return "test-provider" }
func (m *portMockProvider) Chat(_ context.Context, _ ChatRequest) (<-chan ChatEvent, error) {
	ch := make(chan ChatEvent, 2)
	ch <- ChatEvent{Type: ChatEventDelta, Delta: "hello"}
	ch <- ChatEvent{Type: ChatEventDone, Usage: &Usage{TotalTokens: 5}}
	close(ch)
	return ch, nil
}
func (m *portMockProvider) Models() []string  { return []string{"test-model"} }
func (m *portMockProvider) IsAvailable() bool { return true }

type portMockMemory struct{}

func (m *portMockMemory) Store(_ context.Context, _ MemoryEntry) error { return nil }
func (m *portMockMemory) Search(_ context.Context, _ string, _ int) ([]MemoryEntry, error) {
	return []MemoryEntry{{ID: "1", Content: "result"}}, nil
}
func (m *portMockMemory) Forget(_ context.Context, _ string) error { return nil }
func (m *portMockMemory) Stats() (int, error)                      { return 42, nil }

type portMockSandbox struct{}

func (m *portMockSandbox) Execute(_ context.Context, _ Command, _ SandboxMode) (*SandboxResult, error) {
	return &SandboxResult{Stdout: "ok", ExitCode: 0, Duration: 100 * time.Millisecond}, nil
}
func (m *portMockSandbox) Mode() SandboxMode           { return SandboxWorkspace }
func (m *portMockSandbox) ValidatePath(_ string) error { return nil }

// ─── Mock concrete usage ─────────────────────────────────────────────────────

func TestPortMockAgentProducesResponse(t *testing.T) {
	t.Parallel()
	a := &portMockAgent{}
	resp, err := a.Run(context.Background(), AgentRequest{})
	if err != nil {
		t.Fatalf("portMockAgent.Run failed: %v", err)
	}
	if resp.Content != "ok" {
		t.Errorf("expected Content='ok', got %q", resp.Content)
	}
}

func TestPortMockToolExecutes(t *testing.T) {
	t.Parallel()
	tool := &portMockTool{}
	res, err := tool.Execute(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("portMockTool.Execute failed: %v", err)
	}
	if res.Output != "done" {
		t.Errorf("expected Output='done', got %q", res.Output)
	}
	if res.Duration != time.Second {
		t.Errorf("expected Duration=1s, got %v", res.Duration)
	}
}

func TestPortMockProviderStreamsEvents(t *testing.T) {
	t.Parallel()
	p := &portMockProvider{}
	ch, err := p.Chat(context.Background(), ChatRequest{Model: "test", Messages: nil})
	if err != nil {
		t.Fatalf("portMockProvider.Chat failed: %v", err)
	}
	var events []ChatEvent
	for e := range ch {
		events = append(events, e)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Type != ChatEventDelta || events[0].Delta != "hello" {
		t.Errorf("expected delta 'hello', got type=%s delta=%q", events[0].Type, events[0].Delta)
	}
	if events[1].Type != ChatEventDone || events[1].Usage == nil || events[1].Usage.TotalTokens != 5 {
		t.Errorf("expected done with 5 tokens, got type=%s usage=%+v", events[1].Type, events[1].Usage)
	}
}

func TestPortMockMemoryStoresAndSearches(t *testing.T) {
	t.Parallel()
	m := &portMockMemory{}
	if err := m.Store(context.Background(), MemoryEntry{ID: "x", Content: "test"}); err != nil {
		t.Fatalf("portMockMemory.Store failed: %v", err)
	}
	entries, err := m.Search(context.Background(), "test", 10)
	if err != nil {
		t.Fatalf("portMockMemory.Search failed: %v", err)
	}
	if len(entries) != 1 || entries[0].ID != "1" {
		t.Errorf("expected 1 entry with ID='1', got %+v", entries)
	}
	if err := m.Forget(context.Background(), "1"); err != nil {
		t.Errorf("portMockMemory.Forget failed: %v", err)
	}
	count, err := m.Stats()
	if err != nil {
		t.Errorf("portMockMemory.Stats failed: %v", err)
	}
	if count != 42 {
		t.Errorf("expected 42, got %d", count)
	}
}

func TestPortMockSandboxExecutes(t *testing.T) {
	t.Parallel()
	s := &portMockSandbox{}
	res, err := s.Execute(context.Background(), Command{Args: []string{"echo", "hi"}}, SandboxReadOnly)
	if err != nil {
		t.Fatalf("portMockSandbox.Execute failed: %v", err)
	}
	if res.Stdout != "ok" {
		t.Errorf("expected Stdout='ok', got %q", res.Stdout)
	}
	if res.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", res.ExitCode)
	}
	if s.Mode() != SandboxWorkspace {
		t.Errorf("expected SandboxWorkspace, got %v", s.Mode())
	}
	if err := s.ValidatePath("/tmp"); err != nil {
		t.Errorf("portMockSandbox.ValidatePath failed: %v", err)
	}
}

// ─── SandboxMode.String() ────────────────────────────────────────────────────

func TestSandboxModeString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		mode SandboxMode
		want string
	}{
		{SandboxReadOnly, "read-only"},
		{SandboxWorkspace, "workspace"},
		{SandboxFull, "full"},
		{SandboxMode(-1), "unknown"},
		{SandboxMode(99), "unknown"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.want, func(t *testing.T) {
			t.Parallel()
			got := tc.mode.String()
			if got != tc.want {
				t.Errorf("SandboxMode(%d).String() = %q, want %q", int(tc.mode), got, tc.want)
			}
		})
	}
}

// ─── SandboxMode iota values ─────────────────────────────────────────────────

func TestSandboxModeIotaValues(t *testing.T) {
	t.Parallel()

	if SandboxReadOnly != 0 {
		t.Errorf("expected SandboxReadOnly=0, got %d", SandboxReadOnly)
	}
	if SandboxWorkspace != 1 {
		t.Errorf("expected SandboxWorkspace=1, got %d", SandboxWorkspace)
	}
	if SandboxFull != 2 {
		t.Errorf("expected SandboxFull=2, got %d", SandboxFull)
	}
}

// ─── ChatEventType constants ─────────────────────────────────────────────────

func TestChatEventTypeValues(t *testing.T) {
	t.Parallel()

	if ChatEventDelta != "delta" {
		t.Errorf("expected ChatEventDelta='delta', got %q", ChatEventDelta)
	}
	if ChatEventDone != "done" {
		t.Errorf("expected ChatEventDone='done', got %q", ChatEventDone)
	}
	if ChatEventError != "error" {
		t.Errorf("expected ChatEventError='error', got %q", ChatEventError)
	}
}

// ─── JSON round-trip ─────────────────────────────────────────────────────────

func TestAgentResponseJSONRoundTrip(t *testing.T) {
	t.Parallel()

	orig := &AgentResponse{
		Content: "hello world",
		ToolCalls: []ToolCall{
			{ID: "call_1", Type: "function", Function: FunctionCall{Name: "test", Arguments: `{"a":1}`}},
		},
		Usage: Usage{PromptTokens: 10, CompletionTokens: 20, TotalTokens: 30},
	}

	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var got AgentResponse
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if got.Content != orig.Content {
		t.Errorf("Content: got %q, want %q", got.Content, orig.Content)
	}
	if len(got.ToolCalls) != len(orig.ToolCalls) {
		t.Fatalf("ToolCalls length: got %d, want %d", len(got.ToolCalls), len(orig.ToolCalls))
	}
	if got.ToolCalls[0].ID != orig.ToolCalls[0].ID {
		t.Errorf("ToolCalls[0].ID: got %q, want %q", got.ToolCalls[0].ID, orig.ToolCalls[0].ID)
	}
	if got.Usage.TotalTokens != orig.Usage.TotalTokens {
		t.Errorf("Usage.TotalTokens: got %d, want %d", got.Usage.TotalTokens, orig.Usage.TotalTokens)
	}
}

func TestChatEventJSONRoundTrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		evt  ChatEvent
	}{
		{"delta event", ChatEvent{Type: ChatEventDelta, Delta: "hello"}},
		{"done event", ChatEvent{Type: ChatEventDone, Usage: &Usage{TotalTokens: 100}}},
		{"error event", ChatEvent{Type: ChatEventError, Error: nil /* error interface won't round-trip */}},
		{"empty event", ChatEvent{}},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			data, err := json.Marshal(tc.evt)
			if err != nil {
				t.Fatalf("json.Marshal failed: %v", err)
			}

			var got ChatEvent
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("json.Unmarshal failed: %v", err)
			}

			if got.Type != tc.evt.Type {
				t.Errorf("Type: got %q, want %q", got.Type, tc.evt.Type)
			}
			if got.Delta != tc.evt.Delta {
				t.Errorf("Delta: got %q, want %q", got.Delta, tc.evt.Delta)
			}
			// Usage is compared when non-nil
			if tc.evt.Usage != nil {
				if got.Usage == nil {
					t.Fatal("expected non-nil Usage after round-trip")
				}
				if got.Usage.TotalTokens != tc.evt.Usage.TotalTokens {
					t.Errorf("Usage.TotalTokens: got %d, want %d", got.Usage.TotalTokens, tc.evt.Usage.TotalTokens)
				}
			}
		})
	}
}

func TestToolResultJSONRoundTrip(t *testing.T) {
	t.Parallel()

	orig := ToolResult{
		Output:   "command output",
		Error:    "",
		Duration: 5 * time.Second,
	}

	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var got ToolResult
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if got.Output != orig.Output {
		t.Errorf("Output: got %q, want %q", got.Output, orig.Output)
	}
	if got.Error != orig.Error {
		t.Errorf("Error: got %q, want %q", got.Error, orig.Error)
	}
	if got.Duration != orig.Duration {
		t.Errorf("Duration: got %v, want %v", got.Duration, orig.Duration)
	}
}

func TestMemoryEntryJSONRoundTrip(t *testing.T) {
	t.Parallel()

	orig := MemoryEntry{
		ID:      "mem_1",
		Content: "important memory",
		Metadata: map[string]any{
			"source": "agent-1",
			"score":  float64(0.95),
		},
		Score: 0.95,
	}

	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var got MemoryEntry
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if got.ID != orig.ID {
		t.Errorf("ID: got %q, want %q", got.ID, orig.ID)
	}
	if got.Content != orig.Content {
		t.Errorf("Content: got %q, want %q", got.Content, orig.Content)
	}
	if got.Score != orig.Score {
		t.Errorf("Score: got %f, want %f", got.Score, orig.Score)
	}
	// Verify metadata key exists
	source, ok := got.Metadata["source"]
	if !ok {
		t.Error("Metadata['source'] missing after round-trip")
	} else if source != "agent-1" {
		t.Errorf("Metadata['source'] = %v, want 'agent-1'", source)
	}
}

func TestCommandJSONRoundTrip(t *testing.T) {
	t.Parallel()

	orig := Command{
		Args:    []string{"ls", "-la"},
		Env:     map[string]string{"PATH": "/usr/bin"},
		WorkDir: "/home",
	}

	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var got Command
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if len(got.Args) != 2 || got.Args[0] != "ls" {
		t.Errorf("Args: got %v, want ['ls', '-la']", got.Args)
	}
	if got.Env["PATH"] != "/usr/bin" {
		t.Errorf("Env['PATH']: got %q, want '/usr/bin'", got.Env["PATH"])
	}
	if got.WorkDir != "/home" {
		t.Errorf("WorkDir: got %q, want '/home'", got.WorkDir)
	}
}

func TestSandboxResultJSONRoundTrip(t *testing.T) {
	t.Parallel()

	orig := SandboxResult{
		Stdout:   "hello",
		Stderr:   "",
		ExitCode: 0,
		Duration: 2 * time.Second,
	}

	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var got SandboxResult
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if got.Stdout != orig.Stdout {
		t.Errorf("Stdout: got %q, want %q", got.Stdout, orig.Stdout)
	}
	if got.ExitCode != 0 {
		t.Errorf("ExitCode: got %d, want 0", got.ExitCode)
	}
	if got.Duration != orig.Duration {
		t.Errorf("Duration: got %v, want %v", got.Duration, orig.Duration)
	}
}

// ─── ChatEvent with error (edge case) ────────────────────────────────────────

func TestChatEventErrorJSONMarshaling(t *testing.T) {
	t.Parallel()

	// The Error field in ChatEvent is of type error (interface), which
	// marshals to an empty JSON object {} (because Go's stdlib encodes
	// the concrete type's exported fields), and CANNOT be unmarshaled
	// back into an error interface. This test documents that the Type
	// and other fields survive correctly while Error is effectively
	// lost in round-trip.
	orig := ChatEvent{
		Type:  ChatEventError,
		Delta: "",
		Error: context.Canceled,
		Usage: &Usage{TotalTokens: 0},
	}

	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	// Verify Type and other non-error fields survive
	if !strings.Contains(string(data), `"type":"error"`) {
		t.Errorf("JSON missing type field: %s", string(data))
	}

	// Unmarshal the JSON back into a struct that mirrors ChatEvent but
	// with a json.RawMessage for Error to verify raw JSON preservation.
	var raw struct {
		Type  ChatEventType   `json:"type"`
		Delta string          `json:"delta,omitempty"`
		Error json.RawMessage `json:"error,omitempty"`
		Usage *Usage          `json:"usage,omitempty"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("json.Unmarshal into raw struct failed: %v", err)
	}
	if raw.Type != ChatEventError {
		t.Errorf("Type: got %q, want %q", raw.Type, ChatEventError)
	}
	// Error field was marshaled; verify it exists in JSON but is empty
	if len(raw.Error) == 0 {
		t.Log("Error field is empty in JSON (expected for interface{} type)")
	}
}

// ─── Usage zero-value ────────────────────────────────────────────────────────

func TestUsageDefaults(t *testing.T) {
	t.Parallel()

	u := Usage{}
	if u.PromptTokens != 0 {
		t.Errorf("expected PromptTokens=0, got %d", u.PromptTokens)
	}
	if u.CompletionTokens != 0 {
		t.Errorf("expected CompletionTokens=0, got %d", u.CompletionTokens)
	}
	if u.TotalTokens != 0 {
		t.Errorf("expected TotalTokens=0, got %d", u.TotalTokens)
	}
}
