package provider

import (
	"context"
	"fmt"

	"github.com/CoscaAI/cosca/internal/chat"
)

// ─── Null Provider (deterministic mode) ─────────────────────────────────────

// NullProvider implements chat.Provider for the "none" provider: the platform
// running with no cognitive capability at all. It is a first-class, legitimate
// state — not an emergency fallback. Cosca can operate without any AI provider
// (deterministic mode) and still expose knowledge, laws, audit, and plan
// operations.
type NullProvider struct{}

// NewNullProvider creates a NullProvider. It needs no API key, model, or
// endpoint: the absence of a cognitive capability is itself a valid state, so
// the factory always succeeds.
func NewNullProvider() *NullProvider {
	return &NullProvider{}
}

// Name returns "none".
func (p *NullProvider) Name() string { return "none" }

// Models returns the single model identifier that represents the absence of a
// model. Kept non-empty so ProviderAdapter.Model() resolves to "none" instead
// of "unknown".
func (p *NullProvider) Models() []string { return []string{"none"} }

// IsAvailable reports that the provider is always available: running without
// cognitive capability is a valid operational state, not an outage.
func (p *NullProvider) IsAvailable() bool { return true }

// Chat returns a channel that emits exactly one ChatEventError with a clear,
// actionable message and then closes. There are no delta or done events: the
// caller should route to deterministic operations instead of expecting an
// answer.
func (p *NullProvider) Chat(ctx context.Context, req chat.ChatRequest) (<-chan chat.ChatEvent, error) {
	ch := make(chan chat.ChatEvent, 1)

	go func() {
		defer close(ch)
		ch <- chat.ChatEvent{
			Type: chat.ChatEventError,
			Error: fmt.Errorf(
				"capacidade cognitiva indisponível — rode com um provider (ollama, deepseek, etc.) ou consulte operações determinísticas (knowledge, laws, audit, plan)",
			),
		}
	}()

	return ch, nil
}

// Ensure compile-time interface satisfaction.
var _ chat.Provider = (*NullProvider)(nil)
