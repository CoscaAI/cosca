package cli

import (
	"github.com/CoscaAI/cosca/internal/editors"
)

// Editors Adapter
// =============================================================================

// editorDetectorAdapter wraps editors.Manager to provide NewDetector/Detect methods.
type editorDetectorAdapter struct {
	inner *editors.Manager
}

// newEditorDetector creates a detector-like wrapper.
// Used in install.go where editors.NewDetector() is called.
func newEditorDetector() *editorDetectorAdapter {
	cfg := editors.DefaultEditorConfig(".")
	return &editorDetectorAdapter{inner: editors.NewManager(cfg)}
}

// Detect returns the editor name as a string. CLI expects just a string.
// Real Manager.Detect() returns (EditorInfo, error).
func (a *editorDetectorAdapter) Detect() string {
	if a.inner == nil {
		return ""
	}
	info, err := a.inner.Detect()
	if err != nil {
		return ""
	}
	return info.Name
}

// =============================================================================
