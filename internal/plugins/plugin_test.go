package plugins

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestPluginStateString(t *testing.T) {
	t.Parallel()
	tests := []struct {
		state PluginState
		want  string
	}{
		{PluginStateInstalled, "installed"},
		{PluginStateInitialized, "initialized"},
		{PluginStateStarted, "started"},
		{PluginStateStopped, "stopped"},
		{PluginStateError, "error"},
		{PluginState(99), "unknown(99)"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.state.String(); got != tt.want {
				t.Errorf("PluginState.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPluginRuntimeConstants(t *testing.T) {
	t.Parallel()
	tests := []struct {
		runtime PluginRuntime
		want    string
	}{
		{RuntimeGo, "go"},
		{RuntimeWASM, "wasm"},
		{RuntimeExternal, "external"},
		{RuntimeSharedLib, "sharedlib"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if string(tt.runtime) != tt.want {
				t.Errorf("PluginRuntime = %q, want %q", string(tt.runtime), tt.want)
			}
		})
	}
}

func TestPluginManifestValidate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		manifest PluginManifest
		wantErr  bool
	}{
		{
			name:     "empty id",
			manifest: PluginManifest{Name: "test", Version: "1.0", Runtime: RuntimeGo},
			wantErr:  true,
		},
		{
			name:     "empty name",
			manifest: PluginManifest{ID: "test", Version: "1.0", Runtime: RuntimeGo},
			wantErr:  true,
		},
		{
			name:     "empty version",
			manifest: PluginManifest{ID: "test", Name: "Test", Runtime: RuntimeGo},
			wantErr:  true,
		},
		{
			name:     "empty runtime",
			manifest: PluginManifest{ID: "test", Name: "Test", Version: "1.0"},
			wantErr:  true,
		},
		{
			name:     "invalid runtime",
			manifest: PluginManifest{ID: "test", Name: "Test", Version: "1.0", Runtime: "invalid"},
			wantErr:  true,
		},
		{
			name:     "valid manifest",
			manifest: PluginManifest{ID: "test", Name: "Test", Version: "1.0", Runtime: RuntimeGo},
			wantErr:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.manifest.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestPluginManifestValidatePermissions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		permissions []string
		wantErr     bool
	}{
		{"valid: network", []string{"network"}, false},
		{"valid: filesystem", []string{"filesystem"}, false},
		{"valid: multiple", []string{"network", "exec", "environment"}, false},
		{"valid: all", []string{"all"}, false},
		{"invalid: unknown", []string{"unknown_perm"}, true},
		{"valid: none", []string{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := PluginManifest{
				ID: "test", Name: "Test", Version: "1.0",
				Runtime: RuntimeWASM, Permissions: tt.permissions,
			}
			err := m.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestPluginManifestValidateWithRuntimes(t *testing.T) {
	t.Parallel()
	runtimes := []PluginRuntime{RuntimeGo, RuntimeWASM, RuntimeExternal, RuntimeSharedLib}
	for _, rt := range runtimes {
		t.Run(string(rt), func(t *testing.T) {
			m := PluginManifest{
				ID: "test", Name: "Test", Version: "1.0", Runtime: rt,
			}
			if err := m.Validate(); err != nil {
				t.Errorf("Validate() with runtime %q failed: %v", rt, err)
			}
		})
	}
}

func TestPluginDependency(t *testing.T) {
	t.Parallel()
	dep := PluginDependency{
		PluginID: "dep-plugin",
		Version:  ">=1.0.0",
		Optional: true,
	}
	if dep.PluginID != "dep-plugin" {
		t.Errorf("PluginID = %q", dep.PluginID)
	}
	if !dep.Optional {
		t.Error("Optional should be true")
	}
}

func TestPluginHealth(t *testing.T) {
	t.Parallel()
	h := PluginHealth{
		PluginID:  "test-plugin",
		Status:    "healthy",
		LastCheck: time.Now(),
		State:     PluginStateStarted,
	}
	if !h.IsHealthy() {
		t.Error("IsHealthy should return true")
	}
	if h.PluginID != "test-plugin" {
		t.Errorf("PluginID = %q", h.PluginID)
	}
}

func TestPluginHealthUnhealthy(t *testing.T) {
	t.Parallel()
	h := PluginHealth{Status: "unhealthy"}
	if h.IsHealthy() {
		t.Error("IsHealthy should return false for unhealthy status")
	}
}

func TestPluginInfo(t *testing.T) {
	t.Parallel()
	info := PluginInfo{
		Manifest: PluginManifest{
			ID: "test", Name: "Test Plugin", Version: "1.0.0",
		},
		State:   PluginStateInstalled,
		Enabled: true,
	}
	if info.Manifest.ID != "test" {
		t.Errorf("Manifest.ID = %q", info.Manifest.ID)
	}
	if !info.Enabled {
		t.Error("Enabled should be true")
	}
}

func TestBasePlugin(t *testing.T) {
	t.Parallel()
	bp := &BasePlugin{
		IDValue:      "base-plugin",
		NameValue:    "Base Plugin",
		VersionValue: "0.1.0",
		DescValue:    "A test plugin",
		AuthorValue:  "Cosca Team",
	}
	if bp.ID() != "base-plugin" {
		t.Errorf("ID = %q", bp.ID())
	}
	if bp.Name() != "Base Plugin" {
		t.Errorf("Name = %q", bp.Name())
	}
	if bp.Version() != "0.1.0" {
		t.Errorf("Version = %q", bp.Version())
	}
	if bp.Description() != "A test plugin" {
		t.Errorf("Description = %q", bp.Description())
	}
	if bp.Author() != "Cosca Team" {
		t.Errorf("Author = %q", bp.Author())
	}
}

func TestBasePluginLifecycle(t *testing.T) {
	t.Parallel()
	bp := &BasePlugin{IDValue: "test"}
	ctx := &PluginContext{Config: make(map[string]interface{})}

	if err := bp.Init(ctx); err != nil {
		t.Fatalf("Init error: %v", err)
	}
	if bp.State != PluginStateInitialized {
		t.Errorf("State after Init = %v, want %v", bp.State, PluginStateInitialized)
	}

	if err := bp.Start(); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	if bp.State != PluginStateStarted {
		t.Errorf("State after Start = %v, want %v", bp.State, PluginStateStarted)
	}
	if bp.StartedAt.IsZero() {
		t.Error("StartedAt should be set after Start")
	}

	if err := bp.Stop(); err != nil {
		t.Fatalf("Stop error: %v", err)
	}
	if bp.State != PluginStateStopped {
		t.Errorf("State after Stop = %v, want %v", bp.State, PluginStateStopped)
	}
}

func TestBasePluginHealth(t *testing.T) {
	t.Parallel()
	bp := &BasePlugin{IDValue: "test"}
	health, err := bp.Health()
	if err != nil {
		t.Fatalf("Health error: %v", err)
	}
	if health.PluginID != "test" {
		t.Errorf("PluginID = %q", health.PluginID)
	}
	if health.Status != "healthy" {
		t.Errorf("Status = %q", health.Status)
	}
}

func TestBasePluginHealthWithUptime(t *testing.T) {
	t.Parallel()
	bp := &BasePlugin{IDValue: "test"}
	bp.StartedAt = time.Now().Add(-time.Hour)
	health, _ := bp.Health()
	if health.Uptime < time.Hour {
		t.Errorf("Uptime should be >= 1h, got %v", health.Uptime)
	}
}

func TestPluginContext(t *testing.T) {
	t.Parallel()
	cfg := map[string]interface{}{"key": "value"}
	logger := &MockPluginLogger{}
	api := &MockRuntimeAPI{}
	ctx := NewPluginContext(cfg, logger, "/data/dir", api)
	if ctx == nil {
		t.Fatal("NewPluginContext returned nil")
	}
	if ctx.DataDir != "/data/dir" {
		t.Errorf("DataDir = %q", ctx.DataDir)
	}
	if ctx.TempDir != "/data/dir/tmp" {
		t.Errorf("TempDir = %q", ctx.TempDir)
	}
}

func TestPluginContextGetConfig(t *testing.T) {
	t.Parallel()
	ctx := NewPluginContext(
		map[string]interface{}{"key1": "value1", "key2": 42},
		&MockPluginLogger{}, "/tmp", &MockRuntimeAPI{},
	)
	val, err := ctx.GetConfig("key1")
	if err != nil {
		t.Fatalf("GetConfig error: %v", err)
	}
	if val != "value1" {
		t.Errorf("val = %v, want %v", val, "value1")
	}
}

func TestPluginContextGetConfigMissing(t *testing.T) {
	t.Parallel()
	ctx := NewPluginContext(map[string]interface{}{}, &MockPluginLogger{}, "/tmp", &MockRuntimeAPI{})
	_, err := ctx.GetConfig("nonexistent")
	if err == nil {
		t.Error("Expected error for missing config key")
	}
}

func TestPluginContextSetConfig(t *testing.T) {
	t.Parallel()
	ctx := NewPluginContext(nil, &MockPluginLogger{}, "/tmp", &MockRuntimeAPI{})
	ctx.SetConfig("new_key", "new_value")
	val, err := ctx.GetConfig("new_key")
	if err != nil {
		t.Fatalf("GetConfig error: %v", err)
	}
	if val != "new_value" {
		t.Errorf("val = %v", val)
	}
}

func TestErrorTypes(t *testing.T) {
	t.Parallel()
	if ErrPluginNotFound.Error() != "plugin not found" {
		t.Errorf("ErrPluginNotFound = %q", ErrPluginNotFound.Error())
	}
	if ErrPluginNotLoaded.Error() != "plugin not loaded" {
		t.Errorf("ErrPluginNotLoaded = %q", ErrPluginNotLoaded.Error())
	}
	if ErrPluginAlreadyInstalled.Error() != "plugin already installed" {
		t.Errorf("ErrPluginAlreadyInstalled = %q", ErrPluginAlreadyInstalled.Error())
	}
	if ErrPluginDisabled.Error() != "plugin is disabled" {
		t.Errorf("ErrPluginDisabled = %q", ErrPluginDisabled.Error())
	}
}

func TestErrDependencyMissing(t *testing.T) {
	t.Parallel()
	err := &ErrDependencyMissing{
		PluginID: "my-plugin",
		Dependency: PluginDependency{
			PluginID: "dep-plugin",
			Version:  ">=1.0",
		},
	}
	msg := err.Error()
	if msg == "" {
		t.Error("Error message should not be empty")
	}
}

func TestErrPermissionDenied(t *testing.T) {
	t.Parallel()
	err := &ErrPermissionDenied{
		PluginID:   "my-plugin",
		Permission: "network",
	}
	msg := err.Error()
	if msg == "" {
		t.Error("Error message should not be empty")
	}
}

func TestMockPlugin(t *testing.T) {
	t.Parallel()
	p := &MockPlugin{
		IDValue:          "mock",
		NameValue:        "Mock Plugin",
		VersionValue:     "0.0.1",
		DescriptionValue: "A mock",
		AuthorValue:      "Tester",
	}
	if p.ID() != "mock" {
		t.Errorf("ID = %q", p.ID())
	}
	if p.Name() != "Mock Plugin" {
		t.Errorf("Name = %q", p.Name())
	}
	if err := p.Init(&PluginContext{}); err != nil {
		t.Errorf("Init error: %v", err)
	}
	if err := p.Start(); err != nil {
		t.Errorf("Start error: %v", err)
	}
	if err := p.Stop(); err != nil {
		t.Errorf("Stop error: %v", err)
	}
	health, err := p.Health()
	if err != nil {
		t.Fatalf("Health error: %v", err)
	}
	if health.PluginID != "mock" {
		t.Errorf("Health PluginID = %q", health.PluginID)
	}
}

func TestMockRuntimeAPI(t *testing.T) {
	t.Parallel()
	api := &MockRuntimeAPI{}
	val, err := api.GetConfig("key")
	if err != nil {
		t.Errorf("GetConfig error: %v", err)
	}
	if val != nil {
		t.Errorf("val = %v", val)
	}
	if err := api.SetConfig("k", "v"); err != nil {
		t.Errorf("SetConfig error: %v", err)
	}
	if err := api.EmitEvent("test", nil); err != nil {
		t.Errorf("EmitEvent error: %v", err)
	}
	id, err := api.RegisterHook("hook", nil)
	if err != nil {
		t.Errorf("RegisterHook error: %v", err)
	}
	if id != "hook-id" {
		t.Errorf("hook id = %q", id)
	}
	if err := api.UnregisterHook("hid"); err != nil {
		t.Errorf("UnregisterHook error: %v", err)
	}
}

// =============================================================================
// WASM Plugin Loading Tests
// =============================================================================

func TestWASMPlugin_Load_InvalidPath(t *testing.T) {
	t.Parallel()
	loader := NewLoader(DefaultLoaderConfig(t.TempDir()))
	manifest := &PluginManifest{
		ID: "test", Name: "Test", Version: "1.0.0",
		Runtime: RuntimeWASM,
	}
	_, err := loader.loadWASMPlugin(manifest, "/nonexistent/path/plugin.wasm")
	if err == nil {
		t.Error("expected error for non-existent wasm path")
	}
}

func TestWASMPlugin_Load_InvalidExtension(t *testing.T) {
	t.Parallel()
	loader := NewLoader(DefaultLoaderConfig(t.TempDir()))
	tmpFile := filepath.Join(t.TempDir(), "plugin.txt")
	if err := os.WriteFile(tmpFile, []byte("not wasm"), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	manifest := &PluginManifest{
		ID: "test", Name: "Test", Version: "1.0.0",
		Runtime: RuntimeWASM,
	}
	_, err := loader.loadWASMPlugin(manifest, tmpFile)
	if err == nil {
		t.Error("expected error for non-.wasm extension")
	}
}

// =============================================================================
// External Plugin Loading Tests
// =============================================================================

func TestExternalPlugin_Load_NotExecutable(t *testing.T) {
	t.Parallel()
	loader := NewLoader(DefaultLoaderConfig(t.TempDir()))
	tmpFile := filepath.Join(t.TempDir(), "plugin.sh")
	if err := os.WriteFile(tmpFile, []byte("#!/bin/sh"), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	manifest := &PluginManifest{
		ID: "test", Name: "Test", Version: "1.0.0",
		Runtime: RuntimeExternal,
	}
	_, err := loader.loadExternalPlugin(manifest, tmpFile)
	if err == nil {
		t.Error("expected error for non-executable binary")
	}
}

func TestExternalPlugin_Init_WithContext(t *testing.T) {
	if runtime.GOOS == "windows" {
		// O teste depende de: (1) bits de permissão POSIX 0o755 para marcar o
		// binário como executável (não existem no Windows — chmod vira apenas
		// read-only) e (2) shebang /bin/sh para rodar o script (shell POSIX
		// inexistente por padrão no Windows).
		t.Skip("permissão de execução POSIX e shebang /bin/sh — não aplicável no Windows")
	}
	// Create a helper script that implements the JSON protocol
	script := `#!/bin/sh
# Read init message and respond
read line
echo '{"type": "init_ok", "version": "1.0.0"}'
# Read start message and respond
read line
echo '{"type": "start_ok"}'
# Keep reading until stdin closes
while read line; do :; done
`
	scriptPath := filepath.Join(t.TempDir(), "test-plugin.sh")
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write test script: %v", err)
	}

	cfg := DefaultLoaderConfig(t.TempDir())
	cfg.Sandbox.SeccompEnabled = false // shell scripts need broader syscall access
	loader := NewLoader(cfg)
	manifest := &PluginManifest{
		ID: "test-plugin", Name: "Test Plugin", Version: "1.0.0",
		Runtime: RuntimeExternal,
	}

	p, err := loader.loadExternalPlugin(manifest, scriptPath)
	if err != nil {
		t.Fatalf("loadExternalPlugin error: %v", err)
	}

	ctx := NewPluginContext(
		map[string]interface{}{"key": "value"},
		&MockPluginLogger{},
		"/tmp/test-data",
		&MockRuntimeAPI{},
	)

	if err := p.Init(ctx); err != nil {
		t.Fatalf("Init error: %v", err)
	}

	if err := p.Start(); err != nil {
		t.Fatalf("Start error: %v", err)
	}

	if err := p.Stop(); err != nil {
		t.Fatalf("Stop error: %v", err)
	}
}
