// Package config_test provides tests for the Cosca Chat config loader,
// covering defaults, file I/O, environment variable expansion, and
// provider resolution.
package config_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/CoscaAI/cosca/internal/chat/config"
)

func TestSaveRejectsSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("criar symlink no Windows requer privilégio de admin/developer mode — não aplicável")
	}
	root := t.TempDir()
	dir := filepath.Join(root, ".cosca")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "target.yaml")
	if err := os.WriteFile(target, []byte("untouched"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(dir, "config.yaml")); err != nil {
		t.Fatal(err)
	}
	if err := config.Save(root, &config.Config{}); err == nil {
		t.Fatal("Save should reject symlink")
	}
	got, _ := os.ReadFile(target)
	if string(got) != "untouched" {
		t.Fatal("symlink target was modified")
	}
}

func TestSaveRejectsSymlinkRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("criar symlink no Windows requer privilégio de admin/developer mode — não aplicável")
	}
	root := filepath.Join(t.TempDir(), "root-link")
	if err := os.Symlink(t.TempDir(), root); err != nil {
		t.Fatal(err)
	}
	if err := config.Save(root, &config.Config{}); err == nil {
		t.Fatal("Save should reject a symlinked root")
	}
}

func TestSaveRejectsFilesystemRoot(t *testing.T) {
	if err := config.Save(string(filepath.Separator), &config.Config{}); err == nil {
		t.Fatal("Save should reject filesystem root")
	}
}

// ─── DefaultConfig ───────────────────────────────────────────────────────────

func TestDefaultConfig(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()

	if cfg.Version != "1" {
		t.Errorf("Version = %q, want %q", cfg.Version, "1")
	}
	if cfg.Provider.Primary != "deepseek" {
		t.Errorf("Provider.Primary = %q, want %q", cfg.Provider.Primary, "deepseek")
	}
	if cfg.Provider.DeepSeek.Model != "deepseek-v4-flash" {
		t.Errorf("Provider.DeepSeek.Model = %q, want %q", cfg.Provider.DeepSeek.Model, "deepseek-v4-flash")
	}
	if cfg.Provider.DeepSeek.ContextWindow != 200000 {
		t.Errorf("Provider.DeepSeek.ContextWindow = %d, want %d", cfg.Provider.DeepSeek.ContextWindow, 200000)
	}
	if cfg.Provider.OpenAI.Model != "" {
		t.Errorf("Provider.OpenAI.Model = %q, want empty", cfg.Provider.OpenAI.Model)
	}
	if cfg.Sandbox.Mode != "workspace" {
		t.Errorf("Sandbox.Mode = %q, want %q", cfg.Sandbox.Mode, "workspace")
	}
	if cfg.Session.AutoSave != true {
		t.Error("Session.AutoSave should be true")
	}
	if cfg.Session.MaxTurns != 100 {
		t.Errorf("Session.MaxTurns = %d, want %d", cfg.Session.MaxTurns, 100)
	}
}

// ─── Load: file not found → defaults ─────────────────────────────────────────

func TestLoad_FileNotFound(t *testing.T) {
	t.Parallel()

	// Use a temp directory with no .cosca/config.yaml
	tmpDir := t.TempDir()

	cfg, err := config.Load(tmpDir)
	if err != nil {
		t.Fatalf("Load() returned error when file not found: %v", err)
	}
	if cfg == nil {
		t.Fatal("Load() returned nil config")
	}
	// Should return the default config
	if cfg.Provider.Primary != "deepseek" {
		t.Errorf("expected default primary 'deepseek', got %q", cfg.Provider.Primary)
	}
}

// ─── Load: valid config file ─────────────────────────────────────────────────

func TestLoad_ValidConfig(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	coscaDir := filepath.Join(tmpDir, ".cosca")
	if err := os.MkdirAll(coscaDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	yamlContent := []byte(`
version: "2"
provider:
  primary: openai
  openai:
    api_key: sk-test-123
    model: gpt-4o
    base_url: https://api.openai.com/v1
sandbox:
  mode: read-only
session:
  auto_save: false
  max_turns: 50
`)
	if err := os.WriteFile(filepath.Join(coscaDir, "config.yaml"), yamlContent, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg, err := config.Load(tmpDir)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.Version != "2" {
		t.Errorf("Version = %q, want %q", cfg.Version, "2")
	}
	if cfg.Provider.Primary != "openai" {
		t.Errorf("Provider.Primary = %q, want %q", cfg.Provider.Primary, "openai")
	}
	if cfg.Provider.OpenAI.APIKey != "sk-test-123" {
		t.Errorf("Provider.OpenAI.APIKey = %q, want %q", cfg.Provider.OpenAI.APIKey, "sk-test-123")
	}
	if cfg.Provider.OpenAI.Model != "gpt-4o" {
		t.Errorf("Provider.OpenAI.Model = %q, want %q", cfg.Provider.OpenAI.Model, "gpt-4o")
	}
	if cfg.Sandbox.Mode != "read-only" {
		t.Errorf("Sandbox.Mode = %q, want %q", cfg.Sandbox.Mode, "read-only")
	}
	if cfg.Session.AutoSave != false {
		t.Error("Session.AutoSave should be false")
	}
	if cfg.Session.MaxTurns != 50 {
		t.Errorf("Session.MaxTurns = %d, want %d", cfg.Session.MaxTurns, 50)
	}
}

// ─── Load: environment variable expansion ────────────────────────────────────

func TestLoad_EnvVarExpansion(t *testing.T) {
	// Not parallel — modifies environment variables.
	tmpDir := t.TempDir()
	coscaDir := filepath.Join(tmpDir, ".cosca")
	if err := os.MkdirAll(coscaDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// Set env vars to be expanded
	t.Setenv("TEST_API_KEY", "sk-env-test-456")
	t.Setenv("TEST_MODEL", "gpt-4o-mini")

	yamlContent := []byte(`
provider:
  primary: openai
  openai:
    api_key: ${TEST_API_KEY}
    model: $TEST_MODEL
    base_url: https://api.openai.com/v1
`)
	if err := os.WriteFile(filepath.Join(coscaDir, "config.yaml"), yamlContent, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg, err := config.Load(tmpDir)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.Provider.OpenAI.APIKey != "sk-env-test-456" {
		t.Errorf("APIKey after expansion = %q, want %q", cfg.Provider.OpenAI.APIKey, "sk-env-test-456")
	}
	if cfg.Provider.OpenAI.Model != "gpt-4o-mini" {
		t.Errorf("Model after expansion = %q, want %q", cfg.Provider.OpenAI.Model, "gpt-4o-mini")
	}
}

// ─── Load: env var expansion with undefined variable ─────────────────────────

func TestLoad_EnvVarExpansion_Undefined(t *testing.T) {
	// Isolate from the machine's shared global config (~/.config/cosca/config.yaml).
	t.Setenv("COSCA_GLOBAL_CONFIG", filepath.Join(t.TempDir(), "no-global-config.yaml"))

	tmpDir := t.TempDir()
	coscaDir := filepath.Join(tmpDir, ".cosca")
	if err := os.MkdirAll(coscaDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// Use an undefined variable — should remain as-is (or expand to empty).
	yamlContent := []byte(`
provider:
  primary: deepseek
  deepseek:
    api_key: ${UNDEFINED_VAR}
    model: deepseek-chat
`)
	if err := os.WriteFile(filepath.Join(coscaDir, "config.yaml"), yamlContent, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg, err := config.Load(tmpDir)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	// os.Expand replaces undefined variables with empty string.
	if cfg.Provider.DeepSeek.APIKey != "" {
		t.Errorf("APIKey for undefined var = %q, want empty", cfg.Provider.DeepSeek.APIKey)
	}
}

// ─── Save: creates file ──────────────────────────────────────────────────────

func TestSave_CreatesFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.Provider.Primary = "openai"

	if err := config.Save(tmpDir, &cfg); err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	// Verify the file was created.
	configPath := filepath.Join(tmpDir, ".cosca", "config.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("config.yaml was not created by Save()")
	}

	// Verify it's a valid YAML by loading it back.
	loaded, err := config.Load(tmpDir)
	if err != nil {
		t.Fatalf("Load after Save failed: %v", err)
	}
	if loaded.Provider.Primary != "openai" {
		t.Errorf("Provider.Primary after round-trip = %q, want %q", loaded.Provider.Primary, "openai")
	}
}

// ─── Save: nil config is a no-op ─────────────────────────────────────────────

func TestSave_NilConfig(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	if err := config.Save(tmpDir, nil); err != nil {
		t.Fatalf("Save(nil) failed: %v", err)
	}

	// No .cosca directory should have been created.
	if _, err := os.Stat(filepath.Join(tmpDir, ".cosca")); !os.IsNotExist(err) {
		t.Error(".cosca directory was created despite nil config")
	}
}

// ─── Save → Load round-trip ──────────────────────────────────────────────────

func TestSave_RoundTrip(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	original := config.Config{
		Version: "3",
		Provider: config.ProviderConf{
			Primary: "anthropic",
			Anthropic: config.ProviderItem{
				APIKey:  "sk-ant-test",
				Model:   "claude-sonnet-4-20250514",
				BaseURL: "https://api.anthropic.com",
			},
			Ollama: config.OllamaItem{
				Model:   "llama3",
				BaseURL: "http://localhost:11434",
			},
		},
		Sandbox: config.SandboxConf{
			Mode: "full",
		},
		Session: config.SessionConf{
			AutoSave: true,
			MaxTurns: 200,
		},
	}

	if err := config.Save(tmpDir, &original); err != nil {
		t.Fatalf("Save() failed: %v", err)
	}
	saved, err := os.ReadFile(filepath.Join(tmpDir, ".cosca", "config.yaml"))
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}
	if string(saved) == "" || contains(string(saved), "sk-ant-test") {
		t.Fatal("project config must not contain plaintext provider credentials")
	}

	loaded, err := config.Load(tmpDir)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if loaded.Version != original.Version {
		t.Errorf("Version = %q, want %q", loaded.Version, original.Version)
	}
	if loaded.Provider.Primary != original.Provider.Primary {
		t.Errorf("Provider.Primary = %q, want %q", loaded.Provider.Primary, original.Provider.Primary)
	}
	if loaded.Provider.Anthropic.APIKey != "" {
		t.Error("Anthropic.APIKey must not be restored from project config without its external environment")
	}
	if loaded.Provider.Anthropic.APIKeyEnv != "ANTHROPIC_API_KEY" {
		t.Errorf("Anthropic.APIKeyEnv = %q, want external reference", loaded.Provider.Anthropic.APIKeyEnv)
	}
	if loaded.Provider.Anthropic.Model != original.Provider.Anthropic.Model {
		t.Errorf("Anthropic.Model = %q, want %q", loaded.Provider.Anthropic.Model, original.Provider.Anthropic.Model)
	}
	if loaded.Provider.Ollama.Model != original.Provider.Ollama.Model {
		t.Errorf("Ollama.Model = %q, want %q", loaded.Provider.Ollama.Model, original.Provider.Ollama.Model)
	}
	if loaded.Sandbox.Mode != original.Sandbox.Mode {
		t.Errorf("Sandbox.Mode = %q, want %q", loaded.Sandbox.Mode, original.Sandbox.Mode)
	}
	if loaded.Session.AutoSave != original.Session.AutoSave {
		t.Errorf("Session.AutoSave = %v, want %v", loaded.Session.AutoSave, original.Session.AutoSave)
	}
	if loaded.Session.MaxTurns != original.Session.MaxTurns {
		t.Errorf("Session.MaxTurns = %d, want %d", loaded.Session.MaxTurns, original.Session.MaxTurns)
	}
}

func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ─── ResolveProviderConfig ───────────────────────────────────────────────────

func TestResolveProviderConfig_Primary(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	cfg.Provider.Primary = "anthropic"
	cfg.Provider.Anthropic = config.ProviderItem{APIKey: "sk-ant-789", Model: "claude-opus-4-20250514"}

	// Resolve "primary" returns the primary provider (anthropic).
	resolved := cfg.ResolveProviderConfig("primary")
	if resolved == nil {
		t.Fatal("ResolveProviderConfig('primary') returned nil")
	}
	if resolved.APIKey != "sk-ant-789" {
		t.Errorf("APIKey = %q, want %q", resolved.APIKey, "sk-ant-789")
	}
	if resolved.Model != "claude-opus-4-20250514" {
		t.Errorf("Model = %q, want %q", resolved.Model, "claude-opus-4-20250514")
	}

	// Resolve empty string also returns primary.
	resolvedEmpty := cfg.ResolveProviderConfig("")
	if resolvedEmpty == nil {
		t.Fatal("ResolveProviderConfig('') returned nil")
	}
	if resolvedEmpty.APIKey != "sk-ant-789" {
		t.Errorf("APIKey = %q, want %q", resolvedEmpty.APIKey, "sk-ant-789")
	}
}

func TestResolveProviderConfig_ByName(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	cfg.Provider.OpenAI = config.ProviderItem{APIKey: "sk-openai-111", Model: "gpt-4o"}
	cfg.Provider.DeepSeek = config.ProviderItem{APIKey: "sk-deep-222", Model: "deepseek-chat"}
	cfg.Provider.Anthropic = config.ProviderItem{APIKey: "sk-ant-333", Model: "claude-sonnet-4-20250514"}

	tests := []struct {
		name    string
		wantKey string
		wantMod string
	}{
		{"openai", "sk-openai-111", "gpt-4o"},
		{"deepseek", "sk-deep-222", "deepseek-chat"},
		{"anthropic", "sk-ant-333", "claude-sonnet-4-20250514"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			resolved := cfg.ResolveProviderConfig(tc.name)
			if resolved == nil {
				t.Fatalf("ResolveProviderConfig(%q) returned nil", tc.name)
			}
			if resolved.APIKey != tc.wantKey {
				t.Errorf("APIKey = %q, want %q", resolved.APIKey, tc.wantKey)
			}
			if resolved.Model != tc.wantMod {
				t.Errorf("Model = %q, want %q", resolved.Model, tc.wantMod)
			}
		})
	}
}

func TestResolveProviderConfig_NotFound(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()

	// Unknown provider name returns nil.
	if resolved := cfg.ResolveProviderConfig("nonexistent"); resolved != nil {
		t.Errorf("expected nil for unknown provider, got %+v", resolved)
	}

	// Ollama is not a ProviderItem (it's OllamaItem), so it should return nil.
	if resolved := cfg.ResolveProviderConfig("ollama"); resolved != nil {
		t.Errorf("expected nil for ollama (incompatible type), got %+v", resolved)
	}
}

// ─── GetValue / getByKey ──────────────────────────────────────────────────────

func TestGetValue_ValidKeys(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cfg := &config.Config{
		Version: "1",
		Provider: config.ProviderConf{
			Primary: "deepseek",
			DeepSeek: config.ProviderItem{
				APIKey: "sk-ds-test",
				Model:  "deepseek-chat",
			},
			OpenAI: config.ProviderItem{
				APIKey:  "sk-oi-test",
				Model:   "gpt-4o",
				BaseURL: "https://api.openai.com/v1",
			},
			Anthropic: config.ProviderItem{
				APIKey:  "sk-ant-test",
				Model:   "claude-sonnet-4-20250514",
				BaseURL: "https://api.anthropic.com",
			},
			Ollama: config.OllamaItem{
				Model:   "llama3",
				BaseURL: "http://localhost:11434",
			},
		},
		Sandbox: config.SandboxConf{
			Mode: "workspace",
		},
		Session: config.SessionConf{
			MaxTurns: 50,
			AutoSave: false,
		},
	}

	if err := config.Save(tmpDir, cfg); err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	tests := []struct {
		key  string
		want string
	}{
		{"providers.primary", "deepseek"},
		{"providers.deepseek.api_key", "(configured; value redacted)"},
		{"providers.deepseek.model", "deepseek-chat"},
		{"providers.openai.api_key", "(configured; value redacted)"},
		{"providers.openai.model", "gpt-4o"},
		{"providers.openai.base_url", "https://api.openai.com/v1"},
		{"providers.anthropic.api_key", "(configured; value redacted)"},
		{"providers.anthropic.model", "claude-sonnet-4-20250514"},
		{"providers.anthropic.base_url", "https://api.anthropic.com"},
		{"providers.ollama.model", "llama3"},
		{"providers.ollama.base_url", "http://localhost:11434"},
		{"sandbox.mode", "workspace"},
		{"session.max_turns", "50"},
		{"session.auto_save", "false"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.key, func(t *testing.T) {
			got, err := config.GetValue(tmpDir, tc.key)
			if err != nil {
				t.Fatalf("GetValue(%q) failed: %v", tc.key, err)
			}
			if got != tc.want {
				t.Errorf("GetValue(%q) = %q, want %q", tc.key, got, tc.want)
			}
		})
	}
}

func TestGetValue_UnknownKey(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cfg := config.DefaultConfig()
	if err := config.Save(tmpDir, &cfg); err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	_, err := config.GetValue(tmpDir, "nonexistent.key")
	if err == nil {
		t.Fatal("expected error for unknown key, got nil")
	}
}

func TestGetValue_FileNotFound(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	_, err := config.GetValue(tmpDir, "providers.primary")
	if err != nil {
		// Getting a value when no config exists is fine — returns default.
		// But it should NOT error for a valid key.
		t.Fatalf("GetValue with no config file failed: %v", err)
	}
}

func TestGetValue_EmptyValues(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cfg := config.DefaultConfig()
	if err := config.Save(tmpDir, &cfg); err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	// Keys that were never set should return empty string, not an error.
	val, err := config.GetValue(tmpDir, "providers.openai.api_key")
	if err != nil {
		t.Fatalf("GetValue(unset key) failed: %v", err)
	}
	if val != "(not set)" {
		t.Errorf("expected redacted unset status, got %q", val)
	}
}
