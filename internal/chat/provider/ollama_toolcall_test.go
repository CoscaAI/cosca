package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/CoscaAI/cosca/internal/chat"
)

// TestToChatToolCalls_OllamaObjectArguments verifies the decode fix: Ollama's
// native /api/chat returns function.arguments as a JSON *object*, but the
// shared chat.FunctionCall.Arguments is a *string*. toChatToolCalls normalises
// the object to its compact JSON string form so the Tool Executor can parse it.
func TestToChatToolCalls_OllamaObjectArguments(t *testing.T) {
	rawArgs := json.RawMessage(`{"path":"src/hello.go","content":"package main\n"}`)
	in := []ollamaRespToolCall{
		{
			ID:   "call_1",
			Type: "function",
			Function: ollamaRespFunction{
				Name:      "write_file",
				Arguments: rawArgs,
			},
		},
	}

	out := toChatToolCalls(in)
	if len(out) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(out))
	}
	if out[0].Function.Name != "write_file" {
		t.Errorf("name = %q, want write_file", out[0].Function.Name)
	}
	// Arguments must be a JSON *object string* (not a double-quoted string).
	got := out[0].Function.Arguments
	var obj map[string]any
	if err := json.Unmarshal([]byte(got), &obj); err != nil {
		t.Fatalf("arguments should decode as a JSON object, got %q: %v", got, err)
	}
	if obj["path"] != "src/hello.go" {
		t.Errorf("path = %v, want src/hello.go", obj["path"])
	}
}

// TestToOllamaReqToolCalls_StringToObject verifies the request-side fix: when we
// echo the assistant message (with tool_calls) back to Ollama, function.arguments
// must be a JSON *object*. Ollama rejects a JSON string for arguments (the 400
// "Value looks like object, but can't find closing '}' symbol").
func TestToOllamaReqToolCalls_StringToObject(t *testing.T) {
	in := []chat.ToolCall{
		{
			ID:   "call_1",
			Type: "function",
			Function: chat.FunctionCall{
				Name:      "write_file",
				Arguments: `{"path":"src/hello.go","content":"hello"}`,
			},
		},
	}

	out := toOllamaReqToolCalls(in)
	if len(out) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(out))
	}

	// Marshal the whole message and confirm arguments is an object, not a string.
	m := ollamaMessage{Role: "assistant", ToolCalls: out}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(b) != `{"role":"assistant","tool_calls":[{"id":"call_1","type":"function","function":{"name":"write_file","arguments":{"path":"src/hello.go","content":"hello"}}}]}` {
		t.Errorf("unexpected marshaled message:\n%s", string(b))
	}
}

// TestOllamaProvider_Adapter_RoundTrip is the end-to-end proof of the Ollama
// function-calling path: a fake /api/chat that returns arguments as an OBJECT is
// decoded by the provider, surfaced as tool_calls by the adapter, and then
// re-marshaled as an object when echoed back — a full round trip.
func TestOllamaProvider_Adapter_RoundTrip(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/chat", func(w http.ResponseWriter, r *http.Request) {
		// Parse the incoming request: on turn 1 echo a tool_call with object
		// arguments; on turn 2 (after the tool result) return final content.
		var req ollamaRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, `{"error":"%s"}`, err.Error())
			return
		}

		w.Header().Set("Content-Type", "application/json")
		hasToolResult := false
		for _, msg := range req.Messages {
			if msg.Role == "tool" {
				hasToolResult = true
				break
			}
		}
		if hasToolResult {
			// Final answer.
			fmt.Fprint(w, `{"model":"qwen3:8b","message":{"role":"assistant","content":"arquivo criado"},"done":true}`)
			return
		}
		// Turn 1: emit a tool_call with object arguments (Ollama native shape).
		fmt.Fprint(w, `{"model":"qwen3:8b","message":{"role":"assistant","content":"","tool_calls":[{"id":"call_1","type":"function","function":{"name":"write_file","arguments":{"path":"src/hello.go","content":"package main\n"}}}]},"done":true}`)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	provider := NewOllama("qwen3:8b", server.URL)
	adapter := NewProviderAdapter(provider, "qwen3:8b")

	resp, err := adapter.Chat(context.Background(), []chat.Message{
		{Role: chat.RoleUser, Content: "crie hello.go"},
	}, chat.ChatOptions{})
	if err != nil {
		t.Fatalf("adapter.Chat() failed: %v", err)
	}
	if len(resp.Choices) == 0 || len(resp.Choices[0].Message.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %+v", resp.Choices[0])
	}
	msg := resp.Choices[0].Message
	if msg.Role != chat.RoleAssistant {
		t.Errorf("assistant role = %q, want %q", msg.Role, chat.RoleAssistant)
	}
	tc := msg.ToolCalls[0]
	if tc.Function.Name != "write_file" {
		t.Errorf("name = %q, want write_file", tc.Function.Name)
	}
	var args map[string]any
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
		t.Fatalf("arguments should be a JSON object string: %v", err)
	}
	if args["path"] != "src/hello.go" {
		t.Errorf("path = %v, want src/hello.go", args["path"])
	}

	// Echo the tool calls back (the request-side fix) and confirm argument is an
	// object in the outgoing request.
	assistant := msg
	assistant.Role = chat.RoleAssistant
	outResp, err := adapter.Chat(context.Background(), []chat.Message{
		{Role: chat.RoleUser, Content: "crie hello.go"},
		assistant,
		{Role: chat.RoleTool, Content: "Successfully wrote 12 bytes to src/hello.go", ToolCallID: tc.ID},
	}, chat.ChatOptions{})
	if err != nil {
		t.Fatalf("second adapter.Chat() failed: %v", err)
	}
	if len(outResp.Choices) == 0 || outResp.Choices[0].Message.Content == "" {
		t.Fatalf("expected final content on second call, got %+v", outResp.Choices)
	}
}
