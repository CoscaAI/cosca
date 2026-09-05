package engine

// halt_test.go — KILL-SWITCH do kernel (Etapa 3b).
//
// O botão de emergência do Don protege o caminho que mais gasta tokens: quando
// o HaltChecker (kernel.EmergencyManager) reporta halted, o engine NUNCA chama
// o provider — nenhuma chamada LLM acontece.

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/CoscaAI/cosca/internal/chat"
)

// haltedChecker é um HaltChecker de teste que sempre reporta halted=true.
type haltedChecker struct{}

func (haltedChecker) IsHalted() bool { return true }

// TestEngineHaltChecker_BlocksLLMCall: com o kill-switch acionado, o engine
// retorna sem chamar o provider — o contador de chamadas LLM fica em 0.
func TestEngineHaltChecker_BlocksLLMCall(t *testing.T) {
	var llmCalls int64

	provider := newMockProvider()
	provider.chatFunc = func(_ context.Context, _ []chat.Message, _ chat.ChatOptions) (*chat.ChatResponse, error) {
		atomic.AddInt64(&llmCalls, 1)
		return &chat.ChatResponse{
			Choices: []chat.Choice{{Message: chat.Message{Role: chat.RoleAssistant, Content: "nunca deveria rodar"}}},
		}, nil
	}

	reg := NewAgentRegistry()
	eng := NewAgentEngine(
		reg,
		NewContextBuilder(reg, DefaultMaxTokens),
		NewRouter(reg),
		provider,
		nil,
		EngineConfig{
			Model:       "test",
			MaxTurns:    10,
			HaltChecker: haltedChecker{},
		},
		nil, nil, nil,
	)

	_, err := eng.Run(context.Background(), "faz alguma coisa", nil)
	if err != nil {
		// O engine pode reportar resultado sem erro (buildResult) — o que
		// importa é que o LLM não foi chamado.
		t.Logf("Run returned error: %v", err)
	}

	if got := atomic.LoadInt64(&llmCalls); got != 0 {
		t.Fatalf("provider chamado %d vezes, esperava 0 (kill-switch deve bloquear toda chamada LLM)", got)
	}
}

// TestEngineHaltChecker_NoChecker_Unchanged: sem HaltChecker (nil), o
// comportamento é o atual — o provider é chamado.
func TestEngineHaltChecker_NoChecker_Unchanged(t *testing.T) {
	var llmCalls int64

	provider := newMockProvider()
	provider.chatFunc = func(_ context.Context, _ []chat.Message, _ chat.ChatOptions) (*chat.ChatResponse, error) {
		atomic.AddInt64(&llmCalls, 1)
		return &chat.ChatResponse{
			Choices: []chat.Choice{{Message: chat.Message{Role: chat.RoleAssistant, Content: "ok"}}},
		}, nil
	}

	reg := NewAgentRegistry()
	eng := NewAgentEngine(
		reg,
		NewContextBuilder(reg, DefaultMaxTokens),
		NewRouter(reg),
		provider,
		nil,
		EngineConfig{Model: "test", MaxTurns: 2},
		nil, nil, nil,
	)

	_, _ = eng.Run(context.Background(), "oi", nil)
	if got := atomic.LoadInt64(&llmCalls); got == 0 {
		t.Fatal("sem HaltChecker o provider deveria ser chamado (comportamento atual preservado)")
	}
}
