//
// Tests for the `cosca circadian` command tree (internal/cli/circadian.go).
//
// Covers:
//   - Registration of `circadian` (and its 4 subcommands) in the root command
//   - `circadian status` output (JSON via ExecuteContext + text via RunE)
//   - `circadian sleep` against a temp project (runs the ORC synchronously)
//   - `circadian wake` against a temp project (runs the wake ritual)
//
// NOTE: tests that chdir() must NOT run in parallel — they mutate the
// process working directory.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// =============================================================================
// Registration — `circadian` in the root command
// =============================================================================

func TestCircadianCommand_RegisteredInRoot(t *testing.T) {
	cmd := NewRootCommand()
	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "circadian" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("circadian subcommand not registered in root command")
	}
}

func TestCircadianCommand_Properties(t *testing.T) {
	cmd := NewCircadianCommand()
	if cmd == nil {
		t.Fatal("NewCircadianCommand returned nil")
	}
	if cmd.Use != "circadian" {
		t.Errorf("expected Use='circadian', got %q", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("expected non-empty Short description")
	}
	if cmd.Long == "" {
		t.Error("expected non-empty Long description")
	}

	expected := []string{"status", "sleep", "wake", "watch"}
	if len(cmd.Commands()) != len(expected) {
		t.Errorf("expected %d subcommands, got %d", len(expected), len(cmd.Commands()))
	}
	registered := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		registered[sub.Name()] = true
	}
	for _, name := range expected {
		if !registered[name] {
			t.Errorf("missing circadian subcommand: %s", name)
		}
	}
}

func TestCircadianSubcommands_NonNil(t *testing.T) {
	tests := []struct {
		name    string
		factory func() *cobra.Command
		use     string
	}{
		{"status", NewCircadianStatusCommand, "status"},
		{"sleep", NewCircadianSleepCommand, "sleep"},
		{"wake", NewCircadianWakeCommand, "wake"},
		{"watch", NewCircadianWatchCommand, "watch"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			cmd := tt.factory()
			if cmd == nil {
				t.Fatalf("factory for %q returned nil", tt.name)
			}
			if cmd.Use != tt.use {
				t.Errorf("expected Use=%q, got %q", tt.use, cmd.Use)
			}
			if cmd.Short == "" {
				t.Error("expected non-empty Short description")
			}
			if cmd.RunE == nil {
				t.Error("expected RunE to be set")
			}
		})
	}
}

// =============================================================================
// `circadian status`
// =============================================================================

func TestCircadianStatus_Command(t *testing.T) {
	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()

	tmpDir := t.TempDir()
	createKernelTestDB(t, filepath.Join(tmpDir, ".cosca", "knowledge.db"))
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	globalFlags = GlobalFlags{}
	root := NewRootCommand()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"circadian", "status", "--json"})

	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext returned error: %v", err)
	}

	var status CircadianStatus
	if err := json.Unmarshal(buf.Bytes(), &status); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}
	if status.State != "awake" {
		t.Errorf("state = %q, want %q", status.State, "awake")
	}
	if status.Proposed != "awake" {
		t.Errorf("proposed_state = %q, want %q", status.Proposed, "awake")
	}
	if status.CoscaDir != filepath.Join(tmpDir, ".cosca") {
		t.Errorf("cosca_dir = %q, want %q", status.CoscaDir, filepath.Join(tmpDir, ".cosca"))
	}
}

func TestCircadianStatus_RunE_Text(t *testing.T) {
	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()

	tmpDir := t.TempDir()
	createKernelTestDB(t, filepath.Join(tmpDir, ".cosca", "knowledge.db"))
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	globalFlags = GlobalFlags{}
	cmd := NewCircadianStatusCommand()
	var buf bytes.Buffer
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, true) // noColor
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}
	output := buf.String()
	for _, want := range []string{"Circadian Engine", "State", "awake", "Cosca Dir", "Last ORC"} {
		if !strings.Contains(output, want) {
			t.Errorf("status output missing %q; output:\n%s", want, output)
		}
	}
}

// =============================================================================
// `circadian sleep`
// =============================================================================

func TestCircadianSleep_Command(t *testing.T) {
	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()

	tmpDir := t.TempDir()
	createKernelTestDB(t, filepath.Join(tmpDir, ".cosca", "knowledge.db"))
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	globalFlags = GlobalFlags{}
	root := NewRootCommand()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"circadian", "sleep", "--json"})

	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext returned error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}
	if result["started_at"] == nil {
		t.Error("expected started_at in ORC result")
	}
	if result["duration"] == nil {
		t.Error("expected duration in ORC result")
	}
	steps, ok := result["steps"].([]interface{})
	if !ok || len(steps) < 7 {
		t.Errorf("expected 7+ ORC steps, got %v", result["steps"])
	}
}

func TestCircadianSleep_RunE_Text(t *testing.T) {
	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()

	tmpDir := t.TempDir()
	createKernelTestDB(t, filepath.Join(tmpDir, ".cosca", "knowledge.db"))
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	globalFlags = GlobalFlags{}
	cmd := NewCircadianSleepCommand()
	var buf bytes.Buffer
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}
	output := buf.String()
	for _, want := range []string{
		"Operational Rest Cycle",
		"Items Processed",
		"Steps",
		"compact_learnings",
		"generate_report",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("sleep output missing %q; output:\n%s", want, output)
		}
	}
}

// =============================================================================
// `circadian wake`
// =============================================================================

func TestCircadianWake_Command(t *testing.T) {
	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()

	tmpDir := t.TempDir()
	createKernelTestDB(t, filepath.Join(tmpDir, ".cosca", "knowledge.db"))
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	globalFlags = GlobalFlags{}
	root := NewRootCommand()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"circadian", "wake", "--json"})

	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext returned error: %v", err)
	}

	var wake map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &wake); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}
	if wake["knowledge_valid"] != true {
		t.Errorf("knowledge_valid = %v, want true", wake["knowledge_valid"])
	}
	if wake["constitution_loaded"] != true {
		t.Errorf("constitution_loaded = %v, want true", wake["constitution_loaded"])
	}
	if wake["clock_synced"] != true {
		t.Errorf("clock_synced = %v, want true", wake["clock_synced"])
	}
	if wake["summary"] == nil || wake["summary"] == "" {
		t.Error("expected a non-empty summary")
	}
	steps, ok := wake["steps"].([]interface{})
	if !ok || len(steps) != 6 {
		t.Errorf("expected 6 wake steps, got %v", wake["steps"])
	}
}

func TestCircadianWake_RunE_Text(t *testing.T) {
	origWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(origWd) }()

	tmpDir := t.TempDir()
	createKernelTestDB(t, filepath.Join(tmpDir, ".cosca", "knowledge.db"))
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	globalFlags = GlobalFlags{}
	cmd := NewCircadianWakeCommand()
	var buf bytes.Buffer
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}
	output := buf.String()
	for _, want := range []string{
		"Wake Ritual",
		"ORC retomado",
		"P1-P8 carregada",
		"knowledge OK",
		"load_memory",
		"accept_commands",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("wake output missing %q; output:\n%s", want, output)
		}
	}
}
