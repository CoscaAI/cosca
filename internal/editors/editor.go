// Package editors provides the editor adapter system for the Cosca platform.
// It supports automatic detection, setup, validation, and teardown of
// Cosca integration across multiple editors (OpenCode, Claude Code,
// Codex, Cursor, VS Code, Neovim, and generic MCP-compatible editors).
//
// The core types (Editor interface, BaseEditor, EditorConfig, EditorInfo,
// EditorCapabilities) are in the sub-package "types" to avoid import cycles
// between the manager and individual adapter implementations.
package editors

import (
	// Re-export types for convenience
	"github.com/CoscaAI/cosca/internal/editors/types"
)

// Re-exported types for backward compatibility.

// Editor is the interface for editor adapters.
type Editor = types.Editor

// EditorCapabilities describes what features an editor adapter supports.
type EditorCapabilities = types.EditorCapabilities

// EditorInfo provides metadata about a detected editor.
type EditorInfo = types.EditorInfo

// EditorConfig holds configuration for setting up an editor.
type EditorConfig = types.EditorConfig

// BaseEditor provides default Editor interface implementations.
type BaseEditor = types.BaseEditor

// Timer tracks operation duration for editor operations.
type Timer = types.Timer

// Re-exported errors.
var (
	ErrEditorNotDetected   = types.ErrEditorNotDetected
	ErrEditorNotConfigured = types.ErrEditorNotConfigured
)

// Re-exported error types.

// ErrEditorSetupFailed is returned when editor setup encounters an error.
type ErrEditorSetupFailed = types.ErrEditorSetupFailed

// ErrEditorValidationFailed is returned when editor validation fails.
type ErrEditorValidationFailed = types.ErrEditorValidationFailed

// Convenience functions.
var (
	DefaultEditorConfig = types.DefaultEditorConfig
	StartTimer          = types.StartTimer
)
