package neovim

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mitchellh/go-homedir"

	"github.com/CoscaAI/cosca/internal/editors/types"
)

// newNvimSandbox redirects XDG_CONFIG_HOME (and HOME as a safety net) into
// temp dirs and pre-creates the nvim config dir, so Setup/Teardown never
// touch the real ~/.config/nvim. With withInitVim=true a pre-existing
// init.vim forces the Vimscript code path.
func newNvimSandbox(t *testing.T, withInitVim bool) string {
	t.Helper()

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	homedir.Reset() // go-homedir caches the home dir; drop the cache.

	nvimDir := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "nvim")
	if err := os.MkdirAll(nvimDir, 0o755); err != nil {
		t.Fatalf("mkdir nvim config dir: %v", err)
	}
	if withInitVim {
		if err := os.WriteFile(filepath.Join(nvimDir, "init.vim"), []byte(`" user config`+"\n"), 0o644); err != nil {
			t.Fatalf("write init.vim: %v", err)
		}
	}
	return nvimDir
}

func TestSetupLuaCreatesKernelCommand(t *testing.T) {
	nvimDir := newNvimSandbox(t, false)
	a := NewAdapter()
	cfg := types.DefaultEditorConfig(t.TempDir())

	if err := a.Setup(cfg); err != nil {
		t.Fatalf("Setup: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(nvimDir, "init.lua"))
	if err != nil {
		t.Fatalf("init.lua not created: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, `vim.api.nvim_create_user_command("CoscaKernel"`) {
		t.Errorf("init.lua missing CoscaKernel user command:\n%s", content)
	}
	if !strings.Contains(content, "cosca kernel identity") {
		t.Errorf("init.lua missing 'cosca kernel identity' invocation:\n%s", content)
	}
	if !strings.Contains(content, "Carregar o Cosca Kernel") {
		t.Errorf("init.lua missing CoscaKernel description:\n%s", content)
	}
}

func TestSetupVimscriptAddsKernelCommand(t *testing.T) {
	nvimDir := newNvimSandbox(t, true) // pre-existing init.vim -> Vimscript path
	a := NewAdapter()
	cfg := types.DefaultEditorConfig(t.TempDir())

	if err := a.Setup(cfg); err != nil {
		t.Fatalf("Setup: %v", err)
	}

	// With an existing init.vim, no init.lua is created.
	if _, err := os.Stat(filepath.Join(nvimDir, "init.lua")); err == nil {
		t.Error("init.lua must not be created when init.vim exists")
	}

	data, err := os.ReadFile(filepath.Join(nvimDir, "init.vim"))
	if err != nil {
		t.Fatalf("init.vim not readable: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "command! CoscaKernel") {
		t.Errorf("init.vim missing 'command! CoscaKernel':\n%s", content)
	}
	if !strings.Contains(content, "cosca kernel identity") {
		t.Errorf("init.vim missing 'cosca kernel identity' invocation:\n%s", content)
	}
	if !strings.Contains(content, "function! CoscaKernel()") {
		t.Errorf("init.vim missing CoscaKernel function:\n%s", content)
	}
}

func TestSetupIdempotent(t *testing.T) {
	newNvimSandbox(t, false)
	a := NewAdapter()
	cfg := types.DefaultEditorConfig(t.TempDir())

	if err := a.Setup(cfg); err != nil {
		t.Fatalf("first Setup: %v", err)
	}
	if err := a.Setup(cfg); err != nil {
		t.Fatalf("second Setup: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "nvim", "init.lua"))
	if err != nil {
		t.Fatalf("init.lua missing after second Setup: %v", err)
	}
	if count := strings.Count(string(data), `"CoscaKernel"`); count != 1 {
		t.Errorf("CoscaKernel command duplicated: expected 1, got %d", count)
	}
	if count := strings.Count(string(data), "cosca kernel identity"); count != 1 {
		t.Errorf("'cosca kernel identity' duplicated: expected 1, got %d", count)
	}
}

func TestTeardownRemovesKernelCommand(t *testing.T) {
	nvimDir := newNvimSandbox(t, false)
	a := NewAdapter()
	cfg := types.DefaultEditorConfig(t.TempDir())

	if err := a.Setup(cfg); err != nil {
		t.Fatalf("Setup: %v", err)
	}
	if err := a.Teardown(); err != nil {
		t.Fatalf("Teardown: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(nvimDir, "init.lua"))
	if err != nil {
		t.Fatalf("init.lua not readable after Teardown: %v", err)
	}
	content := string(data)
	if strings.Contains(content, "CoscaKernel") {
		t.Errorf("CoscaKernel still present after Teardown:\n%s", content)
	}
	if strings.Contains(content, "Cosca Integration") {
		t.Errorf("Cosca Integration block still present after Teardown:\n%s", content)
	}
}

func TestVersion(t *testing.T) {
	a := NewAdapter()
	v, err := a.Version()
	if err != nil {
		t.Logf("Version not available (editor not installed): %v", err)
		return
	}
	if v == "" {
		t.Error("Version should not be empty")
	}
}

func TestInfo(t *testing.T) {
	a := NewAdapter()
	info, err := a.Info()
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if info.Name == "" {
		t.Error("Info.Name should not be empty")
	}
}

func TestDetect_NotFound(t *testing.T) {
	a := NewAdapter()
	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	t.Cleanup(func() { os.Chdir(origDir) })
	found, err := a.Detect()
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	_ = found
}
