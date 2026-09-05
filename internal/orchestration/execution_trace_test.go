// Regression tests for the additive TraceID field on Execution
// (internal/orchestration/execution_store.go).
//
// The TraceID field must be strictly additive: when not set it serializes as
// empty (omitted from JSON, `omitempty`), so the existing Execution JSON
// contract is unchanged. When a trace ID is passed it is preserved.

package orchestration

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestExecution_TraceID_EmptyWhenNotSet é o teste de não-regressão: um
// Execution sem TraceID produz exatamente o mesmo JSON de antes (sem o campo
// trace_id). Populado apenas quando passado.
func TestExecution_TraceID_EmptyWhenNotSet(t *testing.T) {
	exec := &Execution{
		ID:         "exec-1",
		Prompt:     "p",
		Response:   "r",
		Agent:      "agent-x",
		Provider:   "ollama",
		Model:      "qwen3",
		Status:     "success",
		DurationMs: 12,
		SkillsUsed: []string{"s1"},
	}
	// TraceID fica vazio — não é populado por Store/StoreExecution.
	data, err := json.Marshal(exec)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(data), "trace_id") {
		t.Fatalf("Execution sem TraceID não deveria serializar trace_id: %s", data)
	}
	// Os campos pré-existentes continuam presentes.
	for _, want := range []string{`"id":"exec-1"`, `"prompt":"p"`, `"agent":"agent-x"`, `"duration_ms":12`} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("campo existente ausente do JSON (regressão): %s; falta %s", data, want)
		}
	}
}

// TestExecution_TraceID_SerializesWhenSet verifica que o campo é preservado
// quando um trace ID é passado explicitamente (opt-in), sem tocar nos demais.
func TestExecution_TraceID_SerializesWhenSet(t *testing.T) {
	exec := &Execution{
		ID:      "exec-2",
		Prompt:  "p",
		Agent:   "agent-x",
		Status:  "success",
		TraceID: "TRACE-20260802-7F92",
	}
	data, err := json.Marshal(exec)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(data), `"trace_id":"TRACE-20260802-7F92"`) {
		t.Fatalf("TraceID passado deveria serializar: %s", data)
	}
}

// TestExecutionStore_StoreDoesNotInjectTraceID garante que o comportamento do
// ExecutionStore permanece intocado: Store nunca preenche TraceID por conta
// própria — ele nasce vazio e só é populado por quem o passa explicitamente.
func TestExecutionStore_StoreDoesNotInjectTraceID(t *testing.T) {
	store := NewExecutionStore(10)
	id := store.StoreExecution("", "prompt", "response", "agent-x", "ollama", "qwen3", "success", 1, nil, "")
	if id == "" {
		t.Fatal("StoreExecution deveria atribuir um ID")
	}
	exec := store.Get(id)
	if exec == nil {
		t.Fatal("execução não encontrada")
	}
	if exec.TraceID != "" {
		t.Fatalf("Store não deveria injetar TraceID, got %q", exec.TraceID)
	}
}
