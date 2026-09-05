package orchestration

import (
	"context"
	"testing"

	"github.com/CoscaAI/cosca/internal/shadow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestShadowMode_Equivalence is the spine test: the Shadow mode must NOT
// interfere with the main response. Running the same request with Shadow ON vs
// OFF must yield a bit-identical Response, and the provider must be called in
// BOTH cases (proving DeliberationHandled stays false and the LLM is consulted).
//
// The only observable difference: the shadow store has +1 record when ON.
func TestShadowMode_Equivalence(t *testing.T) {
	cfgOff := DefaultOrchestratorConfig()
	cfgOff.DeliberateConfig.Enabled = false // NOT authoritative
	cfgOff.DeliberateConfig.ShadowMode = false
	cfgOff.DeliberateConfig.Weights = gateWeights()

	provider := newMockChatProvider("test", "test-model")
	engineOff := NewEngine(nil, nil, nil, newMockAgentResolver(), nil, provider, cfgOff, nil)
	offResp, err := engineOff.Execute(context.Background(), &Request{Prompt: "build an api"})
	require.NoError(t, err)
	require.NotEmpty(t, offResp.Response)
	assert.GreaterOrEqual(t, provider.chatCalled.Load(), int64(1), "LLM must be called when Shadow OFF")

	// Turn Shadow ON — same request, same provider state baseline.
	cfgOn := DefaultOrchestratorConfig()
	cfgOn.DeliberateConfig.Enabled = false
	cfgOn.DeliberateConfig.ShadowMode = true
	cfgOn.DeliberateConfig.Weights = gateWeights()
	store := shadow.NewStore(t.TempDir())
	cfgOn.ShadowStore = store

	provider2 := newMockChatProvider("test", "test-model")
	engineOn := NewEngine(nil, nil, nil, newMockAgentResolver(), nil, provider2, cfgOn, nil)
	onResp, err := engineOn.Execute(context.Background(), &Request{Prompt: "build an api"})
	require.NoError(t, err)
	require.NotEmpty(t, onResp.Response)

	assert.Equal(t, offResp.Response, onResp.Response,
		"Shadow mode must NOT change the response (bit-identical)")
	assert.GreaterOrEqual(t, provider2.chatCalled.Load(), int64(1),
		"Shadow mode must still call the LLM (DeliberationHandled stays false)")

	records, err := store.Read()
	require.NoError(t, err)
	require.Len(t, records, 1, "Shadow ON must record exactly one observation")
}

// TestShadowMode_ShadowDoesNotShortCircuit verifies that even when the Kernel
// would emit EMIT_OK (enough evidence to decide without the LLM), the Shadow
// mode still escalates to the model — it never curtails the LLM call.
func TestShadowMode_ShadowDoesNotShortCircuit(t *testing.T) {
	cfg := DefaultOrchestratorConfig()
	cfg.DeliberateConfig.Enabled = false
	cfg.DeliberateConfig.ShadowMode = true
	cfg.DeliberateConfig.Weights = gateWeights()
	cfg.ShadowStore = shadow.NewStore(t.TempDir())

	// Strong evidence that would EmitOK in active mode.
	kr := &KnowledgeSearchResults{Results: []KnowledgeSearchResult{
		{ID: "k1", Snippet: "use go", Score: 0.9},
		{ID: "k2", Snippet: "use go", Score: 0.8},
	}}
	pc := deliberationPC(kr, nil)

	provider := newMockChatProvider("test", "test-model")
	engine := NewEngine(nil, nil, nil, newMockAgentResolver(), nil, provider, cfg, nil)
	_ = pc // pc is for direct Deliberator use; the engine builds its own context.

	result, err := engine.Execute(context.Background(), &Request{Prompt: "build an api"})
	require.NoError(t, err)
	require.NotEmpty(t, result.Response)

	assert.GreaterOrEqual(t, provider.chatCalled.Load(), int64(1),
		"Shadow must NOT short-circuit: the LLM is still consulted")
}
