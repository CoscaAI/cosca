// Package provider_test provides tests for the Cosca Chat provider adapters.
// This file covers the ADR-031 Fase 1 cache-detail promotion end-to-end:
// DeepSeek flat prompt_cache_hit_tokens and OpenAI nested
// prompt_tokens_details.cached_tokens must surface on ChatEventDone.Usage.
package provider_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/CoscaAI/cosca/internal/chat"
	"github.com/CoscaAI/cosca/internal/chat/provider"
)

// collectDoneUsage drena o canal de eventos e devolve o Usage do evento Done.
// Falha o teste se nenhum Done chegar ou se houver evento de erro.
func collectDoneUsage(t *testing.T, ch <-chan chat.ChatEvent) *chat.Usage {
	t.Helper()
	var usage *chat.Usage
	for evt := range ch {
		switch evt.Type {
		case chat.ChatEventDone:
			usage = evt.Usage
		case chat.ChatEventError:
			if evt.Error != nil {
				t.Fatalf("unexpected error event: %v", evt.Error)
			}
		}
	}
	if usage == nil {
		t.Fatal("done event has nil Usage")
	}
	return usage
}

// TestDeepSeekProvider_Chat_UsageCacheHit verifica que o shape DeepSeek flat
// (prompt_cache_hit_tokens) chega decomposto no ChatEventDone.Usage — non-stream.
func TestDeepSeekProvider_Chat_UsageCacheHit(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer sk-test-deepseek" {
			t.Errorf("Authorization header = %q, want %q", r.Header.Get("Authorization"), "Bearer sk-test-deepseek")
		}
		// DeepSeek OpenAI-compat: usage com campos flat de cache no topo.
		resp := `{
			"id": "chatcmpl-ds-1",
			"object": "chat.completion",
			"created": 1234567890,
			"model": "deepseek-chat",
			"choices": [{
				"index": 0,
				"message": {"role": "assistant", "content": "Hello from DeepSeek cache"},
				"finish_reason": "stop"
			}],
			"usage": {
				"prompt_tokens": 2000,
				"completion_tokens": 300,
				"total_tokens": 2300,
				"prompt_cache_hit_tokens": 1500,
				"prompt_cache_miss_tokens": 500
			}
		}`
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, resp)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	p := provider.NewDeepSeekWithBaseURL("sk-test-deepseek", "deepseek-chat", server.URL+"/v1")
	ch, err := p.Chat(context.Background(), chat.ChatRequest{
		Model:    "deepseek-chat",
		Messages: []chat.Message{{Role: chat.RoleUser, Content: "Hi"}},
		Stream:   false,
	})
	if err != nil {
		t.Fatalf("Chat() failed: %v", err)
	}

	usage := collectDoneUsage(t, ch)
	if usage.CachedTokens != 1500 {
		t.Errorf("CachedTokens = %d, want 1500", usage.CachedTokens)
	}
	if usage.PromptTokens != 2000 {
		t.Errorf("PromptTokens = %d, want 2000", usage.PromptTokens)
	}
	cached, effective := usage.CachedAndEffective()
	if cached != 1500 || effective != 500 {
		t.Errorf("CachedAndEffective() = (%d,%d), want (1500,500)", cached, effective)
	}
}

// TestDeepSeekProvider_ChatStream_UsageCacheHit verifica a promoção no chunk
// final do streaming (SSE) com o shape DeepSeek flat.
func TestDeepSeekProvider_ChatStream_UsageCacheHit(t *testing.T) {
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
			`data: {"id":"4","object":"chat.completion.chunk","usage":{"prompt_tokens":2000,"completion_tokens":300,"total_tokens":2300,"prompt_cache_hit_tokens":1500,"prompt_cache_miss_tokens":500}}`,
			`data: [DONE]`,
		}
		for _, line := range lines {
			fmt.Fprintf(w, "%s\n", line)
		}
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	p := provider.NewDeepSeekWithBaseURL("sk-test-deepseek", "deepseek-chat", server.URL+"/v1")
	ch, err := p.Chat(context.Background(), chat.ChatRequest{
		Model:    "deepseek-chat",
		Messages: []chat.Message{{Role: chat.RoleUser, Content: "Hi"}},
		Stream:   true,
	})
	if err != nil {
		t.Fatalf("Chat() failed: %v", err)
	}

	usage := collectDoneUsage(t, ch)
	if usage.CachedTokens != 1500 {
		t.Errorf("CachedTokens = %d, want 1500", usage.CachedTokens)
	}
}

// TestOpenAIProvider_Chat_Usage_NestedDetails é a regressão sistêmica do OpenAI:
// o detalhe nested prompt_tokens_details.cached_tokens (que antes era descartado)
// agora é capturado tanto no non-stream quanto no stream.
func TestOpenAIProvider_Chat_Usage_NestedDetails(t *testing.T) {
	t.Parallel()

	t.Run("non-stream", func(t *testing.T) {
		t.Parallel()

		mux := http.NewServeMux()
		mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
			resp := `{
				"id": "chatcmpl-oa-1",
				"object": "chat.completion",
				"created": 1234567890,
				"model": "gpt-4o",
				"choices": [{
					"index": 0,
					"message": {"role": "assistant", "content": "Hello from OpenAI nested"},
					"finish_reason": "stop"
				}],
				"usage": {
					"prompt_tokens": 1000,
					"completion_tokens": 200,
					"total_tokens": 1200,
					"prompt_tokens_details": {"cached_tokens": 600},
					"completion_tokens_details": {"reasoning_tokens": 40}
				}
			}`
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, resp)
		})
		server := httptest.NewServer(mux)
		defer server.Close()

		p := provider.NewOpenAI("sk-test-openai", "gpt-4o", server.URL+"/v1")
		ch, err := p.Chat(context.Background(), chat.ChatRequest{
			Model:    "gpt-4o",
			Messages: []chat.Message{{Role: chat.RoleUser, Content: "Hi"}},
			Stream:   false,
		})
		if err != nil {
			t.Fatalf("Chat() failed: %v", err)
		}

		usage := collectDoneUsage(t, ch)
		if usage.CachedTokens != 600 {
			t.Errorf("CachedTokens = %d, want 600 (nested cached_tokens)", usage.CachedTokens)
		}
		if usage.ReasoningTokens != 40 {
			t.Errorf("ReasoningTokens = %d, want 40 (nested reasoning_tokens)", usage.ReasoningTokens)
		}
	})

	t.Run("stream", func(t *testing.T) {
		t.Parallel()

		mux := http.NewServeMux()
		mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)

			lines := []string{
				`data: {"id":"1","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}`,
				`data: {"id":"2","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":""},"finish_reason":"stop"}]}`,
				`data: {"id":"3","object":"chat.completion.chunk","usage":{"prompt_tokens":1000,"completion_tokens":200,"total_tokens":1200,"prompt_tokens_details":{"cached_tokens":600}}}`,
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
			Messages: []chat.Message{{Role: chat.RoleUser, Content: "Hi"}},
			Stream:   true,
		})
		if err != nil {
			t.Fatalf("Chat() failed: %v", err)
		}

		usage := collectDoneUsage(t, ch)
		if usage.CachedTokens != 600 {
			t.Errorf("CachedTokens = %d, want 600 (nested cached_tokens)", usage.CachedTokens)
		}
	})
}
