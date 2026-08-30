package provider

import (
	"os"
	"testing"

	"github.com/CoscaAI/cosca/internal/chat"
)

// setEnv sets an env var for the lifetime of a test and restores the previous
// value (or unsets it) on cleanup. loadChatEnv and the provider factories read
// process-wide env vars, so env-sensitive tests must never run in parallel and
// must restore state to avoid leaking into sibling tests.
func setEnv(t *testing.T, key, value string) {
	t.Helper()
	orig, had := os.LookupEnv(key)
	if err := os.Setenv(key, value); err != nil {
		t.Fatalf("Setenv(%q) failed: %v", key, err)
	}
	t.Cleanup(func() {
		if had {
			_ = os.Setenv(key, orig)
		} else {
			_ = os.Unsetenv(key)
		}
	})
}

// TestRegisterChatProviders_FailClosedNoKey prova a LEI DO COFRE (fail-closed):
// sem DEEPSEEK_API_KEY o provider deepseek é registrado como candidato, mas o
// factory RECUSA instanciar; o Select() pula o deepseek e recua para um
// provider LOCAL. O Cosca nunca inicia um provider de nuvem sem chave.
func TestRegisterChatProviders_FailClosedNoKey(t *testing.T) {
	setEnv(t, "DEEPSEEK_API_KEY", "")

	reg := chat.NewChatRegistry()
	if err := RegisterChatProviders(reg, nil); err != nil {
		t.Fatalf("RegisterChatProviders() failed: %v", err)
	}

	// deepseek é registrado (candidato à auto-detecção), mas a instância criada
	// é nil: a factory recusou porque não há chave.
	if _, ok := reg.Get("deepseek"); !ok {
		t.Fatalf("registry should contain deepseek as a registered candidate")
	}
	if p, _ := reg.Get("deepseek"); p != nil {
		t.Fatalf("deepseek instantiated without a key (fail-closed violated): got %T", p)
	}

	// Auto-detection: deepseek é pulado e o Cosca recua para um provider local.
	cfg := chat.DefaultChatRegistryConfig() // AutoDetect = true
	if err := reg.Select(t.Context(), cfg); err != nil {
		t.Fatalf("Select() failed: %v", err)
	}
	got := reg.Name()
	if got == "deepseek" {
		t.Fatalf("Select() selected deepseek without a key (fail-closed violated)")
	}
	if got != "ollama" && got != "gpu" && got != "none" {
		t.Fatalf("Select() did not fall back to a local provider, got %q", got)
	}
}

// TestRegisterChatProviders_DeepSeekSelectableWithKey prova o caso positivo:
// com DEEPSEEK_API_KEY presente, o provider deepseek é registrado E
// selecionável como provider primário.
func TestRegisterChatProviders_DeepSeekSelectableWithKey(t *testing.T) {
	setEnv(t, "DEEPSEEK_API_KEY", "sk-test-non-secret")

	reg := chat.NewChatRegistry()
	if err := RegisterChatProviders(reg, nil); err != nil {
		t.Fatalf("RegisterChatProviders() failed: %v", err)
	}

	// A instância deve ser criada com a chave e reportar o nome "deepseek".
	p, ok := reg.Get("deepseek")
	if !ok {
		t.Fatalf("registry should contain deepseek")
	}
	if p == nil {
		t.Fatalf("deepseek provider could not be instantiated with a key")
	}
	if p.Name() != "deepseek" {
		t.Fatalf("provider Name() = %q, want %q", p.Name(), "deepseek")
	}

	// Auto-detection: gpu falha (sem fabric), deepseek (chave presente) é o
	// primário; ollama/none ficam como fallback.
	cfg := chat.DefaultChatRegistryConfig()
	if err := reg.Select(t.Context(), cfg); err != nil {
		t.Fatalf("Select() failed: %v", err)
	}
	if got := reg.Name(); got != "deepseek" {
		t.Fatalf("Select() should have selected deepseek, got %q", got)
	}
}

// TestSelect_FallbacksToLocal verifica que um primary externo pedido na config
// (openai, não registrado) nunca é selecionado; sem chave o deepseek também é
// pulado (fail-closed), e o Select() recua para um provider local.
func TestSelect_FallbacksToLocal(t *testing.T) {
	setEnv(t, "DEEPSEEK_API_KEY", "")

	reg := chat.NewChatRegistry()
	if err := RegisterChatProviders(reg, nil); err != nil {
		t.Fatalf("RegisterChatProviders() failed: %v", err)
	}

	// Config pedindo um provider externo que não está registrado.
	cfg := chat.DefaultChatRegistryConfig()
	cfg.Primary = "openai"

	if err := reg.Select(t.Context(), cfg); err != nil {
		t.Logf("Select() rejected external primary (expected): %v", err)
	}

	// Garantia crítica: o provider selecionado NUNCA é um externo.
	if got := reg.Name(); got == "openai" || got == "deepseek" || got == "anthropic" {
		t.Fatalf("Select() selected external provider %q (fail-closed violated)", got)
	}
}
