package mcpserver

// replay_test.go — PROTEÇÃO DE REPLAY do MCP (correção do gasto duplicado).
//
// O OpenCode (ou um retry do cliente) pode reenviar a MESMA tools/call quando o
// stream trava ou o usuário manda continuar. Sem proteção, cada reenvio
// RE-EXECUTA a tool — LLM/tokens duplicados e efeitos repetidos. O replayStore
// do Server detecta a mesma chamada (hash de name+arguments) e devolve o
// resultado em cache.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
)

// countingEngine é um engine com uma tool contadora: cada EXECUÇÃO incrementa
// o contador. Se o replay for servido do cache, o contador NÃO sobe no reenvio.
type countingEngine struct {
	*Engine
	calls *int64
}

// newCountingEngine monta um engine com uma tool `count` que incrementa um
// contador atômico e devolve o valor.
func newCountingEngine() (*countingEngine, *int64) {
	var calls int64
	ce := &countingEngine{Engine: NewEngine(), calls: &calls}
	ce.registry = newRegistry()
	ce.registry.register(ToolDef{
		Name:        "count",
		Domain:      "test",
		Organ:       "test",
		Risk:        RiskRead,
		Permission:  PermissionPublic,
		Description: "contador de execuções (teste de replay)",
		InputSchema: `{"type":"object","properties":{}}`,
		Handler: func(_ context.Context, _ json.RawMessage) (*CallResult, error) {
			n := atomic.AddInt64(&calls, 1)
			return &CallResult{
				Content: []ContentItem{{Type: "text", Text: fmt.Sprintf(`{"count":%d}`, n)}},
			}, nil
		},
	})
	return ce, &calls
}

// TestToolCall_ReplayNaoReexecuta é o teste central da proteção de replay:
// a MESMA chamada reenviada (mesmo name+arguments) NÃO re-executa a tool.
func TestToolCall_ReplayNaoReexecuta(t *testing.T) {
	engine, calls := newCountingEngine()
	in := bytes.NewBufferString(strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"count","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"count","arguments":{}}}`,
	}, "\n"))
	out := &bytes.Buffer{}
	srv := NewServer(engine.Engine, in, out)
	if err := srv.Serve(context.Background()); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	// A tool foi executada 1x (o reenvio veio do cache de replay).
	if got := atomic.LoadInt64(calls); got != 1 {
		t.Fatalf("tool executada %d vezes, esperava 1 (replay deve ser servido do cache)", got)
	}

	// As duas respostas são idênticas (mesmo resultado em cache).
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("esperava 2 respostas, got %d", len(lines))
	}
	for i, line := range lines {
		var resp map[string]json.RawMessage
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			t.Fatalf("resp %d: unmarshal: %v", i, err)
		}
		if resp["error"] != nil {
			t.Fatalf("resp %d: erro inesperado: %s", i, resp["error"])
		}
		var result toolCallResult
		if err := json.Unmarshal(resp["result"], &result); err != nil {
			t.Fatalf("resp %d: unmarshal result: %v", i, err)
		}
		if len(result.Content) == 0 || !strings.Contains(result.Content[0].Text, `"count":1`) {
			t.Fatalf("resp %d: conteúdo inesperado: %s", i, result.Content[0].Text)
		}
	}
}

// TestToolCall_ReplayIgnoraChamadasDiferentes: argumentos diferentes geram
// chave diferente — a tool executa de novo (intenção nova, não replay).
func TestToolCall_ReplayIgnoraChamadasDiferentes(t *testing.T) {
	engine, calls := newCountingEngine()
	in := bytes.NewBufferString(strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"count","arguments":{"a":1}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"count","arguments":{"a":2}}}`,
	}, "\n"))
	out := &bytes.Buffer{}
	srv := NewServer(engine.Engine, in, out)
	if err := srv.Serve(context.Background()); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	// Argumentos diferentes = intenções diferentes = 2 execuções legítimas.
	if got := atomic.LoadInt64(calls); got != 2 {
		t.Fatalf("tool executada %d vezes, esperava 2 (chamadas diferentes não são replay)", got)
	}
}

// TestReplayKey_Deterministic: a mesma chamada gera a mesma chave; chamadas
// diferentes geram chaves diferentes.
func TestReplayKey_Deterministic(t *testing.T) {
	args := json.RawMessage(`{"query":"x","limit":5}`)
	if replayKey("cosca.recall", args) != replayKey("cosca.recall", args) {
		t.Fatal("mesma chamada deveria gerar a mesma chave de replay")
	}
	if replayKey("cosca.recall", args) == replayKey("cosca.recall", json.RawMessage(`{"query":"y"}`)) {
		t.Fatal("chamadas diferentes deveriam gerar chaves diferentes")
	}
	if replayKey("cosca.recall", args) == replayKey("cosca.learn", args) {
		t.Fatal("tools diferentes deveriam gerar chaves diferentes")
	}
}
