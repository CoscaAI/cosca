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

// TestProviderAdapter_Chat_PreservesToolCalls é o teste de fogo do BUG A no
// nível do provider+adapter: garante que tool_calls devolvidos pelo LLM (formato
// OpenAI) SOBREVIVEM ao round-trip provider→adapter e chegam ao ChatResponse.
// Antes do fix, o adapter reconstruía apenas Content e Usage e DESCARTAVA as
// tool_calls — por isso o executor sempre via toolCalls==nil e nunca executava a
// tool (o agente só "conversava", não escrevia arquivos).
func TestProviderAdapter_Chat_PreservesToolCalls(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		// Retorna uma resposta OpenAI não-streaming com tool_calls de write_file.
		resp := `{
			"id": "chatcmpl-123",
			"object": "chat.completion",
			"created": 1234567890,
			"model": "gpt-4o",
			"choices": [{
				"index": 0,
				"message": {
					"role": "assistant",
					"content": "",
					"tool_calls": [{
						"id": "call_1",
						"type": "function",
						"function": {"name": "write_file", "arguments": "{\"path\":\"teste.txt\",\"content\":\"ok\"}"}
					}]
				},
				"finish_reason": "tool_calls"
			}],
			"usage": {"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15}
		}`
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, resp)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	adapter := provider.NewProviderAdapter(
		provider.NewOpenAI("sk-test-openai", "gpt-4o", server.URL+"/v1"),
		"gpt-4o",
	)

	resp, err := adapter.Chat(context.Background(), []chat.Message{
		{Role: chat.RoleUser, Content: "crie teste.txt usando write_file"},
	}, chat.ChatOptions{})
	if err != nil {
		t.Fatalf("adapter.Chat() failed: %v", err)
	}

	if len(resp.Choices) == 0 {
		t.Fatal("expect at least one choice")
	}
	toolCalls := resp.Choices[0].Message.ToolCalls
	if len(toolCalls) != 1 {
		t.Fatalf("expected 1 tool call to survive the round-trip, got %d", len(toolCalls))
	}
	if toolCalls[0].Function.Name != "write_file" {
		t.Errorf("tool name = %q, want %q", toolCalls[0].Function.Name, "write_file")
	}
	if resp.Choices[0].FinishReason != chat.FinishReasonToolCalls {
		t.Errorf("finish reason = %q, want %q", resp.Choices[0].FinishReason, chat.FinishReasonToolCalls)
	}
}
