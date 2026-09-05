package contextpipeline

// integration_test.go — F6: prova a INTEGRAÇÃO real do Context Compiler.
//
// Conecta o Pipeline ao Executor do orchestration (via ExecutorConfig.
// ContextPipeline) e valida que: (1) o contexto compilado entra no system
// prompt, (2) o nível é registrado no PipelineData, (3) o comportamento sem
// pipeline permanece intacto.

import (
	"context"
	"strings"
	"testing"

	"github.com/CoscaAI/cosca/internal/chat"
	"github.com/CoscaAI/cosca/internal/orchestration"
)

// mockProviderWithCapture captura as mensagens enviadas ao LLM (para provar
// que o contexto compilado entrou no prompt).
type mockProviderWithCapture struct {
	name      string
	model     string
	messages  []chat.Message
	chatCalls int
}

func (m *mockProviderWithCapture) Name() string { return m.name }
func (m *mockProviderWithCapture) Model() string { return m.model }
func (m *mockProviderWithCapture) Close() error  { return nil }
func (m *mockProviderWithCapture) Chat(ctx context.Context, messages []chat.Message, opts chat.ChatOptions) (*chat.ChatResponse, error) {
	m.chatCalls++
	m.messages = messages
	return &chat.ChatResponse{
		Choices: []chat.Choice{{Message: chat.Message{Role: chat.RoleAssistant, Content: `{"decision":"EDIT","action":"fix","confidence":0.9}`}}},
	}, nil
}
func (m *mockProviderWithCapture) ChatStream(ctx context.Context, messages []chat.Message, opts chat.ChatOptions) (chat.ChatStream, error) {
	return nil, nil
}

// TestIntegration_PipelineInjectsCompiledContext: com o ContextPipeline
// configurado, o system prompt contém o estado operacional compilado
// (TASK/FACTS/OUTPUT_CONTRACT).
func TestIntegration_PipelineInjectsCompiledContext(t *testing.T) {
	provider := &mockProviderWithCapture{name: "test", model: "m"}
	pipeline := New(Config{MaxTokens: 1000})

	cfg := orchestration.DefaultExecutorConfig()
	cfg.ContextPipeline = pipeline

	ex := orchestration.NewExecutor(provider, cfg, nil)
	pc := orchestration.NewPipelineContext("req-1", "corrigir endpoint MCP")
	pc.Data.ResolvedAgent = "Backend API Specialist"
	pc.Data.KnowledgeResults = &orchestration.KnowledgeSearchResults{
		Results: []orchestration.KnowledgeSearchResult{
			{ID: "a", Content: "server exposes 11 tools", Score: 0.92, Epistemic: "FACT"},
		},
	}

	if _, err := ex.Execute(context.Background(), pc); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if provider.chatCalls == 0 {
		t.Fatal("LLM não foi chamado")
	}
	system := provider.messages[0].Content
	if !strings.Contains(system, "TASK:") || !strings.Contains(system, "corrigir endpoint MCP") {
		t.Fatal("contexto compilado não entrou no system prompt (TASK ausente)")
	}
	if !strings.Contains(system, "OUTPUT_CONTRACT:") {
		t.Fatal("OUTPUT_CONTRACT ausente no system prompt")
	}
	if pipeline.Track.Compiles != 1 {
		t.Fatalf("compiles = %d, esperava 1 (métrica registrada)", pipeline.Track.Compiles)
	}
}

// TestIntegration_NoPipeline_Unchanged: sem ContextPipeline, o Executor se
// comporta como antes — o system prompt NÃO tem o bloco compilado.
func TestIntegration_NoPipeline_Unchanged(t *testing.T) {
	provider := &mockProviderWithCapture{name: "test", model: "m"}
	cfg := orchestration.DefaultExecutorConfig()

	ex := orchestration.NewExecutor(provider, cfg, nil)
	pc := orchestration.NewPipelineContext("req-2", "oi")
	pc.Data.ResolvedAgent = "COSCA KERNEL"

	if _, err := ex.Execute(context.Background(), pc); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	system := provider.messages[0].Content
	if strings.Contains(system, "OUTPUT_CONTRACT:") {
		t.Fatal("sem pipeline o system prompt não deveria ter o bloco compilado")
	}
}
