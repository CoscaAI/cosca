// Package executor — integração do guard determinístico (L366): o policy
// bloqueia ações destrutivas ANTES da execução, independente do LLM.
package executor

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/CoscaAI/cosca/internal/chat"
	"github.com/CoscaAI/cosca/internal/policy"
)

func TestPolicyBlocksDestructive(t *testing.T) {
	mt := &mockTool{name: "bash", executeFunc: func(ctx context.Context, params json.RawMessage) (*chat.ToolResult, error) {
		return &chat.ToolResult{Output: "executado"}, nil
	}}
	reg := newMockRegistry()
	reg.tools["bash"] = mt
	ex := New(reg, nil, "/tmp")
	ex.SetPolicy(policy.New())

	// Destrutivo → DENY antes da execução.
	res, _ := ex.Execute(context.Background(), ToolCall{ID: "1", Name: "bash", Input: map[string]interface{}{"command": "rm -rf /tmp/x"}})
	if res.Status != StatusError {
		t.Fatalf("destrutivo: status=%s, esperado ERROR (policy DENY)", res.Status)
	}
	if res.Error == "" || res.Error == "executado" {
		t.Fatalf("destrutivo: erro=%q, esperado mensagem de policy", res.Error)
	}

	// pkill → DENY (L56).
	res2, _ := ex.Execute(context.Background(), ToolCall{ID: "2", Name: "bash", Input: map[string]interface{}{"command": "pkill cosca"}})
	if res2.Status != StatusError {
		t.Fatalf("pkill: status=%s, esperado ERROR (L56)", res2.Status)
	}

	// Leitura → permitida.
	res3, _ := ex.Execute(context.Background(), ToolCall{ID: "3", Name: "bash", Input: map[string]interface{}{"command": "ls -la"}})
	if res3.Status == StatusError && res3.Error != "" {
		t.Logf("ls: erro inesperado=%s", res3.Error)
	}
}
