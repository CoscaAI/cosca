//
// Unit tests for the validate command.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// =============================================================================
// Validate Command — Properties
// =============================================================================

func TestValidateCommand_Properties(t *testing.T) {
	cmd := NewValidateCommand()

	if cmd.Use != "validate" {
		t.Errorf("Use = %q, want %q", cmd.Use, "validate")
	}
	if cmd.Short == "" {
		t.Error("Short should not be empty")
	}
	if cmd.Long == "" {
		t.Error("Long should not be empty")
	}
	if len(cmd.Example) == 0 {
		t.Error("Example should not be empty")
	}
}

func TestValidateCommand_NoArgsRequired(t *testing.T) {
	// The command uses cobra.NoArgs — passing args should error.
	cmd := NewValidateCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"extra"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when passing arguments (NoArgs)")
	}
}

// =============================================================================
// Validate Command — No .cosca Directory
// =============================================================================

func TestValidateCommand_NoCoscaDirectory(t *testing.T) {
	// Cannot run parallel — changes working directory

	origWd, err := os.Getwd()
	if err != nil {
		t.Skipf("cannot get working directory: %v", err)
	}
	defer func() { _ = os.Chdir(origWd) }()

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Skipf("cannot change to temp dir: %v", err)
	}

	cmd := NewValidateCommand()
	var buf bytes.Buffer
	formatter := NewOutputFormatter(&buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), formatter)
	cmd.SetContext(ctx)

	// Act
	err = cmd.RunE(cmd, nil)

	// Assert
	if err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}

	output := buf.String()

	// Should report that .cosca directory is not found
	if !strings.Contains(output, ".cosca directory not found") {
		t.Errorf("expected '.cosca directory not found' error, got: %s", output)
	}

	// Should warn about missing config
	if !strings.Contains(output, "config.yaml not found") {
		t.Error("expected warning about config.yaml not found")
	}

	// Should report validation failure
	if !strings.Contains(output, "Project validation failed") {
		t.Errorf("expected 'Project validation failed', got: %s", output)
	}

	// Should NOT say "Project validation passed"
	if strings.Contains(output, "Project validation passed") {
		t.Error("should not report validation passed when .cosca is missing")
	}
}

// =============================================================================
// Validate Command — With .cosca Directory, No Config
// =============================================================================

func TestValidateCommand_WithCoscaDir_NoConfig(t *testing.T) {
	// Cannot run parallel — changes working directory

	origWd, err := os.Getwd()
	if err != nil {
		t.Skipf("cannot get working directory: %v", err)
	}
	defer func() { _ = os.Chdir(origWd) }()

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Skipf("cannot change to temp dir: %v", err)
	}

	// Create .cosca/ but no config.yaml
	coscaDir := filepath.Join(tmpDir, ".cosca")
	if err := os.MkdirAll(coscaDir, 0755); err != nil {
		t.Fatalf("failed to create .cosca: %v", err)
	}

	cmd := NewValidateCommand()
	var buf bytes.Buffer
	formatter := NewOutputFormatter(&buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), formatter)
	cmd.SetContext(ctx)

	// Act
	err = cmd.RunE(cmd, nil)

	// Assert
	if err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}

	output := buf.String()

	// .cosca directory exists check should pass
	if !strings.Contains(output, ".cosca directory exists") {
		t.Error("expected '.cosca directory exists' check")
	}

	// config.yaml not found warning
	if !strings.Contains(output, "config.yaml not found") {
		t.Error("expected warning about config.yaml not found")
	}

	// No errors, so project should be valid
	if !strings.Contains(output, "Project validation passed") {
		t.Errorf("expected 'Project validation passed', got: %s", output)
	}
}

// =============================================================================
// Validate Command — Full Setup
// =============================================================================

func TestValidateCommand_WithCoscaDirAndConfig(t *testing.T) {
	// Cannot run parallel — changes working directory

	origWd, err := os.Getwd()
	if err != nil {
		t.Skipf("cannot get working directory: %v", err)
	}
	defer func() { _ = os.Chdir(origWd) }()

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Skipf("cannot change to temp dir: %v", err)
	}

	// Create .cosca/ with config.yaml and runtime/
	coscaDir := filepath.Join(tmpDir, ".cosca")
	if err := os.MkdirAll(filepath.Join(coscaDir, "runtime"), 0755); err != nil {
		t.Fatalf("failed to create runtime dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(coscaDir, "config.yaml"), []byte("version: \"1.0\"\n"), 0644); err != nil {
		t.Fatalf("failed to write config.yaml: %v", err)
	}

	cmd := NewValidateCommand()
	var buf bytes.Buffer
	formatter := NewOutputFormatter(&buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), formatter)
	cmd.SetContext(ctx)

	// Act
	err = cmd.RunE(cmd, nil)

	// Assert
	if err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}

	output := buf.String()

	// All checks should pass
	if !strings.Contains(output, ".cosca directory exists") {
		t.Error("expected '.cosca directory exists' check")
	}
	if !strings.Contains(output, "config.yaml exists") {
		t.Error("expected 'config.yaml exists' check")
	}
	if !strings.Contains(output, "runtime directory exists") {
		t.Error("expected 'runtime directory exists' check")
	}
	if !strings.Contains(output, "Project validation passed") {
		t.Errorf("expected 'Project validation passed', got: %s", output)
	}
}

// =============================================================================
// Validate Command — JSON Output
// =============================================================================

func TestValidateCommand_JSONOutput(t *testing.T) {
	// Cannot run parallel — changes working directory

	origWd, err := os.Getwd()
	if err != nil {
		t.Skipf("cannot get working directory: %v", err)
	}
	defer func() { _ = os.Chdir(origWd) }()

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Skipf("cannot change to temp dir: %v", err)
	}

	// Create .cosca/ with config.yaml
	coscaDir := filepath.Join(tmpDir, ".cosca")
	if err := os.MkdirAll(coscaDir, 0755); err != nil {
		t.Fatalf("failed to create .cosca: %v", err)
	}
	if err := os.WriteFile(filepath.Join(coscaDir, "config.yaml"), []byte("version: \"1.0\"\n"), 0644); err != nil {
		t.Fatalf("failed to write config.yaml: %v", err)
	}

	cmd := NewValidateCommand()
	// JSON output is written via printJSON which uses cmd.OutOrStdout(),
	// not the formatter. Set the command's output buffer.
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	formatter := NewOutputFormatter(&buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), formatter)
	cmd.SetContext(ctx)

	// Enable JSON output — need to set a root-level persistent flag
	// Since the command is standalone, cmd.Root() returns cmd itself.
	// Add a persistent "json" flag so IsJSONOutput finds it.
	cmd.PersistentFlags().BoolP("json", "j", false, "json output")
	cmd.PersistentFlags().Set("json", "true")

	// Act
	err = cmd.RunE(cmd, nil)

	// Assert
	if err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}

	output := buf.String()
	var result ValidateResult
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\nOutput:\n%s", err, output)
	}

	if result.ProjectDir == "" {
		t.Error("expected non-empty project_dir in JSON")
	}
	if !result.Valid {
		t.Error("expected project to be valid in JSON output")
	}
	if len(result.Checks) == 0 {
		t.Error("expected at least one check in JSON output")
	}
}

// =============================================================================
// Validate — Text Output in No-Color Mode
// =============================================================================

func TestValidateCommand_NoColorOutput(t *testing.T) {
	// Cannot run parallel — changes working directory

	origWd, err := os.Getwd()
	if err != nil {
		t.Skipf("cannot get working directory: %v", err)
	}
	defer func() { _ = os.Chdir(origWd) }()

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Skipf("cannot change to temp dir: %v", err)
	}

	// Create .cosca/ with config.yaml
	coscaDir := filepath.Join(tmpDir, ".cosca")
	if err := os.MkdirAll(coscaDir, 0755); err != nil {
		t.Fatalf("failed to create .cosca: %v", err)
	}
	if err := os.WriteFile(filepath.Join(coscaDir, "config.yaml"), []byte("version: \"1.0\"\n"), 0644); err != nil {
		t.Fatalf("failed to write config.yaml: %v", err)
	}

	cmd := NewValidateCommand()
	var buf bytes.Buffer
	formatter := NewOutputFormatter(&buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), formatter)
	cmd.SetContext(ctx)

	// Act
	err = cmd.RunE(cmd, nil)

	// Assert
	if err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}

	output := buf.String()
	// In no-color mode, there should be no ANSI escape codes
	if strings.Contains(output, "\033[") {
		t.Error("no-color output should not contain ANSI escape codes")
	}
}
