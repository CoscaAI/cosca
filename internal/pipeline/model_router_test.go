package pipeline

import (
	"context"
	"fmt"
	"testing"

	"github.com/CoscaAI/cosca/internal/chat"
)

// stubChatProvider is a minimal chat.ChatProvider for model routing tests.
type stubChatProvider struct {
	model string
}

func (s *stubChatProvider) Chat(ctx context.Context, messages []chat.Message, opts chat.ChatOptions) (*chat.ChatResponse, error) {
	return nil, nil
}

func (s *stubChatProvider) ChatStream(ctx context.Context, messages []chat.Message, opts chat.ChatOptions) (chat.ChatStream, error) {
	return nil, nil
}

func (s *stubChatProvider) Model() string { return s.model }
func (s *stubChatProvider) Name() string  { return "stub" }
func (s *stubChatProvider) Close() error  { return nil }

func newTestRegistry(providers map[string]string) *chat.ChatRegistry {
	r := chat.NewChatRegistry()
	for name, model := range providers {
		r.Register(name, func(_ context.Context, _ map[string]interface{}) (chat.ChatProvider, error) {
			return &stubChatProvider{model: model}, nil
		}, "test", 10)
	}
	return r
}

// newUnreadyRegistry registers providers whose factories always fail. This
// models an isolated client project (cosca project new) that has a .cosca
// config with registered providers but no LLM credentials/configuration: the
// provider is registered but Get() returns (nil, true).
func newUnreadyRegistry(names []string) *chat.ChatRegistry {
	r := chat.NewChatRegistry()
	for _, name := range names {
		r.Register(name, func(_ context.Context, _ map[string]interface{}) (chat.ChatProvider, error) {
			return nil, fmt.Errorf("%s not configured", name)
		}, "test", 10)
	}
	return r
}

func TestDefaultModelPreferences(t *testing.T) {
	p := DefaultModelPreferences()
	if p.FastModel != "gpt-4o-mini" || p.SmartModel != "gpt-4o" || p.LocalModel != "ollama" || p.CodeModel != "qwen2.5-coder" {
		t.Fatalf("defaults: %+v", p)
	}
}

func TestModelRouterFallbackWithoutRegistry(t *testing.T) {
	r := NewModelRouter(nil)
	choice := r.Select(&TaskNode{Description: "anything"}, nil)
	if choice.Provider != "unknown" || choice.Model != "gpt-4o-mini" {
		t.Fatalf("fallback: %+v", choice)
	}
	if choice.CostEst != 0.01 {
		t.Fatalf("fallback cost = %v", choice.CostEst)
	}
}

func TestModelRouterFallbackWhenRegistryHasNoReadyProvider(t *testing.T) {
	// Registry exists and has registered providers, but none can be
	// instantiated (factory fails). Get() returns (nil, true), so a naive
	// provider.Model() call would panic with a nil-pointer dereference. This
	// test asserts we fail over gracefully to fallback instead of crashing.
	reg := newUnreadyRegistry([]string{"openai", "anthropic", "deepseek"})
	r := NewModelRouter(reg)

	for _, desc := range []string{
		"implement the api handler", // code task
		"rotate the secret key",     // sensitive task
		"list the files",            // default/fast task
	} {
		choice := r.Select(&TaskNode{Description: desc}, nil)
		if choice == nil {
			t.Fatalf("task %q: expected non-nil choice", desc)
		}
		if choice.Provider != "unknown" || choice.Model != "gpt-4o-mini" {
			t.Fatalf("task %q fallback: %+v", desc, choice)
		}
		if choice.Reason != "no ready LLM provider registered, using default" {
			t.Fatalf("task %q fallback reason = %q", desc, choice.Reason)
		}
	}
}

func TestModelRouterReadyProviderSelection(t *testing.T) {
	// Registry with a ready provider returns a normal ModelChoice.
	reg := newTestRegistry(map[string]string{
		"deepseek": "deepseek-v4-flash",
		"ollama":   "qwen2.5-coder:14b",
	})
	r := NewModelRouter(reg)

	choice := r.Select(&TaskNode{Description: "implement the api handler"}, nil)
	if choice == nil {
		t.Fatal("expected non-nil choice")
	}
	if choice.Provider == "unknown" || choice.Model == "" {
		t.Fatalf("expected a real configured choice, got +%v", choice)
	}
}

func TestModelRouterSelectsPreferred(t *testing.T) {
	reg := newTestRegistry(map[string]string{
		"deepseek": "deepseek-v4-flash",
		"ollama":   "qwen2.5-coder:14b",
	})
	r := NewModelRouter(reg)

	// Code task → preferred CodeModel (qwen2.5-coder) via ollama.
	choice := r.Select(&TaskNode{Description: "implement the api handler"}, nil)
	if choice.Provider != "ollama" {
		t.Fatalf("code task provider = %q, want ollama", choice.Provider)
	}

	// Sensitive task → local model preferred (ollama).
	choice = r.Select(&TaskNode{Description: "rotate the secret key"}, nil)
	if choice.Provider != "ollama" {
		t.Fatalf("sensitive task provider = %q", choice.Provider)
	}

	// Simple task → fast model fallback (gpt-4o-mini not registered → deepseek).
	choice = r.Select(&TaskNode{Description: "list the files"}, nil)
	if choice.Reason == "" {
		t.Fatal("missing reason")
	}
}

func TestModelRouterHighComplexity(t *testing.T) {
	reg := newTestRegistry(map[string]string{
		"deepseek": "deepseek-v4-flash",
		"ollama":   "qwen2.5-coder:14b",
	})
	r := NewModelRouter(reg)

	long := make([]byte, 300)
	for i := range long {
		long[i] = 'a'
	}
	task := &TaskNode{
		Description: string(long),                 // +2
		DependsOn:   []string{"a", "b"},           // +2
		InputFiles:  []string{"1", "2", "3", "4"}, // +2
		Priority:    9,                            // +1
	}
	if c := r.assessComplexity(task); c != "high" {
		t.Fatalf("complexity = %q, want high", c)
	}

	// High complexity → smart model preferred (gpt-4o absent → deepseek via fallbacks).
	choice := r.Select(task, nil)
	if choice.Model == "" {
		t.Fatal("empty model")
	}
}

func TestModelRouterAssessComplexity(t *testing.T) {
	r := NewModelRouter(nil)
	if c := r.assessComplexity(nil); c != "low" {
		t.Fatalf("nil complexity = %q", c)
	}
	simple := &TaskNode{Description: "hi"}
	if c := r.assessComplexity(simple); c != "low" {
		t.Fatalf("simple = %q", c)
	}
	medium := &TaskNode{Description: "x", DependsOn: []string{"a", "b"}}
	if c := r.assessComplexity(medium); c != "medium" {
		t.Fatalf("medium = %q", c)
	}
}

func TestModelRouterTaskClassification(t *testing.T) {
	r := NewModelRouter(nil)
	if !r.isCodeTask(&TaskNode{Description: "write a function"}) {
		t.Fatal("code task not detected")
	}
	if r.isCodeTask(&TaskNode{Description: "hello there"}) {
		t.Fatal("non-code task falsely detected")
	}
	if r.isCodeTask(nil) {
		t.Fatal("nil task must not be code")
	}
	if !r.isSensitiveTask(&TaskNode{Description: "handle the password"}) {
		t.Fatal("sensitive task not detected")
	}
	if r.isSensitiveTask(&TaskNode{Description: "normal stuff"}) {
		t.Fatal("non-sensitive falsely detected")
	}
}

func TestEstimateCost(t *testing.T) {
	cases := []struct {
		model string
		want  float64
	}{
		{"gpt-4o-mini", 0.15},
		{"gpt-4o", 2.50},
		{"gpt-4-turbo", 10.00},
		{"claude-sonnet-4", 3.00},
		{"claude-haiku", 0.25},
		{"claude-opus", 15.00},
		{"deepseek-v4", 0.14},
		{"ollama", 0.0},
		{"gemini-pro", 0.50},
		{"unknown-model", 1.0},
	}
	for _, tc := range cases {
		if got := estimateCost(tc.model); got != tc.want {
			t.Errorf("estimateCost(%q) = %v, want %v", tc.model, got, tc.want)
		}
	}
}
