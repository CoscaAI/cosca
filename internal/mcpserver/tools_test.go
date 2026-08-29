// tools_test.go — Testes das TOOLS COGNITIVAS do servidor MCP do COSCA.
//
// Cobre: inventário (7 tools), recall (packet bem-formado), tool desconhecida
// (default-deny), learn read-only (fail-closed), learn com write gate, e os
// helpers do context packet (epistemologia + confidence determinístico).
package mcpserver

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CoscaAI/cosca/internal/chat/mcp"
	"github.com/CoscaAI/cosca/internal/kernel"
	"github.com/CoscaAI/cosca/internal/knowledge"
	"github.com/CoscaAI/cosca/internal/memory"
	"github.com/CoscaAI/cosca/internal/trace"
)

// ─── Helpers ───────────────────────────────────────────────────────────────

// newTestKnowledgeEngine cria um knowledge.Engine real (FTS-only) em um diretório
// temporário, indexando documentos com termos ASCII para busca determinística.
// O chamador deve defer ke.Close() (ou usar t.Cleanup).
func newTestKnowledgeEngine(t *testing.T) *knowledge.Engine {
	t.Helper()
	dir := t.TempDir()

	ke, err := knowledge.New(knowledge.Config{
		DBPath:      filepath.Join(dir, "knowledge.db"),
		RootDir:     dir,
		AutoMigrate: true,
	})
	if err != nil {
		t.Fatalf("knowledge.New: %v", err)
	}
	if err := ke.Init(); err != nil {
		t.Fatalf("knowledge.Init: %v", err)
	}
	t.Cleanup(func() { _ = ke.Close() })

	docs := []struct{ name, content string }{
		{"prov.md", "# Provenance\n\nProvenance P0-P5 identifies where evidence came from: repository, commit, sha256."},
		{"infer.md", "# Inference\n\nInference (INFERRED) is never presented as fact."},
	}
	for _, d := range docs {
		p := filepath.Join(dir, d.name)
		if err := os.WriteFile(p, []byte(d.content), 0o644); err != nil {
			t.Fatalf("write %s: %v", d.name, err)
		}
		if err := ke.IndexDocument(context.Background(), p); err != nil {
			t.Fatalf("index %s: %v", d.name, err)
		}
	}
	return ke
}

// buildTestEngine monta um Engine com knowledge usualmente vazio (para os
// testes de protocolo). Opções extras permitem injetar outros órgãos.
func buildTestEngine(opts ...Option) *Engine {
	return NewEngine(append([]Option{
		WithKernel(kernel.NewEmergencyManager()),
	}, opts...)...)
}

// ─── Teste: inventário de 7 tools ─────────────────────────────────────────

func TestToolsList_HasSevenCognitiveTools(t *testing.T) {
	tools := NewEngine().Tools()
	if len(tools) != 7 {
		t.Fatalf("esperava 7 tools, veio %d (%v)", len(tools), toolNames(tools))
	}

	want := []string{ToolRecall, ToolContext, ToolLearn, ToolObserve, ToolReason, ToolTrace, ToolProject}
	got := toolNames(tools)
	for i, w := range want {
		if got[i] != w {
			t.Fatalf("tools[%d] = %q, esperava %q", i, got[i], w)
		}
	}
}

func toolNames(tools []mcp.ToolInfo) []string {
	out := make([]string, 0, len(tools))
	for _, t := range tools {
		out = append(out, t.Name)
	}
	return out
}

// ─── Teste: cosca.recall devolve packet bem-formado ───────────────────────

func TestRecall_ReturnsWellFormedPacket(t *testing.T) {
	eng := NewEngine(WithKnowledge(newTestKnowledgeEngine(t)))
	res, err := eng.Call(context.Background(), ToolRecall, json.RawMessage(`{"query":"provenance","limit":5}`))
	if err != nil {
		t.Fatalf("Call recall: %v", err)
	}
	if res.IsError {
		t.Fatalf("recall retornou isError: %v", res.Content)
	}

	packet := decodePacket(t, res)
	if packet.Query != "provenance" {
		t.Fatalf("packet.query = %q, esperava %q", packet.Query, "provenance")
	}
	if len(packet.Context) == 0 {
		t.Fatal("esperava >=1 item de contexto para query 'provenance'")
	}
	for _, it := range packet.Context {
		if !validEpistemicSource(it.Source) {
			t.Fatalf("source inválido no packet: %q", it.Source)
		}
		if it.Relevance < 0 || it.Relevance > 1 {
			t.Fatalf("relevance fora de [0,1]: %v", it.Relevance)
		}
		if it.Content == "" {
			t.Fatal("item de contexto sem content")
		}
	}
	if packet.Confidence <= 0 || packet.Confidence > 1 {
		t.Fatalf("confidence fora de (0,1]: %v", packet.Confidence)
	}
	if _, ok := trace.Parse(packet.TraceID); !ok {
		t.Fatalf("trace_id inválido: %q", packet.TraceID)
	}
}

// ─── Teste: tool inexistente → erro (default-deny, I8) ────────────────────

func TestCall_UnknownToolError(t *testing.T) {
	eng := NewEngine()
	_, err := eng.Call(context.Background(), "cosca.nope", nil)
	if err == nil {
		t.Fatal("esperava erro para tool desconhecida")
	}
	if !strings.Contains(err.Error(), "desconhecida") && !strings.Contains(err.Error(), "default-deny") {
		t.Fatalf("mensagem de erro inesperada: %v", err)
	}
}

// ─── Teste: cosca.learn é read-only por padrão (fail-closed) ─────────────

func TestLearn_ReadOnlyFailsClosed(t *testing.T) {
	eng := NewEngine(WithMemory(mustMemoryEngine(t)))
	res, err := eng.Call(context.Background(), ToolLearn, json.RawMessage(`{"content":"aprendizado teste"}`))
	if err != nil {
		t.Fatalf("Call learn: %v", err)
	}
	if !res.IsError {
		t.Fatal("learn sem write gate deveria retornar isError (read-only)")
	}
	if !strings.Contains(res.Content[0].Text, "read-only") {
		t.Fatalf("mensagem read-only esperada, veio: %v", res.Content[0].Text)
	}
}

// ─── Teste: cosca.learn grava quando write gate ativo ─────────────────────

func TestLearn_WriteWhenAllowed(t *testing.T) {
	eng := NewEngine(WithMemory(mustMemoryEngine(t)), WithAllowWrite(true))
	res, err := eng.Call(context.Background(), ToolLearn, json.RawMessage(`{"content":"aprendizado autorizado","type":"pattern","layer":"workspace"}`))
	if err != nil {
		t.Fatalf("Call learn: %v", err)
	}
	if res.IsError {
		t.Fatalf("learn com gate ativo deveria gravar, veio isError: %v", res.Content)
	}
	packet := decodePacket(t, res)
	if len(packet.Context) == 0 {
		t.Fatal("esperava item de confirmação no packet")
	}
}

// ─── Teste: nil-safe — sem órgão, tool devolve erro claro ─────────────────

func TestCall_NilSafeNoKnowledge(t *testing.T) {
	eng := NewEngine() // sem knowledge
	_, err := eng.Call(context.Background(), ToolRecall, json.RawMessage(`{"query":"x"}`))
	if err == nil {
		t.Fatal("esperava erro para recall sem knowledge engine (nil-safe)")
	}
	if !strings.Contains(err.Error(), "indisponível") {
		t.Fatalf("mensagem inesperada: %v", err)
	}
}

// ─── Helpers internos ─────────────────────────────────────────────────────

func mustMemoryEngine(t *testing.T) *memory.MemoryEngine {
	t.Helper()
	me, err := memory.NewEngine(memory.WithConfig(memory.EngineConfig{DataDir: t.TempDir(), AutoPrune: false}))
	if err != nil {
		t.Fatalf("memory.NewEngine: %v", err)
	}
	t.Cleanup(func() { _ = me.Close() })
	return me
}

// decodePacket extrai o ContextPacket do resultado de uma tool call.
func decodePacket(t *testing.T, res *CallResult) *ContextPacket {
	t.Helper()
	if res == nil || len(res.Content) == 0 {
		t.Fatal("resultado sem conteúdo")
	}
	var packet ContextPacket
	if err := json.Unmarshal([]byte(res.Content[0].Text), &packet); err != nil {
		t.Fatalf("decodificar packet: %v (texto=%q)", err, res.Content[0].Text)
	}
	return &packet
}

// validEpistemicSource verifica se o source pertence às 7 classes válidas.
func validEpistemicSource(s string) bool {
	return knowledge.KnowledgeEpistemic(s).Valid()
}
