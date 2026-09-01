package orchestration

// halt_test.go — KILL-SWITCH do kernel no orchestration (Etapa 3b).
//
// Quando o HaltChecker (kernel.EmergencyManager) reporta halted, o Executor
// NUNCA chama o provider — nenhuma chamada LLM acontece no caminho do serve.

import (
	"context"
	"testing"

	"github.com/CoscaAI/cosca/internal/chat"
)

// haltedChecker é um HaltChecker de teste que sempre reporta halted=true.
type haltedChecker struct{}

func (haltedChecker) IsHalted() bool { return true }

// TestExecutorHaltChecker_BlocksLLMCall: com o kill-switch acionado, o
// executor retorna erro sem chamar o provider (chatCalled = 0).
func TestExecutorHaltChecker_BlocksLLMCall(t *testing.T) {
	provider := newMockChatProvider("test", "test-model")
	cfg := DefaultExecutorConfig()
	cfg.HaltChecker = haltedChecker{}

	ex := NewExecutor(provider, cfg, nil)
	_, err := ex.chatWithRetry(context.Background(), nil, chat.DefaultChatOptions())

	if err == nil {
		t.Fatal("esperava erro quando o kernel está haltado")
	}
	if got := provider.chatCalled.Load(); got != 0 {
		t.Fatalf("provider chamado %d vezes, esperava 0 (kill-switch deve bloquear toda chamada LLM)", got)
	}
}

// TestExecutorHaltChecker_NoChecker_Unchanged: sem HaltChecker (nil), o
// comportamento é o atual — o provider é chamado.
func TestExecutorHaltChecker_NoChecker_Unchanged(t *testing.T) {
	provider := newMockChatProvider("test", "test-model")
	cfg := DefaultExecutorConfig()

	ex := NewExecutor(provider, cfg, nil)
	_, err := ex.chatWithRetry(context.Background(), nil, chat.DefaultChatOptions())
	if err != nil {
		t.Fatalf("sem kill-switch o chat deveria funcionar: %v", err)
	}
	if got := provider.chatCalled.Load(); got == 0 {
		t.Fatal("sem HaltChecker o provider deveria ser chamado (comportamento atual preservado)")
	}
}
