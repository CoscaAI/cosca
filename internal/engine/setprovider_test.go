package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSetProvider_TrocaProviderEmRuntime verifica que SetProvider troca o
// provider ativo do engine e do subagent spawner.
func TestSetProvider_TrocaProviderEmRuntime(t *testing.T) {
	_, _, _, firstProvider, _ := setupEngineTest(t)
	eng := buildTestEngine(t, firstProvider)

	// Provider inicial.
	assert.Equal(t, "test-provider", eng.Provider().Name())

	// Novo provider.
	second := newMockProvider()
	second.name = "openai"

	eng.SetProvider(second)

	assert.Equal(t, "openai", eng.Provider().Name(), "engine should use the new provider")
	assert.NotNil(t, eng.spawner, "spawner should exist")
	assert.Equal(t, "openai", eng.spawner.provider.Name(), "spawner should use the new provider")
	assert.Equal(t, "openai", eng.config.Model, "config model should reflect the new provider")
}

// TestSetProvider_NilIgnorado verifica que SetProvider(nil) não quebra nada.
func TestSetProvider_NilIgnorado(t *testing.T) {
	_, _, _, firstProvider, _ := setupEngineTest(t)
	eng := buildTestEngine(t, firstProvider)

	eng.SetProvider(nil)

	assert.Equal(t, "test-provider", eng.Provider().Name(), "nil provider should be ignored")
}

// TestSetProvider_SpawnerCompartilhaProvider verifica que o spawner e o engine
// apontam para o mesmo provider após a troca.
func TestSetProvider_SpawnerCompartilhaProvider(t *testing.T) {
	_, _, _, firstProvider, _ := setupEngineTest(t)
	eng := buildTestEngine(t, firstProvider)

	second := newMockProvider()
	second.name = "ollama"
	eng.SetProvider(second)

	assert.Equal(t, eng.Provider(), eng.spawner.provider, "engine and spawner should share the provider")
}

// buildTestEngine constrói um engine completo com o provider dado.
func buildTestEngine(t *testing.T, p ProviderChat) *AgentEngine {
	t.Helper()
	reg, ctxBldr, router, _, exec := setupEngineTest(t)

	cfg := EngineConfig{
		Model:    p.Name(),
		MaxTurns: 5,
	}

	return NewAgentEngine(
		reg,
		ctxBldr,
		router,
		p,
		exec,
		cfg,
		nil, // memoryRetriever
		nil, // memoryStorer
		nil, // knowledge
	)
}
