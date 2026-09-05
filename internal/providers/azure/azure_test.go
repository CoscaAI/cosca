package azure

import (
	"strings"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Model == "" {
		t.Fatal("default model empty")
	}
}

func TestConfigFromEnv(t *testing.T) {
	t.Setenv("AZURE_OPENAI_API_KEY", "test-key")
	t.Setenv("AZURE_OPENAI_ENDPOINT", "https://cosca.openai.azure.com")
	t.Setenv("AZURE_OPENAI_DEPLOYMENT", "text-embedding-3-small")

	cfg := ConfigFromEnv()
	if cfg.APIKey != "test-key" || cfg.Endpoint != "https://cosca.openai.azure.com" || cfg.Deployment != "text-embedding-3-small" {
		t.Fatalf("cfg: %+v", cfg)
	}
}

func TestNewRequiresEnv(t *testing.T) {
	// Missing credentials → error.
	if _, err := New(Config{}); err == nil {
		t.Fatal("New without credentials must error")
	}

	// With all required config → ok.
	p, err := New(Config{
		APIKey:     "key",
		Endpoint:   "https://cosca.openai.azure.com",
		Deployment: "model",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if p.Name() != "azure" || p.Model() == "" {
		t.Fatalf("provider: %+v", p)
	}
	if p.Dimensions() <= 0 {
		t.Fatalf("dims = %d", p.Dimensions())
	}
	if err := p.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestNewRejectsPartialConfig(t *testing.T) {
	// Missing deployment.
	_, err := New(Config{APIKey: "k", Endpoint: "https://x"})
	if err == nil || !strings.Contains(err.Error(), "AZURE_OPENAI_DEPLOYMENT") {
		t.Fatalf("partial config error: %v", err)
	}
}
