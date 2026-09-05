package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/CoscaAI/cosca/api/rest/handler"
	"github.com/CoscaAI/cosca/internal/agents"
	"github.com/CoscaAI/cosca/internal/chat"
	"github.com/CoscaAI/cosca/internal/orchestration"
)

// TestExecuteEmptyPrompt verifies that Execute returns 400 when the
// prompt is empty.
func TestExecuteEmptyPrompt(t *testing.T) {
	chat.ResetRegistry()
	orchestration.ResetExecutionStore()

	agentsMgr := agents.NewManager("")
	h := handler.NewRunHandler(agentsMgr, nil, nil)

	body := `{"prompt":""}`
	req := httptest.NewRequest("POST", "/v1/run", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Execute(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request, got %d: %s", w.Code, w.Body.String())
	}
}

// TestExecuteMissingPrompt verifies that Execute returns 400 when the
// prompt field is missing from the request body.
func TestExecuteMissingPrompt(t *testing.T) {
	chat.ResetRegistry()
	orchestration.ResetExecutionStore()

	agentsMgr := agents.NewManager("")
	h := handler.NewRunHandler(agentsMgr, nil, nil)

	body := `{"agent":"general"}`
	req := httptest.NewRequest("POST", "/v1/run", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Execute(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request, got %d: %s", w.Code, w.Body.String())
	}
}

// TestExecuteInvalidJSON verifies that Execute returns 400 when the
// request body is not valid JSON.
func TestExecuteInvalidJSON(t *testing.T) {
	chat.ResetRegistry()
	orchestration.ResetExecutionStore()

	agentsMgr := agents.NewManager("")
	h := handler.NewRunHandler(agentsMgr, nil, nil)

	req := httptest.NewRequest("POST", "/v1/run", strings.NewReader("not-json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Execute(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request, got %d", w.Code)
	}
}

// TestExecutePromptTooLong verifies that Execute returns 400 when the
// prompt exceeds the maximum allowed length.
func TestExecutePromptTooLong(t *testing.T) {
	chat.ResetRegistry()
	orchestration.ResetExecutionStore()

	agentsMgr := agents.NewManager("")
	h := handler.NewRunHandler(agentsMgr, nil, nil)

	// Build a prompt that exceeds maxPromptLength (10000).
	longPrompt := strings.Repeat("a", 10001)
	body := `{"prompt":"` + longPrompt + `"}`
	req := httptest.NewRequest("POST", "/v1/run", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Execute(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request, got %d: %s", w.Code, w.Body.String())
	}
}

// TestExecuteNoChatProvider verifies that Execute returns 503 when no
// chat provider is available (nil registry and the global registry has
// no selected provider).
func TestExecuteNoChatProvider(t *testing.T) {
	chat.ResetRegistry()
	orchestration.ResetExecutionStore()

	// With no providers registered and no selection made, the default
	// chat.GetRegistry() will have Name()=="chat-registry" and Model()=="".
	agentsMgr := agents.NewManager("")
	h := handler.NewRunHandler(agentsMgr, nil, nil)

	body := `{"prompt":"Hello, world!"}`
	req := httptest.NewRequest("POST", "/v1/run", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Execute(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 Service Unavailable, got %d: %s", w.Code, w.Body.String())
	}
}

// TestExecutePromptAtMaxLength verifies that prompts at exactly the maximum
// length are accepted (they pass validation — whether they succeed depends
// on provider availability).
func TestExecutePromptAtMaxLength(t *testing.T) {
	chat.ResetRegistry()
	orchestration.ResetExecutionStore()

	agentsMgr := agents.NewManager("")
	h := handler.NewRunHandler(agentsMgr, nil, nil)

	// Prompt at exactly the max length (10000).
	maxPrompt := strings.Repeat("a", 10000)
	body := `{"prompt":"` + maxPrompt + `"}`
	req := httptest.NewRequest("POST", "/v1/run", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Execute(w, req)

	// Should NOT be 400 from prompt length — the prompt is valid.
	// It will be 503 because no provider is configured, but not 400.
	if w.Code == http.StatusBadRequest {
		t.Errorf("max-length prompt should pass validation, got 400: %s", w.Body.String())
	}
}

// TestStreamEmptyPrompt verifies that Stream returns 400 when the
// prompt is empty.
func TestStreamEmptyPrompt(t *testing.T) {
	chat.ResetRegistry()
	orchestration.ResetExecutionStore()

	agentsMgr := agents.NewManager("")
	h := handler.NewRunHandler(agentsMgr, nil, nil)

	body := `{"prompt":""}`
	req := httptest.NewRequest("POST", "/v1/run/stream", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Stream(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request, got %d: %s", w.Code, w.Body.String())
	}
}

// TestStreamInvalidJSON verifies that Stream returns 400 when the
// request body is not valid JSON.
func TestStreamInvalidJSON(t *testing.T) {
	chat.ResetRegistry()
	orchestration.ResetExecutionStore()

	agentsMgr := agents.NewManager("")
	h := handler.NewRunHandler(agentsMgr, nil, nil)

	req := httptest.NewRequest("POST", "/v1/run/stream", strings.NewReader("garbage"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Stream(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request, got %d", w.Code)
	}
}

// TestStreamPromptTooLong verifies that Stream returns 400 when the
// prompt exceeds the maximum allowed length.
func TestStreamPromptTooLong(t *testing.T) {
	chat.ResetRegistry()
	orchestration.ResetExecutionStore()

	agentsMgr := agents.NewManager("")
	h := handler.NewRunHandler(agentsMgr, nil, nil)

	longPrompt := strings.Repeat("a", 10001)
	body := `{"prompt":"` + longPrompt + `"}`
	req := httptest.NewRequest("POST", "/v1/run/stream", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Stream(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request, got %d: %s", w.Code, w.Body.String())
	}
}

// TestStreamNoChatProvider verifies that Stream returns 503 when no
// chat provider is available.
func TestStreamNoChatProvider(t *testing.T) {
	chat.ResetRegistry()
	orchestration.ResetExecutionStore()

	agentsMgr := agents.NewManager("")
	h := handler.NewRunHandler(agentsMgr, nil, nil)

	body := `{"prompt":"Hello, world!"}`
	req := httptest.NewRequest("POST", "/v1/run/stream", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Stream(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 Service Unavailable, got %d: %s", w.Code, w.Body.String())
	}
}

// TestExecuteResponseFormat verifies the response JSON format from a
// successful execution. This test requires a registered chat provider.
func TestExecuteResponseFormat(t *testing.T) {
	// Register a mock chat provider that returns a fixed response.
	chat.ResetRegistry()
	orchestration.ResetExecutionStore()

	chat.GetRegistry().Register("test-mock", mockProviderFactory, "Test mock provider", 1)

	// Select the mock provider so it's available.
	err := chat.GetRegistry().Select(nil, chat.ChatRegistryConfig{
		Primary:    "test-mock",
		AutoDetect: false,
	})
	if err != nil {
		t.Fatalf("failed to select mock provider: %v", err)
	}

	agentsMgr := agents.NewManager("")
	h := handler.NewRunHandler(agentsMgr, chat.GetRegistry(), nil)

	body := `{"prompt":"What is Go?"}`
	req := httptest.NewRequest("POST", "/v1/run", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Execute(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	// Verify response fields.
	if resp["response"] == nil || resp["response"] == "" {
		t.Error("expected 'response' field in response")
	}
	if resp["agent"] == nil || resp["agent"] == "" {
		t.Error("expected 'agent' field in response")
	}
	if resp["duration_ms"] == nil {
		t.Error("expected 'duration_ms' field in response")
	}
	if resp["memory_id"] == nil || resp["memory_id"] == "" {
		t.Error("expected 'memory_id' field in response")
	}
}

// TestExecuteProviderOverrideDoesNotLeak verifies that a request-level
// provider override is applied to a fresh per-request registry and never
// mutates the global singleton.
func TestExecuteProviderOverrideDoesNotLeak(t *testing.T) {
	chat.ResetRegistry()
	orchestration.ResetExecutionStore()

	// The global registry defaults to "global-default".
	chat.GetRegistry().Register("global-default", namedMockProviderFactory("global-default", "global-model"), "Default provider", 1)
	_ = chat.GetRegistry().Select(nil, chat.ChatRegistryConfig{
		Primary:    "global-default",
		AutoDetect: false,
	})

	h := handler.NewRunHandler(nil, nil, nil)
	// The per-request registry only knows "request-provider".
	h.SetRegistryFactory(func() *chat.ChatRegistry {
		reg := chat.NewChatRegistry()
		reg.Register("request-provider", namedMockProviderFactory("request-provider", "request-model"), "Request provider", 1)
		return reg
	})

	// First request explicitly overrides the provider.
	body := `{"prompt":"Hello","provider":"request-provider"}`
	req := httptest.NewRequest("POST", "/v1/run", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Execute(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	// The global singleton must be untouched by the request-level override.
	if name := chat.GetRegistry().Name(); name != "global-default" {
		t.Errorf("global registry leaked provider override: Name() = %q, want %q", name, "global-default")
	}
	if model := chat.GetRegistry().Model(); model != "global-model" {
		t.Errorf("global registry leaked provider override: Model() = %q, want %q", model, "global-model")
	}

	// A subsequent request without an override must still use the global default.
	body2 := `{"prompt":"Hello again"}`
	req2 := httptest.NewRequest("POST", "/v1/run", strings.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	h.Execute(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for default provider, got %d: %s", w2.Code, w2.Body.String())
	}
}

// TestExecuteRegistryFactoryError verifies that Execute returns 503 when
// resolveRegistry fails. Note: Select can never fail for a factory-registered
// registry because ollama's factory always succeeds and RegisterChatProviders
// runs before Select, so the deterministic failure path is a nil registry,
// which makes RegisterChatProviders return an error.
func TestExecuteRegistryFactoryError(t *testing.T) {
	chat.ResetRegistry()
	orchestration.ResetExecutionStore()

	h := handler.NewRunHandler(nil, nil, nil)
	// A nil registry makes chatprovider.RegisterChatProviders fail, so
	// resolveRegistry returns an error and the handler answers 503.
	h.SetRegistryFactory(func() *chat.ChatRegistry {
		return nil
	})

	body := `{"prompt":"Hello","provider":"unknown-provider"}`
	req := httptest.NewRequest("POST", "/v1/run", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Execute(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 Service Unavailable, got %d: %s", w.Code, w.Body.String())
	}
}

// TestSetRegistryFactoryIgnoresNil verifies that passing nil to
// SetRegistryFactory is a no-op and keeps the previously set factory.
func TestSetRegistryFactoryIgnoresNil(t *testing.T) {
	chat.ResetRegistry()
	orchestration.ResetExecutionStore()

	h := handler.NewRunHandler(nil, nil, nil)
	h.SetRegistryFactory(func() *chat.ChatRegistry {
		reg := chat.NewChatRegistry()
		reg.Register("factory-mock", namedMockProviderFactory("factory-mock", "factory-model"), "Factory provider", 1)
		return reg
	})
	// Setting nil must keep the factory above.
	h.SetRegistryFactory(nil)

	body := `{"prompt":"Hello","provider":"factory-mock"}`
	req := httptest.NewRequest("POST", "/v1/run", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Execute(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}
}

// ── Mock ChatProvider ────────────────────────────────────────────────────────

// mockChatProvider implements chat.ChatProvider for testing.
type mockChatProvider struct{}

func (m *mockChatProvider) Name() string       { return "test-mock" }
func (m *mockChatProvider) Model() string      { return "mock-model-v1" }
func (m *mockChatProvider) HealthCheck() error { return nil }
func (m *mockChatProvider) Close() error       { return nil }

func (m *mockChatProvider) Chat(ctx context.Context, messages []chat.Message, opts chat.ChatOptions) (*chat.ChatResponse, error) {
	return &chat.ChatResponse{
		Model: "mock-model-v1",
		Choices: []chat.Choice{
			{
				Index: 0,
				Message: chat.Message{
					Role:    chat.RoleAssistant,
					Content: "Go is a statically typed, compiled programming language designed at Google.",
				},
			},
		},
		Usage: chat.Usage{
			PromptTokens:     10,
			CompletionTokens: 15,
			TotalTokens:      25,
		},
	}, nil
}

func (m *mockChatProvider) ChatStream(ctx context.Context, messages []chat.Message, opts chat.ChatOptions) (chat.ChatStream, error) {
	return nil, nil
}

// mockProviderFactory creates a mock ChatProvider instance.
func mockProviderFactory(ctx context.Context, cfg map[string]interface{}) (chat.ChatProvider, error) {
	return &mockChatProvider{}, nil
}

// namedMockProvider is a chat.ChatProvider with a configurable name and model,
// used to distinguish providers in registry-leak tests.
type namedMockProvider struct {
	name  string
	model string
}

func (m *namedMockProvider) Name() string       { return m.name }
func (m *namedMockProvider) Model() string      { return m.model }
func (m *namedMockProvider) HealthCheck() error { return nil }
func (m *namedMockProvider) Close() error       { return nil }

func (m *namedMockProvider) Chat(ctx context.Context, messages []chat.Message, opts chat.ChatOptions) (*chat.ChatResponse, error) {
	return &chat.ChatResponse{
		Model: m.model,
		Choices: []chat.Choice{
			{Index: 0, Message: chat.Message{Role: chat.RoleAssistant, Content: "response from " + m.name}},
		},
		Usage: chat.Usage{TotalTokens: 1},
	}, nil
}

func (m *namedMockProvider) ChatStream(ctx context.Context, messages []chat.Message, opts chat.ChatOptions) (chat.ChatStream, error) {
	return nil, nil
}

// namedMockProviderFactory returns a factory producing a namedMockProvider.
func namedMockProviderFactory(name, model string) chat.ChatProviderFactory {
	return func(_ context.Context, _ map[string]interface{}) (chat.ChatProvider, error) {
		return &namedMockProvider{name: name, model: model}, nil
	}
}
