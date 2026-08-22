package orchestration

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/internal/chat"
)

// ─── Mock Chat Provider ──────────────────────────────────────────────────────

// mockChatProvider implements chat.ChatProvider for testing.
type mockChatProvider struct {
	name  string
	model string

	// Controllable responses.
	chatResponse *chat.ChatResponse
	chatErr      error
	chatCalled   int

	// chatFn overrides the Chat method when set (allows dynamic behaviour).
	chatFn func(ctx context.Context, messages []chat.Message, opts chat.ChatOptions) (*chat.ChatResponse, error)

	streamResponse *mockChatStream
	streamErr      error
}

func newMockChatProvider(name, model string) *mockChatProvider {
	return &mockChatProvider{name: name, model: model}
}

func (m *mockChatProvider) Chat(ctx context.Context, messages []chat.Message, opts chat.ChatOptions) (*chat.ChatResponse, error) {
	m.chatCalled++
	if m.chatFn != nil {
		return m.chatFn(ctx, messages, opts)
	}
	if m.chatErr != nil {
		return nil, m.chatErr
	}
	if m.chatResponse != nil {
		return m.chatResponse, nil
	}
	return &chat.ChatResponse{
		ID:    "resp-1",
		Model: m.model,
		Choices: []chat.Choice{
			{
				Index: 0,
				Message: chat.Message{
					Role:    chat.RoleAssistant,
					Content: "This is a test response.",
				},
				FinishReason: chat.FinishReasonStop,
			},
		},
		Usage: chat.Usage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
		},
	}, nil
}

func (m *mockChatProvider) ChatStream(_ context.Context, _ []chat.Message, _ chat.ChatOptions) (chat.ChatStream, error) {
	if m.streamErr != nil {
		return nil, m.streamErr
	}
	if m.streamResponse != nil {
		return m.streamResponse, nil
	}
	return &mockChatStream{
		chunks: []chat.ChatStreamChunk{
			{
				ID:    "stream-1",
				Model: m.model,
				Choices: []chat.StreamChoice{
					{Index: 0, Delta: chat.Message{Content: "Hello "}},
				},
			},
			{
				ID:    "stream-1",
				Model: m.model,
				Choices: []chat.StreamChoice{
					{Index: 0, Delta: chat.Message{Content: "World"}},
				},
			},
			{
				ID:    "stream-1",
				Model: m.model,
				Choices: []chat.StreamChoice{
					{Index: 0, Delta: chat.Message{}, FinishReason: chat.FinishReasonStop},
				},
			},
		},
	}, nil
}

func (m *mockChatProvider) Model() string { return m.model }
func (m *mockChatProvider) Name() string  { return m.name }
func (m *mockChatProvider) Close() error  { return nil }

// ─── Mock Chat Stream ────────────────────────────────────────────────────────

type mockChatStream struct {
	chunks []chat.ChatStreamChunk
	pos    int
	closed bool
}

func (m *mockChatStream) Recv() (*chat.ChatStreamChunk, error) {
	if m.pos >= len(m.chunks) {
		return nil, errors.New("EOF")
	}
	chunk := m.chunks[m.pos]
	m.pos++
	return &chunk, nil
}

func (m *mockChatStream) Close() error {
	m.closed = true
	return nil
}

// ─── Tests: ExecutorConfig ────────────────────────────────────────────────────

func TestDefaultExecutorConfig(t *testing.T) {
	cfg := DefaultExecutorConfig()

	if cfg.MaxRetries != 3 {
		t.Errorf("expected MaxRetries=3, got %d", cfg.MaxRetries)
	}
	if cfg.RetryDelay != 2*time.Second {
		t.Errorf("expected RetryDelay=2s, got %v", cfg.RetryDelay)
	}
	if cfg.Timeout != 5*time.Minute {
		t.Errorf("expected Timeout=5m, got %v", cfg.Timeout)
	}
}

func TestNewExecutor_DefaultsApplied(t *testing.T) {
	provider := newMockChatProvider("test", "test-model")
	exec := NewExecutor(provider, ExecutorConfig{}, nil)

	if exec.config.MaxRetries != 3 {
		t.Errorf("expected MaxRetries=3, got %d", exec.config.MaxRetries)
	}
	if exec.config.RetryDelay != 2*time.Second {
		t.Errorf("expected RetryDelay=2s, got %v", exec.config.RetryDelay)
	}
	if exec.config.Timeout != 5*time.Minute {
		t.Errorf("expected Timeout=5m, got %v", exec.config.Timeout)
	}
}

func TestNewExecutor_PreservesCustomConfig(t *testing.T) {
	provider := newMockChatProvider("test", "test-model")
	custom := ExecutorConfig{
		MaxRetries: 5,
		RetryDelay: 1 * time.Second,
		Timeout:    30 * time.Second,
	}
	exec := NewExecutor(provider, custom, nil)

	if exec.config.MaxRetries != 5 {
		t.Errorf("expected MaxRetries=5, got %d", exec.config.MaxRetries)
	}
	if exec.config.RetryDelay != 1*time.Second {
		t.Errorf("expected RetryDelay=1s, got %v", exec.config.RetryDelay)
	}
}

// ─── Tests: Execute ──────────────────────────────────────────────────────────

func TestExecute_Success(t *testing.T) {
	provider := newMockChatProvider("test", "test-model")
	exec := NewExecutor(provider, DefaultExecutorConfig(), nil)

	pc := NewPipelineContext("req-1", "build an api")
	pc = pc.WithResolvedAgent("Backend Chief")
	pc = pc.WithAgentRole("Backend Development Lead")
	pc = pc.WithAgentDepartment("backend")
	pc = pc.WithAgentDescription("Leads backend development")

	result, err := exec.Execute(context.Background(), pc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check that response is stored.
	if result.Data.LLMResponse == "" {
		t.Error("expected LLMResponse to be set")
	}
	if result.Data.LLMModel == "" {
		t.Error("expected LLMModel to be set")
	}
	if result.Data.LLMUsage == nil {
		t.Error("expected LLMUsage to be set")
	}

	if provider.chatCalled != 1 {
		t.Errorf("expected 1 chat call, got %d", provider.chatCalled)
	}
}

func TestExecute_UsesAugmentedPrompt(t *testing.T) {
	provider := newMockChatProvider("test", "test-model")

	// Override Chat to capture messages.
	var capturedMessages []chat.Message
	provider.chatResponse = &chat.ChatResponse{
		ID:    "resp-1",
		Model: "test-model",
		Choices: []chat.Choice{
			{
				Index: 0,
				Message: chat.Message{
					Role:    chat.RoleAssistant,
					Content: "response",
				},
				FinishReason: chat.FinishReasonStop,
			},
		},
		Usage: chat.Usage{TotalTokens: 5},
	}
	provider.chatFn = func(_ context.Context, messages []chat.Message, _ chat.ChatOptions) (*chat.ChatResponse, error) {
		capturedMessages = messages
		return provider.chatResponse, nil
	}

	exec := NewExecutor(provider, DefaultExecutorConfig(), nil)

	pc := NewPipelineContext("req-2", "original prompt")
	pc = pc.WithAugmentedPrompt("augmented: original prompt with context")
	pc = pc.WithResolvedAgent("Backend Chief")
	pc = pc.WithAgentRole("Backend Chief")

	_, err := exec.Execute(context.Background(), pc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(capturedMessages) < 2 {
		t.Fatalf("expected at least 2 messages, got %d", len(capturedMessages))
	}

	userMsg := capturedMessages[1]
	if userMsg.Content != "augmented: original prompt with context" {
		t.Errorf("expected augmented prompt, got %q", userMsg.Content)
	}
}

func TestExecute_NoAugmentedPrompt_FallsBackToRaw(t *testing.T) {
	provider := newMockChatProvider("test", "test-model")

	var capturedMessages []chat.Message
	provider.chatResponse = &chat.ChatResponse{
		ID:    "resp-1",
		Model: "test-model",
		Choices: []chat.Choice{
			{
				Index:        0,
				Message:      chat.Message{Role: chat.RoleAssistant, Content: "response"},
				FinishReason: chat.FinishReasonStop,
			},
		},
		Usage: chat.Usage{TotalTokens: 5},
	}
	provider.chatFn = func(_ context.Context, messages []chat.Message, _ chat.ChatOptions) (*chat.ChatResponse, error) {
		capturedMessages = messages
		return provider.chatResponse, nil
	}

	exec := NewExecutor(provider, DefaultExecutorConfig(), nil)

	pc := NewPipelineContext("req-3", "raw prompt")
	pc = pc.WithResolvedAgent("Backend Chief")
	pc = pc.WithAgentRole("Backend Chief")

	_, err := exec.Execute(context.Background(), pc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	userMsg := capturedMessages[1]
	if userMsg.Content != "raw prompt" {
		t.Errorf("expected raw prompt fallback, got %q", userMsg.Content)
	}
}

func TestExecute_ChatError(t *testing.T) {
	provider := newMockChatProvider("test", "test-model")
	provider.chatErr = errors.New("non-transient fatal error: invalid api key")

	exec := NewExecutor(provider, ExecutorConfig{
		MaxRetries: 0, // no retries
		RetryDelay: 10 * time.Millisecond,
		Timeout:    1 * time.Second,
	}, nil)

	pc := NewPipelineContext("req-4", "test")
	pc = pc.WithResolvedAgent("Test Agent")

	_, err := exec.Execute(context.Background(), pc)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "executor: chat failed") {
		t.Errorf("expected executor error prefix, got: %v", err)
	}
}

func TestExecute_ToolCallsStored(t *testing.T) {
	provider := newMockChatProvider("test", "test-model")
	provider.chatResponse = &chat.ChatResponse{
		ID:    "resp-tools",
		Model: "test-model",
		Choices: []chat.Choice{
			{
				Index: 0,
				Message: chat.Message{
					Role:    chat.RoleAssistant,
					Content: "Let me search for that.",
					ToolCalls: []chat.ToolCall{
						{ID: "tc-1", Type: "function", Function: chat.FunctionCall{Name: "search_codebase", Arguments: `{"query":"test"}`}},
					},
				},
				FinishReason: chat.FinishReasonToolCalls,
			},
		},
		Usage: chat.Usage{TotalTokens: 20},
	}

	exec := NewExecutor(provider, DefaultExecutorConfig(), nil)

	pc := NewPipelineContext("req-5", "search for something")
	pc = pc.WithResolvedAgent("Backend Chief")
	pc = pc.WithAgentRole("Backend Chief")

	result, err := exec.Execute(context.Background(), pc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	toolCalls, ok := result.Data.ToolCalls.([]chat.ToolCall)
	if !ok {
		t.Fatal("tool_calls not found or wrong type")
	}
	if len(toolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(toolCalls))
	}
	if toolCalls[0].Function.Name != "search_codebase" {
		t.Errorf("expected 'search_codebase', got %q", toolCalls[0].Function.Name)
	}
}

// ─── Tests: ExecuteStream ────────────────────────────────────────────────────

func TestExecuteStream_Success(t *testing.T) {
	provider := newMockChatProvider("test", "test-model")
	exec := NewExecutor(provider, DefaultExecutorConfig(), nil)

	pc := NewPipelineContext("req-stream-1", "tell me a story")
	pc = pc.WithResolvedAgent("Backend Chief")
	pc = pc.WithAgentRole("Backend Chief")

	eventCh := make(chan StreamEvent, 10)
	_, err := exec.ExecuteStream(context.Background(), pc, eventCh)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Collect events.
	var events []StreamEvent
	timeout := time.After(2 * time.Second)
loop:
	for {
		select {
		case ev, ok := <-eventCh:
			if !ok {
				break loop
			}
			events = append(events, ev)
		case <-timeout:
			t.Fatal("timeout waiting for stream events")
		}
	}

	if len(events) == 0 {
		t.Fatal("expected at least one stream event")
	}

	// First event should be a progress event.
	if events[0].Type != StreamEventProgress {
		t.Errorf("expected first event to be progress, got %s", events[0].Type)
	}

	// Should have chunk events.
	foundChunk := false
	for _, ev := range events {
		if ev.Type == StreamEventChunk {
			foundChunk = true
			break
		}
	}
	if !foundChunk {
		t.Error("expected at least one chunk event")
	}
}

func TestExecuteStream_StreamError(t *testing.T) {
	provider := newMockChatProvider("test", "test-model")
	secret := "connection refused: /workspace/private/prompt=super-secret"
	provider.streamErr = errors.New(secret)

	exec := NewExecutor(provider, DefaultExecutorConfig(), nil)

	pc := NewPipelineContext("req-stream-err", "test")
	pc = pc.WithResolvedAgent("Test")

	eventCh := make(chan StreamEvent, 10)
	_, err := exec.ExecuteStream(context.Background(), pc, eventCh)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Should still get an error event.
	select {
	case ev := <-eventCh:
		if ev.Type != StreamEventError {
			t.Errorf("expected error event, got %s", ev.Type)
		}
		if strings.Contains(ev.Content, secret) || strings.Contains(fmt.Sprint(ev.Metadata), secret) {
			t.Fatalf("stream error event exposed provider error: %+v", ev)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for error event")
	}
}

func TestExecuteStream_StageTransition(t *testing.T) {
	provider := newMockChatProvider("test", "test-model")
	provider.streamResponse = &mockChatStream{
		chunks: []chat.ChatStreamChunk{
			{
				ID:    "stream-stage",
				Model: "test-model",
				Choices: []chat.StreamChoice{
					{Index: 0, Delta: chat.Message{Content: "[STAGE: knowledge_retrieval]"}},
				},
			},
			{
				ID:    "stream-stage",
				Model: "test-model",
				Choices: []chat.StreamChoice{
					{Index: 0, Delta: chat.Message{Content: "Retrieving knowledge..."}},
				},
			},
			{
				ID:    "stream-stage",
				Model: "test-model",
				Choices: []chat.StreamChoice{
					{Index: 0, Delta: chat.Message{}, FinishReason: chat.FinishReasonStop},
				},
			},
		},
	}

	exec := NewExecutor(provider, DefaultExecutorConfig(), nil)

	pc := NewPipelineContext("req-stage", "test")
	pc = pc.WithResolvedAgent("Test")

	eventCh := make(chan StreamEvent, 10)
	_, err := exec.ExecuteStream(context.Background(), pc, eventCh)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var events []StreamEvent
	timeout := time.After(2 * time.Second)
loop:
	for {
		select {
		case ev, ok := <-eventCh:
			if !ok {
				break loop
			}
			events = append(events, ev)
		case <-timeout:
			t.Fatal("timeout waiting for events")
		}
	}

	foundStageTransition := false
	for _, ev := range events {
		if ev.Type == StreamEventStageTransition {
			foundStageTransition = true
			break
		}
	}
	if !foundStageTransition {
		t.Error("expected at least one stage transition event")
	}
}

func TestExecuteStream_ContextCancellation(t *testing.T) {
	provider := newMockChatProvider("test", "test-model")
	// Use a slow stream that blocks indefinitely.
	provider.streamResponse = &mockChatStream{
		chunks: []chat.ChatStreamChunk{}, // empty — Recv will block (but mock returns EOF)
	}

	exec := NewExecutor(provider, DefaultExecutorConfig(), nil)

	ctx, cancel := context.WithCancel(context.Background())

	pc := NewPipelineContext("req-cancel", "test")
	pc = pc.WithResolvedAgent("Test")

	eventCh := make(chan StreamEvent, 10)
	_, err := exec.ExecuteStream(ctx, pc, eventCh)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Cancel immediately.
	cancel()

	// Wait for events to drain.
	timeout := time.After(2 * time.Second)
loop:
	for {
		select {
		case _, ok := <-eventCh:
			if !ok {
				break loop
			}
		case <-timeout:
			t.Fatal("timeout waiting for events after cancel")
		}
	}

	// The mock returns EOF immediately, so we may not get a cancellation error.
	// Just verify the channel closes.
}

// ─── Tests: Retry Logic ──────────────────────────────────────────────────────

func TestChatWithRetry_SuccessFirstAttempt(t *testing.T) {
	provider := newMockChatProvider("test", "test-model")
	exec := NewExecutor(provider, DefaultExecutorConfig(), nil)

	resp, err := exec.chatWithRetry(context.Background(),
		[]chat.Message{{Role: chat.RoleUser, Content: "hi"}},
		chat.DefaultChatOptions(),
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected response")
	}
	if provider.chatCalled != 1 {
		t.Errorf("expected 1 call, got %d", provider.chatCalled)
	}
}

func TestChatWithRetry_RetriesOnTransientError(t *testing.T) {
	provider := newMockChatProvider("test", "test-model")
	exec := NewExecutor(provider, ExecutorConfig{
		MaxRetries: 3,
		RetryDelay: 10 * time.Millisecond,
		Timeout:    1 * time.Second,
	}, nil)

	callCount := 0
	defaultResp := &chat.ChatResponse{
		ID:    "resp-retry",
		Model: "test-model",
		Choices: []chat.Choice{
			{Index: 0, Message: chat.Message{Role: chat.RoleAssistant, Content: "ok"}, FinishReason: chat.FinishReasonStop},
		},
		Usage: chat.Usage{TotalTokens: 3},
	}

	provider.chatFn = func(_ context.Context, _ []chat.Message, _ chat.ChatOptions) (*chat.ChatResponse, error) {
		callCount++
		if callCount < 3 {
			return nil, errors.New("service unavailable: temporary failure")
		}
		return defaultResp, nil
	}

	resp, err := exec.chatWithRetry(context.Background(),
		[]chat.Message{{Role: chat.RoleUser, Content: "hi"}},
		chat.DefaultChatOptions(),
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected response")
	}
	if callCount < 3 {
		t.Errorf("expected at least 3 calls (2 retries + 1 success), got %d", callCount)
	}
}

func TestChatWithRetry_NonTransientError_NoRetry(t *testing.T) {
	provider := newMockChatProvider("test", "test-model")
	exec := NewExecutor(provider, ExecutorConfig{
		MaxRetries: 3,
		RetryDelay: 10 * time.Millisecond,
		Timeout:    1 * time.Second,
	}, nil)

	callCount := 0
	provider.chatFn = func(_ context.Context, _ []chat.Message, _ chat.ChatOptions) (*chat.ChatResponse, error) {
		callCount++
		return nil, errors.New("invalid api key")
	}

	_, err := exec.chatWithRetry(context.Background(),
		[]chat.Message{{Role: chat.RoleUser, Content: "hi"}},
		chat.DefaultChatOptions(),
	)

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if callCount > 1 {
		t.Errorf("expected only 1 call (no retry for non-transient), got %d", callCount)
	}
}

func TestChatWithRetry_ContextCancelled_BeforeRetry(t *testing.T) {
	provider := newMockChatProvider("test", "test-model")
	exec := NewExecutor(provider, ExecutorConfig{
		MaxRetries: 3,
		RetryDelay: 100 * time.Millisecond,
		Timeout:    10 * time.Second,
	}, nil)

	callCount := 0
	firstCallDone := make(chan struct{})
	provider.chatFn = func(_ context.Context, _ []chat.Message, _ chat.ChatOptions) (*chat.ChatResponse, error) {
		callCount++
		if callCount == 1 {
			close(firstCallDone)
		}
		return nil, errors.New("timeout")
	}

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel after first call.
	go func() {
		<-firstCallDone
		cancel()
	}()

	_, err := exec.chatWithRetry(ctx,
		[]chat.Message{{Role: chat.RoleUser, Content: "hi"}},
		chat.DefaultChatOptions(),
	)

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "context") {
		t.Errorf("expected context error, got: %v", err)
	}
}

// ─── Tests: isTransientError ─────────────────────────────────────────────────

func TestIsTransientError(t *testing.T) {
	tests := []struct {
		errMsg    string
		transient bool
	}{
		{"timeout", true},
		{"deadline exceeded", true},
		{"rate limit exceeded", true},
		{"too many requests", true},
		{"service unavailable", true},
		{"API error 503: Service is too busy...", true},
		{"model is busy", true},
		{"internal server error", true},
		{"bad gateway", true},
		{"gateway timeout", true},
		{"connection reset by peer", true},
		{"connection refused", true},
		{"temporary failure", true},
		{"request was throttled", true},
		{"server overloaded", true},
		{"eof", true},
		{"broken pipe", true},
		{"invalid api key", false},
		{"not found", false},
		{"forbidden", false},
		{"insufficient quota", false},
	}

	for _, tt := range tests {
		t.Run(tt.errMsg, func(t *testing.T) {
			result := isTransientError(errors.New(tt.errMsg))
			if result != tt.transient {
				t.Errorf("isTransientError(%q) = %v, want %v", tt.errMsg, result, tt.transient)
			}
		})
	}

	if isTransientError(nil) {
		t.Error("isTransientError(nil) should be false")
	}
}

// ─── Tests: buildSystemPrompt ────────────────────────────────────────────────

func TestBuildSystemPrompt_Basic(t *testing.T) {
	exec := &Executor{}
	prompt := exec.buildSystemPrompt(
		"Backend Chief",
		"Backend Development Lead",
		"backend",
		"Leads backend development",
		NewPipelineData(),
	)

	if !strings.Contains(prompt, "Backend Chief") {
		t.Error("expected agent name in prompt")
	}
	if !strings.Contains(prompt, "Backend Development Lead") {
		t.Error("expected agent role in prompt")
	}
	if !strings.Contains(prompt, "backend") {
		t.Error("expected department in prompt")
	}
	if !strings.Contains(prompt, "Leads backend development") {
		t.Error("expected description in prompt")
	}
}

func TestBuildSystemPrompt_SameNameAndRole(t *testing.T) {
	exec := &Executor{}
	prompt := exec.buildSystemPrompt(
		"CEO",
		"CEO",
		"ceo",
		"Runs the company",
		NewPipelineData(),
	)

	// Only one occurrence of CEO as role (name always appears).
	count := strings.Count(prompt, "CEO")
	if count > 2 {
		t.Errorf("expected at most 2 occurrences of CEO, got %d: %s", count, prompt)
	}
}

func TestBuildSystemPrompt_WithKnowledge(t *testing.T) {
	exec := &Executor{}
	data := NewPipelineData()
	data.Extra["knowledge_context"] = "This is relevant knowledge."
	prompt := exec.buildSystemPrompt(
		"Backend Chief", "Backend Chief", "backend", "desc",
		data,
	)

	if !strings.Contains(prompt, "RELEVANT KNOWLEDGE") {
		t.Error("expected knowledge context section")
	}
	if !strings.Contains(prompt, "This is relevant knowledge.") {
		t.Error("expected knowledge content")
	}
}

func TestBuildSystemPrompt_WithMemory(t *testing.T) {
	exec := &Executor{}
	data := NewPipelineData()
	data.MemoryContext = "Previous conversation context."
	prompt := exec.buildSystemPrompt(
		"Backend Chief", "Backend Chief", "backend", "desc",
		data,
	)

	if !strings.Contains(prompt, "RELEVANT MEMORY") {
		t.Error("expected memory context section")
	}
	if !strings.Contains(prompt, "Previous conversation context.") {
		t.Error("expected memory content")
	}
}

func TestBuildSystemPrompt_GeneralInstructions(t *testing.T) {
	exec := &Executor{}
	prompt := exec.buildSystemPrompt(
		"Agent", "Agent", "", "",
		NewPipelineData(),
	)

	if !strings.Contains(prompt, "Provide a thorough") {
		t.Error("expected general instructions")
	}
}

// ─── Tests: parseResponse ────────────────────────────────────────────────────

func TestParseResponse_NilResponse(t *testing.T) {
	exec := &Executor{}
	content, toolCalls := exec.parseResponse(nil)

	if content != "" {
		t.Errorf("expected empty content, got %q", content)
	}
	if toolCalls != nil {
		t.Errorf("expected nil tool calls, got %v", toolCalls)
	}
}

func TestParseResponse_EmptyChoices(t *testing.T) {
	exec := &Executor{}
	resp := &chat.ChatResponse{Choices: []chat.Choice{}}
	content, _ := exec.parseResponse(resp)

	if content != "" {
		t.Errorf("expected empty content, got %q", content)
	}
}

func TestParseResponse_WithContent(t *testing.T) {
	exec := &Executor{}
	resp := &chat.ChatResponse{
		Choices: []chat.Choice{
			{Message: chat.Message{Content: "Hello world"}},
		},
	}
	content, _ := exec.parseResponse(resp)

	if content != "Hello world" {
		t.Errorf("expected 'Hello world', got %q", content)
	}
}

func TestParseResponse_WithToolCalls(t *testing.T) {
	exec := &Executor{}
	resp := &chat.ChatResponse{
		Choices: []chat.Choice{
			{
				Message: chat.Message{
					Content: "Using tool...",
					ToolCalls: []chat.ToolCall{
						{ID: "1", Function: chat.FunctionCall{Name: "read_file"}},
					},
				},
			},
		},
	}
	_, toolCalls := exec.parseResponse(resp)

	if len(toolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(toolCalls))
	}
	if toolCalls[0].Function.Name != "read_file" {
		t.Errorf("expected 'read_file', got %q", toolCalls[0].Function.Name)
	}
}

// ─── Tests: detectStageTransition ────────────────────────────────────────────

func TestDetectStageTransition_Found(t *testing.T) {
	exec := &Executor{}
	stage := exec.detectStageTransition("[STAGE: knowledge_retrieval]")

	if stage != "knowledge_retrieval" {
		t.Errorf("expected 'knowledge_retrieval', got %q", stage)
	}
}

func TestDetectStageTransition_NotFound(t *testing.T) {
	exec := &Executor{}
	stage := exec.detectStageTransition("regular text without stage marker")
	if stage != "" {
		t.Errorf("expected empty string, got %q", stage)
	}
}

func TestDetectStageTransition_PartialMatch(t *testing.T) {
	exec := &Executor{}
	stage := exec.detectStageTransition("[STAGE: incomplete")
	if stage != "" {
		t.Errorf("expected empty string for partial match, got %q", stage)
	}
}

func TestDetectStageTransition_Whitespace(t *testing.T) {
	exec := &Executor{}
	stage := exec.detectStageTransition("[STAGE:  agent_routing  ]")
	if stage != "agent_routing" {
		t.Errorf("expected 'agent_routing', got %q", stage)
	}
}

// ─── Tests: Tool Derivation ──────────────────────────────────────────────────

func TestDeriveTools_Backend(t *testing.T) {
	exec := &Executor{}
	data := NewPipelineData()
	data.AgentRole = "Backend Chief"
	data.AgentDepartment = "backend"
	tools := exec.deriveTools(data)

	if len(tools) == 0 {
		t.Fatal("expected at least one tool")
	}

	// Should include read_file, write_file, search_codebase.
	names := toolDefinitionNames(tools)
	if !containsStr(names, "read_file") {
		t.Error("expected read_file tool")
	}
	if !containsStr(names, "write_file") {
		t.Error("expected write_file tool")
	}
	if !containsStr(names, "search_codebase") {
		t.Error("expected search_codebase tool")
	}
}

func TestDeriveTools_Database(t *testing.T) {
	exec := &Executor{}
	data := NewPipelineData()
	data.AgentRole = "Database Chief"
	data.AgentDepartment = "database"
	tools := exec.deriveTools(data)

	names := toolDefinitionNames(tools)
	if !containsStr(names, "execute_sql") {
		t.Error("expected execute_sql tool for database role")
	}
}

func TestDeriveTools_Testing(t *testing.T) {
	exec := &Executor{}
	data := NewPipelineData()
	data.AgentRole = "Testing Chief"
	data.AgentDepartment = "qa"
	tools := exec.deriveTools(data)

	names := toolDefinitionNames(tools)
	if !containsStr(names, "run_tests") {
		t.Error("expected run_tests tool for testing role")
	}
}

func TestDeriveTools_DevOps(t *testing.T) {
	exec := &Executor{}
	data := NewPipelineData()
	data.AgentRole = "DevOps Chief"
	data.AgentDepartment = "devops"
	tools := exec.deriveTools(data)

	names := toolDefinitionNames(tools)
	if !containsStr(names, "execute_command") {
		t.Error("expected execute_command tool for devops role")
	}
}

func TestDeriveTools_Security(t *testing.T) {
	exec := &Executor{}
	data := NewPipelineData()
	data.AgentRole = "Security Chief"
	data.AgentDepartment = "security"
	tools := exec.deriveTools(data)

	names := toolDefinitionNames(tools)
	if !containsStr(names, "scan_vulnerabilities") {
		t.Error("expected scan_vulnerabilities tool for security role")
	}
}

func TestDeriveTools_Deduplication(t *testing.T) {
	// Read_file appears in both backend and frontend; should not be duplicated.
	exec := &Executor{}
	tools := exec.deriveTools(PipelineData{
		AgentRole:       "Backend Frontend Developer",
		AgentDepartment: "backend",
	})

	names := toolDefinitionNames(tools)
	count := 0
	for _, n := range names {
		if n == "read_file" {
			count++
		}
	}
	if count > 1 {
		t.Errorf("expected read_file to not be duplicated, got %d occurrences", count)
	}
}

// ─── Tests: extractString (package-level) ────────────────────────────────────

// extractString is now a package-level function in pipeline.go.
// Tests below validate it works on plain maps.

func TestExtractString_Found(t *testing.T) {
	val := extractString(map[string]interface{}{"key": "value"}, "key")
	if val != "value" {
		t.Errorf("expected 'value', got %q", val)
	}
}

func TestExtractString_Missing(t *testing.T) {
	val := extractString(map[string]interface{}{}, "key")
	if val != "" {
		t.Errorf("expected empty string, got %q", val)
	}
}

func TestExtractString_WrongType(t *testing.T) {
	val := extractString(map[string]interface{}{"key": 42}, "key")
	if val != "" {
		t.Errorf("expected empty string for wrong type, got %q", val)
	}
}

// ─── Tests: buildChatOptions ─────────────────────────────────────────────────

func TestBuildChatOptions_DefaultOptions(t *testing.T) {
	exec := NewExecutor(newMockChatProvider("t", "m"), DefaultExecutorConfig(), nil)
	opts := exec.buildChatOptions(PipelineData{})

	if opts.Temperature != 0.7 {
		t.Errorf("expected default temperature 0.7, got %f", opts.Temperature)
	}
}

func TestBuildChatOptions_WithDerivedTools(t *testing.T) {
	exec := NewExecutor(newMockChatProvider("t", "m"), DefaultExecutorConfig(), nil)
	opts := exec.buildChatOptions(PipelineData{
		AgentRole:       "Backend Chief",
		AgentDepartment: "backend",
	})

	if len(opts.Tools) == 0 {
		t.Fatal("expected tools to be derived for backend role")
	}
}

// ─── Tests: GenerateRequestID ────────────────────────────────────────────────

func TestGenerateRequestID_IsValidUUID(t *testing.T) {
	id := GenerateRequestID()
	if id == "" {
		t.Error("expected non-empty request ID")
	}
	if len(id) != 36 {
		t.Errorf("expected UUID length 36, got %d (%s)", len(id), id)
	}
}

func TestGenerateRequestID_Unique(t *testing.T) {
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := GenerateRequestID()
		if ids[id] {
			t.Errorf("duplicate UUID generated: %s", id)
		}
		ids[id] = true
	}
}

// ─── Tests: toolCallNames ────────────────────────────────────────────────────

func TestToolCallNames(t *testing.T) {
	tcs := []chat.ToolCall{
		{ID: "1", Function: chat.FunctionCall{Name: "read_file"}},
		{ID: "2", Function: chat.FunctionCall{Name: "write_file"}},
	}
	names := toolCallNames(tcs)

	if len(names) != 2 {
		t.Fatalf("expected 2 names, got %d", len(names))
	}
	if names[0] != "read_file" {
		t.Errorf("expected 'read_file', got %q", names[0])
	}
	if names[1] != "write_file" {
		t.Errorf("expected 'write_file', got %q", names[1])
	}
}

func TestToolCallNames_Empty(t *testing.T) {
	names := toolCallNames(nil)
	if len(names) != 0 {
		t.Errorf("expected 0 names, got %d", len(names))
	}
}

// ─── Test: Execute_AutoFillAgentDefaults ─────────────────────────────────────

func TestExecute_DefaultAgentWhenNoneResolved(t *testing.T) {
	provider := newMockChatProvider("test", "test-model")
	var capturedMessages []chat.Message
	provider.chatResponse = &chat.ChatResponse{
		ID:    "resp-default",
		Model: "test-model",
		Choices: []chat.Choice{
			{Index: 0, Message: chat.Message{Role: chat.RoleAssistant, Content: "ok"}, FinishReason: chat.FinishReasonStop},
		},
		Usage: chat.Usage{TotalTokens: 3},
	}
	provider.chatFn = func(_ context.Context, messages []chat.Message, _ chat.ChatOptions) (*chat.ChatResponse, error) {
		capturedMessages = messages
		return provider.chatResponse, nil
	}

	exec := NewExecutor(provider, DefaultExecutorConfig(), nil)

	// No agent info in context data.
	pc := NewPipelineContext("req-default", "hello")

	_, err := exec.Execute(context.Background(), pc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	systemMsg := capturedMessages[0]
	if !strings.Contains(systemMsg.Content, "COSCA KERNEL") {
		t.Errorf("expected default 'COSCA KERNEL' in system prompt, got: %s", systemMsg.Content)
	}
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func toolDefinitionNames(tools []chat.ToolDefinition) []string {
	names := make([]string, len(tools))
	for i, t := range tools {
		names[i] = t.Function.Name
	}
	return names
}

func containsStr(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
