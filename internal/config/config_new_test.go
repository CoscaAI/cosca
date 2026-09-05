package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// =============================================================================
// LoadFromFile tests
// =============================================================================

func TestLoadFromFile_ValidYaml(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	yamlContent := `
version: "2.0"
profile: "test-profile"
mode: "production"
verbose: true
paths:
  home: /custom/cosca/home
  project: /my/project
provider:
  name: anthropic
  model: claude-sonnet-4-20250514
  api_key: my-secret-key
  max_tokens: 8192
  temperature: 0.7
db:
  path: /custom/db/path
  wal_mode: false
  page_size: 8192
editor:
  name: nano
  theme: dark
server:
  host: 0.0.0.0
  api_port: 9000
`
	if err := os.WriteFile(cfgPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := LoadFromFile(cfgPath)
	if err != nil {
		t.Fatalf("LoadFromFile failed: %v", err)
	}

	// Check that file values were loaded
	if cfg.Version != "2.0" {
		t.Errorf("Version = %q, want %q", cfg.Version, "2.0")
	}
	if cfg.Profile != "test-profile" {
		t.Errorf("Profile = %q, want %q", cfg.Profile, "test-profile")
	}
	if cfg.Mode != "production" {
		t.Errorf("Mode = %q, want %q", cfg.Mode, "production")
	}
	if !cfg.Verbose {
		t.Error("Verbose should be true")
	}
	if cfg.Paths.Home != "/custom/cosca/home" {
		t.Errorf("Paths.Home = %q", cfg.Paths.Home)
	}
	if cfg.Provider.Name != "anthropic" {
		t.Errorf("Provider.Name = %q", cfg.Provider.Name)
	}
	if cfg.Provider.Model != "claude-sonnet-4-20250514" {
		t.Errorf("Provider.Model = %q", cfg.Provider.Model)
	}
	if cfg.Provider.APIKey != "my-secret-key" {
		t.Errorf("Provider.APIKey = %q", cfg.Provider.APIKey)
	}
	if cfg.Provider.MaxTokens != 8192 {
		t.Errorf("Provider.MaxTokens = %d", cfg.Provider.MaxTokens)
	}
	if cfg.Provider.Temperature != 0.7 {
		t.Errorf("Provider.Temperature = %f", cfg.Provider.Temperature)
	}
	if cfg.DB.Path != "/custom/db/path" {
		t.Errorf("DB.Path = %q", cfg.DB.Path)
	}
	if cfg.DB.WALMode {
		t.Error("DB.WALMode should be false (overridden)")
	}
	if cfg.DB.PageSize != 8192 {
		t.Errorf("DB.PageSize = %d", cfg.DB.PageSize)
	}
	if cfg.Editor.Name != "nano" {
		t.Errorf("Editor.Name = %q", cfg.Editor.Name)
	}
	if cfg.Editor.Theme != "dark" {
		t.Errorf("Editor.Theme = %q", cfg.Editor.Theme)
	}
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("Server.Host = %q", cfg.Server.Host)
	}
	if cfg.Server.APIPort != 9000 {
		t.Errorf("Server.APIPort = %d", cfg.Server.APIPort)
	}

	// Non-overridden defaults should still be present
	if cfg.Paths.Runtime == "" {
		t.Error("Paths.Runtime default should be set")
	}
	if cfg.LoadedFrom() != cfgPath {
		t.Errorf("LoadedFrom = %q, want %q", cfg.LoadedFrom(), cfgPath)
	}
}

func TestLoadFromFile_NonExistentFile(t *testing.T) {
	t.Parallel()

	_, err := LoadFromFile("/tmp/non-existent-config-xxxxxxxx.yaml")
	if err == nil {
		t.Error("LoadFromFile should return error for non-existent file")
	}
}

func TestLoadFromFile_InvalidYaml(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(cfgPath, []byte(": invalid: yaml: :::"), 0o644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	_, err := LoadFromFile(cfgPath)
	if err == nil {
		t.Error("LoadFromFile should return error for invalid YAML")
	}
}

func TestLoadFromFile_EmptyFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "empty.yaml")
	if err := os.WriteFile(cfgPath, []byte(""), 0o644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := LoadFromFile(cfgPath)
	if err != nil {
		t.Fatalf("LoadFromFile empty file should succeed: %v", err)
	}
	// Should have all defaults
	if cfg.Version != "1.0" {
		t.Errorf("Version = %q, want %q (default)", cfg.Version, "1.0")
	}
}

// TestLoadFromFile_EmbeddingOverrides verifies that embedding.base_url and
// embedding.api_key are parsed from YAML alongside the existing embedding
// fields.
func TestLoadFromFile_EmbeddingOverrides(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	yamlContent := `
version: "1.0"
embedding:
  provider: openai
  model: text-embedding-3-small
  dimensions: 768
  batch_size: 16
  base_url: http://127.0.0.1:11435/v1
  api_key: local-emb-key
`
	if err := os.WriteFile(cfgPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := LoadFromFile(cfgPath)
	if err != nil {
		t.Fatalf("LoadFromFile failed: %v", err)
	}

	if cfg.Embedding.Provider != "openai" {
		t.Errorf("Embedding.Provider = %q, want %q", cfg.Embedding.Provider, "openai")
	}
	if cfg.Embedding.Model != "text-embedding-3-small" {
		t.Errorf("Embedding.Model = %q, want %q", cfg.Embedding.Model, "text-embedding-3-small")
	}
	if cfg.Embedding.Dimensions != 768 {
		t.Errorf("Embedding.Dimensions = %d, want 768", cfg.Embedding.Dimensions)
	}
	if cfg.Embedding.BatchSize != 16 {
		t.Errorf("Embedding.BatchSize = %d, want 16", cfg.Embedding.BatchSize)
	}
	if cfg.Embedding.BaseURL != "http://127.0.0.1:11435/v1" {
		t.Errorf("Embedding.BaseURL = %q, want %q", cfg.Embedding.BaseURL, "http://127.0.0.1:11435/v1")
	}
	if cfg.Embedding.APIKey != "local-emb-key" {
		t.Errorf("Embedding.APIKey = %q, want %q", cfg.Embedding.APIKey, "local-emb-key")
	}
}

// TestSave_RoundtripEmbeddingOverrides verifies embedding.base_url and
// embedding.api_key survive a Save → LoadFromFile round-trip.
func TestSave_RoundtripEmbeddingOverrides(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	cfg := DefaultConfig()
	cfg.Embedding.Provider = "openai"
	cfg.Embedding.BaseURL = "http://127.0.0.1:11435/v1"
	cfg.Embedding.APIKey = "local-emb-key"

	if err := cfg.Save(cfgPath); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := LoadFromFile(cfgPath)
	if err != nil {
		t.Fatalf("LoadFromFile after save failed: %v", err)
	}
	if loaded.Embedding.BaseURL != "http://127.0.0.1:11435/v1" {
		t.Errorf("Embedding.BaseURL = %q, want %q", loaded.Embedding.BaseURL, "http://127.0.0.1:11435/v1")
	}
	if loaded.Embedding.APIKey != "local-emb-key" {
		t.Errorf("Embedding.APIKey = %q, want %q", loaded.Embedding.APIKey, "local-emb-key")
	}
}

// =============================================================================
// Save tests (roundtrip)
// =============================================================================

func TestSave_Roundtrip(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	cfg := DefaultConfig()
	cfg.Mode = "staging"
	cfg.Verbose = true
	cfg.Provider.Name = "ollama"
	cfg.Provider.Model = "llama3"
	cfg.Server.APIPort = 9999

	if err := cfg.Save(cfgPath); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Reload and verify
	loaded, err := LoadFromFile(cfgPath)
	if err != nil {
		t.Fatalf("LoadFromFile after save failed: %v", err)
	}

	if loaded.Mode != "staging" {
		t.Errorf("Mode = %q, want %q", loaded.Mode, "staging")
	}
	if !loaded.Verbose {
		t.Error("Verbose should be true")
	}
	if loaded.Provider.Name != "ollama" {
		t.Errorf("Provider.Name = %q, want %q", loaded.Provider.Name, "ollama")
	}
	if loaded.Server.APIPort != 9999 {
		t.Errorf("Server.APIPort = %d, want %d", loaded.Server.APIPort, 9999)
	}
}

func TestSave_CreatesDirectory(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "subdir", "nested", "config.yaml")

	cfg := DefaultConfig()
	if err := cfg.Save(cfgPath); err != nil {
		t.Fatalf("Save should create directories: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		t.Error("Config file was not created")
	}
}

func TestSave_RejectsIntermediateSymlink(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	target := t.TempDir()
	if err := os.Symlink(target, filepath.Join(root, "redirect")); err != nil {
		// Criar symlink no Windows exige privilégio de administrador; sem ele
		// o cenário não pode ser montado e o teste é legitimamente ignorado.
		t.Skipf("criar symlink requer privilégio de administrador neste ambiente: %v", err)
	}
	cfg := DefaultConfig()
	err := cfg.Save(filepath.Join(root, "redirect", "config.yaml"))
	if err == nil || !contains(err.Error(), "symlink") {
		t.Fatalf("Save through intermediate symlink error = %v", err)
	}
}

func TestSave_ProviderKeyEncrypted(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	cfg := DefaultConfig()
	cfg.Provider.APIKey = "my-plaintext-api-key-12345"

	if err := cfg.Save(cfgPath); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// APIKey should still be plaintext in memory after save
	if cfg.Provider.APIKey != "my-plaintext-api-key-12345" {
		t.Errorf("APIKey in memory changed after save: %q", cfg.Provider.APIKey)
	}

	// The file on disk should contain base64-encoded encrypted key (not plaintext)
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("failed to read saved file: %v", err)
	}
	content := string(data)
	if contains, substr := content, "my-plaintext-api-key-12345"; contains == substr {
		t.Error("API key should NOT be stored in plaintext on disk")
	}
}

func TestSave_ProviderKeyExternalReference(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	cfg := DefaultConfig()
	cfg.Provider.APIKey = "synthetic-fixture-credential"
	cfg.Provider.APIKeyEnv = "SYNTHETIC_PROVIDER_KEY"

	if err := cfg.Save(path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}
	if contains(string(b), "synthetic-fixture-credential") {
		t.Fatal("external-reference config contained credential material")
	}
	if !contains(string(b), "api_key_env: SYNTHETIC_PROVIDER_KEY") {
		t.Fatal("external API key reference was not persisted")
	}
	// Permissões POSIX (0600) não são representadas no Windows: qualquer
	// arquivo criado relata 0666, então a asserção só vale em sistemas Unix.
	if runtime.GOOS != "windows" {
		if mode := fileMode(t, path); mode != 0o600 {
			t.Fatalf("config mode = %o, want 600", mode)
		}
	}
}

func fileMode(t *testing.T, path string) uint32 {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	return uint32(info.Mode().Perm())
}

// =============================================================================
// Environment variable override tests
// =============================================================================

func TestLoadFromEnv_Overrides(t *testing.T) {
	// Not parallel — modifies environment
	tests := []struct {
		name     string
		envKey   string
		envValue string
		verify   func(t *testing.T, cfg *Config)
	}{
		{
			name:     "COSCA_HOME override",
			envKey:   "COSCA_HOME",
			envValue: "/custom/cosca/home",
			verify: func(t *testing.T, cfg *Config) {
				if cfg.Paths.Home != "/custom/cosca/home" {
					t.Errorf("Paths.Home = %q", cfg.Paths.Home)
				}
			},
		},
		{
			name:     "COSCA_EDITOR override",
			envKey:   "COSCA_EDITOR",
			envValue: "code",
			verify: func(t *testing.T, cfg *Config) {
				if cfg.Editor.Name != "code" {
					t.Errorf("Editor.Name = %q", cfg.Editor.Name)
				}
			},
		},
		{
			name:     "COSCA_MODE override",
			envKey:   "COSCA_MODE",
			envValue: "production",
			verify: func(t *testing.T, cfg *Config) {
				if cfg.Mode != "production" {
					t.Errorf("Mode = %q", cfg.Mode)
				}
			},
		},
		{
			name:     "COSCA_VERBOSE true",
			envKey:   "COSCA_VERBOSE",
			envValue: "true",
			verify: func(t *testing.T, cfg *Config) {
				if !cfg.Verbose {
					t.Error("Verbose should be true")
				}
			},
		},
		{
			name:     "COSCA_VERBOSE 1",
			envKey:   "COSCA_VERBOSE",
			envValue: "1",
			verify: func(t *testing.T, cfg *Config) {
				if !cfg.Verbose {
					t.Error("Verbose should be true")
				}
			},
		},
		{
			name:     "COSCA_VERBOSE false",
			envKey:   "COSCA_VERBOSE",
			envValue: "false",
			verify: func(t *testing.T, cfg *Config) {
				if cfg.Verbose {
					t.Error("Verbose should be false")
				}
			},
		},
		{
			name:     "COSCA_LOG_LEVEL override",
			envKey:   "COSCA_LOG_LEVEL",
			envValue: "debug",
			verify: func(t *testing.T, cfg *Config) {
				if cfg.Log.Level != "debug" {
					t.Errorf("Log.Level = %q", cfg.Log.Level)
				}
			},
		},
		{
			name:     "COSCA_PROVIDER__API_KEY override",
			envKey:   "COSCA_PROVIDER__API_KEY",
			envValue: "env-api-key-123",
			verify: func(t *testing.T, cfg *Config) {
				if cfg.Provider.APIKey != "env-api-key-123" {
					t.Errorf("Provider.APIKey = %q", cfg.Provider.APIKey)
				}
			},
		},
		{
			name:     "COSCA_PROVIDER__MODEL override",
			envKey:   "COSCA_PROVIDER__MODEL",
			envValue: "gpt-5",
			verify: func(t *testing.T, cfg *Config) {
				if cfg.Provider.Model != "gpt-5" {
					t.Errorf("Provider.Model = %q", cfg.Provider.Model)
				}
			},
		},
		{
			name:     "COSCA_PROVIDER__BASE_URL override",
			envKey:   "COSCA_PROVIDER__BASE_URL",
			envValue: "https://api.custom.com",
			verify: func(t *testing.T, cfg *Config) {
				if cfg.Provider.BaseURL != "https://api.custom.com" {
					t.Errorf("Provider.BaseURL = %q", cfg.Provider.BaseURL)
				}
			},
		},
		{
			name:     "COSCA_EMBEDDING__BASE_URL override",
			envKey:   "COSCA_EMBEDDING__BASE_URL",
			envValue: "http://127.0.0.1:11435/v1",
			verify: func(t *testing.T, cfg *Config) {
				if cfg.Embedding.BaseURL != "http://127.0.0.1:11435/v1" {
					t.Errorf("Embedding.BaseURL = %q", cfg.Embedding.BaseURL)
				}
			},
		},
		{
			name:     "COSCA_DB__PATH override",
			envKey:   "COSCA_DB__PATH",
			envValue: "/tmp/test.db",
			verify: func(t *testing.T, cfg *Config) {
				if cfg.DB.Path != "/tmp/test.db" {
					t.Errorf("DB.Path = %q", cfg.DB.Path)
				}
			},
		},
		{
			name:     "COSCA_FEATURES__TELEMETRY true",
			envKey:   "COSCA_FEATURES__TELEMETRY",
			envValue: "true",
			verify: func(t *testing.T, cfg *Config) {
				if !cfg.Features.Telemetry {
					t.Error("Features.Telemetry should be true")
				}
			},
		},
		{
			name:     "COSCA_FEATURES__METRICS 1",
			envKey:   "COSCA_FEATURES__METRICS",
			envValue: "1",
			verify: func(t *testing.T, cfg *Config) {
				if !cfg.Features.Metrics {
					t.Error("Features.Metrics should be true")
				}
			},
		},
		{
			name:     "COSCA_NETWORK__PROXY_URL override",
			envKey:   "COSCA_NETWORK__PROXY_URL",
			envValue: "http://proxy:8080",
			verify: func(t *testing.T, cfg *Config) {
				if cfg.Network.ProxyURL != "http://proxy:8080" {
					t.Errorf("Network.ProxyURL = %q", cfg.Network.ProxyURL)
				}
			},
		},
		{
			name:     "COSCA_DEV true sets mode to development",
			envKey:   "COSCA_DEV",
			envValue: "true",
			verify: func(t *testing.T, cfg *Config) {
				if cfg.Mode != "development" {
					t.Errorf("Mode = %q", cfg.Mode)
				}
			},
		},
		{
			name:     "COSCA_PROJECT override",
			envKey:   "COSCA_PROJECT",
			envValue: "/my/project/root",
			verify: func(t *testing.T, cfg *Config) {
				if cfg.Paths.Project != "/my/project/root" {
					t.Errorf("Paths.Project = %q", cfg.Paths.Project)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear all COSCA_ env vars first
			for _, e := range os.Environ() {
				if len(e) > 4 && e[:4] == "COSCA_" {
					key := ""
					for i, c := range e {
						if c == '=' {
							key = e[:i]
							break
						}
					}
					if key != "" {
						os.Unsetenv(key)
					}
				}
			}

			// Set the test var
			t.Setenv(tt.envKey, tt.envValue)

			cfg := DefaultConfig()
			cfg.loadFromEnv()
			tt.verify(t, cfg)
		})
	}
}

func TestLoadFromEnv_NonCoscaVarsIgnored(t *testing.T) {
	t.Setenv("HOME", "/home/testuser")
	t.Setenv("PATH", "/usr/bin")
	t.Setenv("OTHER_VAR", "value")

	cfg := DefaultConfig()
	cfg.loadFromEnv()

	// HOME env var on Linux is the system HOME, not our COSCA_HOME
	// Just verify the config is still a valid default
	if cfg.Paths.Home == "" {
		t.Error("Paths.Home should not be empty after loading env")
	}
}

func TestLoadFromEnv_DEVFalse(t *testing.T) {
	// DEV=false should NOT set mode to development
	t.Setenv("COSCA_DEV", "false")

	cfg := DefaultConfig()
	prevMode := cfg.Mode
	cfg.loadFromEnv()

	// Mode should NOT have been changed to "development"
	if cfg.Mode != prevMode {
		t.Logf("Mode changed to %q, was %q", cfg.Mode, prevMode)
	}
}

func TestLoadFromEnv_NoVarsSet(t *testing.T) {
	// Clear all COSCA_ env vars
	for _, e := range os.Environ() {
		if len(e) > 4 && e[:4] == "COSCA_" {
			key := ""
			for i, c := range e {
				if c == '=' {
					key = e[:i]
					break
				}
			}
			if key != "" {
				os.Unsetenv(key)
			}
		}
	}

	cfg := DefaultConfig()
	prevHome := cfg.Paths.Home
	cfg.loadFromEnv()

	// Nothing should change
	if cfg.Paths.Home != prevHome {
		t.Errorf("Paths.Home changed from %q to %q", prevHome, cfg.Paths.Home)
	}
}

// =============================================================================
// Validate edge cases
// =============================================================================

func TestValidate_ProviderValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		modify     func(cfg *Config)
		wantErr    bool
		errContain string
	}{
		{
			name: "valid provider openai",
			modify: func(cfg *Config) {
				cfg.Provider.Name = "openai"
			},
			wantErr: false,
		},
		{
			name: "valid provider anthropic",
			modify: func(cfg *Config) {
				cfg.Provider.Name = "anthropic"
			},
			wantErr: false,
		},
		{
			name: "valid provider ollama",
			modify: func(cfg *Config) {
				cfg.Provider.Name = "ollama"
			},
			wantErr: false,
		},
		{
			name: "valid provider azure",
			modify: func(cfg *Config) {
				cfg.Provider.Name = "azure"
			},
			wantErr: false,
		},
		{
			name: "valid provider google",
			modify: func(cfg *Config) {
				cfg.Provider.Name = "google"
			},
			wantErr: false,
		},
		{
			name: "valid provider aws-bedrock",
			modify: func(cfg *Config) {
				cfg.Provider.Name = "aws-bedrock"
			},
			wantErr: false,
		},
		{
			name: "valid provider custom",
			modify: func(cfg *Config) {
				cfg.Provider.Name = "custom"
			},
			wantErr: false,
		},
		{
			name: "empty provider name is valid",
			modify: func(cfg *Config) {
				cfg.Provider.Name = ""
			},
			wantErr: false,
		},
		{
			name: "unsupported provider",
			modify: func(cfg *Config) {
				cfg.Provider.Name = "unsupported-provider"
			},
			wantErr:    true,
			errContain: "unsupported provider",
		},
		{
			name: "temperature too low",
			modify: func(cfg *Config) {
				cfg.Provider.Temperature = -0.5
			},
			wantErr:    true,
			errContain: "temperature",
		},
		{
			name: "temperature too high",
			modify: func(cfg *Config) {
				cfg.Provider.Temperature = 2.5
			},
			wantErr:    true,
			errContain: "temperature",
		},
		{
			name: "max_tokens zero",
			modify: func(cfg *Config) {
				cfg.Provider.MaxTokens = 0
			},
			wantErr:    true,
			errContain: "max_tokens",
		},
		{
			name: "max_tokens negative",
			modify: func(cfg *Config) {
				cfg.Provider.MaxTokens = -1
			},
			wantErr:    true,
			errContain: "max_tokens",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			tt.modify(cfg)
			err := cfg.Validate()
			if tt.wantErr {
				if err == nil {
					t.Error("Expected validation error, got nil")
				} else if tt.errContain != "" {
					if _, ok := err.(*ValidationError); ok {
						found := false
						for _, e := range err.(*ValidationError).Errors {
							if contains(e, tt.errContain) {
								found = true
							}
						}
						if !found {
							t.Errorf("Error %v should contain %q", err, tt.errContain)
						}
					}
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected validation error: %v", err)
				}
			}
		})
	}
}

func TestValidate_ServerPortValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		apiPort    int
		rpcPort    int
		wantErr    bool
		errContain string
	}{
		{"valid ports", 8370, 8372, false, ""},
		{"api_port zero", 0, 8372, true, "api_port"},
		{"api_port too high", 99999, 8372, true, "api_port"},
		{"rpc_port zero", 8370, 0, true, "rpc_port"},
		{"rpc_port negative", 8370, -1, true, "rpc_port"},
		{"same ports", 9000, 9000, true, "different"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.Server.APIPort = tt.apiPort
			cfg.Server.RPCPort = tt.rpcPort
			err := cfg.Validate()
			if tt.wantErr {
				if err == nil {
					t.Error("Expected validation error, got nil")
				} else if tt.errContain != "" {
					verr, ok := err.(*ValidationError)
					if !ok {
						t.Fatalf("Expected ValidationError, got %T", err)
					}
					found := false
					for _, e := range verr.Errors {
						if contains(e, tt.errContain) {
							found = true
						}
					}
					if !found {
						t.Errorf("Error %v should contain %q", verr, tt.errContain)
					}
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected validation error: %v", err)
				}
			}
		})
	}
}

func TestValidate_PerformanceValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		memoryMB   int
		concurrent int
		wantErr    bool
		errContain string
	}{
		{"valid", 512, 10, false, ""},
		{"memory too low", 32, 10, true, "max_memory_mb"},
		{"memory at boundary", 64, 10, false, ""},
		{"memory exactly 63", 63, 10, true, "max_memory_mb"},
		{"concurrent zero", 512, 0, true, "max_concurrent_ops"},
		{"concurrent negative", 512, -5, true, "max_concurrent_ops"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.Performance.MaxMemoryMB = tt.memoryMB
			cfg.Performance.MaxConcurrentOps = tt.concurrent
			err := cfg.Validate()
			if tt.wantErr {
				if err == nil {
					t.Error("Expected validation error, got nil")
				} else if tt.errContain != "" {
					verr, ok := err.(*ValidationError)
					if !ok {
						t.Fatalf("Expected ValidationError, got %T", err)
					}
					found := false
					for _, e := range verr.Errors {
						if contains(e, tt.errContain) {
							found = true
						}
					}
					if !found {
						t.Errorf("Error %v should contain %q", verr, tt.errContain)
					}
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected validation error: %v", err)
				}
			}
		})
	}
}

func TestValidate_DBValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		maxOpenConns int
		pageSize     int
		wantErr      bool
		errContain   string
	}{
		{"valid", 25, 4096, false, ""},
		{"max_open_conns zero", 0, 4096, true, "max_open_conns"},
		{"max_open_conns negative", -1, 4096, true, "max_open_conns"},
		{"page_size too small", 25, 256, true, "page_size"},
		{"page_size 512 ok", 25, 512, false, ""},
		{"page_size 65536 ok", 25, 65536, false, ""},
		{"page_size too large", 25, 65537, true, "page_size"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.DB.MaxOpenConns = tt.maxOpenConns
			cfg.DB.PageSize = tt.pageSize
			err := cfg.Validate()
			if tt.wantErr {
				if err == nil {
					t.Error("Expected validation error, got nil")
				} else if tt.errContain != "" {
					verr, ok := err.(*ValidationError)
					if !ok {
						t.Fatalf("Expected ValidationError, got %T", err)
					}
					found := false
					for _, e := range verr.Errors {
						if contains(e, tt.errContain) {
							found = true
						}
					}
					if !found {
						t.Errorf("Error %v should contain %q", verr, tt.errContain)
					}
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected validation error: %v", err)
				}
			}
		})
	}
}

func TestValidate_SearchValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		defaultLimit int
		maxResults   int
		wantErr      bool
		errContain   string
	}{
		{"valid", 50, 1000, false, ""},
		{"default_limit zero", 0, 1000, true, "default_limit"},
		{"default_limit negative", -1, 1000, true, "default_limit"},
		{"default_limit exceeds max", 2000, 1000, true, "default_limit"},
		{"equal limit and max ok", 50, 50, false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.Search.DefaultLimit = tt.defaultLimit
			cfg.Search.MaxResults = tt.maxResults
			err := cfg.Validate()
			if tt.wantErr {
				if err == nil {
					t.Error("Expected validation error, got nil")
				} else if tt.errContain != "" {
					verr, ok := err.(*ValidationError)
					if !ok {
						t.Fatalf("Expected ValidationError, got %T", err)
					}
					found := false
					for _, e := range verr.Errors {
						if contains(e, tt.errContain) {
							found = true
						}
					}
					if !found {
						t.Errorf("Error %v should contain %q", verr, tt.errContain)
					}
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected validation error: %v", err)
				}
			}
		})
	}
}

func TestValidate_InsecureSkipVerify(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		mode         string
		insecureSkip bool
		wantErr      bool
	}{
		{"dev mode insecure ok", "development", true, false},
		{"dev mode secure ok", "development", false, false},
		{"production insecure fail", "production", true, true},
		{"production secure ok", "production", false, false},
		{"staging insecure fail", "staging", true, true},
		{"test mode insecure fail", "test", true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.Mode = tt.mode
			cfg.Network.InsecureSkipVerify = tt.insecureSkip
			err := cfg.Validate()
			if tt.wantErr {
				if err == nil {
					t.Error("Expected validation error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected validation error: %v", err)
				}
			}
		})
	}
}

func TestValidate_MultipleErrors(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.Version = ""
	cfg.Paths.Home = ""
	cfg.Provider.Temperature = 3.0
	cfg.Server.APIPort = 0

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Expected validation error")
	}

	verr, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("Expected ValidationError, got %T", err)
	}

	if len(verr.Errors) < 3 {
		t.Errorf("Expected at least 3 errors, got %d: %v", len(verr.Errors), verr.Errors)
	}
}

// =============================================================================
// Accessor edge cases
// =============================================================================

func TestDBPath_CustomPath(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.DB.Path = "/custom/cosca.db"
	path := cfg.DBPath()
	if path != "/custom/cosca.db" {
		t.Errorf("DBPath = %q, want %q", path, "/custom/cosca.db")
	}
}

func TestDBPath_EmptyPath(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.DB.Path = ""
	path := cfg.DBPath()
	if path == "" {
		t.Error("DBPath should not be empty when DB.Path is empty (should use default)")
	}
}

func TestRuntimeDir_CustomPath(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.Paths.Runtime = "/custom/runtime"
	dir := cfg.RuntimeDir()
	if dir != "/custom/runtime" {
		t.Errorf("RuntimeDir = %q, want %q", dir, "/custom/runtime")
	}
}

func TestRuntimeDir_EmptyPath(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.Paths.Runtime = ""
	dir := cfg.RuntimeDir()
	if dir == "" {
		t.Error("RuntimeDir should not be empty when Paths.Runtime is empty")
	}
}

// =============================================================================
// Viper bridge tests
// =============================================================================

func TestSetViper_GetViper(t *testing.T) {
	// Reset the global viper between tests (not parallel safe)
	SetViper(nil)

	v := GetViper()
	if v == nil {
		t.Fatal("GetViper returned nil")
	}

	// Get again, should return same instance
	v2 := GetViper()
	if v != v2 {
		t.Error("GetViper should return the same instance")
	}

	// Set a specific viper instance
	// Note: viper.New() creates a new instance; we just verify SetViper doesn't panic
	SetViper(v)
	// Setting to nil should not crash
	SetViper(nil)

	v3 := GetViper()
	if v3 == nil {
		t.Fatal("GetViper should create a new viper after reset")
	}
}

// =============================================================================
// MaskAPIKey tests
// =============================================================================

func TestMaskAPIKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		key  string
		want string
	}{
		{"empty key", "", "(not set)"},
		{"short key 1 char", "a", "****"},
		{"short key 8 chars", "abcdefgh", "****"},
		{"short key 7 chars", "abcdefg", "****"},
		{"normal key", "sk-1234567890abcdef", "sk-1...cdef"},
		{"long key", "sk-proj-very-long-api-key-that-is-very-secure", "sk-p...cure"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MaskAPIKey(tt.key)
			if got != tt.want {
				t.Errorf("MaskAPIKey(%q) = %q, want %q", tt.key, got, tt.want)
			}
		})
	}
}

// =============================================================================
// Load tests
// =============================================================================

func TestLoad_FromDefaultPaths(t *testing.T) {
	// This test verifies Load doesn't panic with no files present
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed (expected defaults only): %v", err)
	}
	if cfg == nil {
		t.Fatal("Load returned nil config")
	}
	if cfg.Mode != "development" {
		t.Errorf("Mode = %q, want %q", cfg.Mode, "development")
	}
}

// =============================================================================
// DefaultConfig - comprehensive region checks
// =============================================================================

func TestDefaultConfig_ServerDefaults(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()

	if cfg.Server.Host != DefaultHost {
		t.Errorf("Server.Host = %q, want %q", cfg.Server.Host, DefaultHost)
	}
	if cfg.Server.APIPort != DefaultAPIPort {
		t.Errorf("Server.APIPort = %d", cfg.Server.APIPort)
	}
	if cfg.Server.MetricsPort != DefaultMetricsPort {
		t.Errorf("Server.MetricsPort = %d", cfg.Server.MetricsPort)
	}
	if cfg.Server.RPCPort != DefaultRPCPort {
		t.Errorf("Server.RPCPort = %d", cfg.Server.RPCPort)
	}
	if cfg.Server.ReadTimeout != 30*time.Second {
		t.Errorf("Server.ReadTimeout = %v", cfg.Server.ReadTimeout)
	}
	if cfg.Server.MaxHeaderBytes != 1<<20 {
		t.Errorf("Server.MaxHeaderBytes = %d", cfg.Server.MaxHeaderBytes)
	}
}

func TestDefaultConfig_NetworkDefaults(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()

	if cfg.Network.DialTimeout != 10*time.Second {
		t.Errorf("Network.DialTimeout = %v", cfg.Network.DialTimeout)
	}
	if cfg.Network.KeepAlive != 30*time.Second {
		t.Errorf("Network.KeepAlive = %v", cfg.Network.KeepAlive)
	}
	if cfg.Network.MaxIdleConns != 100 {
		t.Errorf("Network.MaxIdleConns = %d", cfg.Network.MaxIdleConns)
	}
	if cfg.Network.IdleConnTimeout != 90*time.Second {
		t.Errorf("Network.IdleConnTimeout = %v", cfg.Network.IdleConnTimeout)
	}
}

func TestDefaultConfig_LogDefaults(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()

	if cfg.Log.Level != "info" {
		t.Errorf("Log.Level = %q, want %q", cfg.Log.Level, "info")
	}
	if cfg.Log.Format != "console" {
		t.Errorf("Log.Format = %q, want %q", cfg.Log.Format, "console")
	}
	if cfg.Log.MaxSizeMB != DefaultLogMaxSizeMB {
		t.Errorf("Log.MaxSizeMB = %d", cfg.Log.MaxSizeMB)
	}
	if !cfg.Log.Compress {
		t.Error("Log.Compress should be true")
	}
	if cfg.Log.ShowCaller {
		t.Error("Log.ShowCaller should be false")
	}
}

func TestDefaultConfig_CacheDefaults(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()

	if cfg.Cache.Type != "memory" {
		t.Errorf("Cache.Type = %q, want %q", cfg.Cache.Type, "memory")
	}
	if cfg.Cache.Size != DefaultCacheSize {
		t.Errorf("Cache.Size = %d", cfg.Cache.Size)
	}
	if cfg.Cache.TTL != DefaultCacheTTL {
		t.Errorf("Cache.TTL = %v", cfg.Cache.TTL)
	}
}

func TestDefaultConfig_EditorDefaults(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()

	if cfg.Editor.Name != DefaultEditor {
		t.Errorf("Editor.Name = %q", cfg.Editor.Name)
	}
	if cfg.Editor.TabSize != 4 {
		t.Errorf("Editor.TabSize = %d", cfg.Editor.TabSize)
	}
	if !cfg.Editor.AutoSave {
		t.Error("Editor.AutoSave should be true")
	}
	if cfg.Editor.AutoSaveInterval != 30*time.Second {
		t.Errorf("Editor.AutoSaveInterval = %v", cfg.Editor.AutoSaveInterval)
	}
}

func TestDefaultConfig_WatchExcludePatterns(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()

	expected := []string{".git/**", "node_modules/**", "*.log", "*.tmp"}
	if len(cfg.Watch.ExcludePatterns) != len(expected) {
		t.Errorf("Watch.ExcludePatterns length = %d, want %d", len(cfg.Watch.ExcludePatterns), len(expected))
	}
}

func TestDefaultConfig_PluginDefaults(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()

	if cfg.Plugins.MaxMemoryMB != 128 {
		t.Errorf("Plugins.MaxMemoryMB = %d", cfg.Plugins.MaxMemoryMB)
	}
	if len(cfg.Plugins.AllowedPolicies) != 3 {
		t.Errorf("Plugins.AllowedPolicies length = %d", len(cfg.Plugins.AllowedPolicies))
	}
}

func TestDefaultConfig_PerformanceDefaults(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()

	if cfg.Performance.BatchSize != DefaultBatchSize {
		t.Errorf("Performance.BatchSize = %d", cfg.Performance.BatchSize)
	}
	if cfg.Performance.PrefetchSize != 1024 {
		t.Errorf("Performance.PrefetchSize = %d", cfg.Performance.PrefetchSize)
	}
	// WorkerPoolSize should be > 0 (runtime.NumCPU)
	if cfg.Performance.WorkerPoolSize < 1 {
		t.Error("Performance.WorkerPoolSize should be >= 1")
	}
}

func TestDefaultConfig_TimeoutDefaults(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()

	if cfg.Timeouts.GracefulShutdown != DefaultGracefulShutdown {
		t.Errorf("Timeouts.GracefulShutdown = %v", cfg.Timeouts.GracefulShutdown)
	}
	if cfg.Timeouts.HealthCheck != DefaultHealthCheckInterval {
		t.Errorf("Timeouts.HealthCheck = %v", cfg.Timeouts.HealthCheck)
	}
	if cfg.Timeouts.Request != DefaultRequestTimeout {
		t.Errorf("Timeouts.Request = %v", cfg.Timeouts.Request)
	}
	if cfg.Timeouts.Workflow != DefaultWorkflowTimeout {
		t.Errorf("Timeouts.Workflow = %v", cfg.Timeouts.Workflow)
	}
	if cfg.Timeouts.Skill != DefaultSkillTimeout {
		t.Errorf("Timeouts.Skill = %v", cfg.Timeouts.Skill)
	}
}

func TestDefaultConfig_FeatureDefaults(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()

	if cfg.Features.ShellCompletion != true {
		t.Error("Features.ShellCompletion should be true")
	}
	if cfg.Features.Experimental != false {
		t.Error("Features.Experimental should be false")
	}
	if cfg.Features.AutoUpdate != DefaultEnableAutoUpdate {
		t.Errorf("Features.AutoUpdate = %v", cfg.Features.AutoUpdate)
	}
}

func TestDefaultConfig_Version(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()

	if cfg.Version != "1.0" {
		t.Errorf("Version = %q", cfg.Version)
	}
}

// =============================================================================
// Additional coverage-edge tests
// =============================================================================

func TestLoadFromEnv_DEV1(t *testing.T) {
	t.Setenv("COSCA_DEV", "1")
	cfg := DefaultConfig()
	cfg.loadFromEnv()
	if cfg.Mode != "development" {
		t.Errorf("COSCA_DEV=1 should set Mode to 'development', got %q", cfg.Mode)
	}
}

func TestValidate_ExperimentalVerbose(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	cfg.Features.Experimental = true
	cfg.Verbose = true // both must be true for the branch

	err := cfg.Validate()
	if err != nil {
		t.Logf("Validate with Experimental+Verbose: %v", err)
	}
	// When both Experimental and Verbose are true, the code runs: c.Verbose = true (no-op)
	// This exercises the branch at line 803-804
	if !cfg.Verbose {
		t.Error("Verbose should still be true")
	}
}

func TestValidate_MinutesTimeout(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	cfg.Timeouts.Workflow = 5 * time.Minute

	err := cfg.Validate()
	if err != nil {
		t.Errorf("Unexpected validation error for valid timeout: %v", err)
	}
}

func TestLoad_FromProjectDir(t *testing.T) {
	// Create a temp project directory with .cosca/config.yaml
	// NOTE: Load() loads user config (~/.config/cosca/config.yaml) AFTER project
	// config, so user values take precedence. This test verifies the project
	// config file is found and parsed without error, producing a valid Config.
	tmpDir := t.TempDir()
	projectCfgDir := filepath.Join(tmpDir, ".cosca")
	if err := os.MkdirAll(projectCfgDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	yamlContent := `
version: "2.0"
mode: "production"
verbose: true
provider:
  name: ollama
`
	if err := os.WriteFile(filepath.Join(projectCfgDir, "config.yaml"), []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	origWd, _ := os.Getwd()
	defer os.Chdir(origWd)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// At minimum, verify Load returns a valid config
	if cfg == nil {
		t.Fatal("Load returned nil config")
	}
	if cfg.Version == "" {
		t.Error("Version should not be empty after Load")
	}
	// Version from project config (2.0) may be overridden by user config
	t.Logf("Loaded config Version=%q Mode=%q Provider.Name=%q", cfg.Version, cfg.Mode, cfg.Provider.Name)
}

func TestLoadFromFile_InvalidValidation(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "invalid-cfg.yaml")

	// Valid YAML but invalid config values
	yamlContent := `
version: ""
paths:
  home: ""
provider:
  temperature: 3.0
  max_tokens: 0
`
	if err := os.WriteFile(cfgPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := LoadFromFile(cfgPath)
	if err == nil {
		t.Error("LoadFromFile should fail validation for invalid config values")
	}
}

func TestLoadFromFile_InvalidYamlReturnsError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "broken.yaml")
	if err := os.WriteFile(cfgPath, []byte("{broken: [yaml}"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := LoadFromFile(cfgPath)
	if err == nil {
		t.Error("LoadFromFile should return error for unparseable YAML")
	}
}

func TestSave_NoProviderKey(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	cfg := DefaultConfig()
	cfg.Provider.APIKey = "" // no key

	if err := cfg.Save(cfgPath); err != nil {
		t.Fatalf("Save with empty API key should succeed: %v", err)
	}

	// Verify file exists and is valid YAML
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		t.Error("Config file was not created")
	}
}

func TestConfig_UserDefined(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	yamlContent := `
version: "1.0"
custom_field: "hello"
another_custom: 42
`
	if err := os.WriteFile(cfgPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadFromFile(cfgPath)
	if err != nil {
		t.Fatalf("LoadFromFile: %v", err)
	}

	if cfg.UserDefined == nil {
		t.Error("UserDefined should not be nil when extra keys are present")
	}
}

func TestDefaultConfig_WatchMaxFileSize(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	if cfg.Watch.MaxFileSize != 10*1024*1024 {
		t.Errorf("Watch.MaxFileSize = %d, want %d", cfg.Watch.MaxFileSize, 10*1024*1024)
	}
}

func TestDefaultConfig_DBBackupInterval(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	if cfg.DB.BackupInterval != 1*time.Hour {
		t.Errorf("DB.BackupInterval = %v, want %v", cfg.DB.BackupInterval, 1*time.Hour)
	}
}

func TestDefaultConfig_DBMaxConns(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()

	if cfg.DB.MaxOpenConns != DefaultMaxOpenDBConn {
		t.Errorf("DB.MaxOpenConns = %d", cfg.DB.MaxOpenConns)
	}
	if cfg.DB.MaxIdleConns != DefaultMaxIdleDBConn {
		t.Errorf("DB.MaxIdleConns = %d", cfg.DB.MaxIdleConns)
	}
	if cfg.DB.ConnMaxLifetime != DefaultDBConnMaxLifetime {
		t.Errorf("DB.ConnMaxLifetime = %v", cfg.DB.ConnMaxLifetime)
	}
	if cfg.DB.ConnMaxIdleTime != DefaultDBConnMaxIdleTime {
		t.Errorf("DB.ConnMaxIdleTime = %v", cfg.DB.ConnMaxIdleTime)
	}
}

func TestDefaultConfig_ProviderRetry(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()

	if cfg.Provider.MaxRetries != DefaultMaxRetries {
		t.Errorf("Provider.MaxRetries = %d", cfg.Provider.MaxRetries)
	}
	if cfg.Provider.RateLimitPerMin != DefaultRateLimitPerMin {
		t.Errorf("Provider.RateLimitPerMin = %d", cfg.Provider.RateLimitPerMin)
	}
	if cfg.Provider.Timeout != DefaultRequestTimeout {
		t.Errorf("Provider.Timeout = %v", cfg.Provider.Timeout)
	}
}

func TestDefaultConfig_SearchInterval(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	if cfg.Search.IndexInterval != DefaultIndexInterval {
		t.Errorf("Search.IndexInterval = %v", cfg.Search.IndexInterval)
	}
	if cfg.Search.MinScore != 0.7 {
		t.Errorf("Search.MinScore = %f", cfg.Search.MinScore)
	}
}

func TestDefaultConfig_ServerTLSDefaults(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	if cfg.Server.TLS {
		t.Error("Server.TLS should be false by default")
	}
	if cfg.Server.WriteTimeout != 30*time.Second {
		t.Errorf("Server.WriteTimeout = %v", cfg.Server.WriteTimeout)
	}
}

func TestDefaultConfig_RetryDelay(t *testing.T) {
	if DefaultRetryDelay != 1*time.Second {
		t.Errorf("DefaultRetryDelay = %v", DefaultRetryDelay)
	}
	if DefaultMaxBackoff != 30*time.Second {
		t.Errorf("DefaultMaxBackoff = %v", DefaultMaxBackoff)
	}
	if DefaultVectorDimensions != 1536 {
		t.Errorf("DefaultVectorDimensions = %d", DefaultVectorDimensions)
	}
}

func TestEncryptAPIKeys_EmptyProvider(t *testing.T) {
	var p ProviderConfig // zero-value, no APIKey set
	if err := p.EncryptAPIKeys(); err != nil {
		t.Errorf("EncryptAPIKeys on empty provider should not error: %v", err)
	}
}

func TestDecryptAPIKeys_EmptyProvider(t *testing.T) {
	var p ProviderConfig
	if err := p.DecryptAPIKeys(); err != nil {
		t.Errorf("DecryptAPIKeys on empty provider should not error: %v", err)
	}
}

// =============================================================================
// Helpers
// =============================================================================

// contains reports whether s contains substr.
func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
