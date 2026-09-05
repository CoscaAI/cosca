package cli

import (
	"os"
	"testing"

	"github.com/CoscaAI/cosca/internal/config"
)

// setCliEnv sets an env var for the lifetime of a test and restores the previous
// value on cleanup. Process-wide env vars are shared across tests, so state
// must be restored to avoid leaking into sibling tests.
func setCliEnv(t *testing.T, key, value string) {
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

// TestLoadChatEnv_DeepSeek_PropagatesConfig prova que o bloco deepseek de
// loadChatEnv propaga model/base_url/API key do config para as env vars que o
// factory (chat/provider/register_chat.go) lê, permitindo o cosca run usar o
// provider declarado no .cosca/config.yaml.
func TestLoadChatEnv_DeepSeek_PropagatesConfig(t *testing.T) {
	setCliEnv(t, "COSCA_DEEPSEEK_MODEL", "")
	setCliEnv(t, "DEEPSEEK_BASE_URL", "")
	setCliEnv(t, "DEEPSEEK_API_KEY", "")

	cfg := &config.Config{Provider: config.ProviderConfig{
		Name:    "deepseek",
		Model:   "deepseek-v4-flash",
		BaseURL: "https://api.deepseek.com/v1",
		// A chave já foi resolvida pelo config.Load() a partir de api_key_env.
		APIKey: "sk-resolved-from-env",
	}}

	loadChatEnv(cfg)

	if got := os.Getenv("COSCA_DEEPSEEK_MODEL"); got != "deepseek-v4-flash" {
		t.Errorf("COSCA_DEEPSEEK_MODEL = %q, want %q", got, "deepseek-v4-flash")
	}
	if got := os.Getenv("DEEPSEEK_BASE_URL"); got != "https://api.deepseek.com/v1" {
		t.Errorf("DEEPSEEK_BASE_URL = %q, want %q", got, "https://api.deepseek.com/v1")
	}
	if got := os.Getenv("DEEPSEEK_API_KEY"); got != "sk-resolved-from-env" {
		t.Errorf("DEEPSEEK_API_KEY = %q, want %q", got, "sk-resolved-from-env")
	}
}

// TestLoadChatEnv_DeepSeek_FailClosedNoConfigKey prova a borda fail-closed:
// quando o config não tem APIKey carregada (ex.: o usuário seta a chave só no
// env externo, via api_key_env que ainda não resolveu), loadChatEnv NÃO
// injeta DEEPSEEK_API_KEY — a chave deve vir do ambiente, nunca da config
// vazia. Sem chave, o factory deepseek recusa (fail-closed).
func TestLoadChatEnv_DeepSeek_FailClosedNoConfigKey(t *testing.T) {
	setCliEnv(t, "COSCA_DEEPSEEK_MODEL", "")
	setCliEnv(t, "DEEPSEEK_BASE_URL", "")
	setCliEnv(t, "DEEPSEEK_API_KEY", "")

	cfg := &config.Config{Provider: config.ProviderConfig{
		Name:    "deepseek",
		Model:   "deepseek-v4-flash",
		BaseURL: "https://api.deepseek.com/v1",
		// APIKey vazio: não há chave para propagar (fail-closed).
		APIKey: "",
	}}

	loadChatEnv(cfg)

	if got := os.Getenv("COSCA_DEEPSEEK_MODEL"); got != "deepseek-v4-flash" {
		t.Errorf("COSCA_DEEPSEEK_MODEL = %q, want %q", got, "deepseek-v4-flash")
	}
	if got := os.Getenv("DEEPSEEK_BASE_URL"); got != "https://api.deepseek.com/v1" {
		t.Errorf("DEEPSEEK_BASE_URL = %q, want %q", got, "https://api.deepseek.com/v1")
	}
	// A chave NÃO deve ser injetada a partir de uma config vazia.
	if got := os.Getenv("DEEPSEEK_API_KEY"); got != "" {
		t.Errorf("DEEPSEEK_API_KEY = %q, want empty (fail-closed: no key propagated)", got)
	}
}
