package orchestration

// ETAPA 1 — Streaming Kernel-First (FASE 2): prova que POST /v1/run/stream
// atravessa o MESMO fluxo decisório de POST /v1/run. A Deliberação ADR-032
// vive no engine (runStreamingPipeline) e decide ANTES da LLM. Estas provas
// cobrem os dois desfechos:
//
//	(A) O Kernel resolve (EmitOK) → resposta DETERMINÍSTICA sem chamada à LLM.
//	(B) O Kernel escala (Escalate) → Router → provider em chunks em streaming.
//
// Nenhum caminho de deliberação paralelo — apenas reuso (ADR-015/ADR-032).

import (
	"context"
	"errors"
	"testing"

	"github.com/CoscaAI/cosca/internal/chat"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEngine_Deliberation_Stream_EmitOK_SkipsLLM (PROVA A, engine-level):
// request que a deliberação resolve deterministicamente → 0 chamadas a
// ChatProvider.Chat/ChatStream; a resposta é montada pelo Kernel e emitida como
// chunk de stream; o canal fecha SEM evento de erro.
func TestEngine_Deliberation_Stream_EmitOK_SkipsLLM(t *testing.T) {
	provider := newMockChatProvider("test", "test-model")
	provider.chatFn = func(_ context.Context, _ []chat.Message, _ chat.ChatOptions) (*chat.ChatResponse, error) {
		return nil, errors.New("Chat must NOT be called on EmitOK")
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
	require.NotNil(t, engine.deliberator, "deliberator must exist when Enabled")

	events, err := engine.ExecuteStream(context.Background(), &Request{Prompt: "build an api"})
	require.NoError(t, err)

	var chunks []string
	var sawErr bool
	for ev := range events {
		switch ev.Type {
		case StreamEventChunk:
			if ev.Content != "" {
				chunks = append(chunks, ev.Content)
			}
		case StreamEventError:
			sawErr = true
		}
	}

	// Kernel resolveu: resposta determinística presente, sem erro de stream.
	require.NotEmpty(t, chunks, "deterministic response must be streamed as a chunk")
	assert.Contains(t, chunks[0], "use go for the api")
	assert.False(t, sawErr, "kernel-resolved stream must NOT emit an error event")

	// Zero-LLM: nem Chat nem ChatStream foram chamados.
	assert.Equal(t, int64(0), provider.chatCalled.Load(), "Chat must not be called on EmitOK")
	assert.Equal(t, int64(0), provider.streamCalled.Load(), "ChatStream must not be called on EmitOK")
}

// TestEngine_Deliberation_Stream_Escalate_StreamsLLM (PROVA B, engine-level):
// request que escala (sem conhecimento → Escalate) → Router → provider emite
// chunks em streaming; nenhum erro; ChatStream chamado.
func TestEngine_Deliberation_Stream_Escalate_StreamsLLM(t *testing.T) {
	provider := newMockChatProvider("test", "test-model")
	provider.streamResponse = &mockChatStream{
		chunks: []chat.ChatStreamChunk{
			{ID: "s", Model: "test-model", Choices: []chat.StreamChoice{{Index: 0, Delta: chat.Message{Content: "Hello "}}}},
			{ID: "s", Model: "test-model", Choices: []chat.StreamChoice{{Index: 0, Delta: chat.Message{Content: "World"}}}},
			{ID: "s", Model: "test-model", Choices: []chat.StreamChoice{{Index: 0, Delta: chat.Message{}, FinishReason: chat.FinishReasonStop}}},
		},
	}

	resolver := newMockAgentResolver()
	resolver.add("Backend Chief", "Backend Chief", "backend", "Backend dev")

	cfg := DefaultOrchestratorConfig()
	cfg.DeliberateConfig.Enabled = true
	cfg.DeliberateConfig.Weights = gateWeights()

	// Sem conhecimento → contexto insuficiente → Escalate → LLM em streaming.
	engine := NewEngine(nil, nil, nil, resolver, nil, provider, cfg, nil)

	events, err := engine.ExecuteStream(context.Background(), &Request{Prompt: "build an api"})
	require.NoError(t, err)

	var chunks []string
	var sawErr bool
	for ev := range events {
		switch ev.Type {
		case StreamEventChunk:
			if ev.Content != "" {
				chunks = append(chunks, ev.Content)
			}
		case StreamEventError:
			sawErr = true
		}
	}

	require.NotEmpty(t, chunks, "provider chunks must stream after Escalate")
	assert.Equal(t, "Hello ", chunks[0])
	assert.Equal(t, "World", chunks[1])
	assert.False(t, sawErr, "escalated stream must NOT emit an error event")
	assert.GreaterOrEqual(t, provider.streamCalled.Load(), int64(1), "ChatStream must be called on Escalate")
}
