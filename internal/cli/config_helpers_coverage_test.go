//
// Config helper coverage tests — targets applyKeyToConfig (0%),
// setProviderEnvFromKey (0%), and getConfigDir (66.7% → 100%).
//

package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/CoscaAI/cosca/internal/config"
)

// =============================================================================
// applyKeyToConfig — test all key mappings
// =============================================================================

func TestApplyKeyToConfig_ProviderName(t *testing.T) {
	cfg := config.DefaultConfig()
	applyKeyToConfig(cfg, "provider.name", "openai")
	if cfg.Provider.Name != "openai" {
		t.Errorf("Provider.Name = %q, want %q", cfg.Provider.Name, "openai")
	}
}

func TestApplyKeyToConfig_ProviderAPIKey(t *testing.T) {
	cfg := config.DefaultConfig()
	applyKeyToConfig(cfg, "provider.api_key", "sk-test-key")
	if cfg.Provider.APIKey != "sk-test-key" {
		t.Errorf("Provider.APIKey = %q, want %q", cfg.Provider.APIKey, "sk-test-key")
	}
}

func TestApplyKeyToConfig_ProviderModel(t *testing.T) {
	cfg := config.DefaultConfig()
	applyKeyToConfig(cfg, "provider.model", "gpt-4")
	if cfg.Provider.Model != "gpt-4" {
		t.Errorf("Provider.Model = %q, want %q", cfg.Provider.Model, "gpt-4")
	}
}

func TestApplyKeyToConfig_ProviderBaseURL(t *testing.T) {
	cfg := config.DefaultConfig()
	applyKeyToConfig(cfg, "provider.base_url", "https://api.example.com")
	if cfg.Provider.BaseURL != "https://api.example.com" {
		t.Errorf("Provider.BaseURL = %q, want %q", cfg.Provider.BaseURL, "https://api.example.com")
	}
}

func TestApplyKeyToConfig_ProvidersDeepSeekAPIKey(t *testing.T) {
	cfg := config.DefaultConfig()
	applyKeyToConfig(cfg, "providers.deepseek.api_key", "sk-deepseek-key")
	if cfg.Provider.APIKey != "sk-deepseek-key" {
		t.Errorf("Provider.APIKey = %q, want %q", cfg.Provider.APIKey, "sk-deepseek-key")
	}
	if cfg.Provider.Name != "deepseek" {
		t.Errorf("Provider.Name = %q, want %q", cfg.Provider.Name, "deepseek")
	}
}

func TestApplyKeyToConfig_ProvidersDeepSeekModel(t *testing.T) {
	cfg := config.DefaultConfig()
	applyKeyToConfig(cfg, "providers.deepseek.model", "deepseek-chat")
	if cfg.Provider.Model != "deepseek-chat" {
		t.Errorf("Provider.Model = %q, want %q", cfg.Provider.Model, "deepseek-chat")
	}
}

func TestApplyKeyToConfig_ProvidersDeepSeekBaseURL(t *testing.T) {
	cfg := config.DefaultConfig()
	applyKeyToConfig(cfg, "providers.deepseek.base_url", "https://api.deepseek.com")
	if cfg.Provider.BaseURL != "https://api.deepseek.com" {
		t.Errorf("Provider.BaseURL = %q, want %q", cfg.Provider.BaseURL, "https://api.deepseek.com")
	}
}

func TestApplyKeyToConfig_ProvidersDeepSeekName(t *testing.T) {
	cfg := config.DefaultConfig()
	applyKeyToConfig(cfg, "providers.deepseek.name", "deepseek-v3")
	if cfg.Provider.Name != "deepseek-v3" {
		t.Errorf("Provider.Name = %q, want %q", cfg.Provider.Name, "deepseek-v3")
	}
}

func TestApplyKeyToConfig_ProvidersMaxTokens(t *testing.T) {
	cfg := config.DefaultConfig()
	applyKeyToConfig(cfg, "providers.deepseek.max_tokens", "4096")
	if cfg.Provider.MaxTokens != 4096 {
		t.Errorf("Provider.MaxTokens = %d, want 4096", cfg.Provider.MaxTokens)
	}
}

func TestApplyKeyToConfig_ProvidersMaxTokensInvalid(t *testing.T) {
	cfg := config.DefaultConfig()
	origTokens := cfg.Provider.MaxTokens
	applyKeyToConfig(cfg, "providers.deepseek.max_tokens", "not-a-number")
	if cfg.Provider.MaxTokens != origTokens {
		t.Errorf("MaxTokens changed when invalid value passed: %d", cfg.Provider.MaxTokens)
	}
}

func TestApplyKeyToConfig_ProvidersTemperature(t *testing.T) {
	cfg := config.DefaultConfig()
	applyKeyToConfig(cfg, "providers.deepseek.temperature", "0.7")
	if cfg.Provider.Temperature != 0.7 {
		t.Errorf("Provider.Temperature = %f, want 0.7", cfg.Provider.Temperature)
	}
}

func TestApplyKeyToConfig_ProvidersTemperatureInvalid(t *testing.T) {
	cfg := config.DefaultConfig()
	origTemp := cfg.Provider.Temperature
	applyKeyToConfig(cfg, "providers.deepseek.temperature", "not-a-float")
	if cfg.Provider.Temperature != origTemp {
		t.Errorf("Temperature changed when invalid value passed: %f", cfg.Provider.Temperature)
	}
}

func TestApplyKeyToConfig_ProvidersTimeout(t *testing.T) {
	cfg := config.DefaultConfig()
	applyKeyToConfig(cfg, "providers.deepseek.timeout", "30s")
	if cfg.Provider.Timeout.String() != "30s" {
		t.Errorf("Provider.Timeout = %q, want 30s", cfg.Provider.Timeout)
	}
}

func TestApplyKeyToConfig_ProvidersTimeoutInvalid(t *testing.T) {
	cfg := config.DefaultConfig()
	origTimeout := cfg.Provider.Timeout
	applyKeyToConfig(cfg, "providers.deepseek.timeout", "bad-duration")
	if cfg.Provider.Timeout != origTimeout {
		t.Errorf("Timeout changed when invalid value passed: %v", cfg.Provider.Timeout)
	}
}

func TestApplyKeyToConfig_ProvidersMaxRetries(t *testing.T) {
	cfg := config.DefaultConfig()
	applyKeyToConfig(cfg, "providers.deepseek.max_retries", "5")
	if cfg.Provider.MaxRetries != 5 {
		t.Errorf("Provider.MaxRetries = %d, want 5", cfg.Provider.MaxRetries)
	}
}

func TestApplyKeyToConfig_ProvidersMaxRetriesInvalid(t *testing.T) {
	cfg := config.DefaultConfig()
	origRetries := cfg.Provider.MaxRetries
	applyKeyToConfig(cfg, "providers.deepseek.max_retries", "invalid")
	if cfg.Provider.MaxRetries != origRetries {
		t.Errorf("MaxRetries changed when invalid value passed: %d", cfg.Provider.MaxRetries)
	}
}

func TestApplyKeyToConfig_ProvidersRateLimitPerMin(t *testing.T) {
	cfg := config.DefaultConfig()
	applyKeyToConfig(cfg, "providers.deepseek.rate_limit_per_min", "60")
	if cfg.Provider.RateLimitPerMin != 60 {
		t.Errorf("Provider.RateLimitPerMin = %d, want 60", cfg.Provider.RateLimitPerMin)
	}
}

func TestApplyKeyToConfig_ProvidersRateLimitPerMinInvalid(t *testing.T) {
	cfg := config.DefaultConfig()
	origLimit := cfg.Provider.RateLimitPerMin
	applyKeyToConfig(cfg, "providers.deepseek.rate_limit_per_min", "bad")
	if cfg.Provider.RateLimitPerMin != origLimit {
		t.Errorf("RateLimitPerMin changed when invalid value passed: %d", cfg.Provider.RateLimitPerMin)
	}
}

func TestApplyKeyToConfig_UnknownKey(t *testing.T) {
	cfg := config.DefaultConfig()
	orig := *cfg
	applyKeyToConfig(cfg, "unknown.key", "value")
	// Should not change anything; compare struct copies
	if cfg.Provider != orig.Provider {
		t.Error("unknown key should not modify provider config")
	}
}

func TestApplyKeyToConfig_ProvidersShortKey(t *testing.T) {
	cfg := config.DefaultConfig()
	orig := *cfg
	applyKeyToConfig(cfg, "providers", "value")
	if cfg.Provider != orig.Provider {
		t.Error("short providers key should not modify config")
	}
}

func TestApplyKeyToConfig_ProvidersTwoParts(t *testing.T) {
	cfg := config.DefaultConfig()
	applyKeyToConfig(cfg, "providers.deepseek", "value")
	// Only 2 parts after split — field switch is skipped
	_ = cfg // No panic is the assertion
}

// =============================================================================
// setProviderEnvFromKey
// =============================================================================

func TestSetProviderEnvFromKey_ValidAPIKey(t *testing.T) {
	// Arrange: clear env first
	os.Unsetenv("DEEPSEEK_API_KEY")

	// Act
	setProviderEnvFromKey("providers.deepseek.api_key", "sk-secret")

	// Assert
	if got := os.Getenv("DEEPSEEK_API_KEY"); got != "sk-secret" {
		t.Errorf("DEEPSEEK_API_KEY = %q, want %q", got, "sk-secret")
	}

	// Cleanup
	os.Unsetenv("DEEPSEEK_API_KEY")
}

func TestSetProviderEnvFromKey_NotAPIKey(t *testing.T) {
	os.Unsetenv("DEEPSEEK_MODEL")
	// Not an api_key suffix — should be no-op
	setProviderEnvFromKey("providers.deepseek.model", "gpt-4")
	if got := os.Getenv("DEEPSEEK_MODEL"); got != "" {
		t.Errorf("expected no env set for non-api_key, got: %q", got)
	}
}

func TestSetProviderEnvFromKey_NotProviderPrefix(t *testing.T) {
	os.Unsetenv("DEEPSEEK_API_KEY")
	// Not a providers.* key — should be no-op
	setProviderEnvFromKey("provider.api_key", "sk-key")
	if got := os.Getenv("DEEPSEEK_API_KEY"); got != "" {
		t.Errorf("expected no env set for non-providers prefix, got: %q", got)
	}
}

func TestSetProviderEnvFromKey_EmptyKey(t *testing.T) {
	// Empty key — no panic is the assertion
	setProviderEnvFromKey("", "value")
}

func TestSetProviderEnvFromKey_TwoParts(t *testing.T) {
	// Only 2 parts — should be no-op
	setProviderEnvFromKey("providers.api_key", "value")
}

func TestSetProviderEnvFromKey_EmptyProviderName(t *testing.T) {
	os.Unsetenv("_API_KEY")
	// 3 parts but middle is empty: "providers..api_key"
	setProviderEnvFromKey("providers..api_key", "value")
	if got := os.Getenv("_API_KEY"); got != "" {
		t.Errorf("expected no env set for empty provider name")
	}
}

func TestSetProviderEnvFromKey_OpenAI(t *testing.T) {
	os.Unsetenv("OPENAI_API_KEY")
	setProviderEnvFromKey("providers.openai.api_key", "sk-openai")
	if got := os.Getenv("OPENAI_API_KEY"); got != "sk-openai" {
		t.Errorf("OPENAI_API_KEY = %q, want 'sk-openai'", got)
	}
	os.Unsetenv("OPENAI_API_KEY")
}

// =============================================================================
// getConfigDir
// =============================================================================

func TestGetConfigDir_EnvVar(t *testing.T) {
	// Arrange
	os.Setenv("COSCA_PROJECT_DIR", "/custom/project/dir")
	defer os.Unsetenv("COSCA_PROJECT_DIR")

	// Act
	dir := getConfigDir()

	// Assert
	if dir != "/custom/project/dir" {
		t.Errorf("getConfigDir = %q, want /custom/project/dir", dir)
	}
}

func TestGetConfigDir_NoEnvVar(t *testing.T) {
	// Arrange: ensure env var is not set
	os.Unsetenv("COSCA_PROJECT_DIR")

	// Act
	dir := getConfigDir()

	// Assert: should return current working directory
	if dir == "" || dir == "." {
		t.Errorf("expected non-empty working dir, got: %q", dir)
	}
}

// =============================================================================
// configPath
// =============================================================================

func TestConfigPath(t *testing.T) {
	os.Unsetenv("COSCA_PROJECT_DIR")
	path := configPath()
	expected := filepath.Join(getConfigDir(), ".cosca", "config.yaml")
	if path != expected {
		t.Errorf("configPath = %q, want %q", path, expected)
	}
}
