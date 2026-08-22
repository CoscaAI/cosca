package editors

import (
	"testing"

	"github.com/CoscaAI/cosca/internal/editors/types"
)

func TestNewManager(t *testing.T) {
	t.Parallel()
	m := NewManager(types.DefaultEditorConfig(t.TempDir()))
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	if len(m.adapters) == 0 {
		t.Error("adapters should be registered")
	}
}

func TestManagerGetAdapter(t *testing.T) {
	t.Parallel()
	m := NewManager(types.DefaultEditorConfig(t.TempDir()))
	// Should return an editor adapter (even if mock-like)
	adapter, err := m.GetAdapter("opencode")
	if err != nil {
		t.Logf("GetAdapter error (expected if not registered): %v", err)
	}
	if adapter != nil {
		t.Logf("Found adapter: %s", adapter.Name())
	}
}

func TestManagerGetAdapterNotFound(t *testing.T) {
	t.Parallel()
	m := NewManager(types.DefaultEditorConfig(t.TempDir()))
	_, err := m.GetAdapter("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent adapter")
	}
}

func TestManagerList(t *testing.T) {
	t.Parallel()
	m := NewManager(types.DefaultEditorConfig(t.TempDir()))
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	editors := m.List()
	if editors == nil {
		t.Error("List() returned nil — expected a slice (possibly empty)")
	} else {
		t.Logf("Found %d registered editors", len(editors))
	}
}

func TestManagerSetupUnknownEditor(t *testing.T) {
	t.Parallel()
	m := NewManager(types.DefaultEditorConfig(t.TempDir()))
	err := m.Setup("nonexistent")
	if err == nil {
		t.Error("Expected error for unknown editor")
	}
}

func TestManagerValidateUnknownEditor(t *testing.T) {
	t.Parallel()
	m := NewManager(types.DefaultEditorConfig(t.TempDir()))
	err := m.Validate("not-real")
	if err == nil {
		t.Error("Expected error for unknown editor")
	}
}

func TestManagerTeardownUnknownEditor(t *testing.T) {
	t.Parallel()
	m := NewManager(types.DefaultEditorConfig(t.TempDir()))
	err := m.Teardown("ghost-editor")
	if err == nil {
		t.Error("Expected error for unknown editor")
	}
}

func TestManagerDetect(t *testing.T) {
	t.Parallel()
	m := NewManager(types.DefaultEditorConfig(t.TempDir()))
	info, err := m.Detect()
	if err != nil {
		// It's OK if no editor is detected (CI environment)
		t.Logf("Detect returned error (expected in CI): %v", err)
	} else {
		t.Logf("Detected editor: %s", info.Name)
	}
}

func TestManagerDetectAll(t *testing.T) {
	t.Parallel()
	m := NewManager(types.DefaultEditorConfig(t.TempDir()))
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	results := m.DetectAll()
	if results == nil {
		t.Error("DetectAll() returned nil — expected a slice (possibly empty in CI)")
	} else {
		t.Logf("DetectAll returned %d results", len(results))
	}
}

func TestManagerAutoDetectAndSetup(t *testing.T) {
	t.Parallel()
	m := NewManager(types.DefaultEditorConfig(t.TempDir()))
	info, err := m.AutoDetectAndSetup()
	if err != nil {
		// Expected in CI environment
		t.Logf("AutoDetectAndSetup error (expected in CI): %v", err)
	} else {
		t.Logf("Auto detected: %s", info.Name)
	}
}

func TestErrEditorNotDetected(t *testing.T) {
	t.Parallel()
	if ErrEditorNotDetected != types.ErrEditorNotDetected {
		t.Error("ErrEditorNotDetected should be re-exported from types")
	}
}

func TestErrEditorNotConfigured(t *testing.T) {
	t.Parallel()
	if ErrEditorNotConfigured != types.ErrEditorNotConfigured {
		t.Error("ErrEditorNotConfigured should be re-exported from types")
	}
}

func TestEditorTypeAliases(t *testing.T) {
	t.Parallel()
	// Compile-time check: verify that the Editor type alias from types
	// can be assigned nil (interface type) without compilation errors.
	// No runtime assertion needed — this test ensures the type alias compiles.
	var editor types.Editor
	_ = editor
}

func TestConvenienceFunctions(t *testing.T) {
	t.Parallel()
	cfg := DefaultEditorConfig("/test")
	if cfg.ProjectDir != "/test" {
		t.Errorf("ProjectDir = %q", cfg.ProjectDir)
	}
}

func TestManagerConfigPassThrough(t *testing.T) {
	t.Parallel()
	cfg := types.DefaultEditorConfig("/custom/project")
	m := NewManager(cfg)
	if m.config.ProjectDir != "/custom/project" {
		t.Errorf("config.ProjectDir = %q", m.config.ProjectDir)
	}
}

func TestManagerNilAdapters(t *testing.T) {
	t.Parallel()
	m := &Manager{
		adapters: make(map[string]types.Editor),
		config:   types.DefaultEditorConfig(t.TempDir()),
		detected: make(map[string]types.EditorInfo),
	}
	_, err := m.GetAdapter("test")
	if err == nil {
		t.Error("Expected error for empty manager")
	}
}
