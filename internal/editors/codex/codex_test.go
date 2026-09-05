package codex

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CoscaAI/cosca/internal/editors/types"
)

func setupConfig(t *testing.T) types.EditorConfig {
	t.Helper()
	cfg := types.DefaultEditorConfig(t.TempDir())
	cfg.BackupExisting = true
	return cfg
}

func readCodexConfig(t *testing.T, projectDir string) (codexConfig, string) {
	t.Helper()
	cfgPath := filepath.Join(projectDir, ".codex", "config.json")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read %s: %v", cfgPath, err)
	}
	var cfg codexConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("parse %s: %v", cfgPath, err)
	}
	return cfg, string(data)
}

func readAgentsMD(t *testing.T, projectDir string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(projectDir, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	return string(data)
}

func TestSetupCreatesConfigAndAgentsMD(t *testing.T) {
	t.Parallel()
	cfg := setupConfig(t)
	adapter := NewAdapter()

	if err := adapter.Setup(cfg); err != nil {
		t.Fatalf("Setup: %v", err)
	}

	// config.json: boot instruction + all 5 tools.
	codexCfg, content := readCodexConfig(t, cfg.ProjectDir)

	// Tests run in a temp dir (third-party project), so verify the
	// self-contained third-party content.
	if !strings.Contains(codexCfg.Instructions, "Cosca Kernel") {
		t.Errorf("instructions must reference Cosca Kernel, got %q", codexCfg.Instructions)
	}
	if !strings.Contains(codexCfg.Instructions, "consigliere") {
		t.Errorf("instructions must describe the consigliere, got %q", codexCfg.Instructions)
	}
	if !strings.Contains(codexCfg.Instructions, "knowledge.db") {
		t.Errorf("instructions must reference knowledge.db, got %q", codexCfg.Instructions)
	}

	wantTools := map[string]bool{
		"cosca-search":  false,
		"cosca-index":   false,
		"cosca-context": false,
		"cosca-kernel":  false,
		"cosca-run":     false,
	}
	for _, tool := range codexCfg.Tools {
		if _, ok := wantTools[tool.Name]; ok {
			wantTools[tool.Name] = true
		}
		if tool.Name == "cosca-kernel" {
			if !strings.Contains(tool.Description, "KERNEL.md") {
				t.Errorf("cosca-kernel description must mention KERNEL.md, got %q", tool.Description)
			}
			if tool.Args != "kernel identity" {
				t.Errorf("cosca-kernel args = %q, want %q", tool.Args, "kernel identity")
			}
		}
		if tool.Name == "cosca-run" {
			if tool.Args != `run "$QUERY"` {
				t.Errorf("cosca-run args = %q, want run \"$QUERY\"", tool.Args)
			}
		}
	}
	for name, found := range wantTools {
		if !found {
			t.Errorf("tool %q missing from config.json", name)
		}
	}

	// AGENTS.md: self-contained third-party boot block present.
	agents := readAgentsMD(t, cfg.ProjectDir)
	for _, marker := range []string{"## Cosca Kernel — Boot Automático", "knowledge.db", "memory/", "consigliere"} {
		if !strings.Contains(agents, marker) {
			t.Errorf("AGENTS.md missing marker %q", marker)
		}
	}

	_ = content
}

func TestSetupIsIdempotent(t *testing.T) {
	t.Parallel()
	cfg := setupConfig(t)
	adapter := NewAdapter()

	if err := adapter.Setup(cfg); err != nil {
		t.Fatalf("Setup #1: %v", err)
	}
	if err := adapter.Setup(cfg); err != nil {
		t.Fatalf("Setup #2: %v", err)
	}

	codexCfg, _ := readCodexConfig(t, cfg.ProjectDir)

	// No duplicated tools.
	kernelCount := 0
	runCount := 0
	for _, tool := range codexCfg.Tools {
		if tool.Name == "cosca-kernel" {
			kernelCount++
		}
		if tool.Name == "cosca-run" {
			runCount++
		}
	}
	if kernelCount != 1 {
		t.Errorf("cosca-kernel tool must appear exactly once, got %d", kernelCount)
	}
	if runCount != 1 {
		t.Errorf("cosca-run tool must appear exactly once, got %d", runCount)
	}

	// AGENTS.md: boot block not duplicated.
	agents := readAgentsMD(t, cfg.ProjectDir)
	if strings.Count(agents, agentsBootMarker) != 1 {
		t.Errorf("AGENTS.md boot heading must appear exactly once, got %d",
			strings.Count(agents, agentsBootMarker))
	}
}

func TestSetupBackupExisting(t *testing.T) {
	t.Parallel()
	cfg := setupConfig(t)

	// Pre-create config.json.
	codexDir := filepath.Join(cfg.ProjectDir, ".codex")
	if err := os.MkdirAll(codexDir, 0o755); err != nil {
		t.Fatal(err)
	}
	codexFile := filepath.Join(codexDir, "config.json")
	originalCfg := `{"instructions":"old","tools":[{"name":"custom","description":"x","command":"y"}]}`
	if err := os.WriteFile(codexFile, []byte(originalCfg), 0o644); err != nil {
		t.Fatal(err)
	}

	// Pre-create AGENTS.md.
	agentsPath := filepath.Join(cfg.ProjectDir, "AGENTS.md")
	originalAgents := "# My Agents\n\nCustom instructions.\n"
	if err := os.WriteFile(agentsPath, []byte(originalAgents), 0o644); err != nil {
		t.Fatal(err)
	}

	adapter := NewAdapter()
	if err := adapter.Setup(cfg); err != nil {
		t.Fatalf("Setup: %v", err)
	}

	backup, err := os.ReadFile(codexFile + ".bak")
	if err != nil {
		t.Fatalf("config.json backup not created: %v", err)
	}
	if string(backup) != originalCfg {
		t.Errorf("config.json backup mismatch:\n got: %q\nwant: %q", backup, originalCfg)
	}

	agentsBackup, err := os.ReadFile(agentsPath + ".bak")
	if err != nil {
		t.Fatalf("AGENTS.md backup not created: %v", err)
	}
	if string(agentsBackup) != originalAgents {
		t.Errorf("AGENTS.md backup mismatch:\n got: %q\nwant: %q", agentsBackup, originalAgents)
	}

	// User content preserved.
	codexCfg, _ := readCodexConfig(t, cfg.ProjectDir)
	if !hasTool(codexCfg.Tools, "custom") {
		t.Error("pre-existing custom tool was lost")
	}
	if !strings.Contains(readAgentsMD(t, cfg.ProjectDir), "Custom instructions.") {
		t.Error("pre-existing AGENTS.md content was lost")
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

func TestTeardownRemovesCoscaTools(t *testing.T) {
	t.Parallel()
	cfg := setupConfig(t)
	adapter := NewAdapter()

	if err := adapter.Setup(cfg); err != nil {
		t.Fatalf("Setup: %v", err)
	}

	// Teardown uses cwd-relative paths; simulate by swapping ProjectDir.
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(cfg.ProjectDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWD) })

	if err := adapter.Teardown(); err != nil {
		t.Fatalf("Teardown: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(".", ".codex", "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	var teardownCfg codexConfig
	if err := json.Unmarshal(data, &teardownCfg); err != nil {
		t.Fatal(err)
	}
	for _, tool := range teardownCfg.Tools {
		if strings.HasPrefix(tool.Name, "cosca-") {
			t.Errorf("tool %q should have been removed by Teardown", tool.Name)
		}
	}
}
