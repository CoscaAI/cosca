//
// Tests for the `cosca install --all` and `cosca editor` command trees
// (internal/cli/install.go + internal/cli/editor.go).
//
// Covers:
//   - `install --all`: installs Cosca integration into ALL 9 registered
//     editors and creates the expected files (CLAUDE.md, .codex/, .cursorrules,
//     .vscode/, .mcp/, plus home-based configs for opencode/windsurf/zed/neovim)
//   - `editor list`: reports every supported editor with its state
//   - `editor setup --editor <name>`: installs into exactly one editor
//   - `editor setup --editor <invalid>`: fails with a clear error
//
// NOTE: tests that chdir() and redirect $HOME must NOT run in parallel —
// they mutate the process working directory and the go-homedir cache.

package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mitchellh/go-homedir"

	"github.com/CoscaAI/cosca/internal/editors"
)

// allEditorNames lists the 9 editors registered by the editor manager
// (internal/editors/manager.go).
var allEditorNames = []string{
	"claude",
	"codex",
	"cursor",
	"generic_mcp",
	"neovim",
	"opencode",
	"vscode",
	"windsurf",
	"zed",
}

// editorTestEnv redirects the process into a temp sandbox: the working
// directory, $HOME and $XDG_CONFIG_HOME all point inside the temp dir so that
// home-dir based adapters (opencode, windsurf, zed, neovim) never touch the
// real user configuration. Returns the sandbox root.
func editorTestEnv(t *testing.T) string {
	t.Helper()

	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatalf("mkdir home: %v", err)
	}

	origWd, _ := os.Getwd()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWd) })

	// go-homedir caches the home directory after the first call — reset the
	// cache AFTER redirecting $HOME so adapters pick up the sandbox home.
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	homedir.Reset()

	return tmp
}

// =============================================================================
// `cosca install --all`
// =============================================================================

func TestInstallAll(t *testing.T) {
	globalFlags = GlobalFlags{} // avoid cross-test contamination
	tmp := editorTestEnv(t)

	cmd := NewInstallCommand()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, true) // noColor
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)

	if err := cmd.Flags().Set("all", "true"); err != nil {
		t.Fatalf("set --all flag: %v", err)
	}

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("install --all returned error: %v", err)
	}

	// Project-dir files written by the project-based adapters.
	projectFiles := []string{
		"CLAUDE.md",               // claude
		".codex/config.json",      // codex
		".cursorrules",            // cursor
		".opencode/opencode.json", // opencode
		".vscode/tasks.json",      // vscode
		".mcp/cosca-server.json",  // generic_mcp
		".mcp/mcp.json",           // generic_mcp
	}

	// Home-dir files written by the home-based adapters (sandboxed $HOME).
	home := filepath.Join(tmp, "home")
	homeFiles := []string{
		filepath.Join(home, ".windsurf", "config.json"),        // windsurf
		filepath.Join(home, ".config", "zed", "settings.json"), // zed
		filepath.Join(home, ".config", "nvim", "init.lua"),     // neovim
	}

	for _, rel := range projectFiles {
		if _, err := os.Stat(filepath.Join(tmp, rel)); err != nil {
			t.Errorf("expected file %q to be created by install --all: %v", rel, err)
		}
	}
	for _, path := range homeFiles {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected file %q to be created by install --all: %v", path, err)
		}
	}

	// Every one of the 9 editors must be reported in the output.
	for _, name := range allEditorNames {
		if !strings.Contains(buf.String(), name) {
			t.Errorf("install --all output missing editor %q; output:\n%s", name, buf.String())
		}
	}
}

// TestInstallAll_ContinuesOnFailure verifies that a failing editor does not
// abort the whole bulk install: other editors are still configured and the
// command returns nil (failures are reported individually).
func TestInstallAll_ContinuesOnFailure(t *testing.T) {
	globalFlags = GlobalFlags{}
	tmp := editorTestEnv(t)

	cmd := NewInstallCommand()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)

	if err := cmd.Flags().Set("all", "true"); err != nil {
		t.Fatalf("set --all flag: %v", err)
	}

	// Make one project-dir target unwritable so the cursor adapter fails,
	// while every other adapter still succeeds.
	if err := os.WriteFile(filepath.Join(tmp, ".cursorrules"), []byte("locked"), 0o000); err != nil {
		t.Fatalf("lock .cursorrules: %v", err)
	}

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("install --all must not fail as a whole: %v", err)
	}

	// claude still installed even though cursor failed.
	if _, err := os.Stat(filepath.Join(tmp, "CLAUDE.md")); err != nil {
		t.Errorf("CLAUDE.md should still be created despite cursor failure: %v", err)
	}
	if !strings.Contains(buf.String(), "cursor") {
		t.Errorf("output should report the cursor failure; output:\n%s", buf.String())
	}
}

// =============================================================================
// `cosca editor list`
// =============================================================================

func TestEditorList(t *testing.T) {
	globalFlags = GlobalFlags{}
	editorTestEnv(t)

	cmd := NewEditorListCommand()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("editor list returned error: %v", err)
	}

	output := buf.String()
	for _, name := range allEditorNames {
		if !strings.Contains(output, name) {
			t.Errorf("editor list missing editor %q; output:\n%s", name, output)
		}
	}

	// The output must show the state legend.
	for _, want := range []string{"configurado", "detectado", "não configurado"} {
		if !strings.Contains(output, want) {
			t.Errorf("editor list missing state hint %q; output:\n%s", want, output)
		}
	}
}

func TestEditorList_ConfiguredAfterSetup(t *testing.T) {
	globalFlags = GlobalFlags{}
	tmp := editorTestEnv(t)

	// Configure two project-based editors first.
	mgr := newEditorManagerForTest(t, tmp)
	for _, name := range []string{"claude", "codex"} {
		if err := mgr.SetupForce(name); err != nil {
			t.Fatalf("setup %s: %v", name, err)
		}
	}

	cmd := NewEditorListCommand()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("editor list returned error: %v", err)
	}

	output := buf.String()
	// At least claude should show as configured (✓).
	if !strings.Contains(output, "✓ configurado") {
		t.Errorf("expected at least one configured editor after setup; output:\n%s", output)
	}
}

// =============================================================================
// `cosca editor setup --editor <name>`
// =============================================================================

func TestEditorSetup_Specific(t *testing.T) {
	globalFlags = GlobalFlags{}
	tmp := editorTestEnv(t)

	cmd := NewEditorSetupCommand()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)

	if err := cmd.Flags().Set("editor", "claude"); err != nil {
		t.Fatalf("set --editor flag: %v", err)
	}

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("editor setup --editor claude returned error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmp, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("CLAUDE.md not created: %v", err)
	}
	if !strings.Contains(string(data), "Cosca Integration") {
		t.Errorf("CLAUDE.md missing Cosca Integration section:\n%s", string(data))
	}

	if !strings.Contains(buf.String(), "claude integration installed") {
		t.Errorf("expected success message, output:\n%s", buf.String())
	}
}

func TestEditorSetup_Invalid(t *testing.T) {
	globalFlags = GlobalFlags{}
	editorTestEnv(t)

	cmd := NewEditorSetupCommand()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	f := NewOutputFormatter(&buf, OutputFormatText, false, false, true)
	ctx := newContextWithFormatter(context.Background(), f)
	cmd.SetContext(ctx)

	if err := cmd.Flags().Set("editor", "vim"); err != nil {
		t.Fatalf("set --editor flag: %v", err)
	}

	err := cmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("expected error for invalid editor name")
	}
	if !strings.Contains(err.Error(), "vim") {
		t.Errorf("error should mention the invalid editor name, got: %v", err)
	}
	if !strings.Contains(err.Error(), "supported") {
		t.Errorf("error should list supported editors, got: %v", err)
	}

	// The error must reference at least one real registered editor.
	found := false
	for _, name := range allEditorNames {
		if strings.Contains(err.Error(), name) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("error should mention registered editor names, got: %v", err)
	}
}

// =============================================================================
// Helpers
// =============================================================================

// newEditorManagerForTest builds a real editors.Manager pointed at the sandbox
// project dir with BackupExisting enabled (mirrors installAllEditors).
func newEditorManagerForTest(t *testing.T, projectDir string) *editors.Manager {
	t.Helper()
	cfg := editors.DefaultEditorConfig(projectDir)
	cfg.BackupExisting = true
	mgr := editors.NewManager(cfg)
	if mgr == nil {
		t.Fatal("editor manager not available")
	}
	return mgr
}
