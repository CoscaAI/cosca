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
	"github.com/CoscaAI/cosca/internal/vision"
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
	eng := buildTestEngine(WithKernel(kernel.NewEmergencyManager()))
	tools := eng.Tools()
	if len(tools) != 11 {
		t.Fatalf("esperava 11 tools, veio %d (%v)", len(tools), toolNames(tools))
	}

	want := []string{ToolRecall, ToolContext, ToolLearn, ToolObserve, ToolReason, ToolTrace, ToolProject, ToolCost, ToolCLI, ToolSelf, ToolWeb}
	got := toolNames(tools)
	for i, w := range want {
		if i >= len(got) || got[i] != w {
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

// ─── Teste: tools de leitura gravam evento no flight recorder ─────────────

// TestReadTools_RecordTraceEvent (padrão AAA, table-driven) verifica que, ao
// chamar uma tool de leitura que gera context packet, o evento do Trace ID é
// GRAVADO no flight recorder (e.Trace.Append) com Action semântica e Actor
// "mcp". A auditoria da reação do cérebro passa a ser rastreável via cosca.trace.
func TestReadTools_RecordTraceEvent(t *testing.T) {
	tests := []struct {
		name           string
		tool           string
		args           string
		expectedAction string
	}{
		{name: "recall", tool: ToolRecall, args: `{"query":"provenance","limit":5}`, expectedAction: "RECALL"},
		{name: "context", tool: ToolContext, args: `{"query":"provenance","limit":5}`, expectedAction: "CONTEXT"},
		{name: "reason", tool: ToolReason, args: `{"sequence":["A","B","C"]}`, expectedAction: "REASON"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange — engine real (knowledge FTS) + trace store injetado.
			ts := mustTraceStore(t)
			eng := NewEngine(
				WithKnowledge(newTestKnowledgeEngine(t)),
				WithTrace(ts),
			)

			// Act — chama a tool de leitura.
			res, err := eng.Call(context.Background(), tc.tool, json.RawMessage(tc.args))
			if err != nil {
				t.Fatalf("Call %s: %v", tc.tool, err)
			}
			if res.IsError {
				t.Fatalf("%s retornou isError: %v", tc.tool, res.Content)
			}
			packet := decodePacket(t, res)

			// Assert — o mesmo TraceID do packet GRAVA 1+ evento no ledger.
			events, gErr := ts.Get(packet.TraceID)
			if gErr != nil {
				t.Fatalf("Trace.Get(%s): %v", packet.TraceID, gErr)
			}
			if len(events) == 0 {
				t.Fatalf("esperava >=1 evento no flight recorder para %s, veio 0", packet.TraceID)
			}
			var found bool
			for _, ev := range events {
				if ev.TraceID == packet.TraceID && ev.Action == tc.expectedAction && ev.Actor == "mcp" {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("esperava evento Action=%q Actor=mcp para %s, eventos=%+v",
					tc.expectedAction, packet.TraceID, events)
			}
		})
	}
}

// ─── Teste: e.Trace == nil — gravação é best-effort (não panica) ──────────

// TestReadTools_NilTraceIsBestEffort verifica que, sem trace store injetado, a
// tool de leitura NÃO panica e NÃO quebra a resposta — apenas devolve o packet
// normalmente (o trace é auditoria, nunca pré-requisito).
func TestReadTools_NilTraceIsBestEffort(t *testing.T) {
	// Arrange — engine sem trace store (e.Trace == nil).
	eng := NewEngine(WithKnowledge(newTestKnowledgeEngine(t)))

	// Act — chama a tool de leitura.
	res, err := eng.Call(context.Background(), ToolRecall, json.RawMessage(`{"query":"provenance","limit":5}`))
	if err != nil {
		t.Fatalf("Call recall: %v", err)
	}

	// Assert — resposta íntegra, com TraceID, apesar de e.Trace ser nil.
	if res.IsError {
		t.Fatalf("recall retornou isError: %v", res.Content)
	}
	packet := decodePacket(t, res)
	if packet.TraceID == "" {
		t.Fatal("packet deveria ter TraceID mesmo sem trace store")
	}
	if _, ok := trace.Parse(packet.TraceID); !ok {
		t.Fatalf("trace_id inválido: %q", packet.TraceID)
	}
}

// ─── Teste: cosca.observe grava evento OBSERVE no flight recorder ────────

// TestObserve_RecordTraceEvent (AAA) verifica que, ao chamar a tool de
// percepção, o evento do Trace ID do packet é GRAVADO no flight recorder
// (e.Trace.Append) com Action "OBSERVE" e Actor "mcp" — a reação perceptiva
// do cérebro passa a ser auditável via cosca.trace (paridade com
// recall/context/reason/project).
func TestObserve_RecordTraceEvent(t *testing.T) {
	// Arrange — vision func determinística + trace store injetado.
	eng := NewEngine(
		WithVision(testObserveVision),
		WithTrace(mustTraceStore(t)),
	)

	// Act — chama a tool de percepção.
	res, err := eng.Call(context.Background(), ToolObserve, json.RawMessage(`{"video":"clip.mp4"}`))
	if err != nil {
		t.Fatalf("Call observe: %v", err)
	}
	if res.IsError {
		t.Fatalf("observe retornou isError: %v", res.Content)
	}
	packet := decodePacket(t, res)
	if len(packet.Context) == 0 {
		t.Fatal("esperava >=1 item de contexto de percepção")
	}

	// Assert — o mesmo TraceID do packet GRAVA 1+ evento OBSERVE no ledger.
	events, gErr := eng.Trace.Get(packet.TraceID)
	if gErr != nil {
		t.Fatalf("Trace.Get(%s): %v", packet.TraceID, gErr)
	}
	if len(events) == 0 {
		t.Fatalf("esperava >=1 evento no flight recorder para %s, veio 0", packet.TraceID)
	}
	var found bool
	for _, ev := range events {
		if ev.TraceID == packet.TraceID && ev.Action == "OBSERVE" && ev.Actor == "mcp" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("esperava evento Action=OBSERVE Actor=mcp para %s, eventos=%+v", packet.TraceID, events)
	}
}

// ─── Teste: cosca.trace — trace_id LIDO, não um novo (caso vazio) ─────────

// TestTrace_EmptyReturnsSameTraceID (AAA) verifica a SEMÂNTICA do callTrace:
// a tool lê um trace (args.TraceID) e devolve um packet que carrega o MESMO
// trace_id lido — NÃO um recém-gerado. No caso sem eventos (trace store
// vazio), o packet é honesto (context vazio, confidence 0), mas o TraceID é o
// que foi consultado (evita auto-referência/loop: o trace não registra a si
// mesmo).
func TestTrace_EmptyReturnsSameTraceID(t *testing.T) {
	const traceID = "TRACE-20260829-ABC12345"

	// Arrange — trace store Vazio (sem eventos para o id) injetado.
	eng := NewEngine(WithTrace(mustTraceStore(t)))

	// Act — lê um trace id que não tem eventos.
	res, err := eng.Call(context.Background(), ToolTrace, json.RawMessage(`{"trace_id":"`+traceID+`"}`))
	if err != nil {
		t.Fatalf("Call trace: %v", err)
	}
	if res.IsError {
		t.Fatalf("trace retornou isError: %v", res.Content)
	}
	packet := decodePacket(t, res)

	// Assert — o packet carrega o MESMO trace_id lido (não um novo).
	if packet.TraceID != traceID {
		t.Fatalf("packet.TraceID = %q, esperava o MESMO trace lido %q (NÃO um novo)", packet.TraceID, traceID)
	}
	if len(packet.Context) != 0 {
		t.Fatalf("esperava context vazio para trace sem eventos, veio %d", len(packet.Context))
	}
	if packet.Confidence != 0 {
		t.Fatalf("esperava confidence 0 (honesto) para trace sem eventos, veio %v", packet.Confidence)
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

// testObserveVision é uma VisionFunc determinística para o teste de observe:
// devolve um PipelineResult com 1 evento de percepção (OBSERVED, corroborável),
// suficiente para gerar um item de contexto e um trace OBSERVE auditável.
func testObserveVision(ctx context.Context, video string, opts vision.PipelineOptions) (*vision.PipelineResult, error) {
	return &vision.PipelineResult{
		Frames: []vision.FrameRecord{{Frame: 5, Time: 0.5}},
		Events: []vision.PerceptEvent{
			{
				Kind:        vision.EventCurrencyChanged,
				Frame:       5,
				Explanation: "moeda detectada no balcão",
				Observation: vision.Observation{
					Epistemic:  vision.EpistemicObserved,
					Object:     "currency_counter",
					Value:      3,
					Confidence: 0.9,
				},
			},
		},
	}, nil
}

// mustTraceStore cria um trace store SQLite temporário (flight recorder) para
// os testes de auditoria das tools de leitura.
func mustTraceStore(t *testing.T) *trace.Store {
	t.Helper()
	s, err := trace.NewStore(filepath.Join(t.TempDir(), "trace.db"))
	if err != nil {
		t.Fatalf("trace.NewStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
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


