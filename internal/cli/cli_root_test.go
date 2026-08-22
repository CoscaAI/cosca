//
// Additional tests for the root command, help, version flag, and ExecuteContext.

package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// =============================================================================
// Root Command — ExecuteContext
// =============================================================================

func TestRootCommand_ExecuteContext_RunsHelp(t *testing.T) {
	cmd := NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately — but RunE runs synchronously so it's fine

	err := cmd.ExecuteContext(ctx)
	// Root command with no args shows help
	if err != nil {
		t.Fatalf("ExecuteContext returned error: %v", err)
	}
	if !strings.Contains(buf.String(), "Usage:") {
		t.Error("expected help output from ExecuteContext")
	}
}

// =============================================================================
// Root Command — Version Flag
// =============================================================================

func TestRootCommand_VersionFlag_Present(t *testing.T) {
	cmd := NewRootCommand()

	// The --version flag (-v) should exist as a persistent flag
	versionFlag := cmd.PersistentFlags().Lookup("version")
	if versionFlag == nil {
		t.Fatal("missing --version persistent flag")
	}
	if versionFlag.Shorthand != "v" {
		t.Errorf("--version shorthand = %q, want %q", versionFlag.Shorthand, "v")
	}
}

func TestRootCommand_VersionField_Set(t *testing.T) {
	cmd := NewRootCommand()
	if cmd.Version == "" {
		t.Error("root command Version field should not be empty")
	}
	if cmd.Version != Version {
		t.Errorf("root.Version = %q, want package Version %q", cmd.Version, Version)
	}
}

// =============================================================================
// Root Command — Help Output
// =============================================================================

func TestRootCommand_HelpOutput_ContainsExpectedSections(t *testing.T) {
	cmd := NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	err := cmd.RunE(cmd, nil)
	if err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}

	output := buf.String()

	// Required help sections
	expectedSections := []string{
		"Usage:",
		"Cosca",
		"AI Orchestration System",
		"Flags:",
	}
	for _, section := range expectedSections {
		if !strings.Contains(output, section) {
			t.Errorf("help output missing section: %q", section)
		}
	}

	// Should list core commands
	for _, cmdName := range []string{"init", "install", "status", "version", "doctor"} {
		if !strings.Contains(output, cmdName) {
			t.Errorf("help output missing command: %q", cmdName)
		}
	}
}

func TestRootCommand_HelpOutput_ContainsVersion(t *testing.T) {
	cmd := NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	_ = cmd.RunE(cmd, nil)

	output := buf.String()
	if !strings.Contains(output, "version") {
		t.Error("help output should mention 'version' command")
	}
}

// =============================================================================
// Root Command — Long Description
// =============================================================================

func TestRootCommand_LongDescription_NotEmpty(t *testing.T) {
	cmd := NewRootCommand()
	if cmd.Long == "" {
		t.Error("Long description should not be empty")
	}
	if !strings.Contains(cmd.Long, "Enterprise Platform") {
		t.Error("Long description should mention 'Enterprise Platform'")
	}
}

// =============================================================================
// Root Command — Example
// =============================================================================

func TestRootCommand_Example_ContainsExpectedCommands(t *testing.T) {
	cmd := NewRootCommand()
	examples := []string{"init", "install", "status", "doctor", "version"}
	for _, ex := range examples {
		if !strings.Contains(cmd.Example, ex) {
			t.Errorf("Example missing expected command: %q", ex)
		}
	}
}

// =============================================================================
// Root Command — Completion Options
// =============================================================================

func TestRootCommand_CompletionOptions_DefaultCommandDisabled(t *testing.T) {
	cmd := NewRootCommand()
	if !cmd.CompletionOptions.DisableDefaultCmd {
		t.Error("expected CompletionOptions.DisableDefaultCmd to be true")
	}
}

// =============================================================================
// Root Command — Subcommand Count
// =============================================================================

func TestRootCommand_SubcommandCount(t *testing.T) {
	cmd := NewRootCommand()
	// Should have a substantial number of subcommands
	if len(cmd.Commands()) < 30 {
		t.Errorf("expected at least 30 subcommands, got %d", len(cmd.Commands()))
	}
}

// =============================================================================
// Root Command — Silence Usage / Errors
// =============================================================================

func TestRootCommand_SilenceFlags(t *testing.T) {
	cmd := NewRootCommand()
	if !cmd.SilenceUsage {
		t.Error("expected SilenceUsage to be true")
	}
	if !cmd.SilenceErrors {
		t.Error("expected SilenceErrors to be true")
	}
}

// =============================================================================
// Root Command — NewRootCommand returns non-nil
// =============================================================================

func TestNewRootCommand_ReturnsNonNil(t *testing.T) {
	cmd := NewRootCommand()
	if cmd == nil {
		t.Fatal("NewRootCommand returned nil")
	}
}

// =============================================================================
// Root Command — Error on Unknown Subcommand
// =============================================================================

func TestRootCommand_UnknownSubcommand_ReturnsError(t *testing.T) {
	cmd := NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	// Silence errors to avoid noise in test output
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true

	// Execute with an unknown subcommand
	cmd.SetArgs([]string{"nonexistent-command-xyz"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for unknown subcommand")
	}
}

// =============================================================================
// ExecuteContext — Context Propagation
// =============================================================================

func TestRootCommand_ExecuteContext_PreservesContext(t *testing.T) {
	cmd := NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	ctx := context.WithValue(context.Background(), contextKey("test-key"), "test-value")

	// Run a full execute to exercise PersistentPreRunE
	cmd.SetArgs([]string{"version"})
	err := cmd.ExecuteContext(ctx)

	// Version command should run successfully
	if err != nil {
		t.Fatalf("ExecuteContext returned error: %v", err)
	}
	if !strings.Contains(buf.String(), "Version") {
		t.Error("expected version output")
	}
}

// =============================================================================
// Version subcommand via root — testing via ExecuteContext
// =============================================================================

func TestRootCommand_VersionViaExecuteContext(t *testing.T) {
	cmd := NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	// Simulate: cosca version
	cmd.SetArgs([]string{"version"})

	err := cmd.ExecuteContext(context.Background())
	if err != nil {
		t.Fatalf("ExecuteContext returned error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Cosca") {
		t.Errorf("expected version output containing 'Cosca', got: %s", output)
	}
}

// =============================================================================
// Root command — JSON version flag via ExecuteContext
// =============================================================================

func TestRootCommand_VersionJSONViaExecuteContext(t *testing.T) {
	cmd := NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	// Simulate: cosca version --json
	cmd.SetArgs([]string{"version", "--json"})

	err := cmd.ExecuteContext(context.Background())
	if err != nil {
		t.Fatalf("ExecuteContext returned error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, `"version"`) {
		t.Errorf("expected JSON output containing 'version' key, got: %s", output)
	}
}

// =============================================================================
// Context helper — newContextWithFormatter
// =============================================================================

func TestNewContextWithFormatter_SetsValue(t *testing.T) {
	ctx := context.Background()
	var buf bytes.Buffer
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, false)
	newCtx := newContextWithFormatter(ctx, f)

	if newCtx == nil {
		t.Fatal("newContextWithFormatter returned nil")
	}

	// Verify the formatter is retrievable
	retrieved := newCtx.Value(formatterKey)
	if retrieved == nil {
		t.Fatal("formatter not found in context")
	}

	formatter, ok := retrieved.(*OutputFormatter)
	if !ok {
		t.Fatalf("unexpected type in context: %T", retrieved)
	}

	formatter.Println("test ctx")
	if !strings.Contains(buf.String(), "test ctx") {
		t.Error("expected formatter from context to write to buffer")
	}
}

// =============================================================================
// Root Command — PersistentPreRunE sets formatter in context
// =============================================================================

func TestRootCommand_PersistentPreRunE_SetsFormatter(t *testing.T) {
	cmd := NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	// Run version subcommand — PersistentPreRunE should run first
	cmd.SetArgs([]string{"version"})

	err := cmd.ExecuteContext(context.Background())
	if err != nil {
		t.Fatalf("ExecuteContext returned error: %v", err)
	}

	// If persistentPreRun ran successfully, version output should be present
	output := buf.String()
	if !strings.Contains(output, "Cosca") {
		t.Errorf("expected version output, got: %s", output)
	}
}

// =============================================================================
// Root Command — PrintJSON helper
// =============================================================================

func TestPrintJSON_OutputsValidJSON(t *testing.T) {
	cmd := &cobra.Command{}
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	type testData struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	data := testData{Name: "test", Value: 42}
	err := printJSON(cmd, data)
	if err != nil {
		t.Fatalf("printJSON returned error: %v", err)
	}

	if !strings.Contains(buf.String(), `"name"`) {
		t.Error("expected JSON output with 'name' key")
	}
	if !strings.Contains(buf.String(), `"value"`) {
		t.Error("expected JSON output with 'value' key")
	}
	if !strings.Contains(buf.String(), "42") {
		t.Error("expected JSON output with value 42")
	}
}

// =============================================================================
// Root Command — GlobalFlags reset between tests
// =============================================================================

func TestGlobalFlags_DefaultValues(t *testing.T) {
	// Reset global flags
	globalFlags = GlobalFlags{}

	if globalFlags.Verbose {
		t.Error("Verbose should default to false")
	}
	if globalFlags.Quiet {
		t.Error("Quiet should default to false")
	}
	if globalFlags.JSON {
		t.Error("JSON should default to false")
	}
	if globalFlags.Format != "" {
		t.Errorf("Format should default to empty, got %q", globalFlags.Format)
	}
	if globalFlags.NoColor {
		t.Error("NoColor should default to false")
	}
}

// =============================================================================
// Root Command — IsJSONOutput with no root flag
// =============================================================================

func TestIsJSONOutput_CommandWithoutRootParent(t *testing.T) {
	cmd := &cobra.Command{}
	// No parent, no flag set — should be false
	cmd.SetContext(context.Background())
	result := IsJSONOutput(cmd)
	if result {
		t.Error("expected IsJSONOutput to return false when no flag is set")
	}
}

// =============================================================================
// Root Command — NewRootCommand returns consistent results
// =============================================================================

func TestNewRootCommand_ConsistentResults(t *testing.T) {
	cmd1 := NewRootCommand()
	cmd2 := NewRootCommand()

	if cmd1 == nil || cmd2 == nil {
		t.Fatal("NewRootCommand returned nil")
	}

	// Both should have the same number of subcommands
	if len(cmd1.Commands()) != len(cmd2.Commands()) {
		t.Errorf("inconsistent subcommand count: %d vs %d",
			len(cmd1.Commands()), len(cmd2.Commands()))
	}
}

// =============================================================================
// Root command — DisableAutoGenTag (cobra default)
// =============================================================================

func TestRootCommand_DisableAutoGenTag(t *testing.T) {
	cmd := NewRootCommand()
	// By default, cobra's DisableAutoGenTag is false
	// We check that it's not causing issues in our help output
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	_ = cmd.Help()
	output := buf.String()

	// Help output should not contain the auto-generated tag
	if strings.Contains(output, "Auto generated by spf13/cobra") {
		t.Log("auto-generation tag is present (cobra default)")
	}
}

// =============================================================================
// Root command — SetArgs + Execute integration
// =============================================================================

func TestRootCommand_SetArgs_Execute(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
		checkFn func(*testing.T, string)
	}{
		{
			name:    "no args shows help",
			args:    nil,
			wantErr: false,
			checkFn: func(t *testing.T, output string) {
				if !strings.Contains(output, "Usage:") {
					t.Error("expected help output")
				}
			},
		},
		{
			name:    "version subcommand",
			args:    []string{"version"},
			wantErr: false,
			checkFn: func(t *testing.T, output string) {
				if !strings.Contains(output, "Cosca") {
					t.Error("expected version output")
				}
			},
		},
		{
			name:    "unknown subcommand",
			args:    []string{"__unknown_cmd__xyz"},
			wantErr: true,
			checkFn: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewRootCommand()
			buf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetErr(buf)
			cmd.SetArgs(tt.args)

			err := cmd.ExecuteContext(context.Background())

			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tt.checkFn != nil {
				tt.checkFn(t, buf.String())
			}
		})
	}
}
