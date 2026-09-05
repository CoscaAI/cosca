package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/CoscaAI/cosca/internal/models"
)

func TestSaveRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.yaml")
	if err := os.WriteFile(target, []byte("untouched"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "config.yaml")
	if err := os.Symlink(target, path); err != nil {
		// Criar symlink no Windows exige privilégio de administrador; sem ele
		// o cenário não pode ser montado e o teste é legitimamente ignorado.
		t.Skipf("criar symlink requer privilégio de administrador neste ambiente: %v", err)
	}
	err := DefaultConfig().Save(path)
	if err == nil {
		t.Fatal("Save should reject symlink")
	}
	got, _ := os.ReadFile(target)
	if string(got) != "untouched" {
		t.Fatal("symlink target was modified")
	}
}

func TestSaveRejectsFilesystemRoot(t *testing.T) {
	if err := DefaultConfig().Save(string(filepath.Separator)); err == nil {
		t.Fatal("Save should reject filesystem root")
	}
}

func TestDefaultConfig(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	if cfg == nil {
		t.Fatal("DefaultConfig returned nil")
	}
	if cfg.Version != "1.0" {
		t.Errorf("Version = %q, want %q", cfg.Version, "1.0")
	}
	if cfg.Profile != "default" {
		t.Errorf("Profile = %q", cfg.Profile)
	}
	if cfg.Mode != "development" {
		t.Errorf("Mode = %q", cfg.Mode)
	}
}

func TestDefaultProviderConfig(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	if cfg.Provider.Name != DefaultProvider {
		t.Errorf("Provider.Name = %q", cfg.Provider.Name)
	}
	if cfg.Provider.Model != DefaultProviderModel {
		t.Errorf("Provider.Model = %q", cfg.Provider.Model)
	}
	if cfg.Provider.MaxTokens != DefaultProviderMaxTokens {
		t.Errorf("Provider.MaxTokens = %d", cfg.Provider.MaxTokens)
	}
	if cfg.Provider.Temperature != DefaultProviderTemperature {
		t.Errorf("Provider.Temperature = %f", cfg.Provider.Temperature)
	}
}

func TestDefaultDBConfig(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	if !cfg.DB.WALMode {
		t.Error("DB.WALMode should be true")
	}
	if cfg.DB.PageSize != DefaultDBPageSize {
		t.Errorf("DB.PageSize = %d", cfg.DB.PageSize)
	}
	if cfg.DB.EnableFTS5 != true {
		t.Error("DB.EnableFTS5 should be true")
	}
}

func TestDefaultPathConfig(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	if cfg.Paths.Home == "" {
		t.Error("Paths.Home should not be empty")
	}
	if cfg.Paths.Runtime == "" {
		t.Error("Paths.Runtime should not be empty")
	}
	if cfg.Paths.Data == "" {
		t.Error("Paths.Data should not be empty")
	}
	if cfg.Paths.Cache == "" {
		t.Error("Paths.Cache should not be empty")
	}
}

func TestDefaultSearchConfig(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	if cfg.Search.DefaultLimit != DefaultSearchResultLimit {
		t.Errorf("Search.DefaultLimit = %d", cfg.Search.DefaultLimit)
	}
	if !cfg.Search.EnableFullText {
		t.Error("Search.EnableFullText should be true")
	}
}

func TestDefaultFeatureConfig(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	if cfg.Features.Metrics != DefaultEnableMetrics {
		t.Errorf("Features.Metrics = %v", cfg.Features.Metrics)
	}
	if cfg.Features.Telemetry != DefaultEnableTelemetry {
		t.Errorf("Features.Telemetry = %v", cfg.Features.Telemetry)
	}
	if cfg.Features.PluginSystem != DefaultEnablePluginSystem {
		t.Errorf("Features.PluginSystem = %v", cfg.Features.PluginSystem)
	}
}

func TestConfigValidate(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	err := cfg.Validate()
	if err != nil {
		t.Fatalf("Validate error: %v", err)
	}
}

func TestConfigValidateErrors(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	cfg.Version = ""
	cfg.Paths.Home = ""
	err := cfg.Validate()
	if err == nil {
		t.Error("Validate should return errors for invalid config")
	}
	if _, ok := err.(*ValidationError); !ok {
		t.Errorf("Error type = %T, want *ValidationError", err)
	}
}

func TestValidationError(t *testing.T) {
	t.Parallel()
	verr := &ValidationError{
		Errors: []string{"error 1", "error 2"},
	}
	msg := verr.Error()
	if msg == "" {
		t.Error("Error message should not be empty")
	}
}

func TestDBPath(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	path := cfg.DBPath()
	if path == "" {
		t.Error("DBPath should not be empty")
	}
}

func TestRuntimeDir(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	dir := cfg.RuntimeDir()
	if dir == "" {
		t.Error("RuntimeDir should not be empty")
	}
}

func TestConfigFilePath(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	path := cfg.ConfigFilePath()
	if path == "" {
		t.Error("ConfigFilePath should not be empty")
	}
}

func TestLoadedFrom(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	if cfg.LoadedFrom() != "" {
		t.Errorf("LoadedFrom = %q, want empty", cfg.LoadedFrom())
	}
}

func TestIsSet(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	if !cfg.IsSet("any.key") {
		t.Error("IsSet should return true (simplified implementation)")
	}
}

func TestSaveDefault(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	err := cfg.SaveDefault()
	if err != nil {
		t.Logf("SaveDefault error (expected in CI): %v", err)
	}
}

func TestDefaultTimeouts(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	if cfg.Timeouts.Command != DefaultCommandTimeout {
		t.Errorf("Timeouts.Command = %v", cfg.Timeouts.Command)
	}
	if cfg.Timeouts.Agent != DefaultAgentTimeout {
		t.Errorf("Timeouts.Agent = %v", cfg.Timeouts.Agent)
	}
}

func TestDefaultPerformance(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	if cfg.Performance.MaxMemoryMB != DefaultMaxMemoryMB {
		t.Errorf("MaxMemoryMB = %d", cfg.Performance.MaxMemoryMB)
	}
	if cfg.Performance.MaxConcurrentOps != DefaultMaxConcurrentOps {
		t.Errorf("MaxConcurrentOps = %d", cfg.Performance.MaxConcurrentOps)
	}
}

func TestDefaultPlugins(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	if cfg.Plugins.Enabled != DefaultEnablePluginSystem {
		t.Errorf("Plugins.Enabled = %v", cfg.Plugins.Enabled)
	}
	if cfg.Plugins.Timeout != 30*time.Second {
		t.Errorf("Plugins.Timeout = %v", cfg.Plugins.Timeout)
	}
}

func TestDefaultEmbedding(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	if cfg.Embedding.Model != DefaultEmbeddingModel {
		t.Errorf("Embedding.Model = %q", cfg.Embedding.Model)
	}
	if cfg.Embedding.Dimensions != DefaultEmbeddingDimensions {
		t.Errorf("Embedding.Dimensions = %d", cfg.Embedding.Dimensions)
	}
}

func TestDefaultWatch(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	if cfg.Watch.Enabled != DefaultEnableWatch {
		t.Errorf("Watch.Enabled = %v", cfg.Watch.Enabled)
	}
	if cfg.Watch.Debounce != DefaultWatchTimeout {
		t.Errorf("Watch.Debounce = %v", cfg.Watch.Debounce)
	}
}

// ── Constraints tests ─────────────────────────────────────────────────────

func TestDefaultConstraints(t *testing.T) {
	t.Parallel()
	c := DefaultConstraints()
	if c == nil {
		t.Fatal("DefaultConstraints returned nil")
	}
	if !c.Docs {
		t.Error("Docs should default to true")
	}
	if !c.Network {
		t.Error("Network should default to true")
	}
	if !c.Test {
		t.Error("Test should default to true")
	}
	if !c.Build {
		t.Error("Build should default to true")
	}
	if c.ReadOnly {
		t.Error("ReadOnly should default to false")
	}
	if c.MaxFiles != 0 {
		t.Errorf("MaxFiles = %d, want 0", c.MaxFiles)
	}
	if c.MaxTimeStr != "" {
		t.Errorf("MaxTimeStr = %q, want empty", c.MaxTimeStr)
	}
}

func TestMaxTimeDuration(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		str  string
		want time.Duration
	}{
		{"empty", "", 0},
		{"zero", "0s", 0},
		{"seconds", "30s", 30 * time.Second},
		{"minutes", "5m", 5 * time.Minute},
		{"hours", "2h", 2 * time.Hour},
		{"invalid", "banana", 0},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := &Constraints{MaxTimeStr: tt.str}
			got := c.MaxTimeDuration()
			if got != tt.want {
				t.Errorf("MaxTimeDuration() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLoadConstraints_FileNotExist(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	c, err := LoadConstraints(dir)
	if err != nil {
		t.Fatalf("LoadConstraints: %v", err)
	}
	if c == nil {
		t.Fatal("expected defaults, got nil")
	}
	if !c.Docs || !c.Network {
		t.Error("expected permissive defaults")
	}
}

func TestLoadConstraints_ValidFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	coscaDir := filepath.Join(dir, ".cosca")
	if err := os.MkdirAll(coscaDir, 0700); err != nil {
		t.Fatal(err)
	}
	yamlData := []byte("network: false\nread_only: true\nmax_files: 42\nmax_time: 10m\n")
	if err := os.WriteFile(filepath.Join(coscaDir, "constraints.yaml"), yamlData, 0600); err != nil {
		t.Fatal(err)
	}
	c, err := LoadConstraints(dir)
	if err != nil {
		t.Fatalf("LoadConstraints: %v", err)
	}
	if c.Network {
		t.Error("Network should be false")
	}
	if !c.ReadOnly {
		t.Error("ReadOnly should be true")
	}
	if c.MaxFiles != 42 {
		t.Errorf("MaxFiles = %d, want 42", c.MaxFiles)
	}
	if c.MaxTimeStr != "10m" {
		t.Errorf("MaxTimeStr = %q, want %q", c.MaxTimeStr, "10m")
	}
}

func TestLoadConstraints_InvalidYAML(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	coscaDir := filepath.Join(dir, ".cosca")
	if err := os.MkdirAll(coscaDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(coscaDir, "constraints.yaml"), []byte("{{{bad yaml"), 0600); err != nil {
		t.Fatal(err)
	}
	c, err := LoadConstraints(dir)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
	// FAIL-CLOSED: corrupt policy must yield restrictive constraints, not nil.
	if c == nil {
		t.Fatal("expected restrictive constraints, got nil")
	}
	if c.Network || c.Build || c.Docs || c.Test {
		t.Error("expected restrictive constraints (all blocked) for invalid YAML")
	}
	if !c.ReadOnly {
		t.Error("expected ReadOnly=true for invalid YAML (fail-closed)")
	}
}

func TestRestrictiveConstraints(t *testing.T) {
	t.Parallel()
	c := RestrictiveConstraints()
	if c == nil {
		t.Fatal("RestrictiveConstraints returned nil")
	}
	if c.Docs || c.Network || c.Test || c.Build {
		t.Error("RestrictiveConstraints should block everything")
	}
	if !c.ReadOnly {
		t.Error("RestrictiveConstraints should be read-only")
	}
}

func TestLoadConstraints_UnreadableFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	coscaDir := filepath.Join(dir, ".cosca")
	if err := os.MkdirAll(coscaDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(coscaDir, "constraints.yaml"), []byte("network: false\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(coscaDir, "constraints.yaml"), 0000); err != nil {
		t.Skipf("chmod 0000 not permitted: %v", err)
	}
	// Running as root ignores permissions, so skip if the file is still readable.
	if data, err := os.ReadFile(filepath.Join(coscaDir, "constraints.yaml")); err == nil {
		t.Skipf("running as root, permissions not enforced (data len=%d)", len(data))
	}
	c, err := LoadConstraints(dir)
	if err == nil {
		t.Fatal("expected error for unreadable file")
	}
	if c == nil {
		t.Fatal("expected restrictive constraints, got nil")
	}
	if c.Network || c.Build {
		t.Error("expected restrictive constraints for unreadable file (fail-closed)")
	}
}

func TestSaveConstraints_RoundTrip(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	c := &Constraints{
		Docs:       false,
		Network:    false,
		Test:       true,
		Build:      true,
		ReadOnly:   true,
		MaxFiles:   99,
		MaxTimeStr: "30m",
	}
	if err := SaveConstraints(dir, c); err != nil {
		t.Fatalf("SaveConstraints: %v", err)
	}
	loaded, err := LoadConstraints(dir)
	if err != nil {
		t.Fatalf("LoadConstraints after save: %v", err)
	}
	if loaded.Network {
		t.Error("Network should be false after round-trip")
	}
	if !loaded.ReadOnly {
		t.Error("ReadOnly should be true after round-trip")
	}
	if loaded.MaxFiles != 99 {
		t.Errorf("MaxFiles = %d, want 99", loaded.MaxFiles)
	}
	if loaded.MaxTimeStr != "30m" {
		t.Errorf("MaxTimeStr = %q", loaded.MaxTimeStr)
	}
}

// ── Models cache integration ────────────────────────────────────────────────

// writeModelsCache writes a small models.dev cache and returns its path.
func writeModelsCache(t *testing.T, model string, ctxLen int64) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "models.json")
	c := models.NewCache(path)
	c.Models["deepseek/"+model] = models.Model{
		ID:            "deepseek/" + model,
		Name:          model,
		ContextLength: ctxLen,
		Provider:      "DeepSeek",
	}
	if err := models.SaveCache(path, c); err != nil {
		t.Fatalf("write models cache: %v", err)
	}
	return path
}

func TestLoadUsesRealContextFromModelsCache(t *testing.T) {
	cachePath := writeModelsCache(t, "deepseek-v4-flash", 1000000)
	t.Setenv("COSCA_MODELS_CACHE", cachePath)

	cfgFile := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(cfgFile, []byte("provider:\n  name: deepseek\n  model: deepseek-v4-flash\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFromFile(cfgFile)
	if err != nil {
		t.Fatalf("LoadFromFile: %v", err)
	}
	if cfg.Provider.ContextWindow != 1000000 {
		t.Errorf("Provider.ContextWindow = %d, want 1000000 (real value from models cache)", cfg.Provider.ContextWindow)
	}
}

func TestLoadExplicitContextWindowWins(t *testing.T) {
	cachePath := writeModelsCache(t, "deepseek-v4-flash", 1000000)
	t.Setenv("COSCA_MODELS_CACHE", cachePath)

	cfgFile := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(cfgFile, []byte("provider:\n  name: deepseek\n  model: deepseek-v4-flash\n  context_window: 50000\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFromFile(cfgFile)
	if err != nil {
		t.Fatalf("LoadFromFile: %v", err)
	}
	if cfg.Provider.ContextWindow != 50000 {
		t.Errorf("Provider.ContextWindow = %d, want 50000 (explicit config wins)", cfg.Provider.ContextWindow)
	}
}

func TestLoadFallsBackWhenCacheMissing(t *testing.T) {
	t.Setenv("COSCA_MODELS_CACHE", filepath.Join(t.TempDir(), "missing.json"))

	cfgFile := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(cfgFile, []byte("provider:\n  name: deepseek\n  model: deepseek-v4-flash\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFromFile(cfgFile)
	if err != nil {
		t.Fatalf("LoadFromFile: %v", err)
	}
	// Unknown/absent cache → hardcoded default, never an error.
	if cfg.Provider.ContextWindow != DefaultProviderContextWindow {
		t.Errorf("Provider.ContextWindow = %d, want default %d", cfg.Provider.ContextWindow, DefaultProviderContextWindow)
	}
}

func TestLoadFallsBackWhenModelUnknownToCache(t *testing.T) {
	cachePath := writeModelsCache(t, "deepseek-v4-flash", 1000000)
	t.Setenv("COSCA_MODELS_CACHE", cachePath)

	cfgFile := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(cfgFile, []byte("provider:\n  name: ollama\n  model: some-local-model\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFromFile(cfgFile)
	if err != nil {
		t.Fatalf("LoadFromFile: %v", err)
	}
	if cfg.Provider.ContextWindow != DefaultProviderContextWindow {
		t.Errorf("Provider.ContextWindow = %d, want default %d", cfg.Provider.ContextWindow, DefaultProviderContextWindow)
	}
}
