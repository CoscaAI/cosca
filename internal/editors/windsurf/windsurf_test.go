package windsurf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CoscaAI/cosca/internal/editors/types"
)

func TestSetupCreatesProjectRules(t *testing.T) {
	t.Parallel()
	proj := t.TempDir()
	windsurfHome := t.TempDir()

	a := NewAdapter()
	a.windsurfDir = windsurfHome // do not touch the real ~/.windsurf

	cfg := types.DefaultEditorConfig(proj)
	if err := a.Setup(cfg); err != nil {
		t.Fatalf("Setup: %v", err)
	}

	rulesPath := filepath.Join(proj, WindsurfRulesFileName)
	data, err := os.ReadFile(rulesPath)
	if err != nil {
		t.Fatalf(".windsurfrules not created: %v", err)
	}

	// Tests run in a temp dir (third-party project), so verify the
	// self-contained third-party content.
	content := string(data)
	if !strings.Contains(content, "Cosca Kernel") {
		t.Errorf(".windsurfrules must reference Cosca Kernel, got: %s", content)
	}
	if !strings.Contains(content, "consigliere") {
		t.Errorf(".windsurfrules must mention the consigliere persona, got: %s", content)
	}
	if !strings.Contains(content, "knowledge.db") {
		t.Errorf(".windsurfrules must reference knowledge.db, got: %s", content)
	}

	// The global MCP config must still be written (to the overridden dir).
	cfgPath := filepath.Join(windsurfHome, "config.json")
	if _, err := os.Stat(cfgPath); err != nil {
		t.Errorf("windsurf config.json not created: %v", err)
	}
}

func TestSetupProjectRulesIdempotent(t *testing.T) {
	t.Parallel()
	proj := t.TempDir()
	a := NewAdapter()
	a.windsurfDir = t.TempDir()

	cfg := types.DefaultEditorConfig(proj)
	if err := a.Setup(cfg); err != nil {
		t.Fatalf("first Setup: %v", err)
	}
	if err := a.Setup(cfg); err != nil {
		t.Fatalf("second Setup: %v", err)
	}

	rulesPath := filepath.Join(proj, WindsurfRulesFileName)
	data, err := os.ReadFile(rulesPath)
	if err != nil {
		t.Fatalf(".windsurfrules missing after second Setup: %v", err)
	}
	if count := strings.Count(string(data), "AO INICIAR"); count != 1 {
		t.Errorf(".windsurfrules duplicated: expected 1 boot directive, got %d", count)
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
