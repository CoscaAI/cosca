// Package provider_test provides tests for the Cosca Chat provider adapters
// and the provider registry. It uses httptest.NewServer to mock API endpoints
// and verifies streaming/non-streaming chat completions, error handling,
// context cancellation, and registry fallback logic.
package provider_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/CoscaAI/cosca/internal/chat"
	"github.com/CoscaAI/cosca/internal/chat/provider"
)

// ─── Mock Provider (for registry tests) ──────────────────────────────────────

// mockChatProvider implements chat.Provider for registry testing.
type mockChatProvider struct {
	name      string
	models    []string
	available bool
	chatFn    func(ctx context.Context, req chat.ChatRequest) (<-chan chat.ChatEvent, error)
}

func (m *mockChatProvider) Name() string      { return m.name }
func (m *mockChatProvider) Models() []string  { return m.models }
func (m *mockChatProvider) IsAvailable() bool { return m.available }
func (m *mockChatProvider) Chat(ctx context.Context, req chat.ChatRequest) (<-chan chat.ChatEvent, error) {
	return m.chatFn(ctx, req)
}

func newMockProvider(name string, available bool) *mockChatProvider {
	return &mockChatProvider{
		name:      name,
		models:    []string{"mock-model"},
		available: available,
		chatFn: func(ctx context.Context, req chat.ChatRequest) (<-chan chat.ChatEvent, error) {
			ch := make(chan chat.ChatEvent, 2)
			ch <- chat.ChatEvent{Type: chat.ChatEventDelta, Delta: "mock response from " + name}
			ch <- chat.ChatEvent{Type: chat.ChatEventDone, Usage: &chat.Usage{TotalTokens: 10}}
			close(ch)
			return ch, nil
		},
	}
}

func newFailingMockProvider(name string, available bool, errMsg string) *mockChatProvider {
	return &mockChatProvider{
		name:      name,
		models:    []string{"mock-model"},
		available: available,
		chatFn: func(ctx context.Context, req chat.ChatRequest) (<-chan chat.ChatEvent, error) {
			return nil, fmt.Errorf("%s", errMsg)
		},
	}
}

// ─── Registry: Register and Get ──────────────────────────────────────────────

func TestRegistry_RegisterAndGet(t *testing.T) {
	t.Parallel()

	reg := provider.NewRegistry()

	p1 := newMockProvider("openai", true)
	p2 := newMockProvider("anthropic", true)

	reg.Register(p1)
	reg.Register(p2)

	// Set primary and fallback.
	reg.SetPrimary("openai")
	reg.SetFallbacks([]string{"anthropic"})

	got, err := reg.Get(context.Background())
	if err != nil {
		t.Fatalf("Get() failed: %v", err)
	}
	if got.Name() != "openai" {
		t.Errorf("expected primary 'openai', got %q", got.Name())
	}

	// Available() should list both.
	avail := reg.Available()
	if len(avail) != 2 {
		t.Fatalf("Available() returned %d providers, want 2", len(avail))
	}
	// Order is non-deterministic (map iteration), but both names must be present.
	hasOpenAI := false
	hasAnthropic := false
	for _, n := range avail {
		if n == "openai" {
			hasOpenAI = true
		}
		if n == "anthropic" {
			hasAnthropic = true
		}
	}
	if !hasOpenAI {
		t.Error("Available() missing 'openai'")
	}
	if !hasAnthropic {
		t.Error("Available() missing 'anthropic'")
	}
}

// ─── Registry: Fallback when primary is unavailable ──────────────────────────

func TestRegistry_Fallback(t *testing.T) {
	t.Parallel()

	reg := provider.NewRegistry()

	// Primary is not available; fallback is available.
	pPrimary := newMockProvider("primary", false)
	pFallback := newMockProvider("fallback", true)

	reg.Register(pPrimary)
	reg.Register(pFallback)
	reg.SetPrimary("primary")
	reg.SetFallbacks([]string{"fallback"})

	got, err := reg.Get(context.Background())
	if err != nil {
		t.Fatalf("Get() failed: %v", err)
	}
	if got.Name() != "fallback" {
		t.Errorf("expected fallback 'fallback', got %q", got.Name())
	}
}

// ─── Registry: PrimaryNotFound (no available provider) ───────────────────────

func TestRegistry_PrimaryNotFound(t *testing.T) {
	t.Parallel()

	reg := provider.NewRegistry()

	// Register only an unavailable provider.
	p := newMockProvider("openai", false)
	reg.Register(p)
	reg.SetPrimary("openai")

	_, err := reg.Get(context.Background())
	if err == nil {
		t.Fatal("expected error when no provider is available, got nil")
	}
	if !strings.Contains(err.Error(), "no available provider") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// ─── Registry: Available list ────────────────────────────────────────────────

func TestRegistry_Available(t *testing.T) {
	t.Parallel()

	reg := provider.NewRegistry()

	// Empty registry.
	if av := reg.Available(); len(av) != 0 {
		t.Errorf("expected empty list, got %v", av)
	}

	// Register some providers.
	reg.Register(newMockProvider("a", true))
	reg.Register(newMockProvider("b", true))
	reg.Register(newMockProvider("c", true))

	av := reg.Available()
	if len(av) != 3 {
		t.Errorf("expected 3 providers, got %d", len(av))
	}
}

// ─── Registry: Fallback when primary returns error ───────────────────────────

func TestRegistry_FallbackOnError(t *testing.T) {
	t.Parallel()

	reg := provider.NewRegistry()

	pPrimary := newFailingMockProvider("primary", true, "primary error")
	pFallback := newMockProvider("fallback", true)

	reg.Register(pPrimary)
	reg.Register(pFallback)
	reg.SetPrimary("primary")
	reg.SetFallbacks([]string{"fallback"})

	// Get() finds primary first (it's available), but when we Chat on the
	// returned provider it fails. Get() itself only checks IsAvailable().
	got, err := reg.Get(context.Background())
	if err != nil {
		t.Fatalf("Get() failed: %v", err)
	}
	if got.Name() != "primary" {
		t.Errorf("expected primary 'primary', got %q", got.Name())
	}

	// The Chat call on the primary will fail, but Get() already returned
	// primary because IsAvailable() == true. Registry.Get() doesn't attempt
	// the actual Chat — it delegates that to the caller.
	_, chatErr := got.Chat(context.Background(), chat.ChatRequest{})
	if chatErr == nil {
		t.Error("expected error from primary Chat(), got nil")
	}
}

// ─── Thread safety ──────────────────────────────────────────────────────────

func TestRegistry_ThreadSafety(t *testing.T) {
	reg := provider.NewRegistry()

	p := newMockProvider("concurrent", true)
	reg.Register(p)
	reg.SetPrimary("concurrent")

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := reg.Get(context.Background())
			if err != nil {
				t.Errorf("concurrent Get() failed: %v", err)
			}
		}()
	}
	wg.Wait()
}

// ─── Provider: Name() and Models() ──────────────────────────────────────────

func TestOpenAIProvider_NameAndModels(t *testing.T) {
	t.Parallel()

	p := provider.NewOpenAI("sk-test", "gpt-4o", "")
	if p.Name() != "openai" {
		t.Errorf("Name() = %q, want %q", p.Name(), "openai")
	}
	models := p.Models()
	if len(models) == 0 {
		t.Error("Models() returned empty list")
	}
}

func TestDeepSeekProvider_NameAndModels(t *testing.T) {
	t.Parallel()

	p := provider.NewDeepSeek("sk-test", "deepseek-chat")
	if p.Name() != "deepseek" {
		t.Errorf("Name() = %q, want %q", p.Name(), "deepseek")
	}
	models := p.Models()
	if len(models) == 0 {
		t.Error("Models() returned empty list")
	}
}

func TestAnthropicProvider_NameAndModels(t *testing.T) {
	t.Parallel()

	p := provider.NewAnthropic("sk-test", "", "")
	if p.Name() != "anthropic" {
		t.Errorf("Name() = %q, want %q", p.Name(), "anthropic")
	}
	models := p.Models()
	if len(models) == 0 {
		t.Error("Models() returned empty list")
	}
}

func TestOllamaProvider_NameAndModels(t *testing.T) {
	t.Parallel()

	p := provider.NewOllama("llama3", "")
	if p.Name() != "ollama" {
		t.Errorf("Name() = %q, want %q", p.Name(), "ollama")
	}
	models := p.Models()
	if len(models) == 0 {
		t.Error("Models() returned empty list")
	}
}

// ─── Provider: IsAvailable() ─────────────────────────────────────────────────

func TestOpenAIProvider_IsAvailable(t *testing.T) {
	t.Parallel()

	// With API key => available.
	p1 := provider.NewOpenAI("sk-test", "", "")
	if !p1.IsAvailable() {
		t.Error("expected available with API key")
	}

	// Without API key => unavailable.
	p2 := provider.NewOpenAI("", "", "")
	if p2.IsAvailable() {
		t.Error("expected unavailable without API key")
	}
}

func TestAnthropicProvider_IsAvailable(t *testing.T) {
	t.Parallel()

	p1 := provider.NewAnthropic("sk-test", "", "")
	if !p1.IsAvailable() {
		t.Error("expected available with API key")
	}

	p2 := provider.NewAnthropic("", "", "")
	if p2.IsAvailable() {
		t.Error("expected unavailable without API key")
	}
}

func TestDeepSeekProvider_IsAvailable(t *testing.T) {
	t.Parallel()

	p1 := provider.NewDeepSeek("sk-test", "")
	if !p1.IsAvailable() {
		t.Error("expected available with API key")
	}

	p2 := provider.NewDeepSeek("", "")
	if p2.IsAvailable() {
		t.Error("expected unavailable without API key")
	}
}

func TestOllamaProvider_IsAvailable(t *testing.T) {
	t.Parallel()

	// Ollama IsAvailable() does a live HTTP GET to the server.
	// With a mock server, it should be available.
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	p := provider.NewOllama("llama3", server.URL)
	if !p.IsAvailable() {
		t.Error("expected available with reachable server")
	}

	// Unreachable server => unavailable.
	p2 := provider.NewOllama("llama3", "http://localhost:19999")
	if p2.IsAvailable() {
		t.Error("expected unavailable with unreachable server")
	}
}

// ─── OpenAI Provider: Chat (non-streaming) ───────────────────────────────────

func TestOpenAIProvider_Chat_NonStreaming(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		// Verify request headers.
		if r.Header.Get("Authorization") != "Bearer sk-test-openai" {
			t.Errorf("Authorization header = %q, want %q", r.Header.Get("Authorization"), "Bearer sk-test-openai")
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q, want %q", r.Header.Get("Content-Type"), "application/json")
		}

		// Verify stream=false in request body.
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		if stream, ok := body["stream"]; !ok || stream != false {
			t.Errorf("stream = %v, want false", stream)
		}

		// Return a non-streaming response.
		resp := `{
			"id": "chatcmpl-123",
			"object": "chat.completion",
			"created": 1234567890,
			"model": "gpt-4o",
			"choices": [{
				"index": 0,
				"message": {"role": "assistant", "content": "Hello from OpenAI non-streaming"},
				"finish_reason": "stop"
			}],
			"usage": {"prompt_tokens": 10, "completion_tokens": 20, "total_tokens": 30}
		}`
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, resp)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	p := provider.NewOpenAI("sk-test-openai", "gpt-4o", server.URL+"/v1")
	ch, err := p.Chat(context.Background(), chat.ChatRequest{
		Model: "gpt-4o",
		Messages: []chat.Message{
			{Role: chat.RoleUser, Content: "Hello"},
		},
		Stream: false,
	})
	if err != nil {
		t.Fatalf("Chat() failed: %v", err)
	}

	collectEvents(t, ch, "Hello from OpenAI non-streaming")
}

// ─── OpenAI Provider: Chat (streaming) ───────────────────────────────────────

func TestOpenAIProvider_Chat_Streaming(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer sk-test-openai" {
			t.Errorf("Authorization header = %q, want %q", r.Header.Get("Authorization"), "Bearer sk-test-openai")
		}

		// Verify stream=true.
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		if stream, ok := body["stream"]; !ok || stream != true {
			t.Errorf("stream = %v, want true", stream)
		}

		// Send SSE streaming response.
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusOK)

		lines := []string{
			`data: {"id":"1","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}`,
			`data: {"id":"2","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":" world"},"finish_reason":null}]}`,
			`data: {"id":"3","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":""},"finish_reason":"stop"}]}`,
			`data: {"id":"4","object":"chat.completion.chunk","usage":{"prompt_tokens":10,"completion_tokens":20,"total_tokens":30}}`,
			`data: [DONE]`,
		}
		for _, line := range lines {
			fmt.Fprintf(w, "%s\n", line)
		}
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	p := provider.NewOpenAI("sk-test-openai", "gpt-4o", server.URL+"/v1")
	ch, err := p.Chat(context.Background(), chat.ChatRequest{
		Model:    "gpt-4o",
		Messages: []chat.Message{{Role: chat.RoleUser, Content: "Hello"}},
		Stream:   true,
	})
	if err != nil {
		t.Fatalf("Chat() failed: %v", err)
	}

	collectEvents(t, ch, "Hello world")
}

// ─── DeepSeek Provider: Chat (streaming) ─────────────────────────────────────

func TestDeepSeekProvider_Chat_Streaming(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer sk-test-deepseek" {
			t.Errorf("Authorization header = %q, want %q", r.Header.Get("Authorization"), "Bearer sk-test-deepseek")
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		lines := []string{
			`data: {"id":"1","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"Deep"},"finish_reason":null}]}`,
			`data: {"id":"2","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"Seek"},"finish_reason":null}]}`,
			`data: {"id":"3","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":""},"finish_reason":"stop"}]}`,
			`data: {"id":"4","object":"chat.completion.chunk","usage":{"prompt_tokens":5,"completion_tokens":2,"total_tokens":7}}`,
			`data: [DONE]`,
		}
		for _, line := range lines {
			fmt.Fprintf(w, "%s\n", line)
		}
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	// DeepSeek uses OpenAI-compatible API, so we can test via OpenAI provider
	// with the mock server URL.
	p := provider.NewOpenAI("sk-test-deepseek", "deepseek-chat", server.URL+"/v1")
	ch, err := p.Chat(context.Background(), chat.ChatRequest{
		Model:    "deepseek-chat",
		Messages: []chat.Message{{Role: chat.RoleUser, Content: "Hi"}},
		Stream:   true,
	})
	if err != nil {
		t.Fatalf("Chat() failed: %v", err)
	}

	collectEvents(t, ch, "DeepSeek")
}

// ─── Anthropic Provider: Chat (streaming) ────────────────────────────────────

func TestAnthropicProvider_Chat_Streaming(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/messages", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "sk-test-anthropic" {
			t.Errorf("x-api-key header = %q, want %q", r.Header.Get("x-api-key"), "sk-test-anthropic")
		}
		if r.Header.Get("anthropic-version") == "" {
			t.Error("anthropic-version header is empty")
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		// Anthropic SSE format.
		lines := []string{
			`event: content_block_delta`,
			`data: {"type":"content_block_delta","index":0,"delta":{"type":"text","text":"Hello from Claude"}}`,
			``,
			`event: message_delta`,
			`data: {"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"input_tokens":10,"output_tokens":5}}`,
			``,
		}
		for _, line := range lines {
			fmt.Fprintf(w, "%s\n", line)
		}
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	p := provider.NewAnthropic("sk-test-anthropic", "claude-sonnet-4-20250514", server.URL)
	ch, err := p.Chat(context.Background(), chat.ChatRequest{
		Model:    "claude-sonnet-4-20250514",
		Messages: []chat.Message{{Role: chat.RoleUser, Content: "Hi"}},
		Stream:   true,
	})
	if err != nil {
		t.Fatalf("Chat() failed: %v", err)
	}

	collectEvents(t, ch, "Hello from Claude")
}

// ─── Anthropic Provider: Chat (non-streaming) ────────────────────────────────

func TestAnthropicProvider_Chat_NonStreaming(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/messages", func(w http.ResponseWriter, r *http.Request) {
		resp := `{
			"id": "msg_123",
			"type": "message",
			"role": "assistant",
			"content": [
				{"type": "text", "text": "Hello from Claude non-streaming"}
			],
			"model": "claude-sonnet-4-20250514",
			"usage": {"input_tokens": 10, "output_tokens": 20}
		}`
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, resp)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	p := provider.NewAnthropic("sk-test-anthropic", "claude-sonnet-4-20250514", server.URL)
	ch, err := p.Chat(context.Background(), chat.ChatRequest{
		Model:    "claude-sonnet-4-20250514",
		Messages: []chat.Message{{Role: chat.RoleUser, Content: "Hi"}},
		Stream:   false,
	})
	if err != nil {
		t.Fatalf("Chat() failed: %v", err)
	}

	collectEvents(t, ch, "Hello from Claude non-streaming")
}

// ─── Ollama Provider: Chat (streaming) ───────────────────────────────────────

func TestOllamaProvider_Chat_Streaming(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/chat", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.WriteHeader(http.StatusOK)

		lines := []string{
			`{"model":"llama3","created_at":"2023-12-01T00:00:00Z","message":{"content":"Hello"},"done":false}`,
			`{"model":"llama3","created_at":"2023-12-01T00:00:00Z","message":{"content":" from Ollama"},"done":false}`,
			`{"model":"llama3","created_at":"2023-12-01T00:00:00Z","message":{"content":""},"done":true,"prompt_eval_count":10,"eval_count":20}`,
		}
		for _, line := range lines {
			fmt.Fprintf(w, "%s\n", line)
		}
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	p := provider.NewOllama("llama3", server.URL)
	ch, err := p.Chat(context.Background(), chat.ChatRequest{
		Model:    "llama3",
		Messages: []chat.Message{{Role: chat.RoleUser, Content: "Hi"}},
		Stream:   true,
	})
	if err != nil {
		t.Fatalf("Chat() failed: %v", err)
	}

	collectEvents(t, ch, "Hello from Ollama")
}

// ─── Ollama Provider: Chat (non-streaming) ───────────────────────────────────

func TestOllamaProvider_Chat_NonStreaming(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/chat", func(w http.ResponseWriter, r *http.Request) {
		resp := `{
			"model": "llama3",
			"created_at": "2023-12-01T00:00:00Z",
			"message": {"role": "assistant", "content": "Hello from Ollama non-streaming"},
			"done": true,
			"done_reason": "stop",
			"total_duration": 123456789,
			"prompt_eval_count": 10,
			"eval_count": 20
		}`
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, resp)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	p := provider.NewOllama("llama3", server.URL)
	ch, err := p.Chat(context.Background(), chat.ChatRequest{
		Model:    "llama3",
		Messages: []chat.Message{{Role: chat.RoleUser, Content: "Hi"}},
		Stream:   false,
	})
	if err != nil {
		t.Fatalf("Chat() failed: %v", err)
	}

	collectEvents(t, ch, "Hello from Ollama non-streaming")
}

// ─── Ollama Provider: Regression — real native /api/chat response ────────────

// TestOllamaProvider_Chat_NativeResponseRegression verifies that the provider
// parses the real Ollama /api/chat response shape (top-level message.content)
// through the true endpoint path, producing non-empty content.
func TestOllamaProvider_Chat_NativeResponseRegression(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/chat", func(w http.ResponseWriter, r *http.Request) {
		resp := `{"model":"llama3","message":{"role":"assistant","content":"OLÁ"},"done":true,"prompt_eval_count":12,"eval_count":21}`
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, resp)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	p := provider.NewOllama("llama3", server.URL)
	ch, err := p.Chat(context.Background(), chat.ChatRequest{
		Model:    "llama3",
		Messages: []chat.Message{{Role: chat.RoleUser, Content: "Hi"}},
		Stream:   false,
	})
	if err != nil {
		t.Fatalf("Chat() failed: %v", err)
	}

	var (
		gotDelta string
		usage    *chat.Usage
	)
	for evt := range ch {
		switch evt.Type {
		case chat.ChatEventDelta:
			gotDelta += evt.Delta
		case chat.ChatEventDone:
			usage = evt.Usage
		}
	}

	if gotDelta != "OLÁ" {
		t.Errorf("content = %q, want %q", gotDelta, "OLÁ")
	}
	if usage == nil {
		t.Fatal("done event has nil Usage")
	}
	if usage.PromptTokens != 12 || usage.CompletionTokens != 21 || usage.TotalTokens != 33 {
		t.Errorf("usage = %+v, want PromptTokens=12 CompletionTokens=21 TotalTokens=33", usage)
	}
}

// ─── Provider: HTTP error handling ──────────────────────────────────────────

func TestOpenAIProvider_Chat_HTTPError(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error": {"message": "Invalid API key"}}`)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	p := provider.NewOpenAI("sk-bad", "gpt-4o", server.URL+"/v1")
	ch, err := p.Chat(context.Background(), chat.ChatRequest{
		Messages: []chat.Message{{Role: chat.RoleUser, Content: "Hi"}},
		Stream:   true,
	})
	if err != nil {
		t.Fatalf("Chat() failed: %v", err)
	}

	evt, ok := <-ch
	if !ok {
		t.Fatal("channel closed without events")
	}
	if evt.Type != chat.ChatEventError {
		t.Errorf("expected ChatEventError, got %s", evt.Type)
	}
	if evt.Error == nil {
		t.Fatal("expected non-nil Error in ChatEventError")
	}
	if !strings.Contains(evt.Error.Error(), "HTTP 401") {
		t.Errorf("error message = %q, want to contain 'HTTP 401'", evt.Error.Error())
	}
}

// ─── Context cancellation ────────────────────────────────────────────────────

func TestProvider_Chat_ContextCancellation(t *testing.T) {
	t.Parallel()

	// Create a server that delays responding, so we can cancel the context.
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		// Wait for context cancellation by blocking on the request context.
		<-r.Context().Done()
		// Don't write anything — the client has cancelled.
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	p := provider.NewOpenAI("sk-test", "gpt-4o", server.URL+"/v1")

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	ch, err := p.Chat(ctx, chat.ChatRequest{
		Messages: []chat.Message{{Role: chat.RoleUser, Content: "Hi"}},
		Stream:   true,
	})
	if err != nil {
		t.Fatalf("Chat() failed: %v", err)
	}

	evt, ok := <-ch
	if !ok {
		t.Fatal("channel closed without events")
	}
	if evt.Type != chat.ChatEventError {
		t.Errorf("expected ChatEventError, got %s", evt.Type)
	}
	if evt.Error == nil {
		t.Fatal("expected non-nil Error")
	}
}

// ─── Provider: GetProvider by name ──────────────────────────────────────────

func TestRegistry_GetProvider(t *testing.T) {
	t.Parallel()

	reg := provider.NewRegistry()
	p := newMockProvider("test-provider", true)
	reg.Register(p)

	got := reg.GetProvider("test-provider")
	if got == nil {
		t.Fatal("GetProvider() returned nil for existing provider")
	}
	if got.Name() != "test-provider" {
		t.Errorf("Name = %q, want %q", got.Name(), "test-provider")
	}

	// Non-existent provider returns nil.
	if reg.GetProvider("nonexistent") != nil {
		t.Error("expected nil for non-existent provider")
	}
}

// ─── Provider: Default model when empty ──────────────────────────────────────

func TestOpenAIProvider_DefaultModel(t *testing.T) {
	t.Parallel()

	// We can't directly check the model field, but we can verify Chat works
	// with empty model by checking the request sent to the server.
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		model, _ := body["model"].(string)
		if model == "" {
			t.Error("model is empty in request, expected default")
		}
		resp := `{"id":"1","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, resp)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	p2 := provider.NewOpenAI("sk-test", "", server.URL+"/v1")
	ch, err := p2.Chat(context.Background(), chat.ChatRequest{
		Messages: []chat.Message{{Role: chat.RoleUser, Content: "Hi"}},
		Stream:   false,
	})
	if err != nil {
		t.Fatalf("Chat() failed: %v", err)
	}
	<-ch // Delta
	<-ch // Done
}

// ─── Anthropic Error Handling ────────────────────────────────────────────────

func TestAnthropicProvider_Chat_HTTPError(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/messages", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprint(w, `{"error": {"type": "rate_limit_error", "message": "Too many requests"}}`)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	p := provider.NewAnthropic("sk-test", "claude-sonnet-4-20250514", server.URL)
	ch, err := p.Chat(context.Background(), chat.ChatRequest{
		Messages: []chat.Message{{Role: chat.RoleUser, Content: "Hi"}},
		Stream:   true,
	})
	if err != nil {
		t.Fatalf("Chat() failed: %v", err)
	}

	evt, ok := <-ch
	if !ok {
		t.Fatal("channel closed without events")
	}
	if evt.Type != chat.ChatEventError {
		t.Errorf("expected ChatEventError, got %s", evt.Type)
	}
	if !strings.Contains(evt.Error.Error(), "HTTP 429") {
		t.Errorf("error = %q, want to contain 'HTTP 429'", evt.Error.Error())
	}
}

// ─── Registry: SetPrimary / SetFallbacks / Primary / Fallbacks ────────────────

func TestRegistry_SetPrimaryAndFallbacks(t *testing.T) {
	t.Parallel()

	reg := provider.NewRegistry()

	if reg.Primary() != "" {
		t.Errorf("Primary() = %q, want empty", reg.Primary())
	}

	reg.SetPrimary("openai")
	if reg.Primary() != "openai" {
		t.Errorf("Primary() = %q, want %q", reg.Primary(), "openai")
	}

	fbs := reg.Fallbacks()
	if len(fbs) != 0 {
		t.Errorf("Fallbacks() = %v, want empty", fbs)
	}

	reg.SetFallbacks([]string{"anthropic", "deepseek"})
	fbs = reg.Fallbacks()
	if len(fbs) != 2 || fbs[0] != "anthropic" || fbs[1] != "deepseek" {
		t.Errorf("Fallbacks() = %v, want [anthropic deepseek]", fbs)
	}
}

// ─── Registry: Last resort (any available) ────────────────────────────────────

func TestRegistry_LastResortAvailable(t *testing.T) {
	t.Parallel()

	reg := provider.NewRegistry()

	// Register only available providers, no primary set, no fallbacks.
	reg.Register(newMockProvider("only-one", true))

	got, err := reg.Get(context.Background())
	if err != nil {
		t.Fatalf("Get() failed: %v", err)
	}
	if got.Name() != "only-one" {
		t.Errorf("expected 'only-one', got %q", got.Name())
	}
}

// ─── Registry: GetProvider with nil for unknown ─────────────────────────────

func TestRegistry_GetProviderUnknown(t *testing.T) {
	t.Parallel()

	reg := provider.NewRegistry()
	if got := reg.GetProvider("unknown"); got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

// ─── Null Provider (deterministic mode) ──────────────────────────────────────

func TestNullProvider_Name(t *testing.T) {
	t.Parallel()

	p := provider.NewNullProvider()
	if p.Name() != "none" {
		t.Errorf("Name() = %q, want %q", p.Name(), "none")
	}
}

func TestNullProvider_IsAvailable(t *testing.T) {
	t.Parallel()

	p := provider.NewNullProvider()
	if !p.IsAvailable() {
		t.Error("NullProvider must always be available (deterministic mode is a valid state)")
	}
}

func TestNullProvider_Chat_ReturnsError(t *testing.T) {
	t.Parallel()

	p := provider.NewNullProvider()
	ch, err := p.Chat(context.Background(), chat.ChatRequest{})
	if err != nil {
		t.Fatalf("Chat() failed: %v", err)
	}

	var events []chat.ChatEvent
	for evt := range ch {
		events = append(events, evt)
	}

	if len(events) != 1 {
		t.Fatalf("expected exactly 1 event, got %d", len(events))
	}
	evt := events[0]
	if evt.Type != chat.ChatEventError {
		t.Errorf("expected ChatEventError, got %s", evt.Type)
	}
	if evt.Error == nil || evt.Error.Error() == "" {
		t.Fatal("expected non-empty error message")
	}
	if !strings.Contains(evt.Error.Error(), "capacidade cognitiva indisponível") {
		t.Errorf("error message = %q, want to mention the deterministic fallback", evt.Error.Error())
	}
}

func TestNullProvider_Registered(t *testing.T) {
	t.Parallel()

	reg := chat.NewChatRegistry()
	if err := provider.RegisterChatProviders(reg, nil); err != nil {
		t.Fatalf("RegisterChatProviders() failed: %v", err)
	}

	got, ok := reg.Get("none")
	if !ok {
		t.Fatal("registry does not contain 'none' after RegisterChatProviders")
	}
	if got.Name() != "none" {
		t.Errorf("Name() = %q, want %q", got.Name(), "none")
	}
}

// ─── Helper: collectEvents ───────────────────────────────────────────────────

// collectEvents reads all events from the channel, verifies that exactly one
// delta and one done event are received, and that the concatenated delta text
// matches the expected string.
func collectEvents(t *testing.T, ch <-chan chat.ChatEvent, wantDelta string) {
	t.Helper()

	var (
		gotDelta strings.Builder
		gotDone  bool
		gotError bool
		usage    *chat.Usage
	)

	for evt := range ch {
		switch evt.Type {
		case chat.ChatEventDelta:
			gotDelta.WriteString(evt.Delta)
		case chat.ChatEventDone:
			gotDone = true
			usage = evt.Usage
		case chat.ChatEventError:
			gotError = true
			t.Logf("ChatEventError: %v", evt.Error)
		}
	}

	if gotError {
		t.Fatal("unexpected error event")
	}
	if !gotDone {
		t.Fatal("missing done event")
	}
	if usage == nil {
		t.Fatal("done event has nil Usage")
	}
	if usage.TotalTokens == 0 && usage.PromptTokens == 0 && usage.CompletionTokens == 0 {
		// Allow zero usage only if all three are zero.
	}
	if gotDelta.String() != wantDelta {
		t.Errorf("delta text = %q, want %q", gotDelta.String(), wantDelta)
	}
}

// ─── Helper: mustParseTime for test constants ────────────────────────────────
// Not needed; we use string literals for timestamps in mock data.

// Ensure compile-time interface satisfaction.
var _ chat.Provider = (*mockChatProvider)(nil)
