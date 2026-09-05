//
// Tests for the `cosca hook` command group.

package cli

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewHookCommand_HasSubcommands(t *testing.T) {
	cmd := NewHookCommand()
	if cmd.Use != "hook" {
		t.Errorf("Use = %q, want 'hook'", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("Short should not be empty")
	}

	found := map[string]bool{}
	for _, sub := range cmd.Commands() {
		found[sub.Name()] = true
	}
	for _, want := range []string{"install", "post-commit"} {
		if !found[want] {
			t.Errorf("missing hook subcommand: %s", want)
		}
	}
}

func TestRootCommand_IncludesHook(t *testing.T) {
	root := NewRootCommand()
	for _, sub := range root.Commands() {
		if sub.Name() == "hook" {
			return
		}
	}
	t.Error("root command does not include 'hook' subcommand")
}

// runGitHooksTestCmd executes a git command inside dir, failing the test on error.
func runGitHooksTestCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func TestHookPostCommitCommand_RunE(t *testing.T) {
	dir := t.TempDir()
	runGitHooksTestCmd(t, dir, "init", "-b", "main")
	runGitHooksTestCmd(t, dir, "config", "user.email", "kernel@cosca.local")
	runGitHooksTestCmd(t, dir, "config", "user.name", "Cosca Kernel")
	if err := os.WriteFile(filepath.Join(dir, "code.go"), []byte("package code\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	runGitHooksTestCmd(t, dir, "add", "-A")
	runGitHooksTestCmd(t, dir, "commit", "-m", "feat: cli hook test")

	cmd := NewHookPostCommitCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	f := NewOutputFormatter(buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)
	cmd.Flags().Set("dir", dir)

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("hook post-commit RunE returned error: %v", err)
	}
	if !strings.Contains(buf.String(), "Impact Report") {
		t.Errorf("expected success output, got: %q", buf.String())
	}

	report := filepath.Join(dir, ".cosca", "memory", "timeline", "impact-reports")
	entries, err := os.ReadDir(report)
	if err != nil {
		t.Fatalf("impact reports dir not created: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 impact report, got %d", len(entries))
	}
}

func TestHookInstallCommand_RunE(t *testing.T) {
	dir := t.TempDir()
	runGitHooksTestCmd(t, dir, "init", "-b", "main")

	cmd := NewHookInstallCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	f := NewOutputFormatter(buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)
	cmd.Flags().Set("dir", dir)

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("hook install RunE returned error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".git", "hooks", "post-commit")); err != nil {
		t.Errorf("post-commit hook not installed: %v", err)
	}
}
