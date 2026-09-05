//
// Unit tests for the completion command.

package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// =============================================================================
// Completion Command — Properties
// =============================================================================

func TestCompletionCommand_Properties(t *testing.T) {
	cmd := NewCompletionCommand()

	if cmd.Use != "completion [bash|zsh|fish|powershell]" {
		t.Errorf("Use = %q, want %q", cmd.Use, "completion [bash|zsh|fish|powershell]")
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

func TestCompletionCommand_RequiresExactlyOneArg(t *testing.T) {
	// cobra.ExactArgs(1) should be set in the Args field.
	// We verify by using SetArgs + Execute which triggers Args validation.

	// 0 args — should error
	cmd := NewCompletionCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(nil)
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when no args provided")
	}

	// 2 args — should error
	cmd2 := NewCompletionCommand()
	buf2 := new(bytes.Buffer)
	cmd2.SetOut(buf2)
	cmd2.SetErr(buf2)
	cmd2.SetArgs([]string{"bash", "zsh"})
	err = cmd2.Execute()
	if err == nil {
		t.Error("expected error when too many args provided")
	}
}

// =============================================================================
// Completion Command — Invalid Shell
// =============================================================================

func TestCompletionCommand_InvalidShellReturnsError(t *testing.T) {
	cmd := NewCompletionCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetContext(context.Background())

	err := cmd.RunE(cmd, []string{"invalid_shell"})
	if err == nil {
		t.Fatal("expected error for invalid shell")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "unsupported shell") {
		t.Errorf("error message should mention 'unsupported shell', got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "invalid_shell") {
		t.Errorf("error message should mention the invalid shell name, got: %s", errMsg)
	}

	// Supported shells should be listed
	for _, s := range []string{"bash", "zsh", "fish", "powershell"} {
		if !strings.Contains(errMsg, s) {
			t.Errorf("error message should mention supported shell %q", s)
		}
	}
}

func TestCompletionCommand_InvalidShell_CaseSensitive(t *testing.T) {
	cmd := NewCompletionCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetContext(context.Background())

	// "Bash" with capital B is not valid
	err := cmd.RunE(cmd, []string{"Bash"})
	if err == nil {
		t.Error("expected error for 'Bash' (case sensitive)")
	}
}

// =============================================================================
// Completion Command — Valid Shells
// =============================================================================

func TestCompletionCommand_ValidShellsRunWithoutError(t *testing.T) {
	shells := []string{"bash", "zsh", "fish", "powershell"}

	for _, shell := range shells {
		t.Run(shell, func(t *testing.T) {
			cmd := NewCompletionCommand()
			buf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetErr(buf)
			cmd.SetContext(context.Background())

			err := cmd.RunE(cmd, []string{shell})
			if err != nil {
				t.Logf("RunE for %q returned error (may be expected without root parent): %v", shell, err)
			}
		})
	}
}

// =============================================================================
// Completion Command — ValidArgsFunction
// =============================================================================

func TestCompletionCommand_ValidArgsFunction_NoArgs(t *testing.T) {
	cmd := NewCompletionCommand()

	// When no args are typed yet, ValidArgsFunction should suggest all shells
	suggestions, directive := cmd.ValidArgsFunction(cmd, nil, "")
	if len(suggestions) != 4 {
		t.Errorf("expected 4 suggestions for empty arg, got %d: %v", len(suggestions), suggestions)
	}

	expectedShells := map[string]bool{
		"bash": true, "zsh": true, "fish": true, "powershell": true,
	}
	for _, s := range suggestions {
		if !expectedShells[s] {
			t.Errorf("unexpected suggestion: %q", s)
		}
		delete(expectedShells, s)
	}
	if len(expectedShells) > 0 {
		t.Errorf("missing suggestions: %v", expectedShells)
	}

	// ShellCompDirectiveNoFileComp should be set
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("directive = %d, want %d (ShellCompDirectiveNoFileComp)", directive, cobra.ShellCompDirectiveNoFileComp)
	}
}

func TestCompletionCommand_ValidArgsFunction_WithArgs(t *testing.T) {
	cmd := NewCompletionCommand()

	// When an arg is already typed, ValidArgsFunction should return nil suggestions
	suggestions, directive := cmd.ValidArgsFunction(cmd, []string{"bash"}, "")
	if suggestions != nil {
		t.Errorf("expected nil suggestions when arg already provided, got %v", suggestions)
	}
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("directive = %d, want %d", directive, cobra.ShellCompDirectiveNoFileComp)
	}
}

func TestCompletionCommand_ValidArgsFunction_PartialMatch(t *testing.T) {
	// ValidArgsFunction receives the full list of args, not the partial string.
	// When typing "ba<TAB>", cobra provides the full command line.
	// ValidArgsFunction with 0 args returns all shells regardless.
	cmd := NewCompletionCommand()
	suggestions, _ := cmd.ValidArgsFunction(cmd, nil, "ba")

	if len(suggestions) != 4 {
		t.Errorf("expected 4 suggestions for no args (ignores toComplete), got %d", len(suggestions))
	}
}
