package orchestration

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/CoscaAI/cosca/internal/chat"
	"github.com/CoscaAI/cosca/internal/deliberate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Helpers ─────────────────────────────────────────────────────────────────

// deliberationPC builds a PipelineContext primed with the evidence the
// Kernel-First Deliberation stage consumes (knowledge + memory results).
func deliberationPC(kr *KnowledgeSearchResults, mem []MemoryRecord) PipelineContext {
	pc := NewPipelineContext("delib-1", "test prompt")
	pc = pc.WithResolvedAgent("Backend Chief")
	if kr != nil {
		pc = pc.WithKnowledgeResults(kr)
	}
	if mem != nil {
		pc = pc.WithMemoryResults(mem)
	}
	return pc
}

// gateWeights biases the convergence weights so the two dimensions the
// deliberator actually collects (recommendation + premises) can reach the
// EmitOK gate (0.70) — the default A3 weights (0.30/0.25/0.25/0.20) cap the
// covered weight at 0.55, which would make EmitOK unreachable through the
// standard Deliberate path.
func gateWeights() deliberate.ConvergenceWeights {
	return deliberate.ConvergenceWeights{Recommendation: 0.70, Premises: 0.30}
}

// ─── Test 1: Gate de confiança ───────────────────────────────────────────────

func TestDeliberate_GateConfidence(t *testing.T) {
	cfg := DefaultDeliberateConfig()
	cfg.Enabled = true
	cfg.Weights = gateWeights()

	tests := []struct {
		name        string
		kr          *KnowledgeSearchResults
		mem         []MemoryRecord
		wantVerdict deliberate.Emit
	}{
		{
			name: "sufficient evidence -> EmitOK (responds without LLM)",
			kr: &KnowledgeSearchResults{Results: []KnowledgeSearchResult{
				{ID: "k1", Snippet: "use go", Score: 0.9},
				{ID: "k2", Snippet: "use go", Score: 0.8},
			}},
			wantVerdict: deliberate.EmitOK,
		},
		{
			name: "partial evidence -> EmitWithReservations",
			kr: &KnowledgeSearchResults{Results: []KnowledgeSearchResult{
				{ID: "k1", Snippet: "use go", Score: 0.9},
				{ID: "k2", Snippet: "use rust", Score: 0.8},
			}},
			mem: []MemoryRecord{
				{ID: "m1", Content: "team prefers go"},
				{ID: "m2", Content: "team prefers go"},
			},
			wantVerdict: deliberate.EmitWithReservations,
		},
		{
			name:        "no evidence -> Escalate",
			wantVerdict: deliberate.Escalate,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := NewDeliberator(cfg)
			pc := deliberationPC(tt.kr, tt.mem)
			trace, err := d.Deliberate(context.Background(), pc)
			require.NoError(t, err)
			assert.Equal(t, tt.wantVerdict, trace.Verdict)
		})
	}
}

func TestEvaluateEmitWithThresholds_Configurable(t *testing.T) {
	cfg := DefaultDeliberateConfig()
	cfg.Enabled = true
	cfg.EmitThreshold = 0.80
	cfg.ReservationThreshold = 0.60

	tests := []struct {
		name  string
		final float64
		want  deliberate.Emit
	}{
		{"above emit", 0.85, deliberate.EmitOK},
		{"at emit", 0.80, deliberate.EmitOK},
		{"between gates", 0.70, deliberate.EmitWithReservations},
		{"at reservation", 0.60, deliberate.EmitWithReservations},
		{"below reservation", 0.40, deliberate.Escalate},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evaluateEmitWithThresholds(deliberate.ConfidenceBreakdown{Final: tt.final}, cfg)
			assert.Equal(t, tt.want, got)
		})
	}
}

// ─── Test 2: Zero achismo ────────────────────────────────────────────────────

func TestDeliberate_ZeroAchismo_LowScoreDeweighted(t *testing.T) {
	cfg := DefaultDeliberateConfig()
	cfg.Enabled = true
	cfg.Weights = gateWeights()

	// Knowledge results below MinScore (0.50) produce no Position and
	// contribute nothing to convergence — the Kernel concludes "uncertain".
	kr := &KnowledgeSearchResults{Results: []KnowledgeSearchResult{
		{ID: "k-low", Snippet: "guess", Score: 0.10},
		{ID: "k-low2", Snippet: "guess", Score: 0.20},
	}}
	pc := deliberationPC(kr, nil)
	d := NewDeliberator(cfg)
	trace, err := d.Deliberate(context.Background(), pc)
	require.NoError(t, err)

	assert.Empty(t, trace.Positions, "low-score evidence must not become positions")
	assert.Equal(t, 0.0, trace.Convergence)
	assert.Equal(t, deliberate.Escalate, trace.Verdict)
}

func TestDeliberate_CollectPositions_OnlySubstantiated(t *testing.T) {
	cfg := DefaultDeliberateConfig()
	cfg.Enabled = true

	kr := &KnowledgeSearchResults{Results: []KnowledgeSearchResult{
		{ID: "k1", Snippet: "good evidence", Score: 0.9},
		{ID: "k2", Snippet: "", Score: 0.9}, // empty snippet+content -> skipped
	}}
	mem := []MemoryRecord{
		{ID: "m1", Content: "premise"},
		{ID: "m2", Content: ""}, // empty content -> skipped
	}
	pc := deliberationPC(kr, mem)
	d := NewDeliberator(cfg)

	positions := d.collectPositions(pc.Data)
	require.NotEmpty(t, positions)
	for _, p := range positions {
		assert.True(t, p.Effective(), "position %s must be substantiated (zero achismo)", p.ID)
		assert.NotEmpty(t, p.EvidenceIDs, "position %s must carry evidence IDs", p.ID)
	}

	ids := collectEvidenceIDs(positions)
	assert.NotContains(t, ids, "k2")
	assert.NotContains(t, ids, "m2")
}

// ─── Test 3: Contexto limpo ──────────────────────────────────────────────────

func TestBuildCleanContext_Structured(t *testing.T) {
	cfg := DefaultDeliberateConfig()
	cfg.Enabled = true
	cfg.MaxEvidence = 2
	cfg.MaxCharsPerEvidence = 10

	kr := &KnowledgeSearchResults{Results: []KnowledgeSearchResult{
		{ID: "k1", Snippet: "recommendation one", Score: 0.9},
		{ID: "k2", Snippet: "recommendation two", Score: 0.8},
		{ID: "k3", Snippet: "recommendation three", Score: 0.7},
	}}
	pc := deliberationPC(kr, nil)
	d := NewDeliberator(cfg)
	trace, err := d.Deliberate(context.Background(), pc)
	require.NoError(t, err)

	clean := BuildCleanContext(pc.Data, trace, cfg)

	// All structured sections present.
	for _, section := range []string{
		"DELIBERAÇÃO DO KERNEL",
		"POSITIONS",
		"EVIDÊNCIAS",
		"HIPÓTESES",
		"CONTRADIÇÕES",
		"PERGUNTA AO ESPECIALISTA",
	} {
		assert.Contains(t, clean, section)
	}

	// MaxEvidence respected: only the top-2 by score (k1, k2) are rendered as
	// evidence; k3 is excluded from the evidence block.
	assert.Contains(t, clean, "k1 — knowledge.db")
	assert.Contains(t, clean, "k2 — knowledge.db")
	assert.NotContains(t, clean, "k3 — knowledge.db")

	// MaxCharsPerEvidence respected: long claims are truncated (no full dump).
	assert.NotContains(t, clean, "recommendation one")
	assert.Contains(t, clean, "...")
}

func TestBuildCleanContext_NoEvidence(t *testing.T) {
	cfg := DefaultDeliberateConfig()
	cfg.Enabled = true
	pc := deliberationPC(nil, nil)
	d := NewDeliberator(cfg)
	trace, err := d.Deliberate(context.Background(), pc)
	require.NoError(t, err)

	clean := BuildCleanContext(pc.Data, trace, cfg)
	assert.Contains(t, clean, "nenhuma posição substantiada")
	assert.Contains(t, clean, "nenhuma evidência traceável")
}

// ─── Test 4: Fail-closed ─────────────────────────────────────────────────────

func TestDeliberate_FailClosed_ExecutorBehavesAsToday(t *testing.T) {
	provider := newMockChatProvider("test", "test-model")
	resolver := newMockAgentResolver()
	resolver.add("Backend Chief", "Backend Chief", "backend", "Backend dev")

	// Default config: DeliberateConfig.Enabled = false (fail-closed / LEI DO
	// COFRE). The deliberator is never created and the flow is unchanged.
	engine := NewEngine(nil, nil, nil, resolver, nil, provider, DefaultOrchestratorConfig(), nil)
	require.Nil(t, engine.deliberator, "deliberator must be nil when disabled (fail-closed)")

	result, err := engine.Execute(context.Background(), &Request{Prompt: "build an api"})
	require.NoError(t, err)
	require.NotEmpty(t, result.Response)
	// The legacy LLM path is used exactly as before.
	assert.GreaterOrEqual(t, provider.chatCalled.Load(), int64(1))
}

// ─── Test 5: Trilha (DeliberationTrace) ──────────────────────────────────────

func TestDeliberate_Trace(t *testing.T) {
	cfg := DefaultDeliberateConfig()
	cfg.Enabled = true
	cfg.Weights = gateWeights()

	kr := &KnowledgeSearchResults{Results: []KnowledgeSearchResult{
		{ID: "k1", Snippet: "use go", Score: 0.9},
		{ID: "k2", Snippet: "use go", Score: 0.8},
	}}
	pc := deliberationPC(kr, nil)
	d := NewDeliberator(cfg)
	trace, err := d.Deliberate(context.Background(), pc)
	require.NoError(t, err)

	assert.Equal(t, deliberate.EmitOK, trace.Verdict)
	assert.InEpsilon(t, 0.70, trace.Convergence, 0.0001)
	assert.InEpsilon(t, 0.70, trace.Confidence.Final, 0.0001)
	assert.True(t, trace.Deterministic, "EmitOK must be flagged deterministic")
	assert.ElementsMatch(t, []string{"k1", "k2"}, trace.EvidenceIDs)
	assert.Len(t, trace.Positions, 2)
	assert.False(t, trace.Timestamp.IsZero(), "trace must carry a timestamp")
}

func TestDeliberate_Trace_NonDeterministic(t *testing.T) {
	cfg := DefaultDeliberateConfig()
	cfg.Enabled = true
	cfg.Weights = gateWeights()

	// No evidence -> Escalate -> not deterministic (the LLM is consulted).
	pc := deliberationPC(nil, nil)
	d := NewDeliberator(cfg)
	trace, err := d.Deliberate(context.Background(), pc)
	require.NoError(t, err)

	assert.Equal(t, deliberate.Escalate, trace.Verdict)
	assert.False(t, trace.Deterministic)
	assert.Empty(t, trace.EvidenceIDs)
}

// ─── Test 6: Integração com mock ─────────────────────────────────────────────

func TestEngine_Deliberation_EmitOK_SkipsLLM(t *testing.T) {
	provider := newMockChatProvider("test", "test-model")
	var called atomic.Int64
	provider.chatFn = func(_ context.Context, _ []chat.Message, _ chat.ChatOptions) (*chat.ChatResponse, error) {
		called.Add(1)
		return nil, errors.New("provider must NOT be called when the Kernel emits OK")
	}

	resolver := newMockAgentResolver()
	resolver.add("Backend Chief", "Backend Chief", "backend", "Backend dev")

	knowledge := &mockKnowledgeSearcher{
		results: &KnowledgeSearchResults{
			Results: []KnowledgeSearchResult{
				{ID: "k1", Snippet: "use go for the api", Score: 0.95},
				{ID: "k2", Snippet: "use go for the api", Score: 0.90},
			},
			TotalCount: 2,
			Query:      "build an api",
		},
	}

	cfg := DefaultOrchestratorConfig()
	cfg.DeliberateConfig.Enabled = true
	cfg.DeliberateConfig.Weights = gateWeights()

	engine := NewEngine(knowledge, nil, nil, resolver, nil, provider, cfg, nil)
	require.NotNil(t, engine.deliberator)

	result, err := engine.Execute(context.Background(), &Request{Prompt: "build an api"})
	require.NoError(t, err)
	require.NotEmpty(t, result.Response)
	assert.Equal(t, int64(0), called.Load(), "LLM must not be called on EmitOK")
	assert.Contains(t, result.Response, "use go for the api")
}

func TestEngine_Deliberation_Uncertain_CallsLLMWithCleanContext(t *testing.T) {
	provider := newMockChatProvider("test", "test-model")
	var captured []chat.Message
	provider.chatResponse = &chat.ChatResponse{
		ID:    "resp",
		Model: "test-model",
		Choices: []chat.Choice{
			{Index: 0, Message: chat.Message{Role: chat.RoleAssistant, Content: "llm answer"}, FinishReason: chat.FinishReasonStop},
		},
		Usage: chat.Usage{TotalTokens: 5},
	}
	provider.chatFn = func(_ context.Context, messages []chat.Message, _ chat.ChatOptions) (*chat.ChatResponse, error) {
		captured = messages
		return provider.chatResponse, nil
	}

	resolver := newMockAgentResolver()
	resolver.add("Backend Chief", "Backend Chief", "backend", "Backend dev")

	// No knowledge/memory -> Escalate -> the clean context is handed to the LLM.
	cfg := DefaultOrchestratorConfig()
	cfg.DeliberateConfig.Enabled = true
	cfg.DeliberateConfig.Weights = gateWeights()

	engine := NewEngine(nil, nil, nil, resolver, nil, provider, cfg, nil)

	result, err := engine.Execute(context.Background(), &Request{Prompt: "build an api"})
	require.NoError(t, err)
	assert.Equal(t, "llm answer", result.Response)
	assert.GreaterOrEqual(t, provider.chatCalled.Load(), int64(1))

	// The user message handed to the LLM is the clean deliberation context.
	require.Len(t, captured, 2)
	assert.Contains(t, captured[1].Content, "DELIBERAÇÃO DO KERNEL")
}

// ─── Test 7: Regressão (executor determinístico + deliberação) ───────────────

func TestExecutor_DeliberationHandled_SkipsLLM(t *testing.T) {
	provider := newMockChatProvider("test", "test-model")
	var called atomic.Int64
	provider.chatFn = func(_ context.Context, _ []chat.Message, _ chat.ChatOptions) (*chat.ChatResponse, error) {
		called.Add(1)
		return nil, errors.New("must not be called")
	}

	exec := NewExecutor(provider, DefaultExecutorConfig(), nil)

	pc := NewPipelineContext("req-delib", "test")
	pc = pc.WithResolvedAgent("Backend Chief")
	pc = pc.WithAgentRole("Backend Chief")
	pc = pc.WithDeliberationHandled(true) // the Kernel already answered (EmitOK)

	result, err := exec.Execute(context.Background(), pc)
	require.NoError(t, err)
	assert.Equal(t, int64(0), called.Load(), "executor must skip the LLM when DeliberationHandled is set")
	assert.True(t, result.Data.DeliberationHandled)
}

func TestExecutor_DeliberationHandled_False_BehavesAsToday(t *testing.T) {
	provider := newMockChatProvider("test", "test-model")
	exec := NewExecutor(provider, DefaultExecutorConfig(), nil)

	pc := NewPipelineContext("req-legacy", "test")
	pc = pc.WithResolvedAgent("Backend Chief")
	pc = pc.WithAgentRole("Backend Chief")

	// DeliberationHandled defaults to false -> the executor calls the LLM.
	result, err := exec.Execute(context.Background(), pc)
	require.NoError(t, err)
	assert.Equal(t, int64(1), provider.chatCalled.Load())
	assert.False(t, result.Data.DeliberationHandled)
	assert.NotEmpty(t, result.Data.LLMResponse)
}

// ─── Sanity: BuildDeterministicResponse ──────────────────────────────────────

func TestBuildDeterministicResponse_FromPositions(t *testing.T) {
	cfg := DefaultDeliberateConfig()
	cfg.Enabled = true
	cfg.Weights = gateWeights()

	kr := &KnowledgeSearchResults{Results: []KnowledgeSearchResult{
		{ID: "k1", Snippet: "use go for the api", Score: 0.95},
		{ID: "k2", Snippet: "use go for the api", Score: 0.90},
	}}
	pc := deliberationPC(kr, nil)
	d := NewDeliberator(cfg)
	trace, err := d.Deliberate(context.Background(), pc)
	require.NoError(t, err)
	require.Equal(t, deliberate.EmitOK, trace.Verdict)

	resp := BuildDeterministicResponse(pc.Data, trace)
	assert.Contains(t, resp, "use go for the api")
	assert.Contains(t, resp, "Evidências: k1, k2")
}

func TestBuildDeterministicResponse_EmptyWhenNoPositions(t *testing.T) {
	cfg := DefaultDeliberateConfig()
	cfg.Enabled = true
	pc := deliberationPC(nil, nil)
	d := NewDeliberator(cfg)
	trace, err := d.Deliberate(context.Background(), pc)
	require.NoError(t, err)

	assert.Equal(t, "", BuildDeterministicResponse(pc.Data, trace))
}

// ─── Sanity: normalizeDeliberateConfig ───────────────────────────────────────

func TestNormalizeDeliberateConfig_Defaults(t *testing.T) {
	cfg := normalizeDeliberateConfig(DeliberateConfig{})
	assert.Equal(t, deliberate.DefaultEmitThreshold, cfg.EmitThreshold)
	assert.Equal(t, 0.50, cfg.ReservationThreshold)
	assert.Equal(t, 5, cfg.MaxEvidence)
	assert.Equal(t, 300, cfg.MaxCharsPerEvidence)
	assert.Equal(t, 0.50, cfg.MinScore)
	assert.Equal(t, deliberate.DefaultConvergenceWeights(), cfg.Weights)
}

func TestDefaultDeliberateConfig_FailClosed(t *testing.T) {
	cfg := DefaultDeliberateConfig()
	assert.False(t, cfg.Enabled, "default must be fail-closed (disabled)")
	assert.Equal(t, deliberate.DefaultEmitThreshold, cfg.EmitThreshold)
}
