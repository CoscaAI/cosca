package editors

import (
	"testing"

	"github.com/CoscaAI/cosca/internal/editors/types"
)

// mockEditor implements types.Editor for testing.
type mockEditor struct {
	types.BaseEditor
	detectResult  bool
	detectErr     error
	setupErr      error
	validateErr   error
	teardownErr   error
	versionResult string
	versionErr    error
	infoResult    types.EditorInfo
	infoErr       error
}

func (m *mockEditor) Detect() (bool, error)            { return m.detectResult, m.detectErr }
func (m *mockEditor) Setup(_ types.EditorConfig) error { return m.setupErr }
func (m *mockEditor) Validate() error                  { return m.validateErr }
func (m *mockEditor) Teardown() error                  { return m.teardownErr }
func (m *mockEditor) Version() (string, error)         { return m.versionResult, m.versionErr }
func (m *mockEditor) Info() (types.EditorInfo, error)  { return m.infoResult, m.infoErr }

func TestEditorInterfaceCompiles(_ *testing.T) {
	// Compile-time check: ensures mockEditor fully satisfies the types.Editor
	// interface. If mockEditor does not implement all required methods, this
	// test will fail at compile time. No runtime assertion is necessary.
	var _ types.Editor = (*mockEditor)(nil)
}

func TestEditorCapabilitiesReExport(t *testing.T) {
	t.Parallel()
	ec := EditorCapabilities{
		SupportsContext: true,
		SupportsMCP:     true,
	}
	if !ec.SupportsContext {
		t.Error("SupportsContext should be true")
	}
}

func TestEditorInfoReExport(t *testing.T) {
	t.Parallel()
	info := EditorInfo{
		Name:     "Test Editor",
		Detected: true,
	}
	if info.Name != "Test Editor" {
		t.Errorf("Name = %q", info.Name)
	}
}

func TestBaseEditorReExport(t *testing.T) {
	t.Parallel()
	be := BaseEditor{
		NameValue: "re-exported",
	}
	if be.Name() != "re-exported" {
		t.Errorf("Name = %q", be.Name())
	}
}

func TestTimerReExport(t *testing.T) {
	t.Parallel()
	timer := StartTimer("test")
	if timer.Elapsed() < 0 {
		t.Error("Elapsed should be >= 0")
	}
}

func TestDefaultEditorConfigReExport(t *testing.T) {
	t.Parallel()
	cfg := DefaultEditorConfig("/proj")
	if cfg.ProjectDir != "/proj" {
		t.Errorf("ProjectDir = %q", cfg.ProjectDir)
	}
}
