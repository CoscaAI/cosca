package engine

// engine_pending_test.go — Pending Resolution no AgentEngine (Don + professor,
// 2026-09-01): quando o MaxTurns atinge com tool calls pendentes, o engine
// inspeciona o estado e registra a continuação mínima — sem abandonar na reta
// final, sem inventar próximo passo.

import (
	"context"
	"strings"
	"testing"

	"github.com/CoscaAI/cosca/internal/chat"
	"github.com/CoscaAI/cosca/internal/pending"
)

// toolAlwaysProvider sempre devolve tool calls — força o MaxTurns a atingir
// com trabalho pendente.
type toolAlwaysProvider struct {
	*mockProvider
}

func newToolAlwaysProvider() *toolAlwaysProvider {
	p := &toolAlwaysProvider{mockProvider: newMockProvider()}
	p.chatFunc = func(_ context.Context, _ []chat.Message, _ chat.ChatOptions) (*chat.ChatResponse, error) {
		return &chat.ChatResponse{
			Choices: []chat.Choice{
				{
					Index: 0,
					Message: chat.Message{
						Role: chat.RoleAssistant,
						Content: "chamando tool",
						ToolCalls: []chat.ToolCall{{
							ID: "tc-1", Type: "function",
							Function: chat.FunctionCall{Name: "read", Arguments: `{"path":"x"}`},
						}},
					},
					FinishReason: chat.FinishReasonToolCalls,
				},
			},
			Usage: chat.Usage{PromptTokens: 5, CompletionTokens: 5, TotalTokens: 10},
		}, nil
	}
	return p
}

// TestEnginePendingResolution_MaxTurnsWithResolver: com o PendingResolver
// ligado, o engine atinge o MaxTurns com tool calls pendentes SEM quebrar —
// a inspeção roda e o engine encerra com resultado (não pânico).
func TestEnginePendingResolution_MaxTurnsWithResolver(t *testing.T) {
	provider := newToolAlwaysProvider()
	reg := NewAgentRegistry()
	eng := NewAgentEngine(
		reg,
		NewContextBuilder(reg, DefaultMaxTokens),
		NewRouter(reg),
		provider,
		newMockExecutor(), // executor mock — as tool calls são executadas
		EngineConfig{
			Model:           "test",
			MaxTurns:        2,
			PendingResolver: pending.New(2),
		},
		nil, nil, nil,
	)

	result, err := eng.Run(context.Background(), "tarefa com tools", nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result == nil {
		t.Fatal("resultado nil após MaxTurns com pendência")
	}
}

// TestEnginePendingResolution_NoResolver_Unchanged: sem resolver, o engine se
// comporta como antes (MaxTurns para sem inspecionar — sem regressão).
func TestEnginePendingResolution_NoResolver_Unchanged(t *testing.T) {
	provider := newToolAlwaysProvider()
	reg := NewAgentRegistry()
	eng := NewAgentEngine(
		reg,
		NewContextBuilder(reg, DefaultMaxTokens),
		NewRouter(reg),
		provider,
		newMockExecutor(),
		EngineConfig{Model: "test", MaxTurns: 2},
		nil, nil, nil,
	)

	_, err := eng.Run(context.Background(), "tarefa", nil)
	if err != nil {
		t.Fatalf("Run sem resolver deveria funcionar: %v", err)
	}
}

// TestEnginePendingResolution_StateProjection: engineObservations projeta o
// último conteúdo como observação (evidência do estado para a inspeção).
func TestEnginePendingResolution_StateProjection(t *testing.T) {
	obs := engineObservations("resultado parcial do agente")
	if len(obs) != 1 || !strings.Contains(obs[0], "resultado parcial") {
		t.Fatalf("observações erradas: %v", obs)
	}
	if engineObservations("") != nil {
		t.Fatal("sem conteúdo a observação deveria ser nil")
	}
}
