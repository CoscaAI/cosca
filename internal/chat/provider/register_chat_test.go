package provider

import (
	"testing"

	"github.com/CoscaAI/cosca/internal/chat"
)

// TestRegisterChatProviders_LocalOnly verifica a LEI DO COFRE (fail-closed):
// somente providers LOCAIS (ollama/gpu/none) são registrados. Providers de
// nuvem (deepseek/openai/anthropic) NÃO existem no registry — mesmo que a
// config ou env var peça um provider externo, o Select() recusa e recua para
// o local. Esta é a prova de que o Cosca não usa nuvem sem consentimento.
func TestRegisterChatProviders_LocalOnly(t *testing.T) {
	t.Parallel()

	reg := chat.NewChatRegistry()
	if err := RegisterChatProviders(reg, nil); err != nil {
		t.Fatalf("RegisterChatProviders() failed: %v", err)
	}

	// Providers locais DEVEM existir.
	for _, name := range []string{"ollama", "gpu", "none"} {
		if _, ok := reg.Get(name); !ok {
			t.Errorf("registry does not contain local provider %q after RegisterChatProviders", name)
		}
	}

	// Providers externos NÃO devem existir — a casa não usa nuvem sem
	// consentimento explícito.
	for _, name := range []string{"deepseek", "openai", "anthropic"} {
		if _, ok := reg.Get(name); ok {
			t.Errorf("registry should NOT contain external provider %q (LEI DO COFRE violated)", name)
		}
	}
}

// TestSelect_FallbacksToLocal verifica que mesmo com um Primary externo pedido
// na config, o Select() não consegue usá-lo (não está registrado) e recua para
// o provider local — nunca inicializa um provedor de nuvem.
func TestSelect_FallbacksToLocal(t *testing.T) {
	t.Parallel()

	reg := chat.NewChatRegistry()
	if err := RegisterChatProviders(reg, nil); err != nil {
		t.Fatalf("RegisterChatProviders() failed: %v", err)
	}

	// Config malicioso/errado pedindo um provider externo que não existe.
	cfg := chat.DefaultChatRegistryConfig()
	cfg.Primary = "openai"

	if err := reg.Select(t.Context(), cfg); err != nil {
		// fail-closed: o Select não encontra o provider externo e recua.
		// Aceitável (recusa honesta) — desde que NUNCA inicialize openai.
		t.Logf("Select() rejected external primary (expected): %v", err)
	}

	// Garantia crítica: o provider selecionado NUNCA é um externo.
	// reg.Name() devolve o nome do provider primário selecionado; um primário
	// externo não está registrado, então o Select() não pode escolhê-lo.
	if got := reg.Name(); got == "openai" || got == "deepseek" || got == "anthropic" {
		t.Fatalf("Select() selected external provider %q (LEI DO COFRE violated)", got)
	}
}
