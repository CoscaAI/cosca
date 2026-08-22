package bedrock

import "testing"

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Model == "" {
		t.Fatal("default model empty")
	}
}

func TestNewRequiresCredentials(t *testing.T) {
	// Missing AWS credentials → error.
	if _, err := New(Config{}); err == nil {
		t.Fatal("New without credentials must error")
	}
}

func TestNewWithCredentials(t *testing.T) {
	p, err := New(Config{
		Region:          "us-east-1",
		AccessKeyID:     "test-access",
		SecretAccessKey: "test-secret",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if p.Name() != "bedrock" || p.Model() == "" {
		t.Fatalf("provider: %+v", p)
	}
	if p.Dimensions() <= 0 {
		t.Fatalf("dims = %d", p.Dimensions())
	}
	if err := p.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}
