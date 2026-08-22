package opencode

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CoscaAI/cosca/internal/editors/types"
)

// setupConfig returns an EditorConfig pointed at a fresh temp project dir.
func setupConfig(t *testing.T) types.EditorConfig {
	t.Helper()
	cfg := types.DefaultEditorConfig(t.TempDir())
	cfg.BackupExisting = true
	return cfg
}

func readConfig(t *testing.T, projectDir string) (map[string]interface{}, string) {
	t.Helper()
	cfgPath := ConfigPath(projectDir)
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read %s: %v", cfgPath, err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("parse %s: %v", cfgPath, err)
	}
	return parsed, string(data)
}

func TestSetupCreatesOpenCodeConfig(t *testing.T) {
	t.Parallel()
	cfg := setupConfig(t)
	adapter := NewAdapter()

	if err := adapter.Setup(cfg); err != nil {
		t.Fatalf("Setup: %v", err)
	}

	cfgPath := ConfigPath(cfg.ProjectDir)
	if _, err := os.Stat(cfgPath); err != nil {
		t.Fatalf(".opencode/opencode.json not created: %v", err)
	}

	parsed, content := readConfig(t, cfg.ProjectDir)

	// Required fields per the boot spec.
	if parsed["default_agent"] != "cosca-kernel" {
		t.Errorf("default_agent = %v, want cosca-kernel", parsed["default_agent"])
	}
	// Third-party projects use self-contained prompt (framework lives in binary embed).
	if !strings.Contains(content, "internal/embed/cosca") {
		t.Error("config must reference internal/embed/cosca/ (framework in binary)")
	}
	if !strings.Contains(content, "Cosca KERNEL BOOT") {
		t.Error("config must contain the boot instruction")
	}

	agents, ok := parsed["agent"].(map[string]interface{})
	if !ok {
		t.Fatal("agent map missing from config")
	}
	kernel, ok := agents["cosca-kernel"].(map[string]interface{})
	if !ok {
		t.Fatalf("cosca-kernel agent missing: %v", agents)
	}
	if kernel["mode"] != "primary" {
		t.Errorf("cosca-kernel mode = %v, want primary", kernel["mode"])
	}
	prompt, _ := kernel["prompt"].(string)
	if !strings.Contains(prompt, "consigliere") || !strings.Contains(prompt, "internal/embed/cosca") {
		t.Errorf("cosca-kernel prompt must be self-contained, got %q", prompt)
	}

	// The .opencode/ dir must NOT contain framework files.
	if _, err := os.Stat(filepath.Join(cfg.ProjectDir, ".opencode", "cosca")); err == nil {
		t.Error(".opencode/cosca must NOT exist — the framework lives in .cosca/framework")
	}
}

func TestSetupIsIdempotent(t *testing.T) {
	t.Parallel()
	cfg := setupConfig(t)
	adapter := NewAdapter()

	if err := adapter.Setup(cfg); err != nil {
		t.Fatalf("Setup #1: %v", err)
	}
	first, _ := readConfig(t, cfg.ProjectDir)

	if err := adapter.Setup(cfg); err != nil {
		t.Fatalf("Setup #2: %v", err)
	}
	second, _ := readConfig(t, cfg.ProjectDir)

	// default_agent unchanged and still unique.
	if second["default_agent"] != "cosca-kernel" {
		t.Errorf("default_agent after 2nd setup = %v", second["default_agent"])
	}

	agents := second["agent"].(map[string]interface{})
	kernelCount := 0
	for name := range agents {
		if name == "cosca-kernel" {
			kernelCount++
		}
	}
	if kernelCount != 1 {
		t.Errorf("cosca-kernel must appear exactly once in agent map, got %d", kernelCount)
	}

	// instructions: boot instruction must not be duplicated.
	instr, _ := second["instructions"].([]interface{})
	bootCount := 0
	for _, item := range instr {
		if s, ok := item.(string); ok && strings.Contains(s, "Cosca KERNEL BOOT") {
			bootCount++
		}
	}
	if bootCount != 1 {
		t.Errorf("boot instruction must appear exactly once, got %d", bootCount)
	}

	// No .bak should be created on an idempotent no-op setup.
	if _, err := os.Stat(ConfigPath(cfg.ProjectDir) + ".bak"); err == nil {
		t.Error("unexpected .bak created during idempotent setup")
	}

	_ = first // first read only validates the file parses
}

func TestSetupBackupExistingConfig(t *testing.T) {
	t.Parallel()
	cfg := setupConfig(t)
	cfgPath := ConfigPath(cfg.ProjectDir)
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatal(err)
	}
	original := `{"default_agent":"general","custom":{"keep":true}}`
	if err := os.WriteFile(cfgPath, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	adapter := NewAdapter()
	if err := adapter.Setup(cfg); err != nil {
		t.Fatalf("Setup: %v", err)
	}

	backup, err := os.ReadFile(cfgPath + ".bak")
	if err != nil {
		t.Fatalf("backup not created: %v", err)
	}
	if string(backup) != original {
		t.Errorf("backup mismatch:\n got: %s\nwant: %s", backup, original)
	}
}

func TestSetupPreservesExistingKeys(t *testing.T) {
	t.Parallel()
	cfg := setupConfig(t)
	cfgPath := ConfigPath(cfg.ProjectDir)
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatal(err)
	}
	existing := `{"permission":{"read":{"^/home/": "allow"}},"instructions":"existing note"}`
	if err := os.WriteFile(cfgPath, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	adapter := NewAdapter()
	if err := adapter.Setup(cfg); err != nil {
		t.Fatalf("Setup: %v", err)
	}

	parsed, _ := readConfig(t, cfg.ProjectDir)

	// Existing keys survive.
	perm, ok := parsed["permission"].(map[string]interface{})
	if !ok {
		t.Error("existing permission key was lost")
	}
	if _, ok := perm["read"]; !ok {
		t.Error("existing permission.read was lost")
	}

	// Existing instructions are merged, not dropped.
	instr, _ := parsed["instructions"].([]interface{})
	var merged bool
	for _, item := range instr {
		if s, ok := item.(string); ok && s == "existing note" {
			merged = true
		}
	}
	if !merged {
		t.Error("existing instruction string was not merged into the list")
	}
}

func TestConfigPathUsesProjectDir(t *testing.T) {
	t.Parallel()
	// filepath.Join evita hardcodar o separador: no Windows o resultado é
	// "\\tmp\\proj\\.opencode\\opencode.json", não "/tmp/proj/...".
	want := filepath.Join("/tmp/proj", ".opencode", "opencode.json")
	got := ConfigPath("/tmp/proj")
	if got != want {
		t.Errorf("ConfigPath = %q, want %q", got, want)
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
