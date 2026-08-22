//
// Unit tests for config command and its subcommands.

package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// =============================================================================
// Config Command — Parent Command Structure
// =============================================================================

func TestNewConfigCommand_HasAllSubcommands(t *testing.T) {
	cmd := NewConfigCommand()
	expectedSubs := []string{"get", "set", "list", "edit", "reset", "validate"}

	subNames := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		subNames[sub.Name()] = true
	}

	for _, name := range expectedSubs {
		if !subNames[name] {
			t.Errorf("missing config subcommand: %s", name)
		}
	}

	// Verify no unexpected subcommands
	if len(cmd.Commands()) != len(expectedSubs) {
		t.Errorf("expected %d subcommands, got %d", len(expectedSubs), len(cmd.Commands()))
	}
}

func TestNewConfigCommand_Properties(t *testing.T) {
	cmd := NewConfigCommand()

	if cmd.Use != "config" {
		t.Errorf("Use = %q, want %q", cmd.Use, "config")
	}
	if cmd.Short == "" {
		t.Error("Short should not be empty")
	}
	if cmd.Long == "" {
		t.Error("Long should not be empty")
	}
}

// =============================================================================
// Config Get Command
// =============================================================================

func TestNewConfigGetCommand_Properties(t *testing.T) {
	cmd := NewConfigGetCommand()

	if cmd.Use != "get <key>" {
		t.Errorf("Use = %q, want %q", cmd.Use, "get <key>")
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

func TestNewConfigGetCommand_RequiresExactlyOneArg(t *testing.T) {
	// Use cobra.Execute (not RunE directly) to properly trigger Args validation.
	// cobra.ExactArgs(1) is checked before RunE.

	// No args — should error
	cmd := NewConfigGetCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(nil)
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error with no args")
	}

	// Two args — should error
	cmd2 := NewConfigGetCommand()
	buf2 := new(bytes.Buffer)
	cmd2.SetOut(buf2)
	cmd2.SetErr(buf2)
	cmd2.SetArgs([]string{"cache.ttl", "extra"})
	err = cmd2.Execute()
	if err == nil {
		t.Error("expected error with 2 args")
	}
}

// =============================================================================
// Config Set Command
// =============================================================================

func TestNewConfigSetCommand_Properties(t *testing.T) {
	cmd := NewConfigSetCommand()

	if cmd.Use != "set <key> <value>" {
		t.Errorf("Use = %q, want %q", cmd.Use, "set <key> <value>")
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

func TestNewConfigSetCommand_RequiresExactlyTwoArgs(t *testing.T) {
	// Use cobra.Execute to properly trigger Args validation.

	// No args — should error
	cmd := NewConfigSetCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(nil)
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error with no args")
	}

	// One arg — should error
	cmd2 := NewConfigSetCommand()
	buf2 := new(bytes.Buffer)
	cmd2.SetOut(buf2)
	cmd2.SetErr(buf2)
	cmd2.SetArgs([]string{"cache.ttl"})
	err = cmd2.Execute()
	if err == nil {
		t.Error("expected error with 1 arg")
	}

	// Three args — should error
	cmd3 := NewConfigSetCommand()
	buf3 := new(bytes.Buffer)
	cmd3.SetOut(buf3)
	cmd3.SetErr(buf3)
	cmd3.SetArgs([]string{"key", "value", "extra"})
	err = cmd3.Execute()
	if err == nil {
		t.Error("expected error with 3 args")
	}
}

// =============================================================================
// Config List Command
// =============================================================================

func TestNewConfigListCommand_Properties(t *testing.T) {
	cmd := NewConfigListCommand()

	if cmd.Use != "list" {
		t.Errorf("Use = %q, want %q", cmd.Use, "list")
	}
	if cmd.Short == "" {
		t.Error("Short should not be empty")
	}
	if cmd.Long == "" {
		t.Error("Long should not be empty")
	}
}

// =============================================================================
// Config Edit Command
// =============================================================================

func TestNewConfigEditCommand_Properties(t *testing.T) {
	cmd := NewConfigEditCommand()

	if cmd.Use != "edit" {
		t.Errorf("Use = %q, want %q", cmd.Use, "edit")
	}
	if cmd.Short == "" {
		t.Error("Short should not be empty")
	}
	if cmd.Long == "" {
		t.Error("Long should not be empty")
	}
}

// =============================================================================
// Config Reset Command
// =============================================================================

func TestNewConfigResetCommand_Properties(t *testing.T) {
	cmd := NewConfigResetCommand()

	if cmd.Use != "reset" {
		t.Errorf("Use = %q, want %q", cmd.Use, "reset")
	}
	if cmd.Short == "" {
		t.Error("Short should not be empty")
	}
	if cmd.Long == "" {
		t.Error("Long should not be empty")
	}
}

func TestNewConfigResetCommand_HasForceFlag(t *testing.T) {
	cmd := NewConfigResetCommand()

	forceFlag := cmd.Flags().Lookup("force")
	if forceFlag == nil {
		t.Fatal("missing --force flag on config reset")
	}
	if forceFlag.Shorthand != "f" {
		t.Errorf("--force shorthand = %q, want %q", forceFlag.Shorthand, "f")
	}
	if forceFlag.DefValue != "false" {
		t.Errorf("--force default = %q, want %q", forceFlag.DefValue, "false")
	}
}

// =============================================================================
// Config Validate Command
// =============================================================================

func TestNewConfigValidateCommand_Properties(t *testing.T) {
	cmd := NewConfigValidateCommand()

	if cmd.Use != "validate" {
		t.Errorf("Use = %q, want %q", cmd.Use, "validate")
	}
	if cmd.Short == "" {
		t.Error("Short should not be empty")
	}
	if cmd.Long == "" {
		t.Error("Long should not be empty")
	}
}

// =============================================================================
// Config Validate — RunE (with minimal valid config file)
// =============================================================================

func TestConfigValidateCommand_WithValidConfigFile(t *testing.T) {
	// Cannot run in parallel — changes the working directory.

	origWd, err := os.Getwd()
	if err != nil {
		t.Skipf("cannot get working directory: %v", err)
	}
	defer func() { _ = os.Chdir(origWd) }()

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Skipf("cannot change to temp dir: %v", err)
	}

	// Create .cosca/ directory with a minimal valid config file
	coscaDir := filepath.Join(tmpDir, ".cosca")
	if err := os.MkdirAll(coscaDir, 0755); err != nil {
		t.Fatalf("failed to create .cosca dir: %v", err)
	}

	// Minimal valid config that passes Validate()
	minimalCfg := []byte(`version: "1.0"
paths:
  home: /tmp/cosca-test-home
db:
  max_open_conns: 5
  page_size: 4096
server:
  api_port: 8080
  rpc_port: 9090
performance:
  max_memory_mb: 128
  max_concurrent_ops: 5
provider:
  max_tokens: 100
  temperature: 0.5
search:
  default_limit: 10
  max_results: 100
`)
	cfgPath := filepath.Join(coscaDir, "config.yaml")
	if err := os.WriteFile(cfgPath, minimalCfg, 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cmd := NewConfigValidateCommand()
	var buf bytes.Buffer
	formatter := NewOutputFormatter(&buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), formatter)
	cmd.SetContext(ctx)

	// Act
	err = cmd.RunE(cmd, nil)

	// Assert — the minimal config should validate successfully
	if err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Configuration is valid") {
		t.Errorf("expected 'Configuration is valid', got: %s", output)
	}
}

func TestConfigValidateCommand_InvalidConfigDetected(t *testing.T) {
	// Cannot run in parallel — changes the working directory.

	origWd, err := os.Getwd()
	if err != nil {
		t.Skipf("cannot get working directory: %v", err)
	}
	defer func() { _ = os.Chdir(origWd) }()

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Skipf("cannot change to temp dir: %v", err)
	}

	// Create .cosca/ directory with an invalid config (missing required fields)
	coscaDir := filepath.Join(tmpDir, ".cosca")
	if err := os.MkdirAll(coscaDir, 0755); err != nil {
		t.Fatalf("failed to create .cosca dir: %v", err)
	}

	// Config with empty version and paths.home — should fail validation at load time
	invalidCfg := []byte(`version: ""
paths:
  home: ""
db:
  max_open_conns: 0
  page_size: 0
server:
  api_port: 0
  rpc_port: 0
provider:
  max_tokens: 0
`)
	cfgPath := filepath.Join(coscaDir, "config.yaml")
	if err := os.WriteFile(cfgPath, invalidCfg, 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cmd := NewConfigValidateCommand()
	var buf bytes.Buffer
	formatter := NewOutputFormatter(&buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), formatter)
	cmd.SetContext(ctx)

	// Act
	err = cmd.RunE(cmd, nil)

	// Assert — loadConfig validates during load, so invalid config
	// causes an error to be returned before validate command runs.
	if err == nil {
		t.Fatal("expected error for invalid config")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "validation") && !strings.Contains(errMsg, "failed to load") {
		t.Errorf("expected error to mention validation failure, got: %s", errMsg)
	}
}
