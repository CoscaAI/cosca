//
// Package types provides shared type definitions for the editor adapter system.
// This package is intentionally separate to avoid import cycles between
// the editor manager and individual editor adapters.

package types

import (
	"fmt"
	"time"
)

// =============================================================================
// EditorCapabilities
// =============================================================================

// EditorCapabilities describes what features an editor adapter supports.
type EditorCapabilities struct {
	SupportsContext        bool `json:"supports_context"`
	SupportsSearch         bool `json:"supports_search"`
	SupportsExecute        bool `json:"supports_execute"`
	SupportsWatch          bool `json:"supports_watch"`
	SupportsMCP            bool `json:"supports_mcp"`
	SupportsCustomCommands bool `json:"supports_custom_commands"`
	SupportsKeybindings    bool `json:"supports_keybindings"`
}

// =============================================================================
// EditorInfo
// =============================================================================

// EditorInfo provides a snapshot of a detected editor's metadata.
type EditorInfo struct {
	Name          string                 `json:"name"`
	Version       string                 `json:"version,omitempty"`
	Path          string                 `json:"path,omitempty"`
	Capabilities  EditorCapabilities     `json:"capabilities"`
	Config        map[string]interface{} `json:"config,omitempty"`
	Detected      bool                   `json:"detected"`
	SetupRequired bool                   `json:"setup_required"`
	SetupComplete bool                   `json:"setup_complete"`
}

// =============================================================================
// EditorConfig
// =============================================================================

// EditorConfig holds configuration for setting up an editor adapter.
type EditorConfig struct {
	AutoSetup      bool                   `json:"auto_setup"`
	ProjectDir     string                 `json:"project_dir"`
	CoscaBinPath   string                 `json:"cosca_bin_path"`
	CustomConfig   map[string]interface{} `json:"custom_config,omitempty"`
	DryRun         bool                   `json:"dry_run"`
	BackupExisting bool                   `json:"backup_existing"`
}

// DefaultEditorConfig returns a default editor configuration.
func DefaultEditorConfig(projectDir string) EditorConfig {
	return EditorConfig{
		AutoSetup:      false,
		ProjectDir:     projectDir,
		CoscaBinPath:   "cosca",
		BackupExisting: true,
	}
}

// =============================================================================
// Editor Interface
// =============================================================================

// Editor defines the interface that all editor adapters must implement.
type Editor interface {
	Name() string
	Version() (string, error)
	Detect() (bool, error)
	Setup(config EditorConfig) error
	Validate() error
	Teardown() error
	Capabilities() EditorCapabilities
	Info() (EditorInfo, error)
}

// =============================================================================
// BaseEditor
// =============================================================================

// BaseEditor provides default implementations for the Editor interface.
// Embed this struct in adapter implementations and override as needed.
type BaseEditor struct {
	NameValue       string
	Config          EditorConfig
	CapabilitiesVal EditorCapabilities
}

// Name returns the editor name.
func (b *BaseEditor) Name() string {
	return b.NameValue
}

// Capabilities returns the editor's capabilities.
func (b *BaseEditor) Capabilities() EditorCapabilities {
	return b.CapabilitiesVal
}

// =============================================================================
// Errors
// =============================================================================

// ErrEditorNotDetected is returned when no editor is detected.
var (
	ErrEditorNotDetected   = fmt.Errorf("editor not detected")
	ErrEditorNotConfigured = fmt.Errorf("Cosca integration not configured")
)

// ErrEditorSetupFailed is returned when editor setup encounters an error.
type ErrEditorSetupFailed struct {
	Editor string
	Err    error
}

func (e *ErrEditorSetupFailed) Error() string {
	return fmt.Sprintf("editor %q setup failed: %v", e.Editor, e.Err)
}

func (e *ErrEditorSetupFailed) Unwrap() error {
	return e.Err
}

// ErrEditorValidationFailed is returned when editor validation fails.
type ErrEditorValidationFailed struct {
	Editor string
	Reason string
	Err    error
}

func (e *ErrEditorValidationFailed) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("editor %q validation failed: %s: %v", e.Editor, e.Reason, e.Err)
	}
	return fmt.Sprintf("editor %q validation failed: %s", e.Editor, e.Reason)
}

func (e *ErrEditorValidationFailed) Unwrap() error {
	return e.Err
}

// =============================================================================
// Timer
// =============================================================================

// Timer is a simple helper for tracking operation duration.
type Timer struct {
	start time.Time
	name  string
}

// StartTimer starts a named timer.
func StartTimer(name string) Timer {
	return Timer{start: time.Now(), name: name}
}

// Elapsed returns the duration since the timer was started.
func (t Timer) Elapsed() time.Duration {
	return time.Since(t.start)
}
