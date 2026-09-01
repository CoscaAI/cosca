package contextpipeline

import (
	"testing"

	"github.com/CoscaAI/cosca/internal/orchestration"
)

// TestBuildContext_CompilesAndLevels: com conhecimento suficiente, o pipeline
// decide L1 e compila o contexto com as seções canônicas.
func TestBuildContext_CompilesAndLevels(t *testing.T) {
	p := New(Config{MaxTokens: 1000})
	data := &orchestration.PipelineData{
		ResolvedAgent: "Backend API Specialist",
		AgentRole:     "backend",
		KnowledgeResults: &orchestration.KnowledgeSearchResults{
			Results: []orchestration.KnowledgeSearchResult{
				{ID: "a", Content: "server exposes 11 tools", Score: 0.92, Epistemic: "FACT"},
				{ID: "b", Content: "generic_mcp advertises 6 obsolete tools", Score: 0.61},
				{ID: "c", Content: "registry is the source of truth", Score: 0.8},
			},
		},
	}

	text, level := p.BuildContext("corrigir endpoint MCP", data)
	if text == "" {
		t.Fatal("contexto compilado vazio")
	}
	if level.Name != "L1" {
		t.Fatalf("nível = %s, esperava L1 (conhecimento suficiente, sem deliberação)", level.Name)
	}
	if level.Deterministic {
		t.Fatal("sem deliberação real, nunca é determinístico")
	}
	if p.Track.Compiles != 1 {
		t.Fatalf("compiles = %d, esperava 1", p.Track.Compiles)
	}
}

// TestBuildContext_NoKnowledge_Escalates: sem conhecimento/memória, o nível
// cai para L2 (o LLM precisa do contexto completo — não inventa).
func TestBuildContext_NoKnowledge_Escalates(t *testing.T) {
	p := New(Config{MaxTokens: 1000})
	data := &orchestration.PipelineData{ResolvedAgent: "a"}

	_, level := p.BuildContext("pergunta sem contexto", data)
	if level.Name != "L2" {
		t.Fatalf("nível = %s, esperava L2 (sem conhecimento, contexto completo)", level.Name)
	}
}

// TestBuildContext_NilPipeline_Safe: pipeline nil nunca quebra (comportamento
// atual preservado — o Executor não usa o compilador).
func TestBuildContext_NilPipeline_Safe(t *testing.T) {
	var p *Pipeline
	text, level := p.BuildContext("oi", &orchestration.PipelineData{})
	if text != "" || level.Name != "L1" {
		t.Fatalf("pipeline nil: text=%q level=%s, esperava vazio/L1", text, level.Name)
	}
}

// TestBuildContext_MetricsRecorded: a compilação registra economia (raw >
// compiled) quando há MUITO conhecimento — o contrafactual honesto do caso
// real (retrieval inteiro vs estado operacional compilado).
func TestBuildContext_MetricsRecorded(t *testing.T) {
	p := New(Config{MaxTokens: 500})
	data := &orchestration.PipelineData{
		ResolvedAgent: "a",
		KnowledgeResults: &orchestration.KnowledgeSearchResults{
			Results: []orchestration.KnowledgeSearchResult{
				{ID: "a", Content: longContent("informação sobre o módulo A ")},
				{ID: "b", Content: longContent("informação sobre o módulo B ")},
				{ID: "c", Content: longContent("informação sobre o módulo C ")},
				{ID: "d", Content: longContent("informação sobre o módulo D ")},
			},
		},
	}
	p.BuildContext("tarefa", data)
	if p.Track.CompiledTokens <= 0 {
		t.Fatal("tokens compilados não registrados")
	}
	if p.Track.RawTokens < p.Track.CompiledTokens {
		t.Fatalf("raw (%d) deveria ser >= compiled (%d) — com muito conhecimento o compilador economiza", p.Track.RawTokens, p.Track.CompiledTokens)
	}
	if p.Track.SavingsTotal() <= 0 {
		t.Fatalf("economia = %d, esperava > 0 (compilar contexto bruto economiza)", p.Track.SavingsTotal())
	}
}

// longContent gera um texto longo (para simular retrieval volumoso).
func longContent(prefix string) string {
	out := prefix
	for len(out) < 2000 {
		out += prefix
	}
	return out
}

// TestDecodeAction_StructuredAndFailOpen: o decoder da resposta funciona no
// pipeline (instruction packet e prosa fail-open).
func TestDecodeAction_StructuredAndFailOpen(t *testing.T) {
	structured := DecodeAction(`{"decision":"EDIT","target":"x","action":"fix","confidence":0.9}`)
	if structured.Status != "structured" {
		t.Fatalf("packet estruturado deveria ser structured, got %s", structured.Status)
	}

	prose := DecodeAction("vou analisar e voltar")
	if prose.Status != "text" {
		t.Fatalf("prosa deveria ser text (fail-open), got %s", prose.Status)
	}
}
