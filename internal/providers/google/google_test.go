package google

import "testing"

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Model == "" {
		t.Fatal("default model empty")
	}
}

func TestConfigFromEnv(t *testing.T) {
	t.Setenv("GOOGLE_API_KEY", "test-key")
	cfg := ConfigFromEnv()
	if cfg.APIKey != "test-key" {
		t.Fatalf("key = %q", cfg.APIKey)
	}
}

func TestNewRequiresKey(t *testing.T) {
	if _, err := New(Config{}); err == nil {
		t.Fatal("New without key must error")
	}
}

func TestNewWithKey(t *testing.T) {
	p, err := New(Config{APIKey: "test-key"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if p.Name() != "google" || p.Model() == "" {
		t.Fatalf("provider: %+v", p)
	}
	if p.Dimensions() <= 0 {
		t.Fatalf("dims = %d", p.Dimensions())
	}
	if err := p.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}
