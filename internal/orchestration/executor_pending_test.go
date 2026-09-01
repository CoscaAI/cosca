package orchestration

// executor_pending_test.go — Pending Resolution no Executor (Don + professor,
// 2026-09-01): quando o loop de tool-calls termina por limite com trabalho
// pendente, o executor inspeciona o ESTADO e resolve a continuação mínima
// implicada (em vez de abandonar na reta final).

import (
	"context"
	"strings"
	"testing"

	"github.com/CoscaAI/cosca/internal/chat"
	"github.com/CoscaAI/cosca/internal/pending"
)

// infiniteToolProvider sempre devolve tool calls — estoura o MaxToolRounds.
type infiniteToolProvider struct {
	*mockChatProvider
}

func newInfiniteToolProvider() *infiniteToolProvider {
	p := &infiniteToolProvider{mockChatProvider: newMockChatProvider("test", "m")}
	p.chatFn = func(_ context.Context, _ []chat.Message, _ chat.ChatOptions) (*chat.ChatResponse, error) {
		return &chat.ChatResponse{
			ID:    "resp-tool",
			Model: "m",
			Choices: []chat.Choice{
				{
					Index: 0,
					Message: chat.Message{
						Role:      chat.RoleAssistant,
						Content:   "chamando tool",
						ToolCalls: []chat.ToolCall{chat.ToolCall{ID: "tc-1", Type: "function", Function: chat.FunctionCall{Name: "search_codebase", Arguments: `{"query":"x"}`}}},
					},
					FinishReason: chat.FinishReasonToolCalls,
				},
			},
			Usage: chat.Usage{PromptTokens: 5, CompletionTokens: 5, TotalTokens: 10},
		}, nil
	}
	return p
}

// TestExecutor_PendingResolution_Resolvable: com o PendingResolver ligado, um
// loop que estoura o limite com tool calls pendentes NÃO abandona — registra a
// continuação mínima implicada (resolvível).
func TestExecutor_PendingResolution_Resolvable(t *testing.T) {
	provider := newInfiniteToolProvider()
	cfg := DefaultExecutorConfig()
	cfg.MaxToolRounds = 2 // estoura rápido
	cfg.PendingResolver = pending.New(2)

	ex := NewExecutor(provider, cfg, nil)
	pc := NewPipelineContext("req-pr-1", "tarefa com tools")
	pc.Data.ResolvedAgent = "a"

	// O executor usa o toolRunner só se configurado; sem ele, os tool calls
	// são devolvidos mas o loop não executa — vamos simular o estado com
	// observações para a inspeção de pendências reconhecer "persistir".
	result, err := ex.Execute(context.Background(), pc)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	// Com MaxToolRounds=2 e provider infinito, o loop para no limite; sem
	// PendingResolver o comportamento atual é manter o que acumulou. Com o
	// resolver, a inspeção roda (o test valida que não quebra e que a
	// resposta existe).
	if result.Data.LLMResponse == "" {
		t.Fatal("resposta vazia após limite de tool rounds")
	}
}

// TestExecutor_PendingResolution_StateProjection: ObservationsForPending
// projeta as observações do PipelineData.
func TestExecutor_PendingResolution_StateProjection(t *testing.T) {
	pc := NewPipelineContext("req", "tarefa")
	pc.Data.LLMResponse = "resposta do modelo"
	pc.Data.MemoryContext = "memória relevante"
	obs := pc.Data.ObservationsForPending()
	if len(obs) != 2 {
		t.Fatalf("observações = %d, esperava 2", len(obs))
	}
	if !strings.Contains(obs[0], "resposta do modelo") || !strings.Contains(obs[1], "memória relevante") {
		t.Fatalf("observações erradas: %v", obs)
	}
}

// TestExecutor_PendingResolution_DisabledByDefault: sem PendingResolver, o
// comportamento é o atual (sem inspeção — sem regressão).
func TestExecutor_PendingResolution_DisabledByDefault(t *testing.T) {
	provider := newInfiniteToolProvider()
	cfg := DefaultExecutorConfig()
	cfg.MaxToolRounds = 2
	// cfg.PendingResolver = nil (default)

	ex := NewExecutor(provider, cfg, nil)
	pc := NewPipelineContext("req-pr-2", "tarefa")
	pc.Data.ResolvedAgent = "a"

	result, err := ex.Execute(context.Background(), pc)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Data.LLMResponse == "" {
		t.Fatal("sem resolver o executor deveria manter o comportamento atual (não quebrar)")
	}
}
