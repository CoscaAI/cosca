package contextpipeline

import (
	"strings"
	"testing"

	"github.com/CoscaAI/cosca/internal/contextcompile"
	"github.com/CoscaAI/cosca/internal/orchestration"
	"github.com/CoscaAI/cosca/internal/taskaffinity"
	"github.com/CoscaAI/cosca/internal/taskphase"
)

// ── Testes de integração F4 (ADR-045) ──────────────────────────────────────
// Cobrem: BuildTaskContext (nil-safe + perfil completo), EnrichWithProfile
// (STATE enriquecido + nil-safe), e o fluxo TAS no pipeline (rerank aplicado +
// SearchParams ajustado + STATE com stack/target). Mais a regressão com
// TaskContext nil (seção 6.2 do DESIGN-001).

// TestBuildTaskContext_NilState_Safe: estado nil → TaskContext nil (pipeline
// ignora TAS — retrocompatível).
func TestBuildTaskContext_NilState_Safe(t *testing.T) {
	tc := taskaffinity.BuildTaskContext(nil)
	if tc != nil {
		t.Fatalf("BuildTaskContext(nil) = %+v, esperava nil", tc)
	}
}

// TestBuildTaskContext_CompleteState: estado completo → perfil derivado com
// stack/target/affinity e fase detectada.
func TestBuildTaskContext_CompleteState(t *testing.T) {
	state := &taskaffinity.ImplementaçãoState{
		Prompt:      "implementar endpoint REST em Go para o módulo de autenticação",
		WorkingDir:  "C:/proj",
		OpenFiles:   []string{"internal/api/handlers.go", "internal/api/middleware.go"},
		RecentFiles: []string{"internal/api/handlers.go"},
		GoModExists: true,
	}
	tc := taskaffinity.BuildTaskContext(state)
	if tc == nil {
		t.Fatal("BuildTaskContext com estado completo retornou nil")
	}
	if tc.Profile == nil {
		t.Fatal("Profile nil")
	}
	if !tc.Profile.HasStack() {
		t.Fatal("stack não detectado (go.mod existe)")
	}
	if !tc.Profile.HasTarget() {
		t.Fatal("target não detectado (OpenFiles presente)")
	}
	if tc.PhaseDetection == nil {
		t.Fatal("PhaseDetection nil")
	}
	if tc.PhaseDetection.Phase != taskphase.PhaseImplementation {
		t.Fatalf("fase = %s, esperava implementation (termos de implementação + arquivos abertos)", tc.PhaseDetection.Phase)
	}
}

// TestBuildTaskContext_ExplorationState: estado de exploração (sem arquivos,
// termos de exploração) → fase exploration.
func TestBuildTaskContext_ExplorationState(t *testing.T) {
	state := &taskaffinity.ImplementaçãoState{
		Prompt:     "como funciona a busca semântica no projeto?",
		WorkingDir: "C:/proj",
	}
	tc := taskaffinity.BuildTaskContext(state)
	if tc == nil {
		t.Fatal("BuildTaskContext retornou nil")
	}
	if tc.PhaseDetection == nil || tc.PhaseDetection.Phase != taskphase.PhaseExploration {
		t.Fatalf("fase = %v, esperava exploration", tc.PhaseDetection)
	}
}

// TestEnrichWithProfile_StateEnriched: EnrichWithProfile adiciona stack/target
// à seção STATE.
func TestEnrichWithProfile_StateEnriched(t *testing.T) {
	cc := contextcompile.Compile(contextcompile.CompileInput{
		Prompt: "implementar endpoint",
		Agent:  "Backend API Specialist",
		Role:   "backend",
	})
	profile := &taskaffinity.TaskProfile{
		Stack:  []string{"go"},
		Target: "internal/api/handlers.go",
	}
	cc.EnrichWithProfile(profile)

	// A seção STATE deve conter stack e target.
	var stateContent string
	for _, s := range cc.Sections {
		if s.Name == "STATE" {
			stateContent = s.Content
			break
		}
	}
	if !strings.Contains(stateContent, "stack=go") {
		t.Fatalf("STATE sem stack=go: %q", stateContent)
	}
	if !strings.Contains(stateContent, "target=internal/api/handlers.go") {
		t.Fatalf("STATE sem target: %q", stateContent)
	}
}

// TestEnrichWithProfile_NilSafe: EnrichWithProfile com profile nil não quebra.
func TestEnrichWithProfile_NilSafe(t *testing.T) {
	cc := contextcompile.Compile(contextcompile.CompileInput{Prompt: "x"})
	cc.EnrichWithProfile(nil) // não deve panic
	var nilCC *contextcompile.CompiledContext
	nilCC.EnrichWithProfile(&taskaffinity.TaskProfile{}) // não deve panic
}

// TestPipeline_TaskContext_ReranksAndEnriches: com TaskContext presente, o
// pipeline re-pondera os resultados por afinidade E enriquece o STATE.
func TestPipeline_TaskContext_ReranksAndEnriches(t *testing.T) {
	p := New(Config{MaxTokens: 1000})

	// Perfil com afinidade em "api"/"handler"/"rest" — um resultado do projeto
	// deve subir sobre um genérico.
	profile := &taskaffinity.TaskProfile{
		Task:       "implementar endpoint REST em Go",
		Stack:      []string{"go"},
		Target:     "internal/api/handlers.go",
		Affinity:   []string{"api", "handler", "rest", "go", "endpoint"},
		Confidence: 0.95,
	}
	detection := &taskphase.PhaseDetection{Phase: taskphase.PhaseImplementation, Confidence: 0.9}

	data := &orchestration.PipelineData{
		ResolvedAgent: "Backend API Specialist",
		AgentRole:     "backend",
		TaskContext: &taskaffinity.TaskContext{
			Profile:        profile,
			PhaseDetection: detection,
		},
		KnowledgeResults: &orchestration.KnowledgeSearchResults{
			Results: []orchestration.KnowledgeSearchResult{
				// Resultado do projeto (muitos termos de afinidade: handler/rest/api/endpoint/go)
				{ID: "proj", Title: "internal/api/handlers.go", Content: "handler rest api endpoint route go", Score: 0.70, Epistemic: "FACT"},
				// Resultado genérico (poucos termos de afinidade: só "rest")
				{ID: "gen", Title: "REST API docs", Content: "representational state transfer", Score: 0.72, Epistemic: "FACT"},
			},
		},
	}

	text, _ := p.BuildContext("implementar endpoint REST", data)

	// O resultado do projeto deve ter subido (score maior que o genérico agora).
	proj, gen := data.KnowledgeResults.Results[0], data.KnowledgeResults.Results[1]
	if proj.ID != "proj" {
		t.Fatalf("primeiro resultado = %s, esperava proj (re-ponderado por afinidade)", proj.ID)
	}
	if proj.Score <= gen.Score {
		t.Fatalf("proj.Score=%.3f não superou gen.Score=%.3f após rerank", proj.Score, gen.Score)
	}

	// O STATE deve conter stack e target (EnrichWithProfile).
	if !strings.Contains(text, "stack=go") || !strings.Contains(text, "target=internal/api/handlers.go") {
		t.Fatalf("contexto sem stack/target do perfil:\n%s", text)
	}
}

// TestPipeline_TaskContext_AdjustsSearchParams: com PhaseDetection presente,
// o SearchParams é ajustado pela fase (ex.: implementação ativa o grafo).
func TestPipeline_TaskContext_AdjustsSearchParams(t *testing.T) {
	p := New(Config{MaxTokens: 1000})
	detection := &taskphase.PhaseDetection{Phase: taskphase.PhaseImplementation, Confidence: 0.9}

	data := &orchestration.PipelineData{
		ResolvedAgent: "a",
		TaskContext: &taskaffinity.TaskContext{
			Profile:        &taskaffinity.TaskProfile{Task: "implementar", Affinity: []string{"go"}, Confidence: 0.5},
			PhaseDetection: detection,
		},
		KnowledgeResults: &orchestration.KnowledgeSearchResults{},
	}

	p.BuildContext("implementar algo", data)

	// Fase implementação → EnableGraph true, MinScore 0.15.
	if !data.SearchParams.EnableGraph {
		t.Fatal("EnableGraph deveria ser true na fase implementação")
	}
	if data.SearchParams.MinScore != 0.15 {
		t.Fatalf("MinScore = %v, esperava 0.15", data.SearchParams.MinScore)
	}
	if data.SearchParams.Limit != 20 {
		t.Fatalf("Limit = %d, esperava 20", data.SearchParams.Limit)
	}
}

// TestPipeline_NilTaskContext_Regression: com TaskContext nil, o pipeline se
// comporta exatamente como antes (zero regressão).
func TestPipeline_NilTaskContext_Regression(t *testing.T) {
	p := New(Config{MaxTokens: 1000})
	data := &orchestration.PipelineData{
		ResolvedAgent: "Backend API Specialist",
		AgentRole:     "backend",
		KnowledgeResults: &orchestration.KnowledgeSearchResults{
			Results: []orchestration.KnowledgeSearchResult{
				{ID: "a", Content: "server exposes 11 tools", Score: 0.92, Epistemic: "FACT"},
				{ID: "b", Content: "generic_mcp advertises 6 obsolete tools", Score: 0.61},
			},
		},
	}

	// Snapshot antes.
	before := data.KnowledgeResults.Results[0].Score

	text, level := p.BuildContext("corrigir endpoint MCP", data)

	// Sem TaskContext, os resultados não são re-ponderados.
	if data.KnowledgeResults.Results[0].Score != before {
		t.Fatalf("score mudou sem TaskContext: %.3f → %.3f", before, data.KnowledgeResults.Results[0].Score)
	}
	if text == "" {
		t.Fatal("contexto vazio")
	}
	if level.Name == "" {
		t.Fatal("nível vazio")
	}
}
