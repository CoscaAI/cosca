
//go:build integration

package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// EditorInfo represents detected editor information.
type EditorInfo struct {
	Name    string `json:"name"`
	Path    string `json:"path,omitempty"`
	Version string `json:"version,omitempty"`
	Found   bool   `json:"found"`
}

// detectEditor attempts to locate an editor by name using exec.LookPath.
func detectEditor(name string) EditorInfo {
	info := EditorInfo{Name: name}

	// Common editor executable names
	execNames := map[string][]string{
		"opencode": {"opencode", "opencode-cli", "cosca"},
		"vscode":   {"code", "code-insiders", "codium"},
		"neovim":   {"nvim", "neovim"},
		"vim":      {"vim", "gvim"},
		"intellij": {"idea", "idea.sh", "intellij"},
		"emacs":    {"emacs", "emacsclient"},
	}

	candidates, ok := execNames[name]
	if !ok {
		candidates = []string{name}
	}

	for _, exe := range candidates {
		path, err := exec.LookPath(exe)
		if err == nil {
			info.Path = path
			info.Found = true
			info.Version = getEditorVersion(exe)
			break
		}
	}

	return info
}

// getEditorVersion attempts to get the version of an editor binary.
func getEditorVersion(exe string) string {
	// Try --version flag (works for most editors)
	cmd := exec.Command(exe, "--version")
	output, err := cmd.Output()
	if err != nil {
		// Try -version flag
		cmd = exec.Command(exe, "-version")
		output, err = cmd.Output()
		if err != nil {
			return ""
		}
	}
	// Return the first non-empty line
	outputStr := string(output)
	for _, line := range splitLines(outputStr) {
		if line != "" {
			// Return just the first meaningful part
			if len(line) > 80 {
				line = line[:80] + "..."
			}
			return line
		}
	}
	return ""
}

// splitLines splits a string into lines without importing strings.
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			if i > start {
				lines = append(lines, s[start:i])
			}
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// TestEditorDetection_OpenCode tests detection of the opencode editor.
func TestEditorDetection_OpenCode(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	editor := detectEditor("opencode")

	// This test may not find opencode installed, but should not panic
	t.Logf("opencode detection: found=%v, path=%q, version=%q",
		editor.Found, editor.Path, editor.Version)

	if !editor.Found {
		t.Log("opencode not found in PATH (this is expected in CI environments)")
	}
}

// TestEditorDetection_VSCode tests detection of Visual Studio Code.
func TestEditorDetection_VSCode(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	editor := detectEditor("vscode")

	t.Logf("vscode detection: found=%v, path=%q, version=%q",
		editor.Found, editor.Path, editor.Version)

	if !editor.Found {
		t.Log("vscode not found in PATH (this may be expected depending on environment)")
	}
}

// TestEditorDetection_NotFound verifies error handling when editor is missing.
func TestEditorDetection_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Use a clearly non-existent editor name
	editor := detectEditor("nonexistent-editor-12345")

	if editor.Found {
		t.Errorf("expected editor not to be found, but path is %q", editor.Path)
	}
	if editor.Path != "" {
		t.Errorf("expected empty path for non-existent editor, got %q", editor.Path)
	}
}

// TestEditorDetection_ExecutableInTemp verifies detection of a custom
// editor script placed on PATH.
func TestEditorDetection_ExecutableInTemp(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	if runtime.GOOS == "windows" {
		t.Skip("skipping on Windows: exec.LookPath behavior differs")
	}

	// Create a temporary "editor" script
	tmpDir, err := os.MkdirTemp(".", "cosca-editor-test-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	editorPath := filepath.Join(tmpDir, "test-editor")
	editorScript := `#!/bin/sh
echo "Test Editor v1.0.0"
`
	if err := os.WriteFile(editorPath, []byte(editorScript), 0o755); err != nil {
		t.Fatalf("failed to write editor script: %v", err)
	}

	// Save and restore PATH
	origPath := os.Getenv("PATH")
	defer os.Setenv("PATH", origPath)
	os.Setenv("PATH", tmpDir+string(filepath.ListSeparator)+origPath)

	editor := detectEditor("test-editor")
	if !editor.Found {
		t.Fatal("expected test-editor to be found on PATH")
	}
	if editor.Path != editorPath {
		t.Errorf("expected path %q, got %q", editorPath, editor.Path)
	}
	t.Logf("detected editor: name=%s, path=%s, version=%s",
		editor.Name, editor.Path, editor.Version)
}

// TestEditorDetection_AllCommon skips if in short mode, but tries common editors.
func TestEditorDetection_AllCommon(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	editors := []string{"opencode", "vscode", "vim", "neovim", "emacs", "intellij"}

	for _, name := range editors {
		info := detectEditor(name)
		if info.Found {
			t.Logf("found editor: %s at %s [%s]", name, info.Path, info.Version)
		} else {
			t.Logf("editor not found: %s", name)
		}
	}
}
