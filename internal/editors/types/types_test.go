package types

import (
	"errors"
	"testing"
	"time"
)

func TestEditorCapabilities(t *testing.T) {
	t.Parallel()
	ec := EditorCapabilities{
		SupportsContext:        true,
		SupportsSearch:         true,
		SupportsExecute:        false,
		SupportsWatch:          false,
		SupportsMCP:            true,
		SupportsCustomCommands: false,
		SupportsKeybindings:    false,
	}
	if !ec.SupportsContext {
		t.Error("SupportsContext should be true")
	}
	if !ec.SupportsSearch {
		t.Error("SupportsSearch should be true")
	}
	if ec.SupportsExecute {
		t.Error("SupportsExecute should be false")
	}
	if !ec.SupportsMCP {
		t.Error("SupportsMCP should be true")
	}
}

func TestEditorCapabilitiesAllFalse(t *testing.T) {
	t.Parallel()
	ec := EditorCapabilities{}
	if ec.SupportsContext {
		t.Error("default SupportsContext should be false")
	}
}

func TestEditorInfo(t *testing.T) {
	t.Parallel()
	info := EditorInfo{
		Name:          "VS Code",
		Version:       "1.85.0",
		Path:          "/usr/bin/code",
		Detected:      true,
		SetupRequired: true,
		SetupComplete: false,
	}
	if info.Name != "VS Code" {
		t.Errorf("Name = %q", info.Name)
	}
	if info.Version != "1.85.0" {
		t.Errorf("Version = %q", info.Version)
	}
	if !info.Detected {
		t.Error("Detected should be true")
	}
}

func TestEditorConfig(t *testing.T) {
	t.Parallel()
	cfg := EditorConfig{
		AutoSetup:      true,
		ProjectDir:     "/project",
		CoscaBinPath:   "/usr/bin/cosca",
		DryRun:         false,
		BackupExisting: true,
	}
	if !cfg.AutoSetup {
		t.Error("AutoSetup should be true")
	}
	if cfg.ProjectDir != "/project" {
		t.Errorf("ProjectDir = %q", cfg.ProjectDir)
	}
}

func TestDefaultEditorConfig(t *testing.T) {
	t.Parallel()
	cfg := DefaultEditorConfig("/workspace")
	if cfg.AutoSetup {
		t.Error("Default AutoSetup should be false")
	}
	if cfg.ProjectDir != "/workspace" {
		t.Errorf("ProjectDir = %q", cfg.ProjectDir)
	}
	if cfg.CoscaBinPath != "cosca" {
		t.Errorf("CoscaBinPath = %q", cfg.CoscaBinPath)
	}
	if !cfg.BackupExisting {
		t.Error("Default BackupExisting should be true")
	}
}

func TestEditorInterface(t *testing.T) {
	t.Parallel()
	var editor Editor = &MockEditor{}
	if editor.Name() != "mock-editor" {
		t.Errorf("Name = %q", editor.Name())
	}
}

func TestBaseEditor(t *testing.T) {
	t.Parallel()
	be := &BaseEditor{
		NameValue: "test-editor",
		CapabilitiesVal: EditorCapabilities{
			SupportsContext: true,
		},
	}
	if be.Name() != "test-editor" {
		t.Errorf("Name = %q", be.Name())
	}
	if !be.Capabilities().SupportsContext {
		t.Error("SupportsContext should be true")
	}
}

func TestErrorEditorNotDetected(t *testing.T) {
	t.Parallel()
	if ErrEditorNotDetected.Error() != "editor not detected" {
		t.Errorf("Error = %q", ErrEditorNotDetected.Error())
	}
}

func TestErrorEditorNotConfigured(t *testing.T) {
	t.Parallel()
	if ErrEditorNotConfigured.Error() != "Cosca integration not configured" {
		t.Errorf("Error = %q", ErrEditorNotConfigured.Error())
	}
}

func TestErrEditorSetupFailed(t *testing.T) {
	t.Parallel()
	cause := errors.New("permission denied")
	err := &ErrEditorSetupFailed{
		Editor: "vscode",
		Err:    cause,
	}
	if err.Error() == "" {
		t.Error("Error should not be empty")
	}
	if !errors.Is(err, cause) {
		t.Error("Unwrap should return the cause")
	}
}

func TestErrEditorValidationFailed(t *testing.T) {
	t.Parallel()
	err := &ErrEditorValidationFailed{
		Editor: "neovim",
		Reason: "missing config",
	}
	if err.Error() == "" {
		t.Error("Error should not be empty")
	}
}

func TestErrEditorValidationFailedWithErr(t *testing.T) {
	t.Parallel()
	cause := errors.New("file not found")
	err := &ErrEditorValidationFailed{
		Editor: "neovim",
		Reason: "config check",
		Err:    cause,
	}
	if !errors.Is(err, cause) {
		t.Error("Unwrap should return the cause")
	}
}

func TestTimer(t *testing.T) {
	t.Parallel()
	timer := StartTimer("test-op")
	if timer.Elapsed() < 0 {
		t.Error("Elapsed should be >= 0")
	}
	<-time.After(time.Millisecond)
	if timer.Elapsed() <= 0 {
		t.Error("Elapsed should be > 0 after sleep")
	}
}

func TestMockEditorDefaults(t *testing.T) {
	t.Parallel()
	m := &MockEditor{}
	if m.Name() != "mock-editor" {
		t.Errorf("Name = %q", m.Name())
	}
	version, err := m.Version()
	if err != nil {
		t.Fatalf("Version error: %v", err)
	}
	if version != "1.0.0" {
		t.Errorf("Version = %q", version)
	}
	detected, err := m.Detect()
	if err != nil {
		t.Fatalf("Detect error: %v", err)
	}
	if !detected {
		t.Error("Detect should be true")
	}
	if err := m.Setup(EditorConfig{}); err != nil {
		t.Errorf("Setup error: %v", err)
	}
	if err := m.Validate(); err != nil {
		t.Errorf("Validate error: %v", err)
	}
	if err := m.Teardown(); err != nil {
		t.Errorf("Teardown error: %v", err)
	}
	info, err := m.Info()
	if err != nil {
		t.Fatalf("Info error: %v", err)
	}
	if info.Name != "mock-editor" {
		t.Errorf("Info.Name = %q", info.Name)
	}
}
